---
feature: FEAT-0022
title: Durable Memory, Quality Routing, and Workflow Extensions
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 7
depends-on:
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0020: Patch Evidence and Run Artifacts
related:
  - FEAT-0011: Knowledge Integration
  - FEAT-0012: Skills
  - FEAT-0013: Agent Teams
adr-constraints:
  - ADR-0008: Knowledge Layer Architecture
  - ADR-0014: Harness Base Strategy
---

# FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions

## Problem

A professional harness should improve over time. Successful runs should produce
memory candidates, routing should use task quality signals, and common
workflows should become reusable commands or skills. Today modeltap has related
feature directions, but they are not yet tied to the durable run model.

If memory, routing, skills, hooks, and agent teams evolve independently, they
will duplicate prompt logic and miss the opportunity to use run artifacts as the
source of truth.

## Solution

Use run artifacts as the bridge between memory, routing, and workflow
extensions. Successful runs produce memory candidates. Routing decisions use
workflow, risk, context, validation state, and prior outcomes. Skills, hooks,
slash commands, and agent teams become workflow extensions that operate inside
the same durable run contract.

## Key Capabilities

### Memory Candidates

Runs may produce memory candidates from:

- accepted architectural decisions
- project conventions discovered during work
- debugging conclusions
- validation commands that proved useful
- release/process constraints
- user-approved summaries

The runtime server separates durable knowledge from ephemeral traces by rule. Candidate
generation is triggered by accepted ADR/feature/release artifacts, validation
commands the user explicitly approved, and user-marked moments such as
`/remember`. Heuristic candidate generation is a future opt-in capability.

Candidate generation has a default soft cap of 10 candidates per run. Duplicate
candidates are coalesced; over-cap candidates are summarized into a deferred
bucket the user can expand on demand. The harness lets the user inspect, accept,
edit, or reject candidates when policy requires it.

### Active Memory Inspection

When memory influences a run, the user can inspect:

- memory item summary
- source run/artifact
- relevance reason
- scope: global, project, package, file, workflow
- age and confidence when available

Memory conflict resolution uses specificity:

`file > package > project > workflow > global`

Conflicts within a scope are recorded as warnings and presented together.
Retrieval ranking defaults to scope match, then vector similarity, then recency.
The active-memory artifact records the ranking method. Age is always available
from the memory creation timestamp. Confidence is present when the source has a
quantitative score, such as validation pass count or vector similarity; otherwise
the harness renders the field as absent rather than hiding the item.

Durable memory has no automatic expiry but is checked for staleness when
referenced files or symbols disappear. Ephemeral memory ages out after 30 days by
default. Users may pin memories against expiry.

### Quality-Driven Routing

Routing should select models or roles by workflow stage:

- context helper
- implementation
- validation summarizer
- repair
- reviewer
- documentation
- synthesizer

Routing roles are model selections within FEAT-0016 pipeline stages, not new
stages. For example, `context helper` selects a model for `context_plan`,
`validation summarizer` selects a model for summarization within `validation`,
and `repair` selects a model for repair turns that reenter `model_call`.

Routing decisions should record reason, cost, model capability, and outcome so
future tuning can improve quality/cost trade-offs. Routing decisions are stored
as `routing_decision` artifacts under the FEAT-0020 artifact envelope.

Default routing is a fast deterministic policy using workflow, stage, run
history, and configured model preferences. Inference-driven routing is opt-in.
Routing failures fall back to the configured default model and record
`routing_fallback`. Dataset learning is per deployment by default; cross-tenant
aggregation is opt-in with explicit consent and dataset provenance recorded on
each routing decision.

### Workflow Extensions

Skills, hooks, slash commands, and agent teams should align with workflow
contracts:

- skills specialize one run or run stage
- hooks can warn, block, or enrich run/tool stages
- slash commands create or control runs
- agent teams execute as runs with multiple coordinated agents

