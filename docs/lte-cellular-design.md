# LTE and cellular connectivity design

> **Design document, not shipped.** Nothing in this document is implemented in
> committed firmware. The LTE hardware is confirmed present on the car's dongle
> (SIMCOM SIM7600A-H, 2026-10-07), but no radio transport, credential store or
> scheduling logic has been wired into the device. This document synthesises
> research on the project's actual hardware, the EIOTCLUB IoT SIM, and the
> existing contracts into a concrete design for the cellular path. It is written
> against the split repositories and their current state.
>
> The research that informed this document:
> - [EIOTCLUB IoT SIM in US report](../reports/EIOTCLUB%20IoT%20SIM%20in%20US.md)
> - [SIM7600A radio and AT commands](../../research_notes/EIOTCLUB%20IoT%20SIM%20in%20US/sim7600a_radio_and_at_commands.md)
> - [APN plans and data path](../../research_notes/EIOTCLUB%20IoT%20SIM%20in%20US/apn_plans_and_data_path.md)
> - [Roaming policy and reliability](../../research_notes/EIOTCLUB%20IoT%20SIM%20in%20US/roaming_policy_and_reliability.md)
> - [EIOTCLUB identity and switching](../../research_notes/EIOTCLUB%20IoT%20SIM%20in%20US/eiotclub_identity_and_switching.md)

## 1. Store-and-forward, not live streaming

The cellular path is a **store-and-forward** system: record locally, create
compressed encrypted bundles once, prefer BLE to the iPhone and Tailscale,
briefly activate known Wi-Fi when BLE cannot drain the queue, and use LTE as
the independent fallback. Do not keep a live telemetry stream or VPN session
running just to deliver occasional bundles.

