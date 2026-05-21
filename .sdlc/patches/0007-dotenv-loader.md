---
patch: "PATCH-0007"
title: "`.env` Loader for Provider Credentials"
status: "done"
date: "2026-04-20"
related:
  - "PATCH-0004 (secret prefix resolver)"
  - "PATCH-0006 (unified config/data dir)"
  - "ADR-0010 (license compatibility)"
branch: "exploration/integrated-harness"
---

# PATCH-0007: `.env` Loader for Provider Credentials

## Problem

Today the recommended shape for a provider API key is `api_key: env:ANTHROPIC_API_KEY` (PATCH-0004), which means the user must either:

- Export the variable in their shell (`export ANTHROPIC_API_KEY=...` in `.zshrc` / `.bashrc`) — pollutes the whole shell environment and propagates to every subprocess.
- Prefix every `modeltap` invocation with the variable inline — tedious.
- Stash it in a file and `source` it — works, but requires shell gymnastics the user has to remember.

A `.env` file at a well-known location is the industry-standard convention (Node `dotenv`, Python `python-dotenv`, Ruby `dotenv`, etc.). It lets the user drop keys in one file that the tool reads automatically, keeps the shell environment clean, and makes per-project overrides trivial.

modeltap already reads env vars (via Viper's `MODELTAP_*` prefix and PATCH-0004's `env:` resolver). The missing piece is the loader that populates the process environment from a `.env` file at startup.

## Scope

1. Add a new package `internal/dotenv/` with a `Load()` function that:
   - Looks for `./.env` (project-local, relative to CWD) and `$HOME/.modeltap/.env` (user-level).
   - If $XDG_CONFIG_HOME is set, also checks `$XDG_CONFIG_HOME/modeltap/.env` (matches PATCH-0006's config dir behavior).
   - Silently skips missing files (the common case for users not using dotenv).
   - Returns an error for files that exist but are malformed (user paid the cost to create a `.env`; silent failure there would be confusing).
2. Use `github.com/joho/godotenv` (MIT license — Apache-2.0 compatible per ADR-0010).
3. Call `dotenv.Load()` from `cmd/modeltap/main.go` before `cli.NewRootCommand` runs, so every config code path (harness, proxy, server, status, etc.) sees the loaded values.
4. Precedence (from highest to lowest):
   1. Existing process environment (never overridden — matches `godotenv.Load`'s default behavior)
   2. `./.env` (project-local wins over user-level)
   3. `$HOME/.modeltap/.env` or `$XDG_CONFIG_HOME/modeltap/.env` (user-level)
5. Escape hatch: setting `MODELTAP_DOTENV=false` (or `0`/`no`/`off`) disables loader entirely. Useful in CI, containers, or when debugging which values are coming from where.
6. When a file is loaded, record which file(s) into stderr only if `MODELTAP_DEBUG_DOTENV=1` is set. Default is silent — matching Node dotenv's "just works" expectation.
7. Add `.env` to `.gitignore` so users don't accidentally commit local keys.
8. Document in `docs/sample-config.yaml` header: "A `.env` file at ./.env or ~/.modeltap/.env will be auto-loaded at startup (PATCH-0007)."

## Out of Scope

- Variable interpolation beyond what godotenv already supports (it already handles `KEY=${OTHER}`).
- Encrypted `.env` / dotenv-vault equivalent. Separate concern — orthogonal to this patch.
- Hot reload on `.env` change. Load-at-startup only.
- Auto-generating a `.env.example` template. Cheap to add later if demand warrants.
- Warning on loose permissions (e.g., world-readable `.env`). A `.env` file is the user's to own; the warning would be noisy, and PATCH-0004's `file:` resolver already covers the "real" secret-in-a-file path where permissions matter more.
- Changing the default secret-resolver behavior. `env:VAR` / `file:PATH` / pass-through still work identically — `.env` just populates `os.Getenv` before the resolver runs.

## Checklist

- [x] `github.com/joho/godotenv v1.5.1` added — MIT license, Apache-2.0 compatible (ADR-0010)
- [x] `internal/dotenv/dotenv.go` with `Load()` + `MODELTAP_DOTENV` opt-out + `MODELTAP_DEBUG_DOTENV` trace
- [x] Wired into `cmd/modeltap/main.go` before `cli.NewRootCommand`
- [x] `.env` + `.env.local` added to `.gitignore`
- [x] `docs/sample-config.yaml` header updated with usage note
- [x] Tests: missing files (silent), project-local loaded, process env beats file, project-local beats user-level, user-level only, XDG_CONFIG_HOME path, malformed file errors, `MODELTAP_DOTENV=false` skips loader, debug output toggle, all disabled-value variants
- [x] `go vet ./...` clean, `go test ./...` green

## Fix Detail

### File resolution order

```go
candidates := []string{}
if _, err := os.Stat(".env"); err == nil {
    candidates = append(candidates, ".env")
}
for _, userPath := range userLevelPaths() {   // XDG_CONFIG_HOME/modeltap/.env + ~/.modeltap/.env
    if _, err := os.Stat(userPath); err == nil {
        candidates = append(candidates, userPath)
    }
}
```

### Why godotenv.Load (not Overload)

`godotenv.Load` deliberately does NOT overwrite existing env vars. That preserves the usual precedence: a shell-exported `ANTHROPIC_API_KEY` keeps its value even if `.env` also sets one. This matches user intuition (explicit shell export > file) and matches the Node/Python/Ruby dotenv conventions.

### Error shape

- No `.env` anywhere: silent success. Printing "no `.env` found" on every run would be noise for the 90% case.
- `.env` exists but godotenv returns a parse error: return it. Config load continues but the user sees a clear message on stderr.

### Ordering vs PATCH-0004

PATCH-0004's `config.ResolveSecret` reads from `os.Getenv`. PATCH-0007 just makes sure `os.Getenv` has the expected value before the config loader runs. Zero code changes in `internal/config/`.

### Interaction with `MODELTAP_*` viper env-prefix

Unchanged. `.env` loads first; then Viper's `AutomaticEnv` reads from the already-populated environment. So you can also set `MODELTAP_PORT=9090` in `.env` and it behaves identically to exporting it.

### Example user workflow

```bash
cat > ~/.modeltap/.env <<'EOF'
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
EOF
chmod 600 ~/.modeltap/.env
modeltap    # key loaded automatically, no `source`, no `export`
```
