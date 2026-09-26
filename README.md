<p align="center">
  <img src="docs/assets/cairn-banner.svg" alt="Cairn" width="600">
</p>

<h1 align="center">Cairn</h1>
<p align="center"><strong>Garage-sync, offline-first car journal</strong></p>
<p align="center"><em>Your drives. Your data. Your homelab. No cloud required.</em></p>

<p align="center">
  <a href="https://blueoakcouncil.org/license/1.0.0"><img src="https://img.shields.io/badge/license-Blue_Oak_1.0.0-2E86C1?style=flat-square" alt="Blue Oak Model License 1.0.0"></a>
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/status-Phase_0_·_Planning-6C3483?style=flat-square" alt="Project Status"></a>
  <a href="https://github.com/ParkWardRR/Cairn/actions"><img src="https://img.shields.io/github/actions/workflow/status/ParkWardRR/Cairn/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/ParkWardRR/Cairn/issues"><img src="https://img.shields.io/github/issues/ParkWardRR/Cairn?style=flat-square&color=E74C3C" alt="Issues"></a>
  <a href="https://github.com/ParkWardRR/Cairn/pulls"><img src="https://img.shields.io/github/issues-pr/ParkWardRR/Cairn?style=flat-square&color=2ECC71" alt="Pull Requests"></a>
  <a href="https://github.com/ParkWardRR/Cairn/stargazers"><img src="https://img.shields.io/github/stars/ParkWardRR/Cairn?style=flat-square&color=F39C12" alt="Stars"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Zig-F7A41D?style=flat-square&logo=zig&logoColor=white" alt="Zig">
  <img src="https://img.shields.io/badge/Gleam-FFAFF3?style=flat-square&logo=gleam&logoColor=black" alt="Gleam">
  <img src="https://img.shields.io/badge/MoonBit-5C2D91?style=flat-square" alt="MoonBit">
  <img src="https://img.shields.io/badge/Odin-1E90FF?style=flat-square" alt="Odin">
  <img src="https://img.shields.io/badge/Mojo-FF6F00?style=flat-square" alt="Mojo">
  <img src="https://img.shields.io/badge/ESP32-E7352C?style=flat-square&logo=espressif&logoColor=white" alt="ESP32">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/PostGIS-4CAF50?style=flat-square" alt="PostGIS">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/hardware-Freematics_ONE+_B-0D47A1?style=flat-square" alt="Freematics ONE+ Model B">
  <img src="https://img.shields.io/badge/connectivity-Wi--Fi_LAN_only-27AE60?style=flat-square" alt="Wi-Fi LAN Only">
  <img src="https://img.shields.io/badge/cloud-none-95A5A6?style=flat-square" alt="No Cloud">
  <img src="https://img.shields.io/badge/OBD--II-power_only-F39C12?style=flat-square" alt="OBD Power Only">
  <img src="https://img.shields.io/badge/LTE-disabled-E74C3C?style=flat-square" alt="LTE Disabled">
</p>

<p align="center">
  <a href="https://github.com/ParkWardRR/Cairn/graphs/contributors"><img src="https://img.shields.io/github/contributors/ParkWardRR/Cairn?style=flat-square" alt="Contributors"></a>
  <a href="https://github.com/ParkWardRR/Cairn/commits"><img src="https://img.shields.io/github/last-commit/ParkWardRR/Cairn?style=flat-square" alt="Last Commit"></a>
  <a href="https://github.com/ParkWardRR/Cairn/"><img src="https://img.shields.io/github/repo-size/ParkWardRR/Cairn?style=flat-square" alt="Repo Size"></a>
  <a href="#contributing"><img src="https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square" alt="PRs Welcome"></a>
</p>

---

## What is Cairn?

Cairn is an **offline-first vehicle trip journal** built on the [Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus/). It captures GPS and motion data while you drive, stores everything locally on the device's microSD card, and syncs to your homelab **only when you return to your home Wi-Fi**.

No phone required. No cloud account. No cellular connection. No subscription. Just an append-only record of your driving history, owned entirely by you.

### The Experience

| Moment | What Happens |
|--------|-------------|
| **Get in and drive** | Device wakes on motion, starts a local trip session — no phone or app needed |
| **During the drive** | GNSS + IMU capture at adaptive rates, writing to microSD |
| **Park away from home** | Trip finalized locally; device sleeps; nothing is transmitted |
| **Return home** | Device detects trusted Wi-Fi, uploads pending trips over mTLS to your LAN server |
| **After sync** | Server validates, deduplicates, segments, and exposes trips in a local web UI |
| **Review later** | Browse drive history, maps, parking locations, route statistics, and export data |

