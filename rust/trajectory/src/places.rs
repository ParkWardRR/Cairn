use anyhow::Result;
use serde::Serialize;
use std::path::Path;

use crate::geo;
use crate::types;

#[derive(Debug, Clone, Serialize)]
pub struct PlacesOutput {
    pub places: Vec<DiscoveredPlace>,
    pub total_trips: usize,
    pub radius_m: f64,
    pub min_visits: usize,
}

#[derive(Debug, Clone, Serialize)]
pub struct DiscoveredPlace {
    pub place_id: usize,
    pub centroid_lat: f64,
    pub centroid_lon: f64,
    pub visit_count: usize,
    pub trip_ids: Vec<String>,
    pub role: Vec<String>,
    pub typical_times: Vec<String>,
}

/// An endpoint extracted from a trip (start or end).
#[derive(Debug, Clone)]
struct Endpoint {
    lat: f64,
    lon: f64,
    trip_id: String,
    role: String,      // "start" or "end"
    timestamp: String,  // ISO-8601 or raw timestamp
}

/// Extract start and end points from a trip bundle.
fn extract_endpoints(bundle: &types::TripBundle) -> Vec<Endpoint> {
    let mut endpoints = Vec::new();

    let good_samples: Vec<&types::GnssSample> = bundle
        .samples
        .iter()
        .filter(|s| s.fix_quality > 0 && s.satellites >= 3)
        .collect();

    if good_samples.is_empty() {
        return endpoints;
    }

    // Start point: first good sample
    let start = good_samples[0];
    endpoints.push(Endpoint {
        lat: start.lat_deg(),
        lon: start.lon_deg(),
        trip_id: bundle.trip_id().to_string(),
        role: "start".to_string(),
        timestamp: bundle.manifest.started_at.clone(),
    });

    // End point: last good sample
    let end = good_samples[good_samples.len() - 1];
    endpoints.push(Endpoint {
        lat: end.lat_deg(),
        lon: end.lon_deg(),
        trip_id: bundle.trip_id().to_string(),
        role: "end".to_string(),
        timestamp: bundle.manifest.ended_at.clone(),
    });

    endpoints
}

/// DBSCAN-style clustering of endpoints.
fn cluster_endpoints(endpoints: &[Endpoint], radius_m: f64) -> Vec<Vec<usize>> {
    let n = endpoints.len();
    let mut visited = vec![false; n];
    let mut clusters: Vec<Vec<usize>> = Vec::new();

    for i in 0..n {
        if visited[i] {
            continue;
        }
        visited[i] = true;

        let mut cluster = vec![i];
        let mut queue = vec![i];

        while let Some(current) = queue.pop() {
            for j in 0..n {
                if visited[j] {
                    continue;
                }
                let dist = geo::haversine_m(
                    endpoints[current].lat,
                    endpoints[current].lon,
                    endpoints[j].lat,
                    endpoints[j].lon,
                );
                if dist <= radius_m {
                    visited[j] = true;
                    cluster.push(j);
                    queue.push(j);
                }
            }
        }

        clusters.push(cluster);
    }

    clusters
}

/// Run automatic place discovery over all bundles.
pub fn run(
    bundles_dir: &Path,
    radius_m: f64,
    min_visits: usize,
    output_path: Option<&Path>,
) -> Result<PlacesOutput> {
    let bundles = types::load_bundles(bundles_dir)?;

    if bundles.is_empty() {
        anyhow::bail!("No trip bundles found in {}", bundles_dir.display());
    }

    eprintln!("Loaded {} trip bundles", bundles.len());

    // Extract all endpoints
    let mut all_endpoints: Vec<Endpoint> = Vec::new();
    for bundle in &bundles {
        all_endpoints.extend(extract_endpoints(bundle));
    }

    eprintln!("Extracted {} endpoints", all_endpoints.len());

    // Cluster endpoints
    let clusters = cluster_endpoints(&all_endpoints, radius_m);

    // Build place records, filtering by min_visits
    let mut places: Vec<DiscoveredPlace> = Vec::new();
    let mut place_id = 0;

    for cluster_indices in &clusters {
        // Count unique trip visits (a trip visiting as start AND end counts once)
        let mut trip_ids: Vec<String> = cluster_indices
            .iter()
            .map(|&i| all_endpoints[i].trip_id.clone())
            .collect();
        trip_ids.sort();
        trip_ids.dedup();

        if trip_ids.len() < min_visits {
            continue;
        }

        // Compute centroid
        let points: Vec<(f64, f64)> = cluster_indices
            .iter()
            .map(|&i| (all_endpoints[i].lat, all_endpoints[i].lon))
            .collect();
        let (clat, clon) = geo::centroid(&points);

        // Collect roles
        let mut roles: Vec<String> = cluster_indices
            .iter()
            .map(|&i| all_endpoints[i].role.clone())
            .collect();
        roles.sort();
        roles.dedup();

        // Collect timestamps
        let mut times: Vec<String> = cluster_indices
            .iter()
            .map(|&i| all_endpoints[i].timestamp.clone())
            .filter(|t| !t.is_empty())
            .collect();
        times.sort();

        places.push(DiscoveredPlace {
            place_id,
            centroid_lat: clat,
            centroid_lon: clon,
            visit_count: trip_ids.len(),
            trip_ids,
            role: roles,
            typical_times: times,
        });

        place_id += 1;
    }

    places.sort_by(|a, b| b.visit_count.cmp(&a.visit_count));

    let output = PlacesOutput {
        total_trips: bundles.len(),
        radius_m,
        min_visits,
        places,
    };

    // Print summary
    eprintln!(
        "{} recurring places found (>= {} visits, {}m radius)",
        output.places.len(),
        min_visits,
        radius_m
    );
    for p in &output.places {
        eprintln!(
            "  Place {}: ({:.6}, {:.6}) - {} visits, roles: {:?}",
            p.place_id, p.centroid_lat, p.centroid_lon, p.visit_count, p.role
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

    #[test]
    fn cluster_same_point() {
        let endpoints = vec![
            Endpoint {
                lat: 34.0,
                lon: -118.0,
                trip_id: "A".to_string(),
                role: "start".to_string(),
                timestamp: String::new(),
            },
            Endpoint {
                lat: 34.0,
                lon: -118.0,
                trip_id: "B".to_string(),
                role: "start".to_string(),
                timestamp: String::new(),
            },
        ];
        let clusters = cluster_endpoints(&endpoints, 100.0);
        assert_eq!(clusters.len(), 1);
        assert_eq!(clusters[0].len(), 2);
    }

    #[test]
    fn cluster_distant_points() {
        let endpoints = vec![
            Endpoint {
                lat: 34.0,
                lon: -118.0,
                trip_id: "A".to_string(),
                role: "start".to_string(),
                timestamp: String::new(),
            },
            Endpoint {
                lat: 35.0,
                lon: -117.0,
                trip_id: "B".to_string(),
                role: "start".to_string(),
                timestamp: String::new(),
            },
        ];
        let clusters = cluster_endpoints(&endpoints, 100.0);
        assert_eq!(clusters.len(), 2);
    }
}
