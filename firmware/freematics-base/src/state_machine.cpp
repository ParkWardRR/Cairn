#include "state_machine.h"
#include "config.h"
#include <cstring>

// ===========================================================================
// Initialization
// ===========================================================================

void StateMachine::init() {
    state_ = DeviceState::SLEEP;
    stateEnteredAt_ = millis();
    gnssSampleCount_ = 0;
    imuSummaryCount_ = 0;
    memset(currentTripId_, 0, sizeof(currentTripId_));

    initSD();

    Serial.println("[STATE] Initialized -> SLEEP");
}

// ===========================================================================
// Main update loop -- dispatches to current state handler
// ===========================================================================

void StateMachine::update() {
    // Global battery check (any state except SLEEP and LOW_BATTERY_PROTECTION)
    if (state_ != DeviceState::SLEEP &&
        state_ != DeviceState::LOW_BATTERY_PROTECTION) {
        float voltage = getBatteryVoltage();
        if (voltage * 1000.0f < LOW_BATTERY_THRESHOLD_MV) {
            transitionTo(DeviceState::LOW_BATTERY_PROTECTION);
            return;
        }
    }

    switch (state_) {
        case DeviceState::SLEEP:                 handleSleep();              break;
        case DeviceState::ARMING:                handleArming();             break;
        case DeviceState::RECORDING:             handleRecording();          break;
        case DeviceState::STOP_CANDIDATE:        handleStopCandidate();      break;
        case DeviceState::FINALIZING:            handleFinalizing();         break;
        case DeviceState::QUEUED_FOR_HOME_SYNC:  handleQueuedForHomeSync();  break;
        case DeviceState::SYNCING:               handleSyncing();           break;
        case DeviceState::RETAINED:              handleRetained();           break;
        case DeviceState::LOW_BATTERY_PROTECTION:handleLowBatteryProtection(); break;
        case DeviceState::FAULT:                 handleFault();              break;
        default:                                 transitionTo(DeviceState::FAULT); break;
    }
}

// ===========================================================================
// State handlers
// ===========================================================================

void StateMachine::handleSleep() {
    // Wait for motion interrupt (stub)
    if (detectMotion()) {
        transitionTo(DeviceState::ARMING);
    }
}

void StateMachine::handleArming() {
    unsigned long elapsed = millis() - stateEnteredAt_;

    GNSSSample gnss = readGNSS();
    IMUSummary imu  = readIMU();
    float speed     = getSpeedKmh(gnss);

    bool conditionsMet = (speed >= MIN_SPEED_KMH) && isMoving(gnss, imu);

    if (!conditionsMet) {
        // Conditions dropped before the arming window elapsed -- back to sleep
        if (elapsed > ARMING_DURATION_MS) {
            transitionTo(DeviceState::SLEEP);
        }
        return;
    }

    if (elapsed >= ARMING_DURATION_MS) {
        // Sustained motion confirmed -- start recording
        // Generate a placeholder trip ID (real implementation would create a ULID)
        snprintf(currentTripId_, sizeof(currentTripId_), "STUB_%010lu", millis());
        openTripFile(currentTripId_);
        gnssSampleCount_ = 0;
        imuSummaryCount_ = 0;
        transitionTo(DeviceState::RECORDING);
    }
}

void StateMachine::handleRecording() {
    unsigned long now = millis();

    // Check max trip duration
    if (now - stateEnteredAt_ >= MAX_TRIP_DURATION_MS) {
        Serial.println("[RECORDING] Max trip duration reached");
        transitionTo(DeviceState::FINALIZING);
        return;
    }

    // GNSS read at active rate
    uint16_t gnssIntervalMs = (GNSS_RATE_ACTIVE_HZ > 0) ? (1000 / GNSS_RATE_ACTIVE_HZ) : 0;
    if (gnssIntervalMs > 0 && (now - lastGNSSRead_ >= gnssIntervalMs)) {
        GNSSSample sample = readGNSS();
        writeSample(sample);
        gnssSampleCount_++;
        lastGNSSRead_ = now;

        // Check if vehicle has stopped
        float speed = getSpeedKmh(sample);
        IMUSummary imu = readIMU();
        imuSummaryCount_++;
        lastIMURead_ = now;

        if (speed < MIN_SPEED_KMH && !isMoving(sample, imu)) {
            transitionTo(DeviceState::STOP_CANDIDATE);
            return;
        }
    }

    // IMU read at active rate (independent of GNSS, if interval has elapsed)
    uint16_t imuIntervalMs = (IMU_RATE_ACTIVE_HZ > 0) ? (1000 / IMU_RATE_ACTIVE_HZ) : 0;
    if (imuIntervalMs > 0 && (now - lastIMURead_ >= imuIntervalMs)) {
        readIMU();
        imuSummaryCount_++;
        lastIMURead_ = now;
    }
}

