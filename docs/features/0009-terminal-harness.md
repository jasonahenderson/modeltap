---
feature: FEAT-0009
title: Terminal Harness
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: BFF Server
adr-constraints:
  - ADR-0001: Go as primary language
  - ADR-0003: Cobra CLI framework
  - ADR-0013: Terminal UI framework (Bubbletea from day one)
promoted-from:
  - EXP-0008: Integrated Harness
---

# FEAT-0009: Terminal Harness

## Problem

Modeltap v1 has no interactive interface — it is a background proxy with CLI query commands. The integrated harness (EXP-0008) needs a terminal UI that lets users have conversations with AI models through the modeltap server. Without it, the BFF server (FEAT-0008) has no client, and the product's differentiators (cross-model memory, cost-aware routing, knowledge enrichment) have no user-facing surface.

Existing terminal AI tools (Claude Code, aider, Codex) are either single-provider, lack cross-model memory, or run in sandboxed environments. The modeltap harness is a thin client that delegates intelligence to the BFF — it handles terminal rendering, local tool execution, and permission enforcement while the server handles everything else.

## Solution

A terminal UI that connects to the modeltap server via the harness protocol (FEAT-0008), sends conversation turns, streams responses, executes tools locally, and enforces permissions. The harness is implemented in Go per ADR-0001, using Bubbletea (Charm ecosystem) per ADR-0013 for terminal rendering — styled markdown via Glamour, scrollable viewport, persistent status bar, and interactive UI components.

## Key Capabilities

### Conversation Interface

- Multi-line text input with cursor movement, cut/copy/paste, and configurable submit key
- Streaming markdown output rendered with terminal styling (headings, code blocks, lists, bold/italic)
- Command history traversal (up/down arrows) sourced from the BFF (cross-session, cross-project)
- Scrollable conversation viewport with auto-scroll-to-bottom on new output

### Tool Execution

The harness executes tools locally on the user's machine. The BFF forwards model tool calls; the harness decides whether to execute based on the current permission level and execution mode.

**Built-in tool set:**

All tools are required for the initial release. There is no phased tool rollout.

| Tool | Description | Default Permission | Go Implementation |
|------|-------------|-------------------|-------------------|
| Read | Read text file contents | Auto-allow | `os.ReadFile`, line numbering |
| ReadPDF | Extract text from PDF files | Auto-allow | `pdfcpu` or `unipdf` |
| ReadDOCX | Extract text from DOCX files | Auto-allow | `unioffice` |
| ReadImage | Encode image for vision-capable models | Auto-allow | `encoding/base64` + MIME detection |
| ReadSpreadsheet | Parse XLSX/CSV as structured text | Auto-allow | `excelize` / `encoding/csv` |
| Write | Create or overwrite files | Prompt first use | `os.WriteFile` with snapshot |
| Edit | Modify existing files (exact string match) | Prompt first use | String replacement with uniqueness check |
| Bash | Execute shell commands | Prompt per command | `exec.Command`, stdout/stderr capture |
| Glob | Find files by pattern | Auto-allow | `filepath.Glob` / `doublestar` |
| Grep | Search file contents by regex | Auto-allow | `regexp` + file walker, or embed ripgrep |
| Git | Git operations | Prompt for mutations | Shell out to `git` CLI |
| WebSearch | Search the web | Prompt first use | External search API (Brave, SerpAPI) |
| WebFetch | Fetch URL contents as text | Prompt per domain | `net/http` + HTML-to-text extraction |

The Read* tools are unified behind a single `Read` tool call from the model's perspective — the harness detects the file type and applies the appropriate extraction. The model calls `Read` with a file path; the harness returns extracted text regardless of whether the source is `.go`, `.pdf`, `.docx`, `.png`, or `.xlsx`. For images, the harness returns a base64-encoded representation if the current model supports vision, or an error explaining the model does not support images.

