package trip_replay

import "core:crypto/hash"
import "core:encoding/json"
import "core:flags"
import "core:fmt"
import "core:os"
import "core:strings"
import "core:time"

// ---------------------------------------------------------------------------
// CLI flags
// ---------------------------------------------------------------------------

Options :: struct {
	bundle_dir: string   `args:"pos=0,required" usage:"Path to trip bundle directory."`,
	server:     string   `usage:"Ingest server URL (default http://localhost:8443)."`,
	speedup:    f64      `usage:"Replay speed multiplier. 0 = instant (default 1)."`,
	chunk_size: int      `args:"name=chunk-size" usage:"Upload chunk size in bytes (default 262144)."`,
	dry_run:    bool     `args:"name=dry-run" usage:"Validate bundle without uploading."`,
}

// ---------------------------------------------------------------------------
// Manifest struct (matches manifest.json)
// ---------------------------------------------------------------------------

Sample_Count :: struct {
	gnss:             int   `json:"gnss"`,
	imu_raw_windows:  int   `json:"imu_raw_windows"`,
	imu_summary:      int   `json:"imu_summary"`,
}

Manifest :: struct {
	compression:       string       `json:"compression"`,
	device_id:         string       `json:"device_id"`,
	ended_at:          string       `json:"ended_at"`,
	ended_at_ms:       i64          `json:"ended_at_ms"`,
	firmware_version:  string       `json:"firmware_version"`,
	gnss_rate_hz:      int          `json:"gnss_rate_hz"`,
	gnss_sample_count: int          `json:"gnss_sample_count"`,
	imu_rate_hz:       int          `json:"imu_rate_hz"`,
	imu_summary_count: int          `json:"imu_summary_count"`,
	sample_count:      Sample_Count `json:"sample_count"`,
	schema_version:    int          `json:"schema_version"`,
	started_at:        string       `json:"started_at"`,
	started_at_ms:     i64          `json:"started_at_ms"`,
	trip_id:           string       `json:"trip_id"`,
	version:           int          `json:"version"`,
}

// ---------------------------------------------------------------------------
// Bundle file names in the order they appear in the tar
// ---------------------------------------------------------------------------

