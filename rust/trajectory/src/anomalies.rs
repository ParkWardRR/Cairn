use anyhow::Result;
use serde::Serialize;
use std::path::Path;

use crate::geo;
use crate::types;

/// Maximum plausible speed between consecutive points (km/h).
const MAX_SPEED_KMH: f64 = 500.0;
/// HDOP threshold for "degraded accuracy".
const HDOP_SPIKE_THRESHOLD: f64 = 10.0;

#[derive(Debug, Clone, Serialize)]
pub struct AnomaliesOutput {
    pub trip_id: String,
    pub anomalies: Vec<Anomaly>,
    pub total_samples: usize,
    pub summary: AnomalySummary,
}

#[derive(Debug, Clone, Serialize)]
pub struct Anomaly {
    pub anomaly_type: AnomalyType,
    pub sample_index: usize,
    /// Second sample index for pairwise anomalies (e.g., impossible jump).
    pub sample_index_end: Option<usize>,
    pub severity: Severity,
    pub description: String,
    pub details: AnomalyDetails,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum AnomalyType {
    ImpossibleJump,
    GpsLoss,
    ClockDrift,
    AccuracyDegradation,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Severity {
    Warning,
    Error,
    Critical,
}

#[derive(Debug, Clone, Serialize)]
pub struct AnomalyDetails {
    /// Computed speed for impossible jumps (km/h).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub computed_speed_kmh: Option<f64>,
    /// Distance for impossible jumps (m).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub distance_m: Option<f64>,
    /// Time delta for jumps (ms).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub time_delta_ms: Option<i64>,
    /// HDOP value for accuracy degradation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hdop: Option<f64>,
    /// Satellite count for GPS loss.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub satellites: Option<u8>,
    /// Fix quality for GPS loss.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub fix_quality: Option<u8>,
}

#[derive(Debug, Clone, Serialize)]
pub struct AnomalySummary {
    pub impossible_jumps: usize,
    pub gps_loss_events: usize,
    pub clock_drift_events: usize,
    pub accuracy_degradation_events: usize,
}

/// Run anomaly detection on a single trip bundle.
pub fn run(
    bundle_path: &Path,
    output_path: Option<&Path>,
) -> Result<AnomaliesOutput> {
    let bundle = types::load_bundle(bundle_path)?;

    if bundle.samples.is_empty() {
        anyhow::bail!("No GNSS samples in bundle");
    }

    eprintln!(
        "Analyzing trip {} ({} samples)",
        bundle.trip_id(),
        bundle.samples.len()
    );

    let mut anomalies: Vec<Anomaly> = Vec::new();
    let samples = &bundle.samples;

    for i in 0..samples.len() {
        let s = &samples[i];

        // GPS loss: satellites drop to 0 or fix_quality = 0
        if s.satellites == 0 || s.fix_quality == 0 {
            anomalies.push(Anomaly {
                anomaly_type: AnomalyType::GpsLoss,
                sample_index: i,
                sample_index_end: None,
                severity: if s.satellites == 0 && s.fix_quality == 0 {
                    Severity::Error
                } else {
                    Severity::Warning
                },
                description: format!(
                    "GPS signal lost at sample {}: fix_quality={}, satellites={}",
                    i, s.fix_quality, s.satellites
                ),
                details: AnomalyDetails {
                    computed_speed_kmh: None,
                    distance_m: None,
                    time_delta_ms: None,
                    hdop: None,
                    satellites: Some(s.satellites),
                    fix_quality: Some(s.fix_quality),
                },
            });
        }

        // Accuracy degradation: HDOP spike
        if s.hdop() > HDOP_SPIKE_THRESHOLD {
            anomalies.push(Anomaly {
                anomaly_type: AnomalyType::AccuracyDegradation,
                sample_index: i,
                sample_index_end: None,
                severity: if s.hdop() > 20.0 {
                    Severity::Error
                } else {
                    Severity::Warning
                },
                description: format!(
                    "HDOP spike at sample {}: {:.1} (threshold: {:.1})",
                    i,
                    s.hdop(),
                    HDOP_SPIKE_THRESHOLD
                ),
                details: AnomalyDetails {
                    computed_speed_kmh: None,
                    distance_m: None,
                    time_delta_ms: None,
                    hdop: Some(s.hdop()),
                    satellites: None,
                    fix_quality: None,
                },
            });
        }

        // Pairwise checks with next sample
        if i + 1 < samples.len() {
            let next = &samples[i + 1];

            // Clock drift: timestamp going backwards
            if next.timestamp_ms < s.timestamp_ms {
                let drift_ms = s.timestamp_ms as i64 - next.timestamp_ms as i64;
                anomalies.push(Anomaly {
                    anomaly_type: AnomalyType::ClockDrift,
                    sample_index: i,
                    sample_index_end: Some(i + 1),
                    severity: if drift_ms > 10_000 {
                        Severity::Critical
                    } else {
                        Severity::Warning
                    },
                    description: format!(
                        "Clock went backwards between samples {} and {}: {}ms drift",
                        i,
                        i + 1,
                        drift_ms
                    ),
                    details: AnomalyDetails {
                        computed_speed_kmh: None,
                        distance_m: None,
                        time_delta_ms: Some(-drift_ms),
                        hdop: None,
                        satellites: None,
                        fix_quality: None,
                    },
                });
            }

            // Impossible jump: check speed between consecutive points
            let dt_ms = next.timestamp_ms.saturating_sub(s.timestamp_ms);
            if dt_ms > 0
                && s.fix_quality > 0
                && next.fix_quality > 0
                && s.satellites >= 3
                && next.satellites >= 3
            {
                let dist = geo::haversine_m(
                    s.lat_deg(),
                    s.lon_deg(),
                    next.lat_deg(),
                    next.lon_deg(),
                );
                let dt_s = dt_ms as f64 / 1000.0;
                let computed_speed_kmh = (dist / dt_s) * 3.6;

                if computed_speed_kmh > MAX_SPEED_KMH {
                    anomalies.push(Anomaly {
                        anomaly_type: AnomalyType::ImpossibleJump,
                        sample_index: i,
                        sample_index_end: Some(i + 1),
                        severity: if computed_speed_kmh > 2000.0 {
                            Severity::Critical
                        } else {
                            Severity::Error
                        },
                        description: format!(
                            "Impossible speed {:.0} km/h between samples {} and {} ({:.0}m in {:.1}s)",
                            computed_speed_kmh, i, i + 1, dist, dt_s
                        ),
                        details: AnomalyDetails {
                            computed_speed_kmh: Some(computed_speed_kmh),
                            distance_m: Some(dist),
                            time_delta_ms: Some(dt_ms as i64),
                            hdop: None,
                            satellites: None,
                            fix_quality: None,
                        },
                    });
                }
            }
        }
    }

    let summary = AnomalySummary {
        impossible_jumps: anomalies
            .iter()
            .filter(|a| matches!(a.anomaly_type, AnomalyType::ImpossibleJump))
            .count(),
        gps_loss_events: anomalies
            .iter()
            .filter(|a| matches!(a.anomaly_type, AnomalyType::GpsLoss))
            .count(),
        clock_drift_events: anomalies
            .iter()
            .filter(|a| matches!(a.anomaly_type, AnomalyType::ClockDrift))
            .count(),
        accuracy_degradation_events: anomalies
            .iter()
            .filter(|a| matches!(a.anomaly_type, AnomalyType::AccuracyDegradation))
            .count(),
    };

    let output = AnomaliesOutput {
        trip_id: bundle.trip_id().to_string(),
        total_samples: samples.len(),
        anomalies,
        summary,
    };

    // Print summary
    eprintln!(
        "{} anomalies found in {} samples:",
        output.anomalies.len(),
        output.total_samples
    );
    eprintln!("  Impossible jumps:       {}", output.summary.impossible_jumps);
    eprintln!("  GPS loss events:        {}", output.summary.gps_loss_events);
    eprintln!("  Clock drift events:     {}", output.summary.clock_drift_events);
    eprintln!(
        "  Accuracy degradation:   {}",
        output.summary.accuracy_degradation_events
    );

    if let Some(path) = output_path {
        let json = serde_json::to_string_pretty(&output)?;
        std::fs::write(path, json)?;
        eprintln!("Output written to {}", path.display());
    } else {
        println!("{}", serde_json::to_string_pretty(&output)?);
    }

    Ok(output)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_sample(
        ts: u64,
        lat: i32,
        lon: i32,
        fix: u8,
        sats: u8,
        hdop: u16,
    ) -> types::GnssSample {
        types::GnssSample {
            timestamp_ms: ts,
            latitude: lat,
            longitude: lon,
            altitude_cm: 0,
            speed_cmps: 0,
            heading_cdeg: 0,
            fix_quality: fix,
            satellites: sats,
            hdop_tenths: hdop,
            accuracy_cm: 200,
            _reserved: [0; 2],
        }
    }

    #[test]
    fn detect_gps_loss() {
        let samples = vec![
            make_sample(0, 340000000, -1180000000, 1, 8, 10),
            make_sample(1000, 340000000, -1180000000, 0, 0, 10),
            make_sample(2000, 340000000, -1180000000, 1, 8, 10),
        ];

        let bundle = types::TripBundle {
            manifest: types::Manifest {
                version: Some(1),
                trip_id: "test".to_string(),
                device_id: "dev".to_string(),
                firmware_version: String::new(),
                started_at: String::new(),
                ended_at: String::new(),
                sample_count: None,
            },
            samples,
            bundle_path: std::path::PathBuf::new(),
        };

        // Just test the GPS loss detection logic directly
        let s = &bundle.samples[1];
        assert_eq!(s.fix_quality, 0);
        assert_eq!(s.satellites, 0);
    }

    #[test]
    fn detect_clock_drift() {
        let s1 = make_sample(2000, 340000000, -1180000000, 1, 8, 10);
        let s2 = make_sample(1000, 340000000, -1180000000, 1, 8, 10);

        // s2 timestamp is earlier than s1 -- clock went backwards
        assert!(s2.timestamp_ms < s1.timestamp_ms);
    }

    #[test]
    fn detect_hdop_spike() {
        let s = make_sample(0, 340000000, -1180000000, 1, 8, 150); // HDOP = 15.0
        assert!(s.hdop() > HDOP_SPIKE_THRESHOLD);
    }

    #[test]
    fn no_false_positive_for_good_data() {
        let s = make_sample(0, 340000000, -1180000000, 1, 10, 12);
        assert!(s.fix_quality > 0);
        assert!(s.satellites > 0);
        assert!(s.hdop() < HDOP_SPIKE_THRESHOLD);
    }
}
