---
feature: FEAT-0013
title: Agent Teams
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
  - FEAT-0012: Skills (shared prompt/tool configuration patterns)
adr-constraints:
  - ADR-0006: Provider adapter interface (multi-model concurrent usage)
  - ADR-0008: sqlite-vec for shared knowledge across agents
  - ADR-0007: Pre-computed aggregation (per-agent cost tracking)
promoted-from:
  - EXP-0007: Multi-Model Orchestration
  - EXP-0008: Integrated Harness
---

# FEAT-0013: Agent Teams

## Problem

Single-agent conversations hit three limitations:

- **Single perspective**: one model's blind spots persist. A different model catches different issues.
- **Serial execution**: research, review, and analysis run sequentially through one model. Parallel execution across models is faster and produces richer results.
- **One-size-fits-all cost**: every subtask uses the same model, whether it needs a $0.15/turn reasoning model or a $0.00 local model.

Claude Code addresses this with sub-agents, but all sub-agents use the same provider and model, have no persistent shared memory, and coordinate client-side. Modeltap's BFF architecture enables multi-model teams coordinated server-side with shared knowledge.

## Solution

Agent teams are BFF-coordinated groups of specialized agents that work on subtasks of a complex task. Each agent has its own model, system prompt, and tool permissions. The BFF manages the execution flow, context sharing between agents, cost tracking, and result synthesis. The harness renders progress and handles tool execution for all agents.

## Key Capabilities

### Team Definition

A team is a named configuration of agents and a flow that defines execution order:

```yaml
# .modeltap.yaml
teams:
  implement:
    description: Plan, implement, test, and review a feature
    agents:
      planner:
        model: claude-sonnet-4-6
        prompt: prompts/agents/planner.md
        tools: [Read, Glob, Grep, Git]
      backend:
        model: claude-opus-4-6
        prompt: prompts/agents/backend.md
        tools: [Read, Write, Edit, Bash, Glob, Grep, Git]
      tester:
        model: claude-opus-4-6
        prompt: prompts/agents/tester.md
        tools: [Read, Write, Edit, Bash, Glob, Grep]
      reviewer:
        model: gpt-4
        prompt: prompts/agents/reviewer.md
        tools: [Read, Glob, Grep]
      security:
        model: llama-3.1-70b
        prompt: prompts/agents/security.md
        tools: [Read, Glob, Grep]
    flow:
      - agent: planner
        approval: required
      - agent: backend
        depends_on: planner
      - agent: tester
        depends_on: backend
      - parallel:
          - agent: reviewer
            depends_on: tester
          - agent: security
            depends_on: backend
```

### Built-in Team Patterns

| Team | Agents | Flow |
|------|--------|------|
| `implement` | planner → backend → tester → reviewer + security | Serial with parallel review |
| `review` | structural + adversarial (different models) → synthesizer | Parallel then merge |
| `research` | decomposer → N parallel researchers → synthesizer | Fan-out then merge |

### Team Execution

1. User invokes `/team implement "Add rate limiting to the API"`
2. BFF creates agent sessions — each gets its own conversation context with assigned model and system prompt
3. BFF injects shared context: task description, knowledge layer results, prior agent outputs
4. BFF runs agents per the flow definition (serial, parallel, or mixed)
5. Tool calls flow to the harness — the harness executes locally and returns results
6. BFF captures every agent's conversation for the knowledge layer
7. BFF synthesizes results and returns to the harness

### Safe Execution Rules

Multi-agent execution introduces concurrency and conflict risks that single-agent mode does not have. These rules are mandatory and not configurable:

**Write serialization**: at most one agent may have outstanding write tool calls at any time. The BFF serializes write operations across agents. If two agents request writes concurrently (e.g., in a parallel block), the BFF queues the second write until the first completes. Read-only tool calls are not serialized — multiple agents can read concurrently.

**Failure behavior**:
- If a **read-only agent** fails (reviewer, security): the team continues. The failure is reported in the results. Other agents are not affected.
- If a **writing agent** fails (tool error, test failure): the BFF retries once with the error context included. If the retry fails, the team **pauses** and asks the user: `[r]etry with different approach, [s]kip this agent, [a]bort team`.
- If the user aborts: all pending agents are cancelled. Completed work (files written, tests run) remains on disk. The team result shows partial completion.

**Plan-scope enforcement**: when the user approves a plan (from the planner agent), subsequent agents operate within the plan's scope:
- Tool calls that target files or operations outside the plan trigger explicit harness approval, even in accept-edits or auto mode.
- The BFF tracks which files and operations the plan declared and flags out-of-scope tool calls as `plan_deviation: true` in the `tool.call` event.
- The harness shows deviations distinctly: `[backend] ⚠ Out of plan: Edit README.md — approve? [y/n]`

**Cost cap**: team execution stops if the total team cost exceeds the configured `max_team_cost`. The BFF tracks running cost across all agents and halts new agent starts when the cap is approached. In-flight agents complete their current turn but do not start new turns.

**No unsupervised infinite loops**: each agent has a maximum turn count (configurable, default: 20). If an agent hits its turn limit, it is stopped and its partial output is included in the team results.

### Agent Context Sharing

