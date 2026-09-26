package parser

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

// ─── Wire types (binary on-disk layout) ────────────────────────────────────

// rawGNSSSample is the 32-byte little-endian packed GNSS record.
type rawGNSSSample struct {
	TimestampMs uint64
	Latitude    int32  // degrees × 10^7
	Longitude   int32  // degrees × 10^7
	AltitudeCm  int32  // centimetres above WGS-84
	SpeedCmps   uint16 // cm/s
	HeadingCdeg uint16 // centidegrees 0–35999
	FixQuality  uint8  // 0=none, 1=GPS, 2=DGPS, 4=RTK
	Satellites  uint8
	HdopTenths  uint16 // HDOP × 10
	AccuracyCm  uint16 // estimated horizontal accuracy in cm
	Reserved    [2]byte
}

const gnssRecordSize = 32

// rawIMUSummary is the 24-byte little-endian packed IMU summary record.
type rawIMUSummary struct {
	WindowStartMs    uint64
	WindowDurationMs uint16
	AccelPeakXMg     int16
	AccelPeakYMg     int16
	AccelPeakZMg     int16
	AccelRmsMg       uint16
	GyroPeakDps      int16
	Variance         uint16
	Flags            uint8
	Reserved         uint8
}

const imuSummaryRecordSize = 24

// ─── Parsed types (database-friendly) ──────────────────────────────────────

// Manifest mirrors the JSON manifest embedded in every trip bundle.
type Manifest struct {
	Version         int    `json:"version"`
	TripID          string `json:"trip_id"`
	DeviceID        string `json:"device_id"`
	FirmwareVersion string `json:"firmware_version"`
	StartedAt       string `json:"started_at"`
	EndedAt         string `json:"ended_at"`
	SampleCount     struct {
		GNSS          int `json:"gnss"`
		IMUSummary    int `json:"imu_summary"`
		IMURawWindows int `json:"imu_raw_windows"`
	} `json:"sample_count"`
	GNSSRateHz    int    `json:"gnss_rate_hz"`
	IMURateHz     int    `json:"imu_rate_hz"`
	SchemaVersion int    `json:"schema_version"`
	Compression   string `json:"compression"`
}

// LocationSample is a single GNSS fix in human/DB-friendly units.
type LocationSample struct {
	TimestampMs uint64
	Latitude    float64 // degrees
	Longitude   float64 // degrees
	AltitudeM   float64 // metres
	SpeedMps    float64 // m/s
	HeadingDeg  float64 // degrees 0–360
	FixQuality  int
	Satellites  int
	Hdop        float64
	AccuracyM   float64 // metres
}

// MotionSample is a single IMU summary window in DB-friendly form.
type MotionSample struct {
	WindowStartMs    uint64
	WindowDurationMs int
	AccelPeakXMg     int16
	AccelPeakYMg     int16
	AccelPeakZMg     int16
	AccelRmsMg       uint16
	GyroPeakDps      int16
	Variance         uint16
	Flags            uint8
}

// TripEvent is one lifecycle / quality event.
type TripEvent struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// ParsedBundle holds every artefact extracted and decoded from a trip bundle.
type ParsedBundle struct {
	Manifest      Manifest
	Samples       []LocationSample
	MotionSamples []MotionSample
	Events        []TripEvent

	// Computed summaries
	DistanceM     float64
	DurationS     int
	StartLocation [2]float64 // [lat, lon]
	EndLocation   [2]float64 // [lat, lon]
	StartedAt     time.Time
	EndedAt       time.Time
}

// ─── Public entry points ───────────────────────────────────────────────────

// ParseBundleFile opens a bundle file and parses its contents.
// It auto-detects the format: tar archive, or raw samples.bin.
func ParseBundleFile(bundlePath string) (*ParsedBundle, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}
	defer f.Close()

	// Read the first few bytes to detect format
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	f.Seek(0, 0)

	// Check if it's a tar archive (tar magic at offset 257)
	if n >= 263 && string(header[257:262]) == "ustar" {
		return parseTar(f)
	}

	// Not a tar — return error so caller can try ParseRawSamples with metadata
	return nil, fmt.Errorf("not a tar archive (no ustar magic)")
}

