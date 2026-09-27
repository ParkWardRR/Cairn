use anyhow::Result;
use serde::Serialize;
use std::path::Path;

use crate::types;

/// Number of bins for histograms.
const HISTOGRAM_BINS: usize = 20;

#[derive(Debug, Clone, Serialize)]
pub struct DensityOutput {
    pub total_trips: usize,
    pub total_samples: usize,
    pub acceleration: AccelerationProfile,
    pub turning: TurningProfile,
    pub style_segments: StyleSegments,
}

#[derive(Debug, Clone, Serialize)]
pub struct AccelerationProfile {
    /// Histogram of acceleration values (m/s^2). Bins cover [-10, 10] m/s^2.
    pub histogram: Vec<HistogramBin>,
    pub percentiles: Percentiles,
    pub max_accel_mps2: f64,
    pub max_decel_mps2: f64,
}

#[derive(Debug, Clone, Serialize)]
pub struct TurningProfile {
    /// Histogram of turn rates (deg/s). Bins cover [0, 90] deg/s.
    pub histogram: Vec<HistogramBin>,
    pub percentiles: Percentiles,
    pub max_turn_rate_dps: f64,
}

#[derive(Debug, Clone, Serialize)]
pub struct HistogramBin {
    pub bin_min: f64,
    pub bin_max: f64,
    pub count: usize,
}

#[derive(Debug, Clone, Serialize)]
pub struct Percentiles {
    pub p50: f64,
    pub p75: f64,
    pub p90: f64,
    pub p95: f64,
    pub p99: f64,
}

#[derive(Debug, Clone, Serialize)]
pub struct StyleSegments {
    pub gentle_pct: f64,
    pub moderate_pct: f64,
    pub aggressive_pct: f64,
    pub total_drive_samples: usize,
}

/// Acceleration thresholds (m/s^2) for style classification.
const GENTLE_ACCEL_THRESHOLD: f64 = 1.5;
const AGGRESSIVE_ACCEL_THRESHOLD: f64 = 3.0;

/// Turn rate thresholds (deg/s) for style classification.
const GENTLE_TURN_THRESHOLD: f64 = 10.0;
const AGGRESSIVE_TURN_THRESHOLD: f64 = 30.0;

/// Compute acceleration between consecutive speed samples (m/s^2).
fn compute_accelerations(samples: &[types::GnssSample]) -> Vec<f64> {
    let mut accels = Vec::new();

    for window in samples.windows(2) {
        let dt_s = (window[1].timestamp_ms as f64 - window[0].timestamp_ms as f64) / 1000.0;
        if dt_s <= 0.0 || dt_s > 10.0 {
            continue;
        }

        // Both must have valid fix
        if window[0].fix_quality == 0 || window[1].fix_quality == 0 {
            continue;
        }

        let v1 = window[0].speed_mps();
        let v2 = window[1].speed_mps();
        let accel = (v2 - v1) / dt_s;

        accels.push(accel);
    }

    accels
}

/// Compute turn rate between consecutive heading samples (deg/s).
fn compute_turn_rates(samples: &[types::GnssSample]) -> Vec<f64> {
    let mut rates = Vec::new();

    for window in samples.windows(2) {
        let dt_s = (window[1].timestamp_ms as f64 - window[0].timestamp_ms as f64) / 1000.0;
        if dt_s <= 0.0 || dt_s > 10.0 {
            continue;
        }

        if window[0].fix_quality == 0 || window[1].fix_quality == 0 {
            continue;
        }

        // Only compute turn rate when actually moving
        if window[0].speed_cmps < 200 || window[1].speed_cmps < 200 {
            continue;
        }

        let h1 = window[0].heading_deg();
        let h2 = window[1].heading_deg();

        // Shortest angular difference
        let mut delta = h2 - h1;
        if delta > 180.0 {
            delta -= 360.0;
        }
        if delta < -180.0 {
            delta += 360.0;
        }

        let turn_rate = delta.abs() / dt_s;
        rates.push(turn_rate);
    }

    rates
}

/// Build a histogram from values.
fn build_histogram(values: &[f64], min_val: f64, max_val: f64, bins: usize) -> Vec<HistogramBin> {
    let bin_width = (max_val - min_val) / bins as f64;
    let mut histogram: Vec<HistogramBin> = (0..bins)
        .map(|i| {
            let lo = min_val + i as f64 * bin_width;
            HistogramBin {
                bin_min: lo,
                bin_max: lo + bin_width,
                count: 0,
            }
        })
        .collect();

    for &v in values {
        let idx = ((v - min_val) / bin_width).floor() as i64;
        let idx = idx.max(0).min(bins as i64 - 1) as usize;
        histogram[idx].count += 1;
    }

    histogram
}

