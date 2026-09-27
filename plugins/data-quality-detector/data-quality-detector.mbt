// data-quality-detector: WASM plugin for detecting GPS data quality issues.
// Exports alloc() and detect_anomalies() for host interop over linear memory.

// ---- Inline WASM memory primitives ----

///|
extern "wasm" fn wasm_memory_size() -> Int =
  #|(func (result i32) (memory.size))

///|
extern "wasm" fn wasm_memory_grow(pages : Int) -> Int =
  #|(func (param i32) (result i32) (local.get 0) (memory.grow))

///|
extern "wasm" fn load_byte(addr : Int) -> Int =
  #|(func (param i32) (result i32) (local.get 0) (i32.load8_u))

///|
extern "wasm" fn store_byte(addr : Int, val : Int) =
  #|(func (param i32) (param i32) (local.get 0) (local.get 1) (i32.store8))

///|
extern "wasm" fn store_i32_le(addr : Int, val : Int) =
  #|(func (param i32) (param i32) (local.get 0) (local.get 1) (i32.store))

///|
extern "wasm" fn wasm_sqrt(x : Double) -> Double =
  #|(func (param f64) (result f64) (local.get 0) (f64.sqrt))

///|
extern "wasm" fn wasm_abs(x : Double) -> Double =
  #|(func (param f64) (result f64) (local.get 0) (f64.abs))

// ---- Bump allocator ----

///|
priv struct MutInt {
  mut val : Int
}

///|
let bump_ptr : MutInt = { val: 1048576, }

///|
fn ensure_memory(end_addr : Int) -> Unit {
  let needed_pages = (end_addr + 65535) / 65536
  let current = wasm_memory_size()
  if needed_pages > current {
    ignore(wasm_memory_grow(needed_pages - current))
  }
}

///|
#export_name("alloc")
pub fn alloc(size : Int) -> Int {
  let ptr = bump_ptr.val
  bump_ptr.val = ptr + size
  ensure_memory(bump_ptr.val)
  ptr
}

// ---- UTF-8 <-> String helpers ----

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
      let cp = ((b0 & 7) << 18) | ((b1 & 63) << 12) | ((b2 & 63) << 6) | (b3 & 63)
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

// ---- Data types ----

///|
priv struct Sample {
  lat_deg7 : Int
  lon_deg7 : Int
  speed_cmps : Int
  timestamp_ms : Double
  satellites : Int
  hdop_tenths : Int
  accuracy_cm : Int
}

///|
priv struct Anomaly {
  anomaly_type : String
  start_idx : Int
  end_idx : Int
  severity : Double
  detail : String
}

// ---- Math (Haversine) ----

///|
let pi : Double = 3.14159265358979323846

///|
fn to_radians(deg : Double) -> Double {
  deg * pi / 180.0
}

///|
fn sin_approx(x : Double) -> Double {
  let two_pi = 2.0 * pi
  let mut a = x - two_pi * (x / two_pi).to_int().to_double()
  if a > pi {
    a = a - two_pi
  }
  if a < -pi {
    a = a + two_pi
  }
  let x2 = a * a
  let x3 = a * x2
  let x5 = x3 * x2
  let x7 = x5 * x2
  let x9 = x7 * x2
  let x11 = x9 * x2
  a - x3 / 6.0 + x5 / 120.0 - x7 / 5040.0 + x9 / 362880.0 - x11 / 39916800.0
}

///|
fn cos_approx(x : Double) -> Double {
  sin_approx(x + pi / 2.0)
}

