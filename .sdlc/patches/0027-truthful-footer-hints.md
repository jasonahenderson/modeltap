---
patch: "PATCH-0027"
title: "Strip misleading footer hints (sidebar / palette / agents)"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0014 (Harness Conversation Shell)"
  - "FEAT-0024 (Shell UX Chrome — sidebar, palette, autocomplete, agents)"
  - "WU-100 §Definite scope rule"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F9)"
branch: "patch/0027-truthful-footer-hints"
---

# PATCH-0027: Strip misleading footer hints (sidebar / palette / agents)

## Problem

The composer footer rendered by `internal/harnessshell/render.go`
advertises three keybindings that the running shell does not handle:

```go
hint := "Ctrl+B sidebar  Ctrl+T agents  Ctrl+K palette"
```

`render.go` itself flags these surfaces as out of scope (line 13:
"Out-of-scope chrome (sidebar, command palette, agent list/detail,
session..."), and WU-100's commit message (`8f2d392`) says the spike
features "may resurface in a future modeltap top-level harness package
but are not WU-100 work" — a follow-up that never landed in
`internal/harnesshost` either.

The footer also unconditionally renders `"%d background agents
running"` from `RenderInput.AgentCount`, defaulted to 0 when the host
does not populate it, producing the persistent line `"0 background
agents running"` on every shell launch.

Recorded as Finding F9 in `.sdlc/releases/v0.3.0/retrospective.md`.
The corresponding **real** implementation is tracked under FEAT-0024
(Shell UX Chrome) targeting v0.3.x.

## Scope

1. **Replace the misleading hint** in `renderFooter` (render.go) with
   a string that only mentions keybindings that work in the current
   build:

   ```text
   Tab focus  Enter submit  Ctrl+J newline
   ```

   These three keys are actually wired in `model.go`'s focus-input
   handling and the textarea/composer paths.

2. **Hide the agent-count label when zero.** The label still
   surfaces when `AgentCount > 0` (so a future host that populates
   it can opt in without further renderer changes), but no longer
   renders `"0 background agents running"` by default.

3. **No keybindings change in this patch.** Real palette / sidebar
   / agents implementations land via FEAT-0024 work units; this
   patch only stops the footer from making promises the build
   cannot keep.

4. **No `RenderInput` field changes.** The injection seams
   (`Sidebar`, `Overlay`, `FooterHint`, `CommandCatalog`) outlined
   in FEAT-0024 are deliberately deferred to that feature's work
   units.

## Out of Scope

- **Implementing sidebar / palette / agents.** That is FEAT-0024,
  scheduled for v0.3.x.
- **Inline slash-command autocomplete.** Also FEAT-0024.
- **Adding a host-supplied footer hint mechanism.** Also FEAT-0024
  (WU-NNN-F in the spec).

## Checklist

- [ ] `renderFooter` replaces the Ctrl+B/Ctrl+T/Ctrl+K hint string
  with the truthful keys-that-work string
- [ ] `renderFooter` hides the "X background agents running" label
  when `AgentCount == 0`
- [ ] Existing render tests still pass (and any that asserted the
  old hint text are updated)
- [ ] `go test ./...` passes
- [ ] Smoke verification: run `modeltap shell`, confirm the footer
  shows only working keybindings and no agent-count line
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` Finding F9 status
  updated to "Fixed in PATCH-0027 (footer cleanup); real chrome
  tracked under FEAT-0024"
