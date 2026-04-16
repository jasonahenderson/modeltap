# v0.2.0 Workplan Review — Kimi

**Review Date:** 2026-04-16
**Reviewer:** Kimi
**Scope:** Complete v0.2.0 release workplan

Reviewed documents:
- `docs/releases/v0.2.0/plan.md`
- `docs/releases/v0.2.0/track-0-shared.md`
- `docs/releases/v0.2.0/track-a-bff-server.md`
- `docs/releases/v0.2.0/track-b-terminal-harness.md`
- `docs/releases/v0.2.0/track-integration.md`
- `docs/releases/v0.2.0/status.md`

Cross-referenced:
- `docs/features/0008-bff-server.md`
- `docs/features/0009-terminal-harness.md`
- `docs/adr/0008-knowledge-layer-architecture.md`
- `docs/adr/0013-terminal-ui-framework.md`

---

## Executive Summary

The v0.2.0 workplan is comprehensive and well-structured across 52 work units (WU-039 through WU-090). The three-track structure (Track 0 prerequisites, Track A BFF Server, Track B Terminal Harness, plus Integration) provides clear separation of concerns and identifies genuine parallelization opportunities.

**Overall assessment:** Ready for implementation with minor attention items.

---

## Strengths

### Clear Dependency Mapping
The critical path analysis in Track A (~14 sequential WUs) and Track B (~8 sequential WUs) accurately identifies the longest dependency chains. The "WU X → Y → Z" notation in track documents makes sequencing explicit.

### Appropriate Granularity
Work units are sized appropriately (Small/Medium/Large) with clear definitions of done. The parallelization annotations ("Parallelizes with:") reduce false sequencing constraints.

### Protocol-First Design
Track 0's focus on protocol types (WU-039 through WU-044) before implementation ensures the contract between BFF Server and Terminal Harness is stable before parallel development proceeds.

### Testing Strategy
Each track culminates in integration tests (WU-067, WU-087, WU-088), with the final Integration Track providing full-stack validation. This mirrors the pyramid testing approach appropriate for a protocol-heavy release.

### State Machine Clarity
WU-048 (connection lifecycle) and WU-074 (harness connection manager) both document the 9-state lifecycle consistently, reducing risk of protocol mismatches between server and harness.

---

## Attention Items

### 1. WU-045 Storage Schema Scope
**Location:** Track 0, WU-045
**Observation:** While WU-045 commits to `sessions` and `turns` tables, the full scope of session persistence required by FEAT-0008 (routing overrides, pinned items, compaction state, server events) may benefit from explicit enumeration in the work unit.
**Recommendation:** Consider a table schema draft or field list in the WU definition to avoid mid-track schema revisions.

### 2. Track B Local Work Gating
**Location:** Track dependencies
**Observation:** The status.md states "Track 0 must complete before Track A or B begins," but several Track B work units (WU-068 scaffold, WU-075 tool framework) are harness-local and only require WU-039 protocol contract.
**Recommendation:** Evaluate relaxing the blanket gate to allow earlier Track B start for local components, as noted in the existing `plan-review.md`.

### 3. Multi-Model Branching Complexity
**Location:** Track A, WU-060
**Observation:** Multi-model branching introduces significant concurrency complexity (parallel goroutines, branch_id tagging, aggregate completion). The 22 work units in Track A may expand if error handling for partial branch failures requires additional design.
**Recommendation:** Monitor WU-060 closely during implementation; consider a spike or design checkpoint if branch synchronization patterns prove complex.

### 4. Tool Framework Surface Area
**Location:** Track B, WU-075 through WU-079
**Observation:** 13 tools across 5 work units (Read, Write/Edit, Bash/Git, Glob/Grep/WebSearch/WebFetch) is aggressive. The Read tool alone covers 5 file types (text, PDF, DOCX, image, spreadsheet).
**Recommendation:** Verify external dependencies (pdfcpu, unioffice, excelize) are accounted for in dependency management. Consider tiered delivery if tool implementation becomes a bottleneck.

### 5. Context Compaction Threshold
**Location:** Track A, WU-061
**Observation:** The 92% auto-compaction threshold is hardcoded in the work unit. FEAT-0008 specifies "configurable threshold" in success criteria.
**Recommendation:** Ensure WU-061 or WU-057 (provider config) includes configuration surface for this threshold.

---

## Cross-Cutting Concerns

### Cost Tracking Consistency
Cost tracking appears in three places:
- WU-056 (Track A): per-turn cost, session total, aggregation
- WU-069 (Track B): status bar display
- WU-088 (Integration): accuracy verification

The consistency of token counting between provider adapters (WU-043, WU-044, WU-066) and the harness display should be verified during integration.

### MCP Client Timing
**Location:** Track B, WU-081
MCP client implementation is positioned late in Track B (after tools and modes). If MCP tool discovery becomes critical for harness functionality, consider whether WU-081 should move earlier or have reduced scope for initial integration.

### Session Lock Mechanics
**Location:** Track A, WU-050
The session lock ("one harness per session") is essential for correctness but introduces operational need for `session unlock` commands (WU-065). Ensure lock timeout/heartbeat failure scenarios are covered in WU-048 or WU-050.

---

## Documentation Completeness

The release plan is accompanied by:
- ✅ Feature specs (FEAT-0008, FEAT-0009)
- ✅ ADRs (0008 knowledge layer, 0013 terminal UI)
- ✅ Usage guide (to be updated in WU-090)
- ✅ Status tracking document

The Integration Track (WU-088 through WU-090) correctly schedules documentation and changelog updates after implementation completes, avoiding stale documentation during active development.

---

## Risk Assessment

| Risk | Level | Mitigation |
|------|-------|------------|
| Protocol contract churn | Medium | Track 0 sequencing; protocol types well-defined |
| Multi-model concurrency | Medium | WU-060 has dedicated focus; integration tests cover it |
| Tool framework scope | Low-Medium | Well-decomposed across WU-075 through WU-079 |
| Storage schema iteration | Low | WU-045 v2 migration approach; schema should stabilize in Track 0 |
| Cross-track integration | Low | Dedicated Integration Track with full E2E tests |

---

## Recommendations

1. **Proceed with implementation** — The workplan is ready for execution.
2. **Monitor WU-060 and WU-061 closely** — These contain the most complex logic (branching, compaction).
3. **Consider early Track B start** — Evaluate parallelizing WU-068 through WU-075 with Track 0 completion, as they only need protocol contract stability.
4. **Validate cost accuracy early** — Run provider adapter tests (WU-043, WU-044) against real provider responses to verify token counting matches expected behavior.
5. **Maintain status.md** — Update the status document as WUs complete to maintain visibility into release health.

---

*Review complete. No blocking issues identified.*
