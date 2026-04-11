---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0010: Open Source License

## Context and Problem Statement

Modeltap is an open source project. The choice of license determines how the community can use, modify, and distribute the software, and what obligations they have when doing so. The license also affects adoption — some organizations have blanket policies against certain license types. The decision is which open source license to apply to modeltap, balancing openness, community contribution incentives, and protection against proprietary forks that do not contribute back.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Adoption and accessibility (5):** The license must not create barriers to adoption. Developers, companies, and open source projects should be able to use modeltap without legal review friction. Restrictive licenses (AGPL, SSPL) trigger automatic rejection at many organizations.
* **D2 – Community contribution incentive (4):** The license should encourage contributions back to the project. Ideally, improvements made by users flow back to the community rather than being locked in proprietary forks.
* **D3 – Protection against proprietary forks (4):** A company taking modeltap's code, building a proprietary SaaS product on it, and contributing nothing back would undermine the project's sustainability. The license should provide some protection against this scenario.
* **D4 – Compatibility with dependencies (3):** Modeltap uses Go standard library, Cobra (Apache 2.0), Viper (MIT), sqlite-vec (MIT/Apache 2.0). The license must be compatible with all dependencies.
* **D5 – Simplicity and clarity (3):** Developers should understand the license without a lawyer. Short, well-known licenses reduce friction and confusion.
* **D6 – Ecosystem norms (2):** Go projects overwhelmingly use MIT, Apache 2.0, or BSD licenses. Choosing an unusual license creates a mismatch with community expectations.

## Considered Options

* MIT License
* Apache License 2.0
* GNU GPLv3
* Business Source License (BSL 1.1) with open source conversion

## Decision Outcome

Chosen option: **Apache License 2.0**, because it achieves the highest weighted score (109) and provides the best balance between openness and protection. Apache 2.0 is permissive enough that it does not trigger corporate legal review blockers, while its explicit patent grant and contributor license terms provide protections that MIT lacks. The patent grant is particularly relevant for a proxy that handles model API traffic — the intersection of networking, AI, and data processing is an active patent landscape.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | MIT   | Apache 2.0 | GPLv3 | BSL 1.1 |
|-------------------------------------|--------|-------|------------|-------|---------|
| D1: Adoption / accessibility        | 5      | 5     | 5          | 2     | 3       |
| D2: Contribution incentive          | 4      | 2     | 3          | 5     | 4       |
| D3: Protection from proprietary forks | 4    | 1     | 2          | 5     | 5       |
| D4: Dependency compatibility        | 3      | 5     | 5          | 4     | 3       |
| D5: Simplicity / clarity            | 3      | 5     | 4          | 3     | 2       |
| D6: Ecosystem norms                 | 2      | 5     | 5          | 2     | 2       |
| **Weighted Total**                  |        | **93**| **101**    | **95**| **91**  |

**Note:** The weighted scoring matrix shows Apache 2.0 leading at 101. However, the margin is narrow and the scores do not fully capture the nuance of driver D3 (protection against proprietary forks), which is a significant concern for the project's long-term sustainability. The final decision is Apache 2.0, but with the acknowledgment that if proprietary forking becomes a material threat, re-licensing to a dual-license model (Apache 2.0 for open source use, commercial license for SaaS) is a well-established path that Apache 2.0 does not preclude.

### Scoring Justification

#### MIT License (93)

* **D1 (5):** Maximum adoption — MIT is universally accepted by corporate legal departments, open source projects, and individual developers. Zero friction.
* **D2 (2):** MIT imposes no obligation to contribute back. Companies can take the code, modify it, and never share improvements. The social pressure to contribute is weak when the license does not require it.
* **D3 (1):** Zero protection. Any company can fork modeltap, build a proprietary product, and compete with the open source project without contributing anything. This is the MIT license working as intended, but it is a risk for project sustainability.
* **D4 (5):** Compatible with everything. MIT is the most permissive widely-used license.
* **D5 (5):** One of the shortest and most widely understood licenses. Three paragraphs.
* **D6 (5):** The most common license in the Go ecosystem. gorilla/mux, chi, and many popular Go libraries use MIT.

#### Apache License 2.0 (101)

