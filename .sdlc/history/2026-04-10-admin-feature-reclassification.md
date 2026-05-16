# 2026-04-10 — ADMIN: feature reclassification to explorations and patches

## Summary

Reclassified the unimplemented, upstream, and implementation-scoped documents so `.sdlc/features/` now contains only concrete feature contracts that correspond to implemented product work.

## Reclassification Map

- `FEAT-0001` → `EXP-0001`
- `FEAT-0002` → `EXP-0002`
- `FEAT-0005` → `EXP-0005`
- `FEAT-0006` → `PATCH-0002`
- `FEAT-0007` → `EXP-0007`

## What Stayed As Features

- `FEAT-0003` Web Dashboard
- `FEAT-0004` Service Management

## Why

- `FEAT-0003` and `FEAT-0004` are implemented and tied directly to completed work units.
- `0001`, `0002`, `0005`, and `0007` are still upstream framing or open design-space documents, so they fit the exploration taxonomy better.
- `0006` is implementation-scoped provider/routing/parsing work and fits the patch taxonomy better than the feature taxonomy.

## Files Updated

- Moved docs from `.sdlc/features/` into `.sdlc/explorations/` and `.sdlc/patches/`
- Updated indexes in `.sdlc/features/README.md`, `.sdlc/explorations/README.md`, and `.sdlc/patches/README.md`
- Updated cross-references in `CLAUDE.md` and `EXP-0007`
