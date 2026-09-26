const std = @import("std");
const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = Io.Dir;
const hasher = @import("hasher.zig");

pub const FileResult = struct {
    filename: []const u8,
    expected: [hasher.HEX_LEN]u8,
    actual: [hasher.HEX_LEN]u8,
    ok: bool,
};

pub const ValidationResult = struct {
    files: []FileResult,
    all_ok: bool,
    missing: []const []const u8,
    /// Raw sha256sums.txt content -- kept alive because filename slices reference it.
    _sums_content: ?[]const u8 = null,

    pub fn deinit(self: *ValidationResult, allocator: Allocator) void {
        allocator.free(self.files);
        allocator.free(self.missing);
        if (self._sums_content) |c| allocator.free(c);
    }
};

/// Validate a trip bundle directory by checking sha256sums.txt against
/// actual file hashes.
pub fn validateBundle(allocator: Allocator, io: Io, bundle_dir: Dir) !ValidationResult {
    // Read sha256sums.txt (not freed here; stored in result for filename slices)
    const sums_content = bundle_dir.readFileAlloc(io, "sha256sums.txt", allocator, .unlimited) catch |err| {
        if (err == error.FileNotFound)
            return ValidationResult{
                .files = &.{},
                .all_ok = false,
                .missing = &.{},
            };
        return err;
    };

    const entries = try hasher.parseSha256Sums(allocator, sums_content);
    defer allocator.free(entries);

    var results: std.ArrayList(FileResult) = .empty;
    defer results.deinit(allocator);
    var missing_list: std.ArrayList([]const u8) = .empty;
    defer missing_list.deinit(allocator);
    var all_ok = true;

    for (entries) |entry| {
        const actual_hex = hasher.sha256FileHex(allocator, io, bundle_dir, entry.filename) catch |err| {
            if (err == error.FileNotFound) {
                try missing_list.append(allocator, entry.filename);
                all_ok = false;
                continue;
            }
            return err;
        };

        const ok = std.mem.eql(u8, &entry.hash, &actual_hex);
        if (!ok) all_ok = false;

        try results.append(allocator, .{
            .filename = entry.filename,
            .expected = entry.hash,
            .actual = actual_hex,
            .ok = ok,
        });
    }

    return ValidationResult{
        .files = try results.toOwnedSlice(allocator),
        .all_ok = all_ok,
        .missing = try missing_list.toOwnedSlice(allocator),
        ._sums_content = sums_content,
    };
}
