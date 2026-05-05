---
patch: "PATCH-0009"
title: "Root `README.md`"
status: "done"
date: "2026-04-21"
related:
  - "`docs/usage-guide.md`"
  - "`CONTRIBUTING.md`"
  - "`GOVERNANCE.md`"
  - "FEAT-0008–0013 (harness direction)"
branch: "exploration/integrated-harness"
---

# PATCH-0009: Root `README.md`

## Problem

The repo root has no `README.md`. Anyone landing on the GitHub page — or on a local clone — gets a directory listing with no framing: no elevator pitch, no "what is this," no quick start, no pointer to the user guide or contribution docs. The install story ("clone, `make build`, run `./bin/modeltap`") lives only in `docs/usage-guide.md`, which a visitor has to know to open.

This is also the most-visible doc for the reframing in FEAT-0008–0013 — modeltap as an integrated AI environment, not just a capture proxy. A reader who only has the CLI usage guide will walk away thinking the project is narrower than it is, and a potential contributor has nowhere obvious to start.

## Scope

1. Add `README.md` at the repo root with these sections, in this order:
   - **Header** — one-line tagline and a short paragraph positioning modeltap. Must cover both the current v0.1 reality (capture proxy for Anthropic + OpenAI with dashboard, metrics, service management) and the near-term direction (integrated AI environment per FEAT-0008–0013) without overclaiming unshipped features.
   - **Why modeltap** — 3–5 bullets on what the project gives the user that's hard to get elsewhere: local-first capture, cross-provider cost/token accounting, SQLite-backed history, provider-agnostic routing, knowledge layer on the roadmap.
   - **Quick start** — clone → `make build` → `modeltap start` → point a client at `http://localhost:8080` using `ANTHROPIC_BASE_URL` or `OPENAI_BASE_URL`. Short enough to fit on one screen; links to `docs/usage-guide.md` for the full story.
   - **What's in this repo** — brief map: `cmd/`, `internal/`, `pkg/`, `docs/adr`, `docs/features`, `docs/patches`, `docs/releases`. Keep it a table, not prose.
   - **Status** — current release (`docs/releases/<current>/`), link to status.md, honest about "v0.x, interfaces may shift."
   - **Contributing** — one paragraph + link to `CONTRIBUTING.md` and `GOVERNANCE.md`. Call out DCO sign-off, contributor tiers, and the ADR-driven workflow so forkers know the bar before they start.
   - **Forking encouragement** — explicit "fork, experiment, send PRs" paragraph. Mention Apache-2.0 license (ADR-0010) so there's no ambiguity about reuse.
   - **License** — one line, link to `LICENSE`.
2. Cross-link from `docs/usage-guide.md` back to the README at the top (so users who land there first know a project overview exists).
3. Add the README to `docs/releases/v0.2.0/changelog.md` under a docs entry (if the release is still open; otherwise note in status.md).

## Out of Scope

- Badges (CI status, license, Go report card, etc.). Nice to add later; not load-bearing for the v1 README. Can be a follow-up patch.
- Screenshots or GIFs of the dashboard. Adds maintenance burden; the dashboard is already documented in `docs/usage-guide.md`. Revisit if/when there's a marketing push.
- Moving content *out* of `docs/usage-guide.md` into the README. The README links to it; it does not replace it.
- Reworking `CONTRIBUTING.md` or `GOVERNANCE.md`. Link only.
- Promotional copy beyond what's already true. Do not describe FEAT-0008–0013 capabilities as shipped; frame them as "direction."
- A "Features" matrix / roadmap table. That belongs in `docs/releases/` and `docs/features/`; duplicating it in the README guarantees drift.

## Checklist

- [x] `README.md` at repo root with all sections listed in Scope
- [x] Opens with a tagline + 1-paragraph pitch that holds up for both today's capture proxy and the harness direction
- [x] Quick start block works verbatim from a clean clone on macOS (and on Linux, by inspection)
- [x] Links resolve: `docs/usage-guide.md`, `CONTRIBUTING.md`, `GOVERNANCE.md`, `LICENSE`, current release dir
- [x] No claims about unshipped features as if shipped; FEAT-0008–0013 framed as direction (v0.2.0 in-development features are labeled as such)
- [x] `docs/usage-guide.md` has a one-line pointer back to the README at the top
- [x] `gofmt`, `go vet`, and `go test ./...` still clean (no code touched; docs-only change)

## Fix Detail

### Tone

Target reader is a developer who already builds with Claude / GPT / local models and is skimming the repo to decide "is this worth five minutes of my time." That means:

- Lead with a concrete capability, not a manifesto.
- Show the quick-start command block above the fold.
- Don't bury the license or the contribution path.

### Positioning

Two truths to hold simultaneously:

1. **Today:** modeltap is a local-first reverse proxy that captures Anthropic + OpenAI traffic, tracks tokens/cost/latency, stores everything in SQLite, and ships a dashboard and a service manager.
2. **Direction:** FEAT-0008–0013 reframe modeltap as an integrated AI environment — BFF, terminal harness, enterprise auth, knowledge, skills, agent teams.

The README should make both visible without conflating them. A "Today" section and a "Where this is going" section, clearly labeled, is the safe shape.

### Encouraging forks

The harness pivot makes this more relevant than for a typical tool: forkers might want modeltap as the substrate for their own integrated environment. Call this out. Apache-2.0 (ADR-0010) is permissive by design; the README should make that explicit rather than leaving it to a reader to check `LICENSE`.
