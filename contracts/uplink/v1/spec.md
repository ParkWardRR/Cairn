# Device uplink protocol (uplink v1, DRAFT)

How a dongle with Wi-Fi or LTE uploads sealed bundles to the server **by itself**: no phone,
no client certificate, no private key beyond the one it already has. This is the
transport-level counterpart of the relay in [`sync/v1` §13](../../sync/v1/spec.md) and of the
BLE hand-off in [`ble/v1` offload](../../ble/v1/offload.md); all three move the same bundles
to the same receipts.

> **Draft.** Nothing ships this yet. The vectors are produced by a Go reference
> (`tools/uplinkvectors`) and the four signing strings and signatures were reproduced
> byte for byte by a second implementation (Python on OpenSSL, run once by hand). Per
> [how a contract changes](../../README.md), the firmware's C must reproduce them before
> this can be called more than a draft. The threat-model review
> that the issue requires ([issue 23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23))
> has not happened. Decisions that are the owner's are marked **Owner decision**.

Context: [issue 22](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/22)
(this protocol), [issue 19](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/19)
(the dongle gets Wi-Fi and LTE back), and
[bundle format v3](../../format/v3/spec.md) §6 (offer, chunk, commit, receipt).

## 1. What does not change

- **Bundles, manifests and receipts are byte-for-byte the v3 ones.** The manifest is signed by
  the device key; the receipt is signed by the server key pinned in firmware and names the
  bundle's content root. The dongle deletes only after verifying a receipt, whatever path
  carried it. A receipt obtained over this protocol and one obtained over BLE for the same
  bundle are **the same bytes** (the server stores its first receipt and returns it verbatim).
- **The uploader is trusted for availability only.** Confidentiality and integrity come from
  the sealed bundle and the manifest signature, never from the link.
- Offer, chunk, commit and receipt keep their relay meaning, bodies and status codes.

## 2. Authentication: the device signs each request

No client certificate, no CA, nothing new to provision. Each request carries a signature by
the device's **enrolled Ed25519 signing key** (the same key that signs manifests and the
enrolment blob; [`enrolment/v1`](../../enrolment/v1/spec.md)).

```text
Authorization: Cairn-Device device=<32 hex>,ts=<unix seconds>,nonce=<32 hex>,sig=<128 hex>
```

`device` is the 16-byte `device_id`; `nonce` is 16 bytes from the hardware RNG, fresh per
request; `sig` is the 64-byte Ed25519 signature over the **signing string**, all lowercase hex.

### 2.1 The signing string

Eight lines, `\n` between them, **no trailing newline**, UTF-8:

```text
CAIRN-UPLINK-V1
<method, upper case>
<request target: path and query, exactly as sent>
<ts: decimal Unix seconds, as in the header>
<nonce: as in the header>
<sha256 of the request body, lowercase hex; of the empty string when there is none>
<device_id: as in the header>
<meta: the value of the Cairn-Meta header, or empty>
```

