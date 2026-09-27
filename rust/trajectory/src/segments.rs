use anyhow::Result;
use serde::Serialize;
use std::path::Path;

use crate::geo;
use crate::types;

/// Speed threshold (km/h) below which a vehicle is considered stopped.
const STOP_SPEED_KMH: f64 = 2.0;

#[derive(Debug, Clone, Serialize)]
pub struct SegmentsOutput {
    pub trip_id: String,
    pub segments: Vec<Segment>,
    pub total_stops: usize,
    pub total_drives: usize,
    pub min_stop_s: u64,
}

#[derive(Debug, Clone, Serialize)]
pub struct Segment {
    pub segment_id: usize,
    pub segment_type: SegmentType,
    pub start_time_ms: u64,
    pub end_time_ms: u64,
    pub duration_s: f64,
    pub start_lat: f64,
    pub start_lon: f64,
    pub end_lat: f64,
    pub end_lon: f64,
    /// Distance in metres (only meaningful for drive segments).
    pub distance_m: Option<f64>,
    /// Average speed in km/h (only for drive segments).
    pub avg_speed_kmh: Option<f64>,
    pub sample_count: usize,
}

#[derive(Debug, Clone, Serialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum SegmentType {
    Drive,
    Stop,
}

/// Detect whether a sample represents a stopped state.
fn is_stopped(sample: &types::GnssSample) -> bool {
    sample.speed_kmh() < STOP_SPEED_KMH
}

/// Build a segment from a range of samples.
fn build_segment(
    samples: &[types::GnssSample],
    seg_type: SegmentType,
    seg_id: usize,
) -> Segment {
    let first = &samples[0];
    let last = &samples[samples.len() - 1];

    let duration_s = (last.timestamp_ms as f64 - first.timestamp_ms as f64) / 1000.0;

    let (distance_m, avg_speed_kmh) = if seg_type == SegmentType::Drive {
        let mut total_dist = 0.0;
        for window in samples.windows(2) {
            total_dist += geo::haversine_m(
                window[0].lat_deg(),
                window[0].lon_deg(),
                window[1].lat_deg(),
                window[1].lon_deg(),
            );
        }
        let avg_spd = if duration_s > 0.0 {
            (total_dist / duration_s) * 3.6
        } else {
            0.0
        };
        (Some(total_dist), Some(avg_spd))
    } else {
        (None, None)
    };

    Segment {
        segment_id: seg_id,
        segment_type: seg_type,
        start_time_ms: first.timestamp_ms,
        end_time_ms: last.timestamp_ms,
        duration_s,
        start_lat: first.lat_deg(),
        start_lon: first.lon_deg(),
        end_lat: last.lat_deg(),
        end_lon: last.lon_deg(),
        distance_m,
        avg_speed_kmh,
        sample_count: samples.len(),
    }
}

