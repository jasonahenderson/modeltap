---
feature: FEAT-0018
title: Context Planner and Project Rules
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 3
depends-on:
  - FEAT-0016: Managed Codegen Run Pipeline
related:
  - EXP-0012: Code Graphing via AST for Repository-Aware Context
adr-constraints:
  - ADR-0014: Harness Base Strategy
---

# FEAT-0018: Context Planner and Project Rules

## Problem

Code generation quality depends heavily on whether the model sees the right
files, tests, rules, and local conventions before editing. Modeltap currently
supports explicit attachments and project config content, but it does not yet
create an inspectable context plan for implementation work.

Without context planning, the model may miss relevant files, invent project
patterns, ignore accepted decisions, or waste context on unrelated material.

## Solution

Add a context planner for implementation-quality runs. The planner selects and
budgets project rules, user attachments, repository-derived context, memory,
nearby style examples, and validation targets before prompt assembly. The runtime server
owns the durable context plan. The harness helps discover local filesystem and
repository facts, then displays context provenance to the user.

## Key Capabilities

### Project Rule Discovery

The harness discovers configured project-rule sources and sends their content
or metadata to the runtime server. Sources may include:

- `MODELTAP.md`
- `.modeltap/` rule files
- `AGENTS.md`
- `CLAUDE.md`
- `.modeltap.yaml`
- user/global config
- team/server policy metadata

Default precedence is:

`server policy > team policy > project config (.modeltap.yaml) > modeltap project rules (MODELTAP.md/.modeltap/) > compatible project rules (CLAUDE.md/AGENTS.md) > user config > global defaults`

Higher-precedence rules may mark settings as non-overridable. Conflicts are
recorded as warnings on the context-plan artifact with the winning source.
Richer policy-language behavior belongs in the future prompt/policy ADR.

### Repository-Aware Selection

For implementation and debug workflows, the planner should consider:

- user-mentioned files and symbols
- import/package relationships
- sibling and neighboring files
- tests associated with touched packages
- recent git changes
- files already read in the run
- local style examples
- accepted ADRs, features, patches, or release docs when relevant

The context plan snapshots the working tree at `context_plan` time, including
dirty changes. The plan records a fingerprint containing the current commit and
dirty-file digests. A context plan is frozen for one `model_call`; every
subsequent `model_call` in the same run requires either an explicit re-plan event
or reuse of the frozen plan with a recorded reason.

Context planning has a default deadline of 10 seconds. If planning exceeds the
deadline, the runtime server proceeds with the partial plan when policy allows, marks the
artifact `partial_plan`, and records omitted selectors. The harness may maintain
a repo-map cache invalidated by file mtimes; richer incremental indexing is
deferred to EXP-0012 follow-up work.

### Context Provenance

Every selected context item records why it was selected:

- directly attached by user
- mentioned in prompt
- imported by selected file
- test for selected package
- recent local change
- project rule
- memory retrieval
- workflow requirement
- extension
- parent run or synthesis input

The provenance vocabulary is extensible through a structured source field. The
harness exposes this through `/context` and run artifact inspection.

### Budgeting

The runtime server tracks token budgets by category:

- project rules
- selected files/snippets
- memory
- transcript/history
- validation evidence
- tool definitions

The planner may summarize, trim, or reject oversized context before model
dispatch. Default overflow behavior is:

- project rules and user attachments are pinned; overflow rejects dispatch with
  a budget-exceeded reason
- transcript/history is summarized
- selected files/snippets are trimmed least-recently-touched first
- memory is trimmed lowest-relevance first
- validation evidence is summarized, then trimmed by age

These defaults are configurable and recorded in the context-plan artifact.
Per-rule-source content is capped by a configurable byte budget, defaulting to
32 KiB; over-cap rule sources are summarized with a warning, but pinned-category
overflow still rejects the run.

Token estimates use the selected model's tokenizer when available. Otherwise the
planner uses a conservative provider-class estimator. Any routing-driven model
change requires re-budgeting, and the tokenizer or estimator used is recorded on
the plan artifact.

If memory retrieval is unavailable or degraded, the default behavior is skip with
warning and mark the plan `memory_unavailable`. Implementation and devops
workflows may escalate to `waiting_user` so the user can proceed without memory
or wait for recovery.

Full prompt content is runtime-owned and is not transmitted to the harness by
default. The harness sees prompt-layer metadata such as layer name, byte budget,
source category, and provenance, but not raw prompt text. A user-controlled
debug flag may permit prompt content disclosure to the harness for inspection;
that disclosure is scoped per run, requested at run start, off by default,
recorded on the run and disclosure artifact, and may be forbidden by team/server
policy.

## UI / CLI / API Integration

Expected commands:

- `/context` shows active context and provenance
- `/context rules` shows rule sources and precedence
- `/context why <item>` explains why an item was selected
- `/context drop <item>` excludes an item from the next run when policy allows

`<item>` refers to the context-item ID shown by `/context` for the active
context plan.

The protocol needs a context-plan structure correlated with a run ID and exposed
through run details.

## Configuration

Configuration should support:

- additional project rule filenames
- context budget percentages by category
- maximum file/snippet count
- whether to ingest `AGENTS.md` and `CLAUDE.md`
- ignored paths and generated-file patterns
- context-plan deadline and repo-map cache behavior
- project-rule source byte caps

## Non-Goals

- This feature does not implement durable memory promotion; see FEAT-0022.
- This feature does not implement validation execution; see FEAT-0019.
- This feature does not require full semantic indexing in the first release.

## Success Criteria

1. Implementation runs include a context plan before model dispatch.
2. Project rules are discovered with deterministic precedence.
3. Selected files, snippets, tests, rules, and memory items include provenance.
4. The user can inspect active context through the harness.
5. Oversized context is budgeted, summarized, or rejected before provider
   dispatch.
6. The planner improves context selection without requiring the user to attach
   every relevant file manually.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Keeps intelligence in the runtime server while the harness supplies local repo facts |
| Future ADR | Should decide project-rule precedence and prompt-layer ownership |

## Open Questions

1. How much repo-map construction belongs in the harness versus the runtime server?
2. Should AST/symbol indexing from EXP-0012 be required for the first version?
