#include "sd_manager.h"
#include <Arduino.h>
#include <SD.h>
#include <FS.h>
#include <cstring>
#include <cstdio>

// ---------------------------------------------------------------------------
// We keep a module-level File handle since the ESP32 SD library's File
// objects are reference-counted and lightweight.
// ---------------------------------------------------------------------------
static File _currentFile;

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------
SDManager::SDManager()
    : _mounted(false)
    , _fileOpen(false)
{
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------
bool SDManager::init() {
    if (!SD.begin()) {
        Serial.println("[SDManager] SD.begin() failed — card not detected");
        _mounted = false;
        return false;
    }

    _mounted = true;
    Serial.printf("[SDManager] SD card mounted. Total: %llu MB, Used: %llu MB\n",
                  (unsigned long long)(SD.totalBytes() / (1024 * 1024)),
                  (unsigned long long)(SD.usedBytes() / (1024 * 1024)));

    // Ensure base directory structure exists
    if (!SD.exists("/cairn")) {
        SD.mkdir("/cairn");
    }
    if (!SD.exists(BASE_PATH)) {
        SD.mkdir(BASE_PATH);
    }

    if (!hasCapacity()) {
        Serial.printf("[SDManager] Warning: free space below %u MB threshold\n",
                      (unsigned)MIN_FREE_SD_MB);
    }

    return true;
}

// ---------------------------------------------------------------------------
// Capacity
// ---------------------------------------------------------------------------
uint32_t SDManager::getFreeSpaceMB() const {
    if (!_mounted) return 0;
    uint64_t totalBytes = SD.totalBytes();
    uint64_t usedBytes  = SD.usedBytes();
    if (usedBytes > totalBytes) return 0;
    return (uint32_t)((totalBytes - usedBytes) / (1024ULL * 1024ULL));
}

bool SDManager::hasCapacity(uint32_t requiredMB) const {
    return getFreeSpaceMB() >= requiredMB;
}

// ---------------------------------------------------------------------------
// Trip directory management
// ---------------------------------------------------------------------------
bool SDManager::createTripDirectory(const char* tripId) {
    if (!_mounted) {
        Serial.println("[SDManager] Cannot create directory — SD not mounted");
        return false;
    }

    char path[96];
    snprintf(path, sizeof(path), "%s/%s", BASE_PATH, tripId);

    if (SD.exists(path)) {
        Serial.printf("[SDManager] Trip directory already exists: %s\n", path);
        return true;
    }

    if (!SD.mkdir(path)) {
        Serial.printf("[SDManager] Failed to create trip directory: %s\n", path);
        return false;
    }

    Serial.printf("[SDManager] Created trip directory: %s\n", path);
    return true;
}

void SDManager::getTripBundlePath(const char* tripId, const char* filename,
                                  char* outPath, size_t outLen) const {
    snprintf(outPath, outLen, "%s/%s/%s", BASE_PATH, tripId, filename);
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------
bool SDManager::openFile(const char* path, bool append) {
    if (!_mounted) {
        Serial.println("[SDManager] Cannot open file — SD not mounted");
        return false;
    }

    if (_fileOpen) {
        Serial.println("[SDManager] Closing previously open file");
        closeFile();
    }

    _currentFile = SD.open(path, append ? FILE_APPEND : FILE_WRITE);
    if (!_currentFile) {
        Serial.printf("[SDManager] Failed to open file: %s\n", path);
        return false;
    }

    _fileOpen = true;
    return true;
}

size_t SDManager::writeData(const uint8_t* data, size_t len) {
    if (!_fileOpen || !_currentFile) {
        Serial.println("[SDManager] writeData called with no file open");
        return 0;
    }

    size_t written = _currentFile.write(data, len);
    if (written != len) {
        Serial.printf("[SDManager] Partial write: %u of %u bytes\n",
                      (unsigned)written, (unsigned)len);
    }
    return written;
}

void SDManager::closeFile() {
    if (_fileOpen && _currentFile) {
        _currentFile.close();
    }
    _fileOpen = false;
}

bool SDManager::fileExists(const char* path) const {
    if (!_mounted) return false;
    return SD.exists(path);
}

bool SDManager::deleteFile(const char* path) {
    if (!_mounted) {
        Serial.println("[SDManager] Cannot delete file — SD not mounted");
        return false;
    }

    if (!SD.exists(path)) {
        Serial.printf("[SDManager] File does not exist: %s\n", path);
        return false;
    }

    if (!SD.remove(path)) {
        Serial.printf("[SDManager] Failed to delete file: %s\n", path);
        return false;
    }

    return true;
}

// ---------------------------------------------------------------------------
// Trip enumeration
// ---------------------------------------------------------------------------
size_t SDManager::listTrips(char tripIds[][28], size_t maxTrips) const {
    if (!_mounted) return 0;

    File dir = SD.open(BASE_PATH);
    if (!dir || !dir.isDirectory()) {
        Serial.println("[SDManager] Cannot open trips directory");
        return 0;
    }

    size_t count = 0;
    File entry = dir.openNextFile();
    while (entry && count < maxTrips) {
        if (entry.isDirectory()) {
            const char* name = entry.name();
            // On ESP32 SD, entry.name() may return the full path or just the
            // filename depending on the library version. Extract just the
            // directory name.
            const char* lastSlash = strrchr(name, '/');
            const char* dirName = lastSlash ? (lastSlash + 1) : name;

            // Skip hidden directories
            if (dirName[0] != '.') {
                strncpy(tripIds[count], dirName, 27);
                tripIds[count][27] = '\0';
                count++;
            }
        }
        entry = dir.openNextFile();
    }

    dir.close();

    // Trip IDs are timestamp-prefixed, so lexicographic sort == chronological
    // Simple insertion sort (small N)
    for (size_t i = 1; i < count; i++) {
        char temp[28];
        memcpy(temp, tripIds[i], 28);
        size_t j = i;
        while (j > 0 && strcmp(tripIds[j - 1], temp) > 0) {
            memcpy(tripIds[j], tripIds[j - 1], 28);
            j--;
        }
        memcpy(tripIds[j], temp, 28);
    }

    Serial.printf("[SDManager] Found %u trips\n", (unsigned)count);
    return count;
}

bool SDManager::getOldestPrunableTrip(char* tripId) const {
    // Find the oldest trip (first in sorted order).
    // In a full implementation, we would check whether the trip has been synced
    // before marking it as prunable. For now, just return the oldest.
    char trips[64][28];
    size_t count = listTrips(trips, 64);

    if (count == 0) return false;

    strncpy(tripId, trips[0], 27);
    tripId[27] = '\0';
    return true;
}

bool SDManager::pruneTrip(const char* tripId) {
    if (!_mounted) {
        Serial.println("[SDManager] Cannot prune — SD not mounted");
        return false;
    }

    char dirPath[96];
    snprintf(dirPath, sizeof(dirPath), "%s/%s", BASE_PATH, tripId);

    if (!SD.exists(dirPath)) {
        Serial.printf("[SDManager] Trip directory does not exist: %s\n", dirPath);
        return false;
    }

    // Delete known trip files, then remove the directory
    const char* tripFiles[] = {
        "manifest.json",
        "samples.bin",
        "imu_summaries.bin",
        "events.json",
        "sha256sums.txt"
    };

    for (const char* filename : tripFiles) {
        char filePath[128];
        snprintf(filePath, sizeof(filePath), "%s/%s", dirPath, filename);
        if (SD.exists(filePath)) {
            SD.remove(filePath);
        }
    }

    // Also clean up any IMU raw window files (imu_raw_NNNN.bin)
    if (!removeDirectoryRecursive(dirPath)) {
        Serial.printf("[SDManager] Failed to fully remove: %s\n", dirPath);
        return false;
    }

    Serial.printf("[SDManager] Pruned trip: %s\n", tripId);
    return true;
}

// ---------------------------------------------------------------------------
// Recursive directory deletion
// ---------------------------------------------------------------------------
bool SDManager::removeDirectoryRecursive(const char* path) {
    File dir = SD.open(path);
    if (!dir || !dir.isDirectory()) {
        return SD.remove(path);
    }

    File entry = dir.openNextFile();
    while (entry) {
        char childPath[128];
        snprintf(childPath, sizeof(childPath), "%s/%s", path, entry.name());

        if (entry.isDirectory()) {
            removeDirectoryRecursive(childPath);
        } else {
            SD.remove(childPath);
        }
        entry = dir.openNextFile();
    }

    dir.close();
    return SD.rmdir(path);
}

// ---------------------------------------------------------------------------
// Card state
// ---------------------------------------------------------------------------
bool SDManager::isReady() const {
    return _mounted;
}
