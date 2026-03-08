# WU-006: Viper Configuration System

**Date:** 2026-03-07
**Agents:** Designer, Test Engineer, Backend Implementer

## What Was Planned
Implement Viper-based configuration with three-tier precedence (flags > env > file > defaults), wire config CLI commands, and bind flags to the start command.

## What Was Done

### Files Created
- `internal/config/config.go` — Config struct, Load() function using viper.New(), defaults, env binding, YAML file support
- `internal/config/config_test.go` — Tests for defaults, file loading, env var overrides, precedence

### Files Modified
- `internal/cli/config.go` — Wired `config show` (prints resolved config as YAML), `config path` (prints config file path)
- `internal/cli/start.go` — Bound --port and --upstream flags to viper
- `internal/cli/root_test.go` — Updated test expectations for config show/path (no longer stubs)
- `go.mod`, `go.sum` — Added viper v1.21.0 dependency

## Key Decisions
- Used `viper.New()` instance (not global) per ADR-0004
- Config struct returned by Load(), not stored globally
- Home directory expansion for db_path and config file path
- MODELTAP_ prefix for all env vars

## Verification
- `go build ./...` succeeds
- `go test ./...` passes (all CLI + config tests)
- `modeltap config show` prints resolved defaults
- `MODELTAP_PORT=9090 modeltap config show` reflects override
