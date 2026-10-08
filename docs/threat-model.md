# Threat Model

> **Updated 2026-10-04 for v3.** The scope below ("personal, LAN-only") is
> superseded: Cairn now has an enrolled iOS client reachable over a Tailnet, an
> encrypted SD format, and more than one vehicle. The authoritative model,
> actor/path table and threat-to-outcome matrix are in
> [trust-model-v3.md](trust-model-v3.md); this document keeps the per-threat
> rationale and is updated where v3 changes the answer (marked **v3**).


## Scope

Cairn is a personal system, LAN-first, with private remote access over a Tailnet (**v3**). The threat model focuses on data integrity, device authenticity, and preventing unintended data exposure — not defending against nation-state adversaries or operating in hostile network environments.

## Assets

| Asset | Sensitivity | Impact of Compromise |
|-------|------------|---------------------|
| Trip location data | High | Reveals daily routines, home/work locations, habits |
| Device private key | High | Allows impersonation, forged trip uploads |
| ~~Home Wi-Fi credentials~~ | n/a | **Removed 2026-10-05.** The dongle has no Wi-Fi and holds no network credentials |
| Storage root key (`K_root`) | High | **v3.** Decrypts every bundle a device ever recorded; escrowed on the server in the keystore |
| App client identity | High | **v3.** A P-256 key in the phone's Secure Enclave; lets the holder read and write the vehicle's history |
| Keystore master key | High | **v3.** Unwraps every escrowed root; must be kept apart from the backed-up data directory |
| Server CA / certificates | Medium | The phone pins the server's TLS leaf; compromise allows MITM of phone–server traffic. (The legacy device listener's CA is retired with it.) |
| Raw trip bundles | Medium | Historical driving record |
| Derived/aggregated data | Low-Medium | Summaries, places, statistics |

## Trust Boundaries

```
┌─────────────────────────────┐
│ Device (trusted)            │
│  - Ed25519 private key      │
│  - Raw trip data (encrypted)│
│  - Pinned receipt key       │
│  - No network credentials   │
└──────────┬──────────────────┘
           │ BLE, bonded (ciphertext only)
           │ to the enrolled phone, which
           │ relays over signed requests
┌──────────┴──────────────────┐
│ Server (trusted)            │
│  - Server private key       │
│  - Device public keys       │
│  - All trip data            │
│  - Database credentials     │
└─────────────────────────────┘
```

### What is NOT trusted

| Boundary | Why |
|----------|-----|
| Public internet | Never contacted by design |
| Any Wi-Fi network | The dongle never joins one: it has no Wi-Fi |
| The phone, for integrity | It carries ciphertext and cannot forge a receipt; it is trusted for availability only |
| OBD-II bus | Not read; no diagnostic polling in v1 |
| Server-side plugins | None exist: the MoonBit WASM plugin system was retired 2026-10-05 |

## Threats and Mitigations

### T1: Device physical theft

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker obtains physical access to the Freematics device |
| **Data at risk** | Trip bundles on microSD, the storage root and signing seed in NVS (no network credential: there is none) |
| **Mitigation** | **v3:** every frame on the card is AEAD-encrypted (XChaCha20-Poly1305) under keys derived from a per-device root held in NVS; the card holds no key. Flash encryption + NVS encryption (Phase 24) protect the root and signing seed in the chip. The mTLS client key and Wi-Fi credentials that used to be there are gone: the dongle has neither. A stolen *running* device is handled by revocation, not cryptography |
| **Residual risk** | Determined attacker with ESP32 expertise may extract keys from flash |

### T2: Hostile or compromised phone, or a hostile relay

| Aspect | Detail |
|--------|--------|
| **Threat** | The phone (or something between it and the server) is the dongle's only route to the server, so it can drop, delay, reorder or alter what it carries, or try to make the dongle delete data |
| **Mitigation** | The phone carries **ciphertext only** and never sees a key. Chunks are checked against SHA-256 digests in a device-signed manifest; the manifest is verified against the enrolled device key. The dongle deletes only on a server-signed **receipt** verified against a key pinned in firmware **and** naming that bundle's content root, so a phone cannot forge one, replay one for another bundle, or induce a deletion. The BLE bond gates who may pull; an unbonded phone cannot read or write. A revoked phone's relay is refused |
| **Residual risk** | Availability: a hostile phone can refuse to upload, so trips stay on the card until another phone offloads (data is never lost). It can read the manifest's metadata (sizes, counts, times) over a valid bond. A static BLE passkey is the weakest link in that bond; an enrolled-app challenge–response (Phase 22) is planned |

