# Changelog

Contract releases are tagged `contracts-vX.Y.Z` and cut by a maintainer; no CI job tags or
publishes. Protocol directories are versioned independently of the collection tag.

## Unreleased

- `store/v1.1` (additive; needs a tagged release before a server can pin it): the period
  views. `v_trip_period` (one row per trip with its week, month, quarter and year start) and the
  table macro `period_summary(from_day, to_day)` (per-vehicle trips, duration, distance and
  maximum speeds over a half-open date range). A trip belongs to the UTC period it started in.
  `schema.json` gains an optional `macros` object, which the server's compatible-range check
  reads (a pinned macro must exist with the same parameters and columns); a `store/v1.0` file
  is still satisfied. Nothing existing changed. See `store/v1/README.md`. Refs
  ParkWardRR/cairn-vehicle-server#14.

- `store/v1/README.md`: documents the `GET /healthz` JSON (`status`, `build`, `store_contract`), the
  `GET /capabilities` response (tables, views, columns, endpoints) and the minor-version rule (a minor
  increments on any additive change; removing, renaming or repurposing is `store/v2`), taken from the
  server implementation. Documentation only; `schema.json` and every vector are unchanged, so nothing is
  released for it.
- `format/v3/spec.md`: a link to the firmware hardening notes now points at the firmware
  repository. Documentation only; no protocol or vector changed, so nothing is released for it.

## 0.2.0 (2026-10-05)

New vectors and a machine-readable store schema, plus the `ble/v1` and `uplink/v1` drafts. A
manifest with `valid: false` must now be refused at parse by every implementation, so a
runner pinning this release needs the new expectation fields (see below).

- `ble/v1` (**draft**, additive): `device-info.md` (a read-only `DEVICE_INFO` value: capabilities, firmware, identity,
  storage, transports, installed engines, boot timing, as unknown-skipping records; `UPLINK_EVENT` slot
  announcements) and `checkin.md` (four server-signed instruction types with a replay counter, and an unsigned,
  bounded `HOME_TRIGGER`). `PROTOCOL_VERSION` capability bit 3 = device information. 29 vectors including
  negatives. No existing byte or vector changed.
- `uplink/v1` (**draft**, new): the device uplink protocol for direct Wi-Fi and LTE upload. Per-request Ed25519
  signatures by the enrolled device key (`CAIRN-UPLINK-V1`, eight-line signing string), a server key pinned in
  firmware, the relay's offer, chunk, commit and receipt under `/v1/uplink/bundles/`, and 15 deterministic
  vectors including negative cases. Additive: no existing protocol or vector changed.
- `store/v1/schema.json`: the first machine-readable store contract, 8 tables and 10 views
  generated from the server's own native schema (`store/v1.0`). Read as a compatible range
  (see `store/v1/README.md`). Content change: needs a tagged release before a server can pin it.
- `sync/v1/vectors/exchanges.json` (added, with `vectors/README.md`): 83 request/response
  exchanges generated from the server, covering signed and bearer push, pull, ack, vehicle
  scoping, the bundle relay, and negatives (bad signature, replay, wrong scope, expired token,
  unknown vehicle, clock skew, revocation). New vectors, no change to any existing byte;
  `sync/v1` stays `draft` until the iOS app passes them. A release must be tagged for the
  server's CI to check against them.
- `sync/v1/spec.md`: clarified, not changed: a bearer token on a signed-only route is
  `401 signature_required` (what the server has always sent; §2.5 said every 401 is
  `unauthenticated`), and the relay's stable error codes are named (§13).
- `format/v3`: 26 negative vectors (now 59): header magic / version / short, frame header tamper, manifest
  tamper / wrong key / unsupported version / non-canonical / missing mandatory field, one vector per
  manifest binding rule, receipt bad signature / wrong signer / tampered root / unsupported version /
  non-canonical / truncated / trailing bytes, update descriptor tamper / wrong key. New expectation
  fields: `receipt.parse_error`, and `header.parse_error` values `bad_magic`, `short_header`,
  `unsupported_format_version`. A manifest with `valid: false` must now be refused at parse by every
  implementation. Existing vectors are byte-identical.
- `enrolment/v1`: `vectors/negative.json` (32 blobs, 5 stateful sequences).
- Each protocol README lists its negative cases (`sync/v1`'s are in `exchanges.json`).
- A runner that has not learned the new fields fails loudly, so the firmware runners need
  updating before they pin this release.

## 0.1.0 (2026-10-05)

The first release: the contracts move out of the monorepo into their own repository, and
the component repositories build against this tag instead of a local path.

- Collected into one place from the monorepo: `format/v3`, `enrolment/v1`, `sync/v1`,
  `ble/v1` (companion and offload), with their vectors. No byte changed in the move.
- Added `store/v1` and `share/v1` as drafts.
- Byte-level independence check: the `format/v3` vectors (33) pass unchanged in the Go reference,
  the C firmware implementation and the Rust emulator, and the generator reproduces them with no
  diff. `ble/v1` offload vectors are generated from the firmware module and match.
