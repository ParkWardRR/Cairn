#ifndef CAIRN_CONFIG_H
#define CAIRN_CONFIG_H

#include <cstdint>
#include "driver/gpio.h"
#include "driver/adc.h"
#include "driver/spi_master.h"

// ===========================================================================
// Pin assignments -- Freematics ONE+ Model B (ESP32)
//
// Pin mappings are based on the Freematics ONE+ Model B schematic.
// Pins marked "VERIFY" need validation against the actual board rev.
// ===========================================================================

// ---------------------------------------------------------------------------
// SDMMC native interface (4-bit parallel)
// The ESP32 SDMMC host uses dedicated pins (no remapping on most revisions).
// ---------------------------------------------------------------------------
constexpr gpio_num_t PIN_SDMMC_CMD = GPIO_NUM_15;
constexpr gpio_num_t PIN_SDMMC_CLK = GPIO_NUM_14;
constexpr gpio_num_t PIN_SDMMC_D0  = GPIO_NUM_2;
constexpr gpio_num_t PIN_SDMMC_D1  = GPIO_NUM_4;    // 4-bit mode only
constexpr gpio_num_t PIN_SDMMC_D2  = GPIO_NUM_12;   // 4-bit mode only
constexpr gpio_num_t PIN_SDMMC_D3  = GPIO_NUM_13;   // 4-bit mode only

// ---------------------------------------------------------------------------
// IMU SPI bus (VSPI / SPI3) -- MPU-6050 or ICM-20948
// Uses DMA-capable SPI peripheral for burst reads.
// ---------------------------------------------------------------------------
constexpr gpio_num_t PIN_IMU_MOSI = GPIO_NUM_23;    // VERIFY on board
constexpr gpio_num_t PIN_IMU_MISO = GPIO_NUM_19;    // VERIFY on board
constexpr gpio_num_t PIN_IMU_SCLK = GPIO_NUM_18;    // VERIFY on board
constexpr gpio_num_t PIN_IMU_CS   = GPIO_NUM_5;     // VERIFY on board
constexpr spi_host_device_t IMU_SPI_HOST = SPI3_HOST;  // VSPI
constexpr int IMU_SPI_CLOCK_HZ = 8000000;           // 8 MHz SPI clock

// ---------------------------------------------------------------------------
// IMU interrupt pin (motion wake via ULP during deep sleep)
// Must be an RTC GPIO for ULP access during deep sleep.
// RTC GPIOs on ESP32: 0, 2, 4, 12-15, 25-27, 32-39
// ---------------------------------------------------------------------------
constexpr gpio_num_t PIN_IMU_INT = GPIO_NUM_34;      // VERIFY -- RTC GPIO

// ---------------------------------------------------------------------------
// GNSS UART (UART2) -- Freematics GNSS receiver
// The Freematics ONE+ uses UART2 for its internal GNSS module.
// ---------------------------------------------------------------------------
constexpr gpio_num_t PIN_GNSS_TX = GPIO_NUM_17;      // ESP32 TX -> GNSS RX
constexpr gpio_num_t PIN_GNSS_RX = GPIO_NUM_16;      // ESP32 RX <- GNSS TX
constexpr int GNSS_UART_NUM       = 2;               // UART2
constexpr int GNSS_BAUD_RATE      = 115200;

// ---------------------------------------------------------------------------
// Battery voltage ADC
// Connected through a resistor divider on the Freematics board.
// ADC1 channels are safe to use alongside Wi-Fi (ADC2 conflicts).
// ---------------------------------------------------------------------------
constexpr adc1_channel_t BATTERY_ADC_CHANNEL = ADC1_CHANNEL_0;  // GPIO 36
constexpr gpio_num_t PIN_BATTERY_ADC = GPIO_NUM_36;              // VERIFY

// ===========================================================================
// GNSS sampling rates (Hz) per device state
// ===========================================================================
constexpr uint8_t GNSS_RATE_ACTIVE_HZ      = 5;   // Active driving
constexpr uint8_t GNSS_RATE_SLOW_HZ        = 2;   // Slow maneuvering
constexpr uint8_t GNSS_RATE_STATIONARY_HZ  = 1;   // Stationary
constexpr uint8_t GNSS_RATE_PARKED_HZ      = 0;   // Parked (off)

