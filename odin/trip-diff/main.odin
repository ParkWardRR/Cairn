package trip_diff

import "core:encoding/json"
import "core:fmt"
import "core:math"
import "core:mem"
import "core:os"
import "core:strings"

// ── Constants ──────────────────────────────────────────────────────

GNSS_RECORD_SIZE  :: 32
IMU_RECORD_SIZE   :: 24
EARTH_RADIUS_M    :: 6_371_000.0
MINOR_THRESHOLD_M :: 10.0
MAJOR_THRESHOLD_M :: 100.0
TIME_THRESHOLD_MS :: 1000 // 1 second

// ── Types ──────────────────────────────────────────────────────────

GNSS_Sample :: struct #packed {
	timestamp_ms: u64le,
	latitude:     i32le,
	longitude:    i32le,
	altitude_cm:  i32le,
	speed_cmps:   u16le,
	heading_cdeg: u16le,
	fix_quality:  u8,
	satellites:   u8,
	hdop_tenths:  u16le,
	accuracy_cm:  u16le,
	_reserved:    [2]u8,
}
#assert(size_of(GNSS_Sample) == GNSS_RECORD_SIZE)

Bundle :: struct {
	path:       string,
	manifest:   json.Value,
	samples:    []GNSS_Sample,
	gnss_count: int,
	imu_count:  int,
	hashes:     map[string]string,
}

// ── Haversine ──────────────────────────────────────────────────────

haversine :: proc(lat1_e7, lon1_e7, lat2_e7, lon2_e7: i32) -> f64 {
	K := math.PI / 180.0 / 1e7
	lat1 := f64(lat1_e7) * K
	lon1 := f64(lon1_e7) * K
	lat2 := f64(lat2_e7) * K
	lon2 := f64(lon2_e7) * K

	dlat := lat2 - lat1
	dlon := lon2 - lon1

	a := math.sin(dlat / 2.0) * math.sin(dlat / 2.0) +
		math.cos(lat1) * math.cos(lat2) *
		math.sin(dlon / 2.0) * math.sin(dlon / 2.0)
	c := 2.0 * math.atan2(math.sqrt(a), math.sqrt(1.0 - a))
	return EARTH_RADIUS_M * c
}

// ── Bundle Loading ─────────────────────────────────────────────────

join_path :: proc(dir, file: string) -> string {
	return strings.concatenate({dir, "/", file})
}

load_bundle :: proc(dir: string) -> (bundle: Bundle, ok: bool) {
	bundle.path = dir

	// manifest.json
	{
		path := join_path(dir, "manifest.json")
		data, read_err := os.read_entire_file(path, context.allocator)
		if read_err != nil {
			fmt.eprintfln("error: cannot read %s", path)
			return
		}
		val, json_err := json.parse(data, .JSON, true)
		if json_err != .None {
			fmt.eprintfln("error: invalid JSON in %s", path)
			return
		}
		bundle.manifest = val
	}

	// samples.bin
	{
		path := join_path(dir, "samples.bin")
		data, read_err := os.read_entire_file(path, context.allocator)
		if read_err != nil {
			fmt.eprintfln("error: cannot read %s", path)
			return
		}
		bundle.gnss_count = len(data) / GNSS_RECORD_SIZE
		if len(data) % GNSS_RECORD_SIZE != 0 {
			fmt.eprintfln("warning: %s size is not a multiple of %d", path, GNSS_RECORD_SIZE)
		}
		if bundle.gnss_count > 0 {
			bundle.samples = mem.slice_ptr(
				cast(^GNSS_Sample)raw_data(data),
				bundle.gnss_count,
			)
		}
	}

	// imu_summary.bin
	{
		path := join_path(dir, "imu_summary.bin")
		data, read_err := os.read_entire_file(path, context.allocator)
		if read_err != nil {
			fmt.eprintfln("error: cannot read %s", path)
			return
		}
		bundle.imu_count = len(data) / IMU_RECORD_SIZE
	}

	// sha256sums.txt
	{
		path := join_path(dir, "sha256sums.txt")
		data, read_err := os.read_entire_file(path, context.allocator)
		if read_err != nil {
			fmt.eprintfln("error: cannot read %s", path)
			return
		}
		bundle.hashes = make(map[string]string)
		content := string(data)
		for line in strings.split(content, "\n") {
			t := strings.trim_space(line)
			if len(t) == 0 do continue
			// Format: <hash>  <filename>
			idx := strings.index(t, "  ")
			if idx > 0 {
				hash := t[:idx]
				fname := strings.trim_space(t[idx + 2:])
				bundle.hashes[fname] = hash
			}
		}
	}

	ok = true
	return
}

