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
	"github.com/ParkWardRR/Cairn/ingest/internal/storage"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// ── configuration from environment ──────────────────────────────────

	listenAddr := envOr("LISTEN_ADDR", ":8443")
	databaseURL := envOr("DATABASE_URL", "postgres://cairn@localhost/cairn")
	bundleDir := envOr("BUNDLE_DIR", "/data/bundles")
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

	// ── routes ──────────────────────────────────────────────────────────

	mux := http.NewServeMux()

	uploadHandler := handler.NewUploadHandler(queries, store)
	healthHandler := handler.NewHealthHandler(pool)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Health)
	mux.HandleFunc("POST /api/v1/upload/init", uploadHandler.Init)
	mux.HandleFunc("PUT /api/v1/upload/{id}/chunk", uploadHandler.Chunk)
	mux.HandleFunc("POST /api/v1/upload/{id}/finalize", uploadHandler.Finalize)
	mux.HandleFunc("GET /api/v1/devices/{id}/status", uploadHandler.DeviceStatus)

	// ── server ──────────────────────────────────────────────────────────

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
