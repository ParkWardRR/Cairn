export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlInt(bootId)

  const [summary, speedStats, boostStats, elevationStats, distanceStats, wotPulls, harshEvents, lambdaStats, coolantStats, gpsQuality, startPos, endPos] = await Promise.all([
    queryTsdbObjects(`SELECT * FROM v_drive_summary WHERE boot_id = ${id}`),
    queryTsdbObjects(`
      SELECT max(speed_kph) AS top_obd_speed_kph FROM obd WHERE boot_id = ${id}
    `),
    queryTsdbObjects(`
      SELECT max(boost_psi) AS peak_boost_psi FROM boost WHERE boot_id = ${id}
    `),
    queryTsdbObjects(`
      SELECT
        sum(CASE WHEN alt_diff > 0 THEN alt_diff ELSE 0 END) AS elevation_gain_m,
        sum(CASE WHEN alt_diff < 0 THEN abs(alt_diff) ELSE 0 END) AS elevation_loss_m
      FROM (
        SELECT alt_m - lag(alt_m) OVER (ORDER BY mono_ms) AS alt_diff
        FROM position WHERE boot_id = ${id} AND alt_m IS NOT NULL
      )
    `),
    queryTsdbObjects(`
      SELECT coalesce(sum(
        speed_mps * (lead_ms - mono_ms) / 1000.0
      ), 0) AS distance_m FROM (
        SELECT mono_ms, speed_mps,
          lead(mono_ms) OVER (ORDER BY mono_ms) AS lead_ms
        FROM position WHERE boot_id = ${id} AND speed_mps IS NOT NULL
      ) WHERE lead_ms IS NOT NULL
    `),
    queryTsdbObjects(`SELECT count(*) AS wot_pulls FROM v_pulls WHERE boot_id = ${id}`),
    queryTsdbObjects(`SELECT count(*) AS harsh_events FROM imu WHERE boot_id = ${id} AND event_flags > 0`),
    queryTsdbObjects(`SELECT avg(lambda_ratio) AS avg_lambda FROM boost WHERE boot_id = ${id} AND lambda_ratio IS NOT NULL`),
    queryTsdbObjects(`SELECT min(coolant_c) AS min_coolant_c, max(coolant_c) AS max_coolant_c FROM obd WHERE boot_id = ${id} AND coolant_c IS NOT NULL`),
    queryTsdbObjects(`
      SELECT
        count(*) FILTER (WHERE fix_type IS NOT NULL AND fix_type != 0) * 100.0 / nullif(count(*), 0) AS gps_quality_pct
      FROM position WHERE boot_id = ${id}
    `),
    queryTsdbObjects(`SELECT lat, lon FROM position WHERE boot_id = ${id} AND lat IS NOT NULL ORDER BY mono_ms ASC LIMIT 1`),
    queryTsdbObjects(`SELECT lat, lon FROM position WHERE boot_id = ${id} AND lat IS NOT NULL ORDER BY mono_ms DESC LIMIT 1`),
  ])

  if (!summary.length) throw createError({ statusCode: 404, statusMessage: 'Trip not found' })

  let startName: string | null = null
  let endName: string | null = null
  if (startPos[0]?.lat != null) startName = await reverseGeocode(startPos[0].lat, startPos[0].lon)
  if (endPos[0]?.lat != null) endName = await reverseGeocode(endPos[0].lat, endPos[0].lon)

  const gnssTopSpeed = await queryTsdbObjects(`
    SELECT max(speed_mps) * 3.6 AS top_gnss_speed_kph FROM position WHERE boot_id = ${id} AND speed_mps IS NOT NULL
  `)

  return {
    boot_id: id,
    duration_s: summary[0].duration_s,
    top_obd_speed_kph: speedStats[0]?.top_obd_speed_kph ?? null,
    top_gnss_speed_kph: gnssTopSpeed[0]?.top_gnss_speed_kph ?? null,
    peak_boost_psi: boostStats[0]?.peak_boost_psi ?? null,
    elevation_gain_m: elevationStats[0]?.elevation_gain_m ?? 0,
    elevation_loss_m: elevationStats[0]?.elevation_loss_m ?? 0,
    distance_m: distanceStats[0]?.distance_m ?? 0,
    wot_pull_count: wotPulls[0]?.wot_pulls ?? 0,
    harsh_event_count: harshEvents[0]?.harsh_events ?? 0,
    avg_lambda: lambdaStats[0]?.avg_lambda ?? null,
    min_coolant_c: coolantStats[0]?.min_coolant_c ?? null,
    max_coolant_c: coolantStats[0]?.max_coolant_c ?? null,
    gps_quality_pct: gpsQuality[0]?.gps_quality_pct ?? null,
    start_location: startName,
    end_location: endName,
    start: startPos[0] ?? null,
    end: endPos[0] ?? null,
  }
})
