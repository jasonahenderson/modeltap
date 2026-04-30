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
nearby style examples, and validation targets before prompt assembly. The BFF
owns the durable context plan. The harness helps discover local filesystem and
repository facts, then displays context provenance to the user.

## Key Capabilities

### Project Rule Discovery

The harness discovers configured project-rule sources and sends their content
or metadata to the BFF. Sources may include:

- `MODELTAP.md`
- `.modeltap/` rule files
- `AGENTS.md`
- `CLAUDE.md`
- `.modeltap.yaml`
- user/global config
- team/server policy metadata

The planner must define deterministic precedence and conflict behavior.

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

The harness exposes this through `/context` and run artifact inspection.

### Budgeting

The BFF tracks token budgets by category:

- project rules
- selected files/snippets
- memory
- transcript/history
- validation evidence
- tool definitions

The planner may summarize, trim, or reject oversized context before model
dispatch.

## UI / CLI / API Integration

Expected commands:

- `/context` shows active context and provenance
- `/context rules` shows rule sources and precedence
- `/context why <item>` explains why an item was selected
- `/context drop <item>` excludes an item from the next run when policy allows

The protocol needs a context-plan structure correlated with a run ID and exposed
through run details.

## Configuration

Configuration should support:

- additional project rule filenames
- context budget percentages by category
- maximum file/snippet count
- whether to ingest `AGENTS.md` and `CLAUDE.md`
- ignored paths and generated-file patterns

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
| ADR-0014 | Keeps intelligence in the BFF while the harness supplies local repo facts |
| Future ADR | Should decide project-rule precedence and prompt-layer ownership |

## Open Questions

1. Should `MODELTAP.md` take precedence over `AGENTS.md`, or should modeltap
   ingest all compatible rule files with explicit source labels?
2. How much repo-map construction belongs in the harness versus the BFF?
3. Should AST/symbol indexing from EXP-0012 be required for the first version?
4. What context categories are pinned and what categories may be trimmed?
