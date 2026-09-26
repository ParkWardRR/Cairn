# Cairn Roadmap

> Phased development plan for the Cairn offline-first car journal.
> Each phase builds on the previous. Items are checked off as they are completed.

---

## Phase 0 — Define the Contract

**Goal:** Know precisely what constitutes a trip, what gets stored, and what "sync succeeded" means before writing firmware.

**Timeline:** Week 0

### Deliverables

- [ ] **Product specification**
  - [ ] Write one-page statement of core flow
  - [ ] Document non-goals and explicit exclusions
  - [ ] Define privacy defaults and operational model
  - [ ] Specify supported vehicles and mounting considerations

- [ ] **Device state machine**
  - [ ] Define all states: `sleep`, `arming`, `recording`, `finalizing`, `awaiting_home_wifi`, `syncing`, `low_battery_protection`, `fault`
  - [ ] Document transitions between states with trigger conditions
  - [ ] Specify timeout values and debounce thresholds
  - [ ] Diagram the state machine

- [ ] **Trip bundle format**
  - [ ] Design versioned, self-describing bundle schema
  - [ ] Define CBOR manifest structure
  - [ ] Define binary sample encoding for GNSS and IMU
  - [ ] Specify checksum and signature scheme (SHA-256 + Ed25519)
  - [ ] Document zstd compression strategy (server-side; raw on device for v1)

- [ ] **Home network design**
  - [ ] Define trusted SSID/BSSID allowlist mechanism
  - [ ] Plan WPA credential provisioning (serial-first)
  - [ ] Specify local hostname/IP resolution (mDNS + static DHCP)
  - [ ] Design certificate strategy (local CA, device keypair, server pinning)

- [ ] **Data retention policy**
  - [ ] Define device spool period (7–30 day configurable window)
  - [ ] Define server retention tiers (raw, normalized, derived)
  - [ ] Specify backup schedule and restore procedure
  - [ ] Document deletion rules — never delete without server receipt + retention window

- [ ] **Test corpus**
  - [ ] Create synthetic scenario: normal 45-minute drive
  - [ ] Create synthetic scenario: short stop (gas station, drive-through)
  - [ ] Create synthetic scenario: long stop (multi-hour parking)
  - [ ] Create synthetic scenario: no GNSS fix (garage/tunnel)
  - [ ] Create synthetic scenario: interrupted write (power loss mid-trip)
  - [ ] Create synthetic scenario: interrupted upload (Wi-Fi loss mid-sync)

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

- [ ] **Flash baseline firmware**
  - [ ] Set up Arduino IDE / PlatformIO for Freematics ONE+ Model B
  - [ ] Successfully erase, flash, and serial-monitor the device
  - [ ] Document recovery procedure for bricked state
  - [ ] Verify serial debug output is reliable

- [ ] **Disable unused radios**
  - [ ] Confirm LTE modem never initializes (no SIM required)
  - [ ] Disable BLE unless needed for future provisioning
  - [ ] Verify no unintended radio activity with RF monitor

- [ ] **GNSS logger**
  - [ ] Write timestamped GNSS samples to microSD
  - [ ] Capture: latitude, longitude, altitude, speed, heading
  - [ ] Capture: fix quality, satellite count, HDOP/accuracy
  - [ ] Validate 10 Hz sample rate achievable
  - [ ] Collect first real-world drive data

- [ ] **IMU logger**
  - [ ] Capture accelerometer/gyro at controlled rate
  - [ ] Verify timestamp alignment with GNSS samples
  - [ ] Determine useful sample rate (25–50 Hz starting point)

- [ ] **Wi-Fi station mode**
  - [ ] Join trusted home SSID successfully
  - [ ] Expose local health/status endpoint over HTTP
  - [ ] Verify reconnection after power cycle

- [ ] **Power characterization**
  - [ ] Measure active driving current (GNSS + IMU + microSD write)
  - [ ] Measure GNSS-only current (no Wi-Fi, no LTE)
  - [ ] Measure Wi-Fi upload current
  - [ ] Measure deep sleep current
  - [ ] Compare against Freematics spec (~10 mA sleep claim)
  - [ ] Test in both target vehicles

- [ ] **Data durability**
  - [ ] Pull power during active write
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

- [ ] **Ignition / activity heuristic**
  - [ ] Detect drive start from GNSS speed + IMU activity
  - [ ] Use OBD port voltage only as power context (not diagnostics)
  - [ ] Validate heuristic across both target vehicles

