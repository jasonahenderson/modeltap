---
exploration: EXP-0002
title: Multi-User Support
status: exploring
date: 2026-03-03
updated: 2026-04-14
related:
  - EXP-0008: Integrated harness (multi-user is prerequisite for enterprise)
  - EXP-0001: Knowledge layer (per-user isolation, future shared pools)
  - ADR-0002: Storage format (per-user scoping)
  - ADR-0006: Multi-provider support
  - ADR-0007: Pre-computed aggregation (per-user metrics)
  - ADR-0008: Knowledge layer architecture (per-user embeddings)
  - ADR-0012: Service management (server deployment model)
---

# EXP-0002: Multi-User Support

## Overview

Modeltap v1 was designed as a single-user proxy. Multi-user support extends modeltap to serve teams and enterprises where a centrally operated server handles model API traffic for multiple developers. Each developer connects via the modeltap harness (EXP-0008). The server provides identity, isolation, credential management, and aggregate visibility.

Multi-user is a near-term priority — it is prerequisite for enterprise deployment and contract work.

## Problem Statement

Single-user modeltap works for individual developers. Enterprise and team contexts require:

- **Centralized credential management** — the organization controls model API keys, not individual developers. No secrets on laptops, no per-developer API accounts, centralized spend control.
- **User isolation** — one developer's captured prompts, responses, knowledge, and metrics must be invisible to other developers, even when sharing the same server.
- **Aggregate visibility** — administrators need team-wide cost, usage, and model distribution metrics without access to individual conversation content.
- **Pluggable identity** — enterprises have existing identity infrastructure (Okta, Azure AD, SPIFFE/SPIRE, etc.). Modeltap must integrate with what they have, not impose its own.
- **Policy enforcement** — per-user or per-role model access, spend budgets, rate limits. The server is the governance layer.

Without multi-user support, teams must either run one server per person (operational overhead, no aggregate visibility) or share a single-user instance where everyone's data is mixed (no isolation, no privacy, no per-user metrics, no compliance story).

## Deployment Profiles

This exploration and EXP-0008 (Integrated Harness) describe the same product from different angles: EXP-0008 defines what the user interacts with; this document defines how the server supports multiple users. The deployment model must be consistent across both.

The modeltap server is always a separate process from the harness. What changes between profiles is where the server runs, how the harness connects, and which identity provider is appropriate.

### Profile: Solo Developer

- The server runs on the developer's machine as a background service (launchd/systemd, per ADR-0012) or as a subprocess auto-started by the harness.
- The harness connects via Unix domain socket at a well-known path (`~/.local/share/modeltap/server.sock`).
- Identity is OS-attested (socket peer credentials). No explicit auth configuration.
- Provider API keys are in the developer's local config.
- This is the degenerate case of the enterprise architecture where N=1, not a separate code path.

### Profile: Team Server

- The server runs on shared infrastructure (a VM, a container, a dedicated machine).
- Harnesses connect over TLS to a network address.
- Identity via OIDC or admin-provisioned tokens.
- Provider API keys are server-side only.
- Admin manages users, budgets, and model access policies.

### Profile: Enterprise

- The server is managed infrastructure, potentially HA (load balancer → multiple server instances sharing a database, or per-org instances).
- Harnesses connect over mTLS or TLS + OIDC bearer.
- Identity via SPIFFE/SPIRE or enterprise OIDC (Okta, Azure AD).
- Provider API keys are server-side, potentially sourced from a secrets manager (Vault, AWS Secrets Manager).
- Full policy enforcement: model access control, spend budgets, rate limits, audit logging.

### Relationship to EXP-0008

EXP-0008's description of the product as a "single binary" applies to distribution — one compiled artifact contains both the harness CLI and the server. But they run as separate processes in all profiles. The harness binary can start a local server subprocess for the solo profile, but this is a convenience, not a fusion.

EXP-0008 should be updated to reflect this: the harness and server are always separate processes, connected by the modeltap protocol. The solo developer experience hides this separation behind auto-start, but the architecture does not change.

### Relationship to ADR-0012

ADR-0012 (platform-native service managers) was written for single-user local background execution. It remains valid for the solo profile. The team and enterprise profiles use standard server deployment (systemd unit on a server, container orchestration, etc.) rather than the desktop-oriented launchd/systemd integration ADR-0012 describes. ADR-0012 does not need to be superseded — its scope is local service management, which is one deployment profile among several.

