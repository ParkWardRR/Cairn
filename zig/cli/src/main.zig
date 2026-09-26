const std = @import("std");
const common = @import("common");
const bundle_lib = @import("bundle");
const GnssSample = common.GnssSample;
const Manifest = common.Manifest;
const TripBundle = bundle_lib.TripBundle;
const hasher = bundle_lib.hasher;
const validator = bundle_lib.validator;

const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = Io.Dir;
const File = Io.File;
const ArgsIter = std.process.Args.Iterator;

pub fn main(init: std.process.Init) !void {
    const allocator = init.gpa;
    const io = init.io;
    var args = ArgsIter.init(init.minimal.args);
    _ = args.next(); // skip argv[0]

    const command = args.next() orelse {
        printUsage(io);
        return;
    };

    if (std.mem.eql(u8, command, "inspect")) {
        const dir = args.next() orelse return fatal(io, "inspect requires <bundle-dir>\n");
        try cmdInspect(allocator, io, dir);
    } else if (std.mem.eql(u8, command, "validate")) {
        const dir = args.next() orelse return fatal(io, "validate requires <bundle-dir>\n");
        try cmdValidate(allocator, io, dir);
    } else if (std.mem.eql(u8, command, "generate")) {
        try cmdGenerate(allocator, io, &args);
    } else if (std.mem.eql(u8, command, "export-gpx")) {
        const dir = args.next() orelse return fatal(io, "export-gpx requires <bundle-dir>\n");
        try cmdExportGpx(allocator, io, dir, &args);
    } else if (std.mem.eql(u8, command, "export-csv")) {
        const dir = args.next() orelse return fatal(io, "export-csv requires <bundle-dir>\n");
        try cmdExportCsv(allocator, io, dir, &args);
    } else {
        printUsage(io);
    }
}

fn fatal(io: Io, msg: []const u8) void {
    var buf: [512]u8 = undefined;
    var fw = File.stderr().writer(io, &buf);
    fw.interface.writeAll(msg) catch {};
    fw.flush() catch {};
}

fn printUsage(io: Io) void {
    var buf: [512]u8 = undefined;
    var fw = File.stdout().writer(io, &buf);
    fw.interface.writeAll(
        \\tripctl -- Cairn trip bundle tool
        \\
        \\Usage:
        \\  tripctl inspect <bundle-dir>
        \\  tripctl validate <bundle-dir>
        \\  tripctl generate --duration-minutes <N> --output <dir>
        \\  tripctl export-gpx <bundle-dir> --output <file.gpx>
        \\  tripctl export-csv <bundle-dir> --output <file.csv>
        \\
    ) catch {};
    fw.flush() catch {};
}

// -- inspect --

fn cmdInspect(allocator: Allocator, io: Io, dir_path: [:0]const u8) !void {
    var tb = try TripBundle.loadFromDir(allocator, io, dir_path);
    defer tb.deinit();

    const m = tb.manifest;
    var buf: [4096]u8 = undefined;
    var fw = File.stdout().writer(io, &buf);
    const w = &fw.interface;

    try w.print("Trip Bundle: {s}\n", .{dir_path});
    try w.print("  Trip ID:      {s}\n", .{m.trip_id});
    try w.print("  Device ID:    {s}\n", .{m.device_id});
    try w.print("  Firmware:     {s}\n", .{m.firmware_version});
    try w.print("  Started:      {s}\n", .{m.started_at});
    try w.print("  Ended:        {s}\n", .{m.ended_at});
    try w.print("  GNSS rate:    {d} Hz\n", .{m.gnss_rate_hz});
    try w.print("  Schema:       v{d}\n", .{m.schema_version});
    try w.print("  Compression:  {s}\n", .{m.compression});
    try w.print("  Samples:\n", .{});
    try w.print("    GNSS:       {d}\n", .{m.sample_count.gnss});
    try w.print("    IMU summ:   {d}\n", .{m.sample_count.imu_summary});
    try w.print("    IMU raw:    {d}\n", .{m.sample_count.imu_raw_windows});

    const dist_m = tb.totalDistanceMetres();
    const dur_s = tb.durationSecs();
    const dur_min = dur_s / 60.0;
    const avg_kmh = if (dur_s > 0) (dist_m / 1000.0) / (dur_s / 3600.0) else 0;

    try w.print("  Duration:     {d:.1} min\n", .{dur_min});
    try w.print("  Distance:     {d:.2} km\n", .{dist_m / 1000.0});
    try w.print("  Avg speed:    {d:.1} km/h\n", .{avg_kmh});

    if (tb.gnss_samples.len > 0) {
        const first = tb.gnss_samples[0];
        const last = tb.gnss_samples[tb.gnss_samples.len - 1];
        try w.print("  Start coord:  ({d:.7}, {d:.7})\n", .{ first.latDegrees(), first.lonDegrees() });
        try w.print("  End coord:    ({d:.7}, {d:.7})\n", .{ last.latDegrees(), last.lonDegrees() });
    }
    try fw.flush();
}

