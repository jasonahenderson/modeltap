# 2026-04-30 — Session: Claude peer review of v0.3.x plans against FEAT-0015–0022

## Summary

User requested a peer review of the v0.3.x release plans (v0.3.0 through
v0.3.4) for strict adherence to the Professional Harness Runtime feature
series (FEAT-0015 umbrella plus FEAT-0016 through FEAT-0022).

## What was discussed and decided

- Reviewed FEAT-0015 (umbrella) and FEAT-0016 through FEAT-0022 in full.
- Reviewed plan.md, status.md, and changelog.md for v0.3.0 through v0.3.4.
- Cross-referenced WU coverage against each FEAT's Success Criteria, Key
  Capabilities, and Configuration sections.
- Verified the `depends-on` chain across FEAT specs aligns with the
  release ordering.

## Verdict

The v0.3.x plan is structurally sound. Six concrete adherence gaps were
identified, all tractable as pre-Phase-1 edits rather than restructuring:

1. `workflow_type` has no introduction WU across v0.3.0–0.3.4, even though
   it is referenced by v0.3.2 (validation) and v0.3.3 (policy). Natural
   home: v0.3.0.
2. FEAT-0020 SC#4 ("approval decisions inspectable by run") is not
   satisfied within v0.3.2 — the audit-artifact code lands in v0.3.3
   WU-145.
3. Worktree implementation has no clear owner; v0.3.3 status leaves it as
   an open decision.
4. v0.3.2 WU-136 codegen evaluation harness is labeled "PATCH" with no
   `PATCH-NNNN` allocation, blocking traceable commits.
5. FEAT-0017 SC#3 and SC#4 deferrals from v0.3.0 to v0.3.3 WU-144 are
   implicit, not explicit.
6. Workflow slash commands (`/explore`, `/feature`, `/adr`, `/release`,
   `/implement`, `/debug`, `/docs`, `/devops`) are listed in FEAT-0015 as
   umbrella behavior but not anchored to any v0.3.x release.

## Actions taken

- Saved per-release peer review artifacts under each release's
  `.reviews/` directory using the `claude-plan-review.md` convention
  (matches v0.2.1 prior art).
- No FEAT or plan files were modified. The user has not yet requested
  edits.

## Files created

- `docs/releases/v0.3.0/.reviews/claude-plan-review.md`
- `docs/releases/v0.3.1/.reviews/claude-plan-review.md`
- `docs/releases/v0.3.2/.reviews/claude-plan-review.md`
- `docs/releases/v0.3.3/.reviews/claude-plan-review.md`
- `docs/releases/v0.3.4/.reviews/claude-plan-review.md`
- `docs/features/.reviews/syntheses/0015-0022-id-hygiene-claude.md`
- `docs/features/.reviews/syntheses/0015-0022-id-hygiene-claude.json`

## Follow-up review: identifier hygiene across FEAT-0015 – FEAT-0022

After dispositions on the per-release plan reviews, the user requested a
focused identifier-hygiene review across the eight FEAT specs. Saved to
`docs/features/.reviews/syntheses/` per the README's cross-feature
synthesis convention.

Verdict: 9 findings — 0 blocking, 5 significant, 4 advisory. `run_id`
discipline is strong; `session_id ↔ run_id` and `turn_id ↔ run_id`
relationships, `tool_call_id`, the FEAT-0021 "request ID"/`decision_id`
split, and the `branch_id` non-existence choice need explicit naming
before Phase 1 opens.

## Follow-up review: ownership and authority across FEAT-0015 – FEAT-0022

After the identifier review was processed, the user requested an
ownership/authority review with six guiding questions covering run
state, attachment, permission decisions, workspace lifecycle,
disconnected-executor behavior, and artifact ownership for local
outputs. Saved to `docs/features/.reviews/syntheses/`.

Files added:

- `docs/features/.reviews/syntheses/0015-0022-ownership-claude.md`

Note: paired `.json` findings files were initially created for both
syntheses then removed at user request — the project no longer uses the
JSON findings format described in `docs/features/.reviews/README.md`.
The earlier `0015-0022-id-hygiene-claude.json` was also removed.

Verdict: 8 findings — 0 blocking, 5 significant, 3 advisory. The
BFF/harness split is right in spirit but is stated as responsibilities
rather than authority. Five boundaries need an exclusive-authority
contract: run status/stage advancement (BFF-authoritative), attachment
state under multi-client and reconnect, permission decision record
authoring and revocation, workspace lifecycle (cleanup/orphan recovery/
remote ownership), and disconnected-executor behavior (currently OQ-only,
should be a stated principle even before the ADR completes).

## Files modified

None.

## What's next / open items

- User to decide which of the recommended pre-Phase-1 edits to apply, and
  whether Claude should draft the edits.
- Decisions outstanding for v0.3.0 plan: workflow_type WU, cost/usage
  capture in WU-113, explicit FEAT-0017 deferrals, prerequisite
  subsection naming v0.2.x dependencies.
- Decisions outstanding for v0.3.2 plan: FEAT-0020 SC#4 stub vs feature
  amendment, PATCH-NNNN allocation for WU-136.
- Decisions outstanding for v0.3.3 plan: worktree implementation
  ownership, `temp_copy` implementation status.
- Decisions outstanding for v0.3.4 plan: workflow slash command home,
  workflow_type prerequisite confirmation, FEAT-0011/0012/0013 split
  posture.
