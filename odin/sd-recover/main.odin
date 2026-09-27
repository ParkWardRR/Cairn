package sd_recover

import "core:crypto/sha2"
import "core:fmt"
import "core:os"
import "core:path/filepath"
import "core:strings"

// ── Record layouts ──────────────────────────────────────────────────

GNSS_RECORD_SIZE :: 32
IMU_RECORD_SIZE  :: 24

Gnss_Record :: struct #packed {
	timestamp_ms: u64le,
	latitude:     i32le,  // deg * 10^7
	longitude:    i32le,  // deg * 10^7
	altitude_cm:  i32le,
	speed_cmps:   u16le,
	heading_cdeg: u16le,
	fix_quality:  u8,
	satellites:   u8,
	hdop_tenths:  u16le,
	accuracy_cm:  u16le,
	reserved:     [2]u8,
}

#assert(size_of(Gnss_Record) == GNSS_RECORD_SIZE)

// ── Bundle status ───────────────────────────────────────────────────

Bundle_Status :: enum {
	Complete,
	Truncated,
	Corrupt,
}

Bundle_Info :: struct {
	path:               string,
	dir_name:           string,
	status:             Bundle_Status,

	// Samples
	samples_exist:      bool,
	samples_bytes:      int,
	total_gnss_records: int,
	valid_gnss_records: int,
	gnss_truncated:     bool,

	// IMU
	imu_exist:          bool,
	imu_bytes:          int,
	total_imu_records:  int,
	valid_imu_records:  int,
	imu_truncated:      bool,

	// Other files
	manifest_exist:     bool,
	events_exist:       bool,
	sha256sums_exist:   bool,

	// Time range from samples
	first_timestamp_ms: u64,
	last_timestamp_ms:  u64,
}

// ── CLI config ──────────────────────────────────────────────────────

Config :: struct {
	scan_dir:    string,
	output_dir:  string,
	dry_run:     bool,
	min_samples: int,
}

// ── Main ────────────────────────────────────────────────────────────

main :: proc() {
	config: Config
	config.output_dir  = "recovered"
	config.min_samples = 10

	args := os.args[1:]
	if len(args) == 0 {
		usage()
		os.exit(1)
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--output" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --output requires an argument")
				os.exit(1)
			}
			config.output_dir = args[i]
		} else if arg == "--dry-run" {
			config.dry_run = true
		} else if arg == "--min-samples" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --min-samples requires an argument")
				os.exit(1)
			}
			n, ok := parse_int(args[i])
			if !ok {
				fmt.eprintfln("Error: invalid --min-samples value: %s", args[i])
				os.exit(1)
			}
			config.min_samples = n
		} else if arg == "--help" || arg == "-h" {
			usage()
			os.exit(0)
		} else if strings.has_prefix(arg, "-") {
			fmt.eprintfln("Error: unknown flag: %s", arg)
			os.exit(1)
		} else {
			if config.scan_dir != "" {
				fmt.eprintln("Error: multiple scan directories specified")
				os.exit(1)
			}
			config.scan_dir = arg
		}
		i += 1
	}

	if config.scan_dir == "" {
		fmt.eprintln("Error: scan directory required")
		usage()
		os.exit(1)
	}

	// Verify scan directory exists
	if !dir_exists(config.scan_dir) {
		fmt.eprintfln("Error: scan directory does not exist: %s", config.scan_dir)
		os.exit(1)
	}

	// Scan for candidate bundle directories
	candidates := find_bundle_dirs(config.scan_dir)
	if len(candidates) == 0 {
		fmt.println("No trip bundle directories found.")
		os.exit(0)
	}

	fmt.printfln("Found %d candidate bundle(s)\n", len(candidates))

	// Analyze each candidate
	bundles := make([dynamic]Bundle_Info)
	for dir in candidates {
		info := analyze_bundle(dir)
		if info.valid_gnss_records >= config.min_samples {
			append(&bundles, info)
		} else {
			fmt.printfln("  Skipping %s: only %d valid samples (minimum: %d)\n",
				info.dir_name, info.valid_gnss_records, config.min_samples)
		}
	}

	if len(bundles) == 0 {
		fmt.println("No recoverable bundles found.")
		os.exit(0)
	}

	// Print report
	fmt.println("--- Recovery Report -------------------------------------------------------")
	for &info in bundles {
		print_bundle_report(&info)
	}
	fmt.println("--------------------------------------------------------------------------")

	// Write recovered bundles
	if !config.dry_run {
		fmt.printfln("\nWriting recovered bundles to: %s", config.output_dir)
		for &info in bundles {
			write_recovered_bundle(&info, config.output_dir)
		}
		fmt.println("\nRecovery complete.")
	} else {
		fmt.println("\nDry run -- no files written.")
	}
}

