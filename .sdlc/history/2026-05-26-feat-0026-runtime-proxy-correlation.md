# 2026-05-26 - FEAT-0026 runtime-to-proxy correlation

## Context

The user asked how session history should connect to proxy-captured provider
requests/responses, and how `trace_id` should differ from session/run IDs.

## Work

- Created a separate worktree at `../modeltap-feat-0026`.
- Created branch `feat/0026-runtime-proxy-correlation` from `origin/main`.
- Drafted `.sdlc/features/0026-runtime-proxy-correlation.md`.
- Updated `.sdlc/features/README.md` feature index.

## Summary

FEAT-0026 defines:

- canonical relationship: `session -> turn -> run -> proxy capture`
- `run_id` as the exact execution-attempt correlation key
- `trace_id` as a logical execution-lineage key
- parent-run trace inheritance for fork/retry/continue/child runs
- optional user/API-supplied `trace_id` for advanced integrations
- proxy header stripping and capture persistence requirements

## Verification

- Documentation-only change; no code tests run.
