# Cairn Roadmap

> Phased development plan for the Cairn offline-first car journal.
> Each phase builds on the previous. Items are checked off as they are completed.

---

# Cairn v2 — Rebuilding the Core Data Path

**Current work.** Two architecture reviews concluded that v1's design is sound
but only correct on the happy path, and asked for the system to become
*auditable across failures*. Cross-checking the reviews against the code found
three of their recommendations were live defects rather than future concerns:

| Defect | Evidence |
|---|---|
| Only `samples.bin` is uploaded — OBD, IMU-summary, health and event data is silently discarded on every sync | `firmware/.../state_machine.cpp:1184`; the server falls back to `ParseRawSamples` |
| Pruning is not receipt-gated. Retention is measured with `millis()`, which resets on every deep sleep; the Ed25519 signature is never verified and the receipt is never persisted | `state_machine.cpp:510`, `:1312` |
| Transport is plain HTTP on port 8080, not the mTLS on 8443 that the docs claim | `config.h:69` |

Rather than remediate in place, the core data path is being **rebuilt from a
clean slate**: no format compatibility, no migration, no reissuing of historical
receipts. The v1 phases further down this document are retained as history.

## Guiding invariants

1. **A sealed bundle is never mutated.** It is either locally recoverable,
   remotely receipt-confirmed, or both.
2. **No byte is deleted without a locally verified signed receipt.** Time,
   storage pressure and operator impatience are all insufficient justification.
3. **Ordering truth is `(boot_id, monotonic_seq)`, never wall-clock UTC.** GNSS
   time jumps; UTC is an annotation with an uncertainty, not an index.
4. **Honest incompleteness beats fabricated continuity.** A trip with a marked
   GNSS gap is useful; a route interpolated from stale fixes is not.

## Decisions

| Decision | Choice |
|---|---|
| Firmware | ESP-IDF application with `arduino-esp32` as a component, keeping the vendored FreematicsPlus drivers. Rebuild the application, not the hardware access — this repo already contains one failed custom HAL |
| Server | Go, in a top-level `server/` directory (the old `zig/ingest/` contains no Zig) |
| Rebuilt | Firmware, ingest server, database schema, bundle format, emulator |
| Kept | SvelteKit web UI, Odin tools, MoonBit plugins, Gleam orchestrator |
| Stack shape | Core data path is C and Go only; Odin, MoonBit and Gleam remain optional side tooling that must be able to break without affecting a drive |

Target hardware is **classic ESP32** (xtensa LX6, WROVER with PSRAM), not an
S3. There is no secure element, so flash encryption plus a key in NVS is the
accepted ceiling for the device signing key.

## Build order

Server and emulator first, so the device targets a server already proven
against the fault matrix.

### Phase 1 — Bundle format v2 — **complete**

- [x] `docs/bundle-format-v2.md` as a normative spec with byte layouts
- [x] Go reference implementation (`server/format/`): encode, decode, verify, recover
- [x] Framed records: `boot_id`, monotonic `seq`, CRC-32, `prev_crc32` chain
- [x] Deterministic-CBOR manifest, Ed25519-signed over a specified encoding
- [x] Three distinct identifiers: `bundle_id` (ULID), `content_root` (Merkle root
      over members), `transfer_hash` (transport integrity only)
- [x] Nine payload schemas carrying fix type, HDOP, accuracy, satellites and
      source flags — never inferring precision the receiver did not report
- [x] UTC as an estimate with its own uncertainty, separate from the ordering key
- [x] Explicit `GNSS_GAP` record so absence is recorded, never interpolated
- [x] 20 conformance vectors in `fixtures/format-v2/`, generated deterministically
- [x] Conformance runner; 60 test cases green

### Phase 2 — Server: raw-first ingest

- [ ] mTLS listener on 8443: private CA, per-device client certificates, denylist
- [ ] Manifest-first upload; server replies with missing chunk ranges only
- [ ] Content-addressed chunk acceptance — by hash, never by byte offset
- [ ] Content-addressed raw store on ZFS, kept object-like for a later MinIO swap
- [ ] Durable signed receipt, persisted **before** it is returned
- [ ] Persistent signing key; refuse to start with an ephemeral key outside dev mode
- [ ] Idempotency keyed on `content_root`, not on connection or request ID
- [ ] Durable ingest outbox; ingest returns once the raw commit is durable
- [ ] Per-device rate limits and storage quotas

### Phase 3 — Emulator and fault injection

- [ ] Rebuild around a fault-injection layer: interrupt at a named step, a byte
      offset, or a seeded random point
