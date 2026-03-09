# WU-038: Service Documentation and Help Updates

**Date:** 2026-03-08
**Type:** Documentation
**Status:** Complete

## Summary

Updated all documentation, CLI help text, and the dashboard help page to cover the new `modeltap service` commands introduced in WU-035 through WU-037.

## Changes

### docs/usage-guide.md
- Added `modeltap service` to the CLI Commands Reference section with full subcommand table, flags, and examples.
- Added a new "Service Management" section between the CLI reference and Multi-Provider Support sections, covering installation, status checking, log viewing, and uninstallation with platform-specific details (macOS launchd, Linux systemd).
- Added a cross-reference from `modeltap start` to the Service Management section for users wanting persistent background execution.

### internal/cli/service.go
- Added `Example` field to the parent `service` command showing all four subcommands.
- Enhanced `Long` descriptions on `install`, `uninstall`, `status`, and `logs` subcommands with additional context (e.g., data safety note on uninstall, `--lines` flag reminder on logs, verification tip on status).
- Added richer `Example` fields with multi-step workflows (e.g., install then verify with status).

### internal/dashboard/static/help.html
- Added `modeltap service` to the CLI Commands Reference section with subcommand and flags tables.
- Added a new "Service Management" section (as a searchable `data-help-section`) covering install, status, logs, and uninstall with platform notes and code examples. Placed before the Dashboard Usage Tips section.

### internal/cli/status.go
- Added a tip in the `Long` description mentioning `modeltap service install` for persistent background execution.

## Testing

- `go test ./internal/cli/` -- all tests pass.
