export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const bootId = query.boot_id ? sqlString(String(query.boot_id)) : null
  const v = await vehicleScope(event, 'single')

  const where = bootId ? `WHERE boot_id = ${bootId}${v.and()}` : v.where()

  const rows = await queryTsdbObjects(`
    SELECT boot_id, mono_ms, observed_at,
      accel_peak_x_mg, accel_peak_y_mg, accel_peak_z_mg,
      accel_rms_mg, gyro_peak_dps, event_flags
    FROM imu
    ${where}
    ORDER BY boot_id, mono_ms ASC
  `)

  return { samples: rows, vehicle_id: v.id }
})
