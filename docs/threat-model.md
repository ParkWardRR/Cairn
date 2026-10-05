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
- **Phone ↔ server:** HTTPS to the server's app listener (LAN) or `tailscale serve` (Tailnet), with the server's leaf pinned on the LAN path, and a **per-request P-256 signature** on every call. See [app-sync-protocol.md](app-sync-protocol.md).
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

## Recommendations

1. Use a dedicated VLAN or network segment for IoT devices including Cairn
2. Keep the local CA private key offline after initial device enrollment
3. Monitor server access logs for unexpected authentication attempts
4. Back up trip data to a separate system on a different failure domain
5. Consider full-disk encryption on the homelab server
6. Review enrolled app clients (`cairn-admin client list`) and revoke any you no longer use
