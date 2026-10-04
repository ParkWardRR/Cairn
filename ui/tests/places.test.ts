import { describe, expect, it } from 'vitest'
import { clusterPlaces, visitsFromStops, type PlaceVisit } from '../server/utils/places'
import type { Stop } from '../server/utils/stops'

function stop(lat: number, lon: number, duration_s: number, category: Stop['category']): Stop {
  return { start_at: '2026-10-04T20:00:00.000Z', start_offset_s: 0, duration_s, lat, lon, category, inferred: false }
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
      { boot_id: 'a', kind: 'departure', lat: 34.0, lon: -118.4, at: null },
      { boot_id: 'b', kind: 'arrival', lat: 34.0001, lon: -118.4, at: null },
    ])
    expect(places).toHaveLength(1)
    expect(places[0].stops).toBe(0)
    expect(places[0].arrivals).toBe(1)
    expect(places[0].departures).toBe(1)
    expect(places[0].longest_category).toBeNull()
  })
})
