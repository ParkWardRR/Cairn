#ifndef CAIRN_TRIP_TYPES_H
#define CAIRN_TRIP_TYPES_H

#include <cstdint>
#include <cstring>

// ---------------------------------------------------------------------------
// Device state enum
// ---------------------------------------------------------------------------
enum class DeviceState : uint8_t {
    SLEEP,
    ARMING,
    RECORDING,
    STOP_CANDIDATE,
    FINALIZING,
    QUEUED_FOR_HOME_SYNC,
    SYNCING,
    RETAINED,
    PRUNABLE,
    LOW_BATTERY_PROTECTION,
    FAULT
};

// ---------------------------------------------------------------------------
// Binary-packed sample structs (little-endian, ESP32 native)
// ---------------------------------------------------------------------------
#pragma pack(push, 1)

// 32 bytes
struct GNSSSample {
    uint64_t timestamp_ms;      // ms since Unix epoch
    int32_t  latitude;          // degrees x 10^7
    int32_t  longitude;         // degrees x 10^7
    int32_t  altitude_cm;       // cm above WGS84
    uint16_t speed_cmps;        // cm/s
    uint16_t heading_cdeg;      // centidegrees 0-35999
    uint8_t  fix_quality;       // 0=none, 1=GPS, 2=DGPS, 4=RTK
    uint8_t  satellites;        // visible count
    uint16_t hdop_tenths;       // HDOP x 10
    uint16_t accuracy_cm;       // horizontal accuracy cm
    uint8_t  _reserved[2];     // zero-filled
};

// 24 bytes
struct IMUSummary {
    uint64_t window_start_ms;
    uint16_t window_duration_ms;
    int16_t  accel_peak_x_mg;   // milli-g
    int16_t  accel_peak_y_mg;
    int16_t  accel_peak_z_mg;
    uint16_t accel_rms_mg;      // RMS magnitude milli-g
    int16_t  gyro_peak_dps;     // degrees/sec x 10
    uint16_t variance;          // motion intensity metric
    uint8_t  flags;             // bit flags: impact(0x01), hard_brake(0x02), sharp_turn(0x04)
    uint8_t  _reserved;
};

// 20 bytes
struct IMURawSample {
    uint64_t timestamp_ms;
    int16_t  accel_x_mg;
    int16_t  accel_y_mg;
    int16_t  accel_z_mg;
    int16_t  gyro_x_dps10;     // degrees/sec x 10
    int16_t  gyro_y_dps10;
    int16_t  gyro_z_dps10;
};

#pragma pack(pop)

// ---------------------------------------------------------------------------
// Compile-time size checks
// ---------------------------------------------------------------------------
static_assert(sizeof(GNSSSample)  == 32, "GNSSSample must be 32 bytes");
static_assert(sizeof(IMUSummary)  == 24, "IMUSummary must be 24 bytes");
static_assert(sizeof(IMURawSample) == 20, "IMURawSample must be 20 bytes");

// ---------------------------------------------------------------------------
// Trip event (not packed -- used only in-memory and serialized to JSON)
// ---------------------------------------------------------------------------
struct TripEvent {
    enum EventType : uint8_t {
        TRIP_START,
        TRIP_END,
        TRIP_PAUSE,
        TRIP_RESUMED,
        STOP_CANDIDATE_EVT,
        GNSS_QUALITY_DEGRADED,
        GNSS_QUALITY_RESTORED,
        POWER_ANOMALY,
        STORAGE_PRESSURE,
        IMU_EVENT_WINDOW
    };

    EventType type;
    uint64_t  timestamp_ms;
    int32_t   latitude;
    int32_t   longitude;
    char      details[64];
};

// ---------------------------------------------------------------------------
// Trip manifest -- written once when a trip is finalized
// ---------------------------------------------------------------------------
struct TripManifest {
    uint8_t  version;               // = 1
    char     trip_id[27];           // ULID string + null
    char     device_id[32];
    char     firmware_version[16];
    uint64_t started_at_ms;
    uint64_t ended_at_ms;
    uint32_t gnss_sample_count;
    uint32_t imu_summary_count;
    uint16_t imu_raw_window_count;
    uint8_t  gnss_rate_hz;
    uint8_t  imu_rate_hz;
    uint8_t  schema_version;        // = 1
};

#endif // CAIRN_TRIP_TYPES_H