## Architecture

```
Developer laptops                         Enterprise infrastructure
┌──────────────┐  ┌──────────────┐       ┌──────────────────────────────┐
│   Harness    │  │   Harness    │       │       Modeltap Server        │
│   (Alice)    │  │   (Bob)      │       │                              │
│              │  │              │       │  ┌────────────────────────┐  │
│  No API keys │  │  No API keys │       │  │  Identity Provider     │  │
│  No provider │  │  No provider │       │  │  Chain                 │  │
│  knowledge   │  │  knowledge   │       │  │                        │  │
└──────┬───────┘  └──────┬───────┘       │  │  SPIFFE ─┐             │  │
       │                 │               │  │  OIDC ───┤→ resolved   │  │
       │    mTLS / TLS + bearer          │  │  Token ──┤  identity   │  │
       └─────────┬───────┘               │  │  Local ──┘  {user_id,  │  │
                 │                       │  │             roles[],   │  │
                 │                       │  │             auth_method,│ │
                 │                       │  │             attrs{}}   │  │
                 │                       │  └────────────────────────┘  │
                 └──────────────────────►│                              │
                                         │  ┌────────────────────────┐  │
                                         │  │  Provider Credentials  │  │
                                         │  │  (server-owned)        │  │
                                         │  │                        │  │
                                         │  │  Anthropic: sk-ant-... │  │
                                         │  │  OpenAI:    sk-...     │  │
                                         │  │  Ollama:    localhost   │  │
                                         │  └───────────┬────────────┘  │
                                         │              │               │
                                         │  ┌───────────▼────────────┐  │
                                         │  │  Per-User Isolated     │  │
                                         │  │  Storage               │  │
                                         │  │                        │  │
                                         │  │  Alice: capture,       │  │
                                         │  │    knowledge, metrics  │  │
                                         │  │  Bob: capture,         │  │
                                         │  │    knowledge, metrics  │  │
                                         │  └───────────┬────────────┘  │
                                         │              │               │
                                         │  ┌───────────▼────────────┐  │
                                         │  │  Admin Aggregate       │  │
                                         │  │  Metrics (no content)  │  │
                                         │  └────────────────────────┘  │
                                         └──────────────────────────────┘
```

### Key Architectural Decisions

**The server owns all provider API keys.** Developers never see, hold, or configure model API credentials. The server authenticates to providers on behalf of users. This means:

- No secrets on developer machines (except their own identity credential, which is short-lived or platform-attested)
- Key rotation is a server-side operation, invisible to developers
- The server can enforce which providers and models each user or role can access
- Cost attribution is authoritative — the server knows exactly who spent what

**The harness is the only client.** Unlike the original EXP-0002 which showed various AI clients (Claude Code, Cursor, ChatGPT) connecting to the proxy, the integrated architecture (EXP-0008) means only the modeltap harness connects to the server. This simplifies the identity model — no need to guess user identity from API key prefixes or injected headers. The harness authenticates directly.

**Per-user isolation is the default.** There is no shared knowledge pool in the initial implementation. Each user's capture, knowledge layer, and metrics are fully isolated. Shared knowledge is a future opt-in capability (see Deferred: Shared Knowledge below).

## Pluggable Identity Provider Chain

The server supports multiple identity providers through an adapter interface. Auth selection is **server-driven**: the server declares which methods it accepts and their minimum security requirements. The harness selects from the server's allowed set, but cannot downgrade to a method the server has not explicitly enabled.

### Resolved Identity

All providers resolve to a common identity shape:

```
ResolvedIdentity {
    user_id:      string          // unique user identifier
    roles:        []string        // e.g. ["developer"], ["admin", "developer"]
    auth_method:  string          // "spiffe", "oidc", "token", "local"
    attrs:        map[string]any  // provider-specific claims (email, groups, spiffe_id, etc.)
}
```

**Role resolution precedence** (highest wins):

1. Server-side static role assignments (`auth.role_overrides` in config) — always authoritative
2. Provider-derived roles (OIDC group claims, SPIFFE ID path patterns)
3. Default role for the provider (`auth.providers[].default_role`, defaults to `developer`)

When multiple sources assign roles, they are **unioned** — a user who is `developer` from OIDC groups and `admin` from a static override gets `["admin", "developer"]`. Authorization checks test for the presence of a required role, not for an exact match.

