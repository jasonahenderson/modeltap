# Implementation Review Guidelines

Implementation review is the post-implementation quality gate for release,
patch, and substantial work-unit changes. It verifies that the implementation
matches the authorized design or patch scope, that important failure modes are
covered, and that runtime evidence exists for user-visible behavior.

Implementation review is not a replacement for Phase 2 design review,
security review, integration testing, release smoke testing, or release
readiness review. It is the bridge between code completion and those later
gates.

## When To Run

Run an implementation review when any of these are true:

- a release work unit or bundled implementation slice is marked complete
- a patch changes production behavior, persistence, protocol, CLI, daemon,
  shell, provider, or security-sensitive code
- a change introduces or alters user-visible behavior
- a prior review, smoke test, incident, or retrospective asks for one

Small documentation-only, typo-only, or mechanical formatting changes do not
need an implementation review unless they modify process or release authority.

## Required Inputs

The reviewer should read:

- the active release `plan.md` and `status.md`, if release-scoped
- accepted feature, ADR, patch, and design artifacts that authorize the work
- the implementation diff and relevant surrounding code
- tests added or changed for the implementation
- recent history logs that explain decisions, deferrals, or known risks

If a required design or scope artifact is missing, the review should say so
explicitly instead of inferring broad authority from the code.

## Review Modes

Implementation review has two complementary modes.

### Static Conformance Review

Static review checks the code and tests without relying on live operation.
It should cover:

- scope conformance against accepted features, ADRs, patches, and designs
- API, protocol, storage, and persistence compatibility
- data ownership, transaction boundaries, idempotency, and recovery behavior
- error handling, cancellation, timeout, and retry paths
- concurrency, lifecycle, resource bounds, and cleanup
- secrets handling and accidental sensitive output
- logging and diagnostic signal for failures
- test coverage for state transitions, edge cases, and regressions
- deferred work recorded as tracked artifacts rather than informal notes

Static review is where design drift, missing guards, dead protocol fields,
unbounded maps, partial transactions, and missing regression tests are usually
found.

### Runtime Evidence Review

Runtime evidence checks that the built artifact behaves correctly through the
same startup and wiring paths users or operators exercise. This is closely
related to E2E and smoke testing: automated E2E tests should provide as much
of the evidence as feasible, and manual smoke checks cover the remaining
release-critical flows until they are automated.

For user-visible or operational surfaces, the reviewer should verify evidence
for:

- building the production artifact
- launching through the production CLI, daemon, server, shell, or UI path
- exercising the primary user-facing commands or flows
- observing visible output, rendered UI state, or API responses
- confirming logs or diagnostics expose failures with useful context
- checking stale process, socket, config, and environment edge cases when the
  surface depends on daemon or long-running process lifecycle

Runtime evidence does not need to be created by the reviewer personally if a
reliable artifact already exists. The review may cite automated E2E output,
smoke-test logs, screenshots, terminal transcripts, JSON-RPC probes, or CI
jobs. If no runtime evidence exists for a user-visible surface, record that as
a finding.

## Production-Wiring Coverage

When behavior crosses multiple layers, tests that instantiate each component
directly are not enough. The review should look for at least one test or
scripted check that goes through the production constructor or startup path
for the behavior being shipped.

Examples of production-wiring paths include:

- CLI command -> config loading -> server wiring -> handler -> output
- shell command -> host runtime -> JSON-RPC client -> BFF handler -> renderer
- provider config -> provider registry -> model registry -> routing decision
- daemon startup -> socket lifecycle -> client reconnect -> status command

If full E2E automation is too expensive for the current change, the reviewer
should require a focused smoke script or documented manual check and should
recommend follow-up automation.

## Static Analysis And Lint

Implementation review should check whether the relevant lint/static-analysis
gate ran. At minimum, release-bound work should account for:

- `go test ./...`
- `go vet ./...`
- deadcode or staticcheck coverage when available
- project-specific lint, race, integration, or packaging checks when relevant

Write-only state, unused event projections, and unreachable user-visible paths
should be treated as correctness bugs unless a maintainer explicitly accepts
the risk and documents the exception.

## Artifact Format

Implementation reviews for release work live under:

`.sdlc/releases/<version>/.reviews/`

Use a descriptive reviewer-first or release-first filename, for example:

- `v0.3.0-implementation-review.md`
- `codex-runtime-implementation-review.md`
- `claude-storage-implementation-review.md`

The review artifact should include:

- reviewer and date
- reviewed phase, branch, commit, or diff range
- scope and source artifacts reviewed
- commands, E2E jobs, smoke tests, or runtime evidence checked
- verdict
- findings ordered by severity
- discounted claims, when relevant, so future reviewers do not repeat them
- recommendations and pre-release blockers
- disposition table after findings are processed

Use these severity levels:

- **Blocking**: must resolve before release close, merge, or tag
- **High**: likely production correctness, data, security, or operator pain;
  resolve before release unless explicitly deferred
- **Medium**: meaningful risk, missing coverage, or hardening item; track if
  deferred
- **Low/Nit**: polish, clarity, or non-blocking cleanup

## Verdicts

Use one of these verdicts:

- **block**: do not merge, tag, or release until blocking findings are fixed
- **ship-with-fixes**: structurally sound, but named fixes must land first
- **ship-with-followups**: acceptable for release if follow-ups are tracked
- **pass**: no material findings beyond optional polish

Do not use "ready for release" unless runtime evidence has been reviewed or
the absence of runtime evidence is explicitly accepted as an exception.

## Relationship To Release Readiness

Implementation review may conclude that implementation is structurally
complete. Release readiness is a later gate and should also consume:

- implementation review disposition
- security review, when required
- E2E and smoke-test results
- packaging or install evidence, when relevant
- release notes, changelog, and rollback notes
- explicit maintainer decisions for any accepted exceptions

This distinction matters: a structurally complete implementation can still
fail when launched as a real binary through the real user path.
