#!/usr/bin/env python3
"""Cairn device simulator -- generates realistic trip data bundles.

Generates synthetic trip data for the Cairn car journal project, producing
trip bundles with GNSS samples, IMU summaries, events, and manifests.
Routes are generated in the Santa Monica / West LA area with realistic
acceleration curves, stops, and GNSS noise.

Usage:
    python3 simulate_device.py --trips 5 --output ../fixtures/synthetic/
    python3 simulate_device.py --upload --server http://localhost:8443 --trips 3
    python3 simulate_device.py --trips 3 --seed 42 --simulate-interruptions
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import random
import struct
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

CROCKFORD_BASE32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

# Binary struct formats
GNSS_STRUCT = struct.Struct("<QiiiHHBBHH2s")  # 32 bytes
IMU_STRUCT = struct.Struct("<QHhhhHhHBB")  # 24 bytes

assert GNSS_STRUCT.size == 32, f"GNSS struct size: expected 32, got {GNSS_STRUCT.size}"
assert IMU_STRUCT.size == 24, f"IMU struct size: expected 24, got {IMU_STRUCT.size}"

EARTH_RADIUS_M = 6_371_000

# Santa Monica / West LA waypoints: (name, latitude, longitude)
WAYPOINTS_POOL: List[Tuple[str, float, float]] = [
    ("Santa Monica Pier",       34.0094, -118.4973),
    ("Ocean & Colorado",        34.0137, -118.4976),
    ("Ocean & Wilshire",        34.0195, -118.4912),
    ("Montana & 7th",           34.0302, -118.4928),
    ("Lincoln & Wilshire",      34.0262, -118.4710),
    ("Wilshire & Bundy",        34.0371, -118.4531),
    ("Venice & Lincoln",        33.9970, -118.4592),
    ("Main & Rose",             33.9941, -118.4744),
    ("Olympic & 26th",          34.0225, -118.4770),
    ("PCH & Temescal",          34.0450, -118.5250),
    ("Lincoln & Montana",       34.0305, -118.4750),
    ("Pico & Lincoln",          34.0135, -118.4600),
    ("26th & San Vicente",      34.0345, -118.4720),
    ("Clover Park",             34.0165, -118.4640),
    ("Bergamot Station",        34.0255, -118.4665),
]

# Pre-built route templates (indices into WAYPOINTS_POOL).
# Each template defines an ordered sequence of waypoints that follow
# realistic road paths through Santa Monica / West LA.
ROUTE_TEMPLATES: List[List[int]] = [
    # Pier up Ocean Ave, along Montana Ave, down Lincoln Blvd
    [0, 1, 2, 3, 10, 4],
    # Pier to Venice via Main & Rose, up Lincoln
    [0, 7, 6, 11, 4, 8, 2],
    # Wilshire corridor loop
    [2, 4, 14, 5, 4, 8, 2],
    # Montana loop via Lincoln
    [3, 10, 4, 11, 13, 8, 2, 1, 0],
    # PCH excursion from Ocean & Wilshire
    [2, 1, 0, 9, 3, 12, 10, 4],
    # Short local trip around Clover Park
    [2, 8, 13, 14, 4],
    # Venice Beach to Wilshire & Bundy
    [7, 6, 11, 14, 4, 5],
    # Bergamot to Pier via Olympic and Ocean
    [14, 8, 2, 1, 0, 7],
]

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _log(msg: str) -> None:
    """Print a progress message to stderr."""
    print(msg, file=sys.stderr)


def _clamp_i16(v: int) -> int:
    return max(-32768, min(32767, int(v)))


def _clamp_u16(v: int) -> int:
    return max(0, min(65535, int(v)))


def generate_ulid(rng: random.Random, timestamp_ms: Optional[int] = None) -> str:
    """Generate a ULID-like 26-character identifier.

    First 10 characters encode the millisecond timestamp in Crockford base32.
    Remaining 16 characters are random from the same alphabet.
    """
    if timestamp_ms is None:
        timestamp_ms = int(time.time() * 1000)
    ts = timestamp_ms & 0xFFFFFFFFFFFF  # 48-bit timestamp
    time_chars: List[str] = []
    for i in range(9, -1, -1):
        time_chars.append(CROCKFORD_BASE32[(ts >> (i * 5)) & 0x1F])
    rand_chars = [rng.choice(CROCKFORD_BASE32) for _ in range(16)]
    return "".join(time_chars) + "".join(rand_chars)


def haversine_m(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Return the great-circle distance in meters between two points."""
    lat1_r, lon1_r = math.radians(lat1), math.radians(lon1)
    lat2_r, lon2_r = math.radians(lat2), math.radians(lon2)
    dlat = lat2_r - lat1_r
    dlon = lon2_r - lon1_r
    a = (math.sin(dlat / 2) ** 2
         + math.cos(lat1_r) * math.cos(lat2_r) * math.sin(dlon / 2) ** 2)
    return EARTH_RADIUS_M * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))


