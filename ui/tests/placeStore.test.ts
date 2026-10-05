import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { PlaceStore } from '../server/utils/placeStore'
import { PlaceResolver } from '../server/utils/placeResolver'

const base = { consulted: ['osm' as const], failed: [], resolved_at: 1, attempted_at: 1, version: 2, address: null }

describe('PlaceStore', () => {
  it('finds the nearest cached place within the radius only', () => {
    const s = new PlaceStore(':memory:')
    s.save({ lat: 34.0, lon: -118.4, candidates: [], ...base })
    expect(s.nearest(34.0002, -118.4001)).not.toBeNull() // ~25 m
    expect(s.nearest(34.001, -118.4)).toBeNull() // ~110 m
  })

  it('updates in place and keeps the id', () => {
    const s = new PlaceStore(':memory:')
    s.save({ lat: 34, lon: -118.4, candidates: [], ...base })
    const first = s.nearest(34, -118.4)!
    s.save({ ...first, candidates: [{ source: 'osm', name: 'X', category: null, kind: 'poi', lat: 34, lon: -118.4, dist_m: 1 }] })
    const again = s.nearest(34, -118.4)!
    expect(again.id).toBe(first.id)
    expect(again.candidates).toHaveLength(1)
  })

  it('counts source calls per day', () => {
    const s = new PlaceStore(':memory:')
    s.addCalls('geoapify', '2026-10-04', 2)
    s.addCalls('geoapify', '2026-10-04', 2)
    expect(s.callsToday('geoapify', '2026-10-04')).toBe(4)
    expect(s.callsToday('geoapify', '2026-10-05')).toBe(0)
  })

  it('imports a foursquare export once and queries it by box', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'fsq-'))
    const file = join(dir, 'fsq-pois.ndjson')
    writeFileSync(file, [
      { name: 'Ralphs', lat: 34.0001, lon: -118.4, category: 'Retail > Grocery Store' },
      { name: 'Far Away', lat: 35, lon: -118.4, category: null },
      { name: '', lat: 34, lon: -118.4 },
    ].map(r => JSON.stringify(r)).join('\n'))

    const s = new PlaceStore(':memory:')
    expect(await s.importFsq(file)).toBe(2)
    expect(await s.importFsq(file)).toBe(0) // unchanged file is skipped
    expect(s.fsqNear(34, -118.4, 90).map(r => r.name)).toEqual(['Ralphs'])
  })
})

describe('PlaceResolver (offline)', () => {
  it('resolves from the local foursquare data and caches the result', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'res-'))
    writeFileSync(join(dir, 'fsq-pois.ndjson'), JSON.stringify({ name: 'Ralphs', lat: 34.0001, lon: -118.4, category: 'Retail > Grocery Store' }))
    const r = new PlaceResolver({ dataDir: dir, external: false, geoapifyKey: '', overpassUrl: '' }, new PlaceStore(':memory:'))
    await r.ready

    const ctx = { dwell_s: 600, endpoint: false }
    expect(r.lookup(34, -118.4, ctx).status).toBe('pending')
    for (let i = 0; i < 50 && r.pendingCount(); i++) await new Promise(res => setTimeout(res, 20))

    const got = r.lookup(34, -118.4, ctx)
    expect(got.status).toBe('resolved')
    expect(got.name).toBe('Ralphs')
    expect(got.sources).toEqual(['fsq'])
  })

  it('reports none when no source is available', () => {
    const r = new PlaceResolver({ dataDir: ':memory:', external: false, geoapifyKey: '', overpassUrl: '' }, new PlaceStore(':memory:'))
    expect(r.lookup(34, -118.4, { dwell_s: 600, endpoint: false }).status).toBe('none')
  })
})
