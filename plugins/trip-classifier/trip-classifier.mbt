// trip-classifier: WASM plugin for classifying trip telemetry data.
// Exports alloc() and classify() for host interop over linear memory.

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
let bump_ptr : MutInt = { val: 1048576, } // Start at 1MB to avoid MoonBit internal heap

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
priv struct RouteSample {
  lat : Double
  lon : Double
  speed_mps : Double
  altitude_m : Double
}

///|
priv struct Trip {
  id : String
  distance_m : Double
  duration_s : Double
  start_lat : Double
  start_lon : Double
  end_lat : Double
  end_lon : Double
  start_hour : Int
  end_hour : Int
  day_of_week : Int
  avg_speed_mps : Double
  max_speed_mps : Double
  num_stops : Int
  route_samples : Array[RouteSample]
}

///|
priv struct ClassLabel {
  name : String
  confidence : Double
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

///|
fn get_string_field(obj : Map[String, Json], key : String) -> String {
  match obj.get(key) {
    Some(Json::String(s)) => s
    _ => ""
  }
}

// ---- JSON parsing ----

///|
fn parse_route_sample(json : Json) -> RouteSample {
  match json {
    Object(m) =>
      {
        lat: get_number(m, "lat"),
        lon: get_number(m, "lon"),
        speed_mps: get_number(m, "speed_mps"),
        altitude_m: get_number(m, "altitude_m"),
      }
    _ => { lat: 0.0, lon: 0.0, speed_mps: 0.0, altitude_m: 0.0, }
  }
}

///|
fn make_default_trip() -> Trip {
  {
    id: "",
    distance_m: 0.0,
    duration_s: 0.0,
    start_lat: 0.0,
    start_lon: 0.0,
    end_lat: 0.0,
    end_lon: 0.0,
    start_hour: 0,
    end_hour: 0,
    day_of_week: 0,
    avg_speed_mps: 0.0,
    max_speed_mps: 0.0,
    num_stops: 0,
    route_samples: [],
  }
}

///|
fn parse_trip(json : Json) -> Trip {
  match json {
    Object(top) =>
      match top.get("trip") {
        Some(Object(t)) => {
          let samples : Array[RouteSample] = match t.get("route_samples") {
            Some(Json::Array(arr)) => arr.map(parse_route_sample)
            _ => []
          }
          {
            id: get_string_field(t, "id"),
            distance_m: get_number(t, "distance_m"),
            duration_s: get_number(t, "duration_s"),
            start_lat: get_number(t, "start_lat"),
            start_lon: get_number(t, "start_lon"),
            end_lat: get_number(t, "end_lat"),
            end_lon: get_number(t, "end_lon"),
            start_hour: get_int(t, "start_hour"),
            end_hour: get_int(t, "end_hour"),
            day_of_week: get_int(t, "day_of_week"),
            avg_speed_mps: get_number(t, "avg_speed_mps"),
            max_speed_mps: get_number(t, "max_speed_mps"),
            num_stops: get_int(t, "num_stops"),
            route_samples: samples,
          }
        }
        _ => make_default_trip()
      }
    _ => make_default_trip()
  }
}

// ---- Math helpers (pure WASM, no libc dependency) ----

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

// ---- Classification heuristics ----

///|
fn classify_commute(trip : Trip) -> Double {
  let mut score = 0.0
  if trip.day_of_week >= 1 && trip.day_of_week <= 5 {
    score = score + 0.3
  }
  if (trip.start_hour >= 6 && trip.start_hour <= 10) ||
    (trip.start_hour >= 16 && trip.start_hour <= 20) {
    score = score + 0.3
  }
  if trip.distance_m > 3000.0 {
    score = score + 0.2
  }
  if trip.duration_s > 600.0 {
    score = score + 0.2
  }
  score
}

///|
fn classify_short_errand(trip : Trip) -> Double {
  let mut score = 0.0
  if trip.distance_m < 5000.0 {
    score = score + 0.5
  }
  if trip.duration_s < 1200.0 {
    score = score + 0.5
  }
  score
}

///|
fn classify_road_trip(trip : Trip) -> Double {
  let mut score = 0.0
  if trip.distance_m > 100000.0 {
    score = score + 0.6
  }
  if trip.duration_s > 7200.0 {
    score = score + 0.4
  }
  if score > 1.0 {
    1.0
  } else {
    score
  }
}

///|
fn classify_canyon_drive(trip : Trip) -> Double {
  let n = trip.route_samples.length()
  if n < 2 {
    return 0.0
  }
  let mut min_alt = trip.route_samples[0].altitude_m
  let mut max_alt = min_alt
  let mut sum_speed = 0.0
  let mut sum_speed_sq = 0.0
  for i = 0; i < n; i = i + 1 {
    let s = trip.route_samples[i]
    if s.altitude_m < min_alt {
      min_alt = s.altitude_m
    }
    if s.altitude_m > max_alt {
      max_alt = s.altitude_m
    }
    sum_speed = sum_speed + s.speed_mps
    sum_speed_sq = sum_speed_sq + s.speed_mps * s.speed_mps
  }
  let alt_range = max_alt - min_alt
  let nd = n.to_double()
  let mean_speed = sum_speed / nd
  let variance = sum_speed_sq / nd - mean_speed * mean_speed
  let std_dev = wasm_sqrt(wasm_abs(variance))
  let mut score = 0.0
  if alt_range > 200.0 {
    score = score + 0.5
  }
  if std_dev > 5.0 {
    score = score + 0.3
  } else if std_dev > 3.0 {
    score = score + 0.15
  }
  if alt_range > 200.0 && std_dev > 5.0 {
    score = score + 0.2
  }
  if score > 1.0 {
    1.0
  } else {
    score
  }
}

///|
fn classify_night_drive(trip : Trip) -> Double {
  if trip.start_hour >= 22 || trip.start_hour < 5 {
    0.9
  } else if trip.start_hour >= 20 {
    0.4
  } else {
    0.0
  }
}

///|
fn classify_loop_trip(trip : Trip) -> Double {
  let dist = haversine_m(
    trip.start_lat,
    trip.start_lon,
    trip.end_lat,
    trip.end_lon,
  )
  if dist < 500.0 {
    let conf = 1.0 - dist / 500.0
    if conf > 0.95 {
      0.95
    } else {
      conf
    }
  } else {
    0.0
  }
}

// ---- Run all classifiers ----

///|
fn run_classification(trip : Trip) -> Array[ClassLabel] {
  let labels : Array[ClassLabel] = []
  let commute = classify_commute(trip)
  if commute > 0.0 {
    labels.push({ name: "commute", confidence: commute, })
  }
  let errand = classify_short_errand(trip)
  if errand > 0.0 {
    labels.push({ name: "short_errand", confidence: errand, })
  }
  let road = classify_road_trip(trip)
  if road > 0.0 {
    labels.push({ name: "road_trip", confidence: road, })
  }
  let canyon = classify_canyon_drive(trip)
  if canyon > 0.0 {
    labels.push({ name: "canyon_drive", confidence: canyon, })
  }
  let night = classify_night_drive(trip)
  if night > 0.0 {
    labels.push({ name: "night_drive", confidence: night, })
  }
  let loop_t = classify_loop_trip(trip)
  if loop_t > 0.0 {
    labels.push({ name: "loop_trip", confidence: loop_t, })
  }
  // Sort descending by confidence (selection sort, array is small)
  for i = 0; i < labels.length(); i = i + 1 {
    let mut max_idx = i
    for j = i + 1; j < labels.length(); j = j + 1 {
      if labels[j].confidence > labels[max_idx].confidence {
        max_idx = j
      }
    }
    if max_idx != i {
      let tmp = labels[i]
      labels[i] = labels[max_idx]
      labels[max_idx] = tmp
    }
  }
  labels
}

// ---- Output JSON serialization ----

///|
fn format_confidence(val : Double) -> String {
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
fn format_output(labels : Array[ClassLabel]) -> String {
  let buf = StringBuilder()
  buf.write_string(
    "{\"abi_version\":1,\"plugin\":\"trip-classifier\",\"version\":\"0.1.0\",\"labels\":[",
  )
  for i = 0; i < labels.length(); i = i + 1 {
    if i > 0 {
      buf.write_char(',')
    }
    buf.write_string("{\"name\":\"")
    buf.write_string(labels[i].name)
    buf.write_string("\",\"confidence\":")
    buf.write_string(format_confidence(labels[i].confidence))
    buf.write_char('}')
  }
  buf.write_string("]}")
  buf.to_string()
}

///|
fn empty_output() -> String {
  "{\"abi_version\":1,\"plugin\":\"trip-classifier\",\"version\":\"0.1.0\",\"labels\":[]}"
}

// ---- Internal classify logic ----

///|
fn do_classify(input : String) -> String {
  let json : Json = @json.parse(input) catch { _ => return empty_output() }
  let trip = parse_trip(json)
  let labels = run_classification(trip)
  format_output(labels)
}

// ---- Exported classify function ----
// Host calls alloc(n) to get a pointer, writes n bytes of JSON input,
// then calls classify(ptr, len). Returns pointer to output buffer:
//   [4 bytes LE u32 = output length][output JSON bytes]

///|
#export_name("classify")
pub fn classify(ptr : Int, len : Int) -> Int {
  let input_str = read_utf8(ptr, len)
  let output = do_classify(input_str)
  let byte_len = utf8_byte_length(output)
  let out_ptr = alloc(byte_len + 4)
  store_i32_le(out_ptr, byte_len)
  let _ = write_utf8(output, out_ptr + 4)
  out_ptr
}
