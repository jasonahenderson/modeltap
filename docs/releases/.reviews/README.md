# Release Reviews

This directory documents the review convention for release planning and delivery artifacts.

## Canonical Location

Formal review artifacts for a release belong in the release-local `.reviews/` directory:

- `docs/releases/vX.Y.Z/.reviews/`

Examples:

- `docs/releases/v0.2.0/.reviews/codex-plan-review.md`
- `docs/releases/v0.2.0/.reviews/codex-readiness-review.md`
- `docs/releases/v0.2.0/.reviews/codex-post-ship-review.md`

When a work-plan or release review is authored by a specific model, harness, or agent, include that name in the filename when known so the artifact provenance is obvious.

## Scope

Use release-local reviews for:

- implementation plan reviews
- track or sequencing reviews tied to a specific release
- release readiness reviews before ship
- post-ship release retrospectives when they are release-specific artifacts

Do not use `docs/history/` as the canonical home for these reviews. `docs/history/` is the session/work log and should only note that the review happened and link to the release-local artifact.
