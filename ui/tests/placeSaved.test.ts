import { describe, expect, it } from 'vitest'
import { SavedPlaceError, SavedPlaces, cleanSaved } from '../server/utils/placeSaved'
import { PlaceStore } from '../server/utils/placeStore'
import { PlaceResolver } from '../server/utils/placeResolver'

describe('SavedPlaces', () => {
  it('creates, updates and removes', () => {
    const s = new SavedPlaces(':memory:')
    const p = s.create({ name: '  Home ', lat: 34.0, lon: -118.4, category: 'Home' })
    expect(p).toMatchObject({ name: 'Home', category: 'Home', radius_m: 100 })

    const moved = s.update(p.id, { lat: 34.001, radius_m: 150 })!
    expect(moved).toMatchObject({ name: 'Home', lat: 34.001, radius_m: 150 })
    expect(s.update(999, { name: 'x' })).toBeNull()

    expect(s.remove(p.id)).toBe(true)
    expect(s.remove(p.id)).toBe(false)
    expect(s.list()).toEqual([])
  })

  it('matches by circle, nearest centre first', () => {
    const s = new SavedPlaces(':memory:')
    const mall = s.create({ name: 'Mall', lat: 34.0, lon: -118.4, radius_m: 300 })
    const cafe = s.create({ name: 'Cafe', lat: 34.0008, lon: -118.4, radius_m: 40 })
    expect(s.match(34.0008, -118.4)?.id).toBe(cafe.id) // inside both; the cafe is nearer
    expect(s.match(34.0, -118.4015)?.id).toBe(mall.id)
    expect(s.match(34.01, -118.4)).toBeNull()
  })

  it('rejects bad input with messages fit for the user', () => {
    expect(() => cleanSaved({ name: '   ', lat: 1, lon: 1 })).toThrow(SavedPlaceError)
    expect(() => cleanSaved({ name: 'x', lat: 91, lon: 1 })).toThrow(/lat/)
    expect(() => cleanSaved({ name: 'x', lat: 1, lon: 1, radius_m: 5 })).toThrow(/Radius/)
    expect(() => cleanSaved({ name: 'x'.repeat(81), lat: 1, lon: 1 })).toThrow(/80/)
    expect(cleanSaved({ category: '  ' }, true)).toEqual({ category: null })
  })
})

describe('resolver with saved places', () => {
  function make() {
    const saved = new SavedPlaces(':memory:')
    const r = new PlaceResolver({ dataDir: ':memory:', external: false, geoapifyKey: '', overpassUrl: '' }, new PlaceStore(':memory:'), saved)
    return { r, saved }
  }

  it('answers from a saved place with no lookup at all', () => {
    const { r, saved } = make()
    const p = saved.create({ name: 'Home', lat: 34.0, lon: -118.4, category: 'Home' })
    const got = r.lookup(34.0004, -118.4003, { dwell_s: 900, endpoint: false })
    expect(got).toMatchObject({ status: 'resolved', name: 'Home', category: 'Home', confidence: 1, sources: ['user'], saved_id: p.id })
    expect(r.pendingCount()).toBe(0)
  })

  it('applies a new name to past and future visits alike', () => {
    const { r, saved } = make()
    const ctx = { dwell_s: 900, endpoint: false }
    expect(r.lookup(34.0, -118.4, ctx).status).toBe('none')
    saved.create({ name: 'Gym', lat: 34.0, lon: -118.4 })
    expect(r.lookup(34.0, -118.4, ctx).name).toBe('Gym')
  })

  it('offers the cached names near a spot as suggestions, once each', () => {
    const { r } = make()
    r.store.save({
      lat: 34, lon: -118.4, consulted: ['osm'], failed: [], resolved_at: 1, attempted_at: 1, version: 2,
      address: { line: '1 Main St', street: 'Main St', formatted: null },
      candidates: [
        { source: 'osm', name: 'Ralphs', category: 'supermarket', kind: 'poi', lat: 34, lon: -118.4, dist_m: 30 },
        { source: 'fsq', name: 'Ralphs ', category: 'Grocery', kind: 'poi', lat: 34, lon: -118.4, dist_m: 35 },
        { source: 'osm', name: 'Main St', category: 'residential', kind: 'street', lat: 34, lon: -118.4, dist_m: 3 },
        { source: 'osm', name: 'Chase', category: 'bank', kind: 'poi', lat: 34, lon: -118.4, dist_m: 60 },
      ],
    })
    const names = r.suggestions(34, -118.4).map(s => s.name)
    expect(names).toEqual(['Ralphs', 'Chase', '1 Main St'])
  })
})
