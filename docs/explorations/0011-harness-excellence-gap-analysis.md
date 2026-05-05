---
exploration: EXP-0011
title: Harness Excellence Gap Analysis
status: exploring
date: 2026-04-28
related:
  - EXP-0008: Integrated Harness
  - EXP-0009: Harness Prompt Architecture
  - EXP-0010: Harness Comparative Analysis
  - FEAT-0009: Terminal Harness
  - FEAT-0012: Skills and Agent Teams
  - FEAT-0013: Agent Teams
  - FEAT-0014: Harness Conversation Shell
  - ADR-0014: Harness Base Strategy
promoted-to:
  - FEAT-0015: Professional Harness Runtime
---

# EXP-0011: Harness Excellence Gap Analysis

## Context

The Claude Code source-map leak turned the architecture of a best-in-class
terminal coding harness into public design material. Several public analyses
describe the leaked architecture as roughly 512K lines of TypeScript across
about 1.9K source files, and highlight the subsystems that make the product
feel stronger than a generic "chat with tools" wrapper: project memory,
compaction, permissions, tool governance, hooks, and parallel agents.

That matters for modeltap because ADR-0014 already decided not to fork another
harness. Modeltap's strategic bet is a Go/Bubbletea thin client plus a BFF that
owns routing, capture, cost, memory, and orchestration. The remaining question
is sharper: **what makes Claude Code and Codex produce better code from the
same basic model-call shape, and where is modeltap still behind?**

This exploration uses public reporting and clone ecosystem docs as research
input. It does not rely on or copy leaked proprietary code. Sources include:

