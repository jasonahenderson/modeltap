---
feature: FEAT-0015
title: Professional Harness Runtime
status: accepted
date: 2026-04-29
series: Professional Harness Runtime
series-role: umbrella
depends-on:
  - FEAT-0008: Runtime Server
  - FEAT-0009: Terminal Harness
  - FEAT-0014: Harness Conversation Shell
adr-constraints:
  - ADR-0014: Harness Base Strategy
promoted-from:
  - EXP-0011: Harness Excellence Gap Analysis
related:
  - FEAT-0011: Knowledge Integration
  - FEAT-0012: Skills
  - FEAT-0013: Agent Teams
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0017: Durable Runs and Background Agents
  - FEAT-0018: Context Planner and Project Rules
  - FEAT-0019: Validation and Repair Loop
  - FEAT-0020: Patch Evidence and Run Artifacts
  - FEAT-0021: Policy-Grade Tool Runtime
  - FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions
---

# FEAT-0015: Professional Harness Runtime

## Problem

Modeltap's current harness and runtime server architecture can submit turns, stream model
responses, execute local tools, and render a usable terminal shell. That is a
strong foundation, but it is not yet a professional coding harness for complex
structured work.

Users need modeltap to handle tasks such as:

- creating and expanding explorations, feature specs, ADRs, release plans, and
  release status artifacts
- implementing complex features that touch many files
- debugging and fixing issues from pasted logs, stack traces, or descriptions
- producing user documentation and developer documentation
- running devops and release-readiness workflows
- delegating bounded review, validation, or implementation work to background
  agents without losing state

These tasks should not be treated as separate ad hoc chat tricks. They need a
shared runtime that makes work durable, inspectable, resumable, policy-aware,
and evidence-producing.

## Solution

Add a professional harness runtime on top of the existing runtime server and terminal
harness. The runtime treats all meaningful work as a durable run. A run may be
foreground or background, single-agent or multi-agent, read-only or mutating,
attached or detached. The same lifecycle, permission model, artifact capture,
and recovery controls apply across those modes.

The terminal harness remains the primary attached UI. The runtime server owns orchestration
state, prompt/context planning, routing, persistence, and run artifacts. The
local harness or local executor owns filesystem access, local tool execution,
permission enforcement, and optional workspace isolation.

This feature is an umbrella behavior contract. It relates several downstream
features that may be implemented separately, but they should converge on the
same run/runtime model instead of creating incompatible foreground,
background, tool, and agent paths.

## Key Capabilities

### Durable Runs

Every meaningful task is represented as a run with a stable run ID.

Run identity is distinct from session identity. A `session_id` identifies the
conversation container: persisted chat state, turn order, model override, and
session-level operations such as resume, fork, clear, and compact. A run ID
identifies a durable execution of a workflow or task, usually initiated from a
turn within a session. A session can contain many runs, and runtime controls
such as inspect, attach, detach, cancel, continue, retry, and fork address the
run ID rather than the session ID.

Run creation accepts a client-supplied idempotency key so retries after network
loss do not create duplicate runs. Tool-result delivery is idempotent per
`tool_call_id`, and model-call accounting is idempotent per `model_call_id`.

Run events are append-only and monotonically sequenced per `run_id`. Event
streams are resumable from a last-observed sequence number, and reattach must
detect gaps rather than silently skipping events. Persisted run-related records,
including run records, events, workflow contracts, context plans, and artifact
bundles, carry schema version metadata.

Run metadata has three related but distinct axes:

- **status**: whether the run is queued, active, waiting, checkpointed, or
  terminal
- **pipeline stage**: what work the runtime is currently performing
- **attachment state**: whether a harness is actively attached to the run

Canonical run statuses:

- `queued`
- `running`
- `waiting_permission`
- `waiting_user`
- `checkpointed`
- `completed`
- `failed`
- `cancelled`

`waiting_permission` means the run is blocked on approval for a tool or policy
decision. `waiting_user` means the run is blocked on non-permission user input,
such as choosing a plan direction, answering a clarification, or deciding how to
handle an ambiguous failure.

Canonical pipeline stages:

- `preflight`
- `context_plan`
- `prompt_plan`
- `model_call`
- `tool_loop`
- `validation`
- `artifact_capture`
- `checkpoint`
- `completion`

A run can combine these axes. For example, an implementation run may be
`status: waiting_permission`, `stage: tool_loop`, and
`attachment: detached`.

Canonical attachment states:

