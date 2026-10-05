export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id
  const v = await vehicleScope(event, 'single')
  const where = bootId ? `WHERE boot_id = ${sqlString(String(bootId))}${v.and()}` : v.where()

  const rows = await queryTsdbObjects(`
    SELECT * FROM v_boost_curve ${where} ORDER BY boot_id, mono_ms ASC
  `)

  return { boostCurve: rows, vehicle_id: v.id }
})
