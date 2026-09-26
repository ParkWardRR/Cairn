#include "imu_handler.h"
#include <Arduino.h>
#include <cstring>
#include <cmath>

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------
IMUHandler::IMUHandler()
    : _initialized(false)
{
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------
void IMUHandler::init() {
    _initialized = true;
    Serial.println("[IMUHandler] Initialized");
}

// ---------------------------------------------------------------------------
// readRawSample — stub returning near-gravity readings
// Real implementation would read from the Freematics IMU (MPU-6050 or similar).
// ---------------------------------------------------------------------------
IMURawSample IMUHandler::readRawSample() {
    IMURawSample sample;
    memset(&sample, 0, sizeof(sample));

    sample.timestamp_ms = millis();
    sample.accel_x_mg   = 0;      // No lateral acceleration
    sample.accel_y_mg   = 0;      // No forward acceleration
    sample.accel_z_mg   = 1000;   // 1g gravity along Z
    sample.gyro_x_dps10 = 0;
    sample.gyro_y_dps10 = 0;
    sample.gyro_z_dps10 = 0;

    return sample;
}

// ---------------------------------------------------------------------------
// computeSummary — aggregate statistics over a window of raw samples
// ---------------------------------------------------------------------------
IMUSummary IMUHandler::computeSummary(const IMURawSample* window, size_t count) {
    IMUSummary summary;
    memset(&summary, 0, sizeof(summary));

    if (window == nullptr || count == 0) return summary;

    summary.window_start_ms = window[0].timestamp_ms;

    int16_t peakX = 0, peakY = 0, peakZ = 0;
    int16_t peakGyro = 0;
    float   sumMagSq = 0.0f;
    float   sumMag   = 0.0f;

    for (size_t i = 0; i < count; i++) {
        const IMURawSample& s = window[i];

        // Track peak acceleration per axis (max absolute value, preserving sign)
        if (abs(s.accel_x_mg) > abs(peakX)) peakX = s.accel_x_mg;
        if (abs(s.accel_y_mg) > abs(peakY)) peakY = s.accel_y_mg;
        if (abs(s.accel_z_mg) > abs(peakZ)) peakZ = s.accel_z_mg;

        // Track peak gyroscope across all axes
        if (abs(s.gyro_x_dps10) > abs(peakGyro)) peakGyro = s.gyro_x_dps10;
        if (abs(s.gyro_y_dps10) > abs(peakGyro)) peakGyro = s.gyro_y_dps10;
        if (abs(s.gyro_z_dps10) > abs(peakGyro)) peakGyro = s.gyro_z_dps10;

        // Accumulate for RMS magnitude: sqrt(x^2 + y^2 + z^2)
        float mag = sqrtf((float)s.accel_x_mg * s.accel_x_mg +
                          (float)s.accel_y_mg * s.accel_y_mg +
                          (float)s.accel_z_mg * s.accel_z_mg);
        sumMagSq += mag * mag;
        sumMag   += mag;
    }

    // Window duration
    uint64_t endMs = window[count - 1].timestamp_ms;
    uint64_t durMs = endMs - summary.window_start_ms;
    summary.window_duration_ms = (uint16_t)(durMs > 65535 ? 65535 : durMs);

    // Peak values
    summary.accel_peak_x_mg = peakX;
    summary.accel_peak_y_mg = peakY;
    summary.accel_peak_z_mg = peakZ;
    summary.gyro_peak_dps   = peakGyro;

    // RMS acceleration magnitude
    float rms = sqrtf(sumMagSq / (float)count);
    summary.accel_rms_mg = (uint16_t)rms;

    // Variance (stddev of magnitude)
    float meanMag = sumMag / (float)count;
    float sumVariance = 0.0f;
    for (size_t i = 0; i < count; i++) {
        const IMURawSample& s = window[i];
        float mag = sqrtf((float)s.accel_x_mg * s.accel_x_mg +
                          (float)s.accel_y_mg * s.accel_y_mg +
                          (float)s.accel_z_mg * s.accel_z_mg);
        float diff = mag - meanMag;
        sumVariance += diff * diff;
    }
    summary.variance = (uint16_t)sqrtf(sumVariance / (float)count);

    // Event detection
    summary.flags = detectEvent(window, count);

    return summary;
}

// ---------------------------------------------------------------------------
// detectEvent — check for impact, hard braking, sharp turns
// ---------------------------------------------------------------------------
uint8_t IMUHandler::detectEvent(const IMURawSample* window, size_t count) const {
    uint8_t flags = 0;
    if (window == nullptr || count == 0) return flags;

    for (size_t i = 0; i < count; i++) {
        const IMURawSample& s = window[i];

        // Total acceleration magnitude
        float totalAccel = sqrtf((float)s.accel_x_mg * s.accel_x_mg +
                                 (float)s.accel_y_mg * s.accel_y_mg +
                                 (float)s.accel_z_mg * s.accel_z_mg);

        // Impact: any acceleration exceeding 2000 mg
        if (totalAccel > IMPACT_THRESHOLD_MG) {
            flags |= 0x01;
        }

        // Hard brake: forward deceleration > 800 mg
        // Assuming Y-axis is forward (positive = forward, negative = braking)
        if (s.accel_y_mg < -HARD_BRAKE_THRESHOLD_MG) {
            flags |= 0x02;
        }

        // Sharp turn: lateral acceleration > 500 mg
        if (abs(s.accel_x_mg) > SHARP_TURN_THRESHOLD_MG) {
            flags |= 0x04;
        }
    }

    return flags;
}

// ---------------------------------------------------------------------------
// isStationary — low variance means the device is not moving
// ---------------------------------------------------------------------------
bool IMUHandler::isStationary(const IMUSummary& summary) const {
    return summary.variance < STATIONARY_VARIANCE_MAX;
}

// ---------------------------------------------------------------------------
// getRMSAcceleration — compute RMS of acceleration magnitude over window
// ---------------------------------------------------------------------------
uint16_t IMUHandler::getRMSAcceleration(const IMURawSample* window,
                                        size_t count) const {
    if (window == nullptr || count == 0) return 0;

    float sumMagSq = 0.0f;

    for (size_t i = 0; i < count; i++) {
        const IMURawSample& s = window[i];
        float mag = sqrtf((float)s.accel_x_mg * s.accel_x_mg +
                          (float)s.accel_y_mg * s.accel_y_mg +
                          (float)s.accel_z_mg * s.accel_z_mg);
        sumMagSq += mag * mag;
    }

    return (uint16_t)sqrtf(sumMagSq / (float)count);
}
