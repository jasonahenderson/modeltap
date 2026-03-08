# Master Plan

## Last Updated
2026-03-06

## Overview

This plan breaks all v1 scope into ordered, independently completable work units. Each work unit is sized for a single agent session and specifies which agents are needed. Work units are grouped into phases and ordered by dependency.

v2 scope (knowledge layer, MCP server, multi-user support) is listed at the end as future work without detailed work units.

---

## Phase 1: Project Foundation

These work units establish the Go project, build system, and open source scaffolding. No application logic yet.

### WU-001: Go Module Init and Project Scaffolding

- **Description:** Initialize the Go module (`go mod init`), create the directory structure (`cmd/`, `internal/`, `pkg/`), add a minimal `main.go`, and verify `go build` produces a binary.
- **Dependencies:** None
- **Agents:** infra
- **Definition of Done:** `go build ./...` succeeds, binary runs and prints a version string, directory structure matches Go conventions.

### WU-002: Build System and Makefile

- **Description:** Create a Makefile with targets for `build`, `test`, `lint`, `fmt`, and `vet`. Add `.golangci-lint` configuration. Set up goreleaser config for cross-platform binary distribution.
- **Dependencies:** WU-001
- **Agents:** infra
- **Definition of Done:** `make build`, `make test`, `make lint`, and `make fmt` all execute successfully. goreleaser config validates with `goreleaser check`.

### WU-003: CI Pipeline (GitHub Actions)

- **Description:** Create GitHub Actions workflows for CI: lint, test, and build on push/PR. Add DCO sign-off check per ADR-0011.
- **Dependencies:** WU-002
- **Agents:** infra
- **Definition of Done:** CI workflow runs on push, executes lint/test/build, and enforces DCO sign-off. Workflow file is valid YAML.

### WU-004: Open Source Files (License, Contributing, Governance)

- **Description:** Add Apache 2.0 LICENSE file, CONTRIBUTING.md with DCO sign-off instructions and contribution process, and GOVERNANCE.md documenting the BDFL model and contributor tiers per ADR-0010 and ADR-0011.
- **Dependencies:** WU-001
- **Agents:** docs
- **Definition of Done:** LICENSE, CONTRIBUTING.md, and GOVERNANCE.md exist at repo root. LICENSE is Apache 2.0. CONTRIBUTING.md describes fork/branch/PR/DCO workflow. GOVERNANCE.md documents BDFL model and tiers.

---

## Phase 2: CLI and Configuration Framework

These work units establish the Cobra CLI skeleton and Viper configuration system. Commands are stubs that will be wired to real logic in later phases.

### WU-005: Cobra CLI Skeleton

- **Description:** Design the Cobra command structure: root command, `start`, `logs`, `show`, `export`, `config`, `status`, `metrics`, `dashboard`, and `completion` subcommands. Implement as stubs that print placeholders. Add `--version` flag.
- **Dependencies:** WU-001
- **Agents:** designer, tester, backend
- **Definition of Done:** `modeltap --help` lists all subcommands. `modeltap --version` prints version. Each subcommand accepts `--help` and prints usage. All commands compile and unit tests pass.

### WU-006: Viper Configuration System

- **Description:** Implement Viper-based configuration loading from `~/.config/modeltap/config.yaml`, environment variables with `MODELTAP_` prefix, and CLI flag binding. Define all v1 config keys (port, db_path, upstream, retention_days, dashboard settings). Implement the `config` subcommand (`config show`, `config set`, `config path`).
- **Dependencies:** WU-005
- **Agents:** designer, tester, backend
- **Definition of Done:** Config loads from file, env vars, and flags with correct precedence (flags > env > file > defaults). `modeltap config show` prints resolved config. `MODELTAP_PORT=9090 modeltap config show` reflects the override. Unit tests verify precedence.

---

## Phase 3: Storage Layer

These work units build the SQLite storage layer that all other features depend on.

