# Cairn Roadmap

> **v2 is built, deployed and running on the car's dongle. The work now is
> turning what it records into answers.** Phases 1–12 (the rebuild) are complete
> and kept below as the record; Phases 13–18 are what is ahead. The v1 plan is
> dead and lives in [docs/archive/roadmap-v1.md](docs/archive/roadmap-v1.md) —
> nothing in it is scheduled.

Last reviewed 2026-10-03.

## Where Cairn is

| Layer | State |
|---|---|
| Bundle format v2 | **Complete.** Three implementations (Go, Rust, firmware C) agree byte-for-byte on 25 conformance vectors |
| Firmware | **Running on hardware.** Capture → seal → offer → commit → receipt → verified prune; mTLS on 8443; standby with wake-on-motion; secure OTA written, install unexercised |
| Ingest server | **Deployed** as a hardened systemd unit on the Cairn VM, mutual TLS, receipt-gated |
| Decode pipeline | **Complete.** Idempotent and reproducible; PostgreSQL/PostGIS derived layer (not currently running on the VM) |
| Engine telemetry | **Recording** boost, MAP, mixture and fuel trims as `OBD_EXTENDED`; PID support on the real car still being established (Phase 13) |
| Analytical store | **Deployed 2026-10-03.** `cairn-tsdb`, an in-memory DuckDB rebuilt from the CAS and the SD card on every start; 18 bundles load in ~60 ms and every one reproduces (Phase 14) |
| Web UI | SvelteKit app exists; reads the PostgreSQL layer, not the analytical store (Phase 16) |
| Parked current draw | **Unmeasured.** Needs a meter, not a terminal. Still the single most useful measurement left |

## Guiding invariants

1. **A sealed bundle is never mutated.** It is either locally recoverable,
   remotely receipt-confirmed, or both.
2. **No byte is deleted without a locally verified signed receipt.** Time,
   storage pressure and operator impatience are all insufficient justification.
3. **Ordering truth is `(boot_id, monotonic_seq)`, never wall-clock UTC.** GNSS
   time jumps; UTC is an annotation with an uncertainty, not an index.
4. **Honest incompleteness beats fabricated continuity.** A trip with a marked
   GNSS gap is useful; a route interpolated from stale fixes is not.
5. **Anything derived is disposable and must prove it reproduces.** Raw bundles
   are authoritative. A derived store — PostgreSQL or the in-memory one — is
   rebuilt from them, carries the digest that shows the rebuild matched, and is
   not served when it does not.

## Decisions

| Decision | Choice |
|---|---|
| Firmware | ESP-IDF application with `arduino-esp32` as a component, keeping the vendored FreematicsPlus drivers. Rebuild the application, not the hardware access — this repo already contains one failed custom HAL |
| Server | Go, in a top-level `server/` directory |
| Rebuilt | Firmware, ingest server, database schema, bundle format, emulator |
| Kept | SvelteKit web UI, Odin tools, MoonBit plugins, Gleam orchestrator |
| Stack shape | Core data path is C and Go only; Odin, MoonBit and Gleam remain optional side tooling that must be able to break without affecting a drive |
| Analytical store | **In-memory DuckDB**, behind a small Go service. Chosen because the data is tiny (hundreds of KB today) and the useful questions are ASOF joins across streams polled at different rates — OBD against boost against GNSS. InfluxDB 3 was rejected as UTC-keyed and, as far as was checked, without ASOF; QuestDB as a JVM that is memory-mapped rather than in-memory, on a 7.3 GB box shared with the runner and ingest stack |
| Analytical time key | `(boot_id, mono_ms)`, never UTC. UTC rides along as a column |
| v1 | **Dead.** No reader, no migration, no compatibility. Tools that scan an SD card ignore `trips/` and load only sealed v2 bundles from `bundles/` |

Target hardware is **classic ESP32** (xtensa LX6, WROVER with PSRAM), not an
S3. There is no secure element, so flash encryption plus a key in NVS is the
accepted ceiling for the device signing key.

The car: an N20 428i on a BM3 Stage 1 tune, ~E41. For this car fuel trims
and mixture say more about engine health than boost does, which is why Phase 15
leads with them.

---

# Ahead

## Phase 13 — Engine telemetry on the real car — **in progress**

The recording side. Shipped 2026-10-02/03: boost, mixture and fuel trims as
`OBD_EXTENDED`; a PID validation firmware build (`pidtest`) that probes what the
ECU actually answers; lambda scale and raw 0x43/0x44 reads corrected; the
255 kPa MAP ceiling flagged; a fuel-type and ethanol-content probe; GNSS
reading-freshness reporting; standby windows recorded so parked draw is
measurable; parked mode silent on the vehicle bus.

- [x] Boost, MAP, baro, MAF, lambda, STFT/LTFT recorded and decoded
- [x] MAP saturation (`MAPSaturated`) surfaced rather than reported as a boost
      reading — a sensor pinned at its ceiling is not a measurement
- [x] Journal sequence range no longer leaks into the capture's, and the drain
      rate is guarded