void StateMachine::handleStopCandidate() {
    unsigned long elapsed = millis() - stateEnteredAt_;

    // Continue recording at reduced rate
    unsigned long now = millis();
    uint16_t gnssIntervalMs = (GNSS_RATE_SLOW_HZ > 0) ? (1000 / GNSS_RATE_SLOW_HZ) : 0;
    if (gnssIntervalMs > 0 && (now - lastGNSSRead_ >= gnssIntervalMs)) {
        GNSSSample sample = readGNSS();
        writeSample(sample);
        gnssSampleCount_++;
        lastGNSSRead_ = now;

        // Check if movement has resumed
        IMUSummary imu = readIMU();
        imuSummaryCount_++;
        lastIMURead_ = now;

        float speed = getSpeedKmh(sample);
        if (speed >= MIN_SPEED_KMH || isMoving(sample, imu)) {
            transitionTo(DeviceState::RECORDING);
            return;
        }
    }

    // Dwell exceeded -- trip is over
    if (elapsed >= STOP_DWELL_MS) {
        transitionTo(DeviceState::FINALIZING);
    }
}

void StateMachine::handleFinalizing() {
    bool ok = finalizeTripBundle();
    if (ok) {
        Serial.println("[FINALIZING] Trip bundle written successfully");
        transitionTo(DeviceState::QUEUED_FOR_HOME_SYNC);
    } else {
        Serial.println("[FINALIZING] Failed to write trip bundle");
        transitionTo(DeviceState::FAULT);
    }
}

void StateMachine::handleQueuedForHomeSync() {
    unsigned long now = millis();

    // Periodically scan for the trusted Wi-Fi network
    if (now - lastWiFiScan_ >= WIFI_SCAN_INTERVAL_MS) {
        lastWiFiScan_ = now;

        if (scanForTrustedNetwork()) {
            if (connectToHome()) {
                transitionTo(DeviceState::SYNCING);
                return;
            }
        }
    }

    // If we have been waiting a long time without Wi-Fi, go to sleep to save
    // power. The next motion event will wake us and we will try again after the
    // trip completes.
    unsigned long waitTime = now - stateEnteredAt_;
    if (waitTime > WIFI_SCAN_INTERVAL_MS * 10) {
        Serial.println("[QUEUED] No network found -- entering SLEEP to save power");
        transitionTo(DeviceState::SLEEP);
    }
}

void StateMachine::handleSyncing() {
    bool ok = uploadBundle(currentTripId_);
    if (ok) {
        Serial.println("[SYNCING] Upload complete");
        transitionTo(DeviceState::RETAINED);
    } else {
        Serial.println("[SYNCING] Upload failed -- requeueing");
        transitionTo(DeviceState::QUEUED_FOR_HOME_SYNC);
    }
}

void StateMachine::handleRetained() {
    // After the retention window the trip becomes prunable.
    // RETENTION_WINDOW_HOURS is in hours; convert to ms.
    unsigned long retentionMs = (unsigned long)RETENTION_WINDOW_HOURS * 3600UL * 1000UL;
    if (millis() - stateEnteredAt_ >= retentionMs) {
        transitionTo(DeviceState::PRUNABLE);
    }
}

