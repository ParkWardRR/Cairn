export default defineEventHandler(async () => {
  const rows = await queryTsdbObjects(`
    SELECT lat, lon, speed_mps
    FROM position
    WHERE lat != 0 AND lon != 0
      AND speed_mps < 67
      AND (hdop IS NULL OR hdop < 15)
    ORDER BY boot_id, mono_ms
  `)

  return {
    points: rows.map((r: any) => [r.lat, r.lon, Math.min((r.speed_mps ?? 0) * 2.23694, 80)]),
  }
})
