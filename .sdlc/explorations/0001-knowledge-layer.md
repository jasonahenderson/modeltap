---
exploration: EXP-0001
title: Knowledge Layer (Cross-Model Brain)
status: promoted
date: 2026-03-03
related:
  - ADR-0008: Knowledge layer architecture
  - ADR-0009: MCP server for knowledge access
promoted-to:
  - PATCH-0001
---

# EXP-0001: Knowledge Layer (Cross-Model Brain)

## Overview

Modeltap sits between all AI/ML clients and all model API endpoints, capturing every request and response. This unique position makes it the ideal foundation for a **cross-model knowledge layer** — a persistent, searchable, agent-readable memory system that works across every AI tool a user interacts with.

The knowledge layer is an optional module built on top of the proxy core. When enabled, it transforms modeltap from a logging proxy into cross-model knowledge infrastructure.

## Problem Statement

Today's AI memory is fragmented:

- Claude's memory doesn't know what you told ChatGPT
- ChatGPT's memory doesn't follow you into Cursor
- Your phone app doesn't share context with your coding agent
- Every platform has built a walled garden of memory — none of them talk to each other

Users who switch between models, or who want to try a new tool, lose all accumulated context. This creates artificial lock-in and degrades the quality of AI interactions across the board.

Even when history is available, users still lack good control over **what context should stay live right now**:

- Long-running sessions bloat with stale messages, repeated plans, and irrelevant detours
- `/clear` and "new chat" semantics are inconsistent across tools and usually scoped to a single vendor
- Users have no neutral assistant that can inspect the current context window and suggest whether to compact, summarize, preserve, or drop parts of it
- Important working state often dies at model boundaries, even when the user wants the session to continue with a different model

## How Modeltap Solves This

Since modeltap already captures every interaction across every provider, it can:

1. **Build a unified knowledge base** from conversations the user is already having — no manual capture required
2. **Make that knowledge semantically searchable** via vector embeddings, so "what did I decide about authentication?" works even if the word "authentication" was never used
3. **Expose the knowledge via MCP** so any AI client can query the user's full history, regardless of which model or tool the original conversation happened in

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Claude Code  │     │   ChatGPT   │     │   Cursor    │
│   VS Code    │     │   OpenAI    │     │   Ollama    │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────────────┼────────────────────┘
                            │
                    ┌───────▼────────┐
                    │   Modeltap     │
                    │   Proxy Core   │
                    │                │
                    │  ┌───────────┐ │
                    │  │ Capture & │ │
                    │  │  Logging  │ │
                    │  └─────┬─────┘ │
                    └────────┼───────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
      ┌───────▼──────┐ ┌────▼─────┐ ┌──────▼───────┐
      │   SQLite     │ │ Vector   │ │  MCP Server  │
      │  Request Log │ │Embeddings│ │  (Knowledge  │
      │  (ADR-0002)  │ │ (opt-in) │ │   Access)    │
      └──────────────┘ └──────────┘ └──────────────┘
