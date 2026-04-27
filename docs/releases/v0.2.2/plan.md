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

The release executes as a single track. There is no Phase 1 design
gate because the design is the WU-099 Runtime contract that already
shipped; the work is implementation-and-audit only.

Work units (provisional, may shift after the audit):

| WU | Title | Dependencies | Size | Notes |
|----|-------|-------------|------|-------|
| 103 | internal/harness audit and salvage report | — | M | Catalogs every file as keep / refactor / delete with Runtime method mapping. Deliverable is the audit doc; no code changes |
| 104 | Concrete `harnesshost.Runtime` implementation | 103 | L | Production Runtime impl backed by surviving plumbing; tests against in-memory fakes for the BFF and tool layers |
| 105 | Production conversation-shell CLI entrypoint | 104 | M | New CLI command (provisional name TBD: `modeltap shell` or `modeltap chat`) that constructs the Adapter + production Runtime and runs as tea.Program |
| 106 | Plumbing cleanup | 104, 105 | M | Deletes anything 103 categorized as delete; renames or relocates anything 103 categorized as refactor |
| 107 | WU-102 SC3 follow-up | — | S | View-side accessor for viewport state so the manual-scroll-preservation assertion lands as a real test |

**Critical path:** 103 → 104 → 105 → 106. WU-107 runs in parallel with
the rest.

## Risk register

- **R1 — plumbing turns out wholly broken.** If 103 reveals that the
  surviving `internal/harness/` files don't actually work end-to-end,
  104 expands to "build a fresh Runtime impl from scratch with new
  BFF connection / protocol / tool / context layers." That is a much
  larger scope; 104 likely splits into per-subsystem WUs. The release
  may slip or scope down to a subset of the FEAT-0014 boundary
  (e.g., ship without MCP).
- **R2 — partial salvage with awkward shape.** Some plumbing might
  compile but couple awkwardly to the deleted App's tea.Msg lifecycle.
  The audit must call this out and 104 either refactors the surviving
  code to expose Go-native APIs or wraps it in shell-of-shell layers.
- **R3 — branch retarget still pending.** v0.2.1 was tagged on
  `spike/scrolling-surface-eval`. Before tagging v0.2.2 the branch
  should retarget; this is a TPM decision not blocking code work.

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
