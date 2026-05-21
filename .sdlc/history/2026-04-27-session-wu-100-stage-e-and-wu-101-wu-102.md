# 2026-04-27 — Session: WU-100 Stage E + WU-102 + WU-101 reconciliation

## Scope

Continuation of the long v0.2.1 implementation marathon. Closed Stage
E of WU-100 (extract `internal/harnessdemo`, delete spike, replace
CLI), wrote the WU-102 parity sweep, and reconciled the WU-101
docs against the implemented names. Only Stage D-4 (production
wiring against `internal/harness/app_conn.ConnSurface`) remains to
fully close out v0.2.1's WU-100 work.

## Commits landed

```
f80661a WU-100: Stage E-1 — internal/harnessdemo package with FakeRuntime
f8aa1cc WU-100: Stage E-2 — modeltap shell-demo CLI entrypoint
88e91ba WU-100: Stage E-3 — delete internal/harnessspike and harness-spike CLI
1f1e5b9 WU-102: parity and regression sweep
23dff2d WU-101: reconciliation pass — strip provisional placeholders
```

## Capability per commit

### Stage E-1 (`f80661a`) — `internal/harnessdemo`

- `FakeRuntime` implements `harnesshost.Runtime` with synthetic
  `SubmitTurn`, `InterruptRun`, `ResolvePermission`, `LoadPreview`,
  pass-through `DispatchCommand`/`SummarizePaste`. SubmitTurn pre-
  splits the fake reply into per-word chunks and registers the run
  for the Driver to tick out. `/perm` prefixes short-circuit into a
  paused permission-demo state with no chunks until
  ResolvePermission lifts the gate.
- `Driver` is a `tea.Model` that wraps `harnesshost.Adapter` and
  orchestrates fake stream emission. Update intercepts streamTickMsg
  to emit `harness.StreamTokenMsg` / `harness.StreamCompleteMsg` /
  `harness.PermissionPromptMsg` (which the adapter projects to
  HostEvents), and after every adapter Update drains
  FakeRuntime.TakeUnstartedRuns to schedule ticks for newly-eligible
  runs.
- Six tests cover Runtime-impl invariants + a Driver dispatch smoke
  test.

### Stage E-2 (`f8aa1cc`) — `modeltap shell-demo` CLI

- `internal/cli/shell_demo.go` registers the new subcommand. RunE
  constructs `harnessshell.New` (with label and demo placeholder),
  `harnessdemo.New` (FakeRuntime), and `harnessdemo.NewDriver`
  wrapping the two through `harnesshost.Adapter`.
- `internal/cli/root_test.go` updated for the new command.
- The new command coexists with the legacy `harness-spike` for
  side-by-side comparison until Stage E-3 deletes the legacy.

### Stage E-3 (`88e91ba`) — spike deletion

- `internal/harnessspike/{app.go, app_test.go, styles.go}` deleted
  (~3,400 lines).
- `internal/cli/harness_spike.go` deleted.
- `newHarnessSpikeCommand` removed from root.go and root_test.go.
- Spike-only chrome (sidebar, command palette, agent overlays,
  background-agent state, presets) does NOT survive the deletion —
  per WU-100 §"Definite scope rule" those surfaces were spike
  experiments outside the FEAT-0014 conversation-shell contract.
  They may resurface in a future modeltap top-level harness package
  but are not WU-100 work.

### WU-102 (`1f1e5b9`) — parity sweep

- One shell-side gap closed: transcript Enter on a token now
  activates it (paste tokens toggle `Expanded`; file/reference
  tokens emit `LoadPreviewAction` with Source="transcript").
  Mirrors the spike's `openSelectedTranscriptRef` behavior to
  satisfy FEAT-0014's "transcript Enter toggles paste-token
  expansion inline" parity invariant.
- Layer 1 parity tests added in `internal/harnessshell/queue_test.go`
  (multi-item FIFO, queue-survives-interrupt-then-empty-Enter-
  releases verbatim FEAT-0014 SC4, queued-submission preserves
  text+tokens) and `internal/harnessshell/tokens_test.go` (large
  paste capture, dropped-path detection, slash-command not
  classified as path, submitted paste starts expanded, transcript-
  Enter toggle, transcript-Enter file-preview emission).
- Layer 2 host adapter tests already covered by Stage D-1/D-2/D-3
  commits.
- Layer 3 integration tests added in
  `internal/harnesshost/integration_test.go` (submit-stream-complete
  pipeline correlates IDs end-to-end, mid-stream permission pause
  buffer integration with full pumpAdapter cmd-chain chasing).

### WU-101 reconciliation (`23dff2d`)

- 85 provisional placeholders removed across 3 docs.
- `internal/harnessshell/README.md`: status, embedding example
  (now uses `harnesshost.New(shell, runtime)`), action/event
  envelope explicitly documented, `<!-- provisional ... -->`
  markers gone.
- `internal/harnesshost/README.md`: status reflects feature-complete
  adapter sans Stage D-4 production wiring; action/operation table
  updated with actual error-fallback events; runtime/shell event
  table covers the full projection matrix; mid-stream pause section
  reflects the actual `Adapter.forwardEvent` shape.
- `docs/guides/harness-shell-embedding.md`: minimal embedding
  example rewritten to use `harnesshost.Adapter`, both
  Adapter-wrapped and manual-ActionMsg patterns documented; flow
  walkthroughs use concrete type/enum names; adapter dispatch and
  buffered-pause sketches mirror the actual implementation; demo
  CLI section added; reconciliation table replaced with the final-
  names table with no remaining placeholders.

## Cumulative progress in this very long session

This session (started 2026-04-26 picking up uncommitted Stage C
scaffolding) landed 22 implementation commits and 4 admin/log
commits. Net delta:

```
e331ae7 ADMIN: log WU-100 Stage C completion
056d7c6 ADMIN: log WU-100 Stage C checkpoint
5ef453d ADMIN: log WU-100 Stage D adapter completion
+ 19 implementation commits across:
  - WU-100 Stages C-3 through E-3 (Stage A/B/C wire-up + actions +
    intake + adapter + harnessdemo + spike deletion)
  - WU-101 reconciliation
  - WU-102 parity sweep
```

The post-extraction architecture is now structurally complete:
`internal/harnessshell` (reusable shell) + `internal/harnesshost`
(modeltap host adapter) + `internal/harnessdemo` (fake runtime +
shell-demo CLI). Spike is gone. Only Stage D-4 production wiring
remains.

## What's left for v0.2.1 release

- **WU-100 Stage D-4 — production wiring.** Requires two
  architectural decisions before implementation: App composition
  (replace conversation surface in `internal/harness/app.go` vs
  parallel entrypoint) and `Runtime.SubmitTurn` async/sync bridging.
  Captured in `status.md` "Up next".
- **Branch retarget** for `spike/scrolling-surface-eval` to a
  release branch before tagging.
- **Optional WU-102 SC3 follow-up.** Manual scroll preservation has
  no direct automated assertion because the viewport state lives
  inside `Model.View`'s local copy. A test-only accessor would
  close the gap; not blocking release.

## Open items

- See `.sdlc/releases/v0.2.1/status.md` "Open items" for the full
  list.
- Pre-existing test failures (`internal/cli` config-show /
  dashboard tests) remain unchanged and unrelated to WU-100 work;
  documented in the Stage B commit message originally.
