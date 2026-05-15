# FEAT-0017 Findings (Architect + SRE pass)

- Feature: `.sdlc/features/0017-durable-runs-and-background-agents.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 8
- advisory: 4
- top_line: The attachment + queue model is the right shape and the BFF-authoritative grace-period rule is well stated. Gaps cluster around three areas: the run-state vocabulary collides with itself (`blocked` is both an attachment state and a UI grouping), the resume contract leans on FEAT-0015/0016 commitments that are not yet pinned, and the operability of background runs (notification SLA, queue fairness, in-flight tool fate at disconnect, retention coordination) is underdefined for production use.

## Findings

### A1 — Significant

**Reviewer:** Architecture / State Model

**Affected sections:** Key Capabilities → Attachment Semantics

**Summary:** `blocked` and `observable` overlap with run status and create a self-contradictory state list.

**Detail:** §Attachment Semantics:0044-0050 lists four attachment states including `blocked`. §Attachment Semantics:0064-0067 then says `blocked` is "a UI grouping for runs whose status is `waiting_permission` or `waiting_user`; it is not a separate lifecycle status." So the same word is both an attachment state and an explicit non-state. Separately, `observable` is listed as an attachment state but is functionally "a non-attached client that is watching" — which is a property of a *client*, not of the run.

**Recommendation:** Reduce attachment states to `attached` and `detached`. Move `observable` to a per-client subscription concept ("observers"). Remove `blocked` from the attachment-state list and keep it only as a UI grouping over status, as the later text already does.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Resume Contract

**Affected sections:** Key Capabilities → Resume After Restart

**Summary:** Resume contract is a disjunction of three behaviors with no commitment to which.

**Detail:** §Resume After Restart:0114-0119 says the BFF "may replay full events, summarize missed events, or show checkpointed state depending on retention and protocol support." That is three different reconnect contracts: full event replay (idempotency-sensitive), summary-only (loses fidelity), and checkpoint-only (loses post-checkpoint progress). FEAT-0017 success criterion 5 ("harness restart does not erase BFF-known active runs") is satisfied by all three but means very different things to a downstream consumer. This directly depends on FEAT-0015 architect+SRE A1 (event delivery semantics) and A3 (idempotency) — neither of which is pinned.

**Recommendation:** Commit here to "replay from a sequence number when within retention; otherwise summary plus checkpoint." Defer the precise sequence-number contract to the Run Runtime ADR. Cross-reference FEAT-0015 A1/A3 explicitly.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Grace-Period Continuation

**Affected sections:** Key Capabilities → Attachment Semantics

**Summary:** Stages that may continue during the disconnect grace period are not enumerated.

**Detail:** §Attachment Semantics:0058-0062 says "during the grace period, the run may continue only according to its configured background-policy posture for stages that do not require local side effects." If the disconnected harness *was* the local executor, then any tool-loop work requires local side effects and must pause. Conversely, BFF-only stages (`prompt_plan`, in-flight `model_call`) can continue. This is implied but not stated, and intersects with FEAT-0021 §Server-Safe Tools.

**Recommendation:** Enumerate the stages that may continue during grace: `prompt_plan`, in-flight `model_call`, `artifact_capture` over already-captured content. Pause `tool_loop` and `validation` if they require the disconnected executor. Cross-link to FEAT-0021 server-safe tool surface.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Multi-Client Promotion

**Affected sections:** Key Capabilities → Attachment Semantics

**Summary:** Observer-to-attached promotion is unspecified when the attached client disconnects.

**Detail:** §Attachment Semantics:0056-0058 states "a run has at most one attached client at a time; additional connected clients may observe it." If two clients observe a run and the attached client disconnects past the grace period, who wins the next attach? First-come, last-write, deterministic by client ID, or no automatic promotion (run drops to `detached`)? Not specified. FEAT-0015 architect+SRE A4 raised the multi-attach lease at the umbrella; FEAT-0017 is the natural home for the contract.

**Recommendation:** State that attachment never auto-promotes: on disconnect past grace, the run becomes `detached` and clients must explicitly `/attach`. First valid attach claim wins; concurrent attach requests are serialized BFF-side.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Forward Binding

**Affected sections:** Key Capabilities → Run Queue

**Summary:** Queue rows display "current stage" while the stage taxonomy is provisional.

**Detail:** §Run Queue:0099-0101 says each queue row should include "current stage". FEAT-0016's stage list is acknowledged as provisional (FEAT-0016 architect+SRE A1 made it a graph, not a sequence). The harness queue UI is already binding on a vocabulary that may shift.

**Recommendation:** Either say "current pipeline phase (per FEAT-0016 stage taxonomy when available)" or commit to a small, stable display vocabulary (e.g. "planning / running / validating / waiting / done") that maps onto FEAT-0016 stages but does not bind to them.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Cancellation and Workspace

**Affected sections:** UI / CLI / API Integration; Non-Goals

**Summary:** `/cancel <run-id>` does not link to workspace cleanup contracts in FEAT-0015 §Workspace Policy.

**Detail:** §UI / CLI / API Integration:0128 lists `/cancel` but does not say what happens to the run's `worktree` or `temp_copy` workspaces on cancel, whether artifacts captured pre-cancel are retained, or whether sibling runs cancel-cascade. FEAT-0015 architect+SRE A2 raised this at umbrella level; FEAT-0017 is one of the homes for the contract.

**Recommendation:** State that cancel triggers workspace cleanup per FEAT-0015 §Workspace Policy:0249-0257, retains all artifacts captured before the cancel signal, and does not cascade to sibling runs unless the workflow declares cascade-on-cancel.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Notification SLA

**Affected sections:** Configuration

**Summary:** Notification behavior is configurable but has no time-sensitivity contract.

**Detail:** §Configuration:0151 lists "notification behavior for blocked or completed runs" with no contract on when notifications fire, dedup, escalation, or quiet hours. Blocked runs are time-sensitive (the run is paused until the user acts); a delayed or dropped notification effectively halts the pipeline. With multiple background runs queued, notification storms or notification gaps both degrade the surface.

**Recommendation:** Commit to: blocked-run notifications fire within a bounded delay (e.g. 5s) of the `waiting_permission` / `waiting_user` transition; identical-cause notifications coalesce; completed-run notifications are best-effort; notification surface is enumerated (terminal, OS, webhook) with per-surface defaults.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Queue Fairness

**Affected sections:** Configuration; Run Queue

**Summary:** Concurrency limits exist; scheduling, priority, and starvation prevention do not.

**Detail:** §Configuration:0148 names "maximum concurrent background runs" but the queue model does not define how queued runs are admitted when slots free up. With multiple workflows competing (one long `implementation` blocking three quick `docs`), naive FIFO leads to starvation. Without priority or fairness, an attached `/explore` cannot be quickly scheduled while a long detached run holds the only slot.

**Recommendation:** Commit to a default scheduling policy (FIFO is acceptable as a first cut, with explicit "no priority in v1") and name the extension surface (workflow priority, run age boost) for future work. State that foreground-promoted runs do not consume background slots.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / In-flight Tool Fate at Disconnect

**Affected sections:** Key Capabilities → Background Permission Behavior; Attachment Semantics

**Summary:** A `tool_loop` mid-execution at the moment the harness disconnects has no defined fate.

**Detail:** §Background Permission Behavior:0083-0085 covers the case where a tool *requires* local side effects and no executor is connected: the run pauses with `waiting_user`. But if a tool was already running on the local executor at the moment of disconnect (subprocess executing, file write in progress), the spec is silent: does the harness abort it, let it complete, treat the result as void, or report partial progress? This intersects with the cancellation contract (A6) and FEAT-0015 architect+SRE A2.

**Recommendation:** State that tools already running at disconnect are allowed to complete locally, and on reconnect the harness reports each tool's terminal result; if the run reached the grace-period deadline without reconnect, the BFF transitions the run to `failed` with reason `executor_disconnected_during_tool_call`, and pending results from the eventual reconnect are recorded as-is for forensic value.

**Disposition:** accepted

---

### S4 — Significant

**Reviewer:** SRE / Retention Coordination

**Affected sections:** Configuration; Resume After Restart

**Summary:** Run retention, transcript retention, and artifact retention do not coordinate.

**Detail:** §Configuration:0149-0150 names "blocked-run retention" and "completed-run retention." FEAT-0020 §Configuration:0114 names "artifact retention." There is no commitment to whether artifacts can outlive their run, whether transcripts age out before the run record, or whether `/attach` to a recent run guarantees its transcript is still available. A user reattaching at retention-edge could see metadata with no transcript or artifacts.

**Recommendation:** Commit to a precedence: artifact retention >= run retention >= transcript retention; or pin them as a single "run retention" envelope with sub-knobs that cannot be set lower than the run record's TTL. Defer to FEAT-0020 for artifact specifics but cite the rule here.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / Run Liveness

**Affected sections:** Key Capabilities → Run Queue

**Summary:** No heartbeat or stuck-run detection for background runs.

**Detail:** A background run that gets stuck (model_call hangs, a tool subprocess never returns, a permission request lost in transit) is indistinguishable in the queue from a slow run. §Run Queue:0099-0101 displays elapsed time, which lets a human notice but does not help an operator. Echoes FEAT-0015 architect+SRE S2 at the queue level.

**Recommendation:** Surface a `stuck` queue badge when a run has not advanced its stage or sequence number for a configurable interval (default 5 min for active stages, longer for `waiting_*`). Detail can defer to a future operability feature.

**Disposition:** accepted

---

### S6 — Advisory

**Reviewer:** SRE / Notification PII

**Affected sections:** Configuration

**Summary:** Completed-run notifications could leak transcript content over OS or webhook surfaces.

**Detail:** §Configuration:0151 names notification behavior but does not constrain content. A "run completed" notification that includes the run summary or artifact preview leaks data to the OS notification daemon (which is often persistent) or to webhook recipients. For team/enterprise this is a privacy concern.

**Recommendation:** Default notifications to title-only ("Run X completed", no content). Make richer content opt-in per surface and per workflow, recorded as a run artifact.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Reduced run attachment state to `attached`/`detached`; moved observers to per-client subscriptions and kept `blocked` as a UI grouping. |
| A2 | accepted | Pinned resume behavior to replay from sequence number within retention, otherwise summary plus latest checkpoint with fidelity note. |
| A3 | accepted | Enumerated stages that may continue during disconnect grace and paused executor-required `tool_loop`/`validation`; referenced FEAT-0021 for server-safe tools. |
| A4 | accepted | Added no-auto-promotion rule and BFF-serialized first-valid attach claims. |
| A5 | accepted | Changed queue rows to display mapped pipeline phase rather than binding directly to raw stage vocabulary. |
| A6 | accepted | Linked `/cancel` to FEAT-0015 cancellation/workspace cleanup, artifact retention, and cascade-on-cancel policy. |
| S1 | accepted | Added bounded blocked-run notification delay, coalescing, best-effort completion notifications, and title-only defaults. |
| S2 | accepted | Added FIFO default scheduler, no-priority first slice, foreground slot exclusion, and future priority extension surface. |
| S3 | accepted | Defined in-flight tool fate on disconnect, executor-disconnected failure reason, and forensic handling of late results. |
| S4 | accepted | Added FEAT-0015 retention-envelope reference and inspection requirement for recent runs. |
| S5 | accepted | Added `stuck` queue badge based on stage/event inactivity threshold. |
| S6 | accepted | Made notification content title-only by default with opt-in richer content per surface/workflow. |