///|
fn atan_approx(x : Double) -> Double {
  let ax = wasm_abs(x)
  if ax <= 1.0 {
    let x2 = x * x
    let x3 = x * x2
    let x5 = x3 * x2
    let x7 = x5 * x2
    let x9 = x7 * x2
    x - x3 / 3.0 + x5 / 5.0 - x7 / 7.0 + x9 / 9.0
  } else {
    let sign : Double = if x > 0.0 { 1.0 } else { -1.0 }
    let inv = 1.0 / x
    let inv2 = inv * inv
    let inv3 = inv * inv2
    let inv5 = inv3 * inv2
    let inv7 = inv5 * inv2
    let inv9 = inv7 * inv2
    sign * pi / 2.0 - (inv - inv3 / 3.0 + inv5 / 5.0 - inv7 / 7.0 + inv9 / 9.0)
  }
}

///|
fn atan2_approx(y : Double, x : Double) -> Double {
  if x > 0.0 {
    atan_approx(y / x)
  } else if x < 0.0 && y >= 0.0 {
    atan_approx(y / x) + pi
  } else if x < 0.0 {
    atan_approx(y / x) - pi
  } else if y > 0.0 {
    pi / 2.0
  } else if y < 0.0 {
    -(pi / 2.0)
  } else {
    0.0
  }
}

///|
fn haversine_m(
  lat1 : Double,
  lon1 : Double,
  lat2 : Double,
  lon2 : Double,
) -> Double {
  let r = 6371000.0
  let dlat = to_radians(lat2 - lat1)
  let dlon = to_radians(lon2 - lon1)
  let rlat1 = to_radians(lat1)
  let rlat2 = to_radians(lat2)
  let sdlat = sin_approx(dlat / 2.0)
  let sdlon = sin_approx(dlon / 2.0)
  let a = sdlat * sdlat + cos_approx(rlat1) * cos_approx(rlat2) * sdlon * sdlon
  let c = 2.0 * atan2_approx(wasm_sqrt(a), wasm_sqrt(1.0 - a))
  r * c
}

// ---- JSON field extraction helpers ----

///|
fn get_number(obj : Map[String, Json], key : String) -> Double {
  match obj.get(key) {
    Some(Number(n, ..)) => n
    _ => 0.0
  }
}

///|
fn get_int(obj : Map[String, Json], key : String) -> Int {
  match obj.get(key) {
    Some(Number(n, ..)) => n.to_int()
    _ => 0
  }
}

// ---- JSON parsing ----

///|
fn parse_samples(json : Json) -> Array[Sample] {
  let samples : Array[Sample] = []
  match json {
    Object(top) =>
      match top.get("samples") {
        Some(Json::Array(arr)) =>
          for item in arr {
            match item {
              Object(m) =>
                samples.push(
                  {
                    lat_deg7: get_int(m, "lat_deg7"),
                    lon_deg7: get_int(m, "lon_deg7"),
                    speed_cmps: get_int(m, "speed_cmps"),
                    timestamp_ms: get_number(m, "timestamp_ms"),
                    satellites: get_int(m, "satellites"),
                    hdop_tenths: get_int(m, "hdop_tenths"),
                    accuracy_cm: get_int(m, "accuracy_cm"),
                  },
                )
              _ => ()
            }
          }
        _ => ()
      }
    _ => ()
  }
  samples
}

// ---- Detection logic ----

// Maximum plausible speed in m/s: 500 km/h = ~138.9 m/s
///|
let max_speed_mps : Double = 138.9

