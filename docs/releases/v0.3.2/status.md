# v0.3.2 — Status

**Current phase:** Planning draft — Phase 1 not opened  
**Branch:** TBD  
**Scope:** FEAT-0019, FEAT-0020, and codegen evaluation patch

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 127 | Validation artifact and repair-loop ADR/design | M | planned | pending |
| 128 | Artifact storage, retention, and redaction ADR/design | M | planned | pending |
| 129 | Validation plan generator | L | planned | pending |
| 130 | Structured command/check evidence envelopes | M | planned | pending |
| 131 | Failure summarization and repair context injection | L | planned | pending |
| 132 | Repair-attempt memory and stop/ask behavior | M | planned | pending |
| 133 | Run artifact bundle store and API | L | planned | pending |
| 134 | Patch/diff evidence collector | M | planned | pending |
| 135 | Harness artifact inspection surfaces | M | planned | pending |
| 136 | Codegen evaluation harness patch | M | planned | pending |
| 137 | Validation/artifact integration tests and docs | M | planned | pending |

## Gates

- Depends on v0.3.0 run infrastructure.
- Benefits from v0.3.1 context planning but can run with minimal changed-file
  metadata if v0.3.1 is delayed.
- Phase 1 starts only after FEAT-0019/0020 are accepted or the release-open
  `ADMIN:` commit records an explicit design-against-draft exception.
- Phase 1 may include WU-136 only if the supporting PATCH is allocated; without
  that artifact, WU-136 must be removed or deferred before Phase 1 closes.
- Phase 3 is blocked until FEAT-0019/0020 and any in-scope WU-136 PATCH are
  accepted.

## Open Items

- Draft and accept the supporting PATCH for WU-136 before v0.3.2 Phase 1 opens,
  or remove/defer WU-136 from the active release plan before Phase 1 closes.
- Confirm v0.3.0 has introduced `workflow_type` before Phase 3 so WU-129 can
  use workflow-aware validation defaults.
