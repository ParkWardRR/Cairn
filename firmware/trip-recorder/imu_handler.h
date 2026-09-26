#ifndef CAIRN_IMU_HANDLER_H
#define CAIRN_IMU_HANDLER_H

#include "../freematics-base/src/trip_types.h"
#include <cstddef>

// ---------------------------------------------------------------------------
// IMUHandler — reads IMU data, computes summaries, detects events
// ---------------------------------------------------------------------------
class IMUHandler {
public:
    IMUHandler();

    void init();

    // Read a single raw sample (stubbed — returns near-gravity readings)
    IMURawSample readRawSample();

    // Compute an aggregate summary over a window of raw samples
    IMUSummary computeSummary(const IMURawSample* window, size_t count);

    // Detect events in a window; returns bitmask of event flags
    // 0x01 = impact, 0x02 = hard_brake, 0x04 = sharp_turn
    uint8_t detectEvent(const IMURawSample* window, size_t count) const;

    // Check if device is stationary based on summary statistics
    bool isStationary(const IMUSummary& summary) const;

    // Compute RMS acceleration magnitude over a window (in mg)
    uint16_t getRMSAcceleration(const IMURawSample* window, size_t count) const;

private:
    // Event detection thresholds (in mg)
    static constexpr int16_t  IMPACT_THRESHOLD_MG     = 2000;
    static constexpr int16_t  HARD_BRAKE_THRESHOLD_MG = 800;
    static constexpr int16_t  SHARP_TURN_THRESHOLD_MG = 500;

    // Stationary detection: variance below this is considered stationary
    static constexpr uint16_t STATIONARY_VARIANCE_MAX = 100;

    bool _initialized;
};

#endif // CAIRN_IMU_HANDLER_H
