# v0.2.0 Workplan Review — Kimi

**Review Date:** 2026-04-16  
**Reviewer:** Kimi  
**Scope:** Complete v0.2.0 release workplan against FEAT-0008 and FEAT-0009  

Reviewed documents:
- `.sdlc/releases/v0.2.0/plan.md`
- `.sdlc/releases/v0.2.0/track-0-shared.md`
- `.sdlc/releases/v0.2.0/track-a-bff-server.md`
- `.sdlc/releases/v0.2.0/track-b-terminal-harness.md`
- `.sdlc/releases/v0.2.0/track-integration.md`
- `.sdlc/releases/v0.2.0/status.md`
- `.sdlc/features/0008-bff-server.md`
- `.sdlc/features/0009-terminal-harness.md`

---

## Executive Summary

The v0.2.0 workplan is comprehensive and well-structured across 52 work units (WU-039 through WU-090). The three-track structure provides clear separation of concerns.

**Critical Finding:** The workplan has **4 blocking gaps** where the feature specifications require functionality not assigned to any work unit. These must be addressed before implementation proceeds.

---

## Blocking Gaps (Must Fix)

### 1. Protocol Negotiation and Project Context in `capabilities.register`

**Feature Requirement (FEAT-0008):**
- Version exchange during `capabilities.register`
- Version-mismatch rejection
- Project-root capture
- Config-content capture for Layer 4 prompt assembly
- Harness metadata (version, platform)

**Workplan Gap:**
WU-049 only covers "tool catalog registration and schema validation." The definition of done does not include:
- Protocol version negotiation
- Version-mismatch rejection handling
- Project context ownership

**Fix:** Expand WU-049 or add WU-049b for protocol negotiation and project context handling.

---

### 2. Cross-Session Command History

**Feature Requirement (FEAT-0009):**
> "Command history traversal (up/down arrows) sourced from the BFF (cross-session, cross-project)"

Success criterion #3: "Command history traversal (up/down arrows) sourced from the BFF (cross-session, cross-project)."

**Workplan Gap:**
- WU-070 mentions "local input history traversal" but this is harness-local only
- No protocol method for history access (server side)
- No storage/indexing of command history
- No server-side history scoping rules

**Fix:** Add work unit(s) for:
- Protocol message for history access
- Storage schema and CRUD for command history
- Server handler for history queries
- Harness integration to fetch history from server

---

### 3. Server CLI Commands — `server sessions` and `server session <id>`

**Feature Requirement (FEAT-0008):**
> "Operator commands: `modeltap server status` (health overview), `modeltap server sessions` (active session list), `modeltap server session <id>` (session details)."

**Workplan Gap:**
WU-065 includes:
- `modeltap serve`
- `modeltap server status`
- `modeltap session unlock <id>`

Missing:
- `modeltap server sessions`
- `modeltap server session <id>`

**Fix:** Add to WU-065 or create adjacent WU.

---

### 4. Session Storage Schema Completeness

**Feature Requirement (FEAT-0008):**
Session persistence must include:
- project association
- routing overrides
- pinned items
- token totals
- compaction state
- files touched and modified
- server events
- compaction summaries

**Workplan Gap:**
WU-045 commits to "sessions and turns tables" but does not explicitly enumerate these fields. Track A work units (WU-050, WU-055, WU-061) depend on these fields but the schema is defined in Track 0.

Risk: Schema may need revision mid-Track A if fields are discovered missing.

**Fix:** Expand WU-045 to explicitly list all required session fields and any supporting tables (e.g., for server events, file tracking).

---

## Additional Attention Items

### 5. Session Lock Mechanics — Grace Period and Force-Release

**Feature Requirement (FEAT-0008):**
- "grace period (default: 10 seconds, to handle brief reconnections)"
- "heartbeat timeout and releases the lock"
- "force-release a stuck session via `modeltap session unlock <id>`"

**Workplan Coverage:**
- WU-050 mentions "Session lock (one harness per session)"
- WU-048 mentions heartbeat handler

