import type { PlaceAddress, PlaceCandidate } from './placeLabel'
import { distM } from './placeLabel'

// Parsers (pure, unit-tested) and thin fetch wrappers for the external sources.

export const USER_AGENT = 'Cairn/1.0 (+https://github.com/ParkWardRR/Cairn; personal vehicle journal)'

// ---------- OpenStreetMap via Overpass ----------

const OSM_KEYS = ['amenity', 'shop', 'tourism', 'leisure', 'office', 'healthcare', 'aeroway', 'craft'] as const

export function overpassNearQuery(lat: number, lon: number, radiusM: number): string {
  const a = `(around:${radiusM},${lat},${lon})`
  const keys = OSM_KEYS.map(k => `nwr${a}["name"]["${k}"];`).join('\n  ')
  return `[out:json][timeout:25];
(
  ${keys}
  nwr${a}["name"]["railway"="station"];
  way(around:40,${lat},${lon})["highway"]["name"];
);
out center bb tags 80;`
}

// Large named areas that contain the point (an airport, a campus) are not
// matched by `around`, which measures to the outline.
export function overpassAreaQuery(lat: number, lon: number): string {
  return `[out:json][timeout:25];
is_in(${lat},${lon})->.a;
(
  nwr(pivot.a)["name"]["aeroway"="aerodrome"];
  nwr(pivot.a)["name"]["amenity"~"^(university|college|school|hospital)$"];
  nwr(pivot.a)["name"]["leisure"~"^(park|stadium|golf_course|sports_centre)$"];
  nwr(pivot.a)["name"]["landuse"~"^(retail|commercial|education|industrial)$"];
  nwr(pivot.a)["name"]["tourism"];
);
out center bb tags 20;`
}

interface OverpassElement {
  type: string
  lat?: number
  lon?: number
  center?: { lat: number; lon: number }
  bounds?: { minlat: number; minlon: number; maxlat: number; maxlon: number }
  tags?: Record<string, string>
}

// Paths and the like are named but are not where a car stops.
const NON_CAR_HIGHWAY = new Set(['footway', 'path', 'steps', 'cycleway', 'pedestrian', 'track', 'bridleway', 'corridor', 'bus_guideway'])

function distToBox(lat: number, lon: number, b: { minlat: number; minlon: number; maxlat: number; maxlon: number }): number {
  const clat = Math.min(Math.max(lat, b.minlat), b.maxlat)
  const clon = Math.min(Math.max(lon, b.minlon), b.maxlon)
  return distM(lat, lon, clat, clon)
}

function osmCategory(tags: Record<string, string>): string | null {
  for (const k of [...OSM_KEYS, 'railway', 'landuse'] as const) {
    if (tags[k]) return tags[k].replace(/_/g, ' ')
  }
  return null
}

export function parseOverpass(json: { elements?: OverpassElement[] }, lat: number, lon: number, area: boolean): PlaceCandidate[] {
  const out: PlaceCandidate[] = []
  for (const e of json.elements ?? []) {
    const name = e.tags?.name
    if (!name) continue
    // `out center bb` returns bounds without a centre for ways and relations.
    const p = e.type === 'node'
      ? { lat: e.lat, lon: e.lon }
      : e.center ?? (e.bounds ? { lat: (e.bounds.minlat + e.bounds.maxlat) / 2, lon: (e.bounds.minlon + e.bounds.maxlon) / 2 } : undefined)
    if (!p || p.lat == null || p.lon == null) continue
    const hw = e.tags?.highway
    if (hw && NON_CAR_HIGHWAY.has(hw) && !osmCategory(e.tags ?? {})) continue
    // Ways and relations match `around` by their outline, so measure to the
    // bounding box (0 when the point is inside it), not to a far-off centre.
    const dist = area ? 0 : e.type !== 'node' && e.bounds ? distToBox(lat, lon, e.bounds) : distM(lat, lon, p.lat, p.lon)
    const isStreet = !!e.tags?.highway && !osmCategory(e.tags)
    out.push({
      source: 'osm',
      name,
      // A street keeps its highway class so the label can prefer a main road.
      category: isStreet ? hw! : osmCategory(e.tags ?? {}),
      kind: isStreet ? 'street' : area ? 'area' : 'poi',
      lat: p.lat,
      lon: p.lon,
      dist_m: Math.round(dist),
    })
  }
  return out
}

// Overpass reports an overloaded server as an XML page with HTTP 200, and a query
// that ran out of time as JSON with a "remark". Neither is an empty result, and
// caching either as one would hide the place for good.
export function assertOverpassOk(json: unknown): asserts json is { elements: OverpassElement[]; remark?: string } {
  const j = json as { elements?: unknown; remark?: string } | null
  if (!j || typeof j !== 'object' || !Array.isArray(j.elements)) {
    throw new Error('Overpass returned a non-JSON or malformed response (server busy)')
  }
  if (j.remark && /runtime error/i.test(j.remark)) throw new Error(`Overpass: ${j.remark}`)
}