- [Superframeworks: Claude Code leak analysis](https://superframeworks.com/articles/claude-code-source-code-leak)
- [FluxWise: Claude Code source leak security analysis](https://fluxwise.tech/en/resources/articles/2026-04-01-claude-code-source-leak-analysis)
- [Claw Code architecture overview](https://claw-code.codes/)
- [OpenAI Codex CLI docs](https://developers.openai.com/codex/cli)
- [OpenAI Codex app announcement](https://openai.com/index/introducing-the-codex-app/)

## Thesis

An excellent harness is not primarily a terminal UI or a pile of agentic
features. It is an **execution environment** that makes an LLM generate better
code by controlling context, repository understanding, edit mechanics,
validation feedback, permissions, state, and recovery.

The model matters, but the harness turns model capability into repeatable
workflow capability. A weak harness leaks user intent into ambiguous prompts,
forgets project state, asks for approvals at the wrong time, loses work across
sessions, and treats tool execution as a bolt-on. A strong harness makes the
model's operating conditions explicit and durable so it writes code that fits
the project, compiles, passes tests, and minimizes unnecessary churn.

Agentic features are not the goal here. Subagents, hooks, skills, and
background execution only matter when they improve code generation quality:
better context selection, better implementation plans, better validation, better
review, or safer edits.

## Internal Mechanics: Why Basic Calls Feel Better

The visible features are downstream effects. The deeper gap is that excellent
harnesses wrap even a plain "answer this" or "edit this file" call in a richer
preflight, execution, and postflight pipeline.

### What a Strong Basic Call Does

A strong harness does more than send `{message, history, tools}` to a model.
For a normal user turn, it typically performs this sequence:

1. **Load operating policy:** resolve project rules, user rules, team/admin
   rules, active mode, sandbox profile, and permission state.
2. **Assemble context deliberately:** choose current transcript slice, project
   instructions, memory digest, file attachments, retrieved facts, and compacted
   summaries with explicit budget accounting.
3. **Shape the model contract:** inject the right tool set, tool descriptions,
   response style, mode-specific behavior, and task constraints for this turn.
4. **Classify risk before execution:** mark which tools, paths, domains, shell
   commands, and Git operations are allowed, promptable, blocked, or require a
   stronger sandbox.
5. **Stream with state:** track run id, phase, active tool, partial output,
   context/cost counters, cancellation token, and resumability metadata.
6. **Gate every side effect:** validate tool arguments, apply hooks/policy,
   request approval at the narrowest useful granularity, then execute with an
   audit record.
7. **Convert output into artifacts:** store diffs, command logs, test results,
   approvals, failures, costs, memory candidates, and compaction checkpoints.
8. **Recover cleanly:** support interrupt, retry, continue, resume, fork, and
   inspect without making the user reconstruct state from scrollback.

The user experiences this as "the agent is smarter," but the model may be doing
less guesswork because the harness already turned the work into a well-specified
operating environment.

### Modeltap's Current Basic Call Shape

The current production runtime is much thinner. For `SubmitTurn`, it:

1. gets the live BFF client
2. resolves attachments through `ContextManager`
3. creates a turn id, session id, sequence number, mode, content, and
   attachment list
4. calls `ProtocolClient.SubmitTurn`
5. stores the returned session id
6. lets stream/tool/permission events flow back through the adapter

That is a good transport skeleton, and it preserves modeltap's BFF-first
architecture. But it is not yet an excellent call loop. It does not yet own or
surface the full preflight/postflight machinery: project-rule discovery, memory
selection, prompt-layer inspection, policy evaluation, sandbox selection, risk
classification, artifact extraction, run recovery, or learning from the turn.

### The Real Gap

The gap is not "Claude Code has more commands." The gap is that Claude Code and
Codex appear to treat each call as a managed transaction through an agent
runtime. Modeltap currently treats a call mostly as a protocol submission plus
local tool mediation.

For modeltap to feel superior, the BFF and harness need a shared call pipeline:

```
intent -> policy -> context plan -> prompt plan -> model call -> tool loop
       -> artifact capture -> memory update -> recovery checkpoint
```

The harness does not need to own every step. In modeltap, the BFF should own
most intelligence steps. But the harness must display and enforce the pipeline:
what context went in, what policy applies, what tools are live, why approval is
needed, what changed, what was learned, and how to recover.

## Code Generation Quality Mechanics

For modeltap, "better harness" should be measured by the quality of generated
patches, not by the number of autonomous features. The harness should improve
code generation through these internal mechanics.

### 1. Repository-Aware Context Selection

The model needs the right files, symbols, conventions, and tests before it
writes. A quality harness should build a context plan before generation:

- identify relevant files from import graph, package boundaries, tests, recent
  changes, and user-mentioned paths
- include local style examples from nearby code, not just generic instructions
- include accepted ADR/feature/process constraints when they apply
- preserve context provenance so the user can see why a file or rule was used
- avoid stuffing unrelated files that crowd out the useful signal

**Modeltap gap:** `ContextManager` can resolve attachments, but modeltap does
not yet have repo-map selection, symbol-aware retrieval, test discovery, style
sampling, or a visible "why this context" plan for a codegen turn.

### 2. Edit Discipline and Patch Semantics

Good harnesses bias the model toward small, reviewable, project-shaped edits.
The tool layer should make bad edits harder:

- prefer exact-match edits or structured patches over blind rewrites
- require read-before-write for mutated files
- keep diffs narrow and preserve unrelated user changes
- make file creation/deletion explicit
- detect generated churn, formatting-only blast radius, and accidental
  unrelated edits
- expose the patch as an artifact before and after validation

**Modeltap gap:** modeltap has read-before-mutate tracking and exact edit
semantics, which is a strong base. It does not yet have diff-quality checks,
semantic patch summaries, churn detection, unrelated-change detection, or a
codegen-specific "patch quality" loop.

### 3. Validation Feedback Loop

The best codegen harnesses convert compiler, test, lint, and runtime feedback
into the next model input. This is where a basic call becomes a high-quality
coding loop:

- discover the cheapest relevant checks automatically
- run targeted tests before broad tests
- summarize failures with file/line context and prior attempted fixes
- avoid repeating failed fixes
- stop when the evidence is sufficient rather than chasing every possible check
- cite validation evidence in the final answer

**Modeltap gap:** shell commands can run through tools, but the harness does
not yet plan validation, select targeted checks, summarize failures, remember
failed repair attempts, or store validation evidence as a first-class artifact.

### 4. Prompt Contract for Code Changes

The model should receive a stable codegen contract on every implementation
turn. That contract should say how to reason about the repo, how to edit, how
to validate, and how to report. This is separate from product features.

Useful prompt-layer obligations:

- inspect before editing
- follow local patterns over invented abstractions
- preserve unrelated changes
- prefer minimal diffs
- add tests proportional to risk
- run or explain relevant validation
- summarize only material changes and residual risk

**Modeltap gap:** agent instructions exist for this repo, but modeltap the
product does not have a formal, versioned codegen prompt contract that the BFF
assembles for every implementation turn.

### 5. Quality-Oriented Model Routing

Modeltap's BFF can become a quality advantage if routing is based on codegen
needs, not just provider preference:

- use stronger models for architecture-sensitive edits
- use faster/cheaper models for repo search, summarization, and failure
  triage
- route validation-failure repair differently from first-pass generation
- compare candidate patches only when the quality gain justifies the cost
- record which routing decisions correlated with successful patches

**Modeltap gap:** the architecture supports routing, but there is no accepted
quality-driven routing policy for code generation turns.

## Feature Gaps Already Identified

These are real product gaps, but they should be treated as downstream enablers
of code generation quality rather than the primary strategy.

### Context and Memory

- no product-level `MODELTAP.md` / project-rules discovery contract
- no explicit precedence among `MODELTAP.md`, `AGENTS.md`, `CLAUDE.md`, user
  config, team config, and server policy
- no active prompt-layer inspector
- no visible memory digest or "why this memory was injected" provenance
- no project/session/global memory split exposed in the harness
- no automatic context compaction UX beyond the planned protocol surface

### Codegen Quality Loop

- no repo-map or symbol-aware context planner
- no automatic test/check discovery for a requested code change
- no validation-plan artifact before edits
- no structured repair loop that remembers failed fixes
- no diff-quality scoring, churn detection, or unrelated-change detection
- no final evidence artifact tying changes to commands/tests run
- no harness-level codegen benchmark suite

### Tool Runtime and Permissions

- no policy language for commands, paths, domains, or tool classes
- no workspace sandbox profile selection per run
- no command-risk classifier beyond existing dangerous-command checks
- no hookable pre-tool / post-tool lifecycle
- no permission audit surface that groups decisions by run, tool, path, and
  outcome
- no team/admin policy inheritance for harness-side local execution

### Extension Surface

- MCP exists, but skills, hooks, slash commands, and prompt overlays are not one
  coherent extension model
- no command/plugin packaging contract
- no extension permission manifest
- no extension provenance or trust model
- no per-skill tool narrowing

### Session and Run Artifacts

- no first-class run artifact bundle containing prompt/context plan, patch,
  commands, tests, approvals, cost, and outcome
- no diff/test evidence panes in the terminal shell
- no checkpointed continue/retry/fork workflow for a failed implementation run
- no session replay UI that reconstructs task intent, not just transcript text
- no durable task summary promoted into project memory after successful work

### Orchestration and Parallelism

- no scoped reviewer/validator role for codegen quality
- no candidate patch comparison workflow
- no worktree-backed isolation for risky or parallel edits
- no worker diff review or merge path
- no terminal surface for team/subagent status if modeltap later uses BFF
  orchestration

### Security and Capture Hardening

- no accepted fake-tool / anti-distillation posture
- no encrypted or redacted capture mode defined per deployment profile
- no secret-aware prompt/capture policy tied to local files
- no capture retention policy visible from the harness
- no enterprise audit/export story for local tool execution and model calls

## What Excellent Harnesses Have

### 1. A Layered Context Contract

Claude Code's strongest public lesson is that project context is first-class.
Public analyses report that `CLAUDE.md` is injected into every prompt turn and
is budgeted as a serious project-level instruction surface. Codex has the same
category through `AGENTS.md`, rules, skills, memories, and config docs.

The excellence criterion is not the filename. It is the contract:

- project rules are discovered automatically
- scope and precedence are deterministic
- context is injected every relevant turn
- memory is summarized, bounded, and refreshed
- users can inspect what context is active

**Modeltap gap:** EXP-0009 proposes this shape, but the product still lacks a
durable `MODELTAP.md` / project-rules contract, prompt-layer inspector, memory
digest, and context provenance UI.

### 2. Tool Execution as a Governed Runtime

The leaked Claude Code analyses emphasize deep Bash checks and tiered
permissions. Codex similarly foregrounds local file reads, edits, command
execution, approvals, sandboxing, rules, and MCP. Claw Code's public overview
frames tools as permission-gated, independently sandboxed runtime operations.

The important pattern is that tools are not just function calls. They are a
runtime with:

- typed schemas and stable names
- preflight validation
- permission classification
- sandbox / workspace scope
- audit trail
- replayable result records
- user-tunable policy

**Modeltap gap:** modeltap has useful built-in tools, MCP support, and a
permission enforcer. It does not yet have a policy language, command allowlists,
workspace sandbox profile, rich audit UI, hookable pre/post tool lifecycle, or
classifier-assisted approvals.

### 3. Parallel Agents Are Optional Quality Tools

Claude Code leak analyses describe subagent modes and worktree isolation.
Codex's app positioning is explicit: supervise multiple agents, run work in
parallel, use isolated worktrees, and move between agent threads without losing
context. Claw Code claims swarm/subagent support as a core capability.

For code generation quality, parallelism is useful only when it improves the
patch: independent exploration, alternate implementation proposals, targeted
review, or isolated validation. The excellence criterion is not simply "spawn
another model call." It is:

- scoped delegation
- isolated workspaces when edits may conflict
- shared but bounded memory
- visible progress per worker
- reviewable diffs per worker
- cancellation and merge paths
- parent-agent synthesis

**Modeltap gap:** ADR-0014 correctly says the harness can become the terminal
orchestration client, but codegen quality should drive the scope. Current
modeltap has no bounded reviewer/validator role, worktree/session isolation
model, candidate patch comparison, or worker diff review.

### 4. Session State Is Durable and Resumable

Excellent harnesses treat a session as a durable workspace, not a terminal
buffer. Claude Code analyses call out persistent JSONL sessions, explicit
resume, compaction, and memory types. Codex CLI/app share configuration and
history across surfaces, while app agents run in separate project threads.

The excellence criterion:

- every turn, tool call, approval, file diff, and test result is recoverable
- sessions resume without losing task intent
- compaction is explicit and inspectable
- long-running tasks can survive UI restarts
- task artifacts become knowledge, not scrollback

**Modeltap gap:** the BFF architecture is well-positioned for this, but the
current harness shell has not exposed session replay, persistent task state,
compaction controls, or durable run artifacts as first-class UX.

### 5. Hooks, Skills, and Commands Create a Platform

Claude Code analyses report hook stages around tool use, prompt submit, and
session lifecycle. Codex has skills, hooks, rules, plugins, MCP, slash commands,
non-interactive mode, SDK, app server, GitHub Actions, and team-shared config.
Claw Code advertises commands, MCP, plugins, and provider-agnostic clients.

The excellence criterion:

- common workflows become named commands
- domain expertise becomes skills
- policy and automation live outside core code
- hooks can block, warn, or enrich execution
- extension points are auditable and permission-scoped

**Modeltap gap:** FEAT-0012 exists, MCP exists, and slash commands are implied
by FEAT-0009/0014, but modeltap lacks a coherent extension contract that ties
skills, hooks, MCP, commands, permissions, and prompt layering together.

### 6. Security Is Product Behavior, Not a Disclaimer

Claude Code's leaked architecture is reportedly heavy on Bash validation,
prompt-injection handling, anti-distillation mechanics, and remote kill-switch
style controls. Codex documents configurable sandboxing, project/team rules,
and permission prompts for elevated operations.

For modeltap, this category is even more important because the BFF captures
traffic by design.

The excellence criterion:

- local execution is least-privilege by default
- dangerous commands are detected structurally
- secrets and sensitive files have policy-aware handling
- captured prompts can be redacted, encrypted, or hardened
- enterprise admins can set policy without breaking solo workflows
- every elevated operation is attributable

**Modeltap gap:** EXP-0009 names traffic hardening, but no accepted feature or
ADR defines capture hardening, fake-tool/anti-distillation strategy, sandboxing,
or enterprise policy inheritance for harness tool execution.

### 7. The UI Supervises Work, Not Just Chat

Excellent harness UX exposes the agent's operating state: current model,
context budget, mode, cost, running tools, approvals, queue, diffs, tests,
subagents, and recovery options. Codex app goes beyond chat with multi-agent
thread supervision, diff review, worktree checkout, automations, and skills.

The excellence criterion:

- progress is visible without verbose chatter
- approvals are fast and specific
- diffs and test evidence are reviewable in context
- background work has a queue and notification path
- failures become recoverable checkpoints

**Modeltap gap:** FEAT-0014 gives modeltap a good conversation-shell direction,
but the product is still missing the higher-level supervision surfaces:
multi-run dashboard, diff/test evidence panes, task queue, command history,
agent/team status, and run recovery.

## Comparative Signal

| Dimension | Claude Code / clones | Codex | modeltap today |
|---|---|---|---|
| Project rules | `CLAUDE.md` every turn; memory files | `AGENTS.md`, rules, config | AGENTS present for agents; no product `MODELTAP.md` contract |
| Tool runtime | mature tool loop, deep command checks | local reads/edits/commands, sandbox approvals | tools + MCP + permission enforcer, less policy depth |
| Parallelism | subagents, worktree/team patterns reported | app manages parallel agents and worktrees | useful only if scoped to review/validation |
| Memory | persistent session/project/user/reference memory reported | memories, skills, shared config | BFF-capable, not surfaced |
| Extensibility | hooks, slash commands, MCP reported | hooks, skills, plugins, MCP, SDK | MCP mostly; skills/hooks incomplete |
| Security | command checks, injection handling, anti-distillation reported | sandboxing, rules, approvals | permission gates; traffic hardening unpromoted |
| Product surface | terminal-first coding harness | CLI + app + IDE + cloud | terminal shell + BFF architecture |
| Strategic edge | polished single-provider coding agent | multi-surface agent platform | proxy-native cross-model control and capture |

## Modeltap's Advantage

Modeltap should not chase Claude Code by becoming a self-contained agent loop.
Its advantage is different:

1. **Cross-model control:** the BFF can route subtasks by cost, latency,
   capability, and policy.
2. **Central capture:** every turn and tool result can become durable
   institutional memory.
3. **Team deployment path:** the same architecture can support solo, team, and
   enterprise profiles.
4. **Provider neutrality:** skills, memory, and orchestration can outlive any
   single model vendor.

The gap is not architectural direction. The gap is product completeness around
the harness runtime.

## Priority Gap List

1. **Managed turn pipeline:** define the preflight, execution, postflight, and
   recovery stages for every call, including which stages are BFF-owned and
   which are harness-owned.
2. **Codegen context planner:** add repo-map, symbol/test discovery, local
   style sampling, and visible context provenance for implementation turns.
3. **Patch quality loop:** add diff-quality checks, unrelated-change detection,
   validation evidence, and repair-attempt memory.
4. **Project context contract:** define `MODELTAP.md` / `.modeltap/` discovery,
   scope, precedence, and prompt-layer inspection.
5. **Durable memory:** implement session, project, and global memory digests
   through the BFF; expose active memory in the harness.
6. **Policy-grade permissions:** add command/path/domain policy, audited
   approvals, and workspace sandbox profiles.
7. **Run artifacts:** persist diffs, commands, test logs, costs, approvals, and
   failure checkpoints as first-class session artifacts.
8. **Quality-driven routing:** define when to use stronger models, cheap helper
   calls, validation repair calls, or candidate patch comparison.
9. **Hook and skill runtime:** unify skills, hooks, MCP, slash commands, and
   prompt overlays into one extension model where they improve code quality.
10. **Traffic hardening:** promote fake-tool/anti-distillation and encrypted or
   redacted capture modes into a security feature or ADR.
11. **Codegen evaluation harness:** add repeatable harness-quality tests:
   patch correctness, test pass rate, diff minimality, style fit, regression
   avoidance, context-selection quality, and repair-loop success.

## Stack-Ranked Upgrade Assessment

This section ranks concrete BFF and harness changes by expected impact on code
generation quality. It is intentionally not a feature decomposition yet. The
right packaging may be one ADR plus several features, or one feature with
multiple work streams.

Ranking criteria:

- **Quality impact:** how directly the change improves generated code
  correctness, fit, or regression avoidance.
- **Foundation value:** how many later improvements depend on it.
- **Implementation risk:** whether the change can ship incrementally without
  destabilizing the current v0.2.1/v0.2.2 shell/runtime boundary.

### 1. Managed Codegen Turn Pipeline

**Assessment:** highest impact and highest foundation value. Without this,
other improvements remain ad hoc.

**BFF changes:**

- introduce a first-class turn lifecycle for implementation work:
  `preflight -> context_plan -> prompt_plan -> model_call -> tool_loop ->
  artifact_capture -> memory_candidate -> checkpoint`
- store the lifecycle stages as structured run metadata
- emit stage/progress events over the harness protocol
- attach model, prompt-layer, tool, cost, validation, and artifact metadata to
  the run

**Harness changes:**

- render the active run stage without making the transcript noisy
- expose context/prompt/policy summaries on demand
- preserve interrupt/retry/continue/fork affordances against the run id
- keep FEAT-0014's shell boundary intact by treating pipeline updates as host
  events

**Why it improves codegen:** the model stops receiving an under-specified chat
turn and starts operating inside a durable implementation transaction.

**Likely artifact:** ADR or FEAT. ADR if it defines the cross-cutting BFF vs
harness ownership contract; FEAT if it primarily specifies user-visible run
behavior.

### 2. Codegen Context Planner

**Assessment:** the most direct quality upgrade after the pipeline. Better code
mostly starts with better context.

**BFF changes:**

- accept a structured `context_plan` for every implementation turn
- include context provenance in prompt assembly and capture
- support prompt budget accounting by context category
- optionally summarize or reject oversized context before the provider call

**Harness changes:**

- build a lightweight repo map from files, packages, imports, tests, and recent
  changes
- resolve user-mentioned paths plus inferred neighboring files/tests
- sample local style from nearby code
- send selected files/snippets plus provenance to the BFF
- add `/context` detail for "why this file/rule is active"

**Why it improves codegen:** the model sees the right code and conventions
before editing, reducing hallucinated APIs, style drift, and missed tests.

**Likely artifact:** FEAT. This is behavior-scoped and user-visible through
context inspection.

### 3. Validation Feedback Loop

**Assessment:** direct correctness impact. It turns generated code into
validated code and prevents repeated failed repairs.

**BFF changes:**

- model validation as run artifacts: command, scope, exit status, summarized
  failure, files implicated, and timestamp
- feed validation summaries into repair turns
- retain failed repair attempts so the model can avoid loops
- expose validation evidence in session details

**Harness changes:**

- infer likely checks from repo structure and changed files
- run targeted checks first, broad checks later
- capture stdout/stderr in structured envelopes
- present validation plan/evidence compactly in the transcript
- allow user approval for expensive or risky checks

**Why it improves codegen:** compiler/test/lint output becomes a structured
repair signal instead of raw terminal text pasted back into the conversation.

**Likely artifact:** FEAT, possibly bundled with context planner if scoped as
"codegen quality loop."

### 4. Patch Quality and Diff Discipline

**Assessment:** high impact for reducing churn and preserving user trust. This
is where the harness can outperform generic model behavior without more
model calls.

**BFF changes:**

- store patch artifacts and patch summaries per run
- correlate diffs with user request, tool calls, validation, and approvals
- expose patch artifact IDs for replay/review

**Harness changes:**

- compute pre/post diff for every run that mutates files
- flag unrelated changes, broad formatting churn, file deletions, and edits to
  files not read in the run
- summarize patch shape before final response
- optionally require approval for large or suspicious diffs even in permissive
  modes

**Why it improves codegen:** the system can detect "works but sloppy" patches:
too broad, unrelated, poorly scoped, or inconsistent with the requested change.

**Likely artifact:** PATCH for an initial local diff-quality checker; FEAT once
run artifacts and user-visible review behavior are specified.

### 5. Project Rules and Codegen Prompt Contract

**Assessment:** high quality impact, but best after the managed pipeline names
where prompt layers live.

**BFF changes:**

- define prompt-layer order for implementation turns
- version the base codegen prompt contract
- assemble project rules, repo context, memory digest, tool descriptions, mode,
  and validation instructions deterministically
- capture prompt-layer metadata without necessarily exposing full secrets or
  proprietary prompt content

**Harness changes:**

- discover and transmit project-rule files (`MODELTAP.md`, `AGENTS.md`,
  `CLAUDE.md`, and configured alternatives)
- show active rule sources and precedence
- warn when rule files conflict or exceed budget

**Why it improves codegen:** consistent instructions reduce avoidable failures:
unread files, invented abstractions, ignored tests, unexplained validation, and
unrelated churn.

**Likely artifact:** ADR for precedence and prompt composition, plus FEAT for
user-visible project-rule handling.

### 6. Run Artifact Bundle

**Assessment:** strong foundation and moderate direct quality impact. It makes
later context, memory, replay, and evaluation work much easier.

**BFF changes:**

- persist a run bundle containing context plan, prompt plan, tool calls, patch,
  command logs, validation evidence, approvals, model/cost, and final outcome
- expose run bundles through `session.details` or a new run artifact endpoint

**Harness changes:**

- render compact artifact tokens in the transcript
- provide preview/open affordances for patch, validation, and context artifacts
- attach artifact IDs to final summaries

**Why it improves codegen:** it gives the next model call a reliable record of
what happened and gives the user evidence instead of prose-only reassurance.

**Likely artifact:** FEAT. It affects storage, protocol, and UX.

### 7. Codegen Quality Evaluation Suite

**Assessment:** high leverage, indirect user impact. This should start early
even if it ships as internal infrastructure.

**BFF changes:**

- record enough structured run data to score outcomes
- support replay/evaluation fixtures without provider-specific assumptions

**Harness changes:**

- add scripted scenarios that exercise context selection, edits, validation,
  permission prompts, interruption, and recovery
- measure diff minimality, changed-file relevance, test pass rate, repair-loop
  success, and final-answer evidence quality

**Why it improves codegen:** without measurement, modeltap cannot tell whether
a harness change improves code generation or only adds UI surface area.

**Likely artifact:** PATCH first, then possibly a release-quality gate.

### 8. Quality-Driven Routing

**Assessment:** important to modeltap's strategic edge, but depends on run data
and quality metrics. Do not lead with this.

**BFF changes:**

- add codegen routing roles: context helper, implementation, repair,
  validation-summary, review
- route by task risk, changed-file scope, validation state, and cost budget
- record routing decisions and outcomes for later tuning

**Harness changes:**

- expose selected model/routing reason per run stage
- allow user override for high-risk implementation turns

**Why it improves codegen:** stronger models can be reserved for the parts of
the loop where they matter most, while cheaper calls handle retrieval,
summaries, and failure triage.

**Likely artifact:** FEAT or ADR after the evaluation suite can measure
quality/cost tradeoffs.

### 9. Policy-Grade Tool Permissions

**Assessment:** important for trust and enterprise readiness, but its codegen
impact is secondary unless scoped to patch quality and validation.

**BFF changes:**

- accept server/team policy metadata during prompt planning
- include policy constraints in prompt-layer assembly
- capture policy decisions for audit

**Harness changes:**

- add path/command/domain policy rules
- classify commands and file operations dynamically
- show permission decisions grouped by run
- support workspace sandbox profiles for execution tools

**Why it improves codegen:** it makes safe validation and edits more automatic
while preventing risky operations from derailing a run.

**Likely artifact:** FEAT. This is broad enough to avoid treating it as a small
patch.

### 10. Memory From Successful Work

**Assessment:** valuable but should follow artifact capture. Memory without
high-quality source artifacts risks preserving noisy or wrong conclusions.

**BFF changes:**

- promote completed run summaries into project memory candidates
- separate durable decisions from ephemeral debugging traces
- retrieve memory by project, package, file, and task type

**Harness changes:**

- let users inspect, accept, edit, or reject memory candidates
- show which memory items influenced a turn

**Why it improves codegen:** future calls inherit decisions and local
conventions without requiring the user to restate them.

**Likely artifact:** FEAT, likely downstream of run artifact bundles.

### Recommended Sequencing

The recommended first tranche is:

1. **Managed codegen turn pipeline**
2. **Codegen context planner**
3. **Validation feedback loop**
4. **Patch quality and diff discipline**
5. **Codegen quality evaluation suite**

This tranche should be enough to make basic implementation turns measurably
better without depending on broad agentic features. Project rules, run
artifacts, and quality routing should follow closely because they solidify the
same loop, but the first tranche gives the team a concrete quality target.

### Suggested Artifact Split

This likely should not become one large feature. A practical split:

1. **ADR: Managed Codegen Turn Pipeline**
   - Defines BFF/harness ownership, lifecycle stages, artifact categories, and
     prompt-layer boundaries.
2. **FEAT: Codegen Context and Validation Loop**
   - User-visible context planning, validation planning, validation evidence,
     and repair-loop behavior.
3. **PATCH: Patch Quality and Evaluation Harness**
   - Internal diff-quality checks, scripted benchmarks, and evidence scoring.
4. **FEAT: Run Artifacts and Project Memory**
   - Durable run bundles, session replay details, and memory promotion.

The first three are the minimum useful package if the goal is immediate code
generation quality rather than broad harness feature parity.

## Promotion Candidates

This exploration should likely promote into several downstream artifacts rather
than one large feature:

- **ADR:** prompt/context composition and project-rules precedence.
- **ADR or FEAT:** managed turn pipeline and ownership split between BFF and
  harness.
- **FEAT:** codegen context planner and validation-feedback loop.
- **PATCH:** patch-quality checks and codegen evaluation suite.
- **FEAT:** persistent project memory and active-context inspection.
- **FEAT:** harness extension platform: skills, hooks, slash commands, MCP
  policy integration where it improves implementation quality.
- **FEAT or ADR:** capture hardening and anti-distillation posture.

## Open Questions

1. Should `MODELTAP.md` intentionally coexist with `AGENTS.md` and
   `CLAUDE.md`, or should modeltap ingest all three with explicit precedence?
2. Should policy live in project config, server-side team config, or both?
3. Are local tool sandboxes a harness concern, a BFF concern, or a shared
   contract where the BFF describes policy and the harness enforces it?
4. What codegen checks should modeltap run automatically by language/project,
   and which should require user approval?
5. Should modeltap compare multiple candidate patches for high-risk edits, or
   keep one implementation path plus one reviewer pass?
6. What captured data is safe to store by default in solo, team, and enterprise
   deployments?

## Proposed Next Step

Use this exploration as the gap map for the next harness-planning session.
Before opening another implementation release, promote the top three gaps into
formal artifacts:

1. managed turn pipeline
2. codegen context planner + validation feedback loop
3. patch-quality checks + codegen evaluation suite

Those foundations make later work, including skills, hooks, subagents,
worktrees, traffic hardening, and broader orchestration, serve the primary
goal: higher-quality generated code.
