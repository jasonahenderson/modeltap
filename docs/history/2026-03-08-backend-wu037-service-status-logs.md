# WU-037: Service Status and Logs Commands

**Date:** 2026-03-08
**Agent:** Backend
**Status:** Complete

## Summary

Added `modeltap service status` and `modeltap service logs` CLI subcommands for inspecting the modeltap background service.

## Changes

### New file: `internal/service/status.go`
- `ServiceStatus` struct with `Installed`, `Running`, and `PID` fields.
- `Status(platform Platform) (*ServiceStatus, error)` — checks service state:
  - macOS: checks plist file existence and runs `launchctl list com.modeltap.proxy` to determine if loaded/running and extract PID.
  - Linux: checks unit file existence, runs `systemctl --user is-active modeltap` and `systemctl --user show modeltap --property=MainPID`.
- `LogFilePath() (string, error)` — returns `~/.config/modeltap/modeltap.log`.
- `Logs(platform Platform, lines int) (string, error)` — retrieves recent logs:
  - macOS: reads last N lines from the log file.
  - Linux: runs `journalctl --user -u modeltap --no-pager -n <lines>`.

### Updated: `internal/cli/service.go`
- Added `status` subcommand that prints installation state, running status, and PID.
- Added `logs` subcommand with `--lines` / `-n` flag (default 50) that prints recent log output.

### Updated: `internal/cli/service_test.go`
- Extended `TestServiceSubcommandsExist` to verify `status` and `logs` subcommands are registered.
- Extended `TestServiceHelp` to verify `status` and `logs` appear in help output.
- Added `TestServiceStatusHelp` — verifies status help contains expected content.
- Added `TestServiceLogsHelp` — verifies logs help mentions `--lines` and `-n`.
- Added `TestServiceLogsLinesFlag` — verifies default line count of 50 appears in help.

## Test Results

```
ok  github.com/jasonahenderson/modeltap/internal/cli
ok  github.com/jasonahenderson/modeltap/internal/service
```