- `attached`
- `detached`

Observers are per-client subscriptions, not a run attachment state. A blocked
run is a queue/UI grouping for runs in `waiting_permission` or `waiting_user`;
`blocked` is not a lifecycle status or attachment state.

Canonical run control verbs:

- `attach`
- `detach`
- `cancel`
- `continue`
- `retry`
- `fork`

`pause` is not a canonical user command in this series. A run that cannot
advance transitions to an explicit waiting, checkpointed, failed, or cancelled
status with a structured reason.

Run-related identifiers use a shared identity discipline:

| Identifier | Scope |
|---|---|
| `session_id` | conversation container |
| `run_id` | durable workflow execution |
| `turn_id` | one turn within a session; may be owned by at most one run |
| `model_call_id` | one provider/model dispatch within a run |
| `tool_call_id` | one requested tool invocation within a run |
| `check_id` | one validation check within a run |
| `artifact_id` | one run artifact |
| `decision_id` | one policy or approval decision |
| `result_id` | one tool or check execution result |
| `memory_id` | one durable or candidate memory item |
| `workflow_profile_id` | one workflow contract/profile |
| `host_fingerprint` | one local harness/executor host identity |
| `policy_version` | one effective policy version for a run/project context |
| `schema_version` | version marker on persisted run-related records |

Each persisted run-related record carries schema-version metadata. Identifier
formats are implementation details, but uniqueness scope and foreign-key
relationships are part of the runtime contract.

Runs record:

- initiating user request
- workflow type
- mode and permission profile
- selected model or agent/team composition
- context plan and prompt-layer metadata
- tool calls and tool results
- approval decisions
- file diffs and generated artifacts
- validation commands and evidence
- cost, latency, and token usage
- checkpoints and final outcome

Cancellation is cooperative by default and bounded by a runtime deadline. The
runtime server records the cancellation request, asks active model calls and
tool calls to stop, and transitions the run to `cancelled` when active work has
stopped or the deadline expires. In-flight tool calls may either return a final result or an
interrupted result; the runtime must not assume local filesystem writes can be
rolled back automatically. Artifacts, logs, approvals, and partial diffs
captured before cancellation are retained. Cancelling a parent run cascades to
active children unless run policy explicitly opts out.

### Foreground and Background Agents

Foreground and background agents use the same run runtime.

A foreground run is an attached run that blocks the active conversation surface
and can ask interactive questions or permission prompts directly. A background
run is detached or non-focused: it continues without owning the main composer,
appears in a job/run queue, and either operates under pre-approved policy,
pauses for permission, or auto-denies operations outside policy.

Background agents are not a separate execution architecture. They are durable
runs with different attachment and interactivity semantics.

Parallel candidate work uses sibling runs, not branches within a run. A fork or
parallel candidate creates a new `run_id` that may reference a parent `run_id`;
an optional synthesis run may aggregate the sibling results. The runtime does
not introduce a `branch_id` concept for professional runtime work unless a
future ADR explicitly adds intra-run branching.

Only one harness or local executor may hold the foreground attachment lease for
a run at a time. Additional clients may observe run events, but they do not own
the composer or answer permission prompts unless the foreground lease is
transferred.

Run-family budgets and deadlines compose downward by default. Parent run cost,
token, wall-clock, and deadline limits bound child and sibling runs unless
policy explicitly grants a separate budget. Synthesis runs have explicit
timeouts, and parent cancellation or budget exhaustion cascades to active
children unless run policy says otherwise.

Required behavior:

- users can start a run in the foreground or send it to the background
- users can detach from an active run without cancelling it
- users can list, inspect, attach, cancel, continue, retry, or fork runs
- run progress survives harness restart or reconnect when the runtime server remains
  available
- blocked background runs surface a visible permission or user-input inbox
- background agent transcripts remain inspectable separately from the main
  transcript

### Workflow Contracts

Runs have workflow types. A workflow type defines default prompts, tools,
artifact expectations, validation behavior, and approval posture.

Initial workflow types:

| Workflow | Purpose | Default posture |
|---|---|---|
| `exploration` | Create or expand exploration docs and promotion candidates | read-heavy, background-friendly |
| `feature` | Draft or revise feature specs and success criteria | read/write docs, review-friendly |
| `adr` | Compare options, score decisions, and record consequences | read-heavy, approval before final write |
| `release` | Create release plans, tracks, WUs, status, and changelogs | foreground review gates |
| `implementation` | Change product code across many files | validation required |
| `debug` | Analyze pasted logs, reproduce, patch, and validate | foreground until scoped |
| `docs` | Produce user docs, developer docs, examples, and guides | background-friendly |
| `devops` | CI, release packaging, service config, and deployment scripts | strict shell approvals |

