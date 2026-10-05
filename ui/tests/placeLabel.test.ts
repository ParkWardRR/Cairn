import { describe, expect, it } from 'vitest'
import { attributionFor, chooseLabel, normalizeName, sameName, type PlaceCandidate } from '../server/utils/placeLabel'
import { assertOverpassOk, fsqCandidate, parseGeoapifyPlaces, parseGeoapifyReverse, parseOverpass } from '../server/utils/placeSources'

function cand(p: Partial<PlaceCandidate> & Pick<PlaceCandidate, 'source' | 'name' | 'dist_m'>): PlaceCandidate {
  return { category: null, kind: 'poi', lat: 0, lon: 0, ...p }
}

describe('names', () => {
  it('normalises and matches across spellings', () => {
    expect(normalizeName("Trader Joe's")).toBe('trader joe s')
    expect(sameName('The Home Depot', 'Home Depot #1234')).toBe(true)
    expect(sameName('Starbucks', 'Peet’s Coffee')).toBe(false)
  })
})

describe('chooseLabel', () => {
  const visit = { dwell_s: 900, endpoint: false }

  it('prefers the closer business and rewards independent agreement', () => {
    const l = chooseLabel([
      cand({ source: 'osm', name: 'Ralphs', dist_m: 20, category: 'supermarket' }),
      cand({ source: 'fsq', name: 'Ralphs', dist_m: 25, category: 'Grocery Store' }),
      cand({ source: 'osm', name: 'Chase Bank', dist_m: 60 }),
    ], null, visit)
    expect(l.name).toBe('Ralphs')
    expect(l.sources.sort()).toEqual(['fsq', 'osm'])
    expect(l.confidence).toBeGreaterThanOrEqual(0.75)
  })

  it('does not count osm and geoapify agreeing as independent', () => {
    const both = chooseLabel([
      cand({ source: 'osm', name: 'Ralphs', dist_m: 40 }),
      cand({ source: 'geoapify', name: 'Ralphs', dist_m: 40 }),
    ], null, visit)
    const withFsq = chooseLabel([
      cand({ source: 'osm', name: 'Ralphs', dist_m: 40 }),
      cand({ source: 'fsq', name: 'Ralphs', dist_m: 40 }),
    ], null, visit)
    expect(withFsq.confidence).toBeGreaterThan(both.confidence)
  })

  it('demotes generic parking names', () => {
    const l = chooseLabel([
      cand({ source: 'osm', name: 'Parking', dist_m: 5 }),
      cand({ source: 'osm', name: 'Costco Wholesale', dist_m: 55 }),
    ], null, visit)
    expect(l.name).toBe('Costco Wholesale')
  })

  it('names a short stop by its road, not the shop beside it', () => {
    const l = chooseLabel([
      cand({ source: 'osm', name: 'Chipotle', dist_m: 15 }),
      cand({ source: 'osm', name: 'Sepulveda Boulevard', kind: 'street', category: 'street', dist_m: 8 }),
    ], null, { dwell_s: 30, endpoint: false })
    expect(l.name).toBe('Sepulveda Boulevard')
    expect(l.category).toBe('street')
  })

  it('prefers a main road over an alley for a short stop', () => {
    const l = chooseLabel([
      cand({ source: 'osm', name: 'Venice Place North', kind: 'street', category: 'service', dist_m: 3 }),
      cand({ source: 'osm', name: 'Lincoln Boulevard', kind: 'street', category: 'primary', dist_m: 25 }),
    ], null, { dwell_s: 30, endpoint: false })
    expect(l.name).toBe('Lincoln Boulevard')
  })

  it('falls back to the address for a visit with no business nearby', () => {
    const l = chooseLabel([], { line: '123 Main St', street: 'Main St', formatted: '123 Main St, LA' }, visit)
    expect(l.name).toBe('123 Main St')
    expect(l.category).toBe('address')
  })

  it('returns nothing rather than inventing a name', () => {
    expect(chooseLabel([], null, visit).name).toBeNull()
  })

  it('ignores businesses beyond the radius', () => {
    const l = chooseLabel([cand({ source: 'osm', name: 'Far Cafe', dist_m: 300 })], null, visit)
    expect(l.name).toBeNull()
  })
})

