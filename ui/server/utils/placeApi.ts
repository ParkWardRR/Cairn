import type { PlaceResult } from './placeResolver'
import { attributionFor } from './placeLabel'
import { kindForCategory } from '../../shared/utils/placeKinds'

// The shape of a resolved place as the API returns it.
export function placeForApi(r: PlaceResult) {
  return {
    saved_id: r.saved_id ?? null,
    status: r.status,
    name: r.name,
    category: r.category,
    // Which icon to draw: the kind the user picked, or one worked out from the
    // category the lookup returned.
    kind: r.status === 'resolved' ? kindForCategory(r.category, r.name) : 'other',
    address: r.address,
    confidence: r.confidence,
    sources: r.sources,
  }
}

export function attributionForResults(results: PlaceResult[]): string[] {
  return attributionFor(results.flatMap(r => r.sources))
}
