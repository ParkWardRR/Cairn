# Cairn v3 — Trust Model, Transport and Vehicle Scope

> **Amendment 2026-10-05 — the dongle has no Wi-Fi.** All server sync goes through the
> enrolled iOS app: the phone pulls sealed bundles over BLE and uploads them on the
> dongle's behalf ([ble-offload.md](ble-offload.md), [app-sync-protocol.md](app-sync-protocol.md) §13).
> Wherever this document says the dongle talks to the server over mTLS, read: the
> **phone** talks to the server over signed requests, and the dongle's integrity
> evidence is unchanged (Ed25519 manifest, server-signed receipt verified against a
> pinned key). The dongle holds **no network credential of any kind**: no SSID, no
> password, no client certificate, no transport private key. Details in §0.1.

**Status:** normative design for Phases 19–24 in [ROADMAP.md](../ROADMAP.md).
**Date:** 2026-10-04. **Breaking:** yes, deliberately. v2 bundles, v2 manifests
and the single-device-equals-single-car assumption are not carried forward; old
recordings are disposable.

This replaces the "LAN only, one dongle, one car" model with one coherent trust
model covering three things that were previously designed separately:

1. **Encrypted device storage** — a stolen SD card yields ciphertext.
2. **Authenticated multi-transport sync** — the iOS app is a separately
   enrolled client of the server, reachable on the LAN or over Tailscale.
3. **A vehicle-scoped data model** — a second car (a 2017 M240i, B58) joins
   without sharing a dongle's identity, keys or history with the first.

> **The rule that organises everything:** *the SD card is never the security
> boundary.* It is a removable cache of ciphertext. Trust roots live in
> device-held keys, in the server's keystore and in revocable identities; every
> uploaded object is untrusted until its signature, assignment, counter and
> integrity chain verify.

---


## 0.1 What the Wi-Fi removal changes (2026-10-05)

| | Before | Now |
|---|---|---|
| Who talks to the server | The dongle, over mTLS on home Wi-Fi | The **phone**, with signed requests, on LAN or Tailnet |
| Who authorises deletion on the dongle | A server-signed receipt, verified against a pinned key | **Unchanged.** The receipt is now handed over by the phone instead of received directly |
| Who can read a trip in transit | Anyone who broke TLS | Nobody on the path: the phone carries **ciphertext only** (frames are AEAD-encrypted under keys it never holds) |
| What a stolen or hostile phone can do | n/a | Fail to upload. It cannot read a trip, forge a receipt, or make the dongle delete anything |
| Secrets on the chip beyond the storage root | Wi-Fi password, client key | **None** |
| What the BLE bond protects | Live phone GNSS and status | Also the **metadata** of what is stored (counts, sizes, times in the manifest). Not the data |
| Cost | n/a | Trips reach the server only when the phone offloads them; the dongle must be awake and in range. See [ble-offload.md](ble-offload.md) §5 |

## 1. Actors, paths and what authenticates each

| Path | Transport | Authentication | Role | Exposure |
|---|---|---|---|---|
| ~~Dongle → server (home Wi-Fi)~~ | **Removed 2026-10-05.** The dongle has no network stack | n/a | n/a | n/a |
| iPhone → dongle | BLE only | BLE bonding + passkey (the access control); a receipt, not the link, is what authorises a deletion | Live GPS assist, diagnostics, **bundle offload** ([ble-offload.md](ble-offload.md)) | Physical proximity |
| iPhone → server, carrying the dongle's bundles | Signed requests / bearer tokens on `/v1/relay/*` | The phone's enrolled P-256 key; the bundle's own device signature is verified by the server | Upload of encrypted sealed bundles on the dongle's behalf | LAN or Tailnet |
| iPhone → server, LAN | HTTPS, `:8444` | **Signed requests** from an enrolled app identity (P-256) | Browse trips, sync metadata, manage vehicles | LAN |
| iPhone → server, remote | **Tailscale** → `tailscale serve` → loopback HTTP | The same signed requests, **plus** Tailnet reachability | Same API as LAN | Tailnet only |
| Server host → Tailnet | `tailscaled` on the host | Tagged node, ACL policy | Private reachability | Tailnet only |

**Tailscale is reachability, not authorization.** Two consequences drive the
design:

