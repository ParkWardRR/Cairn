/// Automation executor.
///
/// Applies configurable rules stored in the automation_rules table.
/// Rule types:
///   - on_trip_end: tag assignment when a trip finishes
///   - on_arrive_place: notify when arriving at a place
///   - on_depart_place: notify when departing from a place
///
/// Rules are stored with a JSON definition containing conditions and actions.
/// The executor evaluates rules against new trip events and executes the
/// corresponding actions (writing tags, creating notifications).

import gleam/dynamic/decode
import gleam/int
import gleam/io
import gleam/list
import gleam/option.{type Option, Some}
import gleam/string
import pog
import trip_orchestrator/notifier

/// A rule loaded from the database.
pub type Rule {
  Rule(
    id: String,
    name: String,
    trigger_type: String,
    condition_json: String,
    action_json: String,
  )
}

/// A trip event to evaluate against rules.
pub type TripEvent {
  TripEvent(
    trip_id: String,
    event_type: String,
    metadata_json: Option(String),
  )
}

/// Decoder for automation rules.
fn rule_decoder() -> decode.Decoder(Rule) {
  use id <- decode.field(0, decode.string)
  use name <- decode.field(1, decode.string)
  use trigger_type <- decode.field(2, decode.string)
  use condition <- decode.field(3, decode.string)
  use action <- decode.field(4, decode.string)
  decode.success(Rule(
    id: id,
    name: name,
    trigger_type: trigger_type,
    condition_json: condition,
    action_json: action,
  ))
}

/// Decoder for trip events.
fn event_decoder() -> decode.Decoder(TripEvent) {
  use trip_id <- decode.field(0, decode.string)
  use event_type <- decode.field(1, decode.string)
  use metadata <- decode.field(2, decode.optional(decode.string))
  decode.success(TripEvent(
    trip_id: trip_id,
    event_type: event_type,
    metadata_json: metadata,
  ))
}

/// Load all enabled automation rules from the database.
pub fn load_rules(conn: pog.Connection) -> Result(List(Rule), String) {
  let query =
    pog.query(
      "
      SELECT id::text, name, trigger_type, condition::text, action::text
      FROM automation_rules
      WHERE enabled = true
      ORDER BY created_at ASC
      ",
    )
    |> pog.returning(rule_decoder())

  case pog.execute(query: query, on: conn) {
    Ok(response) -> Ok(response.rows)
    Error(_) -> Error("Failed to load automation rules")
  }
}

/// Fetch recent trip events that haven't been processed by automation yet.
/// Uses a simple approach: events from the last processing window.
pub fn fetch_unprocessed_events(
  conn: pog.Connection,
) -> Result(List(TripEvent), String) {
  let query =
    pog.query(
      "
      SELECT te.trip_id, te.event_type, te.metadata::text
      FROM trip_events te
      WHERE te.timestamp_at > now() - INTERVAL '5 minutes'
      ORDER BY te.timestamp_at ASC
      ",
    )
    |> pog.returning(event_decoder())

  case pog.execute(query: query, on: conn) {
    Ok(response) -> Ok(response.rows)
    Error(_) -> Error("Failed to fetch unprocessed events")
  }
}

/// Map event types to rule trigger types.
fn event_matches_trigger(event: TripEvent, rule: Rule) -> Bool {
  case rule.trigger_type {
    "on_trip_end" -> event.event_type == "trip_end"
    "on_arrive_place" -> event.event_type == "place_arrive"
    "on_depart_place" -> event.event_type == "place_depart"
    _ -> False
  }
}

/// Execute a single rule's action against a trip event.
fn execute_action(
  conn: pog.Connection,
  rule: Rule,
  event: TripEvent,
) -> Result(Nil, String) {
  // Determine action type from the action JSON.
  case string.contains(rule.action_json, "\"tag\"") {
    True -> execute_tag_action(conn, rule, event)
    False ->
      case string.contains(rule.action_json, "\"notify\"") {
        True -> execute_notify_action(conn, rule, event)
        False -> {
          io.println(
            "Automation: unknown action type in rule " <> rule.name,
          )
          Ok(Nil)
        }
      }
  }
}

