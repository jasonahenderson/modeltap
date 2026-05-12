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
| F1 | Production wiring never starts provider health checks; Ollama discovery never runs and built-ins are forever flagged "unavailable" | Blocking | **Fixed in PATCH-0021** |
| F2 | `state.status` field in `internal/harnessshell` is written everywhere and never read by the renderer; every `HostStatusEvent` is silently a no-op, hiding output for `/models`, `/sessions`, `/runs`, `/context`, `/history`, `/mcp` | Blocking | **Fixed in PATCH-0018** (HostInfoEvent + RenderInput.Status wiring) |
| F3 | Cloud provider health check probes `http://127.0.0.1:8080` (the local proxy) instead of the upstream, so Anthropic/OpenAI report "unavailable" even with valid keys | Blocking | **Fixed in PATCH-0025** (probe Upstream not Host) |
| F4 | The BFF daemon ↔ TUI shell lifecycle is fragile: auto-spawned daemon stdio is nilled to `/dev/null`, stale daemons silently get reused, sockets aren't reliably cleaned up, `modeltap status` doesn't probe the running daemon, and manual daemon + shell coordination requires two terminals | High | **Sub-item 10b fixed in PATCH-0026** (`MODELTAP_DAEMON_LOG` / `--debug-daemon-log`); 10c already addressed by `Server.startSocketListener` + PATCH-0021; 10a / 10d / 10e / 10f deferred to v0.3.1 |
| F5 | `modeltap logs`, `show`, `export`, `metrics` all return "no store configured" — the `SetXxxStore` test-injection setters exist on each command but no production code path calls them, so every traffic-inspection command in v0.3.0 is non-functional | Blocking | **Fixed in PATCH-0019** (lazy-open + shared store post-PATCH-0020 rename) |
| F6 | `modeltap requests show <prefix>` claims to accept short-id prefixes but `storage.GetRequest` does an exact `WHERE id = ?` lookup; help text is misleading and short prefixes always return "request not found" | Medium | **Fixed in PATCH-0024** (storage prefix-fallback with ambiguity detection) |
| F7 | Every `turn.submit` against Anthropic returns HTTP 400 because the BFF sends `"max_tokens": 0` on the wire (DispatchOpts.MaxTokens left at Go's zero value) | Blocking | **Fixed in PATCH-0022** |
| F8 | `/models` (and likely other slash commands) typed in the production shell are submitted as user content in a turn.submit instead of being intercepted by the shell as host commands and dispatched via RunHostCommandAction | Blocking | **Fixed in PATCH-0023** (host-command dispatch in `emitSubmitOnEnter`) |
| F9 | Composer footer advertises `Ctrl+B sidebar  Ctrl+T agents  Ctrl+K palette` and unconditionally renders `0 background agents running`, but none of those surfaces are wired in the post-WU-100 architecture. The spike (`internal/harnessspike`) had them; WU-100 dropped them as out-of-scope for the reusable shell and the host never re-implemented them | Medium | **Fixed in PATCH-0027** (footer cleanup); real implementation tracked under FEAT-0024 (Shell UX Chrome) for v0.3.x |
| F10 | Session-scoped slash commands (`/model X`, `/context`, `/sessions clear`) fail with `-32602 session_id is required` when typed before the user submits their first turn — sessions were only auto-created on `turn.submit` or rehydrated by `session.resume`. The user has no path to switch model first and then submit | Blocking | **Fixed in PATCH-0028** (`session.create` RPC + harness auto-call on `ConnStateReady`) |
| F11 | After PATCH-0028, the bootstrap goroutine racing a fast user turn could overwrite a turn-assigned session id, leaving subsequent turns on a different conversation than the harness sequence counter expected. Manifested as `-32602 turn.submit sequence 2 does not follow current 0` on the second turn after a fresh shell launch | Blocking | **Fixed in PATCH-0029** (re-check sessionID after `session.create` RPC; existing id wins) |
| F12 | Transcript text (assistant output, run ids, session ids, command output via HostInfoEvent) cannot be selected with the mouse because `tea.WithMouseAllMotion()` captures all mouse events, suppressing the terminal's native click-drag selection. Surfaced when the user tried to copy a `run-...` id from `/runs` output for use in `/run <run-id>` / `/attach <run-id>` and had no way to do so without retyping by hand | Medium | **Fixed in PATCH-0030** (terminal-native selection is the default; `/select` toggles optional mouse-scroll capture via `tea.EnableMouseAllMotion` / `tea.DisableMouse`) |
| F13 | After PATCH-0029 (which fixed the session-race form of the same error), the second user turn in a clean session still fails with `-32602 turn.submit sequence 2 does not follow current 2`. Root cause: the BFF's `Conversation.sequence` counter increments on every message (user and assistant) while the harness's `runtimeState.NextSequence` counts only user submits, so after one round-trip the BFF expects `sequence=3` but the harness sends `sequence=2`. Undetected because the harness's BFF stub accepts any sequence without validating | Blocking | **Fixed in PATCH-0031** (split `userSequence` from per-message storage `sequence`; wire validation uses `userSequence`) |
| F14 | Streaming a turn against a local Ollama-backed model shows the assistant chrome ("working" → "done", Cost: $0.0000) but no response text; the persisted assistant turn has empty content. Root cause: `internal/bff/streaming.go` wraps every provider stream in `SSEParser.Next`, which only returns lines prefixed with `data:`; Ollama's `/api/chat` returns NDJSON (one bare JSON object per line) so every chunk is silently dropped before `provider.ParseStreamEvent` is called. The Ollama adapter comment anticipated this hand-off but it was never wired | Blocking | **Fixed in PATCH-0032** (SSEParser recognizes lines starting with `{` as NDJSON payloads; provider-agnostic prefix detection) |
| F15 | Every slash command that hits a BFF error surfaces the raw JSON-RPC framing to the user — e.g., `/attach <terminal-run-id>` shows `run.attach failed: rpc error -32602: cannot attach terminal run`. The BFF rejection itself is correct per spec (smoke step 10 explicitly accepts terminal-run rejection), but the `rpc error -%d:` formatting from `harness.RPCError.Error()` leaks through `production_runtime.statusError`. Also: the user has no hint that completed runs can still be inspected via `/run <id>` | Medium | **Fixed in PATCH-0033** (unwrap `*harness.RPCError` in `statusError`; friendlier message for terminal-run reject pointing at `/run <id>`) |
| F16 | After PATCH-0030 made terminal-native selection the default, there is no scroll path from the default input-focused state — mouse wheel is off until the user types `/select`, and arrow keys in the composer are consumed for history recall before reaching the transcript viewport. The footer says "Tab focus" but doesn't advertise that Tab switches into a scroll mode. Surfaced when the user could not scroll up to inspect earlier transcript content | Medium | **Fixed in PATCH-0034** (PgUp/PgDn and Alt+Up/Alt+Down scroll the transcript regardless of focus; footer hint advertises the shortcut) |
| F17 | The shell's "what's happening" status during a streaming turn is uninformative — it shows `Submitted` then a flat `Working` / `Done` with no elapsed time, no running token count, no current stage or active tool. Modern AI TUIs (OpenCode, Claude Code, Codex) surface a structured streaming-status line (verb, elapsed time, token counters, interrupt hint). The shell already receives the underlying events (StreamCompleteMsg, CostUpdateMsg, ToolActivityMsg, stage transitions) but the projection layer collapses each into a flat `HostStatusEvent` string that overwrites the previous one | Medium | **v0.3.0 placeholder fixed in PATCH-0035** (track run StartedAt + 1Hz ticker → "Streaming response (Ns)"). Proper structured `StreamingStatus` surface (verb / stage / running tokens / active tool / interrupt hint) tracked under FEAT-0024 for v0.3.1 |
| F18 | After a `/runs` (or any host slash command) returns, the status footer keeps showing "Running /runs" indefinitely — `emitSubmitOnEnter` sets the status on dispatch but no completion event clears it. Affects every host command (/models, /sessions, /run, /runs, /context, /history, /mcp, /attach, /detach, /cancel) | Medium | Deferred to FEAT-0024 / v0.3.1 (proper structured-status surface) |
| F19 | Footer reads "PgUp/Dn scroll" but Mac laptops without dedicated PgUp keys have to know `fn+↑/↓` = PgUp. PATCH-0034's scroll path works, but discoverability for Mac users is poor; Option key is also commonly remapped to send special characters in some terminals so Alt+Up doesn't always work either | Medium | Deferred to FEAT-0024 / v0.3.1 (platform-aware footer hints + Ctrl+↑/↓ universal scroll) |
| F20 | Slash commands typed during a streaming run get enqueued as user content instead of dispatched as host commands. Root cause: `emitSubmitOnEnter` (queue.go:141) checks `s.streaming` BEFORE the slash-command dispatch branch (queue.go:170), so `/cancel`, `/run`, `/runs`, `/detach`, etc. all hit the streaming-queue branch first. Means there is no working `/cancel` path during a long run (Esc Esc is the only working interrupt). Surfaced when smoke step 11's `/cancel <run-id>` was queued instead of cancelling | Blocking | **Fixed in PATCH-0036** (slash-prefix dispatch precedes the streaming-queue branch; regression tests cover slash-during-streaming dispatch and non-slash content still enqueues) |
| F21 | `/run` (no args) returns the active run id, but `/runs` doesn't list that run — only earlier runs appear. Root cause investigated under F23: each shell launch auto-creates a new session via PATCH-0028, so runs from prior shell invocations live on older session ids; `/run <id>` calls `run.details` which doesn't filter by session, but `/runs` filters to the harness's current session id | Medium | Resolved by F23 fix in PATCH-0038 (auto-resume most-recent session on launch instead of always creating a new one; `/sessions current` surfaces active id; orphan-session UX cleaned up) |
| F22 | No `/help` command. The host slash-command surface (modes, model, session, context, run, attach/detach/cancel, retry/continue/fork, shell-native) is undocumented in the binary; users have to read source or trial-and-error. Surfaced when the user tried to discover what `/sessions` could do | Medium | **Fixed in PATCH-0037** (`/help` lists the host slash-command surface; `/help <cmd>` deferred to FEAT-0024 for per-command argument detail) |
| F23 | Sessions accumulate across shell launches: every `modeltap shell` connect creates a fresh session via `session.create` (PATCH-0028 + bootstrapSession), so the DB collects N sessions over a debugging cycle and `/sessions list` returns "a bunch". Modeltap's `/clear` is also semantically wrong vs. user expectations from Claude Code — it only wipes the shell's transcript display, leaving the BFF's working memory intact, so the model keeps responding as if it remembers | Medium | **Fixed in PATCH-0038** (redefine `/clear` as new-session per Claude Code semantics; shell auto-resumes most-recent active session on launch; welcome message hints `/clear for new conversation`; `/sessions current`, `/sessions delete <id>`, `/sessions prune` round out the management surface) |

All twelve smoke-test findings are addressed in `release/v0.3.0`:

- **Release-blocking, fixed:** F1, F2, F3, F5, F7, F8, F10, F11, F13, F14, F20
- **Medium, fixed:** F6, F9 (chrome-truth fix; real chrome
  tracked under FEAT-0024), F12, F15, F16, F17 (placeholder; real
  structured-status surface tracked under FEAT-0024), F21 (resolved
  by F23 fix), F22, F23
- **Medium, deferred to FEAT-0024 / v0.3.1:** F18, F19
- **High, partially fixed:** F4 — sub-item 10b shipped in
  PATCH-0026; 10c was already addressed by existing wiring
  (PATCH-0021); 10a / 10d / 10e / 10f deferred to v0.3.1 with
  retrospective Recommendation 10 carrying the design notes.

The patch sweep that closed these findings lands as PATCH-0018
through PATCH-0028 plus the supporting `ADMIN:` retrospective and
process-doc commits, and queues FEAT-0024 (draft) for v0.3.x. Smoke
verification against the rebuilt binary still required to confirm
end-to-end behavior.

The defects in this release split cleanly into two failure modes:

- **Production wiring forgotten** (F1, F5, and the renderer-side
  half of F2): the code exists, has unit tests, and is correct in
  isolation — but the production constructor or projection path
  never calls it.
- **Wire-format / contract defects** (F7, F8, and arguably F3 and
  F6): the code reaches the wire, but with wrong shape or wrong
  target.

Both classes are reachable by a binary-launch implementation
review (Recommendation 2) plus production-wiring coverage tests
(Recommendation 3). Neither was reachable by the unit-scoped
test suite that v0.3.0 shipped with.

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