- [ ] **Start debounce**
  - [ ] Filter out noise, door slams, and minor garage movement
  - [ ] Require sustained movement for configurable threshold (e.g., 15 seconds)
  - [ ] Test with real-world false triggers

- [ ] **Adaptive sample scheduling**
  - [ ] Active driving: 1–5 Hz GNSS, 25–50 Hz IMU
  - [ ] Slow maneuvering / parking: 1–2 Hz GNSS, 25 Hz IMU
  - [ ] Stationary stop candidate: 0.2–1 Hz GNSS, 10–25 Hz IMU
  - [ ] Parked: GNSS off, low-power motion wake only

- [ ] **Stop debounce**
  - [ ] Avoid fragmenting trips during traffic lights
  - [ ] Avoid fragmenting during fuel stops and drive-throughs
  - [ ] Configurable dwell time threshold
  - [ ] Prefer merging short errands over producing false trips

- [ ] **Parking snapshot**
  - [ ] Record final reliable location with GNSS accuracy
  - [ ] Record heading and timestamp
  - [ ] Mark as parking endpoint in trip events

- [ ] **Local event markers**
  - [ ] Emit: trip_start, trip_stop, trip_pause, trip_resumed
  - [ ] Emit: poor_gnss_quality, gnss_restored
  - [ ] Emit: power_anomaly, storage_pressure
  - [ ] Store events in trip bundle's events.cbor

- [ ] **Storage recovery**
  - [ ] Implement append-only record format with periodic checkpoints
  - [ ] Boot-time scan and recovery of incomplete sessions
  - [ ] Validate recovery after simulated power loss

- [ ] **Capacity controls**
  - [ ] Monitor microSD free space
  - [ ] Prevent exhaustion with configurable threshold
  - [ ] Preserve unsynced data according to retention policy
  - [ ] Alert on next home sync if storage was under pressure

---

## Phase 3 — Home-Only Synchronization

**Goal:** Driving data reaches the homelab only after the device joins trusted home Wi-Fi. Never over cellular. Never to the internet.

**Timeline:** Week 3

### Deliverables

- [ ] **Wi-Fi provisioning**
  - [ ] Serial-based initial configuration
  - [ ] Store trusted network credentials in device NVS
  - [ ] Plan for future captive portal or BLE provisioning

- [ ] **Trusted-network policy**
  - [ ] Only sync after association to allowlisted BSSID/SSID
  - [ ] Reject identically named but untrusted SSIDs
  - [ ] Log and alert on unexpected network association attempts

- [ ] **Device identity**
  - [ ] Generate unique Ed25519 keypair during provisioning
  - [ ] Store private key securely on device
  - [ ] Register public key with homelab server

- [ ] **Transport security**
  - [ ] Implement HTTPS with mutual TLS (mTLS)
  - [ ] Pin local CA certificate on device
  - [ ] Pin server public key / hostname
  - [ ] No public Web PKI dependency

- [ ] **Resumable upload**
  - [ ] Chunked upload using trip ID + content hash + byte offset
  - [ ] Resume after Wi-Fi disconnect or vehicle departure
  - [ ] Handle partial uploads gracefully

- [ ] **Server receipt**
  - [ ] Server issues signed/durable acknowledgment
  - [ ] Receipt ties to trip ID and content hash
  - [ ] Receipt issued only after object + database record committed

- [ ] **Device cleanup**
  - [ ] Delete local trip only after receipt + retention window
  - [ ] Store receipt locally as proof of server acknowledgment
  - [ ] Never delete merely because an HTTP request succeeded

- [ ] **Upload budget**
  - [ ] Limit Wi-Fi transmit duration per sync session
  - [ ] Suspend/resume safely if vehicle leaves garage mid-upload
  - [ ] Prioritize oldest unsynced trips

- [ ] **Local discovery**
  - [ ] mDNS (`cairn.local`) for initial deployment
  - [ ] Static DHCP reservation for production reliability

---

## Phase 4 — Zig Ingest and Data Model

**Goal:** Browse uploaded trips locally. Re-uploading the same file never duplicates data. All core services run in Zig.

**Timeline:** Weeks 3–4

### Deliverables

- [ ] **`bundle` library**
  - [ ] Parse trip bundles from device
  - [ ] Validate checksums and signatures
  - [ ] Hash and verify content integrity
  - [ ] Support schema version migration

- [ ] **`ingestd` service**
  - [ ] mTLS endpoint for device uploads
  - [ ] Resumable chunked upload support
  - [ ] Replay protection (reject duplicate content hashes)
  - [ ] Rate limiting per device
  - [ ] Issue durable receipts on successful commit

