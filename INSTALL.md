# Installing Cairn v0.1.0

This release ships the **Cairn toolchain** — the offline CLI tools for working
with trip bundles, plus the ESP32 firmware for the Freematics ONE+ Model B.

**What is not in this release:** the ingest server, the PostgreSQL schema, and
the SvelteKit web UI. Those are still moving quickly and will land in a later
release. Everything here works standalone against trip bundles on disk, with no
server required.

---

## What you get

| Binary | What it does |
|--------|-------------|
| `tripctl` | Generate, inspect, validate and export trip bundles (GPX/CSV) |
| `trip-inspector` | Full bundle report: manifest, SHA-256 integrity, GNSS/IMU stats, events |
| `trip-diff` | Compare two bundles — route divergence, timing and speed deltas |
| `trip-replay` | Replay a bundle into an ingest server, with speedup and dry-run |
| `route-density` | SVG heatmap of frequently driven roads |
| `sd-recover` | Scan a microSD for incomplete bundles and rebuild manifests/hashes |
| `cairn-emulator` | Device emulator — 15 driving scenarios, no hardware needed |
| `cairn-trajectory` | Trajectory analysis: similarity, places, segments, anomalies, density |
| `cairn-firmware-v0.1.0-esp32.bin` | ESP32 firmware image (see the caveat below) |

Prebuilt for **macOS arm64** (Apple Silicon) and **Linux x86_64**. Both sets are
built from the same commit that this release is tagged from.

---

## Install the prebuilt binaries

