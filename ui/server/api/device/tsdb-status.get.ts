export default defineEventHandler(async () => {
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
      (SELECT count(*) FROM position) AS position_rows,
      (SELECT count(*) FROM obd) AS obd_rows,
      (SELECT count(*) FROM boost) AS boost_rows,
      (SELECT count(*) FROM imu) AS imu_rows,
      (SELECT count(*) FROM status) AS status_rows,
      (SELECT count(*) FROM transition) AS transition_rows,
      (SELECT count(*) FROM gap) AS gap_rows,
      (SELECT count(*) FROM bundles) AS bundle_rows,
      (SELECT count(DISTINCT boot_id) FROM position) AS trips
  `)

  return {
    ok: true,
    tables: tables.length,
    ...counts[0],
  }
})
