# WU-033: CLI Help System

**Date:** 2026-03-08
**Agent:** Backend/Docs
**Status:** Complete

## Summary

Enhanced the CLI help output for all modeltap commands by adding comprehensive
`Long` descriptions and `Example` fields using Cobra's built-in support.

## Changes

### Root command (`internal/cli/root.go`)
- Expanded `Long` description to explain modeltap's purpose, architecture
  (reverse proxy, SQLite storage), and key capabilities (proxying, token
  counting, cost estimation, export, metrics, dashboard).
- Added `Example` field with representative usage patterns covering the most
  common workflows.

### `start` command (`internal/cli/start.go`)
- Added multi-paragraph `Long` description covering proxy behavior, config
  resolution order, graceful shutdown, and dashboard opt-in.
- Added `Example` field with flag combinations (port, upstream, dashboard).

### `logs` command (`internal/cli/logs.go`)
- Expanded `Long` to describe table columns, default sort/limit, and time
  filter syntax (duration shorthand and RFC3339).
- Added examples for provider/model filtering, status filtering, and time
  windows.

### `show` command (`internal/cli/show.go`)
- Expanded `Long` to describe output sections (headers, body, tokens, cost)
  and explain short vs full ID usage.
- Added examples for short ID and full UUID lookups.

### `export` command (`internal/cli/export.go`)
- Expanded `Long` to describe JSONL and CSV formats, included columns, and
  redirect-to-file workflow.
- Added examples for both formats with time filters.

### `metrics` command (`internal/cli/metrics.go`)
- Expanded `Long` to describe default 30-day window, grouping options, and
  output formats.
- Added examples for group-by, format, and time window combinations.

### `metrics rebuild` subcommand (`internal/cli/metrics.go`)
- Added `Long` describing when rebuild is needed (migration, data correction).
- Added `Example` field.

### `status` command (`internal/cli/status.go`)
- Expanded `Long` to list output sections (proxy, database, retention,
  providers) and common use cases.
- Added `Example` field.

### `config` command and subcommands (`internal/cli/config.go`)
- `config`: Added `Long` explaining config sources and `Example` with all
  three subcommands.
- `config show`: Added `Long` describing what is displayed and `Example`.
- `config set`: Added `Long` explaining dotted key path and `Example` with
  common keys.
- `config path`: Added `Long` and `Example` including editor shortcut.

### `dashboard` command (`internal/cli/dashboard.go`)
- Expanded `Long` to explain prerequisite (proxy must be running with
  dashboard enabled) and fallback behavior.
- Added `Example` field.

### `completion` command (`internal/cli/completion.go`)
- Already had comprehensive `Long` descriptions and examples; no changes
  needed.

## Verification

All changes are additive string field additions to Cobra command structs.
No logic, imports, or function signatures were modified.