usage :: proc() {
	fmt.println("Usage: sd-recover <scan-dir> [flags]")
	fmt.println()
	fmt.println("Scans a directory for Cairn trip bundles and recovers incomplete data.")
	fmt.println()
	fmt.println("Flags:")
	fmt.println("  --output <dir>       Output directory (default: recovered/)")
	fmt.println("  --dry-run            Scan and report without writing")
	fmt.println("  --min-samples <N>    Minimum GNSS samples to recover (default: 10)")
	fmt.println("  -h, --help           Show this help")
}

// ── Directory scanning ──────────────────────────────────────────────

find_bundle_dirs :: proc(root: string) -> [dynamic]string {
	result := make([dynamic]string)
	walk_for_bundles(root, &result)
	return result
}

walk_for_bundles :: proc(dir: string, result: ^[dynamic]string) {
	dh, open_err := os.open(dir)
	if open_err != nil {
		return
	}
	defer os.close(dh)

	entries, read_err := os.read_dir(dh, -1, context.allocator)
	if read_err != nil {
		return
	}
	defer os.file_info_slice_delete(entries, context.allocator)

	has_samples := false
	subdirs := make([dynamic]string)
	defer delete(subdirs)

	for entry in entries {
		if entry.name == "samples.bin" && entry.type != .Directory {
			has_samples = true
		}
		if entry.type == .Directory && !strings.has_prefix(entry.name, ".") {
			joined, join_err := filepath.join({dir, entry.name})
			if join_err == nil {
				append(&subdirs, joined)
			}
		}
	}

	if has_samples {
		append(result, strings.clone(dir))
	}

	// Recurse into subdirectories
	for subdir in subdirs {
		walk_for_bundles(subdir, result)
	}
}

// ── Bundle analysis ─────────────────────────────────────────────────

analyze_bundle :: proc(dir: string) -> Bundle_Info {
	info: Bundle_Info
	info.path     = dir
	info.dir_name = filepath.base(dir)

	fmt.printfln("Analyzing: %s", dir)

	// Check samples.bin
	samples_path, _ := filepath.join({dir, "samples.bin"})
	defer delete(samples_path)
	info.samples_exist = file_exists(samples_path)
	if info.samples_exist {
		analyze_samples(&info, samples_path)
	}

	// Check imu_summary.bin
	imu_path, _ := filepath.join({dir, "imu_summary.bin"})
	defer delete(imu_path)
	info.imu_exist = file_exists(imu_path)
	if info.imu_exist {
		analyze_imu(&info, imu_path)
	}

	// Check other files
	manifest_path, _ := filepath.join({dir, "manifest.json"})
	defer delete(manifest_path)
	info.manifest_exist = file_exists(manifest_path)

	events_path, _ := filepath.join({dir, "events.json"})
	defer delete(events_path)
	info.events_exist = file_exists(events_path)

	sha_path, _ := filepath.join({dir, "sha256sums.txt"})
	defer delete(sha_path)
	info.sha256sums_exist = file_exists(sha_path)

	// Determine status
	if info.manifest_exist && info.sha256sums_exist && !info.gnss_truncated && !info.imu_truncated {
		info.status = .Complete
	} else if info.valid_gnss_records > 0 {
		info.status = .Truncated
	} else {
		info.status = .Corrupt
	}

	return info
}

