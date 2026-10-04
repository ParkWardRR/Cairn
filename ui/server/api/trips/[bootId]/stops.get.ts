export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlString(bootId)

  const fixes = await queryTsdbObjects(`
    SELECT lat, lon, speed_mps, mono_ms, observed_at
    FROM position
    WHERE boot_id = ${id} AND lat != 0 AND lon != 0
    ORDER BY mono_ms ASC
  `)

  return { stops: detectStops(fixes as StopFix[]) }
})
