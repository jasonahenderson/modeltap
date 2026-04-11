---
exploration: EXP-0002
title: Multi-User Support
status: exploring
date: 2026-03-03
related:
  - ADR-0002: Storage format (per-user database scoping implications)
  - ADR-0006: Multi-provider support
---

# EXP-0002: Multi-User Support

## Overview

Modeltap v1 is designed as a single-user proxy — one user, one database, one configuration. Multi-user support extends modeltap to serve teams, organizations, and shared environments where multiple people route model API traffic through the same proxy instance. Each user gets isolated capture, metrics, and knowledge while administrators get aggregate visibility across the team.

## Problem Statement

Single-user modeltap works well for individual developers. But real-world usage often involves teams:

- **Shared development environments** where a team runs a single proxy for cost tracking across all members
- **Organizations** that want aggregate model API spend visibility while giving individuals access to their own interaction history
- **Managed deployments** where an infrastructure team operates the proxy on behalf of developers who connect to it
- **Privacy boundaries** where one user's captured prompts and responses must not be visible to another user, even when sharing the same proxy

Without multi-user support, teams must either run one proxy per person (operational overhead) or share a single-user proxy where everyone's data is mixed together (no isolation, no privacy, no per-user metrics).

## How Modeltap Solves This

Multi-user modeltap adds identity, isolation, and aggregate visibility on top of the existing proxy core:

1. **User identification** — Each request flowing through the proxy is associated with a user, identified via API key mapping, HTTP header, or client certificate
2. **Data isolation** — Captured requests, responses, metrics, and knowledge are partitioned by user. User A cannot see User B's interactions
3. **Per-user metrics** — Usage metrics (ADR-0007) are tracked per user in addition to global aggregates. "How much did Alice spend on Claude this month?" is a first-class query
4. **Aggregate visibility** — Administrators see team-wide metrics: total spend, model distribution, usage trends — without seeing individual prompt/response content
5. **Shared knowledge (opt-in)** — Users can opt in to contributing their knowledge layer data (ADR-0008) to a shared team knowledge base, enabling cross-team context while respecting individual privacy

## Architecture

```
┌──────────┐  ┌──────────┐  ┌──────────┐
│  Alice   │  │   Bob    │  │  Carol   │
│ (Claude  │  │ (Cursor) │  │ (ChatGPT)│
│   Code)  │  │          │  │          │
└────┬─────┘  └────┬─────┘  └────┬─────┘
     │              │              │
     │  API key /   │  API key /   │  API key /
     │  header      │  header      │  header
     │              │              │
     └──────────────┼──────────────┘
                    │
            ┌───────▼────────┐
            │   Modeltap     │
            │   Proxy Core   │
            │                │
            │ ┌────────────┐ │
            │ │   User     │ │
            │ │ Resolver   │ │
            │ └─────┬──────┘ │
            └───────┼────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
  ┌─────▼─────┐ ┌──▼────┐ ┌───▼──────┐
  │  Per-User  │ │ Agg   │ │  Shared  │
  │  Storage   │ │Metrics│ │Knowledge │
  │ (isolated) │ │(admin)│ │ (opt-in) │
  └────────────┘ └───────┘ └──────────┘
```

### User Resolver

The user resolver identifies which user is making each request. It supports multiple identification strategies:

- **API key mapping**: The proxy maintains a mapping of upstream API keys to user identities. When Alice's Claude Code sends a request with her Anthropic API key, the resolver maps that key to her user record
- **Header-based**: A custom header (`X-Modeltap-User`) identifies the user. Useful in managed environments where a gateway injects the header
- **Client certificate**: mTLS client certificates identify users. Strongest identity guarantee, appropriate for production deployments

The resolver is a pluggable interface, consistent with modeltap's provider adapter pattern (ADR-0006).

### Data Isolation

All existing tables gain a `user_id` column:

- `requests` — each captured request is tagged with the user who made it
- `hourly_usage` / `daily_usage` (ADR-0007) — metrics are aggregated per user in addition to globally
- `interaction_embeddings` / `interaction_metadata` (ADR-0008) — knowledge layer data is partitioned by user

