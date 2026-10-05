import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { SavedPlaces } from '../server/utils/placeSaved'
import { PlaceStore } from '../server/utils/placeStore'
import { PlaceResolver } from '../server/utils/placeResolver'
import { learnPlaces, recordVisits, type FixRow } from '../server/utils/placeIngest'
import type { PlaceVisit } from '../server/utils/places'

function tmp() { return mkdtempSync(join(tmpdir(), 'saved-')) }

describe('SavedPlaces durability', () => {
  it('migrates a database from before sources existed', async () => {
    const { DatabaseSync } = await import('node:sqlite')
    const dir = tmp()
    const path = join(dir, 's.sqlite')
    const old = new DatabaseSync(path)
    old.exec(`CREATE TABLE saved_places (id INTEGER PRIMARY KEY, name TEXT NOT NULL, category TEXT, lat REAL NOT NULL, lon REAL NOT NULL,
      radius_m INTEGER NOT NULL, note TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`)
    old.prepare('INSERT INTO saved_places (name, lat, lon, radius_m, created_at, updated_at) VALUES (?,?,?,?,?,?)').run('Home', 34, -118.4, 100, 1, 1)
    old.close()

    const s = new SavedPlaces(path)
    expect(s.list()).toMatchObject([{ name: 'Home', source: 'user', learned_from: null }])
  })

  it('writes a JSON mirror after each change and rotates timestamped backups', () => {
    const dir = tmp()
    const s = new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir })
    s.create({ name: 'Home', lat: 34, lon: -118.4 })
    const mirror = JSON.parse(readFileSync(join(dir, 'saved-places.json'), 'utf8'))
    expect(mirror.saved).toHaveLength(1)
    expect(mirror.saved[0]).toMatchObject({ name: 'Home', source: 'user' })

    // A second change inside the hour updates the mirror but not the history.
    s.create({ name: 'Gym', lat: 34.01, lon: -118.4 })
    expect(JSON.parse(readFileSync(join(dir, 'saved-places.json'), 'utf8')).saved).toHaveLength(2)
    expect(readdirSync(join(dir, 'backups'))).toHaveLength(1)

    // An hour later a new copy is kept, and only the newest 30 survive.
    for (let i = 1; i <= 33; i++) s.snapshot(new Date(Date.now() + i * 3_700_000))
    const kept = readdirSync(join(dir, 'backups'))
    expect(kept).toHaveLength(30)
  })

  it('restores from the mirror when the database is lost', () => {
    const dir = tmp()
    const a = new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir })
    a.create({ name: 'Home', lat: 34, lon: -118.4, category: 'Home' })
    const g = a.create({ name: 'Gym', lat: 34.01, lon: -118.4 })
    a.remove(g.id)
    a.close()

    for (const f of readdirSync(dir).filter(f => f.startsWith('s.sqlite'))) rmSync(join(dir, f))
    const b = new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir })
    expect(b.list().map(p => p.name)).toEqual(['Home'])
    expect(b.isDismissed(34.01, -118.4)).toBe(true) // the rejection is restored too
  })

  it('does not overwrite a populated database from the mirror', () => {
    const dir = tmp()
    const a = new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir })
    a.create({ name: 'Home', lat: 34, lon: -118.4 })
    writeFileSync(join(dir, 'saved-places.json'), JSON.stringify({ version: 1, exported_at: '', saved: [{ name: 'Other', lat: 1, lon: 1, radius_m: 100 }], dismissed: [] }))
    a.close()
    expect(new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir }).list().map(p => p.name)).toEqual(['Home'])
  })

  it('reports a failed backup without losing the edit', () => {
    const errors: unknown[] = []
    const dir = tmp()
    const s = new SavedPlaces(join(dir, 's.sqlite'), { backupDir: '/proc/definitely/not/writable', onBackupError: e => errors.push(e) })
    s.create({ name: 'Home', lat: 34, lon: -118.4 })
    expect(s.list()).toHaveLength(1)
    expect(errors.length).toBeGreaterThan(0)
  })

  it('imports an export once, skipping what is already there', () => {
    const a = new SavedPlaces(':memory:')
    a.create({ name: 'Home', lat: 34, lon: -118.4 }, { source: 'learned', learned_from: { trips: 4 } })
    const out = a.exportAll()

    const b = new SavedPlaces(':memory:')
    expect(b.importAll(out)).toEqual({ added: 1, skipped: 0, dismissed: 0 })
    expect(b.importAll(out)).toEqual({ added: 0, skipped: 1, dismissed: 0 })
    expect(b.list()[0]).toMatchObject({ name: 'Home', source: 'learned' })
    expect(() => b.importAll({ nope: true })).toThrow(/export/)
  })

  it('makes an edited or confirmed learned place the user\'s, and remembers a removal', () => {
    const s = new SavedPlaces(':memory:')
    const p = s.create({ name: 'Ralphs', lat: 34, lon: -118.4 }, { source: 'learned' })
    expect(s.confirm(p.id)?.source).toBe('user')

    const q = s.create({ name: 'CVS', lat: 34.01, lon: -118.4 }, { source: 'learned' })
    expect(s.update(q.id, { name: 'CVS Pharmacy' })?.source).toBe('user')

    expect(s.isDismissed(34, -118.4)).toBe(false)
    s.remove(p.id)
    expect(s.isDismissed(34.0003, -118.4)).toBe(true)
  })
})

