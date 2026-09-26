const std = @import("std");
const Sha256 = std.crypto.hash.sha2.Sha256;
const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = Io.Dir;

pub const DIGEST_LEN = Sha256.digest_length; // 32 bytes
pub const HEX_LEN = DIGEST_LEN * 2; // 64 hex chars

/// Compute SHA-256 of an in-memory buffer.
pub fn sha256Buffer(data: []const u8) [DIGEST_LEN]u8 {
    var h = Sha256.init(.{});
    h.update(data);
    return h.finalResult();
}

/// Format a raw digest as lower-case hex.
pub fn hexDigest(digest: *const [DIGEST_LEN]u8) [HEX_LEN]u8 {
    return std.fmt.bytesToHex(digest.*, .lower);
}

/// Compute SHA-256 of a file.
pub fn sha256File(allocator: Allocator, io: Io, dir: Dir, sub_path: []const u8) ![DIGEST_LEN]u8 {
    const content = try dir.readFileAlloc(io, sub_path, allocator, .unlimited);
    defer allocator.free(content);
    return sha256Buffer(content);
}

/// Compute the hex-string SHA-256 of a file.
pub fn sha256FileHex(allocator: Allocator, io: Io, dir: Dir, sub_path: []const u8) ![HEX_LEN]u8 {
    const digest = try sha256File(allocator, io, dir, sub_path);
    return hexDigest(&digest);
}

/// Parse a sha256sums.txt file into an array list of (hash, filename) pairs.
pub const ChecksumEntry = struct {
    hash: [HEX_LEN]u8,
    filename: []const u8,
};

pub fn parseSha256Sums(allocator: Allocator, content: []const u8) ![]ChecksumEntry {
    var entries: std.ArrayList(ChecksumEntry) = .empty;
    errdefer entries.deinit(allocator);

    var lines = std.mem.splitScalar(u8, content, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        // Format: <64 hex chars>  <filename>
        if (line.len < HEX_LEN + 2) continue;
        if (line[HEX_LEN] != ' ' or line[HEX_LEN + 1] != ' ') continue;

        var entry: ChecksumEntry = undefined;
        @memcpy(&entry.hash, line[0..HEX_LEN]);
        entry.filename = line[HEX_LEN + 2 ..];
        try entries.append(allocator, entry);
    }

    return entries.toOwnedSlice(allocator);
}

// -- tests --

test "sha256 of empty buffer" {
    const digest = sha256Buffer("");
    const hex = hexDigest(&digest);
    try std.testing.expectEqualStrings(
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        &hex,
    );
}

test "sha256 of 'abc'" {
    const digest = sha256Buffer("abc");
    const hex = hexDigest(&digest);
    try std.testing.expectEqualStrings(
        "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
        &hex,
    );
}

test "parse sha256sums.txt" {
    const content =
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  manifest.json\n" ++
        "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  samples.bin\n";

    const entries = try parseSha256Sums(std.testing.allocator, content);
    defer std.testing.allocator.free(entries);

    try std.testing.expectEqual(@as(usize, 2), entries.len);
    try std.testing.expectEqualStrings("manifest.json", entries[0].filename);
    try std.testing.expectEqualStrings("samples.bin", entries[1].filename);
}
