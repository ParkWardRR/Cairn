#include "state_machine.h"
#include "config.h"
#include <cstring>
#include <cstdio>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <ArduinoJson.h>
#include "mbedtls/sha256.h"
#include "nvs_flash.h"
#include "nvs.h"
#include "driver/adc.h"

// BLE functions declared in FreematicsPlus.h via ble_spp_server.h

static nvs_handle_t nvsHandle;
static char wifiSSID[32] = WIFI_SSID;
static char wifiPassword[32] = WIFI_PASSWORD;

// OBD PIDs to poll, organized in tiers (stock firmware pattern)
struct PIDEntry {
    byte pid;
    byte tier;
    int value;
    uint32_t ts;
};

static PIDEntry obdData[] = {
    {PID_SPEED, 1},
    {PID_RPM, 1},
    {PID_THROTTLE, 1},
    {PID_ENGINE_LOAD, 1},
    {PID_FUEL_PRESSURE, 2},
    {PID_TIMING_ADVANCE, 2},
    {PID_COOLANT_TEMP, 3},
    {PID_INTAKE_TEMP, 3},
};

// OBDAdapter idle task — read IMU while waiting for ECU responses
void OBDAdapter::idleTasks() {}

// ===========================================================================
// NVS config loading
// ===========================================================================

static void loadConfig() {
    size_t len;
    len = sizeof(wifiSSID);
    nvs_get_str(nvsHandle, "WIFI_SSID", wifiSSID, &len);
    len = sizeof(wifiPassword);
    nvs_get_str(nvsHandle, "WIFI_PWD", wifiPassword, &len);
}

// ===========================================================================
// Initialization
// ===========================================================================

void StateMachine::init() {
    state_ = DeviceState::SLEEP;
    stateEnteredAt_ = millis();
    gnssSampleCount_ = 0;
    imuSummaryCount_ = 0;
    obdSampleCount_ = 0;
    eventCount_ = 0;
    memset(currentTripId_, 0, sizeof(currentTripId_));

    // Init NVS
    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        nvs_flash_erase();
        err = nvs_flash_init();
    }
    if (nvs_open("storage", NVS_READWRITE, &nvsHandle) == ESP_OK) {
        loadConfig();
    }

    // Init OBD coprocessor link
    initOBD();

    // Init IMU
    initMEMS();

    // Init GNSS
    initGNSS();

    // Init SD card (SPI, CS=GPIO5)
    sdReady_ = SD.begin(PIN_SD_CS);
    if (sdReady_) {
        Serial.printf("[SD] Mounted: %llu MB total, %llu MB used\n",
                      SD.totalBytes() / (1024*1024), SD.usedBytes() / (1024*1024));
        SD.mkdir("/cairn");
        SD.mkdir(TRIP_BASE_PATH);
        recoverOrphanedTrips();
    } else {
        Serial.println("[SD] Card mount failed");
    }

    // Read initial battery voltage
    batteryVoltage_ = getBatteryVoltage();
    Serial.printf("[PWR] Battery: %.1f V\n", batteryVoltage_);

#if LOG_EXT_SENSORS == 2
    adc1_config_width(ADC_WIDTH_BIT_12);
    adc1_config_channel_atten(ADC1_CHANNEL_0, ADC_ATTEN_DB_11);
    adc1_config_channel_atten(ADC1_CHANNEL_1, ADC_ATTEN_DB_11);
#elif LOG_EXT_SENSORS == 1
    pinMode(PIN_SENSOR1, INPUT);
    pinMode(PIN_SENSOR2, INPUT);
#endif

#if ENABLE_BLE
    ble_init("Cairn");
#endif

    lastMotionTime_ = millis();
    Serial.println("[STATE] Initialized -> SLEEP");
}

void StateMachine::initOBD() {
#if ENABLE_OBD
    if (sys_.begin()) {
        Serial.printf("[SYS] Device type: %u\n", sys_.devType);
        obd_.begin(sys_.link);

        if (obd_.init()) {
            obdReady_ = true;
            Serial.println("[OBD] ECU connected");

            char buf[128];
            if (obd_.getVIN(buf, sizeof(buf))) {
                memcpy(vin_, buf, sizeof(vin_) - 1);
                Serial.printf("[OBD] VIN: %s\n", vin_);
            }

            dtcCount_ = obd_.readDTC(dtcCodes_, sizeof(dtcCodes_) / sizeof(dtcCodes_[0]));
            if (dtcCount_ > 0) {
                Serial.printf("[OBD] DTCs: %d\n", dtcCount_);
            }
        } else {
            Serial.println("[OBD] ECU not responding (ignition off?)");
        }
    } else {
        Serial.println("[SYS] Coprocessor init failed");
        sys_.begin(false, false);
    }
#else
    sys_.begin(false, false);
#endif
}

void StateMachine::initMEMS() {
#if ENABLE_MEMS
    mems_ = new ICM_42627;
    byte ret = mems_->begin();
    if (ret) {
        memsReady_ = true;
        Serial.println("[IMU] ICM-42627");
        calibrateMEMS();
    } else {
        delete mems_;
        mems_ = nullptr;
        memsReady_ = false;
        Serial.println("[IMU] No sensor found");
    }
#endif
}

void StateMachine::initGNSS() {
    if (sys_.gpsBeginExt()) {
        gpsReady_ = true;
        Serial.println("[GNSS] OK (external)");
    } else if (sys_.gpsBegin()) {
        gpsReady_ = true;
        Serial.println("[GNSS] OK (internal)");
    } else {
        gpsReady_ = false;
        Serial.println("[GNSS] Not available");
    }
    lastGPSTick_ = millis();
}

void StateMachine::calibrateMEMS() {
    if (!memsReady_) return;
    accBias_[0] = accBias_[1] = accBias_[2] = 0;
    int n = 0;
    unsigned long t = millis();
    for (; millis() - t < 1000; n++) {
        float a[3];
        if (!mems_->read(a)) continue;
        accBias_[0] += a[0];
        accBias_[1] += a[1];
        accBias_[2] += a[2];
        delay(10);
    }
    if (n > 0) {
        accBias_[0] /= n;
        accBias_[1] /= n;
        accBias_[2] /= n;
    }
    Serial.printf("[IMU] Bias: %.2f/%.2f/%.2f (%d samples)\n",
                  accBias_[0], accBias_[1], accBias_[2], n);
}