Extensions may narrow tools, set model preferences, define artifact
requirements, or add validation behavior.

Extension trust tiers are:

- built-in extensions: full trust within normal policy
- workspace-local extensions: may narrow tools and add requirements, but cannot
  widen the tool surface
- third-party extensions: must declare capabilities and execute inside FEAT-0021
  policy

Untrusted extensions cannot widen tools, override policy decisions, or bypass
workflow validation requirements.

Hooks have a default 5 second deadline and 256 MiB memory bound. Over-limit hooks
are reported as hook errors and the tool call proceeds or blocks according to
FEAT-0021 hook policy. Workspace-local hooks may raise limits through config;
third-party hooks cannot.

Routing roles are orthogonal to workflow types. A workflow such as `debug` may
use several roles (`context helper`, `repair`, `validation summarizer`,
`reviewer`) during one run, while a role such as `reviewer` may appear in
multiple workflows.

### Cross-Feature Impact

This feature re-anchors FEAT-0012 and FEAT-0013 rather than replacing them:

- FEAT-0012 skills should become workflow/run specializations that can narrow
  tools, alter prompt behavior, and optionally set model preferences for one run
  or run stage.
- FEAT-0013 agent teams should execute as durable runs with multiple coordinated
  agents, shared artifacts, policy-aware tool calls, and run-level traceability.

If FEAT-0012 or FEAT-0013 are accepted before this feature, their accepted
language should be revised or constrained so skills and teams do not bypass the
durable run contract.

## UI / CLI / API Integration

Expected commands:

- `/memory` shows active and candidate memory
- `/memory accept|edit|reject <id>` handles candidates
- `/routing` shows model and role decisions for the run
- `/workflows` lists available workflow profiles
- existing `/skills` and `/team` surfaces should eventually reference workflow
  contracts

`<id>` refers to a memory candidate ID listed by `/memory`.

The runtime server protocol should expose memory candidates, active memory provenance, and
routing explanations through run details.

## Configuration

Configuration should support:

- memory promotion policy
- memory scopes and retention
- workflow routing roles
- model preferences per workflow/stage
- extension manifests
- hook enablement and trust policy
- hook resource limits
- memory candidate caps
- routing fallback policy

## Non-Goals

- This feature does not implement the base knowledge layer from scratch; it
  builds on FEAT-0011.
- This feature does not replace FEAT-0012 or FEAT-0013.
- This feature does not require automatic memory promotion without user or
  policy control.
- This feature does not require candidate-patch comparison in the first slice.

## Success Criteria

1. Successful runs can produce memory candidates linked to source artifacts.
2. Users can inspect and disposition memory candidates.
3. Active memory injected into a run is visible with provenance.
4. Routing decisions are recorded with stage, reason, model, and outcome.
5. Workflow extensions operate inside the durable run contract rather than
   bypassing it.
6. Skills and teams can reference workflow profiles or run-stage behavior.
7. Future routing improvements can be evaluated against stored run outcomes.
8. Routing outcome datasets remain deployment-scoped unless explicit opt-in
   enables broader aggregation.

Acceptance of the workflow-extension portions is gated on coordination with
FEAT-0012 and FEAT-0013. The memory/routing portions may be accepted earlier if
the feature is split or explicitly phased.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0008 | Memory candidates and retrieval build on the knowledge layer |
| ADR-0014 | Keeps routing and orchestration runtime-owned while the harness remains the terminal client |
| Future ADR | Should decide memory promotion defaults, routing role taxonomy, and extension trust boundaries |

## Open Questions

1. How should quality outcomes be scored without overfitting to tests?
2. Should workflow extensions live in `.modeltap.yaml`, `.modeltap/`, or a
   plugin/skill packaging format?
3. When should the system compare multiple candidate patches instead of running
   one implementation plus review?
4. Should this feature split into an earlier memory/routing slice and a later
   workflow-extension slice that waits on FEAT-0012 and FEAT-0013?