### WU-007: SQLite Schema and Store Interface

- **Description:** Design and implement the `Store` interface and SQLite schema for the `requests` table. Use `modernc.org/sqlite` (pure Go). Include columns for id, timestamp, provider, model, method, url, request_headers, request_body, response_status, response_headers, response_body, input_tokens, output_tokens, latency_ms, estimated_cost_usd. Create the database file, run migrations, and implement `SaveRequest`, `GetRequest`, `ListRequests` with filtering (by provider, model, time range, status code), and `Close`.
- **Dependencies:** WU-006
- **Agents:** designer, tester, backend
- **Definition of Done:** Store interface is defined. SQLite implementation creates the database and schema. CRUD operations work with unit tests. Filtering and pagination work in `ListRequests`. WAL mode is enabled.

### WU-008: Retention Pruning

- **Description:** Implement the background retention pruner as a goroutine that periodically deletes records older than `retention_days` from the `requests` table. Configurable via Viper (`retention_days`, `prune_interval`).
- **Dependencies:** WU-007
- **Agents:** tester, backend
- **Definition of Done:** Pruner runs on a configurable interval. Records older than retention_days are deleted. Unit tests verify pruning behavior. Pruner stops cleanly on shutdown.

### WU-009: Export Command

- **Description:** Implement `modeltap export --format jsonl` and `modeltap export --format csv` commands that read from the Store and write to stdout. Support `--since` and `--until` time filters.
- **Dependencies:** WU-007, WU-005
- **Agents:** tester, backend
- **Definition of Done:** `modeltap export --format jsonl` outputs valid JSONL. `modeltap export --format csv` outputs valid CSV. Time filters work correctly. Unit tests cover both formats.

---

## Phase 4: Provider Adapters

These work units build the provider adapter interface and the first two provider implementations.

### WU-010: Provider Interface Definition

- **Description:** Design and define the `Provider` interface with methods: `Name()`, `Detect()`, `ParseRequest()`, `ParseResponse()`, `ReassembleStream()`. Define shared types: `RequestMetadata`, `ResponseMetadata`, `StreamChunk`. Implement a provider registry for registering and looking up providers.
- **Dependencies:** WU-007
- **Agents:** designer, tester, backend
- **Definition of Done:** Provider interface is defined in its own package. Registry supports `Register()` and `Detect()` (auto-detect provider from request). Types compile. Unit tests cover registry behavior.

### WU-011: Anthropic Provider Adapter

- **Description:** Implement the Anthropic provider adapter. Handle API detection (host-based and path-based), request metadata extraction (model, max_tokens), response metadata extraction (input_tokens, output_tokens, stop_reason), and SSE stream reassembly for Anthropic's `message_start`/`content_block_delta`/`message_delta`/`message_stop` event format.
- **Dependencies:** WU-010
- **Agents:** tester, backend
- **Definition of Done:** Anthropic adapter correctly detects Anthropic API requests. Request and response metadata extraction works for non-streaming and streaming responses. SSE stream reassembly produces the complete response text. Unit tests cover all event types with realistic fixtures.

### WU-012: OpenAI Provider Adapter

- **Description:** Implement the OpenAI provider adapter. Handle API detection, request metadata extraction, response metadata extraction (prompt_tokens, completion_tokens), and SSE stream reassembly for OpenAI's `data:` event format with `[DONE]` terminator.
- **Dependencies:** WU-010
- **Agents:** tester, backend
- **Definition of Done:** OpenAI adapter correctly detects OpenAI API requests. Metadata extraction works for non-streaming and streaming responses. SSE stream reassembly handles `data: [DONE]` correctly. Unit tests cover all event types with realistic fixtures.

---

## Phase 5: Proxy Core

These work units build the reverse proxy that ties providers, capture, and storage together.

### WU-013: Basic Reverse Proxy

