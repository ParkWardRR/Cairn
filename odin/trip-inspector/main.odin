package main

import "core:os"
import "core:fmt"
import "core:encoding/json"
import "core:crypto/hash"
import "core:strings"
import "core:math"
import "core:mem"
import "core:slice"
import "core:strconv"
import "core:path/filepath"

// ─── Binary record types ────────────────────────────────────────────────────

GNSS_Sample :: struct #packed {
	timestamp_ms: u64,
	latitude:     i32,
	longitude:    i32,
	altitude_cm:  i32,
	speed_cmps:   u16,
	heading_cdeg: u16,
	fix_quality:  u8,
	satellites:   u8,
	hdop_tenths:  u16,
	accuracy_cm:  u16,
	reserved:     [2]u8,
}

IMU_Summary :: struct #packed {
	window_start_ms:    u64,
	window_duration_ms: u16,
	accel_peak_x_mg:    i16,
	accel_peak_y_mg:    i16,
	accel_peak_z_mg:    i16,
	accel_rms_mg:       u16,
	gyro_peak_dps:      i16,
	variance:           u16,
	flags:              u8,
	reserved:           u8,
}

#assert(size_of(GNSS_Sample) == 32)
#assert(size_of(IMU_Summary) == 24)

// ─── Helpers ────────────────────────────────────────────────────────────────

bytes_to_hex :: proc(data: []u8, allocator := context.allocator) -> string {
	hex_chars := [16]u8{'0','1','2','3','4','5','6','7','8','9','a','b','c','d','e','f'}
	buf := make([]u8, len(data) * 2, allocator)
	for i := 0; i < len(data); i += 1 {
		buf[i * 2] = hex_chars[data[i] >> 4]
		buf[i * 2 + 1] = hex_chars[data[i] & 0x0f]
	}
	return string(buf)
}

sha256_hex :: proc(data: []u8) -> string {
	digest := hash.hash_bytes(.SHA256, data)
	defer delete(digest)
	return bytes_to_hex(digest)
}

json_string :: proc(val: json.Value) -> string {
	#partial switch v in val {
	case json.String:
		return v
	}
	return ""
}

json_int :: proc(val: json.Value) -> i64 {
	#partial switch v in val {
	case json.Integer:
		return v
	case json.Float:
		return i64(v)
	}
	return 0
}

json_float :: proc(val: json.Value) -> f64 {
	#partial switch v in val {
	case json.Float:
		return v
	case json.Integer:
		return f64(v)
	}
	return 0
}

join_path :: proc(dir: string, file: string) -> string {
	return filepath.join({dir, file}) or_else ""
}

format_duration :: proc(ms: u64) -> string {
	total_s := ms / 1000
	hours := total_s / 3600
	mins := (total_s % 3600) / 60
	secs := total_s % 60
	return fmt.tprintf("%dh %02dm %02ds", hours, mins, secs)
}

format_duration_secs :: proc(secs: f64) -> string {
	total_s := u64(secs)
	hours := total_s / 3600
	mins := (total_s % 3600) / 60
	s := total_s % 60
	return fmt.tprintf("%dh %02dm %02ds", hours, mins, s)
}

haversine :: proc(lat1, lon1, lat2, lon2: f64) -> f64 {
	to_rad :: proc(deg: f64) -> f64 { return deg * math.PI / 180.0 }

	dlat := to_rad(lat2 - lat1)
	dlon := to_rad(lon2 - lon1)
	rlat1 := to_rad(lat1)
	rlat2 := to_rad(lat2)

	a := math.sin(dlat / 2) * math.sin(dlat / 2) +
	     math.cos(rlat1) * math.cos(rlat2) *
	     math.sin(dlon / 2) * math.sin(dlon / 2)
	c := 2.0 * math.atan2(math.sqrt(a), math.sqrt(1.0 - a))
	return 6371000.0 * c
}

fix_quality_name :: proc(q: u8) -> string {
	switch q {
	case 0: return "None"
	case 1: return "GPS"
	case 2: return "DGPS"
	case 4: return "RTK"
	}
	return fmt.tprintf("Unknown(%d)", q)
}

// ─── Section printers ───────────────────────────────────────────────────────

HEADER_WIDTH :: 64

