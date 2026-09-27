#ifndef CAIRN_CONFIG_H
#define CAIRN_CONFIG_H

#include <cstdint>

// ===========================================================================
// Pin assignments come from the vendored FreematicsPlus.h.
// Do NOT redeclare them here. This file holds only Cairn-specific
// behavioural thresholds and configuration.
// ===========================================================================

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
// Sync server
// ===========================================================================
constexpr char SERVER_HOSTNAME[64] = "cairn.local";
constexpr uint16_t SERVER_PORT     = 8443;

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

#endif // CAIRN_CONFIG_H
