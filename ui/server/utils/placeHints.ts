import type { Place } from './places'
import { kindForCategory } from '../../shared/utils/placeKinds'
import type { SavedPlace } from './placeSaved'

// Things the engine can work out for itself and offer as a suggestion: it never
// applies them without being asked.

export const HOME_MIN_ENDPOINTS = 4
export const HOME_MIN_TRIPS = 3

// The place that looks most like home: the spot trips most often start from and
// end at, on enough different trips. Null once the user has marked a Home, or if
// nothing stands out. Returns the place id.
export function suggestHome(places: Place[], saved: SavedPlace[]): number | null {
  if (saved.some(s => kindForCategory(s.category, s.name) === 'home')) return null

  let best: Place | null = null
  for (const p of places) {
    const ends = p.arrivals + p.departures
    if (ends < HOME_MIN_ENDPOINTS || p.trips.length < HOME_MIN_TRIPS) continue
    // Somewhere you have already named something other than home is not home.
    if (saved.some(s => Math.hypot((s.lat - p.lat) * 110540, (s.lon - p.lon) * 92000) <= s.radius_m)) continue
    if (!best || ends > best.arrivals + best.departures) best = p
  }
  return best?.id ?? null
}
