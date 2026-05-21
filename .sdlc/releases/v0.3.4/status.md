# v0.3.4 — Status

**Current phase:** Planning draft — Phase 1 not opened  
**Branch:** TBD  
**Scope:** FEAT-0022 memory, routing, and workflow-extension alignment

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 147 | Memory, routing, and extension trust ADR | M | planned | pending |
| 148 | Memory candidate schema and source-artifact links | M | planned | pending |
| 149 | Candidate generation and disposition UI | L | planned | pending |
| 150 | Active memory provenance in run details | M | planned | pending |
| 151 | Routing role taxonomy and policy config | M | planned | pending |
| 152 | Routing decision/outcome capture | M | planned | pending |
| 153 | Workflow profile and extension alignment design | L | planned | pending |
| 154 | Memory/routing/workflow tests and docs | M | planned | pending |

## Gates

- Depends on v0.3.2 run artifacts.
- Workflow-extension acceptance is gated on FEAT-0011/0012/0013 coordination.
- Phase 1 starts only after FEAT-0022 is accepted or the release-open `ADMIN:`
  commit records an explicit design-against-draft exception.
- If FEAT-0011/0012/0013 coordination is not ready, Phase 1 must split out or
  defer WU-153 and rescope WU-154 before design closes.
- Phase 3 is blocked until FEAT-0022 and any in-scope workflow-extension
  dependency revisions are accepted.

## Open Items

- Decide during Phase 1 whether to split memory/routing from workflow-extension
  alignment.
- If WU-153 is deferred, name the future release or approved PATCH that owns it
  and rescope WU-154 to memory/routing tests/docs.
- Confirm v0.3.0 has introduced `workflow_type` before Phase 1 opens.
- Track FEAT-0011, FEAT-0012, and FEAT-0013 acceptance or revision status
  before WU-153 design closes.