- [ ] Rust implementation of format v2, passing the committed vectors
- [ ] Power cut at each storage write step
- [ ] Network loss at each upload chunk
- [ ] Reboot between receipt and prune
- [ ] Corrupt chunk, duplicate upload, server crash during commit
- [ ] Clock reset / GNSS jump; corrupted storage bytes
- [ ] Decoder upgrade reproducibility; plugin timeout; storage full
- [ ] Reproducible from a seed, printed on failure; assertions on the journal and
      database, never stdout
- [ ] **Standing rule: every later phase adds its own matrix rows before it closes**

### Phase 4 — Schema and decode workers

- [ ] Fresh migrations — raw / normalized / derived layers
- [ ] Range-partition high-frequency sample tables by event time
- [ ] Unique `(device_id, content_root)` and unique `bundle_id`
- [ ] Decode, normalize and enrich workers consuming the outbox
- [ ] Reprocessing as a queued job, not a synchronous endpoint
- [ ] MQTT publishes semantic idempotent state only — never raw samples

### Phase 5 — Firmware: ESP-IDF skeleton

- [ ] ESP-IDF project with `arduino-esp32` as a component
- [ ] **Partition table with A/B OTA slots from the first commit**, replacing `huge_app.csv`
- [ ] NVS for `boot_id`, policy version and the flash error journal
- [ ] Flash encryption; device signing key provisioned into NVS
- [ ] Sensor tasks that only emit facts to a queue
- [ ] Boot self-test; real `esp_sleep_get_wakeup_cause()`; pulls on wake pins

### Phase 6 — Firmware: capture lifecycle

- [ ] Three independently persisted regions: capture, bundle, connectivity
- [ ] A single transition controller owns all state; tasks submit facts
- [ ] Durable journal per transition with trigger, reason code and policy version
- [ ] Confidence-scored evidence with start/stop hysteresis
- [ ] 45 s pre-roll buffer written as `pretrip` samples on confirmation

### Phase 7 — Firmware: framed storage and recovery

- [ ] Append-only framed segments per format v2, rotated at a bounded size
- [ ] Atomic seal: temp file, sync, rename to `.sealed`
- [ ] C implementation of format v2, passing the committed vectors
- [ ] Boot recovery runs automatically, not only from a CLI tool
- [ ] SD write and recovery error counts surfaced as device health

### Phase 8 — Firmware: sync and receipt-gated prune

- [ ] mTLS with the private CA pinned on-device
- [ ] Resume by content hash, never byte offset
- [ ] Verify the receipt signature **and** that its `content_root` matches
- [ ] Receipts persisted outside the bundle directory, retained longer than payloads
- [ ] Retention watermark persisted — never `millis()`
- [ ] Transactional prune: `prune_intent` → delete → `prune_complete`, replayed at boot
- [ ] **Hard invariant with a test: no verified receipt ⇒ never pruned**

### Phase 9 — Degraded states and policy tuning

- [ ] `DEGRADED_GNSS` / `_STORAGE` / `_TIME` / `_NETWORK`, `LOW_POWER`, `RECOVERY_REQUIRED`
- [ ] Event-adaptive sampling across GNSS, IMU and OBD
- [ ] Thresholds tuned on real traces, with the policy version journalled

### Phase 10 — Ledger and documentation

- [ ] Admin ledger over the full bundle lifecycle, every transition with a reason code
- [ ] Rewrite README and the docs against what v2 actually does
- [ ] Every documented guarantee has a corresponding row in the Phase 3 matrix

### Phase 11 — Secure OTA

- [ ] Signed images verified before swap; post-boot self-test; automatic rollback
- [ ] Update only while parked and externally powered, never with unreceipted bundles pending

## Deferred deliberately

| Deferred | Reason |
|---|---|
| Arbitrary user plugins | Until the raw format, decoder ABI, receipt semantics and replay tooling are stable |
| MinIO | A CAS directory on ZFS is sufficient; keep the abstraction, skip the daemon |
| TimescaleDB | Native range partitioning first; adopt only on measured need |
| `previous_bundle_root` enforcement | Field is populated in v2; detecting deleted historical bundles is a different threat model from detecting corruption |

---

# v1 History

The phases below are the original v1 plan. They are retained as history; the
v2 rebuild above supersedes Phases 2 through 4 outright and reshapes the rest.

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
  manifest.json       # Trip metadata, device info, VIN, DTC codes, schema v2
  samples.bin         # GNSS samples — 32 bytes each, little-endian
  imu_summary.bin     # IMU summaries — 24 bytes each, rolling windows
  obd.bin             # OBD-II snapshots — 20 bytes each (speed, RPM, throttle, etc.)
  health.bin          # Device health — 16 bytes each (battery, temp, RSSI)
  events.json         # Start/stop/pause/quality/ECU-off/thermal markers
  sha256sums.txt      # Per-file content hashes (ESP32 hardware-accelerated)
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

