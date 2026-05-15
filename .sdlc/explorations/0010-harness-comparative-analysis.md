---
exploration: EXP-0010
title: Harness Comparative Analysis — modeltap, OpenCode, and OpenHarness
status: exploring
date: 2026-04-22
related:
  - ADR-0013: Terminal UI Framework
  - ADR-0006: Multi-Provider Support
  - ADR-0007: Usage Metrics
  - EXP-0008: Integrated Harness
  - EXP-0009: Harness Prompt Architecture
  - EXP-0007: Multi-Model Orchestration
  - FEAT-0009: Terminal Harness
---

# EXP-0010: Harness Comparative Analysis — modeltap, OpenCode, and OpenHarness

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

Modeltap is building a reverse proxy for AI/ML clients with capture, metrics, and multi-model orchestration. The **harness** is the terminal UI that interacts with the BFF (backend-for-frontend) server over JSON-RPC. The user needs the harness to serve a proxy-centric architecture: central model and cost control, central logging, granular context management across multiple models, and group context for pair/team projects.

This document analyzes three candidate harness bases:
1. **modeltap harness** (current, in-tree Go/Bubbletea client)
2. **OpenCode** (archived Go/Bubbletea terminal AI agent, succeeded by Crush)
3. **OpenHarness** (active Python agent runtime with TUI)

The goal is to identify gaps in each base relative to the proxy-centric requirements, and to surface what each codebase would need to become the harness for modeltap's architecture.

---

## 1. modeltap harness (Current)

### Architecture
- **Language:** Go (100%)
- **TUI Framework:** Bubbletea (Charm ecosystem)
- **Runtime model:** Thin client → BFF via JSON-RPC over Unix socket or TLS
- **Agent loop:** Lives in the BFF; harness renders events as Bubbletea messages
- **Tool execution:** Local (13 built-in tools + MCP servers) with permission gating
- **Distribution:** Single compiled binary (server + harness)

### Key Components
- `App` (Bubbletea `tea.Model`) with `StatusBar`, `InputArea`, `ConversationViewport`, `ConnectionUX`
- `ConnectionManager` (auto-start BFF, heartbeat, reconnect)
- `ProtocolClient` (JSON-RPC 2.0 framing)
- `ToolDispatcher` (plan-mode interception, idempotency)
- `PermissionEnforcer` (3 levels: Default, AcceptEdits, Autonomous)
- `ContextManager` (`@file` / glob resolution into `protocol.Attachment`)
- `CompactHandler` (interactive context pruning)
- `MCPManager` (stdio MCP client for external tools)

### Strengths relative to proxy requirements
1. **Native BFF integration:** The harness *is* a thin client. Central model/cost control, logging, and routing are delegated to the BFF by design.
2. **Single binary:** Server and harness ship together. No runtime dependency drift.
3. **Connection resilience:** `ConnectionManager` with degraded/reconnecting/failed states and exponential backoff.
4. **Plan/build/auto modes:** Deeply wired `PlanAccumulator` for reviewing mutating tool calls before execution.
5. **Go type sharing:** Protocol messages, session state, and tool definitions are shared between harness and server.

