# Implementation Plan: v0.3.4 — Memory, Routing, and Workflow Extensions

## Context

`v0.3.4` uses the run/artifact foundation to make modeltap improve over time:
successful work produces memory candidates, routing decisions become
quality-aware, and workflows/skills/teams align with the durable run contract.

This release is intentionally gated. The memory/routing slice can proceed once
run artifacts exist. The workflow-extension slice must coordinate with
FEAT-0011, FEAT-0012, and FEAT-0013 before acceptance.

## Scope

This release covers:

- memory/routing/extension trust ADR
- memory candidate extraction from successful runs
- active memory provenance
- memory candidate inspection and disposition
- routing roles for context helper, implementation, repair, reviewer,
  validation summarizer, documentation, and synthesizer
- routing decision/outcome capture
- workflow profile registry
- alignment plan for skills, hooks, slash commands, and agent teams with durable
  runs

This release does not cover:

- accepting FEAT-0011/0012/0013 without their own review processing
- marketplace/plugin distribution
- unbounded autonomous swarms
- candidate patch comparison unless separately authorized

## Feature Scope

- FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions
- FEAT-0011/0012/0013 coordination, as related/gated inputs

## Approach

Current phase: **Planning draft — Phase 1 not opened.**

If FEAT-0011, FEAT-0012, or FEAT-0013 are not accepted when this release is
opened, Phase 1 must either split this release or mark workflow-extension WUs as
deferred before design closes.

## Work Units

| WU | Title | Dependencies | Size | Feature |
|---|---|---|---|---|
| 147 | Memory, routing, and extension trust ADR | v0.3.2 | M | FEAT-0022 |
| 148 | Memory candidate schema and source-artifact links | 147 | M | FEAT-0022 |
| 149 | Candidate generation and disposition UI | 148 | L | FEAT-0022 |
| 150 | Active memory provenance in run details | 148, 149 | M | FEAT-0022 |
| 151 | Routing role taxonomy and policy config | 147 | M | FEAT-0022 |
| 152 | Routing decision/outcome capture | 151 | M | FEAT-0022 |
| 153 | Workflow profile and extension alignment design | 147, FEAT-0012/0013 coordination | L | FEAT-0022 |
| 154 | Memory/routing/workflow tests and docs | 148-153 | M | FEAT-0022 |

## Detailed WU Plan

### Track A — Decisions

**WU-147: Memory, routing, and extension trust ADR**

Decide memory promotion defaults, routing role taxonomy, skill/hook/team trust
boundaries, and which extension behaviors require accepted revisions to
FEAT-0011/0012/0013.

### Track B — Memory

**WU-148: Memory candidate schema and source-artifact links**

Define memory candidate records, source artifact references, scopes, retention,
confidence, and user disposition states.

**WU-149: Candidate generation and disposition UI**

Generate memory candidates from successful run artifacts and expose accept,
edit, reject, and defer controls in the harness. The default promotion policy
is no silent durable promotion: candidates require explicit user disposition
unless the WU-147 ADR accepts a narrower automatic mode.

**WU-150: Active memory provenance in run details**

Show which memory items influenced a run, why they were selected, their scope,
and their source run/artifact.

### Track C — Routing

**WU-151: Routing role taxonomy and policy config**

Define routing roles for helper, implementation, repair, validation summary,
review, documentation, and synthesis.

**WU-152: Routing decision/outcome capture**

Record model, role, reason, cost, validation/artifact outcome, and later tuning
signals per run stage. The routing-decision record must be concrete enough to
audit why a model or helper role was selected for each routed stage.

### Track D — Workflow Extensions

**WU-153: Workflow profile and extension alignment design**

Align skills, hooks, slash commands, and agent teams with durable run contracts.
This includes assigning a home for workflow slash commands such as `/explore`,
`/feature`, `/adr`, `/release`, `/implement`, `/debug`, `/docs`, and `/devops`;
it may defer command implementation, revise FEAT-0012/FEAT-0013, or produce a
constraint document before implementation.

**WU-154: Memory/routing/workflow tests and docs**

Add tests for candidate generation, memory provenance, routing explanation, and
workflow profile behavior. Document user commands and configuration.

## Phase 1 Design Checklist

- [ ] WU-147 ADR draft
- [ ] WU-148 to WU-150 memory design bundle
- [ ] WU-151 to WU-152 routing design bundle
- [ ] WU-153 extension alignment design and FEAT-0012/0013 coordination plan
- [ ] WU-154 verification/docs design

## Risk Register

- **R1 — dependency gates.** FEAT-0011/0012/0013 may still be proposed or need
  revision. Split the release if the extension slice cannot be accepted.
- **R2 — bad memory.** Memory candidates must be inspectable and reversible; do
  not silently promote noisy conclusions.
- **R3 — routing opacity.** Routing decisions must be explainable and recorded,
  or quality tuning becomes guesswork.
- **R4 — extension drift.** Skills and teams must not bypass durable runs.

## Definition of Done

1. Memory/routing/extension trust ADR is accepted or the release is split to
   isolate memory/routing.
2. Successful runs can produce source-linked memory candidates.
3. Users can inspect and disposition memory candidates.
4. Active memory provenance appears in run details.
5. Routing roles and decisions are recorded with outcomes.
6. Workflow extensions have a documented alignment path with FEAT-0012 and
   FEAT-0013.