///|
fn detect_all(samples : Array[Sample]) -> Array[Anomaly] {
  let anomalies : Array[Anomaly] = []
  let n = samples.length()
  if n < 2 {
    return anomalies
  }

  for i = 1; i < n; i = i + 1 {
    let prev = samples[i - 1]
    let curr = samples[i]

    // Convert deg7 back to degrees
    let lat1 = prev.lat_deg7.to_double() / 10000000.0
    let lon1 = prev.lon_deg7.to_double() / 10000000.0
    let lat2 = curr.lat_deg7.to_double() / 10000000.0
    let lon2 = curr.lon_deg7.to_double() / 10000000.0

    let dt_ms = curr.timestamp_ms - prev.timestamp_ms
    let dt_s = dt_ms / 1000.0

    // 1. Clock drift: timestamps going backwards
    if dt_ms < 0.0 {
      let severity = if wasm_abs(dt_ms) > 60000.0 {
        1.0
      } else {
        wasm_abs(dt_ms) / 60000.0
      }
      anomalies.push(
        {
          anomaly_type: "clock_drift",
          start_idx: i - 1,
          end_idx: i,
          severity,
          detail: "timestamp went backwards",
        },
      )
    }

    // 2. Impossible jump: implied speed > 500 km/h
    if dt_s > 0.1 {
      let dist_m = haversine_m(lat1, lon1, lat2, lon2)
      let implied_speed = dist_m / dt_s
      if implied_speed > max_speed_mps {
        let severity = if implied_speed > max_speed_mps * 5.0 {
          1.0
        } else {
          (implied_speed - max_speed_mps) / (max_speed_mps * 4.0)
        }
        let speed_kmh = (implied_speed * 3.6 + 0.5).to_int()
        anomalies.push(
          {
            anomaly_type: "impossible_jump",
            start_idx: i - 1,
            end_idx: i,
            severity,
            detail: "implied speed \{speed_kmh} km/h",
          },
        )
      }
    }

    // 3. Speed spike: reported speed > 500 km/h (speed_cmps is in cm/s)
    let speed_mps = curr.speed_cmps.to_double() / 100.0
    if speed_mps > max_speed_mps {
      let severity = if speed_mps > max_speed_mps * 3.0 {
        1.0
      } else {
        (speed_mps - max_speed_mps) / (max_speed_mps * 2.0)
      }
      let speed_kmh = (speed_mps * 3.6 + 0.5).to_int()
      anomalies.push(
        {
          anomaly_type: "speed_spike",
          start_idx: i,
          end_idx: i,
          severity,
          detail: "reported speed \{speed_kmh} km/h",
        },
      )
    }

    // 4. GPS loss: satellite count drops to 0
    if curr.satellites == 0 && prev.satellites > 0 {
      anomalies.push(
        {
          anomaly_type: "gps_loss",
          start_idx: i,
          end_idx: i,
          severity: 0.8,
          detail: "satellite count dropped to 0",
        },
      )
    }

    // 5. Satellite drop: significant satellite count decrease (>50%)
    if prev.satellites > 4 && curr.satellites > 0 {
      let drop_ratio = (prev.satellites - curr.satellites).to_double() /
        prev.satellites.to_double()
      if drop_ratio > 0.5 {
        anomalies.push(
          {
            anomaly_type: "satellite_drop",
            start_idx: i,
            end_idx: i,
            severity: drop_ratio,
            detail: "satellites dropped from \{prev.satellites} to \{curr.satellites}",
          },
        )
      }
    }
  }

  // Extend GPS loss spans: merge consecutive 0-satellite readings
  let merged : Array[Anomaly] = []
  let mut gps_loss_start = -1
  let mut gps_loss_end = -1
  for i = 0; i < n; i = i + 1 {
    if samples[i].satellites == 0 {
      if gps_loss_start < 0 {
        gps_loss_start = i
      }
      gps_loss_end = i
    } else {
      if gps_loss_start >= 0 && gps_loss_end - gps_loss_start >= 2 {
        let span = gps_loss_end - gps_loss_start
        let severity = if span > 20 { 1.0 } else { span.to_double() / 20.0 }
        merged.push(
          {
            anomaly_type: "gps_loss",
            start_idx: gps_loss_start,
            end_idx: gps_loss_end,
            severity,
            detail: "GPS loss for \{span} consecutive samples",
          },
        )
      }
      gps_loss_start = -1
      gps_loss_end = -1
    }
  }
  if gps_loss_start >= 0 && gps_loss_end - gps_loss_start >= 2 {
    let span = gps_loss_end - gps_loss_start
    let severity = if span > 20 { 1.0 } else { span.to_double() / 20.0 }
    merged.push(
      {
        anomaly_type: "gps_loss",
        start_idx: gps_loss_start,
        end_idx: gps_loss_end,
        severity,
        detail: "GPS loss for \{span} consecutive samples",
      },
    )
  }

  // Filter out individual gps_loss events if we have a merged span covering them
  for a in anomalies {
    if a.anomaly_type == "gps_loss" {
      // Check if this point is inside a merged span
      let mut covered = false
      for m in merged {
        if a.start_idx >= m.start_idx && a.end_idx <= m.end_idx {
          covered = true
          break
        }
      }
      if !covered {
        merged.push(a)
      }
    } else {
      merged.push(a)
    }
  }

  merged
}

