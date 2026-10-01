# Cairn Bundle Format v2 — Normative Specification

**Status:** normative. Three independent implementations must agree byte-for-byte:
firmware (C, ESP-IDF), server (Go), emulator (Rust). Where this document and an
implementation disagree, this document is correct and the implementation is a
bug.

**Supersedes:** bundle format v1 (`docs/trip-file-format.md`). There is no v1
read path, no migration and no compatibility requirement. v1 bundles are not
readable by v2 tooling.

---

## 1. Design Rules

These follow from the project invariants. Every layout decision below traces to
one of them.

1. **A sealed bundle is never mutated.** Segments are append-only; sealing is a
   one-way transition marked by an atomic rename.
2. **No byte is deleted without a locally verified signed receipt.**
3. **Ordering truth is `(boot_id, seq)`, never wall-clock UTC.** UTC appears
   only as an estimate carrying its own uncertainty.
4. **Honest incompleteness beats fabricated continuity.** Absent data is
   recorded explicitly (`GNSS_GAP`), never interpolated.

### 1.1 Conventions

- All integers are **little-endian**, matching ESP32 native order.
- All offsets and sizes are in bytes.
- `u8/u16/u32/u64` are unsigned; `i8/i16/i32` signed two's complement.
- Reserved fields are **written as zero** and **ignored on read**. They are not
  a compatibility mechanism; `format_version` is.
- Multi-byte payload fields are 4-byte aligned by construction (§3.1).

### 1.2 Checksum

v2 uses **CRC-32** (reflected, polynomial `0xEDB88320` — the IEEE/zlib variant),
not CRC32C/Castagnoli.

> **Deviation, recorded deliberately.** The architecture review specified
> CRC32C. CRC32C has no meaningful error-detection advantage at these frame
> sizes, and CRC-32 is available in ESP32 ROM as `esp_rom_crc32_le`, so the
> constrained implementation gets an optimized routine for free. Availability on
> the device decides it. Go uses `hash/crc32.ChecksumIEEE`; Rust uses
> `crc32fast`.

ESP ROM uses an inverted convention. The canonical device-side invocation is:

```c
uint32_t cairn_crc32(const uint8_t *buf, size_t len) {
    return ~esp_rom_crc32_le(~0u, buf, len);
}
```

This yields values identical to `crc32.ChecksumIEEE` and `crc32fast`.

### 1.3 Hashing and the Merkle tree

SHA-256 throughout (ESP32 hardware-accelerated on the device side).

The Merkle tree is binary with explicit domain separation, so a leaf can never
be reinterpreted as an internal node:

```
leaf(d)          = SHA256(0x00 || d)
internal(l, r)   = SHA256(0x01 || l || r)
```

Build bottom-up over an ordered leaf list. If a level has an odd node count, the
final node is **promoted unchanged** to the next level — it is *not* duplicated.
Duplicating the last node admits two distinct leaf lists with the same root.

An empty leaf list has root `SHA256(0x02)` — a third distinct domain tag, so an
empty tree is not confusable with any leaf or node.

---

## 2. Bundle Layout on Device Storage

A bundle is a directory. Its name is the `bundle_id` (§5.1).

```
/cairn/bundles/<bundle_id>/
    seg-00000000.seg        capture segments, append-only, rotated by size
    seg-00000001.seg
    journal.seg             lifecycle state-transition journal (same framing)
    manifest.cbor           written once, at seal; absent until then
    manifest.sig            Ed25519 signature over manifest.cbor
```

### 2.1 Sealing is an atomic rename

The directory carries its lifecycle state in its own name:

```
<bundle_id>.open     capture in progress; manifest absent
<bundle_id>.sealed   manifest + signature present and verified
```

Sealing procedure, in this order, with no permitted reordering:

1. Flush and `fsync` every segment.
2. Rescan all segments, verifying every frame CRC (§3.3).
3. Build `manifest.cbor` in a temporary file `manifest.cbor.tmp`.
4. `fsync` the temporary file.
5. Write `manifest.sig.tmp`, `fsync`.
6. Rename both temporaries to their final names.
7. `fsync` the directory.
8. Rename `<bundle_id>.open` → `<bundle_id>.sealed`.

