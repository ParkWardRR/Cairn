const std = @import("std");

/// GNSS sample record — 32 bytes, little-endian, fixed-width.
/// At 1 Hz a 30-minute trip produces ~57 KB of GNSS data.
pub const GnssSample = extern struct {
    /// Milliseconds since Unix epoch
    timestamp_ms: u64,
    /// Degrees * 10^7 (e.g. 340195000 = 34.0195000)
    latitude: i32,
    /// Degrees * 10^7
    longitude: i32,
    /// Centimetres above WGS-84 ellipsoid
    altitude_cm: i32,
    /// Ground speed in cm/s
    speed_cmps: u16,
    /// Heading in centidegrees (0–35999)
    heading_cdeg: u16,
    /// 0=none, 1=GPS, 2=DGPS, 4=RTK
    fix_quality: u8,
    /// Visible satellite count
    satellites: u8,
    /// HDOP * 10
    hdop_tenths: u16,
    /// Estimated horizontal accuracy in cm
    accuracy_cm: u16,
    /// Reserved, zero-filled
    _reserved: [2]u8 = .{ 0, 0 },

    pub const SIZE: usize = 32;

    comptime {
        if (@sizeOf(GnssSample) != SIZE)
            @compileError("GnssSample must be exactly 32 bytes");
    }

    // ── convenience accessors ──

    pub fn latDegrees(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.latitude)) / 1e7;
    }

    pub fn lonDegrees(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.longitude)) / 1e7;
    }

    pub fn altMeters(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.altitude_cm)) / 100.0;
    }

    pub fn speedMps(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.speed_cmps)) / 100.0;
    }

    pub fn speedKmh(self: GnssSample) f64 {
        return self.speedMps() * 3.6;
    }

    pub fn headingDegrees(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.heading_cdeg)) / 100.0;
    }

    pub fn hdop(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.hdop_tenths)) / 10.0;
    }

    pub fn accuracyMeters(self: GnssSample) f64 {
        return @as(f64, @floatFromInt(self.accuracy_cm)) / 100.0;
    }
};

/// IMU summary record — 24 bytes per rolling window.
pub const ImuSummary = extern struct {
    /// Window start in ms since Unix epoch
    window_start_ms: u64,
    /// Window duration in ms
    window_duration_ms: u16,
    /// Peak acceleration, milli-g
    accel_peak_x_mg: i16,
    accel_peak_y_mg: i16,
    accel_peak_z_mg: i16,
    /// RMS acceleration magnitude, milli-g
    accel_rms_mg: u16,
    /// Peak gyro rate, degrees/sec * 10
    gyro_peak_dps: i16,
    /// Motion-intensity metric
    variance: u16,
    /// Bit flags: 0x01 impact, 0x02 hard_brake, 0x04 sharp_turn
    flags: u8,
    _reserved: u8 = 0,

    pub const SIZE: usize = 24;

    comptime {
        if (@sizeOf(ImuSummary) != SIZE)
            @compileError("ImuSummary must be exactly 24 bytes");
    }

    pub const FLAG_IMPACT: u8 = 0x01;
    pub const FLAG_HARD_BRAKE: u8 = 0x02;
    pub const FLAG_SHARP_TURN: u8 = 0x04;

    pub fn hasImpact(self: ImuSummary) bool {
        return (self.flags & FLAG_IMPACT) != 0;
    }
    pub fn hasHardBrake(self: ImuSummary) bool {
        return (self.flags & FLAG_HARD_BRAKE) != 0;
    }
    pub fn hasSharpTurn(self: ImuSummary) bool {
        return (self.flags & FLAG_SHARP_TURN) != 0;
    }
};

/// IMU raw sample — 20 bytes, full-rate burst around events.
pub const ImuRaw = extern struct {
    timestamp_ms: u64 align(2),
    accel_x_mg: i16,
    accel_y_mg: i16,
    accel_z_mg: i16,
    gyro_x_dps10: i16,
    gyro_y_dps10: i16,
    gyro_z_dps10: i16,

    pub const SIZE: usize = 20;

    comptime {
        if (@sizeOf(ImuRaw) != SIZE)
            @compileError("ImuRaw must be exactly 20 bytes");
    }
};

