export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id

  let rows
  if (bootId) {
    rows = await queryTsdbObjects(`
      SELECT * FROM v_trim_map WHERE boot_id = ${sqlString(String(bootId))}
      ORDER BY rpm_bin, load_bin
    `)
  } else {
    rows = await queryTsdbObjects(`
      SELECT rpm_bin, load_bin,
        sum(avg_stft * samples) / nullif(sum(samples), 0) AS avg_stft,
        sum(avg_ltft * samples) / nullif(sum(samples), 0) AS avg_ltft,
        sum(samples) AS samples
      FROM v_trim_map
      GROUP BY rpm_bin, load_bin
      ORDER BY rpm_bin, load_bin
    `)
  }

  return { trimMap: rows }
})
