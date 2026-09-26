# Cairn Roadmap

> Phased development plan for the Cairn offline-first car journal.
> Each phase builds on the previous. Items are checked off as they are completed.

---

## Phase 0 — Define the Contract

**Goal:** Know precisely what constitutes a trip, what gets stored, and what "sync succeeded" means before writing firmware.

**Timeline:** Week 0

### Deliverables

- [x] **Product specification**
  - [x] Write one-page statement of core flow
  - [x] Document non-goals and explicit exclusions
  - [x] Define privacy defaults and operational model
  - [x] Specify supported vehicles and mounting considerations

- [x] **Device state machine**
  - [x] Define all states: `sleep`, `arming`, `recording`, `finalizing`, `awaiting_home_wifi`, `syncing`, `low_battery_protection`, `fault`
  - [x] Document transitions between states with trigger conditions
  - [x] Specify timeout values and debounce thresholds
  - [x] Diagram the state machine

- [x] **Trip bundle format**
  - [x] Design versioned, self-describing bundle schema
  - [x] Define CBOR manifest structure
  - [x] Define binary sample encoding for GNSS and IMU
  - [x] Specify checksum and signature scheme (SHA-256 + Ed25519)
  - [x] Document zstd compression strategy (server-side; raw on device for v1)

- [x] **Home network design**
  - [x] Define trusted SSID/BSSID allowlist mechanism
  - [x] Plan WPA credential provisioning (serial-first)
  - [x] Specify local hostname/IP resolution (mDNS + static DHCP)
  - [x] Design certificate strategy (local CA, device keypair, server pinning)

- [x] **Data retention policy**
  - [x] Define device spool period (7–30 day configurable window)
  - [x] Define server retention tiers (raw, normalized, derived)
  - [x] Specify backup schedule and restore procedure
  - [x] Document deletion rules — never delete without server receipt + retention window

- [x] **Test corpus**
  - [x] Create synthetic scenario: normal 45-minute drive
  - [x] Create synthetic scenario: short stop (gas station, drive-through)
  - [x] Create synthetic scenario: long stop (multi-hour parking)
  - [x] Create synthetic scenario: no GNSS fix (garage/tunnel)
  - [x] Create synthetic scenario: interrupted write (power loss mid-trip)
  - [x] Create synthetic scenario: interrupted upload (Wi-Fi loss mid-sync)

### Bundle Structure

```
trip/
  manifest.cbor       # Trip metadata, device info, schema version
  samples.bin.zst     # GNSS + IMU samples (zstd on server; raw on device)
  events.cbor         # Start/stop/pause/quality markers
  sha256sums.txt      # Per-file content hashes
  signature.ed25519   # Device signature over sha256sums.txt
```

### Data Retention States

| State | Location | Meaning |
|-------|----------|---------|
| `recording` | Active device file | Trip being written with periodic fsync |
| `finalized` | Device microSD | Immutable bundle, complete and hashable |
| `queued` | Device microSD | Awaiting trusted home Wi-Fi |
| `uploading` | Device + server staging | Chunked/resumable transfer in progress |
| `acknowledged` | Device + server durable | Server returned receipt for content hash |
| `retained` | Device microSD | Kept for configurable safety window |
| `prunable` | Device microSD | Eligible for removal after receipt + retention |
| `archived` | Homelab backups | Long-term primary record |

---

## Phase 1 — Bench the Hardware

**Goal:** Prove the Freematics ONE+ Model B can produce reliable local GNSS/IMU logs and reconnect to home Wi-Fi — without OBD data, LTE, or a vendor cloud.

**Timeline:** Week 1

### Deliverables

- [x] **Flash baseline firmware**
  - [x] Set up Arduino IDE / PlatformIO for Freematics ONE+ Model B
  - [x] HAL integration: all ESP32 hardware accelerators wired into state machine
  - [ ] Successfully erase, flash, and serial-monitor the device
  - [ ] Document recovery procedure for bricked state
  - [ ] Verify serial debug output is reliable

- [x] **Disable unused radios**
  - [x] Wi-Fi radio powered off during recording (HAL power management)
  - [x] Bluetooth powered off except during provisioning
  - [ ] Confirm LTE modem never initializes (no SIM required)
  - [ ] Verify no unintended radio activity with RF monitor

