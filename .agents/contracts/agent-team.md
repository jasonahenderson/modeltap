# Modeltap Agent Team Contract

All agents inherit from `.agents/contracts/base.md`.

## Principles

1. Work should be broken into units small enough to complete in a single
   session when feasible.
2. Status-based resumption matters; new sessions should be able to continue by
   reading release status and recent history.
3. Accepted ADRs, accepted features, approved patches, and the active release
   plan constrain work.
4. Significant actions should be logged to `docs/history/`.

## Team Roles

- **TPM** — maintains release plan/status and sequences work
- **Design Engineer** — produces detailed technical designs
- **Test Engineer** — defines and writes tests
- **Backend Implementer** — writes Go production code
- **UI Implementer** — owns frontend/dashboard work
- **Infrastructure Engineer** — owns CI, build, release, tooling
- **Integration Tester** — verifies end-to-end behavior
- **Security Reviewer** — reviews completed work for security issues
- **Documentation Specialist** — updates user and developer documentation

## Release Workflow

For release-scoped work, agents operate within the release-level phase rules in
`.agents/process.md`:

1. Phase 1 — design all WUs
2. Phase 2 — review
3. Phase 3 — implement

No phase interleaving is allowed.

## Session Resumption

When resuming release work:

1. Read `docs/releases/<current-version>/status.md`
2. Read `docs/releases/<current-version>/plan.md`
3. Confirm the current phase
4. Check `docs/history/` for recent session logs
5. Continue only with work allowed in the current phase
