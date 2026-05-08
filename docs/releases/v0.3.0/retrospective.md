# v0.3.0 — Smoke Test Retrospective

Captured 2026-05-07, immediately after the v0.3.0 manual smoke test
caught three issues that earlier review and test cycles missed.
Companion to `smoke-test.md`, `status.md`, and the handoff log
`docs/history/2026-05-07-session-v0.3.0-smoke-test.md`.

## Summary

The v0.3.0 release was marked "Phase 3 implementation complete; pending
release close" with `go test ./...` passing, an implementation review
dispositioned, and a release-readiness review concluding "ready for
user release validation". The first manual smoke test against the
shipped binary surfaced three defects:

| # | Defect | Severity | Status |
|---|---|---|---|
| F1 | Production wiring never starts provider health checks; Ollama discovery never runs and built-ins are forever flagged "unavailable" | Blocking | Fix prepared in tree (`bff_wiring.go`, `bff/server.go`); not committed |
| F2 | `state.status` field in `internal/harnessshell` is written everywhere and never read by the renderer; every `HostStatusEvent` is silently a no-op, hiding output for `/models`, `/sessions`, `/runs`, `/context`, `/history`, `/mcp` | Blocking | Open — design-level |
| F3 | Cloud provider health check probes `http://127.0.0.1:8080` (the local proxy) instead of the upstream, so Anthropic/OpenAI report "unavailable" even with valid keys | Open | Was masked by F1; surfaced when F1 was fixed in tree |
| F4 | The BFF daemon ↔ TUI shell lifecycle is fragile: auto-spawned daemon stdio is nilled to `/dev/null`, stale daemons silently get reused, sockets aren't reliably cleaned up, `modeltap status` doesn't probe the running daemon, and manual daemon + shell coordination requires two terminals | High | Open — multiple distinct fixes (see Recommendation 10) |
| F5 | `modeltap logs`, `show`, `export`, `metrics` all return "no store configured" — the `SetXxxStore` test-injection setters exist on each command but no production code path calls them, so every traffic-inspection command in v0.3.0 is non-functional | Blocking | Fix prepared as PATCH-0019 |

F1, F2, and F5 must be resolved before tagging v0.3.0. F3 may
predate v0.3.0; needs investigation before scope decision. F4 is a
class of issues observed during the smoke-test debug session
itself; not release-blocking but materially impacts operator and
contributor ergonomics.

F5 was caught while trying to inspect a captured request body
during smoke-test debug. The same shape as F1 — production wiring
forgotten — and the same root cause as the F2 dead-state-field
problem: a test-injection seam stayed in place but the production
wiring that should have used it was never written. This brings the
v0.3.0 production-wiring defect count to **three** missing-wiring
findings (F1, F2, F5), all reachable by a binary-launch checklist
in implementation review.

## What the prior cycles missed

### Implementation review was static

`.reviews/v0.3.0-implementation-review.md` produced real fixes (relay
errors, foreground-run persistence, heartbeat/stuck projection,
registry bounds) — all internal correctness work. But none of those
fixes required actually launching the binary. F1 and F2 only manifest
when the daemon is running and a user types `/models`. The review
optimized for code correctness, not user-visible behavior.

### Test suite is unit-scoped

`go test ./...` passes. The bug chain that produces F1 is
`cli/bff_wiring.go` → `bff.Server` → `ProviderRegistry` →
`ModelRegistry` → `model.list` handler → harness `CallInto` →
`HostStatusEvent` → renderer. Each link has unit tests, but no test
exercises the full chain against a live socket. Tests that need a
registry construct one directly and populate it; the production wiring
path is not tested.

### Dead-code lint would have caught F2

`state.status` is assigned in 18 places and read in zero. A standard
deadcode pass (e.g. `staticcheck -checks SA4006,U1000`,
`golang.org/x/tools/cmd/deadcode`) would flag the field. CI does not
appear to run one.

### Smoke-test phase came after sign-off

The current release plan structure (Phase 1 design / Phase 2 review /
Phase 3 implementation) does not include a smoke-test phase. The
smoke-test doc exists but runs *after* the readiness review concludes,
making the readiness review's "ready for user release validation"
verdict the de facto last gate. By definition that gate cannot catch
defects that only manifest under user operation.

## What worked

- **Handoff doc.** `docs/history/2026-05-07-session-v0.3.0-smoke-test.md`
  let a fresh session pick up debugging without re-deriving state. The
  diagnostic-suspects ranking was directly used.