// ---- Quality score ----

///|
fn compute_quality_score(
  n : Int,
  anomalies : Array[Anomaly],
) -> Double {
  if n == 0 {
    return 1.0
  }
  // Deduct based on severity of anomalies relative to sample count
  let mut total_penalty = 0.0
  for a in anomalies {
    let span = (a.end_idx - a.start_idx + 1).to_double()
    total_penalty = total_penalty + a.severity * span / n.to_double()
  }
  let score = 1.0 - total_penalty
  if score < 0.0 {
    0.0
  } else {
    score
  }
}

// ---- Output formatting ----

///|
fn format_score(val : Double) -> String {
  let hundredths = (val * 100.0 + 0.5).to_int()
  let whole = hundredths / 100
  let frac = hundredths % 100
  if frac < 10 {
    "\{whole}.0\{frac}"
  } else {
    "\{whole}.\{frac}"
  }
}

///|
fn json_escape(s : String) -> String {
  let buf = StringBuilder()
  for c in s {
    match c {
      '"' => buf.write_string("\\\"")
      '\\' => buf.write_string("\\\\")
      '\n' => buf.write_string("\\n")
      '\r' => buf.write_string("\\r")
      '\t' => buf.write_string("\\t")
      _ => buf.write_char(c)
    }
  }
  buf.to_string()
}

///|
fn format_output(anomalies : Array[Anomaly], quality_score : Double) -> String {
  let buf = StringBuilder()
  buf.write_string("{\"anomalies\":[")
  for i = 0; i < anomalies.length(); i = i + 1 {
    if i > 0 {
      buf.write_char(',')
    }
    let a = anomalies[i]
    buf.write_string("{\"type\":\"")
    buf.write_string(a.anomaly_type)
    buf.write_string("\",\"start_idx\":")
    buf.write_string(a.start_idx.to_string())
    buf.write_string(",\"end_idx\":")
    buf.write_string(a.end_idx.to_string())
    buf.write_string(",\"severity\":")
    buf.write_string(format_score(a.severity))
    buf.write_string(",\"detail\":\"")
    buf.write_string(json_escape(a.detail))
    buf.write_string("\"}")
  }
  buf.write_string("],\"quality_score\":")
  buf.write_string(format_score(quality_score))
  buf.write_char('}')
  buf.to_string()
}

///|
fn empty_output() -> String {
  "{\"anomalies\":[],\"quality_score\":1.00}"
}

// ---- Main logic ----

///|
fn do_detect(input : String) -> String {
  let json : Json = @json.parse(input) catch { _ => return empty_output() }
  let samples = parse_samples(json)
  if samples.length() < 2 {
    return empty_output()
  }
  let anomalies = detect_all(samples)
  let quality_score = compute_quality_score(samples.length(), anomalies)
  format_output(anomalies, quality_score)
}

// ---- Exported function ----

///|
#export_name("detect_anomalies")
pub fn detect_anomalies(ptr : Int, len : Int) -> Int {
  let input_str = read_utf8(ptr, len)
  let output = do_detect(input_str)
  let byte_len = utf8_byte_length(output)
  let out_ptr = alloc(byte_len + 4)
  store_i32_le(out_ptr, byte_len)
  let _ = write_utf8(output, out_ptr + 4)
  out_ptr
}
