package parser

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const fixtureDir = "../../../../fixtures/synthetic/01KEF05GZ0C33N7EKHDK8GY8K9"

func TestParseBundleDir(t *testing.T) {
	// Resolve to absolute path relative to this test file.
	dir := fixtureDir
	if !filepath.IsAbs(dir) {
		// Make it work from the test directory.
		wd, _ := os.Getwd()
		dir = filepath.Join(wd, dir)
	}

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); os.IsNotExist(err) {
		t.Skipf("fixture directory not found: %s", dir)
	}

	bundle, err := ParseBundleDir(dir)
	if err != nil {
		t.Fatalf("ParseBundleDir failed: %v", err)
	}

	// Manifest checks.
	if bundle.Manifest.TripID != "01KEF05GZ0C33N7EKHDK8GY8K9" {
		t.Errorf("trip_id = %q, want 01KEF05GZ0C33N7EKHDK8GY8K9", bundle.Manifest.TripID)
	}
	if bundle.Manifest.DeviceID != "cairn-simulator-01" {
		t.Errorf("device_id = %q, want cairn-simulator-01", bundle.Manifest.DeviceID)
	}

	// GNSS sample count from manifest is 951.
	if len(bundle.Samples) != 951 {
		t.Errorf("sample count = %d, want 951", len(bundle.Samples))
	}

	// Verify first sample is in the Santa Monica area.
	if len(bundle.Samples) > 0 {
		s := bundle.Samples[0]
		if s.Latitude < 33.9 || s.Latitude > 34.1 {
			t.Errorf("first sample lat = %f, want ~34.0", s.Latitude)
		}
		if s.Longitude < -118.6 || s.Longitude > -118.4 {
			t.Errorf("first sample lon = %f, want ~-118.5", s.Longitude)
		}
		if s.FixQuality != 1 {
			t.Errorf("first sample fix_quality = %d, want 1", s.FixQuality)
		}
	}

	// Events.
	if len(bundle.Events) != 6 {
		t.Errorf("event count = %d, want 6", len(bundle.Events))
	}
	if len(bundle.Events) > 0 && bundle.Events[0].Type != "trip_start" {
		t.Errorf("first event type = %q, want trip_start", bundle.Events[0].Type)
	}

	// IMU summaries: manifest says 23750.
	if len(bundle.MotionSamples) != 23750 {
		t.Errorf("motion sample count = %d, want 23750", len(bundle.MotionSamples))
	}

	// Computed summaries.
	if bundle.DistanceM < 100 {
		t.Errorf("distance_m = %f, want > 100", bundle.DistanceM)
	}
	if bundle.DurationS < 60 {
		t.Errorf("duration_s = %d, want > 60", bundle.DurationS)
	}

	t.Logf("Trip %s: %d samples, %d motion, %d events, %.0f m, %d s",
		bundle.Manifest.TripID,
		len(bundle.Samples), len(bundle.MotionSamples), len(bundle.Events),
		bundle.DistanceM, bundle.DurationS)
}

func TestParseBundleTar(t *testing.T) {
	dir := fixtureDir
	if !filepath.IsAbs(dir) {
		wd, _ := os.Getwd()
		dir = filepath.Join(wd, dir)
	}

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); os.IsNotExist(err) {
		t.Skipf("fixture directory not found: %s", dir)
	}

	// Build a tar from the fixture directory.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, name := range []string{"manifest.json", "samples.bin", "events.json", "imu_summary.bin"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write tar data %s: %v", name, err)
		}
	}
	tw.Close()

	// Parse the tar.
	bundle, err := parseTar(&buf)
	if err != nil {
		t.Fatalf("parseTar failed: %v", err)
	}

	if bundle.Manifest.TripID != "01KEF05GZ0C33N7EKHDK8GY8K9" {
		t.Errorf("trip_id = %q", bundle.Manifest.TripID)
	}
	if len(bundle.Samples) != 951 {
		t.Errorf("sample count = %d, want 951", len(bundle.Samples))
	}
}

func TestHaversine(t *testing.T) {
	// Santa Monica Pier to Ocean & Wilshire ~1.5 km.
	d := haversine(34.0094, -118.4973, 34.0195, -118.4912)
	if d < 1000 || d > 2000 {
		t.Errorf("haversine = %f m, want ~1200 m", d)
	}
}
