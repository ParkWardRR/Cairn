# Cairn app sync protocol (v1)

For the iOS companion. Precise enough to implement without reading the server.
Design rationale and threat model: [trust-model-v3.md](../../../docs/trust-model-v3.md).
Server code: `server/internal/{clients,syncapi,audit}`; the signing vector in §2.4
is checked by a server test, so it cannot drift from this page.

**Transport.** JSON over HTTPS. Two base URLs for *one* logical account (§9):
the LAN listener and the Tailnet URL. Same API, same identity, same cursor.

**Authentication.** Not mTLS — `tailscale serve` terminates TLS, so a client
certificate cannot reach Cairn on the Tailnet path. Every request is **signed**
with a P-256 key held in the Secure Enclave (§2). Reaching the server over a
Tailnet is not a credential.

## 1. Endpoints

| Method + path | Auth | Purpose |
|---|---|---|
| `GET /v1/health` | none | Liveness, protocol version, instance id. Reveals nothing about cars or devices |
| `POST /v1/enroll/app` | invitation code + proof | Enrol this installation |
| `POST /v1/auth/token` | signed | Mint a 1-hour bearer token for background transfers |
| `POST /v1/sync/push` | signed or bearer | Send operations |
| `GET /v1/sync/pull` | signed or bearer | Receive changes after a cursor |
| `POST /v1/sync/ack` | signed or bearer | Record how far this client has durably applied |
| `GET /v1/snapshot?vehicle=<id>` | signed or bearer | Parquet snapshot of the analytical store (the History tab's data), filtered to one vehicle. **A client scoped to specific vehicles must name one of its own**; a client scoped to every vehicle (`*`) may omit it for the whole archive. `?format=` and `If-None-Match`/`ETag` are honoured. `403 vehicle_required` / `403 scope` otherwise |
| `GET /v1/devices` | signed, admin | List dongles (status only) |
| `POST /v1/devices/{id}/revoke` | signed, admin | Disable a dongle immediately |
| `GET /v1/clients` | signed, admin | List app installations |
| `POST /v1/clients/{id}/revoke` | signed, admin | Disable an app installation immediately |

Limits: request body ≤ 1 MiB; ≤ 200 operations per push; pull `limit` default
100, max 500.

## 2. Authentication

### 2.1 Key

Generate a **P-256** key in the Secure Enclave
(`SecureEnclave.P256.Signing.PrivateKey`, or `SecKeyCreateRandomKey` with
`kSecAttrTokenIDSecureEnclave`). It must never leave the device. The public key
is sent as the **uncompressed X9.63 point, 65 bytes, lowercase hex** (130
characters, starts `04`). CryptoKit: `publicKey.x963Representation`.

### 2.2 Signed requests

```text
Authorization: Cairn-Sig client="<client_id>",ts="<unix seconds>",nonce="<32 hex>",sig="<base64 DER>"
```

`sig` is the ASN.1 **DER** ECDSA-P256-SHA256 signature (CryptoKit:
`signature.derRepresentation`; `SecKeyCreateSignature` with
`.ecdsaSignatureMessageX962SHA256`) over the UTF-8 bytes of this string, joined
with single `\n` and **no trailing newline**:

```text
CAIRN-SIG-V1
<METHOD>
<request target exactly as sent: path plus ?query>
<ts>
<nonce>
<lowercase hex SHA-256 of the request body; of the empty string if none>
<client_id>
```

- `request target` is the path and query **as you put them on the wire** — do
  not normalise, re-encode or reorder. Sign the same string you send.
- `nonce` is 16 random bytes as 32 lowercase hex characters, fresh per request.
- `ts` is the phone's wall clock. The server accepts ±120 s. If requests start
  failing with 401 and the clock is wrong, that is why; `GET /v1/health` returns
  `server_time` for diagnosing skew.
- A nonce is accepted once per client within the window. **Never retry a
  request with the same `Authorization` header** — build a new one, even for a
  network-level retry.

### 2.3 Bearer tokens (background transfers)

A background `URLSession` task is created ahead of time and may run much later,
past the ±120 s window. For those:

1. `POST /v1/auth/token` (signed, empty body) →
   `{"token":"…","expires_at":"…","scope":"sync"}`
2. Create the background request with `Authorization: Bearer <token>`.

A token lasts 1 hour, works only on `/v1/sync/*` and `/v1/relay/*` (§13), never on
administration or on `/v1/auth/token`, and **stops working the moment the client is revoked** (it is
re-checked against the registry on every use). Treat it as a credential: Keychain
only, never logged. A server restart invalidates outstanding tokens; mint a new
one on 401.

### 2.4 Test vector

Public test key — protects nothing:

| | |
|---|---|
| private scalar (hex) | `11065c969d488e9a896d78d3fcc6d3b831da4d22df873d6f3668b4c55b28b889` |
| public key (X9.63 hex) | `049865616d17bd8336564c615ad4076c347938be436834bae774437ab3530e216b21025bb8042070ee6c9276fa4dd857e2525a8a2915aba1e45761cedfd8aada39` |
| client_id | `0190a1b2c3d47e5f8a6b7c8d9e0f1a2b` |
| method / target | `POST` `/v1/sync/ack` |
| body | `{"cursor":"AAAA"}` |
| ts / nonce | `1790000000` / `00112233445566778899aabbccddeeff` |

Signing string (`␊` = `\n`):

```text
CAIRN-SIG-V1␊POST␊/v1/sync/ack␊1790000000␊00112233445566778899aabbccddeeff␊264ef7813246976045be09d319d4cecdbb620331ca551f04d0e8e07f25946468␊0190a1b2c3d47e5f8a6b7c8d9e0f1a2b
```

A signature over it that **must verify** (ECDSA is randomised, so yours will
differ; verifying this one with the public key above proves your string and your
key handling agree with the server's):

```text
MEUCIDZQYt9QDF5PJ43nMAA9D5/HNm+d+QwpABTSQyLdxMVdAiEAjMSDavgvRW6dg2vGSPelsKCoBezOsZMvDuC770JXZ+0=
```

More vectors, in `contracts/sync/v1/vectors/vectors.json`: a POST with a JSON body, a GET with a
query string and no body, a POST with no body, a PUT with binary bytes, and an enrolment
proof. Each has the exact signing string and a signature that verifies over it with the test
key above. They are pinned by `go test ./internal/syncapi -run AppSyncVectors`.

Whole request/response exchanges, in `contracts/sync/v1/vectors/exchanges.json`: signed and
bearer push, pull, ack, vehicle scoping, the bundle relay (offer, chunk, commit, receipt),
and the negatives (bad signature, replay, wrong scope, expired token, unknown vehicle, clock
skew, revocation). Each step carries its inputs (method, request target, headers, body, the
server's clock, the nonce) and the exact status, error code and body the server answered
with. [`vectors/README.md`](vectors/README.md) lists the cases and how to use them.

### 2.5 Failures

Every authentication failure is `401` with `{"error":"unauthenticated","message":"authentication failed"}` — deliberately uniform, so a probe cannot tell an unknown
client from a bad signature from a replay. (One exception: a bearer token sent to a route that
accepts signatures only, `/v1/auth/token` and administration, is `401 signature_required`, since
the client knows which credential it sent. Do not branch on the cause of any other 401.)
`403 admin_required` means authenticated
but not an admin. `403 funnel_refused` / `403 tailnet_identity_required` are
network-policy refusals. `429 rate_limited`: back off.

## 3. Enrolment

An administrator runs `cairn-admin client invite --role user --vehicles '*'` and
reads you a code shown **once**, e.g. `ee02-6e94-4c7a-389c-6f80-3600-648b-75aa`
(dashes and case are ignored). It expires in 10 minutes and works once.

`POST /v1/enroll/app`, unsigned:

```json
{ "code": "ee026e944c7a389c6f803600648b75aa",
  "name": "Example iPhone",
  "public_key": "04…130 hex…",
  "proof": "<base64 DER signature>" }
```

`proof` is your new key's signature over (single `\n`, no trailing newline):

```text
CAIRN-ENROLL-V1
<code: 32 lowercase hex, dashes removed>
<public_key, lowercase hex>
```

This proves you hold the private half. A bad proof does **not** consume the
invitation. Response `201`:

```json
{ "client_id": "0190…", "role": "user", "vehicles": ["*"], "server_time": "2026-10-05T12:00:00Z",
  "server_identity": { "instance_id": "0011…", "spki_sha256": "…hex…" } }
```

Store `client_id`, `instance_id` and `spki_sha256` (the pin for the LAN TLS
leaf; empty when the listener is plain HTTP behind Tailscale Serve). `403
enrolment_refused` covers wrong, expired and used codes alike.

### 3.1 Replacing a client (key rotation, a restored or replaced phone)

An invitation may name one existing client that it **replaces**. An administrator creates it
with `cairn-admin client invite -replaces <client_id>`; the role, vehicles and name default
to the old client's, so rotating a key does not change what the phone may do (flags
override). There is no HTTP endpoint for creating invitations, so this is administrator-side
only. The app's part is unchanged: it enrols exactly as in §3, with a fresh key, and the
invitation code is the only thing that differs.

Accepting such an invitation is **one write on the server**: the new client is created and
the named client is revoked together. There is no moment at which two keys work, or none.
The new client keeps the old client's owner.

What the app can observe:

- The enrolment response (`201`) is exactly the one in §3. It has no field saying that a
  client was replaced, deliberately, so a probe learns nothing from it.
- After it, a request signed by the **old** key gets the uniform `401` of §2.5. The app
  should treat that as "this installation was replaced", not as a clock problem, only after
  its own enrolment under the new key has succeeded.
- A bad `proof` neither consumes the invitation nor revokes anything, as in §3. The old key
  keeps working until the replacing enrolment succeeds.
- A named client that does not exist is refused when the invitation is created, not when it
  is used. A named client that is already revoked keeps its original revocation reason.

Recovering a restored phone is therefore: the administrator mints a replacing invitation,
reads out the code, the app enrols with a new key, and the old key is dead. This replaces a
three-step recovery (enrol a new client, revoke the old one, move the owner), which left a
window with two working keys and could be interrupted part way.

## 4. Operations (push)

`POST /v1/sync/push`:

```json
{ "operations": [ {
  "operation_id": "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b",
  "client_id": "<your client id>",
  "vehicle_id": "<vehicle id>",
  "kind": "maintenance_event",
  "created_at": "2026-10-05T12:00:00Z",
  "payload_version": 1,
  "payload": { "kind": "oil", "odometer_km": 45210 },
  "content_hash": "<hex sha256 of the canonical payload, §4.1>",
  "idempotency_key": "<optional, ≤128 bytes>",
  "base_revision": { "title": 1 }
} ] }
```

- `operation_id`: a UUID (v7 recommended). Hyphenated upper-case
  (`UUID.uuidString`) and bare lowercase hex are the same id.
- `created_at`: RFC 3339. Informational — **ordering is the server's**.
- `base_revision`: mutable kinds only (§4.2).

### 4.1 Canonical payload and `content_hash`

`content_hash` = lowercase hex SHA-256 of the payload in this canonical form:

- no insignificant whitespace;
- object keys sorted by their UTF-8 bytes, ascending, no duplicates;
- strings UTF-8; only `"`, `\` and code points < U+0020 are escaped (`\"`, `\\`,
  `\u00xx` lowercase). `/`, `<`, `&` and non-ASCII appear **literally**;
- numbers are **integers only**, no exponent, no leading zeros, no `-0`, within
  ±(2⁵³−1). **No floating point.** Send decimals as strings (`"12.34"`) or scaled
  integers (cents, millimetres, tenths of a litre);
- `true`, `false`, `null` as written; nesting ≤ 12.

Vector: input `{ "target": "trip-1", "fields": { "title": "Café run / 5°C", "odometer_km": 45210 } }`
canonicalises to
`{"fields":{"odometer_km":45210,"title":"Café run / 5°C"},"target":"trip-1"}`
with hash `bfb34ea202550b8fb7c04570f8426f1a27d058069d39b2513f4861bc9c882028`.

The server recomputes the hash and refuses a mismatch (`hash_mismatch`).

### 4.2 Kinds

| Kind | Mutable? | Payload |
|---|---|---|
| `trip_annotation` | yes | `{"target":"<trip id>","fields":{…}}` |
| `odometer_correction` | yes | `{"target":"<id>","fields":{…}}` |
| `vehicle_update` | yes | `{"fields":{…}}` (the vehicle is the target) |
| `maintenance_event` | no (append-only) | any JSON object (≤ 32 KiB) |
| `observation` | no (append-only) | any JSON object |

Mutable kinds carry **per-field revisions**. Field names match
`[a-z][a-z0-9_]*`. `base_revision` maps each field you are changing to the
revision you last saw (`0` or absent for a field you have never seen). If any
field's server revision differs the whole operation is refused with `conflict`
and nothing is applied.

### 4.3 Results

`200` with `{"results":[…],"head":N}`, one result per operation, in order:

| `status` | Meaning | Client action |
|---|---|---|
| `accepted` | Recorded. `server_sequence`, and `revisions` for mutable kinds | Mark the outbox row done |
| `duplicate` | Same `operation_id` or `idempotency_key` + same content already recorded. Returns the **original** `server_sequence`; creates nothing | Mark done — this is success |
| `conflict` | `conflicts:[{field,current_revision,current_value}]` | Merge, rebuild with the new `base_revision`, new `operation_id` |
| `rejected` | `reason` below. **Permanent for this operation** | Surface; do not retry unchanged |

Rejection `reason`s: `scope`, `unknown_vehicle`, `archived_vehicle`,
`hash_mismatch`, `payload_too_large`, `unsupported_kind`,
`unsupported_payload_version`, `invalid_payload`, `invalid_operation`,
`client_mismatch`, `operation_id_conflict`, `idempotency_key_conflict`.

**Retry rules.** A retry of an operation uses the **same `operation_id` and the
same content**; it is safe by construction. Reusing an id with different content
is `operation_id_conflict`. Network errors and `5xx`: retry with backoff
(1 s, 2 s, 4 s … capped at 5 min, jittered). `401`: re-sign (check the clock);
`4xx` other than `429`: do not retry the request unchanged.

## 5. Pull

`GET /v1/sync/pull?cursor=<opaque>&limit=100`. Omit `cursor` the first time.

```json
{ "epoch": "…", "cursor": "<opaque>", "has_more": true,
  "changes": [
    { "server_sequence": 12, "at": 1790000000123, "type": "entity",
      "entity_type": "vehicle", "entity_id": "…", "vehicle_id": "…",
      "data": { "display_name": "2017 BMW M240i — B58", "engine_code": "B58",
                "vin_last4": "3456", "archived": false } },
    { "server_sequence": 13, "at": 1790000000456, "type": "operation",
      "operation": { /* the operation as stored, with "revisions" */ } }
  ] }
```

- Changes are in `server_sequence` order and **scoped to your vehicles**.
- `entity_type`: `vehicle`, `assignment` (device↔vehicle, read-only),
  `trip_summary` (server-published). A later record for the same
  `(entity_type, entity_id)` supersedes the earlier.
- The full VIN is never sent; only `vin_last4`.
- **Loop until `has_more` is false**, applying each page transactionally and
  persisting `cursor` in the same transaction.
- The cursor is opaque, durable and **repeatable**: pulling the same cursor again
  returns the same page. Never construct or parse one.
- `410` `{"error":"cursor_reset"}`: the server's data was reset or the cursor is
  older than the retained log. **Discard local server-derived data and pull from
  scratch** (keep your unsent outbox). Do not treat 410 as "up to date".

## 6. Ack

`POST /v1/sync/ack {"cursor":"<the cursor you have durably applied>"}` →
`{"acknowledged":N}`. Send it after the transaction that applied a pull
commits. It only ever moves forward on the server.

## 7. Administration (admin role)

`POST /v1/devices/{id}/revoke` and `POST /v1/clients/{id}/revoke` take
`{"reason":"…"}` (optional) and are effective on the target's next request. They
exist so losing a dongle or a phone is one action from the app. `GET /v1/devices`
and `GET /v1/clients` return status and key ids only — never key material.

## 8. Health and endpoint selection

`GET /v1/health` (no auth) → `{"status":"ok","protocol_version":1,"instance_id":"…","server_time":"…","build":{"version":"v0.3.1-4-gabc123","commit":"<40 hex>"}}`.

`build` says which server build answered; `modified` is added (as `true`) only when the
server was built from a tree with uncommitted changes. It is additive: a client that does
not know `build` ignores it. It is informational and must not be used to choose a route or
to decide that two answers come from the same server; `instance_id` does that.

Use it to choose a route:

1. On a trusted Wi-Fi (or whenever the LAN base URL answers `/v1/health` quickly,
   timeout ~2 s) use the **LAN** URL.
2. Otherwise try the **Tailnet** URL.
3. The `instance_id` from both must equal the one stored at enrolment — if not,
   **stop**: it is a different server.

It is one account on one server: the same `client_id`, the same cursor, the same
outbox. Never keep separate state per route. Surface the route in the UI as
`LAN`, `Tailnet` or `Unreachable`.

## 9. TLS and pinning

On the LAN listener pin `server_identity.spki_sha256` (SHA-256 of the leaf's
SubjectPublicKeyInfo) in a `URLSessionDelegate`; do not rely on the system trust
store alone for a private CA. On the Tailnet URL the certificate is Tailscale's
ordinary one — validate it normally; do not disable validation. Never accept an
arbitrary self-signed certificate.

## 10. Background behaviour

Do not assume Tailscale is up in the background: iOS may not keep the VPN
active. Design as a durable outbox: operations are written locally first, sent
when a route exists, and resumed by background `URLSession` transfers (using a
bearer token, §2.3) or the next foreground sync. Tailscale is a route
preference, not a correctness dependency.

## 11. What must never be logged

Bearer tokens, signatures, request bodies, GPS coordinates, SSIDs, the full VIN,
the invitation code. The server's audit log records actor, route, transport and
a body hash only; hold your own logs to the same standard (the existing
`cairn-drive.log` is user-shareable, so keep these out of it).

## 12. Appendix: the snapshot and trip_summary entities

### Vehicle-scoped snapshot

`GET /v1/snapshot?vehicle=<32 hex>` returns the Parquet archive with **every table
filtered to that one vehicle**; each table carries a `vehicle_id` column. The
manifest's `schema_version` is bumped for this: the app must keep its
`isSupported` guard and tell the user to update rather than misread tables that
now have a column it does not know about.

| Request | Client scope `*` | Client scoped to specific vehicles |
|---|---|---|
| no `vehicle` | whole archive, every car | **403 `vehicle_required`** |
| `vehicle` = one of its own | that car | that car |
| `vehicle` = another car | that car | **403 `scope`** |
| malformed `vehicle` | 400 | 400 |

A vehicle the store has no rows for is `404` upstream, not an empty archive, so
"no data for that car" is distinguishable from "that car has no trips in this
table".

### `trip_summary` entities

Published in `pull` as `entity_type: "trip_summary"`, id `<vehicle_id>:<boot_id>`,
scoped to the vehicle. **All values are integers** (§4.1): timestamps are epoch
milliseconds, speeds are cm/s, distance is whole metres.

```json
{ "boot_id": "…", "device_id": "…",
  "started_ms": 1790000000000, "ended_ms": 1790000600000, "duration_ms": 600000,
  "distance_m": 5230,
  "max_gnss_speed_cmps": 2410, "max_obd_speed_cmps": 2500, "max_rpm": 6100,
  "obd_samples": 300, "gnss_samples": 590, "boost_samples": 300,
  "gap_count": 1, "gap_duration_ms": 4000,
  "bundle_count": 1, "decoder_version": 3 }
```

A later record for the same entity id supersedes the earlier one (a trip whose
bundles were reprocessed, or that grew as more of it arrived). Treat the latest
as the truth; do not merge.

## 13. Bundle relay (the phone uploads on the dongle's behalf)

The dongle has no network ([ble-offload.md](../../ble/v1/offload.md)); the enrolled phone
carries its sealed bundles to the server. These endpoints are the same
offer → chunk → commit exchange the dongle used to make itself, now authenticated
as **the phone** (§2) instead of by a device client certificate. They accept signed
requests or bearer tokens, so a background `URLSession` upload works.

| Endpoint | Body | Response |
|---|---|---|
| `POST /v1/relay/bundles/offer` | `manifest.cbor` verbatim (`application/cbor`); header `X-Cairn-Signature: <manifest.sig, 128 hex>` | `200` `{ "bundle_id", "missing_chunks": [ { "index", "offset", "length", "sha256" } ], "receipt_available": bool }` |
| `PUT /v1/relay/bundles/{bundle_id}/chunks/{sha256}` | the chunk's bytes (`application/octet-stream`) | `200` when the bytes hash to `{sha256}` and match the manifest's descriptor |
| `POST /v1/relay/bundles/{bundle_id}/commit` | empty | `200`, body = the signed receipt (CBOR), **taken verbatim** — the signature covers those exact bytes |
| `GET /v1/relay/bundles/{bundle_id}/receipt` | — | `200` the same receipt again; `404` if none yet |

Unlike the retired device endpoint, `offer` returns each missing chunk's `offset`
and `length` in the bundle byte stream, so the phone needs **no CBOR parser**: it
reads exactly that range from the dongle (`READ`), checks the SHA-256 and `PUT`s it.
Chunks are idempotent; offering a bundle the server already holds returns no
missing chunks and `receipt_available: true`.

### Who the server believes

The manifest is signed by the **device** key, so the server checks the signature
against the enrolled device, that the manifest's assignment is open and its
counter is not a replay (the same rules as before, `docs/trust-model-v3.md` §4).
The phone's signature authenticates *who is relaying*, not *what the data is*.

**Scope.** The bundle's vehicle (from its manifest) must be in the requesting
client's scope, or the answer is `403 scope`. An `*` client may relay for any car.

### Errors

`401` unauthenticated (§2.5) · `401 bad_manifest_signature` (the device's signature over
the manifest does not verify; unlike the rest of §2.5 this is about the data, not the caller) ·
`400 bad_request` (a missing or malformed `X-Cairn-Signature`) · `403 scope` · `403
assignment_refused` (the manifest names an assignment the server did not issue) · `403
no_storage_key` (the device's root was never escrowed) · `404 unknown_bundle` (no offer on
record, including after an out-of-scope offer, which writes nothing) · `404 no_receipt` ·
`422 quarantined` (a counter reused with different content) · `409 chunk_mismatch` (the
bytes do not hash to `{sha256}`, or do not match the descriptor) · `409 chunks_missing`
(commit before every chunk is uploaded) · `429` back off.
A refused or quarantined bundle is **never** receipted, so it stays on the dongle.

### What must never be logged

Nothing new, but note the phone handles ciphertext and signed manifests only; there
is no key material in this exchange.
