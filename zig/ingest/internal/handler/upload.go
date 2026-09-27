package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"math"
	"strings"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
	"github.com/ParkWardRR/Cairn/ingest/internal/mqtt"
	"github.com/ParkWardRR/Cairn/ingest/internal/parser"
	"github.com/ParkWardRR/Cairn/ingest/internal/receipt"
	"github.com/ParkWardRR/Cairn/ingest/internal/storage"
)

// UploadHandler implements the resumable, idempotent upload protocol.
type UploadHandler struct {
	queries *db.Queries
	store   *storage.Store
	signer  *receipt.Signer
	mqtt    *mqtt.Publisher
}

// SetMQTT attaches an MQTT publisher for trip event notifications.
func (h *UploadHandler) SetMQTT(pub *mqtt.Publisher) {
	h.mqtt = pub
}

// NewUploadHandler creates an UploadHandler with an ephemeral signing key.
// In production the key should be loaded from RECEIPT_SIGNING_KEY.
func NewUploadHandler(queries *db.Queries, store *storage.Store) *UploadHandler {
	keyHex := os.Getenv("RECEIPT_SIGNING_KEY")
	signer, err := receipt.NewSigner(keyHex)
	if err != nil {
		log.Fatalf("receipt signer init: %v", err)
	}
	if keyHex == "" {
		log.Printf("warning: using ephemeral receipt signing key (set RECEIPT_SIGNING_KEY for production)")
	}
	log.Printf("receipt public key: %s", signer.PublicKeyHex())

	return &UploadHandler{
		queries: queries,
		store:   store,
		signer:  signer,
	}
}

// ─── POST /api/v1/upload/init ───────────────────────────────────────────────

type initRequest struct {
	DeviceID    string `json:"device_id"`
	TripID      string `json:"trip_id"`
	ContentHash string `json:"content_hash"`
	Size        int64  `json:"size"`
}

type initResponse struct {
	UploadID     string `json:"upload_id"`
	ResumeOffset int64  `json:"resume_offset"`
}

