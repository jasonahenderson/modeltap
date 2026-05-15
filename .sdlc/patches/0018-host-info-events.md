---
patch: "PATCH-0018"
title: "Surface slash-command output via transcript host-info events"
status: "approved"
date: "2026-05-07"
related:
  - "FEAT-0014 (conversation shell)"
  - "FEAT-0008 (BFF server)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F2)"
branch: "patch/0018-host-info-events"
---

# PATCH-0018: Surface slash-command output via transcript host-info events

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

The conversation shell has no surface for displaying multi-line output
from host slash commands. Every command in
`internal/harnesshost/production_runtime.go` that wants to show a
result — `/models`, `/sessions`, `/runs`, `/context`, `/history`,
`/mcp`, `/model` (read), `/compact`, etc. — emits a
`harnessshell.HostStatusEvent` carrying the formatted output text. The
event handler (`internal/harnessshell/events.go:57`,
`applyHostStatus`) writes the text into `state.status` and the kind
into `state.statusKind`.

`state.status` is **never read** by the renderer. The projection
function `(Model).toRenderInput` (`internal/harnessshell/model.go:486`)
copies fields like `Title`, `ModelLabel`, `InputView`, `Streaming`,
`StreamPulse`, `InterruptArmed`, `Messages`, `Queued`,
`InputTokens`, and `PendingPermission` into `RenderInput`, but does
not include `state.status`. A repository-wide grep confirms 18
write-sites and zero read-sites for the field.

Net effect: every host slash command appears to do nothing. The user
types `/models`, the BFF returns the catalog, the harness builds the
output string and emits it, the state field is set, and the user sees
no change in the TUI.

This was caught by the v0.3.0 manual smoke test and is recorded as
Finding F2 in `.sdlc/releases/v0.3.0/retrospective.md`. Until F2 lands,
v0.3.0 ships with every host command silently broken.

## Scope

1. **Introduce `HostInfoEvent`** in
   `internal/harnessshell/types.go` that carries multi-line text and a
   structured kind tag. Implements `isHostEvent()`. Distinct from
   `HostStatusEvent`, which is retained for short single-line chrome
   updates ("Submitted", "Done", "Streaming response").

2. **Add an `applyHostInfo` handler** in
   `internal/harnessshell/events.go` that appends the event's text as
   a `TranscriptItem` (likely a new
   `TranscriptItemKindHostInfo` and `RoleHostInfo`, or reuses an
   existing role with a new kind). Multi-line content renders verbatim
   in the transcript like an assistant or tool result row.

3. **Route command-output emitters in
   `internal/harnesshost/production_runtime.go` to the new event.** All
   `r.sender.Send(harnessshell.HostStatusEvent{Status: <multi-line>,
   ...})` call sites that produce informational text become
   `r.sender.Send(harnessshell.HostInfoEvent{Text: <multi-line>,
   ...})`. `HostStatusEvent` continues to be used for short
   single-line chrome state ("No models available" stays as a
   StatusEvent because chrome can convey it; the empty-list message
   on its own line is fine).

4. **Project the new transcript item into `RenderInput`** in
   `(Model).toRenderInput` so the existing transcript renderer
   surfaces it.

5. **Renderer styling for host-info transcript rows** in
   `internal/harnessshell/render.go` — distinct visual treatment
   from assistant content (e.g., dim or bracketed) so the user can
   tell host-supplied info apart from model output. Implementer's
   call on exact styling.

6. **Wire `state.status` into the renderer.** The field is currently
   write-only; this patch closes the loop so short single-line
   status text (the existing `HostStatusEvent` payload, plus
   shell-internal sets like `"Submitted"`, `"Done"`,
   `"Streaming response"`, `"Permission required"`) is visible in
   the chrome.

   - Add `Status string` and `StatusKind StatusKind` fields to
     `RenderInput` in `render.go`.
   - Project `state.status` and `state.statusKind` into those
     fields in `(Model).toRenderInput`.
   - Render the status string in the bottom chrome (footer area
     below the composer), using `StatusKind` to drive styling
     (e.g., dim for `StatusReady`, accent for `StatusStreaming`,
     warning for `StatusError`, prompt for
     `StatusPermissionPending`). Implementer's call on exact
     placement and styling.
   - When `Status` is empty, the chrome row collapses cleanly (no
     reserved blank line).

