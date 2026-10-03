export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const positions = await queryTsdbObjects(`
    SELECT lat, lon, alt_m, speed_mps, heading_deg, h_acc_m, sats_used, mono_ms, observed_at
    FROM position
    WHERE boot_id = ${id} AND lat IS NOT NULL AND lon IS NOT NULL
    ORDER BY mono_ms ASC
  `)

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
