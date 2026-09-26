#include "gnss_handler.h"
#include <Arduino.h>
#include <cstring>

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------
GNSSHandler::GNSSHandler()
    : _qualityIndex(0)
    , _qualityCount(0)
    , _initialized(false)
{
    memset(_qualityHistory, 0, sizeof(_qualityHistory));
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------
void GNSSHandler::init() {
    _qualityIndex = 0;
    _qualityCount = 0;
    _initialized = true;
    Serial.println("[GNSSHandler] Initialized");
}

// ---------------------------------------------------------------------------
// readSample — stub returning placeholder data
// Real implementation would read from Freematics GNSS module via serial/I2C.
// ---------------------------------------------------------------------------
GNSSSample GNSSHandler::readSample() {
    GNSSSample sample;
    memset(&sample, 0, sizeof(sample));

    sample.timestamp_ms = millis();
    sample.latitude     = 340195000;   // 34.0195 N (Los Angeles area)
    sample.longitude    = -1184912000; // -118.4912 W
    sample.altitude_cm  = 7100;        // 71.00 m
    sample.speed_cmps   = 0;
    sample.heading_cdeg = 0;
    sample.fix_quality  = 1;           // GPS fix
    sample.satellites   = 10;
    sample.hdop_tenths  = 12;          // HDOP 1.2
    sample.accuracy_cm  = 250;         // 2.5 m

    return sample;
}

// ---------------------------------------------------------------------------
// Quality assessment
// ---------------------------------------------------------------------------
GNSSQuality GNSSHandler::assessQuality(const GNSSSample& sample) const {
    if (sample.hdop_tenths < HDOP_EXCELLENT && sample.satellites >= SATS_EXCELLENT) {
        return GNSSQuality::EXCELLENT;
    }
    if (sample.hdop_tenths < HDOP_GOOD && sample.satellites >= SATS_GOOD) {
        return GNSSQuality::GOOD;
    }
    if (sample.hdop_tenths < HDOP_DEGRADED && sample.satellites >= SATS_DEGRADED) {
        return GNSSQuality::DEGRADED;
    }
    return GNSSQuality::NO_FIX;
}

// ---------------------------------------------------------------------------
// Convenience accessors
// ---------------------------------------------------------------------------
float GNSSHandler::getSpeedKmh(const GNSSSample& sample) const {
    // speed_cmps * 0.036 = km/h  (cm/s -> km/h)
    return sample.speed_cmps * 0.036f;
}

bool GNSSHandler::isMoving(const GNSSSample& sample) const {
    return getSpeedKmh(sample) > MIN_SPEED_KMH && sample.fix_quality > 0;
}

float GNSSHandler::getHDOP(const GNSSSample& sample) const {
    return sample.hdop_tenths / 10.0f;
}

bool GNSSHandler::hasFix(const GNSSSample& sample) const {
    return sample.fix_quality > 0 && sample.satellites >= 1;
}

// ---------------------------------------------------------------------------
// Quality trend tracking
// ---------------------------------------------------------------------------
void GNSSHandler::recordQuality(GNSSQuality q) {
    _qualityHistory[_qualityIndex] = q;
    _qualityIndex = (_qualityIndex + 1) % QUALITY_HISTORY_SIZE;
    if (_qualityCount < QUALITY_HISTORY_SIZE) {
        _qualityCount++;
    }
}

GNSSQuality GNSSHandler::getRecentQuality() const {
    if (_qualityCount == 0) return GNSSQuality::NO_FIX;

    // Return the worst quality observed in recent history
    GNSSQuality worst = GNSSQuality::EXCELLENT;
    for (size_t i = 0; i < _qualityCount; i++) {
        if (static_cast<uint8_t>(_qualityHistory[i]) >
            static_cast<uint8_t>(worst)) {
            worst = _qualityHistory[i];
        }
    }
    return worst;
}

bool GNSSHandler::isQualityDegrading() const {
    if (_qualityCount < 3) return false;

    // Check if the last 3 readings are progressively worse
    size_t idx = (_qualityIndex + QUALITY_HISTORY_SIZE - 1) % QUALITY_HISTORY_SIZE;
    uint8_t prev = static_cast<uint8_t>(_qualityHistory[idx]);

    for (int i = 0; i < 2; i++) {
        idx = (idx + QUALITY_HISTORY_SIZE - 1) % QUALITY_HISTORY_SIZE;
        uint8_t curr = static_cast<uint8_t>(_qualityHistory[idx]);
        if (curr >= prev) return false; // Not getting worse
        prev = curr;
    }

    return true;
}