- The app is *always* separately enrolled and authenticated. A device on the
  Tailnet is not thereby a Cairn client.
- `tailscale serve` terminates TLS on the host, so **a client certificate cannot
  reach Cairn on that path**. App authentication therefore cannot be mTLS. It is
  a per-request signature that is identical over the LAN listener and the
  tailnet-proxied one. (The dongle no longer speaks to the server at all; the phone does.)

Funnel is never used. A car journal is location history; Funnel turns a
private-tailnet design into an internet-facing service, and the app listener
rejects any request carrying the Funnel header.

### 1.1 Listeners

| Listener | Default | TLS | Client auth | Audience |
|---|---|---|---|---|
| Device ingest (**legacy; to be retired**, see Cairn #7) | `:8443` | server cert | mTLS required at handshake | none: no dongle uses it any more |
| App API (LAN) | `:8444` | server cert | none at TLS; signed requests | iOS app |
| App API (tailnet) | `127.0.0.1:8445` | plain HTTP, **loopback only** | signed requests; Tailscale identity headers honoured only from loopback | iOS app via `tailscale serve` |
| `cairn-tsdb` | `127.0.0.1:8480` | — | loopback | UI server |
| MQTT | LAN or internal only | mTLS | per-device | embedded devices |
| PostgreSQL / admin | internal network only | — | — | never on the tailnet |

The app API and device ingest are separate listeners because the device listener
demands a client certificate during the handshake: an app that has none could
not even reach a handler that might have authenticated it another way.

---

## 2. Vehicle model

A vehicle is explicit and carried *by the data*, never inferred from the dongle
or a VIN at upload time.

```text
vehicles            id, owner_id, display_name, year, make, model, engine_code,
                    vin_ciphertext, vin_last4, created_at, archived_at
devices             id, public_key, key_id, status, storage_key_version, ...
device_assignments  id, device_id, vehicle_id, seq, starts_at, ends_at,
                    first_counter, last_counter, assigned_by
```

- `engine_code` ("B58") is a label, not an identity. Two cars can share one.
  Display identity is `2017 BMW M240i — B58`.
- The full VIN is sealed at rest (AES-256-GCM, registry key) and never logged;
  only `vin_last4` is displayable.
- One recorder per vehicle at a time. Reassigning a device ends its previous
  assignment; assigning a vehicle another device holds is refused until that
  device is unassigned.
- **Reassignment is an explicit administrative act.** The new assignment ID is
  pushed to the dongle over a trusted connection; every segment and manifest it
  writes thereafter carries the new `vehicle_id` and `assignment_id`.

### 2.1 Assignment validity is order, not time

Intake must reject a bundle that claims an assignment the server does not
recognise, or that belongs to another device or vehicle. It must **not** test
"was the assignment active at capture time": capture time is a GNSS-derived
estimate with its own uncertainty and can be absent (invariant 3), and a device
legitimately uploads a bundle captured under the *old* assignment after the
administrator has moved it, because it has not yet heard.

The test is order, in the device's own counter: a bundle is **superseded** when
the server has already accepted a bundle under a *newer* assignment of the same
device at a *lower* counter. That is exactly "the device used the old identity
after demonstrably switching to the new one" — the only sequence that signals a
mistake or an attack — and it needs no trusted clock. (`internal/vehicles`,
`CheckBundle`.)

### 2.2 The monotonic bundle counter

Every sealed bundle carries `device_counter`, incremented by the device in
**non-volatile storage on the chip, not on the card**, and signed into the
manifest. The server (`internal/counters`) binds each counter to its
`content_root`:

| Presented | Meaning | Server action |
|---|---|---|
| New counter | Normal | Accept, record |
| Same counter, same content | Re-upload, restored card image | Answer with the original receipt (idempotent) |
| **Same counter, different content** | Forgery, cloned device, reflashed unit whose counter went backwards | **Reject and quarantine**; ledger entry with reason |
| Counter jumps ahead | Backlog or lost bundle | Accept; report the hole (`Missing`) |

Counters are per device and survive re-keying: re-enrolment returns the
high-water mark and the device resumes above it, so a rotation cannot reopen
spent values.

---

## 3. Storage encryption (bundle format v3)

Normative layout: [bundle-format-v3.md](bundle-format-v3.md). Summary of the
decisions and why:

| Decision | Reason |
|---|---|
| **Every frame is encrypted**, frame envelope and CRC chain unchanged | Torn-tail detection, structural scan, chain verification, Merkle `content_root` and chunking all work **without any key**. The server can verify integrity before it ever decrypts |
| XChaCha20-Poly1305 | 192-bit nonce makes random nonces safe; software-friendly on an MCU with no SD-encryption hardware |
| **Random 24-byte nonce per frame, never derived from `seq`** | After a torn-tail truncation the same `seq` is legitimately rewritten with *different* plaintext. A seq-derived nonce would reuse (key, nonce) over two plaintexts — a catastrophic stream-cipher failure |
| AAD = the 24-byte frame header + the segment header (all but its CRC) | Binds device, vehicle, assignment, boot, segment index, key version and counter. A frame cannot be moved between segments, cars or devices, or reordered |
| Per-segment key by HKDF-SHA256 from the device root, salted with `vehicle_id` | A key or metadata mistake cannot make ciphertext from one car decrypt under another; two cars sharing a dongle are cryptographically separated |
| Format `CRN3`, 128-byte segment header carrying `vehicle_id`, `assignment_id`, `storage_key_version`, `device_counter` | Everything intake needs to authorise a bundle is readable without a key |
| Manifest v3 adds `vehicle_id`, `assignment_id`, `device_counter`, `storage_key_version`, `encryption_suite` | Signed, so the claim cannot be altered after sealing |

```text
K_root  (32 random bytes, generated on the device with the hardware RNG,
         held in flash-encrypted NVS, escrowed to the server at enrolment)
  └─ K_seg = HKDF-SHA256(ikm = K_root, salt = vehicle_id,
                         info = "cairn/segment/v3" ‖ device_id ‖ assignment_id
                                ‖ boot_id ‖ segment_index_u32le)
       └─ frame = nonce[24] ‖ XChaCha20-Poly1305(K_seg, nonce,
                                                 plaintext, aad = header ‖ seg_hdr)
```

### 3.1 Where the server stands

The server **can** decrypt — it must, to decode — because it holds the escrowed
root. What the design delivers is that **the card alone cannot**, and that the
server's *at-rest bundles are ciphertext too*: the CAS stores exactly what the
card held. Decoding derives keys from the keystore (`internal/keystore`); the
roots are wrapped on disk under a server master key bound to device id and key
version, and **destroying a root crypto-shreds every copy of that device's
history** — CAS, Parquet mirror and old backups alike, which is the only
deletion that reaches backups.

### 3.2 Correction to a common assumption: eFuse is not a key you can derive from

The classic ESP32's flash-encryption key lives in a read-protected eFuse block
that **only the flash-encryption hardware can use**. Software — including HKDF —
cannot read it, so "derive the SD root from the eFuse key" is not possible on
this chip. (The HMAC and Digital Signature peripherals that make that pattern
work exist on the ESP32-S2/S3/C3, not here; the target hardware is a classic
ESP32 WROVER, per ROADMAP "Decisions".)

What actually holds on this chip, and what this design relies on:

- `K_root` is generated on the device and stored in **NVS**. Before flash
  encryption is enabled it is plaintext in on-chip flash — protected only
  against someone who has the *card*, not someone who dumps the *chip*.
- With **flash encryption (release mode) + NVS encryption** enabled, the chip's
  flash — including NVS and the `nvs_keys` partition — is AES-256 encrypted
  under an eFuse key nothing can read back. `K_root`, the Ed25519 signing seed,
  nothing network-related is on the chip to protect: there is no client key or Wi-Fi credential any more.
- A copied card therefore cannot decrypt in another dongle (different `K_root`),
  and a stolen *unlocked, running* dongle is handled by **revocation**, not
  cryptography (§6).

### 3.3 Secrets that must leave the SD card

Current state, checked against the code on 2026-10-04:

| Secret | Today | Required |
|---|---|---|
| Trip data | Plaintext frames on the card | Encrypted frames (format v3) |
| ~~mTLS client certificate key~~ | **Removed.** Was on the card, then in NVS | The dongle has no network client, so there is no key. Earlier firmware's copy is erased from NVS at boot |
| Ed25519 manifest signing seed | NVS (plaintext until flash encryption) | NVS, flash-encrypted |
| ~~Wi-Fi SSID / password~~ | **Removed.** The dongle has no Wi-Fi | Nothing to protect; erased from NVS at boot if earlier firmware left it |
| Pinned CA, receipt key | Compiled into firmware (trust anchors; correct) | Unchanged |

---

## 4. Enrolment

Two enrolments, both human-approved. No unauthenticated endpoint creates trust.

### 4.1 Device

1. The dongle generates `K_root` (hardware RNG) and its Ed25519 signing key on
   first boot. Neither leaves in the clear.
2. It emits an **enrolment blob**: its public key, `device_id`, firmware version,
   and `K_root` *sealed to the server's enrolment public key* (X25519 ephemeral
   key + HKDF + XChaCha20-Poly1305), signed with its own key as proof of
   possession.
