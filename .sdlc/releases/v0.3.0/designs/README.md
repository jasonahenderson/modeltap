# v0.3.0 Design Review Index

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

This directory contains the complete Phase 1 design set for `v0.3.0` Run
Runtime Foundation.

## Release Scope

`v0.3.0` covers:

- FEAT-0015 umbrella constraints for the Professional Harness Runtime
- FEAT-0016 Managed Codegen Run Pipeline
- FEAT-0017 Durable Runs and Background Agents, foundation slice only

The release establishes durable run identity, lifecycle state, stage/status
events, attachment semantics, checkpoint metadata, foreground-run integration,
basic run inspection/list/control commands, reconnect/replay behavior, and
`workflow_type` persistence.

## Review Order

Read in this order:

1. [WU-108 run-runtime ADR design](2026-05-05-design-run-runtime-adr-108.md)
   and [ADR-0015](../../../../docs/adr/0015-run-runtime.md)
2. [WU-109 run storage design](2026-05-05-design-run-storage-109.md)
3. [WU-110 run protocol design](2026-05-05-design-run-protocol-110.md)
4. [WU-111 to WU-113 BFF runtime design](2026-05-05-design-bff-run-runtime-111-113.md)
5. [WU-114 to WU-116 harness surface design](2026-05-05-design-harness-run-surface-114-116.md)
6. [WU-117 verification/docs design](2026-05-05-design-runtime-foundation-verification-117.md)

## Review Focus

Primary questions:

- Does ADR-0015 settle ownership, executor availability, attachment semantics,
  checkpoint requirements, event ordering, and workflow type strongly enough for
  implementation?
- Does WU-109 reserve the right schema extension points for v0.3.1 through
  v0.3.4 without prematurely implementing those later features?
- Does WU-110 preserve `turn.submit` compatibility while introducing enough
  `run.*` surface for list, inspect, attach, detach, cancel, replay, and
  permission resolution?
- Does WU-111 through WU-113 define a transaction-safe BFF flow before provider
  dispatch?
- Does WU-114 through WU-116 preserve the detached transcript invariant and keep
  BFF protocol types out of `internal/harnessshell`?
- Does WU-117 test every release-critical invariant before Phase 3 starts?

## Phase 2 Findings Already Processed

The Phase 2 design reviews are recorded at:

- [`../.reviews/codex-design-review.md`](../.reviews/codex-design-review.md)
- [`../.reviews/claude-design-review.md`](../.reviews/claude-design-review.md)

Processed findings tightened:

- run permission resolution protocol
- `turn.submit` pre-dispatch transaction boundary
- model-call and tool-result idempotency
- attachment summary/lease authority
- run-list input-required and stuck semantics
- observability/liveness scope, including trace IDs, heartbeats, stage timeout,
  and checkpoint compatibility
- `run.create`, blocked/unblocked event naming, run-accounting granularity,
  reentry edges, `/run` subcommand stubs, and sequential v0.3.0 tool-loop policy

## Phase 3 Blocks

Implementation must not start until:

- FEAT-0015, FEAT-0016, and the FEAT-0017 foundation slice are accepted
- ADR-0015 is accepted
- v0.2.x prerequisite BFF/harness surfaces are confirmed reachable from
  `release/v0.3.0`
- an explicit `ADMIN:` commit transitions the release from Phase 2 to Phase 3
