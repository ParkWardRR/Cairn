// ---------------------------------------------------------------------------
// Cairn HAL -- Power Management
//
// The ESP32 provides extensive hardware power management:
//
//   - Dynamic Frequency Scaling (DFS): CPU frequency can be changed at
//     runtime between 240 MHz (max performance), 160 MHz, 80 MHz, and
//     down to 10 MHz.  esp_pm_configure() enables automatic DFS based
//     on FreeRTOS tick activity and power-management locks.
//
//   - Light sleep: CPU halted, peripherals optionally active, RAM
//     retained.  Wake sources: timer, GPIO, UART, touch.  Entry/exit
//     latency ~1 ms.  Current draw ~0.8 mA.
//
//   - Deep sleep: main CPU and most peripherals powered off.  Only RTC
//     controller, RTC memory (8 KB), and ULP coprocessor remain active.
//     Wake sources: RTC timer, external GPIO (ext0/ext1), ULP, touch.
//     Current draw ~10 uA (without ULP) or ~150 uA (with ULP running).
//     All RAM contents lost except RTC slow memory.
//
//   - Brown-out detector: hardware comparator monitors VDD.  If voltage
//     drops below the configured threshold, an interrupt fires (or the
//     chip resets) before data corruption can occur.
//
//   - ADC with calibration: ESP32's ADC1 with factory-calibrated Vref
//     and configurable attenuation for accurate battery voltage reading.
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_POWER_H
#define CAIRN_HAL_POWER_H

#include <cstdint>

#include "esp_pm.h"
#include "esp_sleep.h"
#include "esp_system.h"

// ADC headers
#ifdef CONFIG_IDF_TARGET_ESP32
#include "driver/adc.h"
#include "esp_adc_cal.h"
#endif

namespace CairnHAL {

// -----------------------------------------------------------------------
// Reset reason codes
// -----------------------------------------------------------------------
enum class ResetReason : uint8_t {
    POWER_ON,           // Normal power-on or EN pin reset
    SOFTWARE,           // esp_restart() called
    DEEP_SLEEP_WAKE,    // Woke from deep sleep
    BROWNOUT,           // Brown-out detector triggered reset
    WATCHDOG_TASK,      // Task watchdog timer fired
    WATCHDOG_INT,       // Interrupt watchdog timer fired
    WATCHDOG_RTC,       // RTC watchdog fired
    PANIC,              // Software panic / unhandled exception
    SDIO,               // SDIO reset
    UNKNOWN
};

// -----------------------------------------------------------------------
// Performance mode presets
// -----------------------------------------------------------------------
enum class PerformanceMode : uint8_t {
    HIGH,       // 240 MHz, all peripherals active -- trip recording
    BALANCED,   // 160 MHz, adaptive frequency scaling
    LOW_POWER,  // 80 MHz, reduced peripherals -- idle/queued states
    ULTRA_LOW   // Deep sleep with ULP -- parked state
};

// -----------------------------------------------------------------------
// HalPower -- hardware power management
// -----------------------------------------------------------------------
class HalPower {
public:
    // Initialize power management subsystem:
    //   - Configure dynamic frequency scaling (DFS)
    //   - Set up brown-out detector
    //   - Calibrate ADC for battery voltage reads
    //   - Log the reset reason
    bool init();

    // -- Performance mode presets -----------------------------------------

    // 240 MHz, all peripherals active.
    // Use during active trip recording for maximum sensor throughput.
    bool setPerformanceMode();

    // 160 MHz with automatic DFS.
    // CPU scales between 80-160 MHz based on load.
    bool setBalancedMode();

    // 80 MHz, DFS enabled, unused peripherals disabled.
    // Use during QUEUED_FOR_HOME_SYNC and RETAINED states.
    bool setLowPowerMode();

    // -- Sleep modes -----------------------------------------------------

    // Enter light sleep for a fixed duration.
    // CPU halts but RAM is retained and peripherals can stay active.
    // Returns after wakeup_ms milliseconds (or earlier if GPIO wake).
    // Typical current: ~0.8 mA.
    bool enterLightSleep(uint32_t wakeup_ms);

    // Enter deep sleep.  Main CPU and RAM are powered off.
    // Only RTC memory (8 KB) survives.  On wake, the chip reboots
    // through setup() -- the caller must save state to RTC memory or
    // NVS before calling this.
    //   wakeup_us -- RTC timer wake after this many microseconds.
    //                Set to 0 to disable timer wake (ULP-only wake).
    // Typical current: ~10 uA (no ULP) or ~150 uA (with ULP).
    void enterDeepSleep(uint64_t wakeup_us);

    // -- Battery voltage -------------------------------------------------

    // Read battery voltage using ADC1 hardware with factory calibration.
    // The ESP32 ADC reads the voltage through a resistor divider on the
    // Freematics board.  The attenuation is set to ADC_ATTEN_DB_11 for
    // the full 0-3.3V input range, then scaled by the divider ratio.
    //   Returns voltage in volts (e.g. 12.6 for a healthy car battery).
    float readBatteryVoltageHW();

    // -- Brown-out detection ---------------------------------------------

    // Enable the hardware brown-out detector.
    // The ESP32 comparator monitors VDD33 and triggers a reset if the
    // voltage drops below the threshold.  This prevents flash corruption
    // during sudden power loss.
    //   threshold_v -- approximate threshold (quantized to nearest level)
    //                  Valid range: ~2.4V to ~3.0V.
    bool enableBrownoutDetection(float threshold_v);

    // -- Reset / wake information ----------------------------------------

    // Get the reason for the most recent reset/reboot.
    static ResetReason getResetReason();

    // Get the deep-sleep wake cause (timer, ext0, ext1, ULP, touch).
    static esp_sleep_wakeup_cause_t getWakeupCause();

    // -- Peripheral power control ----------------------------------------

    // Disable peripherals not needed in the current state.
    // Powers off: Wi-Fi, BT, unused GPIO, ADC2 (shared with Wi-Fi).
    void disableUnusedPeripherals();

    // Re-enable all peripherals (call before entering RECORDING state).
    void enableAllPeripherals();

    // Explicitly power off/on the Wi-Fi radio.
    void powerOffWiFi();
    void powerOnWiFi();

    // Explicitly power off/on Bluetooth.
    void powerOffBluetooth();
    void powerOnBluetooth();

private:
    PerformanceMode currentMode_ = PerformanceMode::BALANCED;
    bool            pmConfigured_ = false;

#ifdef CONFIG_IDF_TARGET_ESP32
    esp_adc_cal_characteristics_t adcChars_;
    bool                          adcCalibrated_ = false;
#endif

    // Battery voltage divider ratio for Freematics ONE+ board.
    // The board uses a resistor divider to scale 12V battery to 0-3.3V
    // ADC input range.  Ratio = (R1 + R2) / R2.
    // Verify against actual board schematic and calibrate if needed.
    static constexpr float BATTERY_DIVIDER_RATIO = 5.7f;

    // ADC channel for battery voltage (board-specific).
    static constexpr adc1_channel_t BATTERY_ADC_CHANNEL = ADC1_CHANNEL_0;
};

} // namespace CairnHAL

#endif // CAIRN_HAL_POWER_H
