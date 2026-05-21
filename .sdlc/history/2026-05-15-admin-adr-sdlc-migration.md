# 2026-05-15 — ADMIN: ADR SDLC Migration

## Summary

Moved ADR artifacts into `.sdlc/adr/` so all lifecycle and decision artifacts
live under the SDLC tree.

## Work Completed

- Moved ADRs and ADR review artifacts from the former docs location to `.sdlc/adr/`.
- Updated process, agent, contributor, governance, README, skill, release, patch,
  feature, and history references to the new ADR path.
- Fixed relative ADR links from moved release artifacts.

## Validation

- Stale former-ADR-path reference sweep is clean.
- Markdown ADR links from release artifacts point to `.sdlc/adr/`.
- `go test ./...` passed.
