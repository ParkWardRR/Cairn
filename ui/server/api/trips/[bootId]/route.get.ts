const MAX_IMPLIED_MPS = 56 // ~200 kph — anything faster is a bad fix

function distM(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const dx = (lon2 - lon1) * Math.cos(lat1 * Math.PI / 180) * 111320
  const dy = (lat2 - lat1) * 110540
  return Math.sqrt(dx * dx + dy * dy)
}

function filterOutliers(positions: any[]): any[] {
  if (positions.length < 3) return positions

  const keep: any[] = [positions[0]]
  for (let i = 1; i < positions.length; i++) {
    const prev = keep[keep.length - 1]
    const cur = positions[i]
    const dt = (cur.mono_ms - prev.mono_ms) / 1000
    if (dt <= 0) continue
    const dm = distM(prev.lat, prev.lon, cur.lat, cur.lon)
    if (dm / dt > MAX_IMPLIED_MPS) continue
    keep.push(cur)
  }
  return keep
}

export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const raw = await queryTsdbObjects(`
    SELECT lat, lon, alt_m, speed_mps, heading_deg, h_acc_m, sats_used, mono_ms, observed_at
    FROM v_position
    WHERE boot_id = ${id}
    ORDER BY mono_ms ASC
  `)

  const positions = filterOutliers(raw)

  const gaps = await queryTsdbObjects(`
    SELECT * FROM gap WHERE boot_id = ${id} ORDER BY seq ASC
  `)

  const coordinates = positions.map((p: any) => [p.lon, p.lat, p.alt_m ?? 0])
  const speeds = positions.map((p: any) => p.speed_mps)
  const timestamps = positions.map((p: any) => p.observed_at)
  const headings = positions.map((p: any) => p.heading_deg)
  const accuracies = positions.map((p: any) => p.h_acc_m)
  const sats = positions.map((p: any) => p.sats_used)

  const features: any[] = []

  if (coordinates.length >= 2) {
    features.push({
      type: 'Feature',
      geometry: { type: 'LineString', coordinates },
      properties: { speeds, timestamps, headings, accuracies, sats },
    })
  }

  for (const gap of gaps) {
    features.push({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [0, 0] },
      properties: { type: 'gap', ...gap },
    })
  }

  return {
    type: 'FeatureCollection',
    features,
  }
})