describe('parsers', () => {
  it('parses overpass nodes, ways with centers, and streets', () => {
    const out = parseOverpass({
      elements: [
        { type: 'node', lat: 34.0001, lon: -118.4, tags: { name: 'Blue Bottle', amenity: 'cafe' } },
        { type: 'way', center: { lat: 34.0002, lon: -118.4 }, tags: { name: 'Sepulveda Blvd', highway: 'primary' } },
        { type: 'node', lat: 34, lon: -118.4, tags: { amenity: 'bench' } },
      ],
    }, 34, -118.4, false)
    expect(out).toHaveLength(2)
    expect(out[0]).toMatchObject({ name: 'Blue Bottle', category: 'cafe', kind: 'poi' })
    expect(out[1]).toMatchObject({ kind: 'street', category: 'primary' })
  })

  it('uses the bounding-box midpoint when a way has no centre', () => {
    const out = parseOverpass({
      elements: [
        { type: 'way', bounds: { minlat: 34, minlon: -118.4001, maxlat: 34.0002, maxlon: -118.3999 }, tags: { name: 'Venice Place North', highway: 'service' } },
        { type: 'way', bounds: { minlat: 34, minlon: -118.4001, maxlat: 34.0002, maxlon: -118.3999 }, tags: { name: 'Boardwalk', highway: 'footway' } },
      ],
    }, 34, -118.4, false)
    expect(out).toHaveLength(1) // the footway is not a road
    expect(out[0]).toMatchObject({ kind: 'street', category: 'service' })
  })

  it('measures a large building to its outline, not its centre', () => {
    const [b] = parseOverpass({
      elements: [{
        type: 'way', center: { lat: 34.002, lon: -118.4 },
        bounds: { minlat: 33.9995, minlon: -118.4005, maxlat: 34.0045, maxlon: -118.3995 },
        tags: { name: 'Costco Wholesale', shop: 'wholesale' },
      }],
    }, 34, -118.4, false)
    expect(b.dist_m).toBe(0) // the point is inside the building box
  })

  it('marks containing areas with zero distance', () => {
    const [a] = parseOverpass({
      elements: [{ type: 'relation', center: { lat: 33.94, lon: -118.4 }, tags: { name: 'Los Angeles International Airport', aeroway: 'aerodrome' } }],
    }, 33.95, -118.41, true)
    expect(a).toMatchObject({ kind: 'area', dist_m: 0 })
  })

  it('parses geoapify places and reverse results', () => {
    const [p] = parseGeoapifyPlaces({
      features: [{ properties: { name: 'Ralphs', lat: 34.01, lon: -118.4, distance: 31, categories: ['commercial', 'commercial.food_and_drink.supermarket'] } }],
    }, 34, -118.4)
    expect(p).toMatchObject({ source: 'geoapify', name: 'Ralphs', category: 'supermarket', dist_m: 31 })
    expect(parseGeoapifyReverse({ results: [{ housenumber: '5400', street: 'W Century Blvd', formatted: '5400 W Century Blvd, LA' }] }))
      .toEqual({ line: '5400 W Century Blvd', street: 'W Century Blvd', formatted: '5400 W Century Blvd, LA' })
    expect(parseGeoapifyReverse({ results: [] })).toBeNull()
  })

  it('takes the leaf of a foursquare category path', () => {
    const c = fsqCandidate({ name: 'Joe’s Pizza', lat: 34.0001, lon: -118.4, category: 'Dining and Drinking > Restaurant > Pizzeria' }, 34, -118.4)
    expect(c.category).toBe('Pizzeria')
    expect(c.source).toBe('fsq')
  })
})

describe('attribution', () => {
  it('credits each source that was used', () => {
    expect(attributionFor(['osm'])).toEqual(['© OpenStreetMap contributors'])
    expect(attributionFor(['geoapify', 'fsq'])).toEqual(['© OpenStreetMap contributors', 'Powered by Geoapify', 'Foursquare OS Places'])
    expect(attributionFor([])).toEqual([])
  })
})

describe('overpass response validation', () => {
  it('rejects an overloaded-server page and runtime-error remarks', () => {
    expect(() => assertOverpassOk(null)).toThrow(/busy/)
    expect(() => assertOverpassOk({})).toThrow(/busy/)
    expect(() => assertOverpassOk({ elements: [], remark: 'runtime error: Query timed out in "query" at line 3' })).toThrow(/runtime error/)
  })

  it('accepts a genuine empty answer', () => {
    expect(() => assertOverpassOk({ elements: [] })).not.toThrow()
  })
})
