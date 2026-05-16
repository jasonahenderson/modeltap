---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0011: Contribution Model and Governance

## Context and Problem Statement

Modeltap is an open source project that depends on community contributions for multi-provider support (ADR-0006), embedding model integrations (ADR-0008), and ecosystem growth. The project needs a governance and contribution model that enables community participation while maintaining code quality and architectural coherence. The decision is how to structure contribution processes, decision-making authority, and the path from user to contributor to maintainer.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Contribution accessibility (5):** The barrier to contributing must be low. First-time contributors should be able to submit a provider adapter, bug fix, or documentation improvement without navigating complex governance. High barriers kill community growth.
* **D2 – Code quality and architectural coherence (5):** Modeltap's architecture is defined by a set of ADRs. Contributions must align with these decisions. A governance model that accepts every PR without review undermines architectural integrity. One that rejects too many discourages contribution.
* **D3 – Decision-making clarity (4):** It must be clear who makes decisions, how decisions are made, and how to propose changes. Ambiguity leads to frustration, especially when a contribution is rejected or an ADR is challenged.
* **D4 – Scalability of governance (3):** The governance model should work for 1 maintainer and 5 contributors today, and still work for 5 maintainers and 50 contributors in the future. Over-engineering governance for scale the project does not yet have is a waste; under-engineering it makes growth painful.
* **D5 – Maintainer sustainability (3):** The model should not burn out the initial maintainer(s). Review load, decision fatigue, and community management overhead must be manageable for a small team.
* **D6 – Transparency and trust (3):** Contributors must trust that the project is governed fairly. Decisions should be documented, disagreements should have a resolution path, and the rules should apply equally to everyone.
* **D7 – IP and licensing clarity (2):** Contributions must have clear licensing. The project needs to know it can distribute contributed code under Apache 2.0 (ADR-0010) without ambiguity.

## Considered Options

* Benevolent Dictator for Life (BDFL) with lightweight contribution guide
* Apache-style meritocratic governance (PMC, committers, contributors)
* Consensus-based open governance (all contributors vote)
* BDFL with graduated contributor tiers

## Decision Outcome

Chosen option: **BDFL with graduated contributor tiers**, because it achieves the highest weighted score (115) and provides the right balance between clear decision-making authority and a defined growth path for contributors. The BDFL model gives the project founder clear authority to maintain architectural coherence, while graduated tiers (contributor → committer → maintainer) provide a transparent path for community members to earn increasing trust and responsibility.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | BDFL + guide | Apache-style | Consensus | BDFL + tiers |
|-------------------------------------|--------|-------------|-------------|-----------|-------------|
| D1: Contribution accessibility      | 5      | 5           | 3           | 4         | 5           |
| D2: Code quality / coherence        | 5      | 4           | 5           | 3         | 5           |
| D3: Decision-making clarity         | 4      | 5           | 4           | 2         | 5           |
| D4: Scalability                     | 3      | 2           | 5           | 4         | 4           |
| D5: Maintainer sustainability       | 3      | 3           | 4           | 2         | 4           |
| D6: Transparency / trust            | 3      | 2           | 5           | 5         | 4           |
| D7: IP / licensing clarity          | 2      | 3           | 5           | 3         | 4           |
| **Weighted Total**                  |        | **99**      | **109**     | **86**    | **115**     |

### Scoring Justification

#### BDFL with lightweight contribution guide (99)

* **D1 (5):** Minimal process — fork, branch, PR, review. A CONTRIBUTING.md file explains the basics. Lowest barrier to first contribution.
* **D2 (4):** The BDFL reviews all PRs and enforces architectural alignment. Works well at small scale but relies entirely on one person's review bandwidth.
* **D3 (5):** Crystal clear — one person makes all decisions. No ambiguity about authority.
* **D4 (2):** Does not scale. When the BDFL cannot review every PR, there is no delegation model. Either PRs queue up or quality drops.
* **D5 (3):** The BDFL bears all review and decision burden. No mechanism to share load. Sustainable only at low contribution volume.
* **D6 (2):** Decisions are made by one person with no formal process. Contributors may feel their input is ignored or that decisions are arbitrary.
* **D7 (3):** No formal CLA or DCO. Relies on the implicit license grant from PRs to a project with a LICENSE file. Works in practice but is legally weaker.

#### Apache-style meritocratic governance (109)

* **D1 (3):** Apache governance has significant process overhead — mailing lists, formal voting, proposal templates. Well-documented but intimidating for first-time contributors. The process is designed for large projects and can feel heavy for small ones.
* **D2 (5):** The PMC (Project Management Committee) collectively reviews and enforces architectural decisions. Multiple experienced reviewers catch issues that a single reviewer might miss.
* **D3 (4):** Well-defined roles (PMC, committer, contributor) with documented authority. Voting procedures are specified. Slightly complex but very clear once understood.
* **D4 (5):** Designed to scale to hundreds of contributors. The Apache model has been validated by projects from 5 to 500+ contributors.
* **D5 (4):** Review load is distributed across committers and PMC members. Sustainable at scale.
* **D6 (5):** Maximum transparency — all decisions are on public mailing lists, all votes are recorded, all processes are documented.
* **D7 (5):** Apache ICLA (Individual Contributor License Agreement) provides the strongest IP clarity. Every contributor signs a legal agreement granting license to their contributions.

#### Consensus-based open governance (86)

