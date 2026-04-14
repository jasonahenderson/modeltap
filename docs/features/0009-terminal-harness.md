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
  - ADR-0013: Terminal UI framework (proposed — phased minimal to Bubbletea)
promoted-from:
  - EXP-0008: Integrated Harness
---

# FEAT-0009: Terminal Harness

## Problem

Modeltap v1 has no interactive interface — it is a background proxy with CLI query commands. The integrated harness (EXP-0008) needs a terminal UI that lets users have conversations with AI models through the modeltap server. Without it, the BFF server (FEAT-0008) has no client, and the product's differentiators (cross-model memory, cost-aware routing, knowledge enrichment) have no user-facing surface.

Existing terminal AI tools (Claude Code, aider, Codex) are either single-provider, lack cross-model memory, or run in sandboxed environments. The modeltap harness is a thin client that delegates intelligence to the BFF — it handles terminal rendering, local tool execution, and permission enforcement while the server handles everything else.

## Solution

A terminal UI that connects to the modeltap server via the harness protocol (FEAT-0008), sends conversation turns, streams responses, executes tools locally, and enforces permissions. The harness is implemented in Go per ADR-0001, using a phased UI approach per ADR-0013: minimal prototype first (stdout + readline), Bubbletea for production (styled markdown, scrollable viewport, status bar).

## Key Capabilities

### Conversation Interface

- Multi-line text input with cursor movement, cut/copy/paste, and configurable submit key
- Streaming markdown output rendered with terminal styling (headings, code blocks, lists, bold/italic)
- Command history traversal (up/down arrows) sourced from the BFF (cross-session, cross-project)
- Scrollable conversation viewport with auto-scroll-to-bottom on new output

### Tool Execution

The harness executes tools locally on the user's machine. The BFF forwards model tool calls; the harness decides whether to execute based on the current permission level and execution mode.

**Core tool set:**

| Tool | Description | Default Permission |
|------|-------------|-------------------|
| Read | Read file contents (text, PDF, DOCX, images) | Auto-allow |
| Write | Create new files | Prompt first use |
| Edit | Modify existing files (exact string match) | Prompt first use |
| Bash | Execute shell commands | Prompt per command |
| Glob | Find files by pattern | Auto-allow |
| Grep | Search file contents | Auto-allow |
| Git | Git operations | Prompt for mutations |
| WebSearch | Search the web | Prompt first use |
| WebFetch | Fetch URL contents | Prompt per domain |

**Tool safety guardrails:**
- Edit requires exact string matching — prevents hallucinated file overwrites
- Edit fails if the file has not been Read first in the session
- Write snapshots the original file before overwriting (reversibility)
- Bash displays the full command before execution, requiring explicit approval
- Tool descriptions instruct the model on when to use each tool and when NOT to (e.g., "use Read instead of cat via Bash")

**MCP tool discovery:** The harness connects to configured MCP servers (stdio transport) and discovers their tools. MCP tools appear alongside core tools with the same permission enforcement.

### Permission Model

Three permission levels, matching the proven pattern from Claude Code:

- **Default**: prompt for each tool type on first use. Read-only tools (Read, Glob, Grep) are auto-allowed.
- **Accept edits**: auto-approve file operations (Read, Write, Edit, Glob, Grep). Bash still prompts.
- **Autonomous**: auto-approve all tools within safety limits. Dangerous bash commands (rm -rf, git push --force) still prompt.

Permissions are enforced entirely in the harness. The BFF has no knowledge of the current permission level and cannot override it.

### Execution Modes

The harness supports three modes that control how tool calls are handled:

- **Plan** (`/plan`): collect tool calls into a structured plan. Present for approval before executing. "Step through" executes one at a time.
- **Build** (`/build`, default): execute tool calls as they arrive, subject to the permission level.
- **Auto** (`/auto`): auto-approve within the permission level without per-action prompts.

Execution mode is harness-local. The BFF always forwards tool calls the same way — the harness decides what to do with them.

