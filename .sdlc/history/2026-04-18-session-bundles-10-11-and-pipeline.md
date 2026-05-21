# 2026-04-18 — Session: Bundles 10 + 11, full turn.submit pipeline

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Topic

Continuation of "continue until complete" with the user's explicit
approval to extend the `provider.Provider` interface with
`ParseStreamEvent`. That approval unblocked all of Bundle 10 (streaming
relay + prompt engine + cost tracking), the `handleTurnSubmit`
orchestration that ties Bundles 4/8/9/10 together end-to-end, and
several adjacent handlers (sync recovery, content transform, command
history). Also added the Ollama provider adapter so the BFF supports
all three first-party providers (Anthropic, OpenAI, Ollama).

The BFF server side is now substantially complete for single-model
turns and could be exercised end-to-end against a real provider.

## Work Completed (in commit order)

### Provider interface amendment

`e70e9d9` — `WU-042 amend: ParseStreamEvent on Provider interface`.
New `ParseStreamEvent(data []byte) (*StreamEvent, error)` on the
`Provider` interface plus `StreamEvent` / `StreamEventType` /
`StreamToolCall` / `StreamUsage` types. Implemented for Anthropic
(content_block_delta, content_block_start tool_use, message_delta
usage, message_stop) and OpenAI (choices.delta.content,
choices.delta.tool_calls start/delta, [DONE] sentinel, finish_reason).
All in-tree mocks updated.

### Bundle 10 — Streaming, Prompts, Cost

- `bcfbd57` — `WU-053: streaming relay`. `SSEParser` (data: line
  framing, event-header skip, EOF semantics) + `StreamRelay` driving
  `adapter.ParseStreamEvent` to emit `token.delta` / `tool.call` /
  `turn.complete` notifications. Tool calls accumulated across
  start/delta/end events. Partial-persist on stream error or cancel.
- `a25454d` — `WU-056: cost tracking + cost.update events`.
  `CostTracker.ComputeTurnCost` from registry pricing + token counts,
  `UpdateAfterTurn` accumulates session totals + persists + emits
  `cost.update`.
- `d597207` — `WU-054 + WU-055: PromptEngine 7-layer assembly`. All
  seven FEAT-0008 layers (core behavioral, tool-use, domain, project,
  mode, knowledge stub, session state) with budget-aware trim that
  drops only Layers 6-7 (1-5 are pinned per FEAT-0008 contract).

### handleTurnSubmit orchestration

`ea9d115` — `WU-052/053 wire-up: handleTurnSubmit + turn.cancel +
tool.result`. Central handler that ties everything together:
validate → session ensure/create + lock → user turn append + persist →
command-history append → routing resolve → prompt assemble → dispatch →
stream relay → cost tracker. `turnTracker` maps turn_id → cancel func
so `turn.cancel` aborts in-flight streams. `tool.result` matches the
`tool_call_id` against `Conversation.PendingToolCalls` then stages the
result as a follow-up user-role turn.

Multi-model branching is rejected with a clear error rather than
silently picking the first model — preserves the contract until WU-060
lands.

### Adjacent handlers + adapter

- `3619996` — `WU-091: history.append + history.list handlers`. Scoped
  by user / project / session; opaque "TIMESTAMP|ID" before-cursor
  pagination; HasMore from limit+1 pull.
- `6cb938c` — `WU-064: session.sync recovery handler`. Returns active
  turn id (from `turnTracker`), pending tool calls (from
  `Conversation.PendingToolCalls`), status discrimination (streaming /
  pending_tool_result / complete). `TokenReplayAvailable=false` for
  v1; future enhancement.
- `199cd24` — `WU-063: diagnostic taxonomy helpers`.
  `NewDiagnosticError`, `WithSuggestedCommand`, `DiagnosticOf` — pulls
  the open-coded `protocol.Diagnostic` construction out of the
  handlers.
- `924835e` — `WU-062: content.transform handler`. Server-side
  summarization via cheap-model routing. Best-effort response text
  extraction handles both Anthropic (`content[].text`) and OpenAI
  (`choices[0].message.content`) shapes.
