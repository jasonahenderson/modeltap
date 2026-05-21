# 2026-05-20 — Session Log: session details command patch

## Summary

Created proposed `PATCH-0041` to wire `/sessions show [id]` and
`/sessions details [id]` to the existing `session.details` runtime RPC, with a
recent-runs section sourced from `run.list`.

## Changes

- Added `.sdlc/patches/0041-session-details-command.md`
- Updated `.sdlc/patches/README.md` patch index

## Notes

- Chose `PATCH-0041` after updating local `main`: `PATCH-0036` is already
  owned by main for run/proxy correlation, so the branch-local smoke patches
  were renumbered and delete/prune is now reserved as `PATCH-0040`.
  prior handoff notes for `/sessions delete <id>` / `/sessions prune`.
- The patch explicitly defers richer session browsing, command palette/sidebar
  drill-down, agents overlay, and the TUI-only clear keybinding to FEAT-0024 or
  small follow-up patches.

## Verification

- Reviewed the new patch document and README diff.
- No code or tests were changed.