**Explicit handoff**: the BFF includes prior agents' outputs in subsequent agents' context. The backend sees the planner's plan. The tester sees the backend's code changes.

**Knowledge layer**: all agents query the same per-user knowledge base. This is automatic — agents inherit the session's knowledge access.

**Context budget per agent**: each agent has its own context window (determined by its model). The BFF manages how much prior-agent output to include, summarizing if necessary for agents with smaller windows.

### Tool Execution and Permissions

**Read-only agents** (tool set contains only Read, Glob, Grep, Git status/log/diff): auto-execute without harness permission prompts.

**Writing agents** (tool set includes Write, Edit, Bash, Git mutations): follow the current session permission level. The harness shows which agent is requesting:

```
[backend] Edit internal/api/router.go?
  old: "func SetupRoutes..."
  new: "func SetupRoutes..." (adds rate limit middleware)
[y/n]
```

### Observability

**During execution**: the harness shows per-agent progress.

**After completion**: `/trace` shows the full execution graph:

```
> /trace
Team: implement | 2m 14s | $0.47

planner (claude-sonnet-4-6)     12s  $0.02  1.2K tokens
backend (claude-opus-4-6)       48s  $0.18  8.4K tokens
tester (claude-opus-4-6)        34s  $0.15  6.1K tokens
reviewer (gpt-4)                22s  $0.08  3.2K tokens  ─┐ parallel
security (llama-3.1-70b)        18s  $0.00  2.8K tokens  ─┘
```

### Enterprise Policy Integration

In enterprise deployments (FEAT-0010):

- Agent model assignments must respect the user's role-based model access. An agent configured with `claude-opus-4-6` fails if the user's role denies access to Opus.
- Team total cost counts against the user's spend budget.
- Server-level team policy can require specific agents (e.g., all teams must include a security reviewer).
- Maximum concurrent agents per team is configurable at the server level.

## CLI Integration

```
modeltap teams list                     # List available teams
modeltap teams describe implement       # Show team definition and flow
```

In-session:

```
/team implement "task description"      # Invoke a team
/team implement --step "task"           # Step-through: approve each agent
/team implement --verbose "task"        # Show all tool calls from all agents
/team implement --dry-run "task"        # Plan only, no execution
/trace                                  # Show last team execution graph
```

## Configuration

Team definitions in `.modeltap.yaml` (project-level) or global config. See Team Definition above.

Server-level policy (enterprise):

```yaml
teams:
  policy:
    required_agents:
      - role: security
        tools_max: [Read, Glob, Grep]
    enforce_model_access: true
    max_parallel_agents: 5
    max_team_cost: 5.00
    max_agent_turns: 20
```

## Non-Goals

- **Autonomous long-running swarms**: teams execute a bounded task and return. No infinite loops, no unsupervised background execution.
- **Agent-to-agent direct communication**: agents don't talk to each other. The BFF mediates all sharing.
- **Dynamic team composition from natural language**: "use a cheap model to plan and a strong model to code" assembled at runtime. Future work.
- **Cross-user agent teams**: all agents in a team run within one user's isolation boundary.
- **Agent state persistence**: agent sessions are ephemeral. Knowledge layer captures outputs for future reference, but the agent's conversation is not resumable.

## Success Criteria

1. A user can invoke `/team implement "task"` and the BFF coordinates planner → backend → tester → reviewer + security with the correct model for each agent.
2. Plan approval pauses execution until the user approves, edits, or cancels.
3. Parallel agents execute concurrently and their results are collected before the team completes. **Test**: reviewer and security start at the same time (within 1s), not sequentially.
4. Write serialization works: if two agents in a parallel block request writes, the second waits until the first completes. **Test**: simulate two concurrent write requests, verify they execute serially.
5. Writing agent failure pauses the team and prompts the user. **Test**: inject a tool error in the backend agent, verify the team pauses with retry/skip/abort options.
6. Plan-scope enforcement flags deviations. **Test**: approve a plan targeting files A and B. The backend agent requests Edit on file C. Verify the harness shows a plan-deviation warning.
7. Cost cap stops the team. **Test**: set `max_team_cost: 0.10`, run a team that would cost $0.50. Verify execution stops near the cap.
8. Agent turn limit prevents infinite loops. **Test**: set `max_agent_turns: 3`, give an agent a task that would loop. Verify it stops after 3 turns.
9. `/trace` shows the full execution graph with per-agent model, timing, tokens, and cost.
10. Read-only agents auto-execute without permission prompts. Writing agents follow the session permission level.
11. Enterprise model access is enforced: an agent configured with a denied model fails with a clear error.
12. Custom team definitions in `.modeltap.yaml` work as specified.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0006 (Providers) | Each agent uses a different provider — concurrent multi-provider support required |
| ADR-0007 (Metrics) | Per-agent cost tracking feeds into aggregation with agent/team dimensions |
| ADR-0008 (Knowledge) | Shared knowledge across agents; all agent outputs captured |

## Open Questions

1. **Flow definition expressiveness**: is YAML with `depends_on` and `parallel` sufficient? Or do conditional flows (if tests fail, re-route to a different agent) need to be supported from the start?
2. **Incremental results**: should partial results be available before the full team completes?
3. **Team templates marketplace**: should modeltap provide a community repository of team definitions?
