# Knowledge Layer (Cross-Model Brain)

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

## Key Capabilities

### Automatic Capture

Unlike manual "second brain" systems that require the user to explicitly log thoughts, modeltap captures knowledge passively from conversations the user is already having. Every prompt, every response, every decision discussed with any AI tool flows through the proxy and into the knowledge base.

### Cross-Model Continuity

A user working in Claude Code on a project can switch to ChatGPT for a different perspective, and ChatGPT (via MCP) can access the full context of what was discussed in Claude. No re-explaining, no context loss, no starting from zero.

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

### Privacy and Control

- All data stays local — no cloud sync, no third-party servers
- Users control what gets embedded (capture modes from ADR-0005 apply)
- Embedding can use local models (Ollama) for fully offline operation
- Retention policies control how long data is kept

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0002 (Storage) | Knowledge layer builds on top of SQLite store; vector embeddings may use SQLite vector extensions or a separate store |
| ADR-0005 (Capture Modes) | Capture mode determines what data is available for the knowledge layer to embed |
| ADR-0006 (Multi-Provider) | Knowledge layer only becomes valuable when modeltap captures from multiple providers |
| ADR-0007 (Usage Metrics) | Metrics and knowledge are complementary views of the same captured data |
| ADR-0008 (Knowledge Layer Architecture) | Formal decision on how to implement this feature |
| ADR-0009 (MCP Server) | Formal decision on how to expose knowledge via MCP |

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

### Phase 3: Advanced Features (future)

- Context injection: automatically inject relevant prior context into new requests
- Knowledge graph: relationships between decisions, people, projects
- Team knowledge: shared knowledge bases for teams (with access controls)
- Dashboard: web UI for browsing and visualizing knowledge patterns
