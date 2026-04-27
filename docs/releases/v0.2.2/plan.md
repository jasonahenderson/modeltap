# Implementation Plan: v0.2.2 — Production Conversation-Shell Wiring

## Context

`v0.2.1` extracted the conversation shell into a reusable Bubble Tea
component (`internal/harnessshell`), wired a modeltap host adapter
(`internal/harnesshost`) that consumes shell actions and projects
runtime events back, and shipped a fake-runtime demo
(`internal/harnessdemo` + `modeltap shell-demo`). It explicitly
deferred WU-100 Stage D-4 — the production `Runtime` impl — because
the legacy `modeltap harness` TUI was reported as broken and the
state of its underlying plumbing (BFF connection, JSON-RPC client,
tool dispatcher, context manager, MCP) was unknown.

In v0.2.1 the legacy harness CLI command was deleted; the
`internal/harness/` package files remain on disk (still compile, unit
tests still pass) but are unreachable from any CLI command. v0.2.2
audits that plumbing piece-by-piece and either builds a `Runtime`
implementation on top of the salvageable parts or replaces what's
broken.

## Scope

This release covers:

- audit of the surviving `internal/harness/` files (categorized
  keep / refactor / delete with `harnesshost.Runtime` method
  mapping)
- concrete production `harnesshost.Runtime` implementation backed by
  the salvageable plumbing
- new production CLI entrypoint that wraps `harnessshell` +
  `harnesshost.Adapter` + the production `Runtime` impl
- deletion of any plumbing that turns out unsalvageable
- WU-102 SC3 follow-up: viewport-state accessor for the manual-
  scroll-preservation parity assertion

This release does not cover:

- UX redesign beyond what the new production wiring requires
- new conversation-shell behaviors not already in FEAT-0014
- promoting `internal/harnessshell` into its own repository
- BFF protocol changes beyond what the `Runtime` impl needs

## Approach

The release executes as a single track. Per
`.agents/process.md` §"Release-Level Workflow", the three release
phases apply:

1. **Phase 1 — Design**: every WU produces a design doc under
   `designs/`.
2. **Phase 2 — Review**: user-chosen review of the Phase 1 designs.
3. **Phase 3 — Implementation**: implement all WUs in dependency-
   legal order.

The current phase lives below; phase transitions are explicit
`ADMIN:` commits.

### Carried-over designs (still authoritative)

v0.2.2 implementation builds on these accepted v0.2.1 designs without
duplicating them:

- [WU-098 Shell Component API and Package-Boundary Design](../v0.2.1/designs/2026-04-25-design-shell-component-api-098.md)
  — defines the closed-typed action/event boundary
  (`harnessshell.Action`, `harnessshell.HostEvent`,
  `harnessshell.ActionMsg`). The new `Runtime` impl emits events that
  satisfy this contract without redefining it.
- [WU-099 Modeltap Host Adapter and Integration Design](../v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md)
  — defines the `harnesshost.Runtime` interface that v0.2.2's WU-104
  implements. Implementation strictly conforms; no new methods are
  added to `Runtime` in v0.2.2 unless WU-104's design explicitly
  amends WU-099 and that amendment is reviewed in Phase 2.

### Work units

