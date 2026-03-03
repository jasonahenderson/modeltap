---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# MCP Server for Knowledge Access

## Context and Problem Statement

The knowledge layer (ADR-0008) builds a searchable, embedded knowledge base from captured interactions. To be useful across AI tools, this knowledge must be accessible programmatically by any AI client — not just through the modeltap CLI. The Model Context Protocol (MCP) is an emerging standard for exposing tools and resources to AI assistants. The decision is how to expose modeltap's knowledge base to external AI clients: via MCP, a REST API, or another interface.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Cross-tool accessibility (5):** The core promise of the knowledge layer is that any AI tool can access the user's full interaction history. The access interface must work with Claude Code, Cursor, and other AI clients without requiring per-client integrations.
* **D2 – Semantic richness of the interface (5):** AI clients need more than raw data — they need tools that return contextually useful results. "Search my past decisions about authentication" should return ranked, relevant results with enough context for the AI to use them effectively.
* **D3 – Implementation complexity (4):** Modeltap is a developer tool built by a small team. The access interface should be implementable without a large framework or complex server infrastructure.
* **D4 – Security and access control (4):** The knowledge base contains every interaction a user has had with every AI tool. Access must be controlled — only the local user should be able to query it, and no data should leave the machine without explicit consent.
* **D5 – Discoverability of capabilities (3):** AI clients should be able to discover what tools and resources modeltap exposes without hardcoded knowledge. The interface should be self-describing.
* **D6 – Ecosystem alignment (3):** Choosing an interface that aligns with the direction the AI tooling ecosystem is moving reduces future migration risk and increases adoption.
* **D7 – Offline and local operation (3):** The access interface must work fully offline, consistent with modeltap's local-first architecture (ADR-0002, ADR-0008).

## Considered Options

* MCP server (stdio transport)
* REST API (localhost HTTP)
* GraphQL API
* Direct SQLite access (shared database file)

## Decision Outcome

Chosen option: **MCP server (stdio transport)**, because it achieves the highest weighted score (122) and directly addresses the core use case of cross-tool knowledge access. MCP is purpose-built for AI clients to discover and invoke tools, and the stdio transport means the server runs as a subprocess of the AI client — no network listener, no port management, no firewall concerns. Claude Code, Cursor, and other MCP-compatible clients can connect to modeltap's knowledge base with a single configuration entry.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | MCP (stdio) | REST API | GraphQL | Direct SQLite |
|-------------------------------------|--------|-------------|----------|---------|---------------|
| D1: Cross-tool accessibility        | 5      | 5           | 4        | 3       | 2             |
| D2: Semantic richness               | 5      | 5           | 4        | 4       | 2             |
| D3: Implementation complexity       | 4      | 4           | 5        | 3       | 5             |
| D4: Security / access control       | 4      | 5           | 3        | 3       | 4             |
| D5: Discoverability                 | 3      | 5           | 3        | 4       | 1             |
| D6: Ecosystem alignment             | 3      | 5           | 3        | 2       | 1             |
| D7: Offline / local operation       | 3      | 5           | 4        | 4       | 5             |
| **Weighted Total**                  |        | **122**     | **101**  | **88**  | **80**        |

### Scoring Justification

#### MCP server – stdio transport (122)

* **D1 (5):** MCP is designed for exactly this use case — AI clients discovering and invoking external tools. Claude Code, Cursor, and other editors with MCP support can connect to modeltap's knowledge base with a config entry pointing to the `modeltap mcp` command. No per-client integration code needed.
* **D2 (5):** MCP tools are semantically described — each tool has a name, description, and typed input schema. The AI client understands what `search_knowledge` does, what parameters it accepts, and what results to expect. This enables natural-language-driven queries: the user says "what did I decide about caching?" and the AI client invokes the right tool with the right parameters.
* **D3 (4):** MCP server implementation in Go requires handling JSON-RPC 2.0 over stdio. Libraries exist (e.g., `mark3labs/mcp-go`) or the protocol is simple enough to implement directly. More work than a basic HTTP handler but less than GraphQL. Scored 4 rather than 5 because MCP Go libraries are still maturing.
* **D4 (5):** Stdio transport means the MCP server is a subprocess of the AI client — there is no network listener, no open port, no attack surface. Only the local user's AI client can communicate with the server. Data never leaves the machine unless the AI client explicitly sends it somewhere.
* **D5 (5):** MCP's `tools/list` and `resources/list` methods are purpose-built for capability discovery. AI clients call these methods at connection time to learn what modeltap can do. Self-describing by design.
* **D6 (5):** MCP is backed by Anthropic and adopted by multiple AI tools. It is the emerging standard for tool-to-AI-client communication. Aligning with MCP positions modeltap for maximum ecosystem compatibility.
* **D7 (5):** Stdio transport requires no network. The MCP server reads from stdin, writes to stdout, and accesses the local SQLite database. Fully offline by design.

#### REST API – localhost HTTP (101)

* **D1 (4):** Any HTTP client can call a REST API, including AI clients with web request capabilities. But AI clients do not natively discover REST endpoints — each client would need custom integration or a plugin to know modeltap's API exists and how to call it.
* **D2 (4):** REST endpoints can return rich, structured responses. But REST lacks the semantic tool description that MCP provides — the AI client does not automatically know what `/api/search` does or what parameters it accepts without hardcoded knowledge.
* **D3 (5):** REST APIs in Go are trivially implemented with `net/http`. Well-understood patterns, abundant libraries, easy to test. Simplest implementation option.
* **D4 (3):** A localhost HTTP listener is accessible to any process on the machine. Requires token-based authentication or similar to prevent unauthorized access. Firewalls and port conflicts are additional concerns.
* **D5 (3):** REST APIs can be documented with OpenAPI, but AI clients do not natively discover REST endpoints. Discovery requires manual configuration or a service registry.
* **D6 (3):** REST is universal but not AI-client-specific. The AI tooling ecosystem is moving toward MCP for tool integration. REST would work but is not the direction of travel.
* **D7 (4):** Fully local but requires a network listener. Port conflicts, process management, and "is the server running?" are operational concerns that stdio transport avoids.

