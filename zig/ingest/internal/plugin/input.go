package plugin

import (
	"encoding/json"
	"math"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
)

type classifierEnvelope struct {
	ABIVersion int           `json:"abi_version"`
	Trip       classifierTrip `json:"trip"`
}

type classifierTrip struct {
	ID            string             `json:"id"`
	DistanceM     float64            `json:"distance_m"`
	DurationS     int                `json:"duration_s"`
	StartLat      float64            `json:"start_lat"`
	StartLon      float64            `json:"start_lon"`
	EndLat        float64            `json:"end_lat"`
	EndLon        float64            `json:"end_lon"`
	StartHour     int                `json:"start_hour"`
	EndHour       int                `json:"end_hour"`
	DayOfWeek     int                `json:"day_of_week"`
	AvgSpeedMps   float64            `json:"avg_speed_mps"`
	MaxSpeedMps   float64            `json:"max_speed_mps"`
	NumStops      int                `json:"num_stops"`
	RouteSamples  []routeSampleInput `json:"route_samples"`
}

type routeSampleInput struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	SpeedMps  float64 `json:"speed_mps"`
	AltitudeM float64 `json:"altitude_m"`
}

type redactorEnvelope struct {
	ABIVersion   int                `json:"abi_version"`
	Route        []routeSampleInput `json:"route"`
	PrivacyZones []privacyZoneInput `json:"privacy_zones"`
}

