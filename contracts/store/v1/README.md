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
`GET /capabilities`, `GET /status`, `POST /reload`, `GET /metrics`, `GET /snapshot`
(Parquet, optionally `?vehicle=` and `?format=`), plus the server's local vehicles API for
display names.

## `GET /healthz`

Answers "are you serving, which build are you, and which store contract do you implement?".
The last two are what a deploy check compares: a version number alone would not notice a
release that removed a view the dashboard queries. The response is `200` with a JSON body:

```json
{"status":"ok","build":{"version":"<version>","commit":"<40 hex>"},"store_contract":"store/v1.0"}
```

| Field | Type | Meaning |
|---|---|---|
| `status` | string | `"ok"` while the server is serving |
| `build.version` | string | The build's version label, for example the output of `git describe`; `"dev"` when none was stamped in |
| `build.commit` | string | The commit the binary was built from; `"unknown"` when the toolchain recorded none |
| `build.modified` | boolean | Present, and `true`, only when the tree the binary was built from had uncommitted changes (so the commit alone cannot reproduce it). **Omitted when false** |
| `store_contract` | string | `store/v<major>.<minor>`: the store contract this server implements (see the rule below) |

When the store did not reproduce from the bundles (and the server was not started to serve
unreproduced data anyway), or no store is loaded, `/healthz` is `503` with a plain-text body
(`not reproducible`), not JSON. A consumer reads the status code first and parses the body
only on `200`.

Before this field set existed the body was the bare text `ok`. A consumer that compared the
whole body to `ok` must instead parse the JSON and read `status`. Consumers must ignore
fields they do not know.

## `GET /capabilities`

What the store that is serving actually offers, read from its live catalogue rather than from
a list that was true when someone last edited the code. The response is `200` with a JSON
body (`503` with a plain-text body if no store is loaded yet):

```json
{
  "build": {"version": "<version>", "commit": "<40 hex>"},
  "store": {
    "store_contract": "store/v1.0",
    "schema_fingerprint": "<64 hex>",
    "tables": ["boost", "bundles", "..."],
    "views": ["v_boost_curve", "v_drive_summary", "..."],
    "columns": {"<table or view>": ["vehicle_id", "..."]},
    "column_types": {"<table or view>": ["VARCHAR", "..."]}
  },
  "endpoints": ["POST /query", "GET /healthz", "GET /capabilities", "..."]
}
```

| Field | Type | Meaning |
|---|---|---|
| `build` | object | The same object as in `/healthz` |
| `store.store_contract` | string | The same value as in `/healthz` |
| `store.schema_fingerprint` | string | Lowercase hex SHA-256 of the sorted lines `<name>.<column> <TYPE>`, one per column of every table and view, joined by `\n`. It changes when any column is added, removed or retyped, so two servers with the same fingerprint have the same schema |
| `store.tables` | array of string | Every base table in the store, sorted by name |
| `store.views` | array of string | Every view in the store, sorted by name |
| `store.columns` | object | For every table and view, its column names in declared order |
| `store.column_types` | object | Parallel to `columns`: the DuckDB type of each column, as the catalogue spells it. Added after `/capabilities` first shipped (with the `schema.json` check), so a consumer should treat it as optional |
| `endpoints` | array of string | The endpoints the server serves, each as `<METHOD> <path>` |

`tables`, `views` and the keys of `columns` are what [`schema.json`](schema.json) pins, so a
consumer can compare the two: every table, view and column in `schema.json` must be present
here (under a compatible type), and extras are allowed. As with `/healthz`, ignore fields you
do not know.

## The rule

This contract is **additive**. The `store_contract` string is `store/v<major>.<minor>`:

- The **minor increments on any additive change**: a new table, a new view, or a new column on
  an existing one. An additive change never changes what an existing object or column means.
- **Removing, renaming or repurposing a table, view or column is `store/v2`**, a new major.
  So is retyping a column incompatibly (see the compatible-type rule above).
- A server satisfies a consumer that needs `store/vN.M` when its major is `N` and its minor is
  at least `M`. A different major is a different contract.

The server enforces this with a test that fails when the schema it builds changes without the
minor and the fingerprint being updated together. The web layer's queries are the real
consumer, which is why the route inventory runs against a store started from the server
release the web repository pins, and why a deploy check reads `store_contract` from
`/healthz`.

- **Status:** draft (the schema is machine-checked; `/healthz` and `/capabilities` are documented here from the implementation, not yet machine-checked).
