import { createError, getQuery, type H3Event } from 'h3'

// Vehicle ids are 16-byte UUIDv7s, written like boot_id: 32 lowercase hex chars.
export const VEHICLE_ID_RE = /^[0-9a-f]{32}$/

// 'all' routes list or sum per-trip rows, which is fine across cars. 'single'
// routes average samples (trims, lambda, boost against rpm), so they always
// answer for exactly one car.
export type VehicleMode = 'all' | 'single'

export interface VehicleScope {
  // The vehicle filtered to; null when unfiltered or when there is none to pick.
  id: string | null
  // ' AND [alias.]vehicle_id = ...', or '' when unfiltered.
  and(alias?: string): string
  // ' WHERE [alias.]vehicle_id = ...', or '' when unfiltered.
  where(alias?: string): string
}

// The ?vehicle= query value: absent, '' or 'all' means every vehicle (null).
export function parseVehicleParam(raw: unknown): string | null {
  if (raw == null) return null
  if (Array.isArray(raw)) {
    throw createError({ statusCode: 400, statusMessage: 'vehicle must be given once' })
  }
  const v = String(raw).trim().toLowerCase()
  if (v === '' || v === 'all') return null
  if (!VEHICLE_ID_RE.test(v)) {
    throw createError({ statusCode: 400, statusMessage: 'vehicle must be 32 hex characters' })
  }
  return v
}

// Builds the SQL fragments. `none` makes every query match nothing, for a
// single-vehicle route when the store holds no vehicle at all.
export function buildVehicleScope(id: string | null, none = false): VehicleScope {
  // The id is spliced into SQL, so it is checked here too and not only at the edge.
  if (id != null && !VEHICLE_ID_RE.test(id)) throw new Error(`bad vehicle id: ${id}`)
  const cond = (alias?: string) => {
    if (none) return 'FALSE'
    return `${alias ? `${alias}.` : ''}vehicle_id = '${id}'`
  }
  const filtered = none || id != null
  return {
    id: none ? null : id,
    and: alias => (filtered ? ` AND ${cond(alias)}` : ''),
    where: alias => (filtered ? ` WHERE ${cond(alias)}` : ''),
  }
}

export const LATEST_VEHICLE_SQL
  = 'SELECT vehicle_id FROM v_vehicles ORDER BY last_observed_at DESC NULLS LAST, vehicle_id LIMIT 1'

// Single mode never falls back to "every vehicle": a trim map or boost curve
// averaged over an N20 and a B58 describes no real engine, so with no ?vehicle=
// it answers for the most recently seen car, which is also the first one the
// UI's vehicle list shows.
export async function vehicleScope(event: H3Event, mode: VehicleMode = 'all'): Promise<VehicleScope> {
  const id = parseVehicleParam(getQuery(event).vehicle)
  if (id != null || mode === 'all') return buildVehicleScope(id)

  const rows = await queryTsdbObjects(LATEST_VEHICLE_SQL)
  const latest = rows[0]?.vehicle_id
  if (latest == null) return buildVehicleScope(null, true)
  if (typeof latest !== 'string' || !VEHICLE_ID_RE.test(latest)) {
    throw createError({ statusCode: 502, statusMessage: 'tsdb returned a malformed vehicle_id' })
  }
  return buildVehicleScope(latest)
}