The `auth_method` field is carried on the session and available to authorization policy. This enables rules like "admin actions require OIDC or SPIFFE, not token auth."

### Identity Provider Interface

Each provider implements:

- **CanAuthenticate**: given the connection/request, can this provider verify it?
- **Authenticate**: extract and verify the credential, return a `ResolvedIdentity`
- **Type**: provider name for configuration and logging
- **TransportRequirements**: what transport-level properties this provider requires (e.g., mTLS for SPIFFE, TLS for OIDC)

The chain is evaluated per-connection (not per-request). Once a harness connection is authenticated, the resolved identity is pinned to the session and cannot be re-negotiated without disconnecting.

### Provider: SPIFFE/SPIRE

**How it works**: The harness obtains a short-lived X.509 certificate (SVID) from the local SPIRE agent via the Workload API. The connection to the server uses mTLS. The server validates the SVID against its trust bundle and extracts the user identity from the SPIFFE ID.

**Identity flow**:
```
Harness → SPIRE Agent (Workload API) → receives SVID
Harness → Server (mTLS with SVID as client cert)
Server → validates SVID against trust bundle
Server → parses SPIFFE ID: spiffe://acme.com/modeltap/user/alice → user "alice"
```

**Credential lifecycle**:
- SVIDs are short-lived (typically 1 hour), auto-rotated by SPIRE
- No secrets to provision, distribute, or rotate
- Revocation is instant — remove the SPIRE registration entry

**Server configuration**:
```yaml
auth:
  providers:
    - type: spiffe
      trust_domain: acme.com
      trust_bundle_path: /etc/spire/bundle.pem
      # Identity extraction from SPIFFE ID path
      # spiffe://acme.com/modeltap/user/{user_id}
      user_id_pattern: "modeltap/user/{user_id}"
```

**Harness configuration**: None. The SPIRE agent handles attestation. The harness uses the `go-spiffe` library to call the local Workload API.

**When to use**: Enterprises with existing SPIFFE/SPIRE infrastructure. Kubernetes-native environments. Zero-trust architectures. Strongest security posture — no shared secrets anywhere in the system.

### Provider: OIDC/SSO

**How it works**: The harness performs an OAuth 2.0 flow against the organization's identity provider (Okta, Azure AD, Google Workspace, Keycloak, etc.), obtains a JWT, and presents it as a bearer token on the server connection. The server validates the JWT signature against the IdP's JWKS endpoint.

**Identity flow (Device Authorization Grant — RFC 8628)**:
```
$ modeltap
No active session. Authenticating...
Go to: https://login.acme.com/device
Enter code: ABCD-1234
Waiting for authorization... ✓
Authenticated as alice@acme.com
```

This flow works in headless environments, SSH sessions, and remote terminals where a browser redirect is not possible.

**Identity flow (Localhost Redirect)**:
```
$ modeltap
No active session. Authenticating...
Opening browser for login...
[browser opens → user authenticates → redirect to localhost:9876/callback]
Authenticated as alice@acme.com
```

Better UX for local development where a browser is available.

**Credential lifecycle**:
- Access tokens are short-lived (typically 5-60 minutes)
- Refresh tokens are cached locally (OS keychain when available, encrypted file otherwise)
- Silent re-authentication on subsequent harness launches until refresh token expires
- Token revocation through the IdP's admin console

**Server configuration**:
```yaml
auth:
  providers:
    - type: oidc
      issuer: https://login.acme.com
      client_id: modeltap-harness
      # Optional: restrict to specific email domains
      allowed_domains:
        - acme.com
      # Which JWT claim provides the user ID
      user_id_claim: email  # or "sub"
      # Which JWT claim provides role information
      role_claim: groups
      role_mapping:
        modeltap-admins: admin
        "*": developer
```

**Harness configuration**:
```yaml
server:
  address: modeltap.internal.acme.com:8443
# No auth.type needed — the harness discovers supported methods
# from the server's auth challenge and selects automatically.
```

**When to use**: Enterprises with Okta, Azure AD, Google Workspace, or any OIDC-compliant identity provider. Most common enterprise auth path.

### Provider: Admin-Provisioned Tokens

**How it works**: An administrator generates a static token for each user via the server CLI. The developer configures the token in their harness (environment variable or config file). The server looks up the token and maps it to a user.

