# Agent Team

This document defines the agent team responsible for designing, building, testing, and shipping modeltap. Each agent has a specific role, clear inputs/outputs, and operates on small, independently completable work units.

## Principles

1. **Incremental work units** — Work is broken into tasks small enough to complete in a single session. No task should require multi-session continuity to be useful.
2. **Status-based resumption** — A living status file tracks what's done, in progress, and remaining. Any new session can read it and continue.
3. **ADR/Feature driven** — Only accepted ADRs and accepted features drive work. Other statuses are ignored.
4. **History logging** — Every action is logged to `docs/history/` with a plan before starting and a summary after completion.

## Artifact Boundaries

- `docs/explorations/` holds upstream problem framing and design-space exploration. Explorations can promote into features, patches, or ADRs, but do not authorize implementation by themselves.
- `docs/features/` holds behavior-scoped work that can drive `WU-NNN` implementation once accepted.
- `docs/patches/` holds implementation-scoped work authorization for fixes, tooling, infra, and internal plumbing.
- `docs/adr/` holds architectural decisions with future constraint value.
- `ADMIN:` work covers repo process and instruction changes such as `CLAUDE.md`, `AGENTS.md`, prompts, hooks, or documentation structure.

## Review Artifact Naming

- Canonical per-doc findings keep their stable names: `{stem}-findings.md` and `{stem}-findings.json`.
- Non-canonical work-plan reviews should include the reviewing model or harness name in the filename when known.
- Prefer reviewer-first names for those plan-review artifacts, for example:
  - `codex-plan-review.md`
  - `codex-0008-bff-server-connectivity-review.md`
  - `gpt5-0001-openai-responses-api-support-plan-review.md`

## Agents

### TPM (Technical Program Manager)

**Role:** Coordinates the team. Reads accepted ADRs and features, breaks work into incremental tasks, assigns to agents, tracks progress.

**Responsibilities:**
- Read accepted ADRs (`docs/adr/`) and features (`docs/features/`) to determine project scope
- Break scope into ordered, independently completable work units
- Write and maintain the plan in the current release directory (`docs/releases/<version>/plan.md` and per-track files)
- Update the status file (`docs/releases/<version>/status.md`) after each unit completes
- Determine what to do next when resuming from a prior session
- Ensure agents work in the right order (design before implementation, tests before code, security review after code, docs after all)

**Inputs:** Accepted ADRs, accepted features, current status file
**Outputs:** Plan, task assignments, status updates, session summaries

### Design Engineer

**Role:** Produces detailed technical designs for each work unit before implementation begins.

**Responsibilities:**
- Translate ADR decisions and feature requirements into concrete implementation designs
- Define package structure, interfaces, type definitions, and data flow
- Specify function signatures, error handling patterns, and edge cases
- Identify dependencies between components
- Write design docs that are specific enough for the Implementation Engineer to code from

**Inputs:** Accepted ADRs, feature docs, TPM task assignment
**Outputs:** Design document in `docs/history/<timestamp>-design-<component>.md`

### Test Engineer

**Role:** Writes tests before implementation (TDD). Defines what "done" looks like for each work unit.

**Responsibilities:**
- Read the design document for the current work unit
- Write unit tests, integration tests, and table-driven tests (Go convention)
- Define test fixtures and mock interfaces
- Tests should compile but fail (red phase) until Implementation Engineer writes the code
- Validate tests pass after implementation (green phase)

**Inputs:** Design document for the work unit
**Outputs:** Test files in the appropriate package, logged to history

### Backend Implementer

**Role:** Writes the Go production code to make tests pass. Owns proxy, storage, CLI, and provider code.

**Responsibilities:**
- Read the design document and test files for the current work unit
- Write the minimal code needed to pass all tests
- Follow Go conventions: `gofmt`, `go vet`, effective Go idioms
- Respect ADR decisions (e.g., use Cobra for CLI, Viper for config, SQLite for storage)
- Do not add features beyond what the design specifies
- Owns: proxy core, middleware, storage layer, CLI commands, provider adapters, metrics, config

**Inputs:** Design document, failing test files
**Outputs:** Production Go code files, logged to history

### UI Implementer

**Role:** Builds the web dashboard frontend. Owns all HTML, CSS, JS, and embedded asset code.

**Responsibilities:**
- Read the design document and test files for the current work unit
- Build UI components for the web dashboard (log viewer, metrics display, proxy status)
- Use lightweight frontend approach (htmx, alpine.js, or vanilla JS — no heavy frameworks)
- Implement internal REST API handlers in Go that serve dashboard data
- Embed static assets via Go `embed.FS` so the binary stays self-contained
- Ensure responsive design and accessible markup
- Respect user isolation when multi-user is enabled (users see only their data)

