// Turns candidate names from several sources into one label for a place.
//
// Pure functions only: no I/O, so the scoring is unit-tested and can be re-run
// over cached candidates when the rules change.

export type PlaceSource = 'fsq' | 'osm' | 'geoapify' | 'user'

export interface PlaceCandidate {
  source: PlaceSource
  name: string
  // A human category such as "Restaurant" or "supermarket"; null when unknown.
  category: string | null
  // poi: a business or amenity near the point. area: something large that
  // contains the point (airport, campus). street: a named road next to it.
  kind: 'poi' | 'area' | 'street'
  lat: number
  lon: number
  dist_m: number
}

export interface PlaceAddress {
  // Street and number, e.g. "5400 W Century Blvd". May be partial.
  line: string | null
  street: string | null
  formatted: string | null
}

export interface PlaceContext {
  // Longest dwell seen at the place, in seconds (0 for a bare trip start/end).
  dwell_s: number
  // True when a trip starts or ends here.
  endpoint: boolean
}

export interface PlaceLabel {
  name: string | null
  category: string | null
  address: string | null
  confidence: number
  sources: PlaceSource[]
}

// Bump when the scoring rules change; cached rows are re-scored, not re-fetched.
export const LABEL_VERSION = 1

// Bump when what is fetched or stored per place changes shape (as opposed to how
// it is scored): cached rows from an older version are fetched again.
export const FETCH_VERSION = 2

const POI_RADIUS_M = 90
const STREET_RADIUS_M = 40
// Stops are 3 minutes or longer (see MIN_STOP_S), so every stop is a visit.
export const VISIT_MIN_S = 180

const SOURCE_WEIGHT: Record<PlaceSource, number> = { fsq: 1.0, osm: 0.95, geoapify: 0.85 }
// Geoapify's places are built largely from OpenStreetMap, so the two agreeing
// is weak evidence. Foursquare is independent of both.
const FAMILY: Record<PlaceSource, 'fsq' | 'osm'> = { fsq: 'fsq', osm: 'osm', geoapify: 'osm' }

// Lower is more important. A stop at a light is on the bigger of the roads it
// meets, not the alley behind it.
const ROAD_RANK: Record<string, number> = {
  motorway: 0, trunk: 1, primary: 2, secondary: 3, tertiary: 4,
  residential: 5, unclassified: 5, living_street: 6, service: 8,
}
function roadRank(c: PlaceCandidate): number {
  return ROAD_RANK[c.category ?? ''] ?? 7
}

const GENERIC = new Set(['parking', 'parking lot', 'parking garage', 'parking structure', 'private parking', 'entrance', 'exit', 'building'])

export function normalizeName(s: string): string {
  return s
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/&/g, ' and ')
    .replace(/[^a-z0-9 ]+/g, ' ')
    .replace(/\b(the|inc|llc|co|corp|company|ltd)\b/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

export function sameName(a: string, b: string): boolean {
  const x = normalizeName(a)
  const y = normalizeName(b)
  if (!x || !y) return false
  if (x === y) return true
  const [short, long] = x.length <= y.length ? [x, y] : [y, x]
  return short.length >= 4 && long.includes(short)
}

function isGeneric(c: PlaceCandidate): boolean {
  return GENERIC.has(normalizeName(c.name))
}

interface Scored { c: PlaceCandidate; score: number; agreed: boolean }

function scorePois(cands: PlaceCandidate[]): Scored[] {
  const pois = cands.filter(c => c.kind !== 'street' && c.name.trim() && c.dist_m <= (c.kind === 'area' ? Infinity : POI_RADIUS_M))
  return pois.map((c) => {
    // Areas contain the point, so distance says nothing; weigh them below an
    // exact nearby business.
    const decay = c.kind === 'area' ? 0.7 : Math.exp(-c.dist_m / 45)
    let score = SOURCE_WEIGHT[c.source] * decay
    if (isGeneric(c)) score *= 0.2
    const agreed = pois.some(o => FAMILY[o.source] !== FAMILY[c.source] && sameName(o.name, c.name))
    if (agreed) score += 0.3
    return { c, score, agreed }
  }).sort((a, b) => b.score - a.score)
}

export function chooseLabel(cands: PlaceCandidate[], address: PlaceAddress | null, ctx: PlaceContext): PlaceLabel {
  const addr = address?.line ?? address?.formatted ?? null
  const visit = ctx.endpoint || ctx.dwell_s >= VISIT_MIN_S

  if (visit) {
    const ranked = scorePois(cands)
    const top = ranked[0]
    if (top && top.score >= 0.25) {
      const sources = [...new Set(cands.filter(c => sameName(c.name, top.c.name)).map(c => c.source))]
      return {
        name: top.c.name.trim(),
        category: top.c.category,
        address: addr,
        confidence: Math.min(0.97, Math.round((0.3 + top.score * 0.55) * 100) / 100),
        sources,
      }
    }
    if (addr) {
      return { name: addr, category: 'address', address: addr, confidence: 0.3, sources: address ? ['geoapify'] : [] }
    }
  }

  // A short stop: name the road it was on, not the shop beside it.
  const streets = cands.filter(c => c.kind === 'street' && c.dist_m <= STREET_RADIUS_M)
    .sort((a, b) => roadRank(a) - roadRank(b) || a.dist_m - b.dist_m)
  const street = address?.street ?? streets[0]?.name ?? null
  if (street) {
    const sources: PlaceSource[] = address?.street ? ['geoapify'] : streets[0] ? [streets[0].source] : []
    return { name: street, category: 'street', address: addr, confidence: 0.4, sources }
  }
  return { name: null, category: null, address: addr, confidence: 0, sources: [] }
}

// Haversine-free flat distance, accurate to well under a metre at this scale.
export function distM(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const dx = (lon2 - lon1) * Math.cos(lat1 * Math.PI / 180) * 111320
  const dy = (lat2 - lat1) * 110540
  return Math.sqrt(dx * dx + dy * dy)
}

export function attributionFor(sources: Iterable<PlaceSource>): string[] {
  const set = new Set(sources)
  const out: string[] = []
  if (set.has('osm') || set.has('geoapify')) out.push('© OpenStreetMap contributors')
  if (set.has('geoapify')) out.push('Powered by Geoapify')
  if (set.has('fsq')) out.push('Foursquare OS Places')
  return out
}
