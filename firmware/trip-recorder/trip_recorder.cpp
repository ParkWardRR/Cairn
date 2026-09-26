#include "trip_recorder.h"
#include <Arduino.h>
#include <SD.h>
#include <FS.h>
#include <cstring>
#include <cstdio>

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------
TripRecorder::TripRecorder()
    : _state(TripRecorderState::INACTIVE)
    , _startTimeMs(0)
    , _totalGNSSSamples(0)
    , _totalIMUSummaries(0)
    , _totalIMURawWindows(0)
    , _eventCount(0)
    , _gnssFileOpen(false)
    , _imuFileOpen(false)
{
    memset(_tripId, 0, sizeof(_tripId));
    memset(_deviceId, 0, sizeof(_deviceId));
    memset(_firmwareVersion, 0, sizeof(_firmwareVersion));
}

// ---------------------------------------------------------------------------
// Trip lifecycle
// ---------------------------------------------------------------------------
bool TripRecorder::begin(const char* deviceId, const char* firmwareVersion) {
    if (_state != TripRecorderState::INACTIVE) {
        Serial.println("[TripRecorder] begin() called while already active");
        return false;
    }

    generateTripId(_tripId, sizeof(_tripId));
    strncpy(_deviceId, deviceId, sizeof(_deviceId) - 1);
    strncpy(_firmwareVersion, firmwareVersion, sizeof(_firmwareVersion) - 1);

    _startTimeMs = millis();
    _totalGNSSSamples = 0;
    _totalIMUSummaries = 0;
    _totalIMURawWindows = 0;
    _eventCount = 0;
    _gnssBuf.clear();
    _imuBuf.clear();

    // Create trip directory
    char dirPath[64];
    snprintf(dirPath, sizeof(dirPath), "/cairn/trips/%s", _tripId);
    if (!SD.mkdir(dirPath)) {
        Serial.printf("[TripRecorder] Failed to create directory: %s\n", dirPath);
        return false;
    }

    _state = TripRecorderState::RECORDING;
    Serial.printf("[TripRecorder] Trip started: %s\n", _tripId);

    // Log start event
    addEvent(TripEvent::TRIP_START, 0, 0, "trip_begin");

    return true;
}

void TripRecorder::end() {
    if (_state == TripRecorderState::INACTIVE) return;

    addEvent(TripEvent::TRIP_END, 0, 0, "trip_end");
    flush();
    finalize();

    _state = TripRecorderState::INACTIVE;
    Serial.printf("[TripRecorder] Trip ended: %s\n", _tripId);
}

// ---------------------------------------------------------------------------
// Sample ingestion
// ---------------------------------------------------------------------------
bool TripRecorder::addGNSSSample(const GNSSSample& sample) {
    if (_state != TripRecorderState::RECORDING) return false;

    if (_gnssBuf.isFull()) {
        if (!flush()) {
            Serial.println("[TripRecorder] GNSS buffer full and flush failed");
            return false;
        }
    }

    _gnssBuf.push(sample);
    _totalGNSSSamples++;
    return true;
}

bool TripRecorder::addIMUSummary(const IMUSummary& summary) {
    if (_state != TripRecorderState::RECORDING) return false;

    if (_imuBuf.isFull()) {
        if (!flush()) {
            Serial.println("[TripRecorder] IMU buffer full and flush failed");
            return false;
        }
    }

    _imuBuf.push(summary);
    _totalIMUSummaries++;
    return true;
}

bool TripRecorder::addIMURawWindow(const IMURawSample* window, size_t count) {
    if (_state != TripRecorderState::RECORDING) return false;
    if (window == nullptr || count == 0) return false;

    char filePath[96];
    snprintf(filePath, sizeof(filePath), "/cairn/trips/%s/imu_raw_%04u.bin",
             _tripId, _totalIMURawWindows);

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to open IMU raw file: %s\n", filePath);
        return false;
    }

    size_t written = f.write(reinterpret_cast<const uint8_t*>(window),
                             count * sizeof(IMURawSample));
    f.close();

    if (written != count * sizeof(IMURawSample)) {
        Serial.println("[TripRecorder] Partial IMU raw write");
        return false;
    }

    _totalIMURawWindows++;
    return true;
}