type privacyZoneInput struct {
	Label   string  `json:"label"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	RadiusM float64 `json:"radius_m"`
}

func BuildClassifierInput(trip *db.TripDetail, route []db.RoutePoint) ([]byte, error) {
	samples := make([]routeSampleInput, len(route))
	var totalSpeed, maxSpeed float64
	var numStops int
	for i, p := range route {
		samples[i] = routeSampleInput{
			Lat:       p.Lat,
			Lon:       p.Lon,
			SpeedMps:  p.SpeedMps,
			AltitudeM: p.AltitudeM,
		}
		totalSpeed += p.SpeedMps
		if p.SpeedMps > maxSpeed {
			maxSpeed = p.SpeedMps
		}
		if p.SpeedMps < 0.5 && i > 0 && route[i-1].SpeedMps >= 0.5 {
			numStops++
		}
	}

	var avgSpeed float64
	if len(route) > 0 {
		avgSpeed = totalSpeed / float64(len(route))
	}

	// day_of_week: 0=Sunday through 6=Saturday (matching time.Weekday)
	dow := int(trip.StartedAt.Weekday())
	startHour := trip.StartedAt.Hour()
	endHour := trip.EndedAt.Hour()

	env := classifierEnvelope{
		ABIVersion: 1,
		Trip: classifierTrip{
			ID:           trip.ID,
			DistanceM:    trip.DistanceM,
			DurationS:    trip.DurationS,
			StartLat:     trip.StartLat,
			StartLon:     trip.StartLon,
			EndLat:       trip.EndLat,
			EndLon:       trip.EndLon,
			StartHour:    startHour,
			EndHour:      endHour,
			DayOfWeek:    dow,
			AvgSpeedMps:  math.Round(avgSpeed*100) / 100,
			MaxSpeedMps:  math.Round(maxSpeed*100) / 100,
			NumStops:     numStops,
			RouteSamples: samples,
		},
	}

	return json.Marshal(env)
}

// ─── Export Transformer ───────────────────────────────────────────────────

type exportEnvelope struct {
	ABIVersion int            `json:"abi_version"`
	Trip       exportTrip     `json:"trip"`
	Format     string         `json:"format"`
}

type exportTrip struct {
	TripID      string               `json:"trip_id"`
	StartedAt   string               `json:"started_at"`
	EndedAt     string               `json:"ended_at"`
	DistanceM   float64              `json:"distance_m"`
	DurationS   int                  `json:"duration_s"`
	RouteSamples []exportRouteSample `json:"route_samples"`
}

type exportRouteSample struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	AltitudeM   float64 `json:"altitude_m"`
	SpeedMps    float64 `json:"speed_mps"`
	TimestampMs int64   `json:"timestamp_ms"`
}

func BuildExportInput(trip *db.TripDetail, route []db.RoutePoint, format string) ([]byte, error) {
	samples := make([]exportRouteSample, len(route))
	for i, p := range route {
		samples[i] = exportRouteSample{
			Lat:         p.Lat,
			Lon:         p.Lon,
			AltitudeM:   p.AltitudeM,
			SpeedMps:    p.SpeedMps,
			TimestampMs: p.TimestampMs,
		}
	}

	env := exportEnvelope{
		ABIVersion: 1,
		Trip: exportTrip{
			TripID:       trip.ID,
			StartedAt:    trip.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			EndedAt:      trip.EndedAt.UTC().Format("2006-01-02T15:04:05Z"),
			DistanceM:    trip.DistanceM,
			DurationS:    trip.DurationS,
			RouteSamples: samples,
		},
		Format: format,
	}
	return json.Marshal(env)
}

// ─── Route Scorer ─────────────────────────────────────────────────────────

type routeScorerEnvelope struct {
	ABIVersion int                `json:"abi_version"`
	Route      []routeScorePoint  `json:"route"`
	History    []historicalRoute   `json:"history"`
}

type routeScorePoint struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	SpeedMps    float64 `json:"speed_mps"`
	TimestampMs int64   `json:"timestamp_ms"`
}

type historicalRoute struct {
	Route []routeScoreSimplePoint `json:"route"`
	Count int                     `json:"count"`
}

type routeScoreSimplePoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func BuildRouteScorerInput(route []db.RoutePoint, history []db.RoutePoint, historyCount int) ([]byte, error) {
	rp := make([]routeScorePoint, len(route))
	for i, p := range route {
		rp[i] = routeScorePoint{
			Lat:         p.Lat,
			Lon:         p.Lon,
			SpeedMps:    p.SpeedMps,
			TimestampMs: p.TimestampMs,
		}
	}

	hp := make([]routeScoreSimplePoint, len(history))
	for i, p := range history {
		hp[i] = routeScoreSimplePoint{
			Lat: p.Lat,
			Lon: p.Lon,
		}
	}

	env := routeScorerEnvelope{
		ABIVersion: 1,
		Route:      rp,
		History: []historicalRoute{
			{Route: hp, Count: historyCount},
		},
	}
	return json.Marshal(env)
}

// ─── Data Quality Detector ────────────────────────────────────────────────

type dataQualityEnvelope struct {
	ABIVersion int                   `json:"abi_version"`
	Samples    []dataQualitySample   `json:"samples"`
}

type dataQualitySample struct {
	LatDeg7     int32  `json:"lat_deg7"`
	LonDeg7     int32  `json:"lon_deg7"`
	SpeedCmps   uint16 `json:"speed_cmps"`
	TimestampMs uint64 `json:"timestamp_ms"`
	Satellites  uint8  `json:"satellites"`
	HdopTenths  uint16 `json:"hdop_tenths"`
	AccuracyCm  uint16 `json:"accuracy_cm"`
}

func BuildDataQualityInput(route []db.RoutePoint) ([]byte, error) {
	samples := make([]dataQualitySample, len(route))
	for i, p := range route {
		samples[i] = dataQualitySample{
			LatDeg7:     int32(p.Lat * 1e7),
			LonDeg7:     int32(p.Lon * 1e7),
			SpeedCmps:   uint16(p.SpeedMps * 100),
			TimestampMs: uint64(p.TimestampMs),
			Satellites:  0,  // Not available in RoutePoint; plugin handles gracefully
			HdopTenths:  0,
			AccuracyCm:  0,
		}
	}

	env := dataQualityEnvelope{
		ABIVersion: 1,
		Samples:    samples,
	}
	return json.Marshal(env)
}

// ─── Privacy Redactor ─────────────────────────────────────────────────────

func BuildRedactorInput(route []db.RoutePoint, places []db.PlaceRow) ([]byte, error) {
	rp := make([]routeSampleInput, len(route))
	for i, p := range route {
		rp[i] = routeSampleInput{
			Lat:       p.Lat,
			Lon:       p.Lon,
			SpeedMps:  p.SpeedMps,
			AltitudeM: p.AltitudeM,
		}
	}

	zones := make([]privacyZoneInput, len(places))
	for i, p := range places {
		zones[i] = privacyZoneInput{
			Label:   p.Name,
			Lat:     p.Lat,
			Lon:     p.Lon,
			RadiusM: p.RadiusM,
		}
	}

	return json.Marshal(redactorEnvelope{
		ABIVersion:   1,
		Route:        rp,
		PrivacyZones: zones,
	})
}
