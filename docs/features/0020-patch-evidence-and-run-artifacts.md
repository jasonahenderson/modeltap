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

Every artifact uses a minimal envelope:

- `artifact_id`
- `run_id`
- `type`
- `schema_version`
- `created_at`
- `payload_ref`
- optional `payload_inline`
- `redaction_state`
- `truncated`
- provenance and source IDs when applicable

Initial artifact types include `context_plan`, `prompt_metadata`,
`policy_summary`, `tool_log`, `approval_log`, `command_log`,
`validation_evidence`, `diff`, `changed_files`, `patch_summary`,
`generated_output`, `usage_summary`, `routing_decision`, and `final_summary`.
Per-type payload schemas are deferred to the artifact-storage ADR.

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

Patch evidence is computed incrementally at the end of every `tool_loop` segment
and at the start of each repair turn. The final patch artifact is the cumulative
diff at `artifact_capture`; per-turn diffs are retained as sub-artifacts.

The patch read set contains files explicitly read by tool calls during
`tool_loop` and context-planner attachments whose content was rendered into the
prompt. Validation reads do not count. The read set is recorded on the patch
artifact.

Default warning thresholds are active without extra configuration:

- suspicious churn: more than 50% of changed files have less than 10% net change
- broad formatting: more than 5 files changed with 10 or fewer net lines per file
- generated-file and vendored-path warnings: use FEAT-0018 ignored/generated path
  patterns

Threshold overrides may tune or raise limits, but the warning system is enabled
by default.

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

Every artifact has a stable `artifact_id`. The artifact references its owning
`run_id`; the run stores artifact references and indexes but does not embed
artifact identity as run identity. Artifacts within a bundle remain
independently addressable by `artifact_id`.

Forked runs reference parent artifacts read-only through `inherited_from:
artifact_id` and emit new artifacts with new IDs. Patch evidence on a fork is
computed against the parent's final workspace state or explicit fork snapshot.

BFF metadata is authoritative for artifact existence, identity, and provenance.
Artifact content may live in BFF blob storage or remain harness-owned locally.
Locally stored artifacts include a host fingerprint so the BFF can detect when
content is unreachable from the current harness instance and surface a
`content_unavailable` state on read. The BFF does not silently dereference
artifact records when local content is missing; the artifact remains listed with
metadata and a clear unavailability reason.

On harness reattach, the harness reports artifacts it can serve. The BFF marks
matching local artifacts available again. After a configurable grace period,
defaulting to 30 days, stranded local artifacts transition to `unrecoverable` and
may be garbage-collected. Host fingerprint changes are treated as new hosts; a
user may explicitly rebind local artifacts.

Artifact writes are content-first, metadata-second. The metadata write is the
durability boundary. Orphan blobs are tolerated and reaped by GC; orphan metadata
pointing to missing content is not allowed for newly written artifacts. The same
ordering applies to BFF blob storage and harness-local artifacts.

Artifact retention follows the FEAT-0015 retention envelope. Artifacts age out
with their run unless promoted to durable memory or another explicit durable
record. The run record or tombstone remains long enough to explain artifact
retention and GC decisions.

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
- per-run artifact count cap
- local-artifact stranded grace period

Captured log size caps apply per artifact. Overflow truncates with tail-first
retention, stores a checksum of the full output when available, and marks the
artifact `truncated: true`. Many small artifacts are controlled by the artifact
count cap, not the log-size cap.

Redaction applies at capture time for known secret patterns and at retrieval time
for role-based redaction. Capture-time redaction is irreversible. Policy changes
schedule a re-scan job for stored artifacts.

The default soft cap is 1000 artifacts per run. Overflow coalesces by category,
keeping recent high-signal artifacts and a summary artifact.

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
