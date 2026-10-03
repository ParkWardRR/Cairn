export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'boot_id query param required' })

  const rows = await queryTsdbObjects(`
    SELECT * FROM v_telemetry
    WHERE boot_id = ${sqlString(String(bootId))}
    ORDER BY mono_ms ASC
  `)

  return { telemetry: rows }
})
