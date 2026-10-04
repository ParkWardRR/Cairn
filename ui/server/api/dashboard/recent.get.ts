export default defineEventHandler(async () => {
  const trips = await queryTsdbObjects(`
    SELECT d.*,
      first_pos.observed_at AS start_time,
      first_pos.lat AS start_lat, first_pos.lon AS start_lon,
      last_pos.lat AS end_lat, last_pos.lon AS end_lon
    FROM v_drive_summary d
    LEFT JOIN (
      SELECT boot_id, observed_at, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms ASC) AS rn FROM v_position) WHERE rn = 1
    ) first_pos USING (boot_id)
    LEFT JOIN (
      SELECT boot_id, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms DESC) AS rn FROM v_position) WHERE rn = 1
    ) last_pos USING (boot_id)
    ORDER BY first_pos.observed_at DESC
    LIMIT 5
  `)

  return { trips }
})
