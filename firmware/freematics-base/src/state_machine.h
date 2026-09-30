#ifndef CAIRN_STATE_MACHINE_H
#define CAIRN_STATE_MACHINE_H

#include <Arduino.h>
#include <SD.h>
#include <FS.h>
#include <FreematicsPlus.h>
#include "trip_types.h"
#include "config.h"

class OBDAdapter : public COBD {
protected:
    void idleTasks() override;
};

class StateMachine {
public:
    void init();
    void update();

    DeviceState getCurrentState() const;
    const char* getStateName() const;
    const char* getStateName(DeviceState state) const;

    // BLE command handler needs access to live data
    float getBatteryVoltage();
    void processBLE(int timeout);

    // Exposed for BLE queries
    float batteryVoltage_ = 0;
    char vin_[18] = {};
    uint16_t dtcCodes_[6] = {};
    int dtcCount_ = 0;
    int deviceTemp_ = 0;
    float acc_[3] = {};
    float gyr_[3] = {};
    int16_t rssi_ = 0;

    FreematicsESP32 sys_;
    MEMS_I2C* mems_ = nullptr;
    OBDAdapter obd_;

private:
    DeviceState state_ = DeviceState::SLEEP;

    unsigned long stateEnteredAt_  = 0;
    unsigned long lastGNSSRead_    = 0;
    unsigned long lastIMURead_     = 0;
    unsigned long lastWiFiScan_    = 0;
    unsigned long lastVoltageRead_ = 0;
    unsigned long lastMotionTime_  = 0;
    unsigned long lastGPSTick_     = 0;
    unsigned long lastRSSICheck_   = 0;

    uint32_t gnssSampleCount_      = 0;
    uint32_t imuSummaryCount_      = 0;
    uint32_t obdSampleCount_       = 0;
    char     currentTripId_[27]    = {};

    char tripDir_[64]     = {};
    char samplesPath_[80] = {};
    char imuPath_[80]     = {};
    char obdPath_[80]     = {};
    char healthPath_[80]  = {};

    // Hardware state
    GPS_DATA* gpsData_ = nullptr;
    bool gpsReady_ = false;
    bool memsReady_ = false;
    bool sdReady_ = false;
    bool obdReady_ = false;

    // Accelerometer calibration bias
    float accBias_[3] = {};

    // Adaptive data interval
    int32_t dataInterval_ = 1000;

    // Running SHA-256 context
    bool hashActive_ = false;

    // Arming ring buffer — captures samples before trip officially starts
    static constexpr size_t ARMING_BUF_SIZE = 128;
    GNSSSample armingGnss_[ARMING_BUF_SIZE];
    IMUSummary armingImu_[ARMING_BUF_SIZE];
    uint16_t   armingGnssCount_ = 0;
    uint16_t   armingImuCount_  = 0;

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

    // Initialization
    void initOBD();
    void initMEMS();
    void initGNSS();
    void calibrateMEMS();

    // Sensor reads
    bool        detectMotion();
    GNSSSample  readGNSS();
    IMUSummary  readIMU();
    OBDSnapshot readOBD();
    float       getSpeedKmh(const GNSSSample& s);
    bool        isMoving(const GNSSSample& s, const IMUSummary& imu);

    // Storage
    bool initSD();
    bool openTripFile(const char* tripId);
    bool writeSample(const GNSSSample& s);
    bool writeIMUSummary(const IMUSummary& s);
    bool writeOBDSnapshot(const OBDSnapshot& s);
    bool writeDeviceHealth(const DeviceHealth& h);
    bool finalizeTripBundle();

    // Connectivity
    bool scanForTrustedNetwork();
    bool connectToHome();
    bool uploadBundle(const char* tripId);

    // Standby
    void standby();
    bool waitMotion(long timeout);

    // Helpers
    void generateTripId(char* buf, size_t len);
    bool writeManifest();
    bool writeChecksums();
    void addEvent(TripEvent::EventType type, const char* details);
    void processExtInputs(uint16_t& s1, uint16_t& s2);
    void recoverOrphanedTrips();
    void flushArmingBuffer();

    static constexpr size_t MAX_EVENTS = 64;
    TripEvent events_[MAX_EVENTS];
    uint16_t  eventCount_ = 0;

    uint64_t tripStartMs_ = 0;
};

#endif // CAIRN_STATE_MACHINE_H
