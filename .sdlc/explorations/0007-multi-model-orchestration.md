---
exploration: EXP-0007
title: Multi-Model Orchestration
status: exploring
date: 2026-04-10
related:
  - ADR-0006: Multi-provider support
  - ADR-0008: Knowledge layer architecture
  - ADR-0009: MCP server for knowledge access
  - EXP-0001
  - PATCH-0002
---

# EXP-0007: Multi-Model Orchestration

## Problem

Modeltap already sits on the traffic boundary between users and multiple model providers, but today it treats each request as an isolated interaction. Real work increasingly spans more than one model:

- A user wants one model to research, another to draft, and a third to review
- A user wants parallel opinions from multiple models before deciding
- A user wants to break a large task into smaller units and route each unit to the model best suited for coding, summarization, critique, or planning
- A user wants these workflows without rebuilding context and routing logic inside every client

Without a neutral orchestration layer, users manually copy prompts between tools, pay repeated context costs, and lose visibility into why a given model was chosen for a subtask.

## Solution

Add an orchestration layer above the proxy and knowledge systems that can:

1. Accept a user task and decompose it into sub-units
2. Select execution patterns such as serial, parallel, debate, review, or research synthesis
3. Route each subtask to the best available model using configurable policies
4. Recombine outputs into a coherent result with traceable provenance
5. Use the knowledge layer's curated session state as shared working memory across the workflow

This is a distinct feature from the knowledge layer. The knowledge layer manages memory, context, and session continuity; orchestration manages execution strategy across models.

## Key Capabilities

### Workflow Patterns

- **Serial pipelines**: model A researches, model B drafts, model C reviews or critiques
- **Parallel comparison**: multiple models answer the same question, then modeltap or a selected model synthesizes differences
- **Research fan-out**: split a broad question into sub-questions, dispatch in parallel, merge results into a final brief
- **Think-then-act flows**: use a cheaper reasoning or planning step to structure work before sending implementation tasks to a stronger coding model
- **Reviewer loops**: run secondary review passes for correctness, safety, or style before the final answer is returned

### Task Decomposition and Routing

- Break large prompts into smaller work units such as research, extraction, coding, testing, summarization, or review
- Route units according to configurable policies:
  - best coding model
  - cheapest acceptable model
  - local-only execution
  - fastest model under a quality threshold
  - required provider or forbidden provider lists
- Allow user overrides while still exposing the system's default routing rationale

### Shared Session State

- Consume curated context from EXP-0001 instead of naively replaying raw transcripts into every subtask
- Allow subtasks to inherit pinned constraints, goals, and accepted decisions from the session state
- Record which outputs are promoted back into shared memory, preventing uncontrolled context growth

### Observability

- Show which model handled each unit, with timing, token usage, and cost
- Preserve the execution graph: parent task, subtasks, dependencies, outputs, and merges
- Let users inspect why a model was selected and what context bundle it received

## Example Workflows

### Serial Review

1. User asks for a design proposal
2. Model A generates the proposal
3. Model B critiques risks and edge cases
4. Model C rewrites the proposal incorporating the critique

### Parallel Research

1. User asks for a technology comparison
2. Modeltap decomposes the request into separate research questions
3. Multiple models work those questions in parallel
4. A synthesis step merges the results and highlights disagreement

### Task Routing for Build Work

1. User submits a broad implementation task
2. Pre-processing extracts planning, coding, and review units
3. Planning runs on a lower-cost reasoning model
4. Coding routes to the strongest code-generation model
5. Review routes to a different model to reduce same-model blind spots

## Non-Goals

- Replacing the user's primary AI client UI
- Fully autonomous long-running agent swarms with no user visibility
- Hardcoded "best model" rankings inside the product; routing should be policy-driven and configurable
- Provider-specific workflow logic that cannot generalize beyond Anthropic or OpenAI

## Success Criteria

1. A user can define a serial or parallel workflow once and run it across different providers without changing clients.
2. Modeltap can decompose a broad task into typed sub-units and route them using configurable policies.
3. Every subtask execution records model, provider, latency, token usage, cost, and parent-child lineage.
4. The final synthesis step can consume outputs from multiple providers while preserving links back to source subtasks.
5. Curated session state from EXP-0001 can be shared across subtasks without replaying full raw history each time.

## Relationship to Other Features and ADRs

- Depends on ADR-0006 because model selection only matters when multiple providers are supported.
- Builds on EXP-0001 because shared memory, compaction, and session control are prerequisites for efficient orchestration.
- Uses ADR-0009's MCP-oriented interface as a likely control surface for invoking workflows from external clients.
- Complements PATCH-0002 by making local models eligible routing targets alongside cloud providers.

## Implementation Note

If modeltap integrates external memory or workflow runtimes such as Letta/MemGPT or LangGraph, orchestration should consume those capabilities through modeltap-owned interfaces. Workflow engines may help execute decomposition, routing, and synthesis, but modeltap should remain the owner of provider-neutral routing policy, execution metadata, and cross-model session continuity.

## Open Questions

- Should workflow definitions live in config files, MCP tools, CLI subcommands, or all three?
- How much decomposition should be deterministic rules versus model-generated planning?
- Should synthesis always be explicit as its own step, or can some workflows stream partial results directly to the user?
- Do users need reusable named workflows first, or ad hoc orchestration from natural language is enough for v1?