// ── Haversine helper (used by inspect / generate) ──

/// Returns distance in metres between two WGS-84 points.
pub fn haversineMetres(lat1_deg: f64, lon1_deg: f64, lat2_deg: f64, lon2_deg: f64) f64 {
    const to_rad = std.math.pi / 180.0;
    const dlat = (lat2_deg - lat1_deg) * to_rad;
    const dlon = (lon2_deg - lon1_deg) * to_rad;
    const a = std.math.sin(dlat / 2.0) * std.math.sin(dlat / 2.0) +
        std.math.cos(lat1_deg * to_rad) * std.math.cos(lat2_deg * to_rad) *
        std.math.sin(dlon / 2.0) * std.math.sin(dlon / 2.0);
    const c = 2.0 * std.math.atan2(@sqrt(a), @sqrt(1.0 - a));
    return 6_371_000.0 * c;
}

/// Format a Unix-ms timestamp as ISO-8601 UTC into the supplied buffer.
pub fn formatTimestamp(timestamp_ms: u64, buf: *[20]u8) []const u8 {
    const secs: u64 = timestamp_ms / 1000;
    const es = std.time.epoch.EpochSeconds{ .secs = secs };
    const ed = es.getEpochDay();
    const yd = ed.calculateYearDay();
    const md = yd.calculateMonthDay();
    const ds = es.getDaySeconds();
    _ = std.fmt.bufPrint(buf, "{d:0>4}-{d:0>2}-{d:0>2}T{d:0>2}:{d:0>2}:{d:0>2}Z", .{
        yd.year,
        md.month.numeric(),
        @as(u32, md.day_index) + 1,
        ds.getHoursIntoDay(),
        ds.getMinutesIntoHour(),
        ds.getSecondsIntoMinute(),
    }) catch unreachable;
    return buf[0..20];
}

// ── tests ──

test "GnssSample is 32 bytes" {
    try std.testing.expectEqual(@as(usize, 32), @sizeOf(GnssSample));
}

test "ImuSummary is 24 bytes" {
    try std.testing.expectEqual(@as(usize, 24), @sizeOf(ImuSummary));
}

test "ImuRaw is 20 bytes" {
    try std.testing.expectEqual(@as(usize, 20), @sizeOf(ImuRaw));
}

test "GnssSample serialisation round-trip" {
    const sample = GnssSample{
        .timestamp_ms = 1695667800000,
        .latitude = 340195000,
        .longitude = -1184912000,
        .altitude_cm = 3000,
        .speed_cmps = 1389,
        .heading_cdeg = 9000,
        .fix_quality = 1,
        .satellites = 10,
        .hdop_tenths = 12,
        .accuracy_cm = 250,
    };

    const bytes = std.mem.asBytes(&sample);
    try std.testing.expectEqual(@as(usize, 32), bytes.len);

    const restored = std.mem.bytesToValue(GnssSample, bytes);
    try std.testing.expectEqual(sample.timestamp_ms, restored.timestamp_ms);
    try std.testing.expectEqual(sample.latitude, restored.latitude);
    try std.testing.expectEqual(sample.longitude, restored.longitude);
    try std.testing.expectEqual(sample.speed_cmps, restored.speed_cmps);
    try std.testing.expectEqual(sample.satellites, restored.satellites);
}

test "haversine known distance" {
    // Approximate distance from Santa Monica Pier to Venice Beach ~3.5 km
    const d = haversineMetres(34.0095, -118.4970, 33.9850, -118.4695);
    try std.testing.expect(d > 3000.0 and d < 4500.0);
}

test "formatTimestamp" {
    // 2024-09-25T18:30:00Z in ms
    const ts: u64 = 1727289000000;
    var buf: [20]u8 = undefined;
    const s = formatTimestamp(ts, &buf);
    try std.testing.expectEqualStrings("2024-09-25T18:30:00Z", s);
}