**Inputs:** Design document, feature spec (`docs/features/0003-web-dashboard.md`), failing test files
**Outputs:** Frontend assets, Go API handlers, logged to history

### Infrastructure Engineer

**Role:** Owns build system, CI/CD, release pipeline, and development tooling.

**Responsibilities:**
- Set up Go module, Makefile, and build targets
- Configure goreleaser for cross-platform binary distribution
- Set up GitHub Actions for CI (lint, test, build)
- Create Dockerfile if applicable
- Manage development dependencies and tooling (linters, formatters)
- Set up pre-commit hooks (DCO sign-off check, `go vet`, `gofmt`)

**Inputs:** TPM task assignment, project conventions
**Outputs:** Build/CI configuration files, logged to history

### Integration Tester

**Role:** Writes end-to-end tests that verify components work together correctly.

**Responsibilities:**
- Write integration tests that span multiple components (e.g., proxy -> storage -> API -> UI)
- Test real HTTP flows through the proxy with mock upstream servers
- Verify dashboard API endpoints return correct data from SQLite
- Test multi-provider routing and SSE stream capture end-to-end
- Validate CLI commands produce expected output against a running instance
- Run after Backend and UI Implementers complete their units

**Inputs:** Completed code from Backend and/or UI Implementers
**Outputs:** Integration test files, test results, logged to history

### Security Reviewer

**Role:** Reviews completed code for security vulnerabilities before it's considered done.

**Responsibilities:**
- Review for OWASP top 10 vulnerabilities relevant to the component
- Check for SQL injection (especially with SQLite), command injection, path traversal
- Verify proper input validation at system boundaries
- Check that secrets/credentials are not logged or exposed
- Review error messages for information leakage
- Review dashboard for XSS, CSRF, and authentication bypass
- Flag issues with specific file:line references and severity

**Inputs:** Production code from Backend/UI Implementers
**Outputs:** Security review document in `docs/history/<timestamp>-security-<component>.md`, with pass/fail and any required fixes

### Documentation Specialist

**Role:** Writes user-facing and developer-facing documentation for completed components.

**Responsibilities:**
- Write usage documentation for completed CLI commands
- Write developer documentation for internal packages (godoc-style)
- Update README as features are completed
- Ensure code comments exist where logic isn't self-evident
- Do not document unfinished or planned features

**Inputs:** Completed and security-reviewed code
**Outputs:** Documentation files, README updates, logged to history

## Workflow

### Prime directives — DO NOT VIOLATE

1. **Phases are release-level, not WU-level.** A release moves through Phase 1 → Phase 2 → Phase 3 in strict order. You do not interleave them.
2. **Phase 1 is design for ALL WUs in the release.** Finish every design doc (with subagent pre-review lint on Tier B and Tier C) before Phase 2 begins. No coding in Phase 1. No peer reviews in Phase 1.
3. **Phase 2 is one batched peer-review pass.** The user flags which designs need external peer review; the batch runs in one session. No new designs, no coding in Phase 2. Phase 2 is skippable if the user decides pre-review lint coverage is sufficient.
4. **Phase 3 is implementation for ALL WUs.** Red → green → security → docs per WU, in any dependency-legal order. No new design work in Phase 3 — if implementation reveals a design flaw, file a patch or revise the design doc explicitly, don't silently improvise.
5. **The current phase lives in `docs/releases/<version>/plan.md` §"Phased Execution".** Any action that does not match the current phase is wrong. If you are asked to code during Phase 1, STOP and check the phase.
6. **Pre-review lint is NOT Tier C.** A Claude subagent with fresh context catches mechanical drift; it does not catch Claude-family reasoning blind spots. Tier C is peer-model (different model family) review, user-driven, batched in Phase 2. A subagent never satisfies Tier C.
7. **Peer-review handoff is chat-only — no committed prompt file.** The Designer announces the request in chat (WU/bundle ID + file paths); the user chooses their external model and frames the review themselves. Prescriptive prompts bias the outcome.

If any instruction anywhere else in this document or the codebase contradicts these prime directives, the prime directives win. File an ADMIN commit to reconcile.

### Why release-level phases

Phase boundaries are at the release level, not the WU level — all designs complete before any coding begins. This is a deliberate change from the per-WU end-to-end flow: contract-heavy releases benefit from seeing the complete design surface before fossilizing it in implementation. Catching cross-WU drift costs minutes at the design-doc layer and hours-per-WU once code is written.

