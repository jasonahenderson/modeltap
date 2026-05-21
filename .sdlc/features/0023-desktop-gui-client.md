---
feature: FEAT-0023
title: Desktop GUI Client
status: draft
date: 2026-05-07
depends-on:
  - FEAT-0008: BFF Server (incl. Amendment 001 — session-scoped project context)
  - FEAT-0009: Terminal Harness (reference for harness-role responsibilities)
related:
  - FEAT-0014: Harness Conversation Shell (UX patterns to mirror across surfaces)
  - PATCH-0017: Session-scoped project context refactor
adr-constraints:
  - ADR-0001: Go as primary language (BFF and any sidecar harness stay Go)
  - ADR-0014: Harness Base Strategy (universal orchestration client; the GUI is a second surface, not a replacement)
---

# FEAT-0023: Desktop GUI Client

## Problem

Modeltap's only client today is a terminal harness (FEAT-0009). The TUI is the
right interface for terminal-native users, but it forecloses an audience that
otherwise fits the product: people doing professional knowledge work who want
project-rooted, multi-model, audited AI assistance, but who will not adopt a
terminal as their daily interface. The TUI also enforces a single active chat
at a time, which is acceptable for a developer focused on one task but does
not match how non-terminal users actually work — multiple ongoing
conversations, each scoped to a different project or document set.

Without a desktop GUI, modeltap's reach stops at the terminal. The BFF, the
cross-model memory, the audit trail, and the cost visibility — all of which
are domain-neutral — are gated behind a Bubbletea TUI. ADR-0014 explicitly
chose the harness as the **universal orchestration client**, and a desktop GUI
is the natural second surface that extends "universal" beyond the terminal.

## Solution

Ship a native desktop GUI for macOS and Windows that connects to the BFF over
the same harness protocol the TUI uses (FEAT-0008, post Amendment 001). The
GUI runs **multiple concurrent chats** ("tabs"), each bound to its own
project root and session. The TUI continues to be the single-active-session
specialization of the same protocol; the GUI is the multi-active-session
generalization.

The desktop client takes on the **harness role** as defined by the BFF
protocol: it executes tools locally, enforces permissions, hosts MCP servers,
and supplies project context per session. How that harness role is *embodied*
inside the desktop process is the central architectural question this spec
opens (see Open Questions and the dedicated ADR called for below).

## Key Capabilities

### Multi-Chat Workspace

- Tabbed chat interface. Each tab is one chat = (session_id, project_root,
  capabilities snapshot, model override, permission state).
- Tabs are concurrent over a single BFF connection. Switching tabs does not
  re-handshake or re-register capabilities.
- Per-tab project root, picked at tab creation (folder picker) and editable
  later.
