# v0.2.0 Design Review — Kimi

**Review Date:** 2026-04-17  
**Reviewer:** Kimi  
**Scope:** All v0.2.0 design documents (17 design bundles covering WU-039 through WU-096)

## Documents Reviewed

### Design Bundles (17)
- `designs/2026-04-16-design-wu-039-protocol-core.md`
- `designs/2026-04-16-design-protocol-types-040-041-093.md`
- `designs/2026-04-16-design-provider-formatting-042-043-044.md`
- `designs/2026-04-16-design-storage-045-091-096.md`
- `designs/2026-04-16-design-bff-foundation-046-047-048-049.md`
- `designs/2026-04-16-design-sessions-conversation-050-051-052.md`
- `designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md`
- `designs/2026-04-16-design-model-config-routing-057-058-059-060.md`
- `designs/2026-04-16-design-context-diagnostics-recovery-061-062-063-064.md`
- `designs/2026-04-16-design-cli-ollama-history-065-066-091.md`
- `designs/2026-04-16-design-bubbletea-scaffold-068-069-070-071-072.md`
- `designs/2026-04-16-design-protocol-client-073-074.md`
- `designs/2026-04-16-design-tool-framework-075-076-077-078-079.md`
- `designs/2026-04-16-design-harness-features-080-086-092.md`
- `designs/2026-04-16-design-track-integration-tests-067-087.md`
- `designs/2026-04-16-design-integration-track-088-090-094-095.md`

### Cross-References
- `docs/features/0008-bff-server.md`
- `docs/features/0009-terminal-harness.md`
- `docs/adr/0006-multi-provider-support.md`
- `docs/adr/0008-knowledge-layer-architecture.md`
- `docs/adr/0013-terminal-ui-framework.md`

---

## Executive Summary

The v0.2.0 designs represent a comprehensive architectural blueprint for the BFF Server (FEAT-0008) and Terminal Harness (FEAT-0009). The design bundles demonstrate mature architectural thinking with clear separation of concerns, appropriate abstraction layers, and detailed test strategies.

**Assessment:** The designs are **architecturally sound** with **5 blocking contract inconsistencies** identified (overlapping with prior review), **3 additional design risks**, and **4 attention items** requiring clarification before Phase 3 implementation.

---

## Critical Findings (Blocking)

### CF-1: Multi-Model Branch State Persistence Gap [CONFIRMED]

**Severity:** Blocking  
**Location:** Storage bundle (WU-045) vs. FEAT-0008 contract

**Issue:** The storage design explicitly chooses in-memory-only branch state, reporting `multi_model: null` with `cancelled` status after BFF restart. This directly contradicts FEAT-0008's requirement that `session.sync` return per-branch completed/failed/pending status after reconnect, including BFF crash/restart scenarios.

**Design Document Reference:**
- `designs/2026-04-16-design-storage-045-091-096.md:215-229`: "Multi-model branch state is kept in-memory only"
- `docs/features/0008-bff-server.md:1225-1227`: "`session.sync` returns branch states"

**Impact:** Users lose multi-model turn progress on BFF restart, violating the "sessions persist" success criterion.

**Resolution Required:** Either:
1. Add `session_branches` table to persist branch state (preferred)
2. Amend FEAT-0008 to explicitly exclude BFF restart from recovery guarantees

---

### CF-2: Capabilities Registration State Machine Contradiction [CONFIRMED]

**Severity:** Blocking  
**Location:** BFF Foundation bundle (WU-049)

**Issue:** The design states `capabilities.register` may only be called once per connection (rejecting subsequent calls outside `ConnRegistering` state), while simultaneously defining `capabilities.request` as a server-initiated request for the harness to re-register. These rules are mutually exclusive.

**Design Document Reference:**
- `designs/2026-04-16-design-bff-foundation-046-047-048-049.md:538`: "any later call is rejected outside ConnRegistering"
- Same file `:544-564`: `capabilities.request` expects re-registration

**Resolution Required:** Define explicit state transition for `capabilities.request`:
- Option A: `Ready` → `ReRegistering` → `Ready` transition
- Option B: Allow atomic catalog replacement in `Ready` state

---