```
===============================================================
PHASE 1 — Design (all WUs in the release)
===============================================================

For each WU (or bundle of related WUs):
    TPM assigns task (with default review tier)
        |
        v
    Designer produces design doc
    (Bundle related WUs into one design doc where they share a
     contract surface — see "Bundled designs" below.)
        |
        v
    Subagent pre-review lint (recommended for B and C)
    Claude subagent with fresh context reviews the design against
    source specs; Designer triages findings, resolves cheap ones.
        |
        v
    COMMIT: design doc + lint artifact

After ALL designs are complete, Phase 1 ends.

===============================================================
PHASE 2 — Peer review (opt-in, batched)
===============================================================

User identifies which WUs (or bundles) warrant external peer-model
review beyond the subagent lint. Typically a minority: ADR-level
decisions, externally-facing contracts, security-critical surfaces.

Designer announces the flagged items in chat — WU or bundle
identifier, plus paths to the design doc, pre-review artifact, and
relevant source specs. No prompt file is produced: the external
reviewer is trusted to decide their own framing, and over-prescribing
the review biases the outcome.

User runs the reviews through their chosen external model in one
session. Results committed at
`.reviews/<wu-or-bundle>/<reviewer>-design-review.md` using the
reviewer-first naming convention.

Designer processes findings across the batch; design docs revised
as needed.

    COMMIT: peer-review artifacts + design revisions

Phase 2 may be skipped entirely for a release if the user decides
the subagent lint is sufficient coverage.

===============================================================
PHASE 3 — Implementation (all WUs)
===============================================================

With all designs stable, implementation proceeds in any
dependency-legal order. Per WU:

    Test Engineer -> failing unit tests
        |                              COMMIT: tests (red phase)
        +--------------------------+
        |                          |
        v                          v
    Backend Implementer      UI Implementer
        |                          |
        +--------------------------+
        |                              COMMIT: implementation (green)
        v
    Integration Tester -> end-to-end tests
        |
        v
    Security Reviewer -> review (pass/fail)
        |                    |
        | (pass)             | (fail -> Implementer fixes -> re-review)
        |                              COMMIT: security fixes (if any)
        v
    Documentation Specialist -> docs
        |
        v
    Infrastructure Engineer -> CI/build updates (if needed)
        |                              COMMIT: work unit complete
        v
    TPM logs completion, updates status

WUs may parallelize within Phase 3 subject to their dependency
graph.
```

### Bundled designs

Where multiple WUs share a contract surface, produce ONE design doc covering them all. The bundle's title lists the WU range (e.g., `design-protocol-types-040-041-093.md`). Each constituent WU's status line references the bundle doc.

Bundle when WUs:
- Share a Go package or file set
- Share a protocol surface (message catalog, interface signature)
- Are implementations of one conceptual subsystem where separating designs would force duplication

Do NOT bundle when WUs have independent blast radius or when a design flaw in one wouldn't invalidate the others.

### Phase selection per release

A release's `plan.md` records which phase the release is in. Status updates move WUs within the current phase, and phase transitions are ADMIN commits that mark the boundary.

### Design Review

Design errors are cheapest to fix before tests cement them. Every WU receives a design review during Phase 1, before Phase 3 implementation begins. Depth scales with the WU's risk tier.

#### Tier assignment

The TPM assigns a **default tier** when writing the WU spec in the track file (see `docs/releases/<version>/track-*.md`). The Designer re-evaluates against the rules below after completing the design doc and records the **final tier** at the top of the doc.

Designers may **escalate** (A → B → C) with a recorded reason. Downgrades are not permitted — if a Designer thinks the default is too conservative, they ask the TPM to change the rules rather than skipping review on a specific WU.

#### Tier rules

Apply top-down. First matching rule wins. A WU is Tier C if **any** C rule fires; otherwise Tier B if any B rule fires; otherwise Tier A.

**Tier C — external peer review required** if any of:
- Creates or modifies files in `internal/protocol/`, `internal/bff/{transport,server,connection,capabilities}.go`, or any other package imported by both Track A and Track B.
- Creates or modifies a protocol message type, a shared Go interface, a stable on-disk schema, or an ADR.
- Touches credentials, tokens, TLS config, model-supplied file-path input, shell invocation, network listeners, permission policy, or tool execution.
- Appears as a dependency of another track beyond Track 0 (cross-track surface).

**Tier B — user review required** if any of (and not already C):
- Size = L per `plan.md`.
- Creates ≥3 new `.go` files.
- Modifies `internal/config/config.go` or adds new config keys.
- Adds a new Cobra command.
- Defines UI component layout or key bindings.

