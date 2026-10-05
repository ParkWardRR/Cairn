import type { Place } from './places'
import { clusterPlaces, mergeIntoSaved } from './places'
import type { PlaceResolver } from './placeResolver'

// The places as the app shows them: the visit history clustered, with every
// cluster inside a saved place folded into it, and saved places that have no
// visits yet added as places with no history.
export function buildPlaces(resolver: PlaceResolver): Place[] {
  const clusters = mergeIntoSaved(
    clusterPlaces(resolver.store.allVisits()),
    (lat, lon) => resolver.saved.match(lat, lon),
  )

  const places: Place[] = [...clusters]
  for (const s of resolver.saved.list()) {
    if (!clusters.some(c => resolver.saved.match(c.lat, c.lon)?.id === s.id)) {
      places.push({
        id: places.length, lat: s.lat, lon: s.lon,
        stops: 0, stop_seconds: 0, short: 0, medium: 0, long: 0,
        longest_s: 0, longest_category: null, arrivals: 0, departures: 0, trips: [], last_at: null,
      })
    }
  }
  return places
}
