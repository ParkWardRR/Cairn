mod anomalies;
mod density;
mod geo;
mod places;
mod segments;
mod similarity;
mod types;

use std::path::PathBuf;

use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(
    name = "cairn-trajectory",
    about = "Trajectory analysis experiments for Cairn trip bundles",
    version
)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Route similarity clustering across trips
    Similarity {
        /// Directory containing trip bundles
        #[arg(long)]
        bundles: PathBuf,

        /// Minimum similarity threshold (0.0-1.0) for clustering
        #[arg(long, default_value_t = 0.85)]
        threshold: f64,

        /// Output file path (JSON); prints to stdout if omitted
        #[arg(long)]
        output: Option<PathBuf>,
    },

    /// Automatic place discovery from trip endpoints
    Places {
        /// Directory containing trip bundles
        #[arg(long)]
        bundles: PathBuf,

        /// Clustering radius in metres
        #[arg(long, default_value_t = 100.0)]
        radius_m: f64,

        /// Minimum visit count to surface a place
        #[arg(long, default_value_t = 3)]
        min_visits: usize,

        /// Output file path (JSON); prints to stdout if omitted
        #[arg(long)]
        output: Option<PathBuf>,
    },

    /// Stop and errand segmentation for a single trip
    Segments {
        /// Path to a single trip bundle directory
        #[arg(long)]
        bundle: PathBuf,

        /// Minimum stop duration in seconds
        #[arg(long, default_value_t = 120)]
        min_stop_s: u64,

        /// Output file path (JSON); prints to stdout if omitted
        #[arg(long)]
        output: Option<PathBuf>,
    },

    /// GNSS anomaly detection for a single trip
    Anomalies {
        /// Path to a single trip bundle directory
        #[arg(long)]
        bundle: PathBuf,

        /// Output file path (JSON); prints to stdout if omitted
        #[arg(long)]
        output: Option<PathBuf>,
    },

    /// Driving style analysis across trips
    Density {
        /// Directory containing trip bundles
        #[arg(long)]
        bundles: PathBuf,

        /// Output file path (JSON); prints to stdout if omitted
        #[arg(long)]
        output: Option<PathBuf>,
    },
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Command::Similarity {
            bundles,
            threshold,
            output,
        } => {
            similarity::run(&bundles, threshold, output.as_deref())?;
        }
        Command::Places {
            bundles,
            radius_m,
            min_visits,
            output,
        } => {
            places::run(&bundles, radius_m, min_visits, output.as_deref())?;
        }
        Command::Segments {
            bundle,
            min_stop_s,
            output,
        } => {
            segments::run(&bundle, min_stop_s, output.as_deref())?;
        }
        Command::Anomalies { bundle, output } => {
            anomalies::run(&bundle, output.as_deref())?;
        }
        Command::Density { bundles, output } => {
            density::run(&bundles, output.as_deref())?;
        }
    }

    Ok(())
}
