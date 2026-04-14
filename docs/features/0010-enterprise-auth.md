---
feature: FEAT-0010
title: Enterprise Auth and Multi-User
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0002: SQLite storage (per-user scoping)
  - ADR-0007: Pre-computed aggregation (per-user metrics)
promoted-from:
  - EXP-0002: Multi-User Support
  - EXP-0008: Integrated Harness
---

# FEAT-0010: Enterprise Auth and Multi-User

## Problem

FEAT-0008 (BFF Server) and FEAT-0009 (Terminal Harness) create a single-user product. Enterprise deployment requires:

- **Centralized credentials**: the organization controls model API keys, not individual developers. No secrets on laptops.
- **User identity**: the server must know who is making each request for isolation, metrics, and policy enforcement.
- **Data isolation**: one developer's conversations, knowledge, and metrics must be invisible to others.
- **Aggregate visibility**: administrators need team-wide cost and usage metrics without seeing conversation content.
- **Policy enforcement**: per-role model access, spend budgets, and rate limits.

Without multi-user support, enterprises must run one server per developer (no aggregate visibility, operational overhead) or share a single-user instance (no isolation, no per-user metrics, no compliance story).

## Solution

Add pluggable identity providers, per-user data isolation, server-owned provider credentials, role-based authorization, and policy enforcement to the BFF server. The harness authenticates to the server on connection; the server scopes all data and operations to the authenticated user.

## Key Capabilities

### Pluggable Identity Provider Chain

The server supports multiple identity providers through an adapter interface. Auth negotiation is server-driven: the server declares which methods it accepts, the harness selects from that set. See EXP-0002 for the full auth negotiation protocol including downgrade resistance.

**Phase 1 providers:**

| Provider | Credential | When to use |
|----------|-----------|-------------|
| Token | Admin-provisioned static bearer token | Small teams, POC deployments |
| OIDC | JWT from enterprise IdP (Okta, Azure AD, etc.) | Most enterprise deployments |
| Local socket | OS peer credentials (UID) | Solo developer, local testing |

**Phase 2 provider:**

| Provider | Credential | When to use |
|----------|-----------|-------------|
| SPIFFE/SPIRE | Short-lived X.509 SVID via mTLS | Zero-trust, Kubernetes-native |

**OIDC authentication flows:**
- Device Authorization Grant (RFC 8628) — headless/SSH environments
- Localhost redirect — local development with browser

**Resolved identity shape:**

```
ResolvedIdentity {
    user_id:      string
    roles:        []string          # e.g. ["developer"], ["admin", "developer"]
    auth_method:  string            # "oidc", "token", "local", "spiffe"
    attrs:        map[string]any    # provider-specific claims
}
```

**Role resolution precedence** (highest wins):
1. Server-side static role overrides
2. Provider-derived roles (OIDC group claims, SPIFFE ID patterns)
3. Default role for the provider

### Server-Owned Provider Credentials

Model API keys are configured on the server, not on developer machines:

```yaml
providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
  openai:
    api_key: ${OPENAI_API_KEY}
  ollama:
    host: http://gpu-server.internal:11434
```

Developers never see, hold, or configure provider credentials. Key rotation is a server-side operation. The harness configuration for enterprise users is:

```yaml
server:
  address: modeltap.internal.acme.com:8443
```

### Per-User Data Isolation

All data is partitioned by user ID at the storage layer:

- `requests` — tagged with `user_id`
- `hourly_usage` / `daily_usage` — aggregated per user
- `sessions` / `conversation_history` — scoped to user
- Knowledge layer tables (when FEAT-0011 is active) — partitioned by user

**Enforcement model (Phase 1):** Application-layer isolation via a `UserScopedStore` interface. All queries flow through user-scoped context. No raw SQL escape hatches. Mandatory negative isolation tests (create data as user A, query as user B, assert empty).

**Escalation path:** Per-user SQLite databases if a customer's compliance review requires filesystem-level isolation.

### Roles and Authorization

**Phase 1 roles:**

**`developer`** (default):
- Full access to own conversations, knowledge, and metrics
- No access to other users' data or aggregate metrics
- Cannot manage users or server configuration

**`admin`**:
- Everything developer gets for own data
- Aggregate metrics across all users (spend, usage — no conversation content)
- User management (create/remove users, provision/revoke tokens)
- Server configuration

