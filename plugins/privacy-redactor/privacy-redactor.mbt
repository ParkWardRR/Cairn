///|
// ========== WASM Memory Intrinsics ==========

///|
extern "wasm" fn wasm_memory_grow(delta : Int) -> Int =
  #|(func (param i32) (result i32) (memory.grow (local.get 0)))

///|
extern "wasm" fn load_byte(addr : Int) -> Int =
  #|(func (param i32) (result i32) (i32.load8_u (local.get 0)))

///|
extern "wasm" fn store_byte(addr : Int, val : Int) -> Unit =
  #|(func (param i32) (param i32) (i32.store8 (local.get 0) (local.get 1)))

///|
extern "wasm" fn store_i32(addr : Int, val : Int) -> Unit =
  #|(func (param i32) (param i32) (i32.store (local.get 0) (local.get 1)))

///|
extern "wasm" fn f64_sqrt(x : Double) -> Double =
  #|(func (param f64) (result f64) (f64.sqrt (local.get 0)))

///|
// ========== Bump Allocator for I/O Buffers ==========

///|
priv struct IoState {
  mut base : Int
  mut ptr : Int
}

///|
let io : IoState = { base: 0, ptr: 0, }

///|
fn ensure_io() -> Unit {
  if io.base == 0 {
    let old_pages = wasm_memory_grow(64)
    io.base = old_pages * 65536
    io.ptr = io.base
  }
}

///|
fn io_alloc(size : Int) -> Int {
  ensure_io()
  let ptr = io.ptr
  io.ptr = io.ptr + (size + 7) / 8 * 8
  ptr
}

///|
// ========== UTF-8 Codec ==========

///|
fn read_utf8(ptr : Int, len : Int) -> String {
  let chars : Array[Char] = []
  let mut i = 0
  while i < len {
    let b0 = load_byte(ptr + i)
    if b0 < 128 {
      chars.push(b0.unsafe_to_char())
      i = i + 1
    } else if b0 < 224 {
      let b1 = load_byte(ptr + i + 1)
      let cp = ((b0 & 31) << 6) | (b1 & 63)
      chars.push(cp.unsafe_to_char())
      i = i + 2
    } else if b0 < 240 {
      let b1 = load_byte(ptr + i + 1)
      let b2 = load_byte(ptr + i + 2)
      let cp = ((b0 & 15) << 12) | ((b1 & 63) << 6) | (b2 & 63)
      chars.push(cp.unsafe_to_char())
      i = i + 3
    } else {
      let b1 = load_byte(ptr + i + 1)
      let b2 = load_byte(ptr + i + 2)
      let b3 = load_byte(ptr + i + 3)
      let cp = ((b0 & 7) << 18) |
        ((b1 & 63) << 12) |
        ((b2 & 63) << 6) |
        (b3 & 63)
      chars.push(cp.unsafe_to_char())
      i = i + 4
    }
  }
  String::from_array(chars)
}

///|
fn utf8_byte_length(s : String) -> Int {
  let mut total = 0
  for c in s {
    let cp = c.to_int()
    if cp < 128 {
      total = total + 1
    } else if cp < 2048 {
      total = total + 2
    } else if cp < 65536 {
      total = total + 3
    } else {
      total = total + 4
    }
  }
  total
}

///|
fn write_utf8(s : String, dst : Int) -> Int {
  let mut pos = dst
  for c in s {
    let cp = c.to_int()
    if cp < 128 {
      store_byte(pos, cp)
      pos = pos + 1
    } else if cp < 2048 {
      store_byte(pos, 192 | (cp >> 6))
      store_byte(pos + 1, 128 | (cp & 63))
      pos = pos + 2
    } else if cp < 65536 {
      store_byte(pos, 224 | (cp >> 12))
      store_byte(pos + 1, 128 | ((cp >> 6) & 63))
      store_byte(pos + 2, 128 | (cp & 63))
      pos = pos + 3
    } else {
      store_byte(pos, 240 | (cp >> 18))
      store_byte(pos + 1, 128 | ((cp >> 12) & 63))
      store_byte(pos + 2, 128 | ((cp >> 6) & 63))
      store_byte(pos + 3, 128 | (cp & 63))
      pos = pos + 4
    }
  }
  pos - dst
}

///|
fn write_length_prefixed(s : String) -> Int {
  let byte_len = utf8_byte_length(s)
  let out_ptr = io_alloc(4 + byte_len)
  store_i32(out_ptr, byte_len)
  let _ = write_utf8(s, out_ptr + 4)
  out_ptr
}

///|
// ========== Data Types ==========

///|
priv struct RoutePoint {
  lat : Double
  lon : Double
  speed_mps : Double
  altitude_m : Double
  timestamp_ms : Double
}

///|
priv struct PrivacyZone {
  lat : Double
  lon : Double
  radius_m : Double
  label : String
}

///|
priv struct RedactedPoint {
  lat : Double
  lon : Double
  speed_mps : Double
  altitude_m : Double
  timestamp_ms : Double
  redacted : Bool
}

///|
// ========== Haversine Distance ==========

///|
let earth_radius_m : Double = 6371000.0

///|
fn to_radians(deg : Double) -> Double {
  deg * @math.PI / 180.0
}

