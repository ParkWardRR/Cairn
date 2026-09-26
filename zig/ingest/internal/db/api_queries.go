package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ─── Stats ─────────────────────────────────────────────────────────────────

type Stats struct {
	TotalTrips     int     `json:"total_trips"`
	TotalDistanceM float64 `json:"total_distance_m"`
	TotalDurationS int     `json:"total_duration_s"`
	DeviceCount    int     `json:"device_count"`
}

func (q *Queries) GetStats(ctx context.Context) (*Stats, error) {
	s := &Stats{}
	err := q.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT count(*) FROM trips), 0),
			COALESCE((SELECT sum(distance_m) FROM trips), 0),
			COALESCE((SELECT sum(duration_s) FROM trips), 0),
			COALESCE((SELECT count(*) FROM devices), 0)
	`).Scan(&s.TotalTrips, &s.TotalDistanceM, &s.TotalDurationS, &s.DeviceCount)
	return s, err
}

// ─── Trip listing ──────────────────────────────────────────────────────────

type TripRow struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"device_id"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	DistanceM  float64   `json:"distance_m"`
	DurationS  int       `json:"duration_s"`
	StartLat   float64   `json:"start_lat"`
	StartLon   float64   `json:"start_lon"`
	EndLat     float64   `json:"end_lat"`
	EndLon     float64   `json:"end_lon"`
	BundleHash string    `json:"bundle_hash"`
}

type TripListResult struct {
	Trips []TripRow `json:"trips"`
	Total int       `json:"total"`
}

func (q *Queries) ListTrips(ctx context.Context, limit, offset int, deviceID, tag string, from, to *time.Time) (*TripListResult, error) {
	where := "WHERE 1=1"
	args := []any{}
	argN := 0

	nextArg := func(v any) string {
		argN++
		args = append(args, v)
		return fmt.Sprintf("$%d", argN)
	}

	if deviceID != "" {
		where += " AND t.device_id = " + nextArg(deviceID)
	}
	if tag != "" {
		where += " AND EXISTS (SELECT 1 FROM trip_tags tt WHERE tt.trip_id = t.id AND tt.tag = " + nextArg(tag) + ")"
	}
	if from != nil {
		where += " AND t.started_at >= " + nextArg(*from)
	}
	if to != nil {
		where += " AND t.started_at <= " + nextArg(*to)
	}

	var total int
	err := q.pool.QueryRow(ctx, "SELECT count(*) FROM trips t "+where, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count trips: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.device_id, t.started_at, t.ended_at,
		       t.distance_m, t.duration_s,
		       ST_Y(t.start_location::geometry), ST_X(t.start_location::geometry),
		       ST_Y(t.end_location::geometry), ST_X(t.end_location::geometry),
		       t.bundle_hash
		FROM trips t %s
		ORDER BY t.started_at DESC
		LIMIT %s OFFSET %s
	`, where, nextArg(limit), nextArg(offset))

	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trips: %w", err)
	}
	defer rows.Close()

	var trips []TripRow
	for rows.Next() {
		var t TripRow
		if err := rows.Scan(&t.ID, &t.DeviceID, &t.StartedAt, &t.EndedAt,
			&t.DistanceM, &t.DurationS,
			&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
			&t.BundleHash); err != nil {
			return nil, fmt.Errorf("scan trip: %w", err)
		}
		trips = append(trips, t)
	}
	if trips == nil {
		trips = []TripRow{}
	}

	return &TripListResult{Trips: trips, Total: total}, nil
}

// ─── Trip detail ───────────────────────────────────────────────────────────

type TripDetail struct {
	TripRow
	Summary []byte   `json:"summary"`
	Tags    []string `json:"tags"`
}

func (q *Queries) GetTrip(ctx context.Context, tripID string) (*TripDetail, error) {
	t := &TripDetail{}
	err := q.pool.QueryRow(ctx, `
		SELECT t.id, t.device_id, t.started_at, t.ended_at,
		       t.distance_m, t.duration_s,
		       ST_Y(t.start_location::geometry), ST_X(t.start_location::geometry),
		       ST_Y(t.end_location::geometry), ST_X(t.end_location::geometry),
		       t.bundle_hash, COALESCE(t.summary, '{}'::jsonb)
		FROM trips t WHERE t.id = $1
	`, tripID).Scan(&t.ID, &t.DeviceID, &t.StartedAt, &t.EndedAt,
		&t.DistanceM, &t.DurationS,
		&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
		&t.BundleHash, &t.Summary)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tags, err := q.GetTripTags(ctx, tripID)
	if err != nil {
		return nil, err
	}
	t.Tags = tags
	return t, nil
}

// ─── Trip route (GeoJSON) ──────────────────────────────────────────────────

type RoutePoint struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltitudeM  float64 `json:"altitude_m"`
	SpeedMps   float64 `json:"speed_mps"`
	TimestampMs int64  `json:"timestamp_ms"`
}

