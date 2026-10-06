# Analytical store v1 (draft)

What the web dashboard relies on from the server's analytical store, `cairn-tsdb`. Its
objects and columns are pinned in [`schema.json`](schema.json); this file carries the rest.

**Tables:** `bundles`, `position`, `imu`, `obd`, `boost`, `status`, `transition`, `gap`.
**Views:** `v_telemetry`, `v_reproducibility`, `v_vehicles`, `v_drive_summary`,
`v_trip_summary`, `v_trim_map`, `v_boost_curve`, `v_pulls`, `v_speed_agreement`,
`v_gnss_sources`. Every table and view carries `vehicle_id`.

**`schema.json`** lists every table and view with its columns and DuckDB types, and the
`store/v1.N` it was generated from. It is generated, not written: the server's
`go run ./cmd/dump-schema` starts the real store and dumps the schema it built for itself.
It is read as a **compatible range**: a server release satisfies it when it has every listed
table and view, each listed column under a compatible type, and at least that minor. Extra
tables, views and columns are allowed (that is what a minor adds), and column order does not
matter. A **compatible type** is the same type, or a wider integer of the same signedness
(`UINTEGER` to `UBIGINT`); anything else (`VARCHAR` to `INTEGER`, `FLOAT` to `DOUBLE`,
signed to unsigned, narrowing) is a retype and breaks the contract. The server's CI compares
its native schema with the pinned file; this repository's CI checks only the file's own
shape (valid, known fields, every object has `vehicle_id`).

**Endpoints** (loopback, no authentication of their own): `POST /query`, `GET /healthz`,
`GET /status`, `POST /reload`, `GET /metrics`, `GET /snapshot` (Parquet, optionally
`?vehicle=` and `?format=`), plus the server's local vehicles API for display names.

**The rule:** this contract is **additive**. New views and columns may appear; existing ones
are not repurposed or removed without a new `store/v2`. The web layer's queries are the
real consumer, which is why the route inventory runs against a store started from the
server release the web repository pins.

- **Status:** draft (the schema is machine-checked; the endpoints are still prose).
