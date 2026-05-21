# Plan Review: v0.2.1 — FEAT-0014 + PATCH-0015

**Reviewer:** Claude (Opus 4.7, 1M context)
**Date:** 2026-04-25
**Artifacts reviewed:**
- `.sdlc/releases/v0.2.1/plan.md`
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `.sdlc/features/0014-harness-conversation-shell.md`
- `.sdlc/patches/0015-harness-shell-component-api.md`

## Summary

The plan correctly carves componentization off into its own patch release rather
than retrofitting it into v0.2.0 Phase 3. The track decomposition is reasonable
and the action/event API stance lines up with PATCH-0015. Two structural issues
are worth flagging before Phase 1 work begins: WU-097 risks overlap with
PATCH-0015 itself, and the inversion of "tests after implementation" in WU-102
is a deliberate-but-undocumented deviation from the project's normal
tester-first pattern.

## Strengths

1. **Patch release boundary.** Treating FEAT-0014 + PATCH-0015 as a separate
   v0.2.1 rather than appending to v0.2.0's active Phase 3 respects the release
   contract in `.agents/process.md` (no implicit phase transitions, no silent
   scope creep into an in-flight implementation phase).
2. **Parallelism is real.** WU-098 (component API) and WU-099 (host adapter)
   are genuinely independent design surfaces once WU-097 lands. WU-101 (docs)
   running off the accepted designs in parallel with WU-100 is also correct —
   docs against an API spec, not against shifting implementation.
3. **Behavior-preservation framing.** The plan explicitly forecloses UX
   redesign, branch/retry, and mouse-permission work. Holding that line is
   what makes "behavior-preserving extraction" actually achievable in one
   release.
4. **Scope of "verification" is honest.** The five verification points target
   contract-shape (action/event, no host coupling, host adapter doesn't leak,
   docs are sufficient, package is promotable) rather than feature checkboxes.
   That matches the patch's API-first intent.

## Risks / Open Questions

1. **WU-097 overlaps PATCH-0015.** PATCH-0015 already specifies: shell-owned
   responsibilities, host-owned responsibilities, command split, file/provider/
   permission boundaries, API-shape guidance, invariants to preserve, and a
   preferred three-package shape (reusable shell + host adapter + optional
   demo/fake). WU-097's listed deliverables — "package split strategy,
   behavior-parity constraints, host/runtime separation goals" — largely
   restate that. The genuinely new work in WU-097 is **migration sequencing
   from `internal/harnessspike/`** and an **entanglement audit** of what
   currently lives there. WU-097 should be retargeted to those two outputs to
   avoid producing a document whose first half duplicates PATCH-0015.
2. **WU-098 vs PATCH-0015 §8 ("API-shape guidance").** PATCH-0015 already
   forbids callback-shaped boundaries and lists specific anti-patterns
   (`OnApprove func()`, `Submit(..., onDelta func(...), onDone func(...))`).
   WU-098's design should state explicitly that it inherits PATCH-0015 §8 as
   policy and produces the concrete typed structs/IDs underneath. Otherwise
   reviewers in Phase 2 won't know whether a callback-shaped proposal is
   already out of bounds or still open for discussion.
3. **WU-102 inverts the project's tester-first convention.** CLAUDE.md
   describes the tester role as writing tests *before* implementation (TDD,
   red→green→security→docs). WU-102 is positioned as a regression sweep
   *after* WU-100 lands. For a behavior-preserving extraction that's actually
   defensible — the spike behavior is the oracle, and parity tests can be
   written against the post-extraction surface. But this is a deliberate
   deviation from the documented red-phase pattern and the plan should say so
   explicitly. Otherwise an agent picking up WU-102 may either re-derive the
   pattern or, worse, treat the absence of red-phase tests as a process
   failure to "fix."
