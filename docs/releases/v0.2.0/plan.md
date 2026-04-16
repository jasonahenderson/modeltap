# Implementation Plan: FEAT-0008 (BFF Server) + FEAT-0009 (Terminal Harness)

## Context

Modeltap v1 is a complete reverse proxy (~4,500 lines Go) with capture, metrics, CLI, dashboard, and service management (WU-001 through WU-038). FEAT-0008 and FEAT-0009 transform it into an integrated professional AI environment: a BFF server that manages conversations, routing, and sessions, plus a Bubbletea terminal harness that handles tools, permissions, and UI.

The two features can be built in parallel against a shared protocol contract (`internal/protocol/`). The tracks may also be serialized (recommended: server first).

## Approach

**Three tracks + integration:**
- **Track 0** (7 WUs): Shared prerequisites — protocol types, ADR-0006 amendment, provider formatting, session storage
- **Track A** (22 WUs): FEAT-0008 BFF Server
- **Track B** (20 WUs): FEAT-0009 Terminal Harness
- **Integration** (3 WUs): End-to-end tests, CLI launch, docs

**Total: 52 work units (WU-039 through WU-090)**

---

## Track 0: Shared Prerequisites

Must complete before Track A or Track B begins.

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 039 | Protocol types: core messages and framing | — | M | — |
| 040 | Protocol types: streaming events | 039 | M | 041, 042 |
| 041 | Protocol types: tools, sessions, models, health, errors | 039 | M | 040, 042 |
| 042 | ADR-0006 amendment: provider outbound formatting interface | 039 | S | 040, 041 |
| 043 | Anthropic outbound formatting (`FormatMessages`) | 042 | M | 044, 045 |
| 044 | OpenAI outbound formatting (`FormatMessages`) | 042 | M | 043, 045 |
| 045 | Session and turn storage schema (migration v2) | 039 | L | 043, 044 |

**Critical path:** 039 → 042 → 043/044 (parallel) → done. 045 can run alongside 043/044.

**Key files:**
- NEW `internal/protocol/*.go` — all message/event/payload types with JSON tags
- MODIFY `internal/provider/provider.go` — extend Provider interface with `FormatMessages`, `FormatToolDefinitions`
- MODIFY `internal/provider/anthropic.go` — implement outbound formatting
- MODIFY `internal/provider/openai.go` — implement outbound formatting
- MODIFY `internal/storage/store.go` — add Session/Turn types and methods
- MODIFY `internal/storage/sqlite.go` — migration v2, new table implementations

---

## Track A: FEAT-0008 (BFF Server)

### Foundation (transport, connections, capabilities)

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 046 | JSON-RPC transport layer | 039-041 | M | Track B |
| 047 | Protocol endpoint: socket and TLS listeners | 046 | M | 048 |
| 048 | Connection lifecycle state machine (9 states) | 046 | M | 047 |
| 049 | Capability registration and tool catalog | 046, 041 | S | 047, 048 |

### Sessions and conversation state

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 050 | Session management: create, resume, list, details, lock | 045, 046 | L | 049 |
| 051 | Conversation state: canonical format and persistence | 050, 042 | M | — |
| 052 | Provider format translation: turn dispatch | 051, 043, 044 | M | — |

### Streaming, prompts, cost

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 053 | Streaming relay: SSE to protocol events | 052 | L | 054 |
| 054 | System prompt engine: layers 1-5 | 049, 046 | M | 053 |
| 055 | System prompt engine: layers 6-7 and full assembly | 054, 051 | M | — |
| 056 | Cost tracking and metrics integration | 051, 053 | S | 055 |

### Model config and routing

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 057 | Three-layer model config: provider endpoints | 046 | M | 050, 054 |
| 058 | Three-layer model config: model registry | 057 | M | — |
| 059 | Three-layer model config: hierarchical routing policy | 058, 050 | L | — |
| 060 | Multi-model branching: parallel provider calls | 059, 053 | L | 061 |

### Context, diagnostics, recovery

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 061 | Context window management and interactive compaction | 055, 051 | L | 060 |
| 062 | Content transform (pre-turn summarization) | 052, 059 | S | 061 |
| 063 | Diagnostic taxonomy (MT-CONN-001 through 012) | 048, 041 | M | 050, 054 |
| 064 | In-flight recovery: idempotency and session.sync | 050, 053, 060 | M | 061 |

