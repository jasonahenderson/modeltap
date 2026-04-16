# v0.2.0 Status

## Last Updated
2026-04-16

## Current Phase
**Release Phase 1 — Design.** WU-039 shipped under the earlier per-WU workflow (complete). Track 0 Phase 1 design complete (3 bundles). Tracks A, B, and Integration Phase 1 remaining.

## Planned
See `plan.md`, `track-0-shared.md`, `track-a-bff-server.md`, `track-b-terminal-harness.md`, `track-integration.md`.

58 work units total: WU-039 through WU-096.

## Completed
- [x] WU-039: Protocol types — core messages and framing (2026-04-16) — commits `28213eb` (red), `1aa3830` (green), `50febd0` (security fix SR-039-01). Tests, design doc, and security review doc in `docs/history/`.

### Phase 1 Design Artifacts (Track 0)
- [x] Bundle 1 — Protocol types (WU-040 + 041 + 093) — design `docs/history/2026-04-16-design-protocol-types-040-041-093.md`; pre-review `docs/releases/v0.2.0/.reviews/protocol-types-040-041-093/claude-subagent-pre-review.md`. Commit `f9429e4`.
- [x] Bundle 2 — Provider formatting (WU-042 + 043 + 044) — design `docs/history/2026-04-16-design-provider-formatting-042-043-044.md`; pre-review `docs/releases/v0.2.0/.reviews/provider-formatting-042-043-044/claude-subagent-pre-review.md`. Commit `3fb9588`.
- [x] Bundle 3 — Storage (WU-045 + 091 + 096) — design `docs/history/2026-04-16-design-storage-045-091-096.md`; pre-review `docs/releases/v0.2.0/.reviews/storage-045-091-096/claude-subagent-pre-review.md`. Commit `99c724e`.

## In Progress
(none yet)

## Up Next
Track 0 (all now runnable in parallel — any subset):
- WU-040: Protocol types — streaming events
- WU-041: Protocol types — tools, sessions, models, health, errors
- WU-042: ADR-0006 amendment — provider outbound formatting interface
- WU-045: Session and turn storage schema (migration v2)
- WU-093: Protocol contract — shared golden fixtures and conformance tests (can begin once WU-040 and WU-041 land to cover their types)

Track B harness-local work (WU-068-072, WU-075-079) is also unblocked now that WU-039 is done. Recommended: finish Track 0 first (server-first serialization per plan.md §"Serialization Option").

## Blocked
(none)

## Notes
- Tracks A and B can run in parallel or be serialized (server-first recommended)
- Track 0 gate for Track A: all of WU-039–WU-045 must complete before Track A foundation begins
- Track 0 gate for Track B (relaxed): harness-local WUs (068-072, 075-079) may start after WU-039; protocol-client and session-aware WUs additionally require WU-040/041
- WU-093 (protocol contract fixtures) is a prerequisite for WU-067 and WU-087 (integration suites), not for Track A/B foundation WUs
- WU-096 (migration upgrade tests) parallelizes with WU-043/044 and any Track A WU after WU-045
- Integration track (WU-088-090, 094, 095) runs after both tracks complete
- WU-091 and WU-092 were added 2026-04-16 from the plan review to cover cross-session command history (FEAT-0009)
- WU-093, WU-094, WU-095, WU-096 were added 2026-04-16 from the test-coverage gap review

## Plan Review History
- 2026-04-16 — first round of plan reviews processed; see `.reviews/codex-plan-review.md` and `.reviews/kimi-plan-review.md`, and `docs/history/2026-04-16-release-v0.2.0-plan-review-processed.md`
- 2026-04-16 — feature-gaps review processed; see `.reviews/kimi-plan-review-feat-gaps.md` and `docs/history/2026-04-16-release-v0.2.0-feat-gaps-review-processed.md` (4 blocking items already fixed; 3 attention items applied to WU-048, WU-050, WU-061, WU-081)
- 2026-04-16 — test-coverage gap review: added WU-093 (protocol contract fixtures), WU-094 (security review suite), WU-095 (performance benchmarks and budgets), WU-096 (storage migration upgrade tests); see `docs/history/2026-04-16-release-v0.2.0-test-coverage-gap-review.md`
