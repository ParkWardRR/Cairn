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

  // Stops use the same detector as the trip detail page, so a place always
  // agrees with the stops shown on the trips that visited it.
  const fixes = await queryTsdbObjects(`
    SELECT boot_id, lat, lon, speed_mps, mono_ms, observed_at
    FROM position
    WHERE lat != 0 AND lon != 0
    ORDER BY boot_id, mono_ms
  `)

  const byBoot = new Map<string, any[]>()
  for (const f of fixes) {
    const list = byBoot.get(f.boot_id)
    if (list) list.push(f)
    else byBoot.set(f.boot_id, [f])
  }

  const visits: PlaceVisit[] = []
  for (const [bootId, list] of byBoot) {
    visits.push(...visitsFromStops(bootId, detectStops(list as StopFix[])))
    const base = tripBaseMs(list as StopFix[])
    const at = (f: any) => (base != null ? new Date(base + f.mono_ms).toISOString() : null)
    const first = list[0]
    const last = list[list.length - 1]
    visits.push({ boot_id: bootId, kind: 'departure', lat: first.lat, lon: first.lon, at: at(first) })
    visits.push({ boot_id: bootId, kind: 'arrival', lat: last.lat, lon: last.lon, at: at(last) })
  }

  const resolver = getPlaceResolver()
  const clusters = mergeIntoSaved(clusterPlaces(visits), (la, lo) => resolver.saved.match(la, lo))

  // A saved place with no visits yet (added by hand, or its trips were pruned)
  // still belongs on the map, so it is added as a place with no history.
  const empty = { stops: 0, stop_seconds: 0, short: 0, medium: 0, long: 0, longest_s: 0, longest_category: null, arrivals: 0, departures: 0, trips: [] as string[], last_at: null }
  const places: Place[] = [...clusters]
  for (const s of resolver.saved.list()) {
    if (!clusters.some(c => resolver.saved.match(c.lat, c.lon)?.id === s.id)) {
      places.push({ id: places.length, lat: s.lat, lon: s.lon, ...empty })
    }
  }

  const { results, pending } = lookupPlaces(
    places.map(p => ({
      lat: p.lat,
      lon: p.lon,
      ctx: { dwell_s: p.longest_s, endpoint: p.arrivals + p.departures > 0 },
    })),
  )

  return {
    trips,
    saved: resolver.saved.list(),
    places: places.map((p, i) => ({
      ...p,
      label: placeForApi(results[i]),
      suggestions: resolver.suggestions(p.lat, p.lon),
    })),
    pending,
    attribution: attributionForResults(results),
  }
})
