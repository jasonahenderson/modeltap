# Session Handoff - Harness Spike (Scrolling Surface)

Date: 2026-04-24
Owner: Codex + user
Branch: `spike/scrolling-surface-eval`
Base checkpoint commit: `73236c4` (`SPIKE: iterate harness shell interactions`)

## Current Intent

Evaluate a Claude-style single scrolling surface where the composer sits at the end of the transcript (not fixed), then decide whether to keep this layout and componentize the spike for harness integration.

## What Changed This Session (Uncommitted)

Files changed:
- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`
- `internal/harnessspike/styles.go`

Key behavior changes:
- Scroll-surface composer starts focused on input.
- Input defaults to one visible line.
- Removed `compose` label row to save vertical space.
- Startup seeded assistant copy replaced with more useful test guidance.
- Added spacing between input and footer action/status bar.
- Composer styling changed from dark slab to top/bottom ruled section.
- `alt+enter` now inserts newline (in addition to `ctrl+j`).
- Submit keeps focus on input (no automatic shift to transcript).
- Mouse/touchpad scroll no longer steals focus.
- `ctrl+k` and `ctrl+t` now handled via explicit key types (`tea.KeyCtrlK`, `tea.KeyCtrlT`) instead of string matching.

## Verified by Local Checks

Commands run successfully after latest edits:
- `go test ./internal/harnessspike`
- `go test ./internal/cli -run 'TestRootCommandExecutes|TestVersionFlag|TestSubcommandsRegistered|TestSubcommandsAcceptHelp|TestHelpListsAllSubcommands'`
- `go build ./cmd/modeltap`

## Pending User Validation (Interactive)

Run and validate in terminal:
- `go run ./cmd/modeltap harness-spike`

Specific checks:
1. `ctrl+k` opens command palette.
2. `ctrl+t` opens background agents.
3. `alt+enter` inserts newline in input.
4. Input focus remains active after sending a message.
5. Composer appearance reads as bordered section (no dark slab).
6. Scroll behavior still feels right with composer at transcript tail.

## Chat History Log (This Session)

Chronological log of user-visible checkpoints:
- User requested startup focus on input and single-line composer behavior.
- Composer label was removed and startup seeded copy was replaced with task-relevant guidance.
- Spacing between input and footer was increased for readability.
- Composer visual treatment changed from dark slab to ruled section.
- `alt+enter` newline support was added; input focus is preserved after submit.
- Mouse/touchpad scroll no longer steals focus to transcript.
- `ctrl+k` and `ctrl+t` were fixed to use explicit Bubble Tea key types.
- User asked account/plan details; local auth metadata showed ChatGPT `free` plan cache state.
- User asked how to refresh auth; documented `codex logout`, `codex login`, `codex login status`.
- Node mismatch discovered: shell `node` was v8 while Codex install expected Node 24.
- This handoff file was requested to preserve resume state and immediate next actions.

## Worktree Notes

Current `git status` also includes pre-existing non-spike dirty files:
- `internal/cli/harness.go`
- `internal/harness/app_test.go`
- `internal/harness/compact_test.go`
- `internal/harness/context_test.go`
- `internal/harness/input.go`
- `internal/harness/model.go`
- `internal/harness/sessions_test.go`

Untracked local-only artifacts remain:
- `.claude/worktrees/`
- `modeltap` (built binary)

These were intentionally not touched by this handoff.

## Resume Plan

1. Validate the six interactive checks above.
2. If behavior is accepted, commit spike-only files on `spike/scrolling-surface-eval`.
3. Update the living checklist in `docs/history/2026-04-23-session-harness-spike-shell.md` with accepted decisions.
4. Start packaging/extraction review for standalone component consumption by harness.
