# Architecture

## Overview

Cairn is a two-part system: an **in-vehicle device** that records driving data offline, and a **homelab server** that receives, validates, stores, and presents that data. The two halves communicate exclusively over the home LAN — no cloud, no cellular, no internet required.

## System Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│ Car                                                                │
│                                                                    │
│ Freematics ONE+ Model B                                            │
│  ESP32 + GNSS + IMU + microSD + Wi-Fi                              │
│                                                                    │
│  [drive]  GNSS / IMU → append-only encrypted local trip spool     │
│  [park]   finalize bundle → sleep / periodic trusted-SSID check   │
│  [home]   join home Wi-Fi → mTLS HTTPS upload → verify ACK        │
│  [done]   retain until server receipt is durable → prune safely    │
└──────────────────────────────┬─────────────────────────────────────┘
                               │ LAN only; no Internet required
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│ Homelab                                                             │
│                                                                    │
│ Zig ingestion service                                               │
│  ├─ mTLS / device identity                                         │
│  ├─ resumable, idempotent bundle receiver                          │
│  ├─ signature/hash validation                                      │
│  └─ raw-object persistence                                         │
│                                                                    │
│ PostgreSQL + PostGIS                                                │
│  ├─ raw GNSS samples                                               │
│  ├─ normalized trips                                               │
│  ├─ derived stops / parking locations                              │
│  └─ tags / places / exports                                        │
│                                                                    │
│ Gleam trip-event service                                            │
│  ├─ trip state transitions                                         │
│  ├─ jobs, retries, event semantics                                 │
│  └─ notification/automation policy                                 │
│                                                                    │
│ MoonBit plugin sandbox                                              │
│  ├─ route scoring                                                  │
│  ├─ trip classification                                            │
│  └─ import/export transforms                                       │
│                                                                    │
│ Zig web/API service + PWA                                           │
│  └─ Trip history, map, labels, places, export                     │
│                                                                    │
│ Optional: MQTT → Home Assistant                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Core Rule

**The device must never require the server to complete a drive.** Every trip has a valid local representation on the microSD before any Wi-Fi transfer occurs.

## Component Responsibilities

### Device (Freematics ONE+ Model B)

| Function | Responsibility |
|----------|---------------|
| Position | GNSS sample capture with fix/accuracy data |
| Motion | IMU sampling, including low-power motion wake |
| Drive detection | Lightweight state machine using GNSS speed and motion |
| Time | GNSS-derived UTC when available; monotonic counter always |
| Persistence | Append-only microSD records with periodic checkpoint |
| Network | Home Wi-Fi only; no LTE initialization |
| Sync | Upload finalized bundles with resume and receipt |
| Power | Aggressively disable unused peripherals; sleep outside active/sync |
| Status | Compact local status endpoint or serial diagnostics |
| Updates | USB/serial in v1; signed local OTA after core proven |

### Zig Ingest Service (`ingestd`)

Receives trip bundles from devices over mTLS. Validates signatures and content hashes. Stores raw immutable bundles and inserts normalized data into PostgreSQL/PostGIS. Issues durable receipts that the device uses to confirm successful storage.

### Zig API Service (`api`)

Read-only JSON API serving trip data, places, route geometry, tags, and exports. Powers the local PWA. Uses PostGIS for spatial queries (nearest place, distance, geofencing).

### Gleam Trip Orchestrator

Consumes persisted events from the database (not the raw sensor stream). Manages durable workflows: trip lifecycle projection, reprocessing on algorithm updates, automation execution, notification policy, and data-quality flagging. Runs under OTP supervision for fault tolerance.

### MoonBit Plugin Sandbox

Runs user-defined or experimental enrichments in a WASM sandbox. Plugins receive structured input and return structured output. No database, network, or filesystem access. The host validates all results and records plugin version/hash for reproducibility.

### Odin Native Tools

Standalone desktop utilities for inspecting, diffing, replaying, and analyzing trip data. Not part of the server's operational path.

### Mojo Experimental Lane

Offline trajectory analysis experiments (route clustering, map matching, anomaly detection). Outputs are proposals, not authoritative decisions. No production dependency.

## Data Flow

```
Device microSD ──► finalized bundle
                       │
              home Wi-Fi detected
                       │
                       ▼
              mTLS upload to ingestd
                       │
              ┌────────┼────────┐
              ▼        ▼        ▼
         raw bundle  PostgreSQL  receipt
         (object     (normalized  (back to
          store)      samples)    device)
                       │
                       ▼
              API service + PWA
              Gleam orchestrator
              MoonBit plugins
              MQTT → Home Assistant
```

## Technology Boundary Rules

| Technology | Use For | Do Not Use For |
|-----------|---------|---------------|
| **Zig** | Device tools, parsers, binary format, ingest, API, CLI | Full UI, distributed workflow experimentation |
| **Gleam** | Background processing, job state machines, retry, events | ESP32 firmware, hot telemetry loop |
| **MoonBit** | WASM plugins: classify, redact, transform, score | Firmware, core auth, database access |
| **Odin** | Desktop tools: inspect, diff, replay, heatmap | Always-on backend, embedded core |
| **Mojo** | Experimental analysis: clustering, matching, ML | Required ingestion, production correctness |
| **Arduino/C++** | Freematics firmware hardware enablement | Homelab services |

## Deployment

The homelab stack runs via Docker Compose:

- PostgreSQL + PostGIS
- Zig ingest service
- Zig API service
- Caddy reverse proxy (TLS termination, mDNS)
- Gleam orchestrator (after Phase 6)
- MQTT broker (for Home Assistant events)

All services are LAN-only. No port forwarding. Remote access through VPN only.