// -- validate --

fn cmdValidate(allocator: Allocator, io: Io, dir_path: [:0]const u8) !void {
    var dir = try Dir.cwd().openDir(io, dir_path, .{});
    defer dir.close(io);

    var result = try validator.validateBundle(allocator, io, dir);
    defer result.deinit(allocator);

    var buf: [4096]u8 = undefined;
    var fw = File.stdout().writer(io, &buf);
    const w = &fw.interface;

    try w.print("Bundle validation: {s}\n\n", .{dir_path});

    for (result.files) |f| {
        const icon: []const u8 = if (f.ok) "OK " else "FAIL";
        try w.print("  [{s}] {s}\n", .{ icon, f.filename });
        if (!f.ok) {
            try w.print("         expected: {s}\n", .{f.expected});
            try w.print("         actual:   {s}\n", .{f.actual});
        }
    }

    for (result.missing) |m| {
        try w.print("  [MISS] {s}\n", .{m});
    }

    if (result.all_ok) {
        try w.writeAll("\nResult: PASS -- all checksums match.\n");
    } else {
        try w.writeAll("\nResult: FAIL -- integrity check failed.\n");
    }
    try fw.flush();
}

// -- generate --

fn cmdGenerate(allocator: Allocator, io: Io, args: *ArgsIter) !void {
    var duration_min: u32 = 30;
    var output_dir: [:0]const u8 = "generated_trip";

    while (args.next()) |arg| {
        if (std.mem.eql(u8, arg, "--duration-minutes")) {
            const val = args.next() orelse return fatal(io, "--duration-minutes requires a value\n");
            duration_min = std.fmt.parseInt(u32, val, 10) catch return fatal(io, "invalid --duration-minutes\n");
        } else if (std.mem.eql(u8, arg, "--output")) {
            output_dir = args.next() orelse return fatal(io, "--output requires a value\n");
        }
    }

    const total_secs: u32 = duration_min * 60;
    const sample_count: u32 = total_secs; // 1 Hz

    // Base timestamp: 2026-09-25T18:30:00Z
    const base_ts: u64 = 1790631000 * 1000; // approximate

    const samples = try allocator.alloc(GnssSample, sample_count);
    defer allocator.free(samples);

    // Waypoints for a route around Santa Monica
    const Wp = struct { frac: f64, lat: f64, lon: f64, speed: f64, alt: f64 };
    const waypoints = [_]Wp{
        .{ .frac = 0.00, .lat = 34.0195, .lon = -118.4912, .speed = 0, .alt = 30 },
        .{ .frac = 0.05, .lat = 34.0198, .lon = -118.4900, .speed = 15, .alt = 31 },
        .{ .frac = 0.12, .lat = 34.0205, .lon = -118.4850, .speed = 45, .alt = 38 },
        .{ .frac = 0.22, .lat = 34.0230, .lon = -118.4780, .speed = 55, .alt = 50 },
        .{ .frac = 0.32, .lat = 34.0280, .lon = -118.4740, .speed = 50, .alt = 65 },
        .{ .frac = 0.40, .lat = 34.0320, .lon = -118.4770, .speed = 40, .alt = 72 },
        .{ .frac = 0.50, .lat = 34.0330, .lon = -118.4840, .speed = 35, .alt = 68 },
        .{ .frac = 0.55, .lat = 34.0310, .lon = -118.4870, .speed = 0, .alt = 55 },
        .{ .frac = 0.62, .lat = 34.0310, .lon = -118.4870, .speed = 0, .alt = 55 },
        .{ .frac = 0.72, .lat = 34.0260, .lon = -118.4900, .speed = 50, .alt = 42 },
        .{ .frac = 0.85, .lat = 34.0220, .lon = -118.4910, .speed = 40, .alt = 35 },
        .{ .frac = 0.95, .lat = 34.0198, .lon = -118.4915, .speed = 12, .alt = 30 },
        .{ .frac = 1.00, .lat = 34.0195, .lon = -118.4915, .speed = 0, .alt = 30 },
    };

    // Simple LCG for deterministic jitter
    var rng_state: u64 = 42;
    const jitter = struct {
        fn next(state: *u64) f64 {
            state.* = state.* *% 6364136223846793005 +% 1442695040888963407;
            const bits: u32 = @truncate(state.* >> 33);
            return @as(f64, @floatFromInt(bits)) / @as(f64, @floatFromInt(@as(u32, std.math.maxInt(u32) >> 1))) - 1.0;
        }
    }.next;

    for (0..sample_count) |i| {
        const frac: f64 = @as(f64, @floatFromInt(i)) / @as(f64, @floatFromInt(sample_count));

        // Find surrounding waypoints
        var wp0: usize = 0;
        var wp1: usize = waypoints.len - 1;
        for (0..waypoints.len - 1) |w_idx| {
            if (frac >= waypoints[w_idx].frac and frac < waypoints[w_idx + 1].frac) {
                wp0 = w_idx;
                wp1 = w_idx + 1;
                break;
            }
        }

        const seg_frac = if (waypoints[wp1].frac > waypoints[wp0].frac)
            (frac - waypoints[wp0].frac) / (waypoints[wp1].frac - waypoints[wp0].frac)
        else
            0.0;

        const lat = waypoints[wp0].lat + (waypoints[wp1].lat - waypoints[wp0].lat) * seg_frac;
        const lon = waypoints[wp0].lon + (waypoints[wp1].lon - waypoints[wp0].lon) * seg_frac;
        const speed_kmh = waypoints[wp0].speed + (waypoints[wp1].speed - waypoints[wp0].speed) * seg_frac;
        const alt = waypoints[wp0].alt + (waypoints[wp1].alt - waypoints[wp0].alt) * seg_frac;

        // Add small GPS jitter
        const j1 = jitter(&rng_state);
        const j2 = jitter(&rng_state);
        const lat_jittered = lat + j1 * 0.000005;
        const lon_jittered = lon + j2 * 0.000005;

        // Compute heading from movement direction
        var heading_deg: f64 = 0;
        if (i > 0 and speed_kmh > 1) {
            const prev = samples[i - 1];
            const dlat = lat_jittered - prev.latDegrees();
            const dlon = lon_jittered - prev.lonDegrees();
            heading_deg = std.math.atan2(dlon, dlat) * 180.0 / std.math.pi;
            if (heading_deg < 0) heading_deg += 360.0;
        }

        // GNSS quality: degrade between frac 0.42 and 0.48
        var sats: u8 = 10;
        var hdop_t: u16 = 12; // 1.2
        var fix: u8 = 1;
        var acc: u16 = 250;

        if (frac >= 0.42 and frac <= 0.48) {
            sats = 3;
            hdop_t = 83;
            fix = 0;
            acc = 2500;
        } else {
            _ = jitter(&rng_state);
            const raw_bits: u32 = @truncate(rng_state >> 16);
            const variation: i32 = @as(i32, @intCast(raw_bits % 5)) - 2;
            const sat_val: i32 = 10 + variation;
            sats = @intCast(std.math.clamp(sat_val, 6, 14));
        }

        const speed_cmps: u16 = @intFromFloat(@max(speed_kmh * 100.0 / 3.6, 0));
        const heading_cdeg: u16 = @intFromFloat(@max(@mod(heading_deg * 100.0, 36000.0), 0));

        samples[i] = GnssSample{
            .timestamp_ms = base_ts + @as(u64, @intCast(i)) * 1000,
            .latitude = @intFromFloat(lat_jittered * 1e7),
            .longitude = @intFromFloat(lon_jittered * 1e7),
            .altitude_cm = @intFromFloat(alt * 100.0),
            .speed_cmps = speed_cmps,
            .heading_cdeg = heading_cdeg,
            .fix_quality = fix,
            .satellites = sats,
            .hdop_tenths = hdop_t,
            .accuracy_cm = acc,
        };
    }

    // Build events JSON
    var start_ts_buf: [20]u8 = undefined;
    var end_ts_buf: [20]u8 = undefined;
    var deg_ts_buf: [20]u8 = undefined;
    var res_ts_buf: [20]u8 = undefined;
    const start_ts_str = common.formatTimestamp(base_ts, &start_ts_buf);
    const end_ts_str = common.formatTimestamp(base_ts + @as(u64, total_secs) * 1000, &end_ts_buf);
    const deg_start = base_ts + @as(u64, @intFromFloat(0.42 * @as(f64, @floatFromInt(total_secs)))) * 1000;
    const deg_ts_str = common.formatTimestamp(deg_start, &deg_ts_buf);
    const res_start = base_ts + @as(u64, @intFromFloat(0.48 * @as(f64, @floatFromInt(total_secs)))) * 1000;
    const res_ts_str = common.formatTimestamp(res_start, &res_ts_buf);
    const deg_dur: u64 = @intFromFloat(0.06 * @as(f64, @floatFromInt(total_secs)));

    const events_json = try std.fmt.allocPrint(allocator,
        \\{{
        \\  "events": [
        \\    {{
        \\      "type": "trip_start",
        \\      "timestamp": "{s}",
        \\      "trigger": "gnss_speed+imu_motion",
        \\      "location": {{ "lat": 34.0195, "lon": -118.4912 }}
        \\    }},
        \\    {{
        \\      "type": "gnss_quality_degraded",
        \\      "timestamp": "{s}",
        \\      "hdop": 8.3,
        \\      "satellites": 3,
        \\      "duration_s": {d}
        \\    }},
        \\    {{
        \\      "type": "gnss_quality_restored",
        \\      "timestamp": "{s}",
        \\      "hdop": 1.2,
        \\      "satellites": 10
        \\    }},
        \\    {{
        \\      "type": "trip_end",
        \\      "timestamp": "{s}",
        \\      "trigger": "dwell_threshold",
        \\      "location": {{ "lat": 34.0195, "lon": -118.4915 }},
        \\      "parking_accuracy_m": 4.2
        \\    }}
        \\  ]
        \\}}
    , .{ start_ts_str, deg_ts_str, deg_dur, res_ts_str, end_ts_str });
    defer allocator.free(events_json);

    // Build manifest
    const m = Manifest{
        .trip_id = "01SYNTH00000000000000000000",
        .device_id = "cairn-synth-01",
        .started_at = start_ts_str,
        .ended_at = end_ts_str,
        .sample_count = .{
            .gnss = sample_count,
            .imu_summary = 0,
            .imu_raw_windows = 0,
        },
    };

    // Create a non-owning TripBundle and save
    var tb = TripBundle{
        .allocator = allocator,
        .manifest = m,
        .gnss_samples = samples,
        .events_json = events_json,
    };

    try tb.saveToDir(io, output_dir);

    var out_buf: [256]u8 = undefined;
    var out_fw = File.stdout().writer(io, &out_buf);
    const out_w = &out_fw.interface;
    try out_w.print("Generated {d}-minute trip with {d} GNSS samples\n", .{ duration_min, sample_count });
    try out_w.print("Output: {s}/\n", .{output_dir});
    try out_fw.flush();

    // Prevent deinit from freeing memory we still own
    tb.gnss_samples = &.{};
    tb.events_json = "";
}