### CF-3: Protocol Client/Server Wire Shape Mismatch [CONFIRMED]

**Severity:** Blocking  
**Location:** Protocol Client bundle vs. Protocol Types bundle

**Issue:** The harness protocol client expects a different `RegisterResponse` shape than the server produces:
- Server: `protocol.CapabilitiesRegisterResponse` with `registered`, `server_capabilities`, `rejected`
- Client: Local `RegisterResponse` with `negotiated_version`, `server_version`, `max_frame_size`, `max_attachment_size`

**Design Document Reference:**
- `designs/2026-04-16-design-protocol-types-040-041-093.md:283`: Server response shape
- `designs/2026-04-16-design-protocol-client-073-074.md:185`: Client expected shape

**Resolution Required:** Harmonize to single canonical shape; client must decode server's actual response.

---

### CF-4: Diagnostic Code Taxonomy Inconsistency [CONFIRMED]

**Severity:** Blocking  
**Location:** BFF Foundation bundle vs. Protocol Types/Feature Spec

**Issue:** Design invents `MT-CONN-013` for oversize attachments, but the shared taxonomy only defines `MT-CONN-001` through `MT-CONN-012`.

**Design Document Reference:**
- `designs/2026-04-16-design-bff-foundation-046-047-048-049.md:589`: `MT-CONN-013`
- `docs/features/0008-bff-server.md:503`: Only 12 codes defined

**Resolution Required:** Either:
- Map oversize attachments to existing `MT-CONN-012` (protocol violation)
- Expand taxonomy to 13 codes in all documents

---

### CF-5: Provider Interface Instability [CONFIRMED]

**Severity:** Blocking  
**Location:** Streaming bundle vs. Provider Formatting bundle

**Issue:** The provider-formatting bundle (shared contract) defines `Provider` with `FormatMessages` and `FormatToolDefinitions`. The streaming bundle later adds `ParseStreamEvent` method, noting it "will be added as an amendment."

**Design Document Reference:**
- `designs/2026-04-16-design-provider-formatting-042-043-044.md:84`: Original interface
- `designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md:128`: Additional method

**Resolution Required:** Move `ParseStreamEvent` into the shared provider-formatting bundle before Phase 3.

---

## Design Risks (High Severity)

### DR-1: Token Counting Accuracy Across Provider Boundaries

**Severity:** High  
**Location:** Provider formatting bundle, Prompt engine bundle

**Issue:** The design acknowledges that token counting is "provider-specific" and uses `EstimateMessageTokens()` for context window management. However:
1. Different providers use different tokenization schemes (Claude uses cl100k_base, GPT uses their own)
2. The 7-layer prompt assembly in WU-054/055 must stay under budget, but token estimates may drift from actual counts
3. Compaction triggers at 92% may fire too early/late if estimates are inaccurate

**Design Document Reference:**
- `designs/2026-04-16-design-provider-formatting-042-043-044.md:D4`: "Token estimation doesn't count attachment raw bytes"
- `designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md:D3`: "Token counting accurate"

**Risk:** Context window overflow or premature compaction due to estimation errors.

**Mitigation:** 
- Consider adding per-provider tokenizer integration
- Add "safety margin" to budget (e.g., 95% of actual limit)
- Monitor actual vs. estimated in production

---

### DR-2: SQLite WAL Mode and Migration Transaction Safety

**Severity:** High  
**Location:** Storage bundle

**Issue:** The storage design correctly identifies that v2 migration must be transactional. However:
1. SQLite WAL mode (`PRAGMA journal_mode=WAL`) is enabled via DSN pragma
2. WAL mode enables concurrent readers, but writers still block
3. The design notes "v1 retrofit must tolerate wild DBs" with `CREATE TABLE IF NOT EXISTS`

**Potential Issue:** If WAL checkpoint occurs during migration, or if wild v1 DBs have corrupted state, the atomicity guarantee may be violated.

**Design Document Reference:**
- `designs/2026-04-16-design-storage-045-091-096.md:D1`: Transaction discipline notes

**Mitigation:** 
- Force `PRAGMA wal_checkpoint(FULL)` before migration
- Add migration rollback/recovery test cases
- Consider exclusive locking mode during migration

