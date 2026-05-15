# Architecture Decision Records

This directory holds the architectural decisions that constrain modeltap's design. ADRs are the highest-tier decision document — they capture choices with future constraint value that should not be re-litigated without good reason.

## Current Architecture (Effective Decisions)

modeltap is a single-binary **Go** reverse proxy (ADR-0001) that captures AI/ML API traffic into a per-installation **SQLite** database via `modernc.org/sqlite` in WAL mode (ADR-0002). The CLI uses **Cobra** (ADR-0003) and configuration is managed by **Viper** with non-global instances (ADR-0004). Capture is **always full-fidelity** with retention-based pruning (ADR-0005). Provider adapters implement a clean **multi-provider interface**, with Anthropic and OpenAI built in for v1 (ADR-0006). Usage metrics are pre-computed into **aggregation tables** for fast queries (ADR-0007). The optional knowledge layer uses **sqlite-vec** for vector search (ADR-0008) and exposes its content over an **MCP stdio transport** (ADR-0009). The project ships under **Apache 2.0** (ADR-0010) and is governed under a **BDFL model with contributor tiers** (ADR-0011). Background execution uses **platform-native service managers** (launchd/systemd) rather than a custom daemon (ADR-0012). The terminal harness remains a Bubbletea thin client (ADR-0013, ADR-0014), durable run state is owned by the runtime (ADR-0015), and the former BFF concept is now the **runtime server** with multiple client surfaces (ADR-0016).

## ADR Index

| ADR | Decision | Status |
|-----|----------|--------|
| [0001](0001-programming-language.md) | Go as primary language | Accepted |
| [0002](0002-storage-format.md) | SQLite via modernc.org/sqlite, WAL mode | Accepted |
| [0003](0003-cli-framework.md) | Cobra for CLI | Accepted |
| [0004](0004-configuration-management.md) | Viper, minimal usage, non-global instances | Accepted |
| [0005](0005-capture-mode-strategy.md) | Always full capture, retention-based pruning | Accepted |
| [0006](0006-multi-provider-support.md) | Provider adapter interface, Anthropic + OpenAI for v1 | Accepted |
| [0007](0007-usage-metrics.md) | Pre-computed aggregation tables | Accepted |
| [0008](0008-knowledge-layer-architecture.md) | sqlite-vec, optional module | Accepted |
| [0009](0009-mcp-server-for-knowledge-access.md) | MCP stdio transport | Accepted |
| [0010](0010-open-source-license.md) | Apache 2.0 | Accepted |
| [0011](0011-contribution-model-and-governance.md) | BDFL with contributor tiers | Accepted |
| [0012](0012-background-execution-strategy.md) | launchd + systemd integration | Accepted |
| [0013](0013-terminal-ui-framework.md) | Bubbletea (Charm ecosystem) for terminal UI | Proposed |
| [0014](0014-harness-base-strategy.md) | Continue modeltap harness (universal orchestration client) | Accepted |
| [0015](0015-run-runtime.md) | Run runtime ownership and semantics | Accepted |
| [0016](0016-runtime-server-and-client-surfaces.md) | Runtime server and client surfaces | Accepted |

## When to Write an ADR

- The decision constrains future implementation work in a way that should not be reversed casually
- Multiple plausible options exist and the trade-offs deserve to be recorded
- Future contributors will need to understand *why* the choice was made, not just what was chosen
- The decision has scoring drivers and a defensible rationale

## When NOT to Write an ADR

- The topic is still being framed and the solution space is open → use an **exploration** in `.sdlc/explorations/`
- The work is behavior-scoped (user-facing capability) → use a **feature spec** in `.sdlc/features/`
- The work is implementation-scoped (bug fix, missing endpoint, internal plumbing) → use a **patch** in `.sdlc/patches/`
- The change is process / workflow / instruction-file only → commit with an `ADMIN:` prefix, no doc needed

## Naming

Files: `NNNN-short-title.md` — four-digit zero-padded sequence (e.g. `0006-multi-provider-support.md`).

Identifiers: `ADR-NNNN` in the document title heading (e.g. `# ADR-0006: Multi-Provider Support Strategy`).

Numbering is monotonic; do not reuse a number. Superseded ADRs stay in place with a status update and a forward reference.

## Format

ADRs use YAML frontmatter for machine-readable metadata followed by a structured markdown body:

```markdown
---
status: proposed | accepted | superseded by ADR-NNNN | deprecated
date: YYYY-MM-DD
decision-makers: Name, Name (optional)
parent: FEAT-NNNN
series: Human-readable grouping name
series-role: member
series-order: 0
---

# ADR-NNNN: Title

## Context and Problem Statement
What is the situation that requires a decision? What are the forces at play?

## Decision Drivers
Weighted criteria (1-5, 5 = critical) that the options will be scored against.

## Considered Options
The realistic alternatives that were evaluated.

## Decision Outcome
Chosen option and why. Include consequences (positive, negative, neutral).

## Pros and Cons of the Options
One subsection per option, with a brief evaluation against the drivers.
```

Existing ADRs in this directory follow this shape with minor variations. New ADRs should match.

Required fields:

- `status`
- `date`

Optional grouping fields:

- `parent`: canonical parent artifact ID
- `series`: human-readable grouping name
- `series-role`: `umbrella` or `member`
- `series-order`: optional integer for planned order within the series

Use grouping metadata when an ADR supports an umbrella feature, exploration, or
cross-artifact work stream. Do not encode hierarchy in the ADR identifier.

## Lifecycle

1. **Propose** — Write the ADR with `status: proposed`. Include drivers, options, and a recommended outcome.
2. **Review** — Findings land in `docs/adr/.reviews/{stem}-findings.md` if formal review is required (see `.reviews/README.md`).
3. **Accept** — Status flips to `accepted` once the decision is final. The ADR is now load-bearing for downstream features and patches.
4. **Supersede** — If a later ADR replaces this one, update the status to `superseded by ADR-NNNN` and add a forward-reference note at the top. Do not delete the original — its history matters.

## Commit Convention

- ADR commits should have descriptive subjects that name the ADR (e.g. `ADR-0012: Background Execution Strategy`)
- Use `git commit -s` for DCO sign-off

## Reviews

Review artifacts (canonical findings, plan reviews, syntheses) live under `docs/adr/.reviews/`. See `.reviews/README.md` for the layout.

## Relationship to Other Docs

| Doc Type | Scope | Lives In |
|----------|-------|----------|
| Exploration | Upstream problem framing and design-space exploration | `.sdlc/explorations/` |
| **ADR** | **Architectural decisions with future constraint value** | **`docs/adr/`** |
| Feature spec | Behavior — user-visible capabilities | `.sdlc/features/` |
| Patch | Implementation — fixes, missing endpoints, internal work | `.sdlc/patches/` |
| Work unit (`WU-NNN`) | Planned increments inside a feature, tracked in status.md | `.sdlc/history/` |
