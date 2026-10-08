# Cairn Roadmap

> **v2 is built, deployed and running on the car's dongle. v3 is the next
> architecture: encrypted storage, an enrolled iOS client, Tailscale reach and
> more than one car.** Phases 1–12 (the rebuild) are complete and kept below as
> the record; Phases 13–18 turn what v2 records into answers; **Phases 19–25 are
> the v3 trust-and-transport refactor**, designed as one coherent model in
> [docs/trust-model-v3.md](docs/trust-model-v3.md). The v1 plan is dead; its roadmap document was removed on 2026-10-05
> (it remains in git history) and nothing in it is scheduled. **v2 is now dead too:** v3 breaks the bundle format, the manifest
> and the one-device-one-car assumption on purpose. (Originally with no migration; the owner has since decided a one-way v2 to v3 data migration is wanted, see [server #18](https://github.com/ParkWardRR/cairn-vehicle-server/issues/18).)

Last reviewed 2026-10-05.

> **Where the code lives.** On 2026-10-05 the single repository was split. This one is the
> front door: system documentation, this roadmap, and the shared [contracts](contracts/).
> The server, the web dashboard and the firmware are
> [cairn-vehicle-server](https://github.com/ParkWardRR/cairn-vehicle-server),
> [cairn-vehicle-web-dashboard](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard) and
> [cairn-esp32-device-firmware](https://github.com/ParkWardRR/cairn-esp32-device-firmware); the
> iPhone app has its own. The original is preserved read-only as
> [cairn-original-monorepo-archive](https://github.com/ParkWardRR/cairn-original-monorepo-archive).
> The plan, what was executed and the follow-up issues are in [docs/repo-split-plan.md](docs/repo-split-plan.md).

> **Direction changes since the split (owner, 2026-10-05).** These supersede what the phase history below says.
>
> - **The dongle gets Wi-Fi and LTE back**, besides BLE, to put boot speed and speed to upload first. Phase 26 ("No Wi-Fi") is superseded; the decision, its security gates and the work are in [#19](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/19), the device uplink protocol [#22](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/22), the threat model update [#23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23), firmware [Wi-Fi](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/15), [LTE](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/16), [uplink manager](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/17) and the [credential gate](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/18). Until the firmware ships it, the documents that say "no Wi-Fi" describe the shipped firmware correctly.
>   **Security first:** flash and NVS encryption (Phase 24) ship **before** any network credential is stored on the chip *(superseded twice. First on 2026-10-07 — the car's chip cannot take the eFuse burns Phase 24 needs. Then, later the same day, by shipping: credentials are now in plaintext flash in the gitignored `include/secrets.h` and the transports are live. That is a knowing trade, not an oversight, and its cost is stated at the point of definition — a flash dump permits uploading **as** this device and reading what it uploads, but not deleting anything, because a prune requires a server-signed receipt verified on-device. The narrower protection in the next block is still the right destination.)*. **BLE first:** BLE is the dongle's home state and the one radio is time-sliced, not shared; it goes to Wi-Fi only at home, in bounded slots, returning to BLE to report to the phone, retrying Wi-Fi once if needed, and returning to BLE to check in for new instructions ([schedule](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/17)).
>   **Superseded 2026-10-07 by the owner: LTE carries whole sealed bundles, not digests.** The reasoning that overturned it is the same property that made digests attractive — a digest's acknowledgement can never authorise pruning — which means a digest-only cellular path never frees the card and only postpones the problem it exists to solve. Nor is compression worth building: a trip is about 230 KB, so a 2 GB plan carries some nine thousand of them, and sealed segments are AEAD ciphertext that `gzip -9` reduces only to 90% of raw (measured). **Implemented and proven on 2026-10-07**: the car's dongle attached to T-Mobile US, opened TLSv1.2 through the public Funnel ingress, uploaded 158,906 bytes and pruned the bundle on a verified receipt. `lib/cairn_digest` is kept but unused by the uplink. What the user sees and sets — consumption and limits enforced on the dongle — is still open: [digest contract #26](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/26), [firmware accounting and limits #21](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/21), [server ingest #25](https://github.com/ParkWardRR/cairn-vehicle-server/issues/25). The cellular design is in [docs/lte-cellular-design.md](docs/lte-cellular-design.md); what has actually run is in the firmware's [uplink status](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/network-uplink-status.md).
> - **Wi-Fi and LTE are configured in the web UI and the iOS app**; the configuration reaches the dongle sealed to its key, via the server and/or the phone: [config contract #27](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/27), [server #24](https://github.com/ParkWardRR/cairn-vehicle-server/issues/24), [firmware receiver #22](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/22), [web #16](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard/issues/16), [iOS #30](https://github.com/ParkWardRR/cairn-ios-companion-app/issues/30). **The web layer must authenticate before it takes any secret**: [web #15](https://github.com/ParkWardRR/cairn-vehicle-web-dashboard/issues/15). When no phone is near, the dongle may scan for the home network.
> - **Per-engine YAML profiles** in a folder in the firmware repository; the user compiles in all engines or a chosen few, and the app checks what is installed: [contract #20](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/20), [firmware #13](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/13), [device info #21](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/21), [iOS #27](https://github.com/ParkWardRR/cairn-ios-companion-app/issues/27).
> - **A v2 to v3 data migration is wanted** (the "no migration" lines below are superseded): [server #18](https://github.com/ParkWardRR/cairn-vehicle-server/issues/18).
> - **A dedicated Bluetooth page in the iOS Settings** for setup, troubleshooting, BLE info and several dongles: [iOS #25](https://github.com/ParkWardRR/cairn-ios-companion-app/issues/25), [#26](https://github.com/ParkWardRR/cairn-ios-companion-app/issues/26).
> - **Boot speed and time-to-upload** are measured and budgeted across the system: [#25](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/25). Multipath, torrent-style chunk transfer is an exploration only: [#24](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/24).
> The phase history below was written against the single repository: a path such as `ui/` or
> `firmware/cairn-v2/` is now the root of the matching repository.

> **Direction change (owner, 2026-10-07): the credential gate is answered without eFuse burns.**
>
> - **"Phase 24 first" could never open.** The car's dongle measures `ESP32-D0WDQ6` revision v1.0. Flash and NVS encryption are eFuse burns, burns are ruled out on the only unit, no sacrificial unit was ever bought, and secure boot V2 does not exist on this silicon at all. Gating Wi-Fi and LTE on Phase 24 therefore blocked them permanently rather than ordering them, which is not what "security first" was meant to mean.
> - **In its place:** network credentials are stored under a key the chip cannot reconstruct on its own, derived with the server's help from the already-escrowed device root, so a flash dump yields ciphertext rather than an SSID, a password, an APN and a SIM PIN ([firmware #18](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/18), third option). It is a narrower protection than flash encryption and must not be written up as more: `K_root` and the device signing seed stay readable from an extracted chip, and serial reflashing stays possible.
> - **The open design, which lands before any credential is stored:** where that key lives between boots. RAM only costs autonomy (a dongle that reboots alone cannot rejoin a network until a phone or the server hands the key back); NVS re-creates the plaintext secret under another name; an unwrap over the bonded BLE link fits the uplink schedule but has no wire format, replay rule or no-phone-for-a-week recovery yet. This belongs in the threat model update ([#23](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/23)) and a contract.
> - **Phase 24 is not cancelled**, it is reassigned: it is the plan for whatever hardware replaces this dongle (revision v3.0 or later, so secure boot V2 is available), not for the unit in the car.
> - **The LTE hardware is confirmed and the link works.** A SIMCOM SIM7600A-H answered on the car's own dongle (`env:cairn-netprobe`, 2026-10-07); earlier notes calling the modem absent or unconfirmed were reading an unpowered socket. Later that day the full path ran: attach to T-Mobile US (PLMN 311480), PDP address, TLSv1.2 ECDHE-ECDSA-AES128-GCM through the public Funnel ingress, 158,906 bytes uploaded, receipt verified against the pinned key, bundle pruned. TLS runs on the ESP32 rather than in the modem, because Funnel requires SNI and the module's `enableSNI` is undocumented on this firmware revision. Measured throughput ~3 KB/s, bounded by the 115200 UART and an `AT+CIPSEND` round trip per 1024 bytes — **do not infer cellular throughput from the UART ceiling**. See [docs/lte-cellular-design.md](docs/lte-cellular-design.md) and the firmware's [uplink status](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/network-uplink-status.md).

## Product direction

Where the project is going, in the order the owner chose (it is theirs to change). Each theme
names the repository that owns the work and the contract it touches. Contract changes are
**additive** and follow the contract-first loop in [contracts/README.md](contracts/README.md).

| # | Theme | What it means | Owning repositories | Contract it touches |
|---|---|---|---|---|
| 1 | **History** | Statistics for any period (year, quarter, month, week, a custom range); bookmark, tag and search a route, so "revisit an interesting drive" is not scrolling a list | web (pages, filters); server (new store views such as a yearly summary) | [`store/v1`](contracts/store/v1/README.md): new views, additive |
| 2 | **Vehicle insight** | A **tune record** (when the tune changed, set by the user, stored per vehicle) so "since the tune" has a meaning; baselines and trends; stock against after-tune; a plain "is my car healthy?" summary | server (derived metrics, per-vehicle baselines); web (health and tune views); firmware only if more PIDs are needed | `store/v1` additive; the per-vehicle record |
| 3 | **Sharing** | Pick trips, preview, export as a portable file with redaction (privacy zones that blank a start, end or place). A file the owner sends, **not** a hosted service | web (choose, preview, export); server (anything that issues or checks a share token); iOS (share sheet) | [`share/v1`](contracts/share/v1/README.md) (reserved, draft). **The [threat model](docs/threat-model.md#sharing-design-constraints-before-any-design) comes first** |
| 4 | **Approachability** | A plain-language pass on the web and every README, simple view by default and detail on request, setup docs that do not assume a homelab. Run alongside the others, since it is mostly copy and defaults | every README; web; iOS | none |
| 5 | **Data control** | Export, import and backup for every store that holds user data (trips, places, annotations), as each store gains user data; render nice shareable high-resolution images | server, web, iOS | `store/v1`; `share/v1` |

Open, not scheduled: GPU acceleration for analysis (no demonstrated need yet), and an optional
S3-compatible object store such as Garage on the server for the content-addressed store.

### What the work must not foreclose

- **Trip identity stays stable and portable.** Sharing needs an id that survives export and
  import. Today it is the boot id; do not change it casually.
- **Per-vehicle scoping stays** (`vehicle_id` through the store and the API): insight and
  sharing are both per vehicle.
- **The store contract stays additive.** New views are added; existing ones are not repurposed.
- **Every store keeps an export path**, and user data never moves somewhere an export cannot reach.
- **The web layer keeps working with no cloud.**
- **Privacy zones are a first-class idea**, so the place engine and saved places stay
  reachable from the export path.

### Reasons to use it, as acceptance tests

| Reason | A release is not done until |
|---|---|
| Useful automatic capture | a drive is captured and visible without opening any app |
| Detailed vehicle information | the OBD detail is browsable per trip and per vehicle |
| Control of data | every store holding user data can be exported, imported and backed up, and nothing requires an account or a cloud service |
| Adaptability | a new vehicle, a new place kind or a new metric can be added by configuration or a documented extension point, not by editing core code |

## Where Cairn is

| Layer | State |
|---|---|
| Bundle format v3 | **Complete on the host.** Every frame AEAD-encrypted, every bundle bound to a vehicle, an assignment and a device counter. Three implementations (Go, Rust, firmware C) agree on 33 conformance vectors, structural and keyed. v2 is retired |
| Firmware | **No Wi-Fi (decided 2026-10-05): the dongle holds no network credentials and has no network stack**; all server sync goes through the iOS app over BLE ([Phase 26](#phase-26--no-wi-fi-the-phone-is-the-uplink--superseded)). v3 provisioned and enrolled on the car's dongle (root escrowed, assigned to the 428i); capture, seal and storage checks pass on the real unit with its SD card. **First over-the-air BLE offload completed 2026-10-06**: nine bundles pulled in 62 s, nine receipts verified against the pinned key on the device, nine bundles pruned (dev-pairing build — Just Works, bonds cleared each boot, bench-mode auto-detect; see the firmware README's "Pairing and access" caveat). Secure OTA written, install unexercised |
| Ingest server | **Deployed v3** on the Cairn VM: a legacy mutual-TLS device listener (no dongle uses it now; retired in Phase 26), a separate app listener (LAN TLS verified end to end from the Mac), enrolment, escrowed keys, vehicle and counter binding. Tailscale installed, login pending |
| Decode pipeline | **Complete and per vehicle.** Idempotent and reproducible; PostgreSQL/PostGIS and DuckDB layers both carry `vehicle_id`; non-destructive migration 005 |
| Engine telemetry | **Recording** boost, MAP, mixture, fuel trims and fuel level as `OBD_EXTENDED`; PID support on the real car still being established (Phase 13) |
| Analytical store | **Deployed.** `cairn-tsdb`, an in-memory DuckDB rebuilt from the CAS and the SD card on every start; Parquet snapshot export via CLI and HTTP (Phase 14) |
| Web UI | Nuxt 3 + Vue 3 dashboard (`ui/`), with a vehicle selector; analysis pages are single-vehicle and never blend two cars. Deployed |
| iOS companion | Adopting. Enrolment, signing, encrypted store, vehicles and annotations are in; the **BLE offload codec, session and transfer checks** landed on 2026-10-06 (PR #32), but **CoreBluetooth wiring (#14) is still open** — the app does not yet pair with the dongle, there is still no Mac CI runner. `cmd/cairn-phone` in the server repository is the reference phone and **carried the first real trip on 2026-10-06**, so the iPhone is not on the critical path to it |
| BLE companion | **Running.** Phone GPS reinforcement via NimBLE GATT. Bundle offload specified ([contracts/ble/v1/offload.md](contracts/ble/v1/offload.md)), **implemented in the firmware, flashed and run over the air 2026-10-06** (9 bundles, 62 s, 9 receipts pruned on the device) |
| Parked current draw | **Unmeasured.** Needs a meter, not a terminal. Still the single most useful measurement left |

### The v3 target

| Layer | Target |
|---|---|
| Storage | Every SD frame AEAD-encrypted (XChaCha20-Poly1305, per-frame random nonce, HKDF per-segment keys); the card is a cache of ciphertext, never the security boundary |
| Identity | Devices: Ed25519 manifests + revocation (no device certificates any more). Apps: P-256 Secure-Enclave keys, signed requests, one-time invitations, revocation |
| Reach | LAN, plus Tailscale via `tailscale serve` on the host. Tailscale is reachability, not authorization; Funnel is never used |
| Scope | Vehicles and device assignments are first-class; every bundle is bound to a vehicle, an assignment and a monotonic device counter |
| Hardware | Flash encryption and secure boot on the dongle, gated behind a proven OTA path and a sacrificial unit. **A target for replacement hardware, not for the dongle in the car**: it is revision v1.0, the burns are ruled out on the only unit, and no sacrificial unit was ever bought (2026-10-07) |

## Guiding invariants

1. **A sealed bundle is never mutated.** It is either locally recoverable,
   remotely receipt-confirmed, or both.
2. **No byte is deleted without a locally verified signed receipt.** Time,
   storage pressure and operator impatience are all insufficient justification.
3. **Ordering truth is `(boot_id, monotonic_seq)`, never wall-clock UTC.** GNSS
   time jumps; UTC is an annotation with an uncertainty, not an index.
4. **Honest incompleteness beats fabricated continuity.** A trip with a marked
   GNSS gap is useful; a route interpolated from stale fixes is not.
5. **Anything derived is disposable and must prove it reproduces.** Raw bundles
   are authoritative. A derived store — PostgreSQL or the in-memory one — is
   rebuilt from them, carries the digest that shows the rebuild matched, and is
   not served when it does not.

## Decisions

| Decision | Choice |
|---|---|
| Firmware | ESP-IDF application with `arduino-esp32` as a component, keeping the vendored FreematicsPlus drivers. Rebuild the application, not the hardware access — this repo already contains one failed custom HAL |
| Server | Go, in a top-level `server/` directory |
| Rebuilt | Firmware, ingest server, database schema, bundle format, emulator |
| Retired | **2026-10-05, repo cleanup:** Odin tools, MoonBit plugins (WASM, wazero), the Gleam orchestrator, the Zig bundle/CLI tools (`tripctl`) and the v1 Go ingest, the Rust trajectory tool, the Mojo and `moonbit/` experiments, the v1 Compose/Podman stack and the emulator's v1 capture path. They were optional side tooling that no longer fed anything on the v3 path; they remain in git history. The v0.1.0 release tool tarballs no longer exist |
| Replaced | SvelteKit web UI → Nuxt 3 + Vue 3 dashboard (`ui/`) |
| Stack shape | C (firmware), Go (server), Rust (emulator: conformance and fault matrix only) and the Nuxt/TypeScript UI, plus SQL migrations and shell/systemd deploy scripts. Odin, MoonBit, Gleam and Zig were retired on 2026-10-05; see the Retired row |
| Analytical store | **In-memory DuckDB**, behind a small Go service. Chosen because the data is tiny (hundreds of KB today) and the useful questions are ASOF joins across streams polled at different rates — OBD against boost against GNSS. InfluxDB 3 was rejected as UTC-keyed and, as far as was checked, without ASOF; QuestDB as a JVM that is memory-mapped rather than in-memory, on a 7.3 GB box shared with the runner and ingest stack |
| Analytical time key | `(boot_id, mono_ms)`, never UTC. UTC rides along as a column |
| v1 | **Dead.** No reader, no migration, no compatibility. Tools that scan an SD card ignore `trips/` and load only sealed bundles from `bundles/` |
| v2 | **Dead as of 2026-10-04.** Replaced by v3 (format `CRN3`, manifest v3). No v2 reader and no v2 compatibility; a one-way **data migration** is now wanted ([server #18](https://github.com/ParkWardRR/cairn-vehicle-server/issues/18)) |
| Storage encryption | Application-layer AEAD per frame, because the ESP32 has no SD encryption hardware. Random nonce per frame (a seq-derived nonce would be reused after a torn-tail rewrite). Server escrows each device's root; the card alone cannot decrypt |
| App authentication | **Per-request P-256 signatures**, not mTLS: `tailscale serve` terminates TLS, so a client certificate cannot reach Cairn on the Tailnet path. Separate listener from the device's mTLS port |
| Assignment validity | Judged by the device's monotonic counter, **not** the clock (invariant 3) |
| Counter reuse | A counter bound to different content is a forgery or a rolled-back device: quarantine, never ingest |
| Server-side raw data | Stays ciphertext in the CAS; keys live in a wrapped keystore apart from the data directory; destroying a root crypto-shreds every copy including backups |

Target hardware is **classic ESP32** (xtensa LX6, WROVER with PSRAM), not an
S3. There is no secure element, and the eFuse flash-encryption key cannot be
read or derived from by software, so the root key is a random value in NVS inside
hardware-encrypted flash — flash encryption protects it by protecting the flash
it lives in. See [docs/esp32-hardening.md](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/esp32-hardening.md).

The car: an N20 428i on a BM3 Stage 1 tune, ~E41. For this car fuel trims
and mixture say more about engine health than boost does, which is why Phase 15
leads with them.

---

# Ahead

## Phase 13 — Engine telemetry on the real car — **in progress**

The recording side. Shipped 2026-10-02/03: boost, mixture and fuel trims as
`OBD_EXTENDED`; a PID validation firmware build (`pidtest`) that probes what the
ECU actually answers; lambda scale and raw 0x43/0x44 reads corrected; the
255 kPa MAP ceiling flagged; a fuel-type and ethanol-content probe; GNSS
reading-freshness reporting; standby windows recorded so parked draw is
measurable; parked mode silent on the vehicle bus. Since then: fuel level
(PID 0x2F) added to the capture chain; cold boot cut from ~11 s to ~3 s;
cross-boot capture continuity via `trip_seq` in the manifest; BLE companion
for phone GPS reinforcement via NimBLE GATT, with radio handoff between BLE
and Wi-Fi; GNSS null-island and outlier sanitization at decode.

- [x] Boost, MAP, baro, MAF, lambda, STFT/LTFT recorded and decoded
- [x] Fuel level (PID 0x2F) recorded and decoded
- [x] MAP saturation (`MAPSaturated`) surfaced rather than reported as a boost
      reading — a sensor pinned at its ceiling is not a measurement
- [x] Journal sequence range no longer leaks into the capture's, and the drain
      rate is guarded
- [x] Cross-boot capture continuity: `trip_seq` in the manifest ties a capture
      to the sequence of drives, not just the boot
- [x] Cold boot cut from ~11 s to ~3 s
- [x] BLE companion — phone GPS reinforcement via NimBLE GATT, passkey-protected,
      with golden-vector validation, disconnect cleanup, and storage safety;
      radio handed off between BLE and Wi-Fi (only one 2.4 GHz radio)
- [x] GNSS sanitization at decode: null-island rows (module reports fix but
      no coordinates) dropped; outliers filtered from heatmap, places and
      dashboard
- [ ] **Establish which PIDs this ECU really answers.** `BOOST_CONTROL` (0x70)
      reports `support=1` yet returns `NO DATA` in the validation-build log, so
      "supported" and "answers" are not the same thing; record the supported set for the
      car in `docs/` so the decoder and the views know what to expect
- [ ] Ethanol content: confirm whether the ECU answers 0x51/0x52 at all. If not,
      the ~E41 figure has to come from fuel-trim behaviour (Phase 15), not a PID
- [ ] Park-draw measurement with a meter, using the recorded standby windows as
      the timeline

## Phase 14 — In-memory analytical store — **done**

`server/internal/tsdb`, `server/cmd/cairn-tsdb`, `deploy/systemd/cairn-tsdb.service`.
A derived view: the CAS and the SD card are copied into a private scratch CAS,
each bundle is decoded **twice** through the production decoder, loaded into an
in-memory DuckDB, and reconciled row for row against what the decoder produced.
Nothing is persisted; a restart rebuilds.

- [x] Loads committed server bundles (offer + receipt) and sealed v2 bundles
      from the card, deduplicated on `content_root`
- [x] Reproducibility gate: second decode must match the first digest, and
      per-bundle row counts are re-read from the database; a failing build is
      not served
- [x] Scratch CAS rather than opening the live one — `cas.Open` clears `tmp/`,
      which in the server's data directory holds an upload in flight
- [x] Keyed on `(boot_id, mono_ms)`; `v_telemetry` ASOF-joins boost and GNSS onto
      each OBD poll and exposes `boost_age_ms` / `gnss_age_ms`
- [x] Locked down: single read-only statement per request, always-rolled-back
      transaction, external file/network access disabled, configuration frozen,
      loopback bind
- [x] Deployed on the VM: `MemoryMax=3G`, DuckDB capped at 2 GB, hardened unit
- [x] Measured on the VM at 17.5M synthetic rows: scans 1–20 ms, a double ASOF
      join across 5M anchors ~0.72 s. Real data today is a few thousand rows, so
      this is headroom, not a requirement
- [x] v1 explicitly ignored and documented as dead
- [x] **Reproducibility in CI.** `go test ./...` in `build-server` already runs
      the store's tests; the runner is native on the VM, where gcc is present, so
      cgo builds there. The tests drive synthetic bundles laid out as an SD card
      through the real snapshot, decode and load path, and also cover: v1 `trips/`
      ignored, a member failing its manifest digest refused, and dedup on
      `content_root`
- [x] **Golden digests.** `output_digest` pinned for two synthetic bundles (one
      with a GNSS gap and fix-less samples) at decoder Version 1, so a decoder
      change that alters output fails a test instead of silently shifting every
      number. Mutation-checked: bumping `Version` fails both. A deliberate bump
      updates the pins in the same commit
- [x] **Reload on commit** — `-watch 5s` polls `receipts/` (a receipt is the
      commit point) and the card mirror, and rebuilds once a change has held still
      for a full interval. A rebuild that fails or does not reproduce leaves the
      previous store serving. Verified live: adding and removing a bundle each
      triggered one rebuild with no `/reload`
- [x] **Parity with PostgreSQL.** Decode one bundle set into both layers and
      assert equal row counts per table. Proves the two derived views agree.
      Added `norm.boost_samples` table + migration so PostgreSQL no longer drops
      boost data; `TestParityWithDuckDB` asserts all seven tables match
- [x] `/metrics` — build time, bundle and row counts, reproduced count,
      problems, decoder version, Go heap (DuckDB's native memory is bounded by
      `-memory` and the unit's `MemoryMax`, not reported here)
- [x] Mirror of the card: `deploy/tsdb-mirror.sh <user@host> [card-dir]` — additive
      rsync of `bundles/` as the service user, then reload, then the reproduced
      counts; a build that does not reproduce fails the script
- [x] **Authentication before any non-loopback bind.** Reuses the private CA and
      `internal/mtls` package. Loopback stays plain HTTP; non-loopback requires
      `-tls-cert` and `-tls-key` (binary refuses otherwise). Optional `-tls-client-ca`
      for mutual TLS. Systemd unit has a commented mTLS example
- [x] **Parquet snapshot export.** `GET /snapshot` returns a tar.gz of every table
      as a Parquet file plus a JSON manifest; also available via
      `cairn-tsdb -snapshot <path>`. Cached per build; 4 tests covering the
      handler and the export itself
- [x] **`cairn-push` CLI** (`server/cmd/cairn-push`) — ingest sealed v2 bundles
      from an SD card into the server via mTLS, for when the dongle cannot sync
      on its own

**Persisting the store is deliberately out of scope.** Volatility is the design.
The trigger to reconsider is a measured one: if a rebuild exceeds ~5 s.

## Phase 15 — Analysis views for the car — **in progress**

Views in `internal/tsdb/schema.go`, not code, each with a test on synthetic
data whose answer is known in advance.

- [x] `v_drive_summary` — per boot: duration, max speed, max RPM, OBD/GNSS
      sample counts, gap count and duration, bundle warnings
- [x] `v_trim_map` — STFT/LTFT binned by RPM (500-step) and load (10%-step),
      ASOF-joined to the nearest boost reading within 2 s. For an ethanol blend
      the long-term trim is the honest signal; this is the view that estimates
      drift from the ~E41 baseline
- [x] `v_boost_curve` — boost pressure against RPM, excluding rows whose boost
      reading is stale (> 2 s) or MAP-saturated (≥ 255 kPa)
- [x] `v_pulls` — gap-and-island WOT detection (throttle ≥ 70%, RPM rise > 500,
      ≥ 2 samples), with peak boost, mean lambda, STFT and LTFT per pull from
      the boost readings in the pull's time window (MAP-saturated excluded)
- [x] `v_speed_agreement` — OBD speed against GNSS speed, with ratio (only where
      both > 5 kph), GNSS age, and fix-type filter (3D fix required, < 5 s)
- [x] Every view filters on the `*_age_ms` columns it relies on and says so —
      an ASOF join always finds *something*, so staleness has to be an explicit
      predicate, never a default
- [x] 10 tests covering the views: correct binning, age-staleness exclusion,
      MAP-saturation exclusion, pull detection and non-detection, speed ratio,
      no-fix exclusion, drive summary with gaps and warnings

## Phase 16 — Surfacing it — **in progress**

The Nuxt 3 + Vue 3 dashboard (`ui/`) replaced the SvelteKit and legacy PWA UIs.
It reads the analytical store through server-side API routes proxying to
`cairn-tsdb`, never from the browser directly and never via arbitrary SQL.

- [x] Nuxt 3 app with ECharts, Leaflet, Pinia; talks to `cairn-tsdb` via server
      API routes
- [x] Dashboard: trip totals, drive heatmap (CARTO Voyager tiles with dark
      filter), last trip, device health
- [x] Trip list with mini route maps; trip detail with speed-coloured route,
      interactive timeline, GPS health, point inspection, estimated start
- [x] Boost & Power, Fuel & Tune, Analytics, Behavior, Calibration pages
- [x] Fuel economy page with MPG estimation and tuning context
- [x] Places page with GPS acquisition timing
- [x] Device/system page with health history, decoded bundles, store status
- [x] GNSS outlier filtering across heatmap, places, dashboard and trip routes
- [ ] Cards for the Phase 15 views; each shows its sample count and the age
      threshold it applied

## Phase 17 — Close out Phase 9 with real traces — **planned**

Phase 9's two open items were blocked on bench data by design. Real drives and
the analytical store remove the block.

- [ ] Tune start/stop thresholds against recorded `transition` rows: query the
      scores around each journalled transition for false starts and late stops,
      then change the policy and **bump the policy version** so a decision in the
      data stays explainable
- [ ] Event-adaptive sampling across GNSS, IMU and OBD, once the traces show
      which events matter and what rate they need

## Phase 18 — Hardening — **planned**

- [ ] Storage-full matrix row (open since Phase 3). ~~Plugin timeout row~~ — retired 2026-10-05: the plugin system was removed
- [ ] Exercise the OTA install on the device; it has only been verified on the
      host
- [x] ~~Decide on flash encryption and secure boot~~ — decided 2026-10-04: **yes,
      gated.** Moved to Phase 24 with its prerequisites. Both stay off until the
      OTA path is proven on hardware and a sacrificial unit has been through the
      whole procedure — they are irreversible

---

# v3 — Trust, transport and vehicle scope (Phases 19–25)

Design: [docs/trust-model-v3.md](docs/trust-model-v3.md). Origin: an external
architecture review (2026-10-04) of this repository and the iOS companion. It
asked for one security model across encrypted device storage, authenticated
multi-transport sync and a vehicle-scoped data model, rather than three
independent features. Where cross-checking the review against the code found
something it got wrong, the design says so (classic ESP32 has no software-usable
eFuse key; `tailscale serve` cannot pass client certificates; the mTLS client key
currently lives on the SD card, which contradicts the card-is-not-a-boundary
rule).

Breaking changes are authorised. (There was no migration at the time; a one-way v2 to v3 data migration has since been decided, server #18.)

## Phase 19 — Vehicles, assignments and the device counter — **done**

`server/internal/{vehicles,counters,keystore,jsonstore}`, binding in `internal/intake`.

- [x] `vehicles`: vehicle registry (display identity, `engine_code` as a label
      not an identity, VIN sealed at rest with only the last four readable) and
      device→vehicle assignments with explicit reassignment; one recorder per
      vehicle; archived vehicles accept no new work
- [x] Assignment validity by **counter order, not wall clock**: a bundle under a
      superseded assignment is refused; a late upload from before the switch is
      not. Mutation-checked
- [x] `counters`: per-device monotonic counter bound to `content_root` — same pair
      is an idempotent duplicate, same counter with different content is a
      conflict (forgery / cloned or rolled-back device), holes are reported as
      `Missing` not refused, `Resume` re-bases a re-enrolled device
- [x] `keystore`: per-device storage-root escrow, wrapped under a master key with
      device and version bound as AAD, write-once versions, `Destroy` for
      crypto-shredding; implements the format's `RootKeyResolver`
- [x] `jsonstore`: one durable, externally-editable JSON document implementation
      shared by the new registries (atomic write, 0600, change detection so a CLI
      revocation takes effect without a restart)
- [x] **Intake binds the manifest** to the registries at offer time — unknown or
      mismatched assignment → refused (`403`); superseded → refused; no escrowed
      root for the key version → refused (the server never accepts what it cannot
      decode); counter reused for different content → **quarantined** (`422`,
      manifest and signature kept in the CAS for inspection); a gap → accepted
      with a ledger warning. The counter is bound at **commit**, before the
      receipt, so an abandoned offer spends nothing and two racing offers cannot
      both win. Mutation-checked
- [x] Ledger events `assignment_refused`, `quarantined`, `counter_gap`,
      `key_missing` (every refusal requires a reason)
- [x] `cairn-server -enroll` now requires `-enroll-root` and escrows it, returns
      the counter floor, and `cairn-admin` manages vehicles and assignments
- [x] `vehicle_id` through the derived layers. **decode** carries the manifest's
      vehicle on every row and in the output digest (decoder version 3).
      **tsdb**: every table and view carries it and every join and ASOF join
      keys on it; a two-car test shares boot ids and timestamps so only
      `vehicle_id` can separate the cars, and four join mutations are each caught.
      **PostgreSQL** migration `005_vehicle.sql` is non-destructive (existing rows
      are labelled with the all-zero "unassigned" vehicle, verified on a
      populated database; the default is then dropped so new inserts must name
      a vehicle) and a rollup is a vehicle's day. **Snapshot**:
      `GET /snapshot?vehicle=` filters every table. **UI**: a vehicle selector;
      analysis pages are single-vehicle and never fall back to a blend
- [x] Seeded on the VM: `BMW 428i (F32) — N20` (year not recorded) and
      `2017 BMW M240i — B58`; the dongle is assigned to the 428i
- [x] Emulator matrix rows for the device side (counter across a power cut
      mid-seal, restored card, wrong-key card, torn tail under encryption)
- [x] The DB-only store tests had been **silently skipped** in ordinary runs and
      had gone stale; they now run against a real PostGIS (OrbStack locally, the
      CI service in the pipeline) and pass. One asserted behaviour the null-island
      sanitization had already changed and was restated; one lacked the key
      provider

## Phase 20 — App identity and the sync API — **done and deployed on the LAN; Tailnet awaiting login**

`server/internal/{clients,syncapi,audit}`, `server/cmd/{cairn-admin,cairn-server}`,
[contracts/sync/v1/spec.md](contracts/sync/v1/spec.md). 27 test functions, with the
security-critical ones mutation-checked (replay cache, signature check, timestamp
window, revoked-client check, scope filter, Funnel refusal, bearer-on-admin,
admin check).

- [x] One-time invitations (hash-at-rest, 10-minute TTL, single use) and app
      enrolment with proof of possession; a bad proof does not burn the code;
      P-256 so the key can live in the Secure Enclave
- [x] Per-request signature authentication (±120 s window, nonce cache, body hash
      in the signed string, uniform 401) and 1-hour bearer tokens for background
      `URLSession` uploads, re-checked against the registry on every use so
      revocation kills them
- [x] Transport classification (`loopback` / `lan` / `tailnet`); Tailscale Serve
      identity headers honoured **only** from loopback and only with
      `-trust-tailscale-serve`; Funnel-marked requests rejected; optional
      tailnet-identity allow-list
- [x] `POST /v1/sync/push`, `GET /v1/sync/pull`, `POST /v1/sync/ack`; durable
      append-only log with torn-tail recovery; server-assigned ordering;
      idempotency by operation id and key; per-field revisions with conflict
      results; opaque, repeatable cursor with a reset epoch (`410`)
- [x] `GET /v1/health` for local-first / Tailnet-fallback probing
- [x] Admin revocation: `POST /v1/devices/{id}/revoke`, `/v1/clients/{id}/revoke`
- [x] Append-only audit log; a test greps it for payload markers, coordinates,
      tokens, signatures, public keys and the VIN
- [x] Separate app listener (`-app-addr`); plain HTTP only on loopback
- [x] `GET /v1/snapshot?vehicle=`: authenticated proxy of the loopback
      `cairn-tsdb` snapshot. A scoped client must name a vehicle of its own; a
      full-scope client may omit it. Out-of-scope and malformed ids never reach
      the analytical store (three scope mutations caught)
- [x] Published, test-pinned vectors for the signing string and canonical-JSON
      content hash, for the iOS client
- [x] `cairn-admin`: vehicles, assignments, counters, invitations, client
      revocation
- [x] `trip_summary` entities are published from tsdb's `v_trip_summary` by a
      publisher inside cairn-server (the sync log is single-writer, so the decode
      worker cannot write it). Integers only, idempotent by content digest,
      scoped per vehicle; schema in `contracts/sync/v1/spec.md`
- [x] A loopback-only local listener serves vehicle display names to the UI. It
      has no authentication of its own (the loopback bind is the control) and
      refuses anything carrying Tailscale or Funnel headers
- [x] **Deployed on the VM and exercised end to end over the LAN** from the Mac
      against verified TLS: enrolment, signed pull returning both cars with no
      VIN material, admin listing, unsigned calls refused, and a revocation
      that took effect on the next request
- [ ] Exercise from a real phone over the LAN and the Tailnet (the Tailnet side
      needs the host's Tailscale login to be approved — see Phase 23)
- [x] `cairn-admin device enroll` with the sealed-root enrolment blob, requiring
      the operator to confirm the device's fingerprint; refuses a changed
      signing key, a different root for an escrowed version, a revoked device
      and a shredded key version before writing anything

## Phase 21 — Bundle format v3: encrypted segments — **done on the host; hardware round trip open**

[contracts/format/v3/spec.md](contracts/format/v3/spec.md), `server/format`,
`contracts/format/v3/vectors/` (33 deterministic vectors, regenerate-to-empty-diff
verified).

- [x] `CRN3` segment header (128 bytes) carrying `vehicle_id`, `assignment_id`,
      `storage_key_version`, `device_counter`
- [x] Every frame AEAD-encrypted; CRC chain and Merkle root over **ciphertext**, so
      structure, torn tails and integrity verify without a key
- [x] XChaCha20-Poly1305, random 24-byte nonce per frame; AAD binds the frame
      header and the segment header
- [x] HKDF-SHA256 per-segment keys, salted with `vehicle_id`
- [x] Manifest v3: vehicle, assignment, counter, key version, suite
- [x] Conformance vectors incl. auth-tag tamper, frame transplant, wrong vehicle,
      wrong key version, manifest/segment-header mismatch; every segment vector
      states a structural verdict (no key) and a keyed verdict
- [x] Decode, `cairn-tsdb`, `cairn-worker` and `cairn-verify` read v3 through a
      `KeyProvider`; golden decode digests re-pinned (inputs changed, row counts
      did not)
- [x] Go v2 code and `fixtures/format-v2` removed; CI paths moved to v3
- [x] **Rust emulator port**: 33/33 vectors (23 structural + 22 keyed
      verdicts), 36 unit tests, fault matrix 20/20 including the four new rows
      (torn tail under encryption, card restored to an older image, wrong-key
      card, counter across a power cut mid-seal) and all 20 against a live v3
      server. 11 mutations each caught
- [x] **C firmware port**: new `cf_aead.c` (HChaCha20 / XChaCha20-Poly1305) and
      `cf_hkdf.c`; 33/33 vectors, 9/9 primitive known answers, the device
      re-seals every decrypted vector frame to the vector's exact bytes; fault
      matrix 41/41 with seven new rows; clean under ASan/UBSan; `pio run -e cairn`
      builds (1,143,904 B, 64.3% of the A slot). 8 mutations each caught
- [x] Spec reconciled with all three implementations (OBD_EXTENDED byte 22,
      `boot_id` on the journal, segment-index rules, `header_len`, §7 vector list)
- [x] CI: vectors path, the protocol rows and the crash-during-commit script
      enrol a device with its escrowed root, a vehicle and an assignment
- [x] The "adaptive rates are never slower during a trip" row had been failing
      since the nominal GNSS period and IMU window were tuned to 200 / 100 ms —
      exactly the policy's hardware floors, leaving EVENT no headroom over CRUISE
      on those axes. The row was stale, not the policy: the floors are now
      exported (`CAIRN_FLOOR_*`) and the row asserts never-below-floor, strictly
      finer wherever there is headroom, and that at least one axis has some
      (mutation-checked both ways). Plain `make` runs through to the BLE and
      mtprobe suites again
- [x] Hardware round trip: capture → seal → offload → relay → decode with encryption on (stage A passes; **stage B passed 2026-10-06**, dev-pairing build) —
      **blocked: the dongle has no SD card in it** (`GO_IDLE_STATE failed` on every
      mount attempt). Runbook: [docs/hardware-roundtrip.md](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/hardware-roundtrip.md)
- [x] The firmware host Makefile now tracks every header as a dependency, so a
      stale `build/` can no longer mask or fake a result

## Phase 22 — Firmware: keys, counter, assignment, secrets off the card — **done on the dongle except BLE auth and the SD round trip**

- [x] `K_root` generated on first boot with the hardware RNG (`esp_fill_random`), stored in NVS — built and host-tested, not run on hardware
- [x] Enrolment blob: public key + `K_root` sealed to the server's enrolment key
      (X25519, HKDF, XChaCha20-Poly1305), signed for proof of possession.
      **Verified on the car's dongle**: the server unsealed the real device's
      blob and escrowed its root; byte-identical to the Go reference. X25519 is
      constant-time and checked against RFC 7748. See
      [docs/device-provisioning.md](https://github.com/ParkWardRR/cairn-vehicle-server/blob/main/docs/device-provisioning.md)
- [x] Device bundle counter in NVS (not on the card). It is **reserved and made
      durable before the first segment header exists** (the counter is in every
      header and authenticated into every frame, so it cannot wait until seal) and
      committed again before the manifest is signed; a bundle that never seals
      leaves a gap the server reports, never a reused counter. Built and
      host-tested
- [x] Resume above the server's counter floor after re-enrolment: delivered
      over the console and applied as "raise only, never lower"
- [x] Assignment installed over the console; segments carry the assigned
      `vehicle_id` / `assignment_id`
- [x] ~~mTLS client key moved off the SD card into NVS~~ — **superseded by Phase 26: the
      dongle has no client key at all.** Earlier firmware's copy is erased from NVS at boot;
      a private key found on the card is still reported and ignored
- [x] ~~Wi-Fi credentials provisioned into NVS~~ — **superseded by Phase 26: the dongle
      has no Wi-Fi.** The fields are refused by the console and erased at boot. NVS is plain
      until Phase 24 turns on flash and NVS encryption together
- [x] Serial provisioning protocol and `cairn-provision`: never during a trip, a
      60 s window after boot once provisioned, idle timeout, atomic two-slot
      credential commit, no secret echoed or logged (canary-tested), 8 KiB UART
      receive buffer (a long PEM line overran the default 256 B on real
      hardware). Non-interactive runs are still checked with
      `--expect-device-id` / `--expect-fingerprint`
- [x] The dongle is provisioned and enrolled: same key id as its 2026-10-01
      enrolment, root escrowed as version 1, assigned to the 428i, credentials in
      NVS, boot log says `credentials are provisioned`
- [x] Encrypted append path integrated into capture/seal/prune; recovery scan
      works key-free and never deletes anything because of an authentication
      failure. A resumed bundle keeps the boot, vehicle, assignment and counter
      from its own headers; with no assignment the device logs `UNASSIGNED` and binds
      to zero ids, which the server refuses (the right failure direction)
- [ ] BLE: enrolled-app challenge–response at session start, per-session write
      counter, device fingerprint exposed for the app to verify
- [x] Host fault matrix rows for encrypted append, torn tail under encryption,
      wrong-key card, counter persistence across power cut
- [ ] On-hardware: capture → seal → upload → decode round trip with encryption on
      (see Phase 21: needs an SD card)

## Phase 23 — Tailscale and deployment hardening — **in progress**

[docs/tailscale-deployment.md](https://github.com/ParkWardRR/cairn-vehicle-server/blob/main/docs/tailscale-deployment.md).

- [ ] `tailscaled` on the VM host (**installed, running, SSH off, DNS untouched;
      the login URL is waiting for approval**), then tag it `tag:cairn-server`
      (needs the tag defined in your ACL first), ACL limited to the owner's phone
      and the app port, Funnel off, no subnet router, device approval on
- [ ] `tailscale serve` fronting the loopback app listener; verified unreachable
      from outside the Tailnet and unauthenticated calls refused
- [x] Keystore master key lives in `/etc/cairn` (root:cairn, outside the data
      directory) and is handed to the services as a systemd credential
- [ ] Backup restore tested with the master key held apart from the data
- [x] The legacy v1 `cairn-ingest` user unit, which silently took `:8443` the moment
      the real server stopped, is disabled
- [ ] systemd unit hardening for the app listener. ~~Rootless Podman and
      read-only rootfs where the Compose stack allows~~ — retired 2026-10-05: the
      Compose stack was removed; services run as systemd units
- [ ] MQTT scoped by device and vehicle; no database port published
- [x] 2026-10-05: repo cleanup. Removed the retired Zig, Gleam, MoonBit, Odin, Mojo and
      Rust trajectory tooling, the v1 Go ingest and its `/api/v1` API, the Compose/Podman stack
      and its smoke test, the emulator's v1 capture path and its v1 fixtures, the v1
      firmware sources (the vendored `firmware/cairn-v2/third_party/freematics-base/lib` drivers stay),
      and the v1 roadmap archive; flattened the migrations to
      `deploy/migrations/001_raw.sql` … `005_vehicle.sql`. What remains: C firmware, Go
      server, Rust emulator, Nuxt UI, SQL and shell/systemd deploy

## Phase 24 — ESP32 chip hardening — **planned, gated**

[docs/esp32-hardening.md](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/esp32-hardening.md). Irreversible steps; do not
start until each prerequisite below is checked.

- [ ] Prerequisite: OTA install, rollback and power-loss-during-update exercised on
      hardware
- [ ] Prerequisite: build migrated to `framework = arduino, espidf` with a
      checked-in `sdkconfig.defaults`
- [ ] Prerequisite: chip revision read off the real units (secure boot V1 vs V2)
- [ ] Sacrificial ONE+: secure boot + flash encryption in development mode, full
      verification checklist
- [ ] Release mode on the spare, then the car's unit; JTAG and UART ROM download
      disabled; signing key backed up offline
- [ ] Signed-OTA-only update path confirmed in the field

## Phase 25 — iOS companion adoption — **issues filed 2026-10-05**

Tracked in the companion repository
([ParkWardRR/cairn-companion-ios-esp32-obd2-gps-ble](https://github.com/ParkWardRR/cairn-companion-ios-esp32-obd2-gps-ble)):
tracking issue [#13](https://github.com/ParkWardRR/cairn-companion-ios-esp32-obd2-gps-ble/issues/13)
and work items #1–#12, each filed against the contracts in this repo
([app-sync-protocol](contracts/sync/v1/spec.md) carries the published test vectors). The BLE
authentication item (#9) is blocked on Phase 22.

- [ ] Replace the unauthenticated snapshot URL with an enrolled client
- [ ] Vehicle model and selected-vehicle state
- [ ] Encrypted local store, durable outbox, `SyncEngine`
- [ ] Local-first / Tailnet-fallback endpoint selection
- [ ] BLE session authentication against the new firmware

## Phase 26 — No Wi-Fi: the phone is the uplink — **superseded**

> **Superseded the same day by the owner's decision that the dongle gets Wi-Fi and LTE back** ([#19](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/19)). The BLE offload below stays as one of three paths. What follows is the record of the decision it replaced.

**Decision, 2026-10-05:** the dongle has no Wi-Fi, no TLS client and no network
credentials. Every byte that reaches the server goes through the enrolled iOS app:
the phone pulls sealed bundles over BLE, uploads them, and hands the server's signed
receipt back. Why it is safe: the phone carries ciphertext only and can fail to
upload but cannot read a trip, forge a receipt or make the dongle delete anything
([contracts/ble/v1/offload.md](contracts/ble/v1/offload.md) §1, [trust-model-v3](docs/trust-model-v3.md) §0.1).
What it costs: trips reach the server only when the phone offloads them, so the dongle
must be awake and in range after a drive.

Firmware ([firmware #1](https://github.com/ParkWardRR/cairn-esp32-device-firmware/issues/1)):

- [x] Wi-Fi, HTTP upload, TLS client and HTTP OTA fetch removed; flash image about 1.1 MB → 0.45 MB.
      Receipt verification and receipt-gated pruning are untouched (`cairn_prune.c`)
- [x] Provisioning shrinks to the assignment and counter floor; `SET wifi_*` / `client_*` are refused;
      legacy Wi-Fi and key slots are erased from NVS at boot (host-tested, mutation-checked)
- [x] `secrets.h` holds trust anchors only (enrolment key, receipt key, BLE passkey)
- [x] Host suite green: storage matrix 41/41, format vectors 33/33, provisioning 12/12
- [x] Flashed on the real dongle: the key and certificate were physically overwritten in flash; a residual Wi-Fi password survived (documented in device-provisioning.md; flash encryption is the real fix)
- [x] BLE offload (`lib/cairn_offload` + NimBLE shim): `LIST`, `GET_MANIFEST`, `READ`, `PUT_RECEIPT`, `ABORT`, `TRIP_ACTIVE` refusal, standby held while a phone works. 22 host rows, ASan clean, nine mutations of the receipt gate and checks caught
- [x] Golden vectors `contracts/ble/v1/vectors/offload/`, generated from the firmware's own module; a test fails if they drift
- [x] Flash the offload build (2026-10-06, receipt key pinned)
- [x] Run `cairn-phone offload` against the dongle to completion (2026-10-06): **nine bundles in 62 s, nine receipts verified against the pinned key on the device, nine bundles pruned**. The flashed build uses dev pairing (Just Works, no MITM, bonds cleared each boot, bench-mode auto-detect) to unblock macOS CoreBluetooth; see the firmware README caveat
- [ ] Enrolled-app challenge–response on the BLE link (hardening; not a prerequisite)
- [ ] Restore shipping pairing (MITM + DISPLAY_ONLY + `READ_AUTHEN`/`WRITE_AUTHEN`) once the iPhone client is wired and the macOS pairing failure is understood (not just worked around)

Server ([server #4](https://github.com/ParkWardRR/cairn-vehicle-server/issues/4), [#7](https://github.com/ParkWardRR/cairn-driving-log-selfhosted/issues/7)):

- [x] Relay contract specified ([app-sync-protocol](contracts/sync/v1/spec.md) §13)
- [x] `/v1/relay/bundles/*` on the app API: offer returns offset/length per missing chunk; signed or bearer; scoped by the manifest's vehicle on every step; reuses the intake core. **Deployed to the VM**
- [x] End-to-end test with no radio: the phone client, the firmware's own protocol module (host-built as `offload-sim`) and the server's real relay, including lost and corrupted notifications, a wrongly pinned dongle, an out-of-scope phone and SD bit rot
- [ ] Emulator matrix drives the relay path
- [ ] Retire the `:8443` device listener, device certificates and `/api/v2/firmware/*` once the relay is proven on hardware

iOS ([#14](https://github.com/ParkWardRR/cairn-companion-ios-esp32-obd2-gps-ble/issues/14)):

- [ ] BLE offload client, durable per-bundle state machine, background operation (CoreBluetooth state restoration, background `URLSession`). `server/internal/offloadclient` and `cmd/cairn-phone` are a working reference in Go

The **hardware round trip** ([docs/hardware-roundtrip.md](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/hardware-roundtrip.md) stage B) — capture → seal → BLE offload → relay → receipt → prune on the real dongle — **completed 2026-10-06** (dev-pairing build). Stage B with the shipping pairing is the next hardening milestone.

## Deferred deliberately

| Deferred | Reason |
|---|---|
| A v1 reader or migration | v1 is dead. Not deferred — refused |
| Persisting the analytical store | Rebuild is ~60 ms. Revisit only if a rebuild exceeds ~5 s |
| InfluxDB / QuestDB | Evaluated and rejected for this data and this VM; see Decisions |
| ~~Arbitrary user plugins~~ | Retired 2026-10-05 along with the MoonBit plugin system; not planned. Would need the raw format, decoder ABI, receipt semantics and replay tooling to be stable first |
| MinIO | A CAS directory on ZFS is sufficient; keep the abstraction, skip the daemon |
| TimescaleDB | Native range partitioning first; adopt only on measured need. Analytical reads go to the in-memory store, which lowers that need further |
| `previous_bundle_root` enforcement | Field is populated in v2; detecting deleted historical bundles is a different threat model from detecting corruption |
| ESP32 deep sleep | Not possible on this board: the IMU interrupt is not routed to an RTC-capable GPIO, so there is no wake-on-motion source. The official Freematics firmware polls for the same reason. Timer-wake deep sleep would reset on every wake, and this firmware's boot mounts the card and runs a recovery scan — more costly than the polling it would replace. Standby instead powers peripherals down, clocks the CPU to 80 MHz and light-sleeps between polls |

---

# Completed — Cairn v2 core data path (Phases 1–12)

## Why it was rebuilt

**Complete, and running on hardware as of 2026-10-01.** Two architecture reviews
concluded that v1's design is sound but only correct on the happy path, and
asked for the system to become *auditable across failures*. Cross-checking the
reviews against the code found three of their recommendations were live defects
rather than future concerns:

| Defect | Evidence |
|---|---|
| Only `samples.bin` is uploaded — OBD, IMU-summary, health and event data is silently discarded on every sync | `firmware/.../state_machine.cpp:1184`; the server falls back to `ParseRawSamples` |
| Pruning is not receipt-gated. Retention is measured with `millis()`, which resets on every deep sleep; the Ed25519 signature is never verified and the receipt is never persisted | `state_machine.cpp:510`, `:1312` |
| Transport is plain HTTP on port 8080, not the mTLS on 8443 that the docs claim | `config.h:69` |

Rather than remediate in place, the core data path is being **rebuilt from a
clean slate**: no format compatibility, no migration, no reissuing of historical
receipts. The v1 plan is dead and archived in
the v1 roadmap (removed 2026-10-05; see git history).

## Build order

Server and emulator first, so the device targets a server already proven
against the fault matrix.

### Phase 1 — Bundle format v2 — **complete**

- [x] `docs/bundle-format-v2.md` (since superseded by [bundle-format-v3.md](contracts/format/v3/spec.md)) as a normative spec with byte layouts
- [x] Go reference implementation (`server/format/`): encode, decode, verify, recover
- [x] Framed records: `boot_id`, monotonic `seq`, CRC-32, `prev_crc32` chain
- [x] Deterministic-CBOR manifest, Ed25519-signed over a specified encoding
- [x] Three distinct identifiers: `bundle_id` (ULID), `content_root` (Merkle root
      over members), `transfer_hash` (transport integrity only)
- [x] Nine payload schemas carrying fix type, HDOP, accuracy, satellites and
      source flags — never inferring precision the receiver did not report
- [x] UTC as an estimate with its own uncertainty, separate from the ordering key
- [x] Explicit `GNSS_GAP` record so absence is recorded, never interpolated
- [x] 25 conformance vectors in `fixtures/format-v2/`, generated deterministically
- [x] Conformance runner; 60 test cases green

### Phase 2 — Server: raw-first ingest — **complete**

- [x] mTLS listener on 8443: private CA, per-device client certificates, denylist
- [x] Manifest-first upload; server replies with missing chunk indices only
- [x] Content-addressed chunk acceptance — by hash, never by byte offset
- [x] Content-addressed raw store, kept object-like for a later MinIO swap
- [x] Durable signed receipt, persisted **before** it is returned
- [x] Persistent signing key; refuses to start with an ephemeral key outside dev mode
- [x] Idempotency keyed on `content_root`, not on connection or request ID
- [x] Durable ingest outbox; ingest returns once the raw commit is durable
- [x] Per-device rate limits and storage quotas
- [x] Transport identity bound to the manifest's claimed device identity
- [x] Revocation effective immediately, with no restart
- [x] Verified end to end over real mTLS with `cmd/cairn-syncdemo`

### Phase 3 — Emulator and fault injection — **complete**

- [x] Rust implementation of format v2 (`emulator/src/format/`), **22/22
      committed vectors pass** — byte-identical to the Go reference, including
      the strict manifest canonical-encoding digest
- [x] Fault-injection layer with named interrupt points (`emulator/src/v2/fault.rs`)
- [x] Device lifecycle model: framed capture, atomic seal, boot recovery,
      manifest-first sync, local receipt verification, transactional prune
- [x] Power cut at each capture frame boundary and each sealing step
- [x] Torn write: a partial frame on the medium, at three truncation depths
- [x] Network loss after each upload chunk boundary
- [x] Reboot after receipt, after prune intent, and after payload delete
- [x] Corrupt chunk in transit; duplicate upload; receipt lost in transit
- [x] Clock reset / GNSS jump; corrupted storage bytes
- [x] Server crash during commit, including a signing-key rotation case
      (`tests/server-crash-during-commit.sh`)
- [x] Reproducible from a seed, printed on failure; assertions against the
      device's durable state and the server's reported state, never stdout
- [x] **16 matrix rows pass, 0 skipped.** CI gates conformance and the matrix
- [x] Decoder upgrade reproducibility — closed by Phase 4's decode pipeline
- [ ] Storage full — needs Phase 9's degraded modes. ~~Plugin timeout~~ — retired
      2026-10-05 with the plugin system
- [x] **Standing rule: every later phase adds its own matrix rows before it closes**

#### What the matrix proves

| Row | Property |
|---|---|
| `power-cut-during-capture` | Every frame durably written is recoverable; only an incomplete tail is lost, and the bundle is not marked sealed |
| `power-cut-during-seal` | A sealing failure never discards captured data — all 25 capture frames and 4 journal frames survive a cut at each step |
| `torn-tail` | A partial frame is isolated; the 9 preceding frames survive and the discarded byte count is exact |
| `corrupted-storage-bytes` | A single flipped bit is detected at its own frame; earlier frames remain usable |
| `clock-reset-gnss-jump` | A 30 s backwards UTC jump leaves all frames in sequence, flagged `ESTIMATED_UTC` with accuracy unknown, monotonic time still advancing |
| `prune-without-receipt` | No verified receipt ⇒ never pruned; the payload stays fully intact |
| `reboot-after-receipt-before-prune` | Payload and receipt both survive; the prune simply happens later |
| `reboot-mid-prune` | Fully present or fully pruned; the journal explains which, and the receipt outlives the payload |
| `network-loss-per-chunk` | The device resumes rather than restarts and eventually earns a receipt |
| `corrupt-chunk-in-transit` | Rejected by the server; the retry succeeds with no operator action |
| `duplicate-upload` | Same receipt, zero chunks re-sent, decode backlog unchanged |
| `receipt-lost-in-transit` | A retry returns the already-committed receipt rather than minting a second |
| server crash during commit | Un-receipted or durably recoverable; a retry always converges to exactly one receipt |
| signing-key rotation | A rotated key cannot induce a prune — the device refuses the receipt |

#### What the harness found

| Finding | Significance |
|---|---|
| A clock-jump test that edited a frame after the fact was rejected as `CHAIN_BREAK` | The `prev_crc32` chain works. A jumped sample must be *written during capture*, not patched in afterwards |
| The crash test, run with `-dev`, lost receipt verification across a restart | The ephemeral signing key had rotated — independently reproducing the hazard `receipts.Open` refuses by default. It became a deliberate matrix case |
| A `FaultPoint` variant declared but never constructed | A missing matrix row (the window between verifying a receipt and beginning the prune), not dead code |

### Phase 4 — Schema and decode workers — **complete**

- [x] Fresh migrations in `deploy/migrations/` (originally `v2/`, flattened to `001_raw.sql` … `005_vehicle.sql` on 2026-10-05) — raw / normalized / derived
      layers, no `ALTER`s against the v1 schema
- [x] Range-partitioned sample tables with on-demand monthly partitions and a
      default partition, so a decode can never fail for want of one
- [x] Unique `(device_id, content_root)`; `bundle_id` unique as the primary key,
      and a reused ID with different content is **reported**, not swallowed
- [x] `geom` as a stored generated column, so the geometry cannot drift from the
      coordinates it represents
- [x] Decode worker (`cmd/cairn-worker`) consuming the outbox, in its own process
- [x] Reprocessing as a queued job: `-reprocess <root>` and `-reprocess-all`
- [x] Derived trip builder with drive/stop/gap segmentation, event detector and
      daily rollups
- [x] MQTT publishes semantic idempotent state only — verified against a real
      Mosquitto broker
- [x] Explicit retention policy table
- [x] 11 database integration tests against real PostGIS; 172 Go tests green

#### The two properties that matter

| Property | How it is enforced |
|---|---|
| **Idempotent** | Every write either upserts on a deterministic key or deletes the bundle's rows before reinserting, all in one transaction. A redelivered job costs a repeat, never a duplicate — asserted by re-decoding four times and comparing row counts |
| **Reproducible** | `derived.decode_runs.output_digest` hashes the derived output. Re-decoding at the same version must give the same digest; 20 consecutive decodes verified identical. A decoder upgrade appears as a second row at a higher version, so comparing digests shows exactly which bundles a change altered |

#### Failure isolation, verified

- A sync completes and yields a verifiable receipt **with the database closed** —
  ingest has no database dependency, so a Postgres outage delays the derived
  view and nothing more
- A decode failure leaves the job unacknowledged, the receipt untouched and
  every raw member retrievable. A parked job is never dropped: the raw bundle is
  intact, so a fixed decoder can pick it up later
- A decoder upgrade re-derives from raw with no device re-uploading anything

### Phase 5 — Firmware: project, partitions and logging — **complete**

- [x] Project at `firmware/cairn-v2/` on espressif32 7.1.3. The Arduino core on
      7.x *is* an ESP-IDF 5.x component bundle, so `framework = arduino` already
      exposes `esp_ota_ops`, `nvs_flash`, `esp_sleep` and `esp_rom_crc` — the
      substance of "IDF with arduino-esp32 as a component" without a separate
      multi-gigabyte framework download, and the vendored FreematicsPlus drivers
      keep working unchanged
- [x] **A/B OTA slots from the first commit** (`partitions-ab.csv`), replacing
      `huge_app.csv`: two 1.69 MB app slots, `otadata`, `nvs_keys`, `errlog` and
      `coredump`
- [x] NVS holds the device id (derived from the efuse MAC, so wiping NVS does not
      change which vehicle the data came from), the signing seed and the boot
      count
- [x] Boot self-test (`env:cairn-selftest`) covering CRC-32 through the ROM path,
      SHA-256, Ed25519 sign/verify/tamper-reject, a framed write to the card, a
      seal, and a live sync
- [x] Real `esp_sleep_get_wakeup_cause()`, recorded in every `STATE_TRANSITION`
- [x] Verbose SD logging: one file per boot, RAM-buffered before the card mounts
      so a mount failure is itself diagnosable, WARN/ERROR flushed immediately,
      capped at 16 MiB oldest-first and suspended below 64 MiB free — a testing
      aid must not be able to cost a trip
- [ ] Flash encryption — **deliberately not enabled.** Turning it on in release
      mode is irreversible, which is a poor property for a device already wired
      into a car. The partition exists so enabling it later needs no repartition

### Phase 6 — Firmware: capture lifecycle — **complete**

- [x] Four independent regions — capture, bundle, connectivity, health — rather
      than one flat enum. Independence is the point: losing the network must not
      end a trip, and a degraded sensor must not stop capture
- [x] Every transition journalled with trigger, reason code and the **policy
      version in force**, so a decision in the data stays explainable after the
      thresholds change
- [x] Confidence-scored evidence combining IMU RMS, OBD speed and GNSS speed,
      with separate start/stop thresholds and dwells — stopping requires the
      *absence* of evidence, which is weaker than its presence
- [x] GNSS gaps recorded as `GNSS_GAP` with duration and missed-sample count —
      silence in the data would be indistinguishable from the device being off
- [x] **45 s pre-roll ring** (`src/preroll.c`), 128 slots sized against the
      combined GNSS/IMU/OBD record rate. Records captured before a trip is
      confirmed are held and flushed with `CAIRN_FLAG_PRETRIP`; if the motion
      does not persist the ring is dropped, so a parked car produces nothing
      while a real drive still recovers its first seconds
- [x] **One transition controller owns all state; sensing only reports facts**
      (`src/facts.h`, `src/sensor_task.cpp`). This is a correctness fix, not a
      structural preference: the IMU used to be read in the same loop pass that
      wrote frames, so during a 30 ms card write no samples were taken and the
      RMS and peak a window reported were computed over whatever moments
      happened to miss I/O. Sensing is now pinned to core 0 and its rate no
      longer depends on the controller. The fact queue is bounded, and drops are
      reported in `DEVICE_HEALTH` rather than hidden

> **Correction.** The previous commit marked this phase complete while the
> pre-roll and the controller split were not implemented —
> `CAIRN_PRETRIP_RING_SAMPLES` was defined and never read. Both are now built
> and carry property rows.

### Phase 7 — Firmware: framed storage and recovery — **complete**

- [x] Append-only framed segments per format v2, rotated at 1 MiB, with one chain
      across all `seg-*` files and a separate chain for `journal.seg` (§3.2.1)
- [x] C implementation of format v2 (`lib/cairn_format`, portable C11, no IDF
      dependency) passing **all 25 committed vectors** on the host, clean under
      ASan and UBSan
- [x] Ed25519 vendored from TweetNaCl because mbedTLS has no Ed25519 signing, and
      checked *two* ways: verification against a Go-produced signature, and
      signing compared byte-for-byte against it. Ed25519 is deterministic, so a
      subtly wrong field implementation cannot survive that
- [x] The recovery scan is streaming (`cairn_scan_segment_stream`), so a segment
      far larger than DRAM is recoverable with one frame resident. The buffer
      entry point is a thin wrapper over it, so the device and the conformance
      vectors drive **one** body of code
- [x] Crash-safe seal: the manifest is written *before* the directory is moved,
      so an interrupted seal leaves either a capture holding a valid manifest —
      finished at the next boot without re-signing — or a completed bundle
- [x] Boot recovery runs automatically, truncates a torn tail to the last valid
      frame, and carries the exact discarded byte count into the manifest with
      `recovery_state` raised
- [x] SD write and recovery error counts surfaced in `DEVICE_HEALTH`
- [x] **A 20-row property matrix** (`test/host/faults.c`). The storage layer is
      portable C over a filesystem abstraction (`lib/cairn_fs`), so the host
      tests tear real files mid-frame and mid-header, flip payload bytes, forge
      receipts and interrupt seals — against exactly the code the device runs,
      not a parallel implementation. Mutation-checked: removing the receipt
      signature check fails 2 rows, skipping the torn-tail truncation fails 3

#### The rows

| Family | Property |
|---|---|
| seal | A clean capture seals; the manifest verifies and its content root recomputes from the members on disk |
| seal | An interrupted seal completes at the next boot without re-signing, and twice changes nothing |
| seal | A capture with no manifest is a live capture, not an interrupted seal |
| seal | A sealed bundle is never overwritten, even by a colliding id |
| recovery | A tear mid-frame truncates to the last valid frame; the discarded count is the surviving prefix, not the bytes that never arrived |
| recovery | A tear mid-header — too few bytes even for a length prefix — is still a torn tail, not a condemned segment |
| recovery | A flipped payload byte isolates to one record; the four frames before it survive and the state is `SALVAGED`, distinct from a clean tail recovery |
| recovery | One chain spans segment rotation; sequence numbers stay contiguous across the boundary |
| recovery | The journal chain is independent — four journal writes do not appear as a gap in the capture sequence |
| prune | A receipt signed by an untrusted key deletes nothing |
| prune | A genuine receipt for *different* content deletes nothing |
| prune | A malformed receipt deletes nothing |
| prune | No pinned key, or the all-zero placeholder, deletes nothing |
| prune | A verified receipt deletes the bundle, keeps the receipt and clears the intent |
| prune | An interrupted prune resumes; an intent with no receipt keeps the data and clears the intent |
| preroll | Nothing is written while the trip is unconfirmed |
| preroll | Flush writes in observation order, every frame flagged `PRETRIP`, and the flag does not leak past confirmation |
| preroll | A wrapped ring keeps the newest window and counts what it dropped |
| preroll | An oversized payload is refused rather than truncated into a different observation |

### Phase 8 — Firmware: sync and receipt-gated prune — **complete**

- [x] Manifest-first offer, chunks addressed **by hash** rather than byte offset,
      streamed from the card so a 256 KiB chunk never needs to fit in DRAM
- [x] Verify the receipt signature against a **pinned** server key **and** that
      its `content_root` matches what was uploaded. A valid signature over a
      different bundle is not an acknowledgement of this one
- [x] Receipts persisted outside the bundle directory, and stored *before* being
      acted on — the receipt is the durable evidence, the bundle bytes are not
- [x] Transactional prune: `prune_intent` → delete → completion, replayed at boot
- [x] An unconfigured key prunes **nothing**. A full card loses nothing; a
      wrongly authorized prune loses a trip permanently
- [x] mTLS with the private CA pinned on-device. The CA is compiled into
      firmware because it is the trust anchor — one read from the card could be
      swapped by anyone holding the card — while the client certificate and key
      live on the card, since they are rotatable and the certificate's
      CommonName must be the device id, which is not known until the hardware
      has booted once. Falls back to HTTP with an explicit error naming what is
      missing; it never silently downgrades.
      `deploy/make-certs.sh` issues the chain and emits the CA as a C literal.
      Verified end-to-end against the real server: with a client certificate the
      request succeeds and the server reads the CN as the device id, without one
      the handshake is refused. Running it that way found a defect the script
      would otherwise have shipped — macOS LibreSSL defaults to SHA-1, which Go
      rejects as `insecure algorithm ECDSA-SHA1` while sending the client a
      misleading `unknown ca` alert

> **Not yet run on hardware.** Phases 5–8 compile for the target (62.7% of the
> A slot, 25.2% RAM) and the format agrees byte-for-byte with the Go and Rust
> implementations on the host. That rules out a large class of bugs but not
> driver or timing problems. See
> [docs/v2-firmware-testing.md](https://github.com/ParkWardRR/cairn-esp32-device-firmware/blob/main/docs/v2-firmware-testing.md) for the bench
> procedure, including the destructive tests that are the actual point.

### Phase 9 — Degraded states and policy tuning — **in progress**

- [x] `DEGRADED_GNSS` / `_STORAGE` / `_TIME` / `_NETWORK`, `LOW_POWER`,
      `RECOVERY_REQUIRED`, `DEGRADED_SENSING`, defined as a **bitmap** in
      spec §4.10 and implemented across firmware and Go

  The spec already declared `DEVICE_HEALTH.health_state` as a "bitmap of active
  degraded states (§4.7)" — but §4.7 is `STATE_TRANSITION` and never defined
  one, so the cross-reference was broken and the bitmap existed nowhere. The
  firmware was writing a scalar `0/1/2` into a field every decoder would read as
  a bitmap.

  A bitmap rather than a severity is the whole point: degradation is not
  ordered. A low battery, a missing fix and a full card are different problems
  with different fixes, and a scalar forces a priority between them and
  discards the rest — the old code reported `Critical` for the battery while
  silently losing the fact that position was unavailable too.

  Each bit is set from observed conditions, never inference. `DEGRADED_TIME` is
  set before the first fix of every trip, which is normal rather than
  exceptional — saying so is what stops a reader treating monotonic-only
  timestamps as UTC. Decoders preserve unknown bits rather than masking them,
  so a bundle from newer firmware stays interpretable.

  Covered by a storage-matrix row asserting the bitmap round-trips through a
  real segment with every bit independently recoverable, four Go unit tests
  including the unknown-bit rule, and `cairn-verify`, which now reports the
  union of conditions a bundle recorded. Verified end-to-end: the C firmware
  writes the bitmap and the Go reference reads it back by name.

- [ ] Event-adaptive sampling across GNSS, IMU and OBD
- [ ] Thresholds tuned on real traces, with the policy version journalled —
      blocked on bench data by design; tuning against guesses would be worse
      than leaving the defaults.
      Real drives and the analytical store (Phase 14) now remove the block; the
      work is scheduled as Phase 17

### Phase 10 — Ledger and documentation — **complete**

- [x] Admin ledger over the bundle lifecycle, every transition with a reason
      (`internal/ledger`, read by `cmd/cairn-ledger`).

  Append-only on disk rather than in PostgreSQL, deliberately: ingest has no
  database dependency — a Postgres outage delays the derived view and nothing
  more — and making the audit trail a database write would quietly give it one.
  A nil ledger disables recording rather than failing an upload, because an
  audit trail with veto power over the data it audits is the wrong shape.

  Every refusal must carry a reason, enforced in `Append` rather than trusted to
  call sites — the entries that need one are written on error paths where it is
  easiest to forget, and a refusal without a reason is the one entry nobody can
  act on. Verified end-to-end: an unenrolled device's 403 now appears as
  `device_unknown` with the reason, which is exactly the ambiguity that made
  "403 means unenrolled, not unreachable" worth documenting.

- [x] Every documented guarantee mapped to the row that verifies it, in
      [docs/guarantee-audit.md](docs/guarantee-audit.md), together with an
      explicit list of what is **not** covered.

  Writing it found two documented guarantees with no test at all. Revocation
  taking effect without a restart had none, and the mTLS certificate binding was
  worse than missing: a comment in the test harness deferred to
  `TestClientIdentityBinding` as though it existed. It did not. Both are now
  written and both are mutation-checked — disabling `refreshIfChanged` fails the
  first, removing the CommonName comparison fails the second.

- [x] Docs brought in line with what v2 actually does: the README describes the
      v2 firmware, the verifier, the ledger and OTA, and the audit records where
      the v1 documents are superseded rather than leaving them to be mistaken
      for current

### Phase 11 — Secure OTA — **complete**

- [x] Signed images verified before swap, with a **separate update key**. The
      receipt key says "this data is safe to delete"; the update key says "this
      code is safe to run". A server compromised enough to issue false receipts
      costs stored trips; one that could also sign firmware owns the device, so
      the update key never lives on the server — `cairn-signfw` signs offline
- [x] The ordering, which is the part that matters:
      verify the descriptor signature **before** downloading (otherwise a
      hostile server can make the device write megabytes into its spare slot on
      demand); hash the image **read back out of flash**, not the bytes as they
      arrived (hashing the download proves the transfer, not the write, and a
      partially programmed slot that hashed correctly in RAM is exactly what
      produces a boot loop); set the boot partition **last**
- [x] Post-boot self-test and automatic rollback. The image marks itself valid
      only after the card mounts and the tree is confirmed — an image that boots
      but cannot reach its storage is not a working image, and letting it mark
      itself valid would strand the device one reboot from working. The firmware
      logs loudly when the running slot differs from the configured boot
      partition, since that is the otherwise-invisible signature of a rollback
- [x] Never updates with unreceipted bundles pending, mid-trip, or on an
      unhealthy supply — including when the supply voltage is *unknown*, because
      updating on the strength of a reading the device could not take is the
      wrong direction. Each condition blocks on its own and is named in the log;
      "blocked" without a reason is unactionable
- [x] Version ordering **refuses rather than guesses**: an unparseable version
      on either side, or a pre-release suffix, blocks the update. Guessing an
      order is how a device installs something older than itself
- [ ] ESP-IDF secure boot — deliberately not enabled, the same reasoning that
      keeps flash encryption off: it is irreversible, which is a poor property
      for hardware already in a vehicle. This verifies the application
      signature, so a physically present attacker can still flash over serial

> `docs/ota.md` records the protocol, the four preconditions and the ordering
> argument. Verified end-to-end on the host: `cairn-signfw` signs a real 1.1 MB
> image, the C implementation verifies that exact descriptor and rejects both a
> tampered signature and a tampered descriptor, and the server serves the
> descriptor with its detached signature plus the image addressed by hash.
> Three property rows cover the preconditions, version ordering and descriptor
> strictness. The on-device install itself is unexercised — it needs hardware.

### Phase 12 — Hardware bring-up and deployment — **complete**

First contact with the real dongle, 2026-10-01. Four defects, none of which a
green test suite could have found, and three of which would have stayed hidden
for a long time.

- [x] Flashed over the Mac's USB (CH340 at 460800; 921600 fails with a message
      that reads like a wiring fault). Old v1 trip data and the 4 MB flash image
      backed up before erasing
- [x] **Fixed: the ESP32 ROM CRC-32 wrapper.** `~esp_rom_crc32_le(~0u, ...)`
      applies the inversions the ROM API documents but drops CRC-32/ISO-HDLC's
      final xorout, returning the raw shift register —
      `CRC-32("123456789")` came back `2dfd2d88` against the required
      `cbf43926`. Internally consistent, so the device never notices: frames
      written with the wrong CRC scan back cleanly. Every trip it recorded would
      have been rejected by the verifier
- [x] **Fixed: sealing overflowed the 8 KB loop-task stack.** Merkle tree, CBOR
      manifest and Ed25519 on one task. The canary panic became a *reboot loop*,
      because boot resumes the open capture and re-appends frames each cycle —
      the boot counter reached 92. Measured headroom is now logged after every
      seal, in production as well as the bench build: 8220 bytes free of 16384,
      so the old default was insufficient rather than marginal
- [x] **Fixed: the standby blocker logged every 20 ms** — 160 KB of a 174 KB
      capture, burying every transition and sync result and rolling the 16 MiB
      card log long before anything useful could be found. Logged on change now;
      same window produces 13.5 KB
- [x] **Fixed: `-dev` minted an ephemeral receipt key** even with a persistent
      seed on disk. The server would sign receipts under a key no device had
      pinned; the device would reject all of them and never prune; nothing on
      either side logged an error
- [x] **Known-answer gate promoted into the real firmware.** CRC-32 and SHA-256
      are checked against the specification at boot and capture is *refused* on
      disagreement. Both primitives are platform-configurable — ROM CRC,
      mbedTLS SHA — so neither is covered by the host conformance run, which
      compiles the portable fallback
- [x] Full loop verified on hardware: capture → seal → offer → commit → receipt
      issued → receipt verified against the pinned key → prune. Server ledger:
      5 entries, **0 refusals or failures**
- [x] Deployed as a hardened systemd unit under a dedicated `cairn` user, with
      mutual TLS on `:8443` against a private CA, replacing a transient
      `systemd-run --user` instance that did not survive a reboot
- [x] `GOAMD64=v3` with a CPU-feature guard — **+15.7%** on segment scanning.
      The primitives do not move because they were never compiled Go: CRC-32
      dispatches to PCLMULQDQ and SHA-256 to SHA-NI, measured at 32 GB/s and
      2 GB/s respectively

> The methodology that paid for itself: check documented claims against the
> code, and mutation-test every new test. The CRC defect is the clearest
> argument for cross-implementation conformance vectors existing at all — it was
> undetectable from inside the device, and the device was the only thing that
> could reveal it.

**Mutual TLS verified on hardware**, same evening, once the card could be moved
to a workstation to receive its client certificate:

```
mTLS ready: CA pinned in firmware, client credentials from the card
protocol: https, host: cairn.example.lan port: 8443 url: /api/v2/bundles/offer
offer 0000000000Y7VB098FJR5E1YQ6: 1 of 1 chunks missing
chunk 1/1 sent (2452 bytes, 5267e4d2..)
pruned 0000000000Y7VB098FJR5E1YQ6 (receipt verified against the pinned key)
sync done: 1 offered, 1 receipted, 1 pruned, 1 chunks sent, 0 receipts rejected
```

All three protocol steps ran over HTTPS on 8443, and the server attributed them
to `device=8777228e` — which it can only know from the TLS client certificate's
CommonName. The identity binding is therefore proven against mbedTLS on the real
device rather than only against the Go test client. Ledger: 17 entries, **0
refusals or failures**.

**Standby and wake-on-motion also verified**, found in the card logs rather than
over serial, which is the point of writing them to the card:

```
[PWR] WARN  entering standby
[SENS] INFO GNSS powered down for standby
[PWR] WARN  woke after 62015 ms and 50 polls: MOTION
[LIFE] WARN resumed after 20383 ms standby (MOTION); 2 standby period(s) totalling 82398 ms
```

A `GNSS_GAP` record was written unprompted when the receiver dropped out across
a standby cycle — honest incompleteness, recorded rather than interpolated.

**Still not verified:** parked current draw. It needs a meter, not a terminal,
and remains the single most useful measurement left in the system.
