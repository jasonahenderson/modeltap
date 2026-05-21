# 2026-04-27 — Session: shell splash identity

## Scope

Added a compact empty-state identity block to the reusable
conversation shell so `modeltap shell` and `modeltap shell-demo`
start with visible product branding before the first transcript row.

## Changes

- Wired the existing shell title option into `RenderInput`.
- Rendered a small bordered `modeltap` welcome block when the
  transcript and queue are both empty.
- Set the shell title in production and demo CLI entrypoints.
- Added a focused shell model test for the empty-state welcome block.

## Verification

- `go test ./internal/harnessshell`
- `OPENAI_API_KEY=test ANTHROPIC_API_KEY=test go test ./internal/cli`
  (rerun outside the sandbox because CLI tests bind Unix sockets)