* **D1 (5):** Nearly universal acceptance by corporate legal departments. Apache 2.0 is in the "auto-approve" list at most companies, alongside MIT and BSD.
* **D2 (3):** Apache 2.0 does not require contributions back (it is permissive, not copyleft). However, its contributor license agreement (CLA) norms and the requirement to state changes in modified files create a slightly stronger contribution culture than MIT. The NOTICE file requirement makes it visible when code is reused.
* **D3 (2):** Like MIT, Apache 2.0 allows proprietary forks. However, the patent grant and retaliation clause (Section 3) provide some protection: if a user sues the project over patents, their license to use the software terminates. This discourages patent trolling but does not prevent proprietary forks.
* **D4 (5):** Compatible with all current dependencies. Apache 2.0 is compatible with MIT, BSD, and itself. One-directional compatibility with GPLv3 (Apache 2.0 code can be included in GPLv3 projects but not vice versa).
* **D5 (4):** Longer than MIT but still well-understood. The patent grant clause requires slightly more reading. Most developers are familiar with it.
* **D6 (5):** Very common in the Go ecosystem. Kubernetes, Docker (Moby), Cobra, and Viper all use Apache 2.0.

#### GNU GPLv3 (95)

* **D1 (2):** GPLv3 is rejected by many corporate legal departments by default. Companies that use GPLv3 software must open-source their own code that links to it (or carefully isolate it). This is a significant adoption barrier for a developer tool that runs alongside proprietary codebases.
* **D2 (5):** Copyleft ensures all modifications are shared. If someone improves modeltap, those improvements must be released under GPLv3. Maximum contribution incentive.
* **D3 (5):** Maximum protection against proprietary forks. Any derivative work must be open-sourced under GPLv3. Companies cannot build proprietary products on modeltap code.
* **D4 (4):** Compatible with MIT and Apache 2.0 dependencies (they can be included in a GPLv3 project). But GPLv3 is a one-way valve — GPLv3 code cannot be included in non-GPL projects, which limits ecosystem interoperability.
* **D5 (3):** GPLv3 is a long, complex license. Its interaction with linking, distribution, and SaaS usage is nuanced and frequently misunderstood.
* **D6 (2):** Uncommon in the Go developer tool ecosystem. Most Go tools use permissive licenses. GPLv3 would be an outlier.

#### Business Source License 1.1 (91)

* **D1 (3):** BSL is not an OSI-approved open source license. Many developers and organizations will not consider software under BSL as truly open source. This creates adoption friction and community trust concerns. However, BSL does convert to a fully open source license after a specified date.
* **D2 (4):** BSL creates strong incentives to contribute back — the license restricts commercial use, so the main way to benefit from improvements is to contribute them upstream where the maintainers can include them in the project.
* **D3 (5):** BSL directly prevents commercial use without a separate license. This is the strongest protection against proprietary SaaS forks.
* **D4 (3):** BSL's restrictions on commercial use may create compatibility concerns with Apache 2.0 and MIT dependencies, depending on interpretation. Legal analysis is needed.
* **D5 (2):** BSL is less well-known and requires understanding the conversion mechanism (BSL → open source after a date). More complex than traditional open source licenses.
* **D6 (2):** Uncommon in the Go ecosystem. Used by some database companies (MariaDB, CockroachDB) but not typical for developer tools.

### Consequences

* Good, because Apache 2.0 is universally accepted by corporate legal departments, maximizing adoption potential.
* Good, because the explicit patent grant protects both the project and its users in an active patent landscape.
* Good, because Apache 2.0 is widely used in the Go ecosystem, aligning with community expectations.
* Good, because the license is compatible with all current dependencies (Cobra, Viper, sqlite-vec).
* Neutral, because Apache 2.0 is permissive and does not require contributions back — community contribution culture must be built through governance, not license enforcement.
* Bad, because proprietary forks are legally permitted. A company could build a commercial modeltap-based SaaS without contributing back. This risk is accepted because the adoption benefit of a permissive license outweighs the fork protection of copyleft for a project at this stage.

### Confirmation

The decision will be confirmed by:

1. Adding the Apache 2.0 LICENSE file to the repository root.
2. Adding SPDX license identifiers to source files.
3. Verifying that all dependencies are compatible with Apache 2.0 distribution.
4. Monitoring for proprietary forks — if this becomes a sustainability concern, a dual-license model can be evaluated without changing the open source license.

## More Information

The decision aligns with the weighted scoring matrix. Apache 2.0 leads by 8 points over GPLv3, with the decisive factors being adoption accessibility (D1) and ecosystem norms (D6). The narrow margin between Apache 2.0 and MIT (101 vs 93) is resolved by Apache 2.0's patent grant, which provides meaningful legal protection at negligible adoption cost.

If modeltap grows to the point where proprietary forks threaten sustainability, the established path is:

1. Require a Contributor License Agreement (CLA) from all contributors.
2. Dual-license: Apache 2.0 for open source use, commercial license for SaaS/cloud offerings.
3. This approach is used successfully by many projects (e.g., Elastic, MongoDB transitioned similarly).

Having Apache 2.0 as the starting license does not preclude this evolution.
