# 2026-05-21 — Handoff: PATCH-0041 session details command

## Summary

Next implementation patch: `PATCH-0041`, on branch
`patch/0041-session-details-command`.

The patch wires `/sessions show [id]` and `/sessions details [id]` in the
production shell to the runtime's existing `session.details` RPC, then appends a
small recent-runs section sourced from `run.list`.

The goal is a narrow inspection command: users can list sessions, inspect one,
see its recent durable/background runs, then decide whether to resume the
session or drill into a run.

## Current State

- Patch authorization exists at `.sdlc/patches/0041-session-details-command.md`.
- `internal/runtime/session.go` already implements `session.details`.
- `internal/protocol/sessions.go` already defines `SessionDetail`,
  `TurnSummary`, and `ServerSessionEvent`.
- `internal/harness/client.go` already exposes `SessionDetails`.
- `internal/harnesshost/production_runtime.go` currently handles:
  - `/sessions`
  - `/sessions list`
  - `/sessions resume <id>`
  - `/sessions clear`
  - `/sessions fork`
  - `/sessions current`
- The help row currently omits details/show:
  `session:   /sessions [list|resume <id>|clear|fork|current]`

## PATCH-0041 Implementation Notes

Keep implementation in the host layer unless tests reveal a protocol/client
gap.

Primary file:

- `internal/harnesshost/production_runtime.go`

Expected changes:

1. Extend `handleSessionCommand`:
   - route `show` and `details` to a new helper
   - allow both `/sessions show [id]` and `/session show [id]`
   - preserve existing aliases and current `/clear` semantics

2. Add a helper similar to:
   - `handleSessionDetails(ctx context.Context, id string) error`

3. ID resolution:
   - when `id` is supplied, use it
   - when omitted, use `r.mode.SessionID()`
   - if still empty, return a status error such as
     `session.details requires <id> or an active session`

4. Runtime calls:
   - call `session.details` for the target session
   - call `run.list` with `SessionID: targetID` and a small limit, likely `5`
     or `10`
   - do not fail the whole command if the run list fails after
     `session.details` succeeds; append a short note or omit runs

5. Render one `HostInfoEvent`:
   - session ID and summary
   - created and last-active timestamps
   - model/model override, context percent, total cost
   - turns with sequence, compacted marker, model, cost, summary
   - files touched and modified
   - server events
   - recent runs with run ID, status/stage, attachment/input-needed/stuck
     markers, and title
   - short drill-down hint:
     `/run <id>` for details, `/attach <id>` for a live/detached stream

6. Update help text:
   - include `/sessions show [id]` or `/sessions details [id]`

## Test Targets

Add focused tests in `internal/harnesshost/production_runtime_test.go`.

Minimum coverage:

- `/sessions show <id>` calls `session.details` for that ID
- `/sessions details <id>` is equivalent
- `/sessions show` uses the active session ID
- `/session show <id>` works through the singular alias path
- omitted ID with no active session returns a status error
- formatter includes session fields, turn rows, file/event sections, and recent
  runs
- run-list failure does not hide otherwise valid session details

Verification before PR:

- `go test ./internal/harnesshost`
- `go test ./...`
- `go build ./...`
- `go vet ./...`

## Multiple Runs / Agents Semantics

Treat a session as the foreground conversation boundary and runs as durable
work streams attached to that boundary.

`/sessions show` should not inline run transcripts. It should only summarize
recent runs so a session with multiple runs/agents remains readable.

Drill-down remains run-native:

- `/runs` lists current-session runs
- `/run <run-id>` shows one run
- `/attach <run-id>` follows or rejoins a run stream
- `/detach` leaves a run stream without killing the run

This keeps foreground session history separate from background-agent streams.

## Follow-On Work

Do not expand PATCH-0041 into the richer browser. Keep these as follow-ons:

- **PATCH-0040:** `/sessions delete <id>` and `/sessions prune`.
- **Small patch or FEAT-0024:** TUI-only clear-visible-transcript behavior.
  Prefer `Ctrl+L` and optional `/cls`; do not change `/clear`, which now means
  "new session."
- **FEAT-0024:** interactive session browser/sidebar, command-palette session
  search, richer drill-down, and keyboard navigation.
- **FEAT-0024 / FEAT-0017:** agents overlay that shows live background run
  state across the current session.
- **Future explicit design:** `/history` semantics. Keep it reserved until the
  command-history vs session-transcript distinction is designed.
- **Run surface follow-up:** richer run transcript/event rendering belongs in
  `/run` and `/attach`, not in `/sessions show`.

## Suggested Next Step

Create `patch/0041-session-details-command` from current `main`, implement the
narrow host command, and keep the PR focused on inspectability rather than full
session navigation.