async function overpassOnce(url: string, query: string): Promise<any> {
  const res = await $fetch(url, {
    method: 'POST',
    headers: { 'User-Agent': USER_AGENT, Accept: 'application/json' },
    body: new URLSearchParams({ data: query }),
    timeout: 30_000,
    parseResponse: (text: string) => { try { return JSON.parse(text) } catch { return null } },
  })
  assertOverpassOk(res)
  return res
}

const sleep = (ms: number) => new Promise(r => setTimeout(r, ms))

// Tries each instance in turn. The public instance is often overloaded (504),
// and asks callers to pause 30 s after a 429 or 406, so back off rather than
// hammer it. Runs in a background queue, so waiting is fine.
export async function overpass(urls: string[], query: string): Promise<any> {
  let lastErr: any
  for (const url of urls) {
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        return await overpassOnce(url, query)
      } catch (e: any) {
        lastErr = e
        const code = e?.statusCode ?? e?.response?.status
        await sleep(code === 429 || code === 406 ? 30_000 : 5_000)
      }
    }
  }
  throw lastErr
}

export async function fetchOsm(urls: string[], lat: number, lon: number, wantArea: boolean): Promise<PlaceCandidate[]> {
  const near = parseOverpass(await overpass(urls, overpassNearQuery(lat, lon, 90)), lat, lon, false)
  if (!wantArea) return near
  // Space the second call out: the public instance asks for no parallel use.
  await sleep(1500)
  const area = parseOverpass(await overpass(urls, overpassAreaQuery(lat, lon)), lat, lon, true)
  return [...near, ...area]
}

// ---------- Geoapify ----------

const GEOAPIFY_CATEGORIES = [
  'commercial', 'catering', 'service', 'entertainment', 'leisure', 'tourism', 'healthcare',
  'education', 'accommodation', 'religion', 'sport', 'airport', 'public_transport', 'office',
].join(',')

function geoCategory(cats: unknown): string | null {
  if (!Array.isArray(cats) || !cats.length) return null
  // Most specific wins: "commercial.food_and_drink.supermarket" -> "supermarket".
  const best = [...cats].sort((a, b) => String(b).split('.').length - String(a).split('.').length)[0]
  return String(best).split('.').pop()!.replace(/_/g, ' ')
}

export function parseGeoapifyPlaces(json: { features?: any[] }, lat: number, lon: number): PlaceCandidate[] {
  const out: PlaceCandidate[] = []
  for (const f of json.features ?? []) {
    const p = f.properties ?? {}
    if (!p.name) continue
    const plat = p.lat ?? f.geometry?.coordinates?.[1]
    const plon = p.lon ?? f.geometry?.coordinates?.[0]
    if (plat == null || plon == null) continue
    out.push({
      source: 'geoapify',
      name: p.name,
      category: geoCategory(p.categories),
      kind: 'poi',
      lat: plat,
      lon: plon,
      dist_m: Math.round(p.distance ?? distM(lat, lon, plat, plon)),
    })
  }
  return out
}

export function parseGeoapifyReverse(json: { results?: any[] }): PlaceAddress | null {
  const r = json.results?.[0]
  if (!r) return null
  const line = [r.housenumber, r.street].filter(Boolean).join(' ') || null
  return { line, street: r.street ?? null, formatted: r.formatted ?? null }
}

export async function fetchGeoapify(key: string, lat: number, lon: number): Promise<{ candidates: PlaceCandidate[]; address: PlaceAddress | null }> {
  const places = await $fetch<any>('https://api.geoapify.com/v2/places', {
    query: {
      categories: GEOAPIFY_CATEGORIES,
      filter: `circle:${lon},${lat},90`,
      bias: `proximity:${lon},${lat}`,
      limit: 20,
      apiKey: key,
    },
    headers: { 'User-Agent': USER_AGENT },
    timeout: 20_000,
  })
  const reverse = await $fetch<any>('https://api.geoapify.com/v1/geocode/reverse', {
    query: { lat, lon, format: 'json', limit: 1, apiKey: key },
    headers: { 'User-Agent': USER_AGENT },
    timeout: 20_000,
  })
  return { candidates: parseGeoapifyPlaces(places, lat, lon), address: parseGeoapifyReverse(reverse) }
}

// ---------- Foursquare OS Places (local dataset) ----------

export interface FsqRow {
  name: string
  lat: number
  lon: number
  category: string | null
}

export function fsqCandidate(row: FsqRow, lat: number, lon: number): PlaceCandidate {
  // fsq_category_labels look like "Dining and Drinking > Restaurant > Pizzeria".
  const leaf = row.category ? row.category.split('>').pop()!.trim() : null
  return {
    source: 'fsq',
    name: row.name,
    category: leaf,
    kind: 'poi',
    lat: row.lat,
    lon: row.lon,
    dist_m: Math.round(distM(lat, lon, row.lat, row.lon)),
  }
}
