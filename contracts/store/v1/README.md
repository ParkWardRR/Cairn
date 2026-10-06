# Analytical store v1 (draft)

What the web dashboard relies on from the server's analytical store, `cairn-tsdb`. Its
objects and columns are pinned in [`schema.json`](schema.json); this file carries the rest.

**Tables:** `bundles`, `position`, `imu`, `obd`, `boost`, `status`, `transition`, `gap`.
**Views:** `v_telemetry`, `v_reproducibility`, `v_vehicles`, `v_drive_summary`,
`v_trip_summary`, `v_trim_map`, `v_boost_curve`, `v_pulls`, `v_speed_agreement`,
`v_gnss_sources`, and from **`store/v1.1`** `v_trip_period`. **Table macros** (`store/v1.1`):
`period_summary(from_day, to_day)`. Every table, view and macro result carries `vehicle_id`.

**`schema.json`** lists every table and view (and, from `store/v1.1`, every macro) with its columns and DuckDB types, and the
`store/v1.N` it was generated from. It is generated, not written: the server's
`go run ./cmd/dump-schema` starts the real store and dumps the schema it built for itself.
It is read as a **compatible range**: a server release satisfies it when it has every listed
table and view, each listed column under a compatible type, and at least that minor. A pinned
macro must also exist with the same parameter names in the same order. Extra
tables, views, macros and columns are allowed (that is what a minor adds), and column order does not
matter. A **compatible type** is the same type, or a wider integer of the same signedness
(`UINTEGER` to `UBIGINT`); anything else (`VARCHAR` to `INTEGER`, `FLOAT` to `DOUBLE`,
signed to unsigned, narrowing) is a retype and breaks the contract. The server's CI compares
its native schema with the pinned file; this repository's CI checks only the file's own
shape (valid, known fields, every object has `vehicle_id`).

### Periods (`store/v1.1`)

A view cannot take a parameter, so a summary for any date range is a **table macro**, and the
per-trip rows it is built from are a view a caller can group by calendar period itself. Both
are reached through `POST /query` like any other object.

**`v_trip_period`**: one row per trip (`vehicle_id`, `boot_id`), listing `started_at`,
`ended_at`, `started_on`, and the first day of the trip's ISO week (`week_start`, a Monday),
`month_start`, `quarter_start` and `year_start` (all `DATE`), then `duration_ms`, `distance_m`,
`max_obd_speed_kph` and `max_gnss_speed_kph`. The measures are `v_trip_summary`'s own:
`duration_ms` is the span of everything observed (not only the OBD), `distance_m` sums
great-circle steps between device fixes at most 5 s apart, `max_obd_speed_kph` is
`v_drive_summary.max_speed_kph`, and `max_gnss_speed_kph` is `max_gnss_speed_mps * 3.6`.
`distance_m` and the maxima are `NULL` where the trip has no such samples.

**`period_summary(from_day, to_day)`**: one row per vehicle with a trip in the range
(`vehicle_id`, `trips`, `duration_ms`, `distance_m`, `max_obd_speed_kph`, `max_gnss_speed_kph`).
Arguments are dates or `YYYY-MM-DD` strings.

```sql
SELECT * FROM period_summary('2026-03-01', '2026-04-01') WHERE vehicle_id = '<hex>';  -- March
SELECT year_start, count(*) AS trips, sum(distance_m) AS distance_m
FROM v_trip_period WHERE vehicle_id = '<hex>' GROUP BY year_start ORDER BY year_start;  -- per year
```

**The rules**

- **The range is half-open: `[from_day, to_day)`.** `from_day` is included and `to_day`, the
  first day after, is not, so a month and the next month neither overlap nor leave a gap, and
  "1 to 10 March" is `('2026-03-01', '2026-03-11')`. An inverted or empty range is zero rows.
- **A trip belongs to the period it started in.** Its period is that of `started_at`, the first
  observation on any source, and the whole trip counts there: a drive from 23:59 on 31 March
  to 00:01 on 1 April is a March trip with all its distance and time, and none of it is April's.
  A trip is never split across periods.
- **Days are UTC.** `observed_at` is a timezone-less UTC estimate, so `started_on` and the
  period starts are UTC calendar days and no session `TimeZone` setting changes them. A
  consumer that wants local days passes UTC range edges for its zone's midnight or accepts that
  a late-evening trip in the Americas is the next UTC day.
- **A trip with no wall-clock time** (`started_at` is `NULL`) belongs to no period and is not in
  `v_trip_period`; it is still in `v_trip_summary`.
- **No row means zero.** A vehicle with no trip in the range has no row; a consumer treats
  absence as zero trips, 0 m and 0 s.
- **Types.** `trips` and `duration_ms` are `BIGINT`, `max_obd_speed_kph` is `SMALLINT`.
  DuckDB widens `sum(BIGINT)` to `HUGEINT`, which `POST /query` returns as a string; cast
  (`sum(trips)::BIGINT`) when summing the macro's columns.
- **Isolation.** Every join carries `vehicle_id` equality, so one car's trips can never be
  counted in another's period even where boot ids and start times coincide.

**Endpoints** (loopback, no authentication of their own): `POST /query`, `GET /healthz`,
`GET /status`, `POST /reload`, `GET /metrics`, `GET /snapshot` (Parquet, optionally
`?vehicle=` and `?format=`), plus the server's local vehicles API for display names.

**The rule:** this contract is **additive**. New views and columns may appear; existing ones
are not repurposed or removed without a new `store/v2`. The web layer's queries are the
real consumer, which is why the route inventory runs against a store started from the
server release the web repository pins.

- **Status:** draft (the schema is machine-checked; the endpoints are still prose).
