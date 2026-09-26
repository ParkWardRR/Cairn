// ---------------------------------------------------------------------------
// Cairn HAL -- DMA for Sensor Data
//
// The ESP32 supports DMA on SPI, I2S, I2C, and UART peripherals.
// Using DMA for sensor reads frees the CPU from byte-by-byte transfers:
//
//   - SPI-DMA burst reads from the IMU: instead of 12 individual byte
//     reads per sample at 50 Hz, a single DMA transaction transfers an
//     entire register block in one shot while the CPU does other work.
//
//   - SDMMC DMA writes: the native SDMMC host controller uses DMA
//     internally (not exposed here -- see hal_sdmmc), but SPI-mode SD
//     access can also be DMA-accelerated via this module.
//
// The ESP32 has two DMA-capable SPI peripherals: HSPI (SPI2) and VSPI
// (SPI3).  SPI1 is reserved for flash.  Each can be configured with a
// dedicated DMA channel (or use the auto-allocate API in ESP-IDF 5.x).
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_DMA_H
#define CAIRN_HAL_DMA_H

#include <cstdint>
#include <cstddef>

#include "driver/spi_master.h"

namespace CairnHAL {

// -----------------------------------------------------------------------
// DMA transfer result
// -----------------------------------------------------------------------
struct DMATransferResult {
    bool     success;
    size_t   bytesTransferred;
    uint32_t durationUs;        // Measured transfer time in microseconds
};

// -----------------------------------------------------------------------
// HalDMA -- DMA-accelerated peripheral access
// -----------------------------------------------------------------------
class HalDMA {
public:
    // Initialize an SPI bus with DMA support.
    //   host  -- HSPI_HOST or VSPI_HOST (SPI2 or SPI3)
    //   mosi, miso, sclk -- GPIO pin numbers
    //   cs    -- chip select GPIO
    //   clockHz -- SPI clock frequency (IMU typically 1-10 MHz)
    //
    // Internally allocates a DMA channel via spi_bus_initialize() with
    // dma_chan set to SPI_DMA_CH_AUTO (ESP-IDF 5.x) or explicit channel.
    // The DMA engine handles all byte transfers to/from the SPI FIFO,
    // freeing the CPU for other tasks during the transaction.
    bool initSPIDMA(spi_host_device_t host,
                    int mosi, int miso, int sclk, int cs,
                    int clockHz = 8000000);

    // Perform a DMA-based SPI burst read from the IMU.
    //   startReg -- first register address (with read bit set)
    //   buf      -- destination buffer (must be DMA-capable: 32-bit aligned,
    //               in internal RAM, not on stack for large transfers)
    //   len      -- number of bytes to read
    //
    // The SPI master sends the register address, then clocks in `len` bytes
    // via DMA.  For a 6-axis IMU reading 12 bytes (3x accel + 3x gyro),
    // this completes in a single DMA transaction versus 12 individual reads.
    DMATransferResult readIMUBurst(uint8_t startReg, uint8_t* buf, size_t len);

    // Perform a DMA-based SPI write.
    //   startReg -- first register address (with write bit set)
    //   data     -- source buffer
    //   len      -- number of bytes to write
    DMATransferResult writeSPIBurst(uint8_t startReg, const uint8_t* data,
                                   size_t len);

    // Read a single register (non-DMA, for configuration).
    bool readRegister(uint8_t reg, uint8_t* value);

    // Write a single register (non-DMA, for configuration).
    bool writeRegister(uint8_t reg, uint8_t value);

    // Check if the DMA SPI bus is initialized and ready.
    bool isReady() const;

    // Release the SPI bus and DMA channel.
    void deinit();

private:
    spi_device_handle_t spiDevice_ = nullptr;
    spi_host_device_t   spiHost_   = SPI2_HOST;
    bool                initialized_ = false;

    // DMA-capable transaction buffer (must be in DMA-accessible memory).
    // For small IMU reads this avoids heap allocation per transaction.
    static constexpr size_t TX_BUF_SIZE = 64;
    uint8_t txBuf_[TX_BUF_SIZE] __attribute__((aligned(4)));
    uint8_t rxBuf_[TX_BUF_SIZE] __attribute__((aligned(4)));
};

} // namespace CairnHAL

#endif // CAIRN_HAL_DMA_H
