# Track B: FEAT-0009 Terminal Harness

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Release:** v0.2.0
**WU Range:** WU-068 through WU-087, plus WU-092 (21 work units)
**Depends on:**
- Harness-local WUs (WU-068–WU-072, WU-075–WU-079): only WU-039 (protocol core messages).
- Protocol client / streaming / session-aware WUs (WU-073, WU-074, WU-080+, WU-092): WU-039, WU-040, WU-041 (and, for WU-092, WU-091).
**Can parallelize with:** Track A (BFF Server)

## Bubbletea Scaffold

### WU-068: Bubbletea App Scaffold — Main Model and Layout
**Size:** Medium | **Dependencies:** WU-039

NEW `internal/harness/app.go` — main Bubbletea Model with three zones: status bar (bottom), input area (above status bar), conversation viewport (rest). `internal/harness/model.go` — app state struct, Init/Update/View. Window resize. Focus management. Basic key bindings (quit, scroll). Static placeholder content (no server yet).

**Done:** `modeltap` launches Bubbletea app with three zones. Resize works. Keyboard navigation works.

### WU-069: Status Bar Component
**Size:** Small | **Dependencies:** WU-068 | **Parallelizes with:** WU-070, WU-071

NEW `internal/harness/statusbar.go` — renders: connection indicator (`[●]`/`[◐]`/`[↻]`/`[✗]`), mode indicator (`[plan]`/`[build]`/`[auto]`), model name (with `[override]`), context usage, session cost, call timer. Lipgloss styling.

**Done:** All fields render. Mode, model, context, cost, timer update from state. Connection indicator reflects 4 states.

### WU-070: Input Area Component
**Size:** Medium | **Dependencies:** WU-068 | **Parallelizes with:** WU-069, WU-071

NEW `internal/harness/input.go` — multi-line text input (Bubbletea textarea). Configurable submit key. Command detection (`/` prefix). `@file` syntax detection. Drag-and-drop path detection. Up/down arrow keybindings for history traversal (history source is supplied by WU-092; this WU only wires the keybindings and rendering). Paste size threshold detection.

**Done:** Multi-line works. Submit triggers callback. `/` commands detected. `@file` detected. History arrow keybindings wired with pluggable source interface. Large paste detected.

### WU-071: Conversation Viewport Component
**Size:** Medium | **Dependencies:** WU-068 | **Parallelizes with:** WU-069, WU-070

NEW `internal/harness/viewport.go` — scrollable viewport (Bubbletea viewport). Auto-scroll-to-bottom. Manual scroll-up (keyboard, pauses auto-scroll). Snap back on new input. Message boundaries (user vs. assistant). Per-turn metadata line. Model routing indicator before each response.

**Done:** Scrolls. Auto-scroll works. Manual scroll-up pauses. Snap-back works. Boundaries visible. Metadata rendered.

### WU-072: Streaming Markdown Rendering
**Size:** Medium | **Dependencies:** WU-071 | **Parallelizes with:** WU-073

NEW `internal/harness/markdown.go` — Glamour-based. Debounced redraw (50ms). Headings, code blocks (syntax highlight), lists, bold/italic, inline code. Partial markdown during streaming. Final clean render on completion. Chunked re-rendering for long responses.

**Done:** All markdown elements render. Streaming progressive without flicker. Final render clean. Performance acceptable to 10KB.

## Protocol Client and Connection

### WU-073: Protocol Client — JSON-RPC over Socket/TLS
**Size:** Medium | **Dependencies:** WU-039, WU-040, WU-041 | **Parallelizes with:** WU-069-072

NEW `internal/harness/client.go` — JSON-RPC client. Unix socket or TLS. Sends requests, receives responses and streaming events. Correlation by `id` and `turn_id`. Event callbacks. Reconnection support.

**Done:** Connects, sends, receives. Streaming events dispatched. Reconnect works. Tests with mock server.

### WU-074: Connection Manager — Lifecycle, Auto-Start, Heartbeat
**Size:** Large | **Dependencies:** WU-073 | **Parallelizes with:** WU-072

NEW `internal/harness/connection.go` — lifecycle state machine (9 states). Local auto-start (detect socket, stale handling, start service/subprocess). Heartbeat (15s ping, track pongs). Exponential backoff (1s→30s, 10 retries). State change notifications to UI.

**Done:** All transitions tested. Auto-start handles stale sockets. Heartbeat detects missed pongs. Reconnection works. UI updates.

## Tools

All tools are harness-local and can be built without any server connection.

### WU-075: Tool Framework and Permission Model
**Size:** Medium | **Dependencies:** WU-068 | **Parallelizes with:** WU-073, WU-074

