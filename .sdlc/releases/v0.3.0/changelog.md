# v0.3.0 Changelog

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Status:** Phase 3 implementation complete; pending release close

v0.3.0 ships the run runtime foundation for the Professional Harness Runtime
series.

Implemented scope:

- accepted run-runtime ADR
- durable run IDs and lifecycle metadata
- BFF run registry and run event stream
- existing foreground chat/codegen represented as lightweight runs
- `/run` active-run inspection
- `/runs` or `/jobs` run list
- attach/detach/cancel/reconnect semantics for BFF-known runs

Implemented so far:

- SQLite schema version 3 stores durable runs, run events, checkpoints,
  attachment state, run-turn links, model-call accounting, and tool-result
  accounting.
- `turn.submit` now returns an optional `run_id` and creates a foreground run
  before provider dispatch.
- BFF run handlers expose run create/list/details/events and basic
  attach/detach/cancel/retry/continue/fork surfaces.
- Provider stream completion updates run lifecycle state and token/cost/model
  metadata.
- Legacy turn stream events now carry optional `run_id` correlation so the
  existing foreground transcript can consume durable BFF run IDs without
  losing token deltas.
- The production harness routes `/run`, `/runs`/`/jobs`, `/attach`, `/detach`,
  `/cancel`, `/retry`, `/continue`, and `/fork` to run-native RPC methods.
- Attach conflicts are rejected when another connection owns the run, and
  detach clears the attachment lease.
- Run event replay includes event type metadata, token-delta progress events,
  terminal projection, and `run.permissions` blocker details when a stored
  `run.blocked` event is available.
- Detached run deltas are buffered outside the foreground transcript and replay
  into an explicit attached-run row when the user attaches the run.
- Replay gaps report summary fidelity with checkpoint fallback metadata instead
  of failing the request.
- Implementation-review hardening adds transactional foreground run/turn
  persistence, relay-error failure signaling, stateful `run.heartbeat`
  liveness, bounded in-memory run/turn registries, and focused regression
  coverage for idempotency and durable run-turn ownership.

Release readiness is recorded in `.reviews/v0.3.0-release-readiness.md`.
Publishing and tagging remain a separate release-close decision.
