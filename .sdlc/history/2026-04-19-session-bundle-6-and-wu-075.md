---
date: 2026-04-19
topic: Bundle 6 (protocol client + connection manager) and WU-075 (tool framework) catch-up log
---

# 2026-04-19 — Session: Bundle 6 + WU-075

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Topic

Catch-up log covering three commits that landed between the Bundle 5
session (2026-04-18) and today: WU-073 (`0a7a1db`), WU-074 (`9e2a97b`),
and WU-075 (`0ed8846`). The Bundle 6 ADMIN status commit (`419e3a8`)
was made at the time but no session log was written; this log closes
that gap.

With these three WUs, the harness can now:
- connect to the BFF end-to-end over Unix socket or TLS,
- drive the full FEAT-0008 lifecycle (discover → auto-start? →
  connecting → authenticating? → registering → ready) with
  heartbeat, two-stage degradation, exponential-backoff reconnect,
  and a fast disconnect path,
- translate every server notification into a Bubbletea message,
- dispatch tool calls through a permission-enforcing executor with
  three modes (Default / AcceptEdits / Autonomous) and a dangerous-
  command catalog.

What's still missing before the harness runs a real turn: the App↔
ConnectionManager wiring (the App already handles `ConnStateMsg` /
`StreamTokenMsg` / etc., but nothing instantiates the manager yet),
and the actual tool implementations (WU-076–079).

## Work Completed

### WU-073 — Harness JSON-RPC protocol client (`0a7a1db`)

`internal/harness/client.go` (+ `client_test.go`). The harness-side
twin of WU-046's server transport.

- `Dial(ctx, DialOptions)` connects via Unix socket **or** TLS
  (mutually exclusive) and spins up a read loop in a dedicated
  goroutine.
- `Call(ctx, method, params)` correlates concurrent requests via a
  pending-map keyed by string id. `CallInto` adds typed unmarshal.
- Read loop demuxes `{id + result|error}` → pending channel,
  `{method, no id}` → `EventHandler`. Malformed frames are dropped
  and the loop keeps running.
- `*RPCError` typed error so callers can `IsRPCError` and branch
  on `Code`.
- `Done()` closes when the loop exits; `Err()` carries the cause
  (nil on clean `Close` / EOF).
- Typed helpers per design D2.5: `SubmitTurn`, `CancelTurn`,
  `SendToolResult`, `Register`, `Ping`, `Health`, `SessionResume`,
  `SessionList`, `ContentTransform`.
- No public `Notify` — FEAT-0008 mandates harness→server frames
  carry an id.
- `writeMu` serializes outbound frames; `Call`/`Close` are
  concurrency-safe.

Tests use a `net.Pipe`-backed mock server with scripted per-method
handlers. Coverage: happy-path Call, RPC error, context timeout,
concurrent correlation, notification dispatch, EOF closes Done,
Close stops loop, typed helpers, `CallInto` decoding, dial
validation, and a real Unix-socket smoke test. Race-clean.

### WU-074 — Connection manager (`9e2a97b`)

`internal/harness/connection.go` (+ `connection_test.go`).

- 9-state machine mirroring WU-048's server states, encoded as the
  string constants already in `model.go`. Transitions validated
  against `validHarnessTransitions`; invalid edges silently dropped
  for runtime resilience.
- `ConnectSync` drives the full discover → (auto-start?) →
  connecting → (authenticating?) → registering → ready flow inline
  and returns the terminal error or nil. `Connect` / `Reconnect`
  return `tea.Cmd` wrappers.
- **Auto-start**: when the socket isn't reachable and `AutoStart`
  is on, launches `ServerBinary` (default `modeltap start`),
  detaches it so the server outlives the harness, and polls the
  socket every 200ms up to `StartTimeout`. A `startServerFn` hook
  lets tests inject a fake launch.
- **Heartbeat** per FEAT-0008: ticker at `HeartbeatInterval`,
  Ping-error increments `missedPongs`.
  - 3 missed → `ConnStateDegraded`
  - 5 missed → `ConnStateReconnecting` (stops heartbeat, triggers
    reconnect loop)
  - Successful pong resets the counter; degraded → ready on fresh
    pongs.
