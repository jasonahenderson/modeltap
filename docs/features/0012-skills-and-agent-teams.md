---
feature: FEAT-0012
title: Skills and Agent Teams
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0006: Provider adapter interface (multi-model routing)
  - ADR-0008: sqlite-vec for shared knowledge across agents
promoted-from:
  - EXP-0007: Multi-Model Orchestration
  - EXP-0008: Integrated Harness
---

# FEAT-0012: Skills and Agent Teams

## Problem

FEAT-0008 and FEAT-0009 create a single-agent conversation loop: one user, one model, one conversation. But real work often involves patterns that benefit from specialization:

- **Recurring task patterns**: committing, reviewing PRs, writing tests, and deploying follow the same steps every time. Users shouldn't re-explain the process each session.
- **Multi-perspective work**: code written by one model benefits from review by a different model. A planner should be cheap; a coder should be capable; a security reviewer should run locally. Single-agent, single-model conversations can't do this.
- **Parallel execution**: research, review, and analysis tasks can run concurrently across models, producing better results faster than serial single-model work.

Claude Code addresses recurring patterns with skills (prompt templates) and sub-agents (spawned instances of the same model). But Claude Code's agents are all the same provider and model, have no persistent shared memory, and coordinate client-side. Modeltap's BFF architecture enables something structurally different.

## Solution

Two capabilities that build on the BFF conversation loop:

1. **Skills**: reusable prompt + tool configurations that specialize a single agent interaction. Harness-side. No new server capabilities required.
2. **Agent teams**: multi-agent orchestration where the BFF coordinates multiple model sessions — potentially different models from different providers — with shared knowledge and harness-mediated tool execution.

## Key Capabilities

### Skills

A skill is a named configuration that shapes a single interaction: a specialized system prompt fragment, a tool permission set, and optionally a model preference. Skills are invoked with `/skillname` and execute within the current session.

**Built-in skills:**

| Skill | Description | Model | Tools |
|-------|-------------|-------|-------|
| `/commit` | Stage changes, draft message, commit | session default | Read, Bash, Git |
| `/review-pr` | Fetch PR, read changes, produce review | session default | Read, Glob, Grep, Bash, Git, WebFetch |
| `/test` | Read code, write tests, run, iterate | session default | Read, Write, Edit, Bash, Glob, Grep |
| `/explain` | Explain code or architecture | routing: cheap | Read, Glob, Grep |
| `/refactor` | Refactor code with specific goals | session default | Read, Write, Edit, Bash, Glob, Grep |
| `/debug` | Investigate a failing test or error | session default | Read, Edit, Bash, Glob, Grep |

**Skill definition:**

```yaml
# .modeltap.yaml or ~/.config/modeltap/skills/
skills:
  commit:
    prompt: |
      Review all staged and unstaged changes. Draft a concise commit
      message that focuses on "why" not "what". Follow the project's
      commit conventions. Use git commit -s for DCO sign-off.
    tools: [Read, Bash, Git]

  deploy:
    prompt: prompts/skills/deploy.md    # external file
    model: claude-sonnet-4-6            # override session model
    tools: [Read, Bash, Git, WebFetch]

  review-security:
    prompt: prompts/skills/security-review.md
    model: llama-3.1-70b                # local, no data leaves machine
    tools: [Read, Glob, Grep]
```

**How skills execute:**

1. User types `/commit` (with optional arguments: `/commit --amend`)
2. Harness prepends the skill's prompt to the user's message
3. If the skill specifies a model, the harness sends a `/model` switch for this turn only (reverts after)
4. Tool permissions narrow to the skill's tool set for this interaction
5. The conversation continues normally after the skill completes

Skills are stateless — they don't create new sessions or agent contexts. They shape one interaction within the existing session. The BFF captures the skill invocation like any other turn, so the knowledge layer learns from skill executions.

**Custom skills:** Users define skills in project config (`.modeltap.yaml`) or global config. A team can share skills by checking `.modeltap.yaml` into the repository.

### Agent Teams

An agent team is a named group of specialized agents that the BFF coordinates to accomplish a complex task. Each agent has its own model, system prompt, and tool permissions. The BFF manages the execution flow, shared context, and result synthesis.

