# v0.2.2 — Status

**Current phase:** Phase 1 — Design (complete; awaiting Phase 2 review)
**Branch:** `spike/scrolling-surface-eval` inherited from v0.2.1;
should retarget to a release branch before tagging
**Phase 1 closed:** _pending explicit ADMIN commit; designs complete_
**Phase 2 closed:** —
**Phase 3 closed:** —

## Phase 3 work units

| WU | Title | Size | State | Phase 1 design |
|----|-------|------|-------|----------------|
| 103 | `internal/harness` audit and salvage report | M | Phase 1 done | [`designs/2026-04-27-design-harness-audit-103.md`](designs/2026-04-27-design-harness-audit-103.md) |
| 104 | Concrete `harnesshost.Runtime` implementation | L | Phase 1 done | [`designs/2026-04-27-design-production-wiring-104-106.md`](designs/2026-04-27-design-production-wiring-104-106.md) |
| 105 | Production conversation-shell CLI entrypoint | M | Phase 1 done | bundled, see above |
| 106 | Plumbing cleanup | M | Phase 1 done | bundled, see above |
| 107 | WU-102 SC3 follow-up: viewport-state accessor | S | Phase 1 done | [`designs/2026-04-27-design-viewport-state-accessor-107.md`](designs/2026-04-27-design-viewport-state-accessor-107.md) |

## Phase dependency graph

```
Phase 1 (design) ──→ Phase 2 (review) ──→ Phase 3 (impl)
       ✅                  ⚠ awaiting           ⚠ blocked
```

Within Phase 3:

```
WU-103 audit (no impl; doc-as-deliverable already complete in design)
WU-104 ──→ WU-105 ──→ WU-106
WU-107 (independent)
```

## Up next

- **Phase 2 — Review.** User-chosen review of the three Phase 1
  design docs:
  - `designs/2026-04-27-design-harness-audit-103.md`
  - `designs/2026-04-27-design-production-wiring-104-106.md`
  - `designs/2026-04-27-design-viewport-state-accessor-107.md`
  Reviews land under `docs/releases/v0.2.2/.reviews/` per
  `.agents/process.md` §"Review Artifact Placement".

  Specific review questions raised by the production-wiring design:
  1. Does the BFF already implement `turn.interrupt` (or
     equivalent)? If not, scope decision for the server-side change.
  2. Does `ConnectionManager` already expose a sync-promise wrapper
     around `Client.SubmitTurn`? If yes, the design's "add one"
     plan becomes "use the existing one".
  3. Final CLI command name: design recommends `shell`; alternatives
     are `chat` or keeping `harness`.

- After Phase 2 closes, an explicit `ADMIN:` commit transitions to
  Phase 3.

## In progress

(none — Phase 1 designs complete; awaiting Phase 2 dispatch)

## Done this phase

- v0.2.2 plan opened (`e788d30`, then revised under
  `ADMIN: open v0.2.2 Phase 1 — Design`).
- WU-103 audit moved to `designs/2026-04-27-design-harness-audit-103.md`
  per process §"Design Artifact Placement". Original location
  (root of v0.2.2/) was non-conformant.
- WU-104 + WU-105 + WU-106 design bundle landed at
  `designs/2026-04-27-design-production-wiring-104-106.md`. Resolves
  the v0.2.1-deferred async/sync `SubmitTurn` bridge (block-inside-
  tea.Cmd-via-sync-promise-on-ConnectionManager) and identifies the
  state-holder story for the deleted `AppState`.
- WU-107 design landed at
  `designs/2026-04-27-design-viewport-state-accessor-107.md`.
  Read-only `ViewportState` accessor on `harnessshell.Model` plus a
  parity test that asserts FEAT-0014 SC3.

## Carried-over designs (still authoritative)

v0.2.2 implementation builds on these accepted v0.2.1 designs without
duplicating them:

- [WU-098 Shell Component API and Package-Boundary Design](../v0.2.1/designs/2026-04-25-design-shell-component-api-098.md)
- [WU-099 Modeltap Host Adapter and Integration Design](../v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md)

## Open items

- **Phase 2 dispatch.** Reviewers TBD (TPM may handle inline or
  delegate to external tools per v0.2.1 precedent — Codex / Kimi /
  HIL passes).
- **Branch retarget.** Inherited from v0.2.1; pending TPM decision.
- **Production CLI command name.** Recommendation `shell`; final
  decision happens at Phase 2 review.
- **MCP scope.** v0.2.0 added `internal/harness/mcp*.go`. The
  audit categorized them as keep; the production-wiring design lazy-
  starts MCP only on first MCP-namespaced command. Whether MCP ships
  in v0.2.2 vs. defers to a later release is a Phase 2 decision.
