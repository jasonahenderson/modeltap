# v0.2.0 Status

## Last Updated
2026-04-18

## Current Phase
**Phase 3 — Implementation.** Phase 1 (design) and Phase 2 (review + cross-flow audit) complete. All 22 blocking, 28 attention, 2 critical, and 11 important findings resolved across pre-review lints and cross-flow audit.

**Phase 1 progress:**
- [x] Track 0: 3/3 bundles designed (WU-040–045, 093, 096) — Bundles 1-3
- [x] Track A: 7/7 bundles designed (WU-046–067, 091) — Bundles 4, 8, 9, 10, 11, 12, 14
- [x] Track B: 5/5 bundles designed (WU-068–087, 092) — Bundles 5, 6, 7, 13, 14
- [x] Integration: 1/1 bundle designed (WU-088–090, 094, 095) — Bundle 15

See `plan.md` §"Phase 1 Completion Checklist" for the full checklist.

## Planned
See `plan.md`, `track-0-shared.md`, `track-a-bff-server.md`, `track-b-terminal-harness.md`, `track-integration.md`.

58 work units total: WU-039 through WU-096.

## Completed
- [x] WU-039: Protocol types — core messages and framing (2026-04-16)
- [x] WU-040+041: Protocol streaming events and response types (2026-04-17) — commit `206a720`
- [x] WU-042: Provider outbound formatting interface (2026-04-17) — commit `546bcea`
- [x] WU-045: Session and turn storage schema, migration v2 (2026-04-17) — commit `20eb095`
- [x] WU-043: Anthropic outbound formatting (2026-04-17) — commit `c29f904`
- [x] WU-044: OpenAI outbound formatting (2026-04-17) — commit `ccdef69`
- [x] WU-093: Protocol contract golden fixtures + conformance tests (2026-04-17) — commit `3068586`
- [x] WU-096: Storage migration v1→v2 upgrade tests (2026-04-17) — commit `45fdd4a`
- [x] WU-041 follow-on: MT-CONN-013 attachment-too-large diagnostic (2026-04-17) — commit `e88a6d4`
- [x] WU-046: JSON-RPC transport layer for BFF (2026-04-17) — commit `9c39877`
- [x] WU-048: Connection lifecycle state machine (2026-04-17) — commit `3242ce8`
- [x] WU-045 follow-on: Store.Ping for BFF health handler (2026-04-18) — commit `2d40469`
- [x] WU-047: BFF server listeners, accept loop, health/ready handlers (2026-04-18) — commit `30fe4bf`
- [x] WU-049: Capability registration, version negotiation, project context (2026-04-18) — commit `01e8169`
- [x] PATCH: statusMockProvider implements new Provider interface methods (2026-04-18) — commit `311c33c`
- [x] WU-051: Conversation canonical-format persistence (2026-04-18) — commit `fb8f320`
- [x] WU-050: Session management (resume/list/details/clear/fork) (2026-04-18) — commit `f5b5628`
- [x] WU-057: Provider endpoints registry and health checks (2026-04-18) — commit `03dd6dc`
- [x] WU-058: Model registry (2026-04-18) — commit `359dca8`
- [x] WU-059: Routing policy + model.list/switch handlers (2026-04-18) — commit `ee1e903`
- [x] WU-052: TurnDispatcher provider format translation (2026-04-18) — commit `4738539`
- [x] WU-042 amend: ParseStreamEvent on Provider interface (2026-04-18) — commit `e70e9d9`
- [x] WU-053: Streaming relay (SSE → protocol notifications) (2026-04-18) — commit `bcfbd57`
- [x] WU-056: Cost tracking + cost.update events (2026-04-18) — commit `a25454d`
- [x] WU-054 + WU-055: PromptEngine 7-layer assembly (2026-04-18) — commit `d597207`
- [x] WU-052/053 wire-up: handleTurnSubmit + turn.cancel + tool.result (2026-04-18) — commit `ea9d115`
- [x] WU-091: history.append + history.list handlers (2026-04-18) — commit `3619996`
- [x] WU-064: session.sync recovery handler (2026-04-18) — commit `6cb938c`
- [x] WU-063: diagnostic taxonomy helpers (2026-04-18) — commit `199cd24`
- [x] WU-062: content.transform handler (2026-04-18) — commit `924835e`
- [x] WU-066: Ollama provider adapter (2026-04-18) — commit `fedad7b`
- [x] WU-065: Wire BFF server into modeltap start (2026-04-18) — commit `3d58416`

### Bundle 4 (BFF Foundation) complete
All four WUs in `internal/bff/` landed race-clean. WU-046 transport, WU-048
connection state machine, WU-047 server + listeners + health/ready handlers,
WU-049 capabilities manager + register/update handlers.

### Bundle 8 (Sessions & Conversation) complete
All three WUs landed. WU-050 sessions, WU-051 Conversation, WU-052 dispatch.

### Bundle 9 (Model Config & Routing): 3 of 4 WUs complete
WU-057 providers, WU-058 registry, WU-059 routing landed. WU-060
multi-model branching deferred — handleTurnSubmit currently rejects
multi-model routing with a clear error rather than picking the first
model silently.

### Bundle 10 (Streaming, Prompts, Cost) complete
WU-053 streaming relay, WU-054+055 prompt engine (7-layer assembly with
trim policy), WU-056 cost tracking with cost.update events. The
Provider interface gained ParseStreamEvent (WU-042 amend, commit
e70e9d9) — implemented for Anthropic, OpenAI, and now Ollama.