A crash at any step leaves the bundle recoverable: the raw segments are intact
and complete, and the absence of the `.sealed` suffix means sealing is simply
re-run at boot. **Segments are never discarded because sealing failed.**

### 2.2 Receipts live outside the bundle

```
/cairn/receipts/<bundle_id>.cbor
```

Receipts are stored outside the bundle directory so that pruning a payload can
never destroy the proof of its delivery, and are retained **longer** than the
bundles they acknowledge.

---

## 3. Segment File Format

An append-only sequence of framed records behind a fixed header.

### 3.1 Segment header — 64 bytes

| Offset | Size | Field | Notes |
|---:|---:|---|---|
| 0 | 4 | `magic` | ASCII `CRN2` = `43 52 4E 32` |
| 4 | 2 | `format_version` | u16, = `2` |
| 6 | 2 | `header_len` | u16, = `64` |
| 8 | 16 | `device_id` | 16 raw bytes (§5.2) |
| 24 | 16 | `boot_id` | 16 raw bytes, regenerated every power cycle |
| 40 | 4 | `segment_index` | u32, 0-based within the bundle |
| 44 | 4 | `first_seq` | u32, `seq` of the first frame in this segment |
| 48 | 8 | `opened_monotonic_us` | u64, device monotonic microseconds at open |
| 56 | 4 | `reserved` | zero |
| 60 | 4 | `header_crc32` | CRC-32 over bytes `[0, 60)` |

`header_len` permits a reader to skip a longer header from a future version
without misparsing frames. It does not make a future format readable.

### 3.2 Frame — 24 bytes of envelope plus payload

| Offset | Size | Field | Notes |
|---:|---:|---|---|
| 0 | 2 | `frame_len` | u16, **total** frame bytes including this field and the trailing CRC |
| 2 | 1 | `record_type` | §3.4 |
| 3 | 1 | `schema_version` | payload schema version, independent of `format_version` |
| 4 | 2 | `flags` | §3.5 |
| 6 | 2 | `reserved` | zero |
| 8 | 4 | `seq` | u32, strictly increasing by 1 across a chain (§3.2.1), not per segment |
| 12 | 4 | `monotonic_ms` | u32, milliseconds since the segment header's `opened_monotonic_us` |
| 16 | 4 | `prev_crc32` | u32, the `crc32` field of the preceding frame; `0` for the bundle's first frame |
| 20 | 4 | `reserved2` | zero — aligns the payload to a 4-byte boundary |
| 24 | N | `payload` | §4 |
| 24+N | 4 | `crc32` | CRC-32 over bytes `[0, 24+N)` |

So `frame_len = 28 + N`. Minimum valid `frame_len` is `28` (empty payload);
maximum is `4096`.

`monotonic_ms` as a 32-bit delta from the segment's open covers 49 days, far
beyond any segment lifetime, and costs 4 bytes instead of 8.

#### 3.2.1 Chains

A bundle contains **two independent chains**, each with its own `seq` space and
its own `prev_crc32` linkage:

| Chain | Files | Starts at |
|---|---|---|
| **Capture** | `seg-00000000.seg`, `seg-00000001.seg`, … in index order | `seq` 0, `prev_crc32` 0 in segment 0 |
| **Journal** | `journal.seg` | `seq` 0, `prev_crc32` 0 |

Capture segments continue one chain across a rotation, so verifying segment
*N+1* requires the final state of segment *N*. The journal is separate because
it records transitions that occur when no capture segment is open at all —
waking, sleeping, connecting, uploading, pruning. Forcing it to share a
sequence space would mean the journal could not be written while capture was
closed, which is precisely when most of its interesting entries happen.

A verifier must therefore scan the journal from a zero state, independently of
the capture segments, and must not treat the two chains' sequence numbers as
comparable.

**`prev_crc32` is the chain.** The review asked for a per-record chain hash;
a 32-byte SHA-256 per record would double the size of a 32-byte GNSS sample. The
4-byte predecessor CRC gives what the chain is actually for — detecting that a
record was removed, reordered or spliced — during a linear scan, without needing
the manifest. Whole-bundle tamper detection is provided by the signed
`content_root` (§5.3), which is strictly stronger than per-record chaining.

### 3.3 Recovery scan

Normative algorithm. Runs **automatically at boot**, not only from a CLI tool.

