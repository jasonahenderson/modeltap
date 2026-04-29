---
feature: FEAT-0021
title: Policy-Grade Tool Runtime
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 6
depends-on:
  - FEAT-0009: Terminal Harness
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0017: Durable Runs and Background Agents
adr-constraints:
  - ADR-0014: Harness Base Strategy
promoted-from:
  - FEAT-0015: Professional Harness Runtime
---

# FEAT-0021: Policy-Grade Tool Runtime

## Problem

Modeltap has useful built-in tools and a permission enforcer, but professional
harness behavior requires more than first-use prompts. Tools need path, command,
domain, workspace, workflow, and background/foreground policy. Tool decisions
must be explainable, auditable, and durable.

Without policy-grade tools, background runs either become unsafe or overly
manual, and enterprise/team usage cannot set reliable guardrails.

## Solution

Extend the harness tool runtime with structured policy. The BFF supplies
applicable server/team/project constraints in run metadata and prompt planning.
The local harness/executor enforces local side-effect policy for filesystem,
shell, Git, network, and MCP tools. Every tool decision is recorded as run
evidence.

## Key Capabilities

### Policy Dimensions

Tool policy should be expressible across:

- tool name and namespace
- risk level
- workflow type
- foreground/background attachment state
- workspace mode
- path allow/deny rules
- shell command allow/deny rules
- Git mutation rules
- network/domain rules
- MCP server/tool provenance
- user, project, team, and server policy layers

### Permission Outcomes

A tool request may be:

- auto-allowed
- approved once
- approved for session/run
- approved for path/domain/tool scope
- denied
- paused pending user input
- blocked by non-overridable policy

The harness should explain why a decision happened.

### Workspace-Aware Execution

All local tools execute relative to the run workspace, not an implicit process
cwd. Policy can differ between:

- current workspace
- read-only current workspace
- Git worktree
- temp copy
- remote sandbox

### Audit Trail

Tool decisions are grouped by run and include:

- request ID
- tool name and input summary
- dynamic risk classification
- policy source
- decision
- approver when applicable
- timestamp
- execution result reference

### Hooks and Extensions

Policy should leave room for hooks that can warn, block, or enrich tool
execution. Hook packaging and skill integration may be specified downstream, but
the tool runtime should not make them impossible.

## UI / CLI / API Integration

Expected harness surfaces:

- permission composer with policy reason
- `/permissions` to list pending decisions
- `/policy` to inspect active policy
- run artifact view for approval history

The BFF/run protocol should include policy metadata for prompt planning and
artifact capture, but local execution authority remains harness-owned.

## Configuration

Configuration should support:

- per-tool policy
- per-workflow policy
- path allow/deny lists
- shell command allow/deny lists
- domain allow/deny lists
- background-run defaults
- policy inheritance and override rules

## Non-Goals

- Do not move local side-effect enforcement fully into the BFF.
- Do not silently approve background writes by default.
- Do not implement full enterprise RBAC here; this feature should integrate
  with FEAT-0010 when needed.
- Do not require hooks or plugins in the first slice.

## Success Criteria

1. Tool decisions consider workflow, workspace, and foreground/background state.
2. Path, command, Git, and domain policy can block or require approval.
3. Background runs pause or deny operations outside pre-approved policy.
4. Every tool decision is recorded as run evidence.
5. The user can inspect why a tool request was allowed, denied, or blocked.
6. Existing simple permission levels continue to work as presets over the richer
   policy model.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Preserves harness-side local enforcement with BFF orchestration |
| Future ADR | Should decide policy inheritance, sandbox/workspace boundaries, and non-overridable policy sources |

## Open Questions

1. What policy language is sufficient for v1 without becoming a general-purpose
   rules engine?
2. Which policy layer wins on conflict: user, project, team, or server?
3. Should dangerous command classification be parser-based, pattern-based, or
   both?
4. How should MCP tool provenance and trust be represented?