- **Description:** Implement the core reverse proxy using `httputil.ReverseProxy`. Accept incoming requests, forward to the configured upstream, and return responses. Support configurable listen port and upstream URL. Wire to `modeltap start` command.
- **Dependencies:** WU-006, WU-010
- **Agents:** designer, tester, backend
- **Definition of Done:** `modeltap start` launches a reverse proxy on the configured port. Requests are forwarded to the upstream and responses returned to the client. Non-streaming requests pass through correctly. Unit tests with a mock upstream verify forwarding.

### WU-014: Request/Response Capture Middleware

- **Description:** Implement middleware that captures full request and response bodies, detects the provider, extracts metadata, and saves to the Store. Handle non-streaming responses (capture response body via `io.TeeReader` or response modifier). Wire provider detection from the registry.
- **Dependencies:** WU-013, WU-007, WU-010, WU-011
- **Agents:** designer, tester, backend
- **Definition of Done:** Every proxied request/response is saved to the Store with all metadata fields populated. Provider is auto-detected. Token counts and cost estimates are extracted. Unit tests verify capture with mock upstream.

### WU-015: SSE Stream Capture

- **Description:** Implement SSE stream capture in the proxy middleware. Intercept chunked/streaming responses, forward each chunk to the client in real-time (no added latency), buffer chunks, and reassemble the complete response using the provider adapter's `ReassembleStream`. Save the reassembled response to the Store after streaming completes.
- **Dependencies:** WU-014, WU-011, WU-012
- **Agents:** designer, tester, backend
- **Definition of Done:** Streaming responses are forwarded to the client with no added latency. Complete reassembled response is saved to the Store after stream ends. Works with both Anthropic and OpenAI streaming formats. Unit tests verify with mock SSE upstream.

### WU-016: Multi-Provider Routing

- **Description:** Extend the proxy to support multiple upstream providers simultaneously. Route requests to the correct upstream based on provider detection (host header, path, or configuration). Support a provider-to-upstream mapping in config.
- **Dependencies:** WU-015
- **Agents:** designer, tester, backend
- **Definition of Done:** Proxy routes Anthropic requests to `api.anthropic.com` and OpenAI requests to `api.openai.com` (or configured upstreams). Provider detection drives routing. Config supports provider-to-upstream mapping. Integration tests verify routing with multiple mock upstreams.

---

## Phase 6: Usage Metrics

These work units add metrics aggregation and reporting on top of the captured data.

### WU-017: Metrics Aggregation Tables

- **Description:** Implement `hourly_usage` and `daily_usage` aggregation tables in SQLite per ADR-0007 schema. Update aggregation tables atomically within the same transaction as request writes. Include columns for provider, model, request_count, input_tokens, output_tokens, estimated_cost_usd, avg_latency_ms, error_count.
- **Dependencies:** WU-007, WU-014
- **Agents:** designer, tester, backend
- **Definition of Done:** Aggregation tables are created by migration. Each request write updates the corresponding hourly and daily rollup. Aggregation is atomic with the request write. Unit tests verify rollup accuracy.

### WU-018: Metrics CLI Commands

- **Description:** Implement `modeltap metrics` command with subcommands/flags: `--since`, `--until`, `--group-by` (provider, model, day), `--format` (table, json, csv). Read from aggregation tables. Implement `modeltap metrics rebuild` to recompute aggregations from raw logs.
- **Dependencies:** WU-017, WU-005
- **Agents:** tester, backend
- **Definition of Done:** `modeltap metrics --since 7d` displays usage summary. `--group-by model` breaks down by model. `--format json` outputs JSON. `modeltap metrics rebuild` recomputes aggregation tables. Unit tests verify output correctness.

### WU-019: Cost Estimation with Pricing Table

- **Description:** Implement a configurable pricing table (YAML-based via Viper config) that maps provider/model pairs to per-million-token input/output costs. Apply cost estimates during request capture. Support user-overridable pricing.
- **Dependencies:** WU-017, WU-006
- **Agents:** tester, backend
- **Definition of Done:** Default pricing table includes current Anthropic and OpenAI model prices. Cost estimates are applied to each captured request. Users can override pricing in config. Unit tests verify cost calculation accuracy.

