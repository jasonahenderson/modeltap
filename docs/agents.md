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

Each work unit follows this pipeline:

```
TPM assigns task
    |
    v
Design Engineer -> design doc
    |
    v
Test Engineer -> failing unit tests
    |                                  COMMIT: tests (red phase)
    +--------------------------+
    |                          |
    v                          v
Backend Implementer      UI Implementer
    |                          |
    +--------------------------+
    |                                  COMMIT: implementation (green phase)
    v
Integration Tester -> end-to-end tests
    |
    v
Security Reviewer -> review (pass/fail)
    |                    |
    | (pass)             | (fail -> Implementer fixes -> re-review)
    |                                  COMMIT: security fixes (if any)
    v
Documentation Specialist -> docs
    |
    v
Infrastructure Engineer -> CI/build updates (if needed)
    |                                  COMMIT: work unit complete
    v
TPM logs completion, updates status
```

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
