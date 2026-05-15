# v0.2.0 Changelog

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Status:** unreleased (in development on `exploration/integrated-harness`)

v0.2.0 reframes modeltap from a pure capture proxy into an integrated AI environment. The proxy layer from v0.1 is unchanged and fully retained; v0.2.0 adds a JSON-RPC BFF server, a Bubbletea terminal harness, a complete built-in tool framework, an MCP client, and end-to-end session / routing / context / history machinery.

For a detailed work-unit-level breakdown see `status.md` and per-WU commit messages.

## Headline additions

### Interactive terminal harness (`modeltap` default)

Running `modeltap` with no subcommand now launches an interactive Bubbletea terminal harness. The harness auto-starts the BFF server over a local unix socket, connects, and presents a chat-style UI with a status bar, streaming viewport, and slash-command surface.

### JSON-RPC BFF server

A new BFF (backend-for-frontend) server speaks JSON-RPC 2.0 over unix socket (default) or TLS (enterprise/team). Handles session lifecycle, turn dispatch, streaming relay, cost tracking, provider health checks, model routing, context management, compaction helpers, and cross-session command history.

### Tool framework (13 built-in tools)

`internal/harness/tools/` hosts a four-level permission model (read-only / write / execute / destructive) plus dangerous-command detection and per-tool approval memory. Built-in tools:

- **Read** — text (line-numbered), CSV, image (base64 + MIME), XLSX (excelize/v2), DOCX (stdlib archive/zip + encoding/xml — no UniDoc dep), PDF (ledongthuc/pdf, page-range aware).
- **Write** — with pre-write snapshot in the result.
- **Edit** — exact-match replacement with uniqueness guard; enforces Read-before-mutate via FileTracker.
- **Bash** — `sh -c` with projectRoot cwd, context timeout, combined stdout/stderr, output truncation.
- **Git** — classification-aware risk (read vs mutation vs destructive); auto-allows reads in every mode.
- **Glob** — doublestar/v4, mtime-sorted results.
- **Grep** — stdlib regexp + WalkDir, content / files_with_matches / count modes, glob filter, binary-file skip.
- **WebSearch** — Brave + SerpAPI (injectable base URL).
- **WebFetch** — `net.IP`-based SSRF defense, http/https only, HTML-to-text stripper, 100 KB truncation.

### Harness features (Bundle 13)

- **Connection UX** — persistent banner per connection state (discovering / starting / registering / reconnecting / failed), with diagnostic code surfaced verbatim.
- **Large paste handler** — modal overlay: `[s]ummarize` (via `content.transform`) / `[f]ull` / `[t]runcate` / `[c]ancel` / Esc.
- **File context (`@file`)** — resolves paths and globs into typed protocol.Attachment via Read + Glob tools; `/context` shows the server's context breakdown.
- **Plan / Build / Auto modes** — `/plan`, `/build`, `/auto` slash commands; `Ctrl+P` toggles plan↔build (auto→build).
- **Session commands** — `/sessions`, `/session {resume|clear|fork}`.
- **Model commands** — `/models` (catalog), `/model` (current), `/model <name|auto>` (override).
- **Cross-session command history** — up/down arrow at input top traverses BFF-sourced history. `/history user|project|session` switches scope.
- **MCP client** — stdio JSON-RPC 2.0, subprocess launch with configurable timeout, initialize → tools/list → tool registration. `/mcp status` and `/mcp reconnect <name>`.
- **Theme system** — 24 embedded color palettes (catppuccin, dracula, nord, gruvbox, tokyonight, etc.) ported from OpenCode (MIT, attributed in `NOTICE`). Dynamic `system` theme detects terminal background via OSC 11 / COLORFGBG and generates an adaptive gray scale. Status bar, viewport, and input area are all themed.
- **Sensible keybindings** — default submit key is `Enter` (was unreachable `Ctrl+Enter`); `Tab` toggles plan/build mode alongside `Ctrl+P`; `Ctrl+J` and `Alt+Enter` insert newlines; `/help` lists all slash commands.

