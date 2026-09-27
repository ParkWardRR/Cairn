// route-scorer: WASM plugin for scoring route repetition and novelty.
// Exports alloc() and score_route() for host interop over linear memory.

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
priv struct RoutePoint {
  lat : Double
  lon : Double
  speed_mps : Double
  timestamp_ms : Double
}

///|
priv struct SimplePoint {
  lat : Double
  lon : Double
}

///|
priv struct HistoricalRoute {
  route : Array[SimplePoint]
  count : Int
}

///|
priv struct FavoriteSegment {
  start_idx : Int
  end_idx : Int
  match_count : Int
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
fn parse_route(json : Json) -> Array[RoutePoint] {
  let points : Array[RoutePoint] = []
  match json {
    Object(top) =>
      match top.get("route") {
        Some(Json::Array(arr)) =>
          for item in arr {
            match item {
              Object(m) =>
                points.push(
                  {
                    lat: get_number(m, "lat"),
                    lon: get_number(m, "lon"),
                    speed_mps: get_number(m, "speed_mps"),
                    timestamp_ms: get_number(m, "timestamp_ms"),
                  },
                )
              _ => ()
            }
          }
        _ => ()
      }
    _ => ()
  }
  points
}

///|
fn parse_history(json : Json) -> Array[HistoricalRoute] {
  let routes : Array[HistoricalRoute] = []
  match json {
    Object(top) =>
      match top.get("history") {
        Some(Json::Array(arr)) =>
          for item in arr {
            match item {
              Object(m) => {
                let count = get_int(m, "count")
                let pts : Array[SimplePoint] = match m.get("route") {
                  Some(Json::Array(parr)) => {
                    let result : Array[SimplePoint] = []
                    for p in parr {
                      match p {
                        Object(pm) =>
                          result.push(
                            { lat: get_number(pm, "lat"), lon: get_number(pm, "lon"), },
                          )
                        _ => ()
                      }
                    }
                    result
                  }
                  _ => []
                }
                routes.push({ route: pts, count, })
              }
              _ => ()
            }
          }
        _ => ()
      }
    _ => ()
  }
  routes
}

// ---- Scoring logic ----

// corridor_threshold_m: points within 50m are considered a match
///|
let corridor_m : Double = 50.0

///|
fn score_against_history(
  route : Array[RoutePoint],
  history : Array[HistoricalRoute],
) -> (Double, Array[FavoriteSegment], Double) {
  if route.length() == 0 {
    return (0.0, [], 1.0)
  }
  if history.length() == 0 {
    return (0.0, [], 1.0)
  }

  // For each point in the current route, check if it is within corridor_m
  // of any point in any historical route.
  let n = route.length()
  let matched : Array[Bool] = Array::make(n, false)
  let match_counts : Array[Int] = Array::make(n, 0)

  for hist in history {
    for i = 0; i < n; i = i + 1 {
      let rp = route[i]
      let mut found = false
      for hp in hist.route {
        let dist = haversine_m(rp.lat, rp.lon, hp.lat, hp.lon)
        if dist <= corridor_m {
          found = true
          break
        }
      }
      if found {
        matched[i] = true
        match_counts[i] = match_counts[i] + hist.count
      }
    }
  }

  // Compute repeat_score = fraction of points matched
  let mut matched_count = 0
  for i = 0; i < n; i = i + 1 {
    if matched[i] {
      matched_count = matched_count + 1
    }
  }
  let repeat_score = matched_count.to_double() / n.to_double()
  let novelty_score = 1.0 - repeat_score

  // Identify contiguous matched segments as "favorite segments"
  let segments : Array[FavoriteSegment] = []
  let mut seg_start = -1
  let mut seg_max_count = 0
  for i = 0; i < n; i = i + 1 {
    if matched[i] {
      if seg_start < 0 {
        seg_start = i
        seg_max_count = match_counts[i]
      }
      if match_counts[i] > seg_max_count {
        seg_max_count = match_counts[i]
      }
    } else {
      if seg_start >= 0 {
        // Only record segments of at least 3 points
        if i - seg_start >= 3 {
          segments.push(
            { start_idx: seg_start, end_idx: i - 1, match_count: seg_max_count, },
          )
        }
        seg_start = -1
        seg_max_count = 0
      }
    }
  }
  // Close final segment
  if seg_start >= 0 && n - seg_start >= 3 {
    segments.push(
      { start_idx: seg_start, end_idx: n - 1, match_count: seg_max_count, },
    )
  }

  (repeat_score, segments, novelty_score)
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
fn format_output(
  repeat_score : Double,
  segments : Array[FavoriteSegment],
  novelty_score : Double,
) -> String {
  let buf = StringBuilder()
  buf.write_string("{\"repeat_score\":")
  buf.write_string(format_score(repeat_score))
  buf.write_string(",\"favorite_segments\":[")
  for i = 0; i < segments.length(); i = i + 1 {
    if i > 0 {
      buf.write_char(',')
    }
    let s = segments[i]
    buf.write_string("{\"start_idx\":")
    buf.write_string(s.start_idx.to_string())
    buf.write_string(",\"end_idx\":")
    buf.write_string(s.end_idx.to_string())
    buf.write_string(",\"match_count\":")
    buf.write_string(s.match_count.to_string())
    buf.write_char('}')
  }
  buf.write_string("],\"novelty_score\":")
  buf.write_string(format_score(novelty_score))
  buf.write_char('}')
  buf.to_string()
}

///|
fn empty_output() -> String {
  "{\"repeat_score\":0.00,\"favorite_segments\":[],\"novelty_score\":1.00}"
}

// ---- Main logic ----

///|
fn do_score(input : String) -> String {
  let json : Json = @json.parse(input) catch { _ => return empty_output() }
  let route = parse_route(json)
  let history = parse_history(json)
  let (repeat_score, segments, novelty_score) = score_against_history(
    route, history,
  )
  format_output(repeat_score, segments, novelty_score)
}

// ---- Exported function ----

///|
#export_name("score_route")
pub fn score_route(ptr : Int, len : Int) -> Int {
  let input_str = read_utf8(ptr, len)
  let output = do_score(input_str)
  let byte_len = utf8_byte_length(output)
  let out_ptr = alloc(byte_len + 4)
  store_i32_le(out_ptr, byte_len)
  let _ = write_utf8(output, out_ptr + 4)
  out_ptr
}
