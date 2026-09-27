// export-transformer: WASM plugin for converting trip data into various formats.
// Exports alloc() and export_trip() for host interop over linear memory.

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
priv struct RouteSample {
  lat : Double
  lon : Double
  altitude_m : Double
  speed_mps : Double
  timestamp_ms : Double
}

///|
priv struct TripData {
  trip_id : String
  started_at : String
  ended_at : String
  distance_m : Double
  duration_s : Double
  route_samples : Array[RouteSample]
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
        altitude_m: get_number(m, "altitude_m"),
        speed_mps: get_number(m, "speed_mps"),
        timestamp_ms: get_number(m, "timestamp_ms"),
      }
    _ => { lat: 0.0, lon: 0.0, altitude_m: 0.0, speed_mps: 0.0, timestamp_ms: 0.0, }
  }
}

///|
fn parse_input(json : Json) -> (TripData, String) {
  match json {
    Object(top) => {
      let format = get_string_field(top, "format")
      let trip = match top.get("trip") {
        Some(Object(t)) => {
          let samples : Array[RouteSample] = match t.get("route_samples") {
            Some(Json::Array(arr)) => arr.map(parse_route_sample)
            _ => []
          }
          {
            trip_id: get_string_field(t, "trip_id"),
            started_at: get_string_field(t, "started_at"),
            ended_at: get_string_field(t, "ended_at"),
            distance_m: get_number(t, "distance_m"),
            duration_s: get_number(t, "duration_s"),
            route_samples: samples,
          }
        }
        _ =>
          {
            trip_id: "",
            started_at: "",
            ended_at: "",
            distance_m: 0.0,
            duration_s: 0.0,
            route_samples: [],
          }
      }
      (trip, format)
    }
    _ =>
      (
        {
          trip_id: "",
          started_at: "",
          ended_at: "",
          distance_m: 0.0,
          duration_s: 0.0,
          route_samples: [],
        },
        "",
      )
  }
}

// ---- Format helpers ----

///|
fn format_f7(val : Double) -> String {
  // Format a double with 7 decimal places
  let neg = val < 0.0
  let abs_val = if neg { -val } else { val }
  let int_part = abs_val.to_int()
  let frac = ((abs_val - int_part.to_double()) * 10000000.0 + 0.5).to_int()
  let frac_str = {
    let s = frac.to_string()
    let pad = 7 - s.length()
    let mut result = ""
    let mut i = 0
    while i < pad {
      result = result + "0"
      i = i + 1
    }
    result + s
  }
  let prefix = if neg { "-" } else { "" }
  "\{prefix}\{int_part}.\{frac_str}"
}

///|
fn format_f1(val : Double) -> String {
  let neg = val < 0.0
  let abs_val = if neg { -val } else { val }
  let int_part = abs_val.to_int()
  let frac = ((abs_val - int_part.to_double()) * 10.0 + 0.5).to_int()
  let prefix = if neg { "-" } else { "" }
  "\{prefix}\{int_part}.\{frac}"
}

///|
fn format_f2(val : Double) -> String {
  let neg = val < 0.0
  let abs_val = if neg { -val } else { val }
  let int_part = abs_val.to_int()
  let frac = ((abs_val - int_part.to_double()) * 100.0 + 0.5).to_int()
  let frac_str = if frac < 10 { "0\{frac}" } else { frac.to_string() }
  let prefix = if neg { "-" } else { "" }
  "\{prefix}\{int_part}.\{frac_str}"
}