### Providers

Three in-tree provider adapters: Anthropic, OpenAI, **Ollama** (new). Ollama adapter supports native `/api/chat` + OpenAI-compatible `/v1/chat/completions` and handles newline-delimited JSON streaming alongside SSE.

### Storage

SQLite schema migrated to v2 with sessions, turns, command history, session locks, and compaction metadata. Clean upgrade path from v1 (proxy-only) databases via v1→v2 upgrade tests.

## Architecture decisions

See `docs/adr/` for the full list; v0.2.0 specifically introduced:

- **ADR-0013** — Bubbletea TUI framework for the harness

The BFF JSON-RPC protocol (framing, error codes, handshake) lives in `FEAT-0008 §Protocol` rather than a standalone ADR; the decision was scoped alongside the feature because the protocol exists only to serve the harness↔BFF pair.

## Protocol

Protocol version `1`. The harness registers with `capabilities.register`; the server negotiates a version and advertises `ServerCapabilities` including `max_attachment_size`. Streaming events (`stream.token` / `stream.complete`), cost updates (`cost.update`), diagnostic events, and tool call notifications all flow as server→harness notifications on the same socket.

## Breaking changes

None for v0.1 users — the proxy (`modeltap start`) and its CLI surface (`logs`, `show`, `export`, `metrics`, `dashboard`, `status`, `service`, `config`) are unchanged. Running `modeltap` with no subcommand changed from "print help" to "launch harness"; the help output is still reachable via `modeltap --help` or `modeltap help`.

## Dependency additions

- `github.com/bmatcuk/doublestar/v4` (Apache 2.0) — Glob
- `github.com/xuri/excelize/v2` (BSD-3) — XLSX reading
- `github.com/ledongthuc/pdf` (BSD-3) — PDF text extraction
- Bubbletea toolkit (`charmbracelet/bubbletea`, `bubbles`, `lipgloss`, `glamour`) for the TUI (MIT)

No AGPL / UniDoc / commercial deps were added; DOCX reading is stdlib-only (archive/zip + encoding/xml) per ADR-0010.

## Related artifacts

The canonical index of every numbered doc that drove, constrains, or informs v0.2.0. Per `.agents/process.md`, only `accepted` features and ADRs authorize code; explorations are upstream rationale and patches are implementation-scoped add-ons. Cross-reference this list against commit prefixes (`WU-`, `PATCH-`, `FEAT-`, `ADR-`, `EXP-`) to audit the release.

### Features

| ID | Title | Status | Role in v0.2.0 |
|----|-------|--------|----------------|
| [FEAT-0008](../../features/0008-bff-server.md) | BFF Server | accepted | **Primary deliverable** — full JSON-RPC BFF implemented (Track A, WU-046–067, 091). |
| [FEAT-0009](../../features/0009-terminal-harness.md) | Terminal Harness | accepted | **Primary deliverable** — Bubbletea harness + 13 tools + MCP (Track B, WU-068–087, 092). |
| [FEAT-0010](../../features/0010-enterprise-auth.md) | Enterprise Auth & Multi-User | proposed | Partial hooks only: TLS/mTLS listener surface (WU-047), `user_id` columns on sessions/turns/command history (WU-045). Full multi-user deferred. |
| [FEAT-0011](../../features/0011-knowledge-integration.md) | Knowledge Integration | proposed | Not delivered in v0.2.0. Prompt Layer 6 is an explicit placeholder (WU-054/055); integration lands in a later release. |
| [FEAT-0012](../../features/0012-skills-and-agent-teams.md) | Skills | proposed | Not delivered. Referenced by EXP-0009 as the prompt-architecture consumer. |
| [FEAT-0013](../../features/0013-agent-teams.md) | Agent Teams | proposed | Not delivered. Supersedes WU-060 (multi-model branching explicitly deferred — see status.md). |

### ADRs