Isolation is enforced at the query layer — all data access goes through a user-scoped context that automatically filters to the authenticated user's data.

### Aggregate Metrics (Admin View)

Administrators see team-wide metrics without seeing individual content:

- Total spend by provider, model, and time period across all users
- Per-user spend summaries (Alice spent $X, Bob spent $Y)
- Model usage distribution across the team
- Error rates and latency aggregates

Administrators **never** see individual prompt/response content through the admin view. Content access is restricted to the user who generated it.

### Shared Knowledge (Opt-in)

Users can opt in to contributing their knowledge layer data to a shared team pool:

- Opt-in is per-user, not per-interaction — a user either shares their knowledge or they don't
- Shared knowledge is searchable by all team members via MCP (ADR-0009)
- The MCP server includes both personal and shared knowledge in search results, clearly labeled
- Users can revoke sharing at any time, which removes their contributions from the shared pool

## Key Capabilities

### User Management

```
modeltap users add alice --email alice@example.com
modeltap users add bob --email bob@example.com
modeltap users list
modeltap users remove carol
```

### API Key Mapping

```
modeltap users map-key alice --provider anthropic --key-prefix sk-ant-...abc
modeltap users map-key bob --provider openai --key-prefix sk-...xyz
```

### Per-User Metrics

```
modeltap metrics --user alice --since 30d
modeltap metrics --user bob --group-by model --since 7d
modeltap metrics --all-users --group-by user --since 30d  # admin view
```

### Per-User Knowledge Search

```
modeltap search "authentication approaches" --user alice
modeltap search "database caching" --shared  # search shared knowledge pool
```

### Configuration

```yaml
# ~/.config/modeltap/config.yaml
multi_user:
  enabled: true
  resolver: api_key_mapping  # or header, or client_cert
  admin_users:
    - alice
  shared_knowledge:
    enabled: true
    default_opt_in: false  # users must explicitly opt in
```

## Privacy Model

Multi-user modeltap follows a strict privacy hierarchy:

| Data Type | User Access | Admin Access |
|-----------|------------|-------------|
| Own request/response bodies | ✅ Full access | ❌ No access |
| Own metrics | ✅ Full access | ✅ Aggregated only |
| Own knowledge layer | ✅ Full access | ❌ No access |
| Other user's content | ❌ No access | ❌ No access |
| Team-wide spend totals | ❌ No access | ✅ Full access |
| Per-user spend summaries | ❌ No access | ✅ Totals only |
| Shared knowledge pool | ✅ If opted in | ✅ If opted in |

## Relationship to Other ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0002 (Storage) | Multi-user adds a `user_id` column to the SQLite schema; storage provider interface remains unchanged |
| ADR-0005 (Capture) | Retention policies can be global or per-user in a future iteration |
| ADR-0006 (Multi-Provider) | Provider adapters are user-agnostic; user resolution happens before provider detection |
| ADR-0007 (Metrics) | Aggregation tables gain per-user dimensions; admin view queries across all users |
| ADR-0008 (Knowledge Layer) | Embeddings and metadata are partitioned by user; shared knowledge is a separate queryable pool |
| ADR-0009 (MCP Server) | MCP server must authenticate the requesting user and scope results; network transport (SSE/HTTP) may be needed alongside stdio for multi-user |

## Phasing

### Phase 1: Proxy Core — Single User (v1)

- No multi-user support
- Single database, single configuration
- All data belongs to one implicit user

### Phase 2: User Identification and Isolation (v2+)

- User resolver interface with API key mapping
- Per-user data isolation in existing tables
- Per-user metrics queries via CLI
- Admin aggregate metrics view

### Phase 3: Shared Knowledge and Team Features (v3+)

- Opt-in shared knowledge pool
- Team-wide knowledge search via MCP
- User management CLI commands
- Role-based access (user vs admin)

### Phase 4: Enterprise Features (future)

- SSO/OIDC integration for user identification
- Audit logging for admin actions
- Per-user retention policies
- Compliance and data governance controls
