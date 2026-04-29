# 2026-04-29 — Professional Harness Runtime Feature

## Context

Promoted the EXP-0011 harness-excellence discussion into a feature-level
umbrella document that relates durable runs, foreground/background agents,
workflow contracts, tool governance, artifacts, validation, memory, and routing.

## Work Completed

- Added `docs/features/0015-professional-harness-runtime.md`.
- Defined foreground and background agents as attached/detached durable runs
  sharing the same runtime substrate.
- Captured workflow examples for explorations, features, ADRs, releases,
  implementation, debugging, documentation, and devops.
- Made workspace isolation an explicit policy rather than a default assumption.
- Added a feature relationship map for the downstream ADR, feature, and patch
  artifacts implied by EXP-0011.
- Updated `docs/features/README.md` with FEAT-0015.
- Updated EXP-0011 frontmatter with `promoted-to: FEAT-0015`.

## Notes

- No product code changed.
- FEAT-0015 is `draft`; it relates the future work but does not authorize
  implementation by itself.