3. The operator presents the blob to `cairn-admin device enroll` on a trusted
   workstation (the device can no longer POST it anywhere; there is no network). The
   server unseals, checks the proof of possession, shows the device fingerprint
   for the operator to confirm against what the dongle displays, then records the
   device, escrows `K_root` (version 1) and returns the counter floor.
4. The operator assigns the device to a vehicle (`cairn-admin assign`).

### 4.2 App

1. Admin creates a **one-time invitation** (128-bit code, stored only as SHA-256,
   10-minute TTL, single use) carrying a role and a vehicle scope.
2. The app generates a **P-256 key in the Secure Enclave** (P-256 is the only
   curve the Enclave holds) and calls `POST /v1/enroll/app` with the code and its
   public key, signing `code ‖ public_key` as proof of possession.
3. The server returns the client id and its own identity (instance id, TLS
   SPKI pin). The app pins it.

Every app request thereafter is signed (`docs/app-sync-protocol.md`): ±120 s
timestamp window, nonce cache against replay, body hash in the signed string.
Background `URLSession` tasks, which are created ahead of time and may run
later, use a short-lived (1 h) opaque bearer token minted by a signed request.

---

## 5. App sync

The app is a first-class client: local encrypted store, durable outbox,
idempotent protocol. Full wire protocol in
[app-sync-protocol.md](app-sync-protocol.md); the invariants:

| Requirement | Behaviour |
|---|---|
| Idempotency | Same `operation_id`/`idempotency_key` returns the **original** result, never a second record |
| Ordering | Server-assigned `server_sequence`; device wall-clock is never trusted for order |
| Conflicts | Immutable telemetry is append-only; mutable metadata uses per-field revisions and returns `conflict` with the current value |
| Authorization | Every operation is scoped to an enrolled client **and** an allowed `vehicle_id` |
| Revocation | Effective on the next request, no restart |
| Audit | Actor, transport class, Tailscale identity, request hash, result — **never** payloads, GPS, tokens, signatures, SSIDs or full VINs |
| Cursor | Opaque, durable, repeatable; carries an epoch so a server reset is detected |

**Endpoint selection.** The app holds two base URLs for *one* logical account:

```text
local:     https://cairn.example.lan:8444
tailscale: https://cairn-host.example-tailnet.ts.net
```

It tries the local URL first when on a trusted SSID and falls back to the
Tailnet when the local health probe fails. Same account, same server identity,
same cursor — never a separate "local database" and "Tailscale database".
Tailscale is a route preference, not a reliability guarantee: iOS may not have
the VPN up in the background, so uploads resume when the app next gets time.

---

## 6. Threats and outcomes

| Scenario | Outcome |
|---|---|
| Card inserted into a laptop | Opaque ciphertext. No trip data, no `K_root`, no signing key, and no network key (there is none) |
| Card contents copied to another dongle | Different `K_root`; cannot decrypt, cannot authenticate |
| Attacker edits queued data on the card | AEAD tag fails; CRC chain and Merkle root also break; server quarantines |
| Frames deleted, reordered or spliced | `prev_crc32` chain and AAD binding detect it |
| Old card image restored | Same (counter, content) → idempotent duplicate; a forged different bundle under a spent counter → **conflict, quarantined** |
| Dongle stolen while running | Revoke the device in Cairn (one admin action); further uploads are refused. Cryptography cannot help against a live, unlocked device — minimise the window |
| Dongle stolen powered off | With flash encryption + secure boot: firmware and NVS unreadable, unsigned firmware will not boot |
| Dongle moved between cars | Explicit reassignment; bundles under a superseded or unrecognised assignment are rejected |
| Phone lost | Revoke that app client; its signed requests and bearer tokens stop working immediately |
| Server disk imaged | CAS is ciphertext; keystore roots are wrapped under a master key kept apart from the data dir |
| Server backup stolen | Same, provided the master key is not in the backup |
| Tailnet peer is compromised | Still needs a Cairn app identity; a Tailnet address is not a credential |
| Funnel accidentally enabled | App listener rejects Funnel-marked requests |