**What makes this different from Claude Code sub-agents:**

| Dimension | Claude Code | Modeltap |
|-----------|------------|----------|
| Model per agent | Same (Anthropic only) | Different models, different providers |
| Shared memory | Parent passes context | Knowledge layer across all agents |
| Coordination | Client-side | BFF-side (server orchestration) |
| Cost optimization | No | Cheap planner, expensive coder, local reviewer |
| Cross-provider | No | Security agent on local model, coder on cloud |

**Team definition:**

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
        role: Analyze the task, break into steps, identify files

      backend:
        model: claude-opus-4-6
        prompt: prompts/agents/backend.md
        tools: [Read, Write, Edit, Bash, Glob, Grep, Git]
        role: Implement the plan in production code

      tester:
        model: claude-opus-4-6
        prompt: prompts/agents/tester.md
        tools: [Read, Write, Edit, Bash, Glob, Grep]
        role: Write tests for the implementation, run them, iterate

      reviewer:
        model: gpt-4
        prompt: prompts/agents/reviewer.md
        tools: [Read, Glob, Grep]
        role: Review changes for correctness, design, and edge cases

      security:
        model: llama-3.1-70b
        prompt: prompts/agents/security.md
        tools: [Read, Glob, Grep]
        role: Review for OWASP top 10, injection, data leakage

    flow:
      - agent: planner
        approval: required          # user must approve the plan
      - agent: backend
        depends_on: planner
      - agent: tester
        depends_on: backend
      - parallel:                   # reviewer and security run concurrently
          - agent: reviewer
            depends_on: tester
          - agent: security
            depends_on: backend
```

**Built-in team patterns:**

| Team | Agents | Flow |
|------|--------|------|
| `implement` | planner → backend → tester → reviewer + security | Serial with parallel review |
| `review` | structural reviewer + adversarial reviewer (different models) → synthesis | Parallel then merge |
| `research` | decomposer → N parallel researchers → synthesizer | Fan-out then merge |
| `migrate` | analyzer → planner → implementer → tester | Serial |

### Team Execution Flow

**Invocation:**

```
> /team implement "Add rate limiting to the API endpoints"
```

**BFF orchestration:**

1. BFF creates agent sessions — each agent gets its own conversation context with its assigned model and system prompt
2. BFF injects shared context into each agent:
   - The user's task description
   - Relevant knowledge layer results for the project
   - Outputs from prior agents in the flow (e.g., backend sees planner's plan)
3. BFF runs agents according to the flow definition:
   - Serial steps execute in order
   - Parallel steps execute concurrently
   - Approval steps pause and wait for harness/user response
4. Tool calls from agents flow to the harness for execution
5. BFF captures every agent's conversation for the knowledge layer
6. BFF synthesizes final results and returns to the harness

**Harness display:**

```
Assembling team: implement
───────────────────────────────────────

[planner] claude-sonnet-4-6 | planning...
Plan:
  1. Identify API endpoints (internal/api/router.go)
  2. Design middleware approach
  3. Implement rate limiter (internal/middleware/ratelimit.go)
  4. Add per-endpoint config
  5. Write tests
  6. Review

[a]pprove  [e]dit plan  [c]ancel
> a

[backend] claude-opus-4-6 | implementing...
  Read: internal/api/router.go ✓
  Write: internal/middleware/ratelimit.go ✓
  Edit: internal/api/router.go ✓
  Bash: go build ./... ✓

[tester] claude-opus-4-6 | testing...
  Write: internal/middleware/ratelimit_test.go ✓
  Bash: go test ./internal/middleware/... ✓ (8 passed)

[reviewer] gpt-4 | reviewing...        [security] llama-3.1-70b | scanning...
  ⚠ In-memory store won't work          ✓ No injection vectors
    across multiple instances            ✓ Headers don't leak topology
                                         ⚠ Add health check bypass

─── Team complete | 5 agents | $0.47 | 2m 14s ───

Review findings:
1. [reviewer] In-memory rate limit store needs shared state for
   multi-instance deployments. Consider Redis or database-backed store.
2. [security] Health check endpoints should bypass rate limiting
   to avoid monitoring false positives.

