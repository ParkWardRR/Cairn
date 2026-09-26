const std = @import("std");
const Allocator = std.mem.Allocator;

pub const SampleCount = struct {
    gnss: u32 = 0,
    imu_summary: u32 = 0,
    imu_raw_windows: u32 = 0,
};

pub const Integrity = struct {
    sha256_samples: []const u8 = "",
    sha256_events: []const u8 = "",
};

/// Trip manifest -- serialised as JSON (CBOR planned for v2).
pub const Manifest = struct {
    version: u32 = 1,
    trip_id: []const u8 = "",
    device_id: []const u8 = "cairn-device-01",
    firmware_version: []const u8 = "0.1.0",
    started_at: []const u8 = "",
    ended_at: []const u8 = "",
    sample_count: SampleCount = .{},
    gnss_rate_hz: u32 = 1,
    imu_rate_hz: u32 = 25,
    schema_version: u32 = 1,
    compression: []const u8 = "none",
    integrity: Integrity = .{},

    /// Serialise to pretty-printed JSON.
    pub fn toJson(self: Manifest, allocator: Allocator) ![]u8 {
        return std.json.Stringify.valueAlloc(allocator, self, .{
            .whitespace = .indent_2,
        });
    }

    /// Deserialise from JSON bytes.
    pub fn fromJson(allocator: Allocator, bytes: []const u8) !std.json.Parsed(Manifest) {
        return std.json.parseFromSlice(Manifest, allocator, bytes, .{
            .ignore_unknown_fields = true,
        });
    }
};

// -- tests --

test "Manifest JSON round-trip" {
    const allocator = std.testing.allocator;

    const m = Manifest{
        .trip_id = "TEST0000000000000000000000",
        .device_id = "cairn-test-01",
        .started_at = "2026-09-25T18:30:00Z",
        .ended_at = "2026-09-25T19:00:00Z",
        .sample_count = .{ .gnss = 1800, .imu_summary = 45, .imu_raw_windows = 0 },
        .integrity = .{
            .sha256_samples = "abcd1234",
            .sha256_events = "efgh5678",
        },
    };

    const json_str = try m.toJson(allocator);
    defer allocator.free(json_str);

    const parsed = try Manifest.fromJson(allocator, json_str);
    defer parsed.deinit();

    try std.testing.expectEqualStrings("TEST0000000000000000000000", parsed.value.trip_id);
    try std.testing.expectEqual(@as(u32, 1800), parsed.value.sample_count.gnss);
    try std.testing.expectEqualStrings("abcd1234", parsed.value.integrity.sha256_samples);
}