**Gap:** Grace period duration, heartbeat timeout values, and force-release mechanics are not explicitly specified.

**Recommendation:** Add to WU-048 (connection lifecycle) or WU-050 (session management).

---

### 6. Compaction Configuration Surface

**Feature Requirement (FEAT-0008):**
- "configurable via `context.compact_model`"
- "configurable threshold" (for auto-compaction)

**Workplan Coverage:**
- WU-061 specifies 92% auto-compaction threshold
- WU-057 covers provider config

**Gap:** Configuration surface for compaction model and threshold not explicitly assigned.

**Recommendation:** Ensure WU-057 or WU-061 includes config parsing for `context.compact_model` and configurable threshold.

---

### 7. MCP Tool Discovery Timing

**Feature Requirement (FEAT-0009):**
- "MCP tool discovery (required): The harness connects to configured MCP servers at startup"

**Workplan Coverage:**
WU-081 positions MCP client after tools and modes.

**Question:** If MCP discovery fails or is slow, does it block harness startup? Should there be a degraded mode?

**Recommendation:** Consider positioning WU-081 earlier or specifying startup behavior when MCP is unavailable.

---

## Strengths (Correctly Covered)

| Feature Requirement | Work Unit | Coverage |
|---------------------|-----------|----------|
| Protocol types and framing | WU-039–041 | ✅ Complete |
| Provider outbound formatting | WU-042–044, WU-066 | ✅ Complete |
| Session CRUD + lock | WU-050 | ✅ Covered |
| 7-layer prompt engine | WU-054–055 | ✅ Complete |
| Multi-model branching | WU-060 | ✅ Covered |
| Context compaction | WU-061 | ✅ Covered |
| All 13 built-in tools | WU-075–079 | ✅ Complete |
| Plan/build/auto modes | WU-080 | ✅ Covered |
| Session explorer | WU-084 | ✅ Covered |
| Streaming markdown | WU-072 | ✅ Covered |
| Connection state machine (9 states) | WU-048, WU-074 | ✅ Consistent |
| Tool permission model | WU-075 | ✅ Covered |
| Cost tracking | WU-056, WU-069 | ✅ Covered |
| Integration tests | WU-067, WU-087, WU-088 | ✅ Complete |

---

## Recommendations

### Immediate Actions

1. **Expand WU-049** or add WU-049b for protocol version negotiation, project context capture, and version-mismatch rejection.

2. **Add work unit(s)** for cross-session command history: protocol messages, storage schema, server handlers, harness integration.

3. **Expand WU-065** to include `modeltap server sessions` and `modeltap server session <id>`.

4. **Expand WU-045** to enumerate all required session fields: project association, routing overrides, pinned items, token totals, compaction state, files touched, server events, compaction summaries.

### Implementation Approach

5. **Track 0 must complete before Track A/B** — This is correct and should be maintained. The protocol contract stability is worth the sequencing.

6. **Monitor WU-060 and WU-061** during implementation — These contain the most complex logic (multi-model branching, context compaction).

7. **Validate protocol contract early** — Run WU-039 through WU-044 tests against real provider responses before parallel tracks begin.

---

## Risk Summary

| Risk | Level | Status |
|------|-------|--------|
| Protocol negotiation gap | High | **BLOCKING** — Not assigned |
| Command history gap | High | **BLOCKING** — Not assigned |
| Missing server CLI commands | Medium | **BLOCKING** — Not assigned |
| Storage schema underspecified | Medium | **BLOCKING** — Fields not enumerated |
| Multi-model concurrency | Medium | Managed — WU-060 dedicated |
| Tool framework scope | Low-Medium | Managed — Well-decomposed |
| MCP startup dependency | Low | Consider early in implementation |

---

## Comparison with Existing Review

An existing `plan-review.md` (codex-plan-review.md) identified the same 4 blocking gaps. This review confirms those findings and adds attention items for grace period mechanics, compaction configuration, and MCP timing.

---

*Review complete. 4 blocking gaps identified that must be addressed before implementation.*
