export default defineEventHandler(async () => {
  const trips = await queryTsdbObjects(`
    SELECT
      d.boot_id,
      d.duration_s,
      d.max_speed_kph,
      fp.observed_at AS start_time,
      fp.lat AS start_lat,
      fp.lon AS start_lon,
      lp.lat AS end_lat,
      lp.lon AS end_lon
    FROM v_drive_summary d
    LEFT JOIN (
      SELECT boot_id, observed_at, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms ASC) AS rn FROM position WHERE lat != 0 AND lon != 0) WHERE rn = 1
    ) fp USING (boot_id)
    LEFT JOIN (
      SELECT boot_id, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms DESC) AS rn FROM position WHERE lat != 0 AND lon != 0) WHERE rn = 1
    ) lp USING (boot_id)
    ORDER BY fp.observed_at DESC
  `)

  return { trips }
})
