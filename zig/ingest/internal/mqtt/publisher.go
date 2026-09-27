package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Publisher sends MQTT 3.1.1 messages to a broker.
// Uses a minimal raw-socket implementation to avoid external dependencies.
// If the broker URL is empty, all publishes are no-ops.
type Publisher struct {
	mu      sync.Mutex
	addr    string
	conn    net.Conn
	enabled bool
}

// New creates an MQTT publisher. Pass "" to disable.
func New(brokerAddr string) *Publisher {
	p := &Publisher{
		addr:    brokerAddr,
		enabled: brokerAddr != "",
	}
	if p.enabled {
		if err := p.connect(); err != nil {
			log.Printf("mqtt: initial connection to %s failed: %v (will retry on publish)", brokerAddr, err)
		} else {
			log.Printf("mqtt: connected to %s", brokerAddr)
		}
	}
	return p
}

func (p *Publisher) connect() error {
	conn, err := net.DialTimeout("tcp", p.addr, 5*time.Second)
	if err != nil {
		return err
	}

	// MQTT CONNECT packet (minimal, no auth, clean session)
	clientID := "cairn-ingestd"
	connectPacket := buildConnectPacket(clientID)
	if _, err := conn.Write(connectPacket); err != nil {
		conn.Close()
		return fmt.Errorf("write CONNECT: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 4)
	if _, err := conn.Read(buf); err != nil {
		conn.Close()
		return fmt.Errorf("read CONNACK: %w", err)
	}
	if buf[0] != 0x20 || buf[3] != 0x00 {
		conn.Close()
		return fmt.Errorf("CONNACK rejected: %x", buf)
	}
	conn.SetReadDeadline(time.Time{})

	p.conn = conn
	return nil
}

// Publish sends a message to the given topic. QoS 0 (fire and forget).
func (p *Publisher) Publish(topic string, payload []byte) error {
	if !p.enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		if err := p.connect(); err != nil {
			return fmt.Errorf("mqtt reconnect: %w", err)
		}
	}

	packet := buildPublishPacket(topic, payload)
	if _, err := p.conn.Write(packet); err != nil {
		p.conn.Close()
		p.conn = nil
		return fmt.Errorf("mqtt publish: %w", err)
	}
	return nil
}

// PublishJSON marshals v and publishes to topic.
func (p *Publisher) PublishJSON(topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.Publish(topic, data)
}

// Close disconnects from the broker.
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		// DISCONNECT packet
		p.conn.Write([]byte{0xE0, 0x00})
		p.conn.Close()
		p.conn = nil
	}
}

// ─── Trip event publishing ─────────────────────────────────────────────────

type TripEndEvent struct {
	VehicleID  string    `json:"vehicle_id"`
	TripID     string    `json:"trip_id"`
	Event      string    `json:"event"`
	EndedAt    time.Time `json:"ended_at"`
	DistanceKm float64   `json:"distance_km"`
	DurationS  int       `json:"duration_s"`
	LocalOnly  bool      `json:"local_only"`
}

type SyncEvent struct {
	VehicleID string    `json:"vehicle_id"`
	TripID    string    `json:"trip_id"`
	Event     string    `json:"event"`
	SyncedAt  time.Time `json:"synced_at"`
}

// PublishTripEnd publishes a trip_ended event.
func (p *Publisher) PublishTripEnd(deviceID, tripID string, endedAt time.Time, distanceM float64, durationS int) {
	evt := TripEndEvent{
		VehicleID:  deviceID,
		TripID:     tripID,
		Event:      "trip_ended",
		EndedAt:    endedAt,
		DistanceKm: distanceM / 1000.0,
		DurationS:  durationS,
		LocalOnly:  true,
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/trip_ended", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish trip_ended: %v", err)
	}
}

// PublishSyncCompleted publishes a sync_completed event.
func (p *Publisher) PublishSyncCompleted(deviceID, tripID string) {
	evt := SyncEvent{
		VehicleID: deviceID,
		TripID:    tripID,
		Event:     "sync_completed",
		SyncedAt:  time.Now().UTC(),
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/sync_completed", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish sync_completed: %v", err)
	}
}

