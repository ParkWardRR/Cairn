export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const id = sqlInt(bootId)

  const imuEvents = await queryTsdbObjects(`
    SELECT mono_ms, observed_at, accel_rms_mg, accel_peak_x_mg, accel_peak_y_mg, accel_peak_z_mg,
      gyro_peak_dps, variance, event_flags
    FROM imu
    WHERE boot_id = ${id} AND event_flags > 0
    ORDER BY mono_ms ASC
  `)

  const transitions = await queryTsdbObjects(`
    SELECT * FROM transition
    WHERE boot_id = ${id}
    ORDER BY mono_ms ASC
  `)

  return { imuEvents, transitions }
})
