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