// ===========================================================================
// Main update loop
// ===========================================================================

void StateMachine::update() {
    unsigned long now = millis();

    // Periodic battery voltage check
    if (now - lastVoltageRead_ >= 10000) {
        batteryVoltage_ = getBatteryVoltage();
        lastVoltageRead_ = now;
    }

    // Low battery protection
    if (state_ != DeviceState::SLEEP &&
        state_ != DeviceState::LOW_BATTERY_PROTECTION) {
        if (batteryVoltage_ > 0 && batteryVoltage_ < LOW_BATTERY_THRESHOLD_V) {
            transitionTo(DeviceState::LOW_BATTERY_PROTECTION);
            return;
        }
    }

    // BLE command processing
    processBLE(0);

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
    if (memsReady_ && detectMotion()) {
        transitionTo(DeviceState::ARMING);
        return;
    }

    if (batteryVoltage_ >= ENGINE_ON_VOLTAGE_V) {
        Serial.printf("[SLEEP] Engine voltage detected: %.1f V\n", batteryVoltage_);
        transitionTo(DeviceState::ARMING);
        return;
    }

    delay(500);
}

void StateMachine::handleArming() {
    unsigned long elapsed = millis() - stateEnteredAt_;

    // Try to connect OBD if not ready (engine may have just started)
#if ENABLE_OBD
    if (!obdReady_) {
        if (obd_.init(PROTO_AUTO, true)) {
            obdReady_ = true;
            Serial.println("[OBD] ECU ON");
            addEvent(TripEvent::ECU_ON, "ECU connected during arming");
        }
    }
#endif

    GNSSSample gnss = readGNSS();
    IMUSummary imu  = readIMU();
    float speed     = getSpeedKmh(gnss);

    // Buffer samples so they aren't lost if trip starts
    if (armingGnssCount_ < ARMING_BUF_SIZE) {
        armingGnss_[armingGnssCount_++] = gnss;
    }
    if (armingImuCount_ < ARMING_BUF_SIZE) {
        armingImu_[armingImuCount_++] = imu;
    }

    bool conditionsMet = (speed >= MIN_SPEED_KMH) || isMoving(gnss, imu);

    if (!conditionsMet) {
        if (elapsed > ARMING_DURATION_MS) {
            armingGnssCount_ = 0;
            armingImuCount_ = 0;
            transitionTo(DeviceState::SLEEP);
        }
        return;
    }

    if (elapsed >= ARMING_DURATION_MS) {
        generateTripId(currentTripId_, sizeof(currentTripId_));
        if (!openTripFile(currentTripId_)) {
            transitionTo(DeviceState::FAULT);
            return;
        }

        gnssSampleCount_ = 0;
        imuSummaryCount_ = 0;
        obdSampleCount_ = 0;
        eventCount_ = 0;
        tripStartMs_ = millis();
        dataInterval_ = DATA_INTERVAL_TABLE[0];

        flushArmingBuffer();

        addEvent(TripEvent::TRIP_START, "recording started");
        transitionTo(DeviceState::RECORDING);
    }
}

void StateMachine::handleRecording() {
    unsigned long now = millis();

    if (now - stateEnteredAt_ >= MAX_TRIP_DURATION_MS) {
        Serial.println("[RECORDING] Max trip duration reached");
        addEvent(TripEvent::TRIP_END, "max duration");
        transitionTo(DeviceState::FINALIZING);
        return;
    }

    // --- OBD polling ---
#if ENABLE_OBD
    if (obdReady_) {
        OBDSnapshot snap = readOBD();
        writeOBDSnapshot(snap);
        obdSampleCount_++;

        if (snap.speed_kph >= 2) lastMotionTime_ = now;

        if (obd_.errors >= MAX_OBD_ERRORS) {
            if (!obd_.init()) {
                Serial.println("[OBD] ECU OFF");
                addEvent(TripEvent::ECU_OFF, "ECU stopped responding");
                obdReady_ = false;
            }
        }
    } else {
        if (obd_.init(PROTO_AUTO, true)) {
            obdReady_ = true;
            Serial.println("[OBD] ECU ON");
            addEvent(TripEvent::ECU_ON, "ECU reconnected");
        }
    }
#endif

    // --- GNSS ---
    uint16_t gnssIntervalMs = (GNSS_RATE_ACTIVE_HZ > 0) ? (1000 / GNSS_RATE_ACTIVE_HZ) : 0;
    if (gnssIntervalMs > 0 && (now - lastGNSSRead_ >= gnssIntervalMs)) {
        GNSSSample sample = readGNSS();
        bool gotFix = (sample.satellites > 0 && sample.latitude != 0);
        writeSample(sample);
        gnssSampleCount_++;
        lastGNSSRead_ = now;

        float speed = getSpeedKmh(sample);
        if (speed >= 2) lastMotionTime_ = now;

        // GNSS watchdog
        if (gotFix) {
            lastGPSTick_ = now;
        } else if (GNSS_RESET_TIMEOUT_S > 0 &&
                   now - lastGPSTick_ > (unsigned long)GNSS_RESET_TIMEOUT_S * 1000) {
            Serial.println("[GNSS] Watchdog reset");
            addEvent(TripEvent::GNSS_RESET, "no fix timeout");
            sys_.gpsEnd();
            gpsReady_ = false;
            delay(20);
            initGNSS();
            lastGPSTick_ = now;
        }
    }

    // --- IMU ---
    uint16_t imuIntervalMs = (IMU_RATE_ACTIVE_HZ > 0) ? (1000 / IMU_RATE_ACTIVE_HZ) : 0;
    if (imuIntervalMs > 0 && (now - lastIMURead_ >= imuIntervalMs)) {
        IMUSummary imu = readIMU();
        writeIMUSummary(imu);
        imuSummaryCount_++;
        lastIMURead_ = now;
    }

    // --- Device health (battery, temp, RSSI, ext sensors) ---
    DeviceHealth health;
    memset(&health, 0, sizeof(health));
    health.timestamp_ms = now;
    health.battery_mv = (uint16_t)(batteryVoltage_ * 1000);
    health.device_temp_c = (int8_t)deviceTemp_;
    health.rssi_dbm = (int8_t)rssi_;
    processExtInputs(health.ext_sensor_1, health.ext_sensor_2);
    writeDeviceHealth(health);

    // Thermal throttle
    if (deviceTemp_ >= COOLING_DOWN_TEMP_C) {
        Serial.printf("[THERMAL] High device temp: %d C\n", deviceTemp_);
        addEvent(TripEvent::THERMAL_THROTTLE, "thermal throttle");
    }

    // --- Adaptive interval / stationary detection ---
    unsigned int motionlessSec = (now - lastMotionTime_) / 1000;
    bool stationary = true;
    for (uint8_t i = 0; i < STATIONARY_TIERS; i++) {
        dataInterval_ = DATA_INTERVAL_TABLE[i];
        if (motionlessSec < STATIONARY_TIME_TABLE[i] || STATIONARY_TIME_TABLE[i] == 0) {
            stationary = false;
            break;
        }
    }
    if (stationary) {
        Serial.printf("[RECORDING] Stationary for %u secs — ending trip\n", motionlessSec);
        addEvent(TripEvent::TRIP_END, "stationary timeout");
        transitionTo(DeviceState::FINALIZING);
        return;
    }

    // RSSI monitoring
    if (now - lastRSSICheck_ >= (unsigned long)SIGNAL_CHECK_INTERVAL_S * 1000) {
        if (WiFi.status() == WL_CONNECTED) {
            rssi_ = WiFi.RSSI();
        }
        lastRSSICheck_ = now;
    }

    processBLE(0);
}

