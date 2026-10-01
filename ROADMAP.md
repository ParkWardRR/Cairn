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

- [x] Rust implementation of format v2 (`emulator/src/format/`), **20/20
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
      dependency) passing **all 20 committed vectors** on the host, clean under
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
      than leaving the defaults

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