**Goal:** Prove the Freematics ONE+ Model B can produce reliable local GNSS/IMU/OBD logs and reconnect to home Wi-Fi — without LTE or a vendor cloud.

**Timeline:** Week 1

### Deliverables

- [x] **Flash baseline firmware**
  - [x] Set up PlatformIO for Freematics ONE+ Model B (esp-wrover-kit)
  - [x] Vendor FreematicsPlus library (19 files) + TinyGPS (2 files) — proven hardware drivers
  - [x] Replace broken custom HAL with vendor library for all peripherals
  - [x] Successfully erase, flash, and serial-monitor the device *(flashed repeatedly over USB from a Raspberry Pi flash station; esptool hash-verified, boot captured at 115200)*
  - [ ] Document recovery procedure for bricked state

- [x] **Disable unused radios**
  - [x] Wi-Fi radio powered off during recording
  - [x] BLE disabled at build time (ENABLE_BLE=0), code fully wired for future enable
  - [ ] Confirm LTE modem never initializes (no SIM required)

- [x] **OBD-II engine data** *(harvested from stock firmware)*
  - [x] Tiered PID polling: speed, RPM, throttle, engine load (every cycle); coolant, intake temp, fuel pressure, timing advance (round-robin)
  - [x] Car shutdown detection via ECU-off (3 consecutive OBD errors)
  - [x] VIN retrieval and DTC code reading at startup
  - [x] OBDSnapshot struct (20 bytes packed) logged to obd.bin

- [x] **GNSS logger**
  - [x] Write timestamped GNSS samples to microSD via SPI SD
  - [x] Capture: latitude, longitude, altitude, speed, heading
  - [x] Capture: fix quality, satellite count, HDOP/accuracy
  - [x] NMEA parsing via TinyGPS (vendor library)
  - [x] GNSS watchdog — reset module after 300s with no fix
  - [ ] Validate 10 Hz sample rate achievable
  - [ ] Collect first real-world drive data

- [x] **IMU logger**
  - [x] Capture accelerometer/gyro via I2C from ICM-42627
  - [x] Accelerometer bias calibration (1s sampling at startup and before standby)
  - [x] Impact/hard-brake/sharp-turn detection from accel/gyro
  - [ ] Verify timestamp alignment with GNSS samples
  - [ ] Determine useful sample rate (25–50 Hz starting point)

- [x] **Wi-Fi station mode**
  - [x] WiFi credentials stored in NVS (provisioned via BLE or compile-time)
  - [x] WiFi.begin() with NVS-stored SSID/password
  - [x] RSSI logging and connectivity monitoring
  - [x] Verify association after power cycle *(joins home AP on every cold boot; -62 dBm on ch11)*
  - [x] Bench self-test build (`env:freematics-selftest`) — 2.4 GHz scan, DNS resolution, server health fetch over serial
  - [ ] Expose local health/status endpoint over HTTP

- [x] **Power and standby** *(harvested from stock firmware)*
  - [x] Battery voltage via devType-based reading (ATRV for devType<=12, analogRead for devType>12)
  - [x] Device temperature monitoring from IMU die temp sensor
  - [x] Standby: OBD coprocessor ATLP sleep + IMU bias-calibrated motion wake
  - [x] Voltage jumpstart detection (>14V = engine cranking, wake from standby)
  - [x] Thermal throttle protection (>75°C)
  - [x] DeviceHealth struct (16 bytes packed) logged to health.bin
  - [x] Adaptive data intervals (1Hz moving → 0.5Hz at 10s still → 0.2Hz at 60s → trip end at 180s)
  - [ ] Measure active driving current (GNSS + IMU + OBD + microSD write)
  - [ ] Measure standby current
  - [ ] Test in both target vehicles

- [x] **Data durability (firmware)**
  - [x] fsync after every trip finalization for crash consistency
  - [x] Hardware SHA-256 checksums via mbedtls (ESP32 accelerated)
  - [ ] Pull power during active write (hardware test)
  - [ ] Verify previously finalized data survives
  - [ ] Verify partial write is detectable/recoverable

- [ ] **RF validation**
  - [ ] Compare route quality in vehicle 1
  - [ ] Compare route quality in vehicle 2
  - [ ] Determine if external GNSS antenna or OBD extension cable is needed
  - [ ] Document OBD port location impact on GNSS reception