analyze_samples :: proc(info: ^Bundle_Info, path: string) {
	data, read_err := os.read_entire_file(path, context.allocator)
	if read_err != nil {
		return
	}
	defer delete(data)

	fsize := len(data)
	info.samples_bytes = fsize

	// Check alignment
	remainder := fsize %% GNSS_RECORD_SIZE
	aligned_size := fsize - remainder
	if remainder != 0 {
		info.gnss_truncated = true
	}

	info.total_gnss_records = aligned_size / GNSS_RECORD_SIZE

	// Validate records
	prev_ts: u64 = 0
	valid_count := 0

	for idx := 0; idx < info.total_gnss_records; idx += 1 {
		offset := idx * GNSS_RECORD_SIZE
		rec := (^Gnss_Record)(&data[offset])

		ts  := u64(rec.timestamp_ms)
		lat := f64(i32(rec.latitude))  / 1e7
		lon := f64(i32(rec.longitude)) / 1e7

		// Validate timestamp is monotonically increasing
		if ts <= prev_ts && idx > 0 {
			break
		}
		// Validate lat/lon range
		if lat < -90.0 || lat > 90.0 || lon < -180.0 || lon > 180.0 {
			break
		}
		// Reject obviously zeroed records
		if ts == 0 {
			break
		}

		prev_ts = ts
		valid_count += 1

		if idx == 0 {
			info.first_timestamp_ms = ts
		}
		info.last_timestamp_ms = ts
	}

	info.valid_gnss_records = valid_count
	if valid_count < info.total_gnss_records {
		info.gnss_truncated = true
	}
}

analyze_imu :: proc(info: ^Bundle_Info, path: string) {
	sz, err := get_file_size(path)
	if err != nil {
		return
	}

	info.imu_bytes = int(sz)
	remainder := int(sz) %% IMU_RECORD_SIZE
	aligned_records := int(sz) / IMU_RECORD_SIZE

	if remainder != 0 {
		info.imu_truncated = true
	}

	info.total_imu_records = aligned_records
	info.valid_imu_records = aligned_records  // We accept all aligned IMU records
}

// ── Report printing ─────────────────────────────────────────────────

print_bundle_report :: proc(info: ^Bundle_Info) {
	status_str: string
	switch info.status {
	case .Complete:  status_str = "COMPLETE"
	case .Truncated: status_str = "TRUNCATED"
	case .Corrupt:   status_str = "CORRUPT"
	}

	fmt.printfln("\n  Bundle: %s", info.dir_name)
	fmt.printfln("  Path:   %s", info.path)
	fmt.printfln("  Status: %s", status_str)

	if info.samples_exist {
		fmt.printfln("  GNSS samples: %d valid / %d total records (%d bytes)",
			info.valid_gnss_records, info.total_gnss_records, info.samples_bytes)
	} else {
		fmt.println("  GNSS samples: MISSING")
	}

	if info.imu_exist {
		fmt.printfln("  IMU records:  %d valid / %d total records (%d bytes)",
			info.valid_imu_records, info.total_imu_records, info.imu_bytes)
	} else {
		fmt.println("  IMU records:  MISSING")
	}

	fmt.printfln("  manifest.json:  %s", info.manifest_exist  ? "present" : "MISSING")
	fmt.printfln("  events.json:    %s", info.events_exist    ? "present" : "MISSING")
	fmt.printfln("  sha256sums.txt: %s", info.sha256sums_exist ? "present" : "MISSING")

	if info.valid_gnss_records > 0 {
		first_str := format_timestamp_ms(info.first_timestamp_ms)
		last_str  := format_timestamp_ms(info.last_timestamp_ms)
		defer delete(first_str)
		defer delete(last_str)
		duration_s := (info.last_timestamp_ms - info.first_timestamp_ms) / 1000
		dur_min := duration_s / 60
		dur_sec := duration_s %% 60
		fmt.printfln("  Time range: %s .. %s (%dm%02ds)",
			first_str, last_str, dur_min, dur_sec)
	}
}

