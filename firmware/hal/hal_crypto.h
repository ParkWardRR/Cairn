// ---------------------------------------------------------------------------
// Cairn HAL -- Hardware Crypto Acceleration
//
// The ESP32 contains dedicated hardware accelerators for SHA-256 and AES.
// ESP-IDF compiles mbedtls with CONFIG_MBEDTLS_HARDWARE_SHA and
// CONFIG_MBEDTLS_HARDWARE_AES enabled by default, so every call to
// mbedtls_sha256() / mbedtls_aes_crypt_cbc() is transparently routed to
// the hardware engines -- no software fallback unless explicitly disabled.
//
// The ESP32 also has a true hardware RNG fed by thermal noise in the
// RF subsystem (Wi-Fi / BT must have been active at least once after
// boot for full entropy; otherwise the TRNG still functions but with
// reduced entropy).
//
// Ed25519 is implemented via mbedtls_pk / mbedtls_ecdsa using Curve25519
// operations.  On ESP32 the big-number multiplier hardware accelerates
// the underlying modular arithmetic.
// ---------------------------------------------------------------------------

#ifndef CAIRN_HAL_CRYPTO_H
#define CAIRN_HAL_CRYPTO_H

#include <cstdint>
#include <cstddef>

// ESP-IDF / mbedtls headers -- on ESP32 these route to hardware engines
#include "mbedtls/sha256.h"
#include "mbedtls/aes.h"
#include "mbedtls/pk.h"
#include "mbedtls/entropy.h"
#include "mbedtls/ctr_drbg.h"
#include "esp_system.h"           // esp_random(), esp_fill_random()

namespace CairnHAL {

// -----------------------------------------------------------------------
// Ed25519 key pair (stored in NVS / eFuse)
// -----------------------------------------------------------------------
struct Ed25519KeyPair {
    uint8_t publicKey[32];
    uint8_t privateKey[64];       // Ed25519 expanded private key
    bool    valid;
};

// -----------------------------------------------------------------------
// HalCrypto -- hardware-accelerated cryptographic operations
// -----------------------------------------------------------------------
class HalCrypto {
public:
    // -- One-shot SHA-256 ------------------------------------------------
    // Uses the ESP32 hardware SHA engine via mbedtls.  The hardware engine
    // processes 64-byte blocks directly in silicon, yielding ~2x throughput
    // versus the software path.
    static bool sha256(const uint8_t* data, size_t len, uint8_t out[32]);

    // -- Streaming SHA-256 for large payloads ----------------------------
    // Suitable for hashing multi-megabyte SD card bundles without loading
    // the entire file into RAM.  Each update() feeds a chunk to the
    // hardware engine; finish() produces the final digest.
    bool sha256_start();
    bool sha256_update(const uint8_t* data, size_t len);
    bool sha256_finish(uint8_t out[32]);

    // -- AES-256-CBC encryption / decryption -----------------------------
    // Uses the ESP32 hardware AES engine.  `iv` is modified in-place per
    // the CBC spec; callers should keep a copy if the IV is needed later.
    // `len` MUST be a multiple of 16 (AES block size).
    static bool aes256_encrypt(const uint8_t key[32],
                               uint8_t iv[16],
                               const uint8_t* plaintext,
                               size_t len,
                               uint8_t* ciphertext);

    static bool aes256_decrypt(const uint8_t key[32],
                               uint8_t iv[16],
                               const uint8_t* ciphertext,
                               size_t len,
                               uint8_t* plaintext);

    // -- Hardware RNG ----------------------------------------------------
    // esp_random() reads from the ESP32 true-hardware RNG register.
    // esp_fill_random() fills an arbitrary-length buffer.
    static void generate_random(uint8_t* buf, size_t len);

    // -- Ed25519 key management ------------------------------------------
    // Generate a new device identity key pair.  The private key should be
    // stored in NVS (encrypted partition) immediately after generation.
    static bool generate_ed25519_keypair(Ed25519KeyPair& kp);

    // Sign a hash/data buffer with the device private key.
    // `signature` must be at least 64 bytes.
    static bool sign_ed25519(const uint8_t privateKey[64],
                             const uint8_t* data,
                             size_t len,
                             uint8_t signature[64]);

    // Verify a signature from the sync server using its public key.
    static bool verify_ed25519(const uint8_t publicKey[32],
                               const uint8_t* data,
                               size_t len,
                               const uint8_t signature[64]);

private:
    // Streaming SHA-256 context (one per instance)
    mbedtls_sha256_context sha256_ctx_;
    bool                   sha256_active_ = false;
};

} // namespace CairnHAL

#endif // CAIRN_HAL_CRYPTO_H
