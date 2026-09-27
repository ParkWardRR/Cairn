<p align="center">
  <img src="docs/assets/cairn-banner.svg" alt="Cairn" width="600">
</p>

<h1 align="center">Cairn</h1>
<p align="center"><strong>Offline-first car journal for your homelab</strong></p>
<p align="center"><em>Your drives. Your data. Your server. No cloud required.</em></p>

<p align="center">
  <a href="https://blueoakcouncil.org/license/1.0.0"><img src="https://img.shields.io/badge/license-Blue_Oak_1.0.0-2E86C1?style=flat-square" alt="Blue Oak Model License 1.0.0"></a>
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/status-Phase_1–8_Complete-2ECC71?style=flat-square" alt="Project Status"></a>
  <a href="https://github.com/ParkWardRR/Cairn/actions"><img src="https://img.shields.io/github/actions/workflow/status/ParkWardRR/Cairn/ci.yml?style=flat-square&label=CI" alt="CI"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Zig-F7A41D?style=flat-square&logo=zig&logoColor=white" alt="Zig">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Rust-000000?style=flat-square&logo=rust&logoColor=white" alt="Rust">
  <img src="https://img.shields.io/badge/Gleam-FFAFF3?style=flat-square&logo=gleam&logoColor=black" alt="Gleam">
  <img src="https://img.shields.io/badge/MoonBit-5C2D91?style=flat-square" alt="MoonBit">
  <img src="https://img.shields.io/badge/Odin-1E90FF?style=flat-square" alt="Odin">
  <img src="https://img.shields.io/badge/ESP32-E7352C?style=flat-square&logo=espressif&logoColor=white" alt="ESP32">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/PostGIS-4CAF50?style=flat-square" alt="PostGIS">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/hardware-Freematics_ONE+_B-0D47A1?style=flat-square" alt="Freematics ONE+ Model B">
  <img src="https://img.shields.io/badge/connectivity-Wi--Fi_LAN_only-27AE60?style=flat-square" alt="Wi-Fi LAN Only">
  <img src="https://img.shields.io/badge/cloud-none-95A5A6?style=flat-square" alt="No Cloud">
  <img src="https://img.shields.io/badge/LTE-disabled-E74C3C?style=flat-square" alt="LTE Disabled">
</p>

---

## What is Cairn?

Cairn is an **offline-first vehicle trip journal** built on the [Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus/). It captures GPS and motion data while you drive, stores everything locally on the device's microSD card, and syncs to your homelab **only when you return to your home Wi-Fi**.

No phone. No cloud. No cellular. No subscription. An append-only record of your driving history, owned entirely by you.

| Moment | What Happens |
|--------|-------------|
| **Get in and drive** | Device wakes on motion, starts a local trip session |
| **During the drive** | GNSS + IMU captured at adaptive rates, written to microSD |
| **Park away from home** | Trip finalized locally; device sleeps; nothing transmitted |
| **Return home** | Device detects trusted Wi-Fi, uploads pending trips via mTLS |
| **After sync** | Server validates, deduplicates, and exposes trips in the web UI |
| **Review later** | Browse history, maps, parking, route stats, and export data |

### Design Principles

- **Device never needs the server** — every trip is valid on microSD before any transfer
- **No WAN dependency** — the entire system runs on your LAN
- **Append-only, hash-verified bundles** — SHA-256 + Ed25519 from device to archive
- **Privacy by architecture** — no cloud, no tracking, no third-party data access
- **Each language earns its place** — every tool and service uses the best language for the job

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Vehicle                                                            │
│                                                                     │
│  Freematics ONE+ Model B (ESP32 + GNSS + IMU + microSD + Wi-Fi)    │
│                                                                     │
│  [drive]  GNSS/IMU → append-only trip spool on microSD              │
│  [park]   finalize bundle → SHA-256 + Ed25519 sign → sleep          │
│  [home]   join trusted Wi-Fi → mTLS chunked upload → verify ACK     │
│  [done]   retain until receipt is durable → prune safely             │
└───────────────────────────────┬─────────────────────────────────────┘
                                │ LAN only
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Homelab                                                            │
│                                                                     │
│  ┌──────────────┐   ┌───────────────┐   ┌────────────────────────┐ │
│  │ Go Ingest    │──▶│ PostgreSQL    │──▶│ Go API (16 endpoints)  │ │
│  │ mTLS upload  │   │ + PostGIS     │   │ + PWA (dark theme,     │ │
│  │ resumable    │   │               │   │   Leaflet maps)        │ │
│  │ dedup/receipt│   └───────┬───────┘   └────────────────────────┘ │
│  └──────────────┘           │                                      │
│                    ┌────────┼─────────┐                            │
│                    ▼        ▼         ▼                            │
│  ┌──────────────┐ ┌─────────────┐ ┌────────────────┐              │
│  │ Gleam/OTP    │ │ WASM Plugin │ │ MQTT → Home    │              │
│  │ orchestrator │ │ host (wazero│ │ Assistant      │              │
│  │ 5 actors     │ │ sandbox)    │ │ semantic events│              │
│  └──────────────┘ └─────────────┘ └────────────────┘              │
│                                                                     │
│  Local tools: Odin trip-inspector, trip-diff, trip-replay,         │
│               route-density, sd-recover                             │
│  Emulator:    Rust cairn-emulator (15 scenarios, 5000x speedup)    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Services and Tools