def bearing_deg(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Return the initial bearing from point 1 to point 2 in degrees [0, 360)."""
    lat1_r, lon1_r = math.radians(lat1), math.radians(lon1)
    lat2_r, lon2_r = math.radians(lat2), math.radians(lon2)
    dlon = lon2_r - lon1_r
    x = math.sin(dlon) * math.cos(lat2_r)
    y = (math.cos(lat1_r) * math.sin(lat2_r)
         - math.sin(lat1_r) * math.cos(lat2_r) * math.cos(dlon))
    return math.degrees(math.atan2(x, y)) % 360.0


# ---------------------------------------------------------------------------
# RouteGenerator
# ---------------------------------------------------------------------------


class RouteGenerator:
    """Generates waypoint lists forming realistic driving routes."""

    def __init__(self, rng: random.Random) -> None:
        self.rng = rng

    def generate(self) -> List[Tuple[str, float, float, int]]:
        """Return a route as a list of (name, lat, lon, stop_duration_s) tuples.

        Stops of 15-90 seconds are assigned to 2-4 intermediate waypoints,
        simulating traffic lights and stop signs.
        """
        template = self.rng.choice(ROUTE_TEMPLATES)
        waypoints: List[Tuple[str, float, float, int]] = [
            (WAYPOINTS_POOL[i][0], WAYPOINTS_POOL[i][1],
             WAYPOINTS_POOL[i][2], 0)
            for i in template
        ]

        # Assign stops at 2-4 intermediate waypoints (not first or last).
        interior = list(range(1, len(waypoints) - 1))
        if interior:
            num_stops = self.rng.randint(2, min(4, len(interior)))
            stop_indices = self.rng.sample(interior, num_stops)
            for idx in stop_indices:
                name, lat, lon, _ = waypoints[idx]
                duration = self.rng.randint(15, 90)
                waypoints[idx] = (name, lat, lon, duration)

        return waypoints


# ---------------------------------------------------------------------------
# TripGenerator
# ---------------------------------------------------------------------------


class TripGenerator:
    """Creates a full trip with GNSS samples, IMU summaries, events, and manifest."""

    def __init__(self, rng: random.Random, route_generator: RouteGenerator) -> None:
        self.rng = rng
        self.route_gen = route_generator

    def generate(self, start_time: Optional[datetime] = None) -> Dict[str, Any]:
        """Generate a complete trip dict ready for BundleWriter."""
        waypoints = self.route_gen.generate()

        if start_time is None:
            offset_s = self.rng.randint(0, 7 * 86400)
            start_time = datetime.now(timezone.utc) - timedelta(seconds=offset_s)

        trip_id = generate_ulid(self.rng, int(start_time.timestamp() * 1000))

        gnss_samples, events = self._simulate_drive(waypoints, start_time)

        if gnss_samples:
            elapsed_ms = gnss_samples[-1]["timestamp_ms"] - gnss_samples[0]["timestamp_ms"]
            end_time = start_time + timedelta(milliseconds=elapsed_ms)
        else:
            end_time = start_time

        imu_summaries = self._generate_imu(gnss_samples)

        manifest: Dict[str, Any] = {
            "version": 1,
            "trip_id": trip_id,
            "device_id": "cairn-simulator-01",
            "firmware_version": "0.1.0-sim",
            "started_at": start_time.isoformat(),
            "ended_at": end_time.isoformat(),
            "sample_count": {
                "gnss": len(gnss_samples),
                "imu_summary": len(imu_summaries),
                "imu_raw_windows": 0,
            },
            "gnss_rate_hz": 1,
            "imu_rate_hz": 25,
            "schema_version": 1,
            "compression": "none",
            "integrity": {
                "sha256_manifest": False,
                "sha256_samples": "",  # filled by BundleWriter
                "sha256_events": "",   # filled by BundleWriter
            },
        }

        return {
            "trip_id": trip_id,
            "manifest": manifest,
            "gnss_samples": gnss_samples,
            "imu_summaries": imu_summaries,
            "events": events,
        }

    # -- Driving simulation -------------------------------------------------

    def _simulate_drive(
        self,
        waypoints: List[Tuple[str, float, float, int]],
        start_time: datetime,
    ) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
        """Walk along waypoints, producing 1 Hz GNSS samples and events."""
        samples: List[Dict[str, Any]] = []
        events: List[Dict[str, Any]] = []
        start_ms = int(start_time.timestamp() * 1000)
        current_ms = start_ms

        events.append({
            "type": "trip_start",
            "timestamp": start_time.isoformat(),
            "data": {},
        })

        for seg_idx in range(len(waypoints) - 1):
            _, lat1, lon1, _ = waypoints[seg_idx]
            name_to, lat2, lon2, stop_dur = waypoints[seg_idx + 1]

            dist_m = haversine_m(lat1, lon1, lat2, lon2)
            cruise_mps = self.rng.uniform(8.0, 18.0)  # ~29-65 km/h

            seg_samples = self._generate_segment(
                lat1, lon1, lat2, lon2, dist_m, cruise_mps, current_ms,
            )
            samples.extend(seg_samples)

            if seg_samples:
                current_ms = seg_samples[-1]["timestamp_ms"] + 1000

            # Stopped at waypoint (traffic light / stop sign).
            if stop_dur > 0:
                stop_dt = datetime.fromtimestamp(current_ms / 1000, tz=timezone.utc)
                events.append({
                    "type": "stop_candidate",
                    "timestamp": stop_dt.isoformat(),
                    "data": {
                        "resolved": "resumed",
                        "duration_s": stop_dur,
                        "location": name_to,
                    },
                })
                for _ in range(stop_dur):
                    samples.append(self._make_gnss_sample(
                        timestamp_ms=current_ms,
                        lat=lat2 + self.rng.gauss(0, 0.000001),
                        lon=lon2 + self.rng.gauss(0, 0.000001),
                        speed_mps=0.0,
                        heading=0.0,
                    ))
                    current_ms += 1000

            # Occasional GNSS quality degradation (overpass / tunnel).
            if self.rng.random() < 0.15 and seg_idx < len(waypoints) - 2:
                degrade_dt = datetime.fromtimestamp(current_ms / 1000, tz=timezone.utc)
                events.append({
                    "type": "gnss_quality_degraded",
                    "timestamp": degrade_dt.isoformat(),
                    "data": {
                        "reason": "overpass",
                        "duration_s": self.rng.randint(3, 12),
                    },
                })

        end_dt = datetime.fromtimestamp(current_ms / 1000, tz=timezone.utc)
        events.append({
            "type": "trip_end",
            "timestamp": end_dt.isoformat(),
            "data": {},
        })

        return samples, events

    def _generate_segment(
        self,
        lat1: float, lon1: float,
        lat2: float, lon2: float,
        dist_m: float, cruise_mps: float,
        start_ms: int,
    ) -> List[Dict[str, Any]]:
        """Produce 1 Hz GNSS samples along a segment with a trapezoidal speed profile."""
        if dist_m < 1.0:
            return []

        accel = self.rng.uniform(1.5, 2.5)   # m/s^2
        decel = self.rng.uniform(2.0, 3.5)    # m/s^2

        # Distances needed for accel / decel ramps.
        t_accel = cruise_mps / accel
        d_accel = 0.5 * accel * t_accel ** 2
        t_decel = cruise_mps / decel
        d_decel = 0.5 * decel * t_decel ** 2

        if d_accel + d_decel > dist_m:
            # Triangular profile: cap peak speed to what distance allows.
            peak = math.sqrt(dist_m / (0.5 / accel + 0.5 / decel))
            t_accel = peak / accel
            d_accel = 0.5 * accel * t_accel ** 2
            t_decel = peak / decel
            d_decel = dist_m - d_accel
            t_cruise = 0.0
            d_cruise = 0.0
            cruise_mps = peak
        else:
            d_cruise = dist_m - d_accel - d_decel
            t_cruise = d_cruise / cruise_mps

        total_s = t_accel + t_cruise + t_decel
        num_samples = max(1, int(total_s))
        brng = bearing_deg(lat1, lon1, lat2, lon2)

        samples: List[Dict[str, Any]] = []
        for i in range(num_samples):
            t = float(i)

            if t < t_accel:
                speed = accel * t
                d = 0.5 * accel * t ** 2
            elif t < t_accel + t_cruise:
                dt = t - t_accel
                speed = cruise_mps
                d = d_accel + cruise_mps * dt
            else:
                dt = t - t_accel - t_cruise
                speed = max(0.0, cruise_mps - decel * dt)
                d = d_accel + d_cruise + cruise_mps * dt - 0.5 * decel * dt ** 2

            frac = min(1.0, d / dist_m) if dist_m > 0 else 0.0
            lat = lat1 + frac * (lat2 - lat1) + self.rng.gauss(0, 0.000002)
            lon = lon1 + frac * (lon2 - lon1) + self.rng.gauss(0, 0.000002)

            samples.append(self._make_gnss_sample(
                timestamp_ms=start_ms + i * 1000,
                lat=lat,
                lon=lon,
                speed_mps=speed,
                heading=brng + self.rng.gauss(0, 0.5),
            ))

        return samples

    def _make_gnss_sample(
        self,
        timestamp_ms: int,
        lat: float,
        lon: float,
        speed_mps: float,
        heading: float,
    ) -> Dict[str, Any]:
        """Construct a single GNSS sample dict."""
        return {
            "timestamp_ms": timestamp_ms,
            "latitude": int(round(lat * 1e7)),
            "longitude": int(round(lon * 1e7)),
            "altitude_cm": int(3000 + self.rng.gauss(0, 50)),
            "speed_cmps": _clamp_u16(int(speed_mps * 100)),
            "heading_cdeg": _clamp_u16(int(heading * 100) % 36000),
            "fix_quality": 1,
            "satellites": self.rng.randint(8, 12),
            "hdop_tenths": self.rng.randint(8, 15),
            "accuracy_cm": self.rng.randint(200, 500),
        }

    # -- IMU summary generation ---------------------------------------------

    def _generate_imu(
        self, gnss_samples: List[Dict[str, Any]],
    ) -> List[Dict[str, Any]]:
        """Generate IMU summary records at 25 Hz spanning the trip duration."""
        if not gnss_samples:
            return []

        start_ms = gnss_samples[0]["timestamp_ms"]
        end_ms = gnss_samples[-1]["timestamp_ms"]
        window_ms = 40  # 25 Hz

        summaries: List[Dict[str, Any]] = []
        current_ms = start_ms
        gnss_idx = 0

        while current_ms < end_ms:
            # Advance GNSS index to the nearest sample.
            while (gnss_idx < len(gnss_samples) - 1
                   and gnss_samples[gnss_idx + 1]["timestamp_ms"] <= current_ms):
                gnss_idx += 1

            speed_cmps = gnss_samples[gnss_idx]["speed_cmps"]
            is_moving = speed_cmps > 50  # > 0.5 m/s

            if is_moving:
                speed_factor = min(1.0, speed_cmps / 1500.0)
                base_accel = int(50 + speed_factor * 200)
                base_gyro = int(5 + speed_factor * 30)
            else:
                base_accel = self.rng.randint(5, 20)
                base_gyro = self.rng.randint(0, 5)

            summaries.append({
                "window_start_ms": current_ms,
                "window_duration_ms": window_ms,
                "accel_peak_x_mg": _clamp_i16(int(self.rng.gauss(0, base_accel))),
                "accel_peak_y_mg": _clamp_i16(int(self.rng.gauss(0, base_accel))),
                "accel_peak_z_mg": _clamp_i16(int(self.rng.gauss(0, base_accel * 0.5) + 1000)),
                "accel_rms_mg": _clamp_u16(int(base_accel * 0.7 + self.rng.gauss(0, 10))),
                "gyro_peak_dps": _clamp_i16(int(self.rng.gauss(0, base_gyro))),
                "variance": _clamp_u16(int(base_accel * 2 + self.rng.gauss(0, 20))),
                "flags": 0,
            })

            current_ms += window_ms

        return summaries


# ---------------------------------------------------------------------------
# BundleWriter
# ---------------------------------------------------------------------------


class BundleWriter:
    """Writes a trip bundle to disk in the correct binary/JSON format."""

    def __init__(self, output_dir: str) -> None:
        self.output_dir = Path(output_dir)

    def write(
        self,
        trip: Dict[str, Any],
        simulate_interruption: bool = False,
        rng: Optional[random.Random] = None,
    ) -> Path:
        """Write all bundle files and return the trip directory path."""
        trip_dir = self.output_dir / trip["trip_id"]
        trip_dir.mkdir(parents=True, exist_ok=True)

        # -- Pack GNSS samples -----------------------------------------------
        samples_buf = bytearray()
        for s in trip["gnss_samples"]:
            samples_buf.extend(GNSS_STRUCT.pack(
                s["timestamp_ms"],
                s["latitude"],
                s["longitude"],
                s["altitude_cm"],
                s["speed_cmps"],
                s["heading_cdeg"],
                s["fix_quality"],
                s["satellites"],
                s["hdop_tenths"],
                s["accuracy_cm"],
                b"\x00\x00",
            ))

        # -- Simulate power interruption (truncate mid-write) ----------------
        if simulate_interruption and rng and rng.random() < 0.5 and len(samples_buf) > 64:
            full_len = len(samples_buf)
            truncate_at = rng.randint(full_len // 4, full_len - 1)
            _log(f"  [!] Power interruption: truncating samples.bin at "
                 f"byte {truncate_at}/{full_len}")
            samples_buf = samples_buf[:truncate_at]

            partial_state = {
                "trip_id": trip["trip_id"],
                "bytes_written": truncate_at,
                "expected_bytes": full_len,
                "interrupted": True,
            }
            (trip_dir / "upload_state.partial.json").write_text(
                json.dumps(partial_state, indent=2) + "\n"
            )

        samples_path = trip_dir / "samples.bin"
        samples_path.write_bytes(bytes(samples_buf))

        # -- Pack IMU summaries -----------------------------------------------
        imu_buf = bytearray()
        for s in trip["imu_summaries"]:
            imu_buf.extend(IMU_STRUCT.pack(
                s["window_start_ms"],
                s["window_duration_ms"],
                s["accel_peak_x_mg"],
                s["accel_peak_y_mg"],
                s["accel_peak_z_mg"],
                s["accel_rms_mg"],
                s["gyro_peak_dps"],
                s["variance"],
                s["flags"],
                0,  # reserved
            ))
        imu_path = trip_dir / "imu_summary.bin"
        imu_path.write_bytes(bytes(imu_buf))

        # -- Write events.json ------------------------------------------------
        events_bytes = json.dumps(trip["events"], indent=2).encode("utf-8")
        events_path = trip_dir / "events.json"
        events_path.write_bytes(events_bytes)

        # -- Compute integrity hashes -----------------------------------------
        samples_hash = hashlib.sha256(samples_buf).hexdigest()
        events_hash = hashlib.sha256(events_bytes).hexdigest()
        imu_hash = hashlib.sha256(imu_buf).hexdigest()

        trip["manifest"]["integrity"]["sha256_samples"] = samples_hash
        trip["manifest"]["integrity"]["sha256_events"] = events_hash

        # -- Write manifest.json ----------------------------------------------
        manifest_bytes = json.dumps(trip["manifest"], indent=2).encode("utf-8")
        manifest_path = trip_dir / "manifest.json"
        manifest_path.write_bytes(manifest_bytes)
        manifest_hash = hashlib.sha256(manifest_bytes).hexdigest()

        # -- Write sha256sums.txt ---------------------------------------------
        sums = "\n".join([
            f"{manifest_hash}  manifest.json",
            f"{samples_hash}  samples.bin",
            f"{imu_hash}  imu_summary.bin",
            f"{events_hash}  events.json",
        ]) + "\n"
        (trip_dir / "sha256sums.txt").write_text(sums)

        return trip_dir


# ---------------------------------------------------------------------------
# UploadSimulator
# ---------------------------------------------------------------------------


class UploadSimulator:
    """Simulates the chunked upload protocol using urllib."""

    CHUNK_SIZE = 4096

    def __init__(self, server_url: str) -> None:
        self.server_url = server_url.rstrip("/")

    def upload(self, bundle_dir: Path) -> bool:
        """Upload a trip bundle to the server. Returns True on success."""
        bundle_dir = Path(bundle_dir)

        with open(bundle_dir / "manifest.json", "r") as f:
            manifest = json.load(f)

        trip_id = manifest["trip_id"]

        with open(bundle_dir / "samples.bin", "rb") as f:
            data = f.read()

        content_hash = hashlib.sha256(data).hexdigest()
        _log(f"  Uploading trip {trip_id} ({len(data)} bytes)")

        try:
            # Step 1: POST /upload/init
            init_payload = json.dumps({
                "device_id": "cairn-simulator-01",
                "trip_id": trip_id,
                "content_hash": content_hash,
                "size": len(data),
            }).encode("utf-8")

            req = urllib.request.Request(
                f"{self.server_url}/api/v1/upload/init",
                data=init_payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=30) as resp:
                resp_body = json.loads(resp.read().decode("utf-8"))

            upload_id = resp_body["upload_id"]
            resume_offset = resp_body.get("resume_offset", 0)
            _log(f"  Init OK: upload_id={upload_id}, resume_offset={resume_offset}")

            # Step 2: PUT /upload/{id}/chunk in 4096-byte chunks
            offset = resume_offset
            while offset < len(data):
                chunk = data[offset:offset + self.CHUNK_SIZE]
                chunk_req = urllib.request.Request(
                    f"{self.server_url}/api/v1/upload/{upload_id}/chunk",
                    data=chunk,
                    headers={
                        "Content-Type": "application/octet-stream",
                        "X-Upload-Offset": str(offset),
                    },
                    method="PUT",
                )
                with urllib.request.urlopen(chunk_req, timeout=30) as resp:
                    resp.read()

                offset += len(chunk)
                pct = min(100, int(offset / len(data) * 100))
                _log(f"  Chunk sent: {offset}/{len(data)} ({pct}%)")

            # Step 3: POST /upload/{id}/finalize
            finalize_payload = json.dumps({
                "content_hash": content_hash,
            }).encode("utf-8")
            fin_req = urllib.request.Request(
                f"{self.server_url}/api/v1/upload/{upload_id}/finalize",
                data=finalize_payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(fin_req, timeout=30) as resp:
                resp.read()

            _log(f"  Upload finalized: {trip_id}")
            return True

        except urllib.error.URLError as exc:
            _log(f"  Upload failed (connection): {exc}")
            return False
        except OSError as exc:
            _log(f"  Upload failed (OS): {exc}")
            return False

    def upload_bundle(self, bundle_dir: Path) -> bool:
        """Convenience alias for upload()."""
        return self.upload(bundle_dir)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Cairn device simulator -- generate realistic trip data bundles.",
    )
    parser.add_argument(
        "--trips", type=int, default=1,
        help="Number of trips to generate (default: 1)",
    )
    parser.add_argument(
        "--output", type=str, default=".",
        help="Output directory for trip bundles (default: current dir)",
    )
    parser.add_argument(
        "--upload", action="store_true",
        help="Upload generated trips to server after writing",
    )
    parser.add_argument(
        "--server", type=str, default="http://localhost:8443",
        help="Server URL for upload (default: http://localhost:8443)",
    )
    parser.add_argument(
        "--seed", type=int, default=None,
        help="Random seed for deterministic / reproducible output",
    )
    parser.add_argument(
        "--simulate-interruptions", action="store_true",
        help="Randomly simulate power interruptions (truncated files, partial state)",
    )

    args = parser.parse_args()

    rng = random.Random(args.seed)
    route_gen = RouteGenerator(rng)
    trip_gen = TripGenerator(rng, route_gen)
    writer = BundleWriter(args.output)
    uploader = UploadSimulator(args.server) if args.upload else None

    # When a seed is provided, anchor all timestamps to a fixed reference so
    # output is fully deterministic.  Otherwise use wall-clock time.
    if args.seed is not None:
        base_time = datetime(2026, 1, 15, 8, 0, 0, tzinfo=timezone.utc)
    else:
        base_time = datetime.now(timezone.utc)

    _log(f"Generating {args.trips} trip(s) -> {os.path.abspath(args.output)}")
    if args.seed is not None:
        _log(f"Seed: {args.seed}")

    for i in range(args.trips):
        _log(f"\n--- Trip {i + 1}/{args.trips} ---")

        offset_s = rng.randint(0, 7 * 86400)
        start_time = base_time - timedelta(seconds=offset_s)
        trip = trip_gen.generate(start_time=start_time)
        gnss_n = len(trip["gnss_samples"])
        imu_n = len(trip["imu_summaries"])
        evt_n = len(trip["events"])
        _log(f"  ID:           {trip['trip_id']}")
        _log(f"  GNSS samples: {gnss_n}")
        _log(f"  IMU summaries:{imu_n}")
        _log(f"  Events:       {evt_n}")

        duration_s = 0
        if gnss_n > 1:
            first_ms = trip["gnss_samples"][0]["timestamp_ms"]
            last_ms = trip["gnss_samples"][-1]["timestamp_ms"]
            duration_s = (last_ms - first_ms) / 1000
        _log(f"  Duration:     {duration_s:.0f}s ({duration_s / 60:.1f} min)")

        bundle_path = writer.write(
            trip,
            simulate_interruption=args.simulate_interruptions,
            rng=rng,
        )
        _log(f"  Written to:   {bundle_path}")

        if uploader:
            uploader.upload(bundle_path)

    _log(f"\nDone. {args.trips} trip(s) generated.")


if __name__ == "__main__":
    main()