**Identity flow**:
```
# Admin side
$ modeltap admin create-token --user alice --role developer
Token: mtp_a1b2c3d4e5f6...
Give this to alice. It will not be shown again.

# Developer side
$ export MODELTAP_TOKEN=mtp_a1b2c3d4e5f6...
$ modeltap
Authenticated as alice
```

**Credential lifecycle**:
- Tokens are long-lived by default (configurable expiry)
- Admin can revoke tokens: `modeltap admin revoke-token --user alice`
- Tokens are stored hashed in the server database (bcrypt or argon2)
- Rotation requires admin to issue a new token and developer to update their config

**Server configuration**:
```yaml
auth:
  providers:
    - type: token
      # Tokens managed via CLI: modeltap admin create-token
      # Optional expiry for all tokens
      default_expiry: 90d
```

**Harness configuration**:
```yaml
server:
  address: modeltap.internal.acme.com:8443
# Token is provided via environment variable. The harness presents it
# when the server's auth challenge includes "token" as a supported method.
# MODELTAP_TOKEN=mtp_a1b2c3d4e5f6...
```

**When to use**: Small teams without SSO infrastructure. Quick setup for proof-of-concept deployments. Fallback when OIDC/SPIFFE is not available.

### Provider: Local Socket (Solo/Development)

**How it works**: When the server listens on a Unix domain socket, the OS kernel enforces access via file permissions. The server extracts the connecting process's UID via `SO_PEERCRED` and maps it to the local username.

**Identity flow**:
```
$ modeltap
[server auto-starts on ~/.local/share/modeltap/server.sock]
Authenticated as jasonhenderson (local)
```

**Credential lifecycle**: None. The OS is the identity provider. If you can connect to the socket, you are the user.

**Server configuration**: Automatic when listening on a Unix socket. No explicit auth config needed.

**Harness configuration**: None. Auto-detected when the server address is a socket path or localhost.

**When to use**: Solo developers. Local development. Testing. The degenerate case of the enterprise architecture where N=1.

### Auth Negotiation and Downgrade Resistance

Auth negotiation is **server-driven**. The server dictates which methods are acceptable; the harness selects from that set. The harness cannot offer a weaker method than the server requires.

**Negotiation flow**:

```
Harness connects to server (TLS handshake)
        │
        ├─ If mTLS: server checks client cert immediately
        │  └─ Valid SVID? → SPIFFE auth, session pinned, done
        │  └─ No client cert or invalid? → continue to application-layer auth
        │
        ▼
Server sends auth challenge:
  {
    "supported_methods": ["oidc", "token"],  // only methods server allows
    "required_methods": ["oidc"],            // if set, MUST use one of these
    "oidc": {                                // per-method parameters
      "issuer": "https://login.acme.com",
      "client_id": "modeltap-harness",
      "device_code_endpoint": "...",
      "authorization_endpoint": "..."
    }
  }
        │
        ▼
Harness selects method:
  - If required_methods is set, harness MUST use one of those
  - Otherwise, harness selects from supported_methods
  - Harness preference order: SPIFFE > OIDC > Token > Local
  - If harness cannot satisfy any supported method → connection refused
        │
        ▼
Harness presents credential for selected method
        │
        ▼
Server verifies credential via the selected provider
  - On success: session pinned to {method, resolved identity}
  - On failure: connection closed (no fallback to weaker method)
```

**Downgrade resistance guarantees**:

- **No fallback on failure.** If a credential is presented and fails verification, the connection is closed. The server does not offer the harness a chance to try a weaker method. The harness must reconnect and select a different method from scratch.
- **Server controls the allowed set.** An enterprise that configures only `["oidc"]` in `supported_methods` rejects all token and local auth attempts. The harness cannot circumvent this.
- **`required_methods` is a hard constraint.** When set, the server refuses any method not in the required list, even if other methods are configured as supported. This allows a server to support tokens for service accounts while requiring OIDC for human users (via per-connection-type required lists, or a global minimum).
- **Method is pinned to the session.** Once authenticated, the auth method cannot change without disconnecting. There is no in-session re-negotiation.
- **Transport-level requirements are non-negotiable.** SPIFFE requires mTLS at the TLS layer — it is verified during the handshake, before the application-layer challenge. If the server is configured for SPIFFE-only, connections without a valid client certificate are terminated at the TLS layer, never reaching application code.

**Auth challenge is served over TLS, not plaintext.** The discovery endpoint itself is protected by TLS, preventing a network attacker from modifying the supported methods list to inject a weaker option.

