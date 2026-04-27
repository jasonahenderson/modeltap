# 2026-04-27 — Design: Viewport-State Accessor (WU-107)

## Scope

This design covers **WU-107 only** — closing the WU-102 SC3 follow-up
gap: manual scroll preservation has no direct automated assertion
because viewport state is private inside `Model.View`'s local copy.

This design does **not** redesign:

- the `harnessshell.Model` rendering path from
  [WU-098](../../v0.2.1/designs/2026-04-25-design-shell-component-api-098.md)
- any other shell behavior

## Context

[WU-102](../../v0.2.1/designs/2026-04-25-design-parity-regression-102.md)
required automated coverage for every FEAT-0014 success criterion.
SC3 — "manual scroll position is preserved when not following tail" —
landed without a direct assertion because `Model.View` constructs a
local copy of the viewport, paints content into it, and returns the
view string; the post-View viewport state never escapes the function.
The WU-102 commit
([`1f1e5b9`](../../../../docs/history/2026-04-26-session-wu-100-stage-e-and-wu-101-wu-102.md))
flagged this as the only outstanding coverage gap.

## Goals

1. Provide a public way for tests (and any host that cares) to read
   the current viewport scroll state without round-tripping through
   the rendered string.
2. Land at least one automated assertion that exercises the FEAT-0014
   SC3 invariant.
3. Preserve the existing rule that `Model.View` is pure: View must
   not mutate `Model` state observable to the caller.

## Non-goals

- Exposing internal viewport methods beyond what the SC3 assertion
  needs.
- Changing the rendering pipeline.
- Adding a setter — the test only needs to read.

## Design

### Public API addition on `harnessshell.Model`

Add a `ViewportState()` method that returns a small typed snapshot:

```go
// ViewportState is the read-only snapshot of the shell's transcript
// viewport state. It is populated by the most recent View call and
// reflects the scroll position the user would see if they rendered
// now.
type ViewportState struct {
    // YOffset is the current top-line offset into the rendered
    // content. Zero when scrolled all the way to the top.
    YOffset int
    // AtBottom reports whether the viewport is currently following
    // tail (i.e. additional content would auto-scroll into view).
    AtBottom bool
    // Width and Height are the viewport's current dimensions.
    Width  int
    Height int
}

// ViewportState returns a snapshot of the transcript viewport's
// current scroll state. The snapshot reflects the state after the
// most recent View call (or the initial state if View has not yet
// been called this tick).
func (m Model) ViewportState() ViewportState
```

### Implementation note: where the state is captured

`Model.View` currently makes a local copy of `state.transcript`
(viewport.Model), calls `vp.SetContent(...)`, returns `vp.View()`.
The local copy's YOffset etc. are computed inside SetContent based on
the prior YOffset of `state.transcript` and the new content size.

To populate `ViewportState()` without breaking View's purity, View
captures the post-SetContent viewport state into a private cache on
`Model.state` via a pointer field that is allowed to mutate (`*ViewportState`,
allocated lazily). The `Model` value still does not change observable
behavior — the cache is read-only from the outside, only the
`ViewportState()` accessor reads it.

Alternative considered and rejected: change View to (string,
ViewportState) signature. Rejected because Bubble Tea's `tea.Model`
contract is `View() string`; a different signature breaks the
interface.

Alternative considered and rejected: have Update populate the cache
on every tick by simulating render. Rejected because rendering on
every Update doubles the work and the test ergonomics are no better
than reading after a real View call.

### Test: SC3 parity assertion

`internal/harnessshell/viewport_test.go` (new file):

```go
package harnessshell

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

// TestManualScrollPreservedWhenNotFollowingTail asserts FEAT-0014
// SC3: when the user has scrolled up (i.e. is no longer at bottom),
// new transcript content does not auto-scroll the view back to the
// bottom.
func TestManualScrollPreservedWhenNotFollowingTail(t *testing.T) {
    m := New()
    // Size the viewport.
    updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
    m = updated.(Model)
    // Seed enough transcript to scroll.
    for i := 0; i < 50; i++ {
        m.state.transcriptItems = append(m.state.transcriptItems,
            TranscriptItem{Role: RoleUser, Text: "line"})
    }
    _ = m.View()
    initial := m.ViewportState()
    if !initial.AtBottom {
        t.Fatalf("seed should leave viewport at bottom, got %+v", initial)
    }

    // Scroll up via a mouse-wheel-like message. The exact mechanism
    // depends on the viewport's accepted msg types; the test uses
    // the public KeyMsg("k") path while focused on transcript.
    m.state.focus = FocusTranscript
    for i := 0; i < 5; i++ {
        updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
        m = updated.(Model)
    }
    _ = m.View()
    afterScroll := m.ViewportState()
    if afterScroll.AtBottom {
        t.Fatalf("scrolled up but still at bottom: %+v", afterScroll)
    }

    // Append more transcript content — simulating a streaming delta.
    m.state.transcriptItems = append(m.state.transcriptItems,
        TranscriptItem{Role: RoleAssistant, Text: "more"})
    _ = m.View()
    afterAppend := m.ViewportState()
    if afterAppend.AtBottom {
        t.Fatalf("manual scroll lost on append (auto-followed tail): %+v",
            afterAppend)
    }
    if afterAppend.YOffset != afterScroll.YOffset {
        t.Fatalf("YOffset moved on append: before=%d after=%d (SC3 says scroll preserved)",
            afterScroll.YOffset, afterAppend.YOffset)
    }
}
```

The test is a real assertion that fails if the rendering path ever
changes to auto-follow-tail behavior unintentionally.

### Risk: exposing internal viewport state

Adding `ViewportState()` increases the public API surface of the
shell package. This is acceptable because:

- `ViewportState` is a value type — no reference to the underlying
  `viewport.Model`.
- The accessor is read-only.
- The values are user-visible UI state, not implementation details.

Should the rendering pipeline change in a future release, the
`ViewportState` shape may need to evolve. Per
`.agents/process.md`, additive evolution is fine; breaking changes
require a new ADR.

## Acceptance criteria

WU-107 is complete when:

1. `ViewportState` type and `Model.ViewportState()` method are
   defined in `internal/harnessshell/`.
2. `internal/harnessshell/viewport_test.go` exists and contains at
   least the `TestManualScrollPreservedWhenNotFollowingTail` test.
3. The test passes against the current shell rendering pipeline.
4. WU-102 status note about the SC3 gap is updated to mark it
   covered.

## Implementation effort

Small. ~30 lines added to `model.go` (struct + method + tiny render-
time capture), ~50 lines of test. Single sitting.
