# Pre-Review: Design Bundles 10-15

**Reviewer:** Claude subagent (fresh context)
**Date:** 2026-04-16
**Scope:** 6 design docs covering WU-053 through WU-095
**Focus:** BLOCKING issues only (type mismatches, missing required features, spec contradictions)

## Summary

**Total blocking issues: 3**

---

## Bundle 10: Streaming, Prompts, Cost (WU-053/054/055/056)

**Source specs checked:** FEAT-0008 (system prompt, streaming relay, cost tracking), track-a-bff-server.md

### BLOCKING-01: TokenDelta field name mismatch — `Content` vs `Text`

The design (D2.2) emits `token.delta` with field `Content`:
```go
TokenDelta{TurnID: turnID, BranchID: branchID, Content: chunk}
```

FEAT-0008's protocol types (`internal/protocol/events.go`) define `TokenDelta` with field `Text`:
```go
type TokenDelta struct {
    TurnID   string `json:"turn_id"`
    BranchID string `json:"branch_id,omitempty"`
    Text     string `json:"text"`
}
```

The streaming relay will either fail to compile (if using the shared protocol type) or emit a non-canonical field name (if defining its own struct). Since both BFF and harness import `internal/protocol/`, this will break contract compatibility.

**Fix:** Change `Content: chunk` to `Text: chunk` in D2.2 streaming relay emission code.

### BLOCKING-02: Prompt trimming order contradicts FEAT-0008

The design (D4.4) trims in this order: Layer 6 first, then Layer 7, then Layer 4, then Layer 3. Layers 1, 2, 5 are "never trimmed."

FEAT-0008 states: "the server trims Layer 6 (knowledge injections) first, then Layer 7 (session state summaries), preserving Layers 1-5 which are essential for behavior quality."

The spec says Layers 1-5 are preserved. The design adds Layer 3 (domain config) and Layer 4 (project instructions) to the trim list after 6 and 7. This contradicts the spec's explicit "preserving Layers 1-5" statement. If the design trims Layer 4 (project instructions) or Layer 3 (domain config), users lose project-level and domain-level instructions under pressure -- behavior FEAT-0008 explicitly says should not happen.

**Fix:** Remove Layers 3 and 4 from the trim order. Only Layers 6 and 7 are trimmable per spec. If the system prompt budget is exceeded after trimming 6 and 7, return as-is with a warning (the design already handles the "pinned exceeds budget" case this way).

### No other blocking issues in this bundle.

Cost tracking (WU-056) correctly uses `CostPer1kInput`/`CostPer1kOutput` aligned with FEAT-0008's `cost_per_1k_input`/`cost_per_1k_output` fields. `turn.complete` includes all required fields. Aggregation feed path is consistent with ADR-0007.

---

## Bundle 11: Context, Diagnostics, Recovery (WU-061/062/063/064)

**Source specs checked:** FEAT-0008 (compaction, diagnostics, idempotency, session.sync), track-a-bff-server.md

### No blocking issues.

- WU-061 compaction config uses `PressureWarningThreshold` / `AutoCompactThreshold` matching FEAT-0008's `warning_threshold: 0.78` / `compact_threshold: 0.92`. The track spec's config key names (`pressure_warning_threshold` / `auto_compact_threshold`) differ from FEAT-0008's (`warning_threshold` / `compact_threshold`), but since the track spec is the authoritative WU-level definition and it explicitly calls out the config block, the design follows the track spec correctly.
- WU-062 `content.transform` handler matches FEAT-0008 payload schema. Field names align: `raw_content`, `transform`, `max_output_tokens`.
- WU-063 implements all 12 diagnostic codes. Terminal vs non-terminal classification matches FEAT-0008's taxonomy table. State transitions align with connection lifecycle.
- WU-064 `session.sync` response matches FEAT-0008's JSON schema. `token_replay_available: false` is correctly hardcoded per spec. Multi-model branch state included via `BranchManager.BranchState()`.

---

## Bundle 12: CLI, Ollama Provider, Command History (WU-065/066/091)

**Source specs checked:** FEAT-0008 (CLI commands, Ollama provider), FEAT-0009 (command history), track-a-bff-server.md, track-b-terminal-harness.md

### No blocking issues.

- WU-065 CLI commands match FEAT-0008 CLI Integration section exactly: `serve`, `server status`, `server sessions`, `server session <id>`, `session unlock`.
- WU-066 Ollama adapter implements `FormatMessages`, `FormatToolDefinitions`, and `ParseStreamEvent`. The `ParseStreamEvent` method is the new addition acknowledged in Bundle 10's design (D2.3 note). NDJSON streaming (not SSE) is correctly specified for Ollama. FEAT-0008 lists Ollama in the provider format translation section as "`messages` array with `role` and `content` string" -- the design matches.
- WU-091 `history.list` scoping rules match track-a-bff-server.md: `user_id` always enforced, project and session scope filter additively. Pagination cursor is cursor-based per track spec.

---

## Bundle 13: Harness Features (WU-080/081/082/083/084/085/086/092)

**Source specs checked:** FEAT-0009 (modes, MCP, session explorer, model commands, connection UX, file context, large paste, command history), track-b-terminal-harness.md

