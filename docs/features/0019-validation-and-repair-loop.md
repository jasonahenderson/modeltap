---
feature: FEAT-0019
title: Validation and Repair Loop
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 4
depends-on:
  - FEAT-0016: Managed Codegen Run Pipeline
related:
  - FEAT-0018: Context Planner and Project Rules
adr-constraints:
  - ADR-0014: Harness Base Strategy
---

# FEAT-0019: Validation and Repair Loop

## Problem

An implementation is not complete just because a model edited files. The system
needs evidence: compile results, tests, lint, runtime checks, or a clear reason
why validation was not run. Today validation is available only as generic shell
tool use. Failures are raw text in the transcript, and repeated repair attempts
can loop over the same failed fix.

## Solution

Add validation planning and repair-loop behavior to implementation and debug
runs. The harness discovers and executes relevant checks under policy. The BFF
stores validation evidence as run artifacts and feeds concise failure summaries
back into repair turns.

## Key Capabilities

### Validation Plan

For mutating runs, the system proposes a validation plan based on:

- changed files
- package/module structure
- detected language/toolchain
- workflow type
- prior failures
- user-requested checks

The plan should prefer targeted checks before broad checks.

### Check Execution

The harness executes checks through the normal tool/runtime policy. Checks are
structured as validation artifacts:

- command
- workspace
- start/end time
- exit status
- stdout/stderr references
- summarized failures
- implicated files/lines when detectable

Expensive or risky checks may require approval.

### Failure Summarization

The BFF turns validation output into concise repair context:

- failing test names
- compile/lint diagnostics
- file/line references
- relevant snippets
- previously attempted fixes
- whether the failure appears introduced, pre-existing, or inconclusive

### Repair Attempts

Repair turns record what was attempted and why. The model receives enough
history to avoid repeating failed fixes. The run stops or asks the user when it
hits configured repair limits.

## UI / CLI / API Integration

Expected commands:

- `/validate` runs or previews the validation plan
- `/validate plan` shows planned checks
- `/validate retry` reruns failed checks
- `/repair` continues from validation failure context

The harness displays validation results compactly in the transcript and exposes
full logs through run artifacts.

The first validation slice may run with a minimal context plan containing
changed-file metadata and language/toolchain detection. FEAT-0018 strengthens
repair quality with richer repository context and provenance, but the full
context planner is not a hard prerequisite for basic validation execution.

## Configuration

Configuration should support:

- default validation commands by language/project
- maximum repair attempts
- broad-check approval policy
- ignored known-failing checks
- validation timeout

## Non-Goals

- This feature does not define patch/diff artifact UI; see FEAT-0020.
- This feature does not require model-generated tests for every change.
- This feature does not guarantee that all project checks run automatically.

## Success Criteria

1. Mutating implementation runs produce or explicitly skip a validation plan.
2. Targeted checks are selected before broad checks when possible.
3. Validation output is stored as structured run evidence.
4. Failed validation summaries are usable as repair-turn context.
5. Repair attempts are recorded so repeated failed fixes are visible and
   avoidable.
6. Final run summaries cite validation evidence or explain why validation was
   not run.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Harness executes local checks while BFF owns orchestration and evidence |
| Future ADR | Should decide validation artifact schema and repair-loop limits |

## Open Questions

1. How should modeltap infer validation commands for unknown projects?
2. Should the user approve the validation plan before any commands run?
3. How should known pre-existing failures be marked?
4. What is the default repair-attempt limit?
