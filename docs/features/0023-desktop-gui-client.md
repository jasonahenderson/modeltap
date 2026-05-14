---
feature: FEAT-0023
title: Desktop GUI Client
status: draft
date: 2026-05-07
depends-on:
  - FEAT-0008: Runtime Server, including planned Amendment 001 for session-scoped project context
  - FEAT-0009: Terminal Harness, as the reference harness-role implementation
related:
  - FEAT-0014: Harness Conversation Shell
  - PATCH-0017: Session-scoped project context
adr-constraints:
  - ADR-0001: Go as primary language for the runtime server and harness-side core
  - ADR-0014: Harness Base Strategy
  - ADR-0016: Runtime server and client surfaces
---

# FEAT-0023: Desktop GUI Client

## Problem

Modeltap's only interactive client today is the terminal harness. The TUI is
the right interface for terminal-native users, but it limits modeltap to a
single active chat surface and to users who are comfortable making a terminal
their daily AI workspace.

The runtime server, harness protocol, audit trail, cost visibility, and future
memory layer are not terminal-specific. They are facilities for agentic
workflows. A GUI client should be able to use the same runtime server and the
same harness-role responsibilities without inventing a parallel orchestration
stack.

The current connection-scoped project context also forecloses a natural GUI
workflow: multiple concurrent chats, each bound to a different project or
document set, over one runtime-server connection.

## Solution

Ship a native desktop GUI for macOS and Windows that connects to the runtime
server over the same harness protocol used by the terminal harness. The GUI is
a second client surface, not a replacement for the TUI.

The GUI runs multiple concurrent chats. Each tab is bound to its own session,
project root, capabilities snapshot, model override, and permission state. The
runtime server remains the orchestration authority for conversation state,
routing, provider calls, cost tracking, and run persistence. The GUI owns the
harness role for local tool execution, permissions, project context discovery,
and MCP integration.

PATCH-0017 is the protocol-internal prerequisite: project context must become a
session attribute, not a connection attribute, before a multi-chat client can
avoid cross-tab project-context collisions.

## Key Capabilities

### Multi-Chat Workspace

- Tabbed chat interface. Each tab maps to one `(session_id, project_root)`.
- Tabs are concurrent over one runtime-server connection.
- Switching tabs does not re-handshake or re-register the full tool catalog.
- Per-tab project root, selected at tab creation and editable later.
- Per-tab model override, defaulting to runtime-server routing policy.
- Per-tab unread and streaming indicators.
- Tab persistence across app restarts via `session.resume`.

### Conversation Surface

- Streaming markdown rendering.
- Inline file references, paste expansion, and image previews.
- Per-message cost, token, provider, and model metadata.
- Scrollback, search, copy, and session history navigation.
- Clear connection state when the runtime server is starting, reconnecting,
  degraded, or unavailable.

### Tool Execution and Permissions

- The desktop client owns local tool execution end-to-end, matching the
  harness role defined by ADR-0014.
- Built-in file, shell, Git, search, and MCP tools use the same risk and
  permission model as the terminal harness.
- Permission prompts are graphical sheets or modals.
- Permission state is per-tab, so autonomous work in one tab does not loosen
  policy in another.
- File-write previews use a graphical diff viewer.

### Project Context Per Tab

- Each tab carries its own project context: `root`, `config_file`, and
  `config_content`.
- New sessions bind project context on open; resumed sessions refresh project
  context on resume.
- Changing a tab's project root updates that tab only.
- Recent projects are available for fast tab creation.

### Simple Mode

- Optional user preference that hides developer-oriented controls such as raw
  tool catalogs, MCP server details, slash commands, and advanced permission
  tuning.
- Simple mode does not change the protocol or harness role. It only changes
  what the GUI chooses to expose.

### Distribution

- macOS signed and notarized app bundle.
- Windows signed installer.
- Stable and beta update channels.
- Runtime-server distribution model is a design question: the desktop app may
  bundle a compatible local `modeltap` runtime server, discover an installed
  one, or support both modes.

## CLI / UI / API Integration

- New desktop UI artifact.
- The GUI speaks the same harness protocol as the terminal harness.
- No GUI-specific runtime-server endpoint should be added unless a future ADR
  explains why the shared protocol is insufficient.
- Existing CLI commands continue to operate on the same local config and
  SQLite store when the desktop app uses a local runtime server.

## Configuration

- App preferences live in platform-native locations:
  - macOS: `~/Library/Application Support/Modeltap/`
  - Windows: `%APPDATA%\Modeltap\`
- Runtime-server connection config is shared with the CLI where practical.
- Per-tab UI state lives in the desktop app store, not in the runtime server.
- Session, run, and conversation state remain runtime-server owned.

## Non-Goals

- Replacing the terminal harness.
- Browser or web deployment.
- Mobile clients.
- Cloud-hosted modeltap.
- Redesigning the runtime-server protocol outside the session-scoped project
  context prerequisite.
- Linux desktop in the first release.

## Success Criteria

1. A user can install the desktop client on macOS and Windows with no terminal
   interaction.
2. A user can open at least three concurrent tabs, each with a different
   project root, and submit turns without project, session, model, or
   permission state collision.
3. Tool execution, permissions, and MCP integration work with the same safety
   guarantees as the terminal harness.
4. The terminal harness continues to work against the same runtime server.
5. Runtime-server connection loss and recovery are isolated per tab; one stuck
   session does not block other tabs.
6. Simple mode lets a non-terminal user complete a project-scoped task without
   seeing developer-only controls.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 | The runtime server and reusable harness core stay Go. A native UI shell may use another platform UI technology if a future ADR accepts that choice. |
| ADR-0014 | The GUI is another embodiment of the harness role, not a separate orchestration model. |
| ADR-0016 | Confirms the naming: runtime server is the shared orchestration service; TUI and GUI are client surfaces. |

## Open Questions

1. **Harness-role embodiment.** Does the GUI implement the harness role in a
   native UI runtime, run the Go harness core as a sidecar, or use a hybrid
   app shell that embeds/reuses Go? This is ADR-grade before implementation.
2. **UI toolkit.** Candidate directions include SwiftUI + WinUI/WPF, Wails,
   Tauri, Electron, and Flutter Desktop. This depends on the embodiment ADR.
3. **Runtime-server distribution.** Should the installer bundle a compatible
   runtime server, discover an existing installation, or support both?
4. **MCP management.** The GUI needs a graphical MCP configuration and status
   surface. Its relationship to `~/.modeltap/config.yaml` needs design.
5. **Simple mode boundaries.** The default hidden/exposed tool set needs
   product design before acceptance.
6. **Release sequencing.** No committed target. PATCH-0017 is an enabling
   prerequisite, but the GUI itself is release TBD.

## Implementation Sequencing

1. **PATCH-0017 (release TBD):** move project context to session scope so
   multiple tabs can safely share one runtime-server connection.
2. **ADR-NNNN (release TBD):** decide desktop harness-role embodiment.
3. **FEAT-0023 v1 (release TBD):** desktop shell, multi-tab workspace, tool
   execution, permission prompts, and terminal-harness parity across N tabs.
4. **FEAT-0023 v2 (release TBD):** simple mode, graphical MCP management,
   auto-update, and deeper polish.