///|
fn haversine(
  lat1 : Double,
  lon1 : Double,
  lat2 : Double,
  lon2 : Double,
) -> Double {
  let lat1_r = to_radians(lat1)
  let lat2_r = to_radians(lat2)
  let dlat = to_radians(lat2 - lat1)
  let dlon = to_radians(lon2 - lon1)
  let sin_dlat2 = @math.sin(dlat / 2.0)
  let sin_dlon2 = @math.sin(dlon / 2.0)
  let a = sin_dlat2 * sin_dlat2 +
    @math.cos(lat1_r) * @math.cos(lat2_r) * sin_dlon2 * sin_dlon2
  let c = 2.0 * @math.atan2(f64_sqrt(a), f64_sqrt(1.0 - a))
  earth_radius_m * c
}

///|
// ========== JSON Extraction ==========

///|
fn extract_route(json : Json) -> Array[RoutePoint] {
  let points : Array[RoutePoint] = []
  match json {
    { "route": Array(arr), .. } =>
      for item in arr {
        match item {
          {
            "lat": Number(lat, ..),
            "lon": Number(lon, ..),
            "speed_mps": Number(speed, ..),
            "altitude_m": Number(alt, ..),
            "timestamp_ms": Number(ts, ..),
            ..
          } =>
            points.push({
              lat,
              lon,
              speed_mps: speed,
              altitude_m: alt,
              timestamp_ms: ts,
            })
          _ => ()
        }
      }
    _ => ()
  }
  points
}

///|
fn extract_zones(json : Json) -> Array[PrivacyZone] {
  let zones : Array[PrivacyZone] = []
  match json {
    { "privacy_zones": Array(arr), .. } =>
      for item in arr {
        match item {
          {
            "lat": Number(lat, ..),
            "lon": Number(lon, ..),
            "radius_m": Number(r, ..),
            "label": String(lbl),
            ..
          } => zones.push({ lat, lon, radius_m: r, label: lbl, })
          _ => ()
        }
      }
    _ => ()
  }
  zones
}

///|
// ========== Redaction Logic ==========

///|
fn do_redact(
  route : Array[RoutePoint],
  zones : Array[PrivacyZone],
) -> (Array[RedactedPoint], Array[String], Int) {
  let redacted : Array[RedactedPoint] = []
  let triggered : Array[String] = []
  let mut count = 0
  for point in route {
    let mut inside = false
    let mut snap_lat = point.lat
    let mut snap_lon = point.lon
    for i = 0; i < zones.length(); i = i + 1 {
      let zone = zones[i]
      let dist = haversine(point.lat, point.lon, zone.lat, zone.lon)
      if dist < zone.radius_m {
        inside = true
        snap_lat = zone.lat
        snap_lon = zone.lon
        if !triggered.contains(zone.label) {
          triggered.push(zone.label)
        }
        break
      }
    }
    if inside {
      count = count + 1
      redacted.push({
        lat: snap_lat,
        lon: snap_lon,
        speed_mps: point.speed_mps,
        altitude_m: point.altitude_m,
        timestamp_ms: point.timestamp_ms,
        redacted: true,
      })
    } else {
      redacted.push({
        lat: point.lat,
        lon: point.lon,
        speed_mps: point.speed_mps,
        altitude_m: point.altitude_m,
        timestamp_ms: point.timestamp_ms,
        redacted: false,
      })
    }
  }
  (redacted, triggered, count)
}

///|
// ========== Output Building ==========

///|
fn build_output_json(
  redacted : Array[RedactedPoint],
  triggered : Array[String],
  points_redacted : Int,
  points_total : Int,
) -> String {
  let route_items : Array[Json] = []
  for p in redacted {
    let item : Json = {
      "lat": p.lat,
      "lon": p.lon,
      "speed_mps": p.speed_mps,
      "altitude_m": p.altitude_m,
      "timestamp_ms": p.timestamp_ms,
      "redacted": p.redacted,
    }
    route_items.push(item)
  }
  let zone_items : Array[Json] = []
  for z in triggered {
    zone_items.push(z.to_json())
  }
  let output : Json = {
    "abi_version": 1,
    "plugin": "privacy-redactor",
    "version": "0.1.0",
    "redacted_route": route_items,
    "zones_triggered": zone_items,
    "points_redacted": points_redacted,
    "points_total": points_total,
  }
  output.stringify()
}

///|
fn write_error_output(msg : String) -> Int {
  let empty_route : Array[Json] = []
  let empty_zones : Array[Json] = []
  let err_json : Json = {
    "error": msg,
    "abi_version": 1,
    "plugin": "privacy-redactor",
    "version": "0.1.0",
    "redacted_route": empty_route,
    "zones_triggered": empty_zones,
    "points_redacted": 0,
    "points_total": 0,
  }
  write_length_prefixed(err_json.stringify())
}

///|
// ========== WASM Exported Functions ==========

///|
#export_name("alloc")
pub fn alloc(size : Int) -> Int {
  io_alloc(size)
}

///|
#export_name("redact")
pub fn redact(ptr : Int, len : Int) -> Int {
  let input = read_utf8(ptr, len)
  let json = @json.parse(input) catch {
    _ => return write_error_output("JSON parse error")
  }
  let route = extract_route(json)
  let zones = extract_zones(json)
  let (redacted, triggered, count) = do_redact(route, zones)
  let output = build_output_json(redacted, triggered, count, route.length())
  write_length_prefixed(output)
}