// ── JSON Helpers ───────────────────────────────────────────────────

json_eq :: proc(a, b: json.Value) -> bool {
	switch av in a {
	case json.Null:
		_, is_null := b.(json.Null)
		return is_null
	case json.Boolean:
		bv, bv_ok := b.(json.Boolean)
		return bv_ok && av == bv
	case json.Integer:
		bv, bv_ok := b.(json.Integer)
		if bv_ok do return av == bv
		// Also compare with float in case of mixed types
		fv, fv_ok := b.(json.Float)
		return fv_ok && f64(av) == fv
	case json.Float:
		bv, bv_ok := b.(json.Float)
		if bv_ok do return av == bv
		iv, iv_ok := b.(json.Integer)
		return iv_ok && av == f64(iv)
	case json.String:
		bv, bv_ok := b.(json.String)
		return bv_ok && av == bv
	case json.Array:
		bv, bv_ok := b.(json.Array)
		if !bv_ok || len(av) != len(bv) do return false
		for i in 0 ..< len(av) {
			if !json_eq(av[i], bv[i]) do return false
		}
		return true
	case json.Object:
		bv, bv_ok := b.(json.Object)
		if !bv_ok || len(av) != len(bv) do return false
		for key, val in av {
			bval, has := bv[key]
			if !has || !json_eq(val, bval) do return false
		}
		return true
	}
	return false
}

// Print a json.Value directly to stdout (no intermediate string).
print_val :: proc(val: json.Value) {
	switch v in val {
	case json.Null:
		fmt.print("null")
	case json.Boolean:
		if v {
			fmt.print("true")
		} else {
			fmt.print("false")
		}
	case json.Integer:
		fmt.printf("%d", v)
	case json.Float:
		fmt.printf("%g", v)
	case json.String:
		fmt.print(v)
	case json.Array:
		fmt.printf("[%d items]", len(v))
	case json.Object:
		fmt.print("{")
		i := 0
		for key, elem in v {
			if i > 0 do fmt.print(", ")
			fmt.printf("%s: ", key)
			print_val(elem)
			i += 1
		}
		fmt.print("}")
	}
}

// ── Comparison: Metadata ───────────────────────────────────────────

// Ordered list of manifest keys for deterministic output.
MANIFEST_KEYS := [?]string{
	"trip_id",
	"device_id",
	"firmware_version",
	"schema_version",
	"version",
	"compression",
	"gnss_rate_hz",
	"imu_rate_hz",
	"started_at",
	"ended_at",
	"started_at_ms",
	"ended_at_ms",
	"gnss_sample_count",
	"imu_summary_count",
	"sample_count",
}

compare_metadata :: proc(a, b: ^Bundle) {
	fmt.println("\n--- Metadata ---")

	a_obj, a_ok := a.manifest.(json.Object)
	b_obj, b_ok := b.manifest.(json.Object)
	if !a_ok || !b_ok {
		fmt.eprintln("  error: manifest is not a JSON object")
		return
	}

	diff_count := 0

	// Track which keys we've printed
	printed: map[string]bool
	defer delete(printed)

	// Print known keys in order
	for key in MANIFEST_KEYS {
		a_val, a_has := a_obj[key]
		b_val, b_has := b_obj[key]
		if !a_has && !b_has do continue
		printed[key] = true
		diff_count += print_meta_field(key, a_val, b_val, a_has, b_has)
	}

	// Print any remaining keys not in the ordered list
	for key in a_obj {
		if key in printed do continue
		printed[key] = true
		b_val, b_has := b_obj[key]
		diff_count += print_meta_field(key, a_obj[key], b_val, true, b_has)
	}
	for key in b_obj {
		if key in printed do continue
		printed[key] = true
		diff_count += print_meta_field(key, json.Value{}, b_obj[key], false, true)
	}

	if diff_count == 0 {
		fmt.println("  All metadata fields match.")
	} else {
		fmt.printf("  %d field(s) differ.\n", diff_count)
	}
}

