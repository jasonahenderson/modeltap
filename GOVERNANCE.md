# modeltap Governance

This document describes how modeltap is governed, who makes decisions, and how contributors can grow their role in the project. The governance model is defined in [ADR-0011](docs/adr/0011-contribution-model-and-governance.md).

## Governance Model: BDFL with Graduated Contributor Tiers

modeltap uses a Benevolent Dictator for Life (BDFL) governance model with graduated contributor tiers. This model provides clear decision-making authority while offering a transparent path for community members to earn increasing responsibility.

**BDFL:** Jason Henderson (project founder)

The BDFL has final decision authority on all matters, including architectural direction, releases, and governance changes. This authority is exercised transparently through Architecture Decision Records (ADRs) and public discussion.

## Contributor Tiers

### Tier 1: Contributor

**Who:** Anyone who submits a pull request or opens an issue.

**Authority:**
- Submit pull requests
- Open and comment on issues
- Participate in discussions
- Propose changes

**How to become a Contributor:** Submit a PR. There is no application process or approval needed.

### Tier 2: Committer

**Who:** Contributors who have demonstrated consistent, high-quality contributions.

**Authority:**
- Everything a Contributor can do
- Review and approve pull requests in their area of expertise
- Triage issues

**How to become a Committer:** A contributor is promoted to Committer after merging 5 or more quality pull requests that demonstrate understanding of the project's architecture and coding standards. Promotion is proposed by a Maintainer or the BDFL and announced publicly.

### Tier 3: Maintainer

**Who:** Committers who have demonstrated sustained commitment and deep architectural understanding.

**Authority:**
- Everything a Committer can do
- Merge pull requests to `main`
- Propose Architecture Decision Records (ADRs)
- Release authority (create and publish releases)

**How to become a Maintainer:** Promotion requires sustained contribution over time, demonstrated understanding of the project's ADR-driven architecture, and a track record of high-quality reviews. The BDFL approves Maintainer promotions.

### Tier 4: BDFL

**Who:** Jason Henderson (project founder).

**Authority:**
- Final authority on all decisions
- ADR approval
- Project vision and direction
- Governance changes
- Maintainer promotion

## Decision-Making Process

### Architectural Decisions

Architectural decisions are made through Architecture Decision Records (ADRs), stored in `docs/adr/`. ADRs follow a structured format that includes context, decision drivers, considered options, a scoring matrix, and consequences.

- **Contributors and Committers** can propose ADRs by opening a PR with a draft ADR.
- **Maintainers** can propose and champion ADRs through discussion and review.
- **The BDFL** approves or rejects ADRs. Approval is based on the ADR's analysis quality and alignment with project goals, not on authority alone.

### Day-to-Day Decisions

- **Code changes** are decided through the pull request review process (see [CONTRIBUTING.md](CONTRIBUTING.md)).
- **Issue prioritization** is handled by Maintainers and the BDFL.
- **Release timing** is decided by Maintainers with BDFL approval for major releases.

### Disagreements

If a contributor disagrees with a review decision or an ADR:

1. Discuss in the relevant PR or issue.
2. If unresolved, open a new issue describing the disagreement and proposed alternative.
3. The BDFL makes the final decision and documents the rationale.

## When Governance Evolves

The BDFL model is appropriate for modeltap's current stage. It should be re-evaluated when any of the following conditions are met:

1. **3+ active Maintainers:** The project has enough trusted maintainers to distribute decision-making authority.
2. **BDFL review bottleneck:** The BDFL cannot review all PRs within 48 hours on a sustained basis.
3. **Community request:** Multiple contributors request a more formal governance structure.
4. **Foundation acceptance:** The project is accepted into a foundation (e.g., CNCF, Apache Software Foundation).

When re-evaluation is triggered, the natural evolution is toward a steering committee model that preserves the tier system while distributing authority across multiple maintainers. This transition will be documented in a new ADR.

## Amendments

Changes to this governance document require a new ADR and BDFL approval. The BDFL commits to not changing governance unilaterally without public discussion.