- Per-tab model override (defaults to the BFF's routing policy).
- Per-tab unread / streaming-in-progress indicators.
- Tab persistence across app restarts. Reopening the app restores the prior
  tab set, each via `session.resume`.

### Conversation Surface

- Streaming markdown rendering (parity with the TUI's content surface, not
  necessarily the same renderer).
- Inline file references, paste expansion, image previews.
- Per-message cost / token / model badges, consistent with FEAT-0008's
  cost.update events.
- Scrollback, search, copy.

### Tool Execution and Permissions

- The desktop client owns tool execution end-to-end (Read, Edit, Write, Bash,
  Glob, Grep, Git, WebFetch, WebSearch, MCP-discovered tools), matching
  FEAT-0009's harness role.
- Permission prompts are graphical (modal or sheet) rather than terminal
  inline forms. The decision model — default / accept-edits / autonomous — is
  the same.
- Permission state is per-tab, not global. The user can run an autonomous
  agent in one tab while a default-permission chat is open in another.
- File-write previews use a native diff viewer.

### Project Context Per Tab

- Each tab carries its own project context blob (root, config_file,
  config_content), sent on `session.open` / `session.resume` per Amendment
  001 of FEAT-0008.
- Changing the project root on an existing tab is allowed; the GUI re-sends
  project context on the next turn or via an explicit `project.update` if
  one is added to the protocol.
- "Recent projects" shortcut for fast tab creation.

### Simple Mode for Non-Technical Users

- An optional UI mode that hides the developer surface (MCP server panel, raw
  tool catalog, `/` slash commands, advanced permission tuning) and exposes
  only: chat, file uploads, project picker, model selector, cost. The
  underlying harness role is identical; only the UI surface changes.
- Simple mode is a per-user preference, not a per-tab toggle.
- Simple mode does not change the BFF protocol or the harness contract. It is
  purely a frontend concern.

### Distribution

- macOS: signed and notarized `.app` bundle, distributed as a `.dmg` or via
  Homebrew cask. Apple Silicon native; Intel build optional.
- Windows: signed `.exe` installer (`.msi` or NSIS), x64 + ARM64.
- Auto-update channel (stable / beta), with the BFF version pinned per app
  release to avoid protocol drift.
- The desktop client either bundles a local BFF binary (single-process
  install) or auto-discovers an existing modeltap service installation. See
  Open Questions.

## CLI / UI / API Integration

- New top-level UI artifact, not a CLI command. The BFF gains no new
  surface beyond what Amendment 001 (PATCH-0017) introduces.
- The existing CLI (`modeltap serve`, `modeltap session list`, etc.)
  continues to work and operates on the same SQLite store the desktop
  client uses.
- The desktop client speaks the same harness protocol as the TUI. The
  protocol is the integration contract; no GUI-specific endpoints.

## Configuration

- App-level preferences live in a platform-native location:
  - macOS: `~/Library/Application Support/Modeltap/`
  - Windows: `%APPDATA%\Modeltap\`
- Server connection config (socket path, TLS endpoint, auth) is shared
  with the modeltap CLI when the user runs both on the same machine. The
  desktop client reads the existing `~/.modeltap/config.yaml` if present.
- Per-tab state (project root, model override, permission mode) is
  persisted in the desktop client's app store, not in the BFF.

## Non-Goals

- Replacing the terminal harness. The TUI remains a first-class surface
  for terminal-native users; the GUI is additive.
- Web/browser deployment. This feature is scoped to native desktop only.
  A future browser surface (e.g., for collaborative review) is out of
  scope.
- Mobile clients (iOS, Android). Out of scope.
- A cloud-hosted modeltap. The desktop client connects to a local or
  on-network BFF.
- Replacing the FEAT-0003 web dashboard. The dashboard remains the
  observability surface; the desktop client is the conversation surface.
- Linux desktop. May be added later but is not in scope for the first
  release; the priority audiences are macOS and Windows.

## Success Criteria

1. A user can install the desktop client on macOS and on Windows from a
   signed installer in under five minutes, with no terminal interaction
   required.
2. The user can create at least three concurrent tabs, each bound to a
   different project root, and submit turns in each tab without state
   collision (project root, session id, model override, permission state).
3. Tool execution, permission prompts, and MCP integration work in the
   GUI with the same correctness guarantees the TUI offers.
4. The TUI continues to operate against the same BFF without behavioral
   regression.
5. Simple mode hides the developer surface and exposes a chat-first UI
   that a non-technical productivity user can complete a project-scoped
   task in without reading documentation.
6. The desktop client survives BFF connection loss and recovery (per the
   FEAT-0008 lifecycle) on a per-tab basis: a single tab whose session is
   stuck does not block the others.
7. Auto-update lands a new desktop release without breaking the protocol
   handshake against a previous-version BFF in the supported version
   range.

## Relationship to ADRs

- **ADR-0014 (Harness Base Strategy):** the GUI extends the universal
  orchestration client surface beyond the terminal. It does not contradict
  ADR-0014's choice of the modeltap harness over a fork; it adds a second
  embodiment of the harness role.
- **ADR-0001 (Go primary language):** the BFF and any sidecar harness
  stay Go. The desktop UI shell is *not* required to be Go (see Open
  Questions); ADR-0001 does not prohibit a non-Go UI shell that calls into
  a Go harness sidecar.
- A new ADR is required to settle the **harness-role embodiment** (native
  in-process vs. Go sidecar service). See Open Questions.

## Open Questions

1. **Harness-role embodiment.** Does the desktop client implement the
   harness role natively in the UI runtime (Swift on macOS, .NET on
   Windows), or does it run the existing Go harness as a sidecar service
   and act as a thin frontend? Trade-offs:
   - **Native:** best UX integration; duplicates tool execution,
     permissions, MCP hosting in two languages; long-term maintenance tax.
   - **Sidecar:** reuses the Go harness verbatim; one tool/permission/MCP
     codebase; introduces a local IPC hop and a second process to manage.
   - **Hybrid (Wails / Tauri-style):** Go harness compiled into the same
     process as a webview-based UI. Single binary; UI built in
     web technology; cross-platform. May undercut "native" feel but
     dramatically reduces duplication.
   This is an ADR-grade decision and should be drafted before
   implementation begins.

2. **UI toolkit.** SwiftUI + WinUI/WPF (native), Wails (Go + webview),
   Tauri (Rust + webview), Electron (rejected by default for memory
   footprint), Flutter Desktop. Downstream of question 1.

3. **BFF distribution model.** Does the desktop installer bundle a local
   BFF binary (single-process install, GUI launches BFF as needed), or
   does it require the user to install `modeltap` separately? Bundling is
   smoother for non-technical users; separate install matches the existing
   CLI mental model.

4. **MCP servers in a GUI world.** The TUI inherits MCP servers from
   `~/.modeltap/config.yaml`. The GUI needs a graphical MCP management
   surface — when does that ship, and does it share config with the CLI?

5. **Multi-user / shared host.** Out of scope for v1, but the protocol
   already supports it (FEAT-0010). When does the desktop client gain a
   user-switcher?

6. **Release sequencing.** No committed target. PATCH-0017 lands in
   v0.3.5 regardless, as architectural insurance against multi-chat
   foreclosure. The GUI itself sequences after the harness-role ADR is
   accepted and after the GUI becomes a forcing priority — that may be
   v0.4.0, v0.6.0, or much later. This spec is a forward marker; it does
   not claim a release.

7. **Simple mode boundaries.** Where exactly does the developer surface
   end? File system tools (Read/Edit/Write/Bash) are the obvious
   gating set, but project-rooted chat without file tools is a degraded
   experience. The default tool set for simple mode needs design work.

## Implementation Sequencing (provisional)

This sequencing is a planning sketch, not a commitment. Only PATCH-0017
has a concrete release target; everything below is **release TBD** and
contingent on the GUI becoming a deliberate priority.

1. **PATCH-0017 (v0.3.5):** session-scoped project context. Removes
   the connection-scope foreclosure on multi-chat. Lands regardless of
   when (or whether) the GUI itself ships, as architectural insurance.
2. **ADR-NNNN (release TBD):** harness-role embodiment decision (native
   vs. sidecar vs. hybrid). Required before this feature leaves
   `draft`. Drafted when the GUI moves up the roadmap.
3. **FEAT-0023 v1 (release TBD):** desktop shell + multi-tab workspace
   + tool execution + permissions, parity with the TUI's single-chat
   behavior multiplied by N.
4. **FEAT-0023 v2 (release TBD, post-v1):** simple mode, MCP graphical
   management, auto-update, polish.
