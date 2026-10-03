export default defineEventHandler(async () => {
  const rows = await queryTsdbObjects(`
    SELECT battery_mv, device_temp_c, sd_free_mib, rssi_dbm, health_state, observed_at
    FROM status
    ORDER BY observed_at DESC
    LIMIT 1
  `)

  if (!rows.length) {
    throw createError({ statusCode: 404, statusMessage: 'No device status available' })
  }

  return rows[0]
})