void StateMachine::handleStopCandidate() {
    unsigned long elapsed = millis() - stateEnteredAt_;
    unsigned long now = millis();

    uint16_t gnssIntervalMs = (GNSS_RATE_SLOW_HZ > 0) ? (1000 / GNSS_RATE_SLOW_HZ) : 0;
    if (gnssIntervalMs > 0 && (now - lastGNSSRead_ >= gnssIntervalMs)) {
        GNSSSample sample = readGNSS();
        writeSample(sample);
        gnssSampleCount_++;
        lastGNSSRead_ = now;

        IMUSummary imu = readIMU();
        writeIMUSummary(imu);
        imuSummaryCount_++;
        lastIMURead_ = now;

        float speed = getSpeedKmh(sample);
        if (speed >= MIN_SPEED_KMH || isMoving(sample, imu)) {
            lastMotionTime_ = now;
            addEvent(TripEvent::TRIP_RESUMED, "motion resumed");
            transitionTo(DeviceState::RECORDING);
            return;
        }
    }

    if (elapsed >= STOP_DWELL_MS) {
        addEvent(TripEvent::TRIP_END, "dwell timeout");
        transitionTo(DeviceState::FINALIZING);
    }
}

void StateMachine::handleFinalizing() {
    bool ok = finalizeTripBundle();
    if (ok) {
        Serial.println("[FINALIZING] Trip bundle written");
        transitionTo(DeviceState::QUEUED_FOR_HOME_SYNC);
    } else {
        Serial.println("[FINALIZING] Failed to write trip bundle");
        transitionTo(DeviceState::FAULT);
    }
}

void StateMachine::handleQueuedForHomeSync() {
    unsigned long now = millis();

    if (now - lastWiFiScan_ >= WIFI_SCAN_INTERVAL_MS) {
        lastWiFiScan_ = now;

        if (scanForTrustedNetwork()) {
            if (connectToHome()) {
                transitionTo(DeviceState::SYNCING);
                return;
            }
        }
    }

    if (now - stateEnteredAt_ > WIFI_SCAN_INTERVAL_MS * 10) {
        Serial.println("[QUEUED] No network found — entering standby");
        standby();
    }
}

void StateMachine::handleSyncing() {
    bool ok = uploadBundle(currentTripId_);
    if (ok) {
        Serial.println("[SYNCING] Upload complete");
        WiFi.disconnect(true);
        transitionTo(DeviceState::RETAINED);
    } else {
        Serial.println("[SYNCING] Upload failed — requeueing");
        WiFi.disconnect(true);
        transitionTo(DeviceState::QUEUED_FOR_HOME_SYNC);
    }
}

void StateMachine::handleRetained() {
    unsigned long retentionMs = (unsigned long)RETENTION_WINDOW_HOURS * 3600UL * 1000UL;
    if (millis() - stateEnteredAt_ >= retentionMs) {
        char path[80];
        snprintf(path, sizeof(path), "%s/%s", TRIP_BASE_PATH, currentTripId_);
        SD.rmdir(path);
        transitionTo(DeviceState::PRUNABLE);
    }
}

void StateMachine::handleLowBatteryProtection() {
    Serial.println("[LOW_BATTERY] Finalizing trip, entering deep sleep");
    addEvent(TripEvent::POWER_ANOMALY, "low battery protection");
    finalizeTripBundle();
#if ENABLE_OBD
    obd_.enterLowPowerMode();
#endif
    esp_deep_sleep_start();
}

void StateMachine::handleFault() {
    Serial.println("[FAULT] Error state — rebooting in 5s");
    delay(5000);
    esp_restart();
}

// ===========================================================================
// Standby — harvested from stock firmware
// ===========================================================================

void StateMachine::standby() {
    Serial.println("[STANDBY] Entering standby");

    // Close log file
    if (sdReady_) {
        // SD files are closed per-write, nothing to do
    }

    // Turn off GNSS if configured
    if (!GNSS_ALWAYS_ON && gpsReady_) {
        Serial.println("[GNSS] OFF");
        sys_.gpsEnd(true);
        gpsReady_ = false;
        gpsData_ = nullptr;
    }

    obdReady_ = false;

    // Put coprocessor to sleep
#if ENABLE_OBD
    obd_.enterLowPowerMode();
#endif

    Serial.println("[STANDBY] Waiting for motion or jumpstart...");

    // Calibrate IMU before standby for accurate motion detection
    calibrateMEMS();

    // Block until motion detected or voltage spike
    if (memsReady_) {
        waitMotion(-1);
    } else {
        // Fallback: poll voltage for engine crank
        while (true) {
            delay(5000);
            float v = getBatteryVoltage();
            if (v >= JUMPSTART_VOLTAGE_V) {
                Serial.printf("[STANDBY] Jumpstart voltage: %.1f V\n", v);
                break;
            }
            processBLE(0);
        }
    }

    Serial.println("[STANDBY] WAKEUP");
    sys_.resetLink();

    if (RESET_AFTER_WAKEUP) {
#if ENABLE_MEMS
        if (mems_) mems_->end();
#endif
        ESP.restart();
    }

    // Re-init everything if not resetting
    initOBD();
    initGNSS();
    calibrateMEMS();
    lastMotionTime_ = millis();
    state_ = DeviceState::SLEEP;
    stateEnteredAt_ = millis();
}

