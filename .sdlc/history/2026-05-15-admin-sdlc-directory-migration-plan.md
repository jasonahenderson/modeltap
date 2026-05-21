# 2026-05-15 — ADMIN: SDLC Directory Migration Plan

This plan defines how `modeltap` should migrate lifecycle artifacts from
`docs/` into `.sdlc/`, while preserving the existing release-phase rules and
preparing for a shared `sdlc` process project.

## Objective

Move project lifecycle artifacts into a dedicated `.sdlc/` tree:

- `docs/explorations/` -> `.sdlc/explorations/`
- `docs/features/` -> `.sdlc/features/`
- `docs/patches/` -> `.sdlc/patches/`
- `docs/releases/` -> `.sdlc/releases/`
- `docs/history/` -> `.sdlc/history/`

Keep user-facing and developer-facing product documentation in `docs/`:

- `docs/guides/`
- `docs/usage-guide.md`
- `docs/sample-config.yaml`
- other non-lifecycle docs

ADR placement needed an explicit decision before the move:

- Option A: keep ADRs in their previous docs home because they are durable engineering docs.
- Option B: move ADRs to `.sdlc/adr/` for complete lifecycle co-location.

Recommended first pass: keep ADRs in their previous docs home and revisit after the shared
`sdlc` project defines whether ADRs are lifecycle artifacts or architecture
documentation.

## Target Layout

```text
.sdlc/
  explorations/
  features/
  patches/
  releases/
  history/
  review-artifacts/
docs/
  adr/
  guides/
  usage-guide.md
  sample-config.yaml
```

`.sdlc/review-artifacts/` remains the home for reusable review prompt packs,
model overlays, and vendor execution wrappers. Artifact-specific reviews stay
with the artifact family:

- `.sdlc/features/.reviews/`
- `.sdlc/patches/.reviews/`
- `.sdlc/explorations/.reviews/`
- `.sdlc/releases/<version>/.reviews/`

## Migration Phases

### Phase 0 — Preconditions

- Confirm no active release phase work is in progress that depends on the old
  paths in the same commit.
- Inventory dirty worktree changes and avoid mixing unrelated edits into the
  migration.
- Decide ADR placement.
- Decide whether temporary compatibility READMEs are useful in old `docs/`
  locations.

### Phase 1 — Canonical Process Update

Update the canonical process and entrypoint files before moving artifacts:

- `.agents/process.md`
- `.agents/contracts/agent-team.md`
- `AGENTS.md`
- `docs/agents.md`

Required rule changes:

- Artifact taxonomy canonical homes point to `.sdlc/*`.
- Release phase pointer changes to `.sdlc/releases/<version>/plan.md`.
- Design placement changes to `.sdlc/releases/<version>/designs/`.
- Review placement changes to `.sdlc/releases/<version>/.reviews/`.
- Logging expectation changes to `.sdlc/history/`.
- Session resumption reads `.sdlc/releases/<version>/status.md`,
  `.sdlc/releases/<version>/plan.md`, and recent `.sdlc/history/` logs.

### Phase 2 — Mechanical Directory Move

Move the lifecycle directories with history-preserving `git mv`:

```sh
git mv docs/explorations .sdlc/explorations
git mv docs/features .sdlc/features
git mv docs/patches .sdlc/patches
git mv docs/releases .sdlc/releases
git mv docs/history .sdlc/history
```

If ADRs are included in the migration:

```sh
git mv <former-adr-directory> .sdlc/adr
```

Do not duplicate artifacts between old and new homes.

### Phase 3 — Reference Sweep

Replace live path references across tracked files:

- `docs/history/` -> `.sdlc/history/`
- `docs/explorations/` -> `.sdlc/explorations/`
- `docs/features/` -> `.sdlc/features/`
- `docs/patches/` -> `.sdlc/patches/`
- `docs/releases/` -> `.sdlc/releases/`

If ADRs stay in their previous docs home, do not rewrite ADR path references.

After the sweep, run:

```sh
rg -n 'docs/(history|explorations|features|patches|releases)' \
  AGENTS.md .agents docs .sdlc README.md internal cmd pkg .github
```

Remaining matches must be either:

- intentional historical prose that names the old path, or
- compatibility notes that point readers to the new path.

### Phase 4 — Compatibility Notes

If needed, create small placeholder README files in old locations:

- `docs/explorations/README.md`
- `docs/features/README.md`
- `docs/patches/README.md`
- `docs/releases/README.md`
- `docs/history/README.md`

Each placeholder should only say that the content moved to `.sdlc/<name>/`.
Do not keep indexes or artifact lists in both places.

Recommended first pass: skip compatibility directories unless external links or
tooling require them.

### Phase 5 — Validation

Run lightweight validation:

- `git status --short` to inspect the move set.
- `rg` sweep for stale lifecycle paths.
- `go test ./...` only if non-doc tooling or generated references changed.
- Manual review of `AGENTS.md` and `.agents/process.md` to confirm agents will
  resume from `.sdlc/`.

### Phase 6 — Commit Boundary

Commit the migration as process-only work:

```text
ADMIN: migrate lifecycle artifacts to .sdlc
```

Commit body should mention:

- canonical process paths updated
- lifecycle directories moved
- stale path references swept
- ADR placement decision

Do not include product implementation changes in this commit.

## Shared SDLC Project Boundary

The emerging shared `sdlc` project should own reusable process assets:

- artifact templates
- role contracts
- review prompt packs
- model/vendor review overlays
- hook utilities
- shared config schema
- sync/update tooling

`modeltap` should own project-local lifecycle state:

- `.sdlc/explorations/`
- `.sdlc/features/`
- `.sdlc/patches/`
- `.sdlc/releases/`
- `.sdlc/history/`
- repo-local process overrides

The shared project should not own `modeltap` release plans, status files,
feature specs, patch specs, or session history.

## Follow-Up Work

- Define a process config file that declares lifecycle paths, for example
  `.sdlc/config.json` or `.agents/process-config.json`.
- Make future hooks and validation scripts path-configured instead of
  hardcoding `.sdlc/features`, `.sdlc/patches`, or `.sdlc/releases`.
- Decide whether `docs/agents.md` remains a human-readable overview or moves
  into `.sdlc/`.
- Reconcile this plan with
  `.sdlc/history/2026-04-16-admin-shared-process-core-layout-and-sync-plan.md`
  when the shared `sdlc` project becomes concrete.

## Completion Criteria

- `.agents/process.md` names `.sdlc/*` as the canonical homes for lifecycle
  artifacts.
- Agents can resume release work using `.sdlc/releases/<version>/plan.md` and
  `.sdlc/releases/<version>/status.md`.
- Significant work logs are written to `.sdlc/history/`.
- No unintentional references to old lifecycle paths remain.
- `docs/` contains only product, architecture, guide, and compatibility docs.
