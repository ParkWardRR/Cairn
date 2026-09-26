const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const common_mod = b.createModule(.{
        .root_source_file = b.path("../common/src/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    const bundle_mod = b.createModule(.{
        .root_source_file = b.path("../bundle/src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "common", .module = common_mod },
        },
    });

    const cli_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "common", .module = common_mod },
            .{ .name = "bundle", .module = bundle_mod },
        },
    });

    const exe = b.addExecutable(.{
        .name = "tripctl",
        .root_module = cli_mod,
    });
    b.installArtifact(exe);

    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }
    const run_step = b.step("run", "Run tripctl");
    run_step.dependOn(&run_cmd.step);
}
