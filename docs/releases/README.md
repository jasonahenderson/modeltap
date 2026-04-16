# Releases

This directory organizes implementation plans and delivery artifacts by semver release.

## Release Index

| Version | Status | Scope |
|---------|--------|-------|
| [v0.1.0](v0.1.0/) | shipped | Proxy core, CLI, dashboard, service management |
| [v0.2.0](v0.2.0/) | planning | BFF server + terminal harness (FEAT-0008, FEAT-0009) |

## Structure

Each release directory contains:

```
vX.Y.Z/
├── plan.md                    # Implementation plan — overview, tracks, WU summary
├── track-*.md                 # Per-track WU details (for parallel agent teams)
├── status.md                  # WU completion tracking (created when implementation begins)
└── changelog.md               # What shipped (created at release time)
```

## Versioning Policy

- **Major (vX.0.0)**: breaking changes to the user-facing interface, config format, or data schema
- **Minor (v0.X.0)**: new features (new capabilities, new commands, new config sections)
- **Patch (v0.0.X)**: bug fixes, security patches, documentation updates

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

Release assignments for FEAT-0010+ are tentative and will be confirmed during their planning phase.