- [x] **GNSS logger**
  - [x] Write timestamped GNSS samples to microSD via SDMMC 4-bit DMA
  - [x] Capture: latitude, longitude, altitude, speed, heading
  - [x] Capture: fix quality, satellite count, HDOP/accuracy
  - [x] NMEA parsing from UART2 (Freematics GNSS module)
  - [ ] Validate 10 Hz sample rate achievable
  - [ ] Collect first real-world drive data

- [x] **IMU logger**
  - [x] Capture accelerometer/gyro via SPI-DMA burst reads
  - [x] Impact/hard-brake/sharp-turn detection from accel/gyro
  - [ ] Verify timestamp alignment with GNSS samples
  - [ ] Determine useful sample rate (25–50 Hz starting point)

- [x] **Wi-Fi station mode**
  - [x] BSSID-locked association to trusted home network
  - [x] HAL Wi-Fi scan/connect with power save modes
  - [ ] Expose local health/status endpoint over HTTP
  - [ ] Verify reconnection after power cycle

- [x] **Power characterization (firmware)**
  - [x] Dynamic frequency scaling (240/160/80 MHz) per device state
  - [x] ULP coprocessor motion wake (~150 µA deep sleep)
  - [x] Hardware brownout detection
  - [x] ADC battery voltage with factory calibration
  - [ ] Measure active driving current (GNSS + IMU + microSD write)
  - [ ] Measure deep sleep current
  - [ ] Compare against Freematics spec (~10 mA sleep claim)
  - [ ] Test in both target vehicles

- [x] **Data durability (firmware)**
  - [x] fsync after every trip finalization for crash consistency
  - [x] Hardware SHA-256 checksums for all bundle files
  - [ ] Pull power during active write (hardware test)
  - [ ] Verify previously finalized data survives
  - [ ] Verify partial write is detectable/recoverable

- [ ] **RF validation**
  - [ ] Compare route quality in vehicle 1
  - [ ] Compare route quality in vehicle 2
  - [ ] Determine if external GNSS antenna or OBD extension cable is needed
  - [ ] Document OBD port location impact on GNSS reception

---

## Phase 2 — Offline Trip Recorder

**Goal:** A car ride turns into exactly one or more correctly formed local trip bundles, even with no network available.

**Timeline:** Weeks 2–3

### State Machine

```
SLEEP
  │ motion / GNSS activity / power-rise
  ▼
ARMING
  │ sustained movement for N seconds
  ▼
RECORDING
  │ no meaningful movement for configured dwell time
  ▼
STOP_CANDIDATE
  ├── movement resumes ────────► RECORDING
  └── dwell threshold reached ─► FINALIZING
                                   │
                                   ▼
                             QUEUED_FOR_HOME_SYNC
```

### Deliverables

- [x] **Ignition / activity heuristic**
  - [x] Detect drive start from GNSS speed + IMU activity
  - [x] Use OBD port voltage only as power context (not diagnostics)
  - [ ] Validate heuristic across both target vehicles

- [x] **Start debounce**
  - [x] Filter out noise, door slams, and minor garage movement
  - [x] Require sustained movement for configurable threshold (e.g., 15 seconds)
  - [ ] Test with real-world false triggers

- [x] **Adaptive sample scheduling**
  - [x] Active driving: 1–5 Hz GNSS, 25–50 Hz IMU
  - [x] Slow maneuvering / parking: 1–2 Hz GNSS, 25 Hz IMU
  - [x] Stationary stop candidate: 0.2–1 Hz GNSS, 10–25 Hz IMU
  - [x] Parked: GNSS off, low-power motion wake only

- [x] **Stop debounce**
  - [x] Avoid fragmenting trips during traffic lights
  - [x] Avoid fragmenting during fuel stops and drive-throughs
  - [x] Configurable dwell time threshold
  - [x] Prefer merging short errands over producing false trips

- [x] **Parking snapshot**
  - [x] Record final reliable location with GNSS accuracy
  - [x] Record heading and timestamp
  - [x] Mark as parking endpoint in trip events

- [x] **Local event markers**
  - [x] Emit: trip_start, trip_stop, trip_pause, trip_resumed
  - [x] Emit: poor_gnss_quality, gnss_restored
  - [x] Emit: power_anomaly, storage_pressure
  - [x] Store events in trip bundle's events.cbor