/// Compute percentiles from a sorted slice.
fn compute_percentiles(sorted: &[f64]) -> Percentiles {
    if sorted.is_empty() {
        return Percentiles {
            p50: 0.0,
            p75: 0.0,
            p90: 0.0,
            p95: 0.0,
            p99: 0.0,
        };
    }

    let pct = |p: f64| -> f64 {
        let idx = (p / 100.0 * (sorted.len() - 1) as f64).round() as usize;
        sorted[idx.min(sorted.len() - 1)]
    };

    Percentiles {
        p50: pct(50.0),
        p75: pct(75.0),
        p90: pct(90.0),
        p95: pct(95.0),
        p99: pct(99.0),
    }
}

/// Classify a sample pair as gentle/moderate/aggressive based on accel and turn rate.
fn classify_style(accel_abs: f64, turn_rate: f64) -> &'static str {
    if accel_abs > AGGRESSIVE_ACCEL_THRESHOLD || turn_rate > AGGRESSIVE_TURN_THRESHOLD {
        "aggressive"
    } else if accel_abs > GENTLE_ACCEL_THRESHOLD || turn_rate > GENTLE_TURN_THRESHOLD {
        "moderate"
    } else {
        "gentle"
    }
}

/// Run driving style density analysis over all bundles.
pub fn run(
    bundles_dir: &Path,
    output_path: Option<&Path>,
) -> Result<DensityOutput> {
    let bundles = types::load_bundles(bundles_dir)?;

    if bundles.is_empty() {
        anyhow::bail!("No trip bundles found in {}", bundles_dir.display());
    }

    eprintln!("Loaded {} trip bundles", bundles.len());

    // Aggregate data from all trips
    let mut all_accels: Vec<f64> = Vec::new();
    let mut all_turn_rates: Vec<f64> = Vec::new();
    let mut total_samples = 0;

    for bundle in &bundles {
        total_samples += bundle.samples.len();
        all_accels.extend(compute_accelerations(&bundle.samples));
        all_turn_rates.extend(compute_turn_rates(&bundle.samples));
    }

    eprintln!(
        "Computed {} acceleration values, {} turn rates from {} total samples",
        all_accels.len(),
        all_turn_rates.len(),
        total_samples
    );

    // Build acceleration profile
    let mut sorted_accels = all_accels.clone();
    sorted_accels.sort_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal));

    let max_accel = sorted_accels
        .iter()
        .copied()
        .fold(f64::NEG_INFINITY, f64::max);
    let max_decel = sorted_accels
        .iter()
        .copied()
        .fold(f64::INFINITY, f64::min);

    let accel_histogram = build_histogram(&all_accels, -10.0, 10.0, HISTOGRAM_BINS);
    let accel_abs: Vec<f64> = sorted_accels.iter().map(|v| v.abs()).collect();
    let mut sorted_accel_abs = accel_abs;
    sorted_accel_abs.sort_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal));
    let accel_percentiles = compute_percentiles(&sorted_accel_abs);

    // Build turning profile
    let mut sorted_turns = all_turn_rates.clone();
    sorted_turns.sort_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal));

    let max_turn = sorted_turns
        .iter()
        .copied()
        .fold(0.0_f64, f64::max);

    let turn_histogram = build_histogram(&all_turn_rates, 0.0, 90.0, HISTOGRAM_BINS);
    let turn_percentiles = compute_percentiles(&sorted_turns);

    // Style classification
    let drive_count = all_accels.len().max(all_turn_rates.len());
    let mut gentle = 0usize;
    let mut moderate = 0usize;
    let mut aggressive = 0usize;

    // Classify using whichever data we have
    for i in 0..drive_count {
        let accel = if i < all_accels.len() {
            all_accels[i].abs()
        } else {
            0.0
        };
        let turn = if i < all_turn_rates.len() {
            all_turn_rates[i]
        } else {
            0.0
        };

        match classify_style(accel, turn) {
            "gentle" => gentle += 1,
            "moderate" => moderate += 1,
            "aggressive" => aggressive += 1,
            _ => {}
        }
    }

    let total_classified = gentle + moderate + aggressive;
    let style_segments = StyleSegments {
        gentle_pct: if total_classified > 0 {
            gentle as f64 / total_classified as f64 * 100.0
        } else {
            0.0
        },
        moderate_pct: if total_classified > 0 {
            moderate as f64 / total_classified as f64 * 100.0
        } else {
            0.0
        },
        aggressive_pct: if total_classified > 0 {
            aggressive as f64 / total_classified as f64 * 100.0
        } else {
            0.0
        },
        total_drive_samples: total_classified,
    };

    let output = DensityOutput {
        total_trips: bundles.len(),
        total_samples,
        acceleration: AccelerationProfile {
            histogram: accel_histogram,
            percentiles: accel_percentiles,
            max_accel_mps2: if max_accel.is_finite() { max_accel } else { 0.0 },
            max_decel_mps2: if max_decel.is_finite() { max_decel } else { 0.0 },
        },
        turning: TurningProfile {
            histogram: turn_histogram,
            percentiles: turn_percentiles,
            max_turn_rate_dps: if max_turn.is_finite() { max_turn } else { 0.0 },
        },
        style_segments,
    };

    // Print summary
    eprintln!("Driving style summary:");
    eprintln!(
        "  Gentle:     {:.1}%",
        output.style_segments.gentle_pct
    );
    eprintln!(
        "  Moderate:   {:.1}%",
        output.style_segments.moderate_pct
    );
    eprintln!(
        "  Aggressive: {:.1}%",
        output.style_segments.aggressive_pct
    );
    eprintln!(
        "  Max accel:  {:.2} m/s^2, Max decel: {:.2} m/s^2",
        output.acceleration.max_accel_mps2, output.acceleration.max_decel_mps2
    );
    eprintln!(
        "  Max turn:   {:.1} deg/s",
        output.turning.max_turn_rate_dps
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

    fn make_sample(ts: u64, speed_cmps: u16, heading_cdeg: u16) -> types::GnssSample {
        types::GnssSample {
            timestamp_ms: ts,
            latitude: 340000000,
            longitude: -1180000000,
            altitude_cm: 0,
            speed_cmps,
            heading_cdeg,
            fix_quality: 1,
            satellites: 8,
            hdop_tenths: 10,
            accuracy_cm: 200,
            _reserved: [0; 2],
        }
    }

    #[test]
    fn acceleration_computation() {
        // 0 m/s -> 10 m/s in 1 second = 10 m/s^2
        let samples = vec![
            make_sample(0, 0, 0),
            make_sample(1000, 1000, 0), // 10 m/s
        ];
        let accels = compute_accelerations(&samples);
        assert_eq!(accels.len(), 1);
        assert!((accels[0] - 10.0).abs() < 0.01);
    }

    #[test]
    fn deceleration_computation() {
        // 10 m/s -> 0 m/s in 2 seconds = -5 m/s^2
        let samples = vec![
            make_sample(0, 1000, 0), // 10 m/s
            make_sample(2000, 0, 0),
        ];
        let accels = compute_accelerations(&samples);
        assert_eq!(accels.len(), 1);
        assert!((accels[0] - (-5.0)).abs() < 0.01);
    }

    #[test]
    fn turn_rate_computation() {
        // Heading change of 90 degrees in 1 second while moving
        let samples = vec![
            make_sample(0, 1000, 0),      // heading 0
            make_sample(1000, 1000, 9000), // heading 90
        ];
        let rates = compute_turn_rates(&samples);
        assert_eq!(rates.len(), 1);
        assert!((rates[0] - 90.0).abs() < 0.01);
    }

    #[test]
    fn turn_rate_wraps_around() {
        // 350 -> 10 degrees = 20 degrees change, not 340
        let samples = vec![
            make_sample(0, 1000, 35000),  // 350 degrees
            make_sample(1000, 1000, 1000), // 10 degrees
        ];
        let rates = compute_turn_rates(&samples);
        assert_eq!(rates.len(), 1);
        assert!((rates[0] - 20.0).abs() < 0.01);
    }

    #[test]
    fn style_classification() {
        assert_eq!(classify_style(0.5, 5.0), "gentle");
        assert_eq!(classify_style(2.0, 5.0), "moderate");
        assert_eq!(classify_style(4.0, 5.0), "aggressive");
        assert_eq!(classify_style(0.5, 35.0), "aggressive");
    }

    #[test]
    fn histogram_basic() {
        let values = vec![0.0, 1.0, 2.0, 3.0, 4.0];
        let hist = build_histogram(&values, 0.0, 5.0, 5);
        assert_eq!(hist.len(), 5);
        for bin in &hist {
            assert_eq!(bin.count, 1);
        }
    }

    #[test]
    fn percentiles_basic() {
        let sorted: Vec<f64> = (0..100).map(|i| i as f64).collect();
        let p = compute_percentiles(&sorted);
        assert!((p.p50 - 50.0).abs() < 1.0);
        assert!((p.p90 - 89.0).abs() < 2.0);
        assert!((p.p99 - 98.0).abs() < 2.0);
    }
}
