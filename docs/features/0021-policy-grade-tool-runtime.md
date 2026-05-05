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
related:
  - FEAT-0017: Durable Runs and Background Agents
adr-constraints:
  - ADR-0014: Harness Base Strategy
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

Default layer precedence is `server > team > project > user`. Higher layers may
mark decisions non-overridable. Every decision records the winning layer.

The policy runtime is the single source of dynamic risk classification for tool
calls and validation checks. The classifier implementation may be parser-based,
pattern-based, or hybrid, but downstream features inherit its result.

Domain rules apply to outbound network tools, MCP server endpoints, and any tool
that loads remote URLs. Model-provider calls follow provider routing policy
rather than domain rules, but are recorded with provider identity.

Policy is compiled per run at `preflight` and reused. Risk classification is
cached by canonical input, and scope grants short-circuit re-evaluation for
matching calls. After warm-up, policy evaluation should stay within a 10ms
default latency budget. Tool-call evaluation is rate-limited per run and per
harness; over-limit calls pause the run with reason `tool_call_rate_exceeded`.

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

Approval scopes use conservative matching rules: paths use canonical
prefix-with-trailing-slash matching, domains use suffix matching including
wildcard subdomains, commands use exact canonical argv matching, and tools use
exact tool names.

### Workspace-Aware Execution

All local tools execute relative to the run workspace, not an implicit process
cwd. Policy can differ between:

- `current`
- `current_readonly`
- `worktree`
- `temp_copy`
- `remote`

These identifiers are defined by FEAT-0015's workspace policy and should remain
stable across run metadata, policy config, and artifact records.

### Audit Trail

Tool decisions are grouped by run and include:

- `tool_call_id` for the tool invocation being decided
- `decision_id` for the approval or policy decision
- tool name and input summary
- dynamic risk classification
- policy source
- decision outcome
- approver when applicable
- timestamp
- `result_id` reference to the execution record when a tool runs

Auto-allowed calls still produce a decision record. Scope grants produce a
`decision_id` that may be referenced by multiple subsequent `tool_call_id`s
within the grant scope.

Policy versions are monotonic integers scoped to the effective
server/project/run-creator policy context. The version increments on any policy
change that affects evaluation. Mid-run version changes block new tool decisions
until the harness re-acknowledges the effective version. Every decision artifact
records the policy version.

Permission decisions are made by the harness/local executor because it is the
sole authority for local side effects. The BFF supplies the policy context used
for the decision. After each decision, the harness reports the decision to the
BFF, including `tool_call_id`, `decision_id`, outcome, policy source, and reason;
the BFF records that report as durable run evidence. The BFF may reject a report
when it detects a policy mismatch, such as a stale local policy version or
server-policy override. On rejection, the BFF revokes the decision and the
harness must not deliver the tool result to the model. The harness must
acknowledge the BFF policy version before running tool calls in a run.

Mutating tools use a two-phase flow. Before execution, the harness requests a
BFF commitment for the current policy version and tool intent. The harness runs
the tool only if the commitment is fresh. Revocation applies only to uncommitted
tools or stale decisions; already-executed side effects are retained as audit
facts and may not be assumed reversible. Read-only tools may use a single-phase
decision flow.

Decision records are written to an append-only audit log indexed by
`decision_id`. Run artifacts reference those records, but the audit log is the
source of truth and outlives run mutation or normal run retention where policy
requires.

### Server-Safe Tools

The BFF may host a small set of server-safe tools that produce no local side
effects, such as BFF-side retrieval, citation or quote extraction over already
captured artifacts, and summarization. Server-safe tools follow the same audit
record discipline with `tool_call_id`, `decision_id`, and `result_id`, but do
not require a connected harness. The run-runtime ADR enumerates this tool
surface. Tools not on that authoritative list are harness-owned regardless of
BFF classification, and the harness rejects BFF-side execution for non-listed
tools.

### Hooks and Extensions

Policy should leave room for hooks that can warn, block, or enrich tool
execution. Hook packaging and skill integration may be specified downstream, but
the tool runtime should not make them impossible.

Hooks execute in registration order. A hook error blocks the tool call by
default unless policy config says otherwise. Hooks see the same `tool_call_id`,
policy context, workspace, and decision metadata as the permission decision.

MCP tools are scoped by stable server identity, including endpoint and public-key
fingerprint when available. Tool names are not globally trusted across servers.
Untrusted MCP servers default to per-call approval.

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
- policy evaluation latency and rate limits
- MCP server trust policy

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
7. A foreground-policy slice can ship before FEAT-0017; background-specific
   behavior such as pre-approved detached execution and blocked-run inboxes
   lands after durable background runs exist.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Preserves harness-side local enforcement with BFF orchestration |
| Future ADR | Should decide policy inheritance, sandbox/workspace boundaries, and non-overridable policy sources |

## Open Questions

1. What policy language is sufficient for v1 without becoming a general-purpose
   rules engine?
2. Should dangerous command classification be parser-based, pattern-based, or
   both?
3. What exact server-safe tool list should the run-runtime ADR enumerate?