```
verify segment header: magic, format_version, header_len, header_crc32
  → any failure: segment is unusable; report, do not delete
expected_seq  = header.first_seq
expected_prev = 0 for segment_index 0, else last valid crc32 of the prior segment
offset        = header.header_len

loop:
  if remaining < 28                      → TORN_TAIL, stop
  read frame_len
  if frame_len < 28 or frame_len > 4096  → TORN_TAIL, stop
  if offset + frame_len > file_size      → TORN_TAIL, stop
  compute CRC-32 over [offset, offset+frame_len-4)
  if mismatch with trailing crc32        → CORRUPT_FRAME, stop
  if prev_crc32 != expected_prev         → CHAIN_BREAK, stop
  if seq != expected_seq                 → SEQ_GAP, stop
  accept frame
  expected_seq  += 1
  expected_prev  = this frame's crc32
  offset        += frame_len
```

**Default behaviour is stop-at-first-invalid.** Every frame before the failure
point is valid and retained; everything from the failure point to EOF is
discarded as an incomplete tail. The byte count discarded and the stop reason
are both reported, never silently swallowed.

An optional **salvage mode**, used only by the offline recovery tool and never
on-device, may resynchronize past a damaged region by scanning forward for an
offset where a frame's CRC validates and its `seq` is plausible. Salvage mode
must report the isolated damaged byte range and must mark the resulting bundle
`salvaged`, which makes it ineligible for the normal upload path.

### 3.4 Record types

| Value | Type | Payload |
|---:|---|---|
| `0x01` | `GNSS_SAMPLE` | §4.1 |
| `0x02` | `IMU_SUMMARY` | §4.2 |
| `0x03` | `IMU_RAW_WINDOW` | §4.3 |
| `0x04` | `OBD_SNAPSHOT` | §4.4 |
| `0x05` | `DEVICE_HEALTH` | §4.5 |
| `0x06` | `TRIP_EVENT` | §4.6 |
| `0x07` | `STATE_TRANSITION` | §4.7 |
| `0x08` | `GNSS_GAP` | §4.8 |
| `0x09` | `POLICY_SNAPSHOT` | §4.9 |

Unknown record types encountered by a reader are **skipped using `frame_len`**,
counted, and reported. An unknown type is not an error — it is how a newer
device remains partially readable by an older decoder. The frame CRC still
applies, so a skipped record is still integrity-checked.

### 3.5 Frame flags

| Bit | Name | Meaning |
|---:|---|---|
| 0 | `PRETRIP` | Record came from the pre-roll buffer; it precedes trip confirmation |
| 1 | `DEGRADED` | Captured while a degraded health state was active |
| 2 | `ESTIMATED_UTC` | The UTC basis was an estimate with no valid GNSS fix at write time |
| 3 | `POST_RECOVERY` | Written after a boot recovery, i.e. first record of a resumed capture |
| 4–15 | reserved | zero |

---

## 4. Payload Schemas

All at `schema_version = 1` unless stated.

### 4.1 `GNSS_SAMPLE` — 32 bytes

| Offset | Size | Field | Units |
|---:|---:|---|---|
| 0 | 4 | `lat_e7` | i32, degrees × 10⁷ |
| 4 | 4 | `lon_e7` | i32, degrees × 10⁷ |
| 8 | 4 | `alt_cm` | i32, cm above WGS84 ellipsoid |
| 12 | 2 | `speed_cmps` | u16, cm/s |
| 14 | 2 | `heading_cdeg` | u16, centidegrees, 0–35999 |
| 16 | 2 | `hdop_e2` | u16, HDOP × 100 |
| 18 | 2 | `h_acc_cm` | u16, horizontal accuracy, cm |
| 20 | 2 | `v_acc_cm` | u16, vertical accuracy, cm |
| 22 | 1 | `fix_type` | 0 none, 1 2D, 2 3D, 3 DGPS, 4 RTK-float, 5 RTK-fixed |
| 23 | 1 | `sats_used` | u8 |
| 24 | 1 | `sats_visible` | u8 |
| 25 | 1 | `source_flags` | bit 0 GNSS, 1 GLONASS, 2 Galileo, 3 BeiDou, 4 dead-reckoned |
| 26 | 4 | `utc_offset_ms` | i32, signed delta from `utc_basis_ms` (§5.1) |
| 30 | 2 | `utc_acc_ms` | u16, uncertainty of that UTC estimate; `0xFFFF` = unknown |