Apply reviewer suggestions? [y]es  [n]o  [s]elect
```

### Tool Execution and Permissions

When agents execute tools through the harness, permissions follow a per-agent model:

**Read-only agents** (reviewer, security, planner): auto-execute. Their tool sets contain only read operations. No permission prompts needed.

**Writing agents** (backend, tester): follow the current session permission level (default, accept-edits, autonomous). The harness shows which agent is requesting the action:

```
[backend] Edit internal/api/router.go?
  Replace: "func SetupRoutes..."
  With: "func SetupRoutes..." (adds rate limit middleware)
[y/n]
```

**Plan-scoped approval**: when the user approves a plan, writing agents can execute within the plan's scope without per-action prompts. Tool calls that deviate from the plan trigger explicit approval. This is a refinement of the plan/build mode from FEAT-0009 — the plan becomes a scope boundary, not just a preview.

### Agent Context Sharing

Agents in a team share context through two mechanisms:

**Explicit handoff**: the BFF includes prior agents' outputs in subsequent agents' context. The backend agent sees the planner's plan. The tester sees the backend's code changes. The reviewer sees everything.

**Knowledge layer**: all agents query the same per-user knowledge base. The planner asks "what patterns does this project use?" The security agent asks "what vulnerabilities have we found before?" This is automatic — agents don't need to know about each other's knowledge queries.

**Context budget per agent**: each agent has its own context window (determined by its model). The BFF manages how much prior-agent output to include, summarizing if necessary. A reviewer that uses a model with a 128K window gets the full output; one with a 32K window gets a summary.

### Observability

**During execution**: the harness shows real-time progress per agent (tool calls, status, errors).

**After completion**: `/trace` shows the full execution graph:

```
> /trace
Team: implement | 2m 14s | $0.47

planner (claude-sonnet-4-6)     12s  $0.02  1.2K tokens
  └── plan: 6 steps

backend (claude-opus-4-6)       48s  $0.18  8.4K tokens
  ├── Read: 3 files
  ├── Write: 1 file
  ├── Edit: 2 files
  └── Bash: 1 command

tester (claude-opus-4-6)        34s  $0.15  6.1K tokens
  ├── Write: 1 file
  └── Bash: 2 commands (8 tests passed)

reviewer (gpt-4)                22s  $0.08  3.2K tokens  ─┐ parallel
security (llama-3.1-70b)        18s  $0.00  2.8K tokens  ─┘

Total: 5 agents, 21.7K tokens, $0.43
```

**Cost attribution**: each agent's cost is tracked separately. The team total is displayed. Over time, the knowledge layer accumulates data on which team configurations are cost-effective for which task types.

## CLI Integration

```
modeltap skills list                    # List available skills
modeltap teams list                     # List available teams
modeltap teams describe implement       # Show team definition and flow
```

In-session:

```
/commit                                 # Invoke a skill
/commit --amend                         # Skill with arguments
/team implement "task description"      # Invoke a team
/team implement --step "task"           # Step-through mode
/team implement --verbose "task"        # Show all tool calls
/team implement --dry-run "task"        # Plan only, no execution
/trace                                  # Show last team execution
```

## Configuration

### Skills Configuration

```yaml
# .modeltap.yaml (project-level) or ~/.config/modeltap/config.yaml (global)
skills:
  # Override built-in skill
  commit:
    prompt: |
      Follow this project's commit conventions from CLAUDE.md.
      Always use conventional commits format.
      Use git commit -s for DCO sign-off.
    tools: [Read, Bash, Git]

  # Custom skill
  deploy:
    prompt: prompts/skills/deploy.md
    model: claude-sonnet-4-6
    tools: [Read, Bash, Git, WebFetch]
```

### Agent Teams Configuration

```yaml
# .modeltap.yaml
teams:
  implement:
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

  review:
    agents:
      structural:
        model: claude-opus-4-6
        prompt: prompts/agents/structural-review.md
        tools: [Read, Glob, Grep]
      adversarial:
        model: gpt-4
        prompt: prompts/agents/adversarial-review.md
        tools: [Read, Glob, Grep]
      synthesizer:
        model: claude-sonnet-4-6
        prompt: prompts/agents/synthesize-review.md
        tools: [Read]
    flow:
      - parallel:
          - agent: structural
          - agent: adversarial
      - agent: synthesizer
        depends_on: [structural, adversarial]