// -- export-gpx --

fn cmdExportGpx(allocator: Allocator, io: Io, dir_path: [:0]const u8, args: *ArgsIter) !void {
    var output: [:0]const u8 = "trip.gpx";
    while (args.next()) |arg| {
        if (std.mem.eql(u8, arg, "--output")) {
            output = args.next() orelse return fatal(io, "--output requires a value\n");
        }
    }

    var tb = try TripBundle.loadFromDir(allocator, io, dir_path);
    defer tb.deinit();

    const file = try Dir.cwd().createFile(io, output, .{});
    defer file.close(io);
    var buf: [4096]u8 = undefined;
    var fw = file.writer(io, &buf);
    const w = &fw.interface;

    try w.writeAll("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n");
    try w.writeAll("<gpx version=\"1.1\" creator=\"tripctl\" xmlns=\"http://www.topografix.com/GPX/1/1\">\n");
    try w.print("  <metadata><name>Trip {s}</name></metadata>\n", .{tb.manifest.trip_id});
    try w.writeAll("  <trk>\n");
    try w.print("    <name>Trip {s}</name>\n", .{tb.manifest.trip_id});
    try w.writeAll("    <trkseg>\n");

    for (tb.gnss_samples) |s| {
        var ts_buf: [20]u8 = undefined;
        const ts_str = common.formatTimestamp(s.timestamp_ms, &ts_buf);
        try w.print("      <trkpt lat=\"{d:.7}\" lon=\"{d:.7}\">\n", .{ s.latDegrees(), s.lonDegrees() });
        try w.print("        <ele>{d:.2}</ele>\n", .{s.altMeters()});
        try w.print("        <time>{s}</time>\n", .{ts_str});
        try w.print("        <hdop>{d:.1}</hdop>\n", .{s.hdop()});
        try w.print("        <sat>{d}</sat>\n", .{s.satellites});
        try w.print("        <speed>{d:.2}</speed>\n", .{s.speedMps()});
        try w.print("        <course>{d:.2}</course>\n", .{s.headingDegrees()});
        try w.writeAll("      </trkpt>\n");
    }

    try w.writeAll("    </trkseg>\n");
    try w.writeAll("  </trk>\n");
    try w.writeAll("</gpx>\n");
    try fw.flush();

    var out_buf: [256]u8 = undefined;
    var out_fw = File.stdout().writer(io, &out_buf);
    try out_fw.interface.print("Exported {d} trackpoints to {s}\n", .{ tb.gnss_samples.len, output });
    try out_fw.flush();
}