---

## Phase 7: CLI Query Commands

These work units wire the remaining CLI commands to real data.

### WU-020: Logs Command

- **Description:** Implement `modeltap logs` to list captured requests with filtering (--provider, --model, --since, --until, --status, --limit). Display as a formatted table with columns: timestamp, provider, model, status, tokens, cost, latency.
- **Dependencies:** WU-007, WU-005
- **Agents:** tester, backend
- **Definition of Done:** `modeltap logs` lists recent requests. All filter flags work. Output is a readable table. Unit tests verify filtering and formatting.

### WU-021: Show Command

- **Description:** Implement `modeltap show <request-id>` to display the full detail of a captured request/response, including headers, bodies (with JSON pretty-printing), metadata, and token usage.
- **Dependencies:** WU-007, WU-005
- **Agents:** tester, backend
- **Definition of Done:** `modeltap show <id>` displays full request and response detail. JSON bodies are pretty-printed. Missing IDs produce a clear error. Unit tests verify output.

### WU-022: Status Command

- **Description:** Implement `modeltap status` to display proxy status (running/stopped, port, upstream URL), database info (path, size, record count), and retention settings.
- **Dependencies:** WU-007, WU-005, WU-013
- **Agents:** tester, backend
- **Definition of Done:** `modeltap status` displays proxy running state, database size, record count, and retention config. Unit tests verify output.

---

## Phase 8: Integration Testing and Security Review

These work units validate that all components work together and are secure.

### WU-023: End-to-End Integration Tests

- **Description:** Write integration tests that stand up the full proxy with a mock upstream, send requests through it, and verify that requests are captured, metrics are aggregated, and CLI query commands return the correct data. Cover both Anthropic and OpenAI adapters, streaming and non-streaming.
- **Dependencies:** WU-016, WU-018, WU-020, WU-021
- **Agents:** integration
- **Definition of Done:** Integration tests cover: proxy forwarding, request capture, SSE stream capture and reassembly, metrics aggregation, CLI log/show/metrics/export queries. All tests pass. Tests run in CI.

### WU-024: Security Review

- **Description:** Review all production code for security vulnerabilities: SQL injection in SQLite queries, credential/API key exposure in logs, input validation at proxy boundaries, error message information leakage, path traversal in config/db paths.
- **Dependencies:** WU-023
- **Agents:** security
- **Definition of Done:** Security review document produced with pass/fail per area. All critical and high severity findings are fixed. No SQL injection. No credential logging. No information leakage in error messages.

---

## Phase 9: Web Dashboard

These work units build the web dashboard. They depend on the backend API existing (Phases 3-7).

### WU-025: Dashboard API Endpoints

- **Description:** Design and implement internal REST API endpoints for the dashboard: `GET /api/logs` (paginated, filtered), `GET /api/logs/:id` (detail), `GET /api/metrics` (aggregated, with time range and group-by), `GET /api/status` (proxy status). All endpoints return JSON. Bind to 127.0.0.1 by default.
- **Dependencies:** WU-007, WU-017, WU-013
- **Agents:** designer, tester, backend
- **Definition of Done:** All API endpoints return correct JSON responses. Pagination works. Filters work. Endpoints are bound to localhost. Unit tests cover each endpoint.

### WU-026: Dashboard Frontend - Log Viewer

- **Description:** Build the log viewer page: browse captured requests with filtering (provider, model, time range, status code), expand entries to see full request/response bodies with JSON syntax highlighting, search by content. Use lightweight frontend (htmx, alpine.js, or vanilla JS). Embed assets via `embed.FS`.
- **Dependencies:** WU-025
- **Agents:** designer, tester, ui
- **Definition of Done:** Log viewer page renders in browser. Filtering controls work. Expanding an entry shows full request/response. JSON is syntax-highlighted. Assets are embedded in binary. Responsive design works on desktop and tablet.