// ── Recovery write ──────────────────────────────────────────────────

write_recovered_bundle :: proc(info: ^Bundle_Info, output_dir: string) {
	bundle_dir, _ := filepath.join({output_dir, info.dir_name})
	defer delete(bundle_dir)

	// Create output directory
	make_dirs(bundle_dir)

	fmt.printfln("  Writing: %s", bundle_dir)

	// 1. Write truncated samples.bin (only valid records)
	if info.samples_exist && info.valid_gnss_records > 0 {
		src_path, _ := filepath.join({info.path, "samples.bin"})
		defer delete(src_path)

		data, read_err := os.read_entire_file(src_path, context.allocator)
		if read_err == nil {
			valid_bytes := info.valid_gnss_records * GNSS_RECORD_SIZE
			dst_path, _ := filepath.join({bundle_dir, "samples.bin"})
			defer delete(dst_path)
			_ = os.write_entire_file(dst_path, data[:valid_bytes])
			delete(data)
		}
	}

	// 2. Write imu_summary.bin (truncated to aligned records)
	if info.imu_exist && info.valid_imu_records > 0 {
		src_path, _ := filepath.join({info.path, "imu_summary.bin"})
		defer delete(src_path)

		data, read_err := os.read_entire_file(src_path, context.allocator)
		if read_err == nil {
			valid_bytes := info.valid_imu_records * IMU_RECORD_SIZE
			dst_path, _ := filepath.join({bundle_dir, "imu_summary.bin"})
			defer delete(dst_path)
			_ = os.write_entire_file(dst_path, data[:valid_bytes])
			delete(data)
		}
	}

	// 3. Write manifest.json -- use original if present, else reconstruct
	manifest_src, _ := filepath.join({info.path, "manifest.json"})
	defer delete(manifest_src)
	manifest_dst, _ := filepath.join({bundle_dir, "manifest.json"})
	defer delete(manifest_dst)

	if info.manifest_exist {
		copy_file(manifest_src, manifest_dst)
	} else {
		manifest_json := build_manifest(info)
		defer delete(manifest_json)
		_ = os.write_entire_file(manifest_dst, transmute([]u8)manifest_json)
	}

	// 4. Write events.json -- use original if present, else create minimal
	events_src, _ := filepath.join({info.path, "events.json"})
	defer delete(events_src)
	events_dst, _ := filepath.join({bundle_dir, "events.json"})
	defer delete(events_dst)

	if info.events_exist {
		copy_file(events_src, events_dst)
	} else {
		events_json := build_minimal_events(info)
		defer delete(events_json)
		_ = os.write_entire_file(events_dst, transmute([]u8)events_json)
	}

	// 5. Recompute sha256sums.txt
	sha_dst, _ := filepath.join({bundle_dir, "sha256sums.txt"})
	defer delete(sha_dst)
	compute_sha256sums(bundle_dir, sha_dst)
}

// ── Manifest reconstruction ─────────────────────────────────────────

