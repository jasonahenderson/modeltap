# 2026-05-25 - PATCH-0041 follow-up status

## Context

Continuation of the PATCH-0041 session/details follow-up work recorded in:

- `.sdlc/history/2026-05-22-patch-0041-handoff.md`
- `.sdlc/history/2026-05-22-patch-0041-followup-session-turns.md`

The remaining issues investigated today were command/status semantics and
whether session/run/model inspection surfaces useful persisted state.

## Current State

- Bare `/session` is intended to show compact current-session status.
- `/session show [id]` and `/session details [id]` are intended to show the
  detailed session view, including turn summaries.
- Bare `/run` is intended to show compact current-run status.
- `/run show [id]` and `/run details [id]` are intended to show detailed run
  state, including linked turn summaries and readable lifecycle events.
- Slash commands such as `/models` should not count as conversation turns.
  Defensive filtering now hides legacy command-like turn rows from
  `session.details` and restored conversation state.
- Bare `/model` is intended to report the active session's persisted model
  override before falling back to runtime model-list state.
- Session model rendering now avoids `unknown (override: ...)` when only the
  override is known.
- Local development task setup is now represented as tracked repo tooling under
  `devops/`:
  - `devops/vscode/tasks.json` is the source template for ignored local
    `.vscode/tasks.json` task configuration.
  - `devops/README.md` documents task installation, rebuild/restart usage, and
    the `modeltap stop all` task.
- There is no migration path for polluted local PATCH-0041 development rows.
  For local smoke testing, stop modeltap, delete the configured SQLite database,
  and restart with a clean DB.
- The harness help text and user guide now distinguish session history from run
  lifecycle state:
  - `/session show [id]` exposes session metadata, turn summaries, files, and
    recent runs.
  - `/run show [id]` exposes run lifecycle/debug details, linked turn summaries,
    checkpoints, and recent events.
  - Full turn-content inspection is explicitly called out as not yet exposed by
    a shell command.

## Verification Run

- `go test ./internal/runtime ./internal/harnesshost ./internal/protocol`
- `go test ./internal/runtime ./internal/storage ./internal/harnesshost ./internal/protocol`
- `go test ./...`
- `go build ./...`
- `python3 -m json.tool .vscode/tasks.json`
- `python3 -m json.tool devops/vscode/tasks.json`
- `bash -n rebuild.sh`
- Parsed the `modeltap start` task command from `devops/vscode/tasks.json` and
  checked it with `bash -n -c`.
- `go test ./internal/harnesshost`

## Next Steps

- Do a clean manual smoke of `/session`, `/session show`, `/model`, `/run`, and
  `/run show` using a fresh modeltap start/shell pair.
- Reset local polluted SQLite state by stopping modeltap, deleting the configured
  dev DB, and restarting. Fresh installs default to `~/.modeltap/modeltap.db`;
  legacy installs may still use `~/.config/modeltap/modeltap.db`.
- Split commits if needed: product/runtime PATCH-0041 changes separately from
  any repo tooling changes.