- **Fast disconnect**: a watcher goroutine on `client.Done()`
  transitions to Reconnecting immediately rather than waiting for
  missed-pong timing.
- **Reconnect**: exponential backoff with ±20% jitter, capped at
  `ReconnectMax`, up to `ReconnectMaxRetries`. Each attempt re-runs
  `ConnectSync`.
- **Event bridge** (`HandleEvent`): `token.delta` → `StreamTokenMsg`,
  `turn.complete` → `StreamCompleteMsg`, `tool.call` → `ToolCallMsg`,
  `cost.update` → `CostUpdateMsg`, `model.selected` →
  `ModelUpdateMsg`, `branch.*` → `Branch{Started,Complete,Error}Msg`,
  `compact.suggest` → `BannerMsg`, `status.update` →
  `StatusUpdateMsg`. Unknown methods dropped.

`ProgramSender` interface lets tests inject a recording fake
instead of a real `*tea.Program`.

Tests cover: transition table (allowed + disallowed), Send delivery
on transition, no-op invalid transitions, backoff bounded + grows
+ caps, every event-bridge method, unknown-method drop, nil-sender
safety, end-to-end `ConnectSync` against a mock BFF Unix-socket
listener, auto-start disabled fail, auto-start hook invocation,
`Disconnect` idempotence and post-disconnect reject, heartbeat
3→degraded→5→reconnecting, degraded → ready on successful pong.
Race-clean.

### WU-075 — Tool framework + permission model (`0ed8846`)

`internal/harness/tools/` with `framework.go`, `permission.go`,
`dangerous.go`, `tracker.go` (+ tests).

**Framework primitives:**
- `Tool` interface: `Name` / `Description` / `InputSchema` /
  `OutputEnvelope` / `RiskLevel` / `Execute`. Implementations land
  in WU-076..WU-079.
- `RiskLevel` typed string with the four wire-legal values
  (`read_only` / `write` / `execute` / `destructive`). `IsValid()`
  pinned by tests so a future tool can't slip an unknown level
  into the `capabilities.register` catalog.
- `Registry` with `Register` / `Get` / `Names` / `All`. Insertion
  order preserved. Duplicate registration panics; invalid risk
  panics. `All()` returns `[]protocol.ToolDefinition` ready for
  `capabilities.register`.
- `ToolExecResult` with `Status` (success / rejected / error) plus
  `Output`, `OutputType`, `Error`, `Reason`. `ToProtocol(toolCallID)`
  converts to wire shape; `SuccessResult` / `RejectedResult` /
  `ErrorResult` are convenience constructors.
- `Executor.Execute(ctx, name, input)`:
  1. Registry lookup → `ErrToolNotFound` on miss.
  2. Permission gate via `PermissionEnforcer.Check`.
  3. `PromptCallback` if Check returned `PermPrompt`; nil callback
     means deny.
  4. Approve the tool name on first user-confirmed prompt; repeats
     in the session don't re-prompt.

**Permission model (design D3):**
- `PermissionLevel` = `Default` / `AcceptEdits` / `Autonomous`.
- Decision matrix: `read_only` always allowed; `write` allowed in
  AcceptEdits/Autonomous, prompts in Default; `execute` prompts
  except in Autonomous; `destructive` always prompts even
  Autonomous.
- `alwaysPrompt` handles input-dependent risk: Bash → `IsDangerous`,
  Git → `IsDangerousGit`, WebFetch → absolute block on internal
  network targets (loopback / RFC1918 / link-local — SSRF
  prevention per WU-094).
- Per-tool session approval (`Approve` / `IsApproved`) and per-
  domain approval (`ApproveDomain` / `IsDomainApproved`) for
  WebFetch.

**Dangerous-command catalog (design D4):**
- Bash: `rm -rf` / `-fR` / `-Rf`, `> /dev/`, `chmod 777`,
  `chown -R`, `mkfs`, `dd`, `curl -d`, `wget`, `LD_PRELOAD` /
  `PATH` exports. Case-insensitive flag matching covers flag-order
  variants.
- Git: `push --force` / `-f`, `reset --hard`, `clean -f`, `branch
  -D`, `checkout -- .`.