| ID | Title | Status | Role in v0.2.0 |
|----|-------|--------|----------------|
| [ADR-0001](../../../docs/adr/0001-programming-language.md) | Go as primary language | accepted | Pre-existing constraint. |
| [ADR-0002](../../../docs/adr/0002-storage-format.md) | SQLite, WAL mode | accepted | Pre-existing constraint. Schema extended via migration v2 (WU-045, WU-091). |
| [ADR-0003](../../../docs/adr/0003-cli-framework.md) | Cobra CLI | accepted | Pre-existing constraint. |
| [ADR-0004](../../../docs/adr/0004-configuration-management.md) | Viper configuration | accepted | Pre-existing constraint. |
| [ADR-0005](../../../docs/adr/0005-capture-mode-strategy.md) | Always full capture | accepted | Pre-existing constraint. |
| [ADR-0006](../../../docs/adr/0006-multi-provider-support.md) | Multi-provider adapters | accepted | Extended (not formally amended): `FormatMessages` / `FormatToolDefinitions` added to the Provider interface in WU-042. |
| [ADR-0007](../../../docs/adr/0007-usage-metrics.md) | Pre-computed aggregation tables | accepted | Pre-existing constraint; cost events (WU-056) feed the same tables. |
| [ADR-0008](../../../docs/adr/0008-knowledge-layer-architecture.md) | sqlite-vec knowledge layer | accepted | Pre-existing constraint; not active in v0.2.0 (see FEAT-0011). |
| [ADR-0009](../../../docs/adr/0009-mcp-server-for-knowledge-access.md) | MCP stdio transport | accepted | Governs the harness-side MCP client shape (WU-081). |
| [ADR-0010](../../../docs/adr/0010-open-source-license.md) | Apache 2.0 | accepted | Gates dependency choices — drove the stdlib-only DOCX implementation and the no-UniDoc PDF choice. |
| [ADR-0011](../../../docs/adr/0011-contribution-model-and-governance.md) | BDFL + contributor tiers | accepted | Pre-existing constraint. |
| [ADR-0012](../../../docs/adr/0012-background-execution-strategy.md) | Background execution (proxy service) | accepted | Pre-existing (v0.1). The harness auto-start path leverages the same daemonization primitives. |
| [ADR-0013](../../../docs/adr/0013-terminal-ui-framework.md) | Bubbletea TUI framework | proposed | **New in v0.2.0** — framework selection for the harness. |

### Patches

