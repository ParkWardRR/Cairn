import gleeunit
import gleeunit/should
import trip_orchestrator/lifecycle
import trip_orchestrator/notifier

pub fn main() {
  gleeunit.main()
}

// --- Lifecycle state computation tests ---------------------------------------

pub fn compute_states_created_only_test() {
  let trip =
    lifecycle.TripRow(
      id: "test-001",
      has_samples: False,
      has_ended: False,
      has_upload: False,
      has_events: False,
      has_derivations: False,
    )

  let states = lifecycle.compute_states(trip)
  should.equal(states, [lifecycle.Created])
}

pub fn compute_states_full_lifecycle_test() {
  let trip =
    lifecycle.TripRow(
      id: "test-002",
      has_samples: True,
      has_ended: True,
      has_upload: True,
      has_events: True,
      has_derivations: True,
    )

  let states = lifecycle.compute_states(trip)
  should.equal(states, [
    lifecycle.Created,
    lifecycle.Recording,
    lifecycle.Finalized,
    lifecycle.Uploaded,
    lifecycle.Parsed,
    lifecycle.Enriched,
  ])
}

pub fn compute_states_partial_test() {
  let trip =
    lifecycle.TripRow(
      id: "test-003",
      has_samples: True,
      has_ended: True,
      has_upload: False,
      has_events: False,
      has_derivations: False,
    )

  let states = lifecycle.compute_states(trip)
  should.equal(states, [
    lifecycle.Created,
    lifecycle.Recording,
    lifecycle.Finalized,
  ])
}

// --- Lifecycle state string conversion tests ---------------------------------

pub fn state_to_string_test() {
  should.equal(lifecycle.state_to_string(lifecycle.Created), "created")
  should.equal(lifecycle.state_to_string(lifecycle.Recording), "recording")
  should.equal(lifecycle.state_to_string(lifecycle.Finalized), "finalized")
  should.equal(lifecycle.state_to_string(lifecycle.Uploaded), "uploaded")
  should.equal(lifecycle.state_to_string(lifecycle.Parsed), "parsed")
  should.equal(lifecycle.state_to_string(lifecycle.Enriched), "enriched")
}

// --- Notifier severity tests -------------------------------------------------

pub fn severity_to_string_test() {
  should.equal(notifier.severity_to_string(notifier.Info), "info")
  should.equal(notifier.severity_to_string(notifier.Warning), "warning")
  should.equal(notifier.severity_to_string(notifier.Critical), "critical")
}

pub fn severity_from_string_test() {
  should.equal(notifier.severity_from_string("info"), Ok(notifier.Info))
  should.equal(notifier.severity_from_string("warning"), Ok(notifier.Warning))
  should.equal(notifier.severity_from_string("critical"), Ok(notifier.Critical))
  should.equal(notifier.severity_from_string("INFO"), Ok(notifier.Info))
  should.equal(notifier.severity_from_string("unknown"), Error(Nil))
}

pub fn meets_threshold_test() {
  // Info meets Info threshold
  should.be_true(notifier.meets_threshold(notifier.Info, notifier.Info))
  // Warning meets Info threshold
  should.be_true(notifier.meets_threshold(notifier.Warning, notifier.Info))
  // Critical meets all thresholds
  should.be_true(notifier.meets_threshold(notifier.Critical, notifier.Info))
  should.be_true(notifier.meets_threshold(notifier.Critical, notifier.Warning))
  should.be_true(notifier.meets_threshold(notifier.Critical, notifier.Critical))
  // Info does NOT meet Warning threshold
  should.be_false(notifier.meets_threshold(notifier.Info, notifier.Warning))
  // Info does NOT meet Critical threshold
  should.be_false(notifier.meets_threshold(notifier.Info, notifier.Critical))
}

pub fn category_severity_test() {
  should.equal(notifier.category_severity("storage_pressure"), notifier.Warning)
  should.equal(notifier.category_severity("upload_failure"), notifier.Warning)
  should.equal(notifier.category_severity("quality_flag"), notifier.Info)
  should.equal(
    notifier.category_severity("device_offline_24h"),
    notifier.Critical,
  )
  // Unknown category defaults to Info
  should.equal(notifier.category_severity("unknown"), notifier.Info)
}