**Tier A — self-checklist only** otherwise.

#### Design doc header

Every design doc begins with:

```markdown
## Review Tier
**Assigned:** <A | B | C>
**Basis:** <which rule(s) fired>
**Plan default:** <tier from WU spec>
**Escalation reason:** <required only if assigned > plan default>
```

#### Tier A — self-checklist

The Designer confirms in the design doc that each item holds. A failing item is a blocker until fixed.

1. Every input (ADR, feature spec, dependency WU) is referenced and its relevant constraints captured.
2. Every field from the source spec is enumerated; required/optional labels match the source.
3. Cross-WU types (names, shapes) are consistent with the WUs that produce or consume them.
4. Package and naming conventions (`gofmt`, effective Go, snake_case on the wire) are followed.
5. At least one "what could go wrong" case is listed with mitigation.
6. Scope boundaries are explicit: what is in, what is deferred.

#### Tier B — user review

Designer publishes the design doc and pauses. The user reads, asks questions, and approves or requests changes before the Test Engineer proceeds. Approval is recorded in the design doc or in the commit message that promotes the WU to tests.

#### Tier C — peer-model review (opt-in, batched in Phase 2)

Tier C marks a WU as **eligible for peer-model review**. It does not automatically trigger one. The user decides in Phase 2 which Tier-C WUs (or bundles) warrant external peer review; the rest proceed on the strength of the subagent pre-review lint alone.

Calibration rationale: 33 Tier-C WUs across v0.2.0 would require 33 user-mediated external-model sessions if every one triggered peer review. That is not practical. The tag identifies elevated risk; the Phase 2 opt-in preserves the review capability for the highest-stakes designs (typically 3-8 bundles per release).

A peer-model reviewer uses a **different model family from the Designer**. This is definitional: Tier C exists to catch reasoning blind spots that a same-model reviewer cannot detect. A fresh-context instance of the Designer's own model shares the same training distribution and will miss the same patterns.

Tier-C reviewer options:

1. **External LLM via user-mediated submission** — Codex, Kimi, GPT-5, Gemini, or any other non-Claude model. Designer announces the review request in chat (WU/bundle identifier + file paths); the user supplies whatever framing they prefer to the external model; the resulting artifact is committed. No committed prompt file — a trusted external reviewer decides their own approach, and prescriptive prompts bias the outcome. This matches how plan reviews already work (`.reviews/codex-plan-review.md`, `.reviews/kimi-plan-review.md`).
2. **Human maintainer** using a different model from the Designer's, or reasoning without model assistance.

A Claude subagent with fresh context is **not** a Tier-C option. See "Pre-review lint" below — subagent reviews are Phase 1 work, run on every Tier-C WU as part of design.

Until Modeltap itself supports autonomous cross-model routing (FEAT-0008 + FEAT-0013 `review_*` roles), Tier-C peer reviews are user-driven and batched in Phase 2.

**Bundled reviews:** multiple related WUs sharing a contract surface should go through a single peer review to economize reviewer effort. The review artifact covers all WUs reviewed; each WU's design doc links to it. v0.2.0 bundle candidates:
- Protocol types: WU-040 + WU-041 + WU-093
- Provider formatting: WU-042 + WU-043 + WU-044
- Storage: WU-045 + WU-091 + WU-096
- Connection: WU-046 + WU-047 + WU-048 + WU-049
- Session: WU-050 + WU-051 + WU-064
- Streaming: WU-052 + WU-053 + WU-060
- Routing: WU-057 + WU-058 + WU-059
- Tools: WU-076 + WU-077 + WU-078 + WU-079

Aggressive bundling collapses 33 Tier-C WUs to ~10-12 peer reviews across v0.2.0.

#### Pre-review lint (Phase 1, standard on Tier B and C)

A **pre-review lint** is a Claude subagent with fresh context that reads the design doc and (if applicable) source specs for drift, scope gaps, missing fields, and undocumented assumptions. It runs as part of Phase 1 design, after the Designer produces the doc and before the WU's Phase 1 design is considered complete.

The lint is **not a tier**. It runs on every Tier-B and Tier-C design as the default coverage. It does not satisfy Tier C peer review — it catches mechanical drift, not Claude-family reasoning blind spots. For WUs the user opts into Phase 2 peer review, the lint artifact is referenced in the peer-review prompt so the peer reviewer doesn't waste attention rediscovering lint findings.

