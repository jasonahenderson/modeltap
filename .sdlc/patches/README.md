# Patches

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

Implementation-scoped work authorization documents. Patches are the lightweight counterpart to feature specs (`.sdlc/features/`) and ADRs (`.sdlc/adr/`).

## Patch Index

| Patch | Title | Status |
|-------|-------|--------|
| [PATCH-0001](0001-openai-responses-api-support.md) | OpenAI Responses API Support | proposed |
| [PATCH-0002](0002-local-inference-support.md) | Local Inference Support | proposed |
| [PATCH-0008](0008-moonshot-provider-adapter.md) | Moonshot Provider Adapter (Kimi K2.6) | proposed |
| [PATCH-0004](0004-secret-prefix-resolver.md) | Secret Prefix Resolver for Provider API Keys | done |
| [PATCH-0005](0005-bff-route-via-proxy-default.md) | Route BFF provider traffic through the v0.1 proxy by default | approved |
| [PATCH-0006](0006-unified-config-data-dir.md) | Unified `~/.modeltap/` config & data directory | done |
| [PATCH-0007](0007-dotenv-loader.md) | `.env` loader for provider credentials | done |
| [PATCH-0009](0009-root-readme.md) | Root `README.md` | done |
| [PATCH-0010](0010-makefile-hygiene.md) | Makefile hygiene — PATH-resolved Go + check-only default | done |
| [PATCH-0011](0011-harness-ux-polish.md) | Harness UX Polish — OpenCode theme port, borders, sensible keybindings | done |
| [PATCH-0012](0012-lint-out-of-default-target.md) | Remove `lint` from the Makefile default target | done |
| [PATCH-0013](0013-sqlite-busy-timeout.md) | Set SQLite `busy_timeout` on every pool connection | proposed |
| [PATCH-0014](0014-bff-shutdown-waitgroup-race.md) | Fix BFF Server `sync.WaitGroup` race between accept and Shutdown | approved |
| [PATCH-0015](0015-harness-shell-component-api.md) | Harness Shell Component API | approved |
| [PATCH-0016](0016-pr1-ci-test-failures-triage.md) | Fix v0.2.x test suite failures and lint regressions surfaced by PR #1 CI | approved |
| [PATCH-0017](0017-session-scoped-project-context.md) | Session-scoped project context | proposed |
| [PATCH-0018](0018-host-info-events.md) | Surface slash-command output via transcript host-info events | approved |
| [PATCH-0019](0019-read-command-store-wiring.md) | Wire SQLite store into logs, show, export, metrics commands | approved |
| [PATCH-0020](0020-requests-command-rename.md) | Rename logs/show/export to requests list/show/export | approved |
| [PATCH-0021](0021-bff-health-check-wiring.md) | Wire provider health checks in BFF production startup | approved |
| [PATCH-0022](0022-turn-submit-max-tokens-default.md) | Set default max_tokens for turn.submit dispatch | approved |
| [PATCH-0023](0023-shell-host-command-dispatch.md) | Dispatch host-native slash commands as RunHostCommandAction | approved |
| [PATCH-0024](0024-show-short-id-prefix.md) | Support short-id prefix lookup in requests show | approved |
| [PATCH-0025](0025-cloud-probe-upstream-target.md) | Probe cloud-provider upstream directly in health check | approved |
| [PATCH-0026](0026-shell-debug-daemon-log.md) | Capture auto-spawned daemon stdio to a log file via flag/env | approved |
| [PATCH-0027](0027-truthful-footer-hints.md) | Strip misleading footer hints (sidebar / palette / agents) | approved |
| [PATCH-0028](0028-session-create-rpc.md) | Add session.create RPC and harness auto-call on Ready | approved |
| [PATCH-0029](0029-bootstrap-session-race.md) | Fix bootstrapSession race that overwrites turn-assigned session id | approved |
| [PATCH-0030](0030-shell-select-mode.md) | Add terminal text selection support | done |
| [PATCH-0031](0031-turn-sequence-semantics.md) | Align BFF Conversation.sequence with harness user-turn semantics | proposed |
| [PATCH-0032](0032-sse-ndjson-dual-mode.md) | Make SSEParser pass NDJSON lines through for Ollama-style providers | proposed |
| [PATCH-0033](0033-rpc-error-formatting.md) | Unwrap RPCError framing in shell statusError; friendlier terminal-run reject | proposed |
| [PATCH-0034](0034-focus-agnostic-scroll.md) | Focus-agnostic transcript scroll hotkeys | proposed |
| [PATCH-0035](0035-elapsed-seconds-placeholder.md) | v0.3.0 placeholder: append elapsed seconds to streaming status | proposed |
| [PATCH-0036](0036-slash-commands-during-streaming.md) | Dispatch slash commands before queue check so /cancel works during streaming | proposed |
| [PATCH-0037](0037-help-command.md) | Add /help command listing the host slash-command surface | proposed |
| [PATCH-0038](0038-session-semantics-redefine.md) | Redefine /clear as new-session; auto-resume most-recent session on launch; /sessions current | proposed |

## When to Use a Patch