bool TripRecorder::addEvent(TripEvent::EventType type, int32_t lat, int32_t lon,
                            const char* details) {
    if (_eventCount >= MAX_EVENTS) {
        Serial.println("[TripRecorder] Event buffer full");
        return false;
    }

    TripEvent& evt = _events[_eventCount++];
    evt.type = type;
    evt.timestamp_ms = millis();
    evt.latitude = lat;
    evt.longitude = lon;
    memset(evt.details, 0, sizeof(evt.details));
    if (details) {
        strncpy(evt.details, details, sizeof(evt.details) - 1);
    }

    return true;
}

// ---------------------------------------------------------------------------
// Flush — write buffered samples to SD as raw binary
// ---------------------------------------------------------------------------
bool TripRecorder::flush() {
    if (_state != TripRecorderState::RECORDING) return false;

    bool ok = true;
    if (!_gnssBuf.isEmpty()) {
        ok = writeGNSSBuffer() && ok;
    }
    if (!_imuBuf.isEmpty()) {
        ok = writeIMUBuffer() && ok;
    }

    return ok;
}

bool TripRecorder::writeGNSSBuffer() {
    char filePath[96];
    buildFilePath("samples.bin", filePath, sizeof(filePath));

    File f = SD.open(filePath, FILE_APPEND);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to open GNSS file: %s\n", filePath);
        return false;
    }

    GNSSSample sample;
    size_t written = 0;
    while (_gnssBuf.pop(sample)) {
        size_t n = f.write(reinterpret_cast<const uint8_t*>(&sample),
                           sizeof(GNSSSample));
        if (n != sizeof(GNSSSample)) {
            Serial.println("[TripRecorder] Partial GNSS sample write");
            f.close();
            return false;
        }
        written++;
    }

    f.close();
    Serial.printf("[TripRecorder] Flushed %u GNSS samples\n", (unsigned)written);
    return true;
}

bool TripRecorder::writeIMUBuffer() {
    char filePath[96];
    buildFilePath("imu_summaries.bin", filePath, sizeof(filePath));

    File f = SD.open(filePath, FILE_APPEND);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to open IMU summary file: %s\n",
                      filePath);
        return false;
    }

    IMUSummary summary;
    size_t written = 0;
    while (_imuBuf.pop(summary)) {
        size_t n = f.write(reinterpret_cast<const uint8_t*>(&summary),
                           sizeof(IMUSummary));
        if (n != sizeof(IMUSummary)) {
            Serial.println("[TripRecorder] Partial IMU summary write");
            f.close();
            return false;
        }
        written++;
    }

    f.close();
    Serial.printf("[TripRecorder] Flushed %u IMU summaries\n", (unsigned)written);
    return true;
}

// ---------------------------------------------------------------------------
// Finalize — close trip, write manifest + events + checksums
// ---------------------------------------------------------------------------
bool TripRecorder::finalize() {
    _state = TripRecorderState::FINALIZING;

    bool ok = true;
    ok = writeManifest() && ok;
    ok = writeEvents() && ok;
    ok = writeChecksums() && ok;

    return ok;
}

bool TripRecorder::writeManifest() {
    char filePath[96];
    buildFilePath("manifest.json", filePath, sizeof(filePath));

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to create manifest: %s\n", filePath);
        return false;
    }

    uint64_t elapsedMs = millis() - _startTimeMs;

    // Write manifest as JSON for v1 firmware (simpler than CBOR)
    f.printf("{\n");
    f.printf("  \"version\": 1,\n");
    f.printf("  \"schema_version\": 1,\n");
    f.printf("  \"trip_id\": \"%s\",\n", _tripId);
    f.printf("  \"device_id\": \"%s\",\n", _deviceId);
    f.printf("  \"firmware_version\": \"%s\",\n", _firmwareVersion);
    f.printf("  \"started_at_ms\": %llu,\n", (unsigned long long)_startTimeMs);
    f.printf("  \"ended_at_ms\": %llu,\n",
             (unsigned long long)(_startTimeMs + elapsedMs));
    f.printf("  \"duration_ms\": %llu,\n", (unsigned long long)elapsedMs);
    f.printf("  \"gnss_sample_count\": %u,\n", (unsigned)_totalGNSSSamples);
    f.printf("  \"imu_summary_count\": %u,\n", (unsigned)_totalIMUSummaries);
    f.printf("  \"imu_raw_window_count\": %u,\n", (unsigned)_totalIMURawWindows);
    f.printf("  \"event_count\": %u\n", (unsigned)_eventCount);
    f.printf("}\n");

    f.close();
    Serial.printf("[TripRecorder] Manifest written: %s\n", filePath);
    return true;
}