- [x] **Storage recovery**
  - [x] Implement append-only record format with periodic checkpoints
  - [x] Boot-time scan and recovery of incomplete sessions
  - [ ] Validate recovery after simulated power loss

- [x] **Capacity controls**
  - [x] Monitor microSD free space
  - [x] Prevent exhaustion with configurable threshold
  - [x] Preserve unsynced data according to retention policy
  - [x] Alert on next home sync if storage was under pressure

---

## Phase 3 — Home-Only Synchronization

**Goal:** Driving data reaches the homelab only after the device joins trusted home Wi-Fi. Never over cellular. Never to the internet.

**Timeline:** Week 3

### Deliverables

- [x] **Wi-Fi provisioning**
  - [x] Store trusted network credentials in encrypted NVS
  - [x] Load SSID/PSK/BSSID at boot from NVS
  - [ ] Serial-based initial configuration CLI
  - [ ] Plan for future captive portal or BLE provisioning

- [x] **Trusted-network policy**
  - [x] Only sync after BSSID scan matches stored trusted AP
  - [x] BSSID-locked association prevents rogue SSID attacks
  - [ ] Log and alert on unexpected network association attempts

- [x] **Device identity**
  - [x] Generate Ed25519 keypair via hardware RNG + crypto accelerator
  - [x] Store private key in encrypted NVS
  - [ ] Register public key with homelab server

- [ ] **Transport security**
  - [ ] Implement HTTPS with mutual TLS (mTLS)
  - [ ] Pin local CA certificate on device
  - [ ] Pin server public key / hostname
  - [ ] No public Web PKI dependency

- [x] **Resumable upload**
  - [x] Chunked upload using trip ID + content hash + byte offset
  - [x] Resume after Wi-Fi disconnect or vehicle departure
  - [x] Handle partial uploads gracefully

- [x] **Server receipt**
  - [x] Server issues signed/durable acknowledgment
  - [x] Receipt ties to trip ID and content hash
  - [x] Receipt issued only after object + database record committed

- [x] **Device cleanup**
  - [x] Delete local trip only after receipt + retention window
  - [x] Store receipt locally as proof of server acknowledgment
  - [x] Never delete merely because an HTTP request succeeded

- [ ] **Upload budget**
  - [ ] Limit Wi-Fi transmit duration per sync session
  - [ ] Suspend/resume safely if vehicle leaves garage mid-upload
  - [x] Prioritize oldest unsynced trips

- [x] **Local discovery**
  - [x] mDNS (`cairn.local`) for initial deployment
  - [ ] Static DHCP reservation for production reliability

---

## Phase 4 — Zig Ingest and Data Model

**Goal:** Browse uploaded trips locally. Re-uploading the same file never duplicates data. All core services run in Zig.

**Timeline:** Weeks 3–4

### Deliverables

- [x] **`bundle` library**
  - [x] Parse trip bundles from device
  - [x] Validate checksums and signatures
  - [x] Hash and verify content integrity
  - [ ] Support schema version migration

- [x] **`ingestd` service**
  - [x] mTLS endpoint for device uploads
  - [x] Resumable chunked upload support
  - [x] Replay protection (reject duplicate content hashes)
  - [x] Rate limiting per device
  - [x] Issue durable receipts on successful commit

- [x] **`tripctl` CLI**
  - [x] Inspect local trip bundles
  - [x] Validate storage integrity
  - [x] Generate synthetic test fixtures
  - [x] Export raw data (GPX, GeoJSON, CSV)

- [x] **`trip-sim` simulator**
  - [x] Replay historical drives into ingest pipeline
  - [x] Generate synthetic journeys
  - [x] Support accelerated and real-time replay

- [x] **`api` service**
  - [x] Read-only JSON API for trips
  - [x] Endpoints: trips, places, route segments, tags, exports
  - [x] PostGIS queries for nearest-place and distance

- [x] **Database schema**
  - [x] `devices` — public key, metadata, last-seen, firmware version
  - [x] `uploads` — content hash, state, receipt ID, storage path
  - [x] `trips` — stable ID, start/end, device, summary, bundle hash
  - [x] `location_samples` — raw GNSS with accuracy and sequence
  - [x] `motion_samples` — IMU or downsampled aggregates
  - [x] `trip_events` — start/stop/pause/sync/quality events
  - [x] `places` — named locations with radius
  - [x] `trip_tags` — personal/business/road-trip/private labels
  - [x] `derivations` — algorithm version, output hash, reproducibility
  - [x] All timestamps in UTC; render in viewer per timezone
  - [x] PostGIS extensions enabled

