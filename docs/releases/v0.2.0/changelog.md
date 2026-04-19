# v0.2.0 Changelog

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

### Providers

Three in-tree provider adapters: Anthropic, OpenAI, **Ollama** (new). Ollama adapter supports native `/api/chat` + OpenAI-compatible `/v1/chat/completions` and handles newline-delimited JSON streaming alongside SSE.

### Storage

SQLite schema migrated to v2 with sessions, turns, command history, session locks, and compaction metadata. Clean upgrade path from v1 (proxy-only) databases via v1→v2 upgrade tests.

## Architecture decisions

See `docs/adr/` for the full list; v0.2.0 specifically introduced:

- **ADR-0012** — JSON-RPC 2.0 BFF protocol (framing, error codes, handshake)
- **ADR-0013** — Bubbletea TUI framework for the harness

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

## Known follow-ups (carry to v0.2.1 or later)

- **WU-061 compaction** — server-side compaction handler. Requires a trim-heuristic + harness UX design pass.
- **Plan-mode tool interception** — `PlanAccumulator.Append` isn't called yet. Needs the harness tool executor wired into the ConnectionManager's `tool.call` event bridge.
- **`/drop <file>` and full ContextManager** — per-session attachment bookkeeping (drop, pin, stale detection) isn't implemented; `/context` shows the server's view only.
- **Streaming event delivery to the App** — the WU-088 e2e test asserts turn dispatch to the mock provider but not `StreamTokenMsg` / `StreamCompleteMsg` delivery back into the App. A small wiring step in the event bridge covers this.
- **WU-094 security review** — formal OWASP-style pass.
- **WU-095 performance benchmarks** — scenario design + budgets.