### Gaps relative to proxy requirements
1. **Persistent memory:** No cross-session memory (equivalent to OpenHarness's `MEMORY.md` or `CLAUDE.md`). Sessions are server-managed; harness restarts lose local UI state but server state persists. However, there is no auto-discovery of project context like `CLAUDE.md`.
2. **Tool breadth:** 13 built-in tools vs. 43+ in OpenHarness. Notebook, task scheduling, and advanced search tools are missing.
3. **Extensibility model:** MCP servers only. No hook/plugin system, no slash commands, no skill loading.
4. **Context composition:** Explicit composition contract is still being designed (EXP-0009). There is no memory digest injection per turn.
5. **Chat gateways / headless mode:** Harness is TUI-only. No Slack/Telegram/Discord integration.
6. **Subagent spawning:** Not implemented (FEAT-0013 is planned but not built).

---

## 2. OpenCode (opencode-ai/opencode, archived)

### Architecture
- **Language:** Go (99.2%), Shell (0.8%)
- **TUI Framework:** Bubbletea (Charm ecosystem)
- **Runtime model:** Self-contained agent loop; LLM client lives in the harness process
- **Tool execution:** Built-in tools (glob, grep, ls, view, write, edit, patch, bash, fetch, diagnostics, LSP agent, recursive agent) executed locally
- **Distribution:** Single compiled binary
- **Status:** Archived September 2025; succeeded by **Crush** (`charmbracelet/crush`)
- **Stars:** ~12.1k

### Key Components
- `cmd/` — Cobra CLI entrypoints
- `internal/tui/` — Bubbletea UI components
- `internal/llm/` — LLM provider integrations and tool execution
- `internal/session/` — SQLite-backed session management
- `internal/lsp/` — Language Server Protocol client
- `internal/db/` — SQLite database ops
- Auto-compact: automatic context summarization near token limits
- Custom commands: reusable markdown prompt templates
- MCP support (stdio and SSE)

### Strengths relative to proxy requirements
1. **Direct precedent for Go + Bubbletea:** OpenCode is the closest architectural twin to modeltap's harness. It proves that Bubbletea can sustain a full terminal AI agent with streaming, markdown, plan/build modes, and permission prompts.
2. **Session persistence:** SQLite-backed sessions with auto-compact. A solid foundation for central logging and session replay.
3. **LSP integration:** Code intelligence via `gopls` and other language servers. Useful for a coding-centric harness.
4. **Multi-provider support:** OpenAI, Anthropic, Google, AWS Bedrock, Groq, Azure, OpenRouter, Copilot, VertexAI, Ollama.
5. **Single binary:** Same distribution story as modeltap.

### Gaps relative to proxy requirements
1. **Self-contained agent loop:** Unlike modeltap's harness, OpenCode *is* the agent. The LLM client, tool execution, and routing are in-process. To integrate with modeltap's BFF, you would need to:
   - Extract the LLM client → delegate to BFF over JSON-RPC.
   - Extract the tool execution → either keep local or delegate to BFF.
   - Replace the SQLite session store with BFF-managed sessions.
   - This is a substantial architectural inversion, not a drop-in integration.
2. **No proxy-native cost/model control:** OpenCode routes to providers directly. Central cost control would require adding a proxy layer or BFF integration.
3. **No group context / team features:** Single-user local model. No shared workspaces or cross-user context.
4. **No knowledge layer / memory system:** Beyond SQLite session history, there is no persistent memory or `CLAUDE.md`-equivalent.
5. **Archived upstream:** Unless forked to track Crush, there is no upstream maintenance. A hard fork means full ownership of all future maintenance.

---

## 3. OpenHarness (HKUDS/OpenHarness, active)

### Architecture
- **Language:** Python (≥3.10)
- **TUI Framework:** React / Ink (Node.js-based, embedded in Python wrapper)
- **Runtime model:** Self-contained Python agent runtime with streaming loop
- **Tool execution:** 43+ built-in tools + MCP + plugins + hooks
- **Distribution:** `pip install openharness`; `oh` CLI + `ohmo` persistent agent
- **Status:** Active; ~10.9k stars; MIT license

### Key Components
- `engine/` — Agent loop: query → stream → tool-call → loop
- `tools/` — 43+ tools (file, shell, web, notebook, MCP)
- `skills/` — On-demand `.md` knowledge loading (anthropics/skills compatible)
- `plugins/` — Command/hook/agent extensions (claude-code compatible)
- `permissions/` — Multi-level safety modes, path rules, deny lists
- `hooks/` — PreToolUse / PostToolUse lifecycle
- `commands/` — 54 slash commands (`/help`, `/commit`, `/plan`)
- `memory/` — Persistent `MEMORY.md`, `CLAUDE.md`, context compression
- `coordinator/` — Subagent spawning, team registry, background tasks
- `mcp/` — Model Context Protocol client
- `config/` — Multi-layer settings

### Strengths relative to proxy requirements
1. **Rich tool/framework surface:** 43 tools, hooks, plugins, slash commands, skills. This is a platform, not just a client.
2. **Memory and context management:** `CLAUDE.md` auto-discovery, `MEMORY.md`, context compression. These are first-class features.
3. **Safety/governance:** Multi-level permissions with path rules and deny lists. Fine-grained and plugin-aware.
4. **Multi-provider with profiles:** Native routing across Claude, OpenAI, Codex, Copilot, Ollama, OpenRouter, etc.
5. **Swarm / subagent coordination:** `coordinator/` enables multi-agent teams and background tasks.
6. **Chat gateway (`ohmo`):** Slack, Telegram, Discord, Feishu integrations built-in.

### Gaps relative to proxy requirements
1. **Language/runtime mismatch:** Python + Node.js for TUI. Cannot compile into a Go binary. Integrating with modeltap's Go BFF requires either:
   - A Python client speaking JSON-RPC to the BFF (complex, two-runtimes)
   - Rewriting the TUI layer in Go/Bubbletea (massive effort)
   - Or keeping the BFF in Go and the harness in Python (operational complexity)
2. **Self-contained agent loop:** Like OpenCode, the agent loop is native. Delegating to a BFF would require significant refactoring.
3. **No natural proxy integration point:** OpenHarness routes to providers directly. There is no "thin client / thick server" split. To get central model/cost control, you would need to build a proxy adapter into its provider layer.
4. **Distribution complexity:** `pip install` is not a single binary. The `ohmo` workspace model (`~/.ohmo/`) is personal and local, not team-oriented.
5. **Two-language tax:** Python agents + Go BFF = two build systems, two type systems, two dependency trees. This directly conflicts with ADR-0013's decision driver D1 (single binary) and D3 (language alignment).

---

## 4. Comparative Matrix

| Dimension | modeltap harness | OpenCode | OpenHarness |
|---|---|---|---|
| **Language** | Go | Go | Python (TUI in Node/Ink) |
| **TUI Framework** | Bubbletea | Bubbletea | React / Ink |
| **Runtime Model** | Thin client → BFF | Self-contained agent | Self-contained agent |
| **Agent Loop Location** | BFF (server) | In-process | In-process |
| **Tool Count** | 13 built-in + MCP | ~12 built-in + MCP | 43+ built-in + MCP + plugins |
| **Session Persistence** | BFF-managed | SQLite local | `MEMORY.md` + workspace |
| **Memory / Context** | Per-session only | SQLite history | `CLAUDE.md`, compression, digest |
| **Permission Model** | 3-level enforcer | Basic approval | Multi-level + path rules + hooks |
| **Extensibility** | MCP only | MCP + custom commands | Plugins, hooks, slash commands, skills |
| **Multi-Provider** | Delegated to BFF | Native (many) | Native profiles (many) |
| **Cost Tracking** | BFF-side | Limited | Native per-turn |
| **Subagent / Swarm** | Planned (FEAT-0013) | Recursive agent tool | `coordinator/` full swarm |
| **Chat Gateways** | None | None | Slack, Telegram, Discord, Feishu |
| **Single Binary** | Yes | Yes | No (`pip install`) |
| **Language Alignment w/ BFF** | Perfect | Perfect | Poor |
| **Proxy Integration Ease** | Native | Requires inversion | Requires adapter |
| **Central Logging** | BFF-native | Requires extraction | Requires extraction |
| **Group / Team Context** | BFF-capable | None | Limited (`team registry`) |
| **Upstream Status** | Active (in-tree) | Archived (→ Crush) | Active |

---

## 5. Gap Analysis: Proxy-Centric Requirements

### Requirement 1: Central Model and Cost Control

- **modeltap:** Native. The BFF owns provider routing, model selection, and cost aggregation. The harness displays `ModelUpdateMsg` and `CostUpdateMsg` but does not route.
- **OpenCode:** Requires replacing the native LLM client with a BFF-mediated one. This means gutting `internal/llm/` and rerouting all provider calls through JSON-RPC.
- **OpenHarness:** Requires building a proxy adapter into its provider layer, or replacing its `engine/` loop to delegate to a BFF. The profile-based routing is native and would conflict with BFF-side routing.

### Requirement 2: Central Logging of Activity

- **modeltap:** Native. Captures are stored server-side. The harness is just a viewer.
- **OpenCode:** Has SQLite local sessions, but these are client-local. To centralize, you would need to sync SQLite to the BFF or replace the session store.
- **OpenHarness:** Has event streams and cost tracking, but storage is local/workspace-scoped. Centralizing requires a new transport layer.

### Requirement 3: Granular Control Over Context Across Calls to Multiple Models

- **modeltap:** Partial. EXP-0009 defines a prompt composition contract (base system prompt → project context → memory digest → skill overlay → tool descriptions → conversation history → current turn). This is designed for BFF-side orchestration where a single user turn may fan out to multiple providers.
- **OpenCode:** Auto-compact is single-model. There is no multi-model orchestration layer.
- **OpenHarness:** Profiles are per-conversation, not per-turn multi-model. The `coordinator/` swarm could be adapted, but it is not proxy-native.

### Requirement 4: Group Context for Pair and Team Projects

- **modeltap:** The BFF is architecturally capable of multi-user sessions (EXP-0002: Multi-User Support). The harness is a single-user client, but the server can host shared sessions.
- **OpenCode:** Single-user SQLite DB. No multi-user concept.
- **OpenHarness:** `team registry` and subagent spawning exist, but they are local/Python. There is no server-mediated shared session model.

---

## 6. Fork Scenarios

### 6.1 Fork OpenCode (Hard Fork, Diverge Immediately)

- You inherit a mature Go/Bubbletea AI agent (~12k stars) with session management, LSP, many providers, and auto-compact.
- You must invert the architecture: gut `internal/llm/` to delegate to modeltap BFF. Gut `internal/db/` to use BFF sessions.
- You lose upstream fixes unless you manually backport from Crush.
- Risk: significant architectural surgery on a codebase that was not designed for a thin-client model.

### 6.2 Fork OpenCode (Soft Fork, Track Crush / charmbracelet)

- Same as above, but you maintain a merge strategy with Crush upstream.
- Crush is the successor; it may diverge from OpenCode's architecture.
- Merge conflicts will be constant because the thin-client inversion touches core packages.
- Risk: high merge overhead, uncertain compatibility with Crush's roadmap.

### 6.3 Fork OpenHarness (Hard Fork, Diverge Immediately)

- You inherit a richly featured Python agent platform with tools, memory, plugins, and swarm.
- You must rewrite the TUI in Go/Bubbletea (or ship a Python runtime alongside the Go BFF).
- You must invert the agent loop to delegate to the BFF.
- You lose nothing from upstream because you diverge immediately.
- Risk: massive rewrite effort. Effectively using OpenHarness as a design reference, not a code base.

### 6.4 Fork OpenHarness (Soft Fork, Track Upstream)

- You maintain a merge strategy with HKUDS/OpenHarness.
- You re-apply BFF-inversion patches on every upstream release.
- The Python/Node.js runtime remains an operational burden.
- Risk: unsustainable. Upstream moves fast (active project). Re-applying architectural inversion patches is a tax on every upstream sync.

---

## 7. Conclusions

1. **modeltap harness is the only base that is natively aligned with a proxy-centric architecture.** The thin-client / thick-server split is by design, not retrofit. The modeltap option (Option 1) is to **continue the existing harness and close the identified gaps incrementally** — adding persistent memory, a skill/hook framework, a command palette, chat gateway support, and richer tool governance directly into the in-tree Go/Bubbletea codebase, all while preserving the BFF-native delegation model.
2. **OpenCode is a strong design reference for Bubbletea UX patterns** (streaming markdown debouncing, plan/build modes, session explorer), but forking it would require gutting its core to invert the agent loop. The effort to retrofit BFF delegation may exceed the effort to evolve the existing modeltap harness.
3. **OpenHarness is a strong design reference for agent richness** (tools, memory, plugins, safety governance), but its Python/Node.js runtime makes it a poor fit for a Go-based proxy. Forking it practically means rewriting it.
4. **The biggest gaps across all bases are:**
   - Persistent cross-session memory (all)
   - Project context auto-loading like `CLAUDE.md` (modeltap has EXP-0009; others have it partially)
   - Subagent / team coordination (modeltap planned; OpenHarness native)
   - Chat gateway / headless mode (modeltap missing; OpenHarness native)
   - Hook/plugin/skill extensibility (OpenHarness native; modeltap missing)

5. **Recommendation:** Continue with the modeltap harness. Use OpenCode and OpenHarness as **design references** for specific subsystems (OpenCode for TUI patterns and session management; OpenHarness for tool governance, memory, and plugin architecture). Invest in closing the harness's own gaps by adding the missing functionality directly:
   - A persistent memory layer (`MEMORY.md` equivalent)
   - A skill/hook/plugin framework
   - A slash command palette
   - Chat gateway support at the BFF layer
   - A `CLAUDE.md`-style project-context auto-discovery mechanism

---

## Open Questions

1. If OpenCode/Crush were to expose a thin-client mode (server-mediated loop), would the calculus change?
2. Can modeltap's BFF eventually support enough of the agent loop richness to make a thin client viable for all use cases, or will some users require a thick client?
3. Does OpenHarness's `coordinator/` swarm model suggest a feature for modeltap (subagent orchestration at the proxy layer)?
4. Should modeltap adopt OpenHarness's `CLAUDE.md` project-context convention, or stick with its own `MODELTAP.md`?
5. Is there a world where modeltap ships both a Go thin-client harness *and* a Python thick-client adapter for power users who need OpenHarness-level extensibility?

(End of document)
