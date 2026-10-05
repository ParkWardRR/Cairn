export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id
  const v = await vehicleScope(event, 'single')

  let rows
  if (bootId) {
    rows = await queryTsdbObjects(`
      SELECT * FROM v_trim_map WHERE boot_id = ${sqlString(String(bootId))}${v.and()}
      ORDER BY rpm_bin, load_bin
    `)
  } else {
    // Pooled over trips, so it must be one car's trips only: the vehicle filter
    // is what keeps this from blending two engines' fuel maps.
    rows = await queryTsdbObjects(`
      SELECT rpm_bin, load_bin,
        sum(avg_stft * samples) / nullif(sum(samples), 0) AS avg_stft,
        sum(avg_ltft * samples) / nullif(sum(samples), 0) AS avg_ltft,
        sum(samples) AS samples
      FROM v_trim_map${v.where()}
      GROUP BY vehicle_id, rpm_bin, load_bin
      ORDER BY rpm_bin, load_bin
    `)
  }

  return { trimMap: rows, vehicle_id: v.id }
})
