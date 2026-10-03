export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const bid = sqlString(bootId)

  const [obd, boost, gps] = await Promise.all([
    queryTsdbObjects(`
      SELECT mono_ms, speed_kph, rpm, coolant_c, intake_c,
        throttle_pct, load_pct
      FROM obd WHERE boot_id = ${bid}
      ORDER BY mono_ms
    `),
    queryTsdbObjects(`
      SELECT mono_ms, boost_psi, lambda_ratio, stft_pct, ltft_pct
      FROM boost WHERE boot_id = ${bid}
      ORDER BY mono_ms
    `),
    queryTsdbObjects(`
      SELECT mono_ms, lat, lon, speed_mps, sats_used, fix_type, hdop,
        h_acc_m, alt_m
      FROM position WHERE boot_id = ${bid}
      ORDER BY mono_ms
    `),
  ])

  return { obd, boost, gps }
})