Quality fields are **mandatory and never synthesized**. A reader must not infer
a precision the receiver did not report: where the receiver supplies no accuracy
estimate, `h_acc_cm`/`v_acc_cm` are `0xFFFF` (unknown), not a plausible guess.

A sample with `fix_type = 0` carries no valid position. `lat_e7`/`lon_e7` must be
written as zero and must not be plotted. Prefer a `GNSS_GAP` record (§4.8) over
a run of fix-less samples.

### 4.2 `IMU_SUMMARY` — 20 bytes

| Offset | Size | Field | Units |
|---:|---:|---|---|
| 0 | 2 | `window_ms` | u16, window duration |
| 2 | 2 | `accel_rms_mg` | u16, RMS magnitude, milli-g |
| 4 | 2 | `accel_peak_x_mg` | i16 |
| 6 | 2 | `accel_peak_y_mg` | i16 |
| 8 | 2 | `accel_peak_z_mg` | i16 |
| 10 | 2 | `gyro_peak_dps_e1` | i16, deg/s × 10 |
| 12 | 2 | `variance` | u16, motion-intensity metric |
| 14 | 2 | `sample_count` | u16, raw samples in the window |
| 16 | 1 | `event_flags` | bit 0 impact, 1 hard brake, 2 sharp turn, 3 pothole |
| 17 | 3 | `reserved` | zero |

### 4.3 `IMU_RAW_WINDOW` — variable

A triggered burst, persisted around a detected event rather than continuously.

| Offset | Size | Field |
|---:|---:|---|
| 0 | 2 | `sample_count` u16 |
| 2 | 2 | `interval_us` u16, nominal spacing |
| 4 | 1 | `trigger_reason` u8, mirrors `IMU_SUMMARY.event_flags` bit index |
| 5 | 3 | `reserved`, zero |
| 8 | 12 × `sample_count` | samples |

Each sample is 12 bytes: `accel_x/y/z_mg` (i16 ×3), `gyro_x/y/z_dps_e1` (i16 ×3).

### 4.4 `OBD_SNAPSHOT` — 24 bytes

| Offset | Size | Field | Units |
|---:|---:|---|---|
| 0 | 2 | `speed_kph` | i16, `0x8000` = unavailable |
| 2 | 2 | `rpm` | i16, `0x8000` = unavailable |
| 4 | 2 | `fuel_pressure_kpa` | u16, `0xFFFF` = unavailable |
| 6 | 1 | `throttle_pct` | u8, `0xFF` = unavailable |
| 7 | 1 | `engine_load_pct` | u8, `0xFF` = unavailable |
| 8 | 1 | `coolant_temp_c` | i8, `0x80` = unavailable |
| 9 | 1 | `intake_temp_c` | i8, `0x80` = unavailable |
| 10 | 1 | `timing_advance_deg` | i8, `0x80` = unavailable |
| 11 | 1 | `pid_error_count` | u8, ECU non-responses since the last snapshot |
| 12 | 4 | `pids_requested` | u32 bitmap |
| 16 | 4 | `pids_answered` | u32 bitmap |
| 20 | 2 | `poll_cadence_ms` | u16, **actual** measured cadence, not the target |
| 22 | 2 | `reserved` | zero |

Every field has an explicit unavailable sentinel, and the requested/answered
bitmaps record PID availability per the review. A decoder must distinguish "the
ECU reported 0 kph" from "the ECU did not answer".

### 4.5 `DEVICE_HEALTH` — 16 bytes

| Offset | Size | Field | Units |
|---:|---:|---|---|
| 0 | 2 | `battery_mv` | u16, vehicle supply |
| 2 | 2 | `sd_write_errors` | u16, cumulative this boot |
| 4 | 2 | `sd_free_mib` | u16 |
| 6 | 1 | `device_temp_c` | i8 |
| 7 | 1 | `rssi_dbm` | i8, `0` when not associated |
| 8 | 2 | `ext_sensor_1` | u16 |
| 10 | 2 | `ext_sensor_2` | u16 |
| 12 | 1 | `health_state` | bitmap of active degraded states (§4.10) |
| 13 | 1 | `reboot_count` | u8, consecutive abnormal reboots |
| 14 | 2 | `reserved` | zero |