Workflow contracts are not full agent teams. They are the structured envelope
that makes a run produce the right artifacts and ask for the right approvals.
Artifact-oriented workflows such as `exploration`, `feature`, `adr`, and
`release` produce or revise the existing repository artifact families
(`.sdlc/explorations/`, `.sdlc/features/`, `.sdlc/adr/`, and
`.sdlc/releases/<version>/`). They do not replace the canonical process rules for
those directories. In particular, the `release` workflow must honor the
existing release plan/status/track/changelog structure and the strict
Phase 1 -> Phase 2 -> Phase 3 release process.

### Tool Runtime Integration

Tool calls are first-class run events, not transient stream messages.

Every tool call has a stable `tool_call_id` issued at request time. Tool
requests, tool results, permission decisions, and audit records all reference
this `tool_call_id`. The `tool_call_id` is unique within a run.

For every tool call, the runtime records:

- tool name, namespace, and schema version
- normalized input
- permission decision and policy reason
- execution workspace
- output envelope
- error or rejection reason
- files read, written, or deleted when knowable
- timing and exit status for shell commands

The runtime server may request tools, but local side effects remain harness/executor-owned.
The harness enforces local permission policy and returns structured tool
results to the runtime server.

Permission flow is sequenced as:

1. The harness or local executor detects that a tool request needs approval.
2. The harness reports the pending decision to the runtime server with `run_id` and
   `tool_call_id`.
3. The runtime server records the pending decision, emits `waiting_permission`, and surfaces
   an inbox event for the appropriate attached or authorized user.
4. The user resolves the decision through the harness inbox or attached run.
5. The harness records the decision and policy context; the runtime server persists the
   decision and emits the resulting run transition.

The runtime server is authoritative for run status and permission-inbox state. The harness
is authoritative for local side-effect enforcement.

### Workspace Policy

Workspace isolation is explicit policy, not an automatic property of every
background run.

Supported workspace modes:

- `current`: execute in the user's current project checkout
- `current_readonly`: read current checkout but reject writes
- `worktree`: create or attach to a Git worktree for isolated edits
- `temp_copy`: copy a workspace for risky or non-Git work
- `remote`: execute in a remote or cloud sandbox

These snake_case identifiers are canonical for run/workspace metadata.

Default policy:

- foreground planning and read-only runs use the current workspace
- background read-only runs may use `current_readonly`
- background writing runs either pause before writes or use an isolated
  workspace when configured
- parallel candidate implementations should use separate isolated workspaces
- reviewer and validator agents read the target run workspace, usually
  read-only

The runtime server stores workspace metadata on the run. The local harness/executor
creates and manages local workspaces because it owns the filesystem.

Workspace lifecycle follows the same authority split:

1. Workspace mode and metadata are selected during `preflight`.
2. The local harness/executor owns creation, mutation, and cleanup of `current`,
   `current_readonly`, `worktree`, and `temp_copy` workspaces.
3. `worktree` and `temp_copy` workspaces are cleaned up when the owning run
   reaches a terminal state unless artifacts are explicitly pinned to that
   workspace.
4. Cancellation retains captured artifacts, then triggers workspace cleanup
   according to the same terminal-state rule.
5. On reconnect, the harness scans for orphaned local workspaces whose runs are
   no longer active according to the runtime server and cleans them with user-visible
   notice.
6. If a workspace becomes unexpectedly missing during an active run, the harness
   reports a `workspace_lost` fact and the runtime server transitions the run to `failed`
   with that reason.

`remote` workspaces are owned by the runtime server or remote sandbox provider; the harness
acts as policy and permission relay for those workspaces.

When no harness or local executor is connected, the runtime server never simulates local
side effects. Runs that require local filesystem, process, or workspace effects
pause with an executor-disconnected reason until an executor reconnects or a
server-safe alternative exists under an accepted ADR.

### Context and Prompt Planning

Implementation-quality runs include explicit context and prompt plans.

The runtime should support:

- project-rule discovery and precedence
- repository map and symbol/test discovery
- local style sampling from neighboring files
- user-attached files and pasted content
- memory and prior-run retrieval
- context provenance and token budgeting
- prompt-layer metadata that can be inspected without exposing secrets

