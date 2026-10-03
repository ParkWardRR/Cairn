export default defineEventHandler(async () => {
  const [curve, timing, iatBoost] = await Promise.all([
    queryTsdbObjects(`
      SELECT b.mono_ms, b.boot_id, o.rpm, b.boost_psi, o.intake_c,
        b.lambda_ratio, b.maf_cgps, o.timing_advance_deg
      FROM boost b
      JOIN obd o ON b.boot_id = o.boot_id
        AND abs(b.mono_ms::int - o.mono_ms::int) < 2000
      WHERE b.boost_psi IS NOT NULL AND o.rpm IS NOT NULL
      ORDER BY o.rpm
    `),
    queryTsdbObjects(`
      SELECT o.rpm, o.timing_advance_deg, b.boost_psi
      FROM boost b
      JOIN obd o ON b.boot_id = o.boot_id
        AND abs(b.mono_ms::int - o.mono_ms::int) < 2000
      WHERE o.timing_advance_deg IS NOT NULL
        AND b.boost_psi > 0 AND o.rpm IS NOT NULL
      ORDER BY o.rpm
    `),
    queryTsdbObjects(`
      SELECT b.boost_psi, o.intake_c, o.rpm
      FROM boost b
      JOIN obd o ON b.boot_id = o.boot_id
        AND abs(b.mono_ms::int - o.mono_ms::int) < 2000
      WHERE b.boost_psi > 0 AND o.intake_c IS NOT NULL
      ORDER BY b.boost_psi
    `),
  ])

  return { curve, timing, iatBoost }
})