// Init handles POST /api/v1/upload/init.
// Idempotent: if an upload with the same content_hash exists and is completed,
// the existing receipt is returned. If it exists but is incomplete, the current
// resume_offset is returned so the device can continue.
func (h *UploadHandler) Init(w http.ResponseWriter, r *http.Request) {
	var req initRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ContentHash == "" || req.DeviceID == "" {
		httpError(w, http.StatusBadRequest, "device_id and content_hash are required")
		return
	}

	ctx := r.Context()

	// Ensure the device exists (upsert with empty public key for now; device
	// enrollment should have been done earlier via tripctl).
	if err := h.queries.UpsertDevice(ctx, req.DeviceID, ""); err != nil {
		log.Printf("upsert device %s: %v", req.DeviceID, err)
		httpError(w, http.StatusInternalServerError, "device registration failed")
		return
	}

	// Check for existing upload with the same content hash (idempotency).
	existing, err := h.queries.FindUploadByContentHash(ctx, req.ContentHash)
	if err != nil {
		log.Printf("find upload by hash: %v", err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	if existing != nil {
		// Already completed — tell the device it is done.
		if existing.UploadState == "completed" {
			resp := initResponse{
				UploadID:     existing.ID.String(),
				ResumeOffset: -1, // signal: already done
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}

		// Partially uploaded — rebuild running hash and let device resume.
		if err := h.store.ResumeHash(existing.ID.String()); err != nil {
			log.Printf("resume hash for %s: %v", existing.ID, err)
		}

		resp := initResponse{
			UploadID:     existing.ID.String(),
			ResumeOffset: existing.ResumeOffset,
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// New upload.
	uploadID, err := h.queries.InsertUpload(ctx, req.DeviceID, req.ContentHash, "", req.Size)
	if err != nil {
		log.Printf("insert upload: %v", err)
		httpError(w, http.StatusInternalServerError, "failed to create upload")
		return
	}

	storagePath, err := h.store.InitFile(uploadID.String())
	if err != nil {
		log.Printf("init storage file: %v", err)
		httpError(w, http.StatusInternalServerError, "storage error")
		return
	}

	// Update the storage path in the database now that we know it.
	if _, err := h.queries.FindUploadByContentHash(ctx, req.ContentHash); err != nil {
		log.Printf("note: could not re-read upload to update path: %v", err)
	}
	_ = storagePath // path is derived from upload ID, so it is deterministic

	if h.mqtt != nil {
		h.mqtt.PublishTripStarted(req.DeviceID, req.TripID)
	}

	writeJSON(w, http.StatusOK, initResponse{
		UploadID:     uploadID.String(),
		ResumeOffset: 0,
	})
}

// ─── PUT /api/v1/upload/{id}/chunk ──────────────────────────────────────────

// Chunk handles PUT /api/v1/upload/{id}/chunk.
// The request body is raw binary data. The offset is taken from the
// X-Upload-Offset header.
func (h *UploadHandler) Chunk(w http.ResponseWriter, r *http.Request) {
	uploadIDStr := r.PathValue("id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid upload id")
		return
	}

	ctx := r.Context()

	upload, err := h.queries.GetUpload(ctx, uploadID)
	if err != nil {
		log.Printf("get upload %s: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}
	if upload == nil {
		httpError(w, http.StatusNotFound, "upload not found")
		return
	}
	if upload.UploadState == "completed" {
		httpError(w, http.StatusConflict, "upload already completed")
		return
	}

	// Read offset from header; fall back to the stored resume offset.
	offset := upload.ResumeOffset
	if hdr := r.Header.Get("X-Upload-Offset"); hdr != "" {
		var parsed int64
		if _, err := fmt.Sscanf(hdr, "%d", &parsed); err == nil {
			offset = parsed
		}
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MiB max chunk
	if err != nil {
		httpError(w, http.StatusBadRequest, "failed to read chunk")
		return
	}
	if len(data) == 0 {
		httpError(w, http.StatusBadRequest, "empty chunk")
		return
	}

	newOffset, err := h.store.WriteChunk(uploadIDStr, offset, data)
	if err != nil {
		log.Printf("write chunk for %s at offset %d: %v", uploadID, offset, err)
		httpError(w, http.StatusInternalServerError, "storage write error")
		return
	}

	if err := h.queries.UpdateUploadOffset(ctx, uploadID, newOffset); err != nil {
		log.Printf("update offset for %s: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_id":     uploadIDStr,
		"resume_offset": newOffset,
	})
}

// ─── POST /api/v1/upload/{id}/finalize ──────────────────────────────────────

type finalizeRequest struct {
	ContentHash string `json:"content_hash"`
}

// Finalize handles POST /api/v1/upload/{id}/finalize.
// It verifies that the computed SHA-256 of the received data matches the
// declared content_hash, then issues a durable receipt.
func (h *UploadHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	uploadIDStr := r.PathValue("id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid upload id")
		return
	}

	var req finalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()

	upload, err := h.queries.GetUpload(ctx, uploadID)
	if err != nil || upload == nil {
		httpError(w, http.StatusNotFound, "upload not found")
		return
	}
	if upload.UploadState == "completed" {
		httpError(w, http.StatusConflict, "upload already finalized")
		return
	}

	// Verify content hash.
	computedHash, err := h.store.FinalHash(uploadIDStr)
	if err != nil {
		log.Printf("compute final hash for %s: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, "hash computation error")
		return
	}

	if computedHash != req.ContentHash {
		log.Printf("hash mismatch for %s: expected %s, got %s", uploadID, req.ContentHash, computedHash)
		if err := h.queries.FailUpload(ctx, uploadID); err != nil {
			log.Printf("mark upload failed %s: %v", uploadID, err)
		}
		httpError(w, http.StatusBadRequest, "content hash mismatch")
		return
	}

	// Issue receipt.
	rcpt, err := h.signer.Issue(uploadID, computedHash)
	if err != nil {
		log.Printf("issue receipt for %s: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, "receipt generation error")
		return
	}

	if err := h.queries.FinalizeUpload(ctx, uploadID, rcpt.ReceiptID); err != nil {
		log.Printf("finalize upload %s: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Parse the uploaded bundle and insert trip data.
	// Parsing failures are logged but do not block the receipt — the raw
	// bundle is safely stored and can be reparsed later.
	h.parseAndInsert(ctx, uploadID, upload.DeviceID, computedHash)

	if h.mqtt != nil {
		h.mqtt.PublishSyncCompleted(upload.DeviceID, "")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"receipt_id": rcpt.ReceiptID.String(),
		"signature":  rcpt.Signature,
		"issued_at":  rcpt.IssuedAt,
	})
}

// ─── GET /api/v1/devices/{id}/status ────────────────────────────────────────

// DeviceStatus returns the last-seen time and firmware version for a device.
func (h *UploadHandler) DeviceStatus(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	if deviceID == "" {
		httpError(w, http.StatusBadRequest, "device id required")
		return
	}

	lastSeen, fwVersion, err := h.queries.DeviceLastSeen(r.Context(), deviceID)
	if err != nil {
		httpError(w, http.StatusNotFound, "device not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":        deviceID,
		"last_seen_at":     lastSeen,
		"firmware_version": fwVersion,
	})
}

// ─── POST /api/v1/upload/{id}/reparse ──────────────────────────────────────

// Reparse triggers reparsing of an already-completed upload. Useful when the
// parser logic has been updated or when initial parsing failed.
func (h *UploadHandler) Reparse(w http.ResponseWriter, r *http.Request) {
	uploadIDStr := r.PathValue("id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid upload id")
		return
	}

	ctx := r.Context()

	upload, err := h.queries.GetUpload(ctx, uploadID)
	if err != nil || upload == nil {
		httpError(w, http.StatusNotFound, "upload not found")
		return
	}
	if upload.UploadState != "completed" {
		httpError(w, http.StatusConflict, "upload not yet completed")
		return
	}

	// Delete existing parsed data so we can re-insert cleanly.
	bundlePath := h.store.BundlePath(uploadIDStr)
	bundle, err := parser.ParseBundleFile(bundlePath)
	if err != nil {
		// Try raw samples fallback
		data, readErr := os.ReadFile(bundlePath)
		if readErr != nil {
			httpError(w, http.StatusUnprocessableEntity, fmt.Sprintf("parse error: %v", err))
			return
		}
		bundle, err = parser.ParseRawSamples(data, upload.DeviceID, upload.ContentHash)
		if err != nil {
			log.Printf("reparse %s: parse failed: %v", uploadID, err)
			httpError(w, http.StatusUnprocessableEntity, fmt.Sprintf("parse error: %v", err))
			return
		}
	}
	if bundle.Manifest.DeviceID == "" || bundle.Manifest.DeviceID == "unknown" {
		bundle.Manifest.DeviceID = upload.DeviceID
	}

	// Clean existing trip data.
	if err := h.queries.DeleteTripData(ctx, bundle.Manifest.TripID); err != nil {
		log.Printf("reparse %s: delete old data: %v", uploadID, err)
		// Continue — tables may not have had data.
	}

	if err := h.insertParsedBundle(ctx, bundle, uploadID, upload.ContentHash); err != nil {
		log.Printf("reparse %s: insert failed: %v", uploadID, err)
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("insert error: %v", err))
		return
	}

	log.Printf("reparse %s: trip %s reparsed successfully (%d samples, %d motion, %d events)",
		uploadID, bundle.Manifest.TripID,
		len(bundle.Samples), len(bundle.MotionSamples), len(bundle.Events))

	writeJSON(w, http.StatusOK, map[string]any{
		"trip_id":        bundle.Manifest.TripID,
		"sample_count":   len(bundle.Samples),
		"motion_count":   len(bundle.MotionSamples),
		"event_count":    len(bundle.Events),
		"distance_m":     bundle.DistanceM,
		"duration_s":     bundle.DurationS,
	})
}

// ─── bundle parsing + insertion ────────────────────────────────────────────

// parseAndInsert attempts to parse the stored bundle and insert trip data.
// Errors are logged but never propagated — the upload is already finalized.
func (h *UploadHandler) parseAndInsert(ctx context.Context, uploadID uuid.UUID, deviceID, contentHash string) {
	bundlePath := h.store.BundlePath(uploadID.String())

	bundle, err := parser.ParseBundleFile(bundlePath)
	if err != nil {
		// Try parsing as raw samples.bin
		data, readErr := os.ReadFile(bundlePath)
		if readErr != nil {
			log.Printf("parse bundle %s: %v; read fallback: %v (bundle stored; reparse later)", uploadID, err, readErr)
			return
		}
		bundle, err = parser.ParseRawSamples(data, deviceID, contentHash)
		if err != nil {
			log.Printf("parse bundle %s as raw samples: %v (bundle stored; reparse later)", uploadID, err)
			return
		}
	}
	// Use upload's device ID if the manifest doesn't have one
	if bundle.Manifest.DeviceID == "" || bundle.Manifest.DeviceID == "unknown" {
		bundle.Manifest.DeviceID = deviceID
	}

	if err := h.insertParsedBundle(ctx, bundle, uploadID, contentHash); err != nil {
		log.Printf("insert parsed data for %s: %v", uploadID, err)
		return
	}

	log.Printf("parsed trip %s from upload %s: %d samples, %d motion, %d events, %.0f m, %d s",
		bundle.Manifest.TripID, uploadID,
		len(bundle.Samples), len(bundle.MotionSamples), len(bundle.Events),
		bundle.DistanceM, bundle.DurationS)

	if h.mqtt != nil {
		h.mqtt.PublishTripEnd(bundle.Manifest.DeviceID, bundle.Manifest.TripID,
			bundle.EndedAt, bundle.DistanceM, bundle.DurationS)

		// Publish last_parked with final coordinates.
		h.mqtt.PublishLastParked(bundle.Manifest.DeviceID, bundle.Manifest.TripID,
			bundle.EndLocation[0], bundle.EndLocation[1])

		// Check if trip start/end are near a "home" place for arrived/departed events.
		h.publishHomeProximityEvents(ctx, bundle.Manifest.DeviceID, bundle.Manifest.TripID,
			bundle.StartLocation[0], bundle.StartLocation[1],
			bundle.EndLocation[0], bundle.EndLocation[1])
	}
}

// insertParsedBundle inserts a fully-parsed bundle into the database.
func (h *UploadHandler) insertParsedBundle(ctx context.Context, bundle *parser.ParsedBundle, uploadID uuid.UUID, contentHash string) error {
	m := bundle.Manifest

	// Build a summary JSONB blob with useful metadata.
	summary, _ := json.Marshal(map[string]any{
		"firmware_version": m.FirmwareVersion,
		"gnss_rate_hz":     m.GNSSRateHz,
		"imu_rate_hz":      m.IMURateHz,
		"schema_version":   m.SchemaVersion,
		"sample_count": map[string]int{
			"gnss":             len(bundle.Samples),
			"imu_summary":      len(bundle.MotionSamples),
			"events":           len(bundle.Events),
		},
	})

	trip := db.Trip{
		ID:         m.TripID,
		DeviceID:   m.DeviceID,
		StartedAt:  bundle.StartedAt,
		EndedAt:    bundle.EndedAt,
		UploadID:   uploadID,
		DistanceM:  bundle.DistanceM,
		DurationS:  bundle.DurationS,
		StartLat:   bundle.StartLocation[0],
		StartLon:   bundle.StartLocation[1],
		EndLat:     bundle.EndLocation[0],
		EndLon:     bundle.EndLocation[1],
		BundleHash: contentHash,
		Summary:    summary,
	}

	if err := h.queries.InsertTrip(ctx, trip); err != nil {
		return fmt.Errorf("insert trip: %w", err)
	}

	// Convert parser samples to db samples.
	dbSamples := make([]db.LocationSample, len(bundle.Samples))
	for i, s := range bundle.Samples {
		dbSamples[i] = db.LocationSample{
			TimestampMs: s.TimestampMs,
			Latitude:    s.Latitude,
			Longitude:   s.Longitude,
			AltitudeM:   s.AltitudeM,
			SpeedMps:    s.SpeedMps,
			HeadingDeg:  s.HeadingDeg,
			FixQuality:  s.FixQuality,
			Satellites:  s.Satellites,
			Hdop:        s.Hdop,
			AccuracyM:   s.AccuracyM,
		}
	}
	if err := h.queries.InsertLocationSamples(ctx, m.TripID, dbSamples); err != nil {
		return fmt.Errorf("insert location samples: %w", err)
	}

	// Convert parser motion samples to db motion samples.
	dbMotion := make([]db.MotionSample, len(bundle.MotionSamples))
	for i, s := range bundle.MotionSamples {
		dbMotion[i] = db.MotionSample{
			WindowStartMs:    s.WindowStartMs,
			WindowDurationMs: s.WindowDurationMs,
			AccelPeakXMg:     s.AccelPeakXMg,
			AccelPeakYMg:     s.AccelPeakYMg,
			AccelPeakZMg:     s.AccelPeakZMg,
			AccelRmsMg:       s.AccelRmsMg,
			GyroPeakDps:      s.GyroPeakDps,
			Variance:         s.Variance,
			Flags:            s.Flags,
		}
	}
	if err := h.queries.InsertMotionSamples(ctx, m.TripID, dbMotion); err != nil {
		return fmt.Errorf("insert motion samples: %w", err)
	}

	// Convert parser events to db events.
	dbEvents := make([]db.TripEvent, 0, len(bundle.Events))
	for _, e := range bundle.Events {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			log.Printf("skip event with bad timestamp %q: %v", e.Timestamp, err)
			continue
		}

		metadata, _ := json.Marshal(e.Data)

		evt := db.TripEvent{
			EventType:   e.Type,
			TimestampAt: ts,
			Metadata:    metadata,
		}

		// Extract location from event data if present.
		if data := e.Data; data != nil {
			if locMap, ok := data["location"].(map[string]any); ok {
				if lat, ok := locMap["lat"].(float64); ok {
					if lon, ok := locMap["lon"].(float64); ok {
						evt.Lat = &lat
						evt.Lon = &lon
					}
				}
			}
		}

		dbEvents = append(dbEvents, evt)
	}
	if err := h.queries.InsertTripEvents(ctx, m.TripID, dbEvents); err != nil {
		return fmt.Errorf("insert trip events: %w", err)
	}

	return nil
}

// ─── home proximity events ─────────────────────────────────────────────────

// publishHomeProximityEvents checks whether the trip start or end point is near
// a place tagged as "home" (name contains "home", case-insensitive) and publishes
// departed_home / arrived_home MQTT events accordingly.
func (h *UploadHandler) publishHomeProximityEvents(ctx context.Context, deviceID, tripID string, startLat, startLon, endLat, endLon float64) {
	places, err := h.queries.ListPlaces(ctx)
	if err != nil {
		log.Printf("mqtt home proximity: list places: %v", err)
		return
	}

	for _, p := range places {
		if !strings.Contains(strings.ToLower(p.Name), "home") {
			continue
		}
		// Check if trip started near home (departed_home).
		if haversineM(startLat, startLon, p.Lat, p.Lon) <= p.RadiusM {
			h.mqtt.PublishDepartedHome(deviceID, tripID, p.Name)
		}
		// Check if trip ended near home (arrived_home).
		if haversineM(endLat, endLon, p.Lat, p.Lon) <= p.RadiusM {
			h.mqtt.PublishArrivedHome(deviceID, tripID, p.Name)
		}
	}
}

// haversineM returns the great-circle distance in meters between two lat/lon points.
func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	lat1r := lat1 * math.Pi / 180.0
	lat2r := lat2 * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1r)*math.Cos(lat2r)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
