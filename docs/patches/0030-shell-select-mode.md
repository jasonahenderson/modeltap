---
patch: "PATCH-0030"
title: "Add terminal text selection support"
status: "done"
date: "2026-05-08"
related:
  - "FEAT-0014 (Harness Conversation Shell)"
  - "FEAT-0024 (Shell UX Chrome — superset of this work)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F12)"
branch: "patch/0030-shell-select-mode"
---

# PATCH-0030: Add terminal text selection support

## Problem

Transcript text — assistant output, captured run IDs in `/runs`,
session ids in `/sessions`, command output rendered via
`HostInfoEvent` — cannot be selected with the mouse in the
production shell. The terminal's native click-drag selection is
suppressed because `internal/cli/shell.go` constructs the program
with `tea.WithMouseAllMotion()` so the conversation chrome can
react to wheel scroll, transcript-token clicks, etc.

The user-visible cost surfaced during smoke-test step 8: copying a
`run-...` id from `/runs` output to type into `/run <run-id>` or
`/attach <run-id>` is impossible without retyping by hand. The
hassle compounds for any text the user wants to extract from the
transcript.

Recorded as Finding F12 in `docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Make terminal-native text selection the default.** Production
   shell and demo startup no longer pass `tea.WithMouseAllMotion()`,
   so click-drag selection, copy shortcuts, and paste shortcuts remain
   owned by the terminal.

2. **Keep a shell-native `/select` command** that toggles mouse
   capture at runtime. Implemented alongside `/quit`, `/exit`, and
   `/clear` in `internal/harnessshell/model.go`'s key-input switch
   (those three are the existing shell-native pattern). With selection
   as the default, the first `/select` enters chat mouse-scroll mode;
   the next `/select` returns to selection mode.

3. **Add `mouseCaptureDisabled bool` to `state`.** The renderer
   does not need it — Bubble Tea's mouse-capture state is owned
   by the program runtime, not the model — but the shell tracks
   it so the status line can report which mode is active and the
   toggle has a definite source of truth. It defaults to true.

4. **Dispatch the appropriate Cmd** when toggled. When entering
   chat mouse-scroll mode, return `tea.EnableMouseAllMotion`. When
   returning to selection mode, return `tea.DisableMouse`. The host
   program, after receiving the Cmd via the standard tea.Model
   contract, executes the ANSI escape that hands mouse handling back
   to (or reclaims it from) the terminal.

5. **Status line confirms each mode.** "Selection mode - terminal
   handles mouse; type /select for mouse scroll" / "Chat mode -
   mouse captured for scroll; type /select for terminal selection".

6. **Tests:**
   - `events_test.go` (or `model_test.go`): typing `/select` toggles
     the bool, returns the right Cmd shape, sets the right status.

## Out of Scope

- **Keybinding (e.g. Ctrl+R) toggle.** Common Ctrl+chord candidates
  conflict with terminal/shell conventions; a slash command is
  unambiguous and discoverable. A keybinding could land later via
  FEAT-0024 once the broader UX work decides on chord allocation.
- **Per-row copy actions.** A "copy this run id" action surfaced
  near the row would be ideal but requires UX scaffolding from
  FEAT-0024 (palette / sidebar / hover hints).
- **Auto-restoring selection mode on next keystroke.** Toggle is
  sticky until explicitly toggled back. Less surprising than
  auto-restore during mouse-wheel scrolling.

## Checklist

- [x] Production and demo shell startup leave mouse handling with the terminal
- [x] `/select` recognized as a shell-native command in
  `model.go`'s key handler (similar to `/quit`/`/exit`/`/clear`)
- [x] `state.mouseCaptureDisabled` toggle bool, defaulting to selection mode
- [x] Returns `tea.DisableMouse` or `tea.EnableMouseAllMotion`
  Cmd as appropriate
- [x] Status line reflects current mode
- [x] Tests cover both directions of the toggle
- [x] `go test ./...` passes
- [ ] Manual smoke verification: launch shell, click-drag to select
  transcript text, copy via terminal-native shortcut (Cmd+C /
  Ctrl+Shift+C / etc.); `/select` enables mouse scroll and `/select`
  again restores terminal-native selection
- [x] `docs/patches/README.md` index updated
- [x] `docs/releases/v0.3.0/retrospective.md` Finding F12 status
  updated to "Fixed in PATCH-0030"
