---
status: deferred
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0009: MCP Server for Knowledge Access

## Status

**Deferred** — this ADR depends on ADR-0008 (Knowledge Layer Architecture), which has been deferred. The MCP server decision will be revisited when the knowledge layer is implemented.

## Rationale for Deferral

The MCP server exposes the knowledge layer to AI clients. Without the knowledge layer, there is nothing to expose. The original analysis (MCP stdio transport scored 122, leading REST API by 21 points) remains valid as a starting point when this decision is revisited.

## When to revisit

Revisit this ADR when:

1. ADR-0008 (Knowledge Layer) is un-deferred and implemented.
2. MCP protocol stability is better established (the protocol is still evolving).
3. There is user demand for cross-tool knowledge access.

## Original proposal (preserved for reference)

The original ADR evaluated MCP (stdio transport), REST API, GraphQL, and direct SQLite access. MCP scored highest due to native AI-client integration, stdio security model, and ecosystem alignment. A complementary REST API was noted as a future addition for non-MCP clients.
