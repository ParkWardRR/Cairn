/// Trip lifecycle projector.
///
/// Queries trips and their events from the database and projects coherent
/// lifecycle states: created -> recording -> finalized -> uploaded -> parsed -> enriched.
///
/// The projection is idempotent: re-running produces the same result thanks
/// to ON CONFLICT DO NOTHING on the (trip_id, state) unique constraint.

import gleam/dynamic/decode
import gleam/int
import gleam/io
import gleam/list
import pog

/// A lifecycle state for a trip.
pub type LifecycleState {
  Created
  Recording
  Finalized
  Uploaded
  Parsed
  Enriched
}

/// Convert a lifecycle state to its string representation.
pub fn state_to_string(state: LifecycleState) -> String {
  case state {
    Created -> "created"
    Recording -> "recording"
    Finalized -> "finalized"
    Uploaded -> "uploaded"
    Parsed -> "parsed"
    Enriched -> "enriched"
  }
}

/// A trip row from the database with the fields we need for projection.
pub type TripRow {
  TripRow(
    id: String,
    has_samples: Bool,
    has_ended: Bool,
    has_upload: Bool,
    has_events: Bool,
    has_derivations: Bool,
  )
}

/// Determine which lifecycle states a trip should have based on its data.
pub fn compute_states(trip: TripRow) -> List(LifecycleState) {
  let states = [Created]

  let states = case trip.has_samples {
    True -> list.append(states, [Recording])
    False -> states
  }

  let states = case trip.has_ended {
    True -> list.append(states, [Finalized])
    False -> states
  }

  let states = case trip.has_upload {
    True -> list.append(states, [Uploaded])
    False -> states
  }

  let states = case trip.has_events {
    True -> list.append(states, [Parsed])
    False -> states
  }

  let states = case trip.has_derivations {
    True -> list.append(states, [Enriched])
    False -> states
  }

  states
}

/// Decoder for trip rows with presence flags.
fn trip_row_decoder() -> decode.Decoder(TripRow) {
  use id <- decode.field(0, decode.string)
  use has_samples <- decode.field(1, decode.bool)
  use has_ended <- decode.field(2, decode.bool)
  use has_upload <- decode.field(3, decode.bool)
  use has_events <- decode.field(4, decode.bool)
  use has_derivations <- decode.field(5, decode.bool)
  decode.success(TripRow(
    id: id,
    has_samples: has_samples,
    has_ended: has_ended,
    has_upload: has_upload,
    has_events: has_events,
    has_derivations: has_derivations,
  ))
}

/// Run the lifecycle projector: query all trips, compute their states,
/// and write lifecycle entries idempotently.
pub fn project(conn: pog.Connection) -> Result(Int, String) {
  let query =
    pog.query(
      "
      SELECT
        t.id,
        EXISTS(SELECT 1 FROM location_samples ls WHERE ls.trip_id = t.id) AS has_samples,
        (t.ended_at IS NOT NULL) AS has_ended,
        (t.upload_id IS NOT NULL) AS has_upload,
        EXISTS(SELECT 1 FROM trip_events te WHERE te.trip_id = t.id) AS has_events,
        EXISTS(SELECT 1 FROM derivations d WHERE d.trip_id = t.id) AS has_derivations
      FROM trips t
      ORDER BY t.started_at ASC
      ",
    )
    |> pog.returning(trip_row_decoder())

  case pog.execute(query: query, on: conn) {
    Ok(response) -> {
      let trips = response.rows

      let total_inserted =
        list.fold(trips, 0, fn(acc, trip) {
          let states = compute_states(trip)
          let inserted = write_states(conn, trip.id, states)
          acc + inserted
        })

      io.println(
        "Lifecycle projector: processed "
        <> int.to_string(list.length(trips))
        <> " trips, wrote "
        <> int.to_string(total_inserted)
        <> " state transitions",
      )
      Ok(total_inserted)
    }

    Error(_err) -> {
      io.println("Lifecycle projector: failed to query trips")
      Error("Failed to query trips")
    }
  }
}

/// Write lifecycle states for a trip. Uses ON CONFLICT DO NOTHING for
/// idempotency. Returns the number of rows actually inserted.
fn write_states(
  conn: pog.Connection,
  trip_id: String,
  states: List(LifecycleState),
) -> Int {
  list.fold(states, 0, fn(acc, state) {
    let state_str = state_to_string(state)
    let query =
      pog.query(
        "
        INSERT INTO trip_lifecycle (trip_id, state)
        VALUES ($1, $2)
        ON CONFLICT (trip_id, state) DO NOTHING
        ",
      )
      |> pog.parameter(pog.text(trip_id))
      |> pog.parameter(pog.text(state_str))

    case pog.execute(query: query, on: conn) {
      Ok(_) -> acc + 1
      Error(_) -> acc
    }
  })
}
