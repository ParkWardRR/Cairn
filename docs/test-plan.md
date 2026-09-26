# Test Plan — ELI5 Edition

> How to test Cairn with a Raspberry Pi Zero W and a Freematics ONE+ Model B,
> explained like you're five (but you have a soldering iron).

---

## Your Test Bench

```
┌──────────────────────┐         UART (TX/RX/GND)        ┌─────────────────────┐
│                      │◄───────────────────────────────►│                     │
│  Raspberry Pi Zero W │                                  │  Freematics ONE+    │
│                      │         Wi-Fi (same network)     │  Model B            │
│  - Runs the server   │◄ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ►│                     │
│  - PostgreSQL        │                                  │  - ESP32            │
│  - Ingest service    │                                  │  - GNSS + IMU       │
│  - Web UI            │                                  │  - microSD          │
│  - MQTT broker       │                                  │  - Wi-Fi            │
│                      │                                  │                     │
│  IP: 192.168.X.Y     │                                  │  OBD port or USB    │
│  mDNS: cairn.local   │                                  │  for power          │
└──────────────────────┘                                  └─────────────────────┘
       │
       │ Your laptop browses
       │ http://cairn.local
       ▼
   ┌────────┐
   │ Laptop │
   └────────┘
```

### What each piece does

| Piece | Role in testing | How it connects |
|-------|----------------|-----------------|
| **RPi Zero W** | Pretends to be your homelab server. Runs PostgreSQL, the ingest service, and the web UI. It's on your home Wi-Fi. | Wi-Fi to your router. UART to Freematics. |
| **Freematics ONE+ B** | The actual car device. Records GPS and motion, stores trips on microSD, uploads to the RPi over Wi-Fi. | UART to RPi for debug/flash. Wi-Fi to your router for sync. OBD or USB for power. |
| **Your laptop** | Where you watch logs, browse the web UI, and run CLI tools. | Wi-Fi to same network. SSH into the RPi. |
| **UART cable** | The "debug window" — you can see everything the Freematics is thinking, flash firmware, and send commands. | TX→RX, RX→TX, GND→GND between RPi and Freematics. |

---

## Phase 0 Tests — "Can I even talk to this thing?"

### Test 0.1: UART Hello World

**What you're checking:** The RPi can talk to the Freematics over UART.

**How to do it:**

1. Wire up UART between RPi Zero and Freematics:
   ```
   RPi GPIO 14 (TX) ──► Freematics RX
   RPi GPIO 15 (RX) ◄── Freematics TX
   RPi GND          ──► Freematics GND
   ```
   > **Important:** The RPi Zero uses 3.3V logic. The Freematics ESP32 also uses 3.3V. They're compatible — no level shifter needed.

2. On the RPi, open a serial terminal:
   ```bash
   # Install minicom if you don't have it
   sudo apt install minicom

   # Connect at 115200 baud
   minicom -D /dev/ttyS0 -b 115200
   ```

3. Power on the Freematics (plug into an OBD port or USB power).

4. **Pass:** You see boot messages scrolling. The ESP32 says hello.
   **Fail:** Nothing shows up → check wiring, baud rate, and that the RPi's serial is enabled (`sudo raspi-config` → Interface Options → Serial Port).

### Test 0.2: Firmware Flash from RPi

**What you're checking:** You can flash new firmware to the Freematics from the RPi.

1. Install PlatformIO on the RPi:
   ```bash
   pip3 install platformio
   ```

2. Clone Cairn and build:
   ```bash
   cd ~/Cairn/firmware/freematics-base
   pio run
   ```

3. Flash over serial:
   ```bash
   pio run --target upload --upload-port /dev/ttyS0
   ```
   > You may need to hold the BOOT button on the ESP32 or use the Freematics' auto-reset circuit.

4. **Pass:** Firmware uploads, device reboots, serial shows the Cairn state machine output.
   **Fail:** Upload fails → check if you need to manually put the ESP32 into bootloader mode (hold BOOT, press RESET, release BOOT).

