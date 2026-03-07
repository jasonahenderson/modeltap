# WU-005: Cobra CLI Skeleton

**Date:** 2026-03-06
**Role:** Backend Implementer (also Designer, Test Engineer)
**Status:** Complete

## Summary

Implemented the Cobra CLI command structure for modeltap with stub subcommands.

## What Was Done

### Dependencies
- Added `github.com/spf13/cobra v1.10.2` (plus transitive deps: pflag, mousetrap)

### Files Created
- `internal/cli/root.go` -- Root command with injectable version, registers all subcommands
- `internal/cli/start.go` -- `modeltap start` stub
- `internal/cli/logs.go` -- `modeltap logs` stub
- `internal/cli/show.go` -- `modeltap show <id>` stub (requires exactly 1 arg)
- `internal/cli/export.go` -- `modeltap export` stub
- `internal/cli/config.go` -- `modeltap config` with subcommands: `show`, `set <key> <value>`, `path`
- `internal/cli/status.go` -- `modeltap status` stub
- `internal/cli/metrics.go` -- `modeltap metrics` stub with `rebuild` subcommand
- `internal/cli/dashboard.go` -- `modeltap dashboard` stub
- `internal/cli/completion.go` -- `modeltap completion [bash|zsh|fish|powershell]` using Cobra built-in generators
- `internal/cli/root_test.go` -- 6 test functions, 35 subtests total

### Files Modified
- `cmd/modeltap/main.go` -- Updated to use `cli.NewRootCommand(version)` instead of fmt.Printf

## Design Decisions
- Version is injected into `NewRootCommand(version string)` rather than using a global variable
- Each command is in its own file within `internal/cli/`
- Config subcommands (`show`, `set`, `path`) are in `config.go` alongside the parent
- Metrics subcommand (`rebuild`) is in `metrics.go` alongside the parent
- All stubs print "not implemented yet" and return nil error
- Completion command uses Cobra's built-in shell completion generators

## Test Coverage
- Root command executes without error
- `--version` flag prints injected version string
- All 9 subcommands are registered
- All subcommands (including nested) accept `--help`
- Help output lists all subcommands
- All stub commands produce "not implemented yet" output

## Verification
- `go build ./...` succeeds
- `go test ./...` passes (35/35 tests)
- `modeltap --help` lists all subcommands
- `modeltap --version` prints version

## Notes
- The system Go is 1.17 which has dyld issues on newer macOS; tests verified with Go 1.25.6 at `/usr/local/opt/go/bin/go`. The go.mod retains `go 1.17` as the minimum version per existing project configuration.
