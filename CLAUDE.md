# Modeltap

Reverse proxy for AI/ML clients that captures requests/responses, tracks usage metrics, and provides a cross-model knowledge layer.

## Key References

- Architecture decisions: `docs/adr/` (only `status: accepted` ADRs drive work)
- Feature specs: `docs/features/` (only accepted features drive work)
- Agent team definition: `docs/agents.md`
- Project status: `docs/history/status.md`
- Work logs: `docs/history/`

## Technology Stack (from accepted ADRs)

- **Language:** Go (ADR-0001)
- **Storage:** SQLite via modernc.org/sqlite, WAL mode (ADR-0002)
- **CLI:** Cobra (ADR-0003)
- **Config:** Viper, minimal usage, `viper.New()` not global (ADR-0004)
- **Capture:** Always full, retention-based pruning (ADR-0005)
- **Providers:** Adapter interface, Anthropic + OpenAI for v1 (ADR-0006)
- **Metrics:** Pre-computed aggregation tables (ADR-0007)
- **Knowledge:** sqlite-vec, optional module (ADR-0008)
- **MCP:** Stdio transport (ADR-0009)
- **License:** Apache 2.0 (ADR-0010)
- **Governance:** BDFL with contributor tiers (ADR-0011)

## Agent Team

When working as part of the agent team, follow the workflow defined in `docs/agents.md`. Every action must be logged to `docs/history/`.

### Agent Roles

When invoked with `--agent <name>`, adopt the corresponding role:

- **tpm** — Reads accepted ADRs/features, breaks work into incremental units, maintains plan and status, coordinates agents
- **designer** — Produces detailed technical designs from ADR decisions and feature requirements
- **tester** — Writes unit tests before implementation (TDD), defines "done" criteria
- **backend** — Writes Go production code: proxy, storage, CLI, providers, metrics, config
- **ui** — Builds web dashboard: HTML/CSS/JS, Go API handlers, embedded assets via `embed.FS`
- **infra** — Build system, CI/CD, goreleaser, Makefile, GitHub Actions, Dockerfile
- **integration** — End-to-end tests across components (proxy -> storage -> API -> UI)
- **security** — Reviews code for OWASP top 10, SQL injection, XSS, CSRF, input validation, information leakage
- **docs** — Writes user and developer documentation for completed, reviewed components

### Session Logging

Every session must be logged to `docs/history/`, regardless of whether agent roles are active. At the end of each session (or when the user is done), write a session log to `docs/history/<YYYY-MM-DD>-session-<short-description>.md` containing:
- What was discussed or decided
- What actions were taken
- Files created or modified
- What's next / open items

This ensures continuity across sessions even for planning, review, or ad-hoc conversations.

### Resumption Protocol

1. Read `docs/history/status.md`
2. Check if any "In Progress" items are actually complete (files exist, tests pass)
3. Update status accordingly
4. Pick next task from "Up Next"
5. Log plan before starting, summary after completion

### Commit Policy

Every completed work unit is a commit point. Commit immediately when a work unit's definition of done is met. Do not batch multiple work units into a single commit.

Commit message format:
```
WU-<NNN>: <short description>

<what was done, 1-3 lines>

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

Additional commit points within a work unit (commit these as you go, don't wait):
- Infrastructure/scaffolding is in place and compiles (`go build ./...` passes)
- Tests are written and fail as expected (red phase)
- Implementation passes all tests (green phase)
- Security fixes applied after review

Use `git commit -s` for DCO sign-off on all commits.

### Conventions

- Go code: `gofmt`, `go vet`, effective Go idioms
- Tests: table-driven, in `_test.go` files alongside production code
- Config path: `~/.config/modeltap/config.yaml`
- Env vars: `MODELTAP_` prefix