**Configuration example — enterprise lockdown**:

```yaml
auth:
  # Only OIDC and SPIFFE allowed. Tokens disabled entirely.
  providers:
    - type: spiffe
      trust_domain: acme.com
      trust_bundle_path: /etc/spire/bundle.pem
      user_id_pattern: "modeltap/user/{user_id}"
    - type: oidc
      issuer: https://login.acme.com
      client_id: modeltap-harness
      allowed_domains:
        - acme.com
      user_id_claim: email
  required_methods: [spiffe, oidc]
  # Token and local are not listed — any attempt to use them is rejected
```

**Configuration example — small team (tokens acceptable)**:

```yaml
auth:
  providers:
    - type: oidc
      issuer: https://login.acme.com
      client_id: modeltap-harness
    - type: token
      default_expiry: 90d
  # No required_methods — both OIDC and token are acceptable
```

## Data Isolation

### Storage Model

All data is partitioned by user ID at the storage layer:

- `requests` — each captured request/response is tagged with `user_id`
- `hourly_usage` / `daily_usage` — metrics aggregated per user
- `interaction_embeddings` / `interaction_metadata` — knowledge layer data partitioned by user

### Isolation Enforcement

Content isolation in the initial implementation is **application-layer**: a tenant-scoped storage interface ensures all queries include a `user_id` filter. This is a practical starting point, not an absolute guarantee — a bug in the query layer could theoretically leak data across users. The following guardrails reduce that risk to an acceptable level for Phase 1:

**Mandatory guardrails (Phase 1)**:

- **Tenant-scoped storage interface.** All data access flows through a `UserScopedStore` that accepts a `user_id` at construction time. No method on this interface can query across users. Raw SQL access is not exposed to application code.
- **No unscoped query paths.** The only code that queries across users is the admin aggregate metrics layer, which projects numeric aggregates (SUM, COUNT, AVG) and never returns content fields (request body, response body, knowledge text).
- **Negative isolation tests.** The test suite must include explicit cross-user read tests: create data for user A, query as user B, assert empty result. These tests cover every query path that touches user-partitioned tables.
- **No raw query escape hatches.** The storage interface does not expose a generic `Query(sql string)` method. All queries are pre-defined methods on the scoped interface.

**Threshold for per-user databases**: If a customer's compliance or security review requires kernel-level or filesystem-level isolation between users (e.g., each user's data must be separately encryptable, separately deletable, or provably inaccessible without per-user credentials), the implementation should move to per-user SQLite databases. The storage interface (ADR-0002) already abstracts database access, so this is a refactor of the storage layer, not the application logic.

### Single Database vs. Per-User Database

**Option A: Single database, user_id column (Phase 1)** — Simpler operationally. One backup, one retention policy, one file. Isolation is application-enforced via the guardrails above. Appropriate when the organization trusts the modeltap server software and the isolation test suite.

**Option B: Per-user SQLite databases (Phase 2 or compliance-driven)** — Strongest isolation. Each user gets their own database file. Cross-user data leakage becomes impossible at the storage layer. Cost: more files to manage, per-user backup/retention, aggregate metrics require cross-database queries or a separate metrics-only database.

**Recommendation**: Start with Option A. The guardrails (scoped interface, no raw queries, negative tests) make application-layer isolation defensible for most deployments. Move to Option B when a customer's compliance requirements demand it or when the user count makes per-user databases operationally preferable.

## Roles and Authorization

### Role Model

Roles are additive. A user may hold multiple roles simultaneously (e.g., `["admin", "developer"]`). Authorization checks test for the **presence** of a required role, not for an exact set.

Phase 1 defines two roles. The identity shape (`ResolvedIdentity.roles[]`) and the policy engine support additional roles without schema changes — adding a role means defining its permissions, not changing the authorization framework.

### Phase 1 Roles

**`developer`** (default for all authenticated users):

- Full access to their own capture, knowledge, and metrics
- No access to any other user's data
- No access to aggregate team metrics
- Can configure their own harness preferences
- Cannot manage users, tokens, or server configuration

**`admin`** (assigned via static override or provider-derived groups):

- Everything `developer` gets for their own data
- Aggregate metrics across all users (total spend, per-user spend summaries, model distribution)
- **No access to other users' conversation content** — admin sees numbers, not prompts
- User management: create/remove users, provision/revoke tokens
- Server configuration: provider credentials, routing policies, model access lists
- Audit log access

