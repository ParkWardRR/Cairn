package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool so callers can health-check it.
type Pool = pgxpool.Pool

// Connect creates a connection pool to the database.
func Connect(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// Queries provides database operations.
type Queries struct {
	pool *Pool
}

// New returns a Queries instance backed by pool.
func New(pool *Pool) *Queries {
	return &Queries{pool: pool}
}

// ─── Device operations ──────────────────────────────────────────────────────

// UpsertDevice inserts a device or updates its last_seen_at.
func (q *Queries) UpsertDevice(ctx context.Context, id, publicKey string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO devices (id, public_key, last_seen_at)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE
			SET last_seen_at = now(),
			    updated_at   = now()
	`, id, publicKey)
	return err
}

// DeviceLastSeen returns the last-seen timestamp for a device.
func (q *Queries) DeviceLastSeen(ctx context.Context, deviceID string) (time.Time, string, error) {
	var lastSeen time.Time
	var fwVersion *string
	err := q.pool.QueryRow(ctx, `
		SELECT last_seen_at, firmware_version
		FROM devices WHERE id = $1
	`, deviceID).Scan(&lastSeen, &fwVersion)
	if err != nil {
		return time.Time{}, "", err
	}
	fw := ""
	if fwVersion != nil {
		fw = *fwVersion
	}
	return lastSeen, fw, nil
}

// ─── Upload operations ─────────────────────────────────────────────────────

// Upload represents a row in the uploads table.
type Upload struct {
	ID           uuid.UUID
	DeviceID     string
	ContentHash  string
	UploadState  string
	ReceiptID    *uuid.UUID
	StoragePath  *string
	FileSize     *int64
	ResumeOffset int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FindUploadByContentHash returns an existing upload for the given content hash, if any.
func (q *Queries) FindUploadByContentHash(ctx context.Context, contentHash string) (*Upload, error) {
	u := &Upload{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, device_id, content_hash, upload_state, receipt_id,
		       storage_path, file_size, resume_offset, created_at, updated_at
		FROM uploads WHERE content_hash = $1
	`, contentHash).Scan(
		&u.ID, &u.DeviceID, &u.ContentHash, &u.UploadState, &u.ReceiptID,
		&u.StoragePath, &u.FileSize, &u.ResumeOffset, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// InsertUpload creates a new upload row.
func (q *Queries) InsertUpload(ctx context.Context, deviceID, contentHash, storagePath string, fileSize int64) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.pool.QueryRow(ctx, `
		INSERT INTO uploads (device_id, content_hash, storage_path, file_size, upload_state)
		VALUES ($1, $2, $3, $4, 'initiated')
		RETURNING id
	`, deviceID, contentHash, storagePath, fileSize).Scan(&id)
	return id, err
}

// GetUpload fetches an upload by ID.
func (q *Queries) GetUpload(ctx context.Context, id uuid.UUID) (*Upload, error) {
	u := &Upload{}
	err := q.pool.QueryRow(ctx, `
		SELECT id, device_id, content_hash, upload_state, receipt_id,
		       storage_path, file_size, resume_offset, created_at, updated_at
		FROM uploads WHERE id = $1
	`, id).Scan(
		&u.ID, &u.DeviceID, &u.ContentHash, &u.UploadState, &u.ReceiptID,
		&u.StoragePath, &u.FileSize, &u.ResumeOffset, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// UpdateUploadOffset updates the resume offset and marks state as uploading.
func (q *Queries) UpdateUploadOffset(ctx context.Context, id uuid.UUID, offset int64) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE uploads
		SET resume_offset = $2,
		    upload_state  = 'uploading',
		    updated_at    = now()
		WHERE id = $1
	`, id, offset)
	return err
}

// FinalizeUpload marks an upload as completed and records its receipt.
func (q *Queries) FinalizeUpload(ctx context.Context, id, receiptID uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE uploads
		SET upload_state = 'completed',
		    receipt_id   = $2,
		    updated_at   = now()
		WHERE id = $1
	`, id, receiptID)
	return err
}

// FailUpload marks an upload as failed.
func (q *Queries) FailUpload(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE uploads
		SET upload_state = 'failed',
		    updated_at   = now()
		WHERE id = $1
	`, id)
	return err
}

// ─── Trip operations ────────────────────────────────────────────────────────

// InsertTrip inserts a trip record.
func (q *Queries) InsertTrip(ctx context.Context, id, deviceID string, startedAt time.Time, uploadID uuid.UUID, bundleHash string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO trips (id, device_id, started_at, upload_id, bundle_hash)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, id, deviceID, startedAt, uploadID, bundleHash)
	return err
}
