---
patch: "PATCH-0004"
title: "Secret Prefix Resolver for Provider API Keys"
status: "approved"
date: "2026-04-19"
related:
  - "WU-094 security review (storage-at-rest gap not covered by the Criticals/Highs)"
branch: "exploration/integrated-harness"
---

# PATCH-0004: Secret Prefix Resolver for Provider API Keys

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`ProviderConfig.APIKey` is loaded as a plain string from `~/.config/modeltap/config.yaml`. Users today either commit the key to YAML or export it via `MODELTAP_PROVIDERS_ANTHROPIC_API_KEY`. Neither is good:

- Plaintext YAML means anyone with read access to the config file gets the key; file perms aren't enforced by the loader.
- Env var prefix means the key lives in the user's shell environment and propagates to every subprocess the shell starts (we filter the MCP inheritance path in WU-094 H-6, but not other tooling the user runs in the same shell).

A keychain / password-manager integration is the real answer (Tier 2 / FEAT). This patch is the cheap Tier 1 compromise: let the YAML reference secrets by prefix instead of embedding them, so the config file stays safe to commit or share.

## Scope

1. Add a `ResolveSecret(raw string) (string, error)` helper in `internal/config/`. Accepts the following forms:
   - `env:VAR` — substitute `os.Getenv("VAR")`. Empty value is an error (mis-typed var name is a common failure mode; fail loudly).
   - `file:/abs/path` or `file:~/relative` — read the file contents, trim trailing whitespace. Non-existent path is an error. Reserved for Docker-secrets-style mounts and CI environments that drop secrets to a file.
   - anything else (no matching prefix, no colon) — return verbatim. Preserves the existing "paste the key directly" ergonomic.
2. Apply the resolver to `ProviderConfig.APIKey` in the config load path. Other fields that might carry secrets in future (`TLSCertFile` / `TLSKeyFile` are paths, not secrets — skip) can reuse the helper when the need lands.
3. Update `docs/sample-config.yaml` to show `api_key: env:ANTHROPIC_API_KEY` as the recommended shape, with a note about the `file:` form for container deployments.
4. Unit tests covering the three resolver shapes (pass-through, env, file) + error cases.

## Out of Scope

- Keychain / `op://` / Vault integration — Tier 2, separate FEAT.
- Encrypted-config-at-rest. Separate FEAT.
- `${env:VAR}` string interpolation anywhere in a YAML value (only the whole-value prefix form). A general interpolator needs more thought about nesting and escaping.
- Existing env-var override via `MODELTAP_` prefix — unchanged. Users who already override that way keep working.

## Checklist

- [x] `config.ResolveSecret(raw)` helper implemented
- [x] Applied to `ProviderConfig.APIKey` on load
- [x] Sample config updated to demonstrate `env:` form
- [x] Tests: pass-through, env:VAR (set / empty / missing), file:PATH (exists / missing / with whitespace)
- [x] `go vet ./...` clean, `go test ./...` green

## Fix Detail

Applied on load, not on read — so the rest of the codebase continues to see a plain string. Keeps the blast radius of the patch contained to `internal/config/`.

Failure behavior: on a resolver error at load time, the config load fails with a clear message naming the offending field. Better to refuse to start than to pass through an empty string and fail later with "API key required" from the BFF.
