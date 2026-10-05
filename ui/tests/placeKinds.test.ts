import { describe, expect, it } from 'vitest'
import { PLACE_KINDS, kindForCategory, kindFromLabel, placeIconSvg, placeKind, rankByKind } from '../shared/utils/placeKinds'
import { suggestHome } from '../server/utils/placeHints'
import type { Place } from '../server/utils/places'
import type { SavedPlace } from '../server/utils/placeSaved'

describe('place kinds', () => {
  it('gives every kind its own icon and label', () => {
    expect(new Set(PLACE_KINDS.map(k => k.id)).size).toBe(PLACE_KINDS.length)
    expect(new Set(PLACE_KINDS.map(k => k.paths.join('|'))).size).toBe(PLACE_KINDS.length)
    for (const k of PLACE_KINDS) expect(k.paths.length).toBeGreaterThan(0)
    for (const id of ['home', 'work', 'leisure', 'gym']) expect(PLACE_KINDS.some(k => k.id === id)).toBe(true)
  })

  it('maps what a lookup returns onto a kind', () => {
    const cases: Array<[string, string]> = [
      ['supermarket', 'groceries'],
      ['fast food', 'food'],
      ['cafe', 'food'],
      ['variety store', 'shopping'],
      ['mobile phone', 'shopping'],
      ['fitness centre', 'gym'],
      ['Gym', 'gym'],
      ['Home', 'home'],
      ['aerodrome', 'travel'],
      ['pharmacy', 'health'],
      ['park', 'leisure'],
      ['fuel', 'fuel'],
      ['social facility', 'other'],
      ['street', 'other'],
    ]
    for (const [cat, kind] of cases) expect(kindForCategory(cat), cat).toBe(kind)
  })

  it('prefers the specific kind over the broad one', () => {
    expect(kindForCategory('grocery store')).toBe('groceries') // not shopping
    expect(kindForCategory('health club')).toBe('gym') // not health
  })

  it('matches whole words only', () => {
    expect(kindForCategory('barber')).toBe('other') // not "bar"
    expect(kindForCategory(null, 'The Barn')).toBe('other')
  })

  it('does not let the name override a real category', () => {
    expect(kindForCategory('social facility', 'Westside Food Bank')).toBe('other')
    expect(kindForCategory('supermarket', 'Gym Snacks')).toBe('groceries')
    expect(kindForCategory('street', 'Planet Fitness')).toBe('gym') // a street is no category at all
  })

  it('falls back to the name when there is no category', () => {
    expect(kindForCategory(null, 'Planet Fitness')).toBe('gym')
    expect(kindForCategory('', 'Shell gas station')).toBe('fuel')
    expect(kindForCategory(null, null)).toBe('other')
  })

  it('finds a kind by the label the picker stores', () => {
    expect(kindFromLabel('Leisure')).toBe('leisure')
    expect(kindFromLabel(' gym ')).toBe('gym')
    expect(kindFromLabel('supermarket')).toBeNull()
  })

  it('renders an inline icon and falls back for an unknown kind', () => {
    const svg = placeIconSvg('home', { size: 20, color: '#fff' })
    expect(svg).toContain('width="20"')
    expect(svg).toContain('stroke="#fff"')
    expect(svg).toContain('<path')
    expect(placeIconSvg('nope')).toBe(placeIconSvg('other'))
    expect(placeKind('nope').id).toBe('other')
  })

  it('ranks names matching a kind hint first, keeping the rest in order', () => {
    const names = [
      { name: 'Chase', kind: 'shopping' },
      { name: 'Equinox', kind: 'gym' },
      { name: 'Ralphs', kind: 'groceries' },
      { name: '24 Hour Fitness', kind: 'gym' },
    ]
    expect(rankByKind(names, 'gym').map(n => n.name)).toEqual(['Equinox', '24 Hour Fitness', 'Chase', 'Ralphs'])
    expect(rankByKind(names, null)).toBe(names)
    expect(rankByKind(names, 'other')).toBe(names)
  })
})

describe('suggestHome', () => {
  const place = (id: number, arrivals: number, departures: number, trips: number, lat = 34 + id * 0.01): Place => ({
    id, lat, lon: -118.4, stops: 0, stop_seconds: 0, short: 0, medium: 0, long: 0, longest_s: 0, longest_category: null,
    arrivals, departures, trips: Array.from({ length: trips }, (_, i) => `t${i}`), last_at: null,
  })
  const saved = (name: string, category: string | null, lat: number, radius_m = 100): SavedPlace => ({
    id: 1, name, category, lat, lon: -118.4, radius_m, note: null, source: 'user', learned_from: null, created_at: 0, updated_at: 0,
  })

  it('suggests the spot trips most often start and end at', () => {
    expect(suggestHome([place(0, 1, 1, 2), place(1, 5, 2, 5), place(2, 3, 2, 4)], [])).toBe(1)
  })

  it('needs enough trips to stand out', () => {
    expect(suggestHome([place(0, 2, 1, 3)], [])).toBeNull() // only 3 endpoints
    expect(suggestHome([place(0, 3, 3, 2)], [])).toBeNull() // only 2 trips
  })

  it('stops suggesting once a Home exists', () => {
    expect(suggestHome([place(1, 5, 2, 5)], [saved('Home', null, 40)])).toBeNull()
    expect(suggestHome([place(1, 5, 2, 5)], [saved('My flat', 'Home', 40)])).toBeNull()
  })

  it('does not suggest somewhere already named as something else', () => {
    const p = place(1, 5, 2, 5, 34.01)
    expect(suggestHome([p], [saved('Gym', 'Gym', 34.01)])).toBeNull()
  })
})