### 4.6 `TRIP_EVENT` — variable

| Offset | Size | Field |
|---:|---:|---|
| 0 | 1 | `event_type` u8 |
| 1 | 1 | `detail_len` u8 |
| 2 | 2 | `reserved`, zero |
| 4 | 4 | `lat_e7` i32 |
| 8 | 4 | `lon_e7` i32 |
| 12 | `detail_len` | UTF-8 detail, not NUL-terminated |

`lat_e7`/`lon_e7` are zero when no valid fix was available; position validity is
carried by the nearest `GNSS_SAMPLE`, never assumed.

### 4.7 `STATE_TRANSITION` — 20 bytes

The lifecycle journal record. Written to `journal.seg`, and additionally in-band
in the capture segment for transitions occurring during capture.

| Offset | Size | Field |
|---:|---:|---|
| 0 | 1 | `region` u8 — 1 capture, 2 bundle, 3 connectivity, 4 health |
| 1 | 1 | `from_state` u8 |
| 2 | 1 | `to_state` u8 |
| 3 | 1 | `trigger_event` u8 |
| 4 | 1 | `reason_code` u8 |
| 5 | 1 | `policy_version` u8 |
| 6 | 2 | `reserved`, zero |
| 8 | 2 | `start_score_e2` u16 — evidence score × 100 at decision time |
| 10 | 2 | `stop_score_e2` u16 |
| 12 | 4 | `wake_cause` u32 — raw `esp_sleep_get_wakeup_cause()` |
| 16 | 4 | `reserved2`, zero |

Recording both evidence scores and the `policy_version` at every transition is
what makes a retune attributable and lets `trip-replay` prove *why* a trip
started, continued, split, finalized, retried or was retained.

### 4.8 `GNSS_GAP` — 12 bytes

| Offset | Size | Field |
|---:|---:|---|
| 0 | 4 | `duration_ms` u32 |
| 4 | 2 | `expected_samples` u16 — samples that would have been written |
| 6 | 1 | `cause` u8 — 1 no fix, 2 receiver reset, 3 powered down, 4 tunnel/obstruction, 5 time jump |
| 7 | 1 | `reserved`, zero |
| 8 | 4 | `reserved2`, zero |

This record is the mechanism for invariant 4. A decoder renders a gap as a
discontinuity; it must never join the route across it.

### 4.9 `POLICY_SNAPSHOT` — variable

A deterministic CBOR map (§5) of the active detection thresholds, written once
per bundle at `CONFIRMING_TRIP`. Makes every bundle self-describing with respect
to the policy that produced it, so a retune is reconstructable after the fact.

`policy_version` alone is not enough. It identifies a policy but does not
describe it, so interpreting an old bundle would mean finding the firmware build
that defined that version. Recording the values themselves means a bundle
captured under thresholds nobody remembers is still explainable from the bundle.

| Key | Field | Units |
|---:|---|---|
| 1 | `policy_version` | u8, matches `STATE_TRANSITION.policy_version` and the manifest |
| 2 | `gnss_period_ms` | nominal GNSS sampling period |
| 3 | `imu_window_ms` | IMU summary window |
| 4 | `obd_period_ms` | nominal OBD polling period |
| 5 | `health_period_ms` | health record period |
| 6 | `start_score_threshold_e2` | evidence score × 100 required to suspect motion |
| 7 | `stop_score_threshold_e2` | evidence score × 100 required to suspect rest |
| 8 | `start_dwell_ms` | how long motion must persist before a trip is declared |
| 9 | `stop_dwell_ms` | how long rest must persist before a trip is sealed |
| 10 | `motion_accel_rms_mg` | accelerometer RMS treated as motion |
| 11 | `motion_speed_cmps` | speed treated as motion |
| 12 | `preroll_window_ms` | pre-trip retention window |
| 13 | `preroll_ring_samples` | pre-trip ring capacity, in records |
| 14 | `segment_max_bytes` | segment rotation size |
| 15 | `adaptive_sampling` | u8, 1 when event-adaptive rates are active (§4.9.1) |

