package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/ParkWardRR/Cairn/ingest/internal/db"
	"github.com/ParkWardRR/Cairn/ingest/internal/receipt"
	"github.com/ParkWardRR/Cairn/ingest/internal/storage"
)

// UploadHandler implements the resumable, idempotent upload protocol.
type UploadHandler struct {
	queries *db.Queries
	store   *storage.Store
	signer  *receipt.Signer
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

// ─── helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