Download the archive for your platform from the
[release page](https://github.com/ParkWardRR/Cairn/releases/tag/v0.1.0), then:

```bash
tar xzf cairn-tools-v0.1.0-macos-arm64.tar.gz     # or -linux-x86_64
cd cairn-tools-v0.1.0-macos-arm64

# Verify the checksums that ship alongside the archive
shasum -a 256 -c SHA256SUMS      # macOS
sha256sum -c SHA256SUMS          # Linux

# Put them on your PATH
sudo install -m 755 tripctl trip-inspector trip-diff trip-replay \
                    route-density sd-recover cairn-emulator cairn-trajectory \
                    /usr/local/bin/
```

### macOS: these binaries are unsigned

Cairn is a personal project with no Apple Developer certificate, so Gatekeeper
will refuse to run the downloaded binaries until you clear the quarantine
attribute:

```bash
xattr -d com.apple.quarantine tripctl trip-inspector trip-diff trip-replay \
                              route-density sd-recover cairn-emulator cairn-trajectory
```

If you would rather not trust an unsigned binary — a reasonable position — build
from source instead. It takes a few minutes and the toolchains are all free.

### Linux: mostly glibc

Built on AlmaLinux 10. `tripctl` is statically linked and will run anywhere,
including Alpine. The other seven are dynamically linked against glibc and will
**not** run on musl distributions — build those from source there.

```
tripctl           statically linked      runs anywhere
everything else   glibc, dynamic         needs a glibc distro
```

---

## Try it without any hardware

The emulator produces byte-accurate bundles, so you can exercise the whole
toolchain before the device arrives:

```bash
# Generate a realistic commute
cairn-emulator --scenario normal_commute --output ./trips --seed 42

# The bundle lands under ./trips/<device-id>/<trip-id>/
BUNDLE=$(find ./trips -mindepth 3 -maxdepth 3 -type d | head -1)

trip-inspector "$BUNDLE"
route-density "$BUNDLE" --output heatmap.svg
cairn-trajectory anomalies --bundle "$BUNDLE"
```

`cairn-emulator --help` lists all 15 scenarios, including the awkward ones —
`power_loss_recording`, `power_loss_upload`, `garage_start`, `low_battery`.

`tripctl` can also synthesise a minimal bundle directly:

```bash
tripctl generate --duration-minutes 5 --output ./bundle
tripctl validate ./bundle
tripctl export-gpx ./bundle --output trip.gpx
```

> **Known gap:** `tripctl generate` does not emit `imu_summary.bin`, and
> `trip-inspector` requires it. Use `cairn-emulator` for bundles you intend to
> inspect, and `tripctl` for checksum/export work.

---

## Firmware

### The shipped binary has no network configuration

`cairn-firmware-v0.1.0-esp32.bin` is built with **empty WiFi credentials** and a
generic `cairn.local` server host, deliberately — so the public release carries
nobody's network details. Flashing it verifies your hardware boots and that
GNSS, the IMU and the microSD all come up, but **it will never sync**. For real
use you need to build from source with your own `secrets.h`.

### Flashing

Requires [esptool](https://github.com/espressif/esptool) and a USB connection to
the device.

```bash
esptool --chip esp32 --port /dev/ttyUSB0 --baud 460800 \
  --before default-reset --after hard-reset \
  write_flash -z --flash-mode dio --flash-freq 40m --flash-size 4MB \
  0x10000 cairn-firmware-v0.1.0-esp32.bin
```

Then watch it boot at 115200 baud. A healthy device prints something like:

```
[IMU] ICM-42627
[GNSS] OK (internal)
[SD] Mounted: 15177 MB total, 1 MB used
[STATE] Initialized -> SLEEP
```

### Building firmware with your own credentials

```bash
cd firmware/freematics-base
cp src/secrets.h.example src/secrets.h
# edit src/secrets.h: your SSID, password, and server hostname
pio run -e freematics
```

`src/secrets.h` is gitignored. `config.h` picks it up via `__has_include`, so
the project still builds without it — which means **a missing or misspelled
value fails open to "no network configured" and still compiles cleanly.** The
only symptom is a device that silently never syncs.

To avoid finding that out after a drive, flash the self-test build first:

```bash
pio run -e freematics-selftest
# flash it, then watch the serial output
```

It joins WiFi, resolves your server, and fetches `/api/v1/health`, printing each
step. On failure it dumps every SSID the 2.4 GHz radio can see, with channel and
RSSI — which is how you catch a typo'd SSID or a 5 GHz-only network (the ESP32
has no 5 GHz radio). Reflash `-e freematics` before driving; the self-test code
is not compiled into the production build.

Two notes worth knowing up front:

- **Prefer a DNS name over an IP** for the server. A DHCP lease will move
  eventually, and a hardcoded IP fails silently in the field.
- **`cairn.local` requires mDNS**, which the ESP32 resolver does not do. Use a
  name your router's DNS actually serves.

---

## Build everything from source

```bash
git clone https://github.com/ParkWardRR/Cairn.git
cd Cairn
```

| Component | Toolchain | Command |
|-----------|-----------|---------|
| `tripctl` | [Zig](https://ziglang.org) 0.16.0 | `cd zig/cli && zig build` → `zig-out/bin/tripctl` |
| Odin tools | [Odin](https://odin-lang.org) dev-2026-09 | `odin build odin/<tool>/ -o:speed` |
| `cairn-emulator` | [Rust](https://rustup.rs) 1.93 | `cd emulator && cargo build --release` |
| `cairn-trajectory` | Rust 1.93 | `cd rust/trajectory && cargo build --release` |
| Firmware | [PlatformIO](https://platformio.org) | `cd firmware/freematics-base && pio run -e freematics` |

Odin needs a system LLVM/clang. On Debian/Ubuntu `apt install llvm clang`, on
RHEL-family `dnf install llvm clang`.

The exact versions above are what the v0.1.0 binaries were built with. Zig in
particular still breaks between minor releases, so 0.16.0 is a real
requirement rather than a suggestion.

---

## Hardware

| Item | Notes |
|------|-------|
| [Freematics ONE+ Model B](https://freematics.com/pages/products/freematics-one-plus/) | ESP32 + GNSS + ICM-42627 IMU + OBD-II + microSD |
| microSD card | FAT32. 16 GB is plenty — a 30-minute trip is roughly 90 KB |
| 2.4 GHz WiFi | The ESP32 has no 5 GHz radio |
| USB cable | For flashing and serial monitoring |

No SIM card. The LTE modem is never initialised — Cairn syncs over your LAN
only, by design.

---

## Where to go next

| Document | What it covers |
|----------|---------------|
| [README](README.md) | Project overview, architecture, API |
| [Flashing and Testing](docs/flashing-and-testing.md) | Hardware profile, bench results, bugs found and fixed |
| [Architecture](docs/architecture.md) | System design and data flow |
| [Trip File Format](docs/trip-file-format.md) | Bundle schema and binary sample encoding |
| [Roadmap](ROADMAP.md) | What is done and what is next |

## License

[Blue Oak Model License 1.0.0](LICENSE).