---

## Phase 5 — Local Web UI and Home Assistant

**Goal:** The project feels like the part of Automatic you actually liked — effortless trip history, remembered parking, and useful presence/arrival events.

**Timeline:** Days 31–60

### Web UI Views

- [x] **Today view**
  - [x] Most recent trip summary
  - [x] Last parked location on map
  - [x] Sync status indicator
  - [x] Device battery/health summary

- [x] **Trips view**
  - [x] Time-sorted trip list
  - [x] Map thumbnail per trip
  - [x] Duration, distance, endpoint, tags
  - [x] Filter by date range, tag, place

- [x] **Trip detail view**
  - [x] Full route on map
  - [x] Timeline with stop candidates
  - [x] GNSS quality overlay
  - [x] Raw data download / export buttons

- [x] **Places view**
  - [x] Saved locations with configurable radius
  - [ ] Arrival/departure history
  - [ ] Rename and merge controls

- [x] **Device view**
  - [x] Firmware version
  - [ ] Storage usage
  - [x] Last sync time
  - [ ] Wi-Fi state and low-power state

- [x] **Privacy / data view**
  - [ ] Retention controls
  - [x] Export all data
  - [x] Delete individual trips
  - [ ] Redact start/end areas (privacy zones)

### Home Assistant Integration

- [x] **MQTT event bus**
  - [ ] Publish `trip_started` event
  - [x] Publish `trip_ended` event with distance/duration
  - [ ] Publish `arrived_home` / `departed_home`
  - [x] Publish `sync_completed`
  - [ ] Publish `last_parked` location

- [x] **Resilience**
  - [ ] Persist events for replay if HA is unavailable
  - [x] Never make HA availability a reason for trip failure
  - [x] Semantic events only — no raw GPS stream

---

## Phase 6 — Introduce Gleam Deliberately

**Goal:** Gleam improves the system's event semantics without becoming a second ingestion path. Build only after Zig ingest and the database are stable.

**Timeline:** Days 61–75

### Deliverables

- [x] **Trip lifecycle projector**
  - [x] Convert raw/derived state into coherent trip lifecycle events
  - [x] Maintain event consistency across reprocessing

- [x] **Retry scheduler**
  - [x] Reprocess trips when algorithms improve
  - [x] Idempotent reprocessing with version tracking

- [x] **Automation executor**
  - [x] Apply rules: "on arrival home, update HA presence"
  - [x] Configurable rule engine for user-defined automations

- [x] **Notification policy**
  - [x] Evaluate whether sync/device/storage issues deserve notification
  - [x] Configurable severity thresholds

- [x] **Data-quality queue**
  - [x] Flag impossible GNSS jumps
  - [x] Flag prolonged GPS loss
  - [x] Detect duplicate tracks and clock drift

- [x] **OTP supervision**
  - [x] Fault-tolerant supervision tree
  - [x] Independent restart of failing background tasks
  - [ ] Health monitoring and reporting

---

## Phase 7 — MoonBit Plugin System

**Goal:** User-defined or experimental trip enrichments run in a constrained, portable WASM sandbox. Plugins cannot access the database, network, or filesystem.

**Timeline:** Days 61–75

### Plugin ABI

- [ ] **Design narrow plugin interface**
  - [ ] Define versioned input/output schemas
  - [ ] Deterministic execution — same input always produces same output
  - [ ] Host validates all plugin results
  - [ ] Record plugin version/hash for every derivation

### Plugins

- [ ] **Trip classifier**
  - [ ] Input: normalized trip summary + sampled route/motion
  - [ ] Output: labels (commute, canyon drive, errand, road trip, unknown)

- [ ] **Privacy redactor**
  - [ ] Input: route + configured privacy zones
  - [ ] Output: redacted route/endpoint geometry

- [ ] **Export transformer**
  - [ ] Input: trip model
  - [ ] Output: GPX, GeoJSON, CSV, Markdown trip report

- [ ] **Route scorer**
  - [ ] Input: polyline + places
  - [ ] Output: favorite-road / repeat-route score

- [ ] **Data-quality detector**
  - [ ] Input: timestamped samples
  - [ ] Output: anomaly annotations