// Returns 1 if different, 0 if same.
print_meta_field :: proc(key: string, a_val, b_val: json.Value, a_has, b_has: bool) -> int {
	fmt.printf("  %-24s", key)

	if a_has && b_has {
		if json_eq(a_val, b_val) {
			fmt.print(" =  ")
			print_val(a_val)
			fmt.println()
			return 0
		} else {
			fmt.print(" ≠  A: ")
			print_val(a_val)
			fmt.println()
			fmt.printf("  %-24s     B: ", "")
			print_val(b_val)
			fmt.println()
			return 1
		}
	} else if a_has {
		fmt.print("    A only: ")
		print_val(a_val)
		fmt.println()
		return 1
	} else {
		fmt.print("    B only: ")
		print_val(b_val)
		fmt.println()
		return 1
	}
}

// ── Comparison: Sample Counts ──────────────────────────────────────

compare_sample_counts :: proc(a, b: ^Bundle) {
	fmt.println("\n--- Sample Counts ---")

	gnss_marker := "="
	if a.gnss_count != b.gnss_count do gnss_marker = "≠"
	imu_marker := "="
	if a.imu_count != b.imu_count do imu_marker = "≠"

	fmt.printf("  GNSS samples     %d  %s  %d\n", a.gnss_count, gnss_marker, b.gnss_count)
	fmt.printf("  IMU summaries    %d  %s  %d\n", a.imu_count, imu_marker, b.imu_count)

	if a.gnss_count != b.gnss_count {
		diff := a.gnss_count - b.gnss_count
		if diff > 0 {
			fmt.printf("  A has %d more GNSS samples than B\n", diff)
		} else {
			fmt.printf("  B has %d more GNSS samples than A\n", -diff)
		}
	}
	if a.imu_count != b.imu_count {
		diff := a.imu_count - b.imu_count
		if diff > 0 {
			fmt.printf("  A has %d more IMU summaries than B\n", diff)
		} else {
			fmt.printf("  B has %d more IMU summaries than A\n", -diff)
		}
	}
}

// ── Comparison: GNSS (Route, Timing, Speed) ────────────────────────

compare_gnss :: proc(a, b: ^Bundle) {
	n := min(a.gnss_count, b.gnss_count)

	// ── Route Divergence ───────────────────────────────────────
	fmt.println("\n--- Route Divergence ---")

	if n == 0 {
		fmt.println("  No GNSS samples to compare.")
		fmt.println("\n--- Timing ---")
		fmt.println("  No GNSS samples to compare.")
		fmt.println("\n--- Speed ---")
		fmt.println("  No GNSS samples to compare.")
		return
	}

	minor_count := 0
	major_count := 0
	max_dist: f64 = 0
	max_dist_idx := 0
	sum_dist: f64 = 0

	time_diff_count := 0
	max_time_diff: u64 = 0
	max_time_idx := 0

	max_speed_diff: f64 = 0
	sum_speed_diff: f64 = 0

	is_prefix := true

	for i in 0 ..< n {
		sa := a.samples[i]
		sb := b.samples[i]

		// Route divergence (haversine)
		dist := haversine(
			i32(sa.latitude), i32(sa.longitude),
			i32(sb.latitude), i32(sb.longitude),
		)
		sum_dist += dist
		if dist > max_dist {
			max_dist = dist
			max_dist_idx = i
		}
		if dist > MAJOR_THRESHOLD_M {
			major_count += 1
		} else if dist > MINOR_THRESHOLD_M {
			minor_count += 1
		}

		// Timing
		ts_a := u64(sa.timestamp_ms)
		ts_b := u64(sb.timestamp_ms)
		td: u64
		if ts_a > ts_b {
			td = ts_a - ts_b
		} else {
			td = ts_b - ts_a
		}
		if td > u64(TIME_THRESHOLD_MS) {
			time_diff_count += 1
		}
		if td > max_time_diff {
			max_time_diff = td
			max_time_idx = i
		}

		// Speed
		sp_a := f64(u16(sa.speed_cmps))
		sp_b := f64(u16(sb.speed_cmps))
		sd := math.abs(sp_a - sp_b)
		sum_speed_diff += sd
		if sd > max_speed_diff {
			max_speed_diff = sd
		}

		// Prefix tracking: all fields must match for prefix
		if is_prefix {
			if sa.timestamp_ms != sb.timestamp_ms ||
				sa.latitude != sb.latitude ||
				sa.longitude != sb.longitude ||
				sa.altitude_cm != sb.altitude_cm ||
				sa.speed_cmps != sb.speed_cmps ||
				sa.heading_cdeg != sb.heading_cdeg ||
				sa.fix_quality != sb.fix_quality ||
				sa.satellites != sb.satellites ||
				sa.hdop_tenths != sb.hdop_tenths ||
				sa.accuracy_cm != sb.accuracy_cm {
				is_prefix = false
			}
		}
	}

	diverge_count := minor_count + major_count
	mean_dist := sum_dist / f64(n)

	fmt.printf("  %d of %d samples diverge (%d major, %d minor)\n",
		diverge_count, n, major_count, minor_count)
	fmt.printf("  Max divergence: %.3f m at sample #%d\n", max_dist, max_dist_idx)
	fmt.printf("  Mean divergence: %.3f m\n", mean_dist)

	// ── Timing ─────────────────────────────────────────────────
	fmt.println("\n--- Timing ---")
	fmt.printf("  %d of %d samples have timestamp difference > 1s\n",
		time_diff_count, n)
	fmt.printf("  Max time delta: %d ms", max_time_diff)
	if max_time_diff > 0 {
		fmt.printf(" at sample #%d", max_time_idx)
	}
	fmt.println()

	// ── Speed ──────────────────────────────────────────────────
	fmt.println("\n--- Speed ---")
	mean_speed := sum_speed_diff / f64(n)
	fmt.printf("  Max speed difference: %.2f cm/s\n", max_speed_diff)
	fmt.printf("  Mean speed difference: %.2f cm/s\n", mean_speed)

	// ── Prefix Detection (if counts differ) ────────────────────
	if a.gnss_count != b.gnss_count {
		fmt.println("\n--- Sample Count Mismatch ---")
		shorter_label := "A"
		longer_label := "B"
		shorter_n := a.gnss_count
		longer_n := b.gnss_count
		if a.gnss_count > b.gnss_count {
			shorter_label = "B"
			longer_label = "A"
			shorter_n = b.gnss_count
			longer_n = a.gnss_count
		}
		if is_prefix {
			fmt.printf("  %s (%d samples) is a prefix of %s (%d samples)\n",
				shorter_label, shorter_n, longer_label, longer_n)
		} else {
			fmt.printf("  %s has %d samples, %s has %d samples (not a prefix)\n",
				shorter_label, shorter_n, longer_label, longer_n)
		}
	}
}

