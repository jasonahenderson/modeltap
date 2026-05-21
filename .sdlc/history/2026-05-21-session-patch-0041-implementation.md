# 2026-05-21 — Session: PATCH-0041 session details command

## Summary

Implemented `PATCH-0041`: `/sessions show [id]` and
`/sessions details [id]` now inspect a session from inside the production
shell. The command calls `session.details`, renders a compact detail view, then
adds recent run summaries from `run.list`.

## Changes

- `internal/harnesshost/production_runtime.go`
  - added `show` and `details` subcommands under `/session` and `/sessions`
  - added active-session fallback when no ID is supplied
  - added a compact session detail formatter
  - added recent run summaries and run-native drill-down hints
  - updated `/help` to include `show [id]`
- `internal/harnesshost/testutil/runtime_stub.go`
  - added `session.details`, `session.list`, and `run.list` support
  - added request recording helpers for host-command tests
- `internal/harnesshost/production_runtime_test.go`
  - covered explicit ID, active-session fallback, singular alias, missing
    active session, formatter sections, recent runs, and run-list failure
- `.sdlc/patches/0041-session-details-command.md`
  - marked the patch done
- `.sdlc/patches/README.md`
  - marked `PATCH-0041` done in the patch index

## Follow-On Design Routing

The follow-on work from
`.sdlc/history/2026-05-21-session-patch-0041-handoff.md` should be split by
scope:

- **PATCH-0040** remains the home for `/sessions delete <id>` and
  `/sessions prune`. This is cleanup tooling, not shell chrome.
- **Next small patch, likely PATCH-0042**, should own the TUI-only
  clear-visible-transcript behavior (`Ctrl+L`, optional `/cls`) if we want it
  before the broader chrome work. It must not change `/clear`; `/clear` remains
  "start a new session."
- **FEAT-0024** is the right design home for the robust session browser,
  sidebar session navigation, command-palette session search, slash
  autocomplete, keybindings, and richer drill-down surfaces.
- **FEAT-0024 + FEAT-0017** should jointly cover the agents overlay: FEAT-0024
  owns the shell surface; FEAT-0017 owns the durable/background run data model
  feeding that surface.
- **`/history` needs a separate explicit design decision** before
  implementation. If it means command-history browsing, route it through the
  existing command-history lineage. If it means session transcript browsing,
  design it as part of FEAT-0024's browser/drill-down work. Do not overload it
  inside `PATCH-0041`.
- **Run transcript/event rendering** belongs under the run-native surface
  (`/run`, `/attach`, and future run patches), not under `/sessions show`.

## Verification

- `go test ./internal/harnesshost`
- `go test ./...`
- `go build ./...`
- `go vet ./...`