- **Probe technique.** A 60-line Go program that dialed the BFF socket
  and called `model.list` directly bypassed the TUI alt-screen and
  daemon stderr → /dev/null dead zones. Confirmed the BFF was returning
  data within minutes of the diagnostic deadlock; without it, F1 vs.
  F2 would have been confounded.
- **Phased release process gave clear seams.** Being able to point at
  "the implementation review didn't catch this because…" requires
  there to be an implementation review to point at. The structure
  helps locate where each defect should have been caught.

## Process lessons

The main lesson is not "do more review." v0.3.0 already had substantial
design review, implementation review, and release-readiness review. The gap
was that the final gates proved structural correctness rather than executable
product behavior.

1. **Add release validation before readiness/release close.** The readiness
   review should consume the smoke-test result, not authorize it. A release
   should not reach "ready for user release validation" until the shipped
   binary has passed the release smoke test or the maintainer has explicitly
   accepted a documented exception.

2. **Split implementation review into static and runtime review.** Static
   conformance review remains useful for design drift, transactions, bounds,
   and internal correctness. Runtime review must build the binary, start the
   daemon, launch the production shell, run each top-level slash command, and
   observe rendered output.

3. **Require production-wiring coverage for user-visible surfaces.** New
   shell/BFF/user-facing behavior needs at least one test or scripted check
   through the production constructor path. Unit tests for each link are not
   enough when the failure mode crosses CLI wiring, daemon lifecycle, JSON-RPC,
   harness projection, and rendering.

4. **Make deadcode/static analysis a release gate.** Write-only state fields
   and unused event projections should be treated as bugs. Add staticcheck or
   an equivalent deadcode pass to CI and require a clean result before release
   close.

5. **Make observability part of daemon/TUI done criteria.** A daemonized TUI
   path needs a supported way to capture logs and probe the live JSON-RPC
   surface. Diagnostic tooling such as `modeltap bff call` and a daemon debug
   log path should be expected for future daemon/shell work, not improvised
   during smoke-test failure.

6. **Tighten readiness review wording and checklist.** A readiness review
   should distinguish "implementation structurally complete" from "release
   operationally validated." The checklist should include smoke-test status,
   binary-launch evidence, production-wiring coverage, lint/static-analysis
   status, and any accepted exceptions.

7. **Convert retrospective recommendations into tracked artifacts.** Process
   findings should become ADMIN changes to `.agents/process.md` and release
   templates. Tooling and architecture findings should become PATCH/FEAT
   artifacts rather than informal follow-ups.

## Recommendations

### Process

1. **Add a smoke-test phase to the release plan template,** before the
   readiness review. The readiness review reads the smoke-test result
   instead of authorizing it. Captured in `.agents/process.md`.

2. **Implementation review must include a binary-launch checklist.** At
   minimum: build, start daemon, launch shell, run each top-level
   slash command, observe output. A defect that survives this
   checklist is a code-review defect; a defect caught by it is a
   correctness defect — distinguishing the two is useful.

3. **Treat dead state fields as bugs.** Add a deadcode lint pass to CI
   (`make lint` or its equivalent). Failing the build on a write-only
   field would have blocked F2 from merging.

### Tooling

4. **`modeltap shell --debug-daemon-log <path>`** (or
   `MODELTAP_DAEMON_LOG=<path>`). When set, the auto-spawned daemon's
   stderr/stdout are redirected to the file instead of `/dev/null`.
   Preserves the existing TERM-corruption fix in normal use; gives
   developers a captured log without coordinating multiple terminals.

5. **`modeltap bff call <method> [params-json]`.** A diagnostic CLI
   that dials the running BFF socket, sends a single JSON-RPC
   request, and prints the response. Reusable replacement for the
   throwaway probe used in this session. Useful for smoke tests,
   scripting, support diagnostics. Patch-sized.

6. **`modeltap config show` masks secrets by default.** Currently
   prints raw API keys. Add `--show-secrets` opt-in; default to
   masked. Separate from the v0.3.0 release but worth filing.

### Architecture

7. **`HostStatusEvent` is the wrong primitive for command output.** A
   `/models` invocation produces a multi-line catalog; a `state.status`
   field (even if it were rendered) would only surface a single line in
   chrome. The shell needs a transcript-append path for host-supplied
   informational text — closer to a `HostInfoEvent` that creates a
   transcript item, not a status update. F2's fix is not "wire up
   `state.status`"; it is "introduce the missing event type and route
   command output through it".

