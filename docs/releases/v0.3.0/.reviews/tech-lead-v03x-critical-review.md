# Tech Lead Critical Review: v0.3.x Professional Harness Runtime Plans

**Date:** 2026-04-30  
**Reviewer:** Codex fallback senior-engineering review; requested `tech lead` skill was not available in this session.  
**Scope:** v0.3.0 through v0.3.4 release plans/status files, FEAT-0015 through FEAT-0022, and canonical process rules.

## Verdict

Do not open v0.3.0 Phase 1 until the release train has an explicit authority
gate for draft feature specs and the v0.2.x prerequisite state is reconciled.
The plans are directionally strong and the previous review processing improved
traceability, but the current train still has several places where Phase 1 could
appear process-clean while carrying unresolved product authority or ownership
risks into implementation.

## Findings

### Blocking — WU authority is not gated on accepted feature contracts

The process defines `WU` as an implementation work unit under an accepted
feature and says WU work is the normal path for accepted features already
decomposed into releases. `AGENTS.md` also says to follow accepted ADRs and
accepted features. FEAT-0015 and member FEAT-0016 through FEAT-0022 are still
`status: draft`, while v0.3.0 can be opened by an explicit `ADMIN:` commit and
its WUs are already listed as release work.

This creates a governance gap: the release can enter design against draft
feature contracts, and there is no visible release gate that prevents Phase 3
from starting if someone forgets to promote or explicitly exception those
features. This is especially risky for v0.3.0 because its schema/protocol work
becomes the substrate for the whole train.

References:

- `.agents/process.md`: WU scope and accepted-feature rule
- `AGENTS.md`: accepted-feature convention
- `docs/features/0015-professional-harness-runtime.md`: `status: draft`
- `docs/features/0016-managed-codegen-run-pipeline.md`: `status: draft`
- `docs/releases/v0.3.0/plan.md`: Phase 1 opens by `ADMIN:` commit only

Recommendation: before opening v0.3.0 Phase 1, add a gate that FEAT-0015,
FEAT-0016, and the FEAT-0017 foundation slice are accepted, or record an
explicit ADMIN exception that Phase 1 design is allowed against draft features
but Phase 3 is blocked until acceptance. Carry the same gate pattern to later
v0.3.x releases.

### Blocking — v0.2.x prerequisite state is internally inconsistent

v0.3.0 now requires v0.2.0, v0.2.1, and v0.2.2 surfaces before Phase 3, but
the release index still shows v0.2.0 and v0.2.1 as `planning` while v0.2.2 is
`released`. The status stack reads as impossible or at least ambiguous: a
later harness wiring release is released while the releases it depends on are
still planning.

If that is only metadata drift, it should be fixed before v0.3.0 opens. If it
reflects actual work not merged or not released, v0.3.0 Phase 1 should not
design against unstable FEAT-0008/0009/0014 surfaces without naming the exact
contracts that are safe to depend on.

References:

- `docs/releases/README.md`: v0.2.0/v0.2.1/v0.2.2 statuses
- `docs/releases/v0.3.0/plan.md`: v0.2.x prerequisites
- `docs/releases/v0.3.0/status.md`: prerequisite confirmation is only an open
  item before Phase 3

Recommendation: reconcile the v0.2.x release statuses or add a v0.3.0 Phase 1
entry gate naming the exact committed BFF/harness contracts that are available
for design.

### High — Workspace isolation has no committed 0.3.x implementation owner

FEAT-0015 requires workspace policy to be explicit and testable, and says
background writers must not mutate the current checkout unexpectedly. The
v0.3.3 plan lists all workspace modes, but the planning default explicitly
defers actual `worktree` and `temp_copy` creation/cleanup unless Phase 1 later
promotes them.

That is a reasonable simplification for policy metadata, but it means the
0.3.x plan may ship background write behavior that can only pause or deny,
without a usable isolated writer path. That falls short of the professional
harness scenarios that motivated parallel implementation attempts and separate
working trees.

References:

- `docs/features/0015-professional-harness-runtime.md`: workspace policy
  success criterion