///|
fn xml_escape(s : String) -> String {
  let buf = StringBuilder()
  for c in s {
    match c {
      '&' => buf.write_string("&amp;")
      '<' => buf.write_string("&lt;")
      '>' => buf.write_string("&gt;")
      '"' => buf.write_string("&quot;")
      _ => buf.write_char(c)
    }
  }
  buf.to_string()
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
fn ts_from_ms(ms : Double) -> String {
  // Convert milliseconds to a rough ISO 8601 timestamp.
  // We approximate by treating ms as Unix epoch ms.
  let total_s = (ms / 1000.0).to_int()
  let days = total_s / 86400
  let rem = total_s % 86400
  let hours = rem / 3600
  let mins = (rem % 3600) / 60
  let secs = rem % 60

  // Simple epoch-to-date using days since 1970-01-01
  let mut y = 1970
  let mut d = days
  while true {
    let leap = if (y % 4 == 0 && y % 100 != 0) || y % 400 == 0 {
      1
    } else {
      0
    }
    let year_days = 365 + leap
    if d < year_days {
      break
    }
    d = d - year_days
    y = y + 1
  }
  let leap = if (y % 4 == 0 && y % 100 != 0) || y % 400 == 0 {
    1
  } else {
    0
  }
  let month_days = [
    31, 28 + leap, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31,
  ]
  let mut m = 0
  while m < 12 && d >= month_days[m] {
    d = d - month_days[m]
    m = m + 1
  }
  m = m + 1
  d = d + 1

  let pad2 = fn(n : Int) -> String {
    if n < 10 {
      "0\{n}"
    } else {
      n.to_string()
    }
  }

  "\{y}-\{pad2(m)}-\{pad2(d)}T\{pad2(hours)}:\{pad2(mins)}:\{pad2(secs)}Z"
}

// ---- GPX format ----

///|
fn generate_gpx(trip : TripData) -> String {
  let buf = StringBuilder()
  buf.write_string(
    "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<gpx version=\"1.1\" creator=\"Cairn\"\n     xmlns=\"http://www.topografix.com/GPX/1/1\">\n  <trk>\n    <name>",
  )
  buf.write_string(xml_escape("Trip \{trip.trip_id}"))
  buf.write_string("</name>\n    <trkseg>\n")
  for sample in trip.route_samples {
    let ts = ts_from_ms(sample.timestamp_ms)
    buf.write_string("      <trkpt lat=\"")
    buf.write_string(format_f7(sample.lat))
    buf.write_string("\" lon=\"")
    buf.write_string(format_f7(sample.lon))
    buf.write_string("\">\n        <ele>")
    buf.write_string(format_f1(sample.altitude_m))
    buf.write_string("</ele>\n        <time>")
    buf.write_string(ts)
    buf.write_string("</time>\n        <speed>")
    buf.write_string(format_f2(sample.speed_mps))
    buf.write_string("</speed>\n      </trkpt>\n")
  }
  buf.write_string("    </trkseg>\n  </trk>\n</gpx>\n")
  buf.to_string()
}

// ---- GeoJSON format ----

///|
fn generate_geojson(trip : TripData) -> String {
  let buf = StringBuilder()
  buf.write_string(
    "{\"type\":\"FeatureCollection\",\"features\":[{\"type\":\"Feature\",\"geometry\":{\"type\":\"LineString\",\"coordinates\":[",
  )
  for i = 0; i < trip.route_samples.length(); i = i + 1 {
    if i > 0 {
      buf.write_char(',')
    }
    let s = trip.route_samples[i]
    buf.write_char('[')
    buf.write_string(format_f7(s.lon))
    buf.write_char(',')
    buf.write_string(format_f7(s.lat))
    buf.write_char(',')
    buf.write_string(format_f1(s.altitude_m))
    buf.write_char(']')
  }
  buf.write_string("],\"properties\":{\"trip_id\":\"")
  buf.write_string(json_escape(trip.trip_id))
  buf.write_string("\"}}]}")
  buf.to_string()
}

// ---- CSV format ----

///|
fn generate_csv(trip : TripData) -> String {
  let buf = StringBuilder()
  buf.write_string("timestamp_ms,latitude,longitude,altitude_m,speed_mps\n")
  for sample in trip.route_samples {
    let ts_int = sample.timestamp_ms.to_int64()
    buf.write_string(ts_int.to_string())
    buf.write_char(',')
    buf.write_string(format_f7(sample.lat))
    buf.write_char(',')
    buf.write_string(format_f7(sample.lon))
    buf.write_char(',')
    buf.write_string(format_f1(sample.altitude_m))
    buf.write_char(',')
    buf.write_string(format_f2(sample.speed_mps))
    buf.write_char('\n')
  }
  buf.to_string()
}

// ---- Markdown format ----

///|
fn generate_markdown(trip : TripData) -> String {
  let buf = StringBuilder()
  buf.write_string("# Trip Report\n\n")
  buf.write_string("**Trip ID:** ")
  buf.write_string(trip.trip_id)
  buf.write_char('\n')
  buf.write_string("**Started:** ")
  buf.write_string(trip.started_at)
  buf.write_char('\n')
  buf.write_string("**Ended:** ")
  buf.write_string(trip.ended_at)
  buf.write_char('\n')
  let dist_km = trip.distance_m / 1000.0
  buf.write_string("**Distance:** ")
  buf.write_string(format_f2(dist_km))
  buf.write_string(" km\n")
  let dur_min = trip.duration_s / 60.0
  buf.write_string("**Duration:** ")
  buf.write_string(format_f1(dur_min))
  buf.write_string(" minutes\n")
  let n = trip.route_samples.length()
  buf.write_string("**Route Points:** ")
  buf.write_string(n.to_string())
  buf.write_char('\n')
  if n > 0 {
    let mut max_speed = 0.0
    let mut total_speed = 0.0
    let mut min_alt = trip.route_samples[0].altitude_m
    let mut max_alt = min_alt
    for sample in trip.route_samples {
      if sample.speed_mps > max_speed {
        max_speed = sample.speed_mps
      }
      total_speed = total_speed + sample.speed_mps
      if sample.altitude_m < min_alt {
        min_alt = sample.altitude_m
      }
      if sample.altitude_m > max_alt {
        max_alt = sample.altitude_m
      }
    }
    let avg_speed = total_speed / n.to_double()
    let max_speed_kmh = max_speed * 3.6
    let avg_speed_kmh = avg_speed * 3.6
    buf.write_string("\n## Statistics\n\n")
    buf.write_string("| Metric | Value |\n")
    buf.write_string("|--------|-------|\n")
    buf.write_string("| Avg Speed | ")
    buf.write_string(format_f1(avg_speed_kmh))
    buf.write_string(" km/h |\n")
    buf.write_string("| Max Speed | ")
    buf.write_string(format_f1(max_speed_kmh))
    buf.write_string(" km/h |\n")
    buf.write_string("| Min Altitude | ")
    buf.write_string(format_f1(min_alt))
    buf.write_string(" m |\n")
    buf.write_string("| Max Altitude | ")
    buf.write_string(format_f1(max_alt))
    buf.write_string(" m |\n")
    buf.write_string("| Elevation Range | ")
    buf.write_string(format_f1(max_alt - min_alt))
    buf.write_string(" m |\n")
    // Start and end points
    let first = trip.route_samples[0]
    let last = trip.route_samples[n - 1]
    buf.write_string("\n## Endpoints\n\n")
    buf.write_string("- **Start:** ")
    buf.write_string(format_f7(first.lat))
    buf.write_string(", ")
    buf.write_string(format_f7(first.lon))
    buf.write_char('\n')
    buf.write_string("- **End:** ")
    buf.write_string(format_f7(last.lat))
    buf.write_string(", ")
    buf.write_string(format_f7(last.lon))
    buf.write_char('\n')
  }
  buf.to_string()
}

// ---- Main logic ----

///|
fn do_export(input : String) -> String {
  let json : Json = @json.parse(input) catch { _ => return error_output("JSON parse error") }
  let (trip, format) = parse_input(json)
  let content = match format {
    "gpx" => generate_gpx(trip)
    "geojson" => generate_geojson(trip)
    "csv" => generate_csv(trip)
    "markdown" => generate_markdown(trip)
    _ => generate_gpx(trip)
  }
  let actual_format = match format {
    "gpx" | "geojson" | "csv" | "markdown" => format
    _ => "gpx"
  }
  let escaped = json_escape(content)
  "{\"format\":\"\{actual_format}\",\"content\":\"\{escaped}\"}"
}

///|
fn error_output(msg : String) -> String {
  "{\"format\":\"error\",\"content\":\"\{json_escape(msg)}\"}"
}

// ---- Exported function ----

///|
#export_name("export_trip")
pub fn export_trip(ptr : Int, len : Int) -> Int {
  let input_str = read_utf8(ptr, len)
  let output = do_export(input_str)
  let byte_len = utf8_byte_length(output)
  let out_ptr = alloc(byte_len + 4)
  store_i32_le(out_ptr, byte_len)
  let _ = write_utf8(output, out_ptr + 4)
  out_ptr
}
