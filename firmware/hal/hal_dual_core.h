// ---------------------------------------------------------------------------
// Cairn HAL -- Dual-Core Task Management
//
// The ESP32 has two Xtensa LX6 cores (Core 0 and Core 1).  By default,
// the Arduino framework runs setup() and loop() on Core 1, while
// FreeRTOS system tasks and Wi-Fi/BT run on Core 0.
//
// Cairn's task allocation:
//   Core 0 (PRO_CPU): Wi-Fi sync, NVS writes, TLS, system tasks
//   Core 1 (APP_CPU): Sensor reads (GNSS, IMU), DMA, trip recording
//
// This separation ensures that Wi-Fi operations (which can block for
// tens of milliseconds) never cause missed sensor samples, and sensor
// ISRs never delay Wi-Fi packet handling.
//
// Inter-core communication uses FreeRTOS queues:
//   - Sensor data queue: Core 1 produces samples, Core 0 reads for sync
//   - Command queue: Core 0 sends state transitions, Core 1 adjusts rates
//   - Event queue: either core can post events for logging
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_DUAL_CORE_H
#define CAIRN_HAL_DUAL_CORE_H

#include <cstdint>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/queue.h"
#include "freertos/semphr.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Core assignments
// -----------------------------------------------------------------------
constexpr BaseType_t CORE_SYNC   = 0;   // PRO_CPU: Wi-Fi, sync, NVS
constexpr BaseType_t CORE_SENSOR = 1;   // APP_CPU: Sensors, DMA, recording

// -----------------------------------------------------------------------
// Task priorities (higher = more important)
// -----------------------------------------------------------------------
constexpr UBaseType_t PRIORITY_SENSOR_READ  = 5;   // Highest: sensor ISR/task
constexpr UBaseType_t PRIORITY_STATE_MACHINE = 4;   // State machine updates
constexpr UBaseType_t PRIORITY_RECORDING    = 3;   // SD card writes
constexpr UBaseType_t PRIORITY_SYNC         = 2;   // Wi-Fi sync
constexpr UBaseType_t PRIORITY_HOUSEKEEPING = 1;   // Low-priority maintenance

// -----------------------------------------------------------------------
// Queue message types
// -----------------------------------------------------------------------
enum class QueueMsgType : uint8_t {
    GNSS_SAMPLE,        // GNSSSample ready from sensor task
    IMU_SUMMARY,        // IMUSummary ready from sensor task
    STATE_CHANGE,       // State machine transition command
    SYNC_REQUEST,       // Request sync task to upload
    SYNC_COMPLETE,      // Sync task reports completion
    SHUTDOWN            // Request graceful shutdown
};

struct QueueMessage {
    QueueMsgType type;
    union {
        uint8_t  stateId;         // For STATE_CHANGE
        uint32_t tripCounter;     // For SYNC_REQUEST
        bool     success;         // For SYNC_COMPLETE
    } data;
};

// -----------------------------------------------------------------------
// HalDualCore -- dual-core task and queue management
// -----------------------------------------------------------------------
class HalDualCore {
public:
    // Initialize inter-core communication queues.
    //   sensorQueueDepth  -- max items in sensor data queue
    //   commandQueueDepth -- max items in command queue
    bool init(size_t sensorQueueDepth = 32,
              size_t commandQueueDepth = 8);

    // Create a task pinned to a specific core.
    //   name      -- task name (max 16 chars)
    //   taskFunc  -- FreeRTOS task function
    //   stackSize -- stack in bytes
    //   param     -- parameter passed to task function
    //   priority  -- FreeRTOS priority
    //   core      -- 0 or 1
    //   handle    -- output: task handle (can be NULL)
    bool createPinnedTask(const char* name,
                          TaskFunction_t taskFunc,
                          uint32_t stackSize,
                          void* param,
                          UBaseType_t priority,
                          BaseType_t core,
                          TaskHandle_t* handle = nullptr);

    // -- Queue operations ---------------------------------------------------

    // Send a message to the sensor data queue (from Core 1 -> Core 0).
    bool sendToSensorQueue(const QueueMessage& msg,
                           TickType_t timeout = 0);

    // Receive from sensor data queue (blocking with timeout).
    bool receiveFromSensorQueue(QueueMessage* msg,
                                TickType_t timeout = portMAX_DELAY);

    // Send a command (from Core 0 -> Core 1).
    bool sendCommand(const QueueMessage& msg,
                     TickType_t timeout = 0);

    // Receive a command (from Core 0, blocking with timeout).
    bool receiveCommand(QueueMessage* msg,
                        TickType_t timeout = portMAX_DELAY);

    // -- Mutex for shared state access --------------------------------------

    // Take the state mutex (blocks until available or timeout).
    bool takeMutex(TickType_t timeout = portMAX_DELAY);

    // Release the state mutex.
    void giveMutex();

    // -- Diagnostics --------------------------------------------------------

    // Get free stack high-water mark for a task (in bytes).
    static uint32_t getStackHighWaterMark(TaskHandle_t task);

    // Get the core that the calling task is running on.
    static BaseType_t getCurrentCore();

private:
    QueueHandle_t     sensorQueue_ = nullptr;
    QueueHandle_t     commandQueue_ = nullptr;
    SemaphoreHandle_t stateMutex_ = nullptr;
    bool              initialized_ = false;
};

} // namespace CairnHAL

#endif // CAIRN_HAL_DUAL_CORE_H
