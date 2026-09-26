/// Data quality checker for trip data.
///
/// Analyzes trip data and flags quality issues:
/// - Impossible GNSS jumps (>200m between consecutive samples at reported speed)
/// - Prolonged GPS loss (>5 min gap in samples during a trip)
/// - Duplicate tracks (same start/end within 30s, similar distance)
/// - Suspicious clock drift (timestamps not monotonically increasing)
///
/// Flags are written to the trip_quality_flags table with ON CONFLICT DO UPDATE
/// so re-running updates existing flags rather than duplicating them.

import gleam/dynamic/decode
import gleam/int
import gleam/io
import gleam/list
import gleam/option.{type Option, None, Some}
import pog

/// A quality flag to write.
pub type QualityFlag {
  QualityFlag(
    trip_id: String,
    flag_type: String,
    severity: String,
    details: String,
  )
}

/// Run all quality checks against all trips that have location samples.
pub fn check_all(conn: pog.Connection) -> Result(Int, String) {
  case get_trip_ids(conn) {
    Ok(trip_ids) -> {
      let total_flags =
        list.fold(trip_ids, 0, fn(acc, trip_id) {
          let flags = check_trip(conn, trip_id)
          let written = write_flags(conn, flags)
          acc + written
        })

      io.println(
        "Quality checker: checked "
        <> int.to_string(list.length(trip_ids))
        <> " trips, wrote "
        <> int.to_string(total_flags)
        <> " flags",
      )
      Ok(total_flags)
    }

    Error(_) -> {
      io.println("Quality checker: failed to query trip IDs")
      Error("Failed to query trip IDs")
    }
  }
}

/// Get all trip IDs that have location samples.
fn get_trip_ids(conn: pog.Connection) -> Result(List(String), Nil) {
  let query =
    pog.query("SELECT DISTINCT trip_id FROM location_samples ORDER BY trip_id")
    |> pog.returning(decode.field(0, decode.string, decode.success))

  case pog.execute(query: query, on: conn) {
    Ok(response) -> Ok(list.map(response.rows, fn(row) { row }))
    Error(_) -> Error(Nil)
  }
}

/// Run quality checks on a single trip and return any flags found.
fn check_trip(conn: pog.Connection, trip_id: String) -> List(QualityFlag) {
  let gnss_flags = check_gnss_jumps(conn, trip_id)
  let gps_loss_flags = check_gps_loss(conn, trip_id)
  let clock_flags = check_clock_drift(conn, trip_id)
  let duplicate_flags = check_duplicates(conn, trip_id)
  list.flatten([gnss_flags, gps_loss_flags, clock_flags, duplicate_flags])
}

/// A location sample with timestamp and coordinates.
type Sample {
  Sample(timestamp_ms: Int, lat: Float, lon: Float, speed_mps: Option(Float))
}

/// Decoder for location samples.
fn sample_decoder() -> decode.Decoder(Sample) {
  use ts <- decode.field(0, decode.int)
  use lat <- decode.field(1, decode.float)
  use lon <- decode.field(2, decode.float)
  use speed <- decode.field(3, decode.optional(decode.float))
  decode.success(Sample(timestamp_ms: ts, lat: lat, lon: lon, speed_mps: speed))
}

/// Fetch location samples for a trip ordered by timestamp.
fn get_samples(conn: pog.Connection, trip_id: String) -> List(Sample) {
  let query =
    pog.query(
      "
      SELECT timestamp_ms, latitude, longitude, speed_mps
      FROM location_samples
      WHERE trip_id = $1
      ORDER BY timestamp_ms ASC
      ",
    )
    |> pog.parameter(pog.text(trip_id))
    |> pog.returning(sample_decoder())

  case pog.execute(query: query, on: conn) {
    Ok(response) -> response.rows
    Error(_) -> []
  }
}

/// Check for impossible GNSS jumps: >200m between consecutive samples
/// relative to reported speed.
fn check_gnss_jumps(
  conn: pog.Connection,
  trip_id: String,
) -> List(QualityFlag) {
  let samples = get_samples(conn, trip_id)
  let has_jump = check_pairs_for_jump(samples)
  case has_jump {
    True -> [
      QualityFlag(
        trip_id: trip_id,
        flag_type: "gnss_jump",
        severity: "warning",
        details: "{\"description\": \"Impossible GNSS jump detected (>200m between consecutive samples)\"}",
      ),
    ]
    False -> []
  }
}

