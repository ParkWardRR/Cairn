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

// Trip holds all the fields for inserting a parsed trip.
type Trip struct {
	ID            string
	DeviceID      string
	StartedAt     time.Time
	EndedAt       time.Time
	UploadID      uuid.UUID
	DistanceM     float64
	DurationS     int
	StartLat      float64
	StartLon      float64
	EndLat        float64
	EndLon        float64
	BundleHash    string
	Summary       []byte // JSONB
}

// InsertTrip inserts a complete trip record with computed summaries.
func (q *Queries) InsertTrip(ctx context.Context, t Trip) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO trips (id, device_id, started_at, ended_at, upload_id,
		                   distance_m, duration_s,
		                   start_location, end_location,
		                   bundle_hash, summary)
		VALUES ($1, $2, $3, $4, $5,
		        $6, $7,
		        ST_SetSRID(ST_MakePoint($8, $9), 4326)::geography,
		        ST_SetSRID(ST_MakePoint($10, $11), 4326)::geography,
		        $12, $13)
		ON CONFLICT (id) DO UPDATE SET
		    ended_at       = EXCLUDED.ended_at,
		    distance_m     = EXCLUDED.distance_m,
		    duration_s     = EXCLUDED.duration_s,
		    start_location = EXCLUDED.start_location,
		    end_location   = EXCLUDED.end_location,
		    summary        = EXCLUDED.summary
	`, t.ID, t.DeviceID, t.StartedAt, t.EndedAt, t.UploadID,
		t.DistanceM, t.DurationS,
		t.StartLon, t.StartLat, // ST_MakePoint takes (lon, lat)
		t.EndLon, t.EndLat,
		t.BundleHash, t.Summary,
	)
	return err
}

// LocationSample represents a single GNSS fix for batch insertion.
type LocationSample struct {
	TimestampMs uint64
	Latitude    float64
	Longitude   float64
	AltitudeM   float64
	SpeedMps    float64
	HeadingDeg  float64
	FixQuality  int
	Satellites  int
	Hdop        float64
	AccuracyM   float64
}

// InsertLocationSamples batch-inserts GNSS samples for a trip.
// Uses CopyFrom for the scalar columns, then a single UPDATE to populate
// the PostGIS geography column from lat/lon (CopyFrom can't handle geography).
func (q *Queries) InsertLocationSamples(ctx context.Context, tripID string, samples []LocationSample) error {
	if len(samples) == 0 {
		return nil
	}

	_, err := q.pool.CopyFrom(
		ctx,
		pgx.Identifier{"location_samples"},
		[]string{
			"trip_id", "timestamp_ms", "latitude", "longitude",
			"altitude_m", "speed_mps", "heading_deg",
			"fix_quality", "satellites", "hdop", "accuracy_m",
		},
		&locationSampleSource{tripID: tripID, samples: samples},
	)
	if err != nil {
		return fmt.Errorf("copy samples: %w", err)
	}

	// Backfill the PostGIS geography column from lat/lon
	_, err = q.pool.Exec(ctx, `
		UPDATE location_samples
		SET location = ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
		WHERE trip_id = $1 AND location IS NULL
	`, tripID)
	if err != nil {
		return fmt.Errorf("backfill geography: %w", err)
	}

	return nil
}

type locationSampleSource struct {
	tripID  string
	samples []LocationSample
	idx     int
}

func (s *locationSampleSource) Next() bool {
	return s.idx < len(s.samples)
}

func (s *locationSampleSource) Values() ([]any, error) {
	r := s.samples[s.idx]
	s.idx++
	return []any{
		s.tripID,
		int64(r.TimestampMs),
		r.Latitude,
		r.Longitude,
		r.AltitudeM,
		r.SpeedMps,
		r.HeadingDeg,
		int16(r.FixQuality),
		int16(r.Satellites),
		r.Hdop,
		r.AccuracyM,
	}, nil
}

func (s *locationSampleSource) Err() error { return nil }

// MotionSample represents a single IMU summary window for batch insertion.
type MotionSample struct {
	WindowStartMs    uint64
	WindowDurationMs int
	AccelPeakXMg     int16
	AccelPeakYMg     int16
	AccelPeakZMg     int16
	AccelRmsMg       uint16
	GyroPeakDps      int16
	Variance         uint16
	Flags            uint8
}

// InsertMotionSamples batch-inserts IMU summary records for a trip.
func (q *Queries) InsertMotionSamples(ctx context.Context, tripID string, samples []MotionSample) error {
	if len(samples) == 0 {
		return nil
	}

	_, err := q.pool.CopyFrom(
		ctx,
		pgx.Identifier{"motion_samples"},
		[]string{
			"trip_id", "window_start_ms", "window_duration_ms",
			"accel_peak_x_mg", "accel_peak_y_mg", "accel_peak_z_mg",
			"accel_rms_mg", "gyro_peak_dps", "variance", "flags",
		},
		&motionSampleSource{tripID: tripID, samples: samples},
	)
	return err
}

type motionSampleSource struct {
	tripID  string
	samples []MotionSample
	idx     int
}

func (s *motionSampleSource) Next() bool {
	return s.idx < len(s.samples)
}

func (s *motionSampleSource) Values() ([]any, error) {
	r := s.samples[s.idx]
	s.idx++
	return []any{
		s.tripID,
		int64(r.WindowStartMs),
		int32(r.WindowDurationMs),
		r.AccelPeakXMg,
		r.AccelPeakYMg,
		r.AccelPeakZMg,
		int16(r.AccelRmsMg),
		r.GyroPeakDps,
		int16(r.Variance),
		int16(r.Flags),
	}, nil
}

func (s *motionSampleSource) Err() error { return nil }

// TripEvent represents a single trip event for insertion.
type TripEvent struct {
	EventType   string
	TimestampAt time.Time
	Lat         *float64 // nil if no location
	Lon         *float64
	Metadata    []byte // JSONB
}

// InsertTripEvents inserts all events for a trip.
func (q *Queries) InsertTripEvents(ctx context.Context, tripID string, events []TripEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Use a batch for events (typically a small number per trip).
	batch := &pgx.Batch{}
	for _, e := range events {
		if e.Lat != nil && e.Lon != nil {
			batch.Queue(`
				INSERT INTO trip_events (trip_id, event_type, timestamp_at, location, metadata)
				VALUES ($1, $2, $3,
				        ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
				        $6)
			`, tripID, e.EventType, e.TimestampAt, *e.Lon, *e.Lat, e.Metadata)
		} else {
			batch.Queue(`
				INSERT INTO trip_events (trip_id, event_type, timestamp_at, metadata)
				VALUES ($1, $2, $3, $4)
			`, tripID, e.EventType, e.TimestampAt, e.Metadata)
		}
	}

	br := q.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert event %d: %w", i, err)
		}
	}
	return nil
}

// DeleteTripData removes all parsed data for a trip so it can be reparsed.
func (q *Queries) DeleteTripData(ctx context.Context, tripID string) error {
	// Delete in FK order: events, motion_samples, location_samples, then trip.
	for _, table := range []string{"trip_events", "motion_samples", "location_samples", "trips"} {
		col := "trip_id"
		if table == "trips" {
			col = "id"
		}
		if _, err := q.pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, col), tripID); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	return nil
}
