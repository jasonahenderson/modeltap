# v0.3.3 — Status

**Current phase:** Planning draft — Phase 1 not opened  
**Branch:** TBD  
**Scope:** FEAT-0021 policy-grade tool runtime

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 138 | Policy and workspace boundary ADR | M | planned | pending |
| 139 | Policy schema and inheritance model | L | planned | pending |
| 140 | Workspace mode resolver and run metadata integration | M | planned | pending |
| 141 | Foreground tool policy enforcement | L | planned | pending |
| 142 | Command/path/Git/domain classifiers | L | planned | pending |
| 143 | Permission explanation and `/policy` harness surface | M | planned | pending |
| 144 | Background blocked-operation behavior | M | planned | pending |
| 145 | Tool audit artifacts by run | M | planned | pending |
| 146 | Policy runtime tests and docs | M | planned | pending |

## Gates

- Depends on v0.3.0 run infrastructure.
- Background-specific behavior depends on v0.3.0 attach/detach and queue
  semantics.

## Open Items

- During WU-138/WU-140 design, decide whether `worktree` or `temp_copy`
  workspace creation lands in this release. Planning default is metadata and
  policy shape only.
- Confirm v0.3.0 has introduced `workflow_type` before Phase 3 so policy can
  evaluate workflow-aware defaults.
