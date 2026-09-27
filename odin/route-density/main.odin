package route_density

import "core:fmt"
import "core:math"
import "core:mem"
import "core:os"
import "core:strings"
import "core:slice"
import "core:strconv"
import "core:path/filepath"

// 32-byte GNSS record, little-endian, packed
GNSS_Record :: struct #packed {
	timestamp_ms: u64le,
	latitude:     i32le, // deg * 10^7
	longitude:    i32le, // deg * 10^7
	altitude_cm:  i32le,
	speed_cmps:   u16le,
	heading_cdeg: u16le,
	fix_quality:  u8,
	satellites:   u8,
	hdop_tenths:  u16le,
	accuracy_cm:  u16le,
	reserved:     [2]u8,
}

#assert(size_of(GNSS_Record) == 32)

Color_Scheme :: enum {
	Heat,
	Blue,
}

Config :: struct {
	output:       string,
	width:        int,
	height:       int,
	cell_size:    f64, // meters
	color_scheme: Color_Scheme,
	bundle_dirs:  [dynamic]string,
}

main :: proc() {
	cfg := parse_args()
	defer delete(cfg.bundle_dirs)

	if len(cfg.bundle_dirs) == 0 {
		fmt.eprintln("Usage: route-density <bundle-dir> [bundle-dir...] [flags]")
		fmt.eprintln("Flags:")
		fmt.eprintln("  --output <file.svg>       Output SVG path (default: route-density.svg)")
		fmt.eprintln("  --width <px>              SVG width (default: 1200)")
		fmt.eprintln("  --height <px>             SVG height (default: 900)")
		fmt.eprintln("  --cell-size <meters>      Grid cell size (default: 50)")
		fmt.eprintln("  --color-scheme <name>     \"heat\" or \"blue\" (default: heat)")
		os.exit(1)
	}

	// Read all GNSS samples from all bundles
	samples := make([dynamic]GNSS_Record)
	defer delete(samples)

	trip_count := 0
	for dir in cfg.bundle_dirs {
		samples_path, _ := filepath.join({dir, "samples.bin"})
		defer delete(samples_path)

		data, read_err := os.read_entire_file(samples_path, context.allocator)
		if read_err != nil {
			fmt.eprintfln("Error: cannot read %s", samples_path)
			os.exit(1)
		}
		defer delete(data)

		record_count := len(data) / size_of(GNSS_Record)
		if record_count == 0 {
			fmt.eprintfln("Warning: no records in %s", samples_path)
			continue
		}

		records := mem.slice_ptr(cast(^GNSS_Record)raw_data(data), record_count)
		for &r in records {
			append(&samples, r)
		}
		trip_count += 1
	}

	if len(samples) == 0 {
		fmt.eprintln("Error: no GNSS samples found in any bundle")
		os.exit(1)
	}

	fmt.printfln("Loaded %d samples from %d trip(s)", len(samples), trip_count)

	// Compute bounding box
	min_lat := max(f64)
	max_lat := min(f64)
	min_lon := max(f64)
	max_lon := min(f64)

	for &s in samples {
		lat := f64(s.latitude) / 1e7
		lon := f64(s.longitude) / 1e7
		min_lat = math.min(min_lat, lat)
		max_lat = math.max(max_lat, lat)
		min_lon = math.min(min_lon, lon)
		max_lon = math.max(max_lon, lon)
	}

	// Add 5% padding
	lat_range := max_lat - min_lat
	lon_range := max_lon - min_lon
	pad_lat := lat_range * 0.05
	pad_lon := lon_range * 0.05

	// Handle edge case: all samples at same point
	if lat_range < 1e-7 {
		pad_lat = 0.001
	}
	if lon_range < 1e-7 {
		pad_lon = 0.001
	}

	min_lat -= pad_lat
	max_lat += pad_lat
	min_lon -= pad_lon
	max_lon += pad_lon
	lat_range = max_lat - min_lat
	lon_range = max_lon - min_lon

	// Coordinate conversion constants
	lat_center := (min_lat + max_lat) / 2.0
	lat_to_meters: f64 = 111320.0
	lon_to_meters: f64 = 111320.0 * math.cos(lat_center * math.PI / 180.0)

	lat_range_meters := lat_range * lat_to_meters
	lon_range_meters := lon_range * lon_to_meters

	grid_cols := int(math.ceil(lon_range_meters / cfg.cell_size))
	grid_rows := int(math.ceil(lat_range_meters / cfg.cell_size))

	// Clamp grid to reasonable limits
	if grid_cols < 1 { grid_cols = 1 }
	if grid_rows < 1 { grid_rows = 1 }
	if grid_cols > 10000 { grid_cols = 10000 }
	if grid_rows > 10000 { grid_rows = 10000 }

	fmt.printfln("Grid: %d x %d cells (%.0fm cell size)", grid_cols, grid_rows, cfg.cell_size)
	fmt.printfln("Bounds: lat [%.6f, %.6f] lon [%.6f, %.6f]", min_lat, max_lat, min_lon, max_lon)

	// Allocate grid and count hits
	grid := make([]u32, grid_cols * grid_rows)
	defer delete(grid)

	for &s in samples {
		lat := f64(s.latitude) / 1e7
		lon := f64(s.longitude) / 1e7

		// Compute grid cell
		col := int((lon - min_lon) / lon_range * f64(grid_cols))
		row := int((max_lat - lat) / lat_range * f64(grid_rows)) // flip Y: top = max lat

		col = clamp(col, 0, grid_cols - 1)
		row = clamp(row, 0, grid_rows - 1)

		grid[row * grid_cols + col] += 1
	}

	// Find max count
	max_count: u32 = 0
	nonzero_cells := 0
	for c in grid {
		if c > max_count { max_count = c }
		if c > 0 { nonzero_cells += 1 }
	}

	fmt.printfln("Non-zero cells: %d, max count: %d", nonzero_cells, max_count)

	// Generate SVG
	svg := generate_svg(
		grid, grid_cols, grid_rows,
		cfg.width, cfg.height,
		max_count, trip_count, len(samples),
		cfg.color_scheme,
	)
	defer delete(svg)

	write_err := os.write_entire_file(cfg.output, transmute([]u8)svg)
	if write_err != nil {
		fmt.eprintfln("Error: cannot write %s", cfg.output)
		os.exit(1)
	}

	fmt.printfln("Wrote %s (%d bytes)", cfg.output, len(svg))
}

