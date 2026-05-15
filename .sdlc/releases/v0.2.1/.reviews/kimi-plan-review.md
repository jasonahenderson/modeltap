# Plan Review: v0.2.1 — FEAT-0014 + PATCH-0015

**Reviewer:** kimi-k2.6 (cloud)
**Date:** 2026-04-25
**Artifacts reviewed:**
- `.sdlc/releases/v0.2.1/plan.md`
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md`

## Summary

The plan is well-scoped and the dependency graph is clean. The decision to treat this as a separate patch release rather than extending v0.2.0's Phase 3 is correct — it avoids destabilizing an active implementation track.

## Strengths

1. **Clear separation of concerns:** WU-098 (component API) and WU-099 (host adapter) are parallelizable once WU-097 lands. This is the right split.
2. **Action/event boundary constraint:** Explicitly forbidding callback-shaped APIs in the component boundary will prevent circular dependencies between the shell and modeltap runtime.
3. **Test-last placement:** WU-102 (regression sweep) depends on WU-100 (implementation), which means tests can be written against the actual extracted boundary rather than being speculative.
4. **Repository-readiness:** The constraint that the package should be promotable to its own repo with minimal churn is a good long-term guardrail.

## Risks / Open Questions

1. **WU-097 is the critical bottleneck:** The refactor plan must precisely define what "behavior-preserving" means for the spike. The current spike includes demo/fake backend behavior, permission flows, and command history. Not all of this may belong in the reusable component. WU-097 should explicitly list which spike behaviors are in-scope for extraction and which are host-side concerns.
2. **Permission object contract:** The plan mentions "composer-driven permission handling" from FEAT-0014 but also excludes "production permission-object redesign beyond the agreed shell boundary." This creates a tension — if the current permission model is awkward, extracting it verbatim may bake in bad design. WU-098 should evaluate whether the permission boundary is stable enough to extract.
3. **Size of WU-100:** Marked as Large. If the spike has significant entanglement with demo/fake backend code, the extraction could grow. WU-097 should include a spike entanglement audit to scope WU-100 realistically.
4. **No design-review checkpoint listed:** Phase 2 is mentioned but there is no explicit deliverable for it. Consider adding a `.reviews/` artifact requirement to Phase 2 completion criteria.

## Recommendations

1. **WU-097 should produce a seam map:** A concrete list of which spike files/functions move to the new package vs. stay in modeltap host. This reduces risk for WU-100.
2. **WU-098 should include a provisional package name:** E.g., `pkg/harness/shell` or `internal/harness/shell` — this makes WU-099's adapter design concrete.
3. **WU-099 should define the minimal host interface:** What events must the host send? What actions does the shell emit? A concrete Go interface sketch would be sufficient.

## Verdict

Plan is sound and ready for Phase 1 execution. The biggest risk is under-scoping WU-097 — ensure it produces concrete seam boundaries, not just strategic direction.

## Dispositions

| Finding | Disposition | Notes |
|---|---|---|
| WU-097 needs explicit in-scope vs host-side seam boundaries | Accepted | `WU-097` now includes a seam map and entanglement audit against `internal/harnessspike/`. |
| Permission boundary may be unstable if extracted too literally | Accepted | `WU-098` now constrains the placeholder permission contract to stable rendering metadata plus IDs. |
| WU-100 may be under-scoped without entanglement audit | Accepted | `WU-097` now identifies the current entanglement hotspots that drive extraction risk. |
| Phase 2 lacks explicit `.reviews/` deliverable gate | Accepted | `plan.md` now names `.reviews/` artifacts as the Phase 2 close requirement. |
| WU-098 should include a provisional package name | Accepted | `WU-098` now names `internal/harnessshell` as the initial extraction target. |
| WU-099 should define the minimal host interface | Deferred to WU-099 | This is correct but belongs to the next design doc rather than the plan-processing patch. |
