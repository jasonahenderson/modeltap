---
feature: FEAT-0015
title: Professional Harness Runtime
status: draft
date: 2026-04-29
series: Professional Harness Runtime
series-role: umbrella
depends-on:
  - FEAT-0008: BFF Server
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

Modeltap's current harness and BFF architecture can submit turns, stream model
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

Add a professional harness runtime on top of the existing BFF and terminal
harness. The runtime treats all meaningful work as a durable run. A run may be
foreground or background, single-agent or multi-agent, read-only or mutating,
attached or detached. The same lifecycle, permission model, artifact capture,
and recovery controls apply across those modes.

The terminal harness remains the primary attached UI. The BFF owns orchestration
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

Run metadata has three related but distinct axes:

- **status**: whether the run is queued, active, blocked, checkpointed, or
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

### Foreground and Background Agents

Foreground and background agents use the same run runtime.

A foreground run is an attached run that blocks the active conversation surface
and can ask interactive questions or permission prompts directly. A background
run is detached or non-focused: it continues without owning the main composer,
appears in a job/run queue, and either operates under pre-approved policy,
pauses for permission, or auto-denies operations outside policy.

Background agents are not a separate execution architecture. They are durable
runs with different attachment and interactivity semantics.

Required behavior:

- users can start a run in the foreground or send it to the background
- users can detach from an active run without cancelling it
- users can list, inspect, attach, cancel, continue, retry, or fork runs
- run progress survives harness restart or reconnect when the BFF remains
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
(`docs/explorations/`, `docs/features/`, `docs/adr/`, and
`docs/releases/<version>/`). They do not replace the canonical process rules for
those directories. In particular, the `release` workflow must honor the
existing release plan/status/track/changelog structure and the strict
Phase 1 -> Phase 2 -> Phase 3 release process.

### Tool Runtime Integration

Tool calls are first-class run events, not transient stream messages.

For every tool call, the runtime records:

- tool name, namespace, and schema version
- normalized input
- permission decision and policy reason
- execution workspace
- output envelope
- error or rejection reason
- files read, written, or deleted when knowable
- timing and exit status for shell commands

The BFF may request tools, but local side effects remain harness/executor-owned.
The harness enforces local permission policy and returns structured tool
results to the BFF.

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

The BFF stores workspace metadata on the run. The local harness/executor
creates and manages local workspaces because it owns the filesystem.

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

Successful runs may produce memory candidates. The BFF separates durable
project decisions from ephemeral debugging traces and lets users inspect,
accept, edit, or reject memory before promotion when policy requires it.

Routing should become quality-driven:

- helper models for context summaries and failure triage
- stronger models for implementation and architecture-sensitive decisions
- reviewer models for patch review
- cheaper models for documentation or simple repo-process tasks when adequate

Routing decisions and outcomes are captured so future policy can improve.

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
| 1 | ADR: Run Runtime Ownership and Semantics | Decide BFF vs harness ownership, lifecycle states, attachment semantics, prompt/policy precedence, and workspace policy boundaries |
| 2 | FEAT-0016: Managed Codegen Run Pipeline | Make implementation turns durable run transactions with preflight, context/prompt planning, tool loop, artifact capture, and checkpoints |
| 3 | FEAT-0017: Durable Runs and Background Agents | Add attached/detached run semantics, background run queue, permission inbox, resume/attach/detach, and separate run transcripts |
| 4 | FEAT-0018: Context Planner and Project Rules | Add repo-aware context selection, project-rule discovery, prompt-layer inspection, and provenance |
| 5 | FEAT-0019: Validation and Repair Loop | Add validation planning, structured check evidence, failure summarization, and repair-attempt memory |
| 6 | FEAT-0020: Patch Evidence and Run Artifacts | Persist and inspect diffs, validation logs, approvals, prompt/context plans, cost, and outcomes |
| 7 | FEAT-0021: Policy-Grade Tool Runtime | Add command/path/domain policy, audit grouping, workspace profiles, and richer approval behavior |
| 8 | FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions | Promote successful work to memory, route by workflow/stage/risk, and align skills/hooks/teams with run contracts |

This order is stack-ranked by code-generation quality impact and foundation
value, not by implementation size.

The codegen evaluation harness remains an expected implementation-scoped
supporting patch, but it is not part of this behavior-contract map until a
`PATCH-NNNN` artifact is drafted.

## Future ADRs

Before this series can move from draft/proposed into implementation planning,
the following ADRs or ADR sections should be drafted and accepted where they
have future constraint value:

| ADR topic | Covers | Related features |
|---|---|---|
| Run runtime ownership and semantics | BFF vs harness ownership, run status/stage terminology, attachment state, checkpoint and reconnect semantics, local-executor availability | FEAT-0015, FEAT-0016, FEAT-0017 |
| Project rules and prompt layering | Rule-file precedence, prompt-layer ownership, prompt metadata visibility, context budget authority | FEAT-0018 |
| Validation and repair artifacts | Validation artifact schema, repair-loop limits, failure classification, retry boundaries | FEAT-0019 |
| Artifact storage and redaction | Run artifact storage, blob references, redaction/encryption, retention by deployment profile | FEAT-0020 |
| Policy and workspace boundaries | Policy inheritance, sandbox/workspace modes, non-overridable policy, local vs server enforcement | FEAT-0021 |
| Memory, routing, and extension trust | Memory promotion defaults, routing role taxonomy, skill/hook/team trust boundaries | FEAT-0022 |

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
- allowed background tools
- permission timeout and blocked-run retention
- memory promotion policy

## Non-Goals

- Do not fork an external harness or invert modeltap's BFF-first architecture.
- Do not require every background run to use a Git worktree.
- Do not make background agents a separate execution system from foreground
  runs.
- Do not allow unbounded autonomous swarms.
- Do not bypass local harness permission enforcement for local filesystem or
  shell side effects.
- Do not replace FEAT-0013 Agent Teams; teams become one kind of run/workflow.

## Success Criteria

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
8. The BFF can resume or summarize run streams after harness reconnect.
9. Existing FEAT-0008/0009/0014 behavior remains compatible: normal attached
   chat still works as a foreground run.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Requires modeltap to continue evolving its own BFF-first Go/Bubbletea harness as the universal orchestration client |
| Future ADR | Should decide run ownership, attachment semantics, workspace policy, and prompt/policy precedence before implementation |

## Open Questions

1. Should run queue persistence live in the existing BFF session store or in a
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
7. Can background runs continue local tool execution when no harness or local
   executor is connected, or must they pause / use only server-safe tools until
   a local executor reconnects?
