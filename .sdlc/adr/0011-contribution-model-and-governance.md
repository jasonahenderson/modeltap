---
status: superseded
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0011: Contribution Model and Governance

## Status

**Superseded** — this ADR defined a BDFL governance model with graduated contributor tiers. The governance structure is premature for a project with no code and no contributors. Contribution guidelines have been moved to `CONTRIBUTING.md` in the repository root.

## Decision

Governance is documented in `CONTRIBUTING.md`, not an ADR. The contribution process is:

1. Fork, branch, PR
2. DCO sign-off required (`git commit -s`)
3. PRs reviewed by maintainer
4. Follow existing patterns and ADRs

Formal governance (tiers, promotion criteria, steering committee) will be introduced when the project has active contributors and the current lightweight process is insufficient. See the "When to formalize" section in `CONTRIBUTING.md`.
