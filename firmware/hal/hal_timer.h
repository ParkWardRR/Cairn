// ---------------------------------------------------------------------------
// Cairn HAL -- Hardware Timers
//
// The ESP32 has 4 hardware 64-bit general-purpose timers organized in
// 2 groups of 2 timers each (Timer Group 0 and Timer Group 1).
// Each timer can:
//   - Count up or down at a configurable prescaler divider
//   - Generate an interrupt on alarm match
//   - Auto-reload on alarm
//
// Using hardware timers instead of millis()-based polling provides:
//   - Exact microsecond-precision sample timing
//   - Deterministic interrupt-driven reads (no jitter from loop() delays)
//   - Independent of FreeRTOS tick rate
//
// Timer allocation for Cairn:
//   Timer 0 (Group 0, Timer 0): GNSS sample trigger (1-10 Hz)
//   Timer 1 (Group 0, Timer 1): IMU sample trigger (10-100 Hz)
//   Timer 2 (Group 1, Timer 0): State machine watchdog
//   Timer 3 (Group 1, Timer 1): Reserved / high-res timestamp
//
// ESP-IDF 5.x introduces the gptimer API which supersedes the legacy
// timer_group API.  We use gptimer where available with a fallback.
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_TIMER_H
#define CAIRN_HAL_TIMER_H

#include <cstdint>
#include "driver/gptimer.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Timer callback type
// -----------------------------------------------------------------------
typedef void (*TimerCallback)();

// -----------------------------------------------------------------------
// Timer identifiers
// -----------------------------------------------------------------------
enum class HardwareTimer : uint8_t {
    GNSS_SAMPLE   = 0,   // Timer Group 0, Timer 0
    IMU_SAMPLE    = 1,   // Timer Group 0, Timer 1
    WATCHDOG      = 2,   // Timer Group 1, Timer 0
    HIGH_RES      = 3    // Timer Group 1, Timer 1
};

// -----------------------------------------------------------------------
// HalTimer -- hardware timer management
// -----------------------------------------------------------------------
class HalTimer {
public:
    // Initialize the GNSS sample timer.
    // Fires `callback` at exactly `rate_hz` times per second using
    // hardware Timer Group 0, Timer 0.  The callback runs in ISR
    // context -- it must be short (set a flag, give a semaphore, etc.).
    //   rate_hz  -- sample rate (1-10 Hz for GNSS)
    //   callback -- ISR-safe function pointer
    bool initGNSSTimer(uint8_t rate_hz, TimerCallback callback);

    // Initialize the IMU sample timer.
    // Uses Timer Group 0, Timer 1.
    //   rate_hz  -- sample rate (10-100 Hz for IMU)
    //   callback -- ISR-safe function pointer
    bool initIMUTimer(uint8_t rate_hz, TimerCallback callback);

    // Initialize a hardware watchdog timer.
    // Uses Timer Group 1, Timer 0.  If the watchdog is not reset
    // within timeout_ms, the callback fires.  The callback should
    // log the fault and initiate recovery.
    //   timeout_ms -- watchdog timeout in milliseconds
    bool initWatchdog(uint32_t timeout_ms);

    // Reset (feed) the watchdog timer.  Must be called periodically
    // from the main state machine loop.
    bool feedWatchdog();

    // Dynamically change the GNSS sample rate without stopping the timer.
    // Recalculates the alarm period and updates the hardware register.
    bool setGNSSRate(uint8_t rate_hz);

    // Dynamically change the IMU sample rate without stopping.
    bool setIMURate(uint8_t rate_hz);

    // Stop a specific timer.
    bool stopTimer(HardwareTimer timer);

    // Start a previously stopped timer.
    bool startTimer(HardwareTimer timer);

    // Get a high-resolution microsecond timestamp from hardware counter.
    // Uses Timer Group 1, Timer 1 configured as a free-running counter
    // at 1 MHz (1 us resolution).  Much more precise than millis().
    //
    // Must call initHighResCounter() first.
    bool initHighResCounter();
    uint64_t getHighResTimestamp();

    // Deinitialize all timers.
    void deinitAll();

private:
    // ESP-IDF 5.x gptimer handles
    gptimer_handle_t timers_[4] = {nullptr, nullptr, nullptr, nullptr};
    bool             active_[4] = {false, false, false, false};

    // Internal: create and configure a gptimer with the given alarm period.
    bool configureTimer(HardwareTimer id,
                        uint64_t alarmPeriodUs,
                        gptimer_alarm_cb_t isrCallback,
                        void* userData,
                        bool autoReload);

    // Adapter struct to bridge TimerCallback to gptimer ISR signature
    struct CallbackAdapter {
        TimerCallback userCallback;
    };
    CallbackAdapter adapters_[4] = {};
};

} // namespace CairnHAL

#endif // CAIRN_HAL_TIMER_H
