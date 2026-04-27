# v0.2.2 — Status

**Current phase:** Phase 3 — Implementation (no Phase 1 design gate
for this release; WU-099's Runtime contract is the authoritative
design)
**Branch:** `spike/scrolling-surface-eval` inherited from v0.2.1;
should retarget to a release branch before tagging
**Phase 1 closed:** N/A (no design phase for this release)
**Phase 2 closed:** N/A

## Phase 3 work units

| WU | Title | Size | State | Notes |
|----|-------|------|-------|-------|
| 103 | internal/harness audit and salvage report | M | Up next | Deliverable is the audit doc |
| 104 | Concrete `harnesshost.Runtime` implementation | L | Pending | Blocked on 103 |
| 105 | Production conversation-shell CLI entrypoint | M | Pending | Blocked on 104 |
| 106 | Plumbing cleanup | M | Pending | Blocked on 104, 105 |
| 107 | WU-102 SC3 follow-up: viewport-state accessor | S | Pending | Independent |

## Phase 3 dependency graph

```
103 (audit) ──→ 104 (Runtime impl) ──→ 105 (CLI)
                                        │
                                        ▼
                                       106 (cleanup)

107 (SC3 follow-up) — independent
```

## Up next

- **WU-103 audit.** Walk every file in `internal/harness/`, including
  subpackages (`theme`, `styles`, `tools`). Categorize each as
  keep / refactor / delete with rationale and the
  `harnesshost.Runtime` method it serves (or doesn't). Surface any
  dependencies on App-level tea.Msg lifecycle that need a refactor
  before the file is reusable.
- After 103, scope 104 (Runtime impl) based on what's actually
  salvageable.

## In progress

(none — WU-103 is queued)

## Done this phase

(none — release just opened)

## Open items

- **Branch retarget.** Inherited from v0.2.1; pending TPM decision.
- **Production CLI command name.** Provisional candidates:
  `modeltap shell`, `modeltap chat`, `modeltap session`. Decided
  during WU-105.
- **MCP scope.** v0.2.0 added `internal/harness/mcp*.go`. The audit
  decides whether MCP integration ships in v0.2.2's Runtime impl or
  whether it's deferred to a later release.
