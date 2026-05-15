# FEAT-0015..0022 Continuity Findings (Architect + SRE pass)

- Scope: `.sdlc/features/0015-professional-harness-runtime.md` and member specs FEAT-0016 through FEAT-0022
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- Companion per-feature reviews: `.sdlc/features/.reviews/00{15..22}-*-architect-sre-findings.md`
- total_findings: 12
- blocking: 0
- significant: 9
- advisory: 3
- top_line: Each member spec is internally reasonable but the series does not yet compose into a single specification. Cross-cutting contracts — event delivery, identity schemas, permission flow, retention coordination, observability, budget envelope, vocabulary, disconnected-executor surface — are referenced by multiple features and pinned by none. Resolving them as part of FEAT-0015 (or as a "series architecture lock" before Phase 3) prevents the most likely failure mode: each FEAT shippable in isolation, the aggregate not operable. None of the 12 findings are blocking, but the first 9 should be resolved before Phase 3 begins.

## Findings

### C1 — Significant

**Reviewer:** Series / Vocabulary Coherence

**Affected:** FEAT-0015 §Durable Runs; FEAT-0016 §Run Lifecycle, §Harness Responsibilities; FEAT-0017 §Attachment Semantics

**Summary:** Vocabulary fragments across the series: status, stage, attachment, control verbs.

**Detail:** Examples: `cancel` (FEAT-0015) vs `interrupt` (FEAT-0016) vs both used loosely (FEAT-0017); `blocked` is a status grouping (FEAT-0015) and an attachment state (FEAT-0017); `checkpointed` is a status (FEAT-0015) and `checkpoint` is a stage (FEAT-0016); `pause` is informally used in several places without a formal status. The umbrella attempted to pin the vocabulary but the members re-introduced terms.

**Recommendation:** Add a "Canonical Vocabulary" subsection to FEAT-0015 listing every status, stage, attachment state, and control verb in the series, with a glossary line each. Member specs reference the umbrella vocabulary instead of redefining. Future ADR locks the set.

**Disposition:** accepted

---

### C2 — Significant

**Reviewer:** Series / Identity Schema

**Affected:** All members. Concrete touchpoints: FEAT-0016 §BFF Responsibilities, §Run Lifecycle (turn_id); FEAT-0019 §Check Execution (check_id); FEAT-0020 §Artifact Persistence (artifact_id); FEAT-0021 §Audit Trail (decision_id, result_id, policy version).

**Summary:** The series introduces ~10 ID concepts without a central glossary or schema commitment.

**Detail:** Identifiers in play: `session_id`, `run_id`, `turn_id`, `tool_call_id`, `decision_id`, `result_id`, `check_id`, `artifact_id`, `model_call_id` (proposed in FEAT-0016 review), memory IDs (FEAT-0022), workflow profile IDs, host fingerprints, policy versions, schema versions. Each is mentioned where it is used; none is centrally defined. FEAT-0015 architect+SRE A6 raised schema versioning at the umbrella; the identity-schema half is its sibling.

**Recommendation:** Add an "Identity and Schema" subsection to FEAT-0015 listing every ID concept, its uniqueness scope (global, per-run, per-tool-call, etc.), and its schema-version field. Pin a single schema-versioning rule for all run-related records.

**Disposition:** accepted

---

### C3 — Significant

**Reviewer:** Series / Event Stream Contract

**Affected:** FEAT-0015 §Protocol/API; FEAT-0016 §Pipeline Events; FEAT-0017 §Resume After Restart; FEAT-0020 §Artifact Bundle

**Summary:** Four features depend on a run-event-stream contract that is not pinned anywhere.

**Detail:** FEAT-0015 names "stream run events from a checkpoint." FEAT-0016 names a list of pipeline events. FEAT-0017 §Resume After Restart says BFF "may replay full events, summarize missed events, or show checkpointed state." FEAT-0020 implies events for artifact recording. None commits to ordering, monotonic sequence numbers, gap detection, at-least-once-vs-exactly-once, or event schema. FEAT-0015 architect+SRE A1 + FEAT-0017 architect+SRE A2 + FEAT-0016 architect+SRE A3 are three views of the same gap. This is the single highest-impact cross-feature contract.

**Recommendation:** Pin the event contract once, in FEAT-0015 or in the future Run Runtime ADR with a stub commitment now: monotonic per-`run_id` sequence numbers, append-only, schema-versioned envelope, replay from sequence number, at-least-once delivery with idempotency keys for tool-result and model-call events.

**Disposition:** accepted

---

### C4 — Significant

**Reviewer:** Series / Permission Flow Synchronization

**Affected:** FEAT-0015 §Durable Runs (`waiting_permission` status); FEAT-0017 §Background Permission Behavior, §Run Queue, `/permissions`; FEAT-0021 §Audit Trail, §Permission Outcomes