The important catch: adding an LTE modem to an ESP32 does not automatically
make it a Tailscale client. For direct cellular uploads, the dongle must use
the [uplink/v1](../contracts/uplink/v1/spec.md) protocol (device-signed
requests over TLS with a pinned server key). Tailscale explicitly supports
subnet routers for devices that cannot run its client; WireGuard alone is not
the complete Tailscale system
([Tailscale: devices without Tailscale](https://tailscale.com/docs/reference/devices-without-tailscale)).

## 2. LTE hardware: reuse first, then choose based on the real constraint

The [current Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus-model-b/)
contains a SIM7670 Cat-1 modem. An earlier revision used SIM7070G LTE-M, and
Freematics documents automatic modem identification in its library. The car's
own dongle answered as a **SIMCOM SIM7600A-H** on 2026-10-07 (firmware
`env:cairn-netprobe`, CSQ 18, registration searching). Identify the actual
board/modem before buying another LTE module.

| Situation | Recommendation | Reason / approximate USD |
|---|---|---|
| Existing Model B has a working modem | Reuse it; bring cellular support into Cairn's transport layer | Avoid another radio, antenna, power supply, and enclosure redesign. Current Model B specifies Cat-1 at up to 5 Mbps upload |
| Custom STM32 PCB; small uploads and power matter most | Evaluate a carrier-supported LTE-M module, such as SIM7080G | Appropriate candidate for sparse telemetry; prototype with the [Waveshare SIM7080G board](https://www.waveshare.com/sim7080g-cat-m-nb-iot-hat.htm), about $33. Carrier/SIM compatibility still needs verification |
| Custom PCB; faster backlog draining matters most | Evaluate Cat-1, such as SIM7670G | Up to 5 Mbps theoretical uplink; a development HAT is approximately $45 |
| Dongle itself must be a genuine Tailscale node | Put Linux in the vehicle gateway | Tailscale documents embedded binaries and subnet-router integration, but these are a different platform architecture from the current MCU logger |

**Prototype LTE-M and Cat-1 on the actual carrier in the garage before
selecting either.** Weak-signal behaviour and energy per successfully delivered
bundle matter more than advertised maximum speed.

Do not treat NB-IoT as an interchangeable default for LTE-M in this
moving-vehicle design. Select the module, supported mode, SIM, bands, and
carrier together.

### SIM7600A-H specifics

The SIM7600A-H is a three-band LTE part: **LTE-FDD B2/B4/B12 only**, no
LTE-TDD, WCDMA B2/B5, and no GSM radio at all. LTE Cat 4 at 150/50 Mbps.
Detailed radio analysis, AT command reference, and operational considerations
are in the research notes under
`research_notes/EIOTCLUB IoT SIM in US/`. Key constraints:

- **B12 is the single most important band**, giving 700 MHz low-band on both
  T-Mobile and AT&T.
- **B71 (T-Mobile 600 MHz) is absent**: the module will not attach where
  T-Mobile has moved coverage onto 600 MHz only.
- **B13 absence** makes Verizon's primary low-band unreachable; Verizon is off
  the table.
- **No TLS 1.3.** The module supports up to TLS 1.2 only. If the server is
  TLS-1.3-only this module cannot connect.
- **UART bottleneck.** The real throughput ceiling is the 115200 baud UART at
  roughly 11.5 kB/s, not the radio.
- **10240 byte certificate limit.** Upload only the specific root that signs
  the server; a full CA bundle will not fit.

## 3. Getting to the homelab securely

There are three practical paths. They have different privacy and power
tradeoffs.

| Path | Topology | Assessment |
|---|---|---|
| **Preferred everyday path** | Dongle → BLE → iPhone → Tailscale → private Cairn ingestion service | Preserves the original relay model; avoids dongle WAN credentials |
| **Strict private-tailnet path** | Dongle → nearby Linux gateway → LTE/Wi-Fi → Tailscale → homelab | Best fit if "no public ingestion endpoint" is mandatory. The dongle-gateway link still needs protection; Tailscale encrypts the gateway-homelab portion |
| **Compact standalone-MCU path** | Dongle → LTE or Wi-Fi → public HTTPS ingestion → isolated homelab receiver | Practical choice if size/power rule out Linux. Application-encrypted bundles remain the payload protection; public ingestion is an explicit tradeoff |
| **Funnel variant** | Dongle → HTTPS Tailscale Funnel → dedicated receiver | Convenient, but public -- not a private tailnet connection. Funnel exposes the chosen service to internet clients |

### Recommended approach for Cairn's priorities

Keep the phone path private over Tailscale. For autonomous MCU LTE fallback,
use a narrow HTTPS ingest service on an isolated receiver -- not the
dashboard, database, or general homelab API.

If avoiding a public endpoint outranks enclosure size and idle power, use the
Linux gateway instead. There is no configuration trick that makes an ordinary
MCU HTTPS request directly reach a private Tailscale IP without a suitable
intermediary or client.

For either design, enforce these boundaries:

| Boundary | Requirement |
|---|---|
| Payload confidentiality | Encrypt bundles to the ingestion system before handing them to BLE, Wi-Fi, LTE, or a gateway |
| Device authentication | Per-device credentials, revocable independently; no shared fleet secret |
| Public receiver | Upload-only API, request-size limits, quotas, authentication, and no arbitrary filesystem paths |
| Homelab access | Receiver can reach only the necessary ingest service; no general LAN access |
| Replay protection | Device ID + monotonic sequence + authenticated chunk/bundle identity |
| Firmware | Signed updates, rollback protection appropriate to the selected MCU |
| Vehicle boundary | Upload responses must not become arbitrary CAN/OBD commands |
| Logging | Avoid recording coordinates, VINs, credentials, and full payloads in infrastructure logs |

These align with the existing [threat model](threat-model.md) threats N1-N14.

## 4. Radio scheduling: BLE first, known Wi-Fi bursts, LTE fallback

Separate two decisions: "Who can safely hold this bundle?" and "Which path can
actually deliver it?" A BLE connection alone does not mean the iPhone can reach
the server.

### State machine

| State | Behaviour | Exit condition |
|---|---|---|
| **Capture** | Collect OBD/GNSS locally; keep Wi-Fi off | Bundle/checkpoint ready |
| **BLE preferred** | Offer sealed chunks to the paired iPhone | Phone takes durable custody, or progress stalls for roughly 30-60 s |
| **Known-Wi-Fi probe** | Scan only when justified by location/history or backlog; join provisioned networks only | Authenticated ingestion reachable, or roughly 10-20 s expire |
| **Wi-Fi burst** | Drain eligible backlog; BLE control/GPS remains available where feasible | Queue drained, no progress, or energy/time limit reached |
| **LTE fallback** | Attach and upload budget-approved chunks | Queue drained, capped allowance reached, or repeated failure |
| **Backoff** | Wi-Fi off; LTE asleep/off where supported; continue BLE and local recording | Next retry, new network opportunity, or priority event |

For the garage, make a known usable Wi-Fi network outrank LTE immediately
after a stalled/unavailable BLE relay. Do not burn minutes attempting a
marginal cellular upload before trying the garage AP.

**Switch based on successful upload progress -- not just RSSI.** "Strong Wi-Fi
with no server access" and "registered LTE with unusable throughput" are both
failed delivery paths.

### Prevent oscillation and duplicate uploads

Use transport-independent chunk IDs and receiver deduplication. When changing
paths, continue with missing chunks rather than rebuilding or resending a whole
trip.

Keep two acknowledgement levels:

| Acknowledgement | What it means |
|---|---|
| Phone custody | The iPhone durably stored the encrypted chunk; dongle may stop offering it repeatedly over BLE |
| Server commit | The server durably accepted and verified it; dongle may apply its deletion/retention policy |

Retain the dongle copy until server commit unless storage pressure explicitly
triggers a different policy.

On iOS, enable Core Bluetooth background execution and state restoration, but
do not assume guaranteed continuous execution. Apple documents background
support and also states that the system may terminate these apps
([Apple: Core Bluetooth Background Processing](https://developer.apple.com/library/archive/documentation/NetworkingInternetWeb/Conceptual/CoreBluetooth_concepts/CoreBluetoothBackgroundProcessingForIOSApps/PerformingTasksWhileYourAppIsInTheBackground.html)).
That is why local durable queues and autonomous fallback matter.

## 5. Minimize the cellular data allowance: batch, do not continuously stream

The EIOTCLUB plan is a prepaid data-cap AND time-window, whichever expires
first, with no rollover. The 24 GB/360-day tier at roughly $11.70/month is the
only one whose validity window matches an unattended vehicle deployment; every
30-day tier will silently expire between trips even if the data is untouched.
**The time window, not the data cap, is the operational risk.**

### Data pipeline

```text
OBD/GNSS samples
  → typed binary records
  → timestamp/value deltas
  → independently compressed chunks
  → authenticated encryption
  → durable local queue
  → BLE / Wi-Fi / LTE
  → idempotent server ingestion
  → durable commit acknowledgement
```

Compress before encryption. Do not recompress an already sealed bundle for
each transport.

| Design choice | Starting point |
|---|---|
| Encoding | Preserve the existing contract where possible; avoid verbose per-sample JSON on the wire |
| Compression | Benchmark a low-memory embedded codec against real Cairn traces; do not guess a compression ratio |
| Chunk size | Start with 16-64 KiB compressed payloads, then measure RAM, retry cost, and BLE behaviour |
| Upload pattern | Trip-end/checkpoint bursts, not one network transaction per PID/sample |
| Retry | Resume missing chunks with capped exponential backoff |
| LTE policy | Priority summaries/events first; large raw backlog waits for Wi-Fi unless explicitly permitted |
| Accounting | Include traffic both directions, connection overhead, and retries -- not just payload file sizes |
| Budget | Initially cap device-estimated cellular traffic near 70 MB/month; reserve the remainder until carrier measurements establish overhead |

The cap and chunk sizes are engineering starting points. They are not measured
limits for Cairn.

### Why a budget policy is essential

An illustrative workload of 40 bytes/sample x 10 samples/s x 1 driving
hour/day produces about 43.2 MB/month before compression and networking
overhead. At 100 samples/s, it becomes about 432 MB/month.

Those are examples, not estimates of the current logger. Without actual record
sizes, channel rates, and driving time, the budget cannot be promised.

Keep high-rate raw data locally. Cellular should preferentially carry trip
summaries, engine-health events, and selected diagnostic windows; BLE/Wi-Fi
can drain the comprehensive archive.

### Data accounting on the dongle

The device must meter itself and deliberately over-count. The asymmetry is
justified: under-counting risks a silent mid-trip cutoff requiring a human and
a portal with support latency of days, while over-counting merely sends
slightly fewer digests.

- Count bytes at the lowest visible layer.
- Apply a **fixed multiplier of 1.3-1.5x** for IP/TLS/retransmission overhead.
- Charge a **fixed per-session minimum of 2-5 kB** to cover handshake and
  carrier rounding.
- **Persist counters to non-volatile storage before each send**, not after
  (a parked car loses power unpredictably).
- Keep the budget hierarchy strictly nested: per-trip <= daily <= monthly.
- Reserve headroom by stopping at 85-90% of nominal.
- **Count failed attempts too** -- a failed TLS handshake still moves bytes.
- Emit the device's own counters in the digest so the operator can compare
  against the portal figure.

## 6. Hardware acceleration: use what exists, not "all accelerators"

For the current ESP32 target, use the ESP-IDF/Mbed TLS crypto implementation
rather than bypassing it with hand-written crypto. Espressif documents AES and
SHA hardware acceleration, but the benefit depends on chip revision, SDK,
workload, and synchronization. Its older ESP32 configuration documentation
warns that accelerated AES may offer no speed benefit at 240 MHz
([Espressif: mbed TLS](https://docs.espressif.com/projects/esp-idf/en/stable/esp32/api-reference/protocols/mbedtls.html)).

| Operation | Recommendation |
|---|---|
| Bundle encryption | Preserve the audited existing scheme; if designing anew, benchmark an authenticated cipher supported by the selected MCU |
| AES | Enable supported hardware paths and benchmark realistic chunk sizes |
| SHA | Use supported acceleration, respecting concurrency restrictions documented for the target |
| Compression | Treat as CPU work unless the exact chip demonstrably provides a relevant accelerator |
| SD/modem transfers | Use suitable DMA/buffering where available; verify the actual peripheral/interface configuration |
| Tailscale tunnel | Do not assume the MCU AES engine accelerates WireGuard or that the LTE module offloads Tailscale |
| STM32 redesign | Pick the exact MCU first; crypto peripherals, RAM, secure storage, and DMA are part-specific |

For a newly designed AES-GCM bundle format, nonce uniqueness across resets and
interrupted writes must be a first-class requirement. Do not switch the current
bundle crypto merely to chase an accelerator before checking compatibility and
key/nonce handling.

## 7. "Little leakage" has unavoidable limits

Application-encrypted bundles protect telemetry content, but they do not make
cellular activity invisible.

| Observer / component | Residual exposure to account for |
|---|---|
| Cellular carrier | SIM/subscriber association, cellular location, connection timing, and traffic volume |
| Local Wi-Fi infrastructure | Association and traffic metadata |
| BLE observers | Advertising presence and potentially identifying advertisement fields |
| Public HTTPS infrastructure | Endpoint identity and connection metadata |
| Relay phone/gateway | Bundle sizes/timing; should not need plaintext telemetry |
| Homelab | Plaintext after authorized ingestion, plus logs/backups the owner controls |

Leakage-minimising defaults:

- Generic BLE advertisements with no VIN or route data.
- No discovery broadcasts containing telemetry.
- No random Wi-Fi association.
- No analytics/third-party telemetry.
- Encrypted local bundles.
- Capped burst uploads.

There is a tradeoff: frequent tiny transmissions reduce latency but expose
more activity timing and add overhead. Fixed padding or cover traffic can
conceal more metadata, but consumes the allowance. For a capped cellular plan,
choose encrypted batching without cover traffic.

## 8. What to implement first

| Priority | Change |
|---|---|
| 1 | A transport-neutral persistent chunk queue with server-commit acknowledgements |
| 2 | BLE/Wi-Fi/LTE transport scheduler with progress-based failover and hysteresis |
| 3 | Cellular byte ledger, monthly quota, and priority classes |
| 4 | Reuse the existing Freematics modem, if fitted; measure garage delivery reliability |
| 5 | Isolated HTTPS ingestion for standalone MCU fallback, or Linux/Tailscale gateway if private-only access is mandatory |
| 6 | Benchmark compression, crypto, BLE throughput, and energy on the actual hardware before final PCB selection |

The one detail needed to turn this into a concrete hardware plan: the current
target (existing Freematics Model B or a custom STM32 board) and the exact
fitted modem/STM32 part number.