- `fedad7b` — `WU-066: Ollama provider adapter`. Full Provider
  interface for /api/chat: NDJSON streaming, FormatMessages with tool
  results flattened (Ollama lacks a tool role), Ollama-specific token
  counts (prompt_eval_count + eval_count).

## Bundle Progress After This Session

| Bundle | Status | Note |
|---|---|---|
| 4 (BFF Foundation) | 4/4 | complete |
| 8 (Sessions & Conversation) | 3/3 | complete |
| 9 (Model Config & Routing) | 3/4 | WU-060 multi-model deferred |
| 10 (Streaming, Prompts, Cost) | 4/4 | complete |
| 11 (Context, Diagnostics, Recovery) | 3/4 | WU-061 compaction deferred |

Plus auxiliary: WU-091 command history handlers, WU-066 Ollama, the
`handleTurnSubmit` pipeline that spans the bundles.

## Files Created or Modified

Created (new files):
- `internal/provider/ollama.go`, `ollama_test.go`
- `internal/provider/stream_event_test.go`
- `internal/bff/streaming.go`, `streaming_test.go`
- `internal/bff/prompt.go`, `prompt_test.go`
- `internal/bff/cost.go`, `cost_test.go`
- `internal/bff/turn.go`, `turn_test.go`
- `internal/bff/history.go`, `history_test.go`
- `internal/bff/sync.go`, `sync_test.go`
- `internal/bff/diagnostics.go`, `diagnostics_test.go`
- `internal/bff/transform.go`, `transform_test.go`
- `.sdlc/history/2026-04-18-session-bundles-10-11-and-pipeline.md`

Modified:
- `internal/provider/provider.go` — added ParseStreamEvent + types
- `internal/provider/anthropic.go` — implemented ParseStreamEvent
- `internal/provider/openai.go` — implemented ParseStreamEvent
- `internal/provider/registry_test.go` — mock satisfies new method
- `internal/cli/status_test.go` — mock satisfies new method
- `internal/bff/dispatch_test.go` — stub satisfies new method
- `internal/bff/server.go` — wired adapters/prompts/dispatch/cost/turns
  fields and registered all new handlers
- `.sdlc/releases/v0.2.0/status.md`

## Notes / Decisions

- **Multi-model**: handleTurnSubmit detects multi-model routing
  resolution and returns a structured error rather than silently
  fanning out to one model. This keeps the contract honest until
  WU-060 lands and removes a footgun where users could think their
  multi-model config "worked" but only the first model ran.
- **Cost accuracy on transform**: `content.transform` uses estimated
  token counts (chars/4) for cost computation since the non-streaming
  response parser is best-effort and doesn't extract real token usage.
  Real tokens come from the next `turn.submit` that consumes the
  transform's output.
- **Token replay buffer**: `session.sync` reports
  `TokenReplayAvailable=false`. A future enhancement could buffer the
  last N token.delta payloads so a reconnecting harness can re-render
  the in-flight turn without restarting from the beginning.
- **Compaction deferred**: WU-061 touches both server-side trimming
  heuristics and the harness compaction approval UX
  (`compact.suggest` / `compact.apply`). Implementing it without the
  matching Track B UI would land server code with no end-to-end test.
- **Pre-existing flake**: `TestMetricsAggregation` in
  `internal/integration` continues to fail under `-race` due to a
  SQLite_BUSY race that predates this session. Tracked as a
  separate concern.

## Next / Open Items

When you return:

1. **WU-060 multi-model branching** — design review needed on
   per-branch event tagging contract (especially how `branch.error`
   interacts with `turn.complete` aggregation).
2. **WU-061 compaction** — design review for the trim heuristic +
   harness coordination (what does `compact.apply` actually do
   server-side vs. just notify).
3. **WU-065 CLI integration** — wire `modeltap serve` to start the
   BFF alongside the proxy. Provider/model config loaded from Viper.
   Small WU; mostly plumbing.
4. **WU-067 BFF integration tests** — end-to-end harness driving the
   BFF over its socket. Useful as a guard before connecting Track B
   to it.
5. **Track B** — FEAT-0009 terminal harness. Bundle 5 (Bubbletea
   scaffold, WU-068–072) is parallelizable and a clean entry point.
   Different domain from the BFF work; benefits from its own focused
   session.
