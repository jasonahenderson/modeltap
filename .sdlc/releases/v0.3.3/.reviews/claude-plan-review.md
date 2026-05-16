# v0.3.3 — Plan Review (Claude Opus 4.7)

**Reviewer:** Claude Opus 4.7 (1M context), in-conversation peer review
**Date:** 2026-04-30
**Phase reviewed:** Planning draft (Phase 1 not yet opened)
**Scope:** v0.3.3 Policy-Grade Tool Runtime and Workspaces plan vs FEAT-0021 and FEAT-0017 background slice
**Companion reviews:** v0.3.0, v0.3.1, v0.3.2, v0.3.4 — `claude-plan-review.md` in each

## Verdict

All seven FEAT-0021 success criteria trace cleanly. The plan also correctly
absorbs the FEAT-0017 background-policy slice that was deferred from v0.3.0
(WU-144). **One unresolved scope decision** (worktree implementation
ownership) needs a home before v0.3.3 opens Phase 1.

## Success Criteria Trace

### FEAT-0021 (Policy-Grade Tool Runtime)

| SC | Trace | Status |
|---|---|---|
| #1 tool decisions consider workflow/workspace/foreground/background | WU-139, WU-140 | covered |
| #2 path/command/Git/domain policy can block or require approval | WU-141, WU-142 | covered |
| #3 background runs pause/deny outside policy | WU-144 | covered |
| #4 every tool decision recorded as run evidence | WU-145 | covered |
| #5 user can inspect why allowed/denied/blocked | WU-143 | covered |
| #6 simple permission levels work as presets | WU-141 (explicit) | covered |
| #7 foreground-policy slice can ship before FEAT-0017 background; background-specific behavior lands after | WU-141 first, WU-144 after | covered |

### FEAT-0017 background-policy slice

| SC | Trace | Status |
|---|---|---|
| #3 background pause/auto-deny on unapproved side effects | WU-144 | covered |
| #4 blocked-run inbox surfacing | WU-144 | covered |

## Findings (release-local)

### Finding 1 — worktree implementation has no clear owner

v0.3.3 status Open Items: *"Decide whether workspace isolation
implementation for `worktree` lands in this release or only the
policy/metadata shape."*

FEAT-0015 §"Workspace Policy" lists `worktree` as a canonical workspace
mode. WU-140 ("workspace mode resolver and run metadata integration")
covers metadata. If worktree creation/management code is deferred, no
v0.3.x plan currently owns it.

**Recommendation:** before Phase 1 opens, either:
- (a) commit WU-140 (or a new WU-140b) to also implementing actual worktree
  creation/cleanup for `worktree` mode, or
- (b) explicitly defer worktree implementation to a named patch or to
  v0.4.0+, with an `ADMIN:` decision recorded.

Implementation cannot be left silently unowned because FEAT-0015 SC#7
requires "workspace policy is explicit and testable" and FEAT-0017 §"Run
Queue" implicitly assumes worktree mode is reachable for parallel candidate
implementations.

### Finding 2 — `temp_copy` and `remote` workspace modes are also under-specified

v0.3.3 plan covers all five canonical modes in metadata (WU-140) but says
nothing about implementation status of `temp_copy` (copying a workspace
for risky/non-Git work) or `remote` (cloud sandbox).

The plan §"This release does not cover" includes "remote/cloud executor
implementation beyond policy shape," which is correct for `remote`. It is
silent on `temp_copy`.

**Recommendation:** add `temp_copy` to the "does not cover" list or make
WU-140 responsible for both `current`/`current_readonly` and `temp_copy`
implementations.

## Cross-cutting concerns affecting v0.3.3

### `workflow_type` referenced by WU-139

WU-139 ("Policy schema and inheritance model") expresses policy across
"workflows, foreground/background state, and workspace modes." The
`workflow_type` field has no introduction WU across v0.3.0–0.3.4. See
v0.3.0 review.

**Recommendation:** ensure v0.3.0 establishes workflow_type before v0.3.3
reaches Phase 3.

### Receives the deferred FEAT-0017 SCs from v0.3.0

WU-144 absorbs FEAT-0017 SC#3 and SC#4. Confirmed traced. v0.3.0 plan
should explicitly point to WU-144 as the implementation home (see v0.3.0
review, Finding 3).

### FEAT-0020 SC#4 dependency

If v0.3.2 chose to ship a stub approval-artifact schema (recommended in
v0.3.2 review, Finding 1), WU-145 ("Tool audit artifacts by run") fills
that schema. If v0.3.2 instead amended FEAT-0020 SC#4 to be gated, WU-145
becomes the first place where SC#4 is satisfied.

**Recommendation:** add a cross-reference from WU-145 to FEAT-0020 SC#4 so
the linkage is explicit during Phase 2 review.

## Process notes

- Track structure (Decisions / Enforcement / Harness+Artifacts /
  Verification) matches the FEAT-0021 capability groups.
- Risk register R1 ("rules-engine creep") is the right top risk.
- DoD #5 ("tool decisions are recorded as run artifacts") aligns with both
  FEAT-0021 SC#4 and FEAT-0020 SC#4.

## Recommended pre-Phase-1 edits (priority order)

1. Resolve worktree implementation ownership (Finding 1).
2. Clarify `temp_copy` implementation status in §"This release does not
   cover" or in WU-140 (Finding 2).
3. Add cross-reference from WU-145 to FEAT-0020 SC#4 (cross-cutting).
4. Wait on v0.3.0 workflow_type resolution before opening Phase 1.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Finding | Disposition |
|---|---|
| Worktree implementation ownership unclear | Accepted; default is metadata/policy shape only unless Phase 1 promotes worktree implementation explicitly. |
| `temp_copy` under-specified | Accepted; default is metadata/policy shape only unless Phase 1 promotes temp-copy implementation explicitly. |
| FEAT-0020 SC#4 linkage | Accepted; WU-145 now cross-references the v0.3.2 approval-decision artifact stub. |
| Inherited `workflow_type` dependency | Accepted; WU-139/status now require v0.3.0 `workflow_type` before Phase 3. |