### Test 0.3: RPi Server Stack

**What you're checking:** PostgreSQL, the ingest service, and the API run on the RPi.

1. On the RPi:
   ```bash
   # Install Docker
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker pi

   # Start the stack
   cd ~/Cairn/deploy
   docker compose up -d

   # Check everything is running
   docker compose ps
   ```

2. From your laptop, browse to `http://<rpi-ip>:8080/api/v1/health`

3. **Pass:** You get `{"status": "ok"}` back.
   **Fail:** Check `docker compose logs` for errors. The RPi Zero is slow — give it a minute.

---

## Phase 1 Tests — "Does the hardware actually work?"

### Test 1.1: GNSS Lock

**What you're checking:** The Freematics can get a GPS fix.

1. Put the Freematics near a window or outside (GPS needs sky view).
2. Watch the serial output on the RPi:
   ```bash
   minicom -D /dev/ttyS0 -b 115200
   ```
3. Wait up to 2 minutes for the GNSS to acquire satellites.

4. **Pass:** Serial shows lat/lon coordinates, satellite count ≥ 4, HDOP < 5.
   **Fail (indoors):** Expected. GPS doesn't work well indoors. Move it outside or to a window.
   **Fail (outdoors):** The OBD port location might be blocking the GNSS antenna. Try an extension cable to get the device out from under the dashboard.

### Test 1.2: IMU Activity

**What you're checking:** The accelerometer/gyroscope respond to movement.

1. With serial connected, pick up the Freematics and shake it gently.
2. Look for acceleration values changing in the serial output.

3. **Pass:** Values change when you move it; values are near-zero when it sits still (except gravity on one axis, ~1000 milli-g).
   **Fail:** Check if the IMU is enabled in firmware config.

### Test 1.3: microSD Write

**What you're checking:** Trip data gets written to the SD card.

1. Insert a microSD card (FAT32 formatted) into the Freematics.
2. Power on, let the firmware run for 30 seconds.
3. Power off, pull the SD card, plug it into your laptop.

4. **Pass:** You see trip data files on the card.
   **Fail:** Check SD card format (must be FAT32), check firmware SD init code.

### Test 1.4: Wi-Fi Association

**What you're checking:** The Freematics can join your home Wi-Fi.

1. Configure the firmware with your Wi-Fi credentials (over UART):
   ```
   cairn-provision --ssid "YourNetwork" --psk "YourPassword"
   ```

2. Watch serial output — it should show:
   ```
   [WIFI] Scanning for trusted networks...
   [WIFI] Found SSID "YourNetwork" BSSID AA:BB:CC:DD:EE:FF
   [WIFI] Connecting...
   [WIFI] Connected! IP: 192.168.X.Z
   ```

3. From the RPi, ping the Freematics IP.

4. **Pass:** Ping succeeds. Device is on your network.
   **Fail:** Check SSID/password. Check signal strength — the OBD port might be in a Wi-Fi dead spot (garage walls, etc).

### Test 1.5: Power Draw

**What you're checking:** The device won't drain your car battery while parked.

1. Get a USB power meter (or multimeter in series with power).
2. Measure current in each state:

| State | What to do | Expected |
|-------|-----------|----------|
| Active (GPS + IMU + SD) | Let it run normally | 100-200 mA |
| Wi-Fi upload | Trigger a sync | 150-250 mA |
| Sleep | Wait for it to enter sleep mode | < 20 mA |
| Deep sleep | Leave it idle for 5+ minutes | < 10 mA |

3. **Pass:** Deep sleep current is under 10-15 mA. At 10 mA, a 50Ah car battery would last ~5000 hours (~208 days) — plenty.
   **Fail:** If sleep current is > 20 mA, check that LTE modem and BLE are actually disabled.

---

## Phase 2 Tests — "Does it know when I'm driving?"

