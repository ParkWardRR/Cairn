# Device enrolment: wire format (enrolment v1)

The bytes and the line protocol by which a Cairn dongle hands its server-side
identity to a server over a USB console: the sealed enrolment blob, its
fingerprint, and the console commands. Normative for the firmware
(`lib/cairn_prov`) and the server (`internal/enroll`, `cairn-provision`).
Vectors: [`vectors/vectors.json`](vectors/vectors.json), which both implementations
reproduce byte for byte.

How an operator uses this (what gets provisioned, the procedure, the residual risk,
troubleshooting) is not part of the contract: it is in the server's
`docs/device-provisioning.md`.

**Status:** implemented by the firmware and the server and exercised on real hardware.
No network credential travels over this channel: the dongle has none.

## The sealed enrolment blob

Printed by the device on request (`GET enroll_blob`). 225 bytes, little-endian,
shown as base64:

| Offset | Size | Field |
|---:|---:|---|
| 0 | 4 | magic `CENR` |
| 4 | 1 | version = 1 |
| 5 | 16 | `device_id` |
| 21 | 32 | device Ed25519 public key |
| 53 | 4 | `storage_key_version` |
| 57 | 32 | ephemeral X25519 public key |
| 89 | 24 | XChaCha20-Poly1305 nonce |
| 113 | 32 | ciphertext of `K_root` |
| 145 | 16 | Poly1305 tag |
| 161 | 64 | Ed25519 signature by the device key over bytes `[0, 161)` |

```text
shared = X25519(eph_priv, server_enrol_pub)
key    = HKDF-SHA256(ikm = shared, salt = eph_pub ‖ server_enrol_pub,
                     info = "cairn/enroll-seal/v1", L = 32)
ct‖tag = XChaCha20-Poly1305(key, nonce, K_root, aad = bytes [0, 113))
```

- The **AAD** covers every header field, so identity, key and key version cannot
  be edited without failing the tag.
- The **signature** is proof of possession: whoever produced the blob holds the
  device's signing key, so a blob cannot enrol someone else's public key.
- The ephemeral key and nonce come from the hardware RNG. The device **refuses**
  to produce a blob while the pinned server key is the all-zero placeholder:
  sealing to a key nobody holds would "succeed" and lose the root.
- The server's enrolment private key is wrapped under the keystore master key
  (`<data>/keys/enroll.x25519`). A blob is not secret (it crosses a console, ssh
  and logs), so the key that opens it must be protected exactly like the roots it
  unlocks.
- The C implementation and the Go reference produce **byte-identical** blobs from
  the same inputs; `fixtures/enroll-v1/vectors.json` pins that and both test
  suites check it.

### The fingerprint

`FINGERPRINT` is the first 4 bytes of SHA-256(device public key), 8 hex
characters: the same value as the first half of the device key id. It is a
**mix-up check**, "the blob I am approving came from the unit on my bench", not
authentication of the channel. `cairn-admin device enroll` **requires**
`--confirm-fingerprint`; there is no approve-whatever-arrived mode. For the check
to mean anything the value should come from somewhere other than the console it
guards: a record made when the unit was first enrolled, or the label on a unit
you provisioned yourself. `cairn-provision` accepts `--expect-device-id` and
`--expect-fingerprint` for exactly that, so a non-interactive run is still a
checked one.

## The console protocol

Line-oriented ASCII at 115200 baud. Host commands, device replies prefixed
`@prov ` (the console also carries the device's log, so a bare `OK` would be
ambiguous):

| Host | Device |
|---|---|
| `CAIRN-PROV BEGIN` | `@prov PROV-READY <device_id hex> <fingerprint>` or `@prov ERR refused: …` |
| `GET enroll_blob` | `@prov ENROLL-BLOB <base64>` or `@prov ERR …` |
| `SET assignment <vehicle32hex> <assignment32hex>` | `OK` (both non-zero) |
| `SET counter_floor <n>` | `OK` (decimal u64; only raises) |
| `COMMIT` | `OK`, or `ERR <reason>` (staged values kept; re-send) |
| `CAIRN-PROV END` | `OK` |

`SET wifi_ssid`, `SET wifi_pass`, `SET client_cert` and `SET client_key` no longer
exist and are answered `ERR unknown field`. A host script written for the old
firmware therefore **cannot** put a private key on a current dongle, which is the
point; a host test asserts it.

A successful `COMMIT` closes the session; the device stays silent about lines
sent outside one.

**Rules.**

- **Never during a trip**: whenever the controller is past Idle.
- An **unassigned** device accepts a session at any time (nothing to protect).
- An **assigned** device accepts one only in the first **60 s after boot**, so
  an unattended unit cannot be re-provisioned by someone who plugs in later.
  `cairn-provision --reset` pulses the board's reset line and then begins.
- A session idles out after **30 s** and its staged values are scrubbed.
- The assignment and the counter floor are idempotent and the floor only rises, so
  a `COMMIT` that reports failure is safe to simply send again.
- **No secret is ever echoed or logged.** The log records events and field
  *names* only. A host test sends a canary through every path and asserts it
  appears in no reply and no log line, raw or base64.
