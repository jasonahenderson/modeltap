---
feature: FEAT-0026
title: Runtime-to-Proxy Correlation
status: draft
date: 2026-05-26
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 10
related:
  - FEAT-0017: Durable Runs and Background Agents
  - FEAT-0020: Patch Evidence and Run Artifacts
---

# FEAT-0026: Runtime-to-Proxy Correlation

## Problem

modeltap has two durable records of model activity:

- **Runtime records**: sessions, turns, runs, run events, checkpoints, and
  artifacts owned by the harness/runtime server.
- **Proxy captures**: raw provider request/response exchanges captured by the
  reverse proxy.

Users need these records to line up. When inspecting a session, turn, run, or
proxy capture, modeltap should answer:

- Which conversation produced this provider call?
- Which turn or run caused it?
- Which raw provider request/response belongs to this runtime action?
- Are related retries, forks, continuations, or child runs part of the same
  logical execution lineage?

Without a clear correlation contract, `/session show`, `/run show`, logs,
dashboard views, and future turn/trace inspection will expose overlapping but
disconnected records.

## Current State

The implementation already has partial runtime-to-proxy correlation:

- Runtime runs have `run_id`, `trace_id`, and `session_id`.
- Runtime provider dispatch stamps modeltap-private HTTP headers when routing
  cloud-provider calls through the local capture proxy:
  - `X-Modeltap-Run-Id`
  - `X-Modeltap-Trace-Id`
- The proxy strips those headers before forwarding upstream.
- The proxy stores `run_id` and `trace_id` on captured request rows.
- Proxy captures can be filtered by `run_id` and `trace_id`.

Today, however, every new run receives a fresh `trace_id`. That means
`trace_id` is currently mostly equivalent to `run_id` for grouping purposes.
The missing behavior is trace lineage: related retry, fork, continue, or child
runs should share a trace.

## Solution

Define the canonical relationship as:

```text
session -> turn -> run -> proxy capture
```

Use `run_id` as the exact proxy-correlation anchor for one concrete execution
attempt. Use `trace_id` as the logical execution-lineage anchor across related
runs.

For runtime-owned traffic:

- Top-level runs create or accept a `trace_id`.
- Child, retry, fork, and continue runs inherit their parent run's `trace_id`.
- Provider calls include both `run_id` and `trace_id` as modeltap-private
  headers when routed through the local capture proxy.
- The proxy stores those IDs and strips the headers before upstream.
- Session is derived through `requests.run_id -> runs.id -> runs.session_id`.
- Turn linkage is derived through `run_turns` unless a future implementation
  proves direct `turn_id` on captures is necessary.

For proxy-only traffic from external clients:

- Captures remain uncorrelated unless the client opts in with modeltap-private
  correlation headers.
- Opt-in headers are stripped before upstream.
- If an external client supplies only `trace_id`, the capture is trace-grouped
  but not attached to a runtime session or run.

## Trace ID Semantics

`trace_id` identifies a logical execution lineage, not a conversation and not a
single run attempt.

ID roles:

| ID | Meaning |
|---|---|
| `session_id` | Conversation container |
| `turn_id` | One user/model exchange inside a session |
| `run_id` | One concrete executable attempt or work unit |
| `trace_id` | Related execution lineage across runs/calls |

Trace assignment rules:

1. If a top-level runtime caller supplies `trace_id`, validate and use it.
2. If a top-level runtime caller omits `trace_id`, generate `trace-<uuid>`.
3. If `parent_run_id` is present, load the parent and inherit
   `parent.trace_id`.
4. If `parent_run_id` is present and the caller supplies a conflicting
   `trace_id`, reject the request.
5. If an idempotency key resolves to an existing run, return the existing run
   and preserve its stored `trace_id`.

The harness should not ask normal shell users to specify traces. User-supplied
`trace_id` is an advanced API/integration field for external orchestrators,
imports, replay tools, observability bridges, and tests.

## Key Capabilities

### Runtime Trace Lineage

Runtime run creation supports optional caller-supplied `trace_id`. Parent-based
run creation inherits parent trace automatically.

Applies to:

- foreground `turn.submit` runs
- explicit `run.create`
- future retry/continue flows
- forked or child runs
- background-agent runs

### Proxy Capture Correlation

Proxy captures persist:

- `run_id` for exact execution-attempt lookup
- `trace_id` for lineage lookup

The proxy must continue stripping modeltap-private correlation headers before
upstream so provider APIs never receive internal IDs.

### Derived Session And Turn Lookup

Runtime and UI surfaces can derive related proxy captures:

```text
session captures:
  requests.run_id -> runs.id where runs.session_id = <session_id>

turn captures:
  run_turns.turn_id -> run_turns.run_id -> requests.run_id

run captures:
  requests.run_id = <run_id>

trace captures:
  requests.trace_id = <trace_id>
```

