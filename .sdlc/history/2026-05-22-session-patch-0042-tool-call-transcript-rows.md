# 2026-05-22 — Session: PATCH-0042 tool-call transcript rows

## Summary

Implemented `PATCH-0042`: runtime tool activity now renders as durable
transcript event rows instead of only transient footer status text. This makes
adjacent calls visibly distinct while preserving the active footer/status signal.

## Changes

- `.sdlc/patches/0042-tool-call-transcript-rows.md`
  - added and marked done
- `.sdlc/patches/README.md`
  - added `PATCH-0042` to the patch index
- `internal/harnessshell/types.go`
  - added `ToolActivityEvent` and `ToolActivityState`
- `internal/harnessshell/events.go`
  - added transcript append/update handling keyed by tool call id
  - mirrored latest tool activity into status chrome
- `internal/harnessshell/render.go`
  - mapped tool event rows to distinct inline rendering with indentation and
    state-specific foreground styling
  - mapped `error` event rows to the existing denied/error visual style
- `internal/harnessshell/styles.go`
  - added tool-event styles for running, done, and error/rejected states
  - added a left-border/panel treatment for host-info command output rows
- `internal/harnessshell/model.go`
  - tagged tool-correlated event rows for renderer-specific treatment
- `internal/harnessshell/state.go`
  - added tool-call correlation to event row state
- `internal/harnesshost/projection.go`
  - projected `harness.ToolActivityMsg` to `ToolActivityEvent`
- `internal/harnesshost/README.md`
  - updated runtime-to-shell event mapping
- Tests covered projection, transcript row update behavior, rendering, and
  keeping status chrome in streaming mode while a run is active.

## Verification

- `go test ./internal/harnessshell ./internal/harnesshost`
- `go test ./...`
- `go build ./...`
- `go vet ./...`
