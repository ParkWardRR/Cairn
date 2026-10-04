// MAF-based fuel samples for one trip. The blend-dependent maths happens in the
// browser, because the ethanol blend is a per-browser setting.
//
// MAF is only polled on some rounds of the OBD chain, so a trip typically has a
// few dozen usable samples rather than a continuous series: callers must treat
// anything derived from these as an estimate and show the sample count.
export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const [samples, distance, duration] = await Promise.all([
    queryTsdbObjects(`
      SELECT o.speed_kph, b.maf_cgps, b.lambda_ratio, b.mono_ms
      FROM boost b
      ASOF JOIN obd o ON b.boot_id = o.boot_id AND b.mono_ms >= o.mono_ms
      WHERE b.boot_id = ${id}
        AND b.maf_cgps IS NOT NULL AND b.lambda_ratio IS NOT NULL
        AND b.lambda_ratio > 0.7 AND b.lambda_ratio < 1.3
        AND o.speed_kph IS NOT NULL
      ORDER BY b.mono_ms
    `),
    // Each step is capped: the device logs sparsely while parked, so a long
    // silence between two fixes must not be counted as driving at the last speed.
    queryTsdbObjects(`
      SELECT coalesce(sum(speed_mps * least(lead_ms - mono_ms, 10000) / 1000.0), 0) AS distance_m
      FROM (
        SELECT mono_ms, speed_mps, lead(mono_ms) OVER (ORDER BY mono_ms) AS lead_ms
        FROM position WHERE boot_id = ${id} AND speed_mps IS NOT NULL
      ) WHERE lead_ms IS NOT NULL
    `),
    queryTsdbObjects(`SELECT duration_s FROM v_drive_summary WHERE boot_id = ${id}`),
  ])

  return {
    samples,
    distance_m: Number(distance[0]?.distance_m ?? 0),
    duration_s: duration[0]?.duration_s ?? null,
  }
})