/// Execute a tag assignment action.
fn execute_tag_action(
  conn: pog.Connection,
  rule: Rule,
  event: TripEvent,
) -> Result(Nil, String) {
  let tag = extract_json_value(rule.action_json, "value")
  case tag {
    "" -> {
      io.println("Automation: no tag value found in rule " <> rule.name)
      Ok(Nil)
    }
    tag_value -> {
      let query =
        pog.query(
          "
          INSERT INTO trip_tags (trip_id, tag)
          VALUES ($1, $2)
          ON CONFLICT (trip_id, tag) DO NOTHING
          ",
        )
        |> pog.parameter(pog.text(event.trip_id))
        |> pog.parameter(pog.text(tag_value))

      case pog.execute(query: query, on: conn) {
        Ok(_) -> {
          io.println(
            "Automation: tagged trip "
            <> event.trip_id
            <> " with '"
            <> tag_value
            <> "' (rule: "
            <> rule.name
            <> ")",
          )
          Ok(Nil)
        }
        Error(_) -> Error("Failed to write tag for trip " <> event.trip_id)
      }
    }
  }
}

/// Execute a notification action.
fn execute_notify_action(
  conn: pog.Connection,
  rule: Rule,
  event: TripEvent,
) -> Result(Nil, String) {
  let title = extract_json_value(rule.action_json, "title")
  let title = case title {
    "" -> "Automation: " <> rule.name
    t -> t
  }

  let notification =
    notifier.Notification(
      severity: notifier.Info,
      category: "automation",
      title: title,
      body: Some(
        "Triggered by "
        <> event.event_type
        <> " on trip "
        <> event.trip_id,
      ),
      metadata: Some(
        "{\"rule_id\": \""
        <> rule.id
        <> "\", \"trip_id\": \""
        <> event.trip_id
        <> "\"}",
      ),
    )

  let config = notifier.default_config()
  case notifier.emit(conn, config, notification) {
    Ok(_) -> {
      io.println(
        "Automation: notification sent for rule "
        <> rule.name
        <> " on trip "
        <> event.trip_id,
      )
      Ok(Nil)
    }
    Error(msg) -> Error(msg)
  }
}

/// Very simple JSON value extractor. Looks for "key": "value" patterns.
/// For production use, this should use gleam_json's decoder.
fn extract_json_value(json: String, key: String) -> String {
  let pattern = "\"" <> key <> "\""
  case string.split(json, pattern) {
    [_, rest, ..] -> {
      let rest = string.trim_start(rest)
      case string.starts_with(rest, ":") {
        True -> {
          let rest = string.drop_start(rest, 1)
          let rest = string.trim_start(rest)
          case string.starts_with(rest, "\"") {
            True -> {
              let rest = string.drop_start(rest, 1)
              case string.split(rest, "\"") {
                [value, ..] -> value
                _ -> ""
              }
            }
            False -> ""
          }
        }
        False -> ""
      }
    }
    _ -> ""
  }
}

/// Run the automation executor: load rules, fetch events, match and execute.
/// Returns the number of actions executed.
pub fn run(conn: pog.Connection) -> Result(Int, String) {
  case load_rules(conn) {
    Ok(rules) ->
      case fetch_unprocessed_events(conn) {
        Ok(events) -> {
          let executed =
            list.fold(events, 0, fn(acc, event) {
              let matching_rules =
                list.filter(rules, fn(rule) {
                  event_matches_trigger(event, rule)
                })
              list.fold(matching_rules, acc, fn(inner_acc, rule) {
                case execute_action(conn, rule, event) {
                  Ok(Nil) -> inner_acc + 1
                  Error(msg) -> {
                    io.println(
                      "Automation: action failed for rule "
                      <> rule.name
                      <> ": "
                      <> msg,
                    )
                    inner_acc
                  }
                }
              })
            })

          io.println(
            "Automation: executed "
            <> int.to_string(executed)
            <> " actions from "
            <> int.to_string(list.length(rules))
            <> " rules against "
            <> int.to_string(list.length(events))
            <> " events",
          )
          Ok(executed)
        }
        Error(msg) -> Error(msg)
      }
    Error(msg) -> Error(msg)
  }
}
