export default defineEventHandler(async () => {
  const [curve, timing, iatBoost] = await Promise.all([
    queryTsdbObjects(`
      SELECT b.mono_ms, b.boot_id, o.rpm, b.boost_psi,
        coalesce(o.intake_c,
          (SELECT o2.intake_c FROM obd o2
           WHERE o2.boot_id = b.boot_id AND o2.intake_c IS NOT NULL
           ORDER BY abs(o2.mono_ms::int - b.mono_ms::int) LIMIT 1)
        ) as intake_c,
        b.lambda_ratio, b.maf_cgps, o.timing_advance_deg
      FROM boost b
      ASOF JOIN obd o ON b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.boost_psi IS NOT NULL
      ORDER BY o.rpm
    `),
    queryTsdbObjects(`
      SELECT o.rpm, o.timing_advance_deg, b.boost_psi
      FROM boost b
      ASOF JOIN obd o ON b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE o.timing_advance_deg IS NOT NULL
        AND b.boost_psi > 0 AND o.rpm IS NOT NULL
      ORDER BY o.rpm
    `),
    queryTsdbObjects(`
      SELECT b.boost_psi,
        coalesce(o.intake_c,
          (SELECT o2.intake_c FROM obd o2
           WHERE o2.boot_id = b.boot_id AND o2.intake_c IS NOT NULL
           ORDER BY abs(o2.mono_ms::int - b.mono_ms::int) LIMIT 1)
        ) as intake_c,
        o.rpm
      FROM boost b
      ASOF JOIN obd o ON b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.boost_psi > 0
      ORDER BY b.boost_psi
    `),
  ])

  return { curve, timing, iatBoost }
})