build_manifest :: proc(info: ^Bundle_Info) -> string {
	started_at := format_timestamp_ms(info.first_timestamp_ms)
	ended_at   := format_timestamp_ms(info.last_timestamp_ms)
	defer delete(started_at)
	defer delete(ended_at)

	return fmt.aprintf(
		"{{\n" +
		"  \"schema_version\": 1,\n" +
		"  \"version\": 1,\n" +
		"  \"trip_id\": \"%s\",\n" +
		"  \"device_id\": \"unknown\",\n" +
		"  \"firmware_version\": \"unknown\",\n" +
		"  \"started_at\": \"%s\",\n" +
		"  \"started_at_ms\": %d,\n" +
		"  \"ended_at\": \"%s\",\n" +
		"  \"ended_at_ms\": %d,\n" +
		"  \"gnss_rate_hz\": 5,\n" +
		"  \"gnss_sample_count\": %d,\n" +
		"  \"imu_rate_hz\": 50,\n" +
		"  \"imu_summary_count\": %d,\n" +
		"  \"sample_count\": {{\n" +
		"    \"gnss\": %d,\n" +
		"    \"imu_raw_windows\": 0,\n" +
		"    \"imu_summary\": %d\n" +
		"  }},\n" +
		"  \"compression\": \"none\",\n" +
		"  \"recovered\": true\n" +
		"}}\n",
		info.dir_name,
		started_at, info.first_timestamp_ms,
		ended_at, info.last_timestamp_ms,
		info.valid_gnss_records,
		info.valid_imu_records,
		info.valid_gnss_records,
		info.valid_imu_records,
	)
}

build_minimal_events :: proc(info: ^Bundle_Info) -> string {
	started_at := format_timestamp_ms(info.first_timestamp_ms)
	ended_at   := format_timestamp_ms(info.last_timestamp_ms)
	defer delete(started_at)
	defer delete(ended_at)

	// Read first and last sample for lat/lon
	first_lat, first_lon: f64
	last_lat, last_lon: f64

	src_path, _ := filepath.join({info.path, "samples.bin"})
	defer delete(src_path)
	data, read_err := os.read_entire_file(src_path, context.allocator)
	if read_err == nil {
		defer delete(data)
		if len(data) >= GNSS_RECORD_SIZE {
			first_rec := (^Gnss_Record)(&data[0])
			first_lat = f64(i32(first_rec.latitude))  / 1e7
			first_lon = f64(i32(first_rec.longitude)) / 1e7
		}
		last_offset := (info.valid_gnss_records - 1) * GNSS_RECORD_SIZE
		if last_offset >= 0 && last_offset + GNSS_RECORD_SIZE <= len(data) {
			last_rec := (^Gnss_Record)(&data[last_offset])
			last_lat = f64(i32(last_rec.latitude))  / 1e7
			last_lon = f64(i32(last_rec.longitude)) / 1e7
		}
	}

	return fmt.aprintf(
		"[\n" +
		"  {{\n" +
		"    \"type\": \"trip_start\",\n" +
		"    \"timestamp\": \"%s\",\n" +
		"    \"data\": {{\n" +
		"      \"details\": \"recording started (recovered)\",\n" +
		"      \"lat\": %.7f,\n" +
		"      \"lon\": %.7f\n" +
		"    }}\n" +
		"  }},\n" +
		"  {{\n" +
		"    \"type\": \"trip_end\",\n" +
		"    \"timestamp\": \"%s\",\n" +
		"    \"data\": {{\n" +
		"      \"details\": \"last valid sample (recovered)\",\n" +
		"      \"lat\": %.7f,\n" +
		"      \"lon\": %.7f\n" +
		"    }}\n" +
		"  }}\n" +
		"]\n",
		started_at, first_lat, first_lon,
		ended_at, last_lat, last_lon,
	)
}

// ── SHA-256 computation ─────────────────────────────────────────────

compute_sha256 :: proc(data: []u8) -> [32]u8 {
	ctx: sha2.Context_256
	sha2.init_256(&ctx)
	sha2.update(&ctx, data)
	digest: [32]u8
	sha2.final(&ctx, digest[:])
	return digest
}