**Tool safety guardrails:**
- Edit requires exact string matching — prevents hallucinated file overwrites
- Edit fails if the file has not been Read first in the session
- Write snapshots the original file before overwriting (reversibility)
- Bash displays the full command before execution, requiring explicit approval
- Tool descriptions instruct the model on when to use each tool and when NOT to (e.g., "use Read instead of cat via Bash"). These descriptions are part of the system prompt Layer 2 (FEAT-0008).

**Tool registration with server:** On connection, the harness sends `capabilities.register` (FEAT-0008) declaring all available tools — built-in tools and any MCP-discovered tools — with their names, descriptions, input schemas, and permission levels. The server uses this catalog to populate the model's tool definitions. Only tools the harness has registered are available to the model. When MCP servers connect or disconnect, the harness sends `capabilities.update` to add or remove tools dynamically.

**MCP tool discovery (required):** The harness connects to configured MCP servers (stdio transport) and discovers their tools at startup. MCP tools appear alongside built-in tools with the same permission enforcement and are included in capability registration. MCP is the extension mechanism — users add domain-specific tools, database access, CI/CD integration, and any custom tooling via MCP servers without modifying the harness.

### Permission Model

Three permission levels, matching the proven pattern from Claude Code:

- **Default**: prompt for each tool type on first use. Read-only tools (Read, Glob, Grep) are auto-allowed.
- **Accept edits**: auto-approve file operations (Read, Write, Edit, Glob, Grep). Bash still prompts.
- **Autonomous**: auto-approve all tools within safety limits. Dangerous bash commands (rm -rf, git push --force) still prompt.

Permissions are enforced entirely in the harness. The BFF has no knowledge of the current permission level and cannot override it.

### Execution Modes

The harness operates in one of three modes. The current mode is always visible in the status bar and affects both the system prompt (Layer 5, server-side) and tool call handling (harness-side).

**Visual mode indicator** — always visible in the status bar:

```
[plan] claude-opus-4-6 | 47% ctx | $0.42 | ⏱ 3.2s
```

```
[build] claude-opus-4-6 | 47% ctx | $0.42 | ⏱ 3.2s
```

```
[auto] claude-opus-4-6 [override] | 47% ctx | $0.42 | ⏱ 3.2s
```

**Mode switching** — fast and immediate:
- `/plan`, `/build`, `/auto` commands
- Keyboard shortcut: `Ctrl+P` toggles between plan and build (the two most common modes)
- Mode change takes effect on the next turn — mid-turn mode switches do not interrupt a streaming response
- The harness sends the current mode as a `mode` field on every `turn.submit` so the server injects the appropriate Layer 5 prompt

**Plan mode** (`/plan`):

The model is instructed via the mode prompt (FEAT-0008 Layer 5) to analyze and propose rather than execute. The harness handles tool calls as follows:

- Read-only tools (Read, Glob, Grep, Git status/log/diff): execute silently. The model needs context to plan well.
- Write/Edit/Bash/Git mutation tools: intercepted by the harness, collected into the plan display, NOT executed.

As the model works, the plan accumulates:

```
[plan] claude-opus-4-6 (routing: coding)
→ Analyzing task...
  Read: internal/api/router.go ✓
  Read: internal/middleware/ ✓

Proposed plan:
  1. Create internal/middleware/ratelimit.go
     - Token bucket implementation
     - Per-endpoint configuration
  2. Edit internal/api/router.go
     - Add rate limit middleware to route chain
  3. Edit internal/config/config.go
     - Add rate limit config section
  4. Create internal/middleware/ratelimit_test.go
     - Table-driven tests for token bucket
     - Integration test for middleware chain
  5. Run: go test ./internal/middleware/...

[a]pprove and execute  [e]dit  [s]tep through  [c]ancel
```

**Approve and execute**: the harness switches to build mode, sends a new `turn.submit` with `mode: "build"` and the approved plan as context. The model re-executes with the plan as a reference.

**Step through**: executes one plan step at a time. After each step, the harness pauses:

