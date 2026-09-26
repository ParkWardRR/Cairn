#ifndef CAIRN_STATE_MACHINE_H
#define CAIRN_STATE_MACHINE_H

#include <Arduino.h>
#include "trip_types.h"

class StateMachine {
public:
    void init();
    void update();

    DeviceState getCurrentState() const;
    const char* getStateName() const;
    const char* getStateName(DeviceState state) const;

private:
    // Current device state
    DeviceState state_ = DeviceState::SLEEP;

    // Timing helpers
    unsigned long stateEnteredAt_  = 0;
    unsigned long lastGNSSRead_    = 0;
    unsigned long lastIMURead_     = 0;
    unsigned long lastWiFiScan_    = 0;

    // Trip bookkeeping
    uint32_t gnssSampleCount_      = 0;
    uint32_t imuSummaryCount_      = 0;
    char     currentTripId_[27]    = {};

    // -----------------------------------------------------------------------
    // State handlers
    // -----------------------------------------------------------------------
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

    // Transition helper
    void transitionTo(DeviceState next);

    // -----------------------------------------------------------------------
    // Sensor stubs (to be replaced with real drivers)
    // -----------------------------------------------------------------------
    static bool        detectMotion();
    static GNSSSample  readGNSS();
    static IMUSummary  readIMU();
    static float       getSpeedKmh(const GNSSSample& s);
    static bool        isMoving(const GNSSSample& s, const IMUSummary& imu);

    // -----------------------------------------------------------------------
    // Storage stubs
    // -----------------------------------------------------------------------
    static bool initSD();
    static bool openTripFile(const char* tripId);
    static bool writeSample(const GNSSSample& s);
    static bool finalizeTripBundle();

    // -----------------------------------------------------------------------
    // Connectivity stubs
    // -----------------------------------------------------------------------
    static bool scanForTrustedNetwork();
    static bool connectToHome();
    static bool uploadBundle(const char* tripId);

    // -----------------------------------------------------------------------
    // Power stubs
    // -----------------------------------------------------------------------
    static float getBatteryVoltage();
};

#endif // CAIRN_STATE_MACHINE_H
