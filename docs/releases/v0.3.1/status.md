# v0.3.1 — Status

**Current phase:** Planning draft — Phase 1 not opened  
**Branch:** TBD  
**Scope:** FEAT-0018 context planner and project rules, plus PATCH-0017
session-scoped project context prerequisite

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 118 | Project rules and prompt-layer ADR | M | planned | pending |
| PATCH-0017 | Session-scoped project context | M | planned | pending |
| 119 | Context plan schema and protocol surface | M | planned | pending |
| 120 | Harness project-rule discovery and precedence reporting | M | planned | pending |
| 121 | Lightweight repo map and recent-change scanner | L | planned | pending |
| 122 | Test and style-context discovery | M | planned | pending |
| 123 | runtime server context planner and budget accounting | L | planned | pending |
| 124 | Prompt-plan metadata and context provenance capture | M | planned | pending |
| 125 | Harness `/context` inspection surfaces | M | planned | pending |
| 126 | Context planner verification and docs | M | planned | pending |

## Gates

- Depends on v0.3.0 run infrastructure.
- PATCH-0017 must land before context-plan protocol/schema implementation.
- Phase 1 starts only after explicit release-open `ADMIN:` commit.
- Phase 3 is blocked until FEAT-0018 is accepted and v0.3.0 Phase 3 has
  produced accepted run schema/protocol contracts.

## Open Items

- Decide whether deeper EXP-0012 AST graphing lands here or later. Current plan
  keeps it later.
- Confirm v0.3.0 has introduced `workflow_type` before Phase 3 if context
  planning uses workflow-aware defaults.