### Test 2.1: Desk Shake Test (Fake a Drive)

**What you're checking:** The state machine transitions work without being in a car.

1. With the Freematics on your desk, connected to serial:
2. Shake it steadily for 20 seconds (simulating driving vibration).
3. Watch serial output for: `SLEEP → ARMING → RECORDING`
4. Stop shaking. Wait for the dwell timeout.
5. Watch for: `RECORDING → STOP_CANDIDATE → FINALIZING → QUEUED`

6. **Pass:** All state transitions happen in order. A trip file appears on the SD card.
   **Fail:** Adjust sensitivity thresholds in config. Too sensitive = false starts. Not sensitive enough = missed trips.

### Test 2.2: Real Drive Test

**What you're checking:** It captures a real trip correctly.

1. Install the Freematics in your car's OBD port.
2. Drive for 10-15 minutes, including a 2-minute stop (gas station, etc).
3. Return home.
4. Pull the SD card and inspect on your laptop:
   ```bash
   cd ~/Cairn
   zig/cli/zig-out/bin/tripctl inspect /path/to/trip/
   ```

5. **Pass:**
   - One trip (not fragmented into multiple by the gas stop)
   - Start/end coordinates make sense
   - Duration roughly matches your actual drive time
   - Distance roughly matches your actual route
   - The gas station appears as a STOP_CANDIDATE event that was resolved as "resumed"

6. **Fail (too many trips):** Lower the stop sensitivity — increase `STOP_DWELL_MS`.
   **Fail (no trip):** Increase GNSS/IMU sensitivity — lower thresholds.
   **Fail (garbled coordinates):** Check GNSS antenna reception in that car.

### Test 2.3: Power Yank Test

**What you're checking:** Data survives a power loss.

1. Start a fake drive (shake the device to trigger recording).
2. Wait 30 seconds so some data is written.
3. **Yank the power cable** (simulate stalling, battery disconnect, etc).
4. Power it back on.
5. Check the SD card — the previous data should be recoverable.

6. **Pass:** Previously checkpointed data is intact. Firmware reports recovering an incomplete session on boot.
   **Fail:** Increase checkpoint/fsync frequency. Check that the append-only format handles truncation.

---

## Phase 3 Tests — "Does it sync when I get home?"

### Test 3.1: First Sync

**What you're checking:** The Freematics uploads a trip to the RPi server.

1. Make sure the RPi server stack is running (`docker compose up -d`).
2. Make sure the Freematics has at least one trip in QUEUED state.
3. The Freematics should be in Wi-Fi range of your home network.

4. Watch the RPi ingest logs:
   ```bash
   docker compose logs -f ingest
   ```

5. Watch the Freematics serial output on another terminal.

6. **Pass:** You see:
   - Device: `[SYNC] Initiating upload for trip <id>...`
   - Server: `[INGEST] Received upload init: trip_id=<id>, size=<bytes>`
   - Chunks transferring...
   - Device: `[SYNC] Receipt received. Trip <id> acknowledged.`
   - Server: `[INGEST] Upload finalized. Receipt issued.`

7. Check the database:
   ```bash
   docker compose exec postgres psql -U cairn -c "SELECT id, started_at, distance_m FROM trips;"
   ```

8. **Fail:** Check mTLS certificates. Check that the device knows the server's address. Check firewall rules on the RPi.

### Test 3.2: Interrupted Upload (Wi-Fi Loss)

**What you're checking:** Upload resumes after Wi-Fi drops.

1. Start a sync (trip uploading).
2. Mid-upload, unplug the RPi's Wi-Fi dongle (or move the Freematics out of range).
3. Wait 30 seconds.
4. Restore Wi-Fi.
5. Watch for the upload to resume from where it left off.

6. **Pass:** Upload resumes without re-sending already-transferred chunks. Trip appears exactly once in the database.
   **Fail:** Check resume_offset logic in the ingest service.

