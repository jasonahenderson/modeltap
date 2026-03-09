# WU-031: User Documentation and Usage Guide

**Date:** 2026-03-08
**Agent:** Documentation
**Status:** Complete

## Summary

Created a comprehensive user guide at `docs/usage-guide.md` covering all aspects of modeltap usage. The guide was written by reading the actual CLI source code, config module, and provider adapters to ensure accuracy.

## What Was Documented

1. **Overview** -- description of modeltap as a transparent reverse proxy for AI/ML API traffic with local storage and analysis tools.

2. **Installation** -- building from source with `make build`, binary at `./bin/modeltap`.

3. **Quick Start** -- five-step walkthrough from starting the proxy to opening the dashboard.

4. **Configuration** -- config file location (`~/.config/modeltap/config.yaml`), all available settings with defaults, config subcommands (`show`, `set`, `path`), and environment variable overrides with `MODELTAP_` prefix.

5. **CLI Commands Reference** -- full documentation for all nine commands:
   - `start` -- flags: `--port/-p`, `--upstream/-u`, `--dashboard`, `--dashboard-port`
   - `logs` -- flags: `--limit`, `--provider`, `--model`, `--since`, `--until`, `--status`
   - `show <id>` -- no flags, requires ID argument
   - `export` -- flags: `--format` (jsonl/csv), `--since`, `--until`
   - `metrics` -- flags: `--since`, `--until`, `--group-by` (provider/model/day/hour), `--format` (table/json/csv); subcommand: `rebuild`
   - `status` -- no flags
   - `config` -- subcommands: `show`, `set <key> <value>`, `path`
   - `dashboard` -- no flags
   - `completion` -- subcommands: `bash`, `zsh`, `fish`, `powershell`

6. **Multi-Provider Support** -- automatic detection logic for Anthropic (host, anthropic-version header, x-api-key + /v1/messages) and OpenAI (host, /v1/chat/completions path); provider-specific upstream routing configuration.

7. **Web Dashboard** -- enabling, accessing at http://127.0.0.1:8081, available pages (log viewer, metrics, status).

8. **Supported Providers** -- Anthropic (Messages API, Claude models, SSE streaming) and OpenAI (Chat Completions API, GPT models, SSE streaming).

9. **Time Flag Format** -- duration shorthands (s/m/h/d) and RFC3339 timestamps.

## Source Files Referenced

- `internal/cli/root.go` -- command registration
- `internal/cli/start.go` -- start command flags and behavior
- `internal/cli/logs.go` -- logs command flags and output format
- `internal/cli/show.go` -- show command argument handling and output
- `internal/cli/export.go` -- export formats (jsonl, csv) and time parsing
- `internal/cli/metrics.go` -- metrics flags, group-by options, output formats, rebuild subcommand
- `internal/cli/status.go` -- status output sections
- `internal/cli/config.go` -- config subcommands (show, set, path)
- `internal/cli/dashboard.go` -- dashboard browser opening
- `internal/cli/completion.go` -- shell completion subcommands and installation instructions
- `internal/config/config.go` -- config defaults, file path, env prefix, viper setup
- `internal/provider/anthropic.go` -- Anthropic detection and parsing logic
- `internal/provider/openai.go` -- OpenAI detection and parsing logic
- `Makefile` -- build target

## Key Decisions

- Documented the export format as `jsonl` (not `json`) since the code validates against `jsonl` and `csv`, not `json`.
- Noted that `dashboard.enabled` defaults to `false` (per the code), meaning users must explicitly enable it.
- Documented the config path as `~/.config/modeltap/config.yaml` (per `DefaultConfigPath()`), not `~/.modeltap/config.yaml`.
- Included the `--status` flag on the `logs` command and the `metrics rebuild` subcommand, which were present in the code but not in the original task specification.
