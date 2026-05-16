# 2026-04-28 — Ultron Evaluation (EXP-0013)

## Context

User asked for an exploration of [modelscope/ultron][ultron-repo] — a
self-evolving collective-intelligence service released by ModelScope
(Alibaba) in April 2026. The question: should modeltap integrate it,
mine it for ideas, or just note it as prior art.

[ultron-repo]: https://github.com/modelscope/ultron

## Discussion

- Researched Ultron via README, ROADMAP, HTTP API doc, SDK doc, and
  package layout. Confirmed:
  - Python FastAPI server on `:9999`, no MCP, no Go SDK, no published
    OpenAPI beyond FastAPI default.
  - SQLite-backed with mixin classes (memory / skill / harness / cluster
    / trajectory / SFT-training / user / ingestion / catalog).
  - DashScope `text-embedding-v4` and Qwen via OpenAI-compatible client
    by default; `sentence-transformers` fallback.
  - Apache-2.0 — compatible with ADR-0010.
  - Pre-alpha: created 2026-04-09, ~20 commits, 65 stars, 0 releases at
    time of writing.
- Identified the load-bearing design moves: tiered L0/L1/Full memory with
  hotness decay `exp(-α·days)`, LLM-classified memory type,
  structure-score-gated skill evolution with provenance, intent-expanded
  retrieval, filesystem-first skill format
  (`{slug}-{version}/SKILL.md + _meta.json + scripts/`).
- Mapped overlap to existing modeltap artifacts: EXP-0001 / ADR-0008
  (Memory Hub), FEAT-0011 (Knowledge Integration), FEAT-0012 (Skills and
  Agent Teams), FEAT-0013 (Agent Teams), FEAT-0014 (Harness Conversation
  Shell), EXP-0008 (harness profiles).
- Framed four real options: ignore / mine for design / honor `SKILL.md`
  filesystem layout / sidecar. Rejected sidecar (Option D) for the core
  on grounds of Python runtime, pre-alpha API, DashScope-default
  embedding stack, and split SQLite state.
- Flagged two judgment calls: (1) Ultron's "soul presets" (MBTI/zodiac
  persona injection) are aesthetically opposite to EXP-0009's
  prompt-architecture stance — adopt the format only without that part.
  (2) Ultron is sized for fleets of general-purpose agents; importing
  too much of its framing risks pulling FEAT-0012 toward an agent
  marketplace before the coding-harness story is solid (EXP-0011
  territory).
- User decided to set status to `watching` — not actively pursuing, but
  tracking. Added `watching` to the documented status set in
  `.sdlc/explorations/README.md`.

## Files Created

- `.sdlc/explorations/0013-ultron-evaluation.md` — EXP-0013 exploration
  (status: `watching`).
- `.sdlc/history/2026-04-28-session-ultron-evaluation.md` — this log.

## Files Modified

- `.sdlc/explorations/README.md` — added EXP-0013 to the index; added
  `watching` to the documented status values.

## Open Items / Next Steps

- Defer the half-day read pass through Ultron's component docs and
  `db_*.py` mixins until either (a) Ultron stabilizes (releases, OpenAPI,
  reduced API churn) or (b) FEAT-0011 / FEAT-0012 reach a design point
  where Ultron's specific schemas would meaningfully inform decisions.
- Re-evaluate when there is a tagged release, a published OpenAPI, or
  an MCP surface upstream — any of those would lower the cost of
  integration work and could move this back to `exploring`.
- Do not pursue Option D (sidecar) without an explicit decision in a
  later round.

## Notes

- No code changes. Exploration only — does not authorize implementation.
- Verified Apache-2.0 license against ADR-0010 per the standing
  dep-license-check guidance.
- This session also extends the explorations status taxonomy with
  `watching` to cover external-project tracking — used here for Ultron
  but applicable to any future "wait and see" exploration.