- [ ] **Establish which PIDs this ECU really answers.** `BOOST_CONTROL` (0x70)
      reports `support=1` yet returns `NO DATA` in the validation-build log, so
      "supported" and "answers" are not the same thing; record the supported set for the
      car in `docs/` so the decoder and the views know what to expect
- [ ] Ethanol content: confirm whether the ECU answers 0x51/0x52 at all. If not,
      the ~E41 figure has to come from fuel-trim behaviour (Phase 15), not a PID
- [ ] Park-draw measurement with a meter, using the recorded standby windows as
      the timeline

## Phase 14 — In-memory analytical store — **in progress**

`server/internal/tsdb`, `server/cmd/cairn-tsdb`, `deploy/systemd/cairn-tsdb.service`.
A derived view: the CAS and the SD card are copied into a private scratch CAS,
each bundle is decoded **twice** through the production decoder, loaded into an
in-memory DuckDB, and reconciled row for row against what the decoder produced.
Nothing is persisted; a restart rebuilds.

- [x] Loads committed server bundles (offer + receipt) and sealed v2 bundles
      from the card, deduplicated on `content_root`
- [x] Reproducibility gate: second decode must match the first digest, and
      per-bundle row counts are re-read from the database; a failing build is
      not served
- [x] Scratch CAS rather than opening the live one — `cas.Open` clears `tmp/`,
      which in the server's data directory holds an upload in flight
- [x] Keyed on `(boot_id, mono_ms)`; `v_telemetry` ASOF-joins boost and GNSS onto
      each OBD poll and exposes `boost_age_ms` / `gnss_age_ms`
- [x] Locked down: single read-only statement per request, always-rolled-back
      transaction, external file/network access disabled, configuration frozen,
      loopback bind
- [x] Deployed on the VM: `MemoryMax=3G`, DuckDB capped at 2 GB, hardened unit
- [x] Measured on the VM at 17.5M synthetic rows: scans 1–20 ms, a double ASOF
      join across 5M anchors ~0.72 s. Real data today is a few thousand rows, so
      this is headroom, not a requirement
- [x] v1 explicitly ignored and documented as dead
- [x] **Reproducibility in CI.** `go test ./...` in `build-server` already runs
      the store's tests; the runner is native on the VM, where gcc is present, so
      cgo builds there. The tests drive synthetic bundles laid out as an SD card
      through the real snapshot, decode and load path, and also cover: v1 `trips/`
      ignored, a member failing its manifest digest refused, and dedup on
      `content_root`
- [x] **Golden digests.** `output_digest` pinned for two synthetic bundles (one
      with a GNSS gap and fix-less samples) at decoder Version 1, so a decoder
      change that alters output fails a test instead of silently shifting every
      number. Mutation-checked: bumping `Version` fails both. A deliberate bump
      updates the pins in the same commit
- [x] **Reload on commit** — `-watch 5s` polls `receipts/` (a receipt is the
      commit point) and the card mirror, and rebuilds once a change has held still
      for a full interval. A rebuild that fails or does not reproduce leaves the
      previous store serving. Verified live: adding and removing a bundle each
      triggered one rebuild with no `/reload`
- [ ] **Parity with PostgreSQL.** Decode one bundle set into both layers and
      assert equal row counts per table. Proves the two derived views agree.
      Needs Postgres running, which it currently is not on the VM (containers do
      not auto-start)
