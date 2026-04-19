# v0.2.0 Status

## Last Updated
2026-04-19 (WU-067 + WU-087 + WU-088 integration tests + WU-090 changelog landed)

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
- [x] WU-068: Bubbletea app scaffold (2026-04-18) — commit `ce633c9`
- [x] WU-069: Status bar component (2026-04-18) — commit `9aec280`
- [x] WU-070: Input area component (2026-04-18) — commit `aab2aab`
- [x] WU-072: Streaming markdown rendering (2026-04-18) — commit `7392d5f`
- [x] WU-071: Conversation viewport component (2026-04-18) — commit `966e80a`
- [x] WU-073: Harness JSON-RPC protocol client (2026-04-18) — commit `0a7a1db`
- [x] WU-074: Harness connection manager (2026-04-18) — commit `9e2a97b`
- [x] WU-075: Tool framework + permission model (2026-04-18) — commit `0ed8846`
- [x] WU-077: Write and Edit tools (2026-04-19) — commit `797f278`
- [x] WU-078: Bash and Git tools (2026-04-19) — commit `96ec3b7`
- [x] WU-079: Glob, Grep, WebSearch, WebFetch tools (2026-04-19) — commit `a92bba2`
- [x] WU-076: Read tool (text, CSV, image, PDF, DOCX, XLSX) (2026-04-19) — commit `efc2cee`
- [x] WU-086: Connection UX banner translator (2026-04-19) — commit `eee3b9e`
- [x] WU-083: Large paste handler (summarize/full/truncate/cancel) (2026-04-19) — commit `818a360`
- [x] PATCH-0003: App ↔ ConnectionManager wiring (2026-04-19) — commit `0afa222`
- [x] WU-092: BFF-sourced command history traversal (2026-04-19) — commit `9d7c3ca`
- [x] WU-085: /model and /models commands (2026-04-19) — commit `e880992`
- [x] WU-084: /sessions and /session {resume|clear|fork} (2026-04-19) — commit `d359bce`
- [x] WU-082: file context management (@file + /context) (2026-04-19) — commit `c1f8bd1`
- [x] WU-080: Plan/Build/Auto commands + PlanAccumulator (2026-04-19) — commit `ba2f222`
- [x] WU-081: MCP stdio client + manager + tool adapter (2026-04-19) — commit `cbff50f`
- [x] WU-089: modeltap harness CLI command (2026-04-19) — commit `d6bf0e0`
- [x] ADMIN: tools.Registry.Deregister + MCP reconnect cleanup (2026-04-19) — commit `a00bdc7`
- [x] WU-067: BFF integration tests + register handshake wiring (2026-04-19) — commit `d381a3c`
- [x] WU-087: harness ConnectionManager integration tests (2026-04-19) — commit `ff1f0f0`
- [x] WU-088: end-to-end harness → BFF → mock provider (2026-04-19) — commit `b6c83e1`
- [x] ADMIN: bare modeltap launches harness (2026-04-19) — commit `2ef1037`
- [x] WU-090: v0.2.0 changelog (2026-04-19) — commit `6a04a4e`

### Bundle 4 (BFF Foundation) complete
All four WUs in `internal/bff/` landed race-clean. WU-046 transport, WU-048
connection state machine, WU-047 server + listeners + health/ready handlers,
WU-049 capabilities manager + register/update handlers.

### Bundle 8 (Sessions & Conversation) complete
All three WUs landed. WU-050 sessions, WU-051 Conversation, WU-052 dispatch.

### Bundle 9 (Model Config & Routing): 3 of 4 WUs complete; WU-060 deferred
WU-057 providers, WU-058 registry, WU-059 routing landed.

**WU-060 multi-model branching is deferred — superseded by
sub-agents (FEAT-0013).** Decision recorded 2026-04-18: parallel
model fan-out, per-branch event tagging, cancellation, and recovery
are structurally identical to the FEAT-0013 sub-agent contract.
Building branch infrastructure in `internal/bff/branch.go` would be
duplicate machinery. Routing-based fan-out
(`coding.review: [opus, gpt-5]`) becomes a sub-agent flow;
reconciliation (picking / synthesizing across N results) is out of
scope for sub-agents themselves and lands as either a synthesizer
agent or a harness-side picker. handleTurnSubmit rejects multi-model
routes with an error pointing at FEAT-0013. See
`track-a-bff-server.md` §"WU-060" for the full rationale.

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

### Bundle 5 (Track B Bubbletea scaffold) complete
All five WUs landed in `internal/harness/` (race-clean):
WU-068 App + AppState + layout + KeyMap + Bubbletea Msg types,
WU-069 status bar (connection indicator / mode / model / context
pressure / cost / call timer), WU-070 textarea-based input area
(submit + command parse + @file extraction + history traversal +
paste detection), WU-072 Glamour markdown wrapper (streaming-
tolerant heal pass + 50ms debounce + per-render width handling),
WU-071 conversation viewport (auto-scroll with snap-back, role-
aware rendering, assistant header + footer with metrics).

