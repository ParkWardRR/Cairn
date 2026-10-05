export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event)
  const health = await tsdbHealth()
  if (!health.ok) {
    throw createError({ statusCode: 502, statusMessage: 'TSDB unreachable' })
  }

  const tables = await queryTsdbObjects(`
    SELECT table_name, table_type,
      (SELECT count(*) FROM information_schema.columns c WHERE c.table_name = t.table_name) AS col_count
    FROM information_schema.tables t
    WHERE table_schema = 'main'
    ORDER BY table_type, table_name
  `)

  const counts = await queryTsdbObjects(`
    SELECT
      (SELECT count(*) FROM position${v.where()}) AS position_rows,
      (SELECT count(*) FROM obd${v.where()}) AS obd_rows,
      (SELECT count(*) FROM boost${v.where()}) AS boost_rows,
      (SELECT count(*) FROM imu${v.where()}) AS imu_rows,
      (SELECT count(*) FROM status${v.where()}) AS status_rows,
      (SELECT count(*) FROM transition${v.where()}) AS transition_rows,
      (SELECT count(*) FROM gap${v.where()}) AS gap_rows,
      (SELECT count(*) FROM bundles${v.where()}) AS bundle_rows,
      (SELECT count(*) FROM (SELECT DISTINCT vehicle_id, boot_id FROM position${v.where()})) AS trips
  `)

  return {
    ok: true,
    tables: tables.length,
    ...counts[0],
  }
})
