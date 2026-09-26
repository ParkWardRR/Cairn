package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
)

// ApiHandler serves the read-only trip browsing API and places CRUD.
type ApiHandler struct {
	queries *db.Queries
}

func NewApiHandler(queries *db.Queries) *ApiHandler {
	return &ApiHandler{queries: queries}
}

// ─── GET /api/v1/stats ─────────────────────────────────────────────────────

func (h *ApiHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.queries.GetStats(ctx)
	if err != nil {
		log.Printf("get stats: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	recent, err := h.queries.MostRecentTrip(ctx)
	if err != nil {
		log.Printf("most recent trip: %v", err)
	}

	resp := map[string]any{
		"total_trips":      stats.TotalTrips,
		"total_distance_m": stats.TotalDistanceM,
		"total_duration_s": stats.TotalDurationS,
		"device_count":     stats.DeviceCount,
		"recent_trip":      recent,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── GET /api/v1/trips ─────────────────────────────────────────────────────

func (h *ApiHandler) ListTrips(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := intParam(q.Get("limit"), 20)
	offset := intParam(q.Get("offset"), 0)
	deviceID := q.Get("device_id")
	tag := q.Get("tag")

	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = &t
		}
	}

	result, err := h.queries.ListTrips(r.Context(), limit, offset, deviceID, tag, from, to)
	if err != nil {
		log.Printf("list trips: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── GET /api/v1/trips/{id} ────────────────────────────────────────────────

func (h *ApiHandler) GetTrip(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	trip, err := h.queries.GetTrip(r.Context(), tripID)
	if err != nil {
		log.Printf("get trip %s: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if trip == nil {
		httpError(w, http.StatusNotFound, "trip not found")
		return
	}

	events, _ := h.queries.GetTripEvents(r.Context(), tripID)

	resp := map[string]any{
		"id":          trip.ID,
		"device_id":   trip.DeviceID,
		"started_at":  trip.StartedAt,
		"ended_at":    trip.EndedAt,
		"distance_m":  trip.DistanceM,
		"duration_s":  trip.DurationS,
		"start_lat":   trip.StartLat,
		"start_lon":   trip.StartLon,
		"end_lat":     trip.EndLat,
		"end_lon":     trip.EndLon,
		"bundle_hash": trip.BundleHash,
		"tags":        trip.Tags,
		"events":      events,
		"summary":     json.RawMessage(trip.Summary),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── GET /api/v1/trips/{id}/route ──────────────────────────────────────────

func (h *ApiHandler) GetTripRoute(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	points, err := h.queries.GetTripRoute(r.Context(), tripID)
	if err != nil {
		log.Printf("get trip route %s: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if len(points) == 0 {
		httpError(w, http.StatusNotFound, "no route data")
		return
	}

	coords := make([][]float64, len(points))
	speeds := make([]float64, len(points))
	alts := make([]float64, len(points))
	for i, p := range points {
		coords[i] = []float64{p.Lon, p.Lat, p.AltitudeM}
		speeds[i] = p.SpeedMps
		alts[i] = p.AltitudeM
	}

	geojson := map[string]any{
		"type": "FeatureCollection",
		"features": []map[string]any{
			{
				"type": "Feature",
				"geometry": map[string]any{
					"type":        "LineString",
					"coordinates": coords,
				},
				"properties": map[string]any{
					"speeds":    speeds,
					"altitudes": alts,
				},
			},
		},
	}
	writeJSON(w, http.StatusOK, geojson)
}

// ─── GET /api/v1/trips/{id}/events ─────────────────────────────────────────

func (h *ApiHandler) GetTripEvents(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	events, err := h.queries.GetTripEvents(r.Context(), tripID)
	if err != nil {
		log.Printf("get trip events %s: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// ─── DELETE /api/v1/trips/{id} ─────────────────────────────────────────────

func (h *ApiHandler) DeleteTrip(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	if err := h.queries.FullDeleteTrip(r.Context(), tripID); err != nil {
		log.Printf("delete trip %s: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── POST /api/v1/trips/{id}/tags ──────────────────────────────────────────

func (h *ApiHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	var req struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Tag == "" {
		httpError(w, http.StatusBadRequest, "tag is required")
		return
	}
	if err := h.queries.AddTag(r.Context(), tripID, req.Tag); err != nil {
		log.Printf("add tag %s to %s: %v", req.Tag, tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// ─── DELETE /api/v1/trips/{id}/tags/{tag} ──────────────────────────────────

func (h *ApiHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	tag := r.PathValue("tag")
	if err := h.queries.RemoveTag(r.Context(), tripID, tag); err != nil {
		log.Printf("remove tag %s from %s: %v", tag, tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── GET /api/v1/devices ───────────────────────────────────────────────────

func (h *ApiHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.queries.ListDevices(r.Context())
	if err != nil {
		log.Printf("list devices: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

// ─── GET /api/v1/places ────────────────────────────────────────────────────

func (h *ApiHandler) ListPlaces(w http.ResponseWriter, r *http.Request) {
	places, err := h.queries.ListPlaces(r.Context())
	if err != nil {
		log.Printf("list places: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": places})
}

// ─── POST /api/v1/places ───────────────────────────────────────────────────

func (h *ApiHandler) CreatePlace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string  `json:"name"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		RadiusM float64 `json:"radius_m"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" || req.RadiusM <= 0 {
		httpError(w, http.StatusBadRequest, "name and radius_m are required")
		return
	}

	id, err := h.queries.CreatePlace(r.Context(), req.Name, req.Lat, req.Lon, req.RadiusM)
	if err != nil {
		log.Printf("create place: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ─── PUT /api/v1/places/{id} ───────────────────────────────────────────────

func (h *ApiHandler) UpdatePlace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid place id")
		return
	}
	var req struct {
		Name    string  `json:"name"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		RadiusM float64 `json:"radius_m"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.queries.UpdatePlace(r.Context(), id, req.Name, req.Lat, req.Lon, req.RadiusM); err != nil {
		log.Printf("update place %s: %v", id, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── DELETE /api/v1/places/{id} ────────────────────────────────────────────

func (h *ApiHandler) DeletePlace(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid place id")
		return
	}
	if err := h.queries.DeletePlace(r.Context(), id); err != nil {
		log.Printf("delete place %s: %v", id, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Export endpoints ──────────────────────────────────────────────────────

func (h *ApiHandler) ExportGPX(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	points, err := h.queries.GetTripRoute(r.Context(), tripID)
	if err != nil || len(points) == 0 {
		httpError(w, http.StatusNotFound, "no route data")
		return
	}

	trip, _ := h.queries.GetTrip(r.Context(), tripID)
	name := tripID
	if trip != nil {
		name = fmt.Sprintf("Trip %s", trip.StartedAt.Format("2006-01-02 15:04"))
	}

	w.Header().Set("Content-Type", "application/gpx+xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.gpx"`, tripID))

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="Cairn"
     xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>%s</name>
    <trkseg>
`, xmlEscape(name))

	for _, p := range points {
		ts := time.UnixMilli(p.TimestampMs).UTC().Format(time.RFC3339)
		fmt.Fprintf(w, `      <trkpt lat="%.7f" lon="%.7f">
        <ele>%.1f</ele>
        <time>%s</time>
        <speed>%.2f</speed>
      </trkpt>
`, p.Lat, p.Lon, p.AltitudeM, ts, p.SpeedMps)
	}

	fmt.Fprint(w, `    </trkseg>
  </trk>
</gpx>
`)
}

func (h *ApiHandler) ExportGeoJSON(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	points, err := h.queries.GetTripRoute(r.Context(), tripID)
	if err != nil || len(points) == 0 {
		httpError(w, http.StatusNotFound, "no route data")
		return
	}

	coords := make([][]float64, len(points))
	for i, p := range points {
		coords[i] = []float64{p.Lon, p.Lat, p.AltitudeM}
	}

	geojson := map[string]any{
		"type": "FeatureCollection",
		"features": []map[string]any{
			{
				"type": "Feature",
				"geometry": map[string]any{
					"type":        "LineString",
					"coordinates": coords,
				},
				"properties": map[string]any{
					"trip_id": tripID,
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/geo+json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.geojson"`, tripID))
	json.NewEncoder(w).Encode(geojson)
}

func (h *ApiHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	points, err := h.queries.GetTripRoute(r.Context(), tripID)
	if err != nil || len(points) == 0 {
		httpError(w, http.StatusNotFound, "no route data")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, tripID))

	fmt.Fprintln(w, "timestamp_ms,latitude,longitude,altitude_m,speed_mps")
	for _, p := range points {
		fmt.Fprintf(w, "%d,%.7f,%.7f,%.1f,%.2f\n", p.TimestampMs, p.Lat, p.Lon, p.AltitudeM, p.SpeedMps)
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func intParam(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fallback
	}
	if n > 1000 {
		return 1000
	}
	return n
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// Silence unused import warnings.
var _ = math.MaxFloat64