generate_svg :: proc(
	grid: []u32,
	grid_cols, grid_rows: int,
	svg_width, svg_height: int,
	max_count: u32,
	trip_count: int,
	sample_count: int,
	scheme: Color_Scheme,
) -> string {
	b := strings.builder_make()

	// Header
	fmt.sbprintfln(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		svg_width, svg_height, svg_width, svg_height,
	)

	// Background
	fmt.sbprintfln(&b, `  <rect width="%d" height="%d" fill="#1a1a2e"/>`, svg_width, svg_height)

	// Compute cell pixel sizes — leave margins for title and scale bar
	margin_top := 40
	margin_bottom := 40
	margin_left := 10
	margin_right := 10
	draw_w := svg_width - margin_left - margin_right
	draw_h := svg_height - margin_top - margin_bottom

	cell_w := f64(draw_w) / f64(grid_cols)
	cell_h := f64(draw_h) / f64(grid_rows)

	// Draw density cells
	for row in 0 ..< grid_rows {
		for col in 0 ..< grid_cols {
			count := grid[row * grid_cols + col]
			if count == 0 { continue }

			t := f64(count) / f64(max_count) // 0..1 normalized density
			opacity := 0.3 + t * 0.7         // 0.3 to 1.0

			r, g, bv := density_color(t, scheme)

			x := f64(margin_left) + f64(col) * cell_w
			y := f64(margin_top) + f64(row) * cell_h

			fmt.sbprintfln(&b,
				`  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="rgb(%d,%d,%d)" opacity="%.2f"/>`,
				x, y,
				math.max(cell_w, 1.0), math.max(cell_h, 1.0),
				r, g, bv,
				opacity,
			)
		}
	}

	// Border
	fmt.sbprintfln(&b,
		`  <rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#444" stroke-width="1"/>`,
		margin_left, margin_top, draw_w, draw_h,
	)

	// Title
	fmt.sbprintfln(&b,
		`  <text x="10" y="25" fill="white" font-family="monospace" font-size="16">Route Density - %d trips, %d samples</text>`,
		trip_count, sample_count,
	)

	// Scale bar (bottom area)
	scale_bar_svg(&b, svg_width, svg_height, margin_bottom, scheme)

	// Close
	fmt.sbprintln(&b, `</svg>`)

	return strings.to_string(b)
}