* **D1 (4):** Low barrier to contributing code, but high barrier to decision-making. Every significant decision requires building consensus, which can be slow and frustrating for new contributors who want to move fast.
* **D2 (3):** Consensus governance risks design-by-committee. Architectural coherence suffers when decisions require agreement from all parties. Bold, opinionated design choices (like the ADR-driven architecture) are harder to make and maintain.
* **D3 (2):** Who decides when consensus is reached? How are ties broken? Consensus models often have ambiguous authority, leading to protracted debates and decision paralysis.
* **D4 (4):** Scales reasonably with contributor growth, but decision-making slows as the group grows. Works better for small, aligned groups.
* **D5 (2):** Consensus processes are time-consuming. The maintainer must facilitate discussions, build consensus, and manage disagreements. High overhead.
* **D6 (5):** Maximum inclusivity — every voice is heard. Decisions are collective and transparent.
* **D7 (3):** Same as lightweight BDFL — no formal CLA process.

#### BDFL with graduated contributor tiers (115)

* **D1 (5):** Same low barrier as lightweight BDFL — fork, branch, PR, review. The tier system is about growth, not gatekeeping. First-time contributors are welcome without any process overhead.
* **D2 (5):** The BDFL and promoted maintainers enforce architectural coherence through code review and ADR adherence. Committers who have demonstrated understanding of the architecture can review and approve PRs in their area, increasing review capacity without sacrificing quality.
* **D3 (5):** The BDFL has final authority on all decisions. This is explicit and documented. The tier system adds delegation, not ambiguity — committers can approve PRs, but the BDFL can override. ADRs document the architectural decisions that guide all contributions.
* **D4 (4):** The tier system provides a natural scaling mechanism. As the project grows, contributors are promoted to committers (can review and merge in their area) and maintainers (can make broader decisions). The BDFL role can eventually evolve into a steering committee if needed.
* **D5 (4):** Review load is shared with committers and maintainers as they are promoted. The BDFL delegates review authority for specific areas (e.g., a committer who implemented the OpenAI adapter reviews OpenAI-related PRs).
* **D6 (4):** Decisions are documented in ADRs. The tier system and promotion criteria are public. Less formal than Apache governance but more transparent than pure BDFL.
* **D7 (4):** A Developer Certificate of Origin (DCO) sign-off on commits provides IP clarity without the overhead of a formal CLA. The DCO is a lightweight legal mechanism used by the Linux kernel, Kubernetes, and many other projects.

### Consequences

* Good, because the BDFL model provides clear, fast decision-making that maintains architectural coherence through the project's formative period.
* Good, because graduated tiers give contributors a visible, documented path to increasing responsibility and trust.
* Good, because the contribution process is minimal for first-time contributors — no CLA signing, no mailing list subscription, just a PR with a DCO sign-off.
* Good, because the ADR-driven architecture (ADR-0001 through ADR-0010) provides an objective framework for evaluating contributions, reducing subjective gatekeeping.
* Neutral, because the DCO is less legally robust than a formal CLA, but sufficient for a project of this size and risk profile.
* Bad, because the BDFL model concentrates authority in one person. If the BDFL becomes unavailable, there is no formal succession plan. This should be documented as the project matures.
* Bad, because contributor tier promotion criteria can feel subjective. Clear, documented criteria (e.g., "5 merged PRs with no architectural issues" for committer promotion) mitigate this but do not eliminate it entirely.

### Confirmation

The decision will be confirmed by:

1. Publishing a CONTRIBUTING.md that describes the contribution process, tier system, and promotion criteria.
2. Publishing a GOVERNANCE.md that documents the BDFL model, decision-making authority, and ADR process for architectural changes.
3. Confirming that a new contributor can submit and merge a provider adapter PR following only the CONTRIBUTING.md instructions.
4. Promoting the first community committer when a contributor demonstrates consistent, high-quality contributions.

## More Information

The decision aligns with the weighted scoring matrix. BDFL with graduated tiers leads by 6 points over Apache-style governance (115 vs 109). The decisive advantage is contribution accessibility (D1) — the Apache model's process overhead is not justified at modeltap's current scale, while the graduated tier system provides a growth path toward Apache-like structure if the project reaches that scale.

### Contributor Tiers

| Tier | Role | Authority | Path to Tier |
|------|------|-----------|-------------|
| 1 | **Contributor** | Submit PRs, open issues, participate in discussions | Anyone who submits a PR |
| 2 | **Committer** | Review and approve PRs in their area, triage issues | Consistent quality contributions (guideline: 5+ merged PRs) |
| 3 | **Maintainer** | Merge to main, propose ADRs, release authority | Demonstrated architectural understanding and sustained commitment |
| 4 | **BDFL** | Final authority on all decisions, ADR approval, vision | Project founder (Jason Henderson) |

### Contribution Process

```
1. Fork the repository
2. Create a feature branch
3. Make changes following the project's ADRs and coding standards
4. Sign off commits with DCO (git commit -s)
5. Submit a PR with:
   - Description of what and why
   - Reference to relevant ADR if applicable
   - Tests for new functionality
6. Address review feedback
7. Maintainer merges when approved
```

### DCO Sign-Off

Every commit must include:

```
Signed-off-by: Name <email@example.com>
```

This certifies that the contributor has the right to submit the work under the project's license (Apache 2.0). The DCO is enforced by a CI check on all PRs.

### When to Evolve Governance

The BDFL model should be re-evaluated when:

- The project has 3+ active maintainers
- The BDFL cannot review all PRs within 48 hours
- Multiple contributors request a more formal governance structure
- The project is accepted into a foundation (CNCF, Apache, etc.)

At that point, the natural evolution is toward a steering committee model, preserving the tier system while distributing BDFL authority across multiple maintainers.