Artifact: `docs/releases/<version>/.reviews/<wu-or-bundle>/claude-subagent-pre-review.md`. The Designer triages the lint's findings, resolves Blocking items in the design doc before closing the WU's Phase 1 design, and commits the lint artifact.

Per-tier usage:
- **Tier C:** lint runs on every WU. No exception — this is the substitute for automatic external peer review.
- **Tier B:** lint runs by default; Designer may skip for trivially simple designs with rationale.
- **Tier A:** lint skipped. Tier A is a self-check by the Designer using the checklist; adding a subagent is overkill.

#### Review artifact location and naming

Path: `docs/releases/<version>/.reviews/wu-NNN/<reviewer>-design-review.md`
For bundled reviews: `docs/releases/<version>/.reviews/<wu-range-or-topic>/<reviewer>-design-review.md`.

Reviewer name prefixes follow the existing plan-review convention (`codex-`, `kimi-`, `gpt5-`, `gemini-`, `claude-subagent-`, etc.). Reviewer-first naming matches `CLAUDE.md` §"Review Artifact Naming".

#### Finding severity and handling

Reviewers bucket findings as:

- **Blocking** — must be resolved before the Test Engineer begins. Design doc is updated; commit references the finding.
- **Attention** — should be addressed unless there is a documented reason not to. Can be handled in the same WU or deferred to a named follow-up WU or patch.
- **Nit** — optional. Designer's call.

The Designer updates the design doc in response to Blocking/Attention findings and records the disposition in the review artifact (or a short reply appended to it).

### Commit Points

Commits happen at natural checkpoints within each work unit, not just at the end:
1. **Scaffolding** — new files/packages compile (`go build ./...` passes)
2. **Red phase** — tests written and failing as expected
3. **Green phase** — implementation passes all tests
4. **Security fixes** — if the security reviewer flags issues and they're fixed
5. **Work unit complete** — docs, CI updates, and status logged

Not every work unit hits all commit points. The rule is: **if you've done meaningful, compilable work, commit it before moving on.**

All commits use `git commit -s` for DCO sign-off. See `CLAUDE.md` for commit message format.

Commit prefix summary:

- `WU-NNN:` for implementation under accepted features
- `PATCH-NNNN:` for implementation under an approved patch
- `FEAT-NNNN:`, `ADR-NNNN:`, and `EXP-NNNN:` for editing the corresponding canonical docs
- `ADMIN:` for process-only changes

### Agent Selection

Not every work unit requires all agents. The TPM decides which agents are needed per task:
- Backend-only tasks skip UI Implementer
- UI-only tasks skip Backend Implementer
- Small tasks (e.g., a config change) may skip Design and go straight to Test + Implementation
- Infrastructure tasks may only need Infrastructure Engineer + TPM
- Integration Tester runs when a task spans multiple components

## Work Unit Size Guidelines

A work unit should:
- Be completable in a single agent session
- Produce a compilable, testable artifact
- Have a clear "done" definition
- Not depend on other incomplete work units (or depend only on completed ones)

Examples of good work units:
- "Implement the Provider interface and Anthropic adapter"
- "Add the `modeltap start` CLI command with basic proxy forwarding"
- "Create SQLite schema and storage layer"
- "Add request/response logging middleware"

Examples of bad work units (too large):
- "Build the entire proxy"
- "Implement all CLI commands"
- "Add the knowledge layer"

## History & Status Files

### `docs/releases/<version>/plan.md`
The release plan. Created by TPM at the start of each release cycle. Contains the ordered list of work units with their tracks and dependencies. Per-track detail files (`track-*.md`) accompany the plan.

### `docs/releases/<version>/status.md`
Living status file for the active release. Updated after every work unit completes. Structure:

```markdown
# Project Status

## Last Updated
<timestamp>

## Current Phase
<phase description>

## Completed
- [x] Work unit description (date)

## In Progress
- [ ] Work unit description — <agent currently working> — <notes>

## Up Next
- [ ] Work unit description

## Blocked
- [ ] Work unit description — <reason>
```

### `docs/history/<timestamp>-<agent>-<component>.md`
Individual work logs. Each agent writes one per task with:
- What was planned
- What was done
- Key decisions made
- Any issues encountered
- Files created or modified

### Session Resumption

When starting a new session:
1. TPM reads `docs/releases/<current-version>/status.md`
2. If a task is marked "In Progress", TPM checks if the work was actually completed (files exist, tests pass) and updates accordingly
3. TPM picks the next task from "Up Next" and assigns it
4. Work continues normally

This means any interruption loses at most one work unit of progress, and that unit can be detected and re-assigned on resume.
