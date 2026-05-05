# v0.3.0 — Status

**Current phase:** Planning draft — Phase 1 not opened  
**Branch:** TBD  
**Scope:** Run runtime foundation for FEAT-0016 and the first slice of
FEAT-0017

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 108 | Run runtime ADR | M | planned | pending |
| 109 | Run schema, storage, and migration design | M | planned | pending |
| 110 | Run protocol methods and event taxonomy | M | planned | pending |
| 111 | BFF run registry and lifecycle store | L | planned | pending |
| 112 | `turn.submit` to foreground-run integration | L | planned | pending |
| 113 | Pipeline stage/status emission and checkpoint metadata | M | planned | pending |
| 114 | Harness run projection and active `/run` surface | M | planned | pending |
| 115 | Run list, attach/detach/cancel/retry/continue/fork commands | L | planned | pending |
| 116 | Reconnect/resume behavior for active and detached runs | M | planned | pending |
| 117 | Runtime foundation tests and docs | M | planned | pending |

## Gates

- Phase 1 starts only after an explicit `ADMIN:` commit opens the release and
  either FEAT-0015, FEAT-0016, and the FEAT-0017 foundation slice are accepted
  or the commit records an explicit design-against-draft exception.
- The Phase 1 opening commit must reconcile the v0.2.x release-status mismatch
  or name the committed BFF/harness contracts that v0.3.0 design may depend on.
- Phase 1 closes only after every WU has a design artifact.
- Phase 2 closes only after design reviews are recorded and findings
  dispositioned.
- Phase 3 begins only after the explicit Phase 2 -> Phase 3 `ADMIN:` commit,
  accepted FEAT-0015/0016/0017 foundation scope, accepted run-runtime ADR, and
  reachable v0.2.x prerequisite surfaces.

## Open Items

- Draft and accept the run-runtime ADR.
- Decide the implementation branch.
- Confirm v0.2.0, v0.2.1, and v0.2.2 prerequisite surfaces are reachable from
  the v0.3.0 implementation branch before Phase 3.
- During WU-108/WU-109 design, lock the `workflow_type` enum and persistence
  shape used by v0.3.1 through v0.3.4.
- During WU-109 design, complete the cross-release schema compatibility check
  for context, artifact, policy, workspace, memory, and routing metadata.