/// Run stop/errand segmentation on a single trip bundle.
pub fn run(
    bundle_path: &Path,
    min_stop_s: u64,
    output_path: Option<&Path>,
) -> Result<SegmentsOutput> {
    let bundle = types::load_bundle(bundle_path)?;

    if bundle.samples.is_empty() {
        anyhow::bail!("No GNSS samples in bundle");
    }

    eprintln!(
        "Analyzing trip {} ({} samples)",
        bundle.trip_id(),
        bundle.samples.len()
    );

    // Filter to good-fix samples
    let good_samples: Vec<&types::GnssSample> = bundle
        .samples
        .iter()
        .filter(|s| s.fix_quality > 0)
        .collect();

    if good_samples.is_empty() {
        anyhow::bail!("No samples with valid fix quality");
    }

    // Phase 1: label each sample as stopped or moving
    let labels: Vec<bool> = good_samples.iter().map(|s| is_stopped(s)).collect();

    // Phase 2: group consecutive samples with the same label into raw segments
    let mut raw_segments: Vec<(bool, usize, usize)> = Vec::new(); // (is_stopped, start_idx, end_idx)
    let mut seg_start = 0;
    let mut current_stopped = labels[0];

    for i in 1..labels.len() {
        if labels[i] != current_stopped {
            raw_segments.push((current_stopped, seg_start, i - 1));
            seg_start = i;
            current_stopped = labels[i];
        }
    }
    raw_segments.push((current_stopped, seg_start, labels.len() - 1));

    // Phase 3: filter out stops shorter than min_stop_s (merge them into drives)
    let min_stop_ms = min_stop_s * 1000;
    let mut merged_segments: Vec<(bool, usize, usize)> = Vec::new();

    for (is_stopped, start, end) in &raw_segments {
        let start_ms = good_samples[*start].timestamp_ms;
        let end_ms = good_samples[*end].timestamp_ms;
        let duration_ms = end_ms.saturating_sub(start_ms);

        if *is_stopped && duration_ms < min_stop_ms {
            // Too short to be a real stop -- treat as drive
            if let Some(last) = merged_segments.last_mut() {
                if !last.0 {
                    // Previous was also drive, extend it
                    last.2 = *end;
                    continue;
                }
            }
            merged_segments.push((false, *start, *end));
        } else {
            // Check if we can merge with previous segment of same type
            if let Some(last) = merged_segments.last_mut() {
                if last.0 == *is_stopped {
                    last.2 = *end;
                    continue;
                }
            }
            merged_segments.push((*is_stopped, *start, *end));
        }
    }

    // Phase 4: build final segments
    let mut segments: Vec<Segment> = Vec::new();
    let mut seg_id = 0;

    for (is_stopped, start, end) in &merged_segments {
        let seg_samples: Vec<types::GnssSample> = ((*start)..=(*end))
            .map(|i| *good_samples[i])
            .collect();

        if seg_samples.is_empty() {
            continue;
        }

        let seg_type = if *is_stopped {
            SegmentType::Stop
        } else {
            SegmentType::Drive
        };

        segments.push(build_segment(&seg_samples, seg_type, seg_id));
        seg_id += 1;
    }

    let total_stops = segments
        .iter()
        .filter(|s| s.segment_type == SegmentType::Stop)
        .count();
    let total_drives = segments
        .iter()
        .filter(|s| s.segment_type == SegmentType::Drive)
        .count();

    let output = SegmentsOutput {
        trip_id: bundle.trip_id().to_string(),
        segments,
        total_stops,
        total_drives,
        min_stop_s,
    };

    // Print summary
    eprintln!(
        "{} segments: {} drives, {} stops (min stop: {}s)",
        output.segments.len(),
        total_drives,
        total_stops,
        min_stop_s
    );
    for seg in &output.segments {
        let type_str = match seg.segment_type {
            SegmentType::Drive => "DRIVE",
            SegmentType::Stop => "STOP ",
        };
        let extra = match seg.segment_type {
            SegmentType::Drive => format!(
                "{:.0}m, {:.1} km/h avg",
                seg.distance_m.unwrap_or(0.0),
                seg.avg_speed_kmh.unwrap_or(0.0)
            ),
            SegmentType::Stop => format!(
                "at ({:.6}, {:.6})",
                seg.start_lat, seg.start_lon
            ),
        };
        eprintln!(
            "  {} {}: {:.0}s - {}",
            type_str, seg.segment_id, seg.duration_s, extra
        );
    }

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

    fn make_sample(ts: u64, speed_cmps: u16, lat: i32, lon: i32) -> types::GnssSample {
        types::GnssSample {
            timestamp_ms: ts,
            latitude: lat,
            longitude: lon,
            altitude_cm: 0,
            speed_cmps,
            heading_cdeg: 0,
            fix_quality: 1,
            satellites: 8,
            hdop_tenths: 10,
            accuracy_cm: 200,
            _reserved: [0; 2],
        }
    }

    #[test]
    fn stop_detection() {
        // 2 km/h threshold
        let stopped = make_sample(0, 50, 0, 0); // 0.5 m/s = 1.8 km/h
        let moving = make_sample(0, 1000, 0, 0); // 10 m/s = 36 km/h
        assert!(is_stopped(&stopped));
        assert!(!is_stopped(&moving));
    }

    #[test]
    fn build_drive_segment() {
        let samples = vec![
            make_sample(0, 1000, 340000000, -1180000000),
            make_sample(1000, 1000, 340001000, -1180001000),
            make_sample(2000, 1000, 340002000, -1180002000),
        ];
        let seg = build_segment(&samples, SegmentType::Drive, 0);
        assert_eq!(seg.segment_type, SegmentType::Drive);
        assert!((seg.duration_s - 2.0).abs() < 0.01);
        assert!(seg.distance_m.is_some());
        assert!(seg.distance_m.unwrap() > 0.0);
    }

    #[test]
    fn build_stop_segment() {
        let samples = vec![
            make_sample(0, 0, 340000000, -1180000000),
            make_sample(60000, 0, 340000000, -1180000000),
            make_sample(120000, 0, 340000000, -1180000000),
        ];
        let seg = build_segment(&samples, SegmentType::Stop, 0);
        assert_eq!(seg.segment_type, SegmentType::Stop);
        assert!((seg.duration_s - 120.0).abs() < 0.01);
        assert!(seg.distance_m.is_none());
    }
}