### Test 3.3: Duplicate Rejection

**What you're checking:** Re-uploading the same trip doesn't create duplicates.

1. After a successful sync, restart the Freematics (simulating a retry).
2. If it tries to re-upload the same trip (same content_hash), the server should reject it.

3. **Pass:** Server returns "already received" and the trip count stays the same.
   **Fail:** Check content_hash uniqueness constraint in the uploads table.

### Test 3.4: Multi-Day Offline

**What you're checking:** Device holds trips for days without trying to upload over non-home networks.

1. Take the Freematics away from home Wi-Fi (disconnect your router, or take it out of range).
2. Generate 3-5 trips over a couple of days (or simulate with the desk shake test).
3. Bring it back to home Wi-Fi.

4. **Pass:** All trips upload in order, oldest first. No data loss.
   **Fail:** Check the SD card for the queued bundles. Check that the trusted-network policy isn't accidentally matching other networks.

---

## Phase 4 Tests — "Is the data correct?"

### Test 4.1: Bundle Validation

**What you're checking:** The tripctl CLI can validate bundles end-to-end.

```bash
# Generate a synthetic trip
./tripctl generate --duration-minutes 30 --output /tmp/test-trip/

# Validate it
./tripctl validate /tmp/test-trip/
# Should print: "Bundle valid. SHA-256 checksums match. 1842 GNSS samples."

# Tamper with it
echo "corrupted" >> /tmp/test-trip/samples.bin

# Validate again
./tripctl validate /tmp/test-trip/
# Should print: "INVALID. samples.bin hash mismatch."
```

**Pass:** Valid bundles pass. Tampered bundles fail.

### Test 4.2: GPX Export and Visual Check

**What you're checking:** Exported routes look right on a map.

```bash
# Export a real trip as GPX
./tripctl export-gpx /path/to/real-trip/ --output /tmp/drive.gpx

# Open in Google Earth, GPXSee, or upload to gpx.studio
```

**Pass:** The route on the map matches the road you actually drove.
**Fail:** If the route is jagged or jumps around, check GNSS quality. If it's offset, check coordinate encoding (lat/lon × 10^7).

### Test 4.3: Database Integrity

**What you're checking:** All uploaded data is queryable and consistent.

```sql
-- Count trips
SELECT count(*) FROM trips;

-- Check no duplicate content hashes
SELECT content_hash, count(*) FROM uploads GROUP BY content_hash HAVING count(*) > 1;
-- Should return 0 rows

-- Check samples belong to valid trips
SELECT count(*) FROM location_samples ls
  LEFT JOIN trips t ON ls.trip_id = t.id
  WHERE t.id IS NULL;
-- Should return 0

-- Find nearest place to a trip endpoint
SELECT * FROM find_nearest_place(34.0195, -118.4912, 500);
```

---

## Phase 5 Tests — "Does the UI work?"

### Test 5.1: Web UI Smoke Test

1. Browse to `http://cairn.local:8080` from your laptop.
2. Check each view:
   - Today: shows most recent trip
   - Trips: lists all trips with correct dates
   - Trip detail: route on map, timeline, export buttons
   - Places: saved locations appear
   - Device: shows firmware version, last sync time

3. **Pass:** All views load. Data matches what you know about your drives.
   **Fail:** Check API logs. Check that PostGIS queries work on the RPi.

### Test 5.2: Export Everything

```bash
# From the UI, click "Export All"
# Or via API:
curl http://cairn.local:8080/api/v1/export/all -o cairn-export.zip
```

**Pass:** ZIP contains raw bundles, GPX files, CSV data, and JSON trip summaries for every trip.

---

## Integration Tests — "The Big Ones"

### The Full Week Test

The most important test. Just use Cairn normally for a week.