/// Walk consecutive pairs checking for jumps >200m.
fn check_pairs_for_jump(samples: List(Sample)) -> Bool {
  case samples {
    [] -> False
    [_] -> False
    [a, b, ..rest] -> {
      let dist = haversine_m(a.lat, a.lon, b.lat, b.lon)
      let time_s =
        int.to_float(b.timestamp_ms - a.timestamp_ms) /. 1000.0

      // If distance > 200m and either no time elapsed or speed doesn't explain it
      let max_plausible = case a.speed_mps {
        Some(spd) -> spd *. time_s *. 2.0
        // generous 2x buffer
        None -> 200.0
      }

      case dist >. 200.0 && dist >. max_plausible {
        True -> True
        False -> check_pairs_for_jump([b, ..rest])
      }
    }
  }
}

/// Check for prolonged GPS loss: >5 minutes gap between samples.
fn check_gps_loss(conn: pog.Connection, trip_id: String) -> List(QualityFlag) {
  let samples = get_samples(conn, trip_id)
  let five_min_ms = 5 * 60 * 1000
  let has_gap = check_pairs_for_gap(samples, five_min_ms)
  case has_gap {
    True -> [
      QualityFlag(
        trip_id: trip_id,
        flag_type: "gps_loss",
        severity: "warning",
        details: "{\"description\": \"Prolonged GPS loss detected (>5 minute gap in samples)\"}",
      ),
    ]
    False -> []
  }
}

/// Walk consecutive pairs checking for time gaps exceeding threshold.
fn check_pairs_for_gap(samples: List(Sample), threshold_ms: Int) -> Bool {
  case samples {
    [] -> False
    [_] -> False
    [a, b, ..rest] -> {
      let gap = b.timestamp_ms - a.timestamp_ms
      case gap > threshold_ms {
        True -> True
        False -> check_pairs_for_gap([b, ..rest], threshold_ms)
      }
    }
  }
}

/// Check for clock drift: timestamps not monotonically increasing.
fn check_clock_drift(
  conn: pog.Connection,
  trip_id: String,
) -> List(QualityFlag) {
  let samples = get_samples(conn, trip_id)
  let has_drift = check_monotonic(samples)
  case has_drift {
    True -> [
      QualityFlag(
        trip_id: trip_id,
        flag_type: "clock_drift",
        severity: "warning",
        details: "{\"description\": \"Timestamps are not monotonically increasing\"}",
      ),
    ]
    False -> []
  }
}

/// Check that timestamps are strictly increasing.
fn check_monotonic(samples: List(Sample)) -> Bool {
  case samples {
    [] -> False
    [_] -> False
    [a, b, ..rest] ->
      case b.timestamp_ms <= a.timestamp_ms {
        True -> True
        False -> check_monotonic([b, ..rest])
      }
  }
}

/// Check for duplicate tracks: another trip with same start/end within 30s
/// and similar distance.
fn check_duplicates(
  conn: pog.Connection,
  trip_id: String,
) -> List(QualityFlag) {
  let query =
    pog.query(
      "
      SELECT COUNT(*) FROM trips t1
      JOIN trips t2 ON t1.id != t2.id
        AND t2.id = $1
        AND ABS(EXTRACT(EPOCH FROM (t1.started_at - t2.started_at))) < 30
        AND t1.ended_at IS NOT NULL
        AND t2.ended_at IS NOT NULL
        AND ABS(EXTRACT(EPOCH FROM (t1.ended_at - t2.ended_at))) < 30
        AND t1.distance_m IS NOT NULL
        AND t2.distance_m IS NOT NULL
        AND ABS(t1.distance_m - t2.distance_m) < GREATEST(t1.distance_m, t2.distance_m) * 0.1
      ",
    )
    |> pog.parameter(pog.text(trip_id))
    |> pog.returning(decode.field(0, decode.int, decode.success))

  case pog.execute(query: query, on: conn) {
    Ok(response) ->
      case response.rows {
        [count, ..] if count > 0 -> [
          QualityFlag(
            trip_id: trip_id,
            flag_type: "duplicate_track",
            severity: "info",
            details: "{\"description\": \"Possible duplicate track detected\"}",
          ),
        ]
        _ -> []
      }
    Error(_) -> []
  }
}