### WU-027: Dashboard Frontend - Metrics Display

- **Description:** Build the metrics dashboard page: usage over time (requests, tokens, cost), breakdown by provider and model, configurable time ranges (today, 7d, 30d, custom). Use lightweight charts (chart.js or similar minimal library).
- **Dependencies:** WU-025
- **Agents:** designer, tester, ui
- **Definition of Done:** Metrics page renders charts for usage over time. Provider and model breakdown displays correctly. Time range selector works. Data matches CLI metrics output.

### WU-028: Dashboard Frontend - Proxy Status

- **Description:** Build the proxy status page: current proxy status (running, port, upstream URL), active connections, error rate, database size and record count.
- **Dependencies:** WU-025
- **Agents:** tester, ui
- **Definition of Done:** Status page shows current proxy state. Data refreshes on page load. Displays database size and record count.

### WU-029: Dashboard CLI Integration and Config

- **Description:** Wire dashboard to CLI: `modeltap start --dashboard` enables the dashboard, `modeltap start --dashboard-port 8081` sets the port, `modeltap dashboard` opens the dashboard in the default browser. Add dashboard config to Viper (enabled, port, bind address, auth token).
- **Dependencies:** WU-025, WU-026, WU-027, WU-028, WU-006
- **Agents:** tester, backend
- **Definition of Done:** `modeltap start --dashboard` serves the dashboard. `modeltap dashboard` opens browser. Config keys work. Unit tests verify CLI flag binding and server startup.

### WU-030: Dashboard Security Review

- **Description:** Security review of the dashboard: XSS prevention in log display (especially user-controlled request/response bodies), CSRF protection, authentication bypass, response header security, content-type validation.
- **Dependencies:** WU-029
- **Agents:** security
- **Definition of Done:** Security review document produced. No XSS in log viewer. Response bodies are safely escaped. Auth token (when enabled) is properly validated. All critical findings are fixed.

---

## Phase 10: Documentation and Polish

### WU-031: User Documentation and Usage Guide

- **Description:** Write comprehensive user-facing documentation: README with installation, quickstart, and feature overview. Full usage guide with step-by-step tutorials (setting up the proxy, capturing first request, viewing logs, using metrics, configuring providers). CLI reference for all commands with examples and common workflows. Configuration reference with all keys, defaults, and env var mappings. Architecture overview for contributors.
- **Dependencies:** WU-024, WU-029
- **Agents:** docs
- **Definition of Done:** README includes installation instructions, quickstart guide, and feature list. Usage guide walks through all major workflows with examples. CLI reference covers all commands with flags, examples, and tips. Config reference lists all keys. Architecture doc explains package structure.

### WU-032: Shell Completion Generation

- **Description:** Wire Cobra's built-in shell completion generation for Bash, Zsh, Fish, and PowerShell via `modeltap completion <shell>`. Add installation instructions to documentation.
- **Dependencies:** WU-005
- **Agents:** backend, docs
- **Definition of Done:** `modeltap completion bash/zsh/fish/powershell` generates valid completion scripts. Documentation explains how to install completions.

### WU-033: Comprehensive CLI Help System

- **Description:** Enhance Cobra commands with detailed long descriptions, usage examples, and topic-based help. Add `modeltap help <topic>` support for topics like "providers", "configuration", "capture", "metrics", "dashboard". Each topic provides a focused guide accessible directly from the terminal. Wire Cobra's `Long` and `Example` fields on every command.
- **Dependencies:** WU-031
- **Agents:** backend, docs
- **Definition of Done:** Every command has a detailed `Long` description and at least one `Example`. `modeltap help` lists available topics. `modeltap help providers` (and other topics) prints a focused guide. Tests verify topic commands exist.

### WU-034: Dashboard Help and Documentation Page