**Summary:** Permission flow has three owners and no synchronization story.

**Detail:** FEAT-0015 owns the *status* (`waiting_permission`); FEAT-0017 owns the *queue/inbox* surface (`/permissions`); FEAT-0021 owns the *decision* (the harness decides, BFF records). Sequence questions are not answered: who emits the `waiting_permission` transition (BFF on receiving the harness's "decision needed" report? harness on detecting policy gap?); who owns the inbox state (per-run? per-user? per-server?); how are decisions on detached runs surfaced to the right inbox in multi-user contexts.

**Recommendation:** Add a "Permission Flow" subsection to FEAT-0015 sequencing the actors: harness detects approval need → reports to BFF → BFF emits `waiting_permission` and inbox event → user resolves via inbox → harness records decision → BFF transitions run. Each member feature references this sequence.

**Disposition:** accepted

---

### C5 — Significant

**Reviewer:** Series / Workspace Lifecycle Ownership

**Affected:** FEAT-0015 §Workspace Policy; FEAT-0017 (cancel/cleanup); FEAT-0019 (workspace for checks); FEAT-0020 (artifacts inside isolated workspaces, OQ4); FEAT-0021 (workspace-aware execution)

**Summary:** Workspace lifecycle is referenced by five features without a single owner.

**Detail:** FEAT-0015 names the modes and the cleanup/orphan rules. FEAT-0017 introduces cancel and disconnect events that affect cleanup. FEAT-0019 executes checks against a workspace. FEAT-0020 stores artifacts that may live in workspace blob paths. FEAT-0021 enforces policy per workspace. The result: workspace creation timing, artifact-vs-workspace retention, cleanup criteria across cancel/fail/grace-period, and orphan reaping are spread across features without a coherent contract.

**Recommendation:** Promote workspace lifecycle in FEAT-0015 from a policy section to a small numbered contract: creation at `preflight`; cleanup at terminal status *unless* the run has artifacts pinned to it (FEAT-0020 cross-link); orphan reaping on harness reattach with user-visible notice. Other features only reference it.

**Disposition:** accepted

---

### C6 — Significant

**Reviewer:** Series / Retention Coordination

**Affected:** FEAT-0017 §Configuration (blocked, completed); FEAT-0018 (context plans implicitly retained); FEAT-0019 (validation evidence implicitly retained); FEAT-0020 §Configuration (artifact retention); FEAT-0022 §Configuration (memory retention)

**Summary:** Retention is configurable in five features with no coordination rule.

**Detail:** Each feature owns a retention knob. There is no precedence rule for what survives what. Examples that should not happen but currently can: artifact GC'd before run record; memory promoted from a run whose artifacts have aged out (broken provenance); transcript retained but artifacts gone (inspect shows half a story). FEAT-0017 architect+SRE S4 + FEAT-0020 architect+SRE S1 raise this from each side; the umbrella needs to pin the precedence.

**Recommendation:** Pin a retention envelope in FEAT-0015: `memory ≥ artifact ≥ run record ≥ transcript`. Each member feature's retention knob may shorten *within* the envelope but cannot violate the precedence. Promotion to memory (FEAT-0022) is the single mechanism for outliving the envelope.

**Disposition:** accepted

---

### C7 — Significant

**Reviewer:** Series / Forward References

**Affected:** Multiple

**Summary:** Members cite capabilities upstream/downstream features have not pinned.

**Detail:** Concrete instances:
- FEAT-0017 §Resume After Restart depends on FEAT-0016 checkpoint cadence; FEAT-0016 §Run Lifecycle defines `checkpoint` as a single late stage (FEAT-0016 architect+SRE A3).
- FEAT-0019 §Repair Attempts re-enters the pipeline; FEAT-0016 stage sequence is linear (FEAT-0016 architect+SRE A1, A5).
- FEAT-0020 §Patch Evidence depends on validation timing (FEAT-0019); FEAT-0019 does not pin where checks run relative to capture (FEAT-0020 architect+SRE A3).
- FEAT-0022 §Active Memory Inspection cites FEAT-0018 provenance; FEAT-0018 provenance is closed-set (FEAT-0018 architect+SRE A6).
- FEAT-0021 §Server-Safe Tools cites the run-runtime ADR as the source of truth for the BFF-safe tool surface; the ADR is not yet drafted.

**Recommendation:** Add a "Series Sequencing" subsection to FEAT-0015 listing the cross-references explicitly. Pin each forward reference either with a one-line commitment now or with an explicit "Run Runtime ADR resolves before Phase 3" note. Without this, members can pass review individually while the aggregate is incoherent.

**Disposition:** accepted

---

### C8 — Significant

**Reviewer:** Series / Operability Surface Absent

**Affected:** All members

**Summary:** No member commits to fleet metrics, traces, or operator-facing signals.

**Detail:** The series records cost/latency *on the run* (FEAT-0015 §Durable Runs:0136), audit decisions *on the run* (FEAT-0021), routing decisions *on the run* (FEAT-0022). None commits to: queue-depth metric, time-in-stage histogram, stuck-stage signal, validation-pass-rate metric, permission-inbox-age metric, structured trace ID spanning BFF → harness → provider, or structured logs. The user-facing inbox in FEAT-0017 is the only operability surface and it does not aggregate across runs.

**Recommendation:** Add an "Observability" capability in FEAT-0015 (per the FEAT-0015 architect+SRE S1 recommendation). State that every run carries a trace ID propagated to provider calls and tool subprocesses; per-stage metrics are emitted as a counter+histogram pair; the operability surface is a separate downstream feature.

**Disposition:** accepted

---

### C9 — Significant

**Reviewer:** Series / Cost and Budget Envelope

**Affected:** FEAT-0015 §Configuration; FEAT-0016 §BFF Responsibilities (cost correlation); FEAT-0019 §Repair Attempts (loop limit); FEAT-0022 §Quality-Driven Routing (cost recorded)

**Summary:** Cost and budget concerns span four features without a coherent envelope.

**Detail:** FEAT-0015 names cost/token budgets without stop-behavior. FEAT-0016 correlates cost to the run but not to stages or model_calls (FEAT-0016 architect+SRE S2). FEAT-0019 has repair-attempt limits but no cost-side ceiling (FEAT-0019 architect+SRE S3). FEAT-0022 records routing cost but does not gate routing decisions on remaining budget. Sibling/parallel candidate runs (FEAT-0015) have no parent-budget rule. The aggregate: a multi-candidate implementation with repair loops and routing-driven model upgrades can exceed any reasonable budget without any single feature flagging it.

**Recommendation:** Pin a budget envelope in FEAT-0015: per-run budget bounds children unless overridden; repair loops cost-cap at 3× initial attempt cost; routing must check remaining budget before upgrading model class; budget exhaustion → `waiting_user`. Each member references the envelope.

**Disposition:** accepted

---

### C10 — Advisory

**Reviewer:** Series / Disconnected-Executor Surface

**Affected:** FEAT-0015 §Workspace Policy; FEAT-0017 OQ1, §Background Permission Behavior; FEAT-0021 §Server-Safe Tools

**Summary:** The disconnected-executor rule is referenced in three features with consistent intent and inconsistent specificity.

**Detail:** FEAT-0015 states the rule and defers the BFF-safe tool surface to the ADR. FEAT-0017 names the same open question. FEAT-0021 enumerates a small set (BFF-side retrieval, citation, summarization). No single feature is the source of truth, and the ADR that should resolve it is not yet drafted.

**Recommendation:** Designate FEAT-0021 as the source-of-truth for the BFF-safe tool surface (it already has §Server-Safe Tools). Have FEAT-0015 and FEAT-0017 reference it. The ADR enumerates the binding list; FEAT-0021 ships the slot.

**Disposition:** accepted

---

### C11 — Advisory

**Reviewer:** Series / Open Questions Overlap

**Affected:** FEAT-0015 OQ5; FEAT-0017 OQ1, OQ4; FEAT-0019 OQ4; FEAT-0020 OQ2; FEAT-0021 OQ3; FEAT-0022 OQ5

**Summary:** Multiple Open Questions cover the same topic from different angles.

**Detail:** Examples:
- FEAT-0015 OQ5 (artifact redaction) ↔ FEAT-0020 OQ2 (artifact redaction) — same question.
- FEAT-0017 OQ1 (server-safe tools) ↔ FEAT-0021 §Server-Safe Tools (small set) — answered in FEAT-0021 but FEAT-0017 still asks.
- FEAT-0019 OQ4 (repair limit default) ↔ FEAT-0015 budget envelope (C9) — same root cause.
- FEAT-0021 OQ3 (risk classification) ↔ FEAT-0019 architect+SRE A2 (same gap, validation side).

**Recommendation:** Consolidate Open Questions to a single canonical owner per topic. The umbrella's Open Questions section can become an index pointing to the canonical owner. Resolved questions are removed from the asking feature.

**Disposition:** accepted

---

### C12 — Advisory

**Reviewer:** Series / Slash-Command Namespace

**Affected:** FEAT-0015 §Terminal UI; FEAT-0016, FEAT-0017, FEAT-0018, FEAT-0019, FEAT-0020, FEAT-0021, FEAT-0022 §UI / CLI / API Integration

**Summary:** The slash-command namespace is filled across the series without a registry.

**Detail:** Commands introduced: `/jobs`, `/runs`, `/run`, `/run context`, `/run prompt`, `/run policy`, `/attach`, `/detach`, `/cancel`, `/continue`, `/retry`, `/fork`, `/artifacts`, `/diff`, `/evidence`, `/explore`, `/feature`, `/adr`, `/release`, `/implement`, `/debug`, `/docs`, `/devops`, `/context`, `/context rules`, `/context why`, `/context drop`, `/validate`, `/validate plan`, `/validate retry`, `/repair`, `/permissions`, `/policy`, `/memory`, `/memory accept`, `/routing`, `/workflows`. Two-word commands (`/run context`, `/validate plan`) overlap with `/context` and `/validate` as singular commands; consistency is uneven.

**Recommendation:** Add a "Command Registry" subsection to FEAT-0015 enumerating commands and their owners. Commands of the form `/x sub` should follow a consistent shape across features (always `/x sub`, never `/x-sub`). New commands require registry update.

**Disposition:** accepted

---

## Cross-Reference Map

| Continuity finding | Per-feature touchpoints |
|---|---|
| C1 Vocabulary | FEAT-0015 A2, FEAT-0016 A4, FEAT-0017 A1 |
| C2 Identity Schema | FEAT-0015 A6, FEAT-0020 A1 |
| C3 Event Stream | FEAT-0015 A1, FEAT-0016 A3, FEAT-0017 A2 |
| C4 Permission Flow | FEAT-0021 A1, A2; FEAT-0017 A4 |
| C5 Workspace Lifecycle | FEAT-0017 A6, FEAT-0019 (workspace), FEAT-0020 (OQ4) |
| C6 Retention Coordination | FEAT-0017 S4, FEAT-0020 S1 |
| C7 Forward References | FEAT-0016 A1/A3/A5, FEAT-0019 A1, FEAT-0020 A3, FEAT-0018 A6, FEAT-0021 A4 |
| C8 Operability | FEAT-0015 S1, all members (none) |
| C9 Cost/Budget Envelope | FEAT-0015 A5/S5, FEAT-0016 S2, FEAT-0019 S3, FEAT-0022 S2 |
| C10 Disconnected Executor | FEAT-0021 A4 |
| C11 Open Questions Overlap | FEAT-0015..0022 OQs |
| C12 Slash Commands | FEAT-0015..0022 §UI |

## Suggested Resolution Order

The continuity findings have a partial order. C3 (event stream) and C2 (identity schema) are foundational — most others depend on them. C1 (vocabulary) is mechanical and cheap. C4-C9 are independent of each other but each depends on C1-C3.

1. C1 vocabulary lock — fastest win, unblocks readers.
2. C2 identity schema and C3 event stream — pin together; bind the future Run Runtime ADR to them.
3. C4 permission flow, C5 workspace lifecycle, C6 retention coordination — three independent contracts; can land in parallel.
4. C7 forward references — naturally resolves as 1-3 land; remaining cases call out explicit ADR commitments.
5. C8 operability surface — add the umbrella subsection now, defer the operability feature.
6. C9 budget envelope — relies on C1-C3; commits to per-run, sibling, repair, and routing budget composition.
7. C10-C12 advisories — admin-grade; can land any time before Phase 3 starts.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| C1 | accepted | Added canonical status, stage, attachment, and control-verb vocabulary to FEAT-0015; aligned FEAT-0016/0017 terminology. |
| C2 | accepted | Added FEAT-0015 identity/schema table covering run, turn, tool, check, artifact, decision, result, memory, workflow, host, policy, and schema IDs. |
| C3 | accepted | FEAT-0015 already pins append-only sequenced events; FEAT-0016/0017 now add event buffering, checkpoints, and sequence replay behavior. |
| C4 | accepted | Added FEAT-0015 permission-flow sequence and FEAT-0021 policy-version/audit decision details. |
| C5 | accepted | Reworked FEAT-0015 workspace lifecycle into a numbered contract and linked FEAT-0017 cancellation cleanup to it. |
| C6 | accepted | Added FEAT-0015 retention envelope and member references/constraints in FEAT-0017 and FEAT-0020. |
| C7 | accepted | Added FEAT-0015 series sequencing rules and pinned forward references in member specs where reviewed. |
| C8 | accepted | FEAT-0015 observability/liveness section already names metrics, trace IDs, stuck-stage detection, and failure rates. |
| C9 | accepted | Added FEAT-0015 budget envelope for run families, repair-loop cost, routing budget checks, and budget exhaustion behavior. |
| C10 | accepted | Designated FEAT-0021 as the server-safe tool surface owner while leaving the binding list to the Run Runtime ADR. |
| C11 | accepted | Removed or narrowed resolved open questions in member specs where the review produced a concrete answer. |
| C12 | accepted | Added a FEAT-0015 command registry and consistent `/x subcommand` rule. |