### BLOCKING-03: Missing `context.list` protocol call in `/context` command handler

FEAT-0009 success criterion 19: "/context command shows files, knowledge injections, and token budget." FEAT-0008 defines a `context.list` protocol message (harness -> server) that returns files, knowledge_injections, pinned_items, context_tokens, context_window, context_pct, system_prompt_tokens, and knowledge_injection_tokens.

The design (D3, WU-082) implements `HandleContextCommand` as a local operation:
```go
func (cm *ContextManager) HandleContextCommand(cmd, args string) tea.Cmd
```

The `ContextManager` only has `readTool` and `state` -- no `ConnectionManager` to send `context.list` to the BFF. The `/context` command must query the server for authoritative context data (token counts, knowledge injections, system prompt tokens) that the harness does not have locally. Without the `context.list` call, the harness cannot display token budget or knowledge injection information.

**Fix:** Add `conn *ConnectionManager` to `ContextManager` and implement `/context` as a `context.list` RPC call to the BFF, rendering the server's response. Local-only data (attached files) can supplement the response but cannot replace it.

### No other blocking issues in this bundle.

- WU-080 mode switching matches FEAT-0009: Ctrl+P toggles plan<->build, `/plan`/`/build`/`/auto` commands, mode sent on `turn.submit`. Plan interception uses `RiskLevel` which aligns with the tool catalog's `risk_level` field.
- WU-081 MCP non-blocking startup matches track-b spec's explicit requirement. `/mcp status` and `/mcp reconnect` commands are present.
- WU-083 large paste flow matches FEAT-0009: threshold detection, preview, 4 choices (summarize/full/truncate/cancel), summarize via `content.transform`.
- WU-084 session explorer matches FEAT-0009 success criteria 14-16. Auto-resume logic (single session + no events) is present.
- WU-085 model commands match FEAT-0009 success criteria 11-13. Multi-model branch display with progressive completion is present.
- WU-086 connection UX matches FEAT-0009 success criteria 21-26. All 4 status bar indicators specified. Diagnostic rendering includes code, cause, and suggestion.
- WU-092 command history matches FEAT-0009: BFF-sourced, cross-session, cross-project. Scope switching commands present.

---

## Bundle 14: Track Integration Tests (WU-067/087)

**Source specs checked:** FEAT-0008 success criteria, FEAT-0009 success criteria, track-a-bff-server.md, track-b-terminal-harness.md

### No blocking issues.

- WU-067 test matrix covers all FEAT-0008 success criteria (core 1-9, protocol 19-31, connectivity 10-18). Specific coverage: version negotiation (SC 26+31), project context (SC 26), command history (from WU-091), server sessions/session detail payloads (SC 22-23), idempotency (SC 14), session.sync (SC 13+15), content.transform (SC 28), context pressure (SC 7+24-25), cost tracking (SC 6), multi-model (SC 29), diagnostics (SC 16+18+30).
- WU-087 test matrix covers all FEAT-0009 success criteria. All 13 tools tested (SC 3). Plan mode with approve/step-through (SC 10). Session explorer (SC 14). Command history (SC from FEAT-0009 conversation interface). Connection states (SC 21-26). MCP (SC 18). File context (SC 5+19).
- Shared protocol fixtures (WU-093) dependency is noted, ensuring wire format consistency between mock BFF and real BFF.

---

## Bundle 15: Integration Track (WU-088/089/090/094/095)

**Source specs checked:** track-integration.md, FEAT-0008 success criteria, FEAT-0009 success criteria

### No blocking issues.

- WU-088 E2E test matrix matches track-integration.md exactly: 10 test scenarios covering connect, streaming, tool round-trip, session persistence, model switch, compaction, multi-model, cost accuracy (5%), diagnostic propagation, connection recovery.
- WU-089 CLI integration matches FEAT-0009 CLI Integration section: `modeltap` (no subcommand) launches harness, `--resume`, `--project`, `--model` flags, auto-start, existing subcommands unchanged.
- WU-090 documentation deliverables match track-integration.md: usage guide, config schema docs, help updates, changelog.
- WU-094 security scope covers all items from track-integration.md: path traversal, command injection, SSRF, SQL injection lint, session lock bypass, credential leakage, malformed NDJSON, socket permissions, sequencing, capture redaction. Test file locations match the spec's colocated test requirement.
- WU-095 budget table matches track-integration.md exactly (all 10 scenarios with identical budgets). Makefile target `make bench` specified. Reference machine documented. CI looser budgets acknowledged.

---

## Blocking Issue Summary

| ID | Bundle | WU | Issue | Severity |
|----|--------|----|-------|----------|
| BLOCKING-01 | 10 | 053 | `Content` vs `Text` field name in `TokenDelta` | Type mismatch -- will break protocol contract |
| BLOCKING-02 | 10 | 054/055 | Trimming Layers 3+4 contradicts FEAT-0008 "preserving Layers 1-5" | Spec contradiction |
| BLOCKING-03 | 13 | 082 | `/context` command missing `context.list` server call | Missing required feature |
