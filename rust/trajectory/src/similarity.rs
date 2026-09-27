use anyhow::Result;
use serde::Serialize;
use std::path::Path;

use crate::geo;
use crate::types;

/// Maximum number of points to use for pairwise comparison (for performance).
const MAX_COMPARE_POINTS: usize = 200;

#[derive(Debug, Clone, Serialize)]
pub struct SimilarityOutput {
    pub clusters: Vec<Cluster>,
    pub total_trips: usize,
    pub threshold: f64,
}

#[derive(Debug, Clone, Serialize)]
pub struct Cluster {
    pub cluster_id: usize,
    pub trip_ids: Vec<String>,
    pub member_count: usize,
    pub representative_trip_id: String,
    pub similarity_scores: Vec<PairScore>,
}

#[derive(Debug, Clone, Serialize)]
pub struct PairScore {
    pub trip_a: String,
    pub trip_b: String,
    pub hausdorff_m: f64,
    pub similarity: f64,
}

/// Extract a polyline from GNSS samples, subsampled for comparison.
fn extract_route(samples: &[types::GnssSample]) -> Vec<(f64, f64)> {
    let full: Vec<(f64, f64)> = samples
        .iter()
        .filter(|s| s.fix_quality > 0 && s.satellites >= 3)
        .map(|s| (s.lat_deg(), s.lon_deg()))
        .collect();
    geo::subsample(&full, MAX_COMPARE_POINTS)
}

/// Compute route similarity as a value in [0, 1].
/// threshold_m is the distance at which similarity drops to 0.
fn route_similarity(hausdorff_m: f64, threshold_m: f64) -> f64 {
    if threshold_m <= 0.0 {
        return 0.0;
    }
    (1.0 - hausdorff_m / threshold_m).max(0.0)
}

/// Run route similarity clustering over all bundles in a directory.
pub fn run(
    bundles_dir: &Path,
    threshold: f64,
    output_path: Option<&Path>,
) -> Result<SimilarityOutput> {
    let bundles = types::load_bundles(bundles_dir)?;

    if bundles.is_empty() {
        anyhow::bail!("No trip bundles found in {}", bundles_dir.display());
    }

    eprintln!("Loaded {} trip bundles", bundles.len());

    // Extract routes
    let routes: Vec<(String, Vec<(f64, f64)>)> = bundles
        .iter()
        .map(|b| (b.trip_id().to_string(), extract_route(&b.samples)))
        .collect();

    // The threshold_m maps [0, threshold_m] -> similarity [1, 0].
    // Default threshold=0.85 means "similarity >= 0.85".
    // We use 500m as the distance scale: routes within 500m * (1-threshold) = 75m
    // are considered the same. But let's use a more reasonable scale.
    // "threshold" here is the minimum similarity to group together.
    // We use a fixed distance scale of 1000m for the Hausdorff mapping.
    let distance_scale_m = 1000.0;

    // Compute pairwise similarities
    let n = routes.len();
    let mut similarity_matrix = vec![vec![0.0_f64; n]; n];

    for i in 0..n {
        for j in (i + 1)..n {
            if routes[i].1.len() < 2 || routes[j].1.len() < 2 {
                continue;
            }
            let hausdorff = geo::hausdorff_distance(&routes[i].1, &routes[j].1);
            let sim = route_similarity(hausdorff, distance_scale_m);
            similarity_matrix[i][j] = sim;
            similarity_matrix[j][i] = sim;
        }
        similarity_matrix[i][i] = 1.0;
    }

    // Greedy single-linkage clustering
    let mut assigned = vec![false; n];
    let mut clusters: Vec<Cluster> = Vec::new();
    let mut cluster_id = 0;

    for i in 0..n {
        if assigned[i] {
            continue;
        }
        assigned[i] = true;

        let mut members = vec![i];
        let mut scores = Vec::new();

        // Find all trips similar to this one
        for j in (i + 1)..n {
            if assigned[j] {
                continue;
            }
            if similarity_matrix[i][j] >= threshold {
                assigned[j] = true;
                members.push(j);
                scores.push(PairScore {
                    trip_a: routes[i].0.clone(),
                    trip_b: routes[j].0.clone(),
                    hausdorff_m: geo::hausdorff_distance(&routes[i].1, &routes[j].1),
                    similarity: similarity_matrix[i][j],
                });
            }
        }

        let trip_ids: Vec<String> = members.iter().map(|&idx| routes[idx].0.clone()).collect();
        let representative = trip_ids[0].clone();

        clusters.push(Cluster {
            cluster_id,
            trip_ids: trip_ids.clone(),
            member_count: trip_ids.len(),
            representative_trip_id: representative,
            similarity_scores: scores,
        });

        cluster_id += 1;
    }

    let output = SimilarityOutput {
        total_trips: n,
        threshold,
        clusters,
    };

    // Print summary
    let multi_clusters: Vec<_> = output
        .clusters
        .iter()
        .filter(|c| c.member_count > 1)
        .collect();
    eprintln!(
        "{} clusters found ({} with multiple trips)",
        output.clusters.len(),
        multi_clusters.len()
    );
    for c in &multi_clusters {
        eprintln!(
            "  Cluster {}: {} trips (representative: {})",
            c.cluster_id, c.member_count, c.representative_trip_id
        );
    }

    // Write output
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

    #[test]
    fn similarity_at_zero_distance() {
        let sim = route_similarity(0.0, 1000.0);
        assert!((sim - 1.0).abs() < 1e-10);
    }

    #[test]
    fn similarity_at_full_distance() {
        let sim = route_similarity(1000.0, 1000.0);
        assert!(sim.abs() < 1e-10);
    }

    #[test]
    fn similarity_clamps_below_zero() {
        let sim = route_similarity(2000.0, 1000.0);
        assert!(sim.abs() < 1e-10);
    }

    #[test]
    fn similarity_midpoint() {
        let sim = route_similarity(500.0, 1000.0);
        assert!((sim - 0.5).abs() < 1e-10);
    }
}
