# HIL Phase 2 Design Review: v0.2.1

**Reviewer:** Jason Henderson (HIL)
**Date:** 2026-04-26
**Scope:** Reviewed dispositions and applied fixes from the Codex and Kimi
Phase 2 reviews; surfaced one additional finding the external reviewers
missed.

## Findings

### HIL-001. `internal/harnessspike` end-state is redundant with `internal/harnessdemo`

- **Severity:** significant
- **Location:**
  - `WU-099` §"Fake/Demo Runtime Placement"
  - `WU-100` §"Spike compatibility package", §"Step 7", §"Stage E", §"Cutover
    Strategy / Spike cutover", §"Done Condition"
  - `WU-102` §"Layer 3", §"Verification Sequence", §"Tests that remain as
    thin cutover checks", §"Test Layout Proposal / Cutover tests", §"Test
    compatibility during cutover"
  - `FEAT-0014` §"CLI / UI Integration"
- **What's wrong:** After extraction, the current designs assign two
  end-state roles to `internal/harnessspike`:

  1. WU-100 Stage E: a thin Bubble Tea wrapper around `internal/harnessshell`
     plus a fake/demo host adapter — i.e., a CLI demo program.
  2. WU-102 Layer 3: a small home for cutover-only compatibility tests.

  WU-099 already proposes `internal/harnessdemo` as the canonical fake/demo
  runtime package. So the design ends with two demo packages
  (`harnessspike` + `harnessdemo`) doing overlapping work, plus a "spike"
  vocabulary that's stale once the spike is over. The "spike" concept is
  exploratory-code vocabulary; once the shell is a real component,
  retaining a package called `harnessspike` is misleading and adds a layer.

- **Suggested fix:** Delete `internal/harnessspike` entirely as part of
  v0.2.1. Specifically:

  1. WU-100 Stage E renamed to spike-package deletion. Any "thin demo
     wrapper" responsibilities collapse into `internal/harnessdemo`.
  2. WU-102 Layer 3 tests relocate to `internal/harnesshost` integration
     tests (or a focused top-level harness integration test if that yields
     clearer ownership). No tests remain in `harnessspike`.
  3. WU-099's enumeration of valid hosts is reduced to two: `harnesshost`
     and `harnessdemo`, plus pure test fakes for shell unit tests.
  4. The CLI entrypoint `modeltap harness-spike` will be replaced or
     renamed during v0.2.1 implementation. Final name is TBD; FEAT-0014's
     CLI section is updated to flag this as Phase 3 implementation work.
  5. The "spike behavior is the FEAT-0014 oracle" framing is preserved as
     a transitional concept during extraction (Stages A–D), but the spike
     package itself does not survive the release.

  This makes the v0.2.1 end state genuinely a clean cut: shell, host
  adapter, demo runtime, integration tests. No "spike" anywhere.

## Disposition

| Finding | Severity | Disposition | Rationale |
| --- | --- | --- | --- |
| HIL-001 | significant | accepted | Applied as per-WU doc fixes in subsequent commits. The spike package's end-state role is fully redundant once `harnessdemo` is the canonical fake-runtime home. |

## Notes

- HIL accepted the dispositions on all 28 Codex+Kimi findings as committed
  in `ff7934c` and the per-WU fix commits that followed (`02eccd7`,
  `8aa9e0c`, `836e43d`, `bec019d`, `3dde681`, `69d2b91`, `c66d211`).
- The deferred Kimi #13 (Bubble Tea action message envelope shape) remains
  deferred to WU-100 implementation; HIL did not push to promote it.
