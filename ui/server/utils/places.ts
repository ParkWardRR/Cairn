import type { Stop, StopCategory } from './stops'
import { categorize } from './stops'

// A place is somewhere the car has stopped or started/ended a trip, merged
// across trips: fixes within PLACE_RADIUS_M of a place's centre join it.
export const PLACE_RADIUS_M = 150

export interface PlaceVisit {
  boot_id: string
  kind: 'stop' | 'arrival' | 'departure'
  // A monotonic time that identifies the visit within its trip.
  key_ms: number
  lat: number
  lon: number
  // Wall-clock time, or null when the trip has no trustworthy UTC basis.
  at: string | null
  duration_s?: number
  category?: StopCategory
  inferred?: boolean
}

export interface Place {
  id: number
  lat: number
  lon: number
  stops: number
  stop_seconds: number
  quick: number
  medium: number
  long: number
  longest_s: number
  longest_category: StopCategory | null
  arrivals: number
  departures: number
  trips: string[]
  last_at: string | null
}

function distM(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const dx = (lon2 - lon1) * Math.cos(lat1 * Math.PI / 180) * 111320
  const dy = (lat2 - lat1) * 110540
  return Math.sqrt(dx * dx + dy * dy)
}

export function visitsFromStops(bootId: string, stops: Stop[]): PlaceVisit[] {
  return stops.map(s => ({
    boot_id: bootId,
    kind: 'stop' as const,
    key_ms: s.start_mono_ms,
    lat: s.lat,
    lon: s.lon,
    at: s.start_at,
    duration_s: s.duration_s,
    category: s.category,
    inferred: s.inferred,
  }))
}

// Greedy clustering: each visit joins the nearest place within PLACE_RADIUS_M,
// moving that place's centre to the mean of its visits, or starts a new one.
// Visits are processed in the order given, so the result is deterministic.
export function clusterPlaces(visits: PlaceVisit[]): Place[] {
  interface Acc extends Place { n: number; latSum: number; lonSum: number }
  const accs: Acc[] = []

  for (const v of visits) {
    let best: Acc | null = null
    let bestD = PLACE_RADIUS_M
    for (const a of accs) {
      const d = distM(a.lat, a.lon, v.lat, v.lon)
      if (d <= bestD) { best = a; bestD = d }
    }
    if (!best) {
      best = {
        id: accs.length, lat: v.lat, lon: v.lon, stops: 0, stop_seconds: 0,
        quick: 0, medium: 0, long: 0, longest_s: 0, longest_category: null,
        arrivals: 0, departures: 0, trips: [], last_at: null,
        n: 0, latSum: 0, lonSum: 0,
      }
      accs.push(best)
    }

    best.n++
    best.latSum += v.lat
    best.lonSum += v.lon
    best.lat = best.latSum / best.n
    best.lon = best.lonSum / best.n
    if (!best.trips.includes(v.boot_id)) best.trips.push(v.boot_id)
    if (v.at && (!best.last_at || v.at > best.last_at)) best.last_at = v.at

    if (v.kind === 'arrival') best.arrivals++
    else if (v.kind === 'departure') best.departures++
    else {
      const secs = v.duration_s ?? 0
      best.stops++
      best.stop_seconds += secs
      const cat = v.category ?? categorize(secs)
      best[cat]++
      if (secs > best.longest_s) { best.longest_s = secs; best.longest_category = cat }
    }
  }

  return accs
    .map(({ n: _n, latSum: _a, lonSum: _b, ...place }) => place)
    .sort((a, b) => b.stop_seconds - a.stop_seconds || (b.arrivals + b.departures) - (a.arrivals + a.departures))
    .map((p, i) => ({ ...p, id: i }))
}

// Folds every auto-detected place that lies inside a saved place into that one
// place, centred where the user put it. This is what makes repeat visits to
// somewhere you have named read as one place with a history, however the
// automatic clustering happened to split them.
export function mergeIntoSaved(
  clusters: Place[],
  match: (lat: number, lon: number) => { id: number; lat: number; lon: number } | null,
): Place[] {
  const free: Place[] = []
  const bySaved = new Map<number, Place>()

  for (const c of clusters) {
    const s = match(c.lat, c.lon)
    if (!s) { free.push(c); continue }
    const m = bySaved.get(s.id)
    if (!m) {
      bySaved.set(s.id, { ...c, lat: s.lat, lon: s.lon, trips: [...c.trips] })
      continue
    }
    m.stops += c.stops
    m.stop_seconds += c.stop_seconds
    m.short += c.short
    m.medium += c.medium
    m.long += c.long
    m.arrivals += c.arrivals
    m.departures += c.departures
    for (const t of c.trips) if (!m.trips.includes(t)) m.trips.push(t)
    if (c.longest_s > m.longest_s) { m.longest_s = c.longest_s; m.longest_category = c.longest_category }
    if (c.last_at && (!m.last_at || c.last_at > m.last_at)) m.last_at = c.last_at
  }

  return [...bySaved.values(), ...free]
    .sort((a, b) => b.stop_seconds - a.stop_seconds || (b.arrivals + b.departures) - (a.arrivals + a.departures))
    .map((p, i) => ({ ...p, id: i }))
}
