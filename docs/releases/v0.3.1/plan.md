# Implementation Plan: v0.3.1 — Context Planner and Project Rules

## Context

`v0.3.0` creates durable run infrastructure. `v0.3.1` makes implementation
runs better by planning the context that enters the model call: project rules,
repo facts, user attachments, memory placeholders, style examples, and
provenance.

This release implements FEAT-0018 and the project-rule/prompt-layer ADR work
called out during FEAT-0015 review processing.

## Scope

This release covers:

- project-rule and prompt-layer ADR
- project rule discovery for `MODELTAP.md`, `.modeltap/`, `AGENTS.md`,
  `CLAUDE.md`, `.modeltap.yaml`, user/global config, and server/team policy
  metadata
- context plan schema and run correlation
- lightweight repo map v1: files, packages, imports, tests, recent changes
- style sampling from nearby code
- context provenance and budget accounting
- `/context`, `/context rules`, and `/context why`

This release does not cover:

- full AST/symbol indexing beyond a lightweight repo map
- validation execution or repair loops
- patch evidence bundles
- memory promotion

## Feature Scope

- FEAT-0018: Context Planner and Project Rules
- FEAT-0016: run integration from v0.3.0
- EXP-0012 as advisory input for future deeper AST work

## Approach

Current phase: **Planning draft — Phase 1 not opened.**

## Work Units

| WU | Title | Dependencies | Size | Feature |
|---|---|---|---|---|
| 118 | Project rules and prompt-layer ADR | v0.3.0 | M | FEAT-0018 |
| 119 | Context plan schema and protocol surface | 118 | M | FEAT-0018 |
| 120 | Harness project-rule discovery and precedence reporting | 118 | M | FEAT-0018 |
| 121 | Lightweight repo map and recent-change scanner | 119 | L | FEAT-0018 |
| 122 | Test and style-context discovery | 121 | M | FEAT-0018 |
| 123 | runtime server context planner and budget accounting | 119-122 | L | FEAT-0018 |
| 124 | Prompt-plan metadata and context provenance capture | 123 | M | FEAT-0018 |
| 125 | Harness `/context` inspection surfaces | 123, 124 | M | FEAT-0018 |
| 126 | Context planner verification and docs | 120-125 | M | FEAT-0018 |

## Detailed WU Plan

### Track A — ADR and Protocol

**WU-118: Project rules and prompt-layer ADR**

Decide rule precedence, how `MODELTAP.md` coexists with `AGENTS.md` and
`CLAUDE.md`, which prompt layers are runtime-owned, and what prompt metadata can be
shown safely. Align the prompt metadata taxonomy with the v0.3.0 run prompt
and `turn.submit` compatibility decisions.

**WU-119: Context plan schema and protocol surface**

Define run-correlated context plan payloads, provenance records, budget
categories, and inspection methods. This WU activates the `context_plan`
pipeline stage first defined by v0.3.0.

### Track B — Harness Context Discovery

**WU-120: Harness project-rule discovery and precedence reporting**

Discover local rule files and config sources. Send content/metadata according
to ADR policy and report conflicts or over-budget rule files.

**WU-121: Lightweight repo map and recent-change scanner**

Build a cheap repo map from file tree, package/module boundaries, imports where
easy, git status/diff, and configured ignore/generated patterns. Define cost
ceilings so repo-map generation remains bounded on large workspaces.

**WU-122: Test and style-context discovery**

Infer nearby tests and local style examples from selected files and packages.
No full AST dependency in this release.

### Track C — runtime server Planner and Harness UI

**WU-123: runtime server context planner and budget accounting**

Assemble a context plan from user attachments, rule sources, repo map input,
recent changes, style samples, and memory placeholders. Track budget by
category.

**WU-124: Prompt-plan metadata and context provenance capture**

Store prompt/context metadata on the run without exposing protected prompt
content by default. Real `context_plan` stage events must link to the stored
context plan and provenance records.

**WU-125: Harness `/context` inspection surfaces**

Expose active context, rule sources, precedence, provenance, and why-selected
answers through `/context`, `/context rules`, and `/context why`.

### Track D — Verification

**WU-126: Context planner verification and docs**

Add fixture repos and tests for rule precedence, provenance, budget trimming,
path ignores, and `/context` rendering.

## Phase 1 Design Checklist

- [ ] WU-118 ADR draft/design
- [ ] WU-119 protocol/schema design
- [ ] WU-120 to WU-122 harness discovery design bundle
- [ ] WU-123 to WU-125 planner/UI design bundle
- [ ] WU-126 verification/docs design

## Risk Register

- **R1 — rule precedence conflict.** Multiple rule files can disagree. The ADR
  must define deterministic ordering and visibility.
- **R2 — repo-map cost.** Keep v1 lightweight; deeper AST graphing remains
  downstream.
- **R3 — prompt leakage.** Metadata inspection must not expose protected or
  secret-bearing prompt content by default.

## Definition of Done

1. Project-rule/prompt-layer ADR is accepted.
2. Implementation runs produce context plans with provenance.
3. The runtime server budgets context categories before model dispatch.
4. The harness shows active context and why-selected details.
5. Tests cover rule discovery, provenance, budget behavior, and UI inspection.