// PublishTripStarted publishes a trip_started event when a new upload is initialized.
func (p *Publisher) PublishTripStarted(deviceID, tripID string) {
	evt := struct {
		VehicleID string    `json:"vehicle_id"`
		TripID    string    `json:"trip_id"`
		Event     string    `json:"event"`
		StartedAt time.Time `json:"started_at"`
	}{
		VehicleID: deviceID,
		TripID:    tripID,
		Event:     "trip_started",
		StartedAt: time.Now().UTC(),
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/trip_started", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish trip_started: %v", err)
	}
}

// PublishArrivedHome publishes an arrived_home event when a trip ends near a home place.
func (p *Publisher) PublishArrivedHome(deviceID, tripID, placeName string) {
	evt := struct {
		VehicleID string    `json:"vehicle_id"`
		TripID    string    `json:"trip_id"`
		Event     string    `json:"event"`
		Place     string    `json:"place"`
		ArrivedAt time.Time `json:"arrived_at"`
	}{
		VehicleID: deviceID,
		TripID:    tripID,
		Event:     "arrived_home",
		Place:     placeName,
		ArrivedAt: time.Now().UTC(),
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/arrived_home", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish arrived_home: %v", err)
	}
}

// PublishDepartedHome publishes a departed_home event when a trip starts near a home place.
func (p *Publisher) PublishDepartedHome(deviceID, tripID, placeName string) {
	evt := struct {
		VehicleID  string    `json:"vehicle_id"`
		TripID     string    `json:"trip_id"`
		Event      string    `json:"event"`
		Place      string    `json:"place"`
		DepartedAt time.Time `json:"departed_at"`
	}{
		VehicleID:  deviceID,
		TripID:     tripID,
		Event:      "departed_home",
		Place:      placeName,
		DepartedAt: time.Now().UTC(),
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/departed_home", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish departed_home: %v", err)
	}
}

// PublishLastParked publishes the final GNSS coordinates when a trip ends.
func (p *Publisher) PublishLastParked(deviceID, tripID string, lat, lon float64) {
	evt := struct {
		VehicleID string    `json:"vehicle_id"`
		TripID    string    `json:"trip_id"`
		Event     string    `json:"event"`
		Lat       float64   `json:"lat"`
		Lon       float64   `json:"lon"`
		ParkedAt  time.Time `json:"parked_at"`
	}{
		VehicleID: deviceID,
		TripID:    tripID,
		Event:     "last_parked",
		Lat:       lat,
		Lon:       lon,
		ParkedAt:  time.Now().UTC(),
	}
	topic := fmt.Sprintf("cairn/vehicle/%s/last_parked", deviceID)
	if err := p.PublishJSON(topic, evt); err != nil {
		log.Printf("mqtt: publish last_parked: %v", err)
	}
}

// ─── MQTT 3.1.1 packet builders ────────────────────────────────────────────

func buildConnectPacket(clientID string) []byte {
	// Variable header
	varHeader := []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T', // Protocol Name
		0x04,       // Protocol Level (3.1.1)
		0x02,       // Connect Flags (Clean Session)
		0x00, 0x3C, // Keep Alive (60s)
	}
	// Payload: client ID
	clientIDBytes := []byte(clientID)
	payload := append([]byte{byte(len(clientIDBytes) >> 8), byte(len(clientIDBytes))}, clientIDBytes...)

	remaining := len(varHeader) + len(payload)
	packet := []byte{0x10} // CONNECT
	packet = append(packet, encodeRemainingLength(remaining)...)
	packet = append(packet, varHeader...)
	packet = append(packet, payload...)
	return packet
}

func buildPublishPacket(topic string, payload []byte) []byte {
	topicBytes := []byte(topic)
	topicLen := []byte{byte(len(topicBytes) >> 8), byte(len(topicBytes))}

	remaining := 2 + len(topicBytes) + len(payload)
	packet := []byte{0x30} // PUBLISH QoS 0
	packet = append(packet, encodeRemainingLength(remaining)...)
	packet = append(packet, topicLen...)
	packet = append(packet, topicBytes...)
	packet = append(packet, payload...)
	return packet
}

func encodeRemainingLength(length int) []byte {
	var encoded []byte
	for {
		b := byte(length % 128)
		length /= 128
		if length > 0 {
			b |= 0x80
		}
		encoded = append(encoded, b)
		if length == 0 {
			break
		}
	}
	return encoded
}