### T2b: Evil-twin Wi-Fi network

**Not applicable since 2026-10-05.** The dongle has no Wi-Fi, so there is no network for an impostor to imitate. The phone's own Wi-Fi is protected by TLS pinning to the server's leaf and signed requests.

### T3: Forged trip upload

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker sends fabricated trip data to the server |
| **Mitigation** | Bundles must be signed with the registered device Ed25519 key, and the relaying phone must be an enrolled, unrevoked client scoped to the bundle's vehicle. Server validates both before accepting. |
| **Residual risk** | None unless device key is compromised (see T1) |

### T4: Replay attack

| Aspect | Detail |
|--------|--------|
| **Threat** | Previously uploaded bundle is re-sent |
| **Mitigation** | Server tracks content hashes; duplicates return the original receipt. **v3:** each bundle carries a monotonic device counter held in NVS and signed into the manifest; the server binds counter to content, so the same counter with different content is quarantined as a forgery or a rolled-back device |
| **Residual risk** | None |

### T5: Man-in-the-middle

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker on the LAN, or near the car, intercepts the phone's traffic |
| **Mitigation** | Phone ↔ server: TLS with the server's leaf pinned at enrolment, plus per-request signatures (a captured request cannot be replayed: single-use nonces inside a ±120 s window). Dongle ↔ phone: BLE bonding with encryption, and the payload is ciphertext in any case |
| **Residual risk** | A MITM on BLE during pairing with a known static passkey; limited to metadata and availability (see T2) |

### T6: microSD data recovery after pruning

| Aspect | Detail |
|--------|--------|
| **Threat** | Deleted trip data recovered from microSD using forensic tools |
| **Mitigation** | Encrypt bundles at rest. Consider secure erase when pruning. Document that FAT32 secure erase is not guaranteed. |
| **Residual risk** | Flash storage wear leveling may retain data in unreachable sectors |

### T9: Stolen or cloned SD card (v3)

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker reads, copies, edits or restores the card |
| **Mitigation** | Ciphertext only; AEAD tags and the prev-CRC chain detect edits, deletion and reordering; AAD binds frames to their segment, vehicle and device; a card copied to another dongle cannot decrypt (different root); a restored old image is an idempotent duplicate, and a forged bundle under a spent counter is quarantined |
| **Residual risk** | Frame sizes, timing and the unencrypted segment header (device, vehicle, assignment, counter) are visible. Metadata is not hidden, content is |

### T10: Lost or stolen phone (v3)

| Aspect | Detail |
|--------|--------|
| **Threat** | Someone with the phone reaches the server API |
| **Mitigation** | The app key is in the Secure Enclave and requests are signed per call; an admin revokes the client and it fails on the next request; Tailnet reachability alone is not a credential; bearer tokens live at most one hour |
| **Residual risk** | An unlocked phone in an attacker's hands can use its own key until revoked |

### T11: Tailnet exposure (v3)

| Aspect | Detail |
|--------|--------|
| **Threat** | Tailscale turned into an internet-facing service (Funnel), or a Tailnet peer pivots to Cairn |
| **Mitigation** | Funnel never used and its marker header is rejected; the app listener is loopback-only behind Serve; Serve identity headers are trusted only from loopback; ACL limits the owner's phone to the one port; the dongle, database, MQTT and tsdb are never on the Tailnet; app signatures are required regardless |
| **Residual risk** | A compromised Tailnet identity *and* an enrolled app key together |

### T12: Dongle moved between cars (v3)

| Aspect | Detail |
|--------|--------|
| **Threat** | Trips silently deposited under the wrong vehicle |
| **Mitigation** | Vehicle and assignment are carried in every segment header and manifest (signed); reassignment is an explicit admin action; bundles under an unrecognised, mismatched or superseded assignment are rejected; per-vehicle key salting means a mix-up cannot decrypt across cars |
| **Residual risk** | A wrong assignment made deliberately by the administrator |

### T13: Server disk or backup theft (v3)

