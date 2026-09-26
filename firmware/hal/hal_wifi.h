// ---------------------------------------------------------------------------
// Cairn HAL -- Wi-Fi with Hardware Offload
//
// ESP32 Wi-Fi hardware features used:
//
//   - WPA2/WPA3 encryption in hardware: the Wi-Fi peripheral handles
//     CCMP/GCMP encryption/decryption without CPU involvement.
//
//   - Hardware crypto for TLS: mbedtls TLS handshakes and record
//     encryption use the hardware AES and SHA engines automatically
//     (same acceleration as hal_crypto).
//
//   - BSSID-locked association: connect to a specific access point by
//     MAC address, preventing roaming to a neighbor's identically-named
//     network.  Critical for trusted-network sync.
//
//   - Modem sleep / DTIM power save: between beacon intervals the Wi-Fi
//     radio powers down, reducing current from ~120 mA (active TX/RX) to
//     ~20 mA.  The radio wakes at DTIM intervals to check for buffered
//     frames.
//
//   - Full radio power-off: when Wi-Fi is not needed (during trip
//     recording), the entire radio subsystem can be disabled to save
//     ~80 mA.
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_WIFI_H
#define CAIRN_HAL_WIFI_H

#include <cstdint>

#include "esp_wifi.h"
#include "esp_event.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Wi-Fi scan result for trusted network detection
// -----------------------------------------------------------------------
struct WiFiScanResult {
    char     ssid[33];
    uint8_t  bssid[6];
    int8_t   rssi;
    uint8_t  channel;
    wifi_auth_mode_t authMode;
    bool     isTrusted;           // Matches stored BSSID
};

// -----------------------------------------------------------------------
// Wi-Fi connection state
// -----------------------------------------------------------------------
enum class WiFiState : uint8_t {
    OFF,                // Radio powered off
    IDLE,               // Radio on, not connected
    SCANNING,           // Active scan in progress
    CONNECTING,         // Association/authentication in progress
    CONNECTED,          // Associated and IP obtained
    DISCONNECTING       // Graceful disconnect in progress
};

// -----------------------------------------------------------------------
// HalWiFi -- Wi-Fi management with hardware offload
// -----------------------------------------------------------------------
class HalWiFi {
public:
    // Initialize the Wi-Fi subsystem.
    // Creates the default event loop, initializes the netif layer,
    // and configures the Wi-Fi driver in STA mode.
    // Does NOT power on the radio -- call connect*() or scan*() for that.
    bool init();

    // Scan for a specific BSSID (trusted home network).
    // Performs a targeted scan on the expected channel first (fast path),
    // then falls back to a full scan if not found.
    //   bssid -- 6-byte MAC address of the trusted AP
    //   result -- populated with scan result if found
    //   Returns true if the BSSID was found.
    bool scanForBSSID(const uint8_t bssid[6], WiFiScanResult* result = nullptr);

    // Full passive scan -- returns the number of APs found.
    // Results can be retrieved via getScanResult().
    int16_t scanAll(uint32_t timeout_ms = 5000);

    // Get a specific scan result by index (0-based).
    bool getScanResult(uint16_t index, WiFiScanResult& result);

    // Connect to the trusted network with BSSID lock.
    // BSSID locking ensures we connect to the exact AP, not a
    // neighbor with the same SSID.
    //   ssid  -- network name
    //   psk   -- WPA2/WPA3 pre-shared key
    //   bssid -- 6-byte MAC address to lock to (can be nullptr for any)
    //   timeout_ms -- connection timeout
    //   Returns true if connected and IP obtained.
    bool connectTrusted(const char* ssid, const char* psk,
                        const uint8_t bssid[6],
                        uint32_t timeout_ms = 10000);

    // Disconnect from the current network and idle the radio.
    bool disconnect();

    // Check if we are currently associated and have an IP.
    bool isConnected() const;

    // Get current connection state.
    WiFiState getState() const;

    // Enable modem sleep / DTIM power save.
    // The Wi-Fi radio sleeps between DTIM beacon intervals, reducing
    // current from ~120 mA to ~20 mA.  Slightly increases latency
    // for incoming packets.
    bool enablePowerSave();

    // Disable power save for maximum throughput during uploads.
    bool disablePowerSave();

    // Get the current RSSI (signal strength) in dBm.
    // Only valid when connected.
    int8_t getSignalStrength();

    // Get the current IP address as a string.
    bool getIPAddress(char* buf, size_t bufLen);

    // Completely power off the Wi-Fi radio.
    // Saves ~80 mA.  Must call init() again before reconnecting.
    bool powerOff();

    // Power on the radio (re-initialize if previously powered off).
    bool powerOn();

    // Get the MAC address of this device.
    bool getMACAddress(uint8_t mac[6]);

private:
    WiFiState state_ = WiFiState::OFF;
    bool      initialized_ = false;
    bool      eventLoopStarted_ = false;

    // Event handler for Wi-Fi and IP events.
    static void eventHandler(void* arg, esp_event_base_t eventBase,
                             int32_t eventId, void* eventData);

    // Synchronization: event group bits for connect/disconnect completion.
    // Using FreeRTOS event groups for blocking connect with timeout.
    void* eventGroup_ = nullptr;  // EventGroupHandle_t

    static constexpr int WIFI_CONNECTED_BIT    = (1 << 0);
    static constexpr int WIFI_DISCONNECTED_BIT = (1 << 1);
    static constexpr int WIFI_SCAN_DONE_BIT    = (1 << 2);
};

} // namespace CairnHAL

#endif // CAIRN_HAL_WIFI_H
