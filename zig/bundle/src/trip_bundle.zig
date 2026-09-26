const std = @import("std");
const common = @import("common");
const reader_mod = @import("reader.zig");
const writer_mod = @import("writer.zig");
const hasher = @import("hasher.zig");

const GnssSample = common.GnssSample;
const Manifest = common.Manifest;
const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = Io.Dir;

pub const TripBundle = struct {
    allocator: Allocator,
    manifest: Manifest,
    gnss_samples: []GnssSample,
    events_json: []const u8,
    /// Parsed manifest lifetime holder -- null when manifest was built in-memory.
    _parsed_manifest: ?std.json.Parsed(Manifest) = null,
    /// Raw manifest JSON bytes -- kept alive because parsed strings reference them.
    _manifest_bytes: ?[]const u8 = null,

    pub fn deinit(self: *TripBundle) void {
        if (self.gnss_samples.len > 0)
            self.allocator.free(self.gnss_samples);
        if (self.events_json.len > 0)
            self.allocator.free(@constCast(self.events_json));
        if (self._parsed_manifest) |*p| p.deinit();
        if (self._manifest_bytes) |b| self.allocator.free(b);
    }

    // -- load --

    /// Load a trip bundle from a directory on disk.
    pub fn loadFromDir(allocator: Allocator, io: Io, path: []const u8) !TripBundle {
        var dir = try Dir.cwd().openDir(io, path, .{});
        defer dir.close(io);

        // Read manifest (bytes must stay alive for parsed string slices)
        const manifest_bytes = try dir.readFileAlloc(io, "manifest.json", allocator, .unlimited);

        const parsed = try Manifest.fromJson(allocator, manifest_bytes);

        // Read GNSS samples
        const samples = reader_mod.readGnssSamples(allocator, io, dir, "samples.bin") catch |err| blk: {
            if (err == error.FileNotFound) break :blk @as([]GnssSample, &.{});
            return err;
        };

        // Read events
        const events = dir.readFileAlloc(io, "events.json", allocator, .unlimited) catch |err| blk: {
            if (err == error.FileNotFound) break :blk @as([]u8, &.{});
            return err;
        };

        return TripBundle{
            .allocator = allocator,
            .manifest = parsed.value,
            .gnss_samples = samples,
            .events_json = events,
            ._parsed_manifest = parsed,
            ._manifest_bytes = manifest_bytes,
        };
    }

    // -- save --

    /// Write the complete bundle to a directory, generating sha256sums.txt.
    pub fn saveToDir(self: *const TripBundle, io: Io, path: []const u8) !void {
        // Make dir (and parents)
        Dir.cwd().createDirPath(io, path) catch {};
        var dir = try Dir.cwd().openDir(io, path, .{});
        defer dir.close(io);

        // Write samples.bin
        try writer_mod.writeGnssSamples(io, dir, "samples.bin", self.gnss_samples);

        // Write events.json
        if (self.events_json.len > 0) {
            try dir.writeFile(io, .{ .sub_path = "events.json", .data = self.events_json });
        } else {
            try dir.writeFile(io, .{ .sub_path = "events.json", .data = "{\"events\":[]}" });
        }

        // Compute hashes of samples.bin and events.json
        const samples_hex = try hasher.sha256FileHex(self.allocator, io, dir, "samples.bin");
        const events_hex = try hasher.sha256FileHex(self.allocator, io, dir, "events.json");

        // Build manifest with updated integrity
        var m = self.manifest;
        m.integrity.sha256_samples = &samples_hex;
        m.integrity.sha256_events = &events_hex;

        // Write manifest.json
        const manifest_json = try m.toJson(self.allocator);
        defer self.allocator.free(manifest_json);
        try dir.writeFile(io, .{ .sub_path = "manifest.json", .data = manifest_json });

        // Compute manifest hash
        const manifest_hex = try hasher.sha256FileHex(self.allocator, io, dir, "manifest.json");

        // Build and write sha256sums.txt
        const sums_data = try std.fmt.allocPrint(self.allocator,
            "{s}  manifest.json\n{s}  samples.bin\n{s}  events.json\n",
            .{ manifest_hex, samples_hex, events_hex });
        defer self.allocator.free(sums_data);

        try dir.writeFile(io, .{ .sub_path = "sha256sums.txt", .data = sums_data });
    }

    // -- helpers --

    /// Total trip distance in metres, summing haversine between consecutive samples.
    pub fn totalDistanceMetres(self: *const TripBundle) f64 {
        if (self.gnss_samples.len < 2) return 0;
        var total: f64 = 0;
        for (1..self.gnss_samples.len) |i| {
            const a = self.gnss_samples[i - 1];
            const b = self.gnss_samples[i];
            total += common.haversineMetres(
                a.latDegrees(),
                a.lonDegrees(),
                b.latDegrees(),
                b.lonDegrees(),
            );
        }
        return total;
    }

    /// Trip duration in seconds from first to last sample.
    pub fn durationSecs(self: *const TripBundle) f64 {
        if (self.gnss_samples.len < 2) return 0;
        const first = self.gnss_samples[0].timestamp_ms;
        const last = self.gnss_samples[self.gnss_samples.len - 1].timestamp_ms;
        return @as(f64, @floatFromInt(last - first)) / 1000.0;
    }
};
