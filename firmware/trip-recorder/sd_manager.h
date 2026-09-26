#ifndef CAIRN_SD_MANAGER_H
#define CAIRN_SD_MANAGER_H

#include <cstdint>
#include <cstddef>

// ---------------------------------------------------------------------------
// SDManager — microSD management, capacity checks, file rotation
// ---------------------------------------------------------------------------
class SDManager {
public:
    SDManager();

    // Initialization — mount SD card, create base directories
    bool init();

    // Capacity
    uint32_t getFreeSpaceMB() const;
    bool     hasCapacity(uint32_t requiredMB = 50) const;

    // Trip directory management
    bool createTripDirectory(const char* tripId);
    void getTripBundlePath(const char* tripId, const char* filename,
                           char* outPath, size_t outLen) const;

    // File operations
    bool   openFile(const char* path, bool append = false);
    size_t writeData(const uint8_t* data, size_t len);
    void   closeFile();
    bool   fileExists(const char* path) const;
    bool   deleteFile(const char* path);

    // Trip enumeration and pruning
    size_t listTrips(char tripIds[][28], size_t maxTrips) const;
    bool   getOldestPrunableTrip(char* tripId) const;
    bool   pruneTrip(const char* tripId);

    // Card state
    bool isReady() const;

private:
    static constexpr uint32_t MIN_FREE_SD_MB = 50;
    static constexpr const char* BASE_PATH    = "/cairn/trips";

    bool _mounted;
    bool _fileOpen;

    // Recursive directory deletion helper
    bool removeDirectoryRecursive(const char* path);
};

#endif // CAIRN_SD_MANAGER_H
