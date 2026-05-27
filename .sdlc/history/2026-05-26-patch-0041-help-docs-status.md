# 2026-05-26 - PATCH-0041 help/docs status

## Context

Follow-up to PATCH-0041 command-surface cleanup. The user asked whether session
clearing and turn/run details were represented in help and user docs.

## Changes

- Updated harness `/help` text to document current `/session`, `/session show`,
  `/session clear`, `/clear`, `/run`, and `/run show` semantics.
- Updated `README.md` quickstart command summary to point users at `/session`,
  `/session show`, `/run`, and `/run show`.
- Reworked `docs/usage-guide.md` from a run-only section into "Harness Session
  And Run Commands".
- Documented that `/session clear` clears only live in-memory context and keeps
  stored turns.
- Documented that `/clear` starts a new conversation/session and does not delete
  stored history.
- Documented that run details are lifecycle/debug state and that full
  turn-content inspection is not exposed as a shell command yet.

## Verification

- `go test ./internal/harnesshost`

## Notes

- No `/turn` command was documented because it is not implemented.
- Commit split recommendation remains: product/runtime PATCH-0041 changes,
  repo devops tooling, then docs/help updates.
