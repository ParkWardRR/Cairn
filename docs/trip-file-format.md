# Trip File Format

> **v1 is dead.** This document describes the v1 trip format and is kept only so
> the old layout can be recognised. Nothing reads it: there is no v1 read path,
> no migration and no compatibility requirement, as
> [Bundle Format v2](bundle-format-v2.md) states. Tools that scan an SD card — including `cairn-tsdb` — ignore the
> card's `trips/` directory and load only sealed v2 bundles from `bundles/`. Do
> not add a v1 parser.

## Bundle Structure

Each trip is stored as a self-contained, versioned, checksummed bundle:

```
trip/
  manifest.cbor       # Trip metadata, device info, schema version
  samples.bin.zst     # GNSS + IMU samples (zstd on server; raw .bin on device)
  events.cbor         # Lifecycle and quality events
  sha256sums.txt      # Per-file content hashes
  signature.ed25519   # Device signature over sha256sums.txt
```

## Manifest (CBOR)

```cbor
{
  "version": 1,
  "trip_id": "01J9YTVPAK6WWZQGBQQN8PB7B8",   // ULIDv7
  "device_id": "cairn-mazda-01",
  "device_pubkey": "<base64 Ed25519 public key>",
  "firmware_version": "0.1.0",
  "started_at": "2026-09-25T18:30:00Z",
  "ended_at": "2026-09-25T19:02:14Z",
  "sample_count": {
    "gnss": 1842,
    "imu_summary": 47,
    "imu_raw_windows": 3
  },
  "gnss_rate_hz": 1,
  "imu_rate_hz": 25,
  "schema_version": 1,
  "compression": "none",                       // "zstd" after server repack
  "integrity": {
    "sha256_manifest": false,                   // this file; hash is in sha256sums
    "sha256_samples": "<hex>",
    "sha256_events": "<hex>"
  }
}
```

## GNSS Sample Format

Binary-packed records, fixed-width for streaming reads:

| Field | Type | Size | Unit |
|-------|------|------|------|
| timestamp_ms | uint64 | 8 | ms since Unix epoch |
| latitude | int32 | 4 | degrees × 10^7 |
| longitude | int32 | 4 | degrees × 10^7 |
| altitude_cm | int32 | 4 | centimeters above WGS84 |
| speed_cmps | uint16 | 2 | cm/s |
| heading_cdeg | uint16 | 2 | centidegrees (0–35999) |
| fix_quality | uint8 | 1 | 0=none, 1=GPS, 2=DGPS, 4=RTK |
| satellites | uint8 | 1 | visible satellite count |
| hdop_tenths | uint16 | 2 | HDOP × 10 |
| accuracy_cm | uint16 | 2 | estimated horizontal accuracy in cm |
| _reserved | uint8[2] | 2 | future use, zero-filled |
| **Total** | | **32** | **bytes per sample** |

At 1 Hz, a 30-minute trip produces ~57 KB of GNSS data. At 5 Hz, ~285 KB.

## IMU Sample Format

### Summary Records (default storage)

Rolling windows aggregated from raw IMU data:

| Field | Type | Size | Unit |
|-------|------|------|------|
| window_start_ms | uint64 | 8 | ms since Unix epoch |
| window_duration_ms | uint16 | 2 | window length |
| accel_peak_x_mg | int16 | 2 | milli-g |
| accel_peak_y_mg | int16 | 2 | milli-g |
| accel_peak_z_mg | int16 | 2 | milli-g |
| accel_rms_mg | uint16 | 2 | RMS magnitude milli-g |
| gyro_peak_dps | int16 | 2 | degrees/sec × 10 |
| variance | uint16 | 2 | motion intensity metric |
| flags | uint8 | 1 | bit flags: impact, hard_brake, sharp_turn |
| _reserved | uint8 | 1 | zero-filled |
| **Total** | | **24** | **bytes per window** |

### Raw Windows (event-triggered)

Short bursts of full-rate IMU data retained around unusual events:

| Field | Type | Size | Unit |
|-------|------|------|------|
| timestamp_ms | uint64 | 8 | ms since Unix epoch |
| accel_x_mg | int16 | 2 | milli-g |
| accel_y_mg | int16 | 2 | milli-g |
| accel_z_mg | int16 | 2 | milli-g |
| gyro_x_dps10 | int16 | 2 | degrees/sec × 10 |
| gyro_y_dps10 | int16 | 2 | degrees/sec × 10 |
| gyro_z_dps10 | int16 | 2 | degrees/sec × 10 |
| **Total** | | **20** | **bytes per raw sample** |

## Events (CBOR)

```cbor
{
  "events": [
    {
      "type": "trip_start",
      "timestamp": "2026-09-25T18:30:00Z",
      "trigger": "gnss_speed+imu_motion",
      "location": { "lat": 34.0195, "lon": -118.4912 }
    },
    {
      "type": "gnss_quality_degraded",
      "timestamp": "2026-09-25T18:42:15Z",
      "hdop": 8.3,
      "satellites": 3,
      "duration_s": 45
    },
    {
      "type": "stop_candidate",
      "timestamp": "2026-09-25T18:55:00Z",
      "location": { "lat": 34.0259, "lon": -118.5101 },
      "resolved": "resumed",
      "dwell_s": 120
    },
    {
      "type": "trip_end",
      "timestamp": "2026-09-25T19:02:14Z",
      "trigger": "dwell_threshold",
      "location": { "lat": 34.0195, "lon": -118.4915 },
      "parking_accuracy_m": 4.2
    }
  ]
}
```

### Event Types

| Type | Description |
|------|-------------|
| `trip_start` | Drive session began |
| `trip_end` | Drive session finalized |
| `trip_pause` | Entered stop candidate state |
| `trip_resumed` | Resumed from stop candidate |
| `stop_candidate` | Stop evaluation (with resolution) |
| `gnss_quality_degraded` | HDOP exceeded threshold or satellites dropped |
| `gnss_quality_restored` | Fix quality returned to acceptable range |
| `power_anomaly` | Unexpected voltage change |
| `storage_pressure` | microSD free space below threshold |
| `imu_event_window` | Raw IMU burst captured (impact, hard brake) |

## Integrity

### sha256sums.txt

```
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  manifest.cbor
a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a  samples.bin
b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9  events.cbor
```

### signature.ed25519

Ed25519 signature of sha256sums.txt using the device's private key. Verifiable with the device's registered public key on the server.

## Versioning

The `schema_version` field in the manifest enables forward migration:

- v1: Initial format as documented here
- Future versions must be parseable by the bundle library with explicit migration
- Old bundles are never rewritten in place; migrations produce new derivation records

## Compression

- **On device:** Uncompressed binary records (v1). Compression adds firmware complexity and RAM pressure on ESP32 with marginal benefit for local storage.
- **On server:** zstd compression of samples after validation. Original uncompressed bundle is retained as the immutable archive; compressed copy used for serving.
- Format stability matters more than file size in v1.
