import type { PlaceVisit } from './places'
import { visitsFromStops } from './places'
import { buildPlaces } from './placeBuild'
import type { PlaceResolver } from './placeResolver'
import type { PlaceStore } from './placeStore'
import { detectStops, tripBaseMs, type StopFix } from './stops'

// Place data follows the trips in. Bundles are ingested, decoded and loaded into
// cairn-tsdb; this is the step after that, and it runs when the trips change
// rather than when someone opens a page:
//
//   1. every trip's stops and starts/ends are recorded in the visit history
//      (kept even if a trip later leaves the source data);
//   2. the places they form are named, so lookups are ready before they are seen;
//   3. a place that keeps coming up is learned, so it is known from then on.

export const LEARN_MIN_TRIPS = 3
export const LEARN_MIN_CONFIDENCE = 0.5

export interface IngestResult {
  skipped: boolean
  trips: number
  visits: number
}

let inflight: Promise<IngestResult> | null = null

// Changes whenever a trip is added or its positions change.
async function sourceToken(): Promise<string> {
  const r = await queryTsdbObjects(
    'SELECT count(*) AS n, count(DISTINCT boot_id) AS b, max(mono_ms) AS m, sum(mono_ms) AS s FROM position',
  )
  return JSON.stringify(r[0] ?? {})
}

// Records the visit history for every trip in the source data. Cheap when
// nothing has changed since the last run; single-flight when it has.
export function ingestPlaces(force = false): Promise<IngestResult> {
  if (!inflight) inflight = run(force).finally(() => { inflight = null })
  return inflight
}

async function run(force: boolean): Promise<IngestResult> {
  const resolver = getPlaceResolver()
  const token = await sourceToken()
  if (!force && resolver.store.getMeta('ingest_token') === token) {
    return { skipped: true, trips: resolver.store.visitTripCount(), visits: 0 }
  }

  const fixes = await queryTsdbObjects(`
    SELECT boot_id, lat, lon, speed_mps, mono_ms, observed_at
    FROM position
    WHERE lat != 0 AND lon != 0
    ORDER BY boot_id, mono_ms
  `)

  const { trips, visits } = recordVisits(resolver.store, fixes as FixRow[])

  resolver.store.setMeta('ingest_token', token)
  resolver.store.setMeta('ingested_at', new Date().toISOString())
  return { skipped: false, trips, visits }
}

export interface FixRow extends StopFix {
  boot_id: string
}

// Records every trip's stops and its start and end, replacing what was recorded
// for the trips in `fixes` (ordered by trip, then time) and leaving other trips
// alone.
export function recordVisits(store: PlaceStore, fixes: FixRow[]): { trips: number; visits: number } {
  const byBoot = new Map<string, FixRow[]>()
  for (const f of fixes) {
    const list = byBoot.get(f.boot_id)
    if (list) list.push(f)
    else byBoot.set(f.boot_id, [f])
  }

  let visits = 0
  for (const [bootId, list] of byBoot) {
    const base = tripBaseMs(list)
    const at = (f: FixRow) => (base != null ? new Date(base + f.mono_ms).toISOString() : null)
    const first = list[0]
    const last = list[list.length - 1]
    const v: PlaceVisit[] = [
      ...visitsFromStops(bootId, detectStops(list)),
      { boot_id: bootId, kind: 'departure', key_ms: first.mono_ms, lat: first.lat, lon: first.lon, at: at(first) },
      { boot_id: bootId, kind: 'arrival', key_ms: last.mono_ms, lat: last.lat, lon: last.lon, at: at(last) },
    ]
    store.replaceVisits(bootId, v)
    visits += v.length
  }
  return { trips: byBoot.size, visits }
}

// Names every place (queuing lookups for any not yet known) and saves as
// "learned" those that have come up on enough trips and have a confident,
// specific name. Returns how many it learned. Safe to call repeatedly.
export function learnPlaces(resolver: PlaceResolver, now = new Date()): number {
  let learned = 0
  for (const p of buildPlaces(resolver)) {
    // Looking up queues the name for any place not yet resolved.
    const r = resolver.lookup(p.lat, p.lon, { dwell_s: p.longest_s, endpoint: p.arrivals + p.departures > 0 })

    if (p.trips.length < LEARN_MIN_TRIPS) continue
    if (resolver.saved.match(p.lat, p.lon)) continue
    if (resolver.saved.isDismissed(p.lat, p.lon)) continue
    if (r.status !== 'resolved' || !r.name) continue
    // Streets and bare addresses are where you were, not somewhere you went.
    if (!r.category || r.category === 'street' || r.category === 'address') continue
    if (r.confidence < LEARN_MIN_CONFIDENCE) continue

    resolver.saved.create(
      { name: r.name, category: r.category.slice(0, 24), lat: p.lat, lon: p.lon },
      {
        source: 'learned',
        learned_from: {
          sources: r.sources,
          confidence: r.confidence,
          trips: p.trips.length,
          visits: p.stops + p.arrivals + p.departures,
          learned_at: now.toISOString(),
        },
      },
    )
    learned++
  }
  return learned
}