bool TripRecorder::writeEvents() {
    char filePath[96];
    buildFilePath("events.json", filePath, sizeof(filePath));

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to create events file: %s\n", filePath);
        return false;
    }

    f.print("[\n");
    for (size_t i = 0; i < _eventCount; i++) {
        const TripEvent& evt = _events[i];
        f.printf("  {\"type\": %u, \"timestamp_ms\": %llu, "
                 "\"lat\": %ld, \"lon\": %ld, \"details\": \"%s\"}",
                 (unsigned)evt.type,
                 (unsigned long long)evt.timestamp_ms,
                 (long)evt.latitude,
                 (long)evt.longitude,
                 evt.details);
        if (i < _eventCount - 1) f.print(",");
        f.print("\n");
    }
    f.print("]\n");

    f.close();
    Serial.printf("[TripRecorder] Events written: %u events\n",
                  (unsigned)_eventCount);
    return true;
}

bool TripRecorder::writeChecksums() {
    // SHA256 is stubbed for v1 — write placeholder hashes.
    // A real implementation would compute SHA256 of each file.
    char filePath[96];
    buildFilePath("sha256sums.txt", filePath, sizeof(filePath));

    File f = SD.open(filePath, FILE_WRITE);
    if (!f) {
        Serial.printf("[TripRecorder] Failed to create checksums file: %s\n",
                      filePath);
        return false;
    }

    static const char* PLACEHOLDER_HASH =
        "0000000000000000000000000000000000000000000000000000000000000000";

    const char* files[] = {
        "manifest.json", "samples.bin", "imu_summaries.bin", "events.json"
    };
    for (const char* name : files) {
        f.printf("%s  %s\n", PLACEHOLDER_HASH, name);
    }

    f.close();
    Serial.println("[TripRecorder] Checksums written (placeholder)");
    return true;
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------
bool TripRecorder::isActive() const {
    return _state == TripRecorderState::RECORDING;
}

uint32_t TripRecorder::getSampleCount() const {
    return _totalGNSSSamples;
}

uint32_t TripRecorder::getIMUSummaryCount() const {
    return _totalIMUSummaries;
}

uint32_t TripRecorder::getElapsedMs() const {
    if (_state == TripRecorderState::INACTIVE) return 0;
    return (uint32_t)(millis() - _startTimeMs);
}

const char* TripRecorder::getTripId() const {
    return _tripId;
}

TripRecorderState TripRecorder::getState() const {
    return _state;
}

// ---------------------------------------------------------------------------
// Trip ID generation — timestamp hex + random suffix (ULID-like)
// ---------------------------------------------------------------------------
void TripRecorder::generateTripId(char* out, size_t outLen) {
    if (outLen < 27) {
        if (outLen > 0) out[0] = '\0';
        return;
    }

    // 10 hex chars from millis timestamp, 16 hex chars from random
    uint64_t now = millis();
    uint32_t r1 = esp_random();
    uint32_t r2 = esp_random();

    snprintf(out, outLen, "%010llX%08lX%08lX",
             (unsigned long long)now,
             (unsigned long)r1,
             (unsigned long)r2);
}

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------
void TripRecorder::buildFilePath(const char* filename, char* out,
                                 size_t outLen) const {
    snprintf(out, outLen, "/cairn/trips/%s/%s", _tripId, filename);
}
