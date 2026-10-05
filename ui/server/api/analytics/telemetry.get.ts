export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'boot_id query param required' })
  const v = await vehicleScope(event, 'single')

  const rows = await queryTsdbObjects(`
    SELECT * FROM v_telemetry
    WHERE boot_id = ${sqlString(String(bootId))}${v.and()}
    ORDER BY mono_ms ASC
  `)

  return { telemetry: rows, vehicle_id: v.id }
})
