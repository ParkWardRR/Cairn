import { describe, expect, it } from 'vitest'
import { clusterPlaces, mergeIntoSaved, visitsFromStops, type PlaceVisit } from '../server/utils/places'
import type { Stop } from '../server/utils/stops'

function stop(lat: number, lon: number, duration_s: number, category: Stop['category']): Stop {
  return { start_at: '2026-10-04T20:00:00.000Z', start_offset_s: 0, start_mono_ms: 0, end_mono_ms: 0, duration_s, lat, lon, category, inferred: false }
}

describe('clusterPlaces', () => {
  it('merges nearby stops across trips into one place', () => {
    const visits: PlaceVisit[] = [
      ...visitsFromStops('a', [stop(34.0, -118.4, 30, 'quick')]),
      ...visitsFromStops('b', [stop(34.0003, -118.4002, 900, 'long')]), // ~40 m away
    ]
    const places = clusterPlaces(visits)
    expect(places).toHaveLength(1)
    expect(places[0].stops).toBe(2)
    expect(places[0].quick).toBe(1)
    expect(places[0].long).toBe(1)
    expect(places[0].stop_seconds).toBe(930)
    expect(places[0].longest_category).toBe('long')
    expect(places[0].trips.sort()).toEqual(['a', 'b'])
  })

  it('keeps distant stops apart and ranks by total dwell', () => {
    const places = clusterPlaces([
      ...visitsFromStops('a', [stop(34.0, -118.4, 30, 'quick')]),
      ...visitsFromStops('a', [stop(34.05, -118.4, 1200, 'long')]),
    ])
    expect(places).toHaveLength(2)
    expect(places[0].longest_category).toBe('long')
    expect(places[0].id).toBe(0)
  })

  it('counts trip starts and ends without inventing stops', () => {
    const places = clusterPlaces([
      { boot_id: 'a', kind: 'departure', key_ms: 0, lat: 34.0, lon: -118.4, at: null },
      { boot_id: 'b', kind: 'arrival', key_ms: 9, lat: 34.0001, lon: -118.4, at: null },
    ])
    expect(places).toHaveLength(1)
    expect(places[0].stops).toBe(0)
    expect(places[0].arrivals).toBe(1)
    expect(places[0].departures).toBe(1)
    expect(places[0].longest_category).toBeNull()
  })
})

describe('mergeIntoSaved', () => {
  const home = { id: 7, lat: 34.0, lon: -118.4 }
  const match = (lat: number, lon: number) =>
    Math.hypot((lat - home.lat) * 110540, (lon - home.lon) * 92000) <= 250 ? home : null

  it('folds clusters inside a saved place into one, keeping the history', () => {
    const clusters = clusterPlaces([
      ...visitsFromStops('a', [stop(34.0, -118.4, 400, 'medium')]),
      ...visitsFromStops('b', [stop(34.0016, -118.4, 1500, 'long')]), // ~177 m: a separate cluster
      { boot_id: 'c', kind: 'arrival' as const, key_ms: 1, lat: 34.0001, lon: -118.4, at: '2026-10-04T00:00:00.000Z' },
      ...visitsFromStops('d', [stop(34.05, -118.4, 200, 'short')]),
    ])
    expect(clusters).toHaveLength(3)

    const merged = mergeIntoSaved(clusters, match)
    expect(merged).toHaveLength(2)
    const h = merged.find(p => p.lat === home.lat)!
    expect(h.stops).toBe(2)
    expect(h.stop_seconds).toBe(1900)
    expect(h.longest_category).toBe('long')
    expect(h.arrivals).toBe(1)
    expect(h.trips.sort()).toEqual(['a', 'b', 'c'])
  })

  it('leaves everything alone when nothing is saved', () => {
    const clusters = clusterPlaces(visitsFromStops('a', [stop(34.0, -118.4, 400, 'medium')]))
    expect(mergeIntoSaved(clusters, () => null)).toHaveLength(1)
  })
})