// ===========================================================================
// IMU sampling rates (Hz) per device state
// ===========================================================================
constexpr uint8_t IMU_RATE_ACTIVE_HZ       = 50;
constexpr uint8_t IMU_RATE_SLOW_HZ         = 25;
constexpr uint8_t IMU_RATE_STATIONARY_HZ   = 10;
constexpr uint8_t IMU_RATE_PARKED_HZ       = 0;

// ===========================================================================
// Wi-Fi credentials (placeholder -- real values stored in encrypted NVS)
// ===========================================================================
constexpr char WIFI_SSID[33]  = "YOUR_SSID_HERE";
constexpr char WIFI_BSSID[18] = "00:00:00:00:00:00";

// ===========================================================================
// Sync server
// ===========================================================================
constexpr char SERVER_HOSTNAME[64] = "cairn.local";
constexpr uint16_t SERVER_PORT     = 8443;

// ===========================================================================
// Debounce / state-machine thresholds
// ===========================================================================
constexpr uint32_t ARMING_DURATION_MS  = 15000;          // 15 s
constexpr uint32_t STOP_DWELL_MS       = 300000;         // 5 min
constexpr uint16_t MIN_SPEED_KMH       = 5;

// ===========================================================================
// Storage limits
// ===========================================================================
constexpr uint16_t MIN_FREE_SD_MB      = 50;
constexpr uint32_t MAX_TRIP_DURATION_MS = 14400000;       // 4 hours

// ===========================================================================
// Power
// ===========================================================================
constexpr uint16_t LOW_BATTERY_THRESHOLD_MV = 11500;      // 11.5 V
constexpr float    BROWNOUT_THRESHOLD_V     = 2.7f;       // At divider output

// ===========================================================================
// CPU frequency settings per power mode
// ===========================================================================
constexpr uint32_t CPU_FREQ_PERFORMANCE_MHZ = 240;
constexpr uint32_t CPU_FREQ_BALANCED_MHZ    = 160;
constexpr uint32_t CPU_FREQ_LOW_POWER_MHZ   = 80;

// ===========================================================================
// ULP coprocessor settings
// ===========================================================================
constexpr uint32_t ULP_CHECK_INTERVAL_MS    = 1000;       // Poll IMU every 1 s
constexpr uint16_t ULP_MOTION_THRESHOLD_MG  = 300;        // 0.3 g to trigger wake

// ===========================================================================
// DMA buffer sizes
// ===========================================================================
constexpr size_t DMA_IMU_BURST_BYTES   = 14;   // 1 reg addr + 6 axes * 2 bytes + status
constexpr size_t DMA_SPI_MAX_TRANSFER  = 64;   // Max single DMA transfer

// ===========================================================================
// GNSS quality gates
// ===========================================================================
constexpr uint16_t MAX_HDOP_TENTHS     = 50;              // HDOP 5.0
constexpr uint8_t  MIN_SATELLITES      = 4;

// ===========================================================================
// Connectivity
// ===========================================================================
constexpr uint32_t WIFI_SCAN_INTERVAL_MS = 60000;         // 1 min
constexpr uint16_t UPLOAD_CHUNK_SIZE     = 4096;

// ===========================================================================
// Data retention
// ===========================================================================
constexpr uint16_t RETENTION_WINDOW_HOURS = 168;          // 7 days

// ===========================================================================
// FreeRTOS task configuration
// ===========================================================================
constexpr uint32_t TASK_STACK_SENSOR   = 4096;
constexpr uint32_t TASK_STACK_SYNC     = 8192;   // TLS needs more stack
constexpr uint32_t TASK_STACK_STATE    = 4096;

// ===========================================================================
// SDMMC mount point
// ===========================================================================
constexpr const char* SDMMC_MOUNT_POINT = "/sdcard";

#endif // CAIRN_CONFIG_H