```

### Enterprise Team Management

In enterprise deployments (FEAT-0010), team definitions can be managed at the server level:

```yaml
# Server-level team policy
teams:
  policy:
    # All teams must include a security reviewer
    required_agents:
      - role: security
        tools_max: [Read, Glob, Grep]   # read-only only

    # Agent model assignments must respect user's role model access
    enforce_model_access: true

    # Maximum concurrent agents per team execution
    max_parallel_agents: 5

    # Maximum total cost per team execution
    max_team_cost: 5.00
```

## Non-Goals

- **Autonomous long-running agent swarms**: teams execute a bounded task and return. No infinite loops, no unsupervised background execution.
- **Agent-to-agent direct communication**: agents don't talk to each other. The BFF mediates all context sharing through explicit handoff and the knowledge layer.
- **Dynamic team composition**: for this feature, team membership is defined in config. Dynamic "the model decides which agents to spawn" is future work.
- **Cross-user agent teams**: all agents in a team execute within a single user's context and data isolation boundary (FEAT-0010).
- **Agent state persistence**: agent sessions are ephemeral — they exist for the duration of the team execution. The knowledge layer captures their output for future reference, but the agent's conversation state is not resumable.

## Success Criteria

### Skills

1. Built-in skills (`/commit`, `/test`, `/review-pr`) execute within the current session using the skill's prompt and tool set.
2. Custom skills defined in `.modeltap.yaml` are discoverable via `/skills` and invocable via `/skillname`.
3. Skills that specify a model override use that model for the skill interaction and revert to the session model afterward.
4. Skill tool permissions narrow the available tools — a skill with `tools: [Read, Grep]` cannot trigger a Write or Bash.
5. Skill executions are captured by the knowledge layer like any other turn.

### Agent Teams

6. A user can invoke `/team implement "task"` and the BFF coordinates planner → backend → tester → reviewer + security with the correct model for each agent.
7. Plan approval pauses execution until the user approves, edits, or cancels.
8. Parallel agents (reviewer + security) execute concurrently and their results are collected before the team completes.
9. Each agent's tool calls flow through the harness with appropriate permission enforcement: read-only agents auto-execute, writing agents follow the permission level.
10. `/trace` shows the full execution graph with per-agent model, timing, token usage, and cost.
11. Agent context sharing works: the backend sees the planner's plan, the tester sees the backend's changes, the reviewer sees everything.
12. Knowledge layer queries work within agent sessions — agents benefit from prior project context.
13. Custom team definitions in `.modeltap.yaml` override or extend built-in teams.
14. Team execution respects FEAT-0010 model access and spend budget policies.
15. Total team cost does not exceed the configured `max_team_cost` limit.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0006 (Providers) | Each agent in a team can use a different provider — the adapter interface supports concurrent multi-provider usage |
| ADR-0008 (Knowledge) | Shared knowledge across agents within a team; all agent outputs captured for future retrieval |
| ADR-0007 (Metrics) | Per-agent cost tracking feeds into aggregation tables with agent/team dimensions |

## Open Questions

1. **Flow definition language**: is YAML with `depends_on` and `parallel` blocks expressive enough? Or do complex workflows need a more capable notation (DAG definition, conditionals, loops)?
2. **Agent failure handling**: if the backend agent fails (tests don't pass after N attempts), does the team fail? Retry with a different model? Fall back to the user?
3. **Incremental team results**: should partial results be available before the full team completes? E.g., the user sees the backend's code immediately, doesn't wait for review.
4. **Shared file system state**: when the backend writes files and the tester reads them, this works because both route tool calls through the same harness. But in parallel execution, two writing agents could conflict. Should the BFF enforce serial writes?
5. **Skill discoverability**: should the model be aware of available skills and suggest them? ("This looks like a commit task — want me to use `/commit`?")
6. **Team templates vs. ad hoc**: should the user be able to describe a team in natural language ("use a cheap model to plan, a strong model to code, and review with a different model") and have the BFF assemble it?
