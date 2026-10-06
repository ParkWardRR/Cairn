# Analytical store v1 (draft)

What the web dashboard relies on from the server's analytical store, `cairn-tsdb`. Today it
is described here in prose; a machine-checked `schema.json` and the native-schema
comparison are a later milestone (the split plan's phase 9).

**Tables:** `bundles`, `position`, `imu`, `obd`, `boost`, `status`, `transition`, `gap`.
**Views:** `v_telemetry`, `v_reproducibility`, `v_vehicles`, `v_drive_summary`,
`v_trip_summary`, `v_trim_map`, `v_boost_curve`, `v_pulls`, `v_speed_agreement`,
`v_gnss_sources`. Every table and view carries `vehicle_id`.

**Endpoints** (loopback, no authentication of their own): `POST /query`, `GET /healthz`,
`GET /status`, `POST /reload`, `GET /metrics`, `GET /snapshot` (Parquet, optionally
`?vehicle=` and `?format=`), plus the server's local vehicles API for display names.

**The rule:** this contract is **additive**. New views and columns may appear; existing ones
are not repurposed or removed without a new `store/v2`. The web layer's queries are the
real consumer, which is why the route inventory runs against a store started from the
server release the web repository pins.

- **Status:** draft.