- [x] `/metrics` — build time, bundle and row counts, reproduced count,
      problems, decoder version, Go heap (DuckDB's native memory is bounded by
      `-memory` and the unit's `MemoryMax`, not reported here)
- [x] Mirror of the card: `deploy/tsdb-mirror.sh <user@host> [card-dir]` — additive
      rsync of `bundles/` as the service user, then reload, then the reproduced
      counts; a build that does not reproduce fails the script
- [ ] **Authentication before any non-loopback bind.** Reuse the private CA and
      mTLS. Until then the endpoint stays on `127.0.0.1` because it carries GNSS
      positions and runs caller-supplied SQL

**Persisting the store is deliberately out of scope.** Volatility is the design.
The trigger to reconsider is a measured one: if a rebuild exceeds ~5 s.

## Phase 15 — Analysis views for the car — **planned**

Views in `internal/tsdb/schema.go`, not code, each with a test on a synthetic
bundle whose answer is known in advance.

- [ ] `v_trim_map` — STFT/LTFT binned by load and RPM. For an ethanol blend the
      long-term trim is the honest signal; this is the view that estimates drift
      from the ~E41 baseline
- [ ] `v_pulls` — detect wide-open-throttle pulls (throttle high, RPM rising) and
      summarise each: RPM range, peak boost, boost at fixed RPM bands, lambda
      under load, trims during the pull
- [ ] `v_boost_curve` — boost against RPM across pulls, excluding rows whose
      boost reading is stale or MAP-saturated
- [ ] `v_speed_agreement` — OBD speed against GNSS speed, per drive; a persistent
      ratio is a tyre-size or speedometer error, a growing one is a sensor
- [ ] `v_drive_summary` — per boot: duration, distance, max speed, gaps, warnings
- [ ] Every view filters on the `*_age_ms` columns it relies on and says so —
      an ASOF join always finds *something*, so staleness has to be an explicit
      predicate, never a default

## Phase 16 — Surfacing it — **planned**

- [ ] SvelteKit reads the store through its own server side, never from the
      browser, and through **canned endpoints** rather than arbitrary SQL
- [ ] Cards for the Phase 15 views; each shows its sample count and the age
      threshold it applied
- [ ] Decide which reads move off PostgreSQL. Rule: PostgreSQL keeps what needs
      PostGIS and durable derived state (trips, geometry); the in-memory store
      takes aggregate and cross-stream reads. Writes to either remain the
      decoder's alone

## Phase 17 — Close out Phase 9 with real traces — **planned**

Phase 9's two open items were blocked on bench data by design. Real drives and
the analytical store remove the block.

- [ ] Tune start/stop thresholds against recorded `transition` rows: query the
      scores around each journalled transition for false starts and late stops,
      then change the policy and **bump the policy version** so a decision in the
      data stays explainable
- [ ] Event-adaptive sampling across GNSS, IMU and OBD, once the traces show
      which events matter and what rate they need

## Phase 18 — Hardening — **planned**

- [ ] Plugin timeout and storage-full matrix rows (open since Phase 3)
- [ ] Exercise the OTA install on the device; it has only been verified on the
      host
- [ ] Decide on flash encryption and secure boot once the hardware is no longer
      being reflashed weekly — both are irreversible, which is why they are off

## Deferred deliberately

| Deferred | Reason |
|---|---|
| A v1 reader or migration | v1 is dead. Not deferred — refused |
| Persisting the analytical store | Rebuild is ~60 ms. Revisit only if a rebuild exceeds ~5 s |
| InfluxDB / QuestDB | Evaluated and rejected for this data and this VM; see Decisions |
| Arbitrary user plugins | Until the raw format, decoder ABI, receipt semantics and replay tooling are stable |
| MinIO | A CAS directory on ZFS is sufficient; keep the abstraction, skip the daemon |
| TimescaleDB | Native range partitioning first; adopt only on measured need. Analytical reads go to the in-memory store, which lowers that need further |
| `previous_bundle_root` enforcement | Field is populated in v2; detecting deleted historical bundles is a different threat model from detecting corruption |
| ESP32 deep sleep | Not possible on this board: the IMU interrupt is not routed to an RTC-capable GPIO, so there is no wake-on-motion source. The official Freematics firmware polls for the same reason. Timer-wake deep sleep would reset on every wake, and this firmware's boot mounts the card and runs a recovery scan — more costly than the polling it would replace. Standby instead powers peripherals down, clocks the CPU to 80 MHz and light-sleeps between polls |

---

# Completed — Cairn v2 core data path (Phases 1–12)

## Why it was rebuilt

**Complete, and running on hardware as of 2026-10-01.** Two architecture reviews
concluded that v1's design is sound but only correct on the happy path, and
asked for the system to become *auditable across failures*. Cross-checking the
reviews against the code found three of their recommendations were live defects
rather than future concerns:

| Defect | Evidence |
|---|---|
| Only `samples.bin` is uploaded — OBD, IMU-summary, health and event data is silently discarded on every sync | `firmware/.../state_machine.cpp:1184`; the server falls back to `ParseRawSamples` |
| Pruning is not receipt-gated. Retention is measured with `millis()`, which resets on every deep sleep; the Ed25519 signature is never verified and the receipt is never persisted | `state_machine.cpp:510`, `:1312` |
| Transport is plain HTTP on port 8080, not the mTLS on 8443 that the docs claim | `config.h:69` |

Rather than remediate in place, the core data path is being **rebuilt from a
clean slate**: no format compatibility, no migration, no reissuing of historical
receipts. The v1 plan is dead and archived in
[docs/archive/roadmap-v1.md](docs/archive/roadmap-v1.md).

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
- [x] 25 conformance vectors in `fixtures/format-v2/`, generated deterministically
- [x] Conformance runner; 60 test cases green

### Phase 2 — Server: raw-first ingest — **complete**

- [x] mTLS listener on 8443: private CA, per-device client certificates, denylist
- [x] Manifest-first upload; server replies with missing chunk indices only
- [x] Content-addressed chunk acceptance — by hash, never by byte offset
- [x] Content-addressed raw store, kept object-like for a later MinIO swap
- [x] Durable signed receipt, persisted **before** it is returned
- [x] Persistent signing key; refuses to start with an ephemeral key outside dev mode
- [x] Idempotency keyed on `content_root`, not on connection or request ID
- [x] Durable ingest outbox; ingest returns once the raw commit is durable
- [x] Per-device rate limits and storage quotas
- [x] Transport identity bound to the manifest's claimed device identity
- [x] Revocation effective immediately, with no restart
- [x] Verified end to end over real mTLS with `cmd/cairn-syncdemo`

### Phase 3 — Emulator and fault injection — **complete**

- [x] Rust implementation of format v2 (`emulator/src/format/`), **22/22
      committed vectors pass** — byte-identical to the Go reference, including
      the strict manifest canonical-encoding digest
- [x] Fault-injection layer with named interrupt points (`emulator/src/v2/fault.rs`)
- [x] Device lifecycle model: framed capture, atomic seal, boot recovery,
      manifest-first sync, local receipt verification, transactional prune
- [x] Power cut at each capture frame boundary and each sealing step
- [x] Torn write: a partial frame on the medium, at three truncation depths
- [x] Network loss after each upload chunk boundary
- [x] Reboot after receipt, after prune intent, and after payload delete
- [x] Corrupt chunk in transit; duplicate upload; receipt lost in transit
- [x] Clock reset / GNSS jump; corrupted storage bytes
- [x] Server crash during commit, including a signing-key rotation case
      (`tests/server-crash-during-commit.sh`)
- [x] Reproducible from a seed, printed on failure; assertions against the
      device's durable state and the server's reported state, never stdout
- [x] **16 matrix rows pass, 0 skipped.** CI gates conformance and the matrix
- [x] Decoder upgrade reproducibility — closed by Phase 4's decode pipeline
- [ ] Plugin timeout; storage full — these need Phase 9's degraded modes and the
      plugin queue to exist
- [x] **Standing rule: every later phase adds its own matrix rows before it closes**

#### What the matrix proves

| Row | Property |
|---|---|
| `power-cut-during-capture` | Every frame durably written is recoverable; only an incomplete tail is lost, and the bundle is not marked sealed |
| `power-cut-during-seal` | A sealing failure never discards captured data — all 25 capture frames and 4 journal frames survive a cut at each step |
| `torn-tail` | A partial frame is isolated; the 9 preceding frames survive and the discarded byte count is exact |
| `corrupted-storage-bytes` | A single flipped bit is detected at its own frame; earlier frames remain usable |
| `clock-reset-gnss-jump` | A 30 s backwards UTC jump leaves all frames in sequence, flagged `ESTIMATED_UTC` with accuracy unknown, monotonic time still advancing |
| `prune-without-receipt` | No verified receipt ⇒ never pruned; the payload stays fully intact |
| `reboot-after-receipt-before-prune` | Payload and receipt both survive; the prune simply happens later |
| `reboot-mid-prune` | Fully present or fully pruned; the journal explains which, and the receipt outlives the payload |
| `network-loss-per-chunk` | The device resumes rather than restarts and eventually earns a receipt |
| `corrupt-chunk-in-transit` | Rejected by the server; the retry succeeds with no operator action |
| `duplicate-upload` | Same receipt, zero chunks re-sent, decode backlog unchanged |
| `receipt-lost-in-transit` | A retry returns the already-committed receipt rather than minting a second |
| server crash during commit | Un-receipted or durably recoverable; a retry always converges to exactly one receipt |
| signing-key rotation | A rotated key cannot induce a prune — the device refuses the receipt |

#### What the harness found

| Finding | Significance |
|---|---|
| A clock-jump test that edited a frame after the fact was rejected as `CHAIN_BREAK` | The `prev_crc32` chain works. A jumped sample must be *written during capture*, not patched in afterwards |
| The crash test, run with `-dev`, lost receipt verification across a restart | The ephemeral signing key had rotated — independently reproducing the hazard `receipts.Open` refuses by default. It became a deliberate matrix case |
| A `FaultPoint` variant declared but never constructed | A missing matrix row (the window between verifying a receipt and beginning the prune), not dead code |

### Phase 4 — Schema and decode workers — **complete**

- [x] Fresh migrations in `deploy/migrations/v2/` — raw / normalized / derived
      layers, no `ALTER`s against the v1 schema
- [x] Range-partitioned sample tables with on-demand monthly partitions and a
      default partition, so a decode can never fail for want of one
- [x] Unique `(device_id, content_root)`; `bundle_id` unique as the primary key,
      and a reused ID with different content is **reported**, not swallowed
- [x] `geom` as a stored generated column, so the geometry cannot drift from the
      coordinates it represents
- [x] Decode worker (`cmd/cairn-worker`) consuming the outbox, in its own process
- [x] Reprocessing as a queued job: `-reprocess <root>` and `-reprocess-all`
- [x] Derived trip builder with drive/stop/gap segmentation, event detector and
      daily rollups
- [x] MQTT publishes semantic idempotent state only — verified against a real
      Mosquitto broker
- [x] Explicit retention policy table
- [x] 11 database integration tests against real PostGIS; 172 Go tests green

#### The two properties that matter

| Property | How it is enforced |
|---|---|
| **Idempotent** | Every write either upserts on a deterministic key or deletes the bundle's rows before reinserting, all in one transaction. A redelivered job costs a repeat, never a duplicate — asserted by re-decoding four times and comparing row counts |
| **Reproducible** | `derived.decode_runs.output_digest` hashes the derived output. Re-decoding at the same version must give the same digest; 20 consecutive decodes verified identical. A decoder upgrade appears as a second row at a higher version, so comparing digests shows exactly which bundles a change altered |

#### Failure isolation, verified

- A sync completes and yields a verifiable receipt **with the database closed** —
  ingest has no database dependency, so a Postgres outage delays the derived
  view and nothing more
- A decode failure leaves the job unacknowledged, the receipt untouched and
  every raw member retrievable. A parked job is never dropped: the raw bundle is
  intact, so a fixed decoder can pick it up later
- A decoder upgrade re-derives from raw with no device re-uploading anything

### Phase 5 — Firmware: project, partitions and logging — **complete**

- [x] Project at `firmware/cairn-v2/` on espressif32 7.1.3. The Arduino core on
      7.x *is* an ESP-IDF 5.x component bundle, so `framework = arduino` already
      exposes `esp_ota_ops`, `nvs_flash`, `esp_sleep` and `esp_rom_crc` — the
      substance of "IDF with arduino-esp32 as a component" without a separate
      multi-gigabyte framework download, and the vendored FreematicsPlus drivers
      keep working unchanged
- [x] **A/B OTA slots from the first commit** (`partitions-ab.csv`), replacing
      `huge_app.csv`: two 1.69 MB app slots, `otadata`, `nvs_keys`, `errlog` and
      `coredump`
- [x] NVS holds the device id (derived from the efuse MAC, so wiping NVS does not
      change which vehicle the data came from), the signing seed and the boot
      count
- [x] Boot self-test (`env:cairn-selftest`) covering CRC-32 through the ROM path,
      SHA-256, Ed25519 sign/verify/tamper-reject, a framed write to the card, a
      seal, and a live sync
- [x] Real `esp_sleep_get_wakeup_cause()`, recorded in every `STATE_TRANSITION`
- [x] Verbose SD logging: one file per boot, RAM-buffered before the card mounts
      so a mount failure is itself diagnosable, WARN/ERROR flushed immediately,
      capped at 16 MiB oldest-first and suspended below 64 MiB free — a testing
      aid must not be able to cost a trip
- [ ] Flash encryption — **deliberately not enabled.** Turning it on in release
      mode is irreversible, which is a poor property for a device already wired
      into a car. The partition exists so enabling it later needs no repartition

### Phase 6 — Firmware: capture lifecycle — **complete**

- [x] Four independent regions — capture, bundle, connectivity, health — rather
      than one flat enum. Independence is the point: losing the network must not
      end a trip, and a degraded sensor must not stop capture
- [x] Every transition journalled with trigger, reason code and the **policy
      version in force**, so a decision in the data stays explainable after the
      thresholds change
- [x] Confidence-scored evidence combining IMU RMS, OBD speed and GNSS speed,
      with separate start/stop thresholds and dwells — stopping requires the
      *absence* of evidence, which is weaker than its presence
- [x] GNSS gaps recorded as `GNSS_GAP` with duration and missed-sample count —
      silence in the data would be indistinguishable from the device being off
- [x] **45 s pre-roll ring** (`src/preroll.c`), 128 slots sized against the
      combined GNSS/IMU/OBD record rate. Records captured before a trip is
      confirmed are held and flushed with `CAIRN_FLAG_PRETRIP`; if the motion
      does not persist the ring is dropped, so a parked car produces nothing
      while a real drive still recovers its first seconds
- [x] **One transition controller owns all state; sensing only reports facts**
      (`src/facts.h`, `src/sensor_task.cpp`). This is a correctness fix, not a
      structural preference: the IMU used to be read in the same loop pass that
      wrote frames, so during a 30 ms card write no samples were taken and the
      RMS and peak a window reported were computed over whatever moments
      happened to miss I/O. Sensing is now pinned to core 0 and its rate no
      longer depends on the controller. The fact queue is bounded, and drops are
      reported in `DEVICE_HEALTH` rather than hidden

> **Correction.** The previous commit marked this phase complete while the
> pre-roll and the controller split were not implemented —
> `CAIRN_PRETRIP_RING_SAMPLES` was defined and never read. Both are now built
> and carry property rows.

### Phase 7 — Firmware: framed storage and recovery — **complete**

- [x] Append-only framed segments per format v2, rotated at 1 MiB, with one chain
      across all `seg-*` files and a separate chain for `journal.seg` (§3.2.1)
- [x] C implementation of format v2 (`lib/cairn_format`, portable C11, no IDF
      dependency) passing **all 25 committed vectors** on the host, clean under
      ASan and UBSan
- [x] Ed25519 vendored from TweetNaCl because mbedTLS has no Ed25519 signing, and
      checked *two* ways: verification against a Go-produced signature, and
      signing compared byte-for-byte against it. Ed25519 is deterministic, so a
      subtly wrong field implementation cannot survive that
- [x] The recovery scan is streaming (`cairn_scan_segment_stream`), so a segment
      far larger than DRAM is recoverable with one frame resident. The buffer
      entry point is a thin wrapper over it, so the device and the conformance
      vectors drive **one** body of code
- [x] Crash-safe seal: the manifest is written *before* the directory is moved,
      so an interrupted seal leaves either a capture holding a valid manifest —
      finished at the next boot without re-signing — or a completed bundle
- [x] Boot recovery runs automatically, truncates a torn tail to the last valid
      frame, and carries the exact discarded byte count into the manifest with
      `recovery_state` raised
- [x] SD write and recovery error counts surfaced in `DEVICE_HEALTH`
- [x] **A 20-row property matrix** (`test/host/faults.c`). The storage layer is
      portable C over a filesystem abstraction (`lib/cairn_fs`), so the host
      tests tear real files mid-frame and mid-header, flip payload bytes, forge
      receipts and interrupt seals — against exactly the code the device runs,
      not a parallel implementation. Mutation-checked: removing the receipt
      signature check fails 2 rows, skipping the torn-tail truncation fails 3

#### The rows

| Family | Property |
|---|---|
| seal | A clean capture seals; the manifest verifies and its content root recomputes from the members on disk |
| seal | An interrupted seal completes at the next boot without re-signing, and twice changes nothing |
| seal | A capture with no manifest is a live capture, not an interrupted seal |
| seal | A sealed bundle is never overwritten, even by a colliding id |
| recovery | A tear mid-frame truncates to the last valid frame; the discarded count is the surviving prefix, not the bytes that never arrived |
| recovery | A tear mid-header — too few bytes even for a length prefix — is still a torn tail, not a condemned segment |
| recovery | A flipped payload byte isolates to one record; the four frames before it survive and the state is `SALVAGED`, distinct from a clean tail recovery |
| recovery | One chain spans segment rotation; sequence numbers stay contiguous across the boundary |
| recovery | The journal chain is independent — four journal writes do not appear as a gap in the capture sequence |
| prune | A receipt signed by an untrusted key deletes nothing |
| prune | A genuine receipt for *different* content deletes nothing |
| prune | A malformed receipt deletes nothing |
| prune | No pinned key, or the all-zero placeholder, deletes nothing |
| prune | A verified receipt deletes the bundle, keeps the receipt and clears the intent |
| prune | An interrupted prune resumes; an intent with no receipt keeps the data and clears the intent |
| preroll | Nothing is written while the trip is unconfirmed |
| preroll | Flush writes in observation order, every frame flagged `PRETRIP`, and the flag does not leak past confirmation |
| preroll | A wrapped ring keeps the newest window and counts what it dropped |
| preroll | An oversized payload is refused rather than truncated into a different observation |

### Phase 8 — Firmware: sync and receipt-gated prune — **complete**

- [x] Manifest-first offer, chunks addressed **by hash** rather than byte offset,
      streamed from the card so a 256 KiB chunk never needs to fit in DRAM
- [x] Verify the receipt signature against a **pinned** server key **and** that
      its `content_root` matches what was uploaded. A valid signature over a
      different bundle is not an acknowledgement of this one
- [x] Receipts persisted outside the bundle directory, and stored *before* being
      acted on — the receipt is the durable evidence, the bundle bytes are not
- [x] Transactional prune: `prune_intent` → delete → completion, replayed at boot
- [x] An unconfigured key prunes **nothing**. A full card loses nothing; a
      wrongly authorized prune loses a trip permanently
- [x] mTLS with the private CA pinned on-device. The CA is compiled into
      firmware because it is the trust anchor — one read from the card could be
      swapped by anyone holding the card — while the client certificate and key
      live on the card, since they are rotatable and the certificate's
      CommonName must be the device id, which is not known until the hardware
      has booted once. Falls back to HTTP with an explicit error naming what is
      missing; it never silently downgrades.
      `deploy/make-certs.sh` issues the chain and emits the CA as a C literal.
      Verified end-to-end against the real server: with a client certificate the
      request succeeds and the server reads the CN as the device id, without one
      the handshake is refused. Running it that way found a defect the script
      would otherwise have shipped — macOS LibreSSL defaults to SHA-1, which Go
      rejects as `insecure algorithm ECDSA-SHA1` while sending the client a
      misleading `unknown ca` alert

> **Not yet run on hardware.** Phases 5–8 compile for the target (62.7% of the
> A slot, 25.2% RAM) and the format agrees byte-for-byte with the Go and Rust
> implementations on the host. That rules out a large class of bugs but not
> driver or timing problems. See
> [docs/v2-firmware-testing.md](docs/v2-firmware-testing.md) for the bench
> procedure, including the destructive tests that are the actual point.

### Phase 9 — Degraded states and policy tuning — **in progress**

- [x] `DEGRADED_GNSS` / `_STORAGE` / `_TIME` / `_NETWORK`, `LOW_POWER`,
      `RECOVERY_REQUIRED`, `DEGRADED_SENSING`, defined as a **bitmap** in
      spec §4.10 and implemented across firmware and Go

  The spec already declared `DEVICE_HEALTH.health_state` as a "bitmap of active
  degraded states (§4.7)" — but §4.7 is `STATE_TRANSITION` and never defined
  one, so the cross-reference was broken and the bitmap existed nowhere. The
  firmware was writing a scalar `0/1/2` into a field every decoder would read as
  a bitmap.

  A bitmap rather than a severity is the whole point: degradation is not
  ordered. A low battery, a missing fix and a full card are different problems
  with different fixes, and a scalar forces a priority between them and
  discards the rest — the old code reported `Critical` for the battery while
  silently losing the fact that position was unavailable too.

  Each bit is set from observed conditions, never inference. `DEGRADED_TIME` is
  set before the first fix of every trip, which is normal rather than
  exceptional — saying so is what stops a reader treating monotonic-only
  timestamps as UTC. Decoders preserve unknown bits rather than masking them,
  so a bundle from newer firmware stays interpretable.

  Covered by a storage-matrix row asserting the bitmap round-trips through a
  real segment with every bit independently recoverable, four Go unit tests
  including the unknown-bit rule, and `cairn-verify`, which now reports the
  union of conditions a bundle recorded. Verified end-to-end: the C firmware
  writes the bitmap and the Go reference reads it back by name.

- [ ] Event-adaptive sampling across GNSS, IMU and OBD
- [ ] Thresholds tuned on real traces, with the policy version journalled —
      blocked on bench data by design; tuning against guesses would be worse
      than leaving the defaults.
      Real drives and the analytical store (Phase 14) now remove the block; the
      work is scheduled as Phase 17

### Phase 10 — Ledger and documentation — **complete**

- [x] Admin ledger over the bundle lifecycle, every transition with a reason
      (`internal/ledger`, read by `cmd/cairn-ledger`).

  Append-only on disk rather than in PostgreSQL, deliberately: ingest has no
  database dependency — a Postgres outage delays the derived view and nothing
  more — and making the audit trail a database write would quietly give it one.
  A nil ledger disables recording rather than failing an upload, because an
  audit trail with veto power over the data it audits is the wrong shape.

  Every refusal must carry a reason, enforced in `Append` rather than trusted to
  call sites — the entries that need one are written on error paths where it is
  easiest to forget, and a refusal without a reason is the one entry nobody can
  act on. Verified end-to-end: an unenrolled device's 403 now appears as
  `device_unknown` with the reason, which is exactly the ambiguity that made
  "403 means unenrolled, not unreachable" worth documenting.

- [x] Every documented guarantee mapped to the row that verifies it, in
      [docs/guarantee-audit.md](docs/guarantee-audit.md), together with an
      explicit list of what is **not** covered.

  Writing it found two documented guarantees with no test at all. Revocation
  taking effect without a restart had none, and the mTLS certificate binding was
  worse than missing: a comment in the test harness deferred to
  `TestClientIdentityBinding` as though it existed. It did not. Both are now
  written and both are mutation-checked — disabling `refreshIfChanged` fails the
  first, removing the CommonName comparison fails the second.

- [x] Docs brought in line with what v2 actually does: the README describes the
      v2 firmware, the verifier, the ledger and OTA, and the audit records where
      the v1 documents are superseded rather than leaving them to be mistaken
      for current

### Phase 11 — Secure OTA — **complete**

- [x] Signed images verified before swap, with a **separate update key**. The
      receipt key says "this data is safe to delete"; the update key says "this
      code is safe to run". A server compromised enough to issue false receipts
      costs stored trips; one that could also sign firmware owns the device, so
      the update key never lives on the server — `cairn-signfw` signs offline
- [x] The ordering, which is the part that matters:
      verify the descriptor signature **before** downloading (otherwise a
      hostile server can make the device write megabytes into its spare slot on
      demand); hash the image **read back out of flash**, not the bytes as they
      arrived (hashing the download proves the transfer, not the write, and a
      partially programmed slot that hashed correctly in RAM is exactly what
      produces a boot loop); set the boot partition **last**
- [x] Post-boot self-test and automatic rollback. The image marks itself valid
      only after the card mounts and the tree is confirmed — an image that boots
      but cannot reach its storage is not a working image, and letting it mark
      itself valid would strand the device one reboot from working. The firmware
      logs loudly when the running slot differs from the configured boot
      partition, since that is the otherwise-invisible signature of a rollback
- [x] Never updates with unreceipted bundles pending, mid-trip, or on an
      unhealthy supply — including when the supply voltage is *unknown*, because
      updating on the strength of a reading the device could not take is the
      wrong direction. Each condition blocks on its own and is named in the log;
      "blocked" without a reason is unactionable
- [x] Version ordering **refuses rather than guesses**: an unparseable version
      on either side, or a pre-release suffix, blocks the update. Guessing an
      order is how a device installs something older than itself
- [ ] ESP-IDF secure boot — deliberately not enabled, the same reasoning that
      keeps flash encryption off: it is irreversible, which is a poor property
      for hardware already in a vehicle. This verifies the application
      signature, so a physically present attacker can still flash over serial

> `docs/ota.md` records the protocol, the four preconditions and the ordering
> argument. Verified end-to-end on the host: `cairn-signfw` signs a real 1.1 MB
> image, the C implementation verifies that exact descriptor and rejects both a
> tampered signature and a tampered descriptor, and the server serves the
> descriptor with its detached signature plus the image addressed by hash.
> Three property rows cover the preconditions, version ordering and descriptor
> strictness. The on-device install itself is unexercised — it needs hardware.

### Phase 12 — Hardware bring-up and deployment — **complete**

First contact with the real dongle, 2026-10-01. Four defects, none of which a
green test suite could have found, and three of which would have stayed hidden
for a long time.

- [x] Flashed over the Mac's USB (CH340 at 460800; 921600 fails with a message
      that reads like a wiring fault). Old v1 trip data and the 4 MB flash image
      backed up before erasing
