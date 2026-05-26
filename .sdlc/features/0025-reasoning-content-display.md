---
feature: FEAT-0025
title: Reasoning content streaming, persistence, and display
status: draft
date: 2026-05-25
related:
  - FEAT-0008 (Runtime Server) — protocol carries StreamEvent and TurnComplete
  - FEAT-0009 (Terminal Harness) — viewport renders model output
  - FEAT-0015 (Professional Harness Runtime) — `tool_loop` stage motivates inline reasoning
  - FEAT-0016 (Managed Codegen Run Pipeline) — multi-turn dispatch where reasoning is most visible
  - PATCH-0008 (Moonshot Provider Adapter) — first place reasoning_content was parsed and explicitly dropped
  - EXP-0011 (Harness Excellence Gap Analysis) — UX framing for "show the work"
adr-constraints:
  - ADR-0006: Provider adapter interface is the single seam for provider-shaped reasoning payloads
  - ADR-0005: Capture is always full; reasoning belongs in the captured request/response artifact
promoted-from:
  - PATCH-0008: §"Out of Scope" — reasoning/thinking display was deferred from PATCH-0008 pending a feature spec
---

# FEAT-0025: Reasoning content streaming, persistence, and display

## Problem

Modern providers emit **reasoning** content alongside their final response: Anthropic `thinking` blocks (extended thinking, summarized thinking), OpenAI `reasoning_summary` events for the o-series, Moonshot/Kimi `reasoning_content` SSE deltas, DeepSeek-R1, Ollama reasoning models, and similar shapes from other vendors.

Today modeltap silently discards all of it:

| Layer | Current state | Source |
|---|---|---|
| Provider parse | Moonshot's delta struct has a `ReasoningContent string` field, but `ParseStreamEvent` does not emit it. Anthropic `thinking` blocks and OpenAI reasoning summaries are not parsed at all. | `.sdlc/patches/0008-moonshot-provider-adapter.md:231`; `internal/provider/moonshot.go` |
| Protocol | `StreamEvent` has no reasoning variant. `TurnComplete` has no reasoning field. | `internal/protocol/` |
| Runtime relay | `StreamRelay` only accumulates text, tool calls, and usage. | `.sdlc/patches/0008-moonshot-provider-adapter.md:233` |
| Harness render | `StreamCompleteMsg` has no reasoning field; the viewport has no component for reasoning. | Same source |
| Persistence | `turns` table has no reasoning column; the run-event log has no reasoning event. | `internal/storage/` |

PATCH-0008 explicitly punted on this five-layer gap (`.sdlc/patches/0008-moonshot-provider-adapter.md:69, 72`); the F5 review on that patch (`.sdlc/patches/.reviews/0008-moonshot-provider-adapter-findings.md:63`) flagged it as significant and asked for either a full pipeline design or an explicit punt with a follow-up vehicle. This feature is that follow-up.

The gap matters most in the `tool_loop` stage (FEAT-0015 §"Tool Runtime Integration"). A run that issues five tool calls, waits for results, and emits a final answer currently looks **mute between tool calls**: the user sees `⚙ Bash → ✓ Bash → ⚙ Grep → ✓ Grep → …` and then a final reply, with no view into the model's plan or its interpretation of intermediate results. Comparable harnesses (OpenCode, Claude Code, Cursor) all surface this content; without it modeltap's harness will feel opaque on exactly the workflows FEAT-0015 and FEAT-0016 are designed for.

## Personas

- **Tool-loop operator** (primary). Runs multi-turn pipelines where the model alternates between tool calls and analysis. Needs to see the model's plan before each tool call and its read of each result, otherwise has no basis for trusting or interrupting the run.
- **Reasoning-model user**. Selects an o-series, Anthropic extended-thinking, Kimi-K2-thinking, or DeepSeek-R1 endpoint specifically *because* of the reasoning capability. Paying for reasoning tokens; expects to see what they paid for.
- **Debugger / reviewer**. Investigating why a run produced a surprising tool call, an unexpected file write, or a wrong answer. Reasoning content is often the only artifact that explains intent.
- **Capture/replay consumer** (ADR-0005). Reviews captured request/response pairs after the fact. Reasoning must be in the capture or the capture lies.

