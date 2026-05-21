# FEAT-0020 Findings (Architect + SRE pass)

- Feature: `.sdlc/features/0020-patch-evidence-and-run-artifacts.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 8
- advisory: 4
- top_line: The artifact-bundle list, `artifact_id` stability, and the `content_unavailable` discipline for locally-stored content are the right primitives. The spec underspecifies the artifact *schema* (types, required fields, versioning), the *timing* of patch evidence relative to the pipeline graph, and the operational coordination between artifact retention, run retention, and storage durability. Several "warnings" in §Patch Evidence are described as outputs without their detection rules being defined, so the warning system can ship dormant.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Artifact Schema

**Affected sections:** Key Capabilities → Artifact Bundle; Artifact Persistence

**Summary:** The artifact bundle is a list of categories, not a schema with types or versioning.

**Detail:** §Artifact Bundle:0035-0051 lists 13 artifact categories. There is no commitment to required vs optional, payload shape per category, or version field. FEAT-0015 architect+SRE A6 noted the umbrella-level schema-versioning gap; FEAT-0020 is the natural home for the artifact half. Without schema, replay/inspect across modeltap versions is undefined and tooling cannot rely on artifact contents.

**Recommendation:** Pin a minimal envelope: `artifact_id`, `run_id`, `type` (closed enum), `schema_version`, `created_at`, `payload_ref`, `payload_inline?`, `redaction_state`. Per-category payload schemas defer to the artifact-storage ADR but the envelope ships now.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Local Content Recovery

**Affected sections:** Key Capabilities → Artifact Persistence

**Summary:** `content_unavailable` has no recovery contract.

**Detail:** §Artifact Persistence:0089-0096 describes locally-owned artifacts with a host fingerprint and a `content_unavailable` state on read when the host is unreachable. The contract does not say: how the BFF detects that a previously unavailable artifact is now reachable (re-attach by the same host? probe?), what happens when a host fingerprint changes (laptop rebuild), or when artifact metadata is stranded (host gone forever). Long-running projects accumulate stranded artifacts.

**Recommendation:** State that on harness reattach, the harness reports the set of artifacts it can serve; the BFF flips matching artifacts back to `available`. After a configurable grace (e.g. 30 days), stranded artifacts transition to `unrecoverable` and may be GC'd. Host fingerprint changes are handled as new hosts; the user can rebind via an explicit command.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Patch Evidence Timing

**Affected sections:** Key Capabilities → Patch Evidence

**Summary:** When patch evidence is computed relative to the pipeline graph is unspecified.

**Detail:** §Patch Evidence:0053-0064 says the harness "computes or requests" patch evidence but does not pin the timing. After `tool_loop`? After `validation` (which may itself write files via formatters/test runners)? After repair turns? FEAT-0019 repair attempts can mutate files; if patch evidence is computed before repair, it captures a stale diff; if after, it conflates repair edits with original edits.

**Recommendation:** Compute incremental patch evidence at the end of every `tool_loop` segment and at the start of each repair turn (so each turn's edits are attributable). The final patch artifact is the cumulative diff at `artifact_capture`; per-turn diffs are kept as sub-artifacts.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Read-Set Tracking

**Affected sections:** Key Capabilities → Patch Evidence

**Summary:** "Edits to files not read during the run" requires a defined read set; planner reads vs tool reads are not distinguished.

**Detail:** §Patch Evidence:0061 lists "edits to files not read during the run" as a warning. The read set must be defined: are FEAT-0018 context-planner reads counted (the file was attached to the prompt but never explicitly opened by a tool)? Are FEAT-0019 validation tool reads counted (a test reading a file is not a model read)? Without the rule, the warning misfires.

**Recommendation:** Define the read set as: files explicitly opened or read by tool calls during `tool_loop` (whether by the model or by extensions acting on its behalf). Context-planner attachments count if their content was rendered into the prompt. Validation reads do not count. Record the read set on the patch artifact.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Fork Identity

**Affected sections:** Key Capabilities → Artifact Persistence

**Summary:** Whether artifact IDs are inherited or re-emitted on `/fork` is unspecified.

**Detail:** §Artifact Persistence:0085-0088 says `artifact_id` is stable and references its owning `run_id`. FEAT-0017 lists `/fork <run-id>`. A fork that inherits the parent's artifacts (read-through) and produces its own (write-through) needs a defined identity rule; otherwise inspecting a fork's artifacts can ambiguously reference parent artifacts.

**Recommendation:** Forked runs reference parent artifacts read-only via `inherited_from: artifact_id` and emit their own artifacts with new IDs. Patch evidence on the fork is computed against the parent's final state.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Warning Detection Rules

**Affected sections:** Key Capabilities → Patch Evidence; Configuration; Open Questions

**Summary:** "Suspicious churn warnings" and "broad formatting warnings" are listed as outputs without detection rules.

**Detail:** §Patch Evidence:0058-0063 lists six warnings (suspicious churn, broad formatting, edits to unread files, unrelated local changes, generated-file, vendored-path). Detection rules and thresholds are not defined. §Configuration:0118 names "suspicious diff thresholds" with no defaults. Open Question 3 asks if warnings should *block* — but the spec does not say *how* warnings *fire*.

**Recommendation:** Pin defaults: suspicious churn = >50% of changed files have <10% net change; broad formatting = >5 files changed but ≤10 net lines per file; generated-file/vendored-path defer to FEAT-0018 ignored-paths config. Make all configurable. Without defaults the warning system is shipped-but-dormant.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Retention Coordination

**Affected sections:** Configuration

**Summary:** Artifact retention does not coordinate with run, transcript, or workspace retention.

**Detail:** §Configuration:0114 names "artifact retention." FEAT-0017 names blocked-run and completed-run retention. FEAT-0018 implicitly retains context plans. FEAT-0022 names memory retention. FEAT-0017 architect+SRE S4 already raised this from the run side; FEAT-0020 is the artifact home. Currently, an artifact can outlive its run record (orphan reference) or be GC'd before its run (broken reference).

**Recommendation:** State that artifact retention is bounded by run retention: artifacts age out with their run, with the run record being the last to age out. A separate "promote to durable" path (cross-link to FEAT-0022 memory) survives run aging.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Capture Size Bounds

**Affected sections:** Configuration

**Summary:** "Maximum captured log size" applies to what scope, with what overflow behavior?

**Detail:** §Configuration:0116 names a single value with no scope (per artifact? per run? per command?) and no overflow contract (truncate? reject? store-by-ref-only?). FEAT-0019 architect+SRE S4 raised the validation-output bound; the artifact-side bound is the controlling spec.

**Recommendation:** Apply the cap per artifact, with overflow → truncate-and-checksum (keep tail by default) and a `truncated: true` flag in the artifact envelope. Document that the cap is independent of count caps for many small artifacts.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Redaction Application

**Affected sections:** Configuration; Open Questions

**Summary:** Redaction is named as a configuration but the application point is unspecified.

**Detail:** §Configuration:0117 names "redaction policy"; Open Question 2 asks how to redact in team/enterprise. The spec does not say whether redaction happens at capture time (irreversible), at retrieval time (reversible by privileged readers), or both. Each has different security and recovery implications. Re-redaction when policy changes (newly-classified secret patterns) is also not addressed.

**Recommendation:** Capture-time redaction for known secret patterns (irreversible), retrieval-time redaction for role-based redaction (reversible). On policy change, schedule a re-scan job. Defer the precise policy syntax to the artifact-storage ADR.

**Disposition:** accepted

---

### S4 — Significant

**Reviewer:** SRE / Storage Durability

**Affected sections:** Key Capabilities → Artifact Persistence; Relationship to ADRs

**Summary:** Metadata-content fsync ordering is not specified; corrupt states (metadata pointing to missing content) are silently possible.

**Detail:** §Artifact Persistence:0079-0096 splits metadata (BFF SQLite) from content (BFF blob storage or harness-local). With independent durability paths, metadata can fsync before content (orphan pointer on crash) or after content (orphan blob on crash). FEAT-0015 architect+SRE S3 raised the durability-boundary question; FEAT-0020 is the home for the artifact half.

**Recommendation:** Commit to write-content-first, then write-metadata, with the metadata write being the durability boundary. Orphan blobs are tolerated and reaped by GC; orphan metadata is not allowed. State this for both BFF blob storage and harness-local artifacts.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / Defaults Mean Dormant

**Affected sections:** Configuration

**Summary:** Threshold-based features without defaults ship inactive.

**Detail:** §Configuration:0118 names "suspicious diff thresholds" but no defaults are pinned. A6 covered the architectural side; the operational consequence is that on day-1 deploy, these warnings are silent and the user cannot tell whether the system is healthy or broken.

**Recommendation:** Always ship defaults; document them; require configuration overrides to *raise* thresholds, not to enable them.

**Disposition:** accepted

---

### S6 — Advisory

**Reviewer:** SRE / Per-Run Artifact Count Bounds

**Affected sections:** Key Capabilities → Artifact Bundle; Configuration

**Summary:** No upper bound on artifacts per run.

**Detail:** A run that fails 100 tests produces 100 failure artifacts; a long-running multi-turn run with many tool calls accumulates many command artifacts. No commitment to a per-run artifact count cap or backpressure when the cap is reached. SQLite metadata growth and inspection-UI usability both suffer.

**Recommendation:** Default a soft cap (e.g. 1000 artifacts/run) with overflow → coalesce by category (e.g. 10 most recent + a summary). Make configurable.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added minimal artifact envelope, schema version, redaction/truncation fields, and initial artifact type enum. |
| A2 | accepted | Added reattach availability reporting, stranded-artifact grace, `unrecoverable`, and explicit rebind behavior. |
| A3 | accepted | Added incremental patch evidence at `tool_loop` boundaries and repair starts, plus cumulative final diff. |
| A4 | accepted | Defined patch read set, planner attachment handling, validation exclusion, and read-set recording. |
| A5 | accepted | Added fork artifact inheritance via `inherited_from` and new IDs for fork-produced artifacts. |
| A6 | accepted | Added active default warning thresholds for churn, broad formatting, generated files, and vendored paths. |
| S1 | accepted | Added FEAT-0015 retention-envelope reference and run/tombstone provenance rule for artifact aging. |
| S2 | accepted | Defined per-artifact log cap with tail-first truncation, checksum, and `truncated` flag. |
| S3 | accepted | Added capture-time secret redaction, retrieval-time role redaction, and policy-change re-scan. |
| S4 | accepted | Added content-first, metadata-second durability ordering and orphan blob/metadata handling. |
| S5 | accepted | Stated warning thresholds are enabled by default and overrides tune rather than activate them. |
| S6 | accepted | Added default 1000-artifact soft cap and category coalescing overflow behavior. |
