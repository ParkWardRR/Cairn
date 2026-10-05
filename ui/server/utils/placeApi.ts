import type { PlaceResult } from './placeResolver'
import { attributionFor } from './placeLabel'

// The shape of a resolved place as the API returns it.
export function placeForApi(r: PlaceResult) {
  return {
    status: r.status,
    name: r.name,
    category: r.category,
    address: r.address,
    confidence: r.confidence,
    sources: r.sources,
  }
}

export function attributionForResults(results: PlaceResult[]): string[] {
  return attributionFor(results.flatMap(r => r.sources))
}
