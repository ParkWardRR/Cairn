export type StopCategory = 'quick' | 'medium' | 'long'

export interface Stop {
  // Wall-clock start, or null when the trip has no trustworthy UTC basis.
  start_at: string | null
  // Seconds from the trip's first fix; always available.
  start_offset_s: number
  duration_s: number
  lat: number
  lon: number
  category: StopCategory
  // True when the stop is inferred from a silence in GPS fixes between two
  // nearby points, rather than observed as a run of stationary fixes.
  inferred: boolean
}

export interface StopFix {
  lat: number
  lon: number
  speed_mps: number | null
  mono_ms: number
  observed_at: string | null
}

// Some bundles carry no UTC basis, so their fixes decode to 1970. Wall-clock
// time is therefore rebuilt as one base for the trip plus each fix's monotonic
// offset, taking the base from any fix whose own timestamp is plausible.
export function tripBaseMs(fixes: StopFix[]): number | null {
  for (const f of fixes) {
    if (!f.observed_at) continue
    const t = Date.parse(f.observed_at)
    if (Number.isFinite(t) && t > Date.parse('2020-01-01T00:00:00Z')) return t - f.mono_ms
  }
  return null
}

// Stop length buckets, in seconds. Under QUICK_MAX is a light, a drive-through
// or a pickup; under MEDIUM_MAX is an errand; beyond that the car was parked.
export const QUICK_MAX_S = 120
export const MEDIUM_MAX_S = 600

const MIN_STOP_S = 20
const STILL_RADIUS_M = 60
const STILL_SPEED_MPS = 1.5
// The device records sparsely while stationary, so a pause in fixes between two
// nearby points is itself evidence of a stop.
const SILENT_GAP_S = 120
const SILENT_GAP_RADIUS_M = 400

function distM(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const dx = (lon2 - lon1) * Math.cos(lat1 * Math.PI / 180) * 111320
  const dy = (lat2 - lat1) * 110540
  return Math.sqrt(dx * dx + dy * dy)
}

export function categorize(durationS: number): StopCategory {
  if (durationS < QUICK_MAX_S) return 'quick'
  if (durationS < MEDIUM_MAX_S) return 'medium'
  return 'long'
}

// Finds the places the vehicle stopped, from fixes ordered by mono_ms.
//
// The first and last fixes are the trip's own start and end, which the map
// already marks, so a stop that touches either end is not reported.
export function detectStops(fixes: StopFix[]): Stop[] {
  if (fixes.length < 3) return []
  const base = tripBaseMs(fixes)
  const firstMs = fixes[0].mono_ms

  interface Run { from: number; to: number; secs: number; inferred: boolean }
  const runs: Run[] = []
  let cur: Run | null = null

  for (let i = 1; i < fixes.length; i++) {
    const a = fixes[i - 1]
    const b = fixes[i]
    const dt = (b.mono_ms - a.mono_ms) / 1000
    if (dt <= 0) continue
    const dist = distM(a.lat, a.lon, b.lat, b.lon)

    const slow = (b.speed_mps ?? 0) < STILL_SPEED_MPS
    const observed = dist < STILL_RADIUS_M && (slow || dt > 20)
    const silent = dt >= SILENT_GAP_S && dist < SILENT_GAP_RADIUS_M

    if (observed || silent) {
      if (!cur) {
        cur = { from: i - 1, to: i, secs: 0, inferred: false }
        runs.push(cur)
      }
      cur.to = i
      cur.secs += dt
      if (silent && !observed) cur.inferred = true
    } else {
      cur = null
    }
  }

  const stops: Stop[] = []
  for (const r of runs) {
    if (r.secs < MIN_STOP_S) continue
    if (r.from === 0 || r.to === fixes.length - 1) continue

    const pts = fixes.slice(r.from, r.to + 1)
    const lat = pts.reduce((s, p) => s + p.lat, 0) / pts.length
    const lon = pts.reduce((s, p) => s + p.lon, 0) / pts.length
    stops.push({
      start_at: base != null ? new Date(base + fixes[r.from].mono_ms).toISOString() : null,
      start_offset_s: Math.round((fixes[r.from].mono_ms - firstMs) / 1000),
      duration_s: Math.round(r.secs),
      lat,
      lon,
      category: categorize(r.secs),
      inferred: r.inferred,
    })
  }
  return stops
}
