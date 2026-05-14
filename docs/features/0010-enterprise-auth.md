---
feature: FEAT-0010
title: Enterprise Auth and Multi-User
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: Runtime Server
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

FEAT-0008 (Runtime Server) and FEAT-0009 (Terminal Harness) create a single-user product. Enterprise deployment requires:

- **Centralized credentials**: the organization controls model API keys, not individual developers. No secrets on laptops.
- **User identity**: the server must know who is making each request for isolation, metrics, and policy enforcement.
- **Data isolation**: one developer's conversations, knowledge, and metrics must be invisible to others.
- **Aggregate visibility**: administrators need team-wide cost and usage metrics without seeing conversation content.
- **Policy enforcement**: per-role model access, spend budgets, and rate limits.

Without multi-user support, enterprises must run one server per developer (no aggregate visibility, operational overhead) or share a single-user instance (no isolation, no per-user metrics, no compliance story).

## Solution

Add pluggable identity providers, per-user data isolation, server-owned provider credentials, role-based authorization, and policy enforcement to the runtime server. The harness authenticates to the server on connection; the server scopes all data and operations to the authenticated user.

## Key Capabilities

### Pluggable Identity Provider Chain

The server supports multiple identity providers through an adapter interface. Auth negotiation is server-driven with explicit downgrade resistance guarantees.

### Auth Negotiation Protocol

Auth negotiation occurs at TLS connection time, before `capabilities.register` (FEAT-0008). The protocol is:

1. **TLS handshake**: if the client presents a valid SVID (SPIFFE), the server authenticates via mTLS immediately. Session is pinned to SPIFFE auth. Done.
2. **Auth challenge**: if no mTLS client cert, the server sends an auth challenge over the established TLS connection:
   ```json
   {
     "supported_methods": ["oidc", "token"],
     "required_methods": ["oidc"],
     "oidc": { "issuer": "...", "client_id": "...", "device_code_endpoint": "..." }
   }
   ```
3. **Method selection**: the harness selects from `supported_methods`. If `required_methods` is set, the harness MUST use one of those. If it cannot, the connection is refused.
4. **Credential presentation**: the harness presents the credential for the selected method (JWT for OIDC, bearer token for token auth).
5. **Verification**: the server verifies via the selected provider. On success: session pinned to `{method, resolved identity}`. On failure: connection closed — **no fallback to a weaker method**. The harness must reconnect to try a different method.

**Downgrade resistance guarantees:**
- No fallback on credential failure. The connection closes. No chance to try a weaker method in the same connection.
- `required_methods` is a hard constraint — methods not in the list are rejected even if configured as supported.
- Method is pinned to the session. No in-session re-negotiation.
- The auth challenge is served over TLS, preventing a network attacker from modifying the supported methods list.
- Auth-method gating (see Roles and Authorization) prevents a stolen token from performing operations that require stronger auth even if the token maps to an admin user.

**Session identity caching**: once authenticated, the resolved identity is cached for the connection lifetime. Reconnection requires re-authentication. OIDC access tokens are validated on connection establishment; the server does not re-validate mid-session (token expiry during an active session does not terminate it, but the next connection requires a fresh token).

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

### Admin Bootstrap

The first admin must be established before OIDC or SPIFFE are operational. The bootstrap path:

```
# On first server start, or via explicit bootstrap command:
modeltap admin bootstrap --user alice@acme.com --role admin
```

Bootstrap is permitted only when:
- The server has zero admin users in its database, OR
- The command is run from a local session (Unix socket / localhost) with OS-level access to the server process

Bootstrap creates a token for the first admin. Once at least one admin exists, the bootstrap command is disabled — subsequent admins are created via `modeltap admin create-token` by an existing admin.

**Break-glass recovery**: if OIDC is unavailable and all admin tokens are lost, the server operator can run `modeltap admin bootstrap --force` from the server host via local socket. `--force` requires direct OS access to the server machine and logs an audit event. This ensures recoverability without weakening the normal auth model.

## CLI Integration

```
# Bootstrap (first admin, or break-glass)
modeltap admin bootstrap --user alice@acme.com --role admin

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
3. Data isolation: user A's data is invisible to user B. Negative isolation tests pass for **every data-access surface**:
   - Request/response queries (`requests` table)
   - Session list and resume (`sessions`, `turns` tables)
   - Metrics queries (per-user and aggregate — aggregate must not leak content)
   - Export paths (log export, session export)
   - Knowledge search and injection (when FEAT-0011 is active)
   - MCP-facing knowledge queries (when MCP server is active)
   - Background rebuild/re-embedding jobs (must be user-scoped)
   - Context list (`context.list` protocol message)
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

## Future: Observer Mode

Once multi-user identity is in place, a lightweight observer mode becomes possible: a second user connects to an active session as a read-only viewer. The server copies stream events (token deltas, tool calls, status updates) to the observer but rejects any action messages. The observer harness renders everything minus the input loop — no textarea, no permission prompts, no mode switching.

**Requirements from FEAT-0010**: the observer needs their own authenticated identity so the server can verify they are allowed to observe this session and maintain an audit trail of who watched what.

**Permission model**: observation requires explicit consent. Options:
- The active user approves the observer on join
- An admin grants observe permission to specific users or roles
- The session owner pre-authorizes observers in session settings

**Use cases**:
- Mentor watching an apprentice work (EXP-0005)
- Team lead monitoring a junior developer's AI-assisted workflow
- Live demo of an AI session to stakeholders

**Implementation complexity**: low. The server already streams all events to the active harness. Observer mode adds a second subscriber to the same event stream. No new protocol beyond `session.resume` with an `observe: true` flag and a `session.observer_joined` notification to the active harness.

This is not part of the initial FEAT-0010 scope. It is documented here because it depends on multi-user identity and is a natural follow-on.

## Resolved Questions

1. ~~**JIT provisioning**~~: resolved. The server auto-creates user records on first OIDC login (JIT provisioning). New users get the default role (`developer`). Admin can adjust roles afterward. This removes the onboarding friction of pre-registering every user.

2. ~~**Token storage**~~: resolved. OS keychain (macOS Keychain, Linux Secret Service via D-Bus). Platform-specific but correct — OIDC refresh tokens are credentials and should be stored where the OS stores credentials. Fallback to encrypted file (`~/.config/modeltap/.tokens`, encrypted with a key derived from the user's OS identity) for environments without a keychain.

## Open Questions

1. **Service accounts**: for CI/CD or automation harnesses (headless, no human), which auth provider? Tokens are simplest. SPIFFE is strongest.
2. **Database encryption at rest**: should the server database be encrypted? SQLite options (SQLCipher) add complexity. Filesystem encryption (LUKS, FileVault) may suffice.