---

### DR-3: MCP Tool Discovery Blocking Behavior

**Severity:** High  
**Location:** Harness Features bundle, Tool Framework bundle

**Issue:** The design requires MCP tool discovery at startup ("connects to configured MCP servers at startup"). However:
1. MCP servers may be slow to start or temporarily unavailable
2. The harness cannot register capabilities until MCP tools are discovered
3. No degraded mode is defined for MCP unavailability

**Design Document Reference:**
- `docs/features/0009-terminal-harness.md`: "MCP tool discovery (required)"
- `designs/2026-04-16-design-tool-framework-075-076-077-078-079.md`: No degraded mode specified

**Risk:** Harness startup blocked by MCP server availability.

**Mitigation:** Define MCP timeout behavior and degraded mode (continue with built-in tools only, retry in background).

---

## Attention Items (Medium Severity)

### AI-1: Foreign Key Enforcement Configuration

**Location:** Storage bundle

**Issue:** Foreign key enforcement in SQLite is disabled by default and must be enabled per-connection. The design uses DSN pragma `_pragma=foreign_keys(1)`, but this is connection-pool specific.

**Concern:** If a connection is created without the DSN (e.g., direct `sql.Open`), FKs may be silently disabled.

**Recommendation:** Add runtime check in `NewSQLiteStore` to verify `PRAGMA foreign_keys` returns `1`.

---

### AI-2: MaxFrameSize Constant Definition Location

**Location:** Protocol Core vs. BFF Foundation

**Issue:** `MaxFrameSize = 10 * 1024 * 1024` is defined in `protocol.go` (WU-039) but referenced for enforcement in `connection.go` (WU-048). The design notes this is "exposed and tested" but the constant is not exported for use by the harness.

**Concern:** Harness needs to know max frame size to avoid serializing oversized attachments.

**Recommendation:** Ensure `MaxFrameSize` is exported and accessible to harness via `capabilities.register` response.

---

### AI-3: Session Lock Timeout vs. Grace Period Interaction

**Location:** BFF Foundation bundle

**Issue:** The design defines:
- Heartbeat timeout: 30 seconds
- Grace period: 10 seconds
- Total before lock release: 40 seconds (30s timeout + 10s grace)

However, the exact interaction between heartbeat detection and grace period start is not explicitly sequenced. If heartbeat timeout fires at 30s but grace period started at connection failure, the total may be 30s (overlap) or 40s (sequential).

**Recommendation:** Clarify the state machine: does grace period start at last pong or at heartbeat timeout detection?

---

### AI-4: Tool Result Tri-State Mapping Ambiguity

**Location:** Provider formatting bundle

**Issue:** The canonical `ToolResult` has tri-state status: `success`, `rejected`, `error`. The design maps these to provider formats:
- Anthropic: `is_error: true` for both rejected and error
- OpenAI: No `is_error`, prepend text with `[error: ...]` or `[rejected: ...]`

**Concern:** The distinction between rejected (user declined) and error (execution failed) is lost on Anthropic, which may affect model behavior.

**Recommendation:** Verify if Anthropic's `is_error: true` with explanatory text preserves sufficient distinction for the model.

---

## Design Strengths

### DS-1: Protocol Contract Stability

The `internal/protocol/` package approach (types ARE the spec) eliminates spec drift. Round-trip tests and golden fixtures (WU-093) provide strong contract enforcement.

### DS-2: Provider Adapter Pattern

The `Provider` interface with `FormatMessages`/`FormatToolDefinitions` cleanly separates provider-specific formatting from BFF business logic. The canonical `Message` type with metadata provides necessary provenance.

### DS-3: Layered Prompt Architecture

The 7-layer prompt assembly (Layers 1-5 in WU-054, 6-7 in WU-055) with explicit trimming order (Layer 6 first, then 7, 4, 3) provides predictable behavior under budget constraints.

### DS-4: Transactional Migration Discipline

The v2 migration is correctly designed as transactional (all-or-nothing), with downgrade guards for forward compatibility. The asymmetry with v1 (idempotent) is well-justified.

### DS-5: Permission Model Consistency