| Day | What to check |
|-----|--------------|
| Mon | Install in car. Drive to work and back. Check 2 trips appear after returning home. |
| Tue | Drive to 3 places (work, store, home). Should be 2-3 trips (store might merge with commute). |
| Wed | Don't drive. Device should stay in deep sleep. Battery draw should be negligible. |
| Thu | Short drive (< 5 min, < 2 km). Should still capture it. |
| Fri | Drive somewhere with a parking garage. GNSS quality event should appear for the garage. |
| Sat | Longer trip (30+ min). Check route quality on the map. |
| Sun | Review the week. Every real drive should be in the dashboard. Zero duplicates. |

### The Road Trip Test

1. Take the Freematics on a day trip (2+ hours away from home).
2. Make multiple stops along the way.
3. The device should **not** try to upload anywhere.
4. Return home.
5. All trips should upload after you park in the garage.

**Pass:** Complete trip record. No WAN upload attempts. All data synced on arrival.

### The Disaster Recovery Test

1. Take a database backup:
   ```bash
   docker compose exec postgres pg_dump -U cairn cairn > backup.sql
   ```

2. Copy all raw bundles from the storage volume.

3. Destroy everything:
   ```bash
   docker compose down -v  # removes volumes too
   ```

4. Restore:
   ```bash
   docker compose up -d
   docker compose exec -T postgres psql -U cairn cairn < backup.sql
   # Restore raw bundles to volume
   ```

5. **Pass:** Browse the UI. All trips are back. Exports work. Device can sync new trips.

---

## Quick Reference: Troubleshooting

| Symptom | First thing to check |
|---------|---------------------|
| No serial output | Wiring (TX↔RX crossed?), baud rate, RPi serial enabled |
| No GPS fix | Sky view, antenna placement, OBD extension cable |
| False trip starts | Lower IMU sensitivity, raise speed threshold |
| Trips fragmenting | Increase STOP_DWELL_MS (try 300000 = 5 min) |
| No Wi-Fi connection | SSID/PSK correct, signal strength, BSSID match |
| Upload fails | Check mTLS certs, server running, firewall rules |
| Duplicate trips | Check content_hash constraint, receipt storage |
| Corrupt data after power loss | Increase fsync frequency, check SD card health |
| RPi too slow | Consider RPi 3/4 for production; Zero is fine for dev |
| High sleep current | Verify LTE disabled, BLE disabled, GNSS off in sleep |

## Test Fixture Cheat Sheet

Don't want to drive around? Use the simulator:

```bash
# Generate 5 fake trips
python3 zig/simulator/simulate_device.py --trips 5 --output fixtures/synthetic/

# Upload them to the server
python3 zig/simulator/simulate_device.py --upload --server http://cairn.local:8443 --trips 3

# Or use tripctl to generate and inspect
./tripctl generate --duration-minutes 45 --output /tmp/test-trip/
./tripctl inspect /tmp/test-trip/
./tripctl validate /tmp/test-trip/
./tripctl export-gpx /tmp/test-trip/ --output /tmp/test.gpx
```

---

## Success Checklist

When all of these pass, you've got a working Cairn:

- [ ] UART communication works between RPi and Freematics
- [ ] Firmware flashes from RPi over serial
- [ ] GNSS gets a fix outdoors
- [ ] IMU responds to motion
- [ ] microSD reads and writes
- [ ] Device joins home Wi-Fi
- [ ] Deep sleep current < 15 mA
- [ ] State machine transitions correctly on desk
- [ ] Real drive produces a valid trip bundle
- [ ] Power loss doesn't corrupt finalized data
- [ ] First sync to RPi succeeds
- [ ] Interrupted upload resumes correctly
- [ ] Duplicate uploads are rejected
- [ ] Multi-day offline trips sync on return
- [ ] tripctl validates bundles correctly
- [ ] GPX export matches actual route
- [ ] Database has no orphaned/duplicate records
- [ ] Web UI shows trips and maps
- [ ] Full week test — every drive captured, zero duplicates
- [ ] Backup and restore works
