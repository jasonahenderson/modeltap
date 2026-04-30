---
feature: FEAT-0016
title: Managed Codegen Run Pipeline
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 1
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
  - FEAT-0014: Harness Conversation Shell
adr-constraints:
  - ADR-0014: Harness Base Strategy
---

# FEAT-0016: Managed Codegen Run Pipeline

## Problem

Modeltap can submit a turn, stream a model response, and mediate tool calls, but
an implementation request is still mostly treated as a chat turn. Professional
coding work needs a managed transaction: the system should know what stage the
work is in, what context and policy apply, which tools ran, what changed, what
validation proved, and how to recover if the run fails or is interrupted.

Without a managed pipeline, context planning, validation, patch artifacts,
background runs, and memory promotion become ad hoc features with incompatible
state models.

## Solution

Define code generation as a durable run pipeline owned primarily by the BFF and
surfaced by the harness. Each implementation or debug run moves through explicit
stages:

`preflight -> context_plan -> prompt_plan -> model_call -> tool_loop -> validation -> artifact_capture -> checkpoint -> completion`

The BFF stores lifecycle metadata and emits progress events. The harness renders
the active stage compactly, enforces local side-effect policy, executes local
tools, and lets the user inspect or recover the run.

## Key Capabilities

### Run Lifecycle

The pipeline must support these stage states:

- `preflight`: resolve workflow, mode, model policy, user/project policy, and
  workspace.
- `context_plan`: assemble the files, rules, memory, and user-provided context
  intended for the turn.
- `prompt_plan`: assemble prompt layers and tool definitions with budget
  metadata.
- `model_call`: dispatch to the selected model or agent.
- `tool_loop`: process tool calls and tool results until the model stops or
  policy blocks.
- `validation`: run or record checks when the workflow requires validation.
- `artifact_capture`: collect patch, command, validation, approval, and cost
  evidence.
- `checkpoint`: persist enough state to continue, retry, fork, or inspect.
- `completion`: record final outcome and summary.

### BFF Responsibilities

- create a run ID before dispatching model work
- persist run stage transitions
- assemble or reference context and prompt plans
- route the model call
- correlate stream events, tool calls, cost, and provider usage with the run
- store checkpoint metadata
- expose run details through protocol/API methods

### Harness Responsibilities

- render active run stage and status without noisy transcript chatter
- execute local tools only through the run's policy/workspace context
- surface inspectable summaries for context, prompt, policy, and artifacts
- provide interrupt, retry, continue, and fork actions against the run ID
- preserve FEAT-0014 shell semantics by consuming run events through the host
  boundary

### Pipeline Events

The protocol should expose structured run lifecycle events. Exact names are a
design detail, but the harness needs:

- run started
- stage changed
- stage progress update
- tool call requested
- tool result recorded
- artifact recorded
- checkpoint recorded
- run completed, failed, cancelled, or blocked

## UI / CLI / API Integration

The harness should show the current stage in status chrome and expose details on
demand:

- `/run` shows the active run summary
- `/run context` shows the context plan summary
- `/run prompt` shows prompt-layer metadata
- `/run policy` shows active policy and workspace
- `/continue`, `/retry`, and `/fork` operate on the active run

The BFF protocol needs run-aware submit and inspection surfaces, either by
extending `turn.submit` or adding run-specific methods.

`/run` is singular and always refers to the currently attached run. `/runs` or
`/jobs` is the plural queue/list surface defined by FEAT-0017.

## Configuration

Configuration should allow:

- default pipeline behavior per workflow type
- checkpoint retention
- maximum stage duration before surfacing a warning
- whether prompt-layer metadata is visible by default

## Non-Goals

- This feature does not define background queue UX; see FEAT-0017.
- This feature does not define repository context selection; see FEAT-0018.
- This feature does not define validation planning; see FEAT-0019.
- This feature does not require multi-agent teams.

## Success Criteria

1. An implementation request creates a stable run ID before model dispatch.
2. The BFF records and emits stage transitions for the run.
3. Tool calls, model selection, cost, and terminal outcome are correlated with
   the run ID.
4. The harness renders stage/status and can inspect run metadata on demand.
5. Interrupt, retry, continue, and fork actions address a run ID rather than
   transcript-only state.
6. Existing simple chat remains compatible and can be represented as a
   foreground run.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Requires BFF-first orchestration with the harness as universal orchestration client |
| Future ADR | Should decide run ownership, lifecycle terminology, checkpoint semantics, and prompt/policy boundaries |

## Open Questions

1. Should `turn.submit` become run-aware or should new `run.*` methods wrap
   turn submission?
2. What minimum checkpoint data is required to safely continue a failed run?
3. How should simple attached chat use a lightweight run representation without
   forcing the full codegen pipeline onto low-risk conversation turns?
4. How much prompt metadata can be exposed without leaking protected prompt
   content?
