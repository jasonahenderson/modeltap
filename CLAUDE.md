# Modeltap

Reverse proxy for AI/ML clients that captures requests/responses, tracks usage metrics, and provides a cross-model knowledge layer.

## Key References

- Canonical process rules: `.agents/process.md`
- Agent-team contract: `.agents/contracts/agent-team.md`
- Base agent contract: `.agents/contracts/base.md`
- Architecture decisions: `docs/adr/` (only `status: accepted` ADRs drive work) — see `docs/adr/README.md` for the index, format, and lifecycle
- Explorations: `.sdlc/explorations/` (upstream problem framing; does not by itself authorize implementation) — see `.sdlc/explorations/README.md`
- Feature specs: `.sdlc/features/` (only `status: accepted` features drive work) — see `.sdlc/features/README.md` for the index, format, and lifecycle
- Patches: `.sdlc/patches/` (implementation-scoped fixes, missing-endpoint coverage, internal plumbing) — see `.sdlc/patches/README.md` for when to use vs. ADR or feature spec
- Agent team definition: `docs/agents.md`
- OpenCode / generic agent instructions: `AGENTS.md`
- Release plans and status: `.sdlc/releases/` — each release (vX.Y.Z) has `plan.md`, `status.md`, `track-*.md`, and `changelog.md`
- Current active release: `.sdlc/releases/v0.3.0/`
- Work logs (session history): `.sdlc/history/`

## Precedence

When process guidance overlaps:

1. `.agents/process.md` is canonical for process rules.
2. `.agents/contracts/*.md` define role-specific expectations.
3. `AGENTS.md` is the concise cross-tool entrypoint.
4. `CLAUDE.md` is Claude-specific guidance layered on top.

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

When working as part of the agent team, follow `.agents/contracts/agent-team.md` and `docs/agents.md`. Every significant action must be logged to `.sdlc/history/`.

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

Every session must be logged to `.sdlc/history/`, regardless of whether agent roles are active. At the end of each session (or when the user is done), write a session log to `.sdlc/history/<YYYY-MM-DD>-session-<short-description>.md` containing:
- What was discussed or decided
- What actions were taken
- Files created or modified
- What's next / open items

This ensures continuity across sessions even for planning, review, or ad-hoc conversations.

### Resumption Protocol

1. Read `.sdlc/releases/<current-version>/status.md` (check `.sdlc/releases/README.md` for the current release)
2. Check what release phase is active (Phase 1 design / Phase 2 peer review / Phase 3 implementation — see `plan.md` and `docs/agents.md` §"Workflow")
3. Check if any "In Progress" items are actually complete (files exist, tests pass)
4. Update status accordingly
5. Pick next task from "Up Next" within the current phase
6. Log plan before starting, summary after completion

### Release Execution

Releases execute in three sequential phases at the **release level, not the WU level**. Do not interleave. Canonical rules live in `.agents/process.md`.

1. **Phase 1 — Design ALL WUs across ALL tracks.** Produce design docs. Optional: run pre-review lint (Claude subagent, fresh context) to catch mechanical drift. No coding. No reviews. Phase 1 is not complete until every track has designs for every WU.
2. **Phase 2 — Review.** User decides what to review and how (read directly, send to external model, both, or skip). No new designs. No coding. Begins only when user confirms Phase 1 complete.
3. **Phase 3 — Implement ALL WUs.** Red → green → security → docs per WU, any dependency-legal order. No new designs; revise explicitly if implementation reveals a flaw. Begins only after Phase 2 findings are processed.

Current phase lives in `.sdlc/releases/<version>/plan.md`. Phase transitions are explicit `ADMIN:` commits — never implicit.

### Commit Policy

Every completed work unit is a commit point. Commit immediately when a work unit's definition of done is met. Do not batch multiple work units into a single commit.

Code and product-affecting changes must trace to one of:

- an accepted feature with an active or planned `WU-NNN`
- an approved patch
- an ADR, feature, or exploration authoring/editing change
- an `ADMIN:` task when the change is process-only

Explorations are upstream rationale artifacts. They do **not** authorize code changes by themselves.

Commit prefixes:

- `WU-NNN: <short description>` for implementation work units under accepted features
- `PATCH-NNNN: <short description>` for implementation-scoped work under an approved patch
- `FEAT-NNNN: <short description>` for drafting or revising a feature spec itself
- `ADR-NNNN: <short description>` for drafting or revising an ADR itself
- `EXP-NNNN: <short description>` for drafting or revising an exploration itself
- `ADMIN: <short description>` for repo process / instruction / workflow changes, with no numbered doc required by default

Commit body requirements:

- Briefly state what changed in 1-3 lines
- Reference the canonical doc path when one exists (feature, patch, ADR, exploration)
- Use `git commit -s` for DCO sign-off
- Include `Co-Authored-By` only if the current workflow or user explicitly requires it

Examples:

```text
WU-039: add MCP search_knowledge command

Implements the CLI surface and handler wiring for MCP-backed semantic search.
Ref: .sdlc/features/0003-web-dashboard.md
```

```text
PATCH-0002: add local inference provider support

Adds implementation-scoped provider, routing, and metrics work for MLX and Ollama support.
Ref: .sdlc/patches/0002-local-inference-support.md
```

```text
ADMIN: add exploration taxonomy and agent instruction files

Introduces .sdlc/explorations/README.md and aligns CLAUDE.md / AGENTS.md with commit-prefix rules.
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