// -- export-csv --

fn cmdExportCsv(allocator: Allocator, io: Io, dir_path: [:0]const u8, args: *ArgsIter) !void {
    var output: [:0]const u8 = "trip.csv";
    while (args.next()) |arg| {
        if (std.mem.eql(u8, arg, "--output")) {
            output = args.next() orelse return fatal(io, "--output requires a value\n");
        }
    }

    var tb = try TripBundle.loadFromDir(allocator, io, dir_path);
    defer tb.deinit();

    const file = try Dir.cwd().createFile(io, output, .{});
    defer file.close(io);
    var buf: [4096]u8 = undefined;
    var fw = file.writer(io, &buf);
    const w = &fw.interface;

    try w.writeAll("timestamp_ms,latitude,longitude,altitude_m,speed_kmh,heading_deg,fix_quality,satellites,hdop,accuracy_m\n");

    for (tb.gnss_samples) |s| {
        try w.print("{d},{d:.7},{d:.7},{d:.2},{d:.2},{d:.2},{d},{d},{d:.1},{d:.2}\n", .{
            s.timestamp_ms,
            s.latDegrees(),
            s.lonDegrees(),
            s.altMeters(),
            s.speedKmh(),
            s.headingDegrees(),
            s.fix_quality,
            s.satellites,
            s.hdop(),
            s.accuracyMeters(),
        });
    }
    try fw.flush();

    var out_buf: [256]u8 = undefined;
    var out_fw = File.stdout().writer(io, &out_buf);
    try out_fw.interface.print("Exported {d} samples to {s}\n", .{ tb.gnss_samples.len, output });
    try out_fw.flush();
}