| Aspect | Detail |
|--------|--------|
| **Threat** | Image of the server's disk, or a copied backup |
| **Mitigation** | CAS holds ciphertext; roots are wrapped under a master key kept outside the data directory; full-disk encryption on the host; destroying a root crypto-shreds all copies of that device's history |
| **Residual risk** | Master key stored alongside the backup defeats this; the deployment guide forbids it |

### T7: Plugin escape (retired)

The MoonBit/WASM plugin system this threat applied to was removed on 2026-10-05,
so there is no plugin execution surface. The number is kept so T8 and later
references stay stable. If server-side plugins are ever reintroduced, the original
design was: WASM runtime with no WASI capabilities, no filesystem, network or
database access, host-validated outputs, and no ability to alter raw data.

### T8: Server compromise

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker gains access to homelab server |
| **Mitigation** | LAN-only deployment, no port forwarding. Standard server hardening. Database authentication. Immutable raw bundles (append-only object store). Regular backups to separate location. |
| **Residual risk** | Full data access if server is compromised |

## Authentication Architecture

### Device Identity

- Each device generates a unique Ed25519 keypair during provisioning
- Private key stored in ESP32 NVS; **v3:** NVS encryption and flash encryption (Phase 24) so it is protected in the chip
- Public key registered with server during enrollment
- All trip bundles signed with device private key

### Transport Security