describe('learning repeat places', () => {
  function setup() {
    const saved = new SavedPlaces(':memory:')
    const store = new PlaceStore(':memory:')
    const r = new PlaceResolver({ dataDir: ':memory:', external: false, geoapifyKey: '', overpassUrl: '' }, store, saved)
    const visit = (boot: string, key: number, lat = 34, lon = -118.4): PlaceVisit =>
      ({ boot_id: boot, kind: 'stop', key_ms: key, lat, lon, at: null, duration_s: 600, category: 'medium' })
    const cache = (name: string, category: string, extra: Partial<Parameters<PlaceStore['save']>[0]> = {}) => store.save({
      lat: 34, lon: -118.4, consulted: ['osm'], failed: [], resolved_at: 1, attempted_at: 1, version: 2, address: null,
      candidates: [{ source: 'osm', name, category, kind: 'poi', lat: 34, lon: -118.4, dist_m: 10 }], ...extra,
    })
    return { saved, store, r, visit, cache }
  }

  it('learns a named place after enough trips, once', () => {
    const { saved, store, r, visit, cache } = setup()
    cache('Ralphs', 'supermarket')
    for (const b of ['a', 'b', 'c']) store.replaceVisits(b, [visit(b, 1)])

    expect(learnPlaces(r)).toBe(1)
    expect(saved.list()).toMatchObject([{ name: 'Ralphs', category: 'supermarket', source: 'learned' }])
    expect(JSON.parse(saved.list()[0].learned_from!)).toMatchObject({ trips: 3, sources: ['osm'] })
    expect(learnPlaces(r)).toBe(0)
  })

  it('waits for the third trip', () => {
    const { store, r, visit, cache } = setup()
    cache('Ralphs', 'supermarket')
    for (const b of ['a', 'b']) store.replaceVisits(b, [visit(b, 1)])
    expect(learnPlaces(r)).toBe(0)
  })

  it('does not learn a street, an address or an unnamed spot', () => {
    const { store, r, visit, cache } = setup()
    for (const b of ['a', 'b', 'c']) store.replaceVisits(b, [visit(b, 1)])
    cache('Main Street', 'street', { candidates: [{ source: 'osm', name: 'Main Street', category: 'residential', kind: 'street', lat: 34, lon: -118.4, dist_m: 3 }] })
    expect(learnPlaces(r)).toBe(0)
  })

  it('does not bring back a place the user removed', () => {
    const { saved, store, r, visit, cache } = setup()
    cache('Ralphs', 'supermarket')
    for (const b of ['a', 'b', 'c']) store.replaceVisits(b, [visit(b, 1)])
    learnPlaces(r)
    saved.remove(saved.list()[0].id)
    expect(learnPlaces(r)).toBe(0)
    expect(saved.list()).toEqual([])
  })

  it('leaves a place the user already named alone', () => {
    const { saved, store, r, visit, cache } = setup()
    cache('Ralphs', 'supermarket')
    saved.create({ name: 'Mom\'s store', lat: 34, lon: -118.4 })
    for (const b of ['a', 'b', 'c']) store.replaceVisits(b, [visit(b, 1)])
    expect(learnPlaces(r)).toBe(0)
  })
})

describe('recording the visit history', () => {
  const T0 = Date.parse('2026-10-04T20:00:00Z')
  function trip(boot: string): FixRow[] {
    const fix = (sec: number, lat: number, speed: number): FixRow => ({
      boot_id: boot, lat, lon: -118.4, speed_mps: speed, mono_ms: sec * 1000, observed_at: new Date(T0 + sec * 1000).toISOString(),
    })
    return [
      ...Array.from({ length: 30 }, (_, i) => fix(i, 34 + i * 15 / 110540, 15)),
      ...Array.from({ length: 31 }, (_, i) => fix(30 + i * 10, 34.004, 0.1)), // five minutes parked
      ...Array.from({ length: 30 }, (_, i) => fix(340 + i, 34.004 + i * 15 / 110540, 15)),
    ]
  }

  it('records a trip\'s stop, start and end, and is repeatable', () => {
    const store = new PlaceStore(':memory:')
    expect(recordVisits(store, trip('a'))).toEqual({ trips: 1, visits: 3 })
    expect(recordVisits(store, trip('a'))).toEqual({ trips: 1, visits: 3 })
    expect(store.allVisits().map(v => v.kind).sort()).toEqual(['arrival', 'departure', 'stop'])
    expect(store.visitTripCount()).toBe(1)
  })

  it('keeps the history of trips that are no longer in the source data', () => {
    const store = new PlaceStore(':memory:')
    recordVisits(store, trip('a'))
    recordVisits(store, trip('b'))
    recordVisits(store, trip('b')) // only b is in the data this time
    expect(store.visitTripCount()).toBe(2)
  })
})

describe('SavedPlaces startup mirror', () => {
  it('creates the mirror on first start', () => {
    const dir = tmp()
    new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir })
    expect(existsSync(join(dir, 'saved-places.json'))).toBe(true)
  })

  it('never replaces a mirror it could not read with an empty one', () => {
    const dir = tmp()
    writeFileSync(join(dir, 'saved-places.json'), '{ this is not json')
    new SavedPlaces(join(dir, 's.sqlite'), { backupDir: dir, onBackupError: () => {} })
    expect(readFileSync(join(dir, 'saved-places.json'), 'utf8')).toBe('{ this is not json')
  })
})