/// Write quality flags to the database. Uses ON CONFLICT to update existing
/// flags rather than creating duplicates (idempotent).
fn write_flags(conn: pog.Connection, flags: List(QualityFlag)) -> Int {
  list.fold(flags, 0, fn(acc, flag) {
    let query =
      pog.query(
        "
        INSERT INTO trip_quality_flags (trip_id, flag_type, severity, details)
        VALUES ($1, $2, $3, $4::jsonb)
        ON CONFLICT (trip_id, flag_type) DO UPDATE
          SET severity = EXCLUDED.severity,
              details = EXCLUDED.details,
              flagged_at = now(),
              resolved_at = NULL
        ",
      )
      |> pog.parameter(pog.text(flag.trip_id))
      |> pog.parameter(pog.text(flag.flag_type))
      |> pog.parameter(pog.text(flag.severity))
      |> pog.parameter(pog.text(flag.details))

    case pog.execute(query: query, on: conn) {
      Ok(_) -> acc + 1
      Error(_) -> acc
    }
  })
}

/// Approximate haversine distance in meters between two lat/lon pairs.
fn haversine_m(lat1: Float, lon1: Float, lat2: Float, lon2: Float) -> Float {
  let r = 6_371_000.0
  let d_lat = to_rad(lat2 -. lat1)
  let d_lon = to_rad(lon2 -. lon1)
  let lat1_r = to_rad(lat1)
  let lat2_r = to_rad(lat2)

  let a =
    sin_approx(d_lat /. 2.0)
    *. sin_approx(d_lat /. 2.0)
    +. cos_approx(lat1_r)
    *. cos_approx(lat2_r)
    *. sin_approx(d_lon /. 2.0)
    *. sin_approx(d_lon /. 2.0)

  let c = 2.0 *. atan2_approx(sqrt_approx(a), sqrt_approx(1.0 -. a))
  r *. c
}

/// Degrees to radians.
fn to_rad(deg: Float) -> Float {
  deg *. 3.14159265358979 /. 180.0
}

/// Taylor series sine approximation.
fn sin_approx(x: Float) -> Float {
  let x2 = x *. x
  let x3 = x2 *. x
  let x5 = x3 *. x2
  let x7 = x5 *. x2
  x -. x3 /. 6.0 +. x5 /. 120.0 -. x7 /. 5040.0
}

/// Taylor series cosine approximation.
fn cos_approx(x: Float) -> Float {
  let x2 = x *. x
  let x4 = x2 *. x2
  let x6 = x4 *. x2
  1.0 -. x2 /. 2.0 +. x4 /. 24.0 -. x6 /. 720.0
}

/// Simple square root via Newton's method.
fn sqrt_approx(x: Float) -> Float {
  case x <=. 0.0 {
    True -> 0.0
    False -> newton_sqrt(x, x /. 2.0, 0)
  }
}

fn newton_sqrt(x: Float, guess: Float, iter: Int) -> Float {
  case iter >= 10 {
    True -> guess
    False -> newton_sqrt(x, { guess +. x /. guess } /. 2.0, iter + 1)
  }
}

/// Approximation of atan2.
fn atan2_approx(y: Float, x: Float) -> Float {
  let pi = 3.14159265358979
  case x >. 0.0 {
    True -> atan_approx(y /. x)
    False ->
      case x <. 0.0 && y >=. 0.0 {
        True -> atan_approx(y /. x) +. pi
        False ->
          case x <. 0.0 {
            True -> atan_approx(y /. x) -. pi
            False ->
              case y >. 0.0 {
                True -> pi /. 2.0
                False ->
                  case y <. 0.0 {
                    True -> 0.0 -. pi /. 2.0
                    False -> 0.0
                  }
              }
          }
      }
  }
}

/// Taylor series arctangent for values near zero.
fn atan_approx(x: Float) -> Float {
  let x2 = x *. x
  let x3 = x2 *. x
  let x5 = x3 *. x2
  let x7 = x5 *. x2
  x -. x3 /. 3.0 +. x5 /. 5.0 -. x7 /. 7.0
}