// ── Comparison: Hashes ─────────────────────────────────────────────

compare_hashes :: proc(a, b: ^Bundle) {
	fmt.println("\n--- Hash Comparison ---")

	// Collect filenames from both bundles
	files: [dynamic]string
	defer delete(files)
	seen: map[string]bool
	defer delete(seen)

	for fname in a.hashes {
		if !(fname in seen) {
			append(&files, fname)
			seen[fname] = true
		}
	}
	for fname in b.hashes {
		if !(fname in seen) {
			append(&files, fname)
			seen[fname] = true
		}
	}

	all_match := true
	for fname in files {
		a_hash, a_has := a.hashes[fname]
		b_hash, b_has := b.hashes[fname]

		if a_has && b_has {
			if a_hash == b_hash {
				fmt.printf("  %-20s =  (match)\n", fname)
			} else {
				all_match = false
				fmt.printf("  %-20s ≠  A: %.16s...\n", fname, a_hash)
				fmt.printf("  %-20s     B: %.16s...\n", "", b_hash)
			}
		} else if a_has {
			all_match = false
			fmt.printf("  %-20s    A only\n", fname)
		} else {
			all_match = false
			fmt.printf("  %-20s    B only\n", fname)
		}
	}

	if all_match && len(files) > 0 {
		fmt.println("  All hashes match.")
	}
}

// ── Main ───────────────────────────────────────────────────────────

main :: proc() {
	args := os.args
	if len(args) != 3 {
		prog := "trip-diff"
		if len(args) > 0 do prog = args[0]
		fmt.eprintfln("usage: %s <bundle-dir-a> <bundle-dir-b>", prog)
		os.exit(1)
	}

	dir_a := args[1]
	dir_b := args[2]

	a, a_ok := load_bundle(dir_a)
	if !a_ok do os.exit(1)

	b, b_ok := load_bundle(dir_b)
	if !b_ok do os.exit(1)

	fmt.println("=== Trip Bundle Diff ===")
	fmt.printf("  A: %s\n", a.path)
	fmt.printf("  B: %s\n", b.path)

	compare_metadata(&a, &b)
	compare_sample_counts(&a, &b)
	compare_gnss(&a, &b)
	compare_hashes(&a, &b)

	fmt.println()
}
