export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const rows = await queryTsdbObjects(`
    SELECT * FROM v_telemetry
    WHERE boot_id = ${id}
    ORDER BY mono_ms ASC
  `)

  return { telemetry: rows }
})