## Stories

1. **Stream reasoning inline as it arrives.** When the active model emits reasoning content, the harness shows it in the viewport as a labeled, visually distinct block above the response (the OpenCode pattern: `Thinking:` header, dim/italic body, no input affordance). The block streams progressively, just like response text.
2. **Distinguish reasoning from response.** Reasoning and final response use different rendering treatments. A reasoning block cannot be confused with a final assistant message, copy-pasted as one, or replied to.
3. **Reasoning appears between tool turns.** During a `tool_loop`, the model's reasoning between each tool call and tool result is visible in the transcript at the right point in time, not collapsed into the final answer.
4. **Per-session reasoning visibility toggle.** A user who finds reasoning distracting can hide it (`/reasoning off`) for the current session; the underlying capture and persistence are unaffected.
5. **Captured runs include reasoning.** A run replayed from storage shows the same reasoning blocks at the same positions as the live run did. Capture artifacts (ADR-0005) include reasoning verbatim.
6. **Cost accounting includes reasoning tokens.** Reasoning tokens are counted in usage events and cost rollups, separately tagged where the provider reports them separately (Anthropic `thinking_tokens`, OpenAI `reasoning_tokens`).
7. **Providers without reasoning content behave identically to today.** OpenAI gpt-4o, Anthropic without extended thinking, Ollama base models: no behavioral change, no empty headers.

## Solution

A single reasoning pipeline spans all five layers. Each layer has a small, well-bounded change:

1. **Provider parse** — extend `provider.StreamEvent` with a `Reasoning` variant. Each adapter parses its provider-specific shape into that canonical event. Discardable behind a provider capability flag for providers that never emit it.
2. **Protocol event** — add `notifications/stream` event type `reasoning_delta` carrying `{run_id, turn_id, model_call_id, sequence, text}` plus a `reasoning_done` terminator with summary metadata. Mirror the existing text-streaming shape so the relay and harness changes are minimal.
3. **Runtime relay** — `StreamRelay` accumulates a per-call reasoning buffer in addition to text/tools/usage. Emits `reasoning_delta` to attached clients in arrival order. Records reasoning tokens in usage metadata.
4. **Harness render** — new `ReasoningBlockMsg` and a viewport component that renders blocks with a header (`reasoning`), a dim style, and a stable identity keyed by `(turn_id, model_call_id)`. Block follows the existing transcript event-row pattern from PATCH-0042 so tool-loop runs interleave correctly.
5. **Persistence** — new `turn_reasoning` table or `reasoning` JSONB column on `turns`, accessed via store methods. Capture artifacts (ADR-0005) include reasoning in the raw response body, which is already preserved, and additionally in the structured per-turn record.

The toggle (`/reasoning on|off|status`) is harness-local UI state. Capture and persistence are unconditional; the toggle only controls rendering.

## Architecture decisions to make

### A. Canonical event shape

- `reasoning_delta` mirrors the existing text delta shape with one additional field: `kind`, one of `summary` (Anthropic summarized thinking, OpenAI reasoning_summary) or `raw` (Moonshot reasoning_content, Anthropic non-summarized extended thinking, DeepSeek R1).
- The harness MAY render the two kinds differently but the protocol does not require it.

### B. Block boundaries and identity

- Within one `model_call_id`, all reasoning is one logical block. Identity key: `(turn_id, model_call_id)`. The harness can detect "new block" by `model_call_id` transition, which is exactly the boundary the tool loop creates.
- Block ordering relative to text deltas and tool-call deltas is the order the runtime relay sees the provider emit them. The harness does not reorder.

### C. Anthropic-specific: signed thinking and tool-use interleaving

- Anthropic's extended thinking signs `thinking` blocks for replay safety. The provider adapter MUST preserve the signature in the captured artifact and MAY include it in a `metadata` field on `reasoning_done`. The harness does not need to render the signature.
- Anthropic interleaves `thinking` and `tool_use` blocks across a single response. The relay surfaces them in order; the harness renders them at the matching transcript positions.

