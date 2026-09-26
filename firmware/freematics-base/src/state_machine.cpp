#include "state_machine.h"
#include "config.h"
#include <cstring>
#include <cstdio>

// ===========================================================================
// Initialization
// ===========================================================================

void StateMachine::init() {
    state_ = DeviceState::SLEEP;
    stateEnteredAt_ = millis();
    gnssSampleCount_ = 0;
    imuSummaryCount_ = 0;
    eventCount_ = 0;
    memset(currentTripId_, 0, sizeof(currentTripId_));

    // Initialize all HAL subsystems in correct order
    if (!CairnHAL::init()) {
        Serial.println("[HAL] Critical subsystem init failed");
        transitionTo(DeviceState::FAULT);
        return;
    }
    CairnHAL::printCapabilities();

    // Enable brownout detection
    CairnHAL::power.enableBrownoutDetection(BROWNOUT_THRESHOLD_V);

    // Load Wi-Fi credentials from encrypted NVS
    wifiCredsLoaded_ = CairnHAL::nvs.loadWiFiCredentials(wifiCreds_);
    if (wifiCredsLoaded_) {
        memcpy(trustedBSSID_, wifiCreds_.bssid, 6);
        Serial.println("[NVS] Wi-Fi credentials loaded");
    } else {
        Serial.println("[NVS] No Wi-Fi credentials stored");
    }

    // Check wake reason — if waking from ULP motion detection, go to ARMING
    CairnHAL::WakeReason wake = CairnHAL::HalULP::getWakeReason();
    if (wake == CairnHAL::WakeReason::ULP_MOTION) {
        Serial.printf("[ULP] Motion wake (magnitude: %u mg)\n",
                      CairnHAL::HalULP::getMotionMagnitude());
        CairnHAL::power.setPerformanceMode();
        transitionTo(DeviceState::ARMING);
        return;
    }

    // Start in low-power mode for SLEEP state
    CairnHAL::power.setLowPowerMode();
    CairnHAL::power.powerOffWiFi();

    Serial.println("[STATE] Initialized -> SLEEP");
}

// ===========================================================================
// Main update loop
// ===========================================================================

void StateMachine::update() {
    // Feed hardware watchdog every iteration
    CairnHAL::timer.feedWatchdog();

    // Global battery check
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
    // Configure ULP motion wake program and enter deep sleep
    CairnHAL::power.powerOffWiFi();
    CairnHAL::power.powerOffBluetooth();

    CairnHAL::ulp.loadMotionWakeProgram(ULP_MOTION_THRESHOLD_MG,
                                         ULP_CHECK_INTERVAL_MS);
    CairnHAL::ulp.startMonitoring();

    // Save any pending NVS data before deep sleep wipes RAM
    CairnHAL::nvs.commit();

    // Deep sleep — ULP draws ~150 µA, main CPU off
    // On wake, ESP32 reboots through init() which checks getWakeReason()
    CairnHAL::power.enterDeepSleep(0);
    // Not reached — chip reboots on wake
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
        // Switch to max performance for recording
        CairnHAL::power.setPerformanceMode();
        CairnHAL::power.enableAllPeripherals();

        // Initialize hardware timers for deterministic sample rates
        CairnHAL::timer.initHighResCounter();

        generateTripId(currentTripId_, sizeof(currentTripId_));
        openTripFile(currentTripId_);

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

    // GNSS read at active rate
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

    // Independent IMU reads at higher rate
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
        Serial.println("[FINALIZING] Trip bundle written successfully");
        // Drop to low-power mode — recording is over
        CairnHAL::power.setLowPowerMode();
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

        // Power on Wi-Fi radio for scan
        CairnHAL::power.powerOnWiFi();
        CairnHAL::wifi.init();

        if (scanForTrustedNetwork()) {
            if (connectToHome()) {
                // Disable power save for maximum upload throughput
                CairnHAL::wifi.disablePowerSave();
                transitionTo(DeviceState::SYNCING);
                return;
            }
        }

        // Scan failed — power off Wi-Fi to save energy
        CairnHAL::wifi.powerOff();
    }

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
        CairnHAL::wifi.disconnect();
        CairnHAL::wifi.powerOff();
        transitionTo(DeviceState::RETAINED);
    } else {
        Serial.println("[SYNCING] Upload failed -- requeueing");
        CairnHAL::wifi.disconnect();
        CairnHAL::wifi.powerOff();
        transitionTo(DeviceState::QUEUED_FOR_HOME_SYNC);
    }
}

