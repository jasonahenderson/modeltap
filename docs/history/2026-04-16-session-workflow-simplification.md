# 2026-04-16 — Session: Workflow Simplification

## Summary

Follow-on to the Phase 1 Track 0 session. User directed two simplifications to the design-review workflow:

1. **Drop A/B/C tiers entirely.** The tiering system (mechanical rules, per-WU tags, mandatory gates) added process complexity without proportional value. Replaced with: user decides what to review and how during Phase 2. No tiers, no mandatory gates.

2. **Drop peer-review prompt files.** Review handoff is chat-only — user pastes whatever they want to their chosen reviewer. No committed prompt artifact.

## What changed

- Removed 58 "Review tier:" tags from all four track files
- Removed tier rules, tier assignment, tier headers, tier procedures from `docs/agents.md`
- Removed tier distribution table from `plan.md`
- Simplified prime directives from 7 rules to 5
- `docs/agents.md` §"Design Review" went from ~115 lines to ~25
- `CLAUDE.md` §"Release Execution" and `AGENTS.md` convention #5 simplified accordingly

## What was kept

- Three-phase workflow: design all → review → code all
- Pre-review lint as an optional designer tool
- Finding severity buckets (Blocking/Attention/Nit)
- Review artifact naming convention
- Prime directives (simplified)

## Process evolution this session (chronological)

This was the tail end of a long session that also covered WU-039 implementation, workflow establishment, and Track 0 designs. The workflow went through these states:

1. Per-WU end-to-end (design → test → impl → review per WU)
2. Added Tier A/B/C with mechanical rules
3. Corrected: Tier C = peer-model only, subagent = pre-review lint
4. Simplified: peer-review handoff is chat-only
5. Phased at release level (design all → review → code all)
6. Added prime directives to prevent drift
7. **Final: dropped tiers entirely; review is user's call**

The takeaway: process should be as simple as possible. The three-phase structure is the valuable insight; the tier machinery was overhead.

## Commits

- `1017be1` ADMIN: add prime directives; make phased workflow painfully clear
- `0fa145d` ADMIN: session log for Phase 1 Track 0 + workflow establishment
- `b95edd0` ADMIN: drop A/B/C tiers; simplify review to user's call

## What's next

Fresh session recommended. Resume Phase 1 with Track A designs.
