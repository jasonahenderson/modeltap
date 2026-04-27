# v0.2.2 — Status

**Current phase:** Phase 3 — Implementation (active)
**Branch:** `spike/scrolling-surface-eval` inherited from v0.2.1;
should retarget to a release branch before tagging
**Phase 1 closed:** 2026-04-27
**Phase 2 closed:** 2026-04-27
**Phase 3 closed:** —

## Phase 3 work units

After the Phase 2 review processing, WU-104 split into three slices
(per Codex #1) so WU-105 has a tight dependency on WU-104a alone.

| WU | Title | Size | State | Phase 1 design |
|----|-------|------|-------|----------------|
| 103  | `internal/harness` audit and salvage report | M | Phase 1 done | [`designs/2026-04-27-design-harness-audit-103.md`](designs/2026-04-27-design-harness-audit-103.md) |
| 104a | `Runtime.SubmitTurn` + scaffolding + BFF stub | M | Phase 1 done | bundled |
| 104b | `LoadPreview` + `ResolvePermission` + `InterruptRun` | M | Phase 1 done | bundled |
| 104c | `DispatchCommand` + `SummarizePaste` + MCP lazy-start | M | Phase 1 done | bundled |
| 105  | Production CLI entrypoint (`modeltap shell`) | M | Phase 1 done | bundled |
| 106  | Plumbing cleanup | M | Phase 1 done | bundled |
| 107  | Viewport-state accessor (SC3 follow-up) | S | Phase 1 done | [`designs/2026-04-27-design-viewport-state-accessor-107.md`](designs/2026-04-27-design-viewport-state-accessor-107.md) |

## Phase dependency graph

```
Phase 1 (design) ──→ Phase 2 (review) ──→ Phase 3 (impl)
       ✅                  ✅                  ▶ queued
```

Within Phase 3:

```
WU-103 (already an analysis-as-deliverable; treat as done at design time)
WU-104a ──→ WU-104b ──→ WU-104c
   │
   └────→ WU-105 (parallel after 104a)
                      │
                      ▼
                  WU-106 (cleanup; runs after 104b lands; finishes after 104c)

WU-107 (independent)
```

## Up next

- **Phase 2 → Phase 3 transition.** Explicit `ADMIN:` commit per
  Prime Directive #6, after which Phase 3 implementation begins.
- **WU-104a** kicks off Phase 3. Lands `Runtime.SubmitTurn` plus
  the supporting scaffolding (constructor, `deferredSender`,
  `AttachProgram`, correlation tables, BFF promise integration,
  `testutil/bffstub`).

## In progress

(none — Phase 2 closed; Phase 3 queued)

## Done this phase

- v0.2.2 plan opened `e788d30`, restructured to Phase 1 in
  `9cf82e4` (`ADMIN: open v0.2.2 Phase 1 — Design`).
- WU-103 audit landed at
  `designs/2026-04-27-design-harness-audit-103.md`.
- WU-104+105+106 design bundle landed at
  `a595c4d` then revised post-Phase-2 review (this commit).
- WU-107 design landed at
  `designs/2026-04-27-design-viewport-state-accessor-107.md`.
- Phase 1 closed at `7a4f063` (`ADMIN: close v0.2.2 Phase 1 — Design`).
- Codex Phase 2 design review accepted with 5 findings accepted +
  1 rejected (bare-`modeltap` shell launch). Disposition table at
  the bottom of `.reviews/codex-phase2-design-review.md`.
- Kimi Phase 2 design review accepted with 17 findings accepted
  (1 moot under the bare-`modeltap` rejection). Disposition table
  at the bottom of `.reviews/kimi-phase2-design-review.md`.
- All design docs revised to apply the accepted findings. Major
  changes:
  - WU-104 split into 104a / 104b / 104c (Codex #1).
  - Permission gating architecture moved into `ProductionRuntime`
    via `sync.Map` of per-`ToolCallID` channels (Codex #2 / Kimi #2).
  - `Runtime.InterruptRun` uses existing `CancelTurn` instead of
    inventing a new RPC (Codex #4).
  - `LoadPreview` path resolution via adapter token-attachment
    table (Codex #3).
  - `SubmitTurnSync` resolves the double-notify problem with a
    promise-router that the existing event bridge writes to in
    addition to `ProgramSender` (Kimi #4).
  - `deferredSender` concretely defined (Kimi #5).
  - `AttachProgram` runs before `Start` (Kimi #6).
  - `DispatchCommand` emits `HostStatusEvent` directly via the
    runtime's `ProgramSender` reference (Kimi #3).
  - Viewport-state cache pointer preallocated in `New()` so View
    can mutate through the pointer (Codex #5 / Kimi #13).
  - Layer 3 test scope changed to `net.Listener`-backed BFF stub
    (Kimi #9).
  - Bare `modeltap` does NOT change to launch shell (Codex #6
    rejected).

## Carried-over designs (still authoritative)

v0.2.2 implementation builds on these accepted v0.2.1 designs without
duplicating them:

- [WU-098 Shell Component API and Package-Boundary Design](../v0.2.1/designs/2026-04-25-design-shell-component-api-098.md)
- [WU-099 Modeltap Host Adapter and Integration Design](../v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md)

## Open items

- **Phase 3 dispatch.** Pending the explicit ADMIN phase-transition
  commit. WU-104a is the first implementation unit.
- **Branch retarget.** Inherited from v0.2.1; pending TPM decision.
- **MCP scope.** v0.2.0 added `internal/harness/mcp*.go`. The audit
  categorized them as keep; the production-wiring design lazy-starts
  MCP only on first MCP-namespaced command. Whether MCP ships in
  v0.2.2 vs. defers to a later release is decided at WU-104c
  implementation time based on whether the lazy-start integration
  works against the real MCP processes.
- **Permission timeout.** WU-104b adds a default 5-minute timeout
  on the permission-promise channel plus a cancellation path
  triggered by `Runtime.Close`. If users want a different default,
  it surfaces as a config option in WU-104b.
