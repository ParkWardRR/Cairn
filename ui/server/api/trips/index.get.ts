export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const limit = Math.min(Math.max(parseInt(String(query.limit ?? '20'), 10) || 20, 1), 200)
  const offset = Math.max(parseInt(String(query.offset ?? '0'), 10) || 0, 0)
  const sortParam = String(query.sort ?? '-observed_at')
  const desc = sortParam.startsWith('-')
  const sortCol = sortParam.replace(/^-/, '')
  const allowed = ['observed_at', 'duration_s', 'max_speed_kph', 'max_rpm', 'boot_id']
  const orderBy = allowed.includes(sortCol) ? sortCol : 'observed_at'
  const dir = desc ? 'DESC' : 'ASC'

  const trips = await queryTsdbObjects(`
    SELECT d.*,
      first_pos.observed_at AS start_time,
      first_pos.lat AS start_lat,
      first_pos.lon AS start_lon
    FROM v_drive_summary d
    LEFT JOIN (
      SELECT boot_id, observed_at, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms ASC) AS rn FROM v_position) WHERE rn = 1
    ) first_pos USING (boot_id)
    ORDER BY ${orderBy === 'observed_at' ? 'first_pos.observed_at' : orderBy} ${dir}
    LIMIT ${limit} OFFSET ${offset}
  `)

  const countResult = await queryTsdbObjects(`SELECT count(*) AS total FROM v_drive_summary`)

  return { trips, total: countResult[0]?.total ?? 0 }
})