bool StateMachine::waitMotion(long timeout) {
    if (!memsReady_) return false;
    unsigned long t = millis();
    do {
        float a[3];
        if (!mems_->read(a)) continue;

        float motion = 0;
        for (byte i = 0; i < 3; i++) {
            float m = a[i] - accBias_[i];
            motion += m * m;
        }

        processBLE(100);

        if (motion >= MOTION_THRESHOLD_G * MOTION_THRESHOLD_G) {
            return true;
        }
    } while ((long)(millis() - t) < timeout || timeout == -1);
    return false;
}

// ===========================================================================
// Transition helper
// ===========================================================================

void StateMachine::transitionTo(DeviceState next) {
    Serial.printf("[STATE] %s -> %s\n", getStateName(state_), getStateName(next));
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
// Sensor reads
// ===========================================================================

bool StateMachine::detectMotion() {
    if (!memsReady_) return false;

    float a[3] = {0};
    mems_->read(a);

    float motion = 0;
    for (byte i = 0; i < 3; i++) {
        float m = a[i] - accBias_[i];
        motion += m * m;
    }
    return motion >= MOTION_THRESHOLD_G * MOTION_THRESHOLD_G;
}

GNSSSample StateMachine::readGNSS() {
    GNSSSample s;
    memset(&s, 0, sizeof(s));

    if (!gpsReady_) return s;
    if (!sys_.gpsGetData(&gpsData_)) return s;
    if (!gpsData_) return s;

    s.timestamp_ms = millis();
    s.latitude     = (int32_t)(gpsData_->lat * 1e7);
    s.longitude    = (int32_t)(gpsData_->lng * 1e7);
    s.altitude_cm  = (int32_t)(gpsData_->alt * 100);
    s.speed_cmps   = (uint16_t)(gpsData_->speed * 51.4444f);
    s.heading_cdeg = (uint16_t)(gpsData_->heading * 100);
    s.satellites   = gpsData_->sat;
    s.hdop_tenths  = (uint16_t)(gpsData_->hdop * 10);
    s.fix_quality  = (gpsData_->sat > 0) ? 1 : 0;

    float kph = gpsData_->speed * 1.852f;
    if (kph >= 2) lastMotionTime_ = millis();

    return s;
}

IMUSummary StateMachine::readIMU() {
    IMUSummary summary;
    memset(&summary, 0, sizeof(summary));

    if (!memsReady_) return summary;

    float temp;
    mems_->read(acc_, gyr_, nullptr, &temp);
    deviceTemp_ = (int)temp;

    summary.window_start_ms = millis();
    summary.window_duration_ms = 20;

    summary.accel_peak_x_mg = (int16_t)((acc_[0] - accBias_[0]) * 1000.0f);
    summary.accel_peak_y_mg = (int16_t)((acc_[1] - accBias_[1]) * 1000.0f);
    summary.accel_peak_z_mg = (int16_t)((acc_[2] - accBias_[2]) * 1000.0f);

    float dx = acc_[0] - accBias_[0];
    float dy = acc_[1] - accBias_[1];
    float dz = acc_[2] - accBias_[2];
    float mag_mg = sqrtf(dx*dx + dy*dy + dz*dz) * 1000.0f;
    summary.accel_rms_mg = (uint16_t)mag_mg;

    float gyro_max = fmaxf(fmaxf(fabsf(gyr_[0]), fabsf(gyr_[1])), fabsf(gyr_[2]));
    summary.gyro_peak_dps = (int16_t)(gyro_max * 10.0f);

    summary.variance = (uint16_t)mag_mg;

    summary.flags = 0;
    float total_g = sqrtf(acc_[0]*acc_[0] + acc_[1]*acc_[1] + acc_[2]*acc_[2]);
    if (total_g > IMPACT_THRESHOLD_G) summary.flags |= 0x01;
    if (dx < -HARD_BRAKE_THRESHOLD_G) summary.flags |= 0x02;
    if (fabsf(dy) > SHARP_TURN_THRESHOLD_G) summary.flags |= 0x04;

    return summary;
}

OBDSnapshot StateMachine::readOBD() {
    OBDSnapshot snap;
    memset(&snap, 0, sizeof(snap));
    snap.timestamp_ms = millis();

#if ENABLE_OBD
    if (!obdReady_) return snap;

    // Tiered polling — stock firmware pattern
    static int tierIdx[2] = {0, 0};
    int tier = 1;
    for (byte i = 0; i < sizeof(obdData) / sizeof(obdData[0]); i++) {
        if (obdData[i].tier > tier) {
            tierIdx[tier - 2] = 0;
            tier = obdData[i].tier;
            i += tierIdx[tier - 2]++;
            if (i >= sizeof(obdData) / sizeof(obdData[0]) || obdData[i].tier != tier) {
                tierIdx[tier - 2] = 0;
                i--;
                continue;
            }
        }
        byte pid = obdData[i].pid;
        if (!obd_.isValidPID(pid)) continue;
        int value;
        if (obd_.readPID(pid, value)) {
            obdData[i].ts = millis();
            obdData[i].value = value;
        } else {
            break;
        }
        if (tier > 1) break;
    }

    // Map cached values to snapshot
    for (byte i = 0; i < sizeof(obdData) / sizeof(obdData[0]); i++) {
        switch (obdData[i].pid) {
            case PID_SPEED:          snap.speed_kph = obdData[i].value; break;
            case PID_RPM:            snap.rpm = obdData[i].value; break;
            case PID_THROTTLE:       snap.throttle_pct = obdData[i].value; break;
            case PID_ENGINE_LOAD:    snap.engine_load_pct = obdData[i].value; break;
            case PID_COOLANT_TEMP:   snap.coolant_temp_c = obdData[i].value; break;
            case PID_INTAKE_TEMP:    snap.intake_temp_c = obdData[i].value; break;
            case PID_FUEL_PRESSURE:  snap.fuel_pressure_kpa = obdData[i].value; break;
            case PID_TIMING_ADVANCE: snap.timing_advance_deg = obdData[i].value; break;
        }
    }
#endif
    return snap;
}

float StateMachine::getSpeedKmh(const GNSSSample& s) {
    return static_cast<float>(s.speed_cmps) * 0.036f;
}

bool StateMachine::isMoving(const GNSSSample& s, const IMUSummary& imu) {
    if (getSpeedKmh(s) >= MIN_SPEED_KMH) return true;
    if (imu.accel_rms_mg > ACCEL_RMS_MOVING_MG) return true;
    return false;
}

// ===========================================================================
// External sensor inputs
// ===========================================================================

void StateMachine::processExtInputs(uint16_t& s1, uint16_t& s2) {
    s1 = 0;
    s2 = 0;
#if LOG_EXT_SENSORS == 1
    s1 = digitalRead(PIN_SENSOR1);
    s2 = digitalRead(PIN_SENSOR2);
#elif LOG_EXT_SENSORS == 2
    s1 = adc1_get_raw(ADC1_CHANNEL_0);
    s2 = adc1_get_raw(ADC1_CHANNEL_1);
#endif
}

// ===========================================================================
// Storage — Arduino SD via SPI
// ===========================================================================

void StateMachine::flushArmingBuffer() {
    if (armingGnssCount_ > 0) {
        Serial.printf("[SD] Flushing %u arming GNSS samples\n", armingGnssCount_);
        for (uint16_t i = 0; i < armingGnssCount_; i++) {
            writeSample(armingGnss_[i]);
            gnssSampleCount_++;
        }
        armingGnssCount_ = 0;
    }
    if (armingImuCount_ > 0) {
        Serial.printf("[SD] Flushing %u arming IMU samples\n", armingImuCount_);
        for (uint16_t i = 0; i < armingImuCount_; i++) {
            writeIMUSummary(armingImu_[i]);
            imuSummaryCount_++;
        }
        armingImuCount_ = 0;
    }
}

void StateMachine::recoverOrphanedTrips() {
    if (!sdReady_) return;

    File root = SD.open(TRIP_BASE_PATH);
    if (!root || !root.isDirectory()) return;

    int recovered = 0;
    File entry;
    while ((entry = root.openNextFile())) {
        if (!entry.isDirectory()) { entry.close(); continue; }

        const char* name = entry.name();
        entry.close();

        char manifestPath[96];
        snprintf(manifestPath, sizeof(manifestPath), "%s/%s/manifest.json",
                 TRIP_BASE_PATH, name);
        char samplesPath[96];
        snprintf(samplesPath, sizeof(samplesPath), "%s/%s/samples.bin",
                 TRIP_BASE_PATH, name);

        bool hasManifest = SD.exists(manifestPath);
        bool hasSamples  = SD.exists(samplesPath);

        if (hasSamples && !hasManifest) {
            Serial.printf("[RECOVERY] Orphaned trip: %s\n", name);

            // Set up paths so finalize helpers work
            strncpy(currentTripId_, name, sizeof(currentTripId_) - 1);
            snprintf(tripDir_, sizeof(tripDir_), "%s/%s", TRIP_BASE_PATH, name);
            snprintf(samplesPath_, sizeof(samplesPath_), "%s/samples.bin", tripDir_);
            snprintf(imuPath_, sizeof(imuPath_), "%s/imu_summary.bin", tripDir_);
            snprintf(obdPath_, sizeof(obdPath_), "%s/obd.bin", tripDir_);
            snprintf(healthPath_, sizeof(healthPath_), "%s/health.bin", tripDir_);

            // Count samples from file sizes
            File sf = SD.open(samplesPath_, FILE_READ);
            gnssSampleCount_ = sf ? (sf.size() / sizeof(GNSSSample)) : 0;
            if (sf) sf.close();

            File imf = SD.open(imuPath_, FILE_READ);
            imuSummaryCount_ = imf ? (imf.size() / sizeof(IMUSummary)) : 0;
            if (imf) imf.close();

            File of = SD.open(obdPath_, FILE_READ);
            obdSampleCount_ = of ? (of.size() / sizeof(OBDSnapshot)) : 0;
            if (of) of.close();

            tripStartMs_ = 0;
            eventCount_ = 0;
            addEvent(TripEvent::POWER_ANOMALY, "recovered after power loss");

            writeManifest();
            writeChecksums();
            recovered++;

            Serial.printf("[RECOVERY] Finalized: %u GNSS, %u IMU, %u OBD\n",
                          gnssSampleCount_, imuSummaryCount_, obdSampleCount_);
        }
    }
    root.close();

    if (recovered > 0) {
        Serial.printf("[RECOVERY] Recovered %d orphaned trip(s)\n", recovered);
    }

    // Clear state so normal operation starts clean
    memset(currentTripId_, 0, sizeof(currentTripId_));
    gnssSampleCount_ = 0;
    imuSummaryCount_ = 0;
    obdSampleCount_ = 0;
    eventCount_ = 0;
}

bool StateMachine::initSD() {
    if (sdReady_) return true;
    sdReady_ = SD.begin(PIN_SD_CS);
    if (sdReady_) {
        SD.mkdir("/cairn");
        SD.mkdir(TRIP_BASE_PATH);
    }
    return sdReady_;
}

bool StateMachine::openTripFile(const char* tripId) {
    snprintf(tripDir_, sizeof(tripDir_), "%s/%s", TRIP_BASE_PATH, tripId);
    snprintf(samplesPath_, sizeof(samplesPath_), "%s/samples.bin", tripDir_);
    snprintf(imuPath_, sizeof(imuPath_), "%s/imu_summary.bin", tripDir_);
    snprintf(obdPath_, sizeof(obdPath_), "%s/obd.bin", tripDir_);
    snprintf(healthPath_, sizeof(healthPath_), "%s/health.bin", tripDir_);

    if (!SD.mkdir(tripDir_)) {
        Serial.printf("[SD] Failed to create trip dir: %s\n", tripDir_);
        return false;
    }

    hashActive_ = true;
    Serial.printf("[SD] Trip dir: %s\n", tripDir_);
    return true;
}

bool StateMachine::writeSample(const GNSSSample& s) {
    File f = SD.open(samplesPath_, FILE_APPEND);
    if (!f) return false;
    size_t n = f.write(reinterpret_cast<const uint8_t*>(&s), sizeof(s));
    f.close();
    return n == sizeof(s);
}

bool StateMachine::writeIMUSummary(const IMUSummary& s) {
    File f = SD.open(imuPath_, FILE_APPEND);
    if (!f) return false;
    size_t n = f.write(reinterpret_cast<const uint8_t*>(&s), sizeof(s));
    f.close();
    return n == sizeof(s);
}

bool StateMachine::writeOBDSnapshot(const OBDSnapshot& s) {
    File f = SD.open(obdPath_, FILE_APPEND);
    if (!f) return false;
    size_t n = f.write(reinterpret_cast<const uint8_t*>(&s), sizeof(s));
    f.close();
    return n == sizeof(s);
}

bool StateMachine::writeDeviceHealth(const DeviceHealth& h) {
    File f = SD.open(healthPath_, FILE_APPEND);
    if (!f) return false;
    size_t n = f.write(reinterpret_cast<const uint8_t*>(&h), sizeof(h));
    f.close();
    return n == sizeof(h);
}

bool StateMachine::finalizeTripBundle() {
    if (gnssSampleCount_ == 0 && obdSampleCount_ == 0) return true;

    addEvent(TripEvent::TRIP_END, "finalized");

    if (!writeManifest()) return false;
    if (!writeChecksums()) return false;

    Serial.printf("[SD] Bundle: %u GNSS, %u IMU, %u OBD\n",
                  gnssSampleCount_, imuSummaryCount_, obdSampleCount_);
    return true;
}

bool StateMachine::writeManifest() {
    char filePath[80];
    snprintf(filePath, sizeof(filePath), "%s/manifest.json", tripDir_);

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) return false;

    uint64_t endMs = millis();
    f.printf("{\n");
    f.printf("  \"version\": 2,\n");
    f.printf("  \"schema_version\": 2,\n");
    f.printf("  \"trip_id\": \"%s\",\n", currentTripId_);
    f.printf("  \"vin\": \"%s\",\n", vin_);
    f.printf("  \"started_at_ms\": %llu,\n", (unsigned long long)tripStartMs_);
    f.printf("  \"ended_at_ms\": %llu,\n", (unsigned long long)endMs);
    f.printf("  \"duration_ms\": %llu,\n", (unsigned long long)(endMs - tripStartMs_));
    f.printf("  \"gnss_sample_count\": %u,\n", gnssSampleCount_);
    f.printf("  \"imu_summary_count\": %u,\n", imuSummaryCount_);
    f.printf("  \"obd_sample_count\": %u,\n", obdSampleCount_);
    f.printf("  \"gnss_rate_hz\": %u,\n", GNSS_RATE_ACTIVE_HZ);
    f.printf("  \"imu_rate_hz\": %u,\n", IMU_RATE_ACTIVE_HZ);
    if (dtcCount_ > 0) {
        f.printf("  \"dtc_count\": %d,\n", dtcCount_);
        f.printf("  \"dtc_codes\": [");
        for (int i = 0; i < dtcCount_; i++) {
            f.printf("%s%u", i > 0 ? "," : "", dtcCodes_[i]);
        }
        f.printf("],\n");
    }
    f.printf("  \"obd_enabled\": %s\n", obdSampleCount_ > 0 ? "true" : "false");
    f.printf("}\n");
    f.close();
    return true;
}

