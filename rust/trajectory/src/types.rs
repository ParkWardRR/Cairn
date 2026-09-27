use serde::Serialize;
use std::io::Read;

/// 32-byte little-endian packed GNSS record, matching firmware GnssSample.
#[repr(C, packed)]
#[derive(Clone, Copy, Default, Debug)]
pub struct GnssSample {
    pub timestamp_ms: u64,
    /// Degrees * 10^7
    pub latitude: i32,
    /// Degrees * 10^7
    pub longitude: i32,
    /// Centimetres above WGS-84 ellipsoid
    pub altitude_cm: i32,
    /// Ground speed in cm/s
    pub speed_cmps: u16,
    /// Heading in centidegrees (0-35999)
    pub heading_cdeg: u16,
    /// 0=none, 1=GPS, 2=DGPS, 4=RTK
    pub fix_quality: u8,
    /// Visible satellite count
    pub satellites: u8,
    /// HDOP * 10
    pub hdop_tenths: u16,
    /// Estimated horizontal accuracy in cm
    pub accuracy_cm: u16,
    pub _reserved: [u8; 2],
}

const _: () = assert!(std::mem::size_of::<GnssSample>() == 32);

impl GnssSample {
    pub fn lat_deg(&self) -> f64 {
        self.latitude as f64 / 1e7
    }

    pub fn lon_deg(&self) -> f64 {
        self.longitude as f64 / 1e7
    }

    pub fn speed_kmh(&self) -> f64 {
        self.speed_cmps as f64 * 0.036
    }

    pub fn speed_mps(&self) -> f64 {
        self.speed_cmps as f64 / 100.0
    }

    pub fn heading_deg(&self) -> f64 {
        self.heading_cdeg as f64 / 100.0
    }

    pub fn hdop(&self) -> f64 {
        self.hdop_tenths as f64 / 10.0
    }

    #[allow(dead_code)]
    pub fn accuracy_m(&self) -> f64 {
        self.accuracy_cm as f64 / 100.0
    }

    #[allow(dead_code)]
    pub fn altitude_m(&self) -> f64 {
        self.altitude_cm as f64 / 100.0
    }
}

/// Read GNSS samples from a binary file (flat array of 32-byte records).
pub fn read_samples(path: &std::path::Path) -> anyhow::Result<Vec<GnssSample>> {
    let mut file = std::fs::File::open(path)?;
    let mut buf = Vec::new();
    file.read_to_end(&mut buf)?;

    if buf.len() % 32 != 0 {
        anyhow::bail!(
            "samples.bin size {} is not a multiple of 32 bytes",
            buf.len()
        );
    }

    let count = buf.len() / 32;
    let mut samples = Vec::with_capacity(count);

    for i in 0..count {
        let offset = i * 32;
        let chunk: [u8; 32] = buf[offset..offset + 32].try_into().unwrap();
        let sample: GnssSample = unsafe { std::mem::transmute(chunk) };
        samples.push(sample);
    }

    Ok(samples)
}

/// Trip manifest loaded from manifest.json.
#[derive(Debug, Clone, Serialize, serde::Deserialize)]
pub struct Manifest {
    pub version: Option<u32>,
    pub trip_id: String,
    pub device_id: String,
    #[serde(default)]
    pub firmware_version: String,
    #[serde(default)]
    pub started_at: String,
    #[serde(default)]
    pub ended_at: String,
    pub sample_count: Option<SampleCount>,
}

#[derive(Debug, Clone, Serialize, serde::Deserialize)]
pub struct SampleCount {
    pub gnss: Option<usize>,
    pub imu_summary: Option<usize>,
    pub imu_raw_windows: Option<usize>,
}

/// A loaded trip bundle: manifest + GNSS samples.
#[derive(Debug, Clone)]
pub struct TripBundle {
    pub manifest: Manifest,
    pub samples: Vec<GnssSample>,
    #[allow(dead_code)]
    pub bundle_path: std::path::PathBuf,
}

impl TripBundle {
    pub fn trip_id(&self) -> &str {
        &self.manifest.trip_id
    }
}