### Sandbox Constraints

- [ ] No database access
- [ ] No network access
- [ ] No filesystem access
- [ ] No authority to alter raw data
- [ ] Structured input → structured output only

---

## Phase 8 — Odin Native Tools

**Goal:** An enjoyable, high-performance local utility for debugging and exploring recorded journeys. One focused tool, not another platform.

**Timeline:** Days 75–90

### Deliverables

- [ ] **`trip-inspector`**
  - [ ] Open local trip bundles
  - [ ] Inspect metadata, sample timing, GNSS accuracy
  - [ ] Visualize speed and IMU data

- [ ] **`trip-diff`**
  - [ ] Compare device raw route vs. server-normalized route
  - [ ] Highlight divergence points

- [ ] **`trip-replay`**
  - [ ] Feed recorded trips into test server
  - [ ] Support realistic and accelerated timing

- [ ] **`route-density`**
  - [ ] Generate heatmap of frequently driven roads
  - [ ] Local image/vector output

- [ ] **`sd-recover`**
  - [ ] Scan pulled microSD card for incomplete trip files
  - [ ] Rebuild recoverable trip data

---

## Phase 9 — Mojo Experimental Lane

**Goal:** Explore accelerator-friendly trajectory analysis without making the product depend on it. Start only after several months of clean local data.

**Timeline:** Days 90+

### Experiments

- [ ] **Route similarity**
  - [ ] Cluster trips: "These 14 trips are effectively the same commute"
  - [ ] Output candidate route groups for manual review

- [ ] **Automatic place discovery**
  - [ ] Identify candidate recurring endpoints
  - [ ] Subject to privacy review before surfacing

- [ ] **Stop/errand segmentation**
  - [ ] Identify meaningful stops in long routes
  - [ ] Compare against device-side stop detection

- [ ] **GNSS smoothing / map-matching evaluation**
  - [ ] Compare algorithms against manually checked routes
  - [ ] Benchmark accuracy and performance

- [ ] **Driving-style visualization**
  - [ ] Cluster acceleration/turning patterns
  - [ ] Personal exploration only — not scoring

- [ ] **Anomaly detection**
  - [ ] Flag routes/speed traces that indicate bad GNSS
  - [ ] Distinguish sensor error from real driving behavior

### Guardrails

- [ ] Authoritative output stays in deterministic Zig/Gleam paths
- [ ] Mojo proposes annotations; never decides deletion or trip boundaries
- [ ] No safety-critical actions
- [ ] All outputs reproducible without GPU/accelerator

---

## Build Order Summary

### First 30 Days

| Week | Focus | Key Deliverables |
|------|-------|-----------------|
| 1 | Hardware | Flash Model B, disable LTE, log GNSS + IMU, collect real drives |
| 2 | Firmware | Append-only records, recovery tooling, trip start/stop state machine |
| 3 | Sync | Trusted home Wi-Fi config, Zig upload receiver, bundle validator |
| 4 | Server | Upload at home only, PostgreSQL/PostGIS storage, basic trip list/map |

### Days 31–60

| Focus | Key Deliverables |
|-------|-----------------|
| Quality | Tune trip boundaries, GNSS quality policy, retention, power behavior |
| Product | Parking location, places, labels, GPX/GeoJSON/CSV export |
| Operations | Backups, restore procedure, local CA/device enrollment, monitoring |
| Homelab | MQTT / Home Assistant semantic events |
| Zig | Consolidate bundle parser, simulator, CLI, and ingest daemon |

### Days 61–90

| Focus | Key Deliverables |
|-------|-----------------|
| Gleam | One supervised workflow service for trip projection/reprocessing |
| MoonBit | One read-only plugin: privacy redaction or trip classification |
| Odin | Build `trip-inspector` using real data corpus |
| Mojo | Start one offline experiment: route similarity or map matching |
| Hardening | Power-loss testing, SD exhaustion, Wi-Fi loss, server restart, backup restore |

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Missed meaningful trips | 0 over a two-week test |
| Duplicate trips | 0 |
| Corrupt bundles after forced power loss | 0 |
| Home sync within 10 min of arrival | 95%+ |
| Server dedup correctness | 100% in replay tests |
| WAN dependencies | 0 |
| Data export completeness | 100% |
| Unexplained parked battery impact | None after multi-week validation |