### D. OpenAI-specific: reasoning summaries

- The OpenAI Responses API emits compact reasoning summaries, not raw chain-of-thought. Map to `kind=summary`. Do not synthesize raw reasoning where the provider only gave a summary.

### E. Privacy and exfiltration

- Reasoning content can contain sensitive context from prompts and tool results.
- Default visibility: ON for `solo` mode (single-user local), OFF for any future shared/multi-user mode (gated by FEAT-0010 enterprise auth when that lands).
- Capture artifacts always include reasoning, but logs and stdout MUST NOT print reasoning at default log levels. Add a dedicated `reasoning` log category, default off.

### F. Replay and retry

- Re-running a turn (FEAT-0017 retry) emits a fresh `model_call_id` and a fresh reasoning block. The prior reasoning is retained against its original `model_call_id` for replay/inspection.

## Work units (proposed)

- **WU-NNN-A: provider StreamEvent reasoning variant + Moonshot wire-up.** Add `StreamEventReasoning` to `provider.StreamEvent` with `Kind` (raw/summary). Moonshot adapter emits it from the already-parsed `reasoning_content` field. Anthropic, OpenAI, and Ollama adapters added as separate WUs.
- **WU-NNN-B: protocol reasoning_delta + reasoning_done events.** Extend `internal/protocol/` with the new notifications. Update fixtures. Conformance tests assert ordering relative to text and tool deltas.
- **WU-NNN-C: runtime relay reasoning accumulation and emission.** `StreamRelay` consumes provider reasoning events, accumulates per-call buffer, emits protocol events to attached clients, records reasoning tokens in usage.
- **WU-NNN-D: harness reasoning component.** New `ReasoningBlockMsg`, new component in the conversation shell that renders a `reasoning` header + dim body with stable identity keyed by `(turn_id, model_call_id)`. Follows the PATCH-0042 transcript-row pattern.
- **WU-NNN-E: persistence.** Add reasoning storage (table or column). Store methods. Migration. Backfill not required (pre-feature turns have no reasoning to backfill).
- **WU-NNN-F: per-provider parsers.** Anthropic extended-thinking blocks (raw + summarized variants, signature preservation). OpenAI Responses-API reasoning summaries. Ollama reasoning-model deltas where applicable.
- **WU-NNN-G: harness toggle and config.** `/reasoning on|off|status`. Session-scoped state. Optional global default in `~/.config/modeltap/config.yaml`.
- **WU-NNN-H: cost accounting.** Surface reasoning tokens in `cost.update` events and in the cost dashboard. Tag separately where provider distinguishes.
- **WU-NNN-I: tests and docs.** Conformance fixtures for each provider, integration test exercising a tool-loop run with reasoning interleaved, user-facing doc in `docs/` covering the rendering behavior and the toggle.

## Configuration

```yaml
harness:
  reasoning:
    # Default render visibility. Capture and persistence are always on.
    render: visible      # visible | hidden
    # Style hint for the renderer. Implementation-defined; "dim" matches
    # OpenCode/Claude Code.
    style: dim
```

```text
# Slash commands
/reasoning              # show current state
/reasoning on           # render reasoning blocks in this session
/reasoning off          # suppress rendering in this session
```

## Success criteria

- [ ] Running a Moonshot/Kimi `kimi-k2-thinking` turn produces a reasoning block in the viewport that streams progressively above the final response.
- [ ] Running an Anthropic extended-thinking turn produces reasoning blocks at the correct positions relative to `tool_use` blocks within the same response.
- [ ] Running an OpenAI o-series turn produces a reasoning-summary block tagged `kind=summary`.
- [ ] A `tool_loop` run with three tool calls shows three distinct reasoning blocks at the three pre-tool-call positions, not concatenated into the final reply.
- [ ] `/reasoning off` hides reasoning rendering for the active session without affecting capture, persistence, or token accounting.
- [ ] Replaying a captured run from storage reproduces the same reasoning blocks at the same positions.
- [ ] `cost.update` events surface reasoning tokens distinctly where the provider reports them distinctly.
- [ ] Providers that emit no reasoning (gpt-4o-mini, base Ollama models) behave identically to pre-feature behavior — no empty headers, no transcript artifacts.
- [ ] All conformance fixtures and unit tests pass; integration test exercises a multi-provider tool-loop run with reasoning interleaved.

