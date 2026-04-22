# 2026-04-22 — Release tag policy

Added release tag rules to the project process documentation.

Changed:

- `.agents/process.md` now requires one annotated `vX.Y.Z` tag per shipped
  release and defines the release-close sequence.
- `docs/agents.md` mirrors the tag lifecycle for agent-facing workflow.
- `docs/releases/README.md` documents how release tags work for release
  artifacts and GoReleaser.

Policy:

- Unpublished release tags may move when final commits are added.
- Tag moves must record old SHA, new SHA, and reason in `docs/history/`.
- Published tags are immutable by default; follow-up changes ship as a new patch
  release unless a maintainer explicitly approves a documented correction.