4. **No "Done" gate is named.** Each WU has a `Done:` line ("Accepted
   refactor plan identifies..."), but the plan does not name who accepts.
   Phase 2 is the review phase, but the plan does not list a deliverable
   format for it. Suggest:
   - Phase 1 closes when designs land in `.sdlc/releases/v0.2.1/designs/` (or
     equivalent) and `plan.md` is updated to reflect Phase 2 entry as an
     `ADMIN:` commit.
   - Phase 2 closes with a `.reviews/` artifact (this directory) containing
     the reviews the user chose to commission, plus a Phase 3 entry commit.
5. **Spike runtime fate is unspoken.** PATCH-0015 §"Preferred package shape"
   mentions an "optional demo/fake runtime package." The plan does not say
   whether the existing `internal/harnessspike/` demo/fake behavior moves
   into that package, stays in place, or is dropped. WU-097 (or WU-099)
   should answer this; right now it is a hidden decision that will surface
   during WU-100 implementation.
6. **Permission-object boundary is fragile.** FEAT-0014's "Open Questions"
   defer the production permission object model. The plan defers it too. Fine
   in isolation, but WU-098's permission boundary needs a placeholder shape
   stable enough that the eventual production permission object can replace
   it without a second extraction. If the placeholder is too thin, v0.2.2
   becomes another extraction pass; if too thick, it leaks production
   semantics into a reusable component.
7. **WU-101 docs vs WU-100 implementation drift.** WU-101 is allowed to run
   in parallel with WU-100. Realistically, implementation will surface
   small contract adjustments (renamed fields, additional event variants).
   The plan should expect a doc-revision pass at the end of WU-100, even
   though WU-101's `Done` is reached against the design. Otherwise the
   "another engineer can embed without reading the spike" criterion is met
   on paper but not in the merged artifact.

## Recommendations

1. **Retarget WU-097.** Make its primary outputs:
   - a seam map (what moves out of `internal/harnessspike/`, what stays, what
     splits) referencing concrete files;
   - a migration sequence (which seam moves in which order during WU-100);
   - an entanglement audit identifying coupling between spike UI, demo/fake
     runtime, and permission scaffolding.
   Drop the parts that restate PATCH-0015 and instead reference it.
2. **Add an explicit Phase 2 deliverable.** Require at least one reviewer
   artifact under `.reviews/` per accepted design before Phase 3 opens.
   This pattern is already in flight for plan review (see `kimi-plan-review.md`
   and this file); make it the formal gate.
3. **Document WU-102's deviation.** A one-line note in the track file:
   "Parity tests follow implementation because the existing spike behavior
   is the oracle; this is a deliberate deviation from the standard
   tester-first red phase."
4. **Reconcile WU-101 with implementation drift.** Add a checkpoint to
   WU-100's `Done` criteria: "WU-101 docs reflect final API names and event
   shapes." Or split WU-101 into a design-time doc skeleton and an
   implementation-tail finalization step.
5. **Name the demo/fake runtime decision in WU-097 or WU-099.** Whichever
   design owns "modeltap host adapter" should state where the demo/fake
   runtime currently in `internal/harnessspike/` ends up: companion package,
   adapter-internal, dropped, or test-only.

## Verdict

Plan is sound and ready for Phase 1, conditional on retargeting WU-097 away
from work PATCH-0015 already covers. The plan's biggest hidden costs are not
in WU-100 (the obvious large one) but in the structural ambiguities around
WU-097's value-add, the demo/fake runtime's destination, and the permission
boundary's placeholder shape. Resolving those during Phase 1 design will keep
WU-100 from absorbing them as implementation surprises.

## Dispositions

| Finding | Disposition | Notes |
|---|---|---|
| WU-097 overlaps PATCH-0015 | Accepted | Retargeted in `track-a-harness-shell-componentization.md` and clarified in `2026-04-25-design-refactor-plan-097.md` as seam map, entanglement audit, migration sequence, and demo-runtime decision work. |
| WU-098 should explicitly inherit PATCH-0015 §8 | Accepted | Added explicit policy inheritance text in `2026-04-25-design-shell-component-api-098.md`. |
| WU-102 test-last deviation should be documented | Accepted | Added an explicit note in `track-a-harness-shell-componentization.md`. |
| Phase 2 deliverable format is unnamed | Accepted | Added review gate language to `plan.md`. |
| Demo/fake runtime fate is unspoken | Accepted | `WU-097` now states that fake runtime behavior moves into an optional demo/fake adapter layer outside the reusable shell package. |
| Permission placeholder boundary may be unstable | Accepted | `WU-098` now constrains the placeholder permission contract to rendering metadata plus stable IDs only. |
| WU-101 may drift from WU-100 implementation | Accepted | Added explicit reconciliation expectation to the track file. |