// ParseRawSamples parses a raw samples.bin file (32-byte GNSS records)
// and constructs a ParsedBundle with a synthetic manifest derived from
// the data itself.
func ParseRawSamples(data []byte, deviceID, contentHash string) (*ParsedBundle, error) {
	samples, err := parseSamples(data)
	if err != nil {
		return nil, fmt.Errorf("parse raw samples: %w", err)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("raw samples.bin is empty")
	}

	// Derive timing from first and last sample timestamps
	startMs := samples[0].TimestampMs
	endMs := samples[len(samples)-1].TimestampMs
	startedAt := time.UnixMilli(int64(startMs)).UTC()
	endedAt := time.UnixMilli(int64(endMs)).UTC()

	if deviceID == "" {
		deviceID = "unknown"
	}

	var tripID string
	if contentHash != "" && len(contentHash) >= 8 {
		tripID = fmt.Sprintf("raw-%s-%d", contentHash[:8], startMs)
	} else {
		tripID = fmt.Sprintf("raw-%d", startMs)
	}

	manifest := Manifest{
		Version:  1,
		TripID:   tripID,
		DeviceID: deviceID,
		StartedAt: startedAt.Format(time.RFC3339),
		EndedAt:   endedAt.Format(time.RFC3339),
		SchemaVersion: 1,
	}
	manifest.SampleCount.GNSS = len(samples)

	distanceM := computeDistance(samples)
	durationS := int(endedAt.Sub(startedAt).Seconds())

	startLoc := [2]float64{samples[0].Latitude, samples[0].Longitude}
	endLoc := [2]float64{samples[len(samples)-1].Latitude, samples[len(samples)-1].Longitude}

	return &ParsedBundle{
		Manifest:      manifest,
		Samples:       samples,
		DistanceM:     distanceM,
		DurationS:     durationS,
		StartLocation: startLoc,
		EndLocation:   endLoc,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
	}, nil
}

// ParseBundleDir reads a trip bundle from an unpacked directory containing
// manifest.json, samples.bin, events.json, and optionally imu_summary.bin.
// This is useful for reparsing fixtures or locally-extracted bundles.
func ParseBundleDir(dirPath string) (*ParsedBundle, error) {
	files := make(map[string][]byte)

	for _, name := range []string{"manifest.json", "samples.bin", "events.json", "imu_summary.bin"} {
		data, err := os.ReadFile(filepath.Join(dirPath, name))
		if err != nil {
			if os.IsNotExist(err) && name == "imu_summary.bin" {
				continue // optional
			}
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("required file %s not found in bundle dir: %w", name, err)
			}
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = data
	}

	return assembleBundle(files)
}

// ─── Tar extraction ────────────────────────────────────────────────────────

func parseTar(r io.Reader) (*ParsedBundle, error) {
	tr := tar.NewReader(r)
	files := make(map[string][]byte)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar header: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Strip any leading directory (e.g., "01KEF.../manifest.json" → "manifest.json").
		name := filepath.Base(hdr.Name)

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar entry %s: %w", name, err)
		}
		files[name] = data
	}

	if _, ok := files["manifest.json"]; !ok {
		return nil, fmt.Errorf("bundle tar missing manifest.json")
	}

	return assembleBundle(files)
}

// ─── Assembly from file map ────────────────────────────────────────────────

func assembleBundle(files map[string][]byte) (*ParsedBundle, error) {
	manifest, err := parseManifest(files["manifest.json"])
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	samples, err := parseSamples(files["samples.bin"])
	if err != nil {
		return nil, fmt.Errorf("parse samples: %w", err)
	}

	var motionSamples []MotionSample
	if imuData, ok := files["imu_summary.bin"]; ok && len(imuData) > 0 {
		motionSamples, err = parseMotionSamples(imuData)
		if err != nil {
			return nil, fmt.Errorf("parse imu summaries: %w", err)
		}
	}

	var events []TripEvent
	if evData, ok := files["events.json"]; ok && len(evData) > 0 {
		events, err = parseEvents(evData)
		if err != nil {
			return nil, fmt.Errorf("parse events: %w", err)
		}
	}

	// Parse timestamps from manifest.
	startedAt, err := time.Parse(time.RFC3339, manifest.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", manifest.StartedAt, err)
	}
	endedAt, err := time.Parse(time.RFC3339, manifest.EndedAt)
	if err != nil {
		return nil, fmt.Errorf("parse ended_at %q: %w", manifest.EndedAt, err)
	}

	// Compute summaries.
	distanceM := computeDistance(samples)
	durationS := int(endedAt.Sub(startedAt).Seconds())

	var startLoc, endLoc [2]float64
	if len(samples) > 0 {
		startLoc = [2]float64{samples[0].Latitude, samples[0].Longitude}
		endLoc = [2]float64{samples[len(samples)-1].Latitude, samples[len(samples)-1].Longitude}
	}

	return &ParsedBundle{
		Manifest:      *manifest,
		Samples:       samples,
		MotionSamples: motionSamples,
		Events:        events,
		DistanceM:     distanceM,
		DurationS:     durationS,
		StartLocation: startLoc,
		EndLocation:   endLoc,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
	}, nil
}

