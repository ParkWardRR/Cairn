#ifndef CAIRN_CONFIG_H
#define CAIRN_CONFIG_H

#include <cstdint>

// ===========================================================================
// Local deployment values (WiFi credentials, server host) live in secrets.h,
// which is gitignored — this repo is public, so nothing environment-specific
// belongs in a tracked file. Copy secrets.h.example to secrets.h to set them.
// Every value below has a safe, generic fallback, so a fresh clone builds
// without secrets.h present.
// ===========================================================================
#if defined(__has_include)
#  if __has_include("secrets.h")
#    include "secrets.h"
#  endif
#endif

// ===========================================================================
// Pin assignments come from the vendored FreematicsPlus.h.
// Do NOT redeclare them here. This file holds only Cairn-specific
// behavioural thresholds and configuration.
// ===========================================================================

// ===========================================================================
// OBD-II configuration
// ===========================================================================
constexpr uint8_t MAX_OBD_ERRORS = 3;

// ===========================================================================
// GNSS sampling rates (Hz) per device state
// ===========================================================================
constexpr uint8_t GNSS_RATE_ACTIVE_HZ      = 5;
constexpr uint8_t GNSS_RATE_SLOW_HZ        = 2;
constexpr uint8_t GNSS_RATE_STATIONARY_HZ  = 1;
constexpr uint8_t GNSS_RATE_PARKED_HZ      = 0;

// ===========================================================================
// IMU sampling rates (Hz) per device state
// ===========================================================================
constexpr uint8_t IMU_RATE_ACTIVE_HZ       = 50;
constexpr uint8_t IMU_RATE_SLOW_HZ         = 25;
constexpr uint8_t IMU_RATE_STATIONARY_HZ   = 10;
constexpr uint8_t IMU_RATE_PARKED_HZ       = 0;

// ===========================================================================
// Adaptive data interval (harvested from stock firmware)
// Tracks how long vehicle has been motionless, slows logging to save power.
// ===========================================================================
constexpr uint16_t STATIONARY_TIME_TABLE[] = {10, 60, 180};
constexpr uint16_t DATA_INTERVAL_TABLE[]   = {1000, 2000, 5000};
constexpr uint8_t  STATIONARY_TIERS = 3;

// ===========================================================================
// GNSS watchdog — reset module after this many seconds with no fix
// ===========================================================================
constexpr uint16_t GNSS_RESET_TIMEOUT_S = 300;

// ===========================================================================
// Sync server
// ===========================================================================
// Override CAIRN_SERVER_HOST / CAIRN_SERVER_PORT in secrets.h (or with -D
// build flags) to point at your own server. The default below is deliberately
// generic so release builds carry no personal hostname.
#ifndef CAIRN_SERVER_HOST
#define CAIRN_SERVER_HOST "cairn.local"
#endif
#ifndef CAIRN_SERVER_PORT
#define CAIRN_SERVER_PORT 8080
#endif
constexpr char SERVER_HOSTNAME[64] = CAIRN_SERVER_HOST;
constexpr uint16_t SERVER_PORT     = CAIRN_SERVER_PORT;

// ===========================================================================
// State-machine thresholds
// ===========================================================================
constexpr uint32_t ARMING_DURATION_MS  = 15000;
constexpr uint32_t STOP_DWELL_MS       = 300000;
constexpr uint16_t MIN_SPEED_KMH       = 5;

// ===========================================================================
// Storage limits
// ===========================================================================
constexpr uint16_t MIN_FREE_SD_MB      = 50;
constexpr uint32_t MAX_TRIP_DURATION_MS = 14400000;

// ===========================================================================
// Power
// ===========================================================================
constexpr float LOW_BATTERY_THRESHOLD_V  = 11.5f;
constexpr float ENGINE_ON_VOLTAGE_V      = 13.2f;
constexpr float ENGINE_OFF_VOLTAGE_V     = 12.8f;
constexpr float JUMPSTART_VOLTAGE_V      = 14.0f;

// ===========================================================================
// Motion detection
// ===========================================================================
constexpr float MOTION_THRESHOLD_G  = 0.4f;
constexpr float ACCEL_RMS_MOVING_MG = 200.0f;

// ===========================================================================
// GNSS quality gates
// ===========================================================================
constexpr uint8_t MAX_HDOP      = 50;
constexpr uint8_t MIN_SATELLITES = 4;

// ===========================================================================
// Connectivity
// ===========================================================================
constexpr uint32_t WIFI_SCAN_INTERVAL_MS = 60000;
constexpr uint16_t UPLOAD_CHUNK_SIZE     = 4096;
constexpr uint16_t SIGNAL_CHECK_INTERVAL_S = 10;
constexpr uint16_t PING_BACK_INTERVAL_S   = 900;

// ===========================================================================
// Data retention
// ===========================================================================
constexpr uint16_t RETENTION_WINDOW_HOURS = 168;

// ===========================================================================
// IMU event thresholds (in g)
// ===========================================================================
constexpr float IMPACT_THRESHOLD_G      = 3.0f;
constexpr float HARD_BRAKE_THRESHOLD_G  = 0.8f;
constexpr float SHARP_TURN_THRESHOLD_G  = 0.6f;

// ===========================================================================
// Trip storage base path
// ===========================================================================
constexpr const char* TRIP_BASE_PATH = "/cairn/trips";

// ===========================================================================
// Thermal protection
// ===========================================================================
constexpr int COOLING_DOWN_TEMP_C = 75;

// ===========================================================================
// External sensor inputs
// ===========================================================================
#define LOG_EXT_SENSORS 0
// 0 = disabled, 1 = digital GPIO, 2 = analog ADC

// ===========================================================================
// Standby
// ===========================================================================
constexpr bool RESET_AFTER_WAKEUP = true;
constexpr bool GNSS_ALWAYS_ON     = false;

// ===========================================================================
// WiFi credentials — set these in secrets.h (included at the top of this file),
// via -D build flags, or at runtime over BLE into NVS. Empty defaults mean a
// build without secrets.h simply has no network configured.
// ===========================================================================
#ifndef WIFI_SSID
#define WIFI_SSID ""
#endif
#ifndef WIFI_PASSWORD
#define WIFI_PASSWORD ""
#endif

#endif // CAIRN_CONFIG_H
