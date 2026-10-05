export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event, 'single')
  const [perTrip, flowSamples, lambdaLoad] = await Promise.all([
    queryTsdbObjects(`
      SELECT b.boot_id,
        min(b.observed_at) as first_seen,
        avg(b.ltft_pct)::decimal(4,1) as avg_ltft,
        avg(b.stft_pct)::decimal(4,1) as avg_stft,
        avg(b.lambda_ratio)::decimal(5,3) as avg_lambda,
        max(b.maf_cgps) as peak_maf_cgps,
        count(*) as samples
      FROM boost b
      WHERE b.ltft_pct IS NOT NULL${v.and('b')}
      GROUP BY b.vehicle_id, b.boot_id
      ORDER BY first_seen
    `),
    queryTsdbObjects(`
      SELECT b.mono_ms, o.rpm, b.maf_cgps, b.lambda_ratio,
        b.boost_psi, o.intake_c, o.load_pct, b.ltft_pct, b.stft_pct
      FROM boost b
      JOIN obd o ON b.vehicle_id = o.vehicle_id AND b.boot_id = o.boot_id
        AND abs(b.mono_ms::int - o.mono_ms::int) < 2000
      WHERE b.maf_cgps IS NOT NULL AND b.lambda_ratio IS NOT NULL
        AND b.lambda_ratio > 0 AND b.lambda_ratio < 1.5
        AND o.rpm IS NOT NULL AND o.rpm > 0${v.and('b')}
      ORDER BY o.rpm
    `),
    queryTsdbObjects(`
      SELECT o.load_pct, b.lambda_ratio, o.rpm, b.boost_psi
      FROM boost b
      JOIN obd o ON b.vehicle_id = o.vehicle_id AND b.boot_id = o.boot_id
        AND abs(b.mono_ms::int - o.mono_ms::int) < 2000
      WHERE b.lambda_ratio IS NOT NULL AND o.load_pct IS NOT NULL
        AND o.rpm IS NOT NULL AND b.lambda_ratio < 1.5${v.and('b')}
      ORDER BY o.load_pct
    `),
  ])

  return { perTrip, flowSamples, lambdaLoad, vehicle_id: v.id }
})
