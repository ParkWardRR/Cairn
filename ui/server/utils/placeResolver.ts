import { join } from 'node:path'
import type { PlaceAddress, PlaceCandidate, PlaceContext, PlaceLabel, PlaceSource } from './placeLabel'
import { FETCH_VERSION, VISIT_MIN_S, chooseLabel, normalizeName } from './placeLabel'
import { kindForCategory } from '../../shared/utils/placeKinds'
import { PlaceStore, type CachedPlace } from './placeStore'
import { SavedPlaces } from './placeSaved'
import { fetchGeoapify, fetchOsm, fsqCandidate } from './placeSources'

// Resolves coordinates to names. Lookups never block a request: a cache miss
// queues a background fetch and reports "pending"; the page asks again shortly.

export interface PlaceResult extends PlaceLabel {
  status: 'resolved' | 'pending' | 'none'
  // Set when the name is one the user saved rather than one that was looked up.
  saved_id?: number
}

export interface PlaceSuggestion {
  name: string
  category: string | null
  // What kind of place the name looks like, so the picker can rank by a kind hint.
  kind: string
  source: PlaceSource | 'address'
  dist_m: number
}

const RETRY_FAILED_MS = 10 * 60 * 1000
const NEGATIVE_TTL_MS = 30 * 24 * 60 * 60 * 1000
const OSM_SPACING_MS = 3000
const GEOAPIFY_DAILY_BUDGET = 2400 // free plan is 3000 credits/day; a place costs 2

export interface ResolverConfig {
  dataDir: string
  external: boolean
  geoapifyKey: string
  overpassUrl: string
  fsqFile?: string
}

interface Job { lat: number; lon: number; wantArea: boolean; key: string }

export class PlaceResolver {
  readonly store: PlaceStore
  readonly saved: SavedPlaces
  private queue: Job[] = []
  private queued = new Set<string>()
  private running = false
  private lastOsm = 0
  readonly ready: Promise<void>

  constructor(readonly cfg: ResolverConfig, store?: PlaceStore, saved?: SavedPlaces) {
    this.store = store ?? new PlaceStore(join(cfg.dataDir, 'places.sqlite'))
    // Saved places are written out as JSON beside the database after every change.
    this.saved = saved ?? (cfg.dataDir === ':memory:'
      ? new SavedPlaces(':memory:')
      : new SavedPlaces(join(cfg.dataDir, 'saved-places.sqlite'), { backupDir: cfg.dataDir }))
    const file = cfg.fsqFile ?? join(cfg.dataDir, 'fsq-pois.ndjson')
    this.ready = this.store.importFsq(file).then(
      (n) => { if (n) console.log(`[places] loaded ${n} Foursquare POIs`) },
      (e) => console.error('[places] Foursquare import failed:', e),
    )
  }

  enabledSources(): PlaceSource[] {
    const s: PlaceSource[] = []
    if (this.store.fsqCount() > 0) s.push('fsq')
    if (this.cfg.external) {
      s.push('osm')
      if (this.cfg.geoapifyKey) s.push('geoapify')
    }
    return s
  }

  // Non-blocking: returns what is known now and queues a lookup if needed.
  lookup(lat: number, lon: number, ctx: PlaceContext): PlaceResult {
    // A place the user named wins over anything looked up, for every visit past
    // and future inside its circle.
    const mine = this.saved.match(lat, lon)
    if (mine) {
      return { status: 'resolved', name: mine.name, category: mine.category, address: null, confidence: 1, sources: ['user'], saved_id: mine.id }
    }

    const cached = this.store.nearest(lat, lon)
    const now = Date.now()

    if (cached) {
      const missing = this.enabledSources().filter(s => !cached.consulted.includes(s))
      const retryable = missing.length > 0 && now - cached.attempted_at > RETRY_FAILED_MS
      const expired = cached.candidates.length === 0 && !cached.address && now - cached.resolved_at > NEGATIVE_TTL_MS
      const outdated = cached.version < FETCH_VERSION
      if (retryable || expired || outdated) this.enqueue(lat, lon, ctx)
      const result = this.label(cached, ctx)
      // Nothing yet, but a source failed and will be retried: keep callers waiting.
      if (result.status === 'none' && (missing.length > 0 || cached.failed.length > 0) && this.enabledSources().some(s => !cached.consulted.includes(s))) {
        return { ...result, status: 'pending' }
      }
      return result
    }
    if (this.enabledSources().length === 0) return { status: 'none', name: null, category: null, address: null, confidence: 0, sources: [] }
    this.enqueue(lat, lon, ctx)
    return { status: 'pending', name: null, category: null, address: null, confidence: 0, sources: [] }
  }