compute_sha256sums :: proc(bundle_dir: string, output_path: string) {
	files := [?]string{"samples.bin", "imu_summary.bin", "events.json", "manifest.json"}
	b := strings.builder_make()
	defer strings.builder_destroy(&b)

	for fname in files {
		fpath, _ := filepath.join({bundle_dir, fname})
		defer delete(fpath)

		data, read_err := os.read_entire_file(fpath, context.allocator)
		if read_err != nil {
			continue
		}
		defer delete(data)

		digest := compute_sha256(data)

		// Format hex
		for byte_val in digest {
			fmt.sbprintf(&b, "%02x", byte_val)
		}
		fmt.sbprintf(&b, "  %s\n", fname)
	}

	_ = os.write_entire_file(output_path, transmute([]u8)strings.to_string(b))
}

// ── Timestamp formatting ────────────────────────────────────────────

format_timestamp_ms :: proc(ms: u64) -> string {
	secs := ms / 1000
	return format_unix_to_iso8601(secs)
}

format_unix_to_iso8601 :: proc(epoch_secs: u64) -> string {
	// Convert epoch seconds to broken-down UTC time
	s := i64(epoch_secs)

	// Days since epoch
	days := s / 86400
	time_of_day := s %% 86400
	if time_of_day < 0 {
		time_of_day += 86400
		days -= 1
	}

	hour := time_of_day / 3600
	minute := (time_of_day %% 3600) / 60
	second := time_of_day %% 60

	// Civil date from days since 1970-01-01 (algorithm from Howard Hinnant)
	z := days + 719468
	era := (z if z >= 0 else z - 146096) / 146097
	doe := z - era * 146097                                   // day of era [0, 146096]
	yoe := (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365  // year of era [0, 399]
	y   := yoe + era * 400
	doy := doe - (365 * yoe + yoe / 4 - yoe / 100)           // day of year [0, 365]
	mp  := (5 * doy + 2) / 153                                // month pseudo [0, 11]
	d   := doy - (153 * mp + 2) / 5 + 1                       // day [1, 31]
	m   := mp + (3 if mp < 10 else -9)                         // month [1, 12]
	if m <= 2 {
		y += 1
	}

	return fmt.aprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", y, m, d, hour, minute, second)
}

// ── File utilities ──────────────────────────────────────────────────

file_exists :: proc(path: string) -> bool {
	info, err := os.stat(path, context.allocator)
	if err != nil {
		return false
	}
	os.file_info_delete(info, context.allocator)
	return info.type != .Directory
}

dir_exists :: proc(path: string) -> bool {
	info, err := os.stat(path, context.allocator)
	if err != nil {
		return false
	}
	os.file_info_delete(info, context.allocator)
	return info.type == .Directory
}

get_file_size :: proc(path: string) -> (i64, os.Error) {
	info, err := os.stat(path, context.allocator)
	if err != nil {
		return 0, err
	}
	sz := info.size
	os.file_info_delete(info, context.allocator)
	return sz, nil
}

copy_file :: proc(src: string, dst: string) {
	data, read_err := os.read_entire_file(src, context.allocator)
	if read_err == nil {
		_ = os.write_entire_file(dst, data)
		delete(data)
	}
}

make_dirs :: proc(path: string) {
	// Split path and create each component
	parts := strings.split(path, "/")
	defer delete(parts)

	b := strings.builder_make()
	defer strings.builder_destroy(&b)

	for part in parts {
		if len(part) == 0 {
			strings.write_byte(&b, '/')
			continue
		}
		if strings.builder_len(b) > 0 && b.buf[strings.builder_len(b) - 1] != '/' {
			strings.write_byte(&b, '/')
		}
		strings.write_string(&b, part)
		current := strings.to_string(b)
		if !dir_exists(current) {
			_ = os.make_directory(current)
		}
	}
}

parse_int :: proc(s: string) -> (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	result := 0
	for c in s {
		if c < '0' || c > '9' {
			return 0, false
		}
		result = result * 10 + int(c - '0')
	}
	return result, true
}
