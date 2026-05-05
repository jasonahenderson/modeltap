# FEAT-0016 Findings (Architect + SRE pass)

- Feature: `docs/features/0016-managed-codegen-run-pipeline.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 8
- advisory: 4
- top_line: The pipeline names the right stages, owners, and event categories, but it commits to them as a *linear sequence* of one-shot phases. In practice the codegen pipeline is a graph (repair, replan, parallel tool calls, multi-`model_call` runs) with multiple checkpoint boundaries and per-stage operational signals. Eight findings target the gap between the linear description and the real pipeline shape; four are consistency or operability advisories. None are blocking, but several should be resolved here before FEAT-0017/0019/0020 build on top.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Pipeline Graph

**Affected sections:** Solution; Key Capabilities → Run Lifecycle

**Summary:** The stage sequence is described as linear; the real pipeline has reentry edges that are not modeled.

**Detail:** §Solution:0038 and §Run Lifecycle:0048-0063 present stages as `preflight -> context_plan -> prompt_plan -> model_call -> tool_loop -> validation -> artifact_capture -> checkpoint -> completion`. In practice, the pipeline is a graph: validation failures feed a *repair turn* that re-enters `model_call` (FEAT-0019); tool results inside `tool_loop` may force a `prompt_plan` revisit (FEAT-0018); a `model_call` provider failure may force a retry that repeats `prompt_plan`. §Run Lifecycle:0070 explicitly mentions repair turns and validation triage turns but does not show where those reenter the stage graph. Without the graph, downstream features cannot reason about idempotency at stage boundaries.

**Recommendation:** Replace or supplement the arrow sequence with a small DAG (or numbered list of allowed transitions) naming the legal backedges: `validation -> model_call`, `tool_loop -> prompt_plan`, `model_call -> preflight` (forced restart), and the terminal-only stages. Defer transition policy details to the Run Runtime ADR.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Concurrency

**Affected sections:** Key Capabilities → Run Lifecycle; Pipeline Events

**Summary:** Interleaving between `model_call` and `tool_loop` and parallel tool execution are unmodeled.

**Detail:** Real model streams emit tool calls *during* the response, not after; modern providers also emit parallel tool calls within a single turn. §Run Lifecycle:0056-0058 separates `model_call` and `tool_loop` as discrete stages, but the actual pipeline interleaves them. The feature also does not commit to whether tool calls within a turn execute serially, in-emission-order, or in parallel — which affects determinism, replay, and partial-failure semantics (see S4).

**Recommendation:** State explicitly that `model_call` and `tool_loop` overlap during streaming, and that the run records both as concurrent sub-states until the model stops or `tool_loop` blocks. Pin a default execution policy for parallel tool calls (e.g. parallel-by-default with per-tool serialization where filesystem writes overlap) and mark it as configurable.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Checkpoint Granularity

**Affected sections:** Key Capabilities → Run Lifecycle; Configuration; Open Questions

**Summary:** A single `checkpoint` stage does not match the multi-boundary checkpoint implied by status `checkpointed` and the resume contract.

**Detail:** §Run Lifecycle:0062 lists `checkpoint` as a stage that runs once before `completion`. But FEAT-0015's `checkpointed` status (umbrella line 0099) and the protocol shape "stream events from a checkpoint" (umbrella line 0356) imply checkpoints occur at multiple points — minimally at every stage transition and on every durable artifact. §Open Questions Q2 acknowledges "minimum checkpoint data" is unresolved but the *frequency* and *atomicity* are also unspecified. §Configuration:0138 names "checkpoint retention" but not checkpoint *cadence*.

**Recommendation:** Reframe `checkpoint` as a *property of stage boundaries* rather than a discrete stage: "every legal stage transition produces a checkpoint; the listed `checkpoint` stage is the final pre-completion checkpoint." Add atomicity wording (write-then-rename or transactional) and defer the minimum-data schema to the ADR.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Vocabulary Consistency

**Affected sections:** Key Capabilities → Harness Responsibilities; UI / CLI / API Integration

**Summary:** `interrupt` is introduced here without alignment to FEAT-0015's `cancel`.

**Detail:** §Harness Responsibilities:0098 lists "interrupt, retry, continue, and fork actions". FEAT-0015 §Terminal UI:0335 uses `cancel`. These look semantically distinct (interrupt = pause/halt the active stream; cancel = terminal cancellation), but neither feature defines the difference. If interrupt is intended to be a separate verb (graceful pause, leaving status `running` or transitioning to `waiting_user`), it should be named in the umbrella's status set.

**Recommendation:** Either replace `interrupt` with `cancel` in §Harness Responsibilities, or define `interrupt` here as a distinct verb and add it to the umbrella's command/status vocabulary. Cross-reference resolved A2 in the FEAT-0015 architect+SRE review (cancellation semantics).

**Disposition:** accepted

---

### A5 — Significant

**Reviewer:** Architecture / Repair Stage

**Affected sections:** Key Capabilities → Run Lifecycle

**Summary:** Repair turns are referenced but absent from the stage model.

**Detail:** §Run Lifecycle:0070 says "the run may consume additional turns during its lifecycle, such as repair turns, validation triage turns, and clarification turns." None of these appear in the stage list. FEAT-0019 owns the repair-loop logic, but the *pipeline shape* must show where the loop reenters; otherwise FEAT-0019's repair contract has no defined coupling to the pipeline. Related to A1 but distinct: A1 is about transition edges, A5 is about whether repair is an in-stage activity or a sub-pipeline.

**Recommendation:** Add a sentence to §Run Lifecycle stating that repair, validation triage, and clarification turns reenter `model_call` (or `prompt_plan` when context must be revised) and that each reentry is a new turn within the same run.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Workflow Variability

**Affected sections:** Key Capabilities → Run Lifecycle; Configuration; Success Criteria

**Summary:** Pipeline shape per workflow type is unstated; the `simple chat` compat claim is unverifiable as written.

**Detail:** §Configuration:0137 names "default pipeline behavior per workflow type" but the stage list reads as required for every run. Pure conversational, `docs`, or `exploration` runs likely have no `tool_loop` or `validation`. Success Criterion 6 ("existing simple chat remains compatible and can be represented as a foreground run") implies a pipeline that can collapse to `preflight -> model_call -> completion` for cheap turns, but no such collapse is defined. Open Question 3 surfaces the same concern but does not commit a direction.

**Recommendation:** State that stages are *available* rather than *required*: each workflow type declares its required stages, and stages not used are skipped (recorded as `skipped` with a reason). Optionally: list a "minimal" stage set (preflight, model_call, completion) that simple-chat runs must use.

**Disposition:** accepted

---

### A7 — Advisory

**Reviewer:** Architecture / Run Origin

**Affected sections:** Key Capabilities → Run Lifecycle

**Summary:** "Run is created by one initiating user turn" excludes scheduled, sibling-spawned, or system-initiated runs.

**Detail:** §Run Lifecycle:0069 says "a run is created by one initiating user turn." FEAT-0015 §Foreground and Background Agents:0152 introduces sibling and synthesis runs that are not user-initiated; FEAT-0017 implies background runs may continue across reconnects without a fresh user turn; future scheduled-run capabilities (out of scope today, but hinted at by FEAT-0022) would also bypass this constraint.

**Recommendation:** Soften to "a run is created by an initiating event, typically a user turn but also a parent run's fork, a synthesis aggregation request, or a system-scheduled trigger". Record the initiator type as run metadata.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Stage Timeouts

**Affected sections:** Configuration

**Summary:** "Maximum stage duration before surfacing a warning" is the only stop-behavior; no transition-to-failed contract.

**Detail:** §Configuration:0139 commits only to a *warning* on long stages. There is no commitment to: a hard per-stage timeout that transitions the run to `failed` or `waiting_user` with a structured reason; per-stage default timeouts (model_call typically minutes, tool_loop typically minutes-to-hours, validation typically seconds-to-minutes); or how the timeout interacts with `waiting_permission`. A warning-only signal makes stuck-stage detection a UI affordance rather than an enforcement mechanism — see also FEAT-0015 architect+SRE review S2.

**Recommendation:** Define per-stage default deadlines and the boundary behavior at the deadline: transition to `failed` with reason `stage_timeout`, except `waiting_permission` and `waiting_user` which use the umbrella's permission timeout. Make warning thresholds a separate, lower configurable.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Cost and Token Attribution

**Affected sections:** Key Capabilities → BFF Responsibilities; Pipeline Events

**Summary:** Cost and tokens correlate to the run, not to stages, turns, or individual `model_call`s.

**Detail:** §BFF Responsibilities:0081 commits to correlating "stream events, tool calls, cost, and provider usage with the run". A multi-turn run with several `model_call`s plus parallel tool calls cannot be billed, routed, or capped without finer granularity. Per-stage cost is also necessary to drive FEAT-0022's quality routing decisions.

**Recommendation:** Commit to attribution at three levels: per `model_call_id` (provider/model/tokens/cost), per `tool_call_id` (executor/duration), and per stage (aggregated). Run-level totals are derived sums.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Provider Call Persistence

**Affected sections:** Key Capabilities → Run Lifecycle (`model_call` stage); Open Questions

**Summary:** What is persisted from a `model_call` (prompt, response, tool envelopes, secrets) is unspecified.

**Detail:** §Run Lifecycle:0056 names `model_call` but the feature does not say what content is captured: full assembled prompt, full streamed response, tool-call envelopes, system-prompt segments, secret-redacted variants, or only metadata. This affects replay, cost audit, FEAT-0020 patch evidence, and the redaction question in FEAT-0015 Open Question 5. Open Question 4 here flags the prompt-metadata-vs-protected-prompt tension but not the persistence shape.

**Recommendation:** Commit to persisting at minimum: the prompt-layer plan (not necessarily contents), the assembled tool definitions, the model identity and parameters, the full response stream events, tool-call envelopes, and provider usage. Defer the redaction policy to the artifact storage ADR. Make full-prompt capture a configurable.

**Disposition:** accepted

---

### S4 — Significant

**Reviewer:** SRE / Partial-failure Semantics

**Affected sections:** Key Capabilities → Run Lifecycle (`tool_loop` stage); Pipeline Events

**Summary:** `tool_loop` partial failure (one of N parallel tool calls fails) is not modeled.

**Detail:** With parallel tool calls (A2) and any non-trivial tool surface, partial failure is the common case — one tool times out while two succeed; one is denied by policy while the rest run. The feature does not say whether the run blocks waiting for all calls, surfaces results as they arrive, cancels remaining calls on first failure, or reports a structured partial-failure event. Without this, FEAT-0019's repair planning has no defined input shape and FEAT-0021's audit has no defined per-call disposition.

**Recommendation:** Define `tool_loop` failure as per-call: every `tool_call_id` resolves to `success | failure | denied | timeout | cancelled` independently; the loop continues until the model stops or policy halts the run. A run-level "tool_loop failed" event is only emitted when the model decides it cannot proceed.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / Stream Backpressure

**Affected sections:** Key Capabilities → Pipeline Events; Harness Responsibilities

**Summary:** BFF→harness event-stream backpressure is unspecified.

**Detail:** §Pipeline Events:0103-0114 lists event categories. The feature does not say what happens when the harness consumes events more slowly than the BFF emits them: drop, buffer with bounded size, throttle the upstream `model_call`, or block. This matters for high-rate stages (`tool_loop` with many calls, or a chatty `model_call`) and for slow attached harnesses on remote BFFs.

**Recommendation:** Commit to bounded buffering with a documented overflow policy (drop oldest non-essential progress events; never drop tool/artifact/checkpoint events; pause the model stream as a last resort). Detail can land in FEAT-0017 alongside the resume contract.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added state-graph language, legal reentry edges, and ADR deferral for transition policy and idempotency boundaries. |
| A2 | accepted | Clarified that `model_call` and `tool_loop` may overlap during streaming; added provider emission ordering and conservative concurrent-tool execution policy. |
| A3 | accepted | Reframed checkpoints as stage-boundary durability plus a final pre-completion checkpoint; added atomic BFF persistence language. |
| A4 | accepted | Aligned run controls with FEAT-0015 by replacing `interrupt` with `cancel` in responsibilities, UI, and success criteria. |
| A5 | accepted | Stated that repair, validation triage, and clarification turns reenter `model_call` or `prompt_plan` within the same run. |
| A6 | accepted | Clarified that stages are available rather than required, workflow types declare required/optional stages, skipped stages are recorded, and simple chat may use a minimal pipeline. |
| A7 | accepted | Replaced the single user-turn origin with initiating-event metadata covering user turns, parent forks, synthesis requests, and system-scheduled triggers. |
| S1 | accepted | Added per-stage warning thresholds, hard deadlines, default `stage_timeout` failure behavior, and delegation to umbrella waiting-state timeout policies. |
| S2 | accepted | Added attribution at `model_call_id`, `tool_call_id`, stage aggregate, and derived run-total levels. |
| S3 | accepted | Added minimum `model_call` persistence requirements and made full prompt-content capture configurable under redaction policy. |
| S4 | accepted | Added independent per-tool-call terminal states and clarified that run-level `tool_loop` failure occurs only when model or policy cannot proceed. |
| S5 | accepted | Added bounded BFF-to-harness event buffering, non-droppable essential events, and backpressure/liveness behavior when essential buffers are exhausted. |
