# Modeltap

Reverse proxy for AI/ML clients that captures requests/responses, tracks usage metrics, and provides a cross-model knowledge layer.

## Key References

- Architecture decisions: `docs/adr/` (only `status: accepted` ADRs drive work) — see `docs/adr/README.md` for the index, format, and lifecycle
- Explorations: `docs/explorations/` (upstream problem framing; does not by itself authorize implementation) — see `docs/explorations/README.md`
- Feature specs: `docs/features/` (only `status: accepted` features drive work) — see `docs/features/README.md` for the index, format, and lifecycle
- Patches: `docs/patches/` (implementation-scoped fixes, missing-endpoint coverage, internal plumbing) — see `docs/patches/README.md` for when to use vs. ADR or feature spec
- Agent team definition: `docs/agents.md`
- OpenCode / generic agent instructions: `AGENTS.md`
- Release plans and status: `docs/releases/` — each release (vX.Y.Z) has `plan.md`, `status.md`, `track-*.md`, and `changelog.md`
- Current active release: `docs/releases/v0.2.0/`
- Work logs (session history): `docs/history/`

## Doc Type Taxonomy

| Doc Type | Scope | Lives In | Identifier |
|----------|-------|----------|------------|
| Exploration | Upstream problem framing and design-space exploration | `docs/explorations/` | `EXP-NNNN` |
| ADR | Architectural decisions with future constraint value | `docs/adr/` | `ADR-NNNN` |
| Feature spec | Behavior — user-visible capabilities | `docs/features/` | `FEAT-NNNN` |
| Patch | Implementation — fixes, missing endpoints, internal work | `docs/patches/` | `PATCH-NNNN` |
| Work unit | Planned increments inside an accepted feature | tracked in `docs/releases/<version>/status.md` | `WU-NNN` |
| Admin task | Repo workflow / instruction / process changes | no numbered doc required by default | `ADMIN` |

`PATCH` does not mean semver patch — it means implementation-scoped work. `ADMIN:` commits cover repo workflow / instruction-file changes and don't need a numbered doc.

### Which Artifact to Use

- Use an **exploration** when the problem is still being framed, multiple solution shapes are plausible, or the topic may later promote into a feature, ADR, or patch.
- Use a **feature spec** when the work is behavior-scoped and needs user-visible scope, capabilities, and success criteria.
- Use a **patch** when the work is implementation-scoped and a checklist is enough to define "done."
- Use an **ADR** when the work requires a hard architectural choice with future constraint value.
- Use **ADMIN** for repo workflow / instruction / prompt / process changes such as `CLAUDE.md`, `AGENTS.md`, review structure, status-process rules, or similar meta work.

If a change mixes product work and repo-process work, split it into separate artifacts or commits rather than forcing a single classification.

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

When working as part of the agent team, follow the workflow defined in `docs/agents.md`. `AGENTS.md` is the concise cross-agent version of the same expectations. Every significant action must be logged to `docs/history/`.

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

1. Read `docs/releases/<current-version>/status.md` (check `docs/releases/README.md` for the current release)
2. Check what release phase is active (Phase 1 design / Phase 2 peer review / Phase 3 implementation — see `plan.md` and `docs/agents.md` §"Workflow")
3. Check if any "In Progress" items are actually complete (files exist, tests pass)
4. Update status accordingly
5. Pick next task from "Up Next" within the current phase
6. Log plan before starting, summary after completion

### Release Execution — PRIME DIRECTIVES

Releases execute in three sequential phases at the **release level, not the WU level**. Do not interleave phases. The canonical version of this lives in `docs/agents.md` §"Workflow / Prime directives"; this is the Claude-facing digest.

1. **Phase 1 — Design ALL WUs.** For every WU in the release, produce a design doc. Tier-B and Tier-C WUs also get a subagent pre-review lint (Claude with fresh context) that checks for spec drift and scope gaps. No coding in Phase 1. No peer reviews in Phase 1. Finish every WU's design before Phase 2.
2. **Phase 2 — Batched peer review (opt-in, skippable).** User flags which designs warrant cross-model peer review; the batch runs in one external-model session. No new designs, no coding. Phase 2 can be skipped entirely if subagent lint coverage is deemed sufficient.
3. **Phase 3 — Implement ALL WUs.** Red → green → security → docs per WU, in any dependency-legal order. No new design work during Phase 3 (revise design doc explicitly or file a patch if implementation reveals a flaw).

**Subagent pre-review lint is NOT Tier C.** A Claude subagent shares Claude's training distribution; it cannot catch Claude-family reasoning blind spots. Tier C requires a *different* model family (Codex, Kimi, GPT-5, Gemini, or a human maintainer) — user-driven, batched in Phase 2.

**Peer-review handoff is chat-only — no committed prompt file.** The Designer announces the request in chat with the WU/bundle identifier and file paths; the user supplies their own framing to the external model. Prescriptive prompts bias the outcome.

**Current phase lives in `docs/releases/<version>/plan.md` §"Phased Execution".** Check it on resumption. Any action that doesn't match the current phase is wrong.

**Phase transitions are ADMIN commits** that bump the phase marker in plan.md and (optionally) announce readiness for the next phase.

Every WU carries a **review tier** (A / B / C) in its track-file spec. Tier rules and per-tier procedures live in `docs/agents.md` §"Design Review". Tier C marks a WU as *eligible* for Phase 2 peer review; it does not automatically trigger one.

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
Ref: docs/features/0003-web-dashboard.md
```

```text
PATCH-0002: add local inference provider support

Adds implementation-scoped provider, routing, and metrics work for MLX and Ollama support.
Ref: docs/patches/0002-local-inference-support.md
```

```text
ADMIN: add exploration taxonomy and agent instruction files

Introduces docs/explorations/README.md and aligns CLAUDE.md / AGENTS.md with commit-prefix rules.
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
