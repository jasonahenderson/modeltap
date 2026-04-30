# Implementation Plan: v0.3.0 — Run Runtime Foundation

## Context

`v0.2.x` established the production conversation shell and BFF-backed harness
plumbing. `v0.3.0` begins the Professional Harness Runtime series by turning
foreground chat/codegen work into durable run transactions with explicit stage,
status, attachment, checkpoint, and recovery semantics.

This release implements the foundation required by FEAT-0016 and the first
slice of FEAT-0017. It must also produce the run-runtime ADR before any
implementation starts.

## Scope

This release covers:

- ADR for run runtime ownership and semantics
- run IDs, lifecycle status, pipeline stages, and attachment state
- BFF run registry/store and protocol events
- integration of current `turn.submit` behavior into lightweight foreground
  runs
- active-run inspection through `/run`
- basic run list through `/runs` / `/jobs`
- attach/detach semantics for BFF-known runs
- checkpoint metadata sufficient for inspect/retry/continue/fork designs

This release does not cover:

- repo-aware context planning beyond existing attachments
- validation planning or repair-loop behavior
- patch/diff artifact bundles
- policy-grade command/path/domain rules
- durable memory promotion or quality-driven routing
- full background local-tool execution while no harness/executor is connected

## Feature Scope

- FEAT-0016: Managed Codegen Run Pipeline
- FEAT-0017: Durable Runs and Background Agents, foundation slice only
- FEAT-0015: umbrella constraints

## Approach

The release executes in the repo's strict three phases:

1. **Phase 1 — Design:** design every WU listed below.
2. **Phase 2 — Review:** process design findings.
3. **Phase 3 — Implementation:** implement WUs in dependency-legal order.

Current phase: **Planning draft — Phase 1 not opened.** Opening Phase 1 requires
an explicit `ADMIN:` commit.

## Work Units

| WU | Title | Dependencies | Size | Feature |
|---|---|---|---|---|
| 108 | Run runtime ADR | — | M | FEAT-0015/0016/0017 |
| 109 | Run schema, storage, and migration design | 108 | M | FEAT-0016 |
| 110 | Run protocol methods and event taxonomy | 108 | M | FEAT-0016/0017 |
| 111 | BFF run registry and lifecycle store | 109, 110 | L | FEAT-0016 |
| 112 | `turn.submit` to foreground-run integration | 111 | L | FEAT-0016 |
| 113 | Pipeline stage/status emission and checkpoint metadata | 111, 112 | M | FEAT-0016 |
| 114 | Harness run projection and active `/run` surface | 110, 112 | M | FEAT-0016 |
| 115 | Run list, attach, detach, cancel, retry, continue, fork commands | 113, 114 | L | FEAT-0017 |
| 116 | Reconnect/resume behavior for active and detached runs | 111, 115 | M | FEAT-0017 |
| 117 | Runtime foundation tests and docs | 111-116 | M | FEAT-0016/0017 |

## Detailed WU Plan

### Track A — Decisions and Contracts

**WU-108: Run runtime ADR**

Decide the BFF/harness ownership model, canonical run status values, pipeline
stage vocabulary, attachment state, checkpoint semantics, reconnect behavior,
and whether disconnected local-tool execution is allowed. The ADR must settle
the open executor-availability question before implementation.

**WU-109: Run schema, storage, and migration design**

Design tables or records for runs, run events, run checkpoints, attachment
state, stage/status transitions, and summary metadata. Define retention and
compatibility with existing sessions/turns.

**WU-110: Run protocol methods and event taxonomy**

Design `run.*` or extended `turn.*` protocol surfaces for create/start, list,
details, attach/detach, cancel/pause/resume, event streaming from checkpoint,
and permission correlation.

### Track B — BFF Runtime

**WU-111: BFF run registry and lifecycle store**

Implement the run registry, persistence layer, lifecycle transitions, and
event append path. Runs are scoped by user/project/session and can be queried
independently of transcript scrollback.

**WU-112: `turn.submit` to foreground-run integration**

Wrap existing submit behavior in a lightweight foreground run. Preserve current
shell/BFF behavior while assigning run IDs, recording lifecycle transitions, and
linking turn IDs to run IDs.

**WU-113: Pipeline stage/status emission and checkpoint metadata**

Emit stage/status events for preflight, prompt/model/tool stages, completion,
failure, cancellation, and checkpoint records. Checkpoints must support future
retry/continue/fork behavior even if those actions are initially shallow.

### Track C — Harness Runtime Surface

**WU-114: Harness run projection and active `/run` surface**

Project BFF run events into `harnessshell`/`harnesshost` events. Add `/run`
inspection for the currently attached run without disrupting FEAT-0014 shell
semantics.

**WU-115: Run list, attach, detach, cancel, retry, continue, fork commands**

Add `/runs` or `/jobs` for run listing and host-native commands for attaching,
detaching, cancelling, retrying, continuing, and forking. Early retry/continue
may be checkpoint-aware no-ops if the ADR/design says implementation belongs to
later releases.

**WU-116: Reconnect/resume behavior for active and detached runs**

Define and implement how the harness recovers active/detached run state after
reconnect. The BFF may replay events or return summaries based on checkpoint
availability.

### Track D — Verification and Documentation

**WU-117: Runtime foundation tests and docs**

Add protocol tests, BFF lifecycle tests, harness projection tests, reconnect
tests, and user/developer docs for `/run`, `/runs`, attach/detach, and the
foreground-run model.

## Phase 1 Design Checklist

Phase 1 is complete only when all WUs have design docs under
`docs/releases/v0.3.0/designs/`:

- [ ] WU-108 run runtime ADR design/ADR draft
- [ ] WU-109 storage design
- [ ] WU-110 protocol design
- [ ] WU-111 to WU-113 BFF runtime design bundle
- [ ] WU-114 to WU-116 harness runtime design bundle
- [ ] WU-117 verification/docs design

## Risk Register

- **R1 — ADR scope creep.** The run-runtime ADR can become too broad. Keep it
  focused on ownership, terminology, attachment, checkpoint, and executor
  availability.
- **R2 — compatibility with existing turn flow.** `turn.submit` must continue to
  work as an attached foreground run without breaking v0.2.2 shell behavior.
- **R3 — disconnected background execution.** If local tools require a connected
  harness, background runs must pause clearly rather than pretending to continue
  local side effects.
- **R4 — storage churn.** Run storage should avoid premature artifact schemas
  that v0.3.2 will own.

## Definition of Done

1. Run-runtime ADR is accepted.
2. Every model/harness turn can be represented as a foreground run.
3. The BFF persists run lifecycle metadata and emits run events.
4. The harness can inspect active runs and list BFF-known runs.
5. Attach/detach/cancel/reconnect semantics are implemented to the v0.3.0
   design.
6. Tests cover BFF run lifecycle, protocol events, harness projection, and
   reconnect behavior.