### File Context Management

- `@path/to/file` syntax to attach files to a message
- `@src/**/*.go` glob patterns for multiple files
- Drag and drop detection (terminal emits file paths)
- `/context` command shows files in context, knowledge injections, and token budget
- `/drop <file>` removes a file from context

**File format support:**
- Text files: direct inclusion
- PDF: text extraction
- DOCX: text extraction
- Images: base64-encoded for vision-capable models
- Spreadsheets (XLSX/CSV): parsed as structured text

### Large Paste Handling

When the harness detects a paste exceeding the configurable threshold (default: 2KB):

- Show preview (first 5 lines + line count + byte size)
- Offer: summarize (via cheap model), include full, truncate (first N lines), cancel
- Full paste is always captured by the BFF regardless of what the user chooses for the live context

### Status Bar

Persistent display at bottom of terminal:

```
claude-opus-4-6 | 47% context (38K/80K) | $0.42 session | timer 3.2s
```

- Current model name
- Context window usage (percentage and absolute tokens)
- Running session cost
- Timer for the current model call (starts on send, stops on stream complete)

**Per-turn metadata** displayed inline after each response:

```
--- claude-opus-4-6 | 1,247 in / 3,891 out | $0.08 | 4.1s ---
```

**Context pressure warnings** at configurable thresholds.

### Session Commands

| Command | Description |
|---------|-------------|
| `/compact` | Compress context, retain full history on server |
| `/clear` | Fresh context within the same session |
| `/fork` | Branch session into independent continuation |
| `/model <name>` | Switch models within session |
| `/plan` | Enter plan mode |
| `/build` | Enter build mode (default) |
| `/auto` | Enter auto mode |
| `/cost` | Show session cost breakdown |
| `/trace` | Show model routing for last turn |
| `/context` | Show files, knowledge injections, context budget |
| `/drop <file>` | Remove file from context |
| `/help` | Show available commands |

### Project Awareness

- Running `modeltap` in a git repository scopes the session to that project
- Project-level configuration (`.modeltap.yaml`) overrides global settings
- File operations are relative to the project root
- Session resume defaults to the most recent session for the current project

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
- **Bubbletea UI in the initial delivery**: per ADR-0013, the prototype uses minimal rendering. Bubbletea migration is a follow-up work unit.

## Success Criteria

1. The harness connects to a running server (local socket or remote TLS) and authenticates.
2. A user can type a message, receive a streamed response, and see it rendered in the terminal with markdown styling.
3. The model can request tool calls (Read, Edit, Bash at minimum) which the harness executes locally with permission prompts.
4. The agentic loop works: model reads files, makes changes, runs tests, observes results, and continues — multiple tool call rounds in a single turn.
5. File attachments (`@file`) are included in the turn and visible to the model.
6. Large pastes trigger the summarize/truncate/full flow.
7. Session commands (`/compact`, `/model`, `/cost`, `/context`) work.
8. The status bar displays current model, context usage, session cost, and call timer.
9. Sessions persist — closing and reopening the harness in the same project directory resumes the previous session.
10. Plan mode collects tool calls into a reviewable plan before execution.
11. MCP servers configured in the harness provide discoverable tools to the model.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 (Go) | Harness is Go, same binary as server |
| ADR-0003 (Cobra) | Harness launch and session flags use Cobra subcommands |
| ADR-0013 (Terminal UI) | Phased: minimal prototype, then Bubbletea production UI |
| ADR-0009 (MCP) | Harness is an MCP client for external tool servers |

## Open Questions

1. **Submit key default**: Ctrl+Enter is familiar from chat apps but unusual in terminals. Should Enter submit in single-line mode and Ctrl+Enter in multi-line mode?
2. **Image rendering**: should the harness attempt to render images in terminals that support it (iTerm2 inline images, Kitty graphics protocol), or always describe them as text?
3. **Concurrent tool calls**: when the model requests multiple tool calls in parallel, should the harness execute them concurrently or sequentially?
