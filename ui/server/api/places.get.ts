export default defineEventHandler(async () => {
  const trips = await queryTsdbObjects(`
    SELECT
      d.boot_id,
      d.duration_s,
      d.max_speed_kph,
      fp.observed_at AS start_time,
      fp.lat AS start_lat,
      fp.lon AS start_lon,
      lp.lat AS end_lat,
      lp.lon AS end_lon
    FROM v_drive_summary d
    LEFT JOIN (
      SELECT boot_id, observed_at, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms ASC) AS rn FROM position WHERE lat != 0 AND lon != 0) WHERE rn = 1
    ) fp USING (boot_id)
    LEFT JOIN (
      SELECT boot_id, lat, lon
      FROM (SELECT *, row_number() OVER (PARTITION BY boot_id ORDER BY mono_ms DESC) AS rn FROM position WHERE lat != 0 AND lon != 0) WHERE rn = 1
    ) lp USING (boot_id)
    ORDER BY fp.observed_at DESC
  `)

  // Places come from the recorded visit history, which the ingest keeps current
  // as trips arrive; this just makes sure it is up to date before reading it.
  await ingestPlaces()

  const resolver = getPlaceResolver()
  const places = buildPlaces(resolver)

  const { results, pending } = lookupPlaces(
    places.map(p => ({
      lat: p.lat,
      lon: p.lon,
      ctx: { dwell_s: p.longest_s, endpoint: p.arrivals + p.departures > 0 },
    })),
  )

  const saved = resolver.saved.list()
  return {
    trips,
    saved,
    storage: {
      saved: saved.filter(s => s.source === 'user').length,
      learned: saved.filter(s => s.source === 'learned').length,
      backup_at: resolver.saved.lastBackupAt(),
      trips_recorded: resolver.store.visitTripCount(),
    },
    places: places.map((p, i) => ({
      ...p,
      label: placeForApi(results[i]),
      suggestions: resolver.suggestions(p.lat, p.lon),
    })),
    pending,
    attribution: attributionForResults(results),
  }
})
