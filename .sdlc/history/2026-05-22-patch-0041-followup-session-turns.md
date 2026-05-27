# 2026-05-22 - PATCH-0041 follow-up: session turn activity

## Context

Follow-up from `.sdlc/history/2026-05-22-patch-0041-handoff.md`.
User reported that sessions still appeared to write only the first turn, or
possibly only expose the most recent persisted turn.

## Changes

- Updated the foreground run + turn storage transaction so a submitted user
  turn also refreshes the parent session `updated_at`.
- Added regression coverage that sequential `turn.submit` calls persist both
  user turns and assistant turns in the same session.
- Refined the command split so bare `/session` and `/run` show compact status
  for the current session/run, while `show` / `details` performs the detailed
  drill-down. The host now tracks current run separately from active in-flight
  run, so `/run` still works after completion without blocking `/clear`.
- Filtered command-like legacy rows (for example `/models`) out of restored
  conversation state and `session.details` turn summaries. Slash commands
  remain host commands, not conversation turns.
- Changed bare `/model` to read `model.list` from the Runtime so it reports the
  persisted session override instead of only the host-local label.
- Fixed session status/detail formatting for override-only sessions so they show
  the effective override model instead of `unknown (override: ...)`.
- Made bare `/model` read the active session's persisted detail first, and made
  runtime `model.list` fall back to stored session overrides when active
  in-memory state is unavailable.
- Expanded `run.details` to include linked turn summaries and improved `/run
  show` rendering so it shows turn summaries and readable lifecycle
  events instead of only turn IDs and raw event names.
- Added a VS Code task, `modeltap stop all`, that terminates running
  `modeltap` processes started by the local task workflow.
- Cleared the production host's active-run marker when terminal runtime/shell
  events arrive, so session controls such as `/clear` do not remain blocked by
  a completed run.

## Verification

- `go test ./internal/runtime ./internal/storage ./internal/harnesshost`
- `go test ./...`
- `go build ./...`
