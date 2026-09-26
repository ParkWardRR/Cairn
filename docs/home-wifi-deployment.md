# Home Wi-Fi Deployment

## Network Architecture

```
┌─────────────────────────────────────────────┐
│ Home Network                                 │
│                                              │
│  Router / AP                                 │
│  ├── SSID: "HomeNetwork"                     │
│  │   BSSID: AA:BB:CC:DD:EE:FF               │
│  │                                           │
│  ├── Cairn Server (static DHCP)              │
│  │   IP: 192.168.1.50                        │
│  │   mDNS: cairn.local                       │
│  │   Ports: 443 (mTLS ingest + API)          │
│  │                                           │
│  ├── Freematics Device (DHCP)                │
│  │   Connects only to trusted BSSID          │
│  │   Uploads via mTLS to cairn.local         │
│  │                                           │
│  └── Home Assistant (optional)               │
│      MQTT broker: mqtt.local:1883            │
└─────────────────────────────────────────────┘
```

## Prerequisites

- Home Wi-Fi network with WPA2/WPA3
- Static IP or DHCP reservation for the Cairn server
- Homelab server running Docker
- USB-to-serial adapter for initial device provisioning

## Server Setup

### 1. DNS / Discovery

**mDNS (initial setup):**
- Server advertises `cairn.local` via Avahi/mDNS
- Device resolves `cairn.local` on the home network
- Suitable for development and single-network deployments

**Static DHCP (production):**
- Assign a fixed IP to the server via router DHCP reservation
- Configure device with the static IP as a fallback
- More reliable than mDNS for always-on operation

### 2. TLS Certificates

Generate a local CA and issue certificates:

```bash
# Create local CA (do this once; store CA key offline after)
openssl ecparam -genkey -name prime256v1 -out ca-key.pem
openssl req -new -x509 -key ca-key.pem -out ca.pem -days 3650 \
  -subj "/CN=Cairn Local CA"

# Generate server certificate
openssl ecparam -genkey -name prime256v1 -out server-key.pem
openssl req -new -key server-key.pem -out server.csr \
  -subj "/CN=cairn.local"
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out server.pem -days 825 \
  -extfile <(echo "subjectAltName=DNS:cairn.local,IP:192.168.1.50")

# Generate device certificate
openssl ecparam -genkey -name prime256v1 -out device-key.pem
openssl req -new -key device-key.pem -out device.csr \
  -subj "/CN=cairn-mazda-01"
openssl x509 -req -in device.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out device.pem -days 825
```

### 3. Docker Compose Stack

```yaml
# deploy/compose.yaml
services:
  postgres:
    image: postgis/postgis:16-3.4
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: cairn
      POSTGRES_USER: cairn
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password

  ingest:
    build: ../zig/ingest
    ports:
      - "443:8443"
    volumes:
      - bundles:/data/bundles
      - ./certs:/certs:ro
    environment:
      DATABASE_URL: postgres://cairn@postgres/cairn
      TLS_CERT: /certs/server.pem
      TLS_KEY: /certs/server-key.pem
      TLS_CA: /certs/ca.pem

  api:
    build: ../zig/api
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://cairn@postgres/cairn

  caddy:
    image: caddy:2
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro

volumes:
  pgdata:
  bundles:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

## Device Provisioning

### Step 1: Flash Firmware

```bash
# Connect device via USB
# Flash using PlatformIO or Arduino IDE
# See docs/device-protocol.md for details
```

### Step 2: Configure Network

```bash
# Over USB serial connection
cairn-provision --ssid "HomeNetwork" --bssid "AA:BB:CC:DD:EE:FF" --psk "password"
```

### Step 3: Install Certificates

```bash
# Install CA certificate (device will only trust this CA)
cairn-provision --server-ca ./ca.pem

# Install device certificate and key
cairn-provision --device-cert ./device.pem --device-key ./device-key.pem
```

### Step 4: Generate Device Identity

```bash
# Generate Ed25519 keypair for bundle signing
cairn-provision --generate-identity

# Export public key for server enrollment
cairn-provision --export-pubkey > device-pubkey.pem
```

### Step 5: Enroll on Server

```bash
# Register device with the server
tripctl device enroll \
  --id "cairn-mazda-01" \
  --pubkey ./device-pubkey.pem \
  --vehicle "Mazda" \
  --cert ./device.pem
```

### Step 6: Verify

```bash
# Check device can reach server
cairn-provision --test-connection

# Check server recognizes device
tripctl device status --id "cairn-mazda-01"
```

## Trusted Network Policy

### SSID + BSSID Validation

The device maintains an allowlist of trusted networks:

```
trusted_networks:
  - ssid: "HomeNetwork"
    bssid: "AA:BB:CC:DD:EE:FF"
  - ssid: "HomeNetwork"
    bssid: "AA:BB:CC:DD:EE:00"   # second AP, same network
```

**Rules:**
- Both SSID and BSSID must match an allowlist entry
- An identically-named SSID with an unknown BSSID is rejected
- The device logs rejected association attempts for diagnostics
- No open networks are ever trusted

### Wi-Fi Scanning Behavior

| Device State | Scan Behavior |
|-------------|---------------|
| Parked at home | No scanning; already associated |
| Returning home (queued trips) | Periodic scan for trusted BSSID |
| Parked away (no queued trips) | No scanning; deep sleep |
| Parked away (queued trips) | Sparse periodic scan (e.g., every 5 min) to detect home arrival |

Scanning is conservative to minimize power draw. The device does not continuously scan for networks while parked away from home.

## Firewall / Network Recommendations

| Rule | Direction | Purpose |
|------|-----------|---------|
| Device → Server:443 | Outbound (device) | mTLS upload and health check |
| Server:8080 → LAN clients | Inbound (server) | Web UI access |
| Server → MQTT:1883 | Outbound (server) | Home Assistant events |
| No port forwarding | — | Never expose Cairn to the internet |

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Device won't connect | Verify SSID and BSSID in device config; check Wi-Fi signal in garage |
| mTLS handshake fails | Verify CA cert on device matches server's issuing CA; check cert expiry |
| mDNS not resolving | Check Avahi is running on server; verify device and server on same subnet |
| Upload stalls | Check server logs for rate limiting; verify device has queued bundles |
| No sync after arriving home | Check device is actually associated (serial debug); verify server is reachable |