### Go Ingest Service (`zig/ingest/`)

The core server — receives trip bundles from the device and serves the read API.

| Capability | Details |
|------------|---------|
| **Upload** | Resumable chunked uploads with SHA-256 verification and Ed25519 receipts |
| **Read API** | 16 endpoints: stats, trips, route GeoJSON, events, devices, places CRUD, tags, delete |
| **Export** | GPX, GeoJSON, and CSV per trip |
| **MQTT** | Raw MQTT 3.1.1 publisher (zero external deps) for Home Assistant events |
| **Plugins** | WASM plugin host via wazero — classify trips, redact routes, run custom enrichments |
| **CORS** | Built-in middleware for PWA access |

### PWA (`pwa/`)

Offline-capable dark-theme web UI. No build tools, no npm — vanilla HTML/CSS/JS with Leaflet maps.

- **Today** — most recent trip, last parked location, sync status, device health
- **Trips** — time-sorted list with map thumbnails, filter by date/device/tag, pagination
- **Trip detail** — full route on map with speed-colored rendering, timeline, export buttons
- **Places** — saved locations with configurable radius
- **Devices** — firmware version, last sync, trip counts
- **Settings** — data export, trip deletion

### Gleam Trip Orchestrator (`gleam/trip-orchestrator/`)

Five periodic OTP actor workers running on the Erlang VM:

| Worker | Interval | Purpose |
|--------|----------|---------|
| Lifecycle | 30s | Trip lifecycle state projection |
| Quality | 60s | Flag impossible GNSS jumps, GPS loss, clock drift |
| Scheduler | 120s | Retry queue for reprocessing with version tracking |
| Notifier | 60s | Evaluate sync/device/storage issues for notification |
| Automation | 60s | Execute user-defined rules (e.g., "on arrival home, update HA presence") |

### WASM Plugins (`plugins/`)

MoonBit plugins compiled to WASM, executed in a sandboxed wazero runtime. No WASI — plugins cannot access the database, network, or filesystem.

| Plugin | Input | Output |
|--------|-------|--------|
| **trip-classifier** | Trip summary + route samples | Labels with confidence: commute, errand, road trip, canyon drive, night drive, loop trip |
| **privacy-redactor** | Route points + privacy zones | Redacted route with zone-center snapping, triggered zones, redaction stats |

Plugin ABI: `alloc(size) → ptr`, then `entry(ptr, len) → result_ptr`. Length-prefixed UTF-8 JSON over WASM linear memory. 16 MB memory limit, 30s execution timeout, fresh instance per call.

### Odin Native Tools (`odin/`)

Five standalone CLI tools for offline bundle inspection and debugging:

| Tool | Purpose |
|------|---------|
| **trip-inspector** | Full bundle inspection: manifest, SHA-256 integrity, GNSS/IMU stats, quality flags, events, timing |
| **trip-diff** | Side-by-side bundle comparison with Haversine route divergence, timing/speed diff, prefix detection |
| **trip-replay** | Replay bundles into the ingest server with speedup control, dry-run mode, POSIX tar packaging |
| **route-density** | SVG heatmap from GNSS data with configurable grid size and heat/blue color schemes |
| **sd-recover** | Scan microSD for incomplete bundles, detect truncation/corruption, reconstruct manifests and hashes |

### Rust Emulator (`emulator/`)

Full device emulator producing realistic trip bundles with proper binary format, SHA-256 integrity, and chunked upload support. 15 driving scenarios from normal commutes to power-loss edge cases. Tested against the live ingest service at 5000x speedup.