Keys ascend and are encoded per §5, so the map is byte-identical for identical
policy — which is what lets a reader group bundles by policy without trusting
the version number.

#### 4.9.1 Event-adaptive sampling

When `adaptive_sampling` is 1, the nominal periods above are upper bounds rather
than fixed rates: the device may sample *faster* when the vehicle is doing
something worth resolving, and no slower than nominal while a trip is active.

The direction is deliberate. A reader may assume a record every
`gnss_period_ms` at worst during a trip, so a gap longer than that is still a
gap and still recorded as one (§4.8). Adaptation can only add detail, never
remove it, which keeps the guarantee a bundle makes independent of what the
device decided at the time.

Rates are not themselves recorded per sample. The frames carry their own
monotonic timestamps, so the achieved rate is recoverable from the data rather
than needing to be asserted alongside it.


### 4.10 Degraded-state bitmap

`DEVICE_HEALTH.health_state` is a bitmap, not an enum.

That choice is the point of the field. Degradation is not ordered: a vehicle
can be low on battery *and* without a GNSS fix *and* out of card space at the
same time, and these have different causes and different fixes. A scalar
severity would force a priority between them and silently discard the rest —
reporting "critical" for the battery while losing the fact that position was
also unavailable. Every active condition is therefore its own bit, and a reader
can recover the full set.

| Bit | Value | Name | Meaning |
|---:|---:|---|---|
| 0 | `0x01` | `DEGRADED_GNSS` | No usable position: the receiver is absent, unresponsive, or has reported no fix for longer than one sample period |
| 1 | `0x02` | `DEGRADED_STORAGE` | Writes are failing, or free space has fallen below the floor reserved for capture |
| 2 | `0x04` | `DEGRADED_TIME` | No UTC basis has been established, so sample times are monotonic-only |
| 3 | `0x08` | `DEGRADED_NETWORK` | Bundles are awaiting a receipt and the server could not be reached |
| 4 | `0x10` | `LOW_POWER` | Supply voltage is below the threshold at which capture is still trusted |
| 5 | `0x20` | `RECOVERY_REQUIRED` | The open capture could not be safely extended and must be sealed as-is |
| 6 | `0x40` | `DEGRADED_SENSING` | A sensor other than GNSS is unavailable, or sensor readings were dropped before being recorded |
| 7 | `0x80` | — | Reserved, zero |

`0x00` means no degraded condition is active. It is not the same as "healthy in
every respect this device could measure" — it means nothing on this list is
true, which is the only claim the device is in a position to make.

Two rules follow for writers:

1. A bit is set from *observed* conditions, never from inference. `DEGRADED_GNSS`
   means no fix arrived, not that one seems unlikely.
2. `DEGRADED_TIME` is set whenever the UTC basis is absent, including before the
   first fix of a trip. This is normal rather than exceptional, and saying so is
   what stops a reader treating monotonic-only timestamps as if they were UTC.

A decoder must preserve unknown bits rather than masking them off, so a bundle
from newer firmware stays interpretable for the conditions an older decoder does
understand.

---

## 5. Manifest

`manifest.cbor` is **deterministically encoded CBOR**, RFC 8949 §4.2.1 core
deterministic encoding:

- definite-length maps and arrays only;
- integer map keys, sorted ascending;
- smallest-width integer encoding;
- no indefinite-length strings, no tags, no floats.

Integer keys and a specified encoding together mean the signed byte sequence is
reproducible across implementations and firmware versions. An ad hoc
serialization that varies between versions would invalidate historical
signatures.

### 5.1 Fields

| Key | Name | Type |
|---:|---|---|
| 1 | `manifest_version` | u8, = 2 |
| 2 | `bundle_id` | 16 bytes, ULID |
| 3 | `device_id` | 16 bytes |
| 4 | `device_key_id` | 8 bytes, truncated SHA-256 of the device public key |
| 5 | `boot_id` | 16 bytes |
| 6 | `firmware_version` | text |
| 7 | `schema_version` | u8 |
| 8 | `capture_started_monotonic_us` | u64 |
| 9 | `capture_ended_monotonic_us` | u64 |
| 10 | `utc_basis_ms` | u64, the UTC basis GNSS deltas are relative to |
| 11 | `utc_basis_acc_ms` | u32, uncertainty of that basis |
| 12 | `first_seq` | u32 |
| 13 | `last_seq` | u32 |
| 14 | `record_counts` | map of `record_type` → count |
| 15 | `members` | array of `[name, length, sha256]`, sorted by name |
| 16 | `chunk_descriptors` | array of `[chunk_index, byte_length, sha256]` |
| 17 | `content_root` | 32 bytes (§5.3) |
| 18 | `previous_bundle_root` | 32 bytes, or null for the first bundle |
| 19 | `policy_version` | u8 |
| 20 | `recovery_state` | u8 — 0 clean, 1 recovered tail, 2 salvaged |
| 21 | `discarded_tail_bytes` | u32 |
| 22 | `signature_algorithm` | text, `"ed25519"` |

