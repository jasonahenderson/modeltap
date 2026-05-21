# WU-004: Open Source Files

**Date:** 2026-03-06
**Work Unit:** WU-004
**Agent:** Documentation Specialist

## Summary

Created the three open source governance files at the repository root per ADR-0010 and ADR-0011.

## Files Created

1. **LICENSE** - Standard Apache License 2.0 full text with copyright line: "Copyright 2026 Jason Henderson"
2. **CONTRIBUTING.md** - Contribution guide covering fork/branch/PR workflow, DCO sign-off requirement, Go coding standards (gofmt, go vet, table-driven tests), PR requirements, code review process, and contributor tiers
3. **GOVERNANCE.md** - Governance document describing the BDFL model with graduated contributor tiers (Contributor, Committer, Maintainer, BDFL), decision-making via ADRs, disagreement resolution, and conditions for governance evolution

## Decisions Made

- Used the full Apache 2.0 license text (not an abbreviated version) as is standard practice.
- CONTRIBUTING.md includes practical instructions for DCO sign-off including how to fix forgotten sign-offs (amend and rebase).
- GOVERNANCE.md includes an Amendments section clarifying that governance changes require a new ADR and public discussion, reinforcing the transparency commitment from ADR-0011.
- Contributor tier promotion criteria are documented with specific thresholds (e.g., 5+ merged PRs for Committer) as stated in ADR-0011.

## ADR References

- ADR-0010: Open Source License (Apache 2.0)
- ADR-0011: Contribution Model and Governance (BDFL with graduated tiers)
