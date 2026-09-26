// ---------------------------------------------------------------------------
// Cairn HAL -- Unified Hardware Abstraction Layer
//
// Single include for all ESP32 hardware acceleration modules.
// Call CairnHAL::init() once at boot to initialize all subsystems,
// then use individual module classes (HalCrypto, HalSDMMC, etc.).
//
// The init order matters:
//   1. NVS      -- needed by everything that reads config
//   2. Power    -- set initial CPU frequency, read reset reason
//   3. DMA/SPI  -- sensor bus initialization
//   4. SDMMC    -- mount SD card
//   5. Timer    -- hardware timer allocation
//   6. ULP      -- only if waking from deep sleep needs ULP data
//   7. WiFi     -- initialized but not connected (on-demand)
//   8. DualCore -- create FreeRTOS tasks last
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_H
#define CAIRN_HAL_H

// All HAL modules
#include "hal_crypto.h"
#include "hal_ulp.h"
#include "hal_dma.h"
#include "hal_sdmmc.h"
#include "hal_power.h"
#include "hal_timer.h"
#include "hal_wifi.h"
#include "hal_nvs.h"
#include "hal_dual_core.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// Global HAL instances (singleton-like, one per module)
// -----------------------------------------------------------------------
extern HalCrypto   crypto;
extern HalULP      ulp;
extern HalDMA      dma;
extern HalSDMMC    sdmmc;
extern HalPower    power;
extern HalTimer    timer;
extern HalWiFi     wifi;
extern HalNVS      nvs;
extern HalDualCore dualCore;

// -----------------------------------------------------------------------
// Unified initialization -- call once from setup() / app_main()
// -----------------------------------------------------------------------
// Initializes all HAL subsystems in the correct order.
// Returns true if all critical subsystems initialized successfully.
// Non-critical failures (e.g., ULP on power-on boot) are logged
// but do not cause init() to return false.
bool init();

// -----------------------------------------------------------------------
// Print detected hardware capabilities to Serial
// -----------------------------------------------------------------------
// Logs:
//   - ESP32 chip revision, flash size, PSRAM
//   - Reset reason and wake cause
//   - SD card info (bus width, speed, capacity)
//   - ADC calibration status
//   - Crypto engine availability
//   - Core count and frequencies
void printCapabilities();

} // namespace CairnHAL

#endif // CAIRN_HAL_H