## Non-Goals

- **Editable reasoning.** Reasoning blocks are read-only; the harness will not let the user edit a streamed block before it is sent back to the model (per OpenCode pattern).
- **Reasoning-aware routing.** Choosing models based on reasoning capability is a routing-policy concern (FEAT-0016/FEAT-0022 territory), not this feature.
- **Cross-provider reasoning normalization beyond `kind`.** This feature does not attempt to translate Anthropic thinking into OpenAI-summary form or vice versa. The protocol shape is canonical; the body is opaque per provider.
- **Reasoning in MCP tool results.** MCP servers may produce content that resembles reasoning; this feature does not treat it as such. MCP results stay in the tool-result channel.
- **Mid-stream redaction.** No automatic PII scrubbing on reasoning content in v1. Privacy posture is "capture as-is, hide from shared surfaces by default" (see §"Privacy and exfiltration").
- **Restoring or rendering reasoning from pre-feature captures.** Pre-feature captures do not contain structured reasoning; they are not retroactively augmented.

## Relationship to ADRs

- **ADR-0005 (always-full capture).** Reasoning content is part of the full response and MUST be captured verbatim. This feature does not weaken capture — it adds a structured access path.
- **ADR-0006 (provider adapter interface).** Reasoning shapes are provider-specific; the adapter is the right and only seam. Adding `StreamEventReasoning` is an interface extension, similar in spirit to how `FormatToolDefinitions` was added in WU-042 (see release v0.2.0 changelog).
- **ADR-0011 (BDFL governance).** No governance impact.

## Sequencing

This feature is **not v0.3.0**. Target slot: **v0.3.x**, ideally before FEAT-0017's `tool_loop` user-visible work matures (since reasoning rendering compounds with tool-loop UX). Concrete proposal:

- **v0.3.1 or v0.3.2** if the v0.3.0 close-out completes cleanly. v0.3.1 is currently scoped to FEAT-0018 (context planner) which is independent of this feature; v0.3.2 covers FEAT-0019 (validation) and FEAT-0020 (artifacts) which both benefit from reasoning being visible.
- If v0.3.1 is the slot, WU-A/B/C (provider+protocol+relay) are the minimum to land; WU-D/E (harness+persistence) can ship in the same release or one later.
- A precondition: the foreground tool loop must actually loop (see the open issue in `internal/runtime/turn.go:316-362` and `internal/runtime/dispatch.go:79-87` — `Tools` is dropped on the floor before `FormatMessages` and `handleToolResult` does not re-dispatch). This is independent prerequisite work, likely a PATCH against v0.3.x.

## Open Questions

1. **One reasoning block per `model_call_id`, or per provider-emitted block?** Anthropic can emit multiple `thinking` blocks interleaved with `tool_use`; rendering each as a distinct block matches the provider's intent but adds harness complexity. Recommendation: per provider-emitted block, with `(turn_id, model_call_id, block_index)` identity.
2. **Reasoning in dashboard?** The web dashboard (FEAT-0003) currently shows requests/responses; should it render reasoning distinctly, or rely on the raw response body view? Recommendation: dedicated reasoning section in the turn detail view, deferred to a follow-up WU.
3. **Reasoning in `modeltap requests`?** The CLI `requests` command emits per-request summaries. Adding reasoning summary is cheap; deciding default verbosity is the open question.
4. **Streaming backpressure.** Reasoning blocks can be long (extended thinking on hard problems). Do we cap the rendered length and offer expansion, or render in full? Recommendation: render in full, defer collapse to a follow-up if it becomes a problem.
5. **MCP tool-result content shape.** If a future MCP tool returns content tagged as reasoning, do we promote it to the reasoning channel or keep it in the tool-result channel? Recommendation: keep in tool-result; reasoning is model-emitted, not tool-emitted.