  // Names the sources found near a spot, for the user to pick from when fixing it.
  suggestions(lat: number, lon: number, limit = 8): PlaceSuggestion[] {
    const row = this.store.nearest(lat, lon)
    if (!row) return []
    const seen = new Set<string>()
    const out: PlaceSuggestion[] = []
    for (const c of [...row.candidates].filter(c => c.kind !== 'street' && c.name.trim()).sort((a, b) => a.dist_m - b.dist_m)) {
      const key = normalizeName(c.name)
      if (!key || seen.has(key)) continue
      seen.add(key)
      out.push({ name: c.name.trim(), category: c.category, kind: kindForCategory(c.category, c.name), source: c.source, dist_m: c.dist_m })
      if (out.length >= limit) break
    }
    const addr = row.address?.line ?? row.address?.formatted
    if (addr && out.length < limit) out.push({ name: addr, category: 'address', kind: 'other', source: 'address', dist_m: 0 })
    return out
  }

  private label(row: CachedPlace, ctx: PlaceContext): PlaceResult {
    const l = chooseLabel(row.candidates, row.address, ctx)
    return { ...l, status: l.name ? 'resolved' : 'none' }
  }

  private enqueue(lat: number, lon: number, ctx: PlaceContext) {
    const key = `${lat.toFixed(4)},${lon.toFixed(4)}`
    if (this.queued.has(key)) return
    this.queued.add(key)
    this.queue.push({ lat, lon, wantArea: ctx.endpoint || ctx.dwell_s >= VISIT_MIN_S, key })
    void this.drain()
  }

  pendingCount(): number { return this.queue.length + (this.running ? 1 : 0) }

  private async drain() {
    if (this.running) return
    this.running = true
    try {
      await this.ready
      while (this.queue.length) {
        const job = this.queue.shift()!
        try { await this.resolve(job) } catch (e) { console.error('[places] resolve failed:', e) }
        this.queued.delete(job.key)
      }
    } finally {
      this.running = false
    }
  }

  private async resolve(job: Job) {
    const { lat, lon } = job
    const now = Date.now()
    const day = new Date(now).toISOString().slice(0, 10)
    const existing = this.store.nearest(lat, lon)
    const candidates: PlaceCandidate[] = []
    let address: PlaceAddress | null = existing?.address ?? null
    const consulted: PlaceSource[] = []
    const failed: PlaceSource[] = []

    if (this.store.fsqCount() > 0) {
      for (const r of this.store.fsqNear(lat, lon, 90)) candidates.push(fsqCandidate(r, lat, lon))
      consulted.push('fsq')
    }

    if (this.cfg.external) {
      const wait = this.lastOsm + OSM_SPACING_MS - Date.now()
      if (wait > 0) await new Promise(r => setTimeout(r, wait))
      try {
        candidates.push(...await fetchOsm(this.cfg.overpassUrl.split(',').map(u => u.trim()).filter(Boolean), lat, lon, job.wantArea))
        consulted.push('osm')
      } catch (e: any) {
        failed.push('osm')
        console.error('[places] overpass failed:', e?.message ?? e)
      }
      this.lastOsm = Date.now()

      if (this.cfg.geoapifyKey) {
        if (this.store.callsToday('geoapify', day) + 2 > GEOAPIFY_DAILY_BUDGET) {
          failed.push('geoapify')
        } else {
          try {
            const g = await fetchGeoapify(this.cfg.geoapifyKey, lat, lon)
            this.store.addCalls('geoapify', day, 2)
            candidates.push(...g.candidates)
            address = g.address ?? address
            consulted.push('geoapify')
          } catch (e: any) {
            failed.push('geoapify')
            console.error('[places] geoapify failed:', e?.message ?? e)
          }
        }
      }
    }

    // Keep what an earlier, richer lookup found if this one lost a source.
    const keep = existing?.candidates.filter(c => !consulted.includes(c.source)) ?? []
    const keptSources = (existing?.consulted ?? []).filter(s => !consulted.includes(s))
    this.store.save({
      id: existing?.id,
      lat: existing?.lat ?? lat,
      lon: existing?.lon ?? lon,
      candidates: [...keep, ...candidates],
      address,
      consulted: [...consulted, ...keptSources],
      failed,
      resolved_at: now,
      attempted_at: now,
      version: FETCH_VERSION,
    })
  }
}

let singleton: PlaceResolver | null = null

export function getPlaceResolver(): PlaceResolver {
  if (!singleton) {
    const c = useRuntimeConfig()
    singleton = new PlaceResolver({
      dataDir: String(c.placesDataDir),
      external: String(c.placesExternal) !== 'false',
      geoapifyKey: String(c.geoapifyKey ?? ''),
      overpassUrl: String(c.overpassUrl),
    })
  }
  return singleton
}

// Resolves a list of points and reports whether any are still pending.
export function lookupPlaces(
  points: Array<{ lat: number; lon: number; ctx: PlaceContext }>,
): { results: PlaceResult[]; pending: boolean } {
  const r = getPlaceResolver()
  const results = points.map(p => r.lookup(p.lat, p.lon, p.ctx))
  return { results, pending: results.some(x => x.status === 'pending') }
}
