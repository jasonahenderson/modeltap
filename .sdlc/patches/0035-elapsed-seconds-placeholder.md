---
patch: "PATCH-0035"
title: "v0.3.0 placeholder: append elapsed seconds to streaming status"
status: "proposed"
date: "2026-05-11"
related:
  - "FEAT-0024 (Shell UX Chrome — superset; proper structured status surface)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F17, placeholder fix)"
branch: "patch/0035-elapsed-seconds-placeholder"
---

# PATCH-0035: v0.3.0 placeholder — append elapsed seconds to streaming status

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

The shell's "what is happening" status during a streaming turn is
`Submitted` → `Working` / `Streaming response` → `Done`, with no
elapsed time, no running token count, no current stage, no active
tool, no interrupt hint. OpenCode / Claude Code / Codex all show a
structured streaming-status line that the user actually wants to
read.

The shell already receives the underlying events that would feed a
richer surface (`StreamCompleteMsg` carries `TokenInfo` and
`Duration` at end; `CostUpdateMsg` / `ContextUpdateMsg` /
`ToolActivityMsg` arrive during the turn; BFF stage transitions
exist). The projection layer
(`internal/harnesshost/projection.go`) collapses each into a flat
`HostStatusEvent` string that overwrites whatever was previously
shown.

Recorded as Finding F17. The full structured-status surface
(`StreamingStatus{Verb, Stage, InputTokens, OutputTokens,
ActiveTool, StartedAt, InterruptHint}` + 1Hz ticker + projection
rework + renderer composition) is FEAT-0024 / Shell UX Chrome
scope and tracked for v0.3.1.

This patch lands the smallest useful v0.3.0 increment:
**elapsed-seconds ticker**, so the user gets *some* live feedback
that work is happening and how long it has been.

## Scope

1. **Track `runStartedAt time.Time` in shell `state`.** Set on
   `RunStartedEvent` (or `SubmissionAcceptedEvent` if RunStarted
   hasn't fired yet); clear on terminal run events (Completed /
   Stopped / Failed).

2. **Add a 1Hz tick `tea.Cmd` while `streaming == true`.** Add a
   `streamTickMsg` private type; on `RunStartedEvent` start the
   tick loop, on terminal run events let it expire (the tick
   handler is a no-op once `streaming==false`).

3. **Append `(Ns)` to the streaming status line.** When the shell
   is in streaming state and `runStartedAt` is non-zero, the
   rendered status reads e.g. `Streaming response (4s)` instead
   of just `Streaming response`.

4. **No projection / renderer / contract changes.** The ticker
   only refreshes the existing `status` string; the structured
   status surface is deferred to FEAT-0024.

## Out of Scope

- **Running token / cost / stage / active-tool fields.** Defer
  to FEAT-0024.
- **Verb rotation ("Pondering" / "Cogitating" / etc.).** Defer.
- **Spinner glyph cycling.** Defer.
- **Interrupt-hint composition into status.** Defer; the existing
  `interruptArmed` path already manages "Press Esc again to
  interrupt".

## Checklist

- [ ] `state.runStartedAt` field added
- [ ] `RunStartedEvent` sets `runStartedAt = time.Now()`
- [ ] Terminal run events (Completed / Stopped / Failed) clear it
- [ ] 1Hz tick command (`tea.Tick(time.Second, ...)`) emits a
  private `streamTickMsg` while streaming, no-op once stream
  ends
- [ ] Status text appends `(Ns)` when streaming and `runStartedAt`
  is set
- [ ] Unit test covers: ticker starts on RunStarted, advances
  elapsed, stops on RunCompleted
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` F17 placeholder
  references this patch

## Fix Detail

The ticker is the only non-obvious bit:

```go
type streamTickMsg time.Time

func streamTickCmd() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return streamTickMsg(t)
    })
}
```

In Update, `streamTickMsg` triggers a re-render only if
`streaming` is true; if streaming is false, the tick is dropped
and the loop ends naturally. On `RunStartedEvent` the shell
returns `streamTickCmd()` so the loop begins; subsequent ticks
re-schedule themselves while streaming.

The status text composition stays inside `refresh()` / the
existing status field, e.g.:

```go
if s.streaming && !s.runStartedAt.IsZero() {
    elapsed := time.Since(s.runStartedAt).Round(time.Second)
    if !strings.Contains(s.status, " (") {
        s.statusElapsed = elapsed
    }
}
```

The renderer reads `state.status` plus the new `statusElapsed`
and composes the final footer.
