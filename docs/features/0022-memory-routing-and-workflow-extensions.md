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
  - FEAT-0011: Knowledge Integration
  - FEAT-0012: Skills
  - FEAT-0013: Agent Teams
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0020: Patch Evidence and Run Artifacts
adr-constraints:
  - ADR-0008: Knowledge Layer Architecture
  - ADR-0014: Harness Base Strategy
promoted-from:
  - FEAT-0015: Professional Harness Runtime
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

The BFF separates durable knowledge from ephemeral traces. The harness lets the
user inspect, accept, edit, or reject candidates when policy requires it.

### Active Memory Inspection

When memory influences a run, the user can inspect:

- memory item summary
- source run/artifact
- relevance reason
- scope: global, project, package, file, workflow
- age and confidence when available

### Quality-Driven Routing

Routing should select models or roles by workflow stage:

- context helper
- implementation
- validation summarizer
- repair
- reviewer
- documentation
- synthesizer

Routing decisions should record reason, cost, model capability, and outcome so
future tuning can improve quality/cost trade-offs.

### Workflow Extensions

Skills, hooks, slash commands, and agent teams should align with workflow
contracts:

- skills specialize one run or run stage
- hooks can warn, block, or enrich run/tool stages
- slash commands create or control runs
- agent teams execute as runs with multiple coordinated agents

Extensions may narrow tools, set model preferences, define artifact
requirements, or add validation behavior.

## UI / CLI / API Integration

Expected commands:

- `/memory` shows active and candidate memory
- `/memory accept|edit|reject <id>` handles candidates
- `/routing` shows model and role decisions for the run
- `/workflows` lists available workflow profiles
- existing `/skills` and `/team` surfaces should eventually reference workflow
  contracts

The BFF protocol should expose memory candidates, active memory provenance, and
routing explanations through run details.

## Configuration

Configuration should support:

- memory promotion policy
- memory scopes and retention
- workflow routing roles
- model preferences per workflow/stage
- extension manifests
- hook enablement and trust policy

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

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0008 | Memory candidates and retrieval build on the knowledge layer |
| ADR-0014 | Keeps routing and orchestration BFF-owned while the harness remains the terminal client |
| Future ADR | Should decide memory promotion defaults, routing role taxonomy, and extension trust boundaries |

## Open Questions

1. Which memory candidates should require user approval in solo mode?
2. How should quality outcomes be scored without overfitting to tests?
3. Should workflow extensions live in `.modeltap.yaml`, `.modeltap/`, or a
   plugin/skill packaging format?
4. When should the system compare multiple candidate patches instead of running
   one implementation plus review?