8. **Routing policy validation.** `bff.routing.coding: claude-opus-4-7`
   was observed in the user's config; that model is not in the
   built-in catalog and will fail at use time rather than load time.
   Routing entries should be validated against the registry on Refresh
   with a stderr warning for unknown targets.

9. **Cloud health-check probe target.** `resolveProviderHost` defaults
   cloud providers to the local proxy port (`http://127.0.0.1:<port>`)
   to route through the capture pipeline. The health-check probe
   uses this same host, which means the probe is testing the local
   proxy, not the upstream. The probe should bypass the proxy
   (probe `https://api.anthropic.com` directly) or the proxy must
   correctly answer `HEAD /` with a non-5xx. F3 lives here.

10. **BFF/TUI lifecycle robustness.** F4 covers a class of fragility
    observed during the smoke-test debug session itself. Distinct
    sub-fixes, each patch-sized, none individually blocking but
    together a meaningful operator/contributor ergonomics gap:

    - **a. Stale-daemon detection on shell startup.** Before
      auto-connecting to an existing socket, probe the daemon for
      its binary hash / start time / version and warn (or refuse)
      when it doesn't match the shell binary. The current behavior
      silently reuses a stale daemon — the trap that opened this
      session.
    - **b. Daemon stderr capture during development.** Recommendation
      4 (`--debug-daemon-log`) covers the operator surface; the
      ergonomic principle is that `nil → /dev/null` is correct
      for production but should be opt-out.
    - **c. Reliable socket cleanup.** Sockets sometimes survive
      across daemon shutdowns and produce "another process is
      listening" errors on the next start. Cleanup is best-effort
      in `Server.Shutdown`; consider additional cleanup on `start`
      when the socket exists but no listener answers it.
    - **d. `modeltap status` should probe the running daemon.**
      Currently it reads config and reports providers from disk;
      it should connect to the socket, list registered endpoints
      with their live status, and report whether the daemon's
      binary matches the CLI.
    - **e. Single-terminal daemon-plus-shell mode.** `modeltap
      shell` could optionally background-run the daemon with a
      stdio capture file when no daemon is on the socket, and
      print the path on exit. Reduces the "two terminals + manual
      coordination" tax for first-time users and contributors.
    - **f. Daemon-vs-CLI binary mismatch warning.** If the
      auto-spawned daemon's `os.Args[0]` differs from the shell's
      `os.Executable()`, surface a one-time warning. Catches the
      "I rebuilt but forgot to kill the old daemon" case.

## Outstanding work

- **Decide patch/release scope for F1, F2, F3.** Options: (a) fold all
  three into a v0.3.0 pre-tag sweep, (b) tag v0.3.0 with F1 fixed only
  and ship F2/F3 as v0.3.1, (c) hold v0.3.0 until F2 has a real
  design. Recommend (c) — F2 is design-level and shipping a release
  where every slash command is silently broken devalues the release.
- **Stash or revert in-tree fix.** The `bff_wiring.go` and
  `bff/server.go` edits from this session are uncommitted on
  `release/v0.3.0`. Either commit as a pre-tag fix once scope is
  decided, or stash until the larger fix lands.
- **File the recommendations above as patches/features.** Each numbered
  item is a candidate for its own `PATCH-NNNN` or `FEAT-NNNN`. The
  process items (1–3) likely belong in `.agents/process.md` and the
  release plan template; tooling items (4–6) and architecture items
  (7–9) are patch-scoped. Recommendation 10 (F4) decomposes into
  six patch-sized sub-items (10a–10f); file each independently as
  bandwidth permits.

## Files touched this session

In tree, uncommitted on `release/v0.3.0`:

- `internal/cli/bff_wiring.go` — added `srv.Providers().StartHealthChecks(0)` before `srv.Models().Refresh()`
- `internal/bff/server.go` — `Shutdown` now calls `s.providers.Stop()`
- `.tmp/probe/main.go` — diagnostic probe (throwaway; do not commit)
- `.tmp/start.err`, `.tmp/start.out`, `.tmp/bff.err` — diagnostic output (do not commit)
- `docs/history/2026-05-07-session-v0.3.0-smoke-test.md` — already-pending handoff log from the prior session
- `docs/releases/v0.3.0/retrospective.md` — this file