BUNDLE_FILES :: [?]string{
	"manifest.json",
	"samples.bin",
	"imu_summary.bin",
	"events.json",
	"sha256sums.txt",
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

main :: proc() {
	opts := Options{
		server     = "http://localhost:8443",
		speedup    = 1.0,
		chunk_size = 262144,
	}
	flags.parse_or_exit(&opts, os.args, .Unix)

	bundle_dir := strings.trim_right(opts.bundle_dir, "/")

	// -- 1. Read and validate bundle --------------------------------------

	manifest_bytes, manifest_err := os.read_entire_file(
		strings.concatenate({bundle_dir, "/manifest.json"}),
		context.allocator,
	)
	if manifest_err != nil {
		fatal("cannot read manifest.json")
	}

	manifest: Manifest
	umerr := json.unmarshal(manifest_bytes, &manifest)
	if umerr != nil {
		fatal("failed to parse manifest.json")
	}

	// Parse expected SHA-256 sums
	sha_bytes, sha_err := os.read_entire_file(
		strings.concatenate({bundle_dir, "/sha256sums.txt"}),
		context.allocator,
	)
	if sha_err != nil {
		fatal("cannot read sha256sums.txt")
	}

	expected_hashes := parse_sha256sums(string(sha_bytes))

	// Validate each file's SHA-256
	integrity_ok := true
	for name in BUNDLE_FILES {
		if name == "sha256sums.txt" do continue

		expected, has_expected := expected_hashes[name]
		if !has_expected {
			fmt.eprintfln("  WARN  no hash for %s in sha256sums.txt", name)
			continue
		}

		file_path := strings.concatenate({bundle_dir, "/", name})
		data, readerr := os.read_entire_file(file_path, context.allocator)
		if readerr != nil {
			fmt.eprintfln("  FAIL  cannot read %s", name)
			integrity_ok = false
			continue
		}

		actual := sha256_hex(data)
		if actual != expected {
			fmt.eprintfln("  FAIL  %s  expected %s  got %s", name, expected, actual)
			integrity_ok = false
		} else {
			fmt.printfln("  OK    %s", name)
		}
	}

	if !integrity_ok {
		fatal("bundle integrity check failed")
	}
	fmt.println("Integrity check passed.")

	// -- 2. Print bundle summary ------------------------------------------

	duration_ms := manifest.ended_at_ms - manifest.started_at_ms
	dur_secs    := f64(duration_ms) / 1000.0
	dur_min     := dur_secs / 60.0

	fmt.println()
	fmt.println("=== Bundle Summary ===")
	fmt.printfln("  Trip ID:         %s", manifest.trip_id)
	fmt.printfln("  Device:          %s", manifest.device_id)
	fmt.printfln("  Firmware:        %s", manifest.firmware_version)
	fmt.printfln("  Started:         %s", manifest.started_at)
	fmt.printfln("  Ended:           %s", manifest.ended_at)
	fmt.printfln("  Duration:        %.1f min (%.0f s)", dur_min, dur_secs)
	fmt.printfln("  GNSS samples:    %d  (%d Hz)", manifest.gnss_sample_count, manifest.gnss_rate_hz)
	fmt.printfln("  IMU summaries:   %d  (%d Hz)", manifest.imu_summary_count, manifest.imu_rate_hz)
	fmt.println()

	if opts.dry_run {
		fmt.println("Dry-run mode -- skipping upload.")
		return
	}

	// -- 3. Build tar archive ---------------------------------------------

	fmt.println("Building tar archive...")

	tar_buf: [dynamic]byte

	for name in BUNDLE_FILES {
		file_path := strings.concatenate({bundle_dir, "/", name})
		data, readerr := os.read_entire_file(file_path, context.allocator)
		if readerr != nil {
			fatal(fmt.tprintf("cannot read %s for tar", name))
		}
		tar_append(&tar_buf, name, data)
	}

	// Two 512-byte zero blocks as end-of-archive marker
	for _ in 0 ..< 1024 {
		append(&tar_buf, byte(0))
	}

	tar_data := tar_buf[:]
	content_hash := sha256_hex(tar_data)

	fmt.printfln("Archive size:   %d bytes", len(tar_data))
	fmt.printfln("Content hash:   %s", content_hash)

	// -- 4. Init upload ---------------------------------------------------

	init_body := fmt.tprintf(
		`{{"device_id":"%s","content_hash":"%s"}}`,
		manifest.device_id,
		content_hash,
	)

	init_url := strings.concatenate({opts.server, "/api/v1/upload/init"})
	init_resp, init_ok := curl_post_json(init_url, init_body)
	if !init_ok {
		fatal("upload init failed")
	}

	upload_id := extract_json_string(init_resp, "upload_id")
	if upload_id == "" {
		fatal(fmt.tprintf("no upload_id in init response: %s", init_resp))
	}
	fmt.printfln("Upload ID:      %s", upload_id)

	// -- 5. Send chunks ---------------------------------------------------

	total      := len(tar_data)
	chunk_size := opts.chunk_size
	sent       := 0
	chunk_num  := 0
	num_chunks := (total + chunk_size - 1) / chunk_size

	for sent < total {
		end := min(sent + chunk_size, total)
		chunk := tar_data[sent:end]

		chunk_url := fmt.tprintf(
			"%s/api/v1/upload/%s/chunk",
			opts.server,
			upload_id,
		)
		offset_hdr := fmt.tprintf("X-Upload-Offset: %d", sent)

		chunk_resp, chunk_ok := curl_put_raw(chunk_url, chunk, offset_hdr)
		if !chunk_ok {
			fatal(fmt.tprintf("chunk upload failed at offset %d", sent))
		}

		chunk_num += 1
		pct := int(f64(chunk_num) / f64(num_chunks) * 100.0)
		fmt.printf("\r  Uploading: [%d/%d] %d%%", chunk_num, num_chunks, pct)

		sent = end

		// Simulate timing if speedup > 0
		if opts.speedup > 0 && sent < total {
			// Distribute the trip duration across all chunks
			total_dur_ns := i64(dur_secs * 1e9)
			per_chunk_ns := total_dur_ns / i64(num_chunks)
			sleep_ns     := i64(f64(per_chunk_ns) / opts.speedup)
			if sleep_ns > 0 {
				time.sleep(time.Duration(sleep_ns))
			}
		}

		_ = chunk_resp
	}

	fmt.println()
	fmt.println("All chunks sent.")

	// -- 6. Finalize upload -----------------------------------------------

	manifest_json_str := string(manifest_bytes)

	finalize_body := fmt.tprintf(
		`{{"content_hash":"%s","manifest":%s}}`,
		content_hash,
		manifest_json_str,
	)

	finalize_url := strings.concatenate({
		opts.server,
		"/api/v1/upload/",
		upload_id,
		"/finalize",
	})
	finalize_resp, finalize_ok := curl_post_json(finalize_url, finalize_body)
	if !finalize_ok {
		fatal("finalize request failed")
	}

	fmt.println()
	fmt.println("=== Receipt ===")
	fmt.println(finalize_resp)
}

// ---------------------------------------------------------------------------
// SHA-256 helpers
// ---------------------------------------------------------------------------

sha256_hex :: proc(data: []byte) -> string {
	digest: [32]byte
	ctx: hash.Context
	hash.init(&ctx, .SHA256)
	hash.update(&ctx, data)
	hash.final(&ctx, digest[:])
	return hex_encode(digest[:])
}

hex_encode :: proc(data: []byte) -> string {
	hex_chars := "0123456789abcdef"
	buf := make([]byte, len(data) * 2)
	for b, i in data {
		buf[i * 2]     = hex_chars[b >> 4]
		buf[i * 2 + 1] = hex_chars[b & 0x0f]
	}
	return string(buf)
}

// ---------------------------------------------------------------------------
// sha256sums.txt parser   (format: "<hex>  <filename>\n")
// ---------------------------------------------------------------------------

parse_sha256sums :: proc(text: string) -> map[string]string {
	result: map[string]string
	for line in strings.split_lines(text) {
		trimmed := strings.trim_space(line)
		if len(trimmed) == 0 do continue
		// Format: 64 hex chars, two spaces, filename
		if len(trimmed) < 66 do continue
		hex_part  := trimmed[:64]
		name_part := strings.trim_space(trimmed[64:])
		result[name_part] = hex_part
	}
	return result
}

// ---------------------------------------------------------------------------
// Minimal POSIX tar writer
// ---------------------------------------------------------------------------

tar_append :: proc(buf: ^[dynamic]byte, name: string, data: []byte) {
	header: [512]byte

	// Name field (0..99)
	copy(header[:100], transmute([]byte)name)

	// Mode (100..107) -- 0644
	copy(header[100:108], transmute([]byte)string("0000644\x00"))

	// UID (108..115)
	copy(header[108:116], transmute([]byte)string("0001000\x00"))

	// GID (116..123)
	copy(header[116:124], transmute([]byte)string("0001000\x00"))

	// Size (124..135) -- octal, 11 digits + NUL
	write_octal(header[124:136], len(data))

	// Mtime (136..147) -- use 0
	copy(header[136:148], transmute([]byte)string("00000000000\x00"))

	// Typeflag (156) -- '0' = regular file
	header[156] = '0'

	// Magic (257..262) -- "ustar\x00"
	copy(header[257:263], transmute([]byte)string("ustar\x00"))

	// Version (263..264) -- "00"
	copy(header[263:265], transmute([]byte)string("00"))

	// Checksum placeholder: spaces
	for i in 148 ..< 156 {
		header[i] = ' '
	}

	// Compute checksum (sum of all bytes with checksum field as spaces)
	cksum := 0
	for b in header {
		cksum += int(b)
	}
	write_octal(header[148:156], cksum)
	// NUL terminate + space (the standard says 6 octal digits, NUL, space)
	header[154] = 0
	header[155] = ' '

	// Append header
	append(buf, ..header[:])

	// Append file data
	append(buf, ..data)

	// Pad to 512-byte boundary
	remainder := len(data) % 512
	if remainder != 0 {
		padding := 512 - remainder
		for _ in 0 ..< padding {
			append(buf, byte(0))
		}
	}
}

write_octal :: proc(dst: []byte, value: int) {
	n := len(dst) - 1  // last byte is NUL
	v := value
	for i := n - 1; i >= 0; i -= 1 {
		dst[i] = byte('0' + (v & 7))
		v >>= 3
	}
	dst[n] = 0
}

// ---------------------------------------------------------------------------
// curl-based HTTP helpers
// ---------------------------------------------------------------------------

curl_post_json :: proc(url: string, body: string) -> (response: string, ok: bool) {
	state, stdout, stderr, err := os.process_exec({
		command = {
			"curl", "-s", "-S", "--fail-with-body",
			"-X", "POST",
			"-H", "Content-Type: application/json",
			"-d", body,
			url,
		},
	}, context.allocator)

	if err != nil {
		fmt.eprintfln("curl error: %v", err)
		return "", false
	}
	if !state.success || state.exit_code != 0 {
		fmt.eprintfln("curl failed (exit %d): %s", state.exit_code, string(stderr))
		return "", false
	}
	return string(stdout), true
}

curl_put_raw :: proc(url: string, data: []byte, extra_header: string) -> (response: string, ok: bool) {
	// Write data to a temp file since we cannot pipe stdin easily
	tmp_path := "/tmp/cairn-trip-replay-chunk.bin"
	write_err := os.write_entire_file(tmp_path, data)
	if write_err != nil {
		fmt.eprintln("failed to write temp chunk file")
		return "", false
	}
	defer os.remove(tmp_path)

	data_arg := fmt.tprintf("@%s", tmp_path)
	state, stdout, stderr, err := os.process_exec({
		command = {
			"curl", "-s", "-S", "--fail-with-body",
			"-X", "PUT",
			"-H", "Content-Type: application/octet-stream",
			"-H", extra_header,
			"--data-binary", data_arg,
			url,
		},
	}, context.allocator)

	if err != nil {
		fmt.eprintfln("curl error: %v", err)
		return "", false
	}
	if !state.success || state.exit_code != 0 {
		fmt.eprintfln("curl failed (exit %d): %s", state.exit_code, string(stderr))
		return "", false
	}
	return string(stdout), true
}

// ---------------------------------------------------------------------------
// Quick JSON string field extractor (avoids full parse for simple responses)
// ---------------------------------------------------------------------------

extract_json_string :: proc(json_str: string, key: string) -> string {
	// Look for "key":"value"
	search := fmt.tprintf(`"%s":"`, key)
	idx := strings.index(json_str, search)
	if idx < 0 {
		// Try with space after colon: "key": "value"
		search = fmt.tprintf(`"%s": "`, key)
		idx = strings.index(json_str, search)
		if idx < 0 {
			return ""
		}
	}
	start := idx + len(search)
	end := strings.index(json_str[start:], `"`)
	if end < 0 {
		return ""
	}
	return json_str[start : start + end]
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

fatal :: proc(msg: string) -> ! {
	fmt.eprintfln("FATAL: %s", msg)
	os.exit(1)
}
