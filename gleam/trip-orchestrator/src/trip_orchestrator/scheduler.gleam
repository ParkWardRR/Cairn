/// Retry scheduler for trip reprocessing.
///
/// Manages a reprocess_queue table that tracks which trips need reprocessing
/// when algorithms are updated. Supports prioritization (lower priority number
/// = higher priority, but older trips processed first within same priority)
/// and idempotent operation.

import gleam/dynamic/decode
import gleam/int
import gleam/io
import gleam/list
import pog

/// Configuration for the scheduler.
pub type SchedulerConfig {
  SchedulerConfig(batch_size: Int)
}

/// Default scheduler configuration.
pub fn default_config() -> SchedulerConfig {
  SchedulerConfig(batch_size: 10)
}

/// A queued reprocessing job.
pub type QueuedJob {
  QueuedJob(id: Int, trip_id: String, reason: String, priority: Int)
}

/// Decoder for queued jobs.
fn job_decoder() -> decode.Decoder(QueuedJob) {
  use id <- decode.field(0, decode.int)
  use trip_id <- decode.field(1, decode.string)
  use reason <- decode.field(2, decode.string)
  use priority <- decode.field(3, decode.int)
  decode.success(QueuedJob(
    id: id,
    trip_id: trip_id,
    reason: reason,
    priority: priority,
  ))
}

/// Enqueue a trip for reprocessing. Idempotent: if a pending job already
/// exists for this trip with the same reason, it is not duplicated.
pub fn enqueue(
  conn: pog.Connection,
  trip_id: String,
  reason: String,
  priority: Int,
) -> Result(Nil, String) {
  let query =
    pog.query(
      "
      INSERT INTO reprocess_queue (trip_id, reason, priority)
      SELECT $1, $2, $3
      WHERE NOT EXISTS (
        SELECT 1 FROM reprocess_queue
        WHERE trip_id = $1
          AND reason = $2
          AND completed_at IS NULL
      )
      ",
    )
    |> pog.parameter(pog.text(trip_id))
    |> pog.parameter(pog.text(reason))
    |> pog.parameter(pog.int(priority))

  case pog.execute(query: query, on: conn) {
    Ok(_) -> Ok(Nil)
    Error(_) -> Error("Failed to enqueue trip " <> trip_id)
  }
}

/// Fetch the next batch of pending jobs, ordered by priority (ascending)
/// then by queue time (oldest first).
pub fn fetch_batch(
  conn: pog.Connection,
  config: SchedulerConfig,
) -> Result(List(QueuedJob), String) {
  let query =
    pog.query(
      "
      SELECT id, trip_id, reason, priority
      FROM reprocess_queue
      WHERE completed_at IS NULL
        AND started_at IS NULL
      ORDER BY priority ASC, queued_at ASC
      LIMIT $1
      ",
    )
    |> pog.parameter(pog.int(config.batch_size))
    |> pog.returning(job_decoder())

  case pog.execute(query: query, on: conn) {
    Ok(response) -> Ok(response.rows)
    Error(_) -> Error("Failed to fetch reprocess batch")
  }
}

/// Mark a job as started (in-progress).
pub fn mark_started(conn: pog.Connection, job_id: Int) -> Result(Nil, String) {
  let query =
    pog.query(
      "UPDATE reprocess_queue SET started_at = now() WHERE id = $1 AND started_at IS NULL",
    )
    |> pog.parameter(pog.int(job_id))

  case pog.execute(query: query, on: conn) {
    Ok(_) -> Ok(Nil)
    Error(_) -> Error("Failed to mark job started")
  }
}

/// Mark a job as completed successfully.
pub fn mark_completed(conn: pog.Connection, job_id: Int) -> Result(Nil, String) {
  let query =
    pog.query(
      "UPDATE reprocess_queue SET completed_at = now(), error_message = NULL WHERE id = $1",
    )
    |> pog.parameter(pog.int(job_id))

  case pog.execute(query: query, on: conn) {
    Ok(_) -> Ok(Nil)
    Error(_) -> Error("Failed to mark job completed")
  }
}

/// Mark a job as failed with an error message, resetting started_at so it
/// can be retried.
pub fn mark_failed(
  conn: pog.Connection,
  job_id: Int,
  error_message: String,
) -> Result(Nil, String) {
  let query =
    pog.query(
      "UPDATE reprocess_queue SET started_at = NULL, error_message = $2 WHERE id = $1",
    )
    |> pog.parameter(pog.int(job_id))
    |> pog.parameter(pog.text(error_message))

  case pog.execute(query: query, on: conn) {
    Ok(_) -> Ok(Nil)
    Error(_) -> Error("Failed to mark job failed")
  }
}

/// Process the next batch of reprocessing jobs. For each job, marks it
/// as started, performs a no-op reprocess (the actual reprocessing logic
/// would call into lifecycle/quality modules), and marks it completed.
///
/// Returns the number of successfully processed jobs.
pub fn process_batch(
  conn: pog.Connection,
  config: SchedulerConfig,
) -> Result(Int, String) {
  case fetch_batch(conn, config) {
    Ok(jobs) -> {
      let processed =
        list.fold(jobs, 0, fn(acc, job) {
          case mark_started(conn, job.id) {
            Ok(Nil) -> {
              // The actual reprocessing would happen here: re-run lifecycle
              // projection and quality checks for this trip.
              case mark_completed(conn, job.id) {
                Ok(Nil) -> acc + 1
                Error(msg) -> {
                  io.println(
                    "Scheduler: failed to complete job "
                    <> int.to_string(job.id)
                    <> ": "
                    <> msg,
                  )
                  acc
                }
              }
            }
            Error(msg) -> {
              io.println(
                "Scheduler: failed to start job "
                <> int.to_string(job.id)
                <> ": "
                <> msg,
              )
              acc
            }
          }
        })

      io.println(
        "Scheduler: processed "
        <> int.to_string(processed)
        <> " of "
        <> int.to_string(list.length(jobs))
        <> " queued jobs",
      )
      Ok(processed)
    }
    Error(msg) -> Error(msg)
  }
}
