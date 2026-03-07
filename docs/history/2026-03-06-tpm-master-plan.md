# TPM: Master Plan Creation

**Date:** 2026-03-06
**Agent:** TPM (Technical Program Manager)
**Work Unit:** Initial planning

## What Was Planned

Read all accepted ADRs and features, understand the agent team workflow, and produce a master plan that breaks v1 scope into ordered, independently completable work units.

## What Was Done

1. Read all 11 accepted ADRs (0001-0011) covering:
   - Go language selection (ADR-0001)
   - SQLite storage with JSONL export (ADR-0002)
   - Cobra CLI framework (ADR-0003)
   - Viper configuration management (ADR-0004)
   - Full capture with retention pruning (ADR-0005)
   - Provider adapter interface with Anthropic + OpenAI (ADR-0006)
   - Pre-computed metrics aggregation tables (ADR-0007)
   - Knowledge layer with sqlite-vec (ADR-0008) -- v2
   - MCP server for knowledge access (ADR-0009) -- v2
   - Apache 2.0 license (ADR-0010)
   - BDFL governance with contributor tiers (ADR-0011)

2. Read all 3 accepted features:
   - Web dashboard (v1)
   - Knowledge layer (v2)
   - Multi-user support (v2+)

3. Read `docs/agents.md` to understand the 8-agent team and workflow pipeline (designer -> tester -> backend/ui -> integration -> security -> docs -> infra).

4. Created `docs/history/plan.md` with:
   - 32 work units across 10 phases
   - Phase 1: Project Foundation (WU-001 to WU-004)
   - Phase 2: CLI and Configuration (WU-005 to WU-006)
   - Phase 3: Storage Layer (WU-007 to WU-009)
   - Phase 4: Provider Adapters (WU-010 to WU-012)
   - Phase 5: Proxy Core (WU-013 to WU-016)
   - Phase 6: Usage Metrics (WU-017 to WU-019)
   - Phase 7: CLI Query Commands (WU-020 to WU-022)
   - Phase 8: Integration Testing and Security Review (WU-023 to WU-024)
   - Phase 9: Web Dashboard (WU-025 to WU-030)
   - Phase 10: Documentation and Polish (WU-031 to WU-032)
   - v2 future work listed (knowledge layer, multi-user) without detailed work units
   - Full dependency graph

5. Updated `docs/history/status.md` to reflect planning completion and list first work units as "Up Next".

## Key Decisions Made

- **WU-001 and WU-004 have no mutual dependency**, so Go module init and open source files can be done in parallel by different agents (infra and docs).
- **Dashboard is Phase 9**, after all backend APIs exist, as specified in requirements.
- **Knowledge layer and multi-user are v2** -- listed as future work without detailed work units.
- **Security review happens twice**: once after core backend (WU-024) and once after dashboard (WU-030), since the dashboard introduces browser-facing attack surface (XSS, CSRF).
- **Integration tests (WU-023) are a gate** before security review, ensuring the system works end-to-end before security is assessed.
- **Cost estimation (WU-019) is a separate work unit** from metrics aggregation (WU-017) because pricing table design and accuracy are distinct from the aggregation mechanism.

## Issues Encountered

None. All ADRs and features are well-defined and consistent with each other.

## Files Created or Modified

- Created: `docs/history/plan.md`
- Modified: `docs/history/status.md`
- Created: `docs/history/2026-03-06-tpm-master-plan.md` (this file)