bool StateMachine::writeChecksums() {
    char checksumPath[80];
    snprintf(checksumPath, sizeof(checksumPath), "%s/sha256sums.txt", tripDir_);

    File outFile = SD.open(checksumPath, FILE_WRITE);
    if (!outFile) return false;

    const char* files[] = { "samples.bin", "imu_summary.bin", "obd.bin", "health.bin", "manifest.json" };

    for (const char* name : files) {
        char filePath[96];
        snprintf(filePath, sizeof(filePath), "%s/%s", tripDir_, name);

        File dataFile = SD.open(filePath, FILE_READ);
        if (!dataFile) continue;

        mbedtls_sha256_context ctx;
        mbedtls_sha256_init(&ctx);
        mbedtls_sha256_starts(&ctx, 0);

        uint8_t buf[512];
        while (dataFile.available()) {
            size_t n = dataFile.read(buf, sizeof(buf));
            if (n > 0) mbedtls_sha256_update(&ctx, buf, n);
        }
        dataFile.close();

        uint8_t hash[32];
        mbedtls_sha256_finish(&ctx, hash);
        mbedtls_sha256_free(&ctx);

        char hex[65];
        for (int i = 0; i < 32; i++) {
            snprintf(hex + i * 2, 3, "%02x", hash[i]);
        }
        outFile.printf("%s  %s\n", hex, name);
    }

    outFile.close();
    return true;
}