The Phase 2 review (Codex finding #1) split WU-104 into three
slices so WU-105 can begin as soon as `SubmitTurn` lands:

| WU | Title | Dependencies | Size | Phase 1 design |
|----|-------|-------------|------|----------------|
| 103  | `internal/harness` audit and salvage report | — | M | [`designs/2026-04-27-design-harness-audit-103.md`](designs/2026-04-27-design-harness-audit-103.md) |
| 104a | `Runtime.SubmitTurn` + `ProductionRuntime` scaffolding + `testutil/bffstub` | 103 | M | bundled in [`designs/2026-04-27-design-production-wiring-104-106.md`](designs/2026-04-27-design-production-wiring-104-106.md) |
| 104b | `Runtime.LoadPreview` + `Runtime.ResolvePermission` + `Runtime.InterruptRun` | 104a | M | bundled, see above |
| 104c | `Runtime.DispatchCommand` + `Runtime.SummarizePaste` + MCP lazy-start | 104b, 106 (refactor pass) | M | bundled, see above |
| 105  | Production conversation-shell CLI entrypoint (`modeltap shell`) | 104a | M | bundled, see above |
| 106  | Plumbing cleanup (delete + refactor passes) | 104b, 105 | M | bundled, see above |
| 107  | WU-102 SC3 follow-up: viewport-state accessor | — | S | [`designs/2026-04-27-design-viewport-state-accessor-107.md`](designs/2026-04-27-design-viewport-state-accessor-107.md) |

WU-104a / WU-104b / WU-104c / WU-105 / WU-106 share a contract
surface (the `Runtime` impl, the production CLI, and the cleanup
that follows). Per `.agents/process.md` §"Design Artifact Placement"
("Bundle related WUs that share a contract surface"), they share one
design document.

WU-107 is independent — it adds a viewport-state accessor on
`harnessshell.Model` and a parity test that uses it; no Runtime or
CLI surface area is touched.

**Critical path:** 103 → 104a → 104b → 104c → 106. WU-105 starts
in parallel with WU-104b once WU-104a lands. WU-107 is independent.

### Phase 1 design checklist

Phase 1 is complete when every WU has a design doc under
`docs/releases/v0.2.2/designs/`:

- ✅ WU-103: `2026-04-27-design-harness-audit-103.md`
- ✅ WU-104a + WU-104b + WU-104c + WU-105 + WU-106:
  `2026-04-27-design-production-wiring-104-106.md`
- ✅ WU-107: `2026-04-27-design-viewport-state-accessor-107.md`

## Risk register

- **R1 — plumbing turns out wholly broken.** If WU-103 audit's
  optimism doesn't survive contact with WU-104 implementation, the
  remaining 104 slices expand to "build a fresh impl from scratch
  with new BFF connection / protocol / tool / context layers." That
  is a much larger scope; the release may slip or scope down (e.g.,
  ship without MCP). Mitigation: the WU-104 split (104a / b / c)
  and the WU-105 dependency tightening to 104a-only (per Codex #1)
  mean a partial impl can ship as soon as `SubmitTurn` works; b and
  c can fall back to documented not-implemented errors if needed.
- **R2 — partial salvage with awkward shape.** Some plumbing might
  compile but couple awkwardly to the deleted App's `tea.Msg`
  lifecycle. WU-104's design calls out the refactor seams (sync
  helpers replacing `tea.Cmd`-returning App handlers).
- **R3 — branch retarget still pending.** v0.2.1 was tagged on
  `spike/scrolling-surface-eval`. Before tagging v0.2.2 the branch
  should retarget; this is a TPM decision not blocking code work.
- **R4 — interrupt RPC.** Resolved by Phase 2 review (Codex #4):
  the existing `ProtocolClient.CancelTurn` (against
  `protocol.MethodTurnCancel`) is the production interrupt
  channel. WU-104b uses it. If `CancelTurn` returns an error, the
  runtime synthesizes `harnessshell.RunStoppedEvent` (per Kimi #7)
  rather than `RunFailedEvent` so the UX preserves clean stop
  semantics. No server-side change required.
- **R5 — MCP autostart side effects.** `mcp.go` orchestrates external
  MCP processes. WU-104's design specifies when in the new Runtime's
  lifecycle MCP starts; if the Runtime construction is too early,
  bare `shell-demo` (which doesn't need MCP) starts spending wallclock
  on MCP boot. Mitigation: lazy MCP start triggered only by
  `DispatchCommand("/mcp ...")` or the first MCP-namespaced tool
  call.

## Definition of done

v0.2.2 is complete when:

1. The audit doc exists and every `internal/harness/` file is
   categorized.
2. A production `Runtime` impl ships in `internal/harnesshost` (or a
   sibling package) backed by salvaged plumbing.
3. A new production CLI entrypoint is wired and works against a real
   BFF socket end-to-end.
4. Files categorized as delete in the audit are gone.
5. WU-102 SC3 has a real automated assertion.
6. Release tagged on a release branch (TPM decision pending).

## Current phase

**Phase 2 — Review (complete).** Codex and Kimi reviews dispositioned
2026-04-27; design docs revised to address every blocking and
significant finding. Phase 2 closure is an explicit `ADMIN:` commit;
Phase 3 implementation begins with WU-104a.
