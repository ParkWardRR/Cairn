interface Insight {
  label: string
  value: string
  unit?: string
  detail?: string
  icon?: string
}

function fmt(n: number, decimals = 1): string {
  return n.toFixed(decimals)
}

export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const [
    summary,
    distance,
    elevation,
    peakBoost,
    topRpm,
    wotPulls,
    lambdaStats,
    gpsQuality,
    coolant,
    idleTime,
  ] = await Promise.all([
    // v_drive_summary
    queryTsdbObjects(`SELECT duration_s, max_speed_kph, max_rpm, obd_samples FROM v_drive_summary WHERE boot_id = ${id}`),

    // distance from position speed integration
    queryTsdbObjects(`
      SELECT coalesce(sum(
        speed_mps * (lead_ms - mono_ms) / 1000.0
      ), 0) AS distance_m
      FROM (
        SELECT mono_ms, speed_mps,
          lead(mono_ms) OVER (ORDER BY mono_ms) AS lead_ms
        FROM v_position WHERE boot_id = ${id} AND speed_mps IS NOT NULL
      ) WHERE lead_ms IS NOT NULL
    `),

    // elevation gain/loss
    queryTsdbObjects(`
      SELECT
        sum(CASE WHEN alt_diff > 0 THEN alt_diff ELSE 0 END) AS gain_m,
        sum(CASE WHEN alt_diff < 0 THEN abs(alt_diff) ELSE 0 END) AS loss_m
      FROM (
        SELECT alt_m - lag(alt_m) OVER (ORDER BY mono_ms) AS alt_diff
        FROM v_position WHERE boot_id = ${id} AND alt_m IS NOT NULL
      )
    `),

    // peak boost
    queryTsdbObjects(`SELECT max(boost_psi) AS peak_boost_psi FROM boost WHERE boot_id = ${id} AND boost_psi IS NOT NULL`),

    // top RPM
    queryTsdbObjects(`SELECT max(rpm) AS max_rpm FROM obd WHERE boot_id = ${id} AND rpm IS NOT NULL`),

    // WOT pulls
    queryTsdbObjects(`SELECT count(*) AS cnt FROM v_pulls WHERE boot_id = ${id}`),

    // average lambda
    queryTsdbObjects(`SELECT avg(lambda_ratio) AS avg_lambda FROM boost WHERE boot_id = ${id} AND lambda_ratio IS NOT NULL`),

    // GPS quality — percentage of position samples with nonzero lat/lon
    queryTsdbObjects(`
      SELECT
        count(*) AS total,
        count(*) FILTER (WHERE lat != 0 AND lon != 0) AS good
      FROM position WHERE boot_id = ${id}
    `),

    // coolant temp range
    queryTsdbObjects(`SELECT min(coolant_c) AS min_c, max(coolant_c) AS max_c FROM obd WHERE boot_id = ${id} AND coolant_c IS NOT NULL`),

    // idle time — percentage of OBD samples under 5 kph
    queryTsdbObjects(`
      SELECT
        count(*) AS total,
        count(*) FILTER (WHERE speed_kph < 5) AS idle
      FROM obd WHERE boot_id = ${id} AND speed_kph IS NOT NULL
    `),
  ])

  if (!summary.length) throw createError({ statusCode: 404, statusMessage: 'Trip not found' })

  const insights: Insight[] = []
  const s = summary[0]

  // 1. Duration
  if (s.duration_s != null && s.duration_s > 0) {
    const mins = Math.floor(s.duration_s / 60)
    const secs = Math.round(s.duration_s % 60)
    insights.push({
      label: 'Duration',
      value: mins > 0 ? `${mins}m ${secs}s` : `${secs}s`,
      icon: 'clock',
    })
  }

  // 2. Distance
  const distM = distance[0]?.distance_m
  if (distM != null && distM > 0) {
    const distMi = distM / 1609.344
    const distKm = distM / 1000
    insights.push({
      label: 'Distance',
      value: fmt(distMi),
      unit: 'mi',
      detail: `${fmt(distKm)} km`,
      icon: 'road',
    })

    // 3. Average speed (only if we have distance and duration)
    if (s.duration_s > 0) {
      const avgMph = (distM / s.duration_s) * 2.23694
      const avgKph = (distM / s.duration_s) * 3.6
      insights.push({
        label: 'Avg speed',
        value: fmt(avgMph, 0),
        unit: 'mph',
        detail: `${fmt(avgKph, 0)} kph`,
        icon: 'gauge',
      })
    }
  }

  // 4. Top speed
  if (s.max_speed_kph != null && s.max_speed_kph > 0) {
    const topMph = s.max_speed_kph / 1.60934
    insights.push({
      label: 'Top speed',
      value: fmt(topMph, 0),
      unit: 'mph',
      detail: `${fmt(s.max_speed_kph, 0)} kph`,
      icon: 'speedometer',
    })
  }

  // 5. Elevation gain/loss
  const gain = elevation[0]?.gain_m
  const loss = elevation[0]?.loss_m
  if (gain != null && gain > 0) {
    const gainFt = gain * 3.28084
    insights.push({
      label: 'Elevation gain',
      value: fmt(gainFt, 0),
      unit: 'ft',
      detail: `${fmt(gain, 0)} m`,
      icon: 'mountain',
    })
  }
  if (loss != null && loss > 0) {
    const lossFt = loss * 3.28084
    insights.push({
      label: 'Elevation loss',
      value: fmt(lossFt, 0),
      unit: 'ft',
      detail: `${fmt(loss, 0)} m`,
      icon: 'valley',
    })
  }

  // 6. Peak boost
  const boost = peakBoost[0]?.peak_boost_psi
  if (boost != null && boost > 0) {
    insights.push({
      label: 'Peak boost',
      value: fmt(boost),
      unit: 'psi',
      icon: 'turbo',
    })
  }

  // 7. Top RPM
  const rpm = topRpm[0]?.max_rpm ?? s.max_rpm
  if (rpm != null && rpm > 0) {
    insights.push({
      label: 'Top RPM',
      value: fmt(rpm, 0),
      unit: 'rpm',
      icon: 'tachometer',
    })
  }

  // 8. WOT pulls
  const pullCount = wotPulls[0]?.cnt
  if (pullCount != null && pullCount > 0) {
    insights.push({
      label: 'WOT pulls',
      value: String(pullCount),
      detail: pullCount === 1 ? 'full-throttle pull detected' : 'full-throttle pulls detected',
      icon: 'rocket',
    })
  }

  // 9. Average lambda
  const avgLambda = lambdaStats[0]?.avg_lambda
  if (avgLambda != null) {
    const richLean = avgLambda < 1.0 ? 'rich' : avgLambda > 1.0 ? 'lean' : 'stoich'
    insights.push({
      label: 'Avg lambda',
      value: fmt(avgLambda, 3),
      detail: `Running ${richLean}`,
      icon: 'flame',
    })
  }

  // 10. GPS quality
  const gpsTotal = gpsQuality[0]?.total
  const gpsGood = gpsQuality[0]?.good
  if (gpsTotal != null && gpsTotal > 0) {
    const pct = (gpsGood / gpsTotal) * 100
    insights.push({
      label: 'GPS quality',
      value: fmt(pct, 0),
      unit: '%',
      detail: `${gpsGood} of ${gpsTotal} fixes with position`,
      icon: 'satellite',
    })
  }

  // 11. Coolant temp range
  const minC = coolant[0]?.min_c
  const maxC = coolant[0]?.max_c
  if (minC != null && maxC != null) {
    const minF = minC * 9 / 5 + 32
    const maxF = maxC * 9 / 5 + 32
    insights.push({
      label: 'Coolant range',
      value: `${fmt(minF, 0)}–${fmt(maxF, 0)}°F`,
      detail: `${fmt(minC, 0)}–${fmt(maxC, 0)}°C`,
      icon: 'thermometer',
    })
  }

  // 12. Idle time
  const idleTotal = idleTime[0]?.total
  const idleCount = idleTime[0]?.idle
  if (idleTotal != null && idleTotal > 0) {
    const pct = (idleCount / idleTotal) * 100
    insights.push({
      label: 'Idle time',
      value: fmt(pct, 0),
      unit: '%',
      detail: `Time spent under 3 mph`,
      icon: 'pause',
    })
  }

  return { insights }
})
