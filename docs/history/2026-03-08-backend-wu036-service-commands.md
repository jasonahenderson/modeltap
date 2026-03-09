# WU-036: Service Install and Uninstall Commands

**Date:** 2026-03-08
**Agent:** Backend
**Status:** Complete

## Summary

Added `modeltap service install` and `modeltap service uninstall` CLI commands
that write/remove platform-native service definitions and start/stop the
background service using launchd (macOS) or systemd (Linux).

## Changes

### New Files

- `internal/service/install.go` — Platform-specific Install() and Uninstall()
  functions. macOS uses launchctl bootstrap/bootout with load/unload fallback.
  Linux uses systemctl --user enable/disable with daemon-reload.

- `internal/cli/service.go` — Cobra command tree: `service` parent with
  `install` and `uninstall` subcommands. Install resolves the binary path
  via os.Executable + filepath.EvalSymlinks, config path via
  config.DefaultConfigPath(), and delegates to service.Install().

- `internal/cli/service_test.go` — Tests verifying the service command exists,
  has install/uninstall subcommands, and all help flags work correctly.

### Modified Files

- `internal/cli/root.go` — Registered newServiceCommand() in the root command.
- `internal/cli/root_test.go` — Added "service" to subcommand registration
  tests and help tests.

## Test Results

All tests pass:
- `go test ./internal/cli/` — OK
- `go test ./internal/service/` — OK