**Auth-method-gated permissions:** Admin operations require OIDC or SPIFFE — a stolen token cannot perform admin actions even if it maps to an admin user.

### Policy Enforcement

**Model access control:**

```yaml
policies:
  roles:
    developer:
      allowed_models: [claude-sonnet-*, gpt-4o-mini, llama-*]
      denied_models: [claude-opus-*]
    senior_developer:
      allowed_models: ["*"]
```

**Spend budgets:**

```yaml
policies:
  budgets:
    per_user:
      daily: 10.00
      monthly: 200.00
```

Budget exceeded behavior: block (hard limit), notify admin (soft limit), or fall back to cheaper models.

**Rate limits:**

```yaml
policies:
  rate_limits:
    per_user:
      requests_per_minute: 30
      tokens_per_hour: 100000
```

### Admin Aggregate Metrics

Administrators see team-wide metrics without conversation content:

- Total spend by provider, model, and time period
- Per-user spend summaries (amounts only)
- Model usage distribution
- Budget utilization
- Error rates and latency aggregates
- Active user count and session frequency

### Harness Auth Integration

The harness participates in the auth flow:

1. Connects to server (TLS handshake)
2. Receives auth challenge listing supported methods
3. Selects the best available method
4. Presents credential (JWT, token, SVID, or socket UID)
5. On success: session established, identity cached
6. On failure: connection closed (no fallback to weaker method)

For OIDC: the harness manages the browser flow (device code or localhost redirect), caches refresh tokens locally (OS keychain preferred, encrypted file fallback), and re-authenticates silently on subsequent launches.

## CLI Integration

```
# Admin commands (on server)
modeltap admin create-token --user alice --role developer
modeltap admin revoke-token --user alice
modeltap admin list-users
modeltap admin user-metrics --user alice --since 30d
modeltap admin metrics --all-users --group-by user --since 30d
```

## Configuration

Server-side auth configuration:

```yaml
auth:
  providers:
    - type: oidc
      issuer: https://login.acme.com
      client_id: modeltap-harness
      allowed_domains: [acme.com]
      user_id_claim: email
      role_claim: groups
      role_mapping:
        modeltap-admins: admin
        "*": developer
    - type: token
      default_expiry: 90d

  required_methods: [oidc]          # optional: reject weaker methods

  role_overrides:
    admin: [alice@acme.com]

  method_requirements:
    admin_operations: [oidc, spiffe]
```

## Non-Goals

- **Shared knowledge across users**: per-user isolation is the baseline. Shared knowledge is deferred per EXP-0002.
- **Compliance auditor role**: deferred to future work. Content access across users requires careful design.
- **Multi-tenancy / multi-org**: single org per server. SaaS deployment is future work.
- **SPIFFE/SPIRE provider in Phase 1**: Phase 2 after token and OIDC are proven.

## Success Criteria

1. An admin can create a token for a user, and the user can authenticate to the server with that token.
2. OIDC authentication works with at least one provider (Okta or Keycloak for testing): browser flow, token caching, silent re-auth.
3. Data isolation: user A's conversations, sessions, and metrics are invisible to user B. Negative isolation tests pass for every query path.
4. Admin sees aggregate metrics (total spend per user, model distribution) without seeing any conversation content.
5. Model access control prevents a developer-role user from using a denied model.
6. Spend budget enforcement blocks requests when a user exceeds their daily or monthly limit.
7. Auth-method gating prevents token-authenticated users from performing admin operations.
8. The server rejects weaker auth methods when `required_methods` is configured (downgrade resistance).
9. Local socket auth works for the solo developer profile with zero configuration.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0002 (SQLite) | Per-user data isolation via user_id column or per-user databases |
| ADR-0007 (Metrics) | Aggregation tables gain per-user dimension; admin queries span users |

## Open Questions

1. **JIT provisioning**: should the server auto-create user records on first OIDC login, or require admin pre-registration?
2. **Token storage**: where does the harness cache OIDC refresh tokens? OS keychain is ideal but platform-specific. Encrypted file is simpler but less secure.
3. **Service accounts**: for CI/CD or automation harnesses (headless, no human), which auth provider? Tokens are simplest. SPIFFE is strongest.
4. **Database encryption at rest**: should the server database be encrypted? SQLite options (SQLCipher) add complexity. Filesystem encryption (LUKS, FileVault) may suffice.
