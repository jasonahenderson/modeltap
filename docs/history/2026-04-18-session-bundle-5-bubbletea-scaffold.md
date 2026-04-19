# 2026-04-18 — Session: Bundle 5 (Track B Bubbletea scaffold)

## Topic

Continued the "one at a time" sequence of decision points. Two
decisions landed in this session:

1. **WU-060 deferred** — confirmed multi-model branching is structurally
   identical to FEAT-0013 sub-agents and doesn't deserve duplicate
   machinery. Reconciliation across N parallel results is explicitly
   out of scope for sub-agents themselves and lands separately as
   either a synthesizer agent or a harness-side picker.
2. **Track B Bubbletea scaffold (Bundle 5)** picked over WU-061
   compaction. All five WUs landed in `internal/harness/` race-clean.

## Work Completed

### Decision: defer WU-060 (commit `339625b`)

Recorded the decision in three places: `track-a-bff-server.md`
§"WU-060" (full rationale), `status.md` (short pointer paragraph),
`internal/bff/turn.go` (runtime error message updated). Rationale:
the parallel execution + per-stream event tagging + cancellation
+ recovery semantics are the same hard parts FEAT-0013 sub-agents
must solve. The routing-based ergonomic
(`coding.review: [opus, gpt-5]`) is preserved as a thin sub-agent
flow, not BFF code. The `branch_id` field on streaming events is
retained — sub-agents may reuse it for per-agent tagging.

### Bundle 5 — Track B Bubbletea scaffold

Five WUs in `internal/harness/`, each its own commit:

- `ce633c9` — **WU-068 Bubbletea app scaffold**: App tea.Model,
  AppState (Focus / ConnState / Mode / model / context / cost /
  streaming buffer / messages / banner), three-zone layout
  (viewport | banner | input | status bar), KeyMap with
  configurable submit chord, full Bubbletea Msg surface (15 types
  covering Bundles 5/6/7/13), Update orchestration (global keys
  → focus-aware routing), streaming pipeline that grows the
  active assistant DisplayMessage from StreamTokenMsg/Complete,
  banner auto-clear on tick, dispatchSubmit with command parsing.
- `9aec280` — **WU-069 status bar**: connection-indicator badges
  for all 9 FEAT-0008 states, bracketed mode label, model name
  with override marker, context% with pressure-coloring (yellow
  ≥78%, red ≥92%), cost to 4 decimals, call timer when
  CallActive, formatTokens shorthand (1.2K / 12K / 1.5M).
- `aab2aab` — **WU-070 input area**: bubbles/textarea-based
  multi-line input, Submit produces SubmitMsg with
  command/@file/blank handling, HistorySource interface for WU-092
  with up/down traversal preserving the in-progress draft, paste
  detection via per-Update size delta (≥2KB default → emits
  PasteDetectedMsg), ExtractFileRefs regex, DetectDragDrop
  heuristic for absolute-path bursts.
- `7392d5f` — **WU-072 streaming markdown**: glamour wrapper with
  Render (final) and RenderStreaming (heals partial markdown:
  unclosed fences, inline code, bold; intentionally does NOT heal
  underscore-italic to avoid false positives in snake_case),
  ShouldRedraw / Pending implements 50ms debounce, SetWidth
  re-creates renderer on resize, countInlineBackticks skips fenced
  regions.
- `966e80a` — **WU-071 conversation viewport**: bubbles/viewport
  wrapper with auto-scroll + manual-scroll detection + snap-back
  on bottom, role-aware rendering (user "> ", assistant header +
  Glamour body + footer with metrics, tool call ⚙, tool result
  ✓), per-frame content rebuild from AppState.Messages so streaming
  buffer growth shows immediately.

## Files Created or Modified

Created (all under `internal/harness/`):
- `app.go`, `app_test.go`
- `model.go` (AppState, FocusZone, ConnStateInfo, DisplayMessage,
  TokenInfo, Conn/Role constants)
- `messages.go` (15 Bubbletea Msg types)
- `keys.go` (KeyMap + DefaultKeyMap + submit-chord variants)
- `statusbar.go`, `statusbar_test.go`
- `input.go`, `input_test.go`
- `viewport.go`, `viewport_test.go`
- `markdown.go`, `markdown_test.go`

Modified:
- `internal/bff/turn.go` — multi-model rejection error message
  updated to point at FEAT-0013.
- `docs/releases/v0.2.0/track-a-bff-server.md` §"WU-060"
- `docs/releases/v0.2.0/status.md`
- `go.mod` / `go.sum` — added bubbletea, bubbles, lipgloss, glamour
  and transitive deps (atotto/clipboard, goldmark, etc.).

## Bundle Status After This Session

| Bundle | Status | Note |
|---|---|---|
| 4 (BFF Foundation) | 4/4 | complete |
| 5 (Bubbletea scaffold) | 5/5 | complete (this session) |
| 8 (Sessions & Conversation) | 3/3 | complete |
| 9 (Model Config & Routing) | 3/4 | WU-060 deferred → FEAT-0013 |
| 10 (Streaming, Prompts, Cost) | 4/4 | complete |
| 11 (Context, Diagnostics, Recovery) | 3/4 | WU-061 compaction pending |

Plus auxiliary: WU-091 history handlers, WU-066 Ollama, WU-065
CLI wiring, the `handleTurnSubmit` pipeline.

## Notes / Decisions

- **App.dispatchSubmit lives on the App, not InputArea.Submit**:
  WU-068's tests use App.dispatchSubmit; WU-070 added a parallel
  InputArea.Submit method for direct unit testing of the input
  component. Both produce identical SubmitMsg shapes. WU-073
  (protocol client) will hand off via App, not InputArea.
- **Glamour width handling**: SetWidth re-creates the renderer
  rather than mutating the existing one. glamour v1.0.0 doesn't
  expose a width-setter on TermRenderer. Cheap on resize.
- **Viewport content rebuild on every View**: simpler than
  invalidation, and the message list is small (≤ N turns of ≤
  several KB each). Revisit if profiling shows hotspot.
- **Italic-underscore not healed**: too many false positives
  (snake_case, file paths). Documented in the heal function and a
  test pinning the no-heal contract.
- **No teatest harness**: tests construct App / components
  directly and call Update / View. Faster than spinning up a full
  tea.Program; covers the same surface for unit-level invariants.

## Next / Open Items

When resuming, the natural next steps:

1. **Track B Bundle 6 (WU-073 + 074)**: protocol client + connection
   manager. Without this, the harness UI doesn't actually talk to
   the BFF. This is the "make the harness functional" milestone.
2. **WU-061 compaction**: still waiting on design review for the
   trim heuristic + harness UX flow (compact.suggest →
   compact.plan → compact.apply chain).
3. **WU-067 BFF integration tests**: end-to-end harness driving the
   BFF over its socket. Useful before connecting Track B for real.
4. **Bundle 7 tools** (WU-075–079): the 13 harness-side tools.
   Substantial scope; the framework (WU-075) is the gating piece.

The repo now has a runnable `modeltap start` (proxy + BFF) AND a
buildable `internal/harness/` package. Next milestone is either
end-to-end (proxy ↔ BFF ↔ harness via WU-073/074) or breadth
(WU-061 / WU-067 / Bundle 7).