Ed25519 is deterministic, so a signature is **"must equal"** in the vectors (unlike the app
protocol's randomised ECDSA).

*Domain separation.* The device key also signs manifests (CBOR maps, first byte `0xA0`-`0xBF`)
and enrolment blobs (magic `CENR`). A signing string starts with the ASCII bytes
`CAIRN-UPLINK-V1\n`, which can be neither, so a signature made for one purpose is never valid
for another.

For a chunk upload the body hash is the SHA-256 the device already holds (it is in the URL),
so signing a chunk costs one Ed25519 signature and no extra hashing.

### 2.2 The server's checks, in order

1. Parse the header. Unknown, revoked or malformed `device`, a bad `sig`, or a `nonce` already
   seen inside the window: **`401 {"error":"unauthenticated"}`**, one uniform answer, as in
   [`sync/v1` §2.5](../../sync/v1/spec.md). A probe cannot tell an unknown device from a bad
   signature from a replay.
2. Only **after the signature verifies**, check `ts`: more than **120 s** from the server's
   clock gives `401 {"error":"clock_skew","server_time":"<RFC 3339>"}`. This reveals nothing
   to anyone who cannot already sign as the device.
3. Remember `(device, nonce)` for 240 s (twice the window), so a captured request cannot be
   replayed inside it and ones outside it fail step 2.
4. Apply the per-device and per-address limits (§7) and the route's own checks.

Every authenticated response carries **`Cairn-Server-Time: <unix seconds>`**, so a device with
no trustworthy clock learns the time from its first response and retries once. A device that
has never had GNSS or network time sends its best guess; the worst case is one extra round
trip, and no state is trusted on the device's clock alone.

### 2.3 `Cairn-Meta`

Optional, **covered by the signature** (line 8), so a relay or middlebox cannot alter it:

```text
Cairn-Meta: path=wifi;slot=12        path=lte
```

`path` is `wifi` or `lte`; `slot` is a decimal `u32`, the device's own count of Wi-Fi visits,
present for time-sliced Wi-Fi. The server records `(bundle_id, path, slot, time)` in its
ledger on `offer` and `commit`, which is how the dashboard shows which path moved a bundle.
**It is bookkeeping, never evidence:** it is not in the receipt, a receipt is never
conditioned on it, and the server must not trust it for anything but display. An unrecognised
value is `400`. BLE relayed bundles keep recording the phone as the path.

## 3. Transport security: the server's key is pinned in firmware

TLS 1.3 only. The firmware **pins the SHA-256 of the SubjectPublicKeyInfo** of the server's
uplink TLS key and accepts **no other**, whatever certificate chain is presented: no CA store
on the device, no clock needed to check validity, a smaller and faster handshake.

- **Two pins** are baked in: the current key and a backup, so a key can be rotated without
  stranding a unit in the field. The server's uplink TLS key is therefore a **dedicated,
  long-lived key**, not a short-lived certificate key: reuse the key when the certificate is
  renewed (`--reuse-key` with ACME), or use a self-managed certificate.
- A hostile Wi-Fi network, a rogue base station or a carrier can then see **traffic metadata
  only** (when, how much, to which address). They cannot impersonate the server, so cannot
  learn a nonce's meaning, serve a fake receipt, or withhold selectively without it being
  just an outage.
- *Defence in depth.* Even a fully broken TLS leaves the receipt unforgeable, because the
  dongle verifies it against the pinned receipt key ([`format/v3` §6.3](../../format/v3/spec.md)),
  and the uploader cannot read the data, because it is sealed.
- A device whose pins do not match any key the server holds simply fails to connect. That is
  an outage, not a bypass; it must never fall back to plain HTTP or to unpinned TLS.

## 4. Endpoints

Same shapes and status codes as the relay (`sync/v1` §13), under another prefix and with §2
authentication instead of the phone's.

| Endpoint | Body | Response |
|---|---|---|
| `POST /v1/uplink/bundles/offer` | `manifest.cbor` verbatim; header `X-Cairn-Signature: <manifest.sig, 128 hex>` | `200` `{ "bundle_id", "missing_chunks": [ { "index", "offset", "length", "sha256" } ], "receipt_available": bool }` |
| `PUT /v1/uplink/bundles/{bundle_id}/chunks/{sha256}` | the chunk's bytes | `200` when the bytes hash to `{sha256}` and match the manifest's descriptor |
| `POST /v1/uplink/bundles/{bundle_id}/commit` | empty | `200`, the signed receipt (CBOR) taken verbatim |
| `GET /v1/uplink/bundles/{bundle_id}/receipt` | none | `200` the same receipt; `404` if none yet |

That is **all** the uplink exposes. In particular there is no listing, no firmware download and
no configuration route here; those, if they exist, are specified elsewhere
(`contracts/config/v1`, [issue 27](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/27))
and are not part of this surface.

Errors as in `sync/v1` §13: `403 scope` does not apply (a device has no vehicle scope; the
manifest's assignment is checked instead), `403 assignment_refused`, `403 no_storage_key`,
`422 quarantined`, `409` chunk/descriptor mismatch, `429` back off. A refused or quarantined
bundle is never receipted.

**Who the server believes.** The request signature says which *device* is calling. The
manifest signature says what the *data* is, and is checked exactly as for the relay (open
assignment, counter not a replay, `docs/trust-model-v3.md` §4). A request signed by device A
that offers a manifest signed by device B is `403 assignment_refused`.

## 5. Cheap to start, safe to cut off

A Wi-Fi visit is short and may end at any moment (a trip starts, the slot expires). So:

- **Round trips until the server has answered the first offer: 3** (TCP, TLS 1.3, then the
  `offer` itself, which carries the manifest and returns what is missing). There is no separate hello. TLS session
  resumption is supported but not required.
- **The offer returns what the server already has** (everything not listed as missing), so a
  resumed bundle re-sends nothing. With several bundles pending, send all offers first, then
  chunks in bundle order, so the longest-waiting bundle is most likely to complete.
- **Chunks are idempotent and content-addressed.** A chunk cut off mid-body is simply sent
  again; a duplicate is a no-op `200`.
- **Nothing the device does is irrevocable until it verifies a receipt.** A session cut off at
  any byte loses nothing and leaves the bundle on the card.
- The device does not hold a session: every request stands alone (§2), so a dropped connection
  needs no state to resume.

## 6. Cost and speed

- **No compression.** Frames are XChaCha20-Poly1305 ciphertext; compressing it saves nothing.
- **Chunk size** is the manifest's (format v3 default 256 KiB). Per-request overhead is about
  500 bytes (request line, headers, the 250-byte `Authorization`), under 0.2 % of a default
  chunk; TLS adds about 22 bytes per 16 KiB record.
- **Connections.** One persistent HTTP/1.1 connection by default (keep-alive, no pipelining).
  Wi-Fi may use up to **2** in parallel; LTE uses **1**, since parallel streams on a radio
  that is already the bottleneck add handshake cost without throughput. The server accepts at
  most 2 concurrent requests per device.
- **Signing cost.** One Ed25519 signature per request, in the low milliseconds to low tens of
  milliseconds on an ESP32-class core: negligible next to an LTE round trip, but it is a number
  to **measure** on the real unit before committing to per-chunk signing.
- **Data plan.** A typical drive's bundle is megabytes, so the plan is dominated by payload,
  not protocol. Enforcing a monthly cap is the device's job and is specified with the
  configuration contract (`config/v1`), not here.

## 7. Replay, rate limits, revocation, clock

| Concern | Rule |
|---|---|
| Replay | Per-request nonce remembered for 240 s plus the ±120 s window (§2.2). Bundle-level replay is already blocked by the manifest's counter. Offer, chunk, commit and receipt are idempotent, so even a replayed request inside the window changes nothing. |
| Rate limits (defaults, `SHOULD`) | Before authentication: 30 failed requests per minute per source address, then drop. After: 120 requests per minute and 2 in flight per device. A global cap on bytes being received. Numbers are defaults, not part of the wire contract. |
| Resource exhaustion | Bodies are capped (a manifest at its format limit, a chunk at its descriptor's length, the rest empty); a body longer than declared is refused after that many bytes, not buffered. Authentication is checked **before** the body is read. |
| Enumeration | One uniform `401` for every pre-signature failure (§2.2). Unknown and revoked devices are indistinguishable from bad signatures. |
| Revocation | Revoking a device on the server makes its next request `401`. The dongle sees an outage; it keeps its data and falls back to BLE. A revoked or stolen unit cannot read anything it holds, since the storage root is on the server. |
| Clock | Trusted only coarsely (§2.2). No decision of consequence depends on the device's clock. |
| Same bundle by two paths | BLE and uplink may carry the same bundle concurrently. Offer and chunks are idempotent; the server persists one receipt per bundle and returns it verbatim to both, so both ends see identical bytes. |

## 8. Reachability and exposure

The relay is only reachable by a phone that is on the LAN or the Tailnet. A dongle on LTE is on
neither, so something must be reachable from the public internet. The options:

| Option | Pin the server key? | Verdict |
|---|---|---|
| **A. A dedicated public hostname and port**, a reverse proxy in front of the server forwarding **only** the four routes in §4 (everything else, including the app API and the dashboard, is not reachable on it) | Yes, the proxy presents the pinned key | **Proposed.** |
| B. A tunnel that terminates TLS at a third party | No: the pin would be the provider's | Rejected: it removes the property §3 depends on. Acceptable only as raw TCP pass-through, which makes it option A with extra hops. |
| C. Tailnet only | n/a | Not possible for the dongle (no Tailscale client on the chip, none over LTE). **Supported as a second route** for Wi-Fi at home, over the LAN address, with the same pins and the same protocol. |
| D. A VPN from the dongle (WireGuard) | n/a | A larger firmware and key-management change. A possible later hardening, not a v1 requirement. |

**Owner decision:** option A exposes a narrow, authenticated surface to the internet for the
first time (the server's other listeners are LAN/Tailnet only). It is proposed, not decided.
Its threat model (internet reachability, authentication cost per unauthenticated request,
enumeration, exhaustion, replay) is the subject of
[issue 23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23). Because a
failed request costs the server one hex parse and one Ed25519 verification, with the nonce
check after it, the pre-authentication work is bounded and cheap; that claim is to be checked
by that review and by a measurement, not assumed.

The full connectivity-path analysis — including the three practical paths to the
homelab (BLE → iPhone → Tailscale, Linux gateway, public HTTPS), the
store-and-forward radio scheduling model, the case for a narrow isolated
ingestion endpoint, and the privacy constraints — is in
[lte-cellular-design.md](../../../docs/lte-cellular-design.md).

## 9. Open questions for review

1. **One key, several purposes.** The device key signs manifests, the enrolment blob and now
   requests. Domain separation (§2.1) prevents cross-use, but a stolen key still authenticates
   everything. A per-purpose subkey would help but needs another secret on the chip; this
   depends on flash and NVS encryption landing first
   ([firmware issue 18](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/18)).
2. **Pin rotation procedure.** The two-pin scheme is specified; the update path that replaces
   both pins (firmware update over BLE, since the uplink may be the thing that is broken) is not.
3. **A server-issued challenge instead of timestamp and nonce** would remove the clock
   dependency entirely at the price of one more round trip per session. This draft chooses the
   timestamp for the 3-round-trip start; revisit if clock trust proves weak in the field.
4. **Receipt delivery on the uplink when the device is not listening.** The device initiates
   everything, so there is no server push; a bundle committed just before a cut-off is
   collected by `GET .../receipt` on the next visit.

## 10. Vectors

[`vectors/vectors.json`](vectors/vectors.json) pins four requests (offer, chunk, commit,
receipt) with their exact signing strings, body hashes, header values and **deterministic**
signatures, plus negative cases (tampered target, method, body, meta; wrong device; clock
skew; replay). Every key in it is a public test key derived from a published label and
protects nothing. Regenerate with `go run ./cmd/uplink-vectors` from `tools/`; the test in
`tools/uplinkvectors` fails if the file differs from what the generator produces.
