/// Notification policy engine.
///
/// Evaluates whether issues deserve notification based on severity rules.
///
/// Severity levels: info, warning, critical
/// Default rules:
///   storage_pressure  -> warning
///   upload_failure    -> warning
///   quality_flag      -> info
///   device_offline_24h -> critical
///
/// Notifications are written to the notifications table. The severity
/// threshold is configurable: only notifications at or above the threshold
/// are written.

import gleam/dynamic/decode
import gleam/int
import gleam/io
import gleam/list
import gleam/option.{type Option, None, Some}
import gleam/string
import pog

/// Severity levels ordered by increasing importance.
pub type Severity {
  Info
  Warning
  Critical
}

/// Convert a severity to its string representation.
pub fn severity_to_string(severity: Severity) -> String {
  case severity {
    Info -> "info"
    Warning -> "warning"
    Critical -> "critical"
  }
}

/// Parse a severity from a string.
pub fn severity_from_string(s: String) -> Result(Severity, Nil) {
  case string.lowercase(s) {
    "info" -> Ok(Info)
    "warning" -> Ok(Warning)
    "critical" -> Ok(Critical)
    _ -> Error(Nil)
  }
}

/// Numeric value for severity comparison.
fn severity_level(severity: Severity) -> Int {
  case severity {
    Info -> 0
    Warning -> 1
    Critical -> 2
  }
}

/// Check if a severity meets or exceeds a threshold.
pub fn meets_threshold(severity: Severity, threshold: Severity) -> Bool {
  severity_level(severity) >= severity_level(threshold)
}

/// Configuration for the notifier.
pub type NotifierConfig {
  NotifierConfig(min_severity: Severity)
}

/// Default configuration: notify on all severities.
pub fn default_config() -> NotifierConfig {
  NotifierConfig(min_severity: Info)
}

/// A notification to emit.
pub type Notification {
  Notification(
    severity: Severity,
    category: String,
    title: String,
    body: Option(String),
    metadata: Option(String),
  )
}

/// Determine the severity for a known category.
pub fn category_severity(category: String) -> Severity {
  case category {
    "storage_pressure" -> Warning
    "upload_failure" -> Warning
    "quality_flag" -> Info
    "device_offline_24h" -> Critical
    _ -> Info
  }
}

/// Create a notification from a category and message, applying the default
/// severity rule.
pub fn from_category(
  category: String,
  title: String,
  body: Option(String),
  metadata: Option(String),
) -> Notification {
  Notification(
    severity: category_severity(category),
    category: category,
    title: title,
    body: body,
    metadata: metadata,
  )
}

/// Emit a notification if it meets the severity threshold.
/// Returns True if the notification was written, False if suppressed.
pub fn emit(
  conn: pog.Connection,
  config: NotifierConfig,
  notification: Notification,
) -> Result(Bool, String) {
  case meets_threshold(notification.severity, config.min_severity) {
    False -> Ok(False)
    True -> {
      let body_val = case notification.body {
        Some(b) -> pog.text(b)
        None -> pog.null()
      }
      let meta_val = case notification.metadata {
        Some(m) -> pog.text(m)
        None -> pog.null()
      }

      let query =
        pog.query(
          "
          INSERT INTO notifications (severity, category, title, body, metadata)
          VALUES ($1, $2, $3, $4, $5::jsonb)
          ",
        )
        |> pog.parameter(pog.text(severity_to_string(notification.severity)))
        |> pog.parameter(pog.text(notification.category))
        |> pog.parameter(pog.text(notification.title))
        |> pog.parameter(body_val)
        |> pog.parameter(meta_val)

      case pog.execute(query: query, on: conn) {
        Ok(_) -> Ok(True)
        Error(_) -> Error("Failed to write notification")
      }
    }
  }
}

/// Evaluate quality flags and emit notifications for any new flags found.
/// Returns the number of notifications emitted.
pub fn notify_quality_flags(
  conn: pog.Connection,
  config: NotifierConfig,
) -> Result(Int, String) {
  let row_decoder = {
    use trip_id <- decode.field(0, decode.string)
    use flag_type <- decode.field(1, decode.string)
    use _severity <- decode.field(2, decode.string)
    use _details <- decode.field(3, decode.string)
    decode.success(#(trip_id, flag_type))
  }

  let query =
    pog.query(
      "
      SELECT qf.trip_id, qf.flag_type, qf.severity, COALESCE(qf.details::text, '{}')
      FROM trip_quality_flags qf
      WHERE qf.resolved_at IS NULL
        AND NOT EXISTS (
          SELECT 1 FROM notifications n
          WHERE n.category = 'quality_flag'
            AND n.metadata::jsonb->>'trip_id' = qf.trip_id
            AND n.metadata::jsonb->>'flag_type' = qf.flag_type
        )
      ",
    )
    |> pog.returning(row_decoder)

  case pog.execute(query: query, on: conn) {
    Ok(response) -> {
      let emitted =
        list.fold(response.rows, 0, fn(acc, row) {
          let #(trip_id, flag_type) = row
          let notification =
            from_category(
              "quality_flag",
              "Quality issue: " <> flag_type <> " on trip " <> trip_id,
              Some("Quality flag detected for trip " <> trip_id),
              Some(
                "{\"trip_id\": \""
                <> trip_id
                <> "\", \"flag_type\": \""
                <> flag_type
                <> "\"}",
              ),
            )
          case emit(conn, config, notification) {
            Ok(True) -> acc + 1
            _ -> acc
          }
        })

      io.println(
        "Notifier: emitted "
        <> int.to_string(emitted)
        <> " quality flag notifications",
      )
      Ok(emitted)
    }
    Error(_) -> Error("Failed to query quality flags for notification")
  }
}