- [ ] **External antenna & accessory support (exploration)** — see `docs/hardware-accessories.md`
  - [ ] Bench-test a cheap external L1 active antenna against the onboard ceramic antenna
  - [ ] Evaluate salvage/donor GPS antennas as a low-cost alternative to buying new
  - [ ] Determine if the M9/M10 module exposes an antenna feed, or if a module swap is required
  - [ ] Explore a secondary/redundant GNSS receiver for A/B RF comparison and dropout failover
  - [ ] Decide whether any of the above graduates from bench tooling into shipped firmware/hardware

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
  - [x] ~~mDNS (`cairn.local`) for initial deployment~~ — **abandoned.** The ESP32
        resolver does not do mDNS, so `cairn.local` never resolved from the
        device. The server is now reached by a name the router's DNS serves.
  - [x] Server host configurable per-deployment via untracked `secrets.h`
  - [ ] Static DHCP reservation for production reliability *(the server's lease
        has already moved twice, which is why the firmware uses a DNS name
        rather than a hardcoded IP)*

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
  - [x] Publish `trip_started` event
  - [x] Publish `trip_ended` event with distance/duration
  - [x] Publish `arrived_home` / `departed_home`
  - [x] Publish `sync_completed`
  - [x] Publish `last_parked` location

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

- [x] **Design narrow plugin interface**
  - [x] Define versioned input/output schemas
  - [x] Deterministic execution — same input always produces same output
  - [x] Host validates all plugin results
  - [ ] Record plugin version/hash for every derivation

### Plugins

- [x] **Trip classifier**
  - [x] Input: normalized trip summary + sampled route/motion
  - [x] Output: labels (commute, canyon drive, errand, road trip, unknown)

- [x] **Privacy redactor**
  - [x] Input: route + configured privacy zones
  - [x] Output: redacted route/endpoint geometry

- [x] **Export transformer**
  - [x] Input: trip model
  - [x] Output: GPX, GeoJSON, CSV, Markdown trip report

- [x] **Route scorer**
  - [x] Input: polyline + places
  - [x] Output: favorite-road / repeat-route score

- [x] **Data-quality detector**
  - [x] Input: timestamped samples
  - [x] Output: anomaly annotations

### Sandbox Constraints

- [x] No database access
- [x] No network access
- [x] No filesystem access
- [x] No authority to alter raw data
- [x] Structured input → structured output only

---

## Phase 8 — Odin Native Tools

**Goal:** An enjoyable, high-performance local utility for debugging and exploring recorded journeys. One focused tool, not another platform.

**Timeline:** Days 75–90

### Deliverables

- [x] **`trip-inspector`**
  - [x] Open local trip bundles
  - [x] Inspect metadata, sample timing, GNSS accuracy
  - [x] Visualize speed and IMU data

- [x] **`trip-diff`**
  - [x] Compare device raw route vs. server-normalized route
  - [x] Highlight divergence points

- [x] **`trip-replay`**
  - [x] Feed recorded trips into test server
  - [x] Support realistic and accelerated timing

- [x] **`route-density`**
  - [x] Generate heatmap of frequently driven roads
  - [x] Local image/vector output

- [x] **`sd-recover`**
  - [x] Scan pulled microSD card for incomplete trip files
  - [x] Rebuild recoverable trip data

---

## Phase 9 — Rust Trajectory Experiments

**Goal:** Offline trajectory analysis experiments in Rust. Explore route clustering, place discovery, and driving patterns without making the product depend on it. Start only after several months of clean local data.

**Timeline:** Days 90+

### Experiments

- [x] **Route similarity** (`cairn-trajectory similarity`)
  - [x] Cluster trips: "These 14 trips are effectively the same commute"
  - [x] Output candidate route groups for manual review
  - [x] Hausdorff distance with subsampled polylines

- [x] **Automatic place discovery** (`cairn-trajectory places`)
  - [x] Identify candidate recurring endpoints
  - [x] DBSCAN-style clustering with configurable radius
  - [ ] Subject to privacy review before surfacing

- [x] **Stop/errand segmentation** (`cairn-trajectory segments`)
  - [x] Identify meaningful stops in long routes
  - [x] Drive/stop phase segmentation with configurable thresholds
  - [ ] Compare against device-side stop detection

- [ ] **GNSS smoothing / map-matching evaluation**
  - [ ] Compare algorithms against manually checked routes
  - [ ] Benchmark accuracy and performance

- [x] **Driving-style visualization** (`cairn-trajectory density`)
  - [x] Cluster acceleration/turning patterns
  - [x] Acceleration and turn-rate histograms with percentile stats
  - [x] Personal exploration only — not scoring

- [x] **Anomaly detection** (`cairn-trajectory anomalies`)
  - [x] Flag routes/speed traces that indicate bad GNSS
  - [x] Detect impossible jumps, GPS loss, clock drift, HDOP spikes
  - [x] Distinguish sensor error from real driving behavior

### Guardrails

- [x] Authoritative output stays in deterministic Zig/Gleam paths
- [x] Trajectory tool proposes annotations; never decides deletion or trip boundaries
- [x] No safety-critical actions
- [x] All outputs reproducible without GPU/accelerator

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
| Rust trajectory | Route similarity, place discovery, anomaly detection, driving style |
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
