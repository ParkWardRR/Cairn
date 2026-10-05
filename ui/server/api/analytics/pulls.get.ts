export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id
  const v = await vehicleScope(event, 'single')
  const where = bootId ? `WHERE boot_id = ${sqlString(String(bootId))}${v.and()}` : v.where()

  const rows = await queryTsdbObjects(`
    SELECT * FROM v_pulls ${where} ORDER BY boot_id, start_ms ASC
  `)

  return { pulls: rows, vehicle_id: v.id }
})