### Validation and Repair

Implementation and debug runs include a validation plan when practical.

The runtime should:

- infer targeted checks from changed files and project structure
- run cheap checks before broad checks
- summarize failures with file and line context
- feed validation summaries into repair turns
- remember failed repair attempts within the run
- stop when evidence is sufficient or when policy/user input is required

### Patch Evidence and Run Artifacts

Mutating runs produce reviewable artifacts:

- pre/post diff
- changed-file list
- patch summary
- unrelated-change and churn warnings
- validation evidence
- final outcome summary

The harness should render compact artifact tokens in the transcript and provide
inspection affordances for patches, validation logs, context plans, and
approvals.

### Memory and Routing

Successful runs may produce memory candidates. The runtime server separates durable
project decisions from ephemeral debugging traces and lets users inspect,
accept, edit, or reject memory before promotion when policy requires it.

Routing should become quality-driven:

- helper models for context summaries and failure triage
- stronger models for implementation and architecture-sensitive decisions
- reviewer models for patch review
- cheaper models for documentation or simple repo-process tasks when adequate

Routing decisions and outcomes are captured so future policy can improve.

Every `model_call` runs under an explicit retry and backoff policy. Provider
5xx errors, rate limits, quota failures, and fallback-model choices are recorded
as run events. Fallback routing must not corrupt run state, double-count model
usage, or bypass the run queue and budget policy.

### Observability, Liveness, and Durability

Runs expose operator-facing metrics and structured logs, not only transcript UI.
At minimum, the runtime reports queue depth, time in stage, stuck-stage counts,
validation outcomes, permission-inbox age, and per-stage failure rates. Each run
has a trace ID that propagates across runtime server, harness/executor, and model-provider
calls.

Harnesses and local executors send heartbeats while attached or executing run
work. Runtime stages have deadlines; `model_call` and `tool_loop` deadlines are
configurable. Runs that exceed heartbeat, stage, or permission-input deadlines
transition according to policy, with stuck stages defaulting to `failed` with a
structured reason.

Run durability is runtime-owned. Stage transitions, tool results, permission
decisions, checkpoints, and artifact metadata are durable before they are used
to advance the run. The run-runtime ADR should define exact fsync boundaries
and checkpoint-format compatibility across at least the previous minor version.

Budget exhaustion transitions the run to `waiting_user` by default so the user
can grant more budget, narrow scope, continue in a cheaper mode, or cancel.
Per-run memory, process, file descriptor, disk, token, and cost caps are policy
inputs, and run-family limits compose with the parent budget/deadline rules.
Repair-loop cost is capped by default at three times the initial implementation
attempt cost, and routing must check remaining run budget before upgrading model
class. Policy may override these defaults explicitly.

Retention follows one envelope across the series: memory can outlive artifacts,
artifacts can outlive the active run transcript, and the run record is the last
non-promoted runtime record to age out. Member-feature retention knobs may tune
durations within this envelope, but must not produce artifact metadata without a
run record or promoted memory without provenance.

## UI / CLI / API Integration

### Terminal UI

The conversation shell remains the active attached surface for one foreground
run. Background runs are visible through a compact jobs/runs surface.

Expected commands:

- `/jobs` or `/runs` lists active, blocked, failed, and completed runs
- `/run` shows the currently attached run
- `/attach <run-id>` attaches the foreground surface to a run
- `/detach` returns the current run to the background
- `/cancel <run-id>` cancels a run
- `/continue <run-id>` continues from a checkpoint
- `/retry <run-id>` retries a failed stage
- `/fork <run-id>` creates an independent continuation
- `/artifacts <run-id>` opens the run artifact list

Workflow commands may map to skills, teams, or workflow contracts:

- `/explore`
- `/feature`
- `/adr`
- `/release`
- `/implement`
- `/debug`
- `/docs`
- `/devops`

Command ownership is registered at the umbrella level:

| Command | Owner |
|---|---|
| `/jobs`, `/runs`, `/attach`, `/detach`, `/cancel`, `/continue`, `/retry`, `/fork`, `/permissions` | FEAT-0017 |
| `/run`, `/run context`, `/run prompt`, `/run policy` | FEAT-0016 |
| `/context`, `/context rules`, `/context why`, `/context drop` | FEAT-0018 |
| `/validate`, `/validate plan`, `/validate retry`, `/repair` | FEAT-0019 |
| `/artifacts`, `/diff`, `/evidence` | FEAT-0020 |
| `/policy` | FEAT-0021 |
| `/memory`, `/routing`, `/workflows` | FEAT-0022 |
| `/explore`, `/feature`, `/adr`, `/release`, `/implement`, `/debug`, `/docs`, `/devops` | workflow profiles under FEAT-0015/0022 |

