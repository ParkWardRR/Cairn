// Peak boost, average lambda and trims only mean something for one engine, so
// this answers for a single vehicle, never a blend of cars.
export default defineEventHandler(async (event) => {
  const v = await vehicleScope(event, 'single')
  const [engine, imu] = await Promise.all([
    queryTsdbObjects(`
      SELECT
        round(max(boost_psi), 1) AS peak_boost_psi,
        max(rpm) AS peak_rpm,
        round(avg(CASE WHEN lambda_ratio > 0 THEN lambda_ratio END), 3) AS avg_lambda,
        round(avg(stft_pct), 1) AS avg_stft,
        round(avg(ltft_pct), 1) AS avg_ltft,
        max(coolant_c) AS max_coolant_c,
        count(*) AS obd_samples
      FROM v_telemetry${v.where()}
    `),
    queryTsdbObjects(`
      SELECT
        count(*) AS imu_samples,
        round(max(abs(accel_peak_x_mg)) / 1000.0, 2) AS peak_long_g,
        round(max(abs(accel_peak_y_mg)) / 1000.0, 2) AS peak_lat_g,
        round(max(gyro_peak_dps), 1) AS peak_yaw_dps
      FROM imu${v.where()}
    `),
  ])

  return {
    engine: engine[0] ?? {},
    imu: imu[0] ?? {},
    vehicle_id: v.id,
  }
})
