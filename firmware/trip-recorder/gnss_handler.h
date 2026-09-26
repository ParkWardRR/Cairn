#ifndef CAIRN_GNSS_HANDLER_H
#define CAIRN_GNSS_HANDLER_H

#include "../freematics-base/src/trip_types.h"

// ---------------------------------------------------------------------------
// GNSS fix quality assessment
// ---------------------------------------------------------------------------
enum class GNSSQuality : uint8_t {
    EXCELLENT,   // HDOP < 1.5 && sats >= 8
    GOOD,        // HDOP < 3.0 && sats >= 5
    DEGRADED,    // HDOP < 5.0 && sats >= 3
    NO_FIX,      // Everything else
};

// ---------------------------------------------------------------------------
// GNSSHandler — parses GNSS data and assesses quality
// ---------------------------------------------------------------------------
class GNSSHandler {
public:
    GNSSHandler();

    void init();

    // Read a GNSS sample from the receiver (stubbed for now)
    GNSSSample readSample();

    // Quality assessment
    GNSSQuality assessQuality(const GNSSSample& sample) const;

    // Convenience accessors
    float getSpeedKmh(const GNSSSample& sample) const;
    bool  isMoving(const GNSSSample& sample) const;
    float getHDOP(const GNSSSample& sample) const;
    bool  hasFix(const GNSSSample& sample) const;

    // Quality trend tracking
    GNSSQuality getRecentQuality() const;
    bool        isQualityDegrading() const;

private:
    static constexpr size_t  QUALITY_HISTORY_SIZE = 10;
    static constexpr float   MIN_SPEED_KMH        = 5.0f;

    // Quality thresholds (hdop_tenths values)
    static constexpr uint16_t HDOP_EXCELLENT = 15;  // 1.5
    static constexpr uint16_t HDOP_GOOD      = 30;  // 3.0
    static constexpr uint16_t HDOP_DEGRADED  = 50;  // 5.0

    static constexpr uint8_t SATS_EXCELLENT = 8;
    static constexpr uint8_t SATS_GOOD      = 5;
    static constexpr uint8_t SATS_DEGRADED  = 3;

    GNSSQuality _qualityHistory[QUALITY_HISTORY_SIZE];
    size_t      _qualityIndex;
    size_t      _qualityCount;
    bool        _initialized;

    void recordQuality(GNSSQuality q);
};

#endif // CAIRN_GNSS_HANDLER_H