void StateMachine::handleLowBatteryProtection() {
    // If we were recording, the transition already logged. In a real
    // implementation we would finalize the active trip, disable radios, and
    // enter ESP32 deep sleep.
    Serial.println("[LOW_BATTERY] Finalizing trip if active, entering deep sleep");
    finalizeTripBundle();
    // esp_deep_sleep_start();  // placeholder
}

void StateMachine::handleFault() {
    // Log the fault and wait -- a watchdog / reboot will attempt recovery.
    Serial.println("[FAULT] Error state -- awaiting reboot for recovery");
    delay(5000);
}

// ===========================================================================
// Transition helper
// ===========================================================================

void StateMachine::transitionTo(DeviceState next) {
    Serial.print("[STATE] ");
    Serial.print(getStateName(state_));
    Serial.print(" -> ");
    Serial.println(getStateName(next));

    state_ = next;
    stateEnteredAt_ = millis();
}

// ===========================================================================
// State name lookup
// ===========================================================================

DeviceState StateMachine::getCurrentState() const {
    return state_;
}

const char* StateMachine::getStateName() const {
    return getStateName(state_);
}

const char* StateMachine::getStateName(DeviceState state) const {
    switch (state) {
        case DeviceState::SLEEP:                  return "SLEEP";
        case DeviceState::ARMING:                 return "ARMING";
        case DeviceState::RECORDING:              return "RECORDING";
        case DeviceState::STOP_CANDIDATE:         return "STOP_CANDIDATE";
        case DeviceState::FINALIZING:             return "FINALIZING";
        case DeviceState::QUEUED_FOR_HOME_SYNC:   return "QUEUED_FOR_HOME_SYNC";
        case DeviceState::SYNCING:                return "SYNCING";
        case DeviceState::RETAINED:               return "RETAINED";
        case DeviceState::PRUNABLE:               return "PRUNABLE";
        case DeviceState::LOW_BATTERY_PROTECTION: return "LOW_BATTERY_PROTECTION";
        case DeviceState::FAULT:                  return "FAULT";
        default:                                  return "UNKNOWN";
    }
}

// ===========================================================================
// Sensor stubs
// ===========================================================================

bool StateMachine::detectMotion() {
    // Stub: no motion detected
    return false;
}

GNSSSample StateMachine::readGNSS() {
    GNSSSample s;
    memset(&s, 0, sizeof(s));
    return s;
}

IMUSummary StateMachine::readIMU() {
    IMUSummary s;
    memset(&s, 0, sizeof(s));
    return s;
}

float StateMachine::getSpeedKmh(const GNSSSample& s) {
    // speed_cmps is cm/s. 1 km/h = 100/3.6 cm/s => km/h = cmps * 3.6 / 100
    return static_cast<float>(s.speed_cmps) * 0.036f;
}

bool StateMachine::isMoving(const GNSSSample& s, const IMUSummary& imu) {
    // Simple threshold: speed above MIN_SPEED_KMH or noticeable accel RMS
    if (getSpeedKmh(s) >= MIN_SPEED_KMH) return true;
    if (imu.accel_rms_mg > 200) return true;  // > 0.2 g RMS
    return false;
}

// ===========================================================================
// Storage stubs
// ===========================================================================

bool StateMachine::initSD() {
    Serial.println("[SD] Initialized (stub)");
    return true;
}

bool StateMachine::openTripFile(const char* tripId) {
    Serial.print("[SD] Opened trip file: ");
    Serial.println(tripId);
    return true;
}

bool StateMachine::writeSample(const GNSSSample& /* s */) {
    return true;
}

bool StateMachine::finalizeTripBundle() {
    Serial.println("[SD] Trip bundle finalized (stub)");
    return true;
}

// ===========================================================================
// Connectivity stubs
// ===========================================================================

bool StateMachine::scanForTrustedNetwork() {
    // Stub: no network found
    return false;
}

bool StateMachine::connectToHome() {
    // Stub: connection fails
    return false;
}

bool StateMachine::uploadBundle(const char* /* tripId */) {
    // Stub: upload fails
    return false;
}

// ===========================================================================
// Power stubs
// ===========================================================================

float StateMachine::getBatteryVoltage() {
    // Stub: return a healthy 12.6 V
    return 12.6f;
}