- The work is implementation-scoped: bug fixes, missing endpoint coverage, internal plumbing, tooling, infrastructure, DX, test-harness improvements
- The work affects the product codebase or its delivery system
- The problem and scope are well-defined
- A checklist is sufficient to define "done"
- No personas, user stories, or acceptance criteria are needed

## When NOT to Use a Patch

- The work is still problem framing or design-space exploration → use an **exploration** in `.sdlc/explorations/`
- The work is behavior-scoped (new capability surfaced to users, multiple personas, success criteria) → use a **feature spec** in `.sdlc/features/`
- The work requires an architectural choice with future constraint value → use an **ADR** in `.sdlc/adr/`
- The change is repo process / workflow / instruction-file only (CLAUDE.md, agents.md, skills, hooks) → commit directly with an `ADMIN:` prefix, no doc needed

Patch classification is about scope, not release semantics:

- `PATCH` here does **not** mean semver patch
- Use a patch for implementation-scoped work whether it's a bug fix, a missing-endpoint adapter extension, a CI/tooling addition, or an internal plumbing improvement
- Do **not** use a patch for workflow or process artifacts (those are `ADMIN:` commits)

## Naming

Files: `NNNN-title-with-dashes.md` — four-digit zero-padded sequence (e.g., `0001-openai-responses-api-support.md`).

Identifiers: `PATCH-NNNN` in the document title (e.g., `PATCH-0001`).

Numbering is monotonic across the patches directory; do not reuse a number even if a patch is abandoned.

## Format

Patches use YAML frontmatter, matching features, explorations, and ADRs.

```markdown
---
patch: PATCH-NNNN
title: Human-readable title
status: proposed | approved | done
date: YYYY-MM-DD
related:
  - FEAT-NNNN: Associated feature, ADR, or patch
branch: patch/NNNN-short-name
pr: optional PR link or identifier
parent: FEAT-NNNN
series: Human-readable grouping name
series-role: member
series-order: 1
---

# PATCH-NNNN: Title

## Problem
What's broken or missing, in 2-3 sentences. Include severity and how it was discovered if relevant.

## Scope
Bulleted list of what this patch does. Reference concrete file paths and line numbers where possible.

## Out of Scope
What it explicitly does NOT do. Adjacent bugs, broader refactors, future work.

## Checklist
- [ ] Concrete deliverables, testable items
- [ ] Tests added or updated
- [ ] Build and vet pass

## Fix Detail (optional)
Code snippets, design notes, or commentary that don't fit in the checklist but help a reviewer understand the change.
```

Required fields:

- `patch`
- `title`
- `status`
- `date`

Optional fields:

- `related`
- `branch`
- `pr`
- `release`
- `parent`
- `series`
- `series-role`
- `series-order`

## Lifecycle

1. **Propose** — Write the patch doc with status `proposed`. Number it next in sequence.
2. **Approve** — Human review. Status flips to `approved` once the scope is agreed.
3. **Implement** — Commits land with `PATCH-NNNN:` prefix on a `patch/NNNN-short-name` branch (or directly to main for trivial fixes, noted in the patch doc). Multiple commits may iterate under one patch.
4. **Done** — All checklist items complete, status flips to `done`, PR link recorded in the doc.

## Commit Convention

- Branch: `patch/NNNN-short-name`
- Commit subject: `PATCH-NNNN: short description`
- Commit body references the patch doc path so reviewers can jump to the authorization
- Use `git commit -s` for DCO sign-off, matching the project's existing commit policy in `CLAUDE.md`

## Relationship to Other Docs

| Doc Type | Scope | Lives In |
|----------|-------|----------|
| Exploration | Upstream problem framing and design-space exploration | `.sdlc/explorations/` |
| Feature spec | Behavior — new capabilities, user-visible work | `.sdlc/features/` |
| **Patch** | **Implementation — fixes, missing endpoints, internal work** | **`.sdlc/patches/`** |
| ADR | Architectural decisions with future constraint value | `.sdlc/adr/` |
| Work unit (`WU-NNN`) | Planned increments inside a feature, tracked in status.md | `.sdlc/history/` |

Work units (`WU-NNN`) are the existing planning unit for advancing accepted features through `tpm`. Patches are a separate axis for fixes and small implementation-scoped work that doesn't fit a feature's work-unit plan.

## Grouping

Use optional grouping metadata when a patch belongs to an umbrella feature,
roadmap, or other cross-artifact work stream:

```yaml
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 9
```

Do not encode hierarchy in the patch identifier (`PATCH-0016a`, etc.). Keep the
ID monotonic and put the relationship in metadata.

## Promotion

An exploration may promote directly to a patch when the topic becomes implementation-scoped and a checklist is enough.

If a patch grows past ~8 checklist items, sprouts user stories, or starts requiring an architectural choice, promote it:

- To a **feature spec** if it has become behavior-scoped — keep the patch doc and add a `Promoted to:` reference at the top.
- To an **ADR** if a real architectural decision emerged — same pattern.

## Reviews

Review artifacts (findings, plan reviews, syntheses) live under `.sdlc/patches/.reviews/`. See `.reviews/README.md` for layout.