Subcommands use the `/x subcommand` shape consistently. Adding a command in this
series requires updating this registry.

### Protocol / API

The protocol will need run-oriented methods and events. Exact names are left to
the downstream design, but the runtime needs shapes equivalent to:

- create/start run
- list runs
- get run details
- attach/detach run
- cancel/pause/resume run
- resolve run permission
- list run artifacts
- stream run events from a checkpoint

## Feature Relationship Map

This umbrella feature relates the following downstream product features and
engineering tracks.

| Rank | Artifact | Purpose |
|---|---|---|
| 1 | ADR: Run Runtime Ownership and Semantics | Decide runtime server vs harness ownership, lifecycle states, attachment semantics, prompt/policy precedence, and workspace policy boundaries |
| 2 | FEAT-0016: Managed Codegen Run Pipeline | Make implementation turns durable run transactions with preflight, context/prompt planning, tool loop, artifact capture, and checkpoints |
| 3 | FEAT-0017: Durable Runs and Background Agents | Add attached/detached run semantics, background run queue, permission inbox, resume/attach/detach, and separate run transcripts |
| 4 | FEAT-0018: Context Planner and Project Rules | Add repo-aware context selection, project-rule discovery, prompt-layer inspection, and provenance |
| 5 | FEAT-0019: Validation and Repair Loop | Add validation planning, structured check evidence, failure summarization, and repair-attempt memory |
| 6 | FEAT-0020: Patch Evidence and Run Artifacts | Persist and inspect diffs, validation logs, approvals, prompt/context plans, cost, and outcomes |
| 7 | FEAT-0021: Policy-Grade Tool Runtime | Add command/path/domain policy, audit grouping, workspace profiles, and richer approval behavior |
| 8 | FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions | Promote successful work to memory, route by workflow/stage/risk, and align skills/hooks/teams with run contracts |

This order is stack-ranked by code-generation quality impact and foundation
value, not by implementation size.

Series sequencing rules:

- FEAT-0016 owns the pipeline graph, stage taxonomy, checkpoints, and run-level
  event categories.
- FEAT-0017 owns attached/detached behavior, observer attach claims, background
  queue behavior, and reconnect UX.
- FEAT-0018 owns context-plan snapshots, project-rule precedence, and context
  provenance categories.
- FEAT-0019 owns validation plans, check outcomes, repair limits, and validation
  evidence.
- FEAT-0020 owns artifact envelopes, retention coordination details, and patch
  evidence timing.
- FEAT-0021 owns tool-policy evaluation, server-safe tool classification slots,
  permission decisions, and audit records.
- FEAT-0022 owns memory candidates, routing decisions, extension trust, and
  workflow extension behavior.

The Run Runtime ADR must lock event delivery, transition policy, identity
semantics, server-safe tool enumeration, and durability boundaries before Phase 3
implementation begins for this series.

The codegen evaluation harness remains an expected implementation-scoped
supporting patch, but it is not part of this behavior-contract map until a
`PATCH-NNNN` artifact is drafted.

## Future ADRs

Before this series can move from draft/proposed into implementation planning,
the following ADRs or ADR sections should be drafted and accepted where they
have future constraint value:

| ADR topic | Covers | Related features |
|---|---|---|
| Run runtime ownership and semantics | runtime server vs harness ownership, run status/stage terminology, attachment leases, cancellation, checkpoint and reconnect semantics, local-executor availability | FEAT-0015, FEAT-0016, FEAT-0017 |
| Run event stream, idempotency, and schema semantics | Event ordering, sequence checkpoints, gap detection, idempotency keys, duplicate result handling, `model_call_id`, and run/artifact/workflow schema versioning | FEAT-0015, FEAT-0016, FEAT-0017, FEAT-0020 |
| Project rules and prompt layering | Rule-file precedence, prompt-layer ownership, prompt metadata visibility, context budget authority | FEAT-0018 |
| Validation and repair artifacts | Validation artifact schema, repair-loop limits, failure classification, retry boundaries | FEAT-0019 |
| Artifact storage, redaction, retention, and GC | Run artifact storage, blob references, redaction/encryption, retention windows, garbage collection, and blob lifecycle by deployment profile | FEAT-0020 |
| Policy, workspace, and resource boundaries | Policy inheritance, sandbox/workspace modes, non-overridable policy, local vs server enforcement, resource caps, and budget exhaustion behavior | FEAT-0021 |
| Memory, routing, extension trust, and provider resilience | Memory promotion defaults, routing role taxonomy, retry/backoff, fallback model policy, per-provider quotas, skill/hook/team trust boundaries | FEAT-0022 |
| Runtime operability and upgrade safety | Run metrics, trace propagation, heartbeat/stuck-stage detection, fsync boundaries, rolling restart behavior, and checkpoint compatibility | FEAT-0015, FEAT-0017 |

