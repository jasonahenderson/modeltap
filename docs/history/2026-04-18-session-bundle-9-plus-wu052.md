# 2026-04-18 — Session: Bundle 9 (mostly) + WU-052

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Topic

Continuation of the "continue until complete" autonomous Phase 3 push.
Landed three of Bundle 9's four WUs (WU-057 providers, WU-058 registry,
WU-059 routing) and the WU-052 dispatch that was blocked on them.

Stopped before Bundle 10 (streaming/prompts/cost) because WU-053
requires a `ParseStreamEvent` addition to the `provider.Provider`
interface that belongs under explicit review rather than autonomous
scope expansion, and the rest of Bundle 10 compounds on it.

## Work Completed

### WU-057 — Provider endpoints + health checks

`internal/bff/providers.go`:

- `ProviderRegistry` with thread-safe endpoint map and insertion-order
  tracking (used by WU-058 for duplicate-resolution ordering).
- `ProviderEndpoint` — config plus runtime health/discovery state, own
  mutex for status updates independent of registry-level reads.
- Per-type health probes for Anthropic, OpenAI, Ollama (/api/tags),
  MLX (/v1/models). `CheckAll` runs them concurrently;
  `StartHealthChecks` runs an initial sweep then polls at configurable
  interval (default 60s) with clean `Stop`.
- `ExpandEnvAPIKey` resolves `${ENV_VAR}` wrappers; `lookupEnv` is a
  package var so tests can override.
- `MaskAPIKey` for safe logging (`sk-ant-****ABCD`).

Commit `03dd6dc`.

### WU-058 — Model registry

`internal/bff/registry.go`:

- `ModelRegistry` merges built-in catalog + discovered models + manual
  overrides. Precedence: manual > discovered > builtin (design D3.5).
- `DefaultBuiltinModels` seeds the v1 catalog (claude-opus/sonnet/haiku,
  gpt-5, o4-mini). Built-in entries only surface when a provider
  endpoint of the matching type is configured.
- Status stamp ("ready"/"unavailable") from the endpoint's current
  health, refreshed via `Refresh`.
- `Get` / `Has` / `All` / `ByProvider` accessors. `All` returns a
  deterministic sorted slice.

Commit `359dca8`.

### WU-059 — Routing policy + model.list/switch handlers

`internal/bff/routing.go`:

- `RoutingPolicy` implements the FEAT-0008 flat 3-step dot-path
  resolution: exact → category.default → root default. Not recursive.
- `ResolveForTurn` honors session override first, then routes by Mode
  as routing path (v1 simplification).
- `handleModelList` returns catalog + routing tree + current override
  for the connection's bound session.
- `handleModelSwitch` applies or clears (Model=="auto") a session
  override; unknown model → `CodeModelUnavailable` with MT-CONN-011;
  unknown session → `CodeSessionNotFound`; persisted via
  `UpdateSession`, mirrored onto `ActiveSession`, emits `model.selected`.
- `Server` grew `providers`/`models`/`routing` fields wired in
  `NewServer`; `model.list`/`model.switch` registered alongside
  existing handlers.
- `nopConn` test helper now drains the peer end so notification-
  emitting handlers don't block on pipe writes.

Commit `ee1e903`.

### WU-052 — TurnDispatcher

`internal/bff/dispatch.go` (now unblocked by WU-057):

- `TurnDispatcher` takes both `*ProviderRegistry` (endpoint config /
  health) and `*provider.Registry` (per-type adapter supplying
  `FormatMessages`).
- `Dispatch` returns the streaming `*http.Response` for WU-053 to
  relay; `DispatchSync` reads the body in full for non-streaming
  callers (content.transform in WU-062).
- Per-provider endpoint paths (`/v1/messages`, `/v1/chat/completions`,
  `/api/chat`) and auth headers (x-api-key + anthropic-version for
  Anthropic; Bearer for OpenAI/MLX; none for local Ollama).
- `dispatchError` / `formatError` / `httpError` wrap failures with
  `MT-CONN-009` diagnostics. `ErrWindowTooSmall` is tagged
  `category=budget` so the harness distinguishes a context-budget
  failure from a transport one.

Commit `4738539`.

## Files Created or Modified

Created:
- `internal/bff/providers.go`, `providers_test.go`
- `internal/bff/registry.go`, `registry_test.go`
- `internal/bff/routing.go`, `routing_test.go`
- `internal/bff/dispatch.go`, `dispatch_test.go`
- `docs/history/2026-04-18-session-bundle-9-plus-wu052.md` (this log)

Modified:
- `internal/bff/server.go` — providers/models/routing fields,
  accessors, handler registration.
- `internal/bff/server_test.go` — `nopConn` drains peer end.
- `docs/releases/v0.2.0/status.md`

## Bundle Progress Snapshot

- ✅ Bundle 4 (BFF Foundation): 4/4
- ✅ Bundle 8 (Sessions & Conversation): 3/3
- 🔶 Bundle 9 (Model Config & Routing): 3/4 — WU-060 deferred
  (depends on WU-053 streaming)

## Deferred

- **WU-060 (multi-model branching)** — depends on WU-053 streaming
  relay's per-branch per-chunk emission path.
- **Bundle 10 (streaming, prompts, cost)** — WU-053 requires adding
  `ParseStreamEvent` to the `provider.Provider` interface plus
  per-adapter implementations. That's an interface change that
  affects in-tree adapters (Anthropic, OpenAI) and belongs under
  explicit review, not autonomous scope expansion. WU-054/055 prompt
  engine and WU-056 cost tracking compose on top.

## Next / Open Items

When resuming, Bundle 10 (WU-053 streaming relay) is the natural
next step. Recommended entry point:

1. Amend the `provider.Provider` interface in `internal/provider/`
   with `ParseStreamEvent(data []byte) (*StreamEvent, error)` and a
   matching `StreamEvent` type (design D2.3).
2. Implement `ParseStreamEvent` for `AnthropicProvider` and
   `OpenAIProvider`.
3. Build `StreamRelay` in `internal/bff/streaming.go` that reads the
   `*http.Response` returned by `TurnDispatcher.Dispatch`, emits
   `token.delta`, `tool.call`, and `turn.complete` notifications,
   and appends the final assistant turn via
   `Conversation.AppendAssistantTurn` + `store.CreateTurn`.
4. Wire `handleTurnSubmit` (design D4.5 in Bundle 8 design) to the
   pipeline: validate → session ensure/create → append user turn →
   route → dispatch → relay.

Track B scaffold (Bundle 5, WU-068–072) remains parallelizable.

## Notes

- Bundle 9 tests use `httptest.Server` for provider health/dispatch
  probes. `SetHTTPClient` is the test-hook on both `ProviderRegistry`
  and `TurnDispatcher`.
- `ModelRegistry.DefaultBuiltinModels` is intentionally a small v1
  seed catalog. Full cloud catalogs change too fast to keep in-tree;
  users can supplement via manual config overrides.
- The design specifies `dispatchError` returns an `RPCError`; I used
  the existing `TransportError` type which already carries a
  JSON-RPC code + data blob and plays nicely with the transport
  layer's existing dispatch response path.
- `MaskAPIKey` treats keys of length ≤ 11 as entirely asterisked
  rather than revealing a prefix. Longer keys show the 7-char prefix
  and 4-char suffix per typical API-key conventions.
