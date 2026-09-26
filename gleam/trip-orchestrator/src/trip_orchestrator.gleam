import gleam/erlang/process.{type Subject}
import gleam/int
import gleam/io
import gleam/otp/actor
import pog
import trip_orchestrator/automation
import trip_orchestrator/db
import trip_orchestrator/lifecycle
import trip_orchestrator/notifier
import trip_orchestrator/quality
import trip_orchestrator/scheduler

pub type WorkerMsg {
  Tick
  Shutdown
}

type WorkerState {
  WorkerState(conn: pog.Connection, self: Subject(WorkerMsg))
}

fn start_periodic_worker(
  conn: pog.Connection,
  name: String,
  interval_ms: Int,
  task: fn(pog.Connection) -> Nil,
) -> Result(Subject(WorkerMsg), actor.StartError) {
  actor.new_with_initialiser(5000, fn(self) {
    schedule_tick(self, interval_ms)
    let state = WorkerState(conn: conn, self: self)
    actor.initialised(state)
    |> actor.returning(self)
    |> Ok
  })
  |> actor.on_message(fn(state, msg) {
    case msg {
      Tick -> {
        io.println("[" <> name <> "] Running...")
        task(state.conn)
        schedule_tick(state.self, interval_ms)
        actor.continue(state)
      }
      Shutdown -> {
        io.println("[" <> name <> "] Shutting down")
        actor.stop()
      }
    }
  })
  |> actor.start
  |> result_map_started
}

fn result_map_started(
  res: Result(actor.Started(Subject(WorkerMsg)), actor.StartError),
) -> Result(Subject(WorkerMsg), actor.StartError) {
  case res {
    Ok(started) -> Ok(started.data)
    Error(e) -> Error(e)
  }
}

fn schedule_tick(subject: Subject(WorkerMsg), interval_ms: Int) -> Nil {
  process.send_after(subject, interval_ms, Tick)
  Nil
}

pub fn main() -> Nil {
  io.println("Cairn trip-orchestrator starting...")

  let config = db.default_config()
  io.println(
    "Connecting to database at "
    <> config.host
    <> ":"
    <> int.to_string(config.port),
  )

  let assert Ok(conn) = db.connect(config)
  io.println("Database connection pool started")

  let assert Ok(_) =
    start_periodic_worker(conn, "lifecycle", 30_000, fn(c) {
      let _ = lifecycle.project(c)
      Nil
    })
  io.println("  Started lifecycle projector (30s interval)")

  let assert Ok(_) =
    start_periodic_worker(conn, "quality", 60_000, fn(c) {
      let _ = quality.check_all(c)
      Nil
    })
  io.println("  Started quality checker (60s interval)")

  let assert Ok(_) =
    start_periodic_worker(conn, "scheduler", 120_000, fn(c) {
      let sched_config = scheduler.default_config()
      let _ = scheduler.process_batch(c, sched_config)
      Nil
    })
  io.println("  Started retry scheduler (120s interval)")

  let assert Ok(_) =
    start_periodic_worker(conn, "notifier", 60_000, fn(c) {
      let notify_config = notifier.default_config()
      let _ = notifier.notify_quality_flags(c, notify_config)
      Nil
    })
  io.println("  Started notification evaluator (60s interval)")

  let assert Ok(_) =
    start_periodic_worker(conn, "automation", 60_000, fn(c) {
      let _ = automation.run(c)
      Nil
    })
  io.println("  Started automation executor (60s interval)")

  io.println("All workers started. Trip orchestrator is running.")
  process.sleep_forever()
}
