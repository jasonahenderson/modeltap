# Releases

This directory organizes implementation plans and delivery artifacts by semver release.

## Release Index

| Version | Status | Scope |
|---------|--------|-------|
| [v0.1.0](v0.1.0/) | shipped | Proxy core, CLI, dashboard, service management |
| [v0.2.0](v0.2.0/) | planning | BFF server + terminal harness (FEAT-0008, FEAT-0009) |
| [v0.2.1](v0.2.1/) | planning | Harness conversation-shell componentization (FEAT-0014, PATCH-0015) |
| [v0.2.2](v0.2.2/) | released | Production conversation-shell wiring |
| [v0.3.0](v0.3.0/) | planning draft | Run runtime foundation (FEAT-0016, FEAT-0017 slice) |
| [v0.3.1](v0.3.1/) | planning draft | Context planner and project rules (FEAT-0018) |
| [v0.3.2](v0.3.2/) | planning draft | Validation, repair, and run artifacts (FEAT-0019, FEAT-0020) |
| [v0.3.3](v0.3.3/) | planning draft | Policy-grade tool runtime (FEAT-0021) |
| [v0.3.4](v0.3.4/) | planning draft | Memory, routing, and workflow extensions (FEAT-0022) |

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
| FEAT-0010 (Enterprise Auth) | v0.4.0+ or TBD | Deferred by v0.3.x Professional Harness Runtime planning |
| FEAT-0011 (Knowledge Integration) | related to v0.3.4; implementation TBD | Proposed, coordinated with FEAT-0022 |
| FEAT-0012 (Skills) | related to v0.3.4; implementation TBD | Proposed, needs coordination with FEAT-0022 |
| FEAT-0013 (Agent Teams) | v0.4.0+ or TBD | Proposed, needs coordination with FEAT-0022 |
| FEAT-0014 (Harness Conversation Shell) | v0.2.1 | Planned |
| FEAT-0015 (Professional Harness Runtime) | v0.3.0–v0.3.4 | Umbrella series |
| FEAT-0016 (Managed Codegen Run Pipeline) | v0.3.0 | Planning draft |
| FEAT-0017 (Durable Runs and Background Agents) | v0.3.0 | Foundation slice |
| FEAT-0018 (Context Planner and Project Rules) | v0.3.1 | Planning draft |
| FEAT-0019 (Validation and Repair Loop) | v0.3.2 | Planning draft |
| FEAT-0020 (Patch Evidence and Run Artifacts) | v0.3.2 | Planning draft |
| FEAT-0021 (Policy-Grade Tool Runtime) | v0.3.3 | Planning draft |
| FEAT-0022 (Durable Memory, Quality Routing, and Workflow Extensions) | v0.3.4 | Planning draft |

Release assignments for FEAT-0010+ are tentative and will be confirmed during their planning phase. The v0.3.x plans are draft roadmap artifacts until an explicit `ADMIN:` commit opens each release's Phase 1.