Direct `turn_id` on proxy captures is a non-goal for the first implementation.
It can be added later if one run can produce multiple provider calls that need
per-turn disambiguation beyond `run_turns`.

### User Inspection

The shell should keep conceptual boundaries clear:

- `/session show [id]` shows conversation-oriented state: metadata, turn
  summaries, files, server events, recent runs, and eventually linked capture
  counts.
- `/run show [id]` shows execution-oriented state: lifecycle, stage, status,
  checkpoint, run events, linked turns, and linked proxy captures.
- Future turn inspection can show full turn content plus linked runs and proxy
  captures for that turn.
- Future trace inspection can show all related runs and captures for a lineage.

## CLI / UI / API Integration

### Runtime Protocol

Add optional `trace_id` to runtime run-creation inputs where an external caller
can create top-level runs.

Recommended initial protocol change:

```json
{
  "method": "run.create",
  "params": {
    "session_id": "sess-...",
    "trace_id": "trace-external-123",
    "title": "external workflow"
  }
}
```

Normal shell commands do not need a trace argument.

### Proxy Headers

Continue using modeltap-private headers:

```text
X-Modeltap-Run-Id: run-...
X-Modeltap-Trace-Id: trace-...
```

Optional future external-client headers may include a session hint, but the
canonical runtime path derives session from the run.

### CLI Logs And Dashboard

Existing log/export surfaces that show `run_id` and `trace_id` should remain.
Follow-up UI work may add:

- `logs --run-id <id>`
- `logs --trace-id <id>`
- `logs --session-id <id>` using the join through `runs`
- dashboard filters for run, trace, and session

## Configuration

No new config is required.

Correlation headers are internal modeltap metadata and should remain enabled
for runtime-owned cloud-provider calls routed through the capture proxy.

## Non-Goals

- Do not require normal shell users to know or type trace IDs.
- Do not expose modeltap-private headers to upstream providers.
- Do not attach all external proxy traffic to a synthetic session by default.
- Do not make `trace_id` replace `session_id`, `turn_id`, or `run_id`.
- Do not add direct `turn_id` to proxy captures until a concrete need appears.
- Do not build a full `/trace show` UX in the first implementation patch.

## Implementation Notes

Suggested first implementation patch:

1. Add `trace_id` to `protocol.RunCreate`.
2. Add `TraceID` to internal run-creation options.
3. Centralize trace resolution:
   - existing run by idempotency key wins
   - parent run inherits parent trace
   - supplied top-level trace is validated and used
   - otherwise generate `trace-<uuid>`
4. Update foreground and explicit run creation to use the resolver.
5. Ensure run fork/child creation passes `parent_run_id` and inherits trace.
6. Keep proxy header stripping and request persistence as-is.
7. Add storage/query helpers only where needed by the first user-visible
   inspection surface.

Trace validation should be permissive enough for integration IDs:

- non-empty when supplied
- max length 128
- no whitespace
- no control characters

Generated IDs should keep the existing `trace-<uuid>` shape.

## Success Criteria

1. Top-level runs without caller-supplied trace IDs receive generated
   `trace-<uuid>` values.
2. Top-level `run.create` can accept a valid caller-supplied `trace_id`.
3. Child/fork/retry/continue runs inherit parent `trace_id`.
4. Supplying a conflicting `trace_id` with `parent_run_id` fails with a clear
   validation error.
5. Idempotent run creation returns the existing run with its existing trace.
6. Runtime-owned provider calls routed through the proxy persist the run's
   `run_id` and inherited `trace_id` on request captures.
7. The proxy strips `X-Modeltap-Run-Id` and `X-Modeltap-Trace-Id` before
   forwarding upstream.
8. Tests prove captures can be resolved from request to run to session.
9. Tests prove trace-level capture lookup groups related child runs.
10. User docs explain that `trace_id` is an advanced correlation lineage ID,
    not a normal shell concept.

## Relationship to Existing Features

- **FEAT-0017** defines durable runs and background-agent lifecycle. This
  feature clarifies how those runs correlate with proxy captures and with each
  other across lineage.
- **FEAT-0020** defines patch evidence and run artifacts. Proxy captures are
  one possible evidence source once they can be reliably joined to runs/traces.

## Open Questions

1. Should external proxy-only clients be allowed to supply
   `X-Modeltap-Session-Id`, or should session attachment require a runtime API
   call first?
2. Should `turn_id` be stored directly on request captures if a single run can
   span multiple turns in future workflows?
3. Should trace IDs accept arbitrary external observability IDs, or should
   modeltap normalize them into `trace-<uuid>` plus a separate external ID?
4. Should `/run show` display linked proxy capture IDs immediately, or should
   that wait for a broader inspection UI?