```

### Proxy Core (always on)

- Reverse proxy with SSE stream capture
- Full request/response logging to SQLite
- Usage metrics tracking
- This is modeltap's v1 — useful on its own

### Knowledge Layer (opt-in module)

When enabled via configuration:

- **Vector Embedding**: Captured conversations are asynchronously embedded using a configurable embedding model (e.g., OpenAI `text-embedding-3-small`, local models via Ollama). Embeddings are stored alongside the request log.
- **Semantic Search**: Query past interactions by meaning, not just keywords or timestamps. "What approaches did I consider for database caching?" finds relevant conversations even if those exact words weren't used.
- **Metadata Extraction**: Automatically extract decisions, action items, people, topics, and key concepts from captured conversations.
- **MCP Server**: Expose the knowledge base via Model Context Protocol, allowing any MCP-compatible AI client to query the user's full cross-model history.
- **Context Curation**: Build an active layer on top of captured history that can score recency, relevance, duplication, and user-pinned importance to determine what belongs in the live context window.
- **Session State**: Track session-level state that survives model switches, so commands like "compact this session and continue in another model" operate on a portable session object rather than a single vendor thread.

## Key Capabilities

### Automatic Capture

Unlike manual "second brain" systems that require the user to explicitly log thoughts, modeltap captures knowledge passively from conversations the user is already having. Every prompt, every response, every decision discussed with any AI tool flows through the proxy and into the knowledge base.

### Cross-Model Continuity

A user working in Claude Code on a project can switch to ChatGPT for a different perspective, and ChatGPT (via MCP) can access the full context of what was discussed in Claude. No re-explaining, no context loss, no starting from zero.

### Context Curation and Compaction Guidance

The knowledge layer should not only retrieve prior context, but help the user decide what to do with the current context window.

- Detect context bloat: repeated instructions, obsolete branches, superseded decisions, low-signal chatter
- Distinguish between content that should be dropped, summarized, pinned, or kept verbatim
- Support an interactive "chat with my context" workflow where the user can ask:
  - "What should I compact before switching models?"
  - "Which parts of this thread are still carrying useful state?"
  - "What can I safely prune if I want a smaller working set?"
- Produce explicit compaction plans with tradeoffs, instead of opaque summarization

This makes the knowledge layer an active context steward, not just a passive archive.

### Portable Session Control

Modeltap should provide session controls that span providers and clients:

- `/clear`: start a fresh live session while preserving prior captured history in the knowledge base
- `/compact`: replace verbose live context with a curated compact state derived from the current session
- `/pin`: mark context fragments, decisions, or constraints as mandatory carry-forward state
- `/unpin`: remove previously pinned state from the portable session context
- `/handoff`: package the curated session state for continuation in another model or client

These commands operate on modeltap's session abstraction, not a provider-specific conversation primitive, so the same workflow works across Anthropic, OpenAI, Ollama, and future providers.

### Semantic Search Across All Interactions

```
modeltap search "authentication approaches I considered"
```

Returns semantically relevant results from conversations across all providers, all tools, all time periods — ranked by meaning similarity, not keyword match.

### MCP Server Tools

When the MCP server is enabled, AI clients gain access to tools such as:

- **search_knowledge**: Semantic search across all captured interactions
- **list_recent**: Browse recent interactions, optionally filtered by provider, model, or project
- **get_context**: Retrieve full context for a specific interaction
- **get_decisions**: Surface decisions and action items extracted from past conversations
- **get_stats**: Usage patterns and knowledge base statistics
- **inspect_context**: Analyze the active session context for redundancy, staleness, token pressure, and pinned state
- **suggest_compaction**: Return a structured compaction or pruning plan, including what to summarize, keep verbatim, or discard
- **apply_session_command**: Execute session controls such as `/clear`, `/compact`, `/pin`, `/unpin`, and `/handoff`
- **export_session_state**: Emit a portable session bundle that another model or client can import as continuation context

### Privacy and Control

- All data stays local — no cloud sync, no third-party servers
- Users control what gets embedded (capture modes from ADR-0005 apply)
- Embedding can use local models (Ollama) for fully offline operation
- Retention policies control how long data is kept
- Context curation policies remain user-visible and inspectable; modeltap should not silently discard state the user expects to keep

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0002 (Storage) | Knowledge layer builds on top of SQLite store; vector embeddings may use SQLite vector extensions or a separate store |
| ADR-0005 (Capture Modes) | Capture mode determines what data is available for the knowledge layer to embed |
| ADR-0006 (Multi-Provider) | Knowledge layer only becomes valuable when modeltap captures from multiple providers |
| ADR-0007 (Usage Metrics) | Metrics and knowledge are complementary views of the same captured data |
| ADR-0008 (Knowledge Layer Architecture) | Formal decision on how to implement this feature |
| ADR-0009 (MCP Server) | Formal decision on how to expose knowledge via MCP |

## Implementation Options

The context-management layer should be specified in terms of modeltap-owned interfaces such as session state, pinned context, compaction plans, and handoff bundles. The implementation behind those interfaces can vary.

### Option 1: Native Modeltap Implementation

- Store portable session state directly in modeltap's SQLite database alongside captured interactions
- Implement compaction, pruning suggestions, and carry-forward policies inside modeltap
- Keep the smallest operational surface and the strongest alignment with modeltap's local-first architecture

This is the default architectural direction because it preserves modeltap's role as a neutral cross-client substrate rather than an agent framework.

### Option 2: MemGPT / Letta-Style Hierarchical Memory

MemGPT, now evolved into Letta, is a credible implementation reference for this feature area. Its memory hierarchy maps well to modeltap's context-management goals:

- in-context memory for always-visible working state
- recall memory for recent conversational history
- archival memory for larger searchable history
- automatic compaction or summarization when the live context window fills

This makes Letta-style memory a strong candidate for:

- `inspect_context`
- `suggest_compaction`
- session summarization and carry-forward bundle generation
- deciding what remains verbatim versus what is compressed into durable state

However, modeltap should not adopt Letta's runtime abstractions as its public product model. The risks are:

- coupling modeltap to one agent framework's concepts and lifecycle
- weakening modeltap's cross-client neutrality
- turning a knowledge/session substrate into a full agent runtime by accident

Recommended stance: treat MemGPT/Letta as an optional backend or plugin for compaction and memory-management logic, not as the canonical session abstraction.

### Option 3: Workflow Runtime Integration

Frameworks such as LangGraph may also be useful, especially where context management overlaps with workflow state, checkpointing, and resumability.

- Good fit for stateful execution graphs and resumable workflows
- Weaker fit as the canonical memory substrate for all captured traffic across independent clients

These runtimes are better viewed as consumers of modeltap session state than as the owner of it.

### Recommendation

- Modeltap owns the canonical session abstraction and cross-model commands
- External memory runtimes are optional implementation strategies behind modeltap-managed interfaces
- Any integration must preserve local-first operation, inspectable policies, and provider-neutral behavior

## Phasing

### Phase 1: Proxy Core (v1)

- Reverse proxy with SSE capture
- Request/response logging
- Usage metrics
- CLI for querying logs
- No knowledge layer

### Phase 2: Knowledge Layer (v2)

- Vector embedding of captured interactions
- Semantic search via CLI
- Metadata extraction
- MCP server for knowledge access
- Context inspection and curation APIs
- Portable session state and cross-model session commands

### Phase 3: Advanced Features (future)

- Context injection: automatically inject relevant prior context into new requests
- Knowledge graph: relationships between decisions, people, projects
- Team knowledge: shared knowledge bases for teams (with access controls)
- Dashboard: web UI for browsing and visualizing knowledge patterns
- Policy-driven compaction profiles (coding, research, planning, support)
- Automatic session handoff between clients and models
