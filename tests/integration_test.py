#!/usr/bin/env python3
"""Cairn integration test suite -- exercises the full ingest pipeline.

Runs simulation tests against a live ingest server, verifying uploads,
deduplication, resumable transfers, concurrency, and error handling.

Usage:
    python3 tests/integration_test.py --server http://localhost:8443
    python3 tests/integration_test.py  # defaults to http://localhost:8443
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import random
import shutil
import sys
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

# ---------------------------------------------------------------------------
# Add simulator to import path
# ---------------------------------------------------------------------------

_PROJECT_ROOT = Path(__file__).resolve().parent.parent
_SIMULATOR_DIR = _PROJECT_ROOT / "zig" / "simulator"
sys.path.insert(0, str(_SIMULATOR_DIR))

from simulate_device import (
    BundleWriter,
    RouteGenerator,
    TripGenerator,
    UploadSimulator,
)

# ---------------------------------------------------------------------------
# Global configuration (set from CLI args before tests run)
# ---------------------------------------------------------------------------

SERVER_URL: str = "http://localhost:8443"
CHUNK_SIZE: int = 4096


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _api(path: str) -> str:
    """Build a full API URL."""
    return f"{SERVER_URL.rstrip('/')}{path}"


def _http_json(
    method: str,
    path: str,
    body: Optional[dict] = None,
    raw_body: Optional[bytes] = None,
    headers: Optional[dict] = None,
    timeout: int = 30,
) -> Tuple[int, Any]:
    """Make an HTTP request and return (status_code, parsed_json_or_bytes).

    Returns the response body as parsed JSON when the Content-Type is JSON,
    otherwise as raw bytes. On HTTP errors the status code is still returned
    rather than raising.
    """
    url = _api(path)
    hdrs = dict(headers or {})

    if body is not None:
        data = json.dumps(body).encode("utf-8")
        hdrs.setdefault("Content-Type", "application/json")
    elif raw_body is not None:
        data = raw_body
    else:
        data = None

    req = urllib.request.Request(url, data=data, headers=hdrs, method=method)

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            resp_bytes = resp.read()
            ct = resp.headers.get("Content-Type", "")
            if "json" in ct:
                return resp.status, json.loads(resp_bytes.decode("utf-8"))
            return resp.status, resp_bytes
    except urllib.error.HTTPError as exc:
        resp_bytes = exc.read()
        try:
            return exc.code, json.loads(resp_bytes.decode("utf-8"))
        except (json.JSONDecodeError, UnicodeDecodeError):
            return exc.code, resp_bytes


def _generate_trip(
    seed: Optional[int] = None,
    device_id: str = "cairn-test-device",
    duration_minutes: Optional[int] = None,
) -> Dict[str, Any]:
    """Generate a single trip using the simulator."""
    rng = random.Random(seed)
    route_gen = RouteGenerator(rng)
    trip_gen = TripGenerator(rng, route_gen)

    start_time = datetime(2026, 6, 1, 10, 0, 0, tzinfo=timezone.utc)
    trip = trip_gen.generate(start_time=start_time)
    trip["manifest"]["device_id"] = device_id

    # If a specific duration is requested, extend the trip by repeating
    # the generation with different seeds until we accumulate enough samples.
    if duration_minutes is not None and duration_minutes > 0:
        target_samples = duration_minutes * 60  # 1 Hz
        while len(trip["gnss_samples"]) < target_samples:
            extra_seed = rng.randint(0, 2**31)
            extra_rng = random.Random(extra_seed)
            extra_route = RouteGenerator(extra_rng)
            extra_trip_gen = TripGenerator(extra_rng, extra_route)
            last_ms = trip["gnss_samples"][-1]["timestamp_ms"] + 1000
            extra_start = datetime.fromtimestamp(last_ms / 1000, tz=timezone.utc)
            extra = extra_trip_gen.generate(start_time=extra_start)
            # Rebase timestamps to continue from current trip
            base_offset = last_ms - extra["gnss_samples"][0]["timestamp_ms"]
            for s in extra["gnss_samples"]:
                s["timestamp_ms"] += base_offset
            for s in extra["imu_summaries"]:
                s["window_start_ms"] += base_offset
            trip["gnss_samples"].extend(extra["gnss_samples"])
            trip["imu_summaries"].extend(extra["imu_summaries"])
            trip["events"].extend(extra["events"][1:])  # skip duplicate trip_start

        trip["gnss_samples"] = trip["gnss_samples"][:target_samples]
        trip["manifest"]["sample_count"]["gnss"] = len(trip["gnss_samples"])
        trip["manifest"]["sample_count"]["imu_summary"] = len(trip["imu_summaries"])

    return trip


def _write_bundle(trip: Dict[str, Any], output_dir: str) -> Path:
    """Write a trip bundle to disk and return the bundle directory."""
    writer = BundleWriter(output_dir)
    return writer.write(trip)


def _build_bundle_data(bundle_dir: Path) -> Tuple[bytes, str, dict]:
    """Read bundle data from disk. Returns (samples_bytes, content_hash, manifest)."""
    samples_data = (bundle_dir / "samples.bin").read_bytes()
    content_hash = hashlib.sha256(samples_data).hexdigest()
    with open(bundle_dir / "manifest.json", "r") as f:
        manifest = json.load(f)
    return samples_data, content_hash, manifest


def _upload_init(
    device_id: str,
    trip_id: str,
    content_hash: str,
    size: int,
) -> Tuple[int, Any]:
    """Call POST /api/v1/upload/init."""
    return _http_json("POST", "/api/v1/upload/init", body={
        "device_id": device_id,
        "trip_id": trip_id,
        "content_hash": content_hash,
        "size": size,
    })


def _upload_chunks(
    upload_id: str,
    data: bytes,
    start_offset: int = 0,
    end_offset: Optional[int] = None,
    chunk_size: int = CHUNK_SIZE,
) -> int:
    """Send chunks of data and return the final offset acknowledged by server."""
    if end_offset is None:
        end_offset = len(data)

    offset = start_offset
    while offset < end_offset:
        chunk = data[offset:offset + chunk_size]
        status, resp = _http_json(
            "PUT",
            f"/api/v1/upload/{upload_id}/chunk",
            raw_body=chunk,
            headers={
                "Content-Type": "application/octet-stream",
                "X-Upload-Offset": str(offset),
            },
        )
        if status != 200:
            raise RuntimeError(f"Chunk upload failed at offset {offset}: {status} {resp}")
        offset += len(chunk)

    return offset


def _upload_finalize(upload_id: str, content_hash: str) -> Tuple[int, Any]:
    """Call POST /api/v1/upload/{id}/finalize."""
    return _http_json("POST", f"/api/v1/upload/{upload_id}/finalize", body={
        "content_hash": content_hash,
    })


def _full_upload(
    trip: Dict[str, Any],
    output_dir: str,
    device_id: Optional[str] = None,
) -> Tuple[str, str, dict]:
    """Generate bundle, upload it fully. Returns (upload_id, receipt_id, finalize_resp)."""
    if device_id is not None:
        trip["manifest"]["device_id"] = device_id

    bundle_dir = _write_bundle(trip, output_dir)
    data, content_hash, manifest = _build_bundle_data(bundle_dir)
    dev_id = manifest["device_id"]
    trip_id = manifest["trip_id"]

    status, init_resp = _upload_init(dev_id, trip_id, content_hash, len(data))
    if status != 200:
        raise RuntimeError(f"Init failed: {status} {init_resp}")

    upload_id = init_resp["upload_id"]
    resume_offset = init_resp.get("resume_offset", 0)

    if resume_offset == -1:
        # Already completed
        return upload_id, "", init_resp

    _upload_chunks(upload_id, data, start_offset=resume_offset)

    status, fin_resp = _upload_finalize(upload_id, content_hash)
    if status != 200:
        raise RuntimeError(f"Finalize failed: {status} {fin_resp}")

    return upload_id, fin_resp.get("receipt_id", ""), fin_resp


# ---------------------------------------------------------------------------
# Test Cases
# ---------------------------------------------------------------------------


class CairnIntegrationTests(unittest.TestCase):
    """Integration tests for the Cairn ingest pipeline."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp(prefix="cairn_test_")
        self._start = time.monotonic()

    def tearDown(self):
        elapsed = time.monotonic() - self._start
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        # Print timing alongside standard unittest output
        print(f"  ({elapsed:.2f}s)", file=sys.stderr)

    # -- Connectivity check --------------------------------------------------

    def _check_server(self):
        """Verify the server is reachable before running a test."""
        try:
            status, body = _http_json("GET", "/api/v1/health")
        except Exception as exc:
            self.skipTest(f"Server not reachable at {SERVER_URL}: {exc}")
        self.assertEqual(status, 200, f"Health check failed: {body}")

    # -----------------------------------------------------------------------
    # 1. Normal upload
    # -----------------------------------------------------------------------

    def test_01_normal_upload(self):
        """Generate a trip, upload it, verify server returns a receipt."""
        self._check_server()

        trip = _generate_trip(seed=1001, device_id="test-normal-01")
        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        # Init
        status, init_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        self.assertIn("upload_id", init_resp)
        upload_id = init_resp["upload_id"]
        self.assertEqual(init_resp["resume_offset"], 0)

        # Chunks
        _upload_chunks(upload_id, data)

        # Finalize
        status, fin_resp = _upload_finalize(upload_id, content_hash)
        self.assertEqual(status, 200, f"Finalize failed: {fin_resp}")
        self.assertIn("receipt_id", fin_resp)
        self.assertIn("signature", fin_resp)

        # Health check confirms system is OK after upload
        status, health = _http_json("GET", "/api/v1/health")
        self.assertEqual(status, 200)
        self.assertEqual(health.get("status"), "ok")

    # -----------------------------------------------------------------------
    # 2. Duplicate rejection
    # -----------------------------------------------------------------------

    def test_02_duplicate_rejection(self):
        """Upload the same trip twice; second init should return resume_offset=-1."""
        self._check_server()

        trip = _generate_trip(seed=2001, device_id="test-dedup-01")
        upload_id, receipt_id, fin_resp = _full_upload(trip, self.tmpdir)

        # Now try to upload the exact same data again
        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        status, init_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        self.assertEqual(
            init_resp["resume_offset"], -1,
            "Second upload of same content_hash should return resume_offset=-1",
        )
        # The upload_id should be the same as the original
        self.assertEqual(init_resp["upload_id"], upload_id)

    # -----------------------------------------------------------------------
    # 3. Resumable upload
    # -----------------------------------------------------------------------

    def test_03_resumable_upload(self):
        """Partial upload, disconnect, resume from server-reported offset."""
        self._check_server()

        trip = _generate_trip(seed=3001, device_id="test-resume-01")
        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        # Init
        status, init_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        upload_id = init_resp["upload_id"]

        # Send only the first 2 chunks
        partial_end = min(2 * CHUNK_SIZE, len(data))
        _upload_chunks(upload_id, data, start_offset=0, end_offset=partial_end)

        # "Disconnect" -- re-init with same content_hash
        status, resume_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        self.assertEqual(resume_resp["upload_id"], upload_id)

        resume_offset = resume_resp["resume_offset"]
        self.assertGreater(
            resume_offset, 0,
            "Server should report a nonzero resume_offset after partial upload",
        )
        self.assertNotEqual(
            resume_offset, -1,
            "Upload should not be completed yet",
        )

        # Send remaining data from the resume offset
        _upload_chunks(upload_id, data, start_offset=resume_offset)

        # Finalize
        status, fin_resp = _upload_finalize(upload_id, content_hash)
        self.assertEqual(status, 200, f"Finalize after resume failed: {fin_resp}")
        self.assertIn("receipt_id", fin_resp)

    # -----------------------------------------------------------------------
    # 4. Multiple devices
    # -----------------------------------------------------------------------

    def test_04_multiple_devices(self):
        """Upload trips from 3 different devices, verify each is registered."""
        self._check_server()

        device_ids = [
            "test-multidev-alpha",
            "test-multidev-beta",
            "test-multidev-gamma",
        ]
        upload_ids = []

        for i, dev_id in enumerate(device_ids):
            trip = _generate_trip(seed=4000 + i, device_id=dev_id)
            uid, _, _ = _full_upload(trip, self.tmpdir, device_id=dev_id)
            upload_ids.append(uid)

        # Verify all 3 devices are registered
        for dev_id in device_ids:
            status, resp = _http_json("GET", f"/api/v1/devices/{dev_id}/status")
            self.assertEqual(
                status, 200,
                f"Device {dev_id} should be registered: {resp}",
            )
            self.assertEqual(resp["device_id"], dev_id)

        # Verify we got 3 distinct upload IDs
        self.assertEqual(len(set(upload_ids)), 3, "Each device should have a distinct upload")

    # -----------------------------------------------------------------------
    # 5. Large trip
    # -----------------------------------------------------------------------

    def test_05_large_trip(self):
        """Generate a 60-minute trip (~3600 GNSS samples), upload in chunks."""
        self._check_server()

        trip = _generate_trip(seed=5001, device_id="test-large-01", duration_minutes=60)
        self.assertGreaterEqual(
            len(trip["gnss_samples"]), 3500,
            "Large trip should have at least 3500 GNSS samples",
        )

        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        # Verify bundle is a meaningful size (32 bytes per GNSS sample * 3600 ~= 115 KB)
        self.assertGreater(len(data), 100_000, "Large trip bundle should be > 100 KB")

        # Init
        status, init_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        upload_id = init_resp["upload_id"]

        # Upload all chunks
        _upload_chunks(upload_id, data)

        # Finalize
        status, fin_resp = _upload_finalize(upload_id, content_hash)
        self.assertEqual(status, 200, f"Finalize large trip failed: {fin_resp}")
        self.assertIn("receipt_id", fin_resp)

    # -----------------------------------------------------------------------
    # 6. Concurrent uploads
    # -----------------------------------------------------------------------

    def test_06_concurrent_uploads(self):
        """Upload 5 trips simultaneously using threads."""
        self._check_server()

        results: Dict[int, Optional[str]] = {}  # index -> error message or None
        lock = threading.Lock()

        def _upload_thread(idx: int) -> None:
            try:
                trip = _generate_trip(seed=6000 + idx, device_id=f"test-concurrent-{idx:02d}")
                subdir = os.path.join(self.tmpdir, f"concurrent_{idx}")
                os.makedirs(subdir, exist_ok=True)
                _full_upload(trip, subdir, device_id=f"test-concurrent-{idx:02d}")
                with lock:
                    results[idx] = None
            except Exception as exc:
                with lock:
                    results[idx] = str(exc)

        threads = []
        for i in range(5):
            t = threading.Thread(target=_upload_thread, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join(timeout=120)

        # Check all 5 completed successfully
        self.assertEqual(len(results), 5, f"Expected 5 results, got {len(results)}")
        for idx, err in sorted(results.items()):
            self.assertIsNone(err, f"Thread {idx} failed: {err}")

        # Verify each device is registered
        for i in range(5):
            dev_id = f"test-concurrent-{i:02d}"
            status, resp = _http_json("GET", f"/api/v1/devices/{dev_id}/status")
            self.assertEqual(status, 200, f"Device {dev_id} not found after concurrent upload")

    # -----------------------------------------------------------------------
    # 7. Corrupted hash
    # -----------------------------------------------------------------------

    def test_07_corrupted_hash(self):
        """Upload a trip but send a wrong content_hash at finalize; expect rejection."""
        self._check_server()

        trip = _generate_trip(seed=7001, device_id="test-badhash-01")
        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        # Init with the correct hash
        status, init_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        upload_id = init_resp["upload_id"]

        # Send all chunks (correct data)
        _upload_chunks(upload_id, data)

        # Finalize with a WRONG hash
        wrong_hash = hashlib.sha256(b"this is not the right data").hexdigest()
        status, fin_resp = _upload_finalize(upload_id, wrong_hash)

        self.assertEqual(
            status, 400,
            f"Server should reject hash mismatch with 400, got {status}: {fin_resp}",
        )
        self.assertIn("error", fin_resp)
        error_msg = fin_resp["error"].lower()
        self.assertTrue(
            "hash" in error_msg or "mismatch" in error_msg,
            f"Error should mention hash mismatch: {fin_resp['error']}",
        )

    # -----------------------------------------------------------------------
    # 8. Bundle integrity
    # -----------------------------------------------------------------------

    def test_08_bundle_integrity(self):
        """Upload a trip, re-init with same hash to confirm server has it, verify hash."""
        self._check_server()

        trip = _generate_trip(seed=8001, device_id="test-integrity-01")
        bundle_dir = _write_bundle(trip, self.tmpdir)
        data, content_hash, manifest = _build_bundle_data(bundle_dir)

        # Upload fully
        uid, receipt_id, fin_resp = _full_upload(trip, self.tmpdir, device_id="test-integrity-01")

        # Re-init with same content_hash: server should confirm it is completed
        status, check_resp = _upload_init(
            manifest["device_id"], manifest["trip_id"], content_hash, len(data),
        )
        self.assertEqual(status, 200)
        self.assertEqual(
            check_resp["resume_offset"], -1,
            "Completed upload should return resume_offset=-1",
        )

        # Verify local bundle SHA-256 matches what we declared
        local_hash = hashlib.sha256(data).hexdigest()
        self.assertEqual(
            local_hash, content_hash,
            "Local SHA-256 should match declared content_hash",
        )

        # Verify the sha256sums.txt file in the bundle
        sums_path = bundle_dir / "sha256sums.txt"
        if sums_path.exists():
            sums_text = sums_path.read_text()
            for line in sums_text.strip().split("\n"):
                expected_hash, filename = line.split("  ", 1)
                file_path = bundle_dir / filename
                if file_path.exists():
                    actual_hash = hashlib.sha256(file_path.read_bytes()).hexdigest()
                    self.assertEqual(
                        actual_hash, expected_hash,
                        f"Hash mismatch for {filename} in bundle",
                    )

    # -----------------------------------------------------------------------
    # 9. Power interruption simulation
    # -----------------------------------------------------------------------

    def test_09_power_interruption_simulation(self):
        """Simulate power interruptions and verify the system handles partial bundles."""
        self._check_server()

        rng = random.Random(9001)
        route_gen = RouteGenerator(rng)
        trip_gen = TripGenerator(rng, route_gen)
        writer = BundleWriter(self.tmpdir)

        trip = trip_gen.generate(
            start_time=datetime(2026, 6, 1, 14, 0, 0, tzinfo=timezone.utc),
        )
        trip["manifest"]["device_id"] = "test-interrupt-01"

        # Write with simulated interruption (force it by using a rigged RNG)
        bundle_dir = writer.write(trip, simulate_interruption=True, rng=rng)

        partial_state_path = bundle_dir / "upload_state.partial.json"
        samples_data = (bundle_dir / "samples.bin").read_bytes()

        # Whether or not the simulator actually truncated (it is probabilistic),
        # we should be able to upload what we have. The hash will be of whatever
        # bytes are on disk.
        content_hash = hashlib.sha256(samples_data).hexdigest()

        status, init_resp = _upload_init(
            "test-interrupt-01",
            trip["trip_id"],
            content_hash,
            len(samples_data),
        )
        self.assertEqual(status, 200, f"Init failed for interrupted bundle: {init_resp}")
        upload_id = init_resp["upload_id"]

        if init_resp.get("resume_offset", 0) != -1:
            _upload_chunks(upload_id, samples_data, start_offset=init_resp.get("resume_offset", 0))
            status, fin_resp = _upload_finalize(upload_id, content_hash)
            self.assertEqual(status, 200, f"Finalize failed for interrupted bundle: {fin_resp}")

        # If the file was truncated, verify the partial state file exists
        if partial_state_path.exists():
            with open(partial_state_path) as f:
                partial = json.load(f)
            self.assertTrue(partial.get("interrupted", False))
            self.assertIn("bytes_written", partial)
            self.assertIn("expected_bytes", partial)

    # -----------------------------------------------------------------------
    # 10. Stress test
    # -----------------------------------------------------------------------

    def test_10_stress(self):
        """Upload 20 trips sequentially as fast as possible."""
        self._check_server()

        upload_ids = set()
        receipt_ids = set()
        errors: List[str] = []

        t0 = time.monotonic()
        for i in range(20):
            try:
                trip = _generate_trip(seed=10_000 + i, device_id=f"test-stress-{i:02d}")
                subdir = os.path.join(self.tmpdir, f"stress_{i}")
                os.makedirs(subdir, exist_ok=True)
                uid, rid, _ = _full_upload(trip, subdir, device_id=f"test-stress-{i:02d}")
                upload_ids.add(uid)
                if rid:
                    receipt_ids.add(rid)
            except Exception as exc:
                errors.append(f"Trip {i}: {exc}")

        elapsed = time.monotonic() - t0

        self.assertEqual(
            len(errors), 0,
            f"Stress test had {len(errors)} errors:\n" + "\n".join(errors),
        )
        self.assertEqual(
            len(upload_ids), 20,
            f"Expected 20 unique upload IDs, got {len(upload_ids)}",
        )
        self.assertEqual(
            len(receipt_ids), 20,
            f"Expected 20 unique receipt IDs, got {len(receipt_ids)}",
        )

        print(f"\n  Stress test: 20 trips in {elapsed:.1f}s "
              f"({elapsed / 20:.2f}s/trip)", file=sys.stderr)


# ---------------------------------------------------------------------------
# Custom test runner with clear PASS/FAIL output
# ---------------------------------------------------------------------------


class CairnTestResult(unittest.TextTestResult):
    """Custom test result that prints clear PASS/FAIL with timing."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._test_start: float = 0.0

    def startTest(self, test):
        self._test_start = time.monotonic()
        super().startTest(test)

    def addSuccess(self, test):
        elapsed = time.monotonic() - self._test_start
        super().addSuccess(test)
        self.stream.write(f"  PASS  {test} ({elapsed:.2f}s)\n")
        self.stream.flush()

    def addFailure(self, test, err):
        elapsed = time.monotonic() - self._test_start
        super().addFailure(test, err)
        self.stream.write(f"  FAIL  {test} ({elapsed:.2f}s)\n")
        self.stream.flush()

    def addError(self, test, err):
        elapsed = time.monotonic() - self._test_start
        super().addError(test, err)
        self.stream.write(f"  ERROR {test} ({elapsed:.2f}s)\n")
        self.stream.flush()

    def addSkip(self, test, reason):
        elapsed = time.monotonic() - self._test_start
        super().addSkip(test, reason)
        self.stream.write(f"  SKIP  {test}: {reason} ({elapsed:.2f}s)\n")
        self.stream.flush()


class CairnTestRunner(unittest.TextTestRunner):
    """Test runner using CairnTestResult."""
    resultclass = CairnTestResult


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(
        description="Cairn integration test suite",
    )
    parser.add_argument(
        "--server", type=str, default="http://localhost:8443",
        help="Ingest server URL (default: http://localhost:8443)",
    )
    parser.add_argument(
        "-v", "--verbose", action="store_true",
        help="Verbose test output",
    )
    parser.add_argument(
        "-k", "--filter", type=str, default=None,
        help="Run only tests matching this substring",
    )
    # Collect remaining args for unittest
    args, remaining = parser.parse_known_args()

    global SERVER_URL
    SERVER_URL = args.server

    print(f"Cairn Integration Tests", file=sys.stderr)
    print(f"Server: {SERVER_URL}", file=sys.stderr)
    print(f"{'=' * 60}", file=sys.stderr)

    # Check server connectivity before running tests
    try:
        req = urllib.request.Request(
            f"{SERVER_URL.rstrip('/')}/api/v1/health",
            method="GET",
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            health = json.loads(resp.read().decode("utf-8"))
            print(f"Server health: {health.get('status', 'unknown')}", file=sys.stderr)
    except Exception as exc:
        print(f"WARNING: Server not reachable at {SERVER_URL}: {exc}", file=sys.stderr)
        print(f"Tests that need the server will be skipped.", file=sys.stderr)

    print(f"{'=' * 60}", file=sys.stderr)

    # Build the test suite
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromTestCase(CairnIntegrationTests)

    # Apply filter if specified
    if args.filter:
        filtered = unittest.TestSuite()
        for test in suite:
            if args.filter in str(test):
                filtered.addTest(test)
        suite = filtered

    # Run
    verbosity = 2 if args.verbose else 1
    runner = CairnTestRunner(verbosity=verbosity, stream=sys.stderr)
    result = runner.run(suite)

    # Final summary
    print(f"\n{'=' * 60}", file=sys.stderr)
    total = result.testsRun
    failures = len(result.failures)
    errors = len(result.errors)
    skipped = len(result.skipped)
    passed = total - failures - errors - skipped

    print(f"Results: {passed} passed, {failures} failed, "
          f"{errors} errors, {skipped} skipped out of {total} total",
          file=sys.stderr)

    if failures > 0 or errors > 0:
        print("\nFailed tests:", file=sys.stderr)
        for test, _ in result.failures + result.errors:
            print(f"  - {test}", file=sys.stderr)

    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