```
Step 1/5: Create internal/middleware/ratelimit.go ✓ (87 lines)
[n]ext  [v]iew file  [e]dit plan  [a]pprove remaining  [c]ancel
```

Step-through gives the user maximum control — inspect each change before proceeding.

**Edit**: the user modifies the plan before execution — reorder steps, remove steps, add constraints, change the approach. The edited plan is sent with the build-mode turn.

**Build mode** (`/build`, default):

The model is instructed to execute directly. Tool calls flow through the normal permission model:
- Default permission: prompt per tool type on first use
- Accept-edits: auto-approve file operations
- Autonomous: auto-approve all within safety limits

No plan accumulation. Tool calls execute as they arrive. This is the default mode.

**Auto mode** (`/auto`):

Same as build mode, but the model is additionally instructed to proceed without requesting confirmation. The harness auto-approves tool calls within the configured permission level without per-action prompts. Dangerous operations (force push, rm -rf) still prompt regardless of mode.

### File Context Management

- `@path/to/file` syntax to attach files to a message
- `@src/**/*.go` glob patterns for multiple files
- Drag and drop from file manager: most modern terminals (iTerm2, Kitty, WezTerm, Ghostty, Terminal.app) paste the file path as text when a file is dragged onto the terminal. The harness detects this pattern (absolute path pasted in a burst, file exists on disk) and converts it to a file attachment. Multiple files are detected from space-separated or newline-separated paths:
  ```
  Attached 3 files (7.1 KB):
    internal/middleware/auth.go (2.4 KB)
    internal/config/config.go (1.8 KB)
    internal/api/router.go (2.9 KB)
  ```
- `/context` command shows files in context, knowledge injections, and token budget
- `/drop <file>` removes a file from context

**File format support:**
- Text files: direct inclusion
- PDF: text extraction
- DOCX: text extraction
- Images: base64-encoded for vision-capable models
- Spreadsheets (XLSX/CSV): parsed as structured text

**Attachment wire format**: the harness always sends both the raw content and any transformed representation in the `turn.submit` payload. Specifically:
- `attachments[].raw`: the original file bytes (for capture integrity and reproducibility)
- `attachments[].content`: the extracted/transformed text (for model context)
- `attachments[].content_type`: original MIME type
- `attachments[].transform`: what processing was applied ("pdf_text_extract", "docx_text_extract", "base64_encode", "none")

The server captures the raw payload per ADR-0005 and uses the transformed content for model prompt assembly. This ensures capture correctness (the raw file is always preserved) while allowing the harness to handle format-specific extraction.

### Large Paste Handling

When the harness detects a paste exceeding the configurable threshold (default: 2KB):