---

## Technology Stack

| Language | Role | Why |
|----------|------|-----|
| **Zig** | Bundle format, common types, CLI (`tripctl`) | Low-level control, deterministic resources, cross-compilation |
| **Go** | Ingest service, read API, MQTT, WASM plugin host | Single-binary deploys, strong networking stdlib, zero-dep MQTT |
| **Rust** | Device emulator, crypto verification | Memory safety without GC, excellent for system simulation |
| **Gleam** | Background event processing, job scheduling | Clean distributed-systems model with OTP fault isolation |
| **MoonBit** | WASM plugins: trip classifier, privacy redactor | Portable sandboxed components via WASM, compiles to 62-94 KB |
| **Odin** | Offline CLI tools: inspector, diff, replay, density, recovery | Pleasant native tooling with explicit memory, fast compilation |
| **C++** | Freematics ESP32 firmware | Shortest path to real driving data on vendor hardware |

### Why not Python?

Every language in this stack was chosen for deterministic resources, strong static types, single-binary deployment, and native performance. This repo contains zero Python. All emulation and test tooling is written in Rust.

---

## Repository Layout

```
Cairn/
├── firmware/                    # ESP32 / Freematics firmware (C++)
│   ├── freematics-base/         #   Hardware abstraction layer
│   ├── hal/                     #   HAL drivers (SPI-DMA, SDMMC, UART, Wi-Fi)
│   ├── trip-recorder/           #   Drive detection + logging state machine
│   └── wifi-provisioner/        #   Home network provisioning
├── zig/                         # Core services and libraries
│   ├── common/                  #   Shared types: GnssSample (32B), ImuSummary (24B), Haversine
│   ├── bundle/                  #   Trip bundle parse / validate / sign
│   ├── ingest/                  #   Go ingest service (mTLS upload, read API, MQTT, plugins)
│   ├── cli/                     #   tripctl: generate, validate, inspect, export
│   └── api/                     #   Legacy Zig API (superseded by Go endpoints in ingest)
├── emulator/                    # Rust device emulator (15 scenarios, chunked upload)
├── gleam/
│   └── trip-orchestrator/       # Gleam/OTP event processing (5 actor workers)
├── plugins/                     # MoonBit → WASM plugins
│   ├── trip-classifier/         #   6 classification heuristics with confidence scoring
│   └── privacy-redactor/        #   Haversine zone detection, coordinate snapping
├── odin/                        # Native CLI tools (Odin)
│   ├── trip-inspector/          #   Bundle inspection and integrity verification
│   ├── trip-diff/               #   Side-by-side bundle comparison
│   ├── trip-replay/             #   Replay bundles into ingest server
│   ├── route-density/           #   SVG heatmap generation
│   └── sd-recover/              #   MicroSD recovery for incomplete bundles
├── pwa/                         # Offline-capable dark-theme web UI
│   ├── index.html               #   SPA shell with Leaflet 1.9.4
│   ├── app.js                   #   Router, API client, 6 views
│   ├── style.css                #   Dark theme, responsive layout
│   ├── sw.js                    #   Service worker for offline caching
│   └── manifest.json            #   PWA manifest
├── deploy/                      # Infrastructure
│   ├── compose.yaml             #   PostgreSQL, Caddy, Mosquitto MQTT
│   ├── caddy/                   #   Reverse proxy + PWA static serving
│   ├── mosquitto/               #   MQTT broker config
│   ├── migrations/              #   SQL: init, functions, Gleam tables
│   └── systemd/                 #   Service units
├── tests/                       # Shell smoke tests
├── fixtures/                    # Test data (raw, synthetic, expected)
├── docs/                        # Architecture, protocols, threat model
├── .github/workflows/ci.yml     # CI: Zig, Go, Rust, Gleam, MoonBit, Odin, PWA
├── ROADMAP.md                   # Phased roadmap with checklists
└── LICENSE                      # Blue Oak Model License 1.0.0
```

---

## Data Model

PostgreSQL + PostGIS:

| Table | Purpose |
|-------|---------|
| `devices` | Public key, metadata, last-seen, firmware version |
| `uploads` | Content hash, upload state, receipt ID, storage path |
| `trips` | Stable trip ID, start/end time and location, distance, duration, bundle hash |
| `location_samples` | Raw GNSS with accuracy, altitude, speed, heading, satellites |
| `motion_samples` | IMU summaries or downsampled aggregates |
| `trip_events` | Start/stop/pause/sync/quality events with location |
| `places` | Named locations with PostGIS geography and configurable radius |
| `trip_tags` | Labels: personal, business, road-trip, private |
| `derivations` | Algorithm version, output hash, reproducibility |
| `trip_lifecycle` | Gleam lifecycle state projection |
| `trip_quality_flags` | GNSS anomalies, GPS loss, clock drift flags |
| `reprocess_queue` | Retry queue for algorithm upgrades |
| `notifications` | Sync/device/storage issue notifications |
| `automation_rules` | User-defined trigger/action rules |

## Trip Bundle Format

Each trip is a self-contained directory on the device's microSD:

```
trip/
  manifest.json       # Trip metadata, device info, schema version
  samples.bin         # GNSS samples — 32 bytes each, little-endian, fixed-width
  imu_summary.bin     # IMU summaries — 24 bytes each, rolling windows
  events.json         # Start/stop/pause/quality events with location
  sha256sums.txt      # Per-file content hashes
```

GNSS samples encode latitude and longitude as `degrees * 10^7` (i32), altitude in centimeters (i32), speed in cm/s (u16), with fix quality, satellite count, HDOP, and accuracy fields. At 1 Hz a 30-minute trip produces ~57 KB of GNSS data.

---

## Home Assistant Integration

Cairn publishes semantic MQTT events to your local Mosquitto broker:

```
cairn/vehicle/{id}/trip_ended        # distance, duration, end place
cairn/vehicle/{id}/sync_completed    # trip count, total bytes
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

---

## API Endpoints

All endpoints served by the Go ingest service on port 8443.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check |
| `POST` | `/api/v1/upload/init` | Init chunked upload |
| `PUT` | `/api/v1/upload/{id}/chunk` | Upload chunk |
| `POST` | `/api/v1/upload/{id}/finalize` | Finalize with SHA-256 verification |
| `GET` | `/api/v1/stats` | Dashboard stats |
| `GET` | `/api/v1/trips` | List trips (filter by device, tag, date range) |
| `GET` | `/api/v1/trips/{id}` | Trip detail with tags |
| `GET` | `/api/v1/trips/{id}/route` | Route as GeoJSON |
| `GET` | `/api/v1/trips/{id}/events` | Trip events |
| `GET` | `/api/v1/trips/{id}/export/gpx` | GPX export |
| `GET` | `/api/v1/trips/{id}/export/geojson` | GeoJSON export with speed/altitude |
| `GET` | `/api/v1/trips/{id}/export/csv` | CSV export |
| `DELETE` | `/api/v1/trips/{id}` | Delete trip and all samples |
| `POST` | `/api/v1/trips/{id}/tags` | Add tag |
| `DELETE` | `/api/v1/trips/{id}/tags/{tag}` | Remove tag |
| `POST` | `/api/v1/trips/{id}/classify` | Run trip-classifier plugin |
| `POST` | `/api/v1/trips/{id}/redact` | Run privacy-redactor plugin |
| `GET` | `/api/v1/devices` | List devices with trip counts |
| `GET` | `/api/v1/places` | List places with visit counts |
| `POST` | `/api/v1/places` | Create place |
| `PUT` | `/api/v1/places/{id}` | Update place |
| `DELETE` | `/api/v1/places/{id}` | Delete place |
| `GET` | `/api/v1/plugins` | List loaded WASM plugins |
| `POST` | `/api/v1/plugins/{name}/run` | Execute plugin with arbitrary input |

---

## Quick Start

### Prerequisites

- [Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus/) with microSD card
- Homelab server (Linux) with Podman or Docker
- Home Wi-Fi network with a stable SSID/BSSID

### Server Setup

```bash
git clone https://github.com/ParkWardRR/Cairn.git
cd Cairn

# Start the stack: PostgreSQL, Caddy, Mosquitto
cd deploy
docker compose up -d

# Apply database migrations
cat migrations/001_init.sql migrations/002_functions.sql migrations/003_gleam_tables.sql \
  | docker exec -i cairn-postgres psql -U cairn -d cairn

# The PWA is served at https://cairn.local
```

### Build from Source

```bash
# Go ingest service
cd zig/ingest && go build ./cmd/ingestd/

# Zig CLI tools
cd zig/cli && zig build

# Rust emulator
cd emulator && cargo build --release

# Gleam orchestrator
cd gleam/trip-orchestrator && gleam build

# MoonBit plugins
cd plugins/trip-classifier && moon build --target wasm
cd plugins/privacy-redactor && moon build --target wasm

