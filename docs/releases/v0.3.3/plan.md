# Implementation Plan: v0.3.3 — Policy-Grade Tool Runtime and Workspaces

## Context

`v0.3.3` raises tool execution from first-use prompts to policy-grade runtime
behavior. This release focuses on foreground policy first, then background-aware
policy integration with the run queue from v0.3.0.

## Scope

This release covers:

- policy/workspace ADR
- policy schema for tools, paths, commands, Git, network domains, workflows,
  foreground/background state, and workspace modes
- canonical workspace modes: `current`, `current_readonly`, `worktree`,
  `temp_copy`, `remote`
- foreground-policy enforcement and audit trail
- background policy behavior for paused/auto-denied blocked operations
- permission explanations and `/policy` inspection
- tool decision artifacts grouped by run

This release does not cover:

- full enterprise RBAC
- hook/plugin packaging
- durable memory promotion
- remote/cloud executor implementation beyond policy shape

## Feature Scope

- FEAT-0021: Policy-Grade Tool Runtime
- FEAT-0017: background policy integration slice

## Approach

Current phase: **Planning draft — Phase 1 not opened.**

## Work Units

| WU | Title | Dependencies | Size | Feature |
|---|---|---|---|---|
| 138 | Policy and workspace boundary ADR | v0.3.0 | M | FEAT-0021 |
| 139 | Policy schema and inheritance model | 138 | L | FEAT-0021 |
| 140 | Workspace mode resolver and run metadata integration | 138, 139 | M | FEAT-0021 |
| 141 | Foreground tool policy enforcement | 139 | L | FEAT-0021 |
| 142 | Command/path/Git/domain classifiers | 139, 141 | L | FEAT-0021 |
| 143 | Permission explanation and `/policy` harness surface | 141, 142 | M | FEAT-0021 |
| 144 | Background blocked-operation behavior | 141, 143 | M | FEAT-0017/0021 |
| 145 | Tool audit artifacts by run | 141-144 | M | FEAT-0021 |
| 146 | Policy runtime tests and docs | 138-145 | M | FEAT-0021 |

## Detailed WU Plan

### Track A — Policy Decisions

**WU-138: Policy and workspace boundary ADR**

Decide policy inheritance, non-overridable policy sources, workspace mode
semantics, and local vs BFF enforcement boundaries.

**WU-139: Policy schema and inheritance model**

Define config shapes and merge behavior for user, project, team/server, and
workflow policy layers.

### Track B — Enforcement

**WU-140: Workspace mode resolver and run metadata integration**

Resolve `current`, `current_readonly`, `worktree`, `temp_copy`, and `remote` for
a run. Store workspace metadata on the run.

**WU-141: Foreground tool policy enforcement**

Apply policy to local tool calls for attached foreground runs. Preserve existing
simple permission levels as presets.

**WU-142: Command/path/Git/domain classifiers**

Add dynamic classifiers for shell commands, Git mutations, file paths, and
network domains.

### Track C — Harness and Artifacts

**WU-143: Permission explanation and `/policy` harness surface**

Show why a tool request was allowed, denied, paused, or blocked. Add `/policy`
inspection for the active run.

**WU-144: Background blocked-operation behavior**

For detached runs, pause, auto-deny, or fail operations according to policy.
Surface blocked requests in the run queue/inbox.

**WU-145: Tool audit artifacts by run**

Record tool decisions, policy source, approver, input summary, workspace, and
result reference as run artifacts.

### Track D — Verification

**WU-146: Policy runtime tests and docs**

Add policy matrix tests, classifier tests, background blocked-operation tests,
and user/developer docs.

## Phase 1 Design Checklist

- [ ] WU-138 policy/workspace ADR draft
- [ ] WU-139 policy schema design
- [ ] WU-140 to WU-142 enforcement design bundle
- [ ] WU-143 to WU-145 harness/artifact design bundle
- [ ] WU-146 verification/docs design

## Risk Register

- **R1 — rules-engine creep.** Keep policy expressive enough for v1 without
  becoming a general-purpose policy language.
- **R2 — unsafe defaults.** Background writes must not silently proceed by
  default.
- **R3 — false blocks.** Classifiers need clear override/approval behavior for
  solo workflows.

## Definition of Done

1. Policy/workspace ADR is accepted.
2. Foreground tool calls are evaluated against structured policy.
3. Background blocked operations pause or deny according to policy.
4. `/policy` explains active policy and recent decisions.
5. Tool decisions are recorded as run artifacts.
6. Tests cover policy inheritance, classifiers, workspace modes, and
   background behavior.