### Design Principles

- **Device never needs the server to complete a drive** — every trip is valid on microSD before any transfer
- **No WAN dependency** — the entire system runs on your LAN; no internet required
- **Append-only, hash-verified bundles** — data integrity from device to archive
- **Privacy by architecture** — no cloud, no tracking, no third-party data access

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ Car                                                             │
│                                                                 │
│ Freematics ONE+ Model B                                         │
│  ESP32 + GNSS + IMU + microSD + Wi-Fi                           │
│                                                                 │
│  [drive]  GNSS / IMU → append-only encrypted local trip spool   │
│  [park]   finalize bundle → sleep / periodic SSID check         │
│  [home]   join home Wi-Fi → mTLS upload → verify ACK            │
│  [done]   retain until server receipt is durable → prune safely  │
└──────────────────────────────┬──────────────────────────────────┘
                               │ LAN only; no internet required
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ Homelab                                                         │
│                                                                 │
│ ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐      │
│ │ Zig Ingest  │→ │ PostgreSQL   │→ │ Zig API + PWA      │      │
│ │ (mTLS,      │  │ + PostGIS    │  │ (trips, maps,      │      │
│ │  resumable) │  │              │  │  places, export)   │      │
│ └─────────────┘  └──────┬───────┘  └────────────────────┘      │
│                         │                                       │
│        ┌────────────────┼────────────────┐                     │
│        ▼                ▼                ▼                      │
│ ┌────────────┐  ┌──────────────┐  ┌─────────────┐             │
│ │ Gleam      │  │ MoonBit      │  │ MQTT → HA   │             │
│ │ Orchestr.  │  │ Plugins      │  │ Events      │             │
│ └────────────┘  └──────────────┘  └─────────────┘             │
│                                                                 │
│ Offline tools: Odin trip-inspector · Mojo experiments           │
└─────────────────────────────────────────────────────────────────┘
```

## Technology Stack

| Technology | Role | Why |
|-----------|------|-----|
| **Zig** | Device tooling, binary format, LAN ingest, API, CLI | Low-level control, cross-compilation, deterministic resources |
| **Gleam** | Background event processing, job state machines, retry workflows | Clean distributed-systems model with OTP supervision |
| **MoonBit** | WASM plugins: trip classifier, privacy redactor, export transforms | Portable component/plugin design via WASM sandbox |
| **Odin** | Offline desktop tools: trip inspector, route density, SD recovery | Pleasant native tooling for debugging and exploration |
| **Mojo** | Experimental: route clustering, map matching, trajectory analysis | GPU-friendly analysis without production dependency |
| **Arduino/C++** | Freematics firmware (hardware enablement) | Shortest path to real driving data on ESP32 |

## Explicit Non-Goals

These are **deliberately excluded** from v1:

| Excluded | Reason |
|----------|--------|
| OBD-II diagnostic polling | Adds bus wake-up/compatibility/power complexity with no product value |
| LTE / cellular / WAN upload | Avoids cost, external dependency, and privacy leakage |
| Cloud account or third-party backend | Contradicts local-first ownership |
| Live vehicle tracking | Requires persistent connectivity |
| Mobile native app | A LAN PWA is sufficient until a clear need emerges |
| Automatic emergency dispatch | High liability and no WAN by design |
| Vehicle coding / CAN injection | Unrelated and riskier; needs separate tooling |

## Repository Layout

```
Cairn/
├── docs/                    # Architecture, protocols, threat model
├── firmware/                # ESP32 / Freematics firmware
│   ├── freematics-base/     # Hardware abstraction
│   ├── trip-recorder/       # Drive detection + logging
│   ├── wifi-provisioner/    # Home network provisioning
│   └── test-fixtures/       # Hardware test scenarios
├── zig/                     # Core services (Zig)
│   ├── common/              # Shared types, crypto, config
│   ├── bundle/              # Trip bundle parse/validate/sign
│   ├── ingest/              # mTLS upload receiver
│   ├── api/                 # REST API for web UI
│   ├── cli/                 # tripctl command-line tool
│   └── simulator/           # Trip replay for testing
├── gleam/                   # Event services (Gleam/OTP)
│   └── trip-orchestrator/   # Lifecycle, retry, automation
├── moonbit/                 # WASM plugins (MoonBit)
│   └── plugins/
│       ├── trip-classifier/
│       ├── privacy-redactor/
│       └── exporter/
├── odin/                    # Native tools (Odin)
│   └── trip-inspector/
├── mojo/                    # Experimental (Mojo)
│   └── experiments/
├── deploy/                  # Docker Compose, Caddy, systemd
├── fixtures/                # Test data
├── ROADMAP.md               # Phased roadmap with checklists
└── LICENSE                  # Blue Oak Model License 1.0.0
```

## Quick Start

> Cairn is in **Phase 0 — Planning**. The sections below describe the target setup.

### Prerequisites

- [Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus/) with microSD card
- USB-to-serial adapter for firmware flashing
- Homelab server (Linux/macOS) with Docker
- Home Wi-Fi network with a stable SSID/BSSID

### Device Setup

```bash
# Clone the repository
git clone https://github.com/ParkWardRR/Cairn.git
cd Cairn