// ===========================================================================
// Connectivity
// ===========================================================================

bool StateMachine::scanForTrustedNetwork() {
    int n = WiFi.scanNetworks(false, false, false, 300);
    if (n <= 0) return false;

    for (int i = 0; i < n; i++) {
        Serial.printf("[WIFI] Found: %s (%d dBm)\n",
                      WiFi.SSID(i).c_str(), WiFi.RSSI(i));
    }
    WiFi.scanDelete();
    return n > 0;
}

bool StateMachine::connectToHome() {
    if (!wifiSSID[0]) {
        Serial.println("[WIFI] No SSID configured");
        return false;
    }

    Serial.printf("[WIFI] Connecting to %s\n", wifiSSID);
    WiFi.begin(wifiSSID, wifiPassword);

    unsigned long t = millis();
    while (WiFi.status() != WL_CONNECTED && millis() - t < 10000) {
        delay(500);
        processBLE(0);
    }

    if (WiFi.status() == WL_CONNECTED) {
        Serial.printf("[WIFI] Connected, IP: %s\n", WiFi.localIP().toString().c_str());
        rssi_ = WiFi.RSSI();
        return true;
    }

    Serial.println("[WIFI] Connection failed");
    WiFi.disconnect(true);
    return false;
}

static bool hashFile(const char* path, char* hexOut) {
    File f = SD.open(path, FILE_READ);
    if (!f) return false;

    mbedtls_sha256_context ctx;
    mbedtls_sha256_init(&ctx);
    mbedtls_sha256_starts(&ctx, 0);

    uint8_t buf[512];
    while (f.available()) {
        size_t n = f.read(buf, sizeof(buf));
        if (n > 0) mbedtls_sha256_update(&ctx, buf, n);
    }
    f.close();

    uint8_t hash[32];
    mbedtls_sha256_finish(&ctx, hash);
    mbedtls_sha256_free(&ctx);

    for (int i = 0; i < 32; i++) {
        snprintf(hexOut + i * 2, 3, "%02x", hash[i]);
    }
    hexOut[64] = '\0';
    return true;
}

