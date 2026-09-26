# Threat Model

## Scope

Cairn is a personal, LAN-only system. The threat model focuses on data integrity, device authenticity, and preventing unintended data exposure — not defending against nation-state adversaries or operating in hostile network environments.

## Assets

| Asset | Sensitivity | Impact of Compromise |
|-------|------------|---------------------|
| Trip location data | High | Reveals daily routines, home/work locations, habits |
| Device private key | High | Allows impersonation, forged trip uploads |
| Home Wi-Fi credentials | Medium | Stored on device; enables network access if extracted |
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
| **Mitigation** | Encrypt trip bundles at rest on microSD using a device-specific key. Store private key and Wi-Fi credentials in ESP32 eFuse / secure NVS. Consider NVS encryption. |
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
| **Mitigation** | Server tracks content hashes; rejects duplicates. Idempotent design means replay is harmless even if it passes validation. |
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
- Private key stored in ESP32 secure storage (NVS with encryption)
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
