# Device Protocol

## Hardware Platform

**Freematics ONE+ Model B**
- ESP32 microcontroller
- Integrated 10 Hz GNSS receiver
- 6-axis IMU (accelerometer + gyroscope)
- microSD card slot
- Wi-Fi (802.11 b/g/n)
- LTE modem (deliberately unused)
- OBD-II connector (power only; no diagnostic polling)

## Device State Machine

```
SLEEP
  │ motion detected / GNSS activity / power-rise
  ▼
ARMING
  │ sustained movement for N seconds (configurable, ~15s default)
  ▼
RECORDING
  │ no meaningful movement for configured dwell time
  ▼
STOP_CANDIDATE
  ├── movement resumes within dwell window ──► RECORDING
  └── dwell threshold reached ───────────────► FINALIZING
                                                 │
                                                 ▼
                                           QUEUED_FOR_HOME_SYNC
                                                 │
                                    trusted Wi-Fi detected and associated
                                                 │
                                                 ▼
                                               SYNCING
                                      ├── retryable failure ──► QUEUED
                                      └── receipt verified ───► RETAINED
                                                                  │
                                                         retention window elapsed
                                                                  │
                                                                  ▼
                                                               PRUNABLE
```

### Additional States

| State | Trigger | Behavior |
|-------|---------|----------|
| `LOW_BATTERY_PROTECTION` | Vehicle voltage drops below threshold | Finalize any active trip, disable radios, deep sleep |
| `FAULT` | Unrecoverable error (SD failure, firmware panic) | Log to NVS, emit diagnostic on next boot, attempt recovery |

## Sensor Rates

Starting points for testing — not final values.

| Condition | GNSS | IMU | Storage |
|-----------|-----:|----:|---------|
| Active driving | 1–5 Hz | 25–50 Hz | Record every GNSS sample; batch or window IMU |
| Slow maneuvering | 1–2 Hz | 25 Hz | Preserve parking endpoint confidence |
| Stationary (stop candidate) | 0.2–1 Hz | 10–25 Hz | Evaluate resume vs. finalization |
| Parked away from home | Off after settle | Low-power motion wake | No Wi-Fi scan; sparse checks only |
| Parked at home | Off | Low-power | Wi-Fi only long enough to sync |
| Sync complete | Off | Low-power | Lowest viable sleep state |

## IMU Storage Strategy

Avoid storing raw 50 Hz IMU indefinitely. For normal trips:

- Store rolling summaries: peak acceleration, braking/turn windows, variance
- Retain short high-rate windows around unusual events (impact candidates)
- Keep microSD write load, storage use, and backend growth manageable
- Demonstrate value of raw IMU before committing to full-rate retention

## Trip Detection Heuristics

### Start Conditions

A trip begins when **both** are true:
1. GNSS reports speed above threshold (e.g., > 5 km/h)
2. IMU confirms sustained motion (not just GPS jitter)

The ARMING state requires these conditions to persist for a debounce window (default ~15 seconds) to avoid false starts from door slams, garage door vibration, or parking-lot repositioning.

### Stop Conditions

A trip enters STOP_CANDIDATE when:
1. GNSS speed drops below threshold
2. IMU indicates stationary (within noise floor)

The trip finalizes only after the dwell threshold is reached (e.g., 5 minutes of no meaningful movement). This avoids fragmenting trips during:
- Traffic lights
- Gas station stops
- Drive-through windows
- Short errand pauses

Conservative default: prefer merging short errands into one trip over producing false trip fragments.

## Sync Protocol

### Prerequisites

1. Device has one or more `queued` trip bundles
2. Device detects and associates with a trusted SSID/BSSID
3. mTLS handshake succeeds with homelab server

### Upload Flow

```
Device                                    Server
  │                                         │
  ├── POST /upload/init ──────────────────► │
  │   { trip_id, content_hash, size }       │
  │                                         │
  │ ◄── 200 { upload_id, resume_offset } ──┤
  │                                         │
  ├── PUT /upload/{id}/chunk ─────────────► │
  │   { offset, data }                      │
  │                                         │
  │   (repeat for each chunk)               │
  │                                         │
  ├── POST /upload/{id}/finalize ─────────► │
  │   { content_hash }                      │
  │                                         │
  │ ◄── 200 { receipt_id, signature } ─────┤
  │                                         │
  │   Device stores receipt locally         │
  │   Trip state → ACKNOWLEDGED → RETAINED  │
```

### Resumption

If upload is interrupted (vehicle leaves, Wi-Fi drops):
1. On next home arrival, device re-initiates with same `trip_id` and `content_hash`
2. Server responds with `resume_offset` for any partially received upload
3. Device resumes from that offset

### Upload Budget

- Limit total Wi-Fi transmit time per sync session
- Suspend gracefully if association is lost
- Prioritize oldest unsynced trips first
- Resume on next home arrival

## Power Management

| Mode | Active Peripherals | Estimated Current |
|------|-------------------|-------------------|
| Active drive | ESP32 + GNSS + IMU + microSD | TBD (measure) |
| Upload | ESP32 + Wi-Fi + microSD | TBD (measure) |
| Light sleep | ESP32 (periodic wake) | TBD (measure) |
| Deep sleep | Motion wake interrupt only | ~10 mA (vendor claim, validate) |

### Power-Down Sequence

1. Detect ignition-off (voltage drop or sustained stationary)
2. Finalize active trip bundle
3. Disable GNSS
4. If at home Wi-Fi: attempt sync, then sleep
5. If away: sleep immediately with periodic motion-wake check

## Provisioning

### v1: Serial Provisioning

```
# Over USB serial connection
cairn-provision --ssid "HomeNetwork" --psk "password"
cairn-provision --server-ca ./ca.pem
cairn-provision --generate-identity
```

### Future: BLE or Captive Portal

Planned but not required for v1. Serial provisioning is sufficient for a personal device.

## Firmware Updates

- **v1:** USB/serial flashing only
- **Future:** Signed OTA over home Wi-Fi, only after core system is proven reliable
- Updates must never brick the device or corrupt stored trip data
