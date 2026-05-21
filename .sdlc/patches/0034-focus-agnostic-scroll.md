---
patch: "PATCH-0034"
title: "Focus-agnostic transcript scroll hotkeys"
status: "proposed"
date: "2026-05-11"
related:
  - "PATCH-0030 (terminal selection — removed default mouse capture)"
  - "FEAT-0024 (Shell UX Chrome — superset)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F16)"
branch: "patch/0034-focus-agnostic-scroll"
---

# PATCH-0034: Focus-agnostic transcript scroll hotkeys

## Problem

After PATCH-0030 made terminal-native selection the default,
mouse-wheel scroll only works after the user explicitly types
`/select` to enter chat-scroll mode. The only other scroll path is
keyboard, which is gated behind Tab-to-focus-transcript — and even
then the up/down arrows in the input zone are consumed for
history recall before the user can reach the viewport.

Result: in the default (input-focused, selection-enabled) state,
there is no scroll path at all. A user who just wants to scroll
back through their conversation to inspect previous output is
stuck. Surfaced during smoke testing when the user could not
scroll up to see earlier transcript content. The footer hint says
"Tab focus" but does not advertise the cost or benefit.

Recorded as Finding F16 in `.sdlc/releases/v0.3.0/retrospective.md`.

## Scope

1. **Add focus-agnostic scroll hotkeys in `model.handleKey`** —
   intercept these before any focus-specific handling and forward
   to the viewport directly:
   - `PgUp` / `PgDn` — page scroll (always; supersedes the
     textarea's cursor-page-up/down behavior in input focus, which
     is rarely useful in a chat composer)
   - `Alt+Up` / `Alt+Down` — line scroll (always)

   Earlier draft considered `Shift+PgUp/PgDn` but Bubble Tea's
   `KeyMsg` only has `Alt bool` as a generic modifier; Shift is
   encoded only for a handful of specific keys (`KeyShiftTab`,
   `KeyShiftUp`, etc.) and Shift+PgUp lacks a portable terminal
   escape. Plain `PgUp/PgDn` are universally supported and the
   trade-off (losing textarea's cursor-page behavior in input
   focus) is acceptable for a chat composer.

2. **Update the footer hint.** Replace the current generic right-
   side hint `Tab focus  Enter submit  Ctrl+J newline` with one
   that surfaces the scroll path:
   `PgUp/Dn scroll  Tab focus  Enter submit`.

3. **Test the focus-agnostic intercept.** A new test in
   `internal/harnessshell/events_test.go` (or `model_test.go`)
   confirms that the shortcuts scroll the viewport regardless of
   `state.focus`.

## Out of Scope

- **Restoring mouse-wheel as default.** That trade-off was decided
  in PATCH-0030; the slash command remains the toggle for users
  who prefer wheel-scroll over selection.
- **Auto-scroll-to-bottom button or shortcut.** Worth adding later
  alongside followTail UX in FEAT-0024.
- **Mouse-wheel-with-modifier (e.g. Shift+wheel) escape hatch.**
  Bubble Tea cannot subscribe to modifier-only wheel events
  without enabling mouse capture, which defeats PATCH-0030.

## Checklist

- [ ] `handleKey` intercepts `PgUp`, `PgDown`, `Alt+Up`, `Alt+Down`
  before focus dispatch
- [ ] Each shortcut forwards to viewport scroll (`PageUp`,
  `PageDown`, `LineUp`, `LineDown`) and returns handled=true
- [ ] Footer hint advertises the scroll shortcut
- [ ] Unit test covers focus-agnostic scrolling from FocusInput
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` F16 entry references
  this patch as fix

## Fix Detail

In `handleKey`, after the Tab focus-cycle handler and before the
existing `ctrl+n`/`ctrl+p`/`ctrl+o` cases, intercept the scroll
keys and call the viewport's scroll API directly:

```go
switch msg.Type {
case tea.KeyPgUp:
    m.state.transcript.PageUp()
    return true, m, nil
case tea.KeyPgDown:
    m.state.transcript.PageDown()
    return true, m, nil
}
if msg.Alt {
    switch msg.Type {
    case tea.KeyUp:
        m.state.transcript.LineUp(1)
        return true, m, nil
    case tea.KeyDown:
        m.state.transcript.LineDown(1)
        return true, m, nil
    }
}
```

Calling `PageUp`/`PageDown`/`LineUp`/`LineDown` directly rather
than synthesizing a key message keeps the intent explicit and
avoids any chance of the viewport's own `Update` having a side
effect (e.g., cursor handling on textarea-style widgets).