# Odin tools
for tool in trip-inspector trip-diff trip-replay route-density sd-recover; do
  odin build "odin/$tool/" -o:speed
done
```

### Test with the Emulator

```bash
# Generate a realistic trip and upload to local server
./emulator/target/release/cairn-emulator \
  --scenario normal_commute \
  --server http://localhost:8443 \
  --speedup 1000

# Inspect a local bundle
odin build odin/trip-inspector/ -o:speed
./trip-inspector /path/to/trip/bundle/

# Generate a route density heatmap
odin build odin/route-density/ -o:speed
./route-density /path/to/bundles/ --output heatmap.svg
```

---

## Explicit Non-Goals

| Excluded | Reason |
|----------|--------|
| OBD-II diagnostic polling | Bus wake-up/compatibility/power complexity with no product value |
| LTE / cellular / WAN upload | Cost, external dependency, privacy leakage |
| Cloud account or third-party backend | Contradicts local-first ownership |
| Live vehicle tracking | Requires persistent connectivity |
| Mobile native app | LAN PWA is sufficient until a clear need emerges |
| Automatic emergency dispatch | High liability and no WAN by design |
| Python | See [technology decisions](#technology-stack) |

## Acceptance Criteria

| Test | Pass Condition |
|------|---------------|
| Normal week | Every meaningful drive appears on the dashboard after returning home |
| No phone | Trips work with no phone paired, carried, or charged |
| No internet | WAN disconnected; local sync and UI still work |
| Power interruption | Previously finalized data survives; incomplete trip recoverable via `sd-recover` |
| Multi-day trip | Device retains all data without attempting WAN upload |
| Return home | Pending trips upload once; no duplicates after retries/reboots |
| Bad GNSS | Garage/tunnel segments marked uncertain, not fabricated |
| Storage pressure | Unsynced trips preserved; alert emitted on next home arrival |
| Server restart | Client resumes upload cleanly after server restarts |
| Plugin sandboxing | WASM plugins cannot access database, network, or filesystem |
| Backup restore | Restore into fresh environment; browse and export all historical trips |

## CI Pipeline

The [CI workflow](.github/workflows/ci.yml) runs on a self-hosted runner and validates all languages:

| Job | What it builds/tests |
|-----|---------------------|
| `build-zig` | Zig common, bundle, CLI + smoke test |
| `build-go` | Go ingest service + `go vet` |
| `build-rust` | Rust emulator + binary verification |
| `build-gleam` | Gleam trip-orchestrator build + test |
| `build-plugins` | MoonBit trip-classifier + privacy-redactor → WASM |
| `build-odin` | All 5 Odin tools + trip-inspector smoke test |
| `validate-pwa` | PWA file presence check |
| `emulator-test` | Emulator scenarios against live ingest |
| `integration-test` | End-to-end smoke test |

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, data flow, component responsibilities |
| [Device Protocol](docs/device-protocol.md) | Firmware states, sensor rates, sync protocol |
| [Trip File Format](docs/trip-file-format.md) | Bundle schema, sample encoding (32-byte GNSS, 24-byte IMU) |
| [Threat Model](docs/threat-model.md) | Security boundaries, device identity, transport security |
| [Retention & Backup](docs/retention-and-backup.md) | Data lifecycle from device spool to archive |
| [Home Wi-Fi Deployment](docs/home-wifi-deployment.md) | Network setup, mDNS, certificates, provisioning |
| [Emulation Spec](docs/freematics-emulation-spec.md) | Rust emulator driving scenarios and protocol |
| [Test Plan](docs/test-plan.md) | Hardware testing guide |
| [RTOS Evaluation](docs/rtos-evaluation.md) | Zephyr/NuttX feasibility research (not adopted) |
| [Roadmap](ROADMAP.md) | Phased development plan with checklists |

## Contributing

Cairn is an early-stage personal project. Contributions, ideas, and feedback are welcome.

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Open a pull request

Please read the [Architecture](docs/architecture.md) and [Roadmap](ROADMAP.md) before contributing.

## License

[Blue Oak Model License 1.0.0](https://blueoakcouncil.org/license/1.0.0) — see [LICENSE](LICENSE).

A permissive license designed by lawyers, for clarity. Broad permission to use, share, and modify with no conditions beyond preserving the license notice.

---

<p align="center">
  <sub>Built for the joy of driving and the sovereignty of owning your data.</sub>
</p>
