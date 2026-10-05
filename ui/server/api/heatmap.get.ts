export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event)
  const rows = await queryTsdbObjects(`
    SELECT lat, lon, speed_mps
    FROM position
    WHERE lat != 0 AND lon != 0${v.and()}
    ORDER BY vehicle_id, boot_id, mono_ms
  `)

  return {
    points: rows.map((r: any) => [r.lat, r.lon, Math.min((r.speed_mps ?? 0) * 2.23694, 80)]),
  }
})