The three-level permission model (default/accept-edits/autonomous) with risk-level mapping matches proven patterns from Claude Code and is consistently applied across all 13 tools.

### DS-6: Streaming Debounce Strategy

The Bubbletea streaming workaround (buffer tokens, debounced redraw at 50ms, final clean render) is well-understood and follows OpenCode precedent.

---

## Cross-Bundle Consistency Assessment

| Contract | Status | Notes |
|----------|--------|-------|
| Protocol wire format | ✅ Consistent | Snake_case, JSON-RPC 2.0, NDJSON |
| Provider interface | ⚠️ At risk | `ParseStreamEvent` needs consolidation |
| Session storage schema | ✅ Consistent | v2 migration handles all fields |
| Connection state machine | ⚠️ At risk | Re-registration path needs resolution |
| Tool result tri-state | ✅ Consistent | Mapped correctly in canonical form |
| Diagnostic taxonomy | ⚠️ Broken | `MT-CONN-013` not in shared taxonomy |
| Cost tracking | ✅ Consistent | Per-turn + session total + aggregation |
| Routing policy | ✅ Consistent | Dot-path resolution with fallback |

---

## Implementation Readiness

### Ready to Implement (No Blockers)

| Bundle | WUs | Confidence |
|--------|-----|------------|
| Protocol Core | WU-039 | High |
| Protocol Types | WU-040-041, 093 | High |
| Provider Formatting | WU-042-044 | Medium-High (pending CF-5) |
| Storage | WU-045, 091, 096 | Medium (pending CF-1) |
| Sessions/Conversation | WU-050-052 | High |
| Streaming/Prompts/Cost | WU-053-056 | Medium (pending DR-1) |
| Model Config/Routing | WU-057-060 | High |
| Context/Diagnostics | WU-061-064 | High |
| Bubbletea Scaffold | WU-068-072 | High |
| Tool Framework | WU-075-079 | Medium (pending DR-3) |

### Blocked Pending Resolution

| Bundle | Blocker |
|--------|---------|
| BFF Foundation | CF-2 (re-registration) |
| Protocol Client | CF-3 (wire shape) |
| Integration Tests | CF-1, CF-2, CF-3, CF-4, CF-5 |

---

## Recommendations

### Immediate (Before Phase 3)

1. **Resolve CF-1 through CF-5** — These are cross-bundle contract breaks that will cause integration failures
2. **Clarify DR-3 (MCP degraded mode)** — Define behavior when MCP servers are unavailable
3. **Add FK enforcement verification** (AI-1) — Runtime check in storage initialization

### During Implementation

4. **Monitor DR-1 (token counting)** — Add telemetry to compare estimated vs. actual tokens
5. **Test DR-2 (WAL/migration)** — Add explicit WAL checkpoint and recovery test cases
6. **Verify AI-3 (lock timeout)** — Document exact state machine timing in code comments

### Post-Implementation

7. **Add provider-specific tokenizers** — If estimation errors exceed 5%, integrate actual tokenizers
8. **Evaluate branch state persistence** — If CF-1 remains unresolved, monitor field data for impact

---

## Comparison with Prior Review

This review **confirms all 5 blocking findings** from the prior codex-design-review:
- CF-1 = Finding 1 (multi-model branch state)
- CF-2 = Finding 2 (capabilities re-registration)
- CF-3 = Finding 3 (wire shape mismatch)
- CF-4 = Finding 4 (diagnostic taxonomy)
- CF-5 = Finding 5 (provider interface instability)

**Additional findings** in this review:
- DR-1: Token counting accuracy risk
- DR-2: WAL/migration safety
- DR-3: MCP blocking behavior
- AI-1 through AI-4: Implementation-level concerns

---

## Conclusion

The v0.2.0 designs demonstrate mature architectural thinking with appropriate abstraction boundaries and comprehensive test coverage. The **5 blocking contract inconsistencies** must be resolved before Phase 3 implementation begins to avoid integration failures.

Once blockers are resolved, the remaining **3 design risks** and **4 attention items** can be managed during implementation with the suggested mitigations.

**Overall verdict:** Designs are sound pending resolution of cross-bundle contract breaks.

---

*Review complete. 5 blocking, 3 high-risk, 4 attention items identified.*
