package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
)

// HealthHandler serves the health-check endpoint.
type HealthHandler struct {
	pool *db.Pool
}

// NewHealthHandler returns a HealthHandler.
func NewHealthHandler(pool *db.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// Health responds with the service and database status.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbOK := true
	if err := h.pool.Ping(ctx); err != nil {
		dbOK = false
	}

	status := "ok"
	code := http.StatusOK
	if !dbOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    status,
		"service":   "ingestd",
		"database":  dbOK,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
