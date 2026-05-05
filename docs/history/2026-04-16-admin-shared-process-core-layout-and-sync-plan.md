# Shared Process Core Layout And Sync Plan

This document proposes the best common structure for consolidating document
templates, process contracts, and hook guardrails across:

- `modeltap`
- `keyproxy/alpha`
- `meetingplaceai/alpha`

The goal is shared discipline without forcing the three repos into an
identical product workflow.

## Objective

Adopt one reusable process core for:

- document templates and required sections
- agent contracts and role vocabulary
- hook validation logic
- review artifact naming conventions
- process config shape

Keep repo-specific overlays for:

- directory names
- release execution policy
- project terminology
- tool-specific wiring

## Recommended Architecture

Use a three-layer model.

### Layer 1 — Shared Process Core

Owns the parts that should be identical across repos:

- ADR template and structural rules
- feature-spec template and task-plan shape
- patch template and required sections
- exploration front matter and promotion fields
- review naming conventions
- base contracts and reusable role contracts
- hook utility code and validation rules

### Layer 2 — Repo Overlay

Owns project-specific differences:

- `docs/adr` vs `docs/decisions`
- release execution model
- release directory conventions
- project terminology and examples
- local process additions

### Layer 3 — Tool Adapter

Owns tool-specific behavior only:

- `AGENTS.md`
- `CLAUDE.md`
- `.claude/settings.json`
- hook registration and command wiring

## Proposed Shared Core Layout

```text
process-core/
  contracts/
    base.md
    agent-team.md
    architect.md
    feature-spec.md
    document-review.md
    reviewers.md
  templates/
    madr.md
    feature-spec.md
    patch.md
    exploration.md
    release-plan.md
    release-status.md
  hooks/
    adr_utils.py
    adr-validate.py
    adr-gate.py
    adr-context.py
    adr-stop.py
    adr-compact.py
    patch_utils.py
    patch-validate.py
  schemas/
    process-config.json
```

## Configuration Model

Make hook logic config-driven instead of path-hardcoded.

Each repo should provide a local process config with fields like:

```json
{
  "process": {
    "decisions_dir": "docs/decisions",
    "patches_dir": "docs/patches",
    "features_dir": "docs/features",
    "reviews_dir": "docs/decisions/.reviews",
    "contracts_dir": ".agents/contracts",
    "templates_dir": ".agents/templates",
    "releases_dir": "docs/releases",
    "release_phase_gating": false,
    "adr_hooks_enabled": true,
    "patch_hooks_enabled": true
  }
}
```

`modeltap` should set `release_phase_gating` to `true`.

## Consolidation Rules

### Standardize Fully

- ADR structure and section names
- feature-spec structure and task-plan expectations
- patch required sections and statuses
- exploration front matter fields
- review artifact naming rules
- peer-review identity convention (`tool-model`)
- base hook behavior for structural validation

### Leave Repo-Specific

- doc home names
- release-phase rules
- commit examples and domain terminology
- special release artifacts
- project-specific review emphasis

## Known Drift To Fix Early

The duplicated patch hook utility in both `keyproxy/alpha` and
`meetingplaceai/alpha` still points at `docs/implementation/patches` while the
repos use `docs/patches/`. This should be fixed as part of the first shared-hook
pass rather than patched independently again.

## Rollout Order

1. Define the shared core file set and config schema.
2. Repair hook path assumptions and make hook utilities config-driven.
3. Finish migrating `meetingplaceai/alpha` from `.claude/skills` to `.agents/`.
4. Finish reducing `modeltap` top-level instruction duplication.
5. Reconcile `keyproxy/alpha` against the shared core and remove local drift.
6. Add release templates and optional release-phase guard hooks.
7. Establish an update workflow so the three repos stay aligned.

## Distribution Strategy

Do not use a git submodule for the process core.

Preferred options:

- a dedicated internal repo that is synced into each project
- a subtree-style import
- a versioned sync script that copies approved files into each repo

The simplest operational model is a dedicated source directory or repo plus a
sync script. Each project keeps local copies of the final files so tools can
read them directly.

## Success Criteria

The consolidation is successful when:

- all three repos use the same template shapes for ADRs, features, patches, and
  explorations
- hook validation logic comes from one shared implementation model
- repo-specific differences are declared in config or overlay docs rather than
  hidden in copied scripts
- `AGENTS.md` and `CLAUDE.md` act as adapters, not the primary store of process
  rules
- release-specific differences remain possible without forking the whole system