- `docs/releases/v0.3.3/plan.md`: `worktree`/`temp_copy` default deferral
- `docs/releases/v0.3.3/plan.md`: WU-144 background blocked-operation behavior

Recommendation: make a pre-Phase-1 decision. Either promote `worktree`
implementation into v0.3.3 with explicit tests, or downgrade the 0.3.x
background-writer promise and add a v0.4.0+ owner for isolated writer
workspaces.

### High — The codegen evaluation harness is in v0.3.2 DoD without an artifact

v0.3.2 WU-136 is gated on a future `PATCH-NNNN`, but the v0.3.2 Definition of
Done still requires the codegen evaluation harness patch to exist and run. This
is better than an untracked patch, but it still lets the release enter Phase 1
with a required deliverable that has no numbered artifact, owner, or scope.

References:

- `docs/features/0015-professional-harness-runtime.md`: supporting patch is not
  in the behavior-contract map until drafted
- `docs/releases/v0.3.2/plan.md`: WU-136 depends on `PATCH-NNNN`
- `docs/releases/v0.3.2/plan.md`: DoD requires the patch to exist and run

Recommendation: allocate the PATCH before v0.3.2 Phase 1 opens, or remove
WU-136 from v0.3.2 DoD and treat it as a separate release dependency with its
own approval path.

### Medium — Cross-release ADR sequencing is too distributed for the schema risk

FEAT-0015 says the series needs several ADRs or ADR sections before moving from
draft/proposed into implementation planning. The release train distributes those
ADRs across v0.3.0 through v0.3.4, but v0.3.0 will already define run schema,
event vocabulary, checkpoints, `workflow_type`, and downstream stage names.

Those choices constrain prompt layering, artifact storage, policy/workspace
metadata, and routing. If each later release discovers incompatible
requirements, the run schema becomes migration churn or a lowest-common-
denominator contract.

References:

- `docs/features/0015-professional-harness-runtime.md`: Future ADR list
- `docs/releases/v0.3.0/plan.md`: WU-109/WU-113 schema and stage ownership
- `docs/releases/v0.3.1/plan.md`: prompt/context metadata activation
- `docs/releases/v0.3.2/plan.md`: artifact and approval metadata activation
- `docs/releases/v0.3.3/plan.md`: policy/workspace metadata activation

Recommendation: add a small v0.3.0 Phase 1 deliverable that reviews the run
schema against the future ADR topics before WU-109 closes. It does not need to
settle every later release, but it should reserve extension points and define
compatibility rules.

### Medium — v0.3.4 split path is still mechanically incomplete

v0.3.4 correctly says FEAT-0011/0012/0013 may force a split or WU-153 deferral.
However, the WU table still makes WU-154 depend on WU-153, and the DoD still
requires workflow extensions to have an alignment path. If WU-153 is deferred,
the release needs a mechanically valid alternate checklist and DoD.

References:

- `docs/releases/v0.3.4/plan.md`: split/defer condition
- `docs/releases/v0.3.4/plan.md`: WU-154 depends on WU-153
- `docs/releases/v0.3.4/plan.md`: DoD includes workflow-extension alignment

Recommendation: before v0.3.4 opens, define the split plan explicitly:
memory/routing-only WU-154 scope and DoD if WU-153 is deferred, plus the future
release home for WU-153.

## Positive Notes

- The release train has the right high-level order: durable runs before context,
  validation/artifacts before policy audit enrichment, and memory/routing after
  evidence exists.
- The previous review processing correctly made `workflow_type`, stage
  activation, approval artifacts, and background behavior more traceable.
- The strict release-phase model is preserved; the remaining issues are mostly
  gates and ownership, not a need to reorder the whole train.

## Recommended Actions Before v0.3.0 Phase 1

1. Add a feature-acceptance or explicit-exception gate for v0.3.0.
2. Reconcile v0.2.x release status and identify stable BFF/harness contracts.
3. Add a v0.3.0 schema compatibility design check against later ADR topics.
4. Decide whether isolated writer workspaces are in v0.3.3 or owned by a later
   release.
5. Allocate the WU-136 evaluation PATCH before v0.3.2 Phase 1.
6. Predefine the v0.3.4 split mechanics for WU-153/WU-154.
