export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event, 'single')
  const [samples, perTrip, timingByLoad] = await Promise.all([
    queryTsdbObjects(`
      SELECT o.speed_kph, b.maf_cgps, b.lambda_ratio, o.rpm,
        o.timing_advance_deg, o.load_pct, b.boost_psi, b.ltft_pct,
        b.boot_id, b.mono_ms
      FROM boost b
      ASOF JOIN obd o ON b.vehicle_id = o.vehicle_id AND b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.maf_cgps IS NOT NULL AND b.lambda_ratio IS NOT NULL
        AND b.lambda_ratio > 0.7 AND b.lambda_ratio < 1.3
        AND o.speed_kph IS NOT NULL AND o.speed_kph > 5
        AND o.rpm IS NOT NULL AND o.rpm > 0${v.and('b')}
      ORDER BY b.boot_id, b.mono_ms
    `),
    queryTsdbObjects(`
      SELECT b.boot_id,
        min(b.observed_at) as first_seen,
        count(*) as samples,
        avg(o.speed_kph)::decimal(5,1) as avg_speed_kph,
        max(o.speed_kph) as max_speed_kph,
        avg(b.maf_cgps)::decimal(6,1) as avg_maf,
        avg(b.lambda_ratio)::decimal(5,3) as avg_lambda,
        avg(o.rpm)::decimal(6,0) as avg_rpm,
        avg(o.timing_advance_deg)::decimal(4,1) as avg_timing,
        avg(b.ltft_pct)::decimal(4,1) as avg_ltft,
        avg(o.load_pct)::decimal(4,1) as avg_load,
        (max(b.mono_ms) - min(b.mono_ms)) / 1000.0 as duration_s
      FROM boost b
      ASOF JOIN obd o ON b.vehicle_id = o.vehicle_id AND b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.maf_cgps IS NOT NULL AND b.lambda_ratio IS NOT NULL
        AND b.lambda_ratio > 0.7 AND b.lambda_ratio < 1.3
        AND o.speed_kph IS NOT NULL AND o.speed_kph > 5
        AND o.rpm IS NOT NULL AND o.rpm > 0${v.and('b')}
      GROUP BY b.vehicle_id, b.boot_id
      ORDER BY first_seen
    `),
    queryTsdbObjects(`
      SELECT
        CASE
          WHEN o.load_pct < 20 THEN 'light'
          WHEN o.load_pct < 50 THEN 'medium'
          ELSE 'heavy'
        END as load_zone,
        avg(o.timing_advance_deg)::decimal(4,1) as avg_timing,
        avg(b.lambda_ratio)::decimal(5,3) as avg_lambda,
        avg(b.maf_cgps)::decimal(6,1) as avg_maf,
        avg(o.speed_kph)::decimal(5,1) as avg_speed,
        avg(o.rpm)::decimal(6,0) as avg_rpm,
        count(*) as samples
      FROM boost b
      ASOF JOIN obd o ON b.vehicle_id = o.vehicle_id AND b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.maf_cgps IS NOT NULL AND b.lambda_ratio IS NOT NULL
        AND b.lambda_ratio > 0.7 AND b.lambda_ratio < 1.3
        AND o.speed_kph IS NOT NULL AND o.speed_kph > 5
        AND o.rpm IS NOT NULL AND o.rpm > 0
        AND o.timing_advance_deg IS NOT NULL
        AND o.load_pct IS NOT NULL${v.and('b')}
      GROUP BY b.vehicle_id, load_zone
      ORDER BY avg_speed
    `),
  ])

  return { samples, perTrip, timingByLoad, vehicle_id: v.id }
})