// ─── Individual parsers ────────────────────────────────────────────────────

func parseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.TripID == "" {
		return nil, fmt.Errorf("manifest missing trip_id")
	}
	if m.DeviceID == "" {
		return nil, fmt.Errorf("manifest missing device_id")
	}
	return &m, nil
}

func parseSamples(data []byte) ([]LocationSample, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data)%gnssRecordSize != 0 {
		return nil, fmt.Errorf("samples.bin size %d not a multiple of %d", len(data), gnssRecordSize)
	}

	count := len(data) / gnssRecordSize
	samples := make([]LocationSample, 0, count)

	r := bytes.NewReader(data)
	for i := 0; i < count; i++ {
		var raw rawGNSSSample
		if err := binary.Read(r, binary.LittleEndian, &raw); err != nil {
			return nil, fmt.Errorf("read GNSS record %d: %w", i, err)
		}
		samples = append(samples, LocationSample{
			TimestampMs: raw.TimestampMs,
			Latitude:    float64(raw.Latitude) / 1e7,
			Longitude:   float64(raw.Longitude) / 1e7,
			AltitudeM:   float64(raw.AltitudeCm) / 100.0,
			SpeedMps:    float64(raw.SpeedCmps) / 100.0,
			HeadingDeg:  float64(raw.HeadingCdeg) / 100.0,
			FixQuality:  int(raw.FixQuality),
			Satellites:  int(raw.Satellites),
			Hdop:        float64(raw.HdopTenths) / 10.0,
			AccuracyM:   float64(raw.AccuracyCm) / 100.0,
		})
	}

	return samples, nil
}

func parseMotionSamples(data []byte) ([]MotionSample, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data)%imuSummaryRecordSize != 0 {
		return nil, fmt.Errorf("imu_summary.bin size %d not a multiple of %d", len(data), imuSummaryRecordSize)
	}

	count := len(data) / imuSummaryRecordSize
	samples := make([]MotionSample, 0, count)

	r := bytes.NewReader(data)
	for i := 0; i < count; i++ {
		var raw rawIMUSummary
		if err := binary.Read(r, binary.LittleEndian, &raw); err != nil {
			return nil, fmt.Errorf("read IMU summary record %d: %w", i, err)
		}
		samples = append(samples, MotionSample{
			WindowStartMs:    raw.WindowStartMs,
			WindowDurationMs: int(raw.WindowDurationMs),
			AccelPeakXMg:     raw.AccelPeakXMg,
			AccelPeakYMg:     raw.AccelPeakYMg,
			AccelPeakZMg:     raw.AccelPeakZMg,
			AccelRmsMg:       raw.AccelRmsMg,
			GyroPeakDps:      raw.GyroPeakDps,
			Variance:         raw.Variance,
			Flags:            raw.Flags,
		})
	}

	return samples, nil
}

func parseEvents(data []byte) ([]TripEvent, error) {
	// Events can be either a bare array or {"events": [...]}
	var events []TripEvent
	if err := json.Unmarshal(data, &events); err != nil {
		// Try wrapped format.
		var wrapper struct {
			Events []TripEvent `json:"events"`
		}
		if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parse events (tried array and object): %w", err)
		}
		events = wrapper.Events
	}
	return events, nil
}

// ─── Distance computation ──────────────────────────────────────────────────

const earthRadiusM = 6_371_000.0

func computeDistance(samples []LocationSample) float64 {
	if len(samples) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(samples); i++ {
		total += haversine(
			samples[i-1].Latitude, samples[i-1].Longitude,
			samples[i].Latitude, samples[i].Longitude,
		)
	}
	return total
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	lat1r := lat1 * math.Pi / 180
	lon1r := lon1 * math.Pi / 180
	lat2r := lat2 * math.Pi / 180
	lon2r := lon2 * math.Pi / 180

	dlat := lat2r - lat1r
	dlon := lon2r - lon1r

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1r)*math.Cos(lat2r)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}