static int httpRequest(WiFiClient& client, const char* method,
                       const char* host, uint16_t port, const char* path,
                       const char* contentType, const uint8_t* payload, size_t len,
                       const char* extraHeader, char* respBuf, size_t respBufSize) {
    if (!client.connect(host, port)) return -1;

    client.printf("%s %s HTTP/1.1\r\n", method, path);
    client.printf("Host: %s:%u\r\n", host, port);
    client.printf("Content-Type: %s\r\n", contentType);
    client.printf("Content-Length: %u\r\n", (unsigned)len);
    client.print("Connection: close\r\n");
    if (extraHeader) client.print(extraHeader);
    client.print("\r\n");

    if (payload && len > 0) client.write(payload, len);

    unsigned long t = millis();
    while (!client.available() && millis() - t < 10000) delay(10);

    char statusLine[64] = {};
    if (client.available()) {
        int sl = client.readBytesUntil('\n', statusLine, sizeof(statusLine) - 1);
        statusLine[sl] = '\0';
    }

    int httpCode = 0;
    char* sp = strchr(statusLine, ' ');
    if (sp) httpCode = atoi(sp + 1);

    // Skip headers
    while (client.available()) {
        String line = client.readStringUntil('\n');
        if (line == "\r" || line.length() == 0) break;
    }

    // Read body
    size_t bodyLen = 0;
    if (respBuf && respBufSize > 0) {
        while (client.available() && bodyLen < respBufSize - 1) {
            int b = client.read();
            if (b < 0) break;
            respBuf[bodyLen++] = (char)b;
        }
        respBuf[bodyLen] = '\0';
    }

    client.stop();
    return httpCode;
}

bool StateMachine::uploadBundle(const char* tripId) {
    if (WiFi.status() != WL_CONNECTED) return false;

    char samplesFile[80];
    snprintf(samplesFile, sizeof(samplesFile), "%s/%s/samples.bin",
             TRIP_BASE_PATH, tripId);

    File f = SD.open(samplesFile, FILE_READ);
    if (!f) return false;
    size_t totalSize = f.size();
    f.close();

    char contentHash[65];
    if (!hashFile(samplesFile, contentHash)) return false;

    Serial.printf("[SYNC] Uploading %s (%u bytes, hash=%.16s...)\n",
                  tripId, (unsigned)totalSize, contentHash);

    WiFiClient client;
    char respBuf[512];

    // --- Step 1: Init ---
    JsonDocument initDoc;
    initDoc["device_id"] = WiFi.macAddress().c_str();
    initDoc["trip_id"] = tripId;
    initDoc["content_hash"] = contentHash;
    initDoc["size"] = (int)totalSize;

    char body[256];
    size_t bodyLen = serializeJson(initDoc, body, sizeof(body));

    int code = httpRequest(client, "POST", SERVER_HOSTNAME, SERVER_PORT,
                           "/api/v1/upload/init", "application/json",
                           (const uint8_t*)body, bodyLen, nullptr,
                           respBuf, sizeof(respBuf));

    if (code != 200) {
        Serial.printf("[SYNC] Init failed: HTTP %d\n", code);
        return false;
    }

    JsonDocument respDoc;
    if (deserializeJson(respDoc, respBuf)) {
        Serial.println("[SYNC] Init parse error");
        return false;
    }

    const char* uploadId = respDoc["upload_id"];
    int64_t resumeOffset = respDoc["resume_offset"] | (int64_t)0;

    if (!uploadId) {
        Serial.println("[SYNC] Init: no upload_id");
        return false;
    }

    if (resumeOffset == -1) {
        Serial.println("[SYNC] Already uploaded (server confirms)");
        return true;
    }

    char uploadIdBuf[40];
    strncpy(uploadIdBuf, uploadId, sizeof(uploadIdBuf) - 1);
    uploadIdBuf[sizeof(uploadIdBuf) - 1] = '\0';

    Serial.printf("[SYNC] Init OK: id=%s, resume=%lld\n", uploadIdBuf, resumeOffset);

    // --- Step 2: Chunk upload ---
    f = SD.open(samplesFile, FILE_READ);
    if (!f) return false;

    if (resumeOffset > 0) f.seek(resumeOffset);

    size_t offset = (size_t)resumeOffset;
    uint8_t* chunk = (uint8_t*)malloc(UPLOAD_CHUNK_SIZE);
    if (!chunk) { f.close(); return false; }

    bool uploadOk = true;
    while (offset < totalSize) {
        if (WiFi.status() != WL_CONNECTED) { uploadOk = false; break; }

        size_t toRead = totalSize - offset;
        if (toRead > UPLOAD_CHUNK_SIZE) toRead = UPLOAD_CHUNK_SIZE;

        size_t n = f.read(chunk, toRead);
        if (n == 0) { uploadOk = false; break; }

        char path[96];
        snprintf(path, sizeof(path), "/api/v1/upload/%s/chunk", uploadIdBuf);

        char offsetHdr[48];
        snprintf(offsetHdr, sizeof(offsetHdr), "X-Upload-Offset: %u\r\n", (unsigned)offset);

        code = httpRequest(client, "PUT", SERVER_HOSTNAME, SERVER_PORT,
                           path, "application/octet-stream",
                           chunk, n, offsetHdr, respBuf, sizeof(respBuf));

        if (code != 200) {
            Serial.printf("[SYNC] Chunk failed at %u: HTTP %d\n", (unsigned)offset, code);
            uploadOk = false;
            break;
        }

        offset += n;
        unsigned pct = (unsigned)((uint64_t)offset * 100 / totalSize);
        Serial.printf("[SYNC] %u/%u (%u%%)\n", (unsigned)offset, (unsigned)totalSize, pct);
    }

    free(chunk);
    f.close();

    if (!uploadOk) return false;

    // --- Step 3: Finalize ---
    char finPath[96];
    snprintf(finPath, sizeof(finPath), "/api/v1/upload/%s/finalize", uploadIdBuf);

    JsonDocument finDoc;
    finDoc["content_hash"] = contentHash;
    bodyLen = serializeJson(finDoc, body, sizeof(body));

    code = httpRequest(client, "POST", SERVER_HOSTNAME, SERVER_PORT,
                       finPath, "application/json",
                       (const uint8_t*)body, bodyLen, nullptr,
                       respBuf, sizeof(respBuf));

    if (code == 200) {
        JsonDocument rcptDoc;
        deserializeJson(rcptDoc, respBuf);
        const char* receiptId = rcptDoc["receipt_id"];
        Serial.printf("[SYNC] Finalized, receipt=%s\n", receiptId ? receiptId : "n/a");
    } else {
        Serial.printf("[SYNC] Finalize failed: HTTP %d\n", code);
    }

    return code == 200;
}