7. **Unit tests:**
   - `internal/harnessshell/events_test.go` — `applyHostInfo` appends a
     transcript item with the expected role/kind/text.
   - `internal/harnesshost/production_runtime_test.go` — `/models` and
     at least one other command emit `HostInfoEvent` with the expected
     text shape (table-driven if the existing test structure supports
     it).
   - `internal/harnessshell/render_test.go` — render input containing a
     host-info transcript item produces visible non-empty output for
     the row.

8. **Smoke verification on a real binary.** Build, launch
   `modeltap shell`, run `/models` and `/sessions`, observe the
   catalog and session list rendered in the transcript. Submit a
   message and observe the chrome status transitioning through
   `Submitted` / `Streaming response` / `Done`. Capture the
   verification in the patch's commit message or a session log.

9. **Update `.sdlc/patches/README.md` index** with the new row.

## Out of Scope

- **Finding F1 (health-check wiring).** Already prepared in tree;
  stashed at `stash@{0}` pending its own commit. F1 may land before
  or after this patch; they are independent.
- **Finding F3 (cloud-provider probe target).** Separate root cause
  in `resolveProviderHost` / `checkCloudEndpoint`. Will get its own
  patch.
- **Redesigning the transcript model.** Adding a new kind or role
  is acceptable; reshaping `TranscriptItem` is not.
- **Persistence of host-info events.** They are display-only,
  shell-local, not stored in the BFF transcript record (matching the
  current behavior of `HostStatusEvent`).

## Checklist

- [ ] `HostInfoEvent` defined in `types.go`, implementing
  `isHostEvent()`
- [ ] `applyHostInfo` handler added in `events.go`; appends a
  `TranscriptItem`
- [ ] All informational `HostStatusEvent` emit sites in
  `production_runtime.go` migrated to `HostInfoEvent`
- [ ] `toRenderInput` projects the new transcript items
- [ ] Renderer styling differentiates host-info rows from assistant
  content
- [ ] `RenderInput` carries `Status` and `StatusKind`; renderer
  surfaces them in the chrome
- [ ] `toRenderInput` projects `state.status` and `state.statusKind`
- [ ] Empty `Status` collapses cleanly (no reserved blank line)
- [ ] Unit tests added for the three layers (events, production
  runtime, render)
- [ ] Render test covers chrome status visibility for at least
  `StatusReady`, `StatusStreaming`, `StatusError`,
  `StatusPermissionPending`
- [ ] `go test ./...` passes
- [ ] Smoke verification: `/models`, `/sessions`, `/runs`, `/context`
  visibly render in the transcript; `Submitted` /
  `Streaming response` / `Done` visible in chrome during a turn
- [ ] `.sdlc/patches/README.md` index updated
- [ ] Changelog entry: `.sdlc/releases/v0.3.0/changelog.md` if this
  becomes a v0.3.0 pre-tag fix; `v0.3.1/changelog.md` otherwise
- [ ] Release-scope decision recorded in
  `.sdlc/releases/v0.3.0/retrospective.md` outstanding-work section

## Release-Scope Decision Required

This patch's release home is not yet decided. Options:

- **(a) v0.3.0 pre-tag fix.** Hold the `v0.3.0` tag until this
  patch lands. Aligns with the retrospective's recommendation that
  shipping v0.3.0 with every slash command silently broken devalues
  the release.
- **(b) v0.3.1.** Tag v0.3.0 with F1 fixed only, ship F2/F3 in a
  fast-follow patch release.

Decision belongs in the release plan, not this patch doc. Recording
the decision and updating the changelog target is part of the
checklist above.
