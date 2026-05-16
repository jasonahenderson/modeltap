# 2026-04-10 — ADMIN: explorations and agent instructions

## Summary

Aligned `modeltap`'s repo instructions with the exploration-driven structure used in the `meetingplaceai/alpha` project, while preserving `modeltap`'s existing work-unit implementation flow.

## What Changed

- Added `.sdlc/explorations/README.md` to define the exploration artifact, lifecycle, statuses, front matter, and promotion rules.
- Added root `AGENTS.md` as a concise agent-facing contract for artifact usage, commit prefixes, and `ADMIN` tasks.
- Updated `CLAUDE.md` to include explorations in the taxonomy, clarify when to use each document type, and define commit-prefix/body expectations across `EXP`, `FEAT`, `PATCH`, `ADR`, `WU`, and `ADMIN`.
- Updated `.sdlc/features/README.md`, `.sdlc/patches/README.md`, and `.sdlc/adr/README.md` so each references explorations as the upstream artifact.

## Files Modified

- `CLAUDE.md`
- `AGENTS.md`
- `.sdlc/explorations/README.md`
- `.sdlc/features/README.md`
- `.sdlc/patches/README.md`
- `.sdlc/adr/README.md`
- `.sdlc/history/2026-04-10-admin-explorations-and-agent-instructions.md`

## Notes

- Explorations are explicitly upstream only and do not authorize code changes by themselves.
- `ADMIN:` is now clearly reserved for process/workflow/instruction-file changes.
- Existing `WU-NNN` commits remain the normal implementation path for accepted feature work in `modeltap`.
