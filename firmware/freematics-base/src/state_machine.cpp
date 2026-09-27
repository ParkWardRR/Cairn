#include "state_machine.h"
#include "config.h"
#include <cstring>
#include <cstdio>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include "mbedtls/sha256.h"

// ===========================================================================
// Initialization — using FreematicsPlus for all hardware
// ===========================================================================

void StateMachine::init() {
    state_ = DeviceState::SLEEP;
    stateEnteredAt_ = millis();
    gnssSampleCount_ = 0;
    imuSummaryCount_ = 0;
    eventCount_ = 0;
    memset(currentTripId_, 0, sizeof(currentTripId_));

    // Init OBD coprocessor link (detects device type, sets up UART)
    if (!sys_.begin(true, false)) {
        Serial.println("[SYS] Coprocessor init failed — trying without");
    } else {
        Serial.printf("[SYS] Device type: %u\n", sys_.devType);
    }

    // Init IMU (ICM-42627 via I2C, address 0x68, SDA=21, SCL=22)
    mems_ = new ICM_42627;
    byte memsResult = mems_->begin();
    memsReady_ = (memsResult > 0);
    if (memsReady_) {
        Serial.println("[IMU] ICM-42627 initialized");
    } else {
        Serial.println("[IMU] ICM-42627 init failed");
    }

    // Init GNSS (UART to u-blox module, with UBX configuration)
    gpsReady_ = sys_.gpsBeginExt(GPS_SOFT_BAUDRATE);
    if (gpsReady_) {
        Serial.println("[GPS] GNSS module online");
    } else {
        Serial.println("[GPS] GNSS init failed — retrying with link passthrough");
        gpsReady_ = sys_.gpsBegin();
        if (gpsReady_) {
            Serial.println("[GPS] GNSS via coprocessor link");
        } else {
            Serial.println("[GPS] GNSS not available");
        }
    }

    // Init SD card (SPI, CS=GPIO5)
    sdReady_ = SD.begin(PIN_SD_CS);
    if (sdReady_) {
        uint64_t totalBytes = SD.totalBytes();
        uint64_t usedBytes = SD.usedBytes();
        Serial.printf("[SD] Mounted: %llu MB total, %llu MB used\n",
                      totalBytes / (1024*1024), usedBytes / (1024*1024));
        SD.mkdir(TRIP_BASE_PATH);
    } else {
        Serial.println("[SD] Card mount failed");
    }

    // Read initial battery voltage
    lastBatteryVoltage_ = getBatteryVoltage();
    Serial.printf("[PWR] Battery: %.1f V\n", lastBatteryVoltage_);

    Serial.println("[STATE] Initialized -> SLEEP");
}

// ===========================================================================
// Main update loop
// ===========================================================================

