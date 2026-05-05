# 2026-04-16 — Session: Phase 1 Track 0 Designs + Workflow Establishment

## Summary

Long session covering two streams of work: (1) establishing the release-level phased workflow for v0.2.0 and (2) completing Phase 1 designs for all Track 0 bundles.

## 1. Workflow evolution (5 iterations this session)

The design-review workflow went through rapid evolution based on real feedback from WU-039's implementation + review cycle:

1. **Initial:** per-WU end-to-end (design → test → impl → review per WU).
2. **Added Tier A/B/C** review tiers with mechanical rules. Claude subagent listed as Tier C fallback.
3. **Corrected:** user pointed out Tier C must be peer-model (different model family), not same-model. Subagent repositioned as "pre-review lint."
4. **Simplified:** peer-review handoff is chat-only (WU + file paths), no committed prompt file. The external reviewer decides their own framing.
5. **Phased at release level:** user directed design ALL → peer-review ALL → code ALL. Track-level phasing abandoned in favor of release-level. Prime directives added to prevent drift.

Final state: three process files (`docs/agents.md`, `CLAUDE.md`, `AGENTS.md`) all contain explicit prime directives.

## 2. WU-039 (retroactive, completed this session)

WU-039 was the only WU that went through the old per-WU end-to-end workflow. It now has:
- Implementation (protocol.go, messages.go, protocol_test.go)
- Design doc
- Subagent pre-review (3 B / 7 A / 3 N — all resolved)
- Codex peer review (1 H / 1 M / 1 L — all resolved; found real contract drift on Request.ID omitempty)
- 20 harness→server request types + Notification envelope type
- Security review (SR-039-01 fixed; 4 informational deferred)

## 3. Phase 1 Track 0 designs (3 bundles)

| Bundle | WUs | Design doc | Pre-review findings |
|--------|-----|------------|---------------------|
| Protocol types | 040 + 041 + 093 | `2026-04-16-design-protocol-types-040-041-093.md` | 3B/11A/4N |
| Provider formatting | 042 + 043 + 044 | `2026-04-16-design-provider-formatting-042-043-044.md` | 5B/11A/6N |
| Storage | 045 + 091 + 096 | `2026-04-16-design-storage-045-091-096.md` | 4B/10A/5N |

All Blocking and fix-now Attention items addressed in-design.

Key design decisions (cross-bundle):
- `Notification` type for server→harness streaming events (no id field)
- `TurnSubmitResponse` / `TurnCancelResponse` / `ToolResultResponse` types for streaming-initiation request acks
- `FormatMessagesOpts` struct with Capabilities field for vision gating
- Truncation pair-reconciliation algorithm (handles multi-turn tool interleaving)
- Migration versioning via PRAGMA user_version; v1 idempotent / v2 transactional
- Foreign keys via DSN pragma (per-connection enforcement)
- Multi-model branch state is in-memory only for v0.2.0

## Files created this session

### Design docs
- `docs/history/2026-04-16-design-wu-039-protocol-core.md` (WU-039 — retroactive)
- `docs/history/2026-04-16-design-protocol-types-040-041-093.md` (Track 0 Bundle 1)
- `docs/history/2026-04-16-design-provider-formatting-042-043-044.md` (Track 0 Bundle 2)
- `docs/history/2026-04-16-design-storage-045-091-096.md` (Track 0 Bundle 3)

### Pre-review lint artifacts
- `docs/releases/v0.2.0/.reviews/wu-039/claude-subagent-pre-review.md`
- `docs/releases/v0.2.0/.reviews/wu-039/codex-design-and-code-review.md` (peer review)
- `docs/releases/v0.2.0/.reviews/protocol-types-040-041-093/claude-subagent-pre-review.md`
- `docs/releases/v0.2.0/.reviews/provider-formatting-042-043-044/claude-subagent-pre-review.md`
- `docs/releases/v0.2.0/.reviews/storage-045-091-096/claude-subagent-pre-review.md`

### Security reviews
- `docs/history/2026-04-16-security-wu-039-protocol-core.md`

### Session logs
- `docs/history/2026-04-16-session-v0.2.0-kickoff-and-wu-039.md`
- `docs/history/2026-04-16-release-v0.2.0-test-coverage-gap-review.md`
- `docs/history/2026-04-16-design-review-workflow.md`
- `docs/history/2026-04-16-wu-039-peer-review-codex.md` (user-authored)
- `docs/history/2026-04-16-session-phase1-track0-and-workflow.md` (this file)

### Implementation (WU-039 only)
- `internal/protocol/protocol.go`
- `internal/protocol/messages.go`
- `internal/protocol/protocol_test.go`

### Process files modified
- `docs/agents.md` (multiple revisions; final has prime directives)
- `CLAUDE.md` (release execution section)
- `AGENTS.md` (convention #5 and #6)
- `docs/releases/v0.2.0/plan.md` (phased execution, test tiers, review tiers)
- `docs/releases/v0.2.0/status.md` (phase tracking)
- `docs/releases/v0.2.0/track-0-shared.md` (tier tags, WU-040 envelope decision, WU-093/096 specs)
- `docs/releases/v0.2.0/track-a-bff-server.md` (tier tags, WU-046 inherited criteria)
- `docs/releases/v0.2.0/track-b-terminal-harness.md` (tier tags)
- `docs/releases/v0.2.0/track-integration.md` (tier tags, WU-094/095 specs)

## What's next

**Phase 1 continuation — remaining design artifacts:**
- Track A: 5 bundles + 7 standalones = 12 artifacts
- Track B: 4 bundles + 6 standalones = 10 artifacts
- Integration: 1 bundle + 2 standalones = 3 artifacts
- Total remaining: 25 artifacts (of 28 total)

**Recommended: fresh session.** Current context is saturated. Track A designs will benefit from a clean window.

**Resumption protocol:**
1. Read `docs/releases/v0.2.0/status.md` — confirms Phase 1, Track 0 done.
2. Read `docs/agents.md` §"Prime directives" — confirms Phase 1 = design only, no code.
3. Read `docs/releases/v0.2.0/plan.md` §"Design Review Tiers" and the Track A bundle list there — identifies the next bundles.
4. Pick next Track A bundle and design it.
