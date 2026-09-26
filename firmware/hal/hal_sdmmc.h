// ---------------------------------------------------------------------------
// Cairn HAL -- SDMMC Native SD Host Controller
//
// The ESP32 has a dedicated SDMMC host controller that supports 4-bit
// parallel SD card access.  This is fundamentally different from the
// SPI-mode SD access used by Arduino's SD.h library:
//
//   SPI mode:  1-bit data, shared SPI bus, ~4 MB/s theoretical max
//   SDMMC 1-bit: dedicated SD bus, ~20 MB/s
//   SDMMC 4-bit: 4 parallel data lines, ~40 MB/s theoretical max
//
// For Cairn's trip recording, SDMMC means:
//   - Trip bundle finalization (hashing + writing manifest) is 4-8x faster
//   - Large bundle uploads can be read from SD at full speed
//   - Reduced CPU time spent on SD I/O (DMA handles the transfers)
//
// The SDMMC host uses DMA internally -- all data transfers between the
// SD card and memory go through the DMA engine without CPU involvement.
//
// Pin mapping for Freematics ONE+ Model B (verify against schematic):
//   CLK  = GPIO 14
//   CMD  = GPIO 15
//   D0   = GPIO 2
//   D1   = GPIO 4   (4-bit mode only)
//   D2   = GPIO 12  (4-bit mode only)
//   D3   = GPIO 13  (4-bit mode only)
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_SDMMC_H
#define CAIRN_HAL_SDMMC_H

#include <cstdint>
#include <cstddef>

#include "driver/sdmmc_host.h"
#include "esp_vfs_fat.h"
#include "sdmmc_cmd.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// SD card information
// -----------------------------------------------------------------------
struct SDCardInfo {
    uint64_t totalBytes;
    uint64_t freeBytes;
    uint32_t sectorSize;
    uint32_t clusterSize;
    char     name[16];          // Card name from CID register
    uint8_t  busWidth;          // 1 or 4
    uint32_t maxFreqKHz;        // Negotiated bus frequency
    bool     isUHS;             // Ultra High Speed capable
};

// -----------------------------------------------------------------------
// HalSDMMC -- native SDMMC host controller access
// -----------------------------------------------------------------------
class HalSDMMC {
public:
    // Mount the SD card via the native SDMMC 4-bit parallel interface.
    // This is ~4-8x faster than SPI-mode SD access.
    //   cmd_pin, clk_pin, d0..d3 -- GPIO pin numbers for the SDMMC bus
    //   mountPoint -- VFS mount point (default "/sdcard")
    //
    // Internally calls sdmmc_host_init(), configures 4-bit bus width,
    // and mounts a FAT filesystem via esp_vfs_fat_sdmmc_mount().
    // All subsequent file operations via standard POSIX I/O (fopen, etc.)
    // are routed through the SDMMC DMA engine.
    bool init(int cmd_pin, int clk_pin,
              int d0_pin, int d1_pin, int d2_pin, int d3_pin,
              const char* mountPoint = "/sdcard");

    // Fallback: mount in 1-bit SDMMC mode.
    // Use this if the board does not wire all 4 data lines.
    // Still faster than SPI mode (~2x vs SPI, ~0.5x vs 4-bit).
    bool init1Bit(int cmd_pin, int clk_pin, int d0_pin,
                  const char* mountPoint = "/sdcard");

    // Write a complete file (creates or overwrites).
    // Uses POSIX fwrite which is routed through SDMMC DMA.
    bool write(const char* path, const uint8_t* data, size_t len);

    // Append data to an existing file (creates if not present).
    bool append(const char* path, const uint8_t* data, size_t len);

    // Read a file into a buffer.  Returns the number of bytes read,
    // or -1 on error.
    int32_t read(const char* path, uint8_t* buf, size_t maxLen);

    // Force-flush all buffered writes to physical media.
    // Important after writing trip data to ensure crash consistency.
    bool fsync(const char* path);

    // Check if a file exists.
    bool exists(const char* path) const;

    // Delete a file.
    bool remove(const char* path);

    // Create a directory (and parents if needed).
    bool mkdir(const char* path);

    // Get free bytes on the filesystem.
    uint64_t getFreeBytes() const;

    // Get total filesystem capacity.
    uint64_t getTotalBytes() const;

    // Get detailed card information.
    SDCardInfo getCardInfo() const;

    // Unmount the filesystem and release the SDMMC host.
    void deinit();

    // Check if the card is mounted and accessible.
    bool isMounted() const;

private:
    sdmmc_card_t* card_ = nullptr;
    char          mountPoint_[16] = {};
    bool          mounted_ = false;
    uint8_t       busWidth_ = 0;

    // Internal: configure SDMMC slot with given pin mapping and bus width.
    bool mountCard(const sdmmc_host_t& hostConfig,
                   const sdmmc_slot_config_t& slotConfig,
                   const char* mountPoint);
};

} // namespace CairnHAL

#endif // CAIRN_HAL_SDMMC_H
