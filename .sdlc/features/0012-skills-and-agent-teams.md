---
feature: FEAT-0012
title: Skills
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: Runtime Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0006: Provider adapter interface (model override per skill)
promoted-from:
  - EXP-0008: Integrated Harness
---

# FEAT-0012: Skills

## Problem

Common tasks — committing code, reviewing PRs, writing tests, debugging failures — follow the same pattern every time. Users re-explain the process each session. Existing tools (Claude Code, OpenCode) address this with slash commands and prompt templates, but these are single-provider and don't benefit from cross-model routing or the knowledge layer.

## Solution

Skills are reusable named configurations that specialize a single interaction within the current session: a prompt template, a narrowed tool set, and optionally a model override. Skills are harness-side — they shape one turn, then the session continues normally.

Skills are deliberately simple: a prompt and a tool list. They do not create new sessions, spawn agents, or coordinate multi-model work. Multi-agent orchestration is a separate feature (FEAT-0013).

## Key Capabilities

### Skill Definition

A skill is a named configuration with:
- **prompt**: text prepended to the user's message (inline or path to a file)
- **tools**: the tool set available during the skill interaction (narrows the session's tools)
- **model** (optional): model override for this interaction only, reverts afterward

```yaml
# .modeltap.yaml (project-level) or ~/.config/modeltap/config.yaml (global)
skills:
  commit:
    prompt: |
      Review all staged and unstaged changes. Draft a concise commit
      message that focuses on "why" not "what". Follow the project's
      commit conventions. Use git commit -s for DCO sign-off.
    tools: [Read, Bash, Git]

  test:
    prompt: |
      Read the specified code, write comprehensive tests using the
      project's test patterns. Run the tests and iterate until they pass.
    tools: [Read, Write, Edit, Bash, Glob, Grep]

  explain:
    prompt: |
      Explain the code or architecture clearly and concisely.
      Reference specific files and line numbers.
    model: llama-3.1-8b            # cheap model is sufficient
    tools: [Read, Glob, Grep]

  deploy:
    prompt: prompts/skills/deploy.md   # external file
    model: claude-sonnet-4-6
    tools: [Read, Bash, Git, WebFetch]
```

### Built-in Skills

| Skill | Description | Model | Tools |
|-------|-------------|-------|-------|
| `/commit` | Stage changes, draft message, commit | session default | Read, Bash, Git |
| `/review-pr` | Fetch PR, read changes, produce review | session default | Read, Glob, Grep, Bash, Git, WebFetch |
| `/test` | Read code, write tests, run, iterate | session default | Read, Write, Edit, Bash, Glob, Grep |
| `/explain` | Explain code or architecture | routing: cheap | Read, Glob, Grep |
| `/refactor` | Refactor code with specific goals | session default | Read, Write, Edit, Bash, Glob, Grep |
| `/debug` | Investigate a failing test or error | session default | Read, Edit, Bash, Glob, Grep |

Built-in skills can be overridden by project or global config.

### Skill Execution

1. User types `/commit` (with optional arguments: `/commit --amend`)
2. Harness prepends the skill's prompt to the user's message
3. Harness narrows the available tool set to the skill's `tools` list. Tool registration is NOT updated on the server — the harness simply rejects tool calls outside the skill's set. This keeps skills purely harness-side with no protocol changes.
4. If the skill specifies a model, the harness sends `model.switch` before the turn and switches back after `turn.complete`
5. The conversation continues normally after the skill completes — the skill is one turn, not a mode change

Skills are stateless. They do not persist between turns. The skill's prompt and any model override apply to exactly one interaction.

### Custom Skills

Users define skills in project config (`.modeltap.yaml`) or global config. A team shares skills by checking `.modeltap.yaml` into the repository. Custom skills follow the same schema as built-in skills.

### Skill Discovery

```
> /skills
Available skills:
  /commit       Stage changes, draft message, commit
  /test         Read code, write tests, run, iterate
  /review-pr    Fetch PR, read changes, produce review
  /explain      Explain code or architecture
  /refactor     Refactor code with specific goals
  /debug        Investigate a failing test or error
  /deploy       [custom] Deploy to staging (prompts/skills/deploy.md)
```

## CLI Integration

```
modeltap skills list              # List available skills (built-in + custom)
```

In-session:

```
/commit                           # Invoke skill
/commit --amend                   # Skill with arguments
/test internal/auth/              # Skill with target
/skills                           # List available skills
```

## Configuration

See Skill Definition above. Skills are defined in:
1. Project config (`.modeltap.yaml`) — highest priority, shared via source control
2. Global config (`~/.config/modeltap/config.yaml`) — user-level defaults
3. Built-in defaults — lowest priority, always available

## Non-Goals

- **Multi-agent orchestration**: skills are single-agent. Multi-model coordination is FEAT-0013.
- **Skill chaining or pipelines**: a skill is one turn. Running multiple skills in sequence is manual (`/commit` then `/deploy`).
- **Server-side skill execution**: skills are harness-side prompt templates. The server does not know about skills.
- **Dynamic skill suggestion**: the model does not proactively suggest skills. This may be added later.

## Success Criteria

1. Built-in skills (`/commit`, `/test`, `/review-pr`) execute within the current session and produce correct results for their domain.
2. Custom skills defined in `.modeltap.yaml` are discoverable via `/skills` and invocable via `/skillname`.
3. Skills that specify a model override use that model for the skill interaction and revert to the session model afterward. **Test**: session is on model A, invoke a skill with model B, verify the turn uses model B, verify the next turn uses model A.
4. Skill tool narrowing works: a skill with `tools: [Read, Grep]` causes the harness to reject Write or Bash tool calls during that interaction. **Test**: invoke a read-only skill, verify the model receives only the skill's tools in its tool definitions, verify a Write tool call is rejected.
5. Skill executions are captured by the knowledge layer like any other turn. **Test**: invoke `/commit`, verify the turn appears in knowledge search.
6. Project-level skills override built-in defaults. **Test**: define a custom `/commit` in `.modeltap.yaml`, invoke `/commit`, verify the custom prompt is used.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0006 (Providers) | Model override per skill uses the existing routing mechanism |

## Open Questions

1. **Skill arguments**: how are arguments (`/commit --amend`) passed to the skill prompt? String interpolation? Appended to the prompt? Structured parameters?
2. **Skill discoverability**: should the model be aware of available skills and suggest them when appropriate?
3. **Skill composition**: should skills be composable (`/test && /commit`)? Or is sequential manual invocation sufficient?