## Document Placement Guidance

The current repo taxonomy does not have a separate `EPIC`, `PROGRAM`, or
`INITIATIVE` artifact type. Use the existing artifacts this way:

- Use an **exploration** for early grouping and problem framing across many
  possible downstream artifacts.
- Use an **umbrella feature** when the group itself describes a user-visible
  behavior contract, as this document does.
- Use a **release plan** when the group is about shipping a selected set of WUs
  in a specific release.
- Use an **ADR** when the group depends on a cross-cutting architectural
  decision.
- Use a **patch** only when the grouped work is implementation-scoped and can be
  completed by checklist.
- Use directory READMEs as indexes, not as substitutes for product contracts.

## Configuration

Configuration details should be specified in downstream features, but the
runtime will likely need:

- default workflow profiles
- default workspace policy per workflow
- background run concurrency limits
- maximum run cost and token budgets
- per-run memory, process, file descriptor, and disk caps
- allowed background tools
- permission timeout and blocked-run retention
- run record and artifact blob retention windows
- memory promotion policy

## Non-Goals

- Do not fork an external harness or invert modeltap's runtime-first architecture.
- Do not require every background run to use a Git worktree.
- Do not make background agents a separate execution system from foreground
  runs.
- Do not allow unbounded autonomous swarms.
- Do not bypass local harness permission enforcement for local filesystem or
  shell side effects.
- Do not replace FEAT-0013 Agent Teams; teams become one kind of run/workflow.

## Success Criteria

Acceptance scope: this umbrella is accepted as the governing behavior contract
for the Professional Harness Runtime series. v0.3.0 implements only the run
runtime foundation; context planning, validation, artifacts, policy-grade tools,
memory/routing, isolated writer workspaces, and workflow-extension commands
remain assigned to their downstream features/releases.

1. Foreground and background work share the same durable run lifecycle and run
   artifact model.
2. A user can start a long-running implementation, docs, exploration, ADR, or
   debug task, detach from it, inspect progress later, and reattach without
   losing state.
3. A blocked background run surfaces a visible pending-permission or
   pending-user-input item without corrupting the active foreground transcript.
4. Tool calls are stored as structured run events with approval, output,
   workspace, and outcome metadata.
5. Mutating runs produce patch artifacts and validation evidence that are
   inspectable from the harness.
6. Workflow types apply different tool, validation, artifact, and permission
   defaults.
7. Workspace policy is explicit and testable; background writers do not mutate
   the current checkout unexpectedly unless policy and user approval allow it.
8. The runtime server can resume or summarize run streams after harness reconnect.
9. Existing FEAT-0008/0009/0014 behavior remains compatible: normal attached
   chat still works as a foreground run.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Requires modeltap to continue evolving its own runtime-first Go/Bubbletea harness as the universal orchestration client |
| ADR-0015 | Decides run ownership, attachment semantics, executor availability, checkpoint semantics, liveness, and event ordering for the v0.3.0 foundation |

## Open Questions

These remain downstream design questions after v0.3.0 foundation acceptance:

1. Should run queue persistence live in the existing runtime server session store or in a
   separate run/job table family?
2. Which workflow types should ship first: `implementation`, `debug`,
   `docs`, and `exploration`; or all initial workflow contracts together?
3. Should background runs auto-deny non-preapproved tool calls or pause by
   default?
4. What is the default workspace policy for background writing runs in solo
   mode?
5. How should run artifacts be redacted or encrypted in team and enterprise
   deployments?
6. Should workflow commands be implemented as skills, as first-class run
   profiles, or as a unified extension model that includes both?
7. Which server-safe tool surfaces, if any, may continue without a connected
   harness/local executor under the disconnected-executor rule?