Charm dependencies added: bubbletea, bubbles, lipgloss, glamour.

### Bundle 7 (Tools) complete
All five WUs landed. WU-075 framework + permission model, WU-077
Write + Edit, WU-078 Bash + Git, WU-079 Glob + Grep + WebSearch
+ WebFetch, WU-076 Read. BashTool runs via `sh -c` with projectRoot
cwd, context-bound timeout (default 120s, cap 600s), combined
stdout/stderr, and 100KB-trailing-half output truncation. GitTool
uses an in-tool `ClassifyGit` helper; the permission layer
auto-allows reads (status / log / diff / show / …) in every mode,
prompts mutations per the matrix, and routes dangerous forms (push
--force, reset --hard, branch -D, …) through `alwaysPrompt`. WU-079
adds Glob (doublestar/v4, mtime-sorted), Grep (stdlib regexp +
WalkDir with content / files_with_matches / count modes, context
lines, case-insensitive, glob filter, binary skip), WebSearch
(Brave + SerpAPI with injectable base URLs), and WebFetch
(net.IP-based SSRF defense-in-depth, HTML-to-text stripper, 100KB
truncation). WU-076 Read auto-detects format by extension then
magic bytes and dispatches to format readers: text (line-numbered,
offset/limit), CSV (tab-separated table), Image (base64 +
DetectContentType), XLSX (excelize/v2, BSD-3), DOCX (stdlib
archive/zip + encoding/xml — no UniDoc/unioffice dep,
ADR-0010-clean), PDF (ledongthuc/pdf BSD-3 with page-range parser,
10-page threshold forces explicit range, 20-page per-call cap).
Successful reads mark the FileTracker so Edit can mutate without
a separate Read.

### Bundle 6 (Protocol client + Connection manager) complete
WU-073 ProtocolClient: JSON-RPC over Unix socket / TLS with
request/response correlation, notification dispatch, typed helpers
(SubmitTurn, Register, Ping, Health, SessionResume, etc.).
WU-074 ConnectionManager: 9-state lifecycle, optional auto-start
of `modeltap start`, heartbeat with FEAT-0008 two-stage degradation
(3 missed → degraded, 5 missed → reconnecting), exponential
backoff reconnect with jitter, fast disconnect detection via
client.Done(), event bridge that translates server notifications
into Bubbletea messages.

The harness can now connect to the BFF end-to-end. What's missing
to actually drive a turn through to the user is the App↔manager
wiring (consumer of the ConnStateMsg/StreamTokenMsg/etc. messages
the App already handles) and the tool framework (WU-075).

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

Bundles 4, 5, 6, 8, 10 complete; Bundles 9 and 11 partial; the
harness has working connection lifecycle and event bridge. Remaining:

- **Track B Bundle 7** complete (WU-075, 077, 078, 079, 076).
  All 13 built-in tools live under `internal/harness/tools/`.
  Dep choices deviated from the original design for licensing:
  swapped UniDoc/unioffice for stdlib `archive/zip` + `encoding/xml`
  DOCX extraction, and pdfcpu for `ledongthuc/pdf` (BSD-3) text
  extraction. Both are Apache-2.0-compatible per ADR-0010.
- **Track B Bundle 13** complete: all 8 WUs landed 2026-04-19.
  WU-086 connection UX, WU-083 paste handler, WU-092 history
  traversal, WU-085 model commands, WU-084 session commands,
  WU-082 file context management, WU-080 plan-mode commands, and
  WU-081 MCP client. PATCH-0003 wired the App to an injectable
  ConnSurface. Two follow-ups queued: (1) plan-mode tool
  interception needs a harness-side tool executor wired into the
  ConnectionManager's tool.call event bridge; (2) tools.Registry
  doesn't yet support Deregister, so MCP reconnect leaves old
  tools in place (dup-guard keeps this safe but not ideal).
- **WU-061** compaction — server-side, still needs design discussion
  on the trim heuristic + harness UX flow.
- **WU-067** BFF integration tests — end-to-end test harness driving
  the BFF over its socket. The harness ProtocolClient is now the
  natural test driver.
- **WU-087** harness integration tests — end-to-end against a mock
  BFF. Easier now that ConnectionManager + ProtocolClient exist.
- **Integration track** (WU-094, 095, full WU-090 usage guide):
  security review, performance benchmarks, user-led doc sweep.
  WU-067/087/088 integration tests and the changelog portion of
  WU-090 landed 2026-04-19; WU-089 CLI launch landed 2026-04-19.
- **WU-060** multi-model branching: deferred — superseded by FEAT-0013
  sub-agents.

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