The signature covers the deterministic CBOR encoding of keys 1–22. It is stored
separately in `manifest.sig`, so the signed bytes are exactly the file bytes with
nothing to strip or re-encode.

`previous_bundle_root` is **populated but not enforced** in v2. It enables
detecting deleted historical bundles later, which is a different threat model
from detecting corruption within one bundle.

### 5.2 Identities, and why there are three

The review is right that a content hash makes a poor operational handle. v2
keeps three distinct identifiers with non-overlapping jobs:

| Identifier | Job | Stability |
|---|---|---|
| `bundle_id` | ULID. Retries, receipts, support, directory naming, log correlation | Assigned at bundle open; never changes |
| `content_root` | Identity of the *data*. Deduplication, idempotency, the signed commitment | A function of content; identical data yields an identical root |
| `transfer_hash` | SHA-256 of a transferred byte stream. Transport integrity only | Per transfer; not an identity |

`device_id` is 16 raw bytes rather than a string, keeping the segment header
fixed-width. It is the truncated SHA-256 of the device's enrollment certificate
public key.

### 5.3 `content_root`

Merkle root (§1.3) over one leaf per bundle member, members sorted by raw name
bytes ascending:

```
leaf_input = len(name) as u16 LE || name bytes || sha256(member contents)
leaf       = SHA256(0x00 || leaf_input)
```

`manifest.cbor` and `manifest.sig` are **not** members — the root is an input to
the manifest, so it cannot cover it.

Defining identity over members rather than over an archive's bytes keeps it
independent of archive framing. Hashing a tar stream would make identity depend
on mtime, uid and padding, so re-packing identical data would produce a
different identity.

---

## 6. Transfer Protocol

Manifest-first, content-addressed, resumable by hash.

```
1. OFFER     device → POST /api/v2/bundles/offer      { manifest.cbor, manifest.sig }
             server verifies signature, device enrollment, key id
             server → { bundle_id, missing_chunks: [indices], existing_receipt? }

2. TRANSFER  for each missing chunk:
               device → PUT /api/v2/bundles/{bundle_id}/chunks/{sha256}
               server verifies the chunk hash against chunk_descriptors
               server → { accepted, remaining: [indices] }

3. COMMIT    device → POST /api/v2/bundles/{bundle_id}/commit
             server reassembles members, recomputes member digests,
               recomputes content_root, compares to the signed manifest
             server durably commits raw objects + manifest
             server persists the receipt BEFORE responding
             server → { receipt.cbor }

4. VERIFY    device verifies the receipt signature against the pinned server key
             device verifies receipt.content_root == its own manifest's
             device persists the receipt outside the bundle directory
             device marks the bundle RECEIPT_VERIFIED
```

### 6.1 The bundle byte stream, and how chunks map to members

Chunks and members are different partitions of the same bytes, so the mapping
between them must be exact rather than conventional.

The **bundle byte stream** is the concatenation of member contents in canonical
member order — sorted by raw name bytes ascending, the same order the content
root uses:

```
stream = contents(members[0]) || contents(members[1]) || ... || contents(members[n-1])
```

Chunks partition that stream in order and without gaps or overlap.
`chunk_descriptors[i]` covers the stream byte range:

```
start(i) = sum of chunk_descriptors[j].byte_length for j < i
end(i)   = start(i) + chunk_descriptors[i].byte_length
```

Two consistency requirements follow, and a server must check both before
committing:

1. `sum(chunk_descriptors[*].byte_length)` equals `sum(members[*].length)`.
   A mismatch means the manifest is internally inconsistent, whatever its
   signature says.