# Flash the firmware (Phase 1)
cd firmware/freematics-base
# See docs/device-protocol.md for flashing instructions
```

### Server Setup

```bash
# Start the homelab stack (Phase 4+)
cd deploy
docker compose up -d

# The local web UI will be available at https://cairn.local
```

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, data flow, and component responsibilities |
| [Device Protocol](docs/device-protocol.md) | Firmware states, sensor rates, and sync protocol |
| [Trip File Format](docs/trip-file-format.md) | Bundle schema, CBOR metadata, and sample encoding |
| [Threat Model](docs/threat-model.md) | Security boundaries, device identity, and transport security |
| [Retention & Backup](docs/retention-and-backup.md) | Data lifecycle from device spool to archive |
| [Home Wi-Fi Deployment](docs/home-wifi-deployment.md) | Network setup, mDNS, certificates, and provisioning |
| [Roadmap](ROADMAP.md) | Full phased roadmap with checklists |

## Data Model

Core tables (PostgreSQL + PostGIS):

| Table | Purpose |
|-------|---------|
| `devices` | Public key, metadata, last-seen, firmware version |
| `uploads` | Content hash, upload state, receipt ID, storage path |
| `trips` | Stable trip ID, start/end, device, summary, bundle hash |
| `location_samples` | Raw GNSS with accuracy and sequence |
| `motion_samples` | IMU data or downsampled motion aggregates |
| `trip_events` | Start/stop/pause/sync/quality events |
| `places` | Named locations (home, work, trailhead, etc.) |
| `trip_tags` | Personal/business/road-trip/private labels |
| `derivations` | Algorithm version, output hash, reproducibility metadata |

## Home Assistant Integration

Cairn publishes semantic MQTT events to your local broker — not raw GPS spam:

```
cairn/vehicle/{id}/trip_started
cairn/vehicle/{id}/trip_ended
cairn/vehicle/{id}/arrived_home
cairn/vehicle/{id}/departed_home
cairn/vehicle/{id}/sync_completed
cairn/vehicle/{id}/last_parked
```

```json
{
  "vehicle_id": "mazda",
  "trip_id": "01J9YTVPAK6WWZQGBQQN8PB7B8",
  "event": "trip_ended",
  "ended_at": "2026-09-26T06:05:24Z",
  "distance_km": 18.42,
  "duration_s": 1934,
  "end_place": "home",
  "local_only": true
}
```

## Acceptance Criteria

| Test | Pass Condition |
|------|---------------|
| Normal week | Every meaningful drive appears on the dashboard after returning home |
| No phone | Trips work with no phone paired, carried, or charged |
| No internet | WAN disconnected; local sync and UI still work |
| Power interruption | Previously finalized data survives; incomplete trip recoverable |
| Multi-day trip | Device retains all data without attempting WAN upload |
| Return home | Pending trips upload once; no duplicates after retries/reboots |
| Bad GNSS | Garage/tunnel segments marked uncertain, not fabricated |
| Storage pressure | Unsynced trips preserved; alert emitted on next home arrival |
| Server restart | Client resumes upload cleanly after server restarts |
| Backup restore | Restore into fresh environment; browse and export all historical trips |

## Contributing

Cairn is an early-stage personal project. Contributions, ideas, and feedback are welcome.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes
4. Open a pull request

Please read the [Architecture](docs/architecture.md) and [Roadmap](ROADMAP.md) before contributing.

## License

[Blue Oak Model License 1.0.0](https://blueoakcouncil.org/license/1.0.0) — see [LICENSE](LICENSE).

A permissive license designed by lawyers, for clarity. It grants broad permission to use, share, and modify this software with no conditions beyond preserving the license notice.

---

<p align="center">
  <sub>Built for the joy of driving and the sovereignty of owning your data.</sub>
</p>
