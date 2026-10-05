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
| Home Wi-Fi credentials | Medium | Stored on device; enables network access if extracted |
| Storage root key (`K_root`) | High | **v3.** Decrypts every bundle a device ever recorded; escrowed on the server in the keystore |
| App client identity | High | **v3.** A P-256 key in the phone's Secure Enclave; lets the holder read and write the vehicle's history |
| Keystore master key | High | **v3.** Unwraps every escrowed root; must be kept apart from the backed-up data directory |
| Server CA / certificates | Medium | Compromise allows MITM of device-server communication |
| Raw trip bundles | Medium | Historical driving record |
| Derived/aggregated data | Low-Medium | Summaries, places, statistics |

## Trust Boundaries

```
┌─────────────────────────────┐
│ Device (trusted)            │
│  - Ed25519 private key      │
│  - Wi-Fi credentials        │
│  - Raw trip data            │
│  - Pinned server CA         │
└──────────┬──────────────────┘
           │ mTLS over home LAN
           │ (trusted network segment)
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
| Non-home Wi-Fi networks | Device only syncs on trusted BSSID/SSID |
| Identically-named SSIDs | BSSID validation prevents evil-twin attacks |
| OBD-II bus | Not read; no diagnostic polling in v1 |
| MoonBit plugins | Run in WASM sandbox; no network/disk/DB access |

## Threats and Mitigations

### T1: Device physical theft

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker obtains physical access to the Freematics device |
| **Data at risk** | Trip bundles on microSD, Wi-Fi credentials, device private key |
| **Mitigation** | **v3:** every frame on the card is AEAD-encrypted (XChaCha20-Poly1305) under keys derived from a per-device root held in NVS; the card holds no key. Flash encryption + NVS encryption (Phase 24) protect the root, signing seed, mTLS key and Wi-Fi credentials in the chip. The mTLS client key must move off the card (Phase 22) — today it is on it. A stolen *running* device is handled by revocation, not cryptography |
| **Residual risk** | Determined attacker with ESP32 expertise may extract keys from flash |

### T2: Evil-twin Wi-Fi network

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker creates Wi-Fi network with same SSID as home network |
| **Mitigation** | Validate BSSID (MAC address) in addition to SSID. Require mTLS — device will reject any server without the pinned CA. |
| **Residual risk** | MAC spoofing is possible but attacker still cannot complete mTLS handshake |

### T3: Forged trip upload

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker sends fabricated trip data to the server |
| **Mitigation** | mTLS requires valid device certificate. Bundles must be signed with registered device Ed25519 key. Server validates signature before accepting. |
| **Residual risk** | None unless device key is compromised (see T1) |

### T4: Replay attack

| Aspect | Detail |
|--------|--------|
| **Threat** | Previously uploaded bundle is re-sent |
| **Mitigation** | Server tracks content hashes; duplicates return the original receipt. **v3:** each bundle carries a monotonic device counter held in NVS and signed into the manifest; the server binds counter to content, so the same counter with different content is quarantined as a forgery or a rolled-back device |
| **Residual risk** | None |

### T5: Man-in-the-middle on LAN

| Aspect | Detail |
|--------|--------|
| **Threat** | Attacker on home network intercepts device-server communication |
| **Mitigation** | mTLS with pinned local CA. Device rejects any certificate not signed by pinned CA. Server rejects any client without valid device certificate. |
| **Residual risk** | Requires compromise of the local CA private key |

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

### T7: Plugin escape

| Aspect | Detail |
|--------|--------|
| **Threat** | Malicious or buggy MoonBit plugin escapes sandbox |
| **Mitigation** | WASM runtime with no WASI capabilities granted. No filesystem, network, or database access. Host validates all outputs. Plugins cannot alter raw data. |
| **Residual risk** | WASM runtime bugs (mitigated by keeping runtime updated) |

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

- HTTPS with mutual TLS (mTLS)
- Device pins a local CA certificate (not public Web PKI)
- Device pins the server hostname / public key
- Server validates client certificate against enrolled device list
- No shared device secrets — each device has unique credentials

### Certificate Strategy

```
Local CA (self-signed, kept offline after initial setup)
  ├── Server certificate (cairn.local)
  └── Device certificates (one per enrolled device)
```

- CA private key stored offline or in a hardware security module
- Server certificate includes mDNS name and static IP as SANs
- Device certificates include device ID as the common name
- Certificate rotation planned but not required for v1

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
6. Review and rotate device certificates on a reasonable schedule
