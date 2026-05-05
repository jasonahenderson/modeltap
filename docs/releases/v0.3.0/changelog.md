# v0.3.0 Changelog

**Status:** Phase 3 implementation

v0.3.0 is planned to ship the run runtime foundation for the Professional
Harness Runtime series.

Anticipated scope:

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
- The production harness routes `/run`, `/runs`/`/jobs`, `/attach`, `/detach`,
  `/cancel`, `/retry`, `/continue`, and `/fork` to run-native RPC methods.

Remaining before ship: reconnect/resume behavior, detached transcript invariant
coverage, attach conflict hardening, and final release-readiness review.
