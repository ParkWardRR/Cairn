export default defineEventHandler(async (event) => {
  const bootId = getRouterParam(event, 'bootId')
  if (!bootId) throw createError({ statusCode: 400, statusMessage: 'bootId required' })
  const bid = sqlString(bootId)

  const summary = await queryTsdbObjects(`
    SELECT * FROM v_drive_summary WHERE boot_id = ${bid}
  `)
  if (!summary.length) {
    throw createError({ statusCode: 404, statusMessage: 'Trip not found' })
  }

  const firstPos = await queryTsdbObjects(`
    SELECT observed_at, lat, lon FROM position
    WHERE boot_id = ${bid}
    ORDER BY mono_ms ASC LIMIT 1
  `)

  const lastPos = await queryTsdbObjects(`
    SELECT observed_at, lat, lon FROM position
    WHERE boot_id = ${bid}
    ORDER BY mono_ms DESC LIMIT 1
  `)

  const [events, fixDelay, prevTrip] = await Promise.all([
    queryTsdbObjects(`
      SELECT count(*) AS harsh_event_count FROM imu
      WHERE boot_id = ${bid} AND event_flags > 0
    `),
    queryTsdbObjects(`
      SELECT
        (SELECT min(mono_ms) FROM obd WHERE boot_id = ${bid}) AS first_obd_ms,
        (SELECT max(mono_ms) FROM obd WHERE boot_id = ${bid}) AS last_obd_ms,
        (SELECT min(mono_ms) FROM position WHERE boot_id = ${bid}) AS first_pos_ms,
        (SELECT min(mono_ms) FROM position WHERE boot_id = ${bid} AND lat != 0 AND lon != 0) AS first_fix_ms
    `),
    queryTsdbObjects(`
      SELECT p.lat, p.lon, p.observed_at
      FROM position p
      JOIN (
        SELECT boot_id FROM v_drive_summary
        WHERE boot_id != ${bid}
        AND boot_id IN (
          SELECT boot_id FROM position GROUP BY boot_id
          HAVING max(observed_at) < (SELECT min(observed_at) FROM position WHERE boot_id = ${bid})
        )
        ORDER BY (SELECT max(observed_at) FROM position WHERE boot_id = v_drive_summary.boot_id) DESC
        LIMIT 1
      ) prev ON p.boot_id = prev.boot_id
      WHERE p.lat != 0 AND p.lon != 0
      ORDER BY p.mono_ms DESC
      LIMIT 1
    `),
  ])

  const fd = fixDelay[0] ?? {}
  return {
    ...summary[0],
    start: firstPos[0] ?? null,
    end: lastPos[0] ?? null,
    harsh_event_count: events[0]?.harsh_event_count ?? 0,
    first_obd_ms: fd.first_obd_ms ?? null,
    last_obd_ms: fd.last_obd_ms ?? null,
    first_pos_ms: fd.first_pos_ms ?? null,
    first_fix_ms: fd.first_fix_ms ?? null,
    prev_end: prevTrip[0] ?? null,
  }
})
