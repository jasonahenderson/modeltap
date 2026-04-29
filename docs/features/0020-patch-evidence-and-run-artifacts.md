---
feature: FEAT-0020
title: Patch Evidence and Run Artifacts
status: draft
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 5
depends-on:
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0019: Validation and Repair Loop
adr-constraints:
  - ADR-0014: Harness Base Strategy
promoted-from:
  - FEAT-0015: Professional Harness Runtime
---

# FEAT-0020: Patch Evidence and Run Artifacts

## Problem

Professional coding work needs reviewable evidence. A final assistant message is
not enough for multi-file edits, debugging fixes, release docs, or devops
changes. Users need to inspect what changed, what commands ran, what approvals
were granted, what tests proved, and what residual risk remains.

## Solution

Persist run artifacts and expose them through the harness. Mutating runs produce
patch evidence. All meaningful runs may produce context, prompt, tool,
approval, validation, cost, and output artifacts. Artifacts are compact in the
transcript and inspectable on demand.

## Key Capabilities

### Artifact Bundle

Each run may contain:

- context plan
- prompt-layer metadata
- policy/workspace summary
- tool call log
- approval log
- command log
- validation evidence
- pre/post diff
- changed-file list
- patch summary
- generated docs/specs
- cost and usage summary
- final outcome summary

### Patch Evidence

For mutating runs, the harness computes or requests:

- files added, edited, deleted, renamed
- diff summary
- suspicious churn warnings
- broad formatting warnings
- edits to files not read during the run
- unrelated local changes detected before or after the run
- generated-file or vendored-path warnings

The goal is not to block every large diff, but to make patch shape visible.

### Artifact Inspection

The transcript should show compact artifact tokens, not long raw logs by
default. Users can expand or preview artifacts:

- diff
- validation log
- command output
- approval decision
- context plan
- generated document

### Artifact Persistence

The BFF stores artifact metadata and durable references. Large local files or
logs may remain harness/executor-owned if they cannot safely be copied into BFF
storage. The artifact record must say where and how the artifact can be read.

## UI / CLI / API Integration

Expected commands:

- `/artifacts`
- `/artifacts <run-id>`
- `/diff <run-id>`
- `/evidence <run-id>`

The protocol/API should support artifact listing, artifact details, and artifact
content or preview retrieval.

## Configuration

Configuration should support:

- artifact retention
- maximum captured log size
- redaction policy
- generated-file patterns
- suspicious diff thresholds
- whether large artifacts are stored inline or by reference

## Non-Goals

- This feature does not define the initial run pipeline; see FEAT-0016.
- This feature does not define background queue behavior; see FEAT-0017.
- This feature does not replace Git as the source of truth for repository
  history.

## Success Criteria

1. Every run can expose an artifact list.
2. Mutating runs produce patch evidence and changed-file summaries.
3. Validation and command logs are linked to the run.
4. Approval decisions are inspectable by run.
5. The harness renders artifact tokens compactly and can preview details.
6. Final assistant summaries reference artifact IDs or evidence when relevant.
7. Suspicious patch shape is surfaced before the user is asked to trust the
   result.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0002 | Storage constraints affect artifact persistence |
| ADR-0014 | Harness and BFF split artifact capture according to local side-effect ownership |
| Future ADR | Should decide artifact storage, redaction, and retention boundaries |

## Open Questions

1. Which artifacts are stored in SQLite versus filesystem blob storage?
2. How should artifacts be redacted in team/enterprise profiles?
3. Should suspicious patch warnings block autonomous mode?
4. How should artifacts reference files inside isolated workspaces?