void StateMachine::update() {
    // Periodic battery voltage check (every 10s, avoid hammering coprocessor)
    unsigned long now = millis();
    if (now - lastVoltageRead_ >= 10000) {
        lastBatteryVoltage_ = getBatteryVoltage();
        lastVoltageRead_ = now;
    }

    if (state_ != DeviceState::SLEEP &&
        state_ != DeviceState::LOW_BATTERY_PROTECTION) {
        if (lastBatteryVoltage_ > 0 && lastBatteryVoltage_ < LOW_BATTERY_THRESHOLD_V) {
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
    // Check for motion via IMU
    if (memsReady_ && detectMotion()) {
        transitionTo(DeviceState::ARMING);
        return;
    }

    // Check for engine start via voltage
    if (lastBatteryVoltage_ >= ENGINE_ON_VOLTAGE_V) {
        Serial.printf("[SLEEP] Engine voltage detected: %.1f V\n", lastBatteryVoltage_);
        transitionTo(DeviceState::ARMING);
        return;
    }

    delay(500);
}

void StateMachine::handleArming() {
    unsigned long elapsed = millis() - stateEnteredAt_;

    GNSSSample gnss = readGNSS();
    IMUSummary imu  = readIMU();
    float speed     = getSpeedKmh(gnss);

    bool conditionsMet = (speed >= MIN_SPEED_KMH) && isMoving(gnss, imu);

    if (!conditionsMet) {
        if (elapsed > ARMING_DURATION_MS) {
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
        eventCount_ = 0;
        tripStartMs_ = millis();

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

    uint16_t gnssIntervalMs = (GNSS_RATE_ACTIVE_HZ > 0) ? (1000 / GNSS_RATE_ACTIVE_HZ) : 0;
    if (gnssIntervalMs > 0 && (now - lastGNSSRead_ >= gnssIntervalMs)) {
        GNSSSample sample = readGNSS();
        writeSample(sample);
        gnssSampleCount_++;
        lastGNSSRead_ = now;

        float speed = getSpeedKmh(sample);
        IMUSummary imu = readIMU();
        writeIMUSummary(imu);
        imuSummaryCount_++;
        lastIMURead_ = now;

        if (speed < MIN_SPEED_KMH && !isMoving(sample, imu)) {
            addEvent(TripEvent::STOP_CANDIDATE_EVT, "speed below threshold");
            transitionTo(DeviceState::STOP_CANDIDATE);
            return;
        }
    }

    uint16_t imuIntervalMs = (IMU_RATE_ACTIVE_HZ > 0) ? (1000 / IMU_RATE_ACTIVE_HZ) : 0;
    if (imuIntervalMs > 0 && (now - lastIMURead_ >= imuIntervalMs)) {
        IMUSummary imu = readIMU();
        writeIMUSummary(imu);
        imuSummaryCount_++;
        lastIMURead_ = now;
    }
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
        Serial.println("[QUEUED] No network found — entering SLEEP");
        transitionTo(DeviceState::SLEEP);
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
    esp_deep_sleep_start();
}

void StateMachine::handleFault() {
    Serial.println("[FAULT] Error state — rebooting in 5s");
    delay(5000);
    esp_restart();
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
// Sensor reads — backed by FreematicsPlus
// ===========================================================================

bool StateMachine::detectMotion() {
    if (!memsReady_) return false;

    float acc[3] = {0};
    mems_->read(acc);

    float magnitude = sqrtf(acc[0]*acc[0] + acc[1]*acc[1] + acc[2]*acc[2]);
    float deviation = fabsf(magnitude - 1.0f);
    return deviation > MOTION_THRESHOLD_G;
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
    // TinyGPS speed is in knots*100, convert to cm/s (1 knot = 51.4444 cm/s)
    s.speed_cmps   = (uint16_t)(gpsData_->speed * 51.4444f);
    s.heading_cdeg = (uint16_t)(gpsData_->heading * 100);
    s.satellites   = gpsData_->sat;
    s.hdop_tenths  = (uint16_t)(gpsData_->hdop * 10);
    s.fix_quality  = (gpsData_->sat > 0) ? 1 : 0;

    return s;
}

IMUSummary StateMachine::readIMU() {
    IMUSummary summary;
    memset(&summary, 0, sizeof(summary));

    if (!memsReady_) return summary;

    float acc[3] = {0};
    float gyr[3] = {0};
    mems_->read(acc, gyr);

    summary.window_start_ms = millis();
    summary.window_duration_ms = 20;

    // acc[] is in g from FreematicsPlus, convert to milli-g
    summary.accel_peak_x_mg = (int16_t)(acc[0] * 1000.0f);
    summary.accel_peak_y_mg = (int16_t)(acc[1] * 1000.0f);
    summary.accel_peak_z_mg = (int16_t)(acc[2] * 1000.0f);

    float mag_mg = sqrtf(acc[0]*acc[0] + acc[1]*acc[1] + acc[2]*acc[2]) * 1000.0f;
    summary.accel_rms_mg = (uint16_t)mag_mg;

    // gyr[] is in dps from FreematicsPlus
    float gyro_max = fmaxf(fmaxf(fabsf(gyr[0]), fabsf(gyr[1])), fabsf(gyr[2]));
    summary.gyro_peak_dps = (int16_t)(gyro_max * 10.0f);

    summary.variance = (uint16_t)fabsf(mag_mg - 1000.0f);

    summary.flags = 0;
    float total_g = sqrtf(acc[0]*acc[0] + acc[1]*acc[1] + acc[2]*acc[2]);
    if (total_g > IMPACT_THRESHOLD_G) summary.flags |= 0x01;
    if (acc[0] < -HARD_BRAKE_THRESHOLD_G) summary.flags |= 0x02;
    if (fabsf(acc[1]) > SHARP_TURN_THRESHOLD_G) summary.flags |= 0x04;

    return summary;
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
// Storage — Arduino SD via SPI
// ===========================================================================

bool StateMachine::initSD() {
    if (sdReady_) return true;
    sdReady_ = SD.begin(PIN_SD_CS);
    if (sdReady_) {
        SD.mkdir(TRIP_BASE_PATH);
    }
    return sdReady_;
}

bool StateMachine::openTripFile(const char* tripId) {
    snprintf(tripDir_, sizeof(tripDir_), "%s/%s", TRIP_BASE_PATH, tripId);
    snprintf(samplesPath_, sizeof(samplesPath_), "%s/samples.bin", tripDir_);
    snprintf(imuPath_, sizeof(imuPath_), "%s/imu_summary.bin", tripDir_);

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

bool StateMachine::finalizeTripBundle() {
    if (gnssSampleCount_ == 0) return true;

    addEvent(TripEvent::TRIP_END, "finalized");

    if (!writeManifest()) return false;
    if (!writeChecksums()) return false;

    Serial.printf("[SD] Bundle finalized: %u GNSS, %u IMU\n",
                  gnssSampleCount_, imuSummaryCount_);
    return true;
}

bool StateMachine::writeManifest() {
    char filePath[80];
    snprintf(filePath, sizeof(filePath), "%s/manifest.json", tripDir_);

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) return false;

    uint64_t endMs = millis();
    f.printf("{\n");
    f.printf("  \"version\": 1,\n");
    f.printf("  \"schema_version\": 1,\n");
    f.printf("  \"trip_id\": \"%s\",\n", currentTripId_);
    f.printf("  \"started_at_ms\": %llu,\n", (unsigned long long)tripStartMs_);
    f.printf("  \"ended_at_ms\": %llu,\n", (unsigned long long)endMs);
    f.printf("  \"duration_ms\": %llu,\n", (unsigned long long)(endMs - tripStartMs_));
    f.printf("  \"gnss_sample_count\": %u,\n", gnssSampleCount_);
    f.printf("  \"imu_summary_count\": %u,\n", imuSummaryCount_);
    f.printf("  \"gnss_rate_hz\": %u,\n", GNSS_RATE_ACTIVE_HZ);
    f.printf("  \"imu_rate_hz\": %u\n", IMU_RATE_ACTIVE_HZ);
    f.printf("}\n");
    f.close();
    return true;
}

bool StateMachine::writeChecksums() {
    // SHA-256 using ESP32 hardware-accelerated mbedtls
    char checksumPath[80];
    snprintf(checksumPath, sizeof(checksumPath), "%s/sha256sums.txt", tripDir_);

    File outFile = SD.open(checksumPath, FILE_WRITE);
    if (!outFile) return false;

    const char* files[] = { "samples.bin", "imu_summary.bin", "manifest.json" };

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
    Serial.println("[SD] SHA-256 checksums written");
    return true;
}

// ===========================================================================
// Connectivity — ESP32 WiFi
// ===========================================================================

bool StateMachine::scanForTrustedNetwork() {
    int n = WiFi.scanNetworks(false, false, false, 300);
    if (n <= 0) return false;

    // Look for any known network — in production, check NVS-stored SSID/BSSID
    for (int i = 0; i < n; i++) {
        Serial.printf("[WIFI] Found: %s (%d dBm)\n",
                      WiFi.SSID(i).c_str(), WiFi.RSSI(i));
    }
    WiFi.scanDelete();
    return n > 0;
}

bool StateMachine::connectToHome() {
    // In production, load SSID/PSK from NVS
    // For now, log the attempt
    Serial.println("[WIFI] Connect to home network (stub)");
    return false;
}

bool StateMachine::uploadBundle(const char* tripId) {
    if (WiFi.status() != WL_CONNECTED) return false;

    // Compute content hash of samples.bin for deduplication
    char samplesFile[80];
    snprintf(samplesFile, sizeof(samplesFile), "%s/%s/samples.bin",
             TRIP_BASE_PATH, tripId);

    File f = SD.open(samplesFile, FILE_READ);
    if (!f) return false;

    mbedtls_sha256_context ctx;
    mbedtls_sha256_init(&ctx);
    mbedtls_sha256_starts(&ctx, 0);

    uint8_t readBuf[UPLOAD_CHUNK_SIZE];
    size_t totalSize = 0;
    while (f.available()) {
        size_t n = f.read(readBuf, sizeof(readBuf));
        if (n > 0) {
            mbedtls_sha256_update(&ctx, readBuf, n);
            totalSize += n;
        }
    }
    f.close();

    uint8_t hash[32];
    mbedtls_sha256_finish(&ctx, hash);
    mbedtls_sha256_free(&ctx);

    char hashHex[65];
    for (int i = 0; i < 32; i++) {
        snprintf(hashHex + i * 2, 3, "%02x", hash[i]);
    }

    Serial.printf("[SYNC] Ready to upload %s (%u bytes, hash=%s)\n",
                  tripId, (unsigned)totalSize, hashHex);

    // TODO: Implement HTTP upload to cairn.local:8443
    // POST /api/v1/upload/init  { trip_id, content_hash, size }
    // PUT  /api/v1/upload/{id}/chunk
    // POST /api/v1/upload/{id}/finalize

    return false;
}

// ===========================================================================
// Power — via OBD coprocessor ATRV command
// ===========================================================================

float StateMachine::getBatteryVoltage() {
    if (!sys_.link) return 0;

    char buf[32];
    int n = sys_.link->sendCommand("ATRV\r", buf, sizeof(buf), 500);
    if (n <= 0) return 0;

    // Response is like "12.5V" or "14.2V\r>"
    float voltage = 0;
    char* p = buf;
    while (*p && (*p < '0' || *p > '9') && *p != '.') p++;
    if (*p) voltage = atof(p);

    return voltage;
}

// ===========================================================================
// Helpers
// ===========================================================================

void StateMachine::generateTripId(char* buf, size_t len) {
    if (len < 27) { if (len > 0) buf[0] = '\0'; return; }

    // 10 hex chars from millis timestamp + 16 hex chars from esp_random
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
