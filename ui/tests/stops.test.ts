import { describe, expect, it } from 'vitest'
import { categorize, detectStops, type StopFix } from '../server/utils/stops'

const T0 = Date.parse('2026-10-04T20:00:00Z')

function fix(sec: number, lat: number, lon: number, speed: number): StopFix {
  return { lat, lon, speed_mps: speed, mono_ms: sec * 1000, observed_at: new Date(T0 + sec * 1000).toISOString() }
}

// Driving north at 15 m/s, one fix per second.
function drive(from: number, secs: number, lat0: number): StopFix[] {
  return Array.from({ length: secs }, (_, i) => fix(from + i, lat0 + (i * 15) / 110540, -118.4, 15))
}

describe('categorize', () => {
  it('buckets by duration', () => {
    expect(categorize(30)).toBe('quick')
    expect(categorize(119)).toBe('quick')
    expect(categorize(120)).toBe('medium')
    expect(categorize(599)).toBe('medium')
    expect(categorize(600)).toBe('long')
  })
})

describe('detectStops', () => {
  it('rebuilds wall-clock from one base when later fixes decode to 1970', () => {
    const lat = 34.0 + 60 * 15 / 110540
    const fixes = [
      ...drive(0, 60, 34.0),
      ...Array.from({ length: 31 }, (_, i) => fix(60 + i * 10, lat, -118.4, 0.1)),
      ...drive(380, 60, lat),
    ].map(f => (f.mono_ms >= 60_000 ? { ...f, observed_at: new Date(f.mono_ms).toISOString() } : f))
    const [stop] = detectStops(fixes)
    expect(stop.start_at).toBe(new Date(T0 + 59_000).toISOString())
    expect(stop.start_offset_s).toBe(59)
  })

  it('finds a stationary run between two drives', () => {
    const lat = 34.0 + 60 * 15 / 110540
    const fixes = [
      ...drive(0, 60, 34.0),
      ...Array.from({ length: 31 }, (_, i) => fix(60 + i * 10, lat, -118.4, 0.1)), // 5 min parked
      ...drive(380, 60, lat),
    ]
    const stops = detectStops(fixes)
    expect(stops).toHaveLength(1)
    expect(stops[0].category).toBe('medium')
    expect(stops[0].inferred).toBe(false)
    expect(stops[0].duration_s).toBeGreaterThanOrEqual(290)
  })

  it('infers a stop from a long silence between nearby fixes', () => {
    const lat = 34.0 + 60 * 15 / 110540
    const fixes = [
      ...drive(0, 60, 34.0),
      ...drive(60 + 2160, 60, lat + 130 / 110540), // 36 min later, 130 m on
    ]
    // Needs fixes on both sides, so add a trailing drive past the stop.
    const stops = detectStops([...fixes, ...drive(2400, 30, lat + 2000 / 110540)])
    expect(stops).toHaveLength(1)
    expect(stops[0].category).toBe('long')
    expect(stops[0].inferred).toBe(true)
  })

  it('ignores a stop at the very start or end of the trip', () => {
    const fixes = [
      ...Array.from({ length: 20 }, (_, i) => fix(i * 10, 34.0, -118.4, 0)),
      ...drive(200, 60, 34.0),
    ]
    expect(detectStops(fixes)).toHaveLength(0)
  })

  it('does not treat steady driving as a stop', () => {
    expect(detectStops(drive(0, 300, 34.0))).toHaveLength(0)
  })
})