### Auth-Method-Gated Permissions

Some operations may require a specific auth method regardless of role. The `auth_method` field on the resolved identity enables rules like:

- Admin operations (user management, server config) require `oidc` or `spiffe` — not `token`
- Content export requires `oidc` or `spiffe`
- Read-only operations (own data, own metrics) are allowed with any auth method

This prevents a stolen long-lived token from being used for privileged admin actions even if the token maps to an admin user.

### Future: Compliance Auditor

Not implemented initially, but the role model supports it. A `compliance_auditor` role would have read access to all conversation content for audit/legal purposes, gated by additional controls (reason logging, time-limited access, notification to affected users). This is a sensitive capability that should be designed carefully when a customer requires it.

### Future: Custom Roles

The policy examples in this document reference `senior_developer` as a role with broader model access. This is supported by the role model without changes — an admin defines the role name in policy configuration and assigns it via static overrides or provider-derived group mappings. The authorization engine does not have a hardcoded role list.

## Policy Enforcement

The server enforces organizational policies that individual harnesses cannot override:

### Model Access Control

```yaml
policies:
  roles:
    developer:
      allowed_models:
        - claude-sonnet-*
        - gpt-4o-mini
        - llama-*          # all local models
      denied_models:
        - claude-opus-*    # too expensive for general use
    senior_developer:
      allowed_models:
        - "*"              # unrestricted
```

### Spend Budgets

```yaml
policies:
  budgets:
    per_user:
      daily: 10.00
      monthly: 200.00
    per_role:
      developer:
        monthly: 150.00
      senior_developer:
        monthly: 500.00
```

When a budget is exceeded, the server can:
- Block requests (hard limit)
- Allow but notify admin (soft limit)
- Fall back to cheaper models (graceful degradation)

### Rate Limits

```yaml
policies:
  rate_limits:
    per_user:
      requests_per_minute: 30
      tokens_per_hour: 100000
```

## Aggregate Metrics (Admin View)

Administrators see team-wide metrics without seeing individual content:

- Total spend by provider, model, and time period across all users
- Per-user spend summaries (Alice spent $X, Bob spent $Y) — amounts only, no content
- Model usage distribution across the team
- Budget utilization (who's approaching limits)
- Error rates and latency aggregates
- Active user count and session frequency

These metrics build on the existing pre-computed aggregation tables (ADR-0007), extended with a user dimension.

## Privacy Model

| Data Type | Developer (own) | Developer (other) | Admin |
|-----------|:-:|:-:|:-:|
| Request/response content | ✅ | ❌ | ❌ |
| Knowledge layer | ✅ | ❌ | ❌ |
| Own usage metrics | ✅ | ❌ | ✅ (aggregated) |
| Own spend | ✅ | ❌ | ✅ (totals) |
| Team-wide spend | ❌ | ❌ | ✅ |
| Model distribution | ❌ | ❌ | ✅ |
| User management | ❌ | ❌ | ✅ |
| Server configuration | ❌ | ❌ | ✅ |

Content isolation is enforced at the application layer via the tenant-scoped storage interface and negative isolation tests described in Data Isolation above. No role in the initial implementation has an API or query path to access another user's conversation content. This is an application-level guarantee, not a kernel-level or database-level guarantee — see the isolation guardrails and per-user database threshold for the conditions under which stronger enforcement is required.

## Server Configuration

```yaml
# modeltap-server.yaml

# Listen address
server:
  address: 0.0.0.0:8443
  tls:
    cert: /etc/modeltap/server.crt
    key: /etc/modeltap/server.key

# Identity providers
auth:
  providers:
    - type: spiffe
      trust_domain: acme.com
      trust_bundle_path: /etc/spire/bundle.pem
      user_id_pattern: "modeltap/user/{user_id}"
      default_role: developer

    - type: oidc
      issuer: https://login.acme.com
      client_id: modeltap-harness
      allowed_domains:
        - acme.com
      user_id_claim: email
      role_claim: groups
      role_mapping:
        modeltap-admins: admin
        modeltap-senior: senior_developer
        "*": developer
      default_role: developer

    - type: token
      default_expiry: 90d
      default_role: developer

  # Hard constraint: only these methods accepted (omit to allow all configured)
  required_methods: [spiffe, oidc]

  # Static role overrides (highest precedence, unioned with provider-derived roles)
  role_overrides:
    admin:
      - alice@acme.com
      - spiffe://acme.com/modeltap/admin/*
    senior_developer:
      - bob@acme.com

  # Auth-method gating for sensitive operations
  method_requirements:
    admin_operations: [oidc, spiffe]  # user mgmt, server config
    content_export: [oidc, spiffe]    # bulk data export

# Provider credentials (server-owned)
providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
  openai:
    api_key: ${OPENAI_API_KEY}
  ollama:
    host: http://localhost:11434

# Routing policy
routing:
  default: claude-sonnet-4-6
  coding: claude-opus-4-6
  cheap: llama-3.1-8b
  embedding: nomic-embed-text

# Per-role policies
policies:
  roles:
    developer:
      allowed_models:
        - claude-sonnet-*
        - gpt-4o-mini
        - llama-*
      budget:
        monthly: 150.00
    senior_developer:
      allowed_models:
        - "*"
      budget:
        monthly: 500.00

# Storage
storage:
  path: /var/lib/modeltap/data.db
  retention: 90d

# Knowledge layer
knowledge:
  enabled: true
  embedding_model: nomic-embed-text
  auto_enrich: true
```

## Deferred: Shared Knowledge

Per-user knowledge isolation is the baseline. Shared knowledge is explicitly deferred to a future exploration. When it is revisited, the design should address:

- Opt-in granularity (per-user? per-project? per-interaction?)
- What is shared (extracted insights and decisions, never raw conversation content)
- How shared knowledge is labeled (who contributed, when, confidence)
- Revocation (removing contributions when a user opts out)
- Privacy review (what if an insight implicitly reveals conversation content?)

Shared knowledge should be implemented as a separate storage surface (distinct table or database) when the feature is built, not as a flag on the per-user tables. This avoids creating any cross-user query path in the isolated storage layer before the feature is designed and reviewed.

## Relationship to Other Explorations

| Exploration | Relationship |
|-------------|-------------|
| EXP-0008 (Integrated Harness) | Multi-user is prerequisite. The harness is the only client; auth happens at the harness-to-server connection. |
| EXP-0001 (Knowledge Layer) | Knowledge is per-user isolated. The knowledge layer's value (cross-model memory, context enrichment) works within each user's silo. |
| EXP-0005 (Apprenticeship) | Requires multi-user as prerequisite. Mentor/apprentice is a cross-user interaction with explicit consent. |
| EXP-0007 (Orchestration) | Orchestration runs within a user's session. No cross-user orchestration in v1. |
| PATCH-0002 (Local Inference) | Local models are server-side. The server routes to Ollama/MLX; developers don't need local models installed. Enterprise may run GPU servers centrally. |

## Phasing

### Phase 1: Identity and Isolation (near-term, prerequisite for enterprise)

- Pluggable identity provider interface
- Token auth provider (simplest to build, sufficient for first engagement)
- OIDC provider (required for most enterprise clients)
- Per-user data isolation (user_id column, scoped queries)
- Per-user metrics
- Admin aggregate metrics (spend, usage, no content)
- Server-owned provider credentials
- Two roles: developer, admin

### Phase 2: Policy and Governance

- Model access control per role
- Spend budgets (per-user and per-role)
- Rate limits
- SPIFFE/SPIRE provider
- Audit logging for admin actions

### Phase 3: Shared Knowledge and Team Features (future)

- Opt-in shared knowledge pool
- Cross-user insight discovery
- Team-level project scoping
- Compliance auditor role

## Open Questions

1. **Token storage on the harness side**: for OIDC, where does the harness cache refresh tokens? OS keychain (macOS Keychain, Linux Secret Service) is ideal but adds platform-specific complexity. Encrypted file is simpler but less secure.

2. **User provisioning**: with OIDC, should the server auto-create user records on first login (JIT provisioning)? Or must an admin pre-register every user?

3. **Service account identity**: for CI/CD or automation harnesses (headless, no human), which auth provider is appropriate? Tokens are simplest. SPIFFE is strongest. OIDC service accounts are possible but awkward.

4. **Database encryption at rest**: the server database contains all conversation content for all users. Should it be encrypted? SQLite encryption options (SQLCipher, SEE) add complexity. Filesystem-level encryption (LUKS, FileVault) may be sufficient.

5. **Multi-tenancy**: for a managed/SaaS deployment, should the server support multiple organizations with full isolation? Or is single-org the only deployment model?