void StateMachine::handleRetained() {
    unsigned long retentionMs = (unsigned long)RETENTION_WINDOW_HOURS * 3600UL * 1000UL;
    if (millis() - stateEnteredAt_ >= retentionMs) {
        // Prune trip data from SD
        char path[80];
        snprintf(path, sizeof(path), "%s/cairn/trips/%s",
                 SDMMC_MOUNT_POINT, currentTripId_);
        // Delete trip files but keep receipt in NVS
        CairnHAL::sdmmc.remove(path);
        transitionTo(DeviceState::PRUNABLE);
    }
}

void StateMachine::handleLowBatteryProtection() {
    Serial.println("[LOW_BATTERY] Finalizing trip if active, entering deep sleep");
    addEvent(TripEvent::POWER_ANOMALY, "low battery protection");
    finalizeTripBundle();
    CairnHAL::nvs.commit();
    CairnHAL::power.enterDeepSleep(0);
}

void StateMachine::handleFault() {
    Serial.println("[FAULT] Error state -- awaiting reboot for recovery");
    CairnHAL::nvs.commit();
    delay(5000);
    esp_restart();
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
// Sensor implementations (HAL-backed)
// ===========================================================================

bool StateMachine::detectMotion() {
    // Read IMU via DMA burst and check for significant acceleration
    uint8_t rawBuf[DMA_IMU_BURST_BYTES];
    CairnHAL::DMATransferResult result =
        CairnHAL::dma.readIMUBurst(0x3B, rawBuf, sizeof(rawBuf));

    if (!result.success) return false;

    // Parse raw accelerometer values (big-endian from MPU-6050)
    int16_t ax = (int16_t)((rawBuf[0] << 8) | rawBuf[1]);
    int16_t ay = (int16_t)((rawBuf[2] << 8) | rawBuf[3]);
    int16_t az = (int16_t)((rawBuf[4] << 8) | rawBuf[5]);

    // Convert to milli-g (assuming ±2g range: 16384 LSB/g)
    float magnitude = sqrtf((float)ax*ax + (float)ay*ay + (float)az*az) / 16.384f;
    return magnitude > ULP_MOTION_THRESHOLD_MG;
}

GNSSSample StateMachine::readGNSS() {
    GNSSSample s;
    memset(&s, 0, sizeof(s));

    // Read NMEA sentences from UART2 (Freematics GNSS module)
    char nmea[256];
    int len = 0;
    unsigned long start = millis();
    while (millis() - start < 100 && len < (int)sizeof(nmea) - 1) {
        if (Serial2.available()) {
            char c = Serial2.read();
            nmea[len++] = c;
            if (c == '\n') break;
        }
    }
    nmea[len] = '\0';

    // Parse $GNGGA or $GPGGA sentence
    if (len > 6 && (strncmp(nmea + 3, "GGA", 3) == 0)) {
        // Field parsing: time,lat,N/S,lon,E/W,quality,sats,hdop,alt,...
        char* field[15];
        int nf = 0;
        field[nf++] = nmea + 7; // skip $xxGGA,
        for (int i = 7; i < len && nf < 15; i++) {
            if (nmea[i] == ',') {
                nmea[i] = '\0';
                field[nf++] = nmea + i + 1;
            }
        }

        if (nf >= 10) {
            s.timestamp_ms = millis(); // TODO: parse UTC from field[0]

            // Latitude: ddmm.mmmm
            if (field[1][0]) {
                double raw = atof(field[1]);
                int deg = (int)(raw / 100);
                double min = raw - deg * 100;
                double lat = deg + min / 60.0;
                if (field[2][0] == 'S') lat = -lat;
                s.latitude = (int32_t)(lat * 1e7);
            }

            // Longitude: dddmm.mmmm
            if (field[3][0]) {
                double raw = atof(field[3]);
                int deg = (int)(raw / 100);
                double min = raw - deg * 100;
                double lon = deg + min / 60.0;
                if (field[4][0] == 'W') lon = -lon;
                s.longitude = (int32_t)(lon * 1e7);
            }

            s.fix_quality = atoi(field[5]);
            s.satellites = atoi(field[6]);
            s.hdop_tenths = (uint16_t)(atof(field[7]) * 10);
            s.altitude_cm = (int32_t)(atof(field[8]) * 100);
        }
    }

    // Parse $GNRMC or $GPRMC for speed and heading
    // (In a full implementation this would buffer multiple sentences
    // per fix cycle. For now speed/heading come from the next RMC.)

    return s;
}

IMUSummary StateMachine::readIMU() {
    IMUSummary summary;
    memset(&summary, 0, sizeof(summary));

    // DMA burst read of all 6 axes from MPU-6050 (register 0x3B, 14 bytes)
    uint8_t rawBuf[14];
    CairnHAL::DMATransferResult result =
        CairnHAL::dma.readIMUBurst(0x3B, rawBuf, sizeof(rawBuf));

    if (!result.success) return summary;

    summary.window_start_ms = millis();
    summary.window_duration_ms = 20; // ~50 Hz window

    // Accelerometer (big-endian, ±2g = 16384 LSB/g)
    int16_t ax = (int16_t)((rawBuf[0] << 8) | rawBuf[1]);
    int16_t ay = (int16_t)((rawBuf[2] << 8) | rawBuf[3]);
    int16_t az = (int16_t)((rawBuf[4] << 8) | rawBuf[5]);
    // rawBuf[6..7] = temperature (skip)

    // Gyroscope (big-endian, ±250 dps = 131 LSB/dps)
    int16_t gx = (int16_t)((rawBuf[8] << 8) | rawBuf[9]);
    int16_t gy = (int16_t)((rawBuf[10] << 8) | rawBuf[11]);
    int16_t gz = (int16_t)((rawBuf[12] << 8) | rawBuf[13]);

    // Convert to milli-g (±2g range)
    summary.accel_peak_x_mg = (int16_t)(ax * 1000 / 16384);
    summary.accel_peak_y_mg = (int16_t)(ay * 1000 / 16384);
    summary.accel_peak_z_mg = (int16_t)(az * 1000 / 16384);

    float mag_mg = sqrtf((float)summary.accel_peak_x_mg * summary.accel_peak_x_mg +
                         (float)summary.accel_peak_y_mg * summary.accel_peak_y_mg +
                         (float)summary.accel_peak_z_mg * summary.accel_peak_z_mg);
    summary.accel_rms_mg = (uint16_t)mag_mg;

    // Convert gyro to dps×10
    int16_t gyro_max = max(max(abs(gx), abs(gy)), abs(gz));
    summary.gyro_peak_dps = (int16_t)(gyro_max * 10 / 131);

    summary.variance = (uint16_t)(mag_mg - 1000); // deviation from 1g rest

    // Event detection flags
    summary.flags = 0;
    if (mag_mg > 3000) summary.flags |= 0x01; // impact > 3g
    if (summary.accel_peak_x_mg < -800) summary.flags |= 0x02; // hard brake
    if (abs(summary.accel_peak_y_mg) > 600) summary.flags |= 0x04; // sharp turn

    return summary;
}

float StateMachine::getSpeedKmh(const GNSSSample& s) {
    return static_cast<float>(s.speed_cmps) * 0.036f;
}

bool StateMachine::isMoving(const GNSSSample& s, const IMUSummary& imu) {
    if (getSpeedKmh(s) >= MIN_SPEED_KMH) return true;
    if (imu.accel_rms_mg > 200) return true;
    return false;
}

// ===========================================================================
// Storage implementations (HAL SDMMC)
// ===========================================================================

bool StateMachine::initSD() {
    bool ok = CairnHAL::sdmmc.init(
        PIN_SDMMC_CMD, PIN_SDMMC_CLK,
        PIN_SDMMC_D0, PIN_SDMMC_D1, PIN_SDMMC_D2, PIN_SDMMC_D3,
        SDMMC_MOUNT_POINT);

    if (!ok) {
        // Fallback to 1-bit mode
        ok = CairnHAL::sdmmc.init1Bit(PIN_SDMMC_CMD, PIN_SDMMC_CLK,
                                       PIN_SDMMC_D0, SDMMC_MOUNT_POINT);
    }

    if (ok) {
        CairnHAL::SDCardInfo info = CairnHAL::sdmmc.getCardInfo();
        Serial.printf("[SD] Mounted: %s, %u-bit, %lluMB free / %lluMB total\n",
                      info.name, info.busWidth,
                      info.freeBytes / (1024*1024),
                      info.totalBytes / (1024*1024));

        // Create base directory structure
        char basePath[48];
        snprintf(basePath, sizeof(basePath), "%s/cairn/trips", SDMMC_MOUNT_POINT);
        CairnHAL::sdmmc.mkdir(basePath);
    }
    return ok;
}

bool StateMachine::openTripFile(const char* tripId) {
    snprintf(tripDir_, sizeof(tripDir_), "%s/cairn/trips/%s",
             SDMMC_MOUNT_POINT, tripId);
    snprintf(samplesPath_, sizeof(samplesPath_), "%s/samples.bin", tripDir_);
    snprintf(imuPath_, sizeof(imuPath_), "%s/imu_summary.bin", tripDir_);

    if (!CairnHAL::sdmmc.mkdir(tripDir_)) {
        Serial.printf("[SD] Failed to create trip dir: %s\n", tripDir_);
        return false;
    }

    // Start streaming SHA-256 hash of samples (hardware-accelerated)
    hashActive_ = CairnHAL::crypto.sha256_start();

    Serial.printf("[SD] Trip dir: %s\n", tripDir_);
    return true;
}

bool StateMachine::writeSample(const GNSSSample& s) {
    bool ok = CairnHAL::sdmmc.append(samplesPath_,
                                      reinterpret_cast<const uint8_t*>(&s),
                                      sizeof(s));
    if (ok && hashActive_) {
        CairnHAL::crypto.sha256_update(
            reinterpret_cast<const uint8_t*>(&s), sizeof(s));
    }
    return ok;
}

bool StateMachine::writeIMUSummary(const IMUSummary& s) {
    return CairnHAL::sdmmc.append(imuPath_,
                                   reinterpret_cast<const uint8_t*>(&s),
                                   sizeof(s));
}

bool StateMachine::finalizeTripBundle() {
    if (gnssSampleCount_ == 0) return true; // nothing to finalize

    addEvent(TripEvent::TRIP_END, "finalized");

    // Flush all buffered writes to physical media
    CairnHAL::sdmmc.fsync(samplesPath_);
    CairnHAL::sdmmc.fsync(imuPath_);

    if (!writeManifest()) return false;
    if (!writeChecksums()) return false;

    Serial.printf("[SD] Bundle finalized: %u GNSS, %u IMU\n",
                  gnssSampleCount_, imuSummaryCount_);
    return true;
}

bool StateMachine::writeManifest() {
    // Build manifest JSON
    char json[512];
    int n = snprintf(json, sizeof(json),
        "{\n"
        "  \"version\": 1,\n"
        "  \"trip_id\": \"%s\",\n"
        "  \"started_at_ms\": %llu,\n"
        "  \"ended_at_ms\": %llu,\n"
        "  \"gnss_sample_count\": %u,\n"
        "  \"imu_summary_count\": %u,\n"
        "  \"gnss_rate_hz\": %u,\n"
        "  \"imu_rate_hz\": %u,\n"
        "  \"schema_version\": 1\n"
        "}",
        currentTripId_,
        (unsigned long long)tripStartMs_,
        (unsigned long long)millis(),
        gnssSampleCount_,
        imuSummaryCount_,
        GNSS_RATE_ACTIVE_HZ,
        IMU_RATE_ACTIVE_HZ);

    char manifestPath[80];
    snprintf(manifestPath, sizeof(manifestPath), "%s/manifest.json", tripDir_);
    return CairnHAL::sdmmc.write(manifestPath,
                                  reinterpret_cast<const uint8_t*>(json), n);
}

bool StateMachine::writeChecksums() {
    // Compute SHA-256 for each file using hardware crypto
    char checksumPath[80];
    snprintf(checksumPath, sizeof(checksumPath), "%s/sha256sums.txt", tripDir_);

    char sums[512];
    int offset = 0;

    // samples.bin — finish the streaming hash
    uint8_t hash[32];
    if (hashActive_) {
        CairnHAL::crypto.sha256_finish(hash);
        hashActive_ = false;
    } else {
        // Fallback: hash entire file
        uint8_t buf[4096];
        int32_t bytesRead = CairnHAL::sdmmc.read(samplesPath_, buf, sizeof(buf));
        if (bytesRead > 0) {
            CairnHAL::HalCrypto::sha256(buf, bytesRead, hash);
        }
    }
    for (int i = 0; i < 32; i++) {
        offset += snprintf(sums + offset, sizeof(sums) - offset, "%02x", hash[i]);
    }
    offset += snprintf(sums + offset, sizeof(sums) - offset, "  samples.bin\n");

    // imu_summary.bin
    uint8_t imuBuf[4096];
    int32_t imuBytes = CairnHAL::sdmmc.read(imuPath_, imuBuf, sizeof(imuBuf));
    if (imuBytes > 0) {
        CairnHAL::HalCrypto::sha256(imuBuf, imuBytes, hash);
        for (int i = 0; i < 32; i++) {
            offset += snprintf(sums + offset, sizeof(sums) - offset, "%02x", hash[i]);
        }
        offset += snprintf(sums + offset, sizeof(sums) - offset,
                           "  imu_summary.bin\n");
    }

    // manifest.json
    char manifestPath[80];
    snprintf(manifestPath, sizeof(manifestPath), "%s/manifest.json", tripDir_);
    uint8_t mBuf[1024];
    int32_t mBytes = CairnHAL::sdmmc.read(manifestPath, mBuf, sizeof(mBuf));
    if (mBytes > 0) {
        CairnHAL::HalCrypto::sha256(mBuf, mBytes, hash);
        for (int i = 0; i < 32; i++) {
            offset += snprintf(sums + offset, sizeof(sums) - offset, "%02x", hash[i]);
        }
        offset += snprintf(sums + offset, sizeof(sums) - offset,
                           "  manifest.json\n");
    }

    return CairnHAL::sdmmc.write(checksumPath,
                                  reinterpret_cast<const uint8_t*>(sums), offset);
}

// ===========================================================================
// Connectivity implementations (HAL Wi-Fi)
// ===========================================================================

bool StateMachine::scanForTrustedNetwork() {
    if (!wifiCredsLoaded_) return false;
    return CairnHAL::wifi.scanForBSSID(trustedBSSID_);
}

bool StateMachine::connectToHome() {
    if (!wifiCredsLoaded_) return false;
    return CairnHAL::wifi.connectTrusted(
        wifiCreds_.ssid, wifiCreds_.psk, trustedBSSID_, 10000);
}

bool StateMachine::uploadBundle(const char* tripId) {
    if (!CairnHAL::wifi.isConnected()) return false;

    // Read the entire bundle directory into a tar-like blob for upload
    // In production this would stream files individually using the
    // chunked upload protocol. For now, read samples.bin and upload it.

    // Compute content hash of the bundle for deduplication
    uint8_t bundleHash[32];
    char samplesFile[80];
    snprintf(samplesFile, sizeof(samplesFile),
             "%s/cairn/trips/%s/samples.bin", SDMMC_MOUNT_POINT, tripId);

    // Read file in chunks, hash with hardware SHA-256
    CairnHAL::crypto.sha256_start();
    uint8_t readBuf[UPLOAD_CHUNK_SIZE];
    int32_t totalSize = 0;
    FILE* f = fopen(samplesFile, "rb");
    if (!f) return false;

    while (true) {
        size_t n = fread(readBuf, 1, sizeof(readBuf), f);
        if (n == 0) break;
        CairnHAL::crypto.sha256_update(readBuf, n);
        totalSize += n;
    }
    fclose(f);
    CairnHAL::crypto.sha256_finish(bundleHash);

    // Convert hash to hex string
    char hashHex[65];
    for (int i = 0; i < 32; i++) {
        snprintf(hashHex + i * 2, 3, "%02x", bundleHash[i]);
    }

    // TODO: Implement HTTP client for resumable upload protocol
    // POST /api/v1/upload/init  { device_id, content_hash, size }
    // PUT  /api/v1/upload/{id}/chunk (X-Upload-Offset header)
    // POST /api/v1/upload/{id}/finalize { content_hash }
    //
    // For now, log what would be uploaded.
    Serial.printf("[SYNC] Would upload %s (%d bytes, hash=%s)\n",
                  tripId, totalSize, hashHex);

    // Store receipt stub in NVS
    CairnHAL::nvs.storeServerReceipt(tripId, bundleHash, 32);
    CairnHAL::nvs.commit();

    return true;
}

// ===========================================================================
// Power implementation (HAL)
// ===========================================================================

float StateMachine::getBatteryVoltage() {
    return CairnHAL::power.readBatteryVoltageHW();
}

// ===========================================================================
// Helpers
// ===========================================================================

void StateMachine::generateTripId(char* buf, size_t len) {
    // Generate a ULID-like ID using hardware RNG for randomness
    // Format: timestamp (10 chars base32) + random (16 chars base32) = 26 chars
    static const char base32[] = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

    uint64_t ts = millis(); // In production, use real epoch time from GNSS
    uint8_t rnd[10];
    CairnHAL::HalCrypto::generate_random(rnd, sizeof(rnd));

    // Encode timestamp (10 chars, big-endian base32)
    for (int i = 9; i >= 0; i--) {
        buf[i] = base32[ts & 0x1F];
        ts >>= 5;
    }
    // Encode randomness (16 chars)
    int ri = 0;
    for (int i = 10; i < 26; i++) {
        buf[i] = base32[rnd[ri % sizeof(rnd)] & 0x1F];
        rnd[ri % sizeof(rnd)] >>= 5;
        if (rnd[ri % sizeof(rnd)] == 0) ri++;
        else continue;
        ri++;
    }
    buf[26] = '\0';
}

void StateMachine::addEvent(TripEvent::EventType type, const char* details) {
    if (eventCount_ >= MAX_EVENTS) return;

    TripEvent& e = events_[eventCount_++];
    e.type = type;
    e.timestamp_ms = millis();
    e.latitude = 0;  // populated from last GNSS fix in production
    e.longitude = 0;
    strncpy(e.details, details, sizeof(e.details) - 1);
    e.details[sizeof(e.details) - 1] = '\0';
}