### CLI and providers

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 065 | CLI: serve, server status, session unlock | 047, 048, 050, 063 | M | 064 |
| 066 | Ollama provider adapter (full interface with formatting) | 042 | M | any from 052+ |
| 067 | BFF server integration tests | all Track A | L | — |

**Track A critical path:** 046 → 050 → 051 → 052 → 053 → 055 → 061 and 052 → 059 → 060.

---

## Track B: FEAT-0009 (Terminal Harness)

### Bubbletea scaffold

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 068 | Bubbletea app scaffold: main model and layout | 039 | M | Track A |
| 069 | Status bar component | 068 | S | 070, 071 |
| 070 | Input area component (multi-line, commands, @file) | 068 | M | 069, 071 |
| 071 | Conversation viewport component (scrollable) | 068 | M | 069, 070 |
| 072 | Streaming markdown rendering (Glamour, debounced) | 071 | M | 073 |

### Protocol client and connection

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 073 | Protocol client: JSON-RPC over socket/TLS | 039-041 | M | 069-072 |
| 074 | Connection manager: lifecycle, auto-start, heartbeat | 073 | L | 072 |

### Tools (all harness-local, no server needed)

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 075 | Tool framework and permission model | 068 | M | 073, 074 |
| 076 | Read tools (text, PDF, DOCX, image, spreadsheet) | 075 | L | 077, 078, 079 |
| 077 | Write and Edit tools | 075 | M | 076, 078, 079 |
| 078 | Bash and Git tools | 075 | M | 076, 077, 079 |
| 079 | Glob, Grep, WebSearch, WebFetch tools | 075 | M | 076, 077, 078 |

### Features

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 080 | Plan/build/auto modes with Ctrl+P toggle | 075, 070, 071 | L | 079, 081 |
| 081 | MCP client: stdio transport and tool discovery | 075, 073 | M | 080 |
| 082 | File context management (@file, drag-drop, /context) | 076, 070 | M | 080, 081 |
| 083 | Large paste handling (content.transform) | 070, 073 | S | 082 |
| 084 | Session explorer and session commands | 073, 071, 068 | L | 080, 085 |
| 085 | Model commands and multi-model branch display | 073, 071 | M | 084 |
| 086 | Connection UX: states, banners, diagnostics | 074, 069 | M | 084, 085 |
| 087 | Harness integration tests with mock server | all Track B | L | — |

**Track B critical path:** 068 → 075 → 076 → 082 → 084 (tools → features).

---

## Integration Track

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 088 | End-to-end: harness → BFF → mock provider | 067, 087 | L | 089 |
| 089 | CLI and harness launch integration | 067, 087 | M | 088 |
| 090 | Documentation and config schema updates | 088, 089 | M | — |

---

## Summary

| Track | WUs | Count | Key Deliverable |
|-------|-----|-------|----------------|
| 0 (Shared) | 039-045 | 7 | Protocol types, provider formatting, session storage |
| A (BFF) | 046-067 | 22 | Server: transport, sessions, routing, streaming, compaction, diagnostics |
| B (Harness) | 068-087 | 20 | UI: Bubbletea, 13 tools, modes, MCP, session explorer, connection UX |
| Integration | 088-090 | 3 | E2E tests, CLI launch, docs |
| **Total** | **039-090** | **52** | |

## Serialization Option

If serializing rather than parallelizing:
- **Option 1 (Server first):** Track 0 → Track A → Track B → Integration. Recommended. The BFF is fully testable with the test harness before the real harness exists.
- **Option 2 (Harness first):** Track 0 → Track B → Track A → Integration. More visual progress early, but harness integrates against mock only.

## Verification

After all WUs complete:
1. `go build ./...` passes
2. `go test -race ./...` passes (all unit + integration tests)
3. `golangci-lint run ./...` passes
4. Manual smoke test: `modeltap` launches harness, connects to local server, send a turn to a real provider, receive streamed response, execute a Read tool, see result in terminal
5. Session persistence: close harness, reopen, resume session with context intact
6. Model switch: `/model llama-3.1-8b`, send a turn, verify routing indicator shows correct model
7. Compaction: `/compact`, review categories, apply, verify context reduced
8. Plan mode: Ctrl+P, submit task, verify plan displayed with no file modifications