What is **not** claimed: protection from a compromised *server* (it holds the
roots), from an attacker with the live unlocked dongle (revoke it), or from a
sufficiently equipped attacker who dumps an unencrypted ESP32 before flash
encryption is enabled (§7).

---

## 7. ESP32 hardening

Detailed procedure and irreversibility notes: [esp32-hardening.md](esp32-hardening.md).
In short:

| Control | Status | Notes |
|---|---|---|
| Application-layer AEAD on every SD frame | **Phase 21** | No SD encryption hardware exists; this is the only layer that can protect the card |
| Hardware RNG for every nonce and key | **Phase 21** | `esp_fill_random`; never timestamps or sequence numbers |
| Flash encryption (release mode) | **Phase 24 — gated** | Irreversible; do on a sacrificial unit first; needs the build migrated off precompiled Arduino libs |
| Secure boot | **Phase 24 — gated** | Irreversible; V1 vs V2 depends on chip revision (check first) |
| Signed OTA | Written (Phase 11), **install not yet exercised on hardware** | Must be proven, with rollback, before locking anything |
| JTAG / UART ROM download disabled | Phase 24, after the above | Part of release lock-down |
| Tailscale on the dongle | **No** | Not realistic on the ESP32. Tailscale runs on the host and the iPhone |

---

## 8. BLE

BLE bonding is a local transport and bootstrap channel, not the security
boundary for telemetry. The companion protocol gains: an enrolled-app
challenge-response at session start (so an unenrolled phone cannot inject GNSS
fixes even if it knows the passkey), a per-session counter on writes
(replay-safe), and the dongle's `device_id` + public-key fingerprint exposed for
the app to verify against the server's record. Specified in the iOS companion
issues; firmware work in Phase 22.

---

## 9. Deployment hardening

Keep the services as separate systemd units (`deploy/systemd`), with Caddy fronting the
UI only. Run Tailscale as a **host-level** unit — never embed Tailnet credentials in a
container or compose file.

| Layer | Hardening |
|---|---|
| Host | Full-disk encryption; unattended security updates; locked-down SSH |
| Tailscale | Host install; tag `tag:cairn-server`; ACL allows only the owner's phone to TCP 443/8444; **Funnel off**; no subnet router; no SSH; device approval on; one-time tagged auth key, never in an image or repo; MagicDNS name |
| Containers | If PostgreSQL or a broker runs in one: rootless Podman where practical; read-only rootfs; dropped capabilities; isolated networks |
| Database | No published port |
| MQTT | LAN or internal only; mTLS per device; ACL topics scoped by device and vehicle |
| Keys | Runtime secret files, mode 0600; **keystore master key kept apart from the backed-up data directory**; never in image layers, Git, logs or environment dumps |
| Backups | Encrypted, versioned, restore-tested, separate from live credentials |
| Logs | Never raw telemetry, GPS points, bearer tokens, certificates, SSIDs or full VINs |

See [tailscale-deployment.md](tailscale-deployment.md).

---

## 10. What changed from v2 and why it is safe to break

| v2 | v3 |
|---|---|
| Format `CRN2`, plaintext frames | Format `CRN3`, every frame AEAD-encrypted |
| Manifest v2: device only | Manifest v3: device + vehicle + assignment + counter + key version |
| One device ≙ one car | Vehicles and assignments are first-class |
| Device registry: id + key | + storage-key escrow, revocation, counter high-water |
| No app identity (an unauthenticated URL) | Enrolled, signed, revocable, audited app clients |
| LAN only | LAN + Tailnet, one logical account |
| ~~mTLS key on the card~~ | **Gone**: the dongle has no mTLS key |

There is no migration. v2 recordings were test data; the v1 rule ("never write a
reader for a dead format") now applies to v2 as well.
