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

## Open Items

- Draft and accept the supporting PATCH for WU-136 before any WU-136
  implementation commit.
- Confirm v0.3.0 has introduced `workflow_type` before Phase 3 so WU-129 can
  use workflow-aware validation defaults.