/// Load a single trip bundle from a directory.
pub fn load_bundle(dir: &std::path::Path) -> anyhow::Result<TripBundle> {
    let manifest_path = dir.join("manifest.json");
    if !manifest_path.exists() {
        anyhow::bail!("No manifest.json in {}", dir.display());
    }

    let manifest_str = std::fs::read_to_string(&manifest_path)?;
    let manifest: Manifest = serde_json::from_str(&manifest_str)?;

    let samples_path = dir.join("samples.bin");
    let samples = if samples_path.exists() {
        read_samples(&samples_path)?
    } else {
        Vec::new()
    };

    Ok(TripBundle {
        manifest,
        samples,
        bundle_path: dir.to_path_buf(),
    })
}

/// Load all trip bundles from a directory (each subdirectory with manifest.json).
pub fn load_bundles(dir: &std::path::Path) -> anyhow::Result<Vec<TripBundle>> {
    let mut bundles = Vec::new();

    // Check if this directory itself is a bundle
    if dir.join("manifest.json").exists() {
        bundles.push(load_bundle(dir)?);
        return Ok(bundles);
    }

    // Otherwise scan subdirectories
    let entries = std::fs::read_dir(dir)?;
    for entry in entries {
        let entry = entry?;
        let path = entry.path();
        if path.is_dir() && path.join("manifest.json").exists() {
            match load_bundle(&path) {
                Ok(bundle) => bundles.push(bundle),
                Err(e) => {
                    eprintln!("Warning: skipping {}: {}", path.display(), e);
                }
            }
        }
    }

    bundles.sort_by(|a, b| a.manifest.started_at.cmp(&b.manifest.started_at));

    Ok(bundles)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn gnss_sample_size() {
        assert_eq!(std::mem::size_of::<GnssSample>(), 32);
    }

    #[test]
    fn gnss_sample_accessors() {
        let sample = GnssSample {
            timestamp_ms: 1695667800000,
            latitude: 340195000,
            longitude: -1184912000,
            altitude_cm: 3000,
            speed_cmps: 1389,
            heading_cdeg: 9000,
            fix_quality: 1,
            satellites: 10,
            hdop_tenths: 12,
            accuracy_cm: 250,
            _reserved: [0; 2],
        };

        assert!((sample.lat_deg() - 34.0195).abs() < 1e-6);
        assert!((sample.lon_deg() - (-118.4912)).abs() < 1e-6);
        assert!((sample.altitude_m() - 30.0).abs() < 0.01);
        assert!((sample.speed_kmh() - 50.004).abs() < 0.01);
        assert!((sample.heading_deg() - 90.0).abs() < 0.01);
        assert!((sample.hdop() - 1.2).abs() < 0.01);
        assert!((sample.accuracy_m() - 2.5).abs() < 0.01);
    }

    #[test]
    fn gnss_sample_roundtrip() {
        let sample = GnssSample {
            timestamp_ms: 1695667800000,
            latitude: 340195000,
            longitude: -1184912000,
            altitude_cm: 3000,
            speed_cmps: 1389,
            heading_cdeg: 9000,
            fix_quality: 1,
            satellites: 10,
            hdop_tenths: 12,
            accuracy_cm: 250,
            _reserved: [0; 2],
        };

        let bytes: [u8; 32] = unsafe { std::mem::transmute(sample) };
        let restored: GnssSample = unsafe { std::mem::transmute(bytes) };

        // Copy fields out of packed struct before comparing (avoid unaligned refs)
        let ts = { restored.timestamp_ms };
        let lat = { restored.latitude };
        let lon = { restored.longitude };
        let spd = { restored.speed_cmps };
        let sats = { restored.satellites };

        assert_eq!({ sample.timestamp_ms }, ts);
        assert_eq!({ sample.latitude }, lat);
        assert_eq!({ sample.longitude }, lon);
        assert_eq!({ sample.speed_cmps }, spd);
        assert_eq!({ sample.satellites }, sats);
    }
}