NEW `internal/harness/tools/framework.go` — tool executor interface, registry, permission enforcer. Three levels (default, accept-edits, autonomous). Risk level mapping. Permission prompt flow. Dangerous command detection. `internal/harness/tools/permission.go`. Result formatting (success, rejected, error).

**Done:** Framework registers, checks permissions, dispatches. Three levels work. Dangerous detection works. Prompt integrates with Bubbletea.

### WU-076: Read Tools (text, PDF, DOCX, image, spreadsheet)
**Size:** Large | **Dependencies:** WU-075 | **Parallelizes with:** WU-077, WU-078, WU-079

NEW `internal/harness/tools/read.go` — unified Read with file type detection (extension + magic bytes). Text: `os.ReadFile` + line numbering. PDF: `pdfcpu`/`unipdf`. DOCX: `unioffice`. Images: base64 + MIME. Spreadsheets: `excelize`/`encoding/csv`.

**Done:** All 5 types work. Auto-detection correct. Tests with sample files.

### WU-077: Write and Edit Tools
**Size:** Medium | **Dependencies:** WU-075 | **Parallelizes with:** WU-076, WU-078, WU-079

NEW `internal/harness/tools/write.go` — Write with snapshot. `internal/harness/tools/edit.go` — Edit with exact match, uniqueness check, requires prior Read. File tracking.

**Done:** Write creates/overwrites with snapshot. Edit requires Read. Uniqueness check works. Ambiguous match fails.

### WU-078: Bash and Git Tools
**Size:** Medium | **Dependencies:** WU-075 | **Parallelizes with:** WU-076, WU-077, WU-079

NEW `internal/harness/tools/bash.go` — `exec.Command`, stdout/stderr capture, timeout, working directory. `internal/harness/tools/git.go` — shells out to `git`, read vs. mutation classification. Dangerous command detection.

**Done:** Bash executes and captures. Git operations work. Classification correct. Dangerous detected.

### WU-079: Glob, Grep, WebSearch, WebFetch Tools
**Size:** Medium | **Dependencies:** WU-075 | **Parallelizes with:** WU-076, WU-077, WU-078

NEW `internal/harness/tools/glob.go` (`doublestar` for `**`), `grep.go` (regexp + file walker), `websearch.go` (Brave/SerpAPI), `webfetch.go` (`net/http` + HTML-to-text).

**Done:** Glob matches `**`. Grep returns matches with context. WebSearch returns structured results. WebFetch converts HTML.

## Features

### WU-080: Plan/Build/Auto Modes with Ctrl+P Toggle
**Size:** Large | **Dependencies:** WU-075, WU-070, WU-071 | **Parallelizes with:** WU-079, WU-081

NEW `internal/harness/modes.go` — mode state. `/plan`, `/build`, `/auto` commands. Ctrl+P toggle. Mode on every `turn.submit`. Plan mode: intercept writes, accumulate plan display, execute reads. Plan UI: [a]pprove, [e]dit, [s]tep through, [c]ancel. Step-through with per-step pause. Approve switches to build with plan as context.

**Done:** Mode switching via commands and Ctrl+P. Plan accumulates. Approve/step/edit/cancel work. Indicator updates.

### WU-081: MCP Client — stdio Transport and Tool Discovery
**Size:** Medium | **Dependencies:** WU-075, WU-073 | **Parallelizes with:** WU-080

NEW `internal/harness/mcp.go` — MCP client (stdio). Connects at startup. Discovers tools. Maps to protocol catalog (`mcp:<server>` namespace). `capabilities.update` on connect/disconnect. Config parsing.

**Startup behavior (degraded mode):** MCP discovery is **not** startup-blocking. The harness launches and connects to the BFF using only the built-in tool catalog if MCP is unavailable or slow. Per-server semantics:

- Each configured MCP server is launched asynchronously with a configurable start timeout (default: 5s).
- On success, the server's tools are added to the catalog and a `capabilities.update` is sent to the BFF.
- On failure or timeout, the harness renders a transient banner (`MCP server <name> failed: <reason>`) and continues without that server's tools. The connection manager retries in the background with exponential backoff.
- `/mcp status` command lists configured servers with per-server state (connected, retrying, failed). `/mcp reconnect <name>` forces a retry.

**Done:** Connects. Discovery works. Namespace correct. Dynamic update works. Config parses. MCP failures never block harness launch or BFF connection. Banner and `/mcp` commands behave as specified. Tests cover: all MCP servers healthy, one MCP server fails at startup, all MCP servers fail, successful late reconnect after startup failure.

### WU-082: File Context Management
**Size:** Medium | **Dependencies:** WU-076, WU-070 | **Parallelizes with:** WU-080, WU-081

