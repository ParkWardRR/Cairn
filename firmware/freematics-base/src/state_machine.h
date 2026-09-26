#ifndef CAIRN_STATE_MACHINE_H
#define CAIRN_STATE_MACHINE_H

#include <Arduino.h>
#include "trip_types.h"
#include "../hal/hal.h"
#include "config.h"

class StateMachine {
public:
    void init();
    void update();

    DeviceState getCurrentState() const;
    const char* getStateName() const;
    const char* getStateName(DeviceState state) const;

private:
    DeviceState state_ = DeviceState::SLEEP;

    unsigned long stateEnteredAt_  = 0;
    unsigned long lastGNSSRead_    = 0;
    unsigned long lastIMURead_     = 0;
    unsigned long lastWiFiScan_    = 0;

    uint32_t gnssSampleCount_      = 0;
    uint32_t imuSummaryCount_      = 0;
    char     currentTripId_[27]    = {};

    // Trip file paths built from trip ID
    char tripDir_[64]    = {};
    char samplesPath_[80] = {};
    char imuPath_[80]     = {};

    // Wi-Fi credentials loaded from NVS at init
    CairnHAL::NVSWiFiCredentials wifiCreds_ = {};
    uint8_t trustedBSSID_[6] = {};
    bool    wifiCredsLoaded_ = false;

    // Running SHA-256 for samples.bin during recording
    bool hashActive_ = false;

    // State handlers
    void handleSleep();
    void handleArming();
    void handleRecording();
    void handleStopCandidate();
    void handleFinalizing();
    void handleQueuedForHomeSync();
    void handleSyncing();
    void handleRetained();
    void handleLowBatteryProtection();
    void handleFault();

    void transitionTo(DeviceState next);

    // Sensor reads (HAL-backed)
    bool       detectMotion();
    GNSSSample readGNSS();
    IMUSummary readIMU();
    float      getSpeedKmh(const GNSSSample& s);
    bool       isMoving(const GNSSSample& s, const IMUSummary& imu);

    // Storage (HAL SDMMC-backed)
    bool initSD();
    bool openTripFile(const char* tripId);
    bool writeSample(const GNSSSample& s);
    bool writeIMUSummary(const IMUSummary& s);
    bool finalizeTripBundle();

    // Connectivity (HAL Wi-Fi-backed)
    bool scanForTrustedNetwork();
    bool connectToHome();
    bool uploadBundle(const char* tripId);

    // Power (HAL-backed)
    float getBatteryVoltage();

    // Helpers
    void generateTripId(char* buf, size_t len);
    bool writeManifest();
    bool writeChecksums();
    void addEvent(TripEvent::EventType type, const char* details);

    // Event buffer for current trip
    static constexpr size_t MAX_EVENTS = 64;
    TripEvent events_[MAX_EVENTS];
    uint16_t  eventCount_ = 0;

    uint64_t tripStartMs_ = 0;
};

#endif // CAIRN_STATE_MACHINE_H
