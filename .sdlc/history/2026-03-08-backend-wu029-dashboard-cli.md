# WU-029: Dashboard CLI Integration and Config

**Date:** 2026-03-08
**Role:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Integrated the dashboard web server into the CLI, adding `--dashboard` and `--dashboard-port` flags to `modeltap start`, and replacing the `modeltap dashboard` stub with a command that prints the dashboard URL and attempts to open it in the default browser.

## Changes

### `internal/cli/start.go`
- Added `--dashboard` boolean flag (enables dashboard server alongside proxy)
- Added `--dashboard-port` integer flag (defaults to 8081)
- Both flags bind to Viper config keys (`dashboard.enabled`, `dashboard.port`)
- When dashboard is enabled, creates an `APIHandler`, starts `ListenAndServe` in a goroutine, and prints the dashboard URL
- Dashboard server lifecycle is tied to the same context as the proxy (graceful shutdown)

### `internal/cli/dashboard.go`
- Replaced stub with a command that loads config and prints `Dashboard: http://<bind>:<port>`
- Attempts to open URL in default browser using platform-appropriate command (`open` on macOS, `xdg-open` on Linux, `rundll32` on Windows)
- Falls back to printing the URL if browser open fails

### `internal/cli/root_test.go`
- Removed `dashboard` from `TestStubCommandsOutput` since it is no longer a stub

### `internal/cli/dashboard_test.go` (new)
- `TestDashboardCommandExists`: verifies `modeltap dashboard` runs and outputs a URL
- `TestDashboardOutputContainsDefaultPort`: verifies default port 8081 appears in output
- `TestStartDashboardFlagRecognized`: verifies `--dashboard` appears in `start --help`
- `TestStartDashboardPortFlagRecognized`: verifies `--dashboard-port` appears in `start --help`

### Config Integration
- `MODELTAP_DASHBOARD_ENABLED=true` environment variable works via existing Viper env binding
- `MODELTAP_DASHBOARD_PORT=9090` works similarly
- Config file keys `dashboard.enabled`, `dashboard.port`, `dashboard.bind` all respected

## Test Results

All CLI tests pass (33/33). Build succeeds with no errors.

Pre-existing flaky test in `internal/proxy` (`TestCaptureMiddleware_DetectsProviderAndExtractsMetadata` latency timing) is unrelated to this work.
