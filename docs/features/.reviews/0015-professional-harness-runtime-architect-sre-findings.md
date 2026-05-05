# FEAT-0015 Findings (Architect + SRE pass)

- Feature: `docs/features/0015-professional-harness-runtime.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 9
- advisory: 3
- top_line: The umbrella covers the workflow/topic surface well, but cross-cutting *runtime guarantees* — event delivery, cancellation, idempotency, deadline/budget propagation, observability, durability boundaries, provider resilience — are not committed to. Without them, FEAT-0016..0022 can each be internally consistent and still produce a non-operable system in aggregate. None of these are blocking; most should be one-line commitments here plus a row in §Future ADRs, with detail deferred to the Run Runtime ADR or to downstream features.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Event Stream

**Affected sections:** Key Capabilities → Durable Runs; Protocol / API

**Summary:** Run-event delivery and resume semantics are not specified at the umbrella.

**Detail:** FEAT-0015:0356 lists "stream run events from a checkpoint" as a required protocol shape, but the umbrella does not commit to ordering, monotonic sequence numbers, gap detection on reattach, at-least-once vs exactly-once delivery, or schema versioning of events. FEAT-0017's "Resume After Restart" and FEAT-0020's artifact tail both depend on this contract, and they will diverge if it is not pinned upstream.

**Recommendation:** Add a one-line commitment in §Durable Runs that run events are append-only, monotonically sequenced per `run_id`, and resumable from a sequence number on reattach. Add a "Run event stream and idempotency semantics" row to §Future ADRs.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Lifecycle

**Affected sections:** Key Capabilities → Durable Runs; UI / CLI / API Integration

**Summary:** Cancellation semantics are named as a command but not defined as a contract.

**Detail:** `/cancel <run-id>` appears in §Terminal UI:0335 and `cancelled` is a terminal status, but the umbrella does not say whether cancel is cooperative or forced, what happens to in-flight tool calls, whether `tool_loop` partial filesystem writes are rolled back, whether artifacts captured before cancel are retained, or whether cancel cascades to sibling/synthesis runs. This intersects directly with the workspace cleanup contract in §Workspace Policy:0249-0257 and the disconnected-executor rule in §Workspace Policy:0260-0263.

**Recommendation:** Add a short "Cancellation" paragraph to §Durable Runs naming: cooperative-with-deadline default, in-flight tool-call fate, partial-artifact retention rule, and whether cancel cascades to siblings. Defer enforcement details to FEAT-0017.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Idempotency

**Affected sections:** Key Capabilities → Durable Runs; Tool Runtime Integration

**Summary:** Idempotency contract is missing for run creation and tool-result resubmission.

**Detail:** §Tool Runtime Integration:0202 states `tool_call_id` is stable and unique within a run, which is good for indexing. But the umbrella does not commit to idempotency for *run creation* (a retried create-run call after a flaky network), tool-result resubmission after harness reconnect mid-`tool_loop`, or duplicate `model_call` accounting. Without an idempotency-key contract, FEAT-0017's resume story will silently double-bill or double-execute under network partition.

**Recommendation:** Add a one-liner to §Durable Runs: run creation accepts a client-supplied idempotency key; tool-result delivery and `model_call` accounting are exactly-once per `tool_call_id` / `model_call_id`. Detail in FEAT-0017 and the Run Runtime ADR.

**Disposition:** accepted

---

### A4 — Advisory

**Reviewer:** Architecture / Attachment

**Affected sections:** Key Capabilities → Foreground and Background Agents; UI / CLI / API Integration

**Summary:** Multi-attach lease is undefined.

**Detail:** §Terminal UI:0334 lists `/attach <run-id>`. If two harnesses (laptop + desktop, or two team members in enterprise mode) attach to the same run, the umbrella does not say who owns the composer, who receives `waiting_permission` prompts, or what happens if both answer. Attachment state is described as a single boolean ("attached/detached/non-focused") in §Durable Runs:0090-0092, which does not model multi-client.

**Recommendation:** Either declare single-attach as a runtime invariant (only one harness may hold the foreground lease at a time, second attach is observer-only) or open a question for the Run Runtime ADR. Note in §Foreground and Background Agents which it is.

**Disposition:** accepted

---

### A5 — Significant

**Reviewer:** Architecture / Run Family

**Affected sections:** Key Capabilities → Foreground and Background Agents; Configuration

**Summary:** Deadlines and budgets do not propagate across run families.

**Detail:** §Foreground and Background Agents:0152-0155 introduces sibling runs and an optional synthesis run with parent references. §Configuration:0427-0431 mentions "maximum run cost and token budgets" and "permission timeout". There is no rule for whether a parent's deadline or budget bounds its siblings, whether synthesis waits indefinitely if a sibling deadlocks, or whether cancel/budget-exhaustion cascades. The §Non-Goals:0438 promise of "no unbounded autonomous swarms" is unenforceable without this.

**Recommendation:** Add a "Run Family Budgets and Deadlines" subsection (or paragraph in §Foreground and Background Agents) committing to: parent budget/deadline bounds children by default; synthesis runs have an explicit timeout; cancel cascades to active children unless run policy says otherwise.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Schema Versioning

**Affected sections:** Key Capabilities → Tool Runtime Integration; Future ADRs

**Summary:** Only tool input schema is versioned; the run record, workflow contract, context plan, and artifact bundle are not.

**Detail:** §Tool Runtime Integration:0210 names "tool name, namespace, and schema version" for each tool call. Run records, workflow contracts, context plans (FEAT-0018), and artifact bundles (FEAT-0020) are durable across modeltap upgrades but the umbrella does not commit to schema versioning for them. Replay or inspect of a v0.3 run under v0.4 is undefined.

**Recommendation:** Add a row to §Future ADRs: "Run record, artifact bundle, and workflow contract schema versioning". Optionally add one line to §Durable Runs that every persisted run-related record carries a schema version field.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Observability

**Affected sections:** Key Capabilities (new subsection); Configuration

**Summary:** No fleet-level operability surface for runs.

**Detail:** §Durable Runs:0136 commits to capturing cost, latency, and token usage *on the run*. The umbrella does not commit to operator-facing metrics (queue depth, time-in-stage histogram, stuck-stage rate, validation pass rate, permission-inbox age), structured logs, or trace IDs spanning BFF → harness → provider. The promise that "blocked background runs surface a visible permission or user-input inbox" (§Foreground and Background Agents:0167) is a UI surface, not an operator surface.

**Recommendation:** Add an "Observability" capability subsection naming: per-stage metrics, queue-depth metric, structured trace ID per run that propagates into model-provider calls, and a stuck-stage detection signal. Detail can land in a future operability feature, but the contract belongs here.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Liveness

**Affected sections:** Key Capabilities → Workspace Policy; new "Liveness" section

**Summary:** No heartbeat or stuck-stage timeout contract.

**Detail:** §Workspace Policy:0254-0258 specifies a `workspace_lost` fact and `failed` transition when a workspace becomes unexpectedly missing. There is no analogous contract for executor heartbeat loss, `model_call` hangs, tool processes that never return, or runs stuck in `waiting_permission` past a deadline. Background runs in particular need this — without it, a stuck run is indistinguishable from a slow run.

**Recommendation:** Add a short "Liveness" paragraph: harness/executor heartbeats; per-stage default deadlines (with the model_call and tool_loop deadlines configurable); `waiting_permission` timeout policy; stuck-stage transitions to `failed` with a structured reason.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Durability and Upgrade

**Affected sections:** Key Capabilities → Durable Runs; Future ADRs

**Summary:** Durability boundaries and upgrade compatibility for in-flight runs are unstated.

**Detail:** The umbrella promises that "run progress survives harness restart or reconnect when the BFF remains available" (§Foreground and Background Agents:0166). It does not say what survives a *BFF* crash mid-`tool_loop`, where the fsync points are, whether a tool result is durable before being fed to the next `model_call`, or whether checkpointed runs survive a BFF version upgrade. ADR-0002 commits storage to SQLite/WAL but the runtime contract that durable runs are built on top of WAL is not stated. Rolling-restart and upgrade safety for in-flight runs are also not committed to.

**Recommendation:** Add a "Durability" paragraph or row to §Future ADRs: at minimum name fsync-at-stage-transition as the default and commit to checkpoint-format compatibility across at least the previous one minor version. Defer specifics to the Run Runtime ADR.

**Disposition:** accepted

---

### S5 — Significant

**Reviewer:** SRE / Resource and Budget Limits

**Affected sections:** Configuration; Non-Goals

**Summary:** Budget stop-behavior at the boundary is undefined.

**Detail:** §Configuration:0428-0429 lists "background run concurrency limits" and "maximum run cost and token budgets". The umbrella does not say what happens *at* the boundary: hard stop, transition to `waiting_user`, transition to `waiting_permission`, request additional budget, or cancel-cascade siblings. Same gap for memory/FD/disk caps, which are not listed at all. The §Non-Goals:0438 swarm rule needs at least one concrete enforcement hook.

**Recommendation:** Add one paragraph to §Durable Runs (or a new "Resource Limits" subsection) naming: budget-exhaustion default behavior (transition to `waiting_user` is the safe default), per-run resource caps as policy inputs, and that limits compose across run families per A5.

**Disposition:** accepted

---

### S6 — Significant

**Reviewer:** SRE / Provider Resilience

**Affected sections:** Key Capabilities → Memory and Routing; new "Resilience" note

**Summary:** No policy for provider failure, rate-limit, or fallback.

**Detail:** §Durable Runs:0118 names `model_call` as a stage. The umbrella does not commit to behavior on provider 5xx, 429 / rate-limit backoff, fallback model selection, or per-provider quotas. Routing in FEAT-0022 is described as quality-driven, not failure-driven, so the resilience story currently has no home.

**Recommendation:** Add one paragraph to §Memory and Routing or as a sibling subsection: every `model_call` runs under a retry/backoff policy with a configurable fallback model; provider failures are recorded as run events but do not corrupt the run; rate-limit handling does not bypass the queue. Detail in FEAT-0022.

**Disposition:** accepted

---

### S7 — Advisory

**Reviewer:** SRE / Retention

**Affected sections:** Configuration; Open Questions

**Summary:** Run record and artifact-blob retention/GC are not specified.

**Detail:** ADR-0005 commits to retention-based pruning for *capture*. The umbrella does not say how long checkpointed runs persist, whether failed runs age out faster than completed runs, or how artifact blobs (referenced via the future "Artifact storage and redaction" ADR) are garbage-collected. Open Question 5 covers redaction but not lifecycle.

**Recommendation:** Either add a §Configuration line naming default run-record and artifact-blob retention windows, or add "Retention and GC defaults for runs and artifacts" as a §Future ADRs row alongside the redaction ADR.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added append-only, per-run monotonic event sequence and resume-from-sequence/gap-detection commitments; added future ADR coverage for event stream semantics. |
| A2 | accepted | Added a cancellation paragraph covering cooperative deadline behavior, in-flight model/tool calls, non-rollback of local writes, partial artifact retention, and parent-child cascade default. |
| A3 | accepted | Added run creation idempotency keys plus idempotent tool-result delivery and model-call accounting by stable IDs. |
| A4 | accepted | Added a single foreground attachment lease invariant; additional clients are observer-only unless the lease is transferred. |
| A5 | accepted | Added run-family budget/deadline propagation, synthesis timeout, and cascade defaults for cancellation and budget exhaustion. |
| A6 | accepted | Added schema-version metadata for persisted run-related records and future ADR coverage for run/artifact/workflow schema semantics. |
| S1 | accepted | Added an observability subsection with queue, stage, stuck-run, validation, permission-age, failure-rate, structured log, and trace propagation commitments. |
| S2 | accepted | Added heartbeat, stage-deadline, configurable `model_call`/`tool_loop` deadline, permission timeout, and structured stuck-stage failure behavior. |
| S3 | accepted | Added BFF-owned durability commitments and future ADR coverage for fsync boundaries, rolling restart behavior, and checkpoint compatibility. |
| S5 | accepted | Added budget exhaustion defaulting to `waiting_user`, per-run resource caps, and composed family limits. |
| S6 | accepted | Added model-call retry/backoff, provider failure event recording, fallback-model policy, and queue/budget preservation language. |
| S7 | accepted | Added configuration lines for run/artifact retention and expanded the artifact ADR row to cover retention and GC. |