**FileTracker (design D5):** records canonical absolute paths read
in this session so Edit / Write tools (WU-077) can enforce the
"Read before mutate" contract.

Tests cover: RiskLevel validity, registry duplicate/invalid panics,
insertion-order preservation, `ToProtocol` round-trip, executor
not-found / read-only allow / prompt-no-callback / prompt-approved
remember / prompt-denied / autonomous-allow-execute / destructive-
always-prompts; permission matrix at every level for all four
risk tiers; bash dangerous catalog table; git dangerous catalog
table; first-use approval memory; WebFetch domain approval;
WebFetch internal-URL deny; FileTracker canonicalization.
Race-clean.

## Files Created or Modified

Created under `internal/harness/`:
- `client.go`, `client_test.go`
- `connection.go`, `connection_test.go`

Created under `internal/harness/tools/`:
- `framework.go`, `framework_test.go`
- `permission.go`, `permission_test.go`
- `dangerous.go`
- `tracker.go`

Modified:
- `.sdlc/releases/v0.2.0/status.md` — WU-073/074 completion + Bundle 6
  complete note (commit `419e3a8`).

## Bundle Status After This Session

| Bundle | Status | Note |
|---|---|---|
| 4 (BFF Foundation) | 4/4 | complete |
| 5 (Bubbletea scaffold) | 5/5 | complete |
| 6 (Protocol client + Manager) | 2/2 | complete |
| 7 (Tools) | 1/5 | WU-075 framework done; WU-076–079 pending |
| 8 (Sessions & Conversation) | 3/3 | complete |
| 9 (Model Config & Routing) | 3/4 | WU-060 deferred → FEAT-0013 |
| 10 (Streaming, Prompts, Cost) | 4/4 | complete |
| 11 (Context, Diagnostics, Recovery) | 3/4 | WU-061 compaction pending |

Plus auxiliary: WU-091 history handlers, WU-066 Ollama, WU-065 CLI
wiring, the `handleTurnSubmit` pipeline.

## Notes / Decisions

- **Harness-side state strings reuse `model.go` constants.** No new
  enum type in `connection.go` — the states are already stringly-
  typed in the App model and the state machine uses the same
  values. Keeps wire traffic, UI rendering, and transition
  validation aligned.
- **Invalid transitions are silently dropped**, not errored. The
  reasoning: a heartbeat tick arriving after disconnect shouldn't
  crash the runtime. Tests pin the accepted edges; anything not
  on the table is a no-op.
- **Auto-start detaches the server.** The subprocess is released
  (`cmd.Process.Release()`) so it survives harness exit. This
  matches the FEAT-0008 "server outlives harness" expectation and
  lets `modeltap harness` reconnect to an already-running daemon.
- **Prompt approval is per-session, not persisted.** First-use
  approval memory lives in the `Executor` for the life of the
  harness process. Persistent policy is a future WU (not in
  v0.2.0 scope).
- **SSRF prevention in WebFetch is allow-by-default-block-private.**
  The permission enforcer blocks loopback / RFC1918 / link-local
  absolutely — these are never allowed regardless of mode. Public
  URLs prompt in Default, allow in AcceptEdits/Autonomous unless
  the domain is pre-approved.

## Next / Open Items

Bundle 7 remaining, all depend only on WU-075 and parallelize:
- **WU-076** Read tools (text / PDF / DOCX / image / spreadsheet) — Large
- **WU-077** Write + Edit — Medium (FileTracker already in place)
- **WU-078** Bash + Git — Medium (dangerous catalogs already in
  `dangerous.go`)
- **WU-079** Glob / Grep / WebSearch / WebFetch — Medium (WebFetch
  private-net block already in permission enforcer)

Other open items:
- **WU-067** BFF integration tests — much easier now that
  `ProtocolClient` exists as a test driver.
- **WU-087** harness integration tests — easier now that
  `ConnectionManager` + `ProtocolClient` exist.
- **WU-061** compaction — still waiting on trim-heuristic +
  harness-UX design pass.
- **App↔ConnectionManager wiring** — the App already handles the
  event-bridge messages; nothing yet instantiates the manager on
  startup. A small WU or part of Bundle 15 integration.