print_banner :: proc() {
	fmt.println()
	for _ in 0 ..< HEADER_WIDTH { fmt.print("=") }
	fmt.println()
	pad := (HEADER_WIDTH - 14) / 2
	for _ in 0 ..< pad { fmt.print(" ") }
	fmt.println("TRIP INSPECTOR")
	for _ in 0 ..< HEADER_WIDTH { fmt.print("=") }
	fmt.println()
}

print_section :: proc(title: string) {
	fmt.println()
	fmt.printf("-- %s ", title)
	used := 4 + len(title)
	for _ in used ..< HEADER_WIDTH { fmt.print("-") }
	fmt.println()
}

print_row :: proc(label: string, value: string) {
	fmt.printf("  %-22s %s\n", label, value)
}

// ─── Main ───────────────────────────────────────────────────────────────────

main :: proc() {
	args := os.args
	if len(args) < 2 {
		fmt.eprintln("Usage: trip-inspector <bundle-dir>")
		os.exit(1)
	}

	bundle_dir := args[1]

	// ── Read all files ──────────────────────────────────────────────

	manifest_path := join_path(bundle_dir, "manifest.json")
	manifest_data, manifest_err := os.read_entire_file(manifest_path, context.allocator)
	if manifest_err != nil {
		fmt.eprintfln("Error: cannot read %s: %v", manifest_path, manifest_err)
		os.exit(1)
	}

	samples_path := join_path(bundle_dir, "samples.bin")
	samples_data, samples_err := os.read_entire_file(samples_path, context.allocator)
	if samples_err != nil {
		fmt.eprintfln("Error: cannot read %s: %v", samples_path, samples_err)
		os.exit(1)
	}

	imu_path := join_path(bundle_dir, "imu_summary.bin")
	imu_data, imu_err := os.read_entire_file(imu_path, context.allocator)
	if imu_err != nil {
		fmt.eprintfln("Error: cannot read %s: %v", imu_path, imu_err)
		os.exit(1)
	}

	events_path := join_path(bundle_dir, "events.json")
	events_data, events_err := os.read_entire_file(events_path, context.allocator)
	if events_err != nil {
		fmt.eprintfln("Error: cannot read %s: %v", events_path, events_err)
		os.exit(1)
	}

	sums_path := join_path(bundle_dir, "sha256sums.txt")
	sums_data, sums_err := os.read_entire_file(sums_path, context.allocator)
	if sums_err != nil {
		fmt.eprintfln("Error: cannot read %s: %v", sums_path, sums_err)
		os.exit(1)
	}

	// ── Parse manifest ──────────────────────────────────────────────

	manifest_val, manifest_parse_err := json.parse(manifest_data)
	if manifest_parse_err != .None {
		fmt.eprintfln("Error: invalid manifest.json: %v", manifest_parse_err)
		os.exit(1)
	}

	root := manifest_val.(json.Object)
	trip_id := json_string(root["trip_id"])
	device_id := json_string(root["device_id"])
	firmware := json_string(root["firmware_version"])
	started_at := json_string(root["started_at"])
	ended_at := json_string(root["ended_at"])
	started_at_ms := u64(json_int(root["started_at_ms"]))
	ended_at_ms := u64(json_int(root["ended_at_ms"]))
	gnss_rate_hz := json_int(root["gnss_rate_hz"])
	imu_rate_hz := json_int(root["imu_rate_hz"])
	schema_version := json_int(root["schema_version"])
	compression := json_string(root["compression"])

	sample_counts := root["sample_count"].(json.Object)
	gnss_count := json_int(sample_counts["gnss"])
	imu_summary_count := json_int(sample_counts["imu_summary"])
	imu_raw_windows := json_int(sample_counts["imu_raw_windows"])

	duration_ms := ended_at_ms - started_at_ms

	// ── Print manifest ──────────────────────────────────────────────

	print_banner()
	print_section("Manifest")
	print_row("Trip ID", trip_id)
	print_row("Device", device_id)
	print_row("Firmware", firmware)
	print_row("Started", started_at)
	print_row("Ended", ended_at)
	print_row("Duration", format_duration(duration_ms))
	print_row("GNSS Samples", fmt.tprintf("%d  (%d Hz)", gnss_count, gnss_rate_hz))
	print_row("IMU Summaries", fmt.tprintf("%d  (%d Hz)", imu_summary_count, imu_rate_hz))
	print_row("IMU Raw Windows", fmt.tprintf("%d", imu_raw_windows))
	print_row("Schema Version", fmt.tprintf("%d", schema_version))
	print_row("Compression", compression)

	// ── Integrity checks ────────────────────────────────────────────

	print_section("Integrity")

	// Parse sha256sums.txt: lines of "hash  filename"
	Expected_Sum :: struct {
		hash:     string,
		filename: string,
	}
	expected_sums: [dynamic]Expected_Sum
	sums_str := string(sums_data)
	for line in strings.split_lines(sums_str) {
		trimmed := strings.trim_space(line)
		if len(trimmed) == 0 { continue }
		// Format: "hash  filename" (two spaces)
		parts := strings.split_n(trimmed, "  ", 2)
		if len(parts) == 2 {
			append(&expected_sums, Expected_Sum{hash = parts[0], filename = parts[1]})
		}
	}

	// Verify each file
	File_Check :: struct {
		name:  string,
		data:  []u8,
	}
	checks := []File_Check{
		{"samples.bin", samples_data},
		{"imu_summary.bin", imu_data},
		{"events.json", events_data},
	}

	for check in checks {
		actual_hex := sha256_hex(check.data)
		found := false
		matched := false
		for es in expected_sums {
			if es.filename == check.name {
				found = true
				matched = (actual_hex == es.hash)
				break
			}
		}
		if !found {
			print_row(check.name, "SKIP  (not in sha256sums.txt)")
		} else if matched {
			print_row(check.name, fmt.tprintf("OK    %s...", actual_hex[:12]))
		} else {
			print_row(check.name, fmt.tprintf("FAIL  computed %s...", actual_hex[:12]))
		}
	}

	// ── Parse binary records ────────────────────────────────────────

	gnss_samples := mem.slice_data_cast([]GNSS_Sample, samples_data)
	imu_summaries := mem.slice_data_cast([]IMU_Summary, imu_data)

	// ── GNSS Statistics ─────────────────────────────────────────────

	print_section("GNSS Statistics")

	if len(gnss_samples) == 0 {
		print_row("Total Samples", "0 (no data)")
	} else {
		total_gnss := len(gnss_samples)
		print_row("Total Samples", fmt.tprintf("%d", total_gnss))

		first_ts := gnss_samples[0].timestamp_ms
		last_ts := gnss_samples[total_gnss - 1].timestamp_ms
		gnss_duration_ms := last_ts - first_ts
		print_row("Duration", format_duration(gnss_duration_ms))

		// Speed stats (convert cm/s to km/h)
		min_speed_cmps: u16 = max(u16)
		max_speed_cmps: u16 = 0
		sum_speed: f64 = 0

		// Altitude stats
		min_alt: i32 = max(i32)
		max_alt: i32 = min(i32)
		sum_alt: f64 = 0

		// Satellite stats
		min_sats: u8 = max(u8)
		max_sats: u8 = 0
		sum_sats: f64 = 0

		// HDOP stats
		min_hdop: u16 = max(u16)
		max_hdop: u16 = 0
		sum_hdop: f64 = 0

		// Accuracy stats
		min_acc: u16 = max(u16)
		max_acc: u16 = 0
		sum_acc: f64 = 0

		// Fix quality counts
		fix_counts: [256]int

		// Total distance
		total_distance: f64 = 0

		for i := 0; i < total_gnss; i += 1 {
			s := gnss_samples[i]

			if s.speed_cmps < min_speed_cmps { min_speed_cmps = s.speed_cmps }
			if s.speed_cmps > max_speed_cmps { max_speed_cmps = s.speed_cmps }
			sum_speed += f64(s.speed_cmps)

			if s.altitude_cm < min_alt { min_alt = s.altitude_cm }
			if s.altitude_cm > max_alt { max_alt = s.altitude_cm }
			sum_alt += f64(s.altitude_cm)

			if s.satellites < min_sats { min_sats = s.satellites }
			if s.satellites > max_sats { max_sats = s.satellites }
			sum_sats += f64(s.satellites)

			if s.hdop_tenths < min_hdop { min_hdop = s.hdop_tenths }
			if s.hdop_tenths > max_hdop { max_hdop = s.hdop_tenths }
			sum_hdop += f64(s.hdop_tenths)

			if s.accuracy_cm < min_acc { min_acc = s.accuracy_cm }
			if s.accuracy_cm > max_acc { max_acc = s.accuracy_cm }
			sum_acc += f64(s.accuracy_cm)

			fix_counts[s.fix_quality] += 1

			// Haversine distance to previous sample
			if i > 0 {
				prev := gnss_samples[i - 1]
				lat1 := f64(prev.latitude) / 1e7
				lon1 := f64(prev.longitude) / 1e7
				lat2 := f64(s.latitude) / 1e7
				lon2 := f64(s.longitude) / 1e7
				total_distance += haversine(lat1, lon1, lat2, lon2)
			}
		}

		n := f64(total_gnss)
		cmps_to_kmh :: proc(v: f64) -> f64 { return v * 0.036 }

		print_row("Speed (km/h)",
			fmt.tprintf("min: %.1f  avg: %.1f  max: %.1f",
				cmps_to_kmh(f64(min_speed_cmps)),
				cmps_to_kmh(sum_speed / n),
				cmps_to_kmh(f64(max_speed_cmps))))

		print_row("Altitude (m)",
			fmt.tprintf("min: %.1f  avg: %.1f  max: %.1f",
				f64(min_alt) / 100.0,
				(sum_alt / n) / 100.0,
				f64(max_alt) / 100.0))

		print_row("Total Distance",
			fmt.tprintf("%.2f km  (%.0f m, Haversine)",
				total_distance / 1000.0, total_distance))

		// Fix quality breakdown
		fix_labels := [?]struct{q: u8, name: string}{
			{0, "None"}, {1, "GPS"}, {2, "DGPS"}, {4, "RTK"},
		}
		parts: [dynamic]string
		for fl in fix_labels {
			c := fix_counts[fl.q]
			if c > 0 {
				append(&parts, fmt.tprintf("%s: %d", fl.name, c))
			}
		}
		print_row("Fix Quality", strings.join(parts[:], "  "))

		print_row("Satellites",
			fmt.tprintf("min: %d  avg: %.1f  max: %d",
				min_sats, sum_sats / n, max_sats))

		print_row("HDOP",
			fmt.tprintf("min: %.1f  avg: %.1f  max: %.1f",
				f64(min_hdop) / 10.0,
				(sum_hdop / n) / 10.0,
				f64(max_hdop) / 10.0))

		print_row("Accuracy (m)",
			fmt.tprintf("min: %.2f  avg: %.2f  max: %.2f",
				f64(min_acc) / 100.0,
				(sum_acc / n) / 100.0,
				f64(max_acc) / 100.0))

		// ── GNSS Quality Flags ──────────────────────────────────────

		print_section("GNSS Quality Flags")

		gap_count := 0
		speed_over_200 := 0
		alt_jump_count := 0
		fix_zero_count := fix_counts[0]
		hdop_over_5 := 0

		for i := 0; i < total_gnss; i += 1 {
			s := gnss_samples[i]

			// HDOP > 5.0 (stored as tenths)
			if s.hdop_tenths > 50 {
				hdop_over_5 += 1
			}

			// Speed > 200 km/h = 200/0.036 = 5555.6 cm/s
			if s.speed_cmps > 5556 {
				speed_over_200 += 1
			}

			if i > 0 {
				prev := gnss_samples[i - 1]

				// Gap > 5s
				dt := s.timestamp_ms - prev.timestamp_ms
				if dt > 5000 {
					gap_count += 1
				}

				// Altitude jump > 100m (10000 cm)
				alt_diff := i32(s.altitude_cm) - i32(prev.altitude_cm)
				if alt_diff < 0 { alt_diff = -alt_diff }
				if alt_diff > 10000 {
					alt_jump_count += 1
				}
			}
		}

		flag_status :: proc(count: int) -> string {
			if count == 0 { return "none" }
			return fmt.tprintf("%d", count)
		}

		print_row("Gaps > 5s", flag_status(gap_count))
		print_row("Speed > 200 km/h", flag_status(speed_over_200))
		print_row("Alt jumps > 100m", flag_status(alt_jump_count))
		print_row("Fix quality 0", flag_status(fix_zero_count))
		print_row("HDOP > 5.0", flag_status(hdop_over_5))
	}

	// ── IMU Statistics ──────────────────────────────────────────────

	print_section("IMU Statistics")

	total_imu := len(imu_summaries)
	print_row("Total Summaries", fmt.tprintf("%d", total_imu))

	if total_imu > 0 {
		impact_count := 0
		hard_brake_count := 0
		sharp_turn_count := 0
		peak_accel: u16 = 0

		for i := 0; i < total_imu; i += 1 {
			rec := imu_summaries[i]

			if rec.flags & 0x01 != 0 { impact_count += 1 }
			if rec.flags & 0x02 != 0 { hard_brake_count += 1 }
			if rec.flags & 0x04 != 0 { sharp_turn_count += 1 }

			if rec.accel_rms_mg > peak_accel { peak_accel = rec.accel_rms_mg }
		}

		print_row("Impacts", fmt.tprintf("%d", impact_count))
		print_row("Hard Brakes", fmt.tprintf("%d", hard_brake_count))
		print_row("Sharp Turns", fmt.tprintf("%d", sharp_turn_count))
		print_row("Peak Accel (RMS)", fmt.tprintf("%d mg", peak_accel))
	}

	// ── Events ──────────────────────────────────────────────────────

	print_section("Events")

	events_val, events_parse_err := json.parse(events_data)
	if events_parse_err != .None {
		print_row("Error", fmt.tprintf("invalid events.json: %v", events_parse_err))
	} else {
		events_arr := events_val.(json.Array)
		if len(events_arr) == 0 {
			print_row("(none)", "")
		} else {
			fmt.printf("  %-28s %-18s %-26s %s\n", "Timestamp", "Type", "Location", "Details")
			for ev in events_arr {
				obj := ev.(json.Object)
				ts := json_string(obj["timestamp"])
				etype := json_string(obj["type"])
				data_obj := obj["data"].(json.Object)
				details := json_string(data_obj["details"])
				lat := json_float(data_obj["lat"])
				lon := json_float(data_obj["lon"])

				// Truncate timestamp for display (remove timezone offset for brevity)
				ts_display := ts
				if len(ts) > 23 {
					ts_display = ts[:23]
				}

				loc_str: string
				if lat == 0.0 && lon == 0.0 {
					loc_str = "(no location)"
				} else {
					loc_str = fmt.tprintf("(%.4f, %.4f)", lat, lon)
				}

				fmt.printf("  %-28s %-18s %-26s %s\n", ts_display, etype, loc_str, details)
			}
			print_row("Total Events", fmt.tprintf("%d", len(events_arr)))
		}
	}

	// ── Sample Timing ───────────────────────────────────────────────

	print_section("Sample Timing")

	if len(gnss_samples) > 0 {
		first_ts := gnss_samples[0].timestamp_ms
		last_ts := gnss_samples[len(gnss_samples) - 1].timestamp_ms
		actual_count := len(gnss_samples)
		gnss_span_s := f64(last_ts - first_ts) / 1000.0

		expected_count: i64 = 0
		if gnss_rate_hz > 0 {
			expected_count = i64(gnss_span_s) * gnss_rate_hz + 1
		}

		effective_rate: f64 = 0
		if gnss_span_s > 0 {
			effective_rate = f64(actual_count - 1) / gnss_span_s
		}

		print_row("First Timestamp", fmt.tprintf("%d ms  (%s)", first_ts, started_at))
		print_row("Last Timestamp", fmt.tprintf("%d ms  (%s)", last_ts, ended_at))
		print_row("Span", format_duration(last_ts - first_ts))
		print_row("Expected Samples", fmt.tprintf("~%d  (at %d Hz)", expected_count, gnss_rate_hz))
		print_row("Actual Samples", fmt.tprintf("%d", actual_count))
		print_row("Effective Rate", fmt.tprintf("%.2f Hz", effective_rate))
	} else {
		print_row("(no GNSS samples)", "")
	}

	fmt.println()
}