2. `chunk_descriptors[i].index == i`. Indices are positional, not identifiers.

Member boundaries and chunk boundaries are deliberately independent: a member
may span several chunks and a chunk may span several members. Tying them
together would force a re-chunk whenever a member's size changed.

Reassembly is therefore: concatenate chunks in index order to rebuild the
stream, split the stream at the cumulative member lengths, verify each member's
SHA-256, then recompute `content_root` and compare it to the signed value.

### 6.2 Rules

- **Chunks are addressed by hash, never by byte offset.** This is what makes
  re-chunking safe and deduplication possible. A byte offset cannot survive
  either endpoint changing its chunk size.
- Default chunk size is 256 KiB. The device may choose any size; the manifest's
  `chunk_descriptors` are authoritative for that bundle.
- Idempotency is keyed on `content_root`, not on connection, request ID or
  `bundle_id`. Re-offering identical data returns the existing receipt.
- A chunk acknowledgement is **not** a receipt. It means bytes were accepted,
  not that a reconstructable bundle was committed.
- Every protocol step is journaled on the device before it is attempted, so a
  reboot resumes rather than rediscovers.
- Transport is mTLS on 8443. The device pins the private CA; the server verifies
  per-device client certificates and consults a denylist.

### 6.3 Receipt

Deterministic CBOR, same rules as the manifest.

| Key | Name | Type |
|---:|---|---|
| 1 | `receipt_version` | u8, = 2 |
| 2 | `receipt_id` | 16 bytes |
| 3 | `device_id` | 16 bytes |
| 4 | `bundle_id` | 16 bytes |
| 5 | `content_root` | 32 bytes |
| 6 | `server_ingest_utc_ms` | u64 |
| 7 | `server_key_id` | 8 bytes |
| 8 | `ingest_schema_version` | u8 |
| 9 | `stored_object_ids` | array of text |
| 10 | `signature_algorithm` | text, `"ed25519"` |
| 11 | `signature` | 64 bytes over the deterministic encoding of keys 1–10 |

A receipt is only an acknowledgement if **both** its signature verifies against
the pinned server key **and** its `content_root` equals what the device
uploaded. A valid signature over a different bundle is not an acknowledgement of
this one.

---

## 7. Conformance

An implementation is conformant when it passes every vector in
`fixtures/format-v2/`. The vectors are the executable form of this document.

| Vector | Asserts |
|---|---|
| `valid-minimal` | One frame of each record type, clean seal |
| `valid-multi-segment` | `seq` continuity and `prev_crc32` chaining across a rotation |
| `torn-tail-mid-frame` | Truncation inside a payload → `TORN_TAIL`, preceding frames retained, discarded byte count exact |
| `torn-tail-mid-header` | Truncation inside a frame envelope → `TORN_TAIL` |
| `bad-frame-crc` | Single flipped payload bit → `CORRUPT_FRAME` at that frame, not before |
| `chain-break-spliced` | One frame removed from the middle → `CHAIN_BREAK` |
| `seq-gap` | `seq` skips a value → `SEQ_GAP` |
| `bad-header-crc` | Corrupt segment header → segment unusable, not deleted |
| `unknown-record-type` | Unknown type skipped via `frame_len`, counted, remainder parsed |
| `clock-jump` | UTC estimate jumps backwards mid-segment → ordering unchanged, record retained |
| `gnss-gap` | A gap record is preserved and never interpolated across |
| `empty-segment` | Header only, zero frames → valid, empty |
| `max-frame` | `frame_len = 4096` accepted; 4097 rejected |
| `manifest-determinism` | Same logical manifest encodes to identical bytes across implementations |
| `manifest-bad-signature` | Flipped signature bit → rejected |
| `merkle-odd-leaves` | Odd leaf count promotes rather than duplicates; root matches the reference |
| `merkle-empty` | Empty tree root = `SHA256(0x02)` |
| `content-root-member-order` | Member ordering is by raw name bytes; permuted input yields the same root |
| `receipt-wrong-content-root` | Valid signature over a different root → rejected as an acknowledgement |

Each vector directory contains the input bytes, the expected parse verdict and
the expected derived values, so a conformance runner needs no implementation
knowledge beyond this document.