- Show preview (first 5 lines + line count + byte size)
- Offer: summarize (via cheap model), include full, truncate (first N lines), cancel
- **Full or truncate**: the harness includes the chosen representation directly in `turn.submit` with `paste.raw` (full content for capture) and `paste.content` (user's choice).
- **Summarize**: the harness sends `content.transform` (FEAT-0008) to the BFF with the raw paste and `transform: "summarize"`. The BFF routes to the cheap model, captures the raw content per ADR-0005, and returns the summary. The harness then includes the summary in `turn.submit`. This preserves the rule that the harness never speaks provider protocols directly — all model calls go through the BFF, even pre-turn transformations. The `content.transform` call is captured and cost-attributed separately from the conversation turn.

### Status Bar

Persistent display at bottom of terminal:

```
claude-opus-4-6 | 47% context (38K/80K) | $0.42 session | timer 3.2s
```

- Current model name (shows `[override]` when user has overridden routing policy)
- Context window usage (percentage and absolute tokens)
- Running session cost
- Timer for the current model call (starts on send, stops on stream complete)

**Model routing indicator**: before each response begins streaming, the harness displays the `model.selected` event:

```
→ claude-opus-4-6 (routing: coding)
```

Or when overridden:

```
→ claude-opus-4-6 [override]
```

This ensures the user always knows which model is about to respond and why it was chosen, before any tokens appear.

**Per-turn metadata** displayed inline after each response:

```
--- claude-opus-4-6 | 1,247 in / 3,891 out | $0.08 | 4.1s ---
```

**Context pressure warnings** at configurable thresholds. When the server sends `compact.suggest`, the harness displays:

```
⚠ Context at 78% — consider /compact to review and prune
```

### Interactive Compaction

When the user types `/compact`, the harness sends `session.compact` to the server and renders the returned `compact.plan`:

```
> /compact

Analyzing context (38K / 80K tokens, 47%)...

Category breakdown:
  [A] Architecture decisions (3.2K)    ★ high value, referenced 4x
  [B] Debugging session (8.1K)         ○ resolved, not referenced since turn 6
  [C] Test iteration (6.4K)            ○ tests passing, 3 retry cycles
  [D] File contents (12.8K)            ◐ 4 files, 2 still relevant
  [E] Planning discussion (4.1K)       ◐ plan approved, some items pending
  [F] System/tool overhead (3.4K)      ○ tool call metadata, low value

Suggestions:
  B → summarize (8.1K → ~0.5K)  "fixed DB connection timeout by..."
  C → summarize (6.4K → ~0.3K)  "tests pass after retry config fix"
  D → drop stale (12.8K → 6.2K) keep handler.go, types.go; drop config.go, old router.go
  F → drop (3.4K → 0)           tool call metadata, reproducible from capture

Estimated savings: 21.8K tokens (57% reduction)
Retained: A (full), D (partial), E (full)

[a]pply all  [s]elect  [e]dit  [c]ancel
```

**Apply all**: sends `compact.apply` with the server's suggested actions for every category.

**Select mode**: the user chooses an action per category:

```
> s
Select action per category:
  [B] Debugging session (8.1K)     [s]ummarize  [k]eep  [d]rop  [p]in
> s
  [C] Test iteration (6.4K)        [s]ummarize  [k]eep  [d]rop  [p]in
> d
  [D] File contents (12.8K)        [s]elect files...
    config.go (3.1K)               [k]eep  [d]rop
> d
    old router.go (3.5K)           [k]eep  [d]rop
> d
    handler.go (3.4K)              [k]eep  [d]rop
> k
    types.go (2.8K)                [k]eep  [d]rop
> k
  [F] Tool overhead (3.4K)         [s]ummarize  [k]eep  [d]rop  [p]in
> d

Applied. Context: 38K → 19.4K (24% of window). Full history in knowledge layer.
```

**Edit mode**: opens the compaction plan as editable text for advanced users.

Categories marked with ★ (high value) or as pinned are not suggested for removal but can still be compacted if the user explicitly chooses. The user always has final authority over what stays in context.

**Auto-compaction notification**: when the server auto-compacts at 92%, the harness displays what happened:

```
⚠ Context at 92% — auto-compacted 4 categories
  Debugging session: summarized (8.1K → 0.5K)
  Test iteration: dropped (6.4K)
  Tool overhead: dropped (3.4K)
  Stale files: dropped config.go, old router.go (6.6K)
  Total freed: 22.0K tokens. Use /compact to review or adjust.
```

### Session Explorer

When the user launches `modeltap`, the harness queries `session.list` and presents recent sessions:

```
Recent sessions for ~/Projects/modeltap:

  1. [active]  2h ago   47% ctx   $1.23   "rate limiting implementation"
     Last: backend agent completed, reviewer found 2 issues
     Files: ratelimit.go, router.go, ratelimit_test.go

  2. [paused]  1d ago   23% ctx   $0.41   "auth middleware JWT migration"
     Last: plan approved, implementation not started
     Files: auth.go, config.go

  3. [paused]  3d ago   61% ctx   $2.87   "database connection pooling"
     Last: tests passing, PR ready for review
     Files: pool.go, pool_test.go, db.go

Other projects:
  4. [active]  5h ago   ~/Projects/api-gateway   $0.18   "CORS config"

[1-4] resume  [n]ew session  [d]etails <n>
```

**Details view** (`d 2`):

```
Session: auth middleware JWT migration
  ID:          sess_a8f3c2
  Created:     2026-04-13 14:22
  Last active: 2026-04-14 09:15
  Model:       claude-opus-4-6 (no override, routing: default)
  Turns:       12
  Context:     23% (18.4K / 80K)
  Cost:        $0.41

  Timeline:
    Turn 1:  Read auth.go, config.go
    Turn 2:  Planned JWT migration (6 steps)
    Turn 3:  User approved plan
    Turn 4-7: [compacted] researched JWT libraries
    Turn 8:  Decision: use golang-jwt/jwt/v5
    Turn 9:  Started implementation — paused by user

  Pinned:
    - "Use golang-jwt/jwt/v5, not dgrijalva"
    - "Token expiry: 15min access, 7d refresh"

  Files touched: auth.go (read), config.go (read)
  Files modified: none yet

[r]esume  [f]ork  [c]ompact before resume  [b]ack
```

"Compact before resume" runs interactive compaction (see above) before restoring the session, which is useful when returning to a session days later with stale context.

**Server event notifications** on resume:

```
Resuming session: "rate limiting implementation"
  ⚠ Server restarted since last session (2026-04-15 03:00)
  ⚠ Auto-compacted: debugging session (8.1K → 0.5K), stale files dropped
  Context: 47% → 31% (freed 12.8K tokens)
  Use /compact to review what changed.

>
```

If the session has only one recent session for the current project and no server events to report, the harness resumes it directly without showing the explorer.

### Model Commands

**`/model <name>`** — override the routing policy for this session:

```
> /model claude-opus-4-6
Model override set: claude-opus-4-6
All turns will use this model until cleared with /model auto.
```

**`/model auto`** — clear the override and return to routing policy:

```
> /model auto
Model override cleared. Routing policy will select models per turn.
```

**`/models`** — list available models with routing roles, capabilities, and cost:

```
> /models

Provider endpoints:
  anthropic       cloud    ✓ connected
  openai          cloud    ✓ connected
  ollama-local    local    ✓ 3 models
  ollama-gpu      remote   ✓ 2 models
  mlx-local       local    ✗ not running

Available models:
                                   Provider         Cost/1K       Context
  claude-opus-4-6     anthropic       $0.015/$0.075    200K   tool_use, vision
    Strongest reasoning and code generation
  claude-sonnet-4-6   anthropic       $0.003/$0.015    200K   tool_use, vision
    Fast, balanced for most tasks
  gpt-5.4             openai          $0.010/$0.030    128K   tool_use, vision
    Strong reviewer, different perspective
  llama-3.1-8b        ollama-local    $0.000/$0.000    128K   tool_use
    Fast local model, simple tasks
  llama-3.1-70b       ollama-gpu      $0.000/$0.000    128K   tool_use
    Strong local model, security review
  nomic-embed-text    ollama-local    $0.000/$0.000      —    embedding

Routing policy:
  default                → claude-sonnet-4-6

  backend:
    default              → claude-opus-4-6
    design               → claude-sonnet-4-6
    code                 → claude-opus-4-6
    review               → gpt-5.4
    review_security      → llama-3.1-70b (local)

  frontend:
    default              → claude-opus-4-6
    review               → gpt-5.4

  infrastructure:
    default              → claude-sonnet-4-6
    review               → [gpt-5.4, claude-opus-4-6]  ×2 parallel

  planning:
    default              → claude-sonnet-4-6
    review               → gpt-5.4

Current: routing policy (no override)
```

When a model override is active:

```
Current: claude-opus-4-6 [override] — /model auto to clear
```

**Multi-model review display**: when the BFF routes to a multi-model role, each reviewer runs as a background thread with progressive completion. Results stream as each reviewer finishes — the harness does not wait for all reviewers before showing any output:

```
→ code_review (2 reviewers)
  [gpt-5.4]          ●●● reviewing...
  [claude-opus-4-6]  ●●● reviewing...

[gpt-5.4] ✓ complete (3.1s, $0.08)
  ⚠ Rate limiter uses in-memory store — won't scale across instances.
  ✓ Token bucket implementation is correct.
  ✓ Config validation handles edge cases.

  [claude-opus-4-6]  ●●● reviewing...

[claude-opus-4-6] ✓ complete (4.2s, $0.12)
  ✓ Implementation looks solid.
  ⚠ Missing rate limit headers in response (X-RateLimit-Remaining).
  ⚠ Health check endpoint should bypass rate limiting.

─── 2 reviewers | $0.20 total | 4.2s ───
```

Each reviewer's output is rendered in its own labeled section as soon as it completes. Spinners show which reviewers are still working. The BFF streams `branch.started`, `token.delta` (tagged with `branch_id`), `branch.complete`/`branch.error`, and aggregate `turn.complete` events (FEAT-0008). The harness routes `token.delta` events to the appropriate section by `branch_id`. This is the same pattern as Claude Code's background sub-agents — parallel work with progressive results.

### Session Commands

| Command | Description |
|---------|-------------|
| `/compact` | Interactive context analysis and compaction |
| `/clear` | Fresh context within the same session |
| `/fork` | Branch session into independent continuation |
| `/model <name>` | Override routing policy for this session |
| `/model auto` | Clear override, return to routing policy |
| `/models` | List available models with roles, cost, and capabilities |
| `/plan` | Enter plan mode |
| `/build` | Enter build mode (default) |
| `/auto` | Enter auto mode |
| `/cost` | Show session cost breakdown |
| `/trace` | Show model routing for last turn |
| `/context` | Show files, knowledge injections, context budget |
| `/drop <file>` | Remove file from context |
| `/sessions` | Show session explorer (also shown on launch) |
| `/help` | Show available commands |

### Connection UX and Recovery

The harness renders the BFF connection lifecycle (FEAT-0008) as visible UI state. The user always knows the connection status and what to do when something goes wrong.

**Status bar connection indicator** (leftmost element):

```
[●] [build] claude-opus-4-6 | 47% ctx | $0.42 | ⏱ 3.2s       # ready (green)
[◐] [build] claude-opus-4-6 | 47% ctx | $0.42                 # degraded (yellow)
[↻] reconnecting (attempt 3/10)...                              # reconnecting
[✗] connection failed — MT-CONN-001 service not running         # failed (red)
```

**Transient state banners** for states the user doesn't normally see:

```
Starting local server...                              # starting
Authenticating (OIDC)...                              # authenticating
Registering tools (14 built-in + 3 MCP)...            # registering
```

These banners appear during connection establishment and disappear when `ready` is reached.

**Degraded state**: when `connection.health` reports unhealthy dependencies, the status bar shows `[◐]` and a banner describes what's degraded:

```
⚠ Provider ollama-gpu unavailable — routing fallback active. /models for details.
```

The user can continue working — degraded means the server is reachable but some routes may fail or fall back.

**Reconnecting state**: during automatic reconnect, the harness shows retry progress:

```
↻ Connection lost. Reconnecting (attempt 2/10, next in 2s)...
```

If a turn was in-flight when the connection dropped, the harness shows what happened after `session.sync`:

```
↻ Reconnected. Recovering session...
  In-flight turn: model requested Edit on auth.go, awaiting tool result.
  Resuming from pending tool call.
```

Or if the turn completed while disconnected:

```
↻ Reconnected. Turn completed while disconnected.
  [response displayed]
```

**Failed state**: when auto-recovery is exhausted or a terminal error occurs, the harness renders the diagnostic with suggested action:

```
✗ MT-CONN-004 version_mismatch: server protocol v2, harness supports v1.
  → Update the harness: modeltap upgrade
```

```
✗ MT-CONN-006 auth_expired: OIDC token expired and refresh failed.
  → Re-authenticate: modeltap auth login
```

**In-session diagnostic commands:**

| Command | Description |
|---------|-------------|
| `/status` | Show current connection health (same as `modeltap server status`) |
| `/reconnect` | Force reconnection attempt |
| `/session unlock` | Force-release a stuck session lock |

**Multi-model stream recovery**: if the connection drops during a parallel multi-model review, `session.sync` returns per-reviewer state. The harness renders completed reviewers' results immediately and shows the remaining reviewer(s) as resumed:

```
↻ Reconnected. Multi-model review in progress.
  [gpt-5.4] ✓ complete (recovered)
    ⚠ Rate limiter uses in-memory store
  [claude-opus-4-6] ●●● resumed streaming...
```

### Project Awareness

- Running `modeltap` in a git repository scopes the session to that project
- The harness transmits the project root and config content to the server via the `project` field in `capabilities.register` (FEAT-0008). The server uses this for session scoping, system prompt Layer 4 (project instructions), knowledge layer project scoping, and path normalization.
- Project-level configuration (`.modeltap.yaml` or `MODELTAP.md`) is read by the harness and sent to the server as content (the server may be remote and cannot read the harness's filesystem directly)
- File paths in tool calls and results are relative to the project root
- Session resume defaults to the most recent session for the current project
- If the project config changes between sessions, the harness sends the updated content on `session.resume`

## CLI Integration

```
modeltap                          # Launch harness (connect to server, new or resume session)
modeltap --resume <session-id>    # Resume a specific session
modeltap --project <path>         # Session scoped to a project directory
modeltap --model <name>           # Start with a specific model
```

Existing subcommands (`modeltap logs`, `modeltap metrics`, `modeltap export`, `modeltap service`) remain unchanged.

## Configuration

New harness-specific configuration:

```yaml
# Harness config (in ~/.config/modeltap/config.yaml or .modeltap.yaml)

server:
  # For solo profile: auto-start local server if not running
  # For team/enterprise: address of remote server
  address: localhost
  socket: ~/.local/share/modeltap/server.sock

harness:
  # Permission level
  permissions: default          # default | accept-edits | autonomous

  # Input
  submit_key: ctrl+enter        # or esc-enter, enter (single-line mode)
  paste_threshold: 2048         # bytes, trigger large paste handling

  # Display
  theme: auto                   # auto | dark | light
  show_cost: true               # show cost in status bar
  show_context: true            # show context usage in status bar

# MCP tool servers
mcp:
  servers:
    - name: github
      command: gh-mcp-server
      transport: stdio
```

## Non-Goals

- **Server-side features**: the harness is a thin client. Routing, knowledge, orchestration, auth, and capture are server-side (FEAT-0008, FEAT-0010, FEAT-0011).
- **Web or IDE frontend**: the terminal is the first frontend. Other frontends are future work.
- **Custom tool implementation**: the core tool set is fixed. Domain-specific tools are added via MCP servers, not harness modifications.
- **Non-terminal frontends**: the terminal harness is the first and only frontend. Web or IDE frontends are future work.

## Success Criteria

All criteria use Bubbletea for rendering (per ADR-0013 — no phased UI approach).

1. The harness connects to a running server (local socket or remote TLS), performs capability registration (FEAT-0008 `capabilities.register`), and establishes a session.
2. A user can type a message in a multi-line input area, receive a streamed response, and see it rendered with styled markdown (headings, code blocks, lists, bold/italic) via Glamour.
3. All built-in tools work: Read (text, PDF, DOCX, images, spreadsheets), Write, Edit, Bash, Glob, Grep, Git, WebSearch, WebFetch. Each executes locally with appropriate permission prompts.
4. The agentic loop works: model reads files, makes changes, runs tests, observes results, and continues — multiple tool call rounds in a single turn.
5. File attachments (`@file`, globs, drag-and-drop path detection) are included in the turn and visible to the model.
6. Large pastes trigger the summarize/truncate/full flow.
7. `/compact` shows categorized context breakdown with per-category action selectors (summarize, keep, drop, pin) and file-level drill-down.
8. The persistent status bar displays `[plan]`/`[build]`/`[auto]` mode, current model (with `[override]` indicator), context usage, session cost, and call timer.
9. `Ctrl+P` toggles between plan and build mode. The mode indicator updates immediately.
10. Plan mode: reads execute silently, writes accumulate into a structured plan. Approve-and-execute, step-through, and edit-plan flows work.
11. The model routing indicator displays before each response, showing which model was selected and why.
12. `/model <name>` overrides routing for the session; `/model auto` clears the override. Override persists across harness reconnection.
13. `/models` lists available models with routing roles, capabilities, cost, and current override status.
14. The session explorer displays on launch with recent sessions, summaries, context usage, and cost. TUI list navigation with details pane.
15. Session details view shows turn timeline (including compacted turns), pinned items, files touched, and server events.
16. Sessions persist — closing and reopening the harness in the same project directory shows the session explorer.
17. Scrollable conversation viewport with auto-scroll-to-bottom and keyboard-driven scroll-up.
18. MCP servers configured in the harness provide discoverable tools to the model (sent via `capabilities.update`).
19. `/context` command shows files, knowledge injections, and token budget.
20. The harness registers its tool catalog with the server via `capabilities.register`, and only registered tools appear in model prompts.

### Connection UX

21. The status bar shows connection state: `[●]` ready, `[◐]` degraded, `[↻]` reconnecting, `[✗]` failed.
22. Degraded state shows a banner identifying the unhealthy dependency and fallback active.
23. Reconnecting state shows retry progress (attempt count, next retry timing).
24. Failed state renders the diagnostic code, cause, auto-repair attempted, and suggested command.
25. After reconnect during an in-flight turn, the harness displays `session.sync` results: pending tool calls, completed branches, or completed turn.
26. `/status` shows full connection health (FEAT-0008 `connection.health` output).
27. Large paste summarization uses `content.transform` (FEAT-0008) — the harness never calls a provider directly.

## Parallel Build Strategy

This feature can be built in parallel with FEAT-0008 (BFF Server). See FEAT-0008's Parallel Build Strategy section for the full approach.

**Harness team builds against a mock server**: a Go server that returns scripted protocol responses per FEAT-0008's Protocol Payload Schemas. The mock covers: turn streaming, tool calls, session list/details, model list/selected, compaction plans, connection health, diagnostic codes, and multi-model branch events.

**Harness-local work that needs no server**: all 13 built-in tools (Read with format detection, Edit, Write, Bash, Glob, Grep, Git, WebSearch, WebFetch), Bubbletea component architecture, permission enforcement, plan/build/auto mode logic, MCP client integration, file context management, and large paste detection.

**Integration surface**: the protocol contract in FEAT-0008 (Protocol Payload Schemas, Canonical Field Names, Tool Catalog Schema). Both sides should import a shared interface definition for automated contract testing.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 (Go) | Harness is Go, same binary as server |
| ADR-0003 (Cobra) | Harness launch and session flags use Cobra subcommands |
| ADR-0013 (Terminal UI) | Bubbletea (Charm ecosystem) from day one |
| ADR-0009 (MCP) | Harness is an MCP client for external tool servers |

## Open Questions

1. **Submit key default**: Ctrl+Enter is familiar from chat apps but unusual in terminals. Should Enter submit in single-line mode and Ctrl+Enter in multi-line mode?
2. **Image rendering**: should the harness attempt to render images in terminals that support it (iTerm2 inline images, Kitty graphics protocol), or always describe them as text?
3. **Concurrent tool calls**: when the model requests multiple tool calls in parallel, should the harness execute them concurrently or sequentially?
