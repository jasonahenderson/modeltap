# Modeltap Process Definitions

This document is the canonical, tool-agnostic process reference for `modeltap`.
Tool-specific instruction files such as `AGENTS.md` and `CLAUDE.md` should point
here rather than restating the same rules in full.

## Scope

This file defines:

- artifact taxonomy
- release-phase rules
- review artifact placement
- commit policy
- activity logging expectations

Detailed role contracts live in `.agents/contracts/`.

## Artifact Taxonomy

Choose the artifact by scope of change, not by size and not by semver
intuition.

| Type | Scope | Canonical Home |
|---|---|---|
| `EXP` | upstream problem framing and design-space exploration | `docs/explorations/` |
| `FEAT` | behavior-scoped product definition | `docs/features/` |
| `PATCH` | implementation-scoped product or engineering-system work | `docs/patches/` |
| `ADR` | architectural decision with future constraint value | `docs/adr/` |
| `WU` | implementation work unit under an accepted feature | `docs/releases/<version>/` |
| `ADMIN` | repo process / workflow / instruction changes | no numbered doc required by default |

Crystal-clear rules:

- `PATCH` does not mean semver patch. It means implementation-scoped work.
- `EXP` does not authorize implementation by itself.
- `WU` work is the normal implementation path for accepted features already
  decomposed into release work units.
- `ADMIN` is for process-only changes such as hooks, prompts, skills, contracts,
  review conventions, doc-structure rules, and instruction files.
- If a change mixes process work and product work, split it into separate
  commits or tracked artifacts.

## Artifact Grouping Metadata

Artifacts keep simple monotonic IDs (`FEAT-NNNN`, `PATCH-NNNN`, `ADR-NNNN`,
`EXP-NNNN`). Do not encode hierarchy in the identifier with suffixes such as
`FEAT-0015a`.

When multiple artifacts belong to the same roadmap, umbrella, or work stream,
use optional grouping metadata instead:

- `parent`: canonical parent artifact ID, for example `FEAT-0015`
- `series`: human-readable grouping name, for example `Professional Harness Runtime`
- `series-role`: the artifact's role in the group, usually `umbrella` or `member`
- `series-order`: optional integer used for planned ordering inside the group

YAML-frontmatter artifacts (`EXP`, `FEAT`, `ADR`, `PATCH`) use these fields in
frontmatter. Release plans can reference the same fields in normal markdown
tables when useful.

Grouping metadata is informational. It does not change artifact status,
acceptance, or implementation authority. A child artifact can be draft,
accepted, deferred, superseded, or implemented independently from its parent.

## Release-Level Workflow

`modeltap` executes releases in three strict phases at the release level:

1. **Phase 1 — Design**: design all WUs across all tracks
2. **Phase 2 — Review**: user-chosen review of completed designs
3. **Phase 3 — Implementation**: implement all WUs in dependency-legal order

Prime directives:

1. Phases are release-level, not WU-level.
2. Phase 1 is not complete until every track has designs for every WU in the
   active release plan.
3. Phase 2 contains no new design work and no implementation.
4. Phase 3 contains no silent design improvisation; if implementation reveals a
   flaw, revise the design explicitly.
5. The current phase lives in `docs/releases/<version>/plan.md`.
6. Phase transitions are explicit `ADMIN:` commits, never implicit.

## Design Artifact Placement

- Design docs live under the active release directory's `designs/`
  subdirectory: `docs/releases/<version>/designs/`.
- One file per design bundle. Bundle related WUs that share a contract surface.
- File naming: `<date>-design-<short-name>-<wu-range>.md`.
- Design docs are release deliverables, not session history. `docs/history/`
  records sessions and work logs; `designs/` holds the designs themselves.

## Review Artifact Placement

- Release planning and delivery reviews live under the active release
  directory's `.reviews/` subdirectory.
- Canonical per-doc findings keep the single filename `{stem}-findings.md`.
  Dispositions live in a table at the bottom of that file; no sidecar JSON.
- Non-canonical review artifacts should include the reviewer identity when
  known, for example `codex-plan-review.md`.
- `docs/history/` should record that the review happened and point to the
  canonical review artifact path.

## Release Tags

Every shipped release has exactly one annotated Git tag named `vX.Y.Z`.
The tag points at the release commit: the commit where the release status,
changelog, readiness review, and any final release notes are complete.

Release close sequence:

1. Finish all release-scoped work and update `docs/releases/<version>/status.md`
   to shipped/complete.
2. Finalize `docs/releases/<version>/changelog.md`.
3. Record or link the release-readiness review under
   `docs/releases/<version>/.reviews/`.
4. Commit those final release artifacts with an `ADMIN:` commit.
5. Create an annotated tag:
   `git tag -a vX.Y.Z -m "modeltap vX.Y.Z"`.
6. Run release packaging from that tag. GoReleaser derives `{{ .Version }}`
   from the tag.

Tag update policy:

- Before a release is published, announced, or used for distribution, the
  `vX.Y.Z` tag is a release-candidate pointer and may be moved if final commits
  are added.
- To move an unpublished tag, delete/recreate or force-update it locally:
  `git tag -f -a vX.Y.Z -m "modeltap vX.Y.Z" <new-release-commit>`.
- If the tag was pushed but the release is still unpublished, update the remote
  tag explicitly: `git push --force-with-lease origin refs/tags/vX.Y.Z`.
- Any tag move must be logged in `docs/history/` with the old SHA, new SHA, and
  reason.
- After a release is published or announced, the tag is immutable by default.
  New commits must ship as a new version, usually the next patch release. Moving
  a published tag requires an explicit maintainer decision and a correction note
  in the release history.

## Commit Policy

Every meaningful change should use one of these prefixes:

- `WU-NNN:` — implementation work under an accepted feature work unit
- `PATCH-NNNN:` — implementation-scoped work under an approved patch
- `FEAT-NNNN:` — drafting or revising a feature spec
- `ADR-NNNN:` — drafting or revising an ADR
- `EXP-NNNN:` — drafting or revising an exploration
- `ADMIN:` — process/admin/instruction changes

Commit body requirements:

- 1-3 short lines describing what changed
- reference the canonical doc path when one exists
- use `git commit -s` for DCO sign-off

## Logging

Significant work should be logged to `docs/history/` before stopping.

At minimum, log:

- process or instruction-file changes
- release planning/design work
- review processing
- implementation sessions that materially change the repo

## Precedence

When process definitions overlap:

1. `.agents/process.md` is the canonical process source.
2. `.agents/contracts/*.md` define role-specific obligations.
3. `AGENTS.md` is the concise cross-tool entrypoint.
4. `CLAUDE.md` is tool-specific guidance layered on top of the canonical
   process.