| ID | Title | Status | Role in v0.2.0 |
|----|-------|--------|----------------|
| [PATCH-0001](../../patches/0001-openai-responses-api-support.md) | OpenAI Responses API support | proposed | Not landed in v0.2.0; kept for a later patch release. |
| [PATCH-0002](../../patches/0002-local-inference-support.md) | Local inference (MLX + Ollama) | proposed | Ollama adapter landed via WU-066. MLX provider-type scaffolding in place; not a full deliverable. |
| [PATCH-0003](../../patches/0003-harness-app-conn-mgr-wiring.md) | Harness App ↔ ConnectionManager wiring | approved | Shipped: `ConnSurface` / `deferredSender` wiring between the Bubbletea App and the connection manager. |
| [PATCH-0004](../../patches/0004-secret-prefix-resolver.md) | Secret prefix resolver (`env:` / `file:`) | done | Shipped: `config.ResolveSecret` applied to provider API keys; sample config updated. |
| [PATCH-0005](../../patches/0005-bff-route-via-proxy-default.md) | Route BFF provider traffic through the v0.1 proxy by default | approved | Shipped: harness conversations now flow through the proxy's capture tables; cloud providers default to `http://127.0.0.1:<port>` unless `host:` is set. |
| [PATCH-0006](../../patches/0006-unified-config-data-dir.md) | Unified `~/.modeltap/` config & data directory | done | Shipped: single canonical dir for config, DB, socket, and log; XDG override and one-release legacy fallback preserved. |
| [PATCH-0007](../../patches/0007-dotenv-loader.md) | `.env` loader for provider credentials | done | Shipped: `./.env` and `~/.modeltap/.env` are auto-loaded at startup; opt-out via `MODELTAP_DOTENV=false`. |
| [PATCH-0009](../../patches/0009-root-readme.md) | Root `README.md` | done | Shipped: repo-root `README.md` with pitch, quick start, repo map, contributor entry points, and Apache-2.0 framing. |
| [PATCH-0010](../../patches/0010-makefile-hygiene.md) | Makefile hygiene — PATH-resolved Go + check-only default | done | Shipped: `GO ?= go` (PATH-resolved with env-var override), new `fmt-check` target, `all:` no longer mutates source. |
| [PATCH-0012](../../patches/0012-lint-out-of-default-target.md) | Remove `lint` from the Makefile default target | proposed | `make` no longer requires `golangci-lint`; `make lint` remains explicit for developers and CI. |
| [PATCH-0013](../../patches/0013-sqlite-busy-timeout.md) | Set SQLite `busy_timeout` on every pool connection | proposed | Fixes dropped captures under concurrent upstream traffic; `busy_timeout=5000` added to the DSN so writers briefly block instead of returning `SQLITE_BUSY`. |
| [PATCH-0014](../../patches/0014-bff-shutdown-waitgroup-race.md) | Fix BFF Server `sync.WaitGroup` race between accept and Shutdown | approved | Shipped: `wg.Add(1)` hoisted into the accept loop (caller already holds a wg reference) so concurrent `Shutdown -> wg.Wait` can no longer race with a fresh connection being spawned. Unblocks `go test -race ./...`. |
| [PATCH-0016](../../patches/0016-pr1-ci-test-failures-triage.md) | Fix v0.2.x test suite failures and lint regressions surfaced by PR #1 CI | approved | Shipped: pin `:memory:` SQLite pool to one connection, drop racy `s.ln=nil` in harness mockBFF, bound `BashTool` timeout cleanup with `cmd.WaitDelay`, replace bash brace expansion with portable `awk` in truncation test, rename deprecated `reflect.Ptr` to `reflect.Pointer` for govet `inline`. Unblocks PR #1's CI on Linux. |

### Explorations

Explorations are upstream rationale — they do not authorize code, but they frame v0.2.0's direction.

| ID | Title | Status | Relationship to v0.2.0 |
|----|-------|--------|-----------------------|
| [EXP-0007](../../explorations/0007-multi-model-orchestration.md) | Multi-Model Orchestration | exploring | Motivates FEAT-0013 (deferred); explains why WU-060 was rejected. |
| [EXP-0008](../../explorations/0008-integrated-harness.md) | Integrated Harness — Modeltap as Professional AI Environment | exploring | **Origin story for v0.2.0.** FEAT-0008/0009/0010/0011/0012/0013 all trace back to this exploration. |
| [EXP-0009](../../explorations/0009-harness-prompt-architecture.md) | Harness Prompt Architecture — Lessons from the Claude Code Leak | exploring | Informs the system prompt engine (WU-054/055, seven-layer design). |

## Known follow-ups (carry to v0.2.1 or later)

- **`/drop <file>` and full ContextManager** — per-session attachment bookkeeping (drop, pin, stale detection) isn't implemented; `/context` shows the server's view only.
- **WU-094 Medium / Low follow-ups** — 5 Mediums + 19 Lows from the security review remain; tracked in `status.md` and `.reviews/security/`.
- **Multi-model branching** — WU-060 explicitly deferred to FEAT-0013; `turn.submit` rejects array routes with a clear error.
- **OpenAI Responses API support** — PATCH-0001 proposed; not landed.
- **Full local inference (MLX)** — PATCH-0002 proposed; Ollama shipped, MLX adapter scaffolding only.
- **FEAT-0010 multi-user** — TLS/mTLS listener + `user_id` schema hooks are in; full user management, auth flow, and session isolation checks remain.
