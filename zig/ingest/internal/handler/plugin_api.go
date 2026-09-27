package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
	"github.com/ParkWardRR/Cairn/ingest/internal/plugin"
)

type PluginHandler struct {
	host    *plugin.Host
	queries *db.Queries
}

func NewPluginHandler(host *plugin.Host, queries *db.Queries) *PluginHandler {
	return &PluginHandler{host: host, queries: queries}
}

func (h *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"plugins": h.host.List()})
}

func (h *PluginHandler) RunPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	input, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	output, err := h.host.Execute(name, input)
	if err != nil {
		log.Printf("plugin %s execution: %v", name, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (h *PluginHandler) ClassifyTrip(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	ctx := r.Context()

	trip, err := h.queries.GetTrip(ctx, tripID)
	if err != nil {
		log.Printf("classify trip %s: get trip: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if trip == nil {
		httpError(w, http.StatusNotFound, "trip not found")
		return
	}

	route, err := h.queries.GetTripRoute(ctx, tripID)
	if err != nil {
		log.Printf("classify trip %s: get route: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	input, err := plugin.BuildClassifierInput(trip, route)
	if err != nil {
		log.Printf("classify trip %s: build input: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "failed to build plugin input")
		return
	}

	output, err := h.host.Execute("trip-classifier", input)
	if err != nil {
		log.Printf("classify trip %s: execute: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (h *PluginHandler) ExportTrip(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	ctx := r.Context()

	// Get format from query parameter; default to "gpx".
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "gpx"
	}

	trip, err := h.queries.GetTrip(ctx, tripID)
	if err != nil {
		log.Printf("export trip %s: get trip: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if trip == nil {
		httpError(w, http.StatusNotFound, "trip not found")
		return
	}

	route, err := h.queries.GetTripRoute(ctx, tripID)
	if err != nil {
		log.Printf("export trip %s: get route: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	input, err := plugin.BuildExportInput(trip, route, format)
	if err != nil {
		log.Printf("export trip %s: build input: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "failed to build plugin input")
		return
	}

	output, err := h.host.Execute("export-transformer", input)
	if err != nil {
		log.Printf("export trip %s: execute: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (h *PluginHandler) ScoreRoute(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	ctx := r.Context()

	route, err := h.queries.GetTripRoute(ctx, tripID)
	if err != nil {
		log.Printf("score route %s: get route: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if len(route) == 0 {
		httpError(w, http.StatusNotFound, "no route data for trip")
		return
	}

	// For history, we pass an empty set; the caller can provide history via
	// the generic plugin runner if more sophisticated scoring is needed.
	input, err := plugin.BuildRouteScorerInput(route, nil, 0)
	if err != nil {
		log.Printf("score route %s: build input: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "failed to build plugin input")
		return
	}

	output, err := h.host.Execute("route-scorer", input)
	if err != nil {
		log.Printf("score route %s: execute: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (h *PluginHandler) DetectAnomalies(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	ctx := r.Context()

	route, err := h.queries.GetTripRoute(ctx, tripID)
	if err != nil {
		log.Printf("detect anomalies %s: get route: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if len(route) == 0 {
		httpError(w, http.StatusNotFound, "no route data for trip")
		return
	}

	input, err := plugin.BuildDataQualityInput(route)
	if err != nil {
		log.Printf("detect anomalies %s: build input: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "failed to build plugin input")
		return
	}

	output, err := h.host.Execute("data-quality-detector", input)
	if err != nil {
		log.Printf("detect anomalies %s: execute: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (h *PluginHandler) RedactTrip(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("id")
	ctx := r.Context()

	route, err := h.queries.GetTripRoute(ctx, tripID)
	if err != nil {
		log.Printf("redact trip %s: get route: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if len(route) == 0 {
		httpError(w, http.StatusNotFound, "no route data for trip")
		return
	}

	places, err := h.queries.ListPlaces(ctx)
	if err != nil {
		log.Printf("redact trip %s: list places: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	input, err := plugin.BuildRedactorInput(route, places)
	if err != nil {
		log.Printf("redact trip %s: build input: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, "failed to build plugin input")
		return
	}

	output, err := h.host.Execute("privacy-redactor", input)
	if err != nil {
		log.Printf("redact trip %s: execute: %v", tripID, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}
