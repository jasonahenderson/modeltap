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

- Canonical per-doc findings keep the single filename `{stem}-findings.md`. Dispositions live in a table at the bottom of that file; no sidecar JSON.
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
**Outputs:** Design document in `docs/releases/<version>/designs/<date>-design-<component>.md`

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

1. **Phases are release-level, not WU-level.** Phase 1 → Phase 2 → Phase 3, strict order, no interleaving.
2. **Phase 1 = design ALL WUs across ALL tracks.** No coding. No reviews. Just design docs (with optional pre-review lint). Phase 1 is not complete until every track (0, A, B, Integration) has design docs for every WU. Completing one track's designs does not authorize advancing to Phase 2 or 3.
3. **Phase 2 = review.** User decides what to review and how. No new designs. No coding. Phase 2 begins only after the user confirms Phase 1 is complete.
4. **Phase 3 = implement ALL WUs.** No new designs. If implementation reveals a design flaw, revise the design doc explicitly — don't silently improvise. Phase 3 begins only after Phase 2 findings are processed.
5. **Current phase lives in `docs/releases/<version>/plan.md`.** Any action outside the current phase is wrong. Phase transitions are explicit ADMIN commits — never implicit.

If any instruction elsewhere contradicts these, the prime directives win.

### Why release-level phases

Phase boundaries are at the release level, not the WU level — all designs complete before any coding begins. This is a deliberate change from the per-WU end-to-end flow: contract-heavy releases benefit from seeing the complete design surface before fossilizing it in implementation. Catching cross-WU drift costs minutes at the design-doc layer and hours-per-WU once code is written.

```
===============================================================
PHASE 1 — Design (all WUs in the release)
===============================================================

For each WU (or bundle of related WUs):
    TPM assigns task
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

### Release tags

Every shipped release gets one annotated Git tag named `vX.Y.Z`. The tag points
at the final release commit after status, changelog, release-readiness review,
and final notes are committed.

Before publication, the tag may move when final commits are added. Recreate it
with `git tag -f -a vX.Y.Z -m "modeltap vX.Y.Z" <new-release-commit>` and, if it
was already pushed, update it with
`git push --force-with-lease origin refs/tags/vX.Y.Z`. Log the old SHA, new SHA,
and reason in `docs/history/`.

After publication, tags are immutable by default. New commits ship as a new
version, usually the next patch release, unless a maintainer explicitly approves
a correction and documents it.

### Design Review

Design errors are cheapest to fix before code cements them. Phase 1 produces the designs; Phase 2 reviews them.

#### Phase 2 review — user's call

The user decides what to review, how to review it, and when it's sufficient:

- Read and approve designs directly
- Send designs to an external model (Codex, Kimi, GPT-5, Gemini, etc.)
- Both
- Skip Phase 2 entirely

There is no tiering system. There are no mandatory review gates. The user owns the risk judgment.

Review artifacts committed at `docs/releases/<version>/.reviews/<wu-or-bundle>/<reviewer>-review.md` using the reviewer-first naming convention.

#### Pre-review lint (optional designer tool)

A **pre-review lint** is a Claude subagent with fresh context that reads a design doc against source specs, checking for mechanical drift, scope gaps, missing fields, and undocumented assumptions. The Designer decides when it's worth running — it is not mandatory.

The lint catches spec-drift and mechanical issues. It does **not** catch reasoning blind spots specific to the Designer's model family — only a different model or human reviewer can do that.

Artifact: `docs/releases/<version>/.reviews/<wu-or-bundle>/claude-subagent-pre-review.md`.

#### Finding severity (for any review)

- **Blocking** — must resolve before Phase 3 implementation.
- **Attention** — should address unless documented reason not to.
- **Nit** — optional.

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
