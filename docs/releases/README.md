# Releases

This directory organizes implementation plans and delivery artifacts by semver release.

## Release Index

| Version | Status | Scope |
|---------|--------|-------|
| [v0.1.0](v0.1.0/) | shipped | Proxy core, CLI, dashboard, service management |
| [v0.2.0](v0.2.0/) | planning | BFF server + terminal harness (FEAT-0008, FEAT-0009) |
| [v0.2.1](v0.2.1/) | planning | Harness conversation-shell componentization (FEAT-0037, PATCH-0015) |

## Structure

Each release directory contains:

```
vX.Y.Z/
├── plan.md                    # Implementation plan — overview, tracks, WU summary
├── track-*.md                 # Per-track WU details (for parallel agent teams)
├── .reviews/                  # Release-local review artifacts
├── status.md                  # WU completion tracking (created when implementation begins)
└── changelog.md               # What shipped (created at release time)
```

## Review Artifacts

Formal review artifacts for a release live inside that release directory:

- `docs/releases/vX.Y.Z/.reviews/`

When a work-plan or release review is authored by a specific model, harness, or agent, include that name in the filename when known:

- `docs/releases/vX.Y.Z/.reviews/codex-plan-review.md`

Use this location for:

- plan reviews
- release-readiness reviews
- post-ship release reviews

`docs/history/` remains the work-log location, but it should point to the canonical review artifact rather than serving as the review artifact itself.

## Versioning Policy

- **Major (vX.0.0)**: breaking changes to the user-facing interface, config format, or data schema
- **Minor (v0.X.0)**: new features (new capabilities, new commands, new config sections)
- **Patch (v0.0.X)**: bug fixes, security patches, documentation updates

## Git Tags

Each shipped release is represented by one annotated Git tag named `vX.Y.Z`.
The tag points at the final release commit, after status, changelog, and
release-readiness artifacts are committed. Packaging tools such as GoReleaser
derive the release version from this tag.

If final commits land after the tag is created but before the release is
published, move the tag to the new release commit:

```bash
git tag -f -a vX.Y.Z -m "modeltap vX.Y.Z" <new-release-commit>
git push --force-with-lease origin refs/tags/vX.Y.Z
```

Log tag moves in `docs/history/` with the old SHA, new SHA, and reason. Once a
release is published or announced, treat its tag as immutable; ship follow-up
changes as a new patch release unless a maintainer explicitly approves and
documents a correction.

## Feature-to-Release Mapping

Features are assigned to releases based on dependency chain and delivery priority:

| Feature | Release | Description |
|---------|---------|-------------|
| FEAT-0003 (Web Dashboard) | v0.1.0 | Shipped |
| FEAT-0004 (Service Management) | v0.1.0 | Shipped |
| FEAT-0008 (BFF Server) | v0.2.0 | In planning |
| FEAT-0009 (Terminal Harness) | v0.2.0 | In planning |
| FEAT-0010 (Enterprise Auth) | v0.3.0 | Planned |
| FEAT-0011 (Knowledge Integration) | v0.3.0 or v0.4.0 | Planned |
| FEAT-0012 (Skills) | v0.3.0 or v0.4.0 | Planned |
| FEAT-0013 (Agent Teams) | v0.4.0+ | Planned |
| FEAT-0037 (Harness Conversation Shell) | v0.2.1 | Planned |

Release assignments for FEAT-0010+ are tentative and will be confirmed during their planning phase.
