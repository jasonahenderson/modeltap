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

| WU | Title | Dependencies | Size | Phase 1 design |
|----|-------|-------------|------|----------------|
| 103 | `internal/harness` audit and salvage report | — | M | [`designs/2026-04-27-design-harness-audit-103.md`](designs/2026-04-27-design-harness-audit-103.md) |
| 104 | Concrete `harnesshost.Runtime` implementation | 103 | L | bundled with 105 + 106 in [`designs/2026-04-27-design-production-wiring-104-106.md`](designs/2026-04-27-design-production-wiring-104-106.md) |
| 105 | Production conversation-shell CLI entrypoint | 104 | M | bundled, see above |
| 106 | Plumbing cleanup | 104, 105 | M | bundled, see above |
| 107 | WU-102 SC3 follow-up: viewport-state accessor | — | S | [`designs/2026-04-27-design-viewport-state-accessor-107.md`](designs/2026-04-27-design-viewport-state-accessor-107.md) |

WU-104 / WU-105 / WU-106 share a contract surface (the `Runtime` impl
and the production CLI together replace the deleted legacy harness;
the cleanup that follows removes the files the audit categorized as
delete). Per `.agents/process.md` §"Design Artifact Placement"
("Bundle related WUs that share a contract surface"), they share one
design document.

WU-107 is independent — it adds a viewport-state accessor on
`harnessshell.Model` and a parity test that uses it; no Runtime or
CLI surface area is touched.

**Critical path:** 103 → 104 → 105 → 106. WU-107 runs in parallel
with the rest.

### Phase 1 design checklist

Phase 1 is complete when every WU has a design doc under
`docs/releases/v0.2.2/designs/`:

- ✅ WU-103: `2026-04-27-design-harness-audit-103.md`
- ✅ WU-104 + WU-105 + WU-106: `2026-04-27-design-production-wiring-104-106.md`
- ✅ WU-107: `2026-04-27-design-viewport-state-accessor-107.md`

## Risk register

- **R1 — plumbing turns out wholly broken.** If WU-103 audit's
  optimism doesn't survive contact with WU-104 implementation, WU-104
  expands to "build a fresh Runtime impl from scratch with new BFF
  connection / protocol / tool / context layers." That is a much
  larger scope; WU-104 likely splits into per-subsystem WUs. The
  release may slip or scope down (e.g., ship without MCP).
  Mitigation: WU-104's design explicitly identifies the order of
  Runtime methods to land (`SubmitTurn` first, foundational; others
  follow) so a partial impl can ship if the bottom of the plumbing
  is unsound.
- **R2 — partial salvage with awkward shape.** Some plumbing might
  compile but couple awkwardly to the deleted App's `tea.Msg`
  lifecycle. WU-104's design calls out the refactor seams (sync
  helpers replacing `tea.Cmd`-returning App handlers).
- **R3 — branch retarget still pending.** v0.2.1 was tagged on
  `spike/scrolling-surface-eval`. Before tagging v0.2.2 the branch
  should retarget; this is a TPM decision not blocking code work.
- **R4 — interrupt RPC may not exist on the BFF.** WU-103 audit
  flagged that `client.go`'s `ConnProtocolClient` interface didn't
  show an explicit Interrupt method during the survey. WU-104 either
  adds one (server-side change too) or surfaces
  `RunFailedEvent{Message: "interrupt unsupported"}`. If a server-
  side change is required, that bumps scope and may need an
  ADR.
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

**Phase 2 — Review (in progress).** Phase 1 closed
2026-04-27 with all three design bundles landed under
`docs/releases/v0.2.2/designs/`. Phase 2 begins with the user
deciding the review path (read directly, send to external models,
or both); reviews land under `docs/releases/v0.2.2/.reviews/`.
