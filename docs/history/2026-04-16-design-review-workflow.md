# 2026-04-16 — Design Review Workflow Added to Agent Team

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

Post-WU-039, identified a gap in the agent workflow: the pipeline went Designer → Tester with no review step. Design errors baked into tests and cascaded. WU-039's design was produced and consumed by the same agent (me, in different roles), with no fresh-eyes check before tests were written.

## Decision

Added a **Design Review** stage between Designer and Test Engineer with three tiers:

- **Tier A** — self-checklist by the Designer. Low-risk WUs.
- **Tier B** — user review required before handoff. Mid-risk.
- **Tier C** — external peer review (preferred) or Claude subagent review (fallback). High-risk: contract surfaces, security-sensitive, cross-track.

Tier selection is mechanical (rule-based, yes/no checks), escalate-only, and recorded in each design doc's header.

### Rules (from `docs/agents.md`)

**Tier C** if any of:
- Files in `internal/protocol/`, `internal/bff/{transport,server,connection,capabilities}.go`, or any package imported by both Track A and Track B
- Creates/modifies a protocol message, shared interface, stable on-disk schema, or ADR
- Touches credentials, tokens, TLS, model-supplied file paths, shell invocation, network listeners, permission policy, or tool execution
- Cross-track dependency beyond Track 0

**Tier B** if any of (and not already C):
- Size = L
- ≥3 new `.go` files
- Modifies config.go or adds config keys
- Adds a new Cobra command
- Defines UI layout or key bindings

**Tier A** otherwise.

### Tier C reviewer options (priority order)

1. External LLM via user-mediated submission (Codex, Kimi, GPT-5, Gemini) — matches existing plan-review pattern
2. Claude subagent with fresh context — **not a substitute for #1**; labeled as `claude-subagent-*` to reflect same-model scope
3. Human maintainer when available

Multi-model routing is planned as a Modeltap feature (FEAT-0008 + FEAT-0013 review roles). Until that ships, Tier-C external review is user-driven.

## Tier Distribution for v0.2.0

Applied the rules to all 58 WUs:

| Track | C | B | A | Total |
|-------|---|---|---|-------|
| 0 (shared) | 9 | 0 | 0 | 9 |
| A (BFF) | 11 | 8 | 4 | 23 |
| B (Harness) | 11 | 8 | 2 | 21 |
| Integration | 2 | 2 | 1 | 5 |
| **Total** | **33** | **18** | **7** | **58** |

57% Tier C reflects that v0.2.0 is mostly contract and infrastructure work. The ratio will drop in later releases as the foundation stabilizes.

## Bundled Review Option

WUs sharing a contract surface may go through one review. Candidates for bundling:
- **Protocol types bundle:** WU-040 + WU-041 + WU-093 (all protocol-types expansion)
- **Tool bundle:** WU-076 + WU-077 + WU-078 + WU-079 (all harness tools under the same permission model)
- **Storage bundle:** WU-045 + WU-091 + WU-096 (all storage-schema / migration work)

Reduces review overhead for closely related WUs. Artifact name uses the topic or WU range.

## Files Modified

- `docs/agents.md` — added "Design Review" section, updated workflow diagram
- `docs/releases/v0.2.0/track-0-shared.md` — tier on all 9 WUs
- `docs/releases/v0.2.0/track-a-bff-server.md` — tier on all 23 WUs
- `docs/releases/v0.2.0/track-b-terminal-harness.md` — tier on all 21 WUs
- `docs/releases/v0.2.0/track-integration.md` — tier on all 5 WUs

## Retroactive WU-039 Review

WU-039 shipped without review (design-review workflow didn't exist yet). Running a retroactive Tier-C review via Claude subagent as the first trial of the new process. Artifact will land at `docs/releases/v0.2.0/.reviews/wu-039/claude-subagent-design-review.md`. Any blocking findings will be fixed before WU-040 begins.

## What's Next

1. Retroactive WU-039 review (this session)
2. Apply workflow starting WU-040
3. Reconsider rule calibration after 5–10 WUs of real experience with the tiers
