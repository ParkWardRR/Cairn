const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // ── common module ──
    const common_mod = b.createModule(.{
        .root_source_file = b.path("common/src/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    // ── bundle module ──
    const bundle_mod = b.createModule(.{
        .root_source_file = b.path("bundle/src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "common", .module = common_mod },
        },
    });

    // ── tripctl CLI executable ──
    const cli_mod = b.createModule(.{
        .root_source_file = b.path("cli/src/main.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "common", .module = common_mod },
            .{ .name = "bundle", .module = bundle_mod },
        },
    });

    const cli_exe = b.addExecutable(.{
        .name = "tripctl",
        .root_module = cli_mod,
    });
    b.installArtifact(cli_exe);

    // ── run step ──
    const run_cmd = b.addRunArtifact(cli_exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }
    const run_step = b.step("run", "Run tripctl");
    run_step.dependOn(&run_cmd.step);

    // ── tests ──

    // common tests
    const common_test_mod = b.createModule(.{
        .root_source_file = b.path("common/src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    const common_tests = b.addTest(.{
        .root_module = common_test_mod,
    });
    const run_common_tests = b.addRunArtifact(common_tests);

    // bundle tests
    const bundle_test_mod = b.createModule(.{
        .root_source_file = b.path("bundle/src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "common", .module = common_mod },
        },
    });
    const bundle_tests = b.addTest(.{
        .root_module = bundle_test_mod,
    });
    const run_bundle_tests = b.addRunArtifact(bundle_tests);

    const test_step = b.step("test", "Run all tests");
    test_step.dependOn(&run_common_tests.step);
    test_step.dependOn(&run_bundle_tests.step);
}