NEW `internal/harness/context.go` — `@path/to/file` parsing, `@src/**/*.go` glob expansion, drag-drop detection (burst paste of absolute paths). Attachment formatting (raw + content + content_type + transform). `/context` and `/drop` commands.

**Done:** `@file` loads. Globs expand. Drag-drop detected. Attachments formatted. Commands work.

### WU-083: Large Paste Handling
**Size:** Small | **Dependencies:** WU-070, WU-073 | **Parallelizes with:** WU-082

NEW `internal/harness/paste.go` — size detection (2KB threshold). Preview (first 5 lines). User choice: summarize/full/truncate/cancel. Summarize uses `content.transform`. Paste payload formatted.

**Done:** Detection works. Preview correct. All choices work. Summarize uses BFF.

### WU-084: Session Explorer and Session Commands
**Size:** Large | **Dependencies:** WU-073, WU-071, WU-068 | **Parallelizes with:** WU-080, WU-085

NEW `internal/harness/explorer.go` — Bubbletea component. Queries `session.list` on launch. Renders sessions with summary/context/cost. TUI navigation. Details view (`session.details`). Resume, fork, compact-before-resume. Auto-resume if single session + no events. `/sessions` re-shows explorer. Server event notifications. `/compact` with interactive category UI. `/cost`, `/trace`, `/clear`, `/fork`, `/help`.

**Done:** Explorer on launch. List renders. Details with timeline. Resume/fork/compact work. All `/` commands work.

### WU-085: Model Commands and Multi-Model Branch Display
**Size:** Medium | **Dependencies:** WU-073, WU-071 | **Parallelizes with:** WU-084

NEW `internal/harness/models.go` — `/model <name>`, `/model auto`, `/models` display (providers, models, routing tree, override). Multi-model rendering: progressive completion, per-reviewer sections, spinners, `branch_id` routing, aggregate cost/timing.

**Done:** Model commands work. Display formatted. Branch display progressive. Tokens routed by `branch_id`. Spinners work.

### WU-086: Connection UX — States, Banners, Diagnostics
**Size:** Medium | **Dependencies:** WU-074, WU-069 | **Parallelizes with:** WU-084, WU-085

NEW `internal/harness/connux.go` — status bar connection states (green/yellow/arrow/x). Transient banners (starting, authenticating, registering). Degraded banner. Reconnecting display (attempt count, timing). Failed state (diagnostic code, cause, suggestion). `/status`, `/reconnect`, `/session unlock`. Multi-model recovery display.

**Done:** All states render. Banners appear/disappear. Diagnostics actionable. Commands work.

### WU-087: Harness Integration Tests with Mock Server
**Size:** Large | **Dependencies:** all Track B

NEW `internal/harness/integration_test.go` — E2E with mock BFF. Tests: launch, connect, register (version + project context), turn with streaming, tool round-trips (all 13 tools), permission prompts, plan mode, session explorer, model display, compaction UI, command history traversal, connection states, diagnostics.

**Done:** Integration tests cover FEAT-0009 success criteria. Mock covers all protocol messages. All pass.

## Command History (Harness Side)

### WU-092: BFF-Sourced Command History Traversal
**Size:** Medium | **Dependencies:** WU-070, WU-073, WU-091 | **Parallelizes with:** WU-080-086

Required to satisfy FEAT-0009's "command history traversal (up/down arrows) sourced from the BFF (cross-session, cross-project)".

Implements:

- NEW `internal/harness/history.go` — client-side history controller.
  - Fetches history on harness launch via `history.list` (default scope `user`, limit 200).
  - Caches entries locally with cursor metadata; pages older entries on demand as the user arrows past the cache.
  - Up/down arrow traversal integrates with WU-070 input area: arrows move through the cached window; typing over the current entry commits it as a new draft.
  - Scope toggle: `/history project`, `/history session`, `/history user` commands change the server-side scope and refresh.
  - On `turn.submit` the harness relies on the server's automatic append (WU-091); no client-side append is required.
- Replaces the "local input history traversal" previously implied in WU-070: WU-070 wires the keybindings and renders the current entry, but the source of truth is WU-092.

**Done:** Up/down arrows traverse BFF-sourced history across sessions and projects. Scope commands work. Paging older entries works. Works offline with an empty history (degrades gracefully if `history.list` unavailable). Tests with mock BFF cover scope switches, paging, and traversal wrap-around.

## Critical Path

```
068 → 075 → 076 → 082 → 084 (scaffold → tools → file context → session explorer)
068 → 071 → 072 (scaffold → viewport → markdown)
073 → 074 (protocol client → connection manager)
```

Three parallel chains. Longest: scaffold → tools → features (~8 sequential WUs). WU-092 (command history) runs off WU-070/WU-073/WU-091 and does not extend the critical path.
