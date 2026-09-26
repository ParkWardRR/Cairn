package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
	"github.com/ParkWardRR/Cairn/ingest/internal/handler"
	"github.com/ParkWardRR/Cairn/ingest/internal/mqtt"
	"github.com/ParkWardRR/Cairn/ingest/internal/storage"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// ── configuration from environment ──────────────────────────────────

	listenAddr := envOr("LISTEN_ADDR", ":8443")
	databaseURL := envOr("DATABASE_URL", "postgres://cairn@localhost/cairn")
	bundleDir := envOr("BUNDLE_DIR", "/data/bundles")
	mqttBroker := os.Getenv("MQTT_BROKER_URL")
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	// ── database ────────────────────────────────────────────────────────

	pool, err := db.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	// ── storage ─────────────────────────────────────────────────────────

	store, err := storage.New(bundleDir)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	// ── MQTT ────────────────────────────────────────────────────────────

	mqttPub := mqtt.New(mqttBroker)
	defer mqttPub.Close()

	// ── routes ──────────────────────────────────────────────────────────

	mux := http.NewServeMux()

	uploadHandler := handler.NewUploadHandler(queries, store)
	uploadHandler.SetMQTT(mqttPub)
	healthHandler := handler.NewHealthHandler(pool)
	apiHandler := handler.NewApiHandler(queries)

	// Ingest endpoints
	mux.HandleFunc("GET /api/v1/health", healthHandler.Health)
	mux.HandleFunc("POST /api/v1/upload/init", uploadHandler.Init)
	mux.HandleFunc("PUT /api/v1/upload/{id}/chunk", uploadHandler.Chunk)
	mux.HandleFunc("POST /api/v1/upload/{id}/finalize", uploadHandler.Finalize)
	mux.HandleFunc("GET /api/v1/devices/{id}/status", uploadHandler.DeviceStatus)
	mux.HandleFunc("POST /api/v1/upload/{id}/reparse", uploadHandler.Reparse)

	// Read API
	mux.HandleFunc("GET /api/v1/stats", apiHandler.Stats)
	mux.HandleFunc("GET /api/v1/trips", apiHandler.ListTrips)
	mux.HandleFunc("GET /api/v1/trips/{id}", apiHandler.GetTrip)
	mux.HandleFunc("GET /api/v1/trips/{id}/route", apiHandler.GetTripRoute)
	mux.HandleFunc("GET /api/v1/trips/{id}/events", apiHandler.GetTripEvents)
	mux.HandleFunc("GET /api/v1/trips/{id}/export/gpx", apiHandler.ExportGPX)
	mux.HandleFunc("GET /api/v1/trips/{id}/export/geojson", apiHandler.ExportGeoJSON)
	mux.HandleFunc("GET /api/v1/trips/{id}/export/csv", apiHandler.ExportCSV)
	mux.HandleFunc("DELETE /api/v1/trips/{id}", apiHandler.DeleteTrip)
	mux.HandleFunc("POST /api/v1/trips/{id}/tags", apiHandler.AddTag)
	mux.HandleFunc("DELETE /api/v1/trips/{id}/tags/{tag}", apiHandler.RemoveTag)
	mux.HandleFunc("GET /api/v1/devices", apiHandler.ListDevices)
	mux.HandleFunc("GET /api/v1/places", apiHandler.ListPlaces)
	mux.HandleFunc("POST /api/v1/places", apiHandler.CreatePlace)
	mux.HandleFunc("PUT /api/v1/places/{id}", apiHandler.UpdatePlace)
	mux.HandleFunc("DELETE /api/v1/places/{id}", apiHandler.DeletePlace)

	// ── server ──────────────────────────────────────────────────────────

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if tlsCert != "" && tlsKey != "" {
			srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			log.Printf("ingestd listening on %s (TLS)", listenAddr)
			if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error: %v", err)
			}
		} else {
			log.Printf("ingestd listening on %s (plaintext — development only)", listenAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error: %v", err)
			}
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("stopped")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Upload-Offset")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
