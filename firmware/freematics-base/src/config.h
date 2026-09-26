#ifndef CAIRN_CONFIG_H
#define CAIRN_CONFIG_H

#include <cstdint>

// ---------------------------------------------------------------------------
// GNSS sampling rates (Hz) per device state
// ---------------------------------------------------------------------------
constexpr uint8_t GNSS_RATE_ACTIVE_HZ      = 5;   // Active driving
constexpr uint8_t GNSS_RATE_SLOW_HZ        = 2;   // Slow maneuvering
constexpr uint8_t GNSS_RATE_STATIONARY_HZ  = 1;   // Stationary
constexpr uint8_t GNSS_RATE_PARKED_HZ      = 0;   // Parked (off)

// ---------------------------------------------------------------------------
// IMU sampling rates (Hz) per device state
// ---------------------------------------------------------------------------
constexpr uint8_t IMU_RATE_ACTIVE_HZ       = 50;
constexpr uint8_t IMU_RATE_SLOW_HZ         = 25;
constexpr uint8_t IMU_RATE_STATIONARY_HZ   = 10;
constexpr uint8_t IMU_RATE_PARKED_HZ       = 0;

// ---------------------------------------------------------------------------
// Wi-Fi credentials (placeholders)
// ---------------------------------------------------------------------------
constexpr char WIFI_SSID[33]  = "YOUR_SSID_HERE";
constexpr char WIFI_BSSID[18] = "00:00:00:00:00:00";

// ---------------------------------------------------------------------------
// Sync server
// ---------------------------------------------------------------------------
constexpr char SERVER_HOSTNAME[64] = "cairn.local";
constexpr uint16_t SERVER_PORT     = 8443;

// ---------------------------------------------------------------------------
// Debounce / state-machine thresholds
// ---------------------------------------------------------------------------
constexpr uint32_t ARMING_DURATION_MS  = 15000;          // 15 s
constexpr uint32_t STOP_DWELL_MS       = 300000;         // 5 min
constexpr uint16_t MIN_SPEED_KMH       = 5;

// ---------------------------------------------------------------------------
// Storage limits
// ---------------------------------------------------------------------------
constexpr uint16_t MIN_FREE_SD_MB      = 50;
constexpr uint32_t MAX_TRIP_DURATION_MS = 14400000;       // 4 hours

// ---------------------------------------------------------------------------
// Power
// ---------------------------------------------------------------------------
constexpr uint16_t LOW_BATTERY_THRESHOLD_MV = 11500;      // 11.5 V

// ---------------------------------------------------------------------------
// GNSS quality gates
// ---------------------------------------------------------------------------
constexpr uint16_t MAX_HDOP_TENTHS     = 50;              // HDOP 5.0
constexpr uint8_t  MIN_SATELLITES      = 4;

// ---------------------------------------------------------------------------
// Connectivity
// ---------------------------------------------------------------------------
constexpr uint32_t WIFI_SCAN_INTERVAL_MS = 60000;         // 1 min
constexpr uint16_t UPLOAD_CHUNK_SIZE     = 4096;

// ---------------------------------------------------------------------------
// Data retention
// ---------------------------------------------------------------------------
constexpr uint16_t RETENTION_WINDOW_HOURS = 168;          // 7 days

#endif // CAIRN_CONFIG_H
