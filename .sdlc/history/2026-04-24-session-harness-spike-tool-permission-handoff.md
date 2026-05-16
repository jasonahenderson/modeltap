# Session Handoff — Harness Spike (Tool / Permission Inline Approval)

Date: 2026-04-24
Branch: `spike/scrolling-surface-eval`
Base commit: `eb55263` (`SPIKE: scrolling-surface checkpoint — ▎ marker, flat surface, arrow history`)
Companion session log: `.sdlc/history/2026-04-24-session-harness-spike-tool-permission.md`

## Current Intent

Priority #1 on the spike is **partially finished**. The `/perm` inline
approval flow works end-to-end with `y` / `n` keybindings, grant continues
with tool execution + streamed reply, deny short-circuits with a trailing
assistant note. Next session should move to priority #2 (stop / retry /
branch controls for streaming) and return to the partial items on #1 as
they come up.

## What Changed This Session (Uncommitted)

Modified:

- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`
- `internal/harnessspike/styles.go`
- `.sdlc/history/2026-04-23-session-harness-spike-shell.md`

New:

- `.sdlc/history/2026-04-24-session-harness-spike-tool-permission.md`
- `.sdlc/history/2026-04-24-session-harness-spike-tool-permission-handoff.md`

Key behavior changes:

- `/perm` slash command drives a live request → permission event and
  pauses for user decision.
- `y` / `Y` grants (gated on empty input): appends `granted` + `running`
  + `done` events and starts streaming the assistant reply.
- `n` / `N` denies (gated on empty input): appends `denied` event and
  a short trailing assistant message, no tool execution.
- Inline "press y to grant · n to deny" hint rendered in yellow next to
  the active permission event.
- New event styles: `eventGrantedStyle` (bold green), `eventDeniedStyle`
  (bold red), `permissionHintStyle` (bold yellow).
- Default session preset now returns `nil`. Fresh app and `/clear` on
  the default session yield an empty transcript. Named presets (Tool
  Demo, Reference Layout, Dummy Stream) are unchanged.
- Packaging / extraction review has been pulled out of the numbered
  priority list and declared a merge gate.

## Verified by Local Checks

- `go build ./...`
- `go test ./internal/harnessspike`

## Pending User Validation (Interactive)

Run:

    go run ./cmd/modeltap harness-spike

Checks:

1. Startup: transcript is empty, input composer renders with the `▎`
   prompt marker, no seeded assistant blurb.
2. Type `/perm` and press Enter. Expect:
   - a user row
   - an event row "Read workspace/README.md" (requested)
   - a permission event "Permission required to read workspace file"
     with the inline hint "press y to grant · n to deny".
3. With input empty, press `y`. Expect:
   - the permission event turns green / bold (granted)
   - two more events appear (running, then done)
   - assistant begins streaming.
4. Type `/perm` again, then press `n` with input empty. Expect:
   - the permission event turns red / bold (denied)
   - no running/done events
   - a short trailing assistant note
   - no stream.
5. Type `/perm`, then type "yes please" into the input, then press `y`.
   Expect the permission to remain pending (typing wins; `y` does not
   grant while input is non-empty).
6. Type `/clear` on the default session. Expect the transcript to end
   up empty (no reseed).
7. Confirm arrow history on the composer: submit a couple of commands,
   then press `Up` / `Down` to walk history.

## Chat History Log (This Session)

Chronological log of user-visible checkpoints:

- Committed the previous session's work as `eb55263` (`SPIKE:`
  scrolling-surface checkpoint).
- Began priority #1: tool / permission event rendering. Mapped the
  existing state (events already render inline; what was missing was
  the interactivity).
- Locked the "inline" placement decision.
- User directed the inline keybinding approach (inline `y` / `n` rather
  than modal).
- Implemented `/perm` command, `pendingPermission`, grant / deny, new
  styles, inline hint.
- User requested removal of the inserted first dummy message. Emptied
  the default session preset; updated tests.
- Updated the running checklist:
  - Moved packaging into a "Required before merging back to `main`"
    section.
  - Kept priority #1 numbered but annotated as
    `(partially finished)`.
  - Enumerated open items under "Partially checked off → Inline tool /
    permission event rendering".
  - Rewrote Checkpoint and Immediate test targets to match today.
- Wrote this handoff and the companion session log.

## Worktree Notes

Current `git status` also includes pre-existing non-spike dirty files
carried over from earlier sessions:

- `internal/cli/harness.go`
- `internal/harness/app_test.go`
- `internal/harness/compact_test.go`
- `internal/harness/context_test.go`
- `internal/harness/input.go`
- `internal/harness/model.go`
- `internal/harness/sessions_test.go`

Untracked local-only artifacts that should stay untracked:

- `.claude/worktrees/`
- `modeltap` (built binary)
- Other non-spike history docs from parallel sessions
  (`2026-04-23-session-terminal-response-leak-iteration.md`,
  `2026-04-24-session-harness-focus-submit-model-fixes.md`).

These were intentionally not touched by this handoff.

## Resume Plan

1. Validate the seven interactive checks above.
2. If behavior is accepted, commit this session's spike-scoped work
   (app.go, app_test.go, styles.go, the checklist doc, this session's
   log, and this handoff) as a single `SPIKE:` commit. Use the existing
   commit-policy conventions (`git commit -s`, no Co-Authored-By unless
   required).
3. Begin priority #2: **stop / retry / branch controls for streaming.**
   Stop already exists (two-step Esc). Retry and branch are missing.
   Decide:
   - Does retry replay the last user prompt or open it for edit?
   - Does branch snapshot the transcript before retry, or mark the
     original reply as superseded in place?
4. Return to open items on priority #1 as natural opportunities arise:
   - "Always allow for this session" scope option on approval.
   - Multiple simultaneous / queued permissions.
   - Tool parameter / target display on permission events.
   - Permissions that originate mid-stream (not only from `/perm`).
   - Deny-with-reason input.
5. Merge gate (when the spike is otherwise done): **packaging /
   extraction review** — can the spike shell stand alone as a releasable
   component, and what seams are required if the harness embeds it.