- [ ] **`tripctl` CLI**
  - [ ] Inspect local trip bundles
  - [ ] Validate storage integrity
  - [ ] Generate synthetic test fixtures
  - [ ] Export raw data (GPX, GeoJSON, CSV)

- [ ] **`trip-sim` simulator**
  - [ ] Replay historical drives into ingest pipeline
  - [ ] Generate synthetic journeys
  - [ ] Support accelerated and real-time replay

- [ ] **`api` service**
  - [ ] Read-only JSON API for trips
  - [ ] Endpoints: trips, places, route segments, tags, exports
  - [ ] PostGIS queries for nearest-place and distance

- [ ] **Database schema**
  - [ ] `devices` — public key, metadata, last-seen, firmware version
  - [ ] `uploads` — content hash, state, receipt ID, storage path
  - [ ] `trips` — stable ID, start/end, device, summary, bundle hash
  - [ ] `location_samples` — raw GNSS with accuracy and sequence
  - [ ] `motion_samples` — IMU or downsampled aggregates
  - [ ] `trip_events` — start/stop/pause/sync/quality events
  - [ ] `places` — named locations with radius
  - [ ] `trip_tags` — personal/business/road-trip/private labels
  - [ ] `derivations` — algorithm version, output hash, reproducibility
  - [ ] All timestamps in UTC; render in viewer per timezone
  - [ ] PostGIS extensions enabled

---

## Phase 5 — Local Web UI and Home Assistant

**Goal:** The project feels like the part of Automatic you actually liked — effortless trip history, remembered parking, and useful presence/arrival events.

**Timeline:** Days 31–60

### Web UI Views

- [ ] **Today view**
  - [ ] Most recent trip summary
  - [ ] Last parked location on map
  - [ ] Sync status indicator
  - [ ] Device battery/health summary

- [ ] **Trips view**
  - [ ] Time-sorted trip list
  - [ ] Map thumbnail per trip
  - [ ] Duration, distance, endpoint, tags
  - [ ] Filter by date range, tag, place

- [ ] **Trip detail view**
  - [ ] Full route on map
  - [ ] Timeline with stop candidates
  - [ ] GNSS quality overlay
  - [ ] Raw data download / export buttons

- [ ] **Places view**
  - [ ] Saved locations with configurable radius
  - [ ] Arrival/departure history
  - [ ] Rename and merge controls

- [ ] **Device view**
  - [ ] Firmware version
  - [ ] Storage usage
  - [ ] Last sync time
  - [ ] Wi-Fi state and low-power state

- [ ] **Privacy / data view**
  - [ ] Retention controls
  - [ ] Export all data
  - [ ] Delete individual trips
  - [ ] Redact start/end areas (privacy zones)

### Home Assistant Integration

- [ ] **MQTT event bus**
  - [ ] Publish `trip_started` event
  - [ ] Publish `trip_ended` event with distance/duration
  - [ ] Publish `arrived_home` / `departed_home`
  - [ ] Publish `sync_completed`
  - [ ] Publish `last_parked` location

- [ ] **Resilience**
  - [ ] Persist events for replay if HA is unavailable
  - [ ] Never make HA availability a reason for trip failure
  - [ ] Semantic events only — no raw GPS stream

---

## Phase 6 — Introduce Gleam Deliberately

**Goal:** Gleam improves the system's event semantics without becoming a second ingestion path. Build only after Zig ingest and the database are stable.

**Timeline:** Days 61–75

### Deliverables

- [ ] **Trip lifecycle projector**
  - [ ] Convert raw/derived state into coherent trip lifecycle events
  - [ ] Maintain event consistency across reprocessing

- [ ] **Retry scheduler**
  - [ ] Reprocess trips when algorithms improve
  - [ ] Idempotent reprocessing with version tracking

- [ ] **Automation executor**
  - [ ] Apply rules: "on arrival home, update HA presence"
  - [ ] Configurable rule engine for user-defined automations

- [ ] **Notification policy**
  - [ ] Evaluate whether sync/device/storage issues deserve notification
  - [ ] Configurable severity thresholds

- [ ] **Data-quality queue**
  - [ ] Flag impossible GNSS jumps
  - [ ] Flag prolonged GPS loss
  - [ ] Detect duplicate tracks and clock drift

- [ ] **OTP supervision**
  - [ ] Fault-tolerant supervision tree
  - [ ] Independent restart of failing background tasks
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
