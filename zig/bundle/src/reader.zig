const std = @import("std");
const common = @import("common");
const GnssSample = common.GnssSample;
const ImuSummary = common.ImuSummary;
const ImuRaw = common.ImuRaw;
const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = Io.Dir;

/// Read a binary samples file containing fixed-width GNSS records.
pub fn readGnssSamples(allocator: Allocator, io: Io, dir: Dir, filename: []const u8) ![]GnssSample {
    const bytes = try dir.readFileAlloc(io, filename, allocator, .unlimited);
    defer allocator.free(bytes);

    if (bytes.len % GnssSample.SIZE != 0)
        return error.InvalidRecordAlignment;

    const count = bytes.len / GnssSample.SIZE;
    const samples = try allocator.alloc(GnssSample, count);
    errdefer allocator.free(samples);

    for (0..count) |i| {
        const offset = i * GnssSample.SIZE;
        samples[i] = std.mem.bytesToValue(GnssSample, bytes[offset..][0..GnssSample.SIZE]);
    }
    return samples;
}

/// Read binary IMU summary records.
pub fn readImuSummaries(allocator: Allocator, io: Io, dir: Dir, filename: []const u8) ![]ImuSummary {
    const bytes = try dir.readFileAlloc(io, filename, allocator, .unlimited);
    defer allocator.free(bytes);

    if (bytes.len % ImuSummary.SIZE != 0)
        return error.InvalidRecordAlignment;

    const count = bytes.len / ImuSummary.SIZE;
    const summaries = try allocator.alloc(ImuSummary, count);
    errdefer allocator.free(summaries);

    for (0..count) |i| {
        const offset = i * ImuSummary.SIZE;
        summaries[i] = std.mem.bytesToValue(ImuSummary, bytes[offset..][0..ImuSummary.SIZE]);
    }
    return summaries;
}

/// Read binary IMU raw samples.
pub fn readImuRawSamples(allocator: Allocator, io: Io, dir: Dir, filename: []const u8) ![]ImuRaw {
    const bytes = try dir.readFileAlloc(io, filename, allocator, .unlimited);
    defer allocator.free(bytes);

    if (bytes.len % ImuRaw.SIZE != 0)
        return error.InvalidRecordAlignment;

    const count = bytes.len / ImuRaw.SIZE;
    const raw = try allocator.alloc(ImuRaw, count);
    errdefer allocator.free(raw);

    for (0..count) |i| {
        const offset = i * ImuRaw.SIZE;
        raw[i] = std.mem.bytesToValue(ImuRaw, bytes[offset..][0..ImuRaw.SIZE]);
    }
    return raw;
}

/// Read the entire file into a heap buffer.
pub fn readFileAlloc(allocator: Allocator, io: Io, dir: Dir, filename: []const u8) ![]u8 {
    return dir.readFileAlloc(io, filename, allocator, .unlimited);
}
