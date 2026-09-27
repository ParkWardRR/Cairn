/// Earth radius in metres (mean radius).
const EARTH_RADIUS_M: f64 = 6_371_000.0;

/// Haversine distance in metres between two WGS-84 points (degrees).
pub fn haversine_m(lat1: f64, lon1: f64, lat2: f64, lon2: f64) -> f64 {
    let r1 = lat1.to_radians();
    let r2 = lat2.to_radians();
    let dlat = r2 - r1;
    let dlon = (lon2 - lon1).to_radians();
    let a = (dlat / 2.0).sin().powi(2) + r1.cos() * r2.cos() * (dlon / 2.0).sin().powi(2);
    EARTH_RADIUS_M * 2.0 * a.sqrt().atan2((1.0 - a).sqrt())
}

/// Simplified Hausdorff distance between two polylines.
/// Returns max(directed_hausdorff(A->B), directed_hausdorff(B->A)).
/// Each polyline is a slice of (lat, lon) pairs in degrees.
pub fn hausdorff_distance(a: &[(f64, f64)], b: &[(f64, f64)]) -> f64 {
    let ab = directed_hausdorff(a, b);
    let ba = directed_hausdorff(b, a);
    ab.max(ba)
}

/// Directed Hausdorff: max over points in A of the min distance to any point in B.
fn directed_hausdorff(a: &[(f64, f64)], b: &[(f64, f64)]) -> f64 {
    let mut max_dist = 0.0_f64;
    for &(lat_a, lon_a) in a {
        let min_dist = b
            .iter()
            .map(|&(lat_b, lon_b)| haversine_m(lat_a, lon_a, lat_b, lon_b))
            .fold(f64::MAX, f64::min);
        max_dist = max_dist.max(min_dist);
    }
    max_dist
}

/// Discrete Frechet distance between two polylines.
/// Available for more precise route comparison; not currently called by any subcommand.
#[allow(dead_code)]
/// Each polyline is a slice of (lat, lon) pairs in degrees.
/// Uses dynamic programming. O(n*m) time and space.
pub fn frechet_distance(a: &[(f64, f64)], b: &[(f64, f64)]) -> f64 {
    if a.is_empty() || b.is_empty() {
        return f64::MAX;
    }

    let n = a.len();
    let m = b.len();
    let mut dp = vec![vec![f64::NEG_INFINITY; m]; n];

    for i in 0..n {
        for j in 0..m {
            let d = haversine_m(a[i].0, a[i].1, b[j].0, b[j].1);
            if i == 0 && j == 0 {
                dp[i][j] = d;
            } else if i == 0 {
                dp[i][j] = d.max(dp[i][j - 1]);
            } else if j == 0 {
                dp[i][j] = d.max(dp[i - 1][j]);
            } else {
                let prev = dp[i - 1][j].min(dp[i][j - 1]).min(dp[i - 1][j - 1]);
                dp[i][j] = d.max(prev);
            }
        }
    }

    dp[n - 1][m - 1]
}

/// Subsample a polyline to at most `max_points` evenly spaced points.
pub fn subsample(points: &[(f64, f64)], max_points: usize) -> Vec<(f64, f64)> {
    if points.len() <= max_points || max_points < 2 {
        return points.to_vec();
    }
    let step = (points.len() - 1) as f64 / (max_points - 1) as f64;
    (0..max_points)
        .map(|i| {
            let idx = (i as f64 * step).round() as usize;
            points[idx.min(points.len() - 1)]
        })
        .collect()
}

/// Compute the centroid of a set of (lat, lon) points.
pub fn centroid(points: &[(f64, f64)]) -> (f64, f64) {
    if points.is_empty() {
        return (0.0, 0.0);
    }
    let n = points.len() as f64;
    let sum_lat: f64 = points.iter().map(|p| p.0).sum();
    let sum_lon: f64 = points.iter().map(|p| p.1).sum();
    (sum_lat / n, sum_lon / n)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn haversine_known_distance() {
        // Santa Monica Pier to Venice Beach: ~3.5 km
        let d = haversine_m(34.0095, -118.4970, 33.9850, -118.4695);
        assert!(d > 3000.0 && d < 4500.0, "got {} m", d);
    }

    #[test]
    fn haversine_same_point() {
        let d = haversine_m(34.0, -118.0, 34.0, -118.0);
        assert!(d.abs() < 0.01);
    }

    #[test]
    fn hausdorff_identical() {
        let route = vec![(34.0, -118.0), (34.01, -118.01), (34.02, -118.02)];
        let d = hausdorff_distance(&route, &route);
        assert!(d < 1.0, "identical routes should have ~0 distance, got {}", d);
    }

    #[test]
    fn hausdorff_different() {
        let a = vec![(34.0, -118.0), (34.01, -118.01)];
        let b = vec![(35.0, -117.0), (35.01, -117.01)];
        let d = hausdorff_distance(&a, &b);
        assert!(d > 100_000.0, "distant routes should be far apart, got {}", d);
    }

    #[test]
    fn frechet_identical() {
        let route = vec![(34.0, -118.0), (34.01, -118.01), (34.02, -118.02)];
        let d = frechet_distance(&route, &route);
        assert!(d < 1.0, "identical routes: frechet should be ~0, got {}", d);
    }

    #[test]
    fn frechet_different() {
        let a = vec![(34.0, -118.0), (34.01, -118.01)];
        let b = vec![(35.0, -117.0), (35.01, -117.01)];
        let d = frechet_distance(&a, &b);
        assert!(d > 100_000.0, "distant routes: frechet should be large, got {}", d);
    }

    #[test]
    fn subsample_preserves_small() {
        let points = vec![(1.0, 2.0), (3.0, 4.0)];
        let result = subsample(&points, 10);
        assert_eq!(result.len(), 2);
    }

    #[test]
    fn subsample_reduces() {
        let points: Vec<(f64, f64)> = (0..100).map(|i| (i as f64, i as f64)).collect();
        let result = subsample(&points, 10);
        assert_eq!(result.len(), 10);
        // First and last should match
        assert_eq!(result[0], points[0]);
        assert_eq!(result[9], points[99]);
    }

    #[test]
    fn centroid_basic() {
        let points = vec![(0.0, 0.0), (2.0, 4.0)];
        let (lat, lon) = centroid(&points);
        assert!((lat - 1.0).abs() < 1e-10);
        assert!((lon - 2.0).abs() < 1e-10);
    }
}