- [x] **Fixed: the ESP32 ROM CRC-32 wrapper.** `~esp_rom_crc32_le(~0u, ...)`
      applies the inversions the ROM API documents but drops CRC-32/ISO-HDLC's
      final xorout, returning the raw shift register —
      `CRC-32("123456789")` came back `2dfd2d88` against the required
      `cbf43926`. Internally consistent, so the device never notices: frames
      written with the wrong CRC scan back cleanly. Every trip it recorded would
      have been rejected by the verifier
- [x] **Fixed: sealing overflowed the 8 KB loop-task stack.** Merkle tree, CBOR
      manifest and Ed25519 on one task. The canary panic became a *reboot loop*,
      because boot resumes the open capture and re-appends frames each cycle —
      the boot counter reached 92. Measured headroom is now logged after every
      seal, in production as well as the bench build: 8220 bytes free of 16384,
      so the old default was insufficient rather than marginal
- [x] **Fixed: the standby blocker logged every 20 ms** — 160 KB of a 174 KB
      capture, burying every transition and sync result and rolling the 16 MiB
      card log long before anything useful could be found. Logged on change now;
      same window produces 13.5 KB
- [x] **Fixed: `-dev` minted an ephemeral receipt key** even with a persistent
      seed on disk. The server would sign receipts under a key no device had
      pinned; the device would reject all of them and never prune; nothing on
      either side logged an error
