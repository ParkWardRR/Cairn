package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestPlugins(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// trip-classifier
	src := filepath.Join("..", "..", "..", "..", "plugins", "trip-classifier", "_build", "wasm", "release", "build", "trip-classifier.wasm")
	wasmBytes, err := os.ReadFile(src)
	if err != nil {
		src = filepath.Join("..", "..", "..", "..", "plugins", "trip-classifier", "_build", "wasm", "debug", "build", "trip-classifier.wasm")
		wasmBytes, err = os.ReadFile(src)
		if err != nil {
			t.Skipf("trip-classifier.wasm not found: %v", err)
		}
	}
	os.WriteFile(filepath.Join(dir, "trip-classifier.wasm"), wasmBytes, 0644)
	os.WriteFile(filepath.Join(dir, "trip-classifier.json"), []byte(`{
		"name": "trip-classifier",
		"version": "0.1.0",
		"type": "classifier",
		"entry_function": "classify",
		"description": "Classifies trips"
	}`), 0644)

	// privacy-redactor
	src2 := filepath.Join("..", "..", "..", "..", "plugins", "privacy-redactor", "_build", "wasm", "release", "build", "privacy-redactor.wasm")
	wasmBytes2, err := os.ReadFile(src2)
	if err != nil {
		src2 = filepath.Join("..", "..", "..", "..", "plugins", "privacy-redactor", "_build", "wasm", "debug", "build", "privacy-redactor.wasm")
		wasmBytes2, err = os.ReadFile(src2)
		if err != nil {
			t.Skipf("privacy-redactor.wasm not found: %v", err)
		}
	}
	os.WriteFile(filepath.Join(dir, "privacy-redactor.wasm"), wasmBytes2, 0644)
	os.WriteFile(filepath.Join(dir, "privacy-redactor.json"), []byte(`{
		"name": "privacy-redactor",
		"version": "0.1.0",
		"type": "redactor",
		"entry_function": "redact",
		"description": "Redacts privacy zones"
	}`), 0644)

	return dir
}

func TestHostLoadPlugins(t *testing.T) {
	dir := setupTestPlugins(t)
	host, err := NewHost(dir)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	defer host.Close()

	plugins := host.List()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	names := map[string]bool{}
	for _, p := range plugins {
		names[p.Name] = true
	}
	if !names["trip-classifier"] {
		t.Error("trip-classifier not loaded")
	}
	if !names["privacy-redactor"] {
		t.Error("privacy-redactor not loaded")
	}
}

func TestClassifyCommute(t *testing.T) {
	dir := setupTestPlugins(t)
	host, err := NewHost(dir)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	defer host.Close()

	input := `{
		"abi_version": 1,
		"trip": {
			"id": "test-trip-1",
			"distance_m": 15000,
			"duration_s": 1800,
			"start_lat": 37.7749,
			"start_lon": -122.4194,
			"end_lat": 37.3861,
			"end_lon": -122.0839,
			"start_hour": 8,
			"end_hour": 9,
			"day_of_week": 1,
			"avg_speed_mps": 8.3,
			"max_speed_mps": 25.0,
			"num_stops": 3,
			"route_samples": [
				{"lat": 37.7749, "lon": -122.4194, "speed_mps": 10.0, "altitude_m": 16.0},
				{"lat": 37.6, "lon": -122.3, "speed_mps": 25.0, "altitude_m": 20.0},
				{"lat": 37.3861, "lon": -122.0839, "speed_mps": 5.0, "altitude_m": 30.0}
			]
		}
	}`

	output, err := host.Execute("trip-classifier", []byte(input))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, string(output))
	}

	labels, ok := result["labels"].([]any)
	if !ok {
		t.Fatalf("expected labels array, got %T: %s", result["labels"], string(output))
	}
	if len(labels) == 0 {
		t.Fatal("expected at least one label")
	}

	t.Logf("classifier output: %s", string(output))

	found := false
	for _, l := range labels {
		lm := l.(map[string]any)
		if lm["name"] == "commute" {
			found = true
			conf := lm["confidence"].(float64)
			if conf < 0.5 {
				t.Errorf("commute confidence too low: %f", conf)
			}
		}
	}
	if !found {
		t.Error("expected 'commute' label for weekday morning 15km trip")
	}
}