- **Description:** Add a searchable help/documentation page to the web dashboard. Serves the usage guide, CLI reference, and configuration reference as browsable HTML. Includes client-side search (filter by keyword across all docs). Accessible from the dashboard navigation. Content sourced from the markdown docs written in WU-031, rendered to HTML at build time or via a Go markdown renderer.
- **Dependencies:** WU-029, WU-031
- **Agents:** designer, tester, ui
- **Definition of Done:** Dashboard has a "Help" or "Docs" page in navigation. All user documentation is browsable. Search box filters content by keyword. Content matches the CLI/config reference. Responsive layout.

---

## v2 - Future Work (Not Planned in Detail)

The following features are accepted but scoped for v2. They will be planned in detail after v1 ships.

### Knowledge Layer (ADR-0008, ADR-0009)

- sqlite-vec integration for vector embeddings
- Background embedding pipeline with configurable embedding model (cloud or local)
- Semantic search CLI command (`modeltap search`)
- Metadata extraction (decisions, action items, topics)
- MCP server (`modeltap mcp`) for cross-tool knowledge access via stdio transport
- Knowledge dashboard UI in web interface

### Multi-User Support (feature: multi-user-support)

- User resolver interface (API key mapping, header-based, client certificate)
- Per-user data isolation (`user_id` column across all tables)
- Per-user metrics and admin aggregate views
- User management CLI commands (`modeltap users`)
- Shared knowledge pool (opt-in)
- Dashboard per-user views and admin panel

---

## Dependency Graph

```
WU-001 (Go module init)
  |
  +---> WU-002 (Build system) ---> WU-003 (CI pipeline)
  |
  +---> WU-004 (Open source files)
  |
  +---> WU-005 (Cobra CLI skeleton)
           |
           +---> WU-006 (Viper config)
           |       |
           |       +---> WU-007 (SQLite schema + Store)
           |       |       |
           |       |       +---> WU-008 (Retention pruning)
           |       |       +---> WU-009 (Export command)
           |       |       +---> WU-010 (Provider interface)
           |       |       |       |
           |       |       |       +---> WU-011 (Anthropic adapter)
           |       |       |       +---> WU-012 (OpenAI adapter)
           |       |       |
           |       |       +---> WU-020 (Logs command)
           |       |       +---> WU-021 (Show command)
           |       |
           |       +---> WU-013 (Basic reverse proxy)
           |       |       |
           |       |       +---> WU-014 (Capture middleware) [+WU-007, WU-010, WU-011]
           |       |       |       |
           |       |       |       +---> WU-015 (SSE stream capture) [+WU-012]
           |       |       |       |       |
           |       |       |       |       +---> WU-016 (Multi-provider routing)
           |       |       |       |
           |       |       |       +---> WU-017 (Metrics agg tables)
           |       |       |               |
           |       |       |               +---> WU-018 (Metrics CLI)
           |       |       |               +---> WU-019 (Cost estimation)
           |       |       |
           |       |       +---> WU-022 (Status command)
           |       |       +---> WU-025 (Dashboard API) [+WU-007, WU-017]
           |       |               |
           |       |               +---> WU-026 (Dashboard log viewer)
           |       |               +---> WU-027 (Dashboard metrics)
           |       |               +---> WU-028 (Dashboard status)
           |       |               +---> WU-029 (Dashboard CLI) [+WU-026-028, WU-006]
           |       |                       |
           |       |                       +---> WU-030 (Dashboard security review)
           |       |
           |       +---> WU-019 (Cost estimation) [+WU-017]
           |
           +---> WU-032 (Shell completions)

WU-016 + WU-018 + WU-020 + WU-021 ---> WU-023 (Integration tests)
WU-023 ---> WU-024 (Security review)
WU-024 + WU-029 ---> WU-031 (User documentation + usage guide)
WU-031 ---> WU-033 (CLI help system)
WU-029 + WU-031 ---> WU-034 (Dashboard help/docs page)
```