- [x] **Known-answer gate promoted into the real firmware.** CRC-32 and SHA-256
      are checked against the specification at boot and capture is *refused* on
      disagreement. Both primitives are platform-configurable — ROM CRC,
      mbedTLS SHA — so neither is covered by the host conformance run, which
      compiles the portable fallback
- [x] Full loop verified on hardware: capture → seal → offer → commit → receipt
      issued → receipt verified against the pinned key → prune. Server ledger:
      5 entries, **0 refusals or failures**
- [x] Deployed as a hardened systemd unit under a dedicated `cairn` user, with
      mutual TLS on `:8443` against a private CA, replacing a transient
      `systemd-run --user` instance that did not survive a reboot
- [x] `GOAMD64=v3` with a CPU-feature guard — **+15.7%** on segment scanning.
      The primitives do not move because they were never compiled Go: CRC-32
      dispatches to PCLMULQDQ and SHA-256 to SHA-NI, measured at 32 GB/s and
      2 GB/s respectively

> The methodology that paid for itself: check documented claims against the
> code, and mutation-test every new test. The CRC defect is the clearest
> argument for cross-implementation conformance vectors existing at all — it was
> undetectable from inside the device, and the device was the only thing that
> could reveal it.

**Mutual TLS verified on hardware**, same evening, once the card could be moved
to a workstation to receive its client certificate:

```
mTLS ready: CA pinned in firmware, client credentials from the card
protocol: https, host: cairn.alpina.casa port: 8443 url: /api/v2/bundles/offer
offer 0000000000Y7VB098FJR5E1YQ6: 1 of 1 chunks missing
chunk 1/1 sent (2452 bytes, 5267e4d2..)
pruned 0000000000Y7VB098FJR5E1YQ6 (receipt verified against the pinned key)
sync done: 1 offered, 1 receipted, 1 pruned, 1 chunks sent, 0 receipts rejected
```

All three protocol steps ran over HTTPS on 8443, and the server attributed them
to `device=8777228e` — which it can only know from the TLS client certificate's
CommonName. The identity binding is therefore proven against mbedTLS on the real
device rather than only against the Go test client. Ledger: 17 entries, **0
refusals or failures**.

**Standby and wake-on-motion also verified**, found in the card logs rather than
over serial, which is the point of writing them to the card:

```
[PWR] WARN  entering standby
[SENS] INFO GNSS powered down for standby
[PWR] WARN  woke after 62015 ms and 50 polls: MOTION
[LIFE] WARN resumed after 20383 ms standby (MOTION); 2 standby period(s) totalling 82398 ms
```

A `GNSS_GAP` record was written unprompted when the receiver dropped out across
a standby cycle — honest incompleteness, recorded rather than interpolated.

**Still not verified:** parked current draw. It needs a meter, not a terminal,
and remains the single most useful measurement left in the system.