func TestClassifyRoadTrip(t *testing.T) {
	dir := setupTestPlugins(t)
	host, err := NewHost(dir)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	defer host.Close()

	input := `{
		"abi_version": 1,
		"trip": {
			"id": "road-trip-1",
			"distance_m": 250000,
			"duration_s": 10800,
			"start_lat": 37.7749,
			"start_lon": -122.4194,
			"end_lat": 34.0522,
			"end_lon": -118.2437,
			"start_hour": 6,
			"end_hour": 9,
			"day_of_week": 6,
			"avg_speed_mps": 23.1,
			"max_speed_mps": 35.0,
			"num_stops": 2,
			"route_samples": []
		}
	}`

	output, err := host.Execute("trip-classifier", []byte(input))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]any
	json.Unmarshal(output, &result)

	labels := result["labels"].([]any)
	found := false
	for _, l := range labels {
		lm := l.(map[string]any)
		if lm["name"] == "road_trip" {
			found = true
			if lm["confidence"].(float64) < 0.7 {
				t.Errorf("road_trip confidence too low")
			}
		}
	}
	if !found {
		t.Error("expected 'road_trip' label for 250km trip")
	}
	t.Logf("classifier output: %s", string(output))
}

func TestRedactPrivacyZones(t *testing.T) {
	dir := setupTestPlugins(t)
	host, err := NewHost(dir)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	defer host.Close()

	input := `{
		"abi_version": 1,
		"route": [
			{"lat": 37.7749, "lon": -122.4194, "speed_mps": 0.0, "altitude_m": 16.0, "timestamp_ms": 1700000000000},
			{"lat": 37.7750, "lon": -122.4195, "speed_mps": 5.0, "altitude_m": 16.0, "timestamp_ms": 1700000001000},
			{"lat": 37.80, "lon": -122.40, "speed_mps": 15.0, "altitude_m": 20.0, "timestamp_ms": 1700000060000},
			{"lat": 37.85, "lon": -122.35, "speed_mps": 20.0, "altitude_m": 25.0, "timestamp_ms": 1700000120000},
			{"lat": 37.3861, "lon": -122.0839, "speed_mps": 0.0, "altitude_m": 30.0, "timestamp_ms": 1700000600000}
		],
		"privacy_zones": [
			{"lat": 37.7749, "lon": -122.4194, "radius_m": 200.0, "label": "home"},
			{"lat": 37.3861, "lon": -122.0839, "radius_m": 150.0, "label": "work"}
		]
	}`

	output, err := host.Execute("privacy-redactor", []byte(input))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, string(output))
	}

	t.Logf("redactor output: %s", string(output))

	redactedRoute, ok := result["redacted_route"].([]any)
	if !ok {
		t.Fatalf("expected redacted_route array")
	}
	if len(redactedRoute) != 5 {
		t.Fatalf("expected 5 points, got %d", len(redactedRoute))
	}

	// First two points should be redacted (within 200m of home)
	p0 := redactedRoute[0].(map[string]any)
	if p0["redacted"] != true {
		t.Error("point 0 should be redacted (home zone)")
	}

	// Middle point should not be redacted
	p2 := redactedRoute[2].(map[string]any)
	if p2["redacted"] != false {
		t.Error("point 2 should not be redacted")
	}

	// Last point should be redacted (within 150m of work)
	p4 := redactedRoute[4].(map[string]any)
	if p4["redacted"] != true {
		t.Error("point 4 should be redacted (work zone)")
	}

	pointsRedacted, _ := result["points_redacted"].(float64)
	if pointsRedacted < 2 {
		t.Errorf("expected at least 2 points redacted, got %v", pointsRedacted)
	}
}
