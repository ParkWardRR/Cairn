// ---------------------------------------------------------------------------
// Cairn HAL -- Non-Volatile Storage (Encrypted)
//
// ESP32 NVS (Non-Volatile Storage) stores key-value pairs in a dedicated
// flash partition.  ESP-IDF supports NVS encryption using a key stored
// in eFuse, which transparently encrypts all values at rest.
//
// Cairn uses NVS for:
//   - Device identity Ed25519 private key (encrypted)
//   - Trusted Wi-Fi credentials (encrypted)
//   - Server sync receipts (proof that a trip was uploaded)
//   - Device configuration that must survive deep sleep and power loss
//
// NVS encryption requires:
//   1. Flash encryption enabled in eFuse (one-time provisioning)
//   2. An NVS encryption key partition ("nvs_key") in the partition table
//   3. CONFIG_NVS_ENCRYPTION=y in sdkconfig
//
// Without flash encryption, NVS still works but values are stored in
// plaintext.  The API is identical either way.
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_NVS_H
#define CAIRN_HAL_NVS_H

#include <cstdint>
#include <cstddef>

#include "nvs_flash.h"
#include "nvs.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Wi-Fi credential record stored in NVS
// -----------------------------------------------------------------------
struct NVSWiFiCredentials {
    char    ssid[33];
    char    psk[65];
    uint8_t bssid[6];
    bool    valid;
};

// -----------------------------------------------------------------------
// HalNVS -- encrypted non-volatile key-value storage
// -----------------------------------------------------------------------
class HalNVS {
public:
    // Initialize the NVS subsystem.
    //   encrypted -- if true, attempt to use NVS encryption.
    //                Requires flash encryption to be enabled in eFuse.
    //                Falls back to plaintext if encryption keys are
    //                not available (logs a warning).
    bool init(bool encrypted = false);

    // -- Device identity key (Ed25519) -----------------------------------

    // Store the device's Ed25519 private key.
    // This is the most sensitive datum -- in a production build it
    // should be stored in encrypted NVS or directly in eFuse.
    bool storeDeviceKey(const uint8_t* key, size_t len);

    // Load the device's Ed25519 private key.
    //   key    -- output buffer (must be at least maxLen bytes)
    //   maxLen -- buffer size
    //   Returns the number of bytes read, or -1 on error.
    int32_t loadDeviceKey(uint8_t* key, size_t maxLen);

    // Store the device's Ed25519 public key.
    bool storeDevicePublicKey(const uint8_t* key, size_t len);

    // Load the device's Ed25519 public key.
    int32_t loadDevicePublicKey(uint8_t* key, size_t maxLen);

    // Check whether a device key pair has been provisioned.
    bool hasDeviceKey();

    // -- Wi-Fi credentials -----------------------------------------------

    // Store trusted Wi-Fi network credentials.
    //   ssid  -- network name (max 32 chars)
    //   psk   -- pre-shared key (max 64 chars)
    //   bssid -- 6-byte MAC address of trusted AP
    bool storeWiFiCredentials(const char* ssid, const char* psk,
                              const uint8_t bssid[6]);

    // Load trusted Wi-Fi network credentials.
    bool loadWiFiCredentials(NVSWiFiCredentials& creds);

    // Check if Wi-Fi credentials are stored.
    bool hasWiFiCredentials();

    // Clear stored Wi-Fi credentials.
    bool clearWiFiCredentials();

    // -- Server sync receipts --------------------------------------------

    // Store a server receipt (cryptographic proof of upload).
    //   tripId  -- ULID or hex trip identifier
    //   receipt -- receipt bytes (e.g. server signature)
    //   len     -- receipt length
    bool storeServerReceipt(const char* tripId,
                            const uint8_t* receipt, size_t len);

    // Load a server receipt for a trip.
    //   tripId  -- trip identifier
    //   receipt -- output buffer
    //   maxLen  -- buffer size
    //   Returns bytes read, or -1 if not found.
    int32_t loadServerReceipt(const char* tripId,
                              uint8_t* receipt, size_t maxLen);

    // Check if a server receipt exists for a trip.
    bool hasReceipt(const char* tripId);

    // Delete a server receipt (after pruning the trip from SD).
    bool deleteReceipt(const char* tripId);

    // -- Generic key-value operations ------------------------------------

    // Store an arbitrary blob.
    bool storeBlob(const char* key, const uint8_t* data, size_t len);

    // Load an arbitrary blob.  Returns bytes read or -1.
    int32_t loadBlob(const char* key, uint8_t* data, size_t maxLen);

    // Store a uint32 value.
    bool storeU32(const char* key, uint32_t value);

    // Load a uint32 value.
    bool loadU32(const char* key, uint32_t* value);

    // Store a string.
    bool storeString(const char* key, const char* value);

    // Load a string.  Returns bytes read or -1.
    int32_t loadString(const char* key, char* buf, size_t maxLen);

    // Delete a key.
    bool eraseKey(const char* key);

    // Commit any pending writes to flash.
    bool commit();

    // Erase the entire NVS namespace (factory reset).
    bool eraseAll();

    // Close the NVS handle.
    void deinit();

private:
    nvs_handle_t handle_ = 0;
    bool         initialized_ = false;
    bool         encrypted_ = false;

    // NVS namespace for Cairn data.
    static constexpr const char* NAMESPACE = "cairn";

    // Key prefixes for different data types.
    static constexpr const char* KEY_DEVICE_PRIVKEY = "dev_privkey";
    static constexpr const char* KEY_DEVICE_PUBKEY  = "dev_pubkey";
    static constexpr const char* KEY_WIFI_SSID      = "wifi_ssid";
    static constexpr const char* KEY_WIFI_PSK       = "wifi_psk";
    static constexpr const char* KEY_WIFI_BSSID     = "wifi_bssid";
    static constexpr const char* KEY_RECEIPT_PREFIX  = "rcpt_";
};

} // namespace CairnHAL

#endif // CAIRN_HAL_NVS_H