- **Dongle ↔ phone:** BLE with bonding and encryption. Bulk data is already AEAD ciphertext.
- **Phone ↔ server:** HTTPS to the server's app listener (LAN) or `tailscale serve` (Tailnet), with the server's leaf pinned on the LAN path, and a **per-request P-256 signature** on every call. See [app-sync-protocol.md](../contracts/sync/v1/spec.md).
- **The dongle has no network credential and no network stack.** Nothing to rotate, nothing to extract.
- **Legacy:** the `:8443` mTLS device listener and its private CA remain deployed only until the relay is proven on hardware; they are then removed (Cairn #7).

## Privacy Controls

| Control | Implementation |
|---------|---------------|
| Privacy zones | Configurable areas where start/end points are redacted |
| Trip deletion | User can delete any trip from server (and request device prune) |
| Data export | Full export of all raw and derived data at any time |
| No telemetry | System sends no data outside the home LAN |
| No cloud | No third-party accounts, APIs, or data sharing |
| Retention policy | Configurable per-device and server-wide retention windows |

## Sharing: design constraints before any design

Nothing in Cairn shares a trip today. The only things that leave the owner's hands are the
whole-store snapshot and the saved-places export, both deliberate and both the owner's own.
Sharing a **selected trip with someone else** (roadmap theme 3, with the portable
[`share/v1`](../contracts/share/v1/README.md) format) changes the threat model, because data
that was safe on the owner's own server is handed to people and places the owner no longer
controls. This section is written first, so the design has to meet it instead of explaining
it away afterwards.

**Scope.** A *share* is a derived export of chosen trips (a file, or a rendered image), made
by the owner and sent by the owner. It is **not** a bundle: it carries no proof that the
device captured it, and the format must not suggest otherwise. There is no hosted sharing
service and no cloud; if one is ever proposed it needs its own review here.

| # | Threat | Why it matters | Constraint on the design |
|---|---|---|---|
| S1 | **Home, work and routine disclosed** by where a trip starts and ends | The start and end of most trips are the owner's home and the places they visit; one shared route can name the address | Privacy zones are a first-class idea: start, end and chosen places are blanked or trimmed **by default**, and the owner sees what was removed |
| S2 | **Re-identification from the route itself**, even with zones | A rare road, a long loop or a precise timestamp can identify a person without any id | Trim more than the zone, offer coarse or shifted timestamps, and say plainly that a route is hard to anonymise |
| S3 | **Identifier linkage** across shares | A stable boot id, vehicle id or device id lets two shared trips be tied to one car and one owner | A share carries **per-share pseudonymous ids** and no stable owner, vehicle or device identifier |
| S4 | **Metadata that was not meant to leave** | Device serials, firmware versions, key fingerprints, host names, file paths, image metadata | The format is an **allow-list**: only named fields are exported, so a new internal field cannot leak by default |
| S5 | **Telemetry that becomes evidence or reveals the tune** | Speed against a limit can be used against the owner; engine data shows how the car is tuned | Speed and OBD detail are **excluded by default** and included only by an explicit choice, with a warning |
| S6 | **No recall** | A file cannot be taken back once sent | Do not promise revocation. The preview shows exactly what the recipient will receive, and sharing is opt-in per trip |
| S7 | **Forged or altered shares** | A recipient cannot verify a share, and a forged one can be passed off as the owner's | A share includes an integrity digest, is labelled as unverified derived data, and never reuses the bundle receipt or signature formats |
| S8 | **Rendered images leak through their tiles** | Fetching map tiles tells a provider where the trip was; an image can embed metadata | Render with locally held map data where possible, apply the same redaction before rendering, and strip metadata |
| S9 | **Other people in the data** | Passengers, and places that belong to others (a friend's address saved as a place) | Saved place names and categories are redacted by default; the owner chooses what a share names |
| S10 | **A token-issuing service, if one is ever built** | Guessable tokens, over-wide scope, no expiry | It must reuse the existing client-authentication model, issue narrow tokens that expire, and be reviewed in this document first |

**Required before `share/v1` leaves draft:** every row above has a concrete mechanism in the
format or the app; a preview that shows the recipient's view; default-private behaviour (a
fresh share exports the least it can); and an independent review of the draft against this
table.

## Planned: the networked dongle (Wi-Fi and LTE)

> **Planned, not shipped.** The owner decided on 2026-10-05 that the dongle gets Wi-Fi and LTE
> back, alongside BLE ([issue 19](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/19)).
> Everything above this section is true of the **shipped** firmware, which has no network stack and
> holds no network credential; it stays as written until the firmware that changes it ships. This
> section is the model that firmware must meet **before** it ships, in the same way the sharing
> section below was written before any sharing design. Status of every mitigation here is
> **planned**; none is implemented, and those the issue did not already name (for example joining only networks the dongle was given, a PIN-locked capped SIM, an LTE-only modem) are the author's proposals for the review to accept or change. Protocol: [`uplink/v1`](../contracts/uplink/v1/spec.md) (draft).
> Tracking: [issue 23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23).

### Rule zero: encrypt first

**No network credential is provisioned onto a unit until flash encryption and NVS encryption are
enabled on that unit** ([firmware issue 18](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/18)).
This is a **gate**, not an accepted risk, for the Wi-Fi password, the LTE APN and SIM PIN, and
the pinned server keys. A unit without it keeps working over BLE only. (Rule confirmed by the
owner on 2026-10-05.) What an attacker gets:

| Attacker | Before flash and NVS encryption | After |
|---|---|---|
| Dumps the chip's flash | Everything in NVS in clear: the storage root, the device signing seed, and (if provisioned) the Wi-Fi password and SIM PIN. Cairn therefore forbids provisioning credentials in this state | Ciphertext only |
| Steals the unit, powered off | As above | Flash and NVS encryption: nothing readable. Unsigned firmware is still *not* refused on the car's unit (ESP32-D0WDQ6 revision v1.0 cannot use secure boot V2; V1 is weaker, irreversible and not planned). A unit of revision v3.0 or later would add secure boot V2 |
| Steals the unit, running or able to be powered on | The device key and, once provisioned, the credentials | The device key and the pinned server keys work **as that device** until revoked; it does **not** yield a list of the owner's home networks in the clear, because the dongle stores only what it was given for networks it must join and nothing else |
| Reads the SD card | Ciphertext only (unchanged, format v3) | Unchanged |

A stolen running unit is handled by revocation (one admin action), not cryptography. A SIM in a
stolen unit is separately handled by the carrier (N3).

### New threats and their disposition

Each row has a mitigation or an explicit accepted-risk entry. "Gate" means a precondition for
shipping, not a nice-to-have.

| # | Threat | Mitigation (planned) | Residual risk / accepted |
|---|---|---|---|
| N1 | **Credentials on the chip**: Wi-Fi password, LTE APN and SIM PIN, server pins; NVS is plaintext today | Rule zero (gate). Secrets are write-only on the dongle (nothing reads them back over any interface), never logged, scrubbed from RAM after use. Pins are public keys, not secrets, but an attacker who can change them can redirect the unit, so they live in the encrypted, signed-firmware region | A chip dump of a unit locked down late (an early build in the field) reveals what was provisioned before. Accepted for those units only if they were never given a credential; otherwise re-provision after enabling encryption |
| N2 | **Evil-twin Wi-Fi** (T2b, applicable again) | Server key **pinned** in firmware and every request signed (uplink §2, §3), so an impostor sees TLS handshake metadata and traffic volume but cannot impersonate the server, read data (sealed), or forge a receipt (pinned receipt key). The dongle never falls back to unpinned TLS or plain HTTP | Metadata: when the unit is awake and how much it sends. It will also hand a captive network its Wi-Fi password **if the network is allowed to ask for one**, so the dongle joins only networks it was explicitly given (SSID and security type, WPA2/3 only; no open networks, no portal flows) |
| N3 | **SIM theft** (a stolen dongle's SIM is a data plan) | The SIM is locked with a PIN held only in the encrypted store; the data cap is enforced on the dongle (N13) and on the carrier's plan, which should be a capped, data-only SIM with no voice or SMS | A thief who defeats the PIN with a SIM swap into another device spends only the plan's cap. Accepted |
| N4 | **Rogue base station, downgrade, IMSI and SMS exposure** | The application does not depend on the carrier: TLS pinned end to end, sealed data. The modem is configured for LTE-only (no 2G fallback) where the module allows, and SMS is not used for any control path | The carrier and a rogue cell see that a SIM is attached and the volume it moves, and a rogue cell can force a denial of service (jam or refuse attach). Both accepted; BLE remains the fallback |
| N5 | **A device-facing endpoint reachable from the internet** | Four routes only, on a dedicated hostname or port, nothing of the app API or dashboard reachable on it; authentication checked before any body is read; uniform `401` for every pre-signature failure (no enumeration); per-address and per-device rate limits; bounded bodies; per-request nonce and ±120 s window (uplink §7, §8). The exposure option is **proposed, not decided** (an owner decision) | Volumetric denial of service is not solved by protocol (an upstream firewall or the host's limits do that). The cost of an unauthenticated request is one hex parse and one signature verification, and **must be measured** under load before this endpoint is opened |
| N6 | **The same bundle arriving by BLE and LTE at once** | Offer and chunks are idempotent and content-addressed; the server persists one receipt per bundle and returns it verbatim to every path, so both receipts are the same bytes and the dongle prunes once (a second receipt for an already-pruned bundle is ignored) | None expected; covered by a drill when the firmware exists |
| N7 | **Phone-to-dongle instructions** (the check-in) are a new remote-control surface: a malicious or stolen enrolled phone, replay, flooding, an unauthenticated nearby central feeding instructions | Allow-listed message types only (nothing that deletes data, changes credentials, or raises the data cap), signed by an enrolled client key, a per-session counter against replay, a rate limit, and an audit trail on the dongle and in the app. Anything beyond the allow-list needs the sealed configuration path (N10) | A stolen enrolled phone can issue allow-listed instructions until revoked; the allow-list is chosen so that is an annoyance (a check-in, a slot request), never a loss |
| N8 | **Time-sliced radio**: the dongle is deaf to BLE during a Wi-Fi slot | A slot is bounded (a hard maximum duration, set so the longest deaf window is short next to a trip and cannot be extended by the network); nothing safety-relevant depends on BLE being up; a trip starting ends the slot ([firmware issue 17](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/17)) | During a slot an attacker can cause no more harm than during any other period, and cannot reach BLE-only functions; they can at worst time an attack to the window, which the bound limits. Accepted |
| N9 | **"Home" detection** leaks where the owner lives if it stores coordinates or a network list | Prefer the **phone-asserted** option: the app tells the dongle "you are home" (an allow-listed instruction, N7), so the dongle stores no coordinates and no list of places; a scanned option stores only the one Wi-Fi network it was given and leaks nothing beyond N1; a **geofence** on the dongle stores coordinates and is the worst on exactly this property, so it is the last choice | The Wi-Fi network the dongle was provisioned with is itself a hint about where it lives, protected by rule zero |
| N10 | **Configuration passes through a web UI and a server** on its way to the dongle | The web layer is modelled as a **credential-handling system**: authenticated sessions only, write-only fields (a stored secret is never shown again), no secret in a log, a URL, the browser's storage or an error message, TLS only. The server seals the configuration **to the device's key** so neither the server's database nor an intermediary holds usable plaintext at rest, and the dongle verifies it before applying anything ([issue 27](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/27)) | The web UI sees a secret while the owner types it. A compromised server or browser can read what is typed; that is the cost of configuring from a browser, and the reason the sealed path and the allow-list exist. Accepted, with the owner informed |
| N11 | **The LTE digest** could be mistaken for proof of upload | A digest is **provisional and is never a prune receipt**: it is a different message type with a different signing context, and the dongle's prune logic accepts only a v3 receipt verified against the pinned key, naming the bundle's content root ([issue 26](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/26)) | None, provided the negative vectors (a digest presented as a receipt) are enforced in the firmware |
| N12 | **Live location ticks**, if ever enabled, are a tracking surface | Off by default and not part of the first networked release. If added: opt-in per vehicle, a coarse position, its own enrolled-client scope, a retention limit, and an off switch that does not need the network | If enabled, the server and anyone with that scope can see where the car is, by design |
| N13 | **The LTE data cap is also a safety control** | The cap is enforced **on the dongle**, below a hard ceiling compiled into the firmware that **cannot be raised remotely without an authorised, sealed message** (N10); a bug or a stolen unit cannot spend past it | Within the ceiling the plan can be spent by a compromised device before revocation. Accepted: that is what the ceiling is for |
| N14 | **Physical access to the USB port** | Still the trust boundary until secure boot ships, which on the car's unit (revision v1.0, no secure boot V2) means until it is replaced by a revision v3.0 or later unit: whoever holds the unit can re-provision an unassigned or freshly rebooted device. The existing 60 s post-boot window and the "never during a trip" rule remain | Closed only by secure boot (V2, a revision v3.0+ unit) and the lock-down in `esp32-hardening.md`; flash encryption alone does not stop re-flashing over USB while development mode is on |

### What changes in the matrix

- T2b returns for the networked dongle (N2).
- T3 (forged upload): a bundle may now arrive directly from the device as well as via a phone;
  the manifest signature and assignment checks are the same, and the request signature adds
  *who is calling* (uplink §2).
- T4 (replay): adds the per-request nonce and window (N5).
- T5 (man in the middle): adds dongle ↔ server over the internet, covered by the pinned key (N2, N4).
- T1 (physical theft): adds credentials to what a chip dump could reveal; see rule zero.

### Review

**Not yet done.** This section is a first draft of the model by the author of the protocol, which
is the wrong person to approve it. The issue's done-condition is a review that has passed, recorded
here with its date and reviewer; until then the networked dongle must not ship. The most useful
things for a reviewer to attack: the cost of an unauthenticated request (N5), the late-lock-down
units (N1), the "allow-list is harmless" claim for phone instructions (N7), and the interaction of
the data-cap ceiling with revocation latency (N13).

### Residual exposure from cellular activity

Application-encrypted bundles protect telemetry content, but they do not make
cellular activity invisible. The following observers see metadata regardless of
payload encryption:

| Observer / component | Residual exposure |
|---|---|
| Cellular carrier | SIM/subscriber association, cellular location, connection timing, traffic volume |
| Local Wi-Fi infrastructure | Association and traffic metadata |
| BLE observers | Advertising presence and potentially identifying advertisement fields |
| Public HTTPS infrastructure | Endpoint identity and connection metadata |
| Relay phone / gateway | Bundle sizes and timing; should not need plaintext telemetry |
| Homelab | Plaintext after authorised ingestion, plus logs/backups the owner controls |

Leakage-minimising defaults for the networked dongle: generic BLE
advertisements with no VIN or route data, no discovery broadcasts containing
telemetry, no random Wi-Fi association, no analytics or third-party telemetry,
encrypted local bundles, and capped burst uploads. Frequent tiny transmissions
reduce latency but expose more activity timing and add overhead; for a capped
cellular plan, encrypted batching without cover traffic is the right tradeoff.
The full cellular design is in [lte-cellular-design.md](lte-cellular-design.md).

## Recommendations

1. Use a dedicated VLAN or network segment for IoT devices including Cairn
2. Keep the local CA private key offline after initial device enrollment
3. Monitor server access logs for unexpected authentication attempts
4. Back up trip data to a separate system on a different failure domain
5. Consider full-disk encryption on the homelab server
6. Review enrolled app clients (`cairn-admin client list`) and revoke any you no longer use
