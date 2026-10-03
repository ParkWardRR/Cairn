export default defineEventHandler(async () => {
  const allTime = await queryTsdbObjects(`
    SELECT
      count(*) AS total_trips,
      coalesce(sum(duration_s), 0) AS total_duration_s,
      coalesce(max(max_speed_kph), 0) AS max_speed_kph
    FROM v_drive_summary
  `)

  const distance = await queryTsdbObjects(`
    SELECT coalesce(sum(dist_m), 0) AS total_distance_m FROM (
      SELECT boot_id, sum(segment_m) AS dist_m FROM (
        SELECT boot_id,
          speed_mps * (lead(mono_ms) OVER (PARTITION BY boot_id ORDER BY mono_ms) - mono_ms) / 1000.0 AS segment_m
        FROM position
        WHERE speed_mps IS NOT NULL
      ) GROUP BY boot_id
    )
  `)

  const today = await queryTsdbObjects(`
    SELECT count(*) AS trips, coalesce(sum(duration_s), 0) AS duration_s, coalesce(max(max_speed_kph), 0) AS max_speed_kph
    FROM v_drive_summary d
    JOIN (SELECT boot_id, min(observed_at) AS started FROM position GROUP BY boot_id) p USING (boot_id)
    WHERE p.started >= current_date
  `)

  const week = await queryTsdbObjects(`
    SELECT count(*) AS trips, coalesce(sum(duration_s), 0) AS duration_s, coalesce(max(max_speed_kph), 0) AS max_speed_kph
    FROM v_drive_summary d
    JOIN (SELECT boot_id, min(observed_at) AS started FROM position GROUP BY boot_id) p USING (boot_id)
    WHERE p.started >= current_date - INTERVAL 7 DAY
  `)

  const month = await queryTsdbObjects(`
    SELECT count(*) AS trips, coalesce(sum(duration_s), 0) AS duration_s, coalesce(max(max_speed_kph), 0) AS max_speed_kph
    FROM v_drive_summary d
    JOIN (SELECT boot_id, min(observed_at) AS started FROM position GROUP BY boot_id) p USING (boot_id)
    WHERE p.started >= current_date - INTERVAL 30 DAY
  `)

  return {
    allTime: {
      ...allTime[0],
      total_distance_m: distance[0]?.total_distance_m ?? 0,
    },
    today: today[0],
    week: week[0],
    month: month[0],
  }
})
