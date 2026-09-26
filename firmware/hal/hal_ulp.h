// ---------------------------------------------------------------------------
// Cairn HAL -- ULP Coprocessor for Motion Wake
//
// The ESP32 contains an Ultra Low Power (ULP) coprocessor that runs at
// ~8 MHz with access to ~8 KB of RTC slow memory.  It continues to
// operate during deep sleep while the main Xtensa cores are powered off.
//
// During parked state, instead of keeping the main CPU in light sleep
// polling the IMU (~10 mA), we load a small ULP program that
// periodically reads the IMU interrupt/status register over RTC I2C.
// If the motion magnitude exceeds a configured threshold the ULP sets
// a wake flag and halts, which triggers the main CPU to wake from deep
// sleep via esp_sleep_enable_ulp_wakeup().
//
// This drops parked-state power from ~10 mA to ~150 uA (ULP + RTC only).
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_ULP_H
#define CAIRN_HAL_ULP_H

#include <cstdint>
#include "driver/gpio.h"

// ESP-IDF ULP support
#include "esp32/ulp.h"
#include "driver/rtc_io.h"
#include "soc/rtc_cntl_reg.h"
#include "esp_sleep.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Wake reason codes -- returned after the main CPU resumes from deep sleep
// -----------------------------------------------------------------------
enum class WakeReason : uint8_t {
    POWER_ON,           // First boot / power cycle
    ULP_MOTION,         // ULP detected motion above threshold
    TIMER,              // RTC timer expired (periodic Wi-Fi scan)
    EXTERNAL_INT,       // External interrupt pin (e.g. button)
    TOUCHPAD,           // Touchpad wake (unused in Cairn)
    UNKNOWN
};

// -----------------------------------------------------------------------
// RTC slow memory layout for ULP <-> main CPU shared data
// Stored in RTC_SLOW_ATTR variables that survive deep sleep.
// -----------------------------------------------------------------------
struct ULPSharedData {
    uint16_t motionMagnitude;     // Last motion reading that triggered wake
    uint16_t threshold;           // Current threshold (set by main CPU)
    uint16_t wakeFlag;            // Non-zero if ULP triggered wake
    uint16_t checkCount;          // Number of checks since ULP started
};

// -----------------------------------------------------------------------
// HalULP -- ULP coprocessor management for ultra-low-power motion wake
// -----------------------------------------------------------------------
class HalULP {
public:
    // Initialize ULP subsystem and configure the IMU interrupt pin as
    // an RTC GPIO (required for ULP I2C / GPIO access during deep sleep).
    bool init(gpio_num_t imuInterruptPin);

    // Load the ULP motion-wake program into RTC slow memory.
    //   threshold_mg   -- motion magnitude threshold in milli-g
    //   check_interval_ms -- how often the ULP polls the IMU (typ. 500-2000 ms)
    //
    // The ULP program loop:
    //   1. Wait for check_interval_ms using ULP timer
    //   2. Read IMU status/accel register over RTC I2C
    //   3. Compare magnitude against threshold in RTC slow memory
    //   4. If exceeded: set wake flag, halt (triggers main CPU wake)
    //   5. If not exceeded: increment check counter, loop to step 1
    bool loadMotionWakeProgram(uint16_t threshold_mg,
                               uint32_t check_interval_ms);

    // Enable ULP wakeup source and start the ULP program.
    // After calling this, the caller should enter deep sleep via
    // HalPower::enterDeepSleep().  The main CPU will wake when the
    // ULP detects motion or a timer expires.
    bool startMonitoring();

    // After waking from deep sleep, determine why we woke up.
    static WakeReason getWakeReason();

    // Read the motion magnitude that triggered the ULP wake.
    // Only valid when getWakeReason() == ULP_MOTION.
    // Returns the value in milli-g as stored by the ULP program.
    static uint16_t getMotionMagnitude();

    // Read the number of ULP check cycles that elapsed before wake.
    static uint16_t getCheckCount();

    // Update the threshold without reloading the entire ULP program.
    // Writes directly to the RTC slow memory variable.
    void setThreshold(uint16_t threshold_mg);

private:
    gpio_num_t imuPin_ = GPIO_NUM_MAX;
    bool       programLoaded_ = false;
    bool       initialized_ = false;

    // Build the ULP assembly program into the instruction buffer.
    bool assembleMotionProgram(uint16_t threshold_mg,
                               uint32_t check_interval_us);
};

} // namespace CairnHAL

#endif // CAIRN_HAL_ULP_H