func (q *Queries) GetTripRoute(ctx context.Context, tripID string) ([]RoutePoint, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT latitude, longitude, altitude_m, speed_mps, timestamp_ms
		FROM location_samples
		WHERE trip_id = $1
		ORDER BY timestamp_ms ASC
	`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pts []RoutePoint
	for rows.Next() {
		var p RoutePoint
		if err := rows.Scan(&p.Lat, &p.Lon, &p.AltitudeM, &p.SpeedMps, &p.TimestampMs); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	if pts == nil {
		pts = []RoutePoint{}
	}
	return pts, nil
}

// ─── Trip events ───────────────────────────────────────────────────────────

type TripEventRow struct {
	EventType   string    `json:"event_type"`
	TimestampAt time.Time `json:"timestamp_at"`
	Lat         *float64  `json:"lat,omitempty"`
	Lon         *float64  `json:"lon,omitempty"`
	Metadata    []byte    `json:"metadata"`
}

func (q *Queries) GetTripEvents(ctx context.Context, tripID string) ([]TripEventRow, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT event_type, timestamp_at,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       COALESCE(metadata, '{}'::jsonb)
		FROM trip_events
		WHERE trip_id = $1
		ORDER BY timestamp_at ASC
	`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TripEventRow
	for rows.Next() {
		var e TripEventRow
		if err := rows.Scan(&e.EventType, &e.TimestampAt, &e.Lat, &e.Lon, &e.Metadata); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if events == nil {
		events = []TripEventRow{}
	}
	return events, nil
}

// ─── Tags ──────────────────────────────────────────────────────────────────

func (q *Queries) GetTripTags(ctx context.Context, tripID string) ([]string, error) {
	rows, err := q.pool.Query(ctx, `SELECT tag FROM trip_tags WHERE trip_id = $1 ORDER BY tag`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (q *Queries) AddTag(ctx context.Context, tripID, tag string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO trip_tags (trip_id, tag) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, tripID, tag)
	return err
}

func (q *Queries) RemoveTag(ctx context.Context, tripID, tag string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM trip_tags WHERE trip_id = $1 AND tag = $2`, tripID, tag)
	return err
}

// ─── Devices ───────────────────────────────────────────────────────────────

type DeviceRow struct {
	ID              string    `json:"id"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	FirmwareVersion string    `json:"firmware_version"`
	TripCount       int       `json:"trip_count"`
	TotalDistanceM  float64   `json:"total_distance_m"`
}

func (q *Queries) ListDevices(ctx context.Context) ([]DeviceRow, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT d.id, d.last_seen_at, COALESCE(d.firmware_version, ''),
		       COALESCE(tc.cnt, 0), COALESCE(tc.dist, 0)
		FROM devices d
		LEFT JOIN (
			SELECT device_id, count(*) AS cnt, sum(distance_m) AS dist
			FROM trips GROUP BY device_id
		) tc ON tc.device_id = d.id
		ORDER BY d.last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []DeviceRow
	for rows.Next() {
		var d DeviceRow
		if err := rows.Scan(&d.ID, &d.LastSeenAt, &d.FirmwareVersion, &d.TripCount, &d.TotalDistanceM); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []DeviceRow{}
	}
	return devices, nil
}

// ─── Places ────────────────────────────────────────────────────────────────

type PlaceRow struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	RadiusM    float64   `json:"radius_m"`
	VisitCount int       `json:"visit_count"`
}

func (q *Queries) ListPlaces(ctx context.Context) ([]PlaceRow, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT p.id, p.name,
		       ST_Y(p.location::geometry), ST_X(p.location::geometry),
		       p.radius_m,
		       COALESCE(vc.cnt, 0)
		FROM places p
		LEFT JOIN (
			SELECT p2.id AS place_id, count(DISTINCT t.id) AS cnt
			FROM places p2
			JOIN trips t ON ST_DWithin(t.end_location, p2.location, p2.radius_m)
			GROUP BY p2.id
		) vc ON vc.place_id = p.id
		ORDER BY p.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var places []PlaceRow
	for rows.Next() {
		var p PlaceRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Lat, &p.Lon, &p.RadiusM, &p.VisitCount); err != nil {
			return nil, err
		}
		places = append(places, p)
	}
	if places == nil {
		places = []PlaceRow{}
	}
	return places, nil
}

func (q *Queries) CreatePlace(ctx context.Context, name string, lat, lon, radiusM float64) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.pool.QueryRow(ctx, `
		INSERT INTO places (name, location, radius_m)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4)
		RETURNING id
	`, name, lon, lat, radiusM).Scan(&id)
	return id, err
}

func (q *Queries) UpdatePlace(ctx context.Context, id uuid.UUID, name string, lat, lon, radiusM float64) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE places
		SET name = $2, location = ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, radius_m = $5
		WHERE id = $1
	`, id, name, lon, lat, radiusM)
	return err
}

func (q *Queries) DeletePlace(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM places WHERE id = $1`, id)
	return err
}

// ─── Trip deletion ─────────────────────────────────────────────────────────

func (q *Queries) FullDeleteTrip(ctx context.Context, tripID string) error {
	for _, table := range []string{"trip_tags", "trip_events", "motion_samples", "location_samples"} {
		if _, err := q.pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE trip_id = $1", table), tripID); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	_, err := q.pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, tripID)
	return err
}

// ─── Recent trip for stats ─────────────────────────────────────────────────

func (q *Queries) MostRecentTrip(ctx context.Context) (*TripRow, error) {
	t := &TripRow{}
	err := q.pool.QueryRow(ctx, `
		SELECT t.id, t.device_id, t.started_at, t.ended_at,
		       t.distance_m, t.duration_s,
		       ST_Y(t.start_location::geometry), ST_X(t.start_location::geometry),
		       ST_Y(t.end_location::geometry), ST_X(t.end_location::geometry),
		       t.bundle_hash
		FROM trips t
		ORDER BY t.started_at DESC LIMIT 1
	`).Scan(&t.ID, &t.DeviceID, &t.StartedAt, &t.EndedAt,
		&t.DistanceM, &t.DurationS,
		&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
		&t.BundleHash)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return t, err
}