#### GraphQL API (88)

* **D1 (3):** Same discoverability limitations as REST, plus GraphQL support in AI clients is less common than REST support.
* **D2 (4):** GraphQL's typed schema and flexible queries are well-suited for knowledge retrieval. Clients can request exactly the fields they need.
* **D3 (3):** GraphQL in Go requires a schema definition, resolver implementation, and a GraphQL library (gqlgen, graphql-go). More implementation complexity than REST for limited additional benefit in this use case.
* **D4 (3):** Same localhost security concerns as REST.
* **D5 (4):** GraphQL's introspection is built-in — clients can query the schema to discover available queries and types. Better self-description than REST.
* **D6 (2):** GraphQL is not the direction AI client tooling is moving. Over-engineered for this use case.
* **D7 (4):** Same as REST — fully local but requires a network listener.

#### Direct SQLite access (80)

* **D1 (2):** Requires each AI client to know modeltap's database schema and query it directly. No abstraction layer. Schema changes break all clients. Only works for clients that can open SQLite files.
* **D2 (2):** Raw SQL queries return raw data. No semantic tool descriptions, no structured knowledge retrieval. The AI client must know the exact schema and write SQL.
* **D3 (5):** No server to implement — clients just open the SQLite file. Simplest possible "implementation."
* **D4 (4):** File permissions control access. No network exposure. But concurrent access from multiple clients and the proxy itself requires careful WAL mode configuration to avoid locking issues.
* **D5 (1):** No discoverability. Clients must know the schema in advance.
* **D6 (1):** No AI tooling ecosystem supports direct SQLite access as a standard integration pattern.
* **D7 (5):** Fully local, no network, no server process. Maximum simplicity for offline operation.

### Consequences

* Good, because MCP provides native AI-client integration — users add one config entry and every MCP-compatible tool gains access to their cross-model knowledge base.
* Good, because stdio transport eliminates all network security concerns — no open ports, no authentication tokens, no firewall configuration.
* Good, because MCP's tool discovery means AI clients automatically learn what modeltap can do, enabling natural-language-driven knowledge queries.
* Good, because the architecture aligns with the emerging AI tooling ecosystem, positioning modeltap for broad adoption.
* Neutral, because MCP Go libraries are still maturing, but the protocol is simple enough to implement directly if needed.
* Bad, because non-MCP clients cannot access the knowledge base without an MCP-to-HTTP bridge or a separate REST API (which could be added later as a complementary interface).
* Bad, because MCP is still an emerging standard — if adoption stalls or the protocol changes significantly, modeltap's integration layer would need updating.

### Confirmation

The decision will be confirmed by:

1. Implementing `modeltap mcp` as a stdio-based MCP server that exposes knowledge layer tools.
2. Connecting Claude Code to modeltap's MCP server and successfully querying past interactions via natural language.
3. Verifying that Cursor or another MCP-compatible client can also connect without modeltap-specific code changes.
4. Confirming that the MCP server starts in under 1 second and adds no measurable overhead to the proxy core.

## More Information

The decision aligns with the weighted scoring matrix. MCP leads by 21 points over REST, reflecting the strong alignment between MCP's design goals and modeltap's cross-tool knowledge access use case.

### MCP Server Tools (planned)

```json
{
  "tools": [
    {
      "name": "search_knowledge",
      "description": "Semantic search across all captured AI interactions. Returns the most relevant past conversations, decisions, and context.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "query": { "type": "string", "description": "Natural language search query" },
          "limit": { "type": "integer", "default": 10 },
          "provider": { "type": "string", "description": "Filter by provider (anthropic, openai, etc.)" },
          "since": { "type": "string", "description": "ISO 8601 date to filter results after" }
        },
        "required": ["query"]
      }
    },
    {
      "name": "list_recent",
      "description": "List recent AI interactions, optionally filtered by provider, model, or project.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "limit": { "type": "integer", "default": 20 },
          "provider": { "type": "string" },
          "model": { "type": "string" },
          "project": { "type": "string" }
        }
      }
    },
    {
      "name": "get_context",
      "description": "Retrieve the full context of a specific past interaction, including request and response.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "request_id": { "type": "string", "description": "The ID of the interaction to retrieve" }
        },
        "required": ["request_id"]
      }
    },
    {
      "name": "get_decisions",
      "description": "Surface decisions and action items extracted from past AI conversations.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "since": { "type": "string" },
          "project": { "type": "string" },
          "limit": { "type": "integer", "default": 20 }
        }
      }
    },
    {
      "name": "get_stats",
      "description": "Usage statistics and knowledge base overview — total interactions, providers used, models, topics.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "since": { "type": "string" }
        }
      }
    }
  ]
}
```

### MCP Configuration Example

Users configure their AI client to connect to modeltap's MCP server:

```json
{
  "mcpServers": {
    "modeltap": {
      "command": "modeltap",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

A complementary REST API (`modeltap api`) may be added in a future iteration for non-MCP clients, web dashboards, and programmatic access. This does not conflict with the MCP-first decision — it would be an additional interface, not a replacement.