scale_bar_svg :: proc(b: ^strings.Builder, svg_width, svg_height, margin_bottom: int, scheme: Color_Scheme) {
	bar_y := svg_height - margin_bottom + 10
	bar_x := 10
	bar_w := 200
	bar_h := 12
	steps := 20

	fmt.sbprintfln(b,
		`  <text x="%d" y="%d" fill="#aaa" font-family="monospace" font-size="11">Low</text>`,
		bar_x, bar_y - 2,
	)

	for i in 0 ..< steps {
		t := f64(i) / f64(steps - 1)
		r, g, bv := density_color(t, scheme)
		x := bar_x + i * (bar_w / steps)
		w := bar_w / steps + 1

		fmt.sbprintfln(b,
			`  <rect x="%d" y="%d" width="%d" height="%d" fill="rgb(%d,%d,%d)"/>`,
			x, bar_y, w, bar_h,
			r, g, bv,
		)
	}

	fmt.sbprintfln(b,
		`  <text x="%d" y="%d" fill="#aaa" font-family="monospace" font-size="11">High</text>`,
		bar_x + bar_w + 5, bar_y + bar_h - 1,
	)
}

// Returns (r, g, b) for a density value t in [0, 1]
density_color :: proc(t: f64, scheme: Color_Scheme) -> (u8, u8, u8) {
	switch scheme {
	case .Heat:
		// dark red -> orange -> yellow -> white
		if t < 0.33 {
			s := t / 0.33
			return lerp_u8(139, 255, s), lerp_u8(0, 69, s), 0
		} else if t < 0.66 {
			s := (t - 0.33) / 0.33
			return 255, lerp_u8(69, 215, s), 0
		} else {
			s := (t - 0.66) / 0.34
			return 255, lerp_u8(215, 255, s), lerp_u8(0, 255, s)
		}
	case .Blue:
		// dark blue -> blue -> cyan -> white
		if t < 0.33 {
			s := t / 0.33
			return 0, 0, lerp_u8(128, 255, s)
		} else if t < 0.66 {
			s := (t - 0.33) / 0.33
			return 0, lerp_u8(0, 255, s), 255
		} else {
			s := (t - 0.66) / 0.34
			return lerp_u8(0, 255, s), 255, 255
		}
	}
	return 255, 255, 255
}

lerp_u8 :: proc(a, b: u8, t: f64) -> u8 {
	return u8(f64(a) + (f64(b) - f64(a)) * clamp(t, 0, 1))
}

parse_args :: proc() -> Config {
	cfg := Config{
		output       = "route-density.svg",
		width        = 1200,
		height       = 900,
		cell_size    = 50.0,
		color_scheme = .Heat,
	}

	args := os.args[1:]
	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--output" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --output requires a value")
				os.exit(1)
			}
			cfg.output = args[i]
		} else if arg == "--width" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --width requires a value")
				os.exit(1)
			}
			val, ok := strconv.parse_int(args[i])
			if !ok || val < 1 {
				fmt.eprintfln("Error: invalid width: %s", args[i])
				os.exit(1)
			}
			cfg.width = val
		} else if arg == "--height" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --height requires a value")
				os.exit(1)
			}
			val, ok := strconv.parse_int(args[i])
			if !ok || val < 1 {
				fmt.eprintfln("Error: invalid height: %s", args[i])
				os.exit(1)
			}
			cfg.height = val
		} else if arg == "--cell-size" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --cell-size requires a value")
				os.exit(1)
			}
			val, ok := strconv.parse_f64(args[i])
			if !ok || val <= 0 {
				fmt.eprintfln("Error: invalid cell-size: %s", args[i])
				os.exit(1)
			}
			cfg.cell_size = val
		} else if arg == "--color-scheme" {
			i += 1
			if i >= len(args) {
				fmt.eprintln("Error: --color-scheme requires a value")
				os.exit(1)
			}
			if args[i] == "heat" {
				cfg.color_scheme = .Heat
			} else if args[i] == "blue" {
				cfg.color_scheme = .Blue
			} else {
				fmt.eprintfln("Error: unknown color scheme: %s (use \"heat\" or \"blue\")", args[i])
				os.exit(1)
			}
		} else if strings.has_prefix(arg, "-") {
			fmt.eprintfln("Error: unknown flag: %s", arg)
			os.exit(1)
		} else {
			append(&cfg.bundle_dirs, arg)
		}

		i += 1
	}

	return cfg
}