### Bundle 11 (Context, Diagnostics, Recovery): 3 of 4 WUs complete
WU-062 content.transform, WU-063 diagnostic taxonomy helpers, WU-064
session.sync recovery handler. WU-061 compaction is deferred — its
design touches Conversation truncation + the harness compaction UX,
warranting an explicit design pass before implementation.

### handleTurnSubmit pipeline complete
turn.submit now end-to-end: validate → session ensure/create → user
turn append + persist → command-history append → routing resolve →
prompt assemble → dispatch → stream relay → cost tracker. turn.cancel
and tool.result handlers join the dispatcher. The BFF is functionally
useful end-to-end for single-model turns against any registered
provider.

### Provider adapters
Three first-party adapters implement the full Provider interface:
Anthropic (`internal/provider/anthropic.go`), OpenAI
(`internal/provider/openai.go`), Ollama (`internal/provider/ollama.go`).

### Phase 1 Design Artifacts (Track 0)
- [x] Bundle 1 — Protocol types (WU-040 + 041 + 093) — design `designs/2026-04-16-design-protocol-types-040-041-093.md`; pre-review `docs/releases/v0.2.0/.reviews/protocol-types-040-041-093/`. Commit `f9429e4`.
- [x] Bundle 2 — Provider formatting (WU-042 + 043 + 044) — design `designs/2026-04-16-design-provider-formatting-042-043-044.md`; pre-review `docs/releases/v0.2.0/.reviews/provider-formatting-042-043-044/`. Commit `3fb9588`.
- [x] Bundle 3 — Storage (WU-045 + 091 + 096) — design `designs/2026-04-16-design-storage-045-091-096.md`; pre-review `docs/releases/v0.2.0/.reviews/storage-045-091-096/`. Commit `99c724e`.

### Phase 1 Design Artifacts (Tracks A, B, Integration)
- [x] Bundle 4 — BFF Foundation (WU-046-049) — design `designs/2026-04-16-design-bff-foundation-046-047-048-049.md`; pre-review + fixes. Commits `f636403`, `4baa5fc`.
- [x] Bundle 5 — Bubbletea Scaffold (WU-068-072) — design `designs/2026-04-16-design-bubbletea-scaffold-068-069-070-071-072.md`; pre-review + fixes. Commit `f636403`.
- [x] Bundle 6 — Protocol Client (WU-073-074) — design `designs/2026-04-16-design-protocol-client-073-074.md`; pre-review + fixes. Commit `f636403`.
- [x] Bundle 7 — Tool Framework + Tools (WU-075-079) — design `designs/2026-04-16-design-tool-framework-075-076-077-078-079.md`; pre-review + fixes. Commits `f636403`, `2fc6e30`.
- [x] Bundle 8 — Sessions & Conversation (WU-050-052) — design `designs/2026-04-16-design-sessions-conversation-050-051-052.md`; pre-review + fixes. Commits `2fd809b`, `4baa5fc`.
- [x] Bundle 9 — Model Config & Routing (WU-057-060) — design `designs/2026-04-16-design-model-config-routing-057-058-059-060.md`; pre-review + fixes. Commits `2fd809b`, `4baa5fc`.
- [x] Bundle 10 — Streaming, Prompts, Cost (WU-053-056) — design `designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md`; pre-review + fixes. Commits `2fd809b`, `2fc6e30`.
- [x] Bundle 11 — Context, Diagnostics, Recovery (WU-061-064) — design `designs/2026-04-16-design-context-diagnostics-recovery-061-062-063-064.md`; pre-review. Commit `2fd809b`.
- [x] Bundle 12 — CLI, Ollama, Command History (WU-065, 066, 091) — design `designs/2026-04-16-design-cli-ollama-history-065-066-091.md`; pre-review. Commit `2fd809b`.
- [x] Bundle 13 — Harness Features (WU-080-086, 092) — design `designs/2026-04-16-design-harness-features-080-086-092.md`; pre-review + fixes. Commits `2fd809b`, `2fc6e30`.
- [x] Bundle 14 — Track Integration Tests (WU-067, 087) — design `designs/2026-04-16-design-track-integration-tests-067-087.md`; pre-review. Commit `2fd809b`.
- [x] Bundle 15 — Integration Track (WU-088-090, 094, 095) — design `designs/2026-04-16-design-integration-track-088-090-094-095.md`; pre-review. Commit `2fd809b`.

### Pre-Review Blocking Findings (all resolved)
- Bundles 4-6: 10 blockers (heartbeat direction, CapabilitiesRegisterResponse type, ConnectionPong fields, capabilities.request, ConnState strings, Mode type, connection states, mode rendering, degradation threshold, Notify removal)
- Bundles 8-9: 6 blockers (turn serialization format, session.resume response type, session.sync export, config map format, routing resolution depth)
- Bundles 7/10/13: 6 blockers (RiskLevel conversion, ToolCallID flow, history types, TokenDelta field name, prompt trimming order, context.list RPC)

## In Progress
(none)

## Up Next

The BFF server side is now substantially complete for single-model
turns. Remaining Track A work:

- **WU-060** multi-model branching — needs explicit design review for
  the per-branch event tagging + aggregate scoring contract.
- **WU-061** compaction (`session.compact` / `compact.apply`) — needs
  design review for the trim heuristic + harness UX coordination.
- **WU-065** CLI integration — wire `modeltap serve` to start the BFF
  alongside the proxy.
- **WU-067** BFF integration tests — end-to-end test harness driving
  the BFF over its socket.

Track B (FEAT-0009 terminal harness) is not started — Bundle 5 (WU-068–072
Bubbletea scaffold) is parallelizable and depends only on WU-039 (done).
Track B is a different domain (Bubbletea, terminal rendering, file
context UX) and warrants a dedicated session.

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
