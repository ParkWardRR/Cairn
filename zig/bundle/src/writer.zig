const std = @import("std");
const common = @import("common");
const GnssSample = common.GnssSample;
const ImuSummary = common.ImuSummary;
const ImuRaw = common.ImuRaw;
const Io = std.Io;
const Dir = Io.Dir;

/// Write GNSS samples as a binary file (fixed-width 32-byte records).
pub fn writeGnssSamples(io: Io, dir: Dir, filename: []const u8, samples: []const GnssSample) !void {
    const file = try dir.createFile(io, filename, .{});
    defer file.close(io);
    for (samples) |*sample| {
        try file.writeStreamingAll(io, std.mem.asBytes(sample));
    }
}

/// Write IMU summary records as a binary file.
pub fn writeImuSummaries(io: Io, dir: Dir, filename: []const u8, summaries: []const ImuSummary) !void {
    const file = try dir.createFile(io, filename, .{});
    defer file.close(io);
    for (summaries) |*s| {
        try file.writeStreamingAll(io, std.mem.asBytes(s));
    }
}

/// Write IMU raw samples as a binary file.
pub fn writeImuRawSamples(io: Io, dir: Dir, filename: []const u8, raw: []const ImuRaw) !void {
    const file = try dir.createFile(io, filename, .{});
    defer file.close(io);
    for (raw) |*r| {
        try file.writeStreamingAll(io, std.mem.asBytes(r));
    }
}
