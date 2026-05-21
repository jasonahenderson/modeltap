# 2026-04-24 — Harness shell feature doc

## Goal

Capture the harness spike shell behavior as a behavior-scoped feature spec
instead of leaving it only in spike history and checklist notes.

## What changed

- Added `.sdlc/features/0014-harness-conversation-shell.md`
- Indexed the new feature in `.sdlc/features/README.md`

## Why

The harness spike has now settled the main shell interaction model:

- single scrolling transcript surface
- tail-mounted composer
- queued follow-up behavior
- non-modal composer-driven permission controls
- inline paste expansion with path/reference file tokens

Those behaviors are concrete enough to deserve a feature-level contract
separate from the broader `FEAT-0009` terminal harness definition.

## Notes

- `FEAT-0036` is already in use on another branch, so this doc uses
  `FEAT-0014`.
- The new feature deliberately excludes packaging details, production
  permission object modeling, and retry/branch semantics.