// ===========================================================================
// Power
// ===========================================================================

float StateMachine::getBatteryVoltage() {
#if ENABLE_OBD
    if (sys_.devType > 12) {
        return (float)(analogRead(A0) * 45) / 4095;
    }
    return obd_.getVoltage();
#else
    return 0;
#endif
}

// ===========================================================================
// BLE SPP command interface (harvested from stock firmware)
// ===========================================================================

void StateMachine::processBLE(int timeout) {
#if ENABLE_BLE
    char* cmd = ble_recv_command(timeout);
    if (!cmd) return;

    char *p = strchr(cmd, '\r');
    if (p) *p = 0;

    char buf[48];
    int bufsize = sizeof(buf);
    int n = 0;

    Serial.printf("[BLE] %s", cmd);

    if (!strcmp(cmd, "UPTIME") || !strcmp(cmd, "TICK")) {
        n = snprintf(buf, bufsize, "%lu", millis());
    } else if (!strcmp(cmd, "BATT")) {
        n = snprintf(buf, bufsize, "%.2f", batteryVoltage_);
    } else if (!strcmp(cmd, "RESET")) {
        ESP.restart();
    } else if (!strcmp(cmd, "OFF")) {
        standby();
        n = snprintf(buf, bufsize, "OK");
    } else if (!strcmp(cmd, "ON?")) {
        n = snprintf(buf, bufsize, "%u", state_ != DeviceState::SLEEP ? 1 : 0);
    } else if (!strcmp(cmd, "STATE")) {
        n = snprintf(buf, bufsize, "%s", getStateName());
    } else if (!strcmp(cmd, "VIN")) {
        n = snprintf(buf, bufsize, "%s", vin_[0] ? vin_ : "N/A");
    } else if (!strcmp(cmd, "TEMP")) {
        n = snprintf(buf, bufsize, "%d", deviceTemp_);
    } else if (!strcmp(cmd, "ACC")) {
        n = snprintf(buf, bufsize, "%.1f/%.1f/%.1f", acc_[0], acc_[1], acc_[2]);
    } else if (!strcmp(cmd, "GYRO")) {
        n = snprintf(buf, bufsize, "%.1f/%.1f/%.1f", gyr_[0], gyr_[1], gyr_[2]);
    } else if (!strcmp(cmd, "GF")) {
        n = snprintf(buf, bufsize, "%f",
                     sqrtf(acc_[0]*acc_[0] + acc_[1]*acc_[1] + acc_[2]*acc_[2]));
    } else if (!strcmp(cmd, "RSSI")) {
        n = snprintf(buf, bufsize, "%d", rssi_);
    } else if (!strcmp(cmd, "SSID?")) {
        n = snprintf(buf, bufsize, "%s", wifiSSID[0] ? wifiSSID : "-");
    } else if (!strncmp(cmd, "SSID=", 5)) {
        n = snprintf(buf, bufsize, "%s",
                     nvs_set_str(nvsHandle, "WIFI_SSID", cmd + 5) == ESP_OK ? "OK" : "ERR");
        loadConfig();
    } else if (!strcmp(cmd, "WPWD?")) {
        n = snprintf(buf, bufsize, "%s", wifiPassword[0] ? wifiPassword : "-");
    } else if (!strncmp(cmd, "WPWD=", 5)) {
        n = snprintf(buf, bufsize, "%s",
                     nvs_set_str(nvsHandle, "WIFI_PWD", cmd + 5) == ESP_OK ? "OK" : "ERR");
        loadConfig();
    } else if (!strcmp(cmd, "LAT") && gpsData_) {
        n = snprintf(buf, bufsize, "%f", gpsData_->lat);
    } else if (!strcmp(cmd, "LNG") && gpsData_) {
        n = snprintf(buf, bufsize, "%f", gpsData_->lng);
    } else if (!strcmp(cmd, "SPD") && gpsData_) {
        n = snprintf(buf, bufsize, "%d", (int)(gpsData_->speed * 1852 / 1000));
    } else if (!strcmp(cmd, "SAT") && gpsData_) {
        n = snprintf(buf, bufsize, "%u", (unsigned)gpsData_->sat);
    } else {
        n = snprintf(buf, bufsize, "ERROR");
    }

    Serial.printf(" -> %s\n", buf);
    if (n < bufsize - 1) buf[n++] = '\r';
    buf[n] = 0;
    ble_send_response(buf, n, cmd);
#else
    if (timeout) delay(timeout);
#endif
}

// ===========================================================================
// Helpers
// ===========================================================================

void StateMachine::generateTripId(char* buf, size_t len) {
    if (len < 27) { if (len > 0) buf[0] = '\0'; return; }
    uint64_t now = millis();
    uint32_t r1 = esp_random();
    uint32_t r2 = esp_random();
    snprintf(buf, len, "%010llX%08lX%08lX",
             (unsigned long long)now,
             (unsigned long)r1,
             (unsigned long)r2);
}

void StateMachine::addEvent(TripEvent::EventType type, const char* details) {
    if (eventCount_ >= MAX_EVENTS) return;
    TripEvent& e = events_[eventCount_++];
    e.type = type;
    e.timestamp_ms = millis();
    e.latitude = 0;
    e.longitude = 0;
    strncpy(e.details, details, sizeof(e.details) - 1);
    e.details[sizeof(e.details) - 1] = '\0';
}
