# FEAT-0015 through FEAT-0022 — Ownership and Authority Review

**Reviewer:** Claude Opus 4.7 (1M context)
**Date:** 2026-04-30
**Scope:** Authority and ownership boundaries between BFF and harness/local executor across the Professional Harness Runtime umbrella (FEAT-0015) and its seven member features (FEAT-0016–0022).
**Status:** All specs are `draft`.

## Top-line summary

| total_findings | blocking | significant | advisory | top_line |
|---|---|---|---|---|
| 8 | 0 | 5 | 3 | The BFF/harness split is correct in spirit; five authority boundaries are stated as responsibilities rather than as exclusive-authority contracts and need to say who decides, who reports, and who reconciles. |

## Verdict

The umbrella's ownership story is right: BFF orchestrates, harness/local executor enforces local side-effects, the protocol carries the boundary. Every FEAT reads coherently in isolation. **What is missing is the explicit authority verb** — "BFF stores X" and "harness manages X" can both be true without saying who *decides*, who *reports*, and what happens on conflict.

Five boundaries need an authority contract before Phase 1 opens. Three more deserve clarification but are tractable in design.

## Authority matrix (current, derived from specs)

| Concern | BFF | Harness/Local Executor | Authority is explicit? |
|---|---|---|---|
| Run lifecycle (create/destroy) | persists, emits | renders | partial — see F1 |
| Run status / pipeline stage transitions | persists, emits | reports facts | implicit — see F1 |
| Attachment state | "stores run state and progress" | "renders queue" | implicit — see F2 |
| Context plan | owns durable context plan | discovers local repo facts | clear |
| Prompt assembly / prompt-layer metadata | BFF-owned, prompt content not exposed by default | reads metadata only | clear (F8 advisory) |
| Routing decisions | BFF-owned (routes model call) | n/a | clear |
| Tool request | BFF requests | harness executes/denies | clear |
| Local tool execution | n/a | sole authority | clear |
| Permission decision (approve/deny/pause) | supplies policy | makes decision | clear in spirit, ambiguous on record authoring — see F3 |
| Permission decision record (audit) | "Tool decisions… run evidence" | "explains why" | implicit — see F3 |
| Workspace creation/cleanup | stores metadata | creates and manages | partial — see F4 |
| Workspace orphan recovery | — | — | unspecified — see F4 |
| Disconnected-executor run progress | — | — | open question — see F5 |
| Artifact metadata | stores | n/a | clear |
| Artifact content (large local files) | reference only | content owner | partial — see F6 |
| Memory candidate generation | generates from artifacts | surfaces for disposition | clear |

## What's clean (do not change)

- **Local-side-effect authority lives in the harness.** FEAT-0021 is unambiguous: *"local execution authority remains harness-owned."* The non-goal *"Do not move local side-effect enforcement fully into the BFF"* reinforces this.
- **Routing, prompt assembly, context plan are BFF-owned.** FEAT-0015, FEAT-0016, FEAT-0018, FEAT-0022 align on this.
- **Memory candidate generation is BFF-owned, disposition is user-via-harness.** FEAT-0022 §"Memory Candidates" implies BFF generation; `/memory accept|edit|reject` puts disposition on the user. Clean.
- **Tool request flow is correctly directional.** BFF requests, harness executes or denies, harness returns structured results. FEAT-0015 §"Tool Runtime Integration" and FEAT-0021 are consistent.
- **Prompt content is BFF-owned and not exposed to the harness by default.** FEAT-0018 §"Budgeting" + §"UI / CLI / API" — *"prompt-layer metadata that can be inspected without exposing secrets."*

## Findings

### F1 — Who advances run status and pipeline stage? — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0016 §"BFF Responsibilities", §"Pipeline Events"; FEAT-0017 §"Attachment Semantics"

FEAT-0016 says the BFF "persist run stage transitions" and emits stage events. It also says the harness can "provide interrupt, retry, continue, and fork actions against the run ID." Local-executor outcomes (tool result, permission denied, validation failure, user cancel from the harness composer) cause transitions — but the spec does not say whether the harness reports facts and the BFF advances state, or the harness can directly set state.

This matters for two reasons:
1. **Conflict resolution.** If the harness reports "tool denied" and the BFF concurrently emits "stage_changed → completion" because of a timeout, who wins?
2. **Multi-client.** If a run has one attached client and one observer, both could send conflicting actions (cancel vs continue). Without a single authority, the run can split state.

**Recommendation:** add to FEAT-0016 §"BFF Responsibilities" or a new §"State Authority":

> Run status and pipeline-stage transitions are BFF-authoritative. The harness reports facts (tool results, user actions taken locally, local errors, executor disconnect); the BFF integrates these reports and is the sole emitter of `status_changed` and `stage_changed` events. Harness rendering reflects the last BFF-emitted state. Harness commands (`/cancel`, `/retry`, `/continue`, `/fork`) are *requests* to the BFF; the BFF acknowledges and emits the resulting transition or rejects with reason.

### F2 — Attachment state semantics under multi-client and reconnect — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0017 §"Attachment Semantics", §"Resume After Restart"

FEAT-0017 says *"the BFF stores run state and progress. The harness renders the attached run in the conversation shell"* and lists four attachment states (`attached`, `detached`, `observable`, `blocked`). Recent revisions added current-session run scoping, which helps. Still missing:

- **Single-attached-client guarantee.** Can two harness instances (desktop + web) both claim `attached`? FEAT-0017's `observable` state implies a many-watchers model, but doesn't say whether `attached` is exclusive.
- **Disconnect handling.** If an attached harness disconnects without `/detach`, does the run go to `detached` immediately, after a grace period, or to `blocked` waiting for the same client?
- **Authority for the attachment transition.** Does the harness claim attachment, or does the BFF grant it? Today both are described as "the user attaches" with no actor.

**Recommendation:** add to FEAT-0017 §"Attachment Semantics":

> Attachment state is BFF-authoritative. A run has at most one attached client at a time; additional connected clients may observe (`observable`). The harness *requests* attach via `/attach <run-id>` and the BFF grants or rejects (already attached elsewhere, run terminal, etc.). Harness disconnect transitions an attached run to `detached` after a configurable grace period (default 60s); within the grace window the run continues per its background-policy posture for non-side-effect stages. The BFF is the sole authority that sets attachment state.

### F3 — Permission decision record authoring is ambiguous — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0021 §"Audit Trail", §"Solution"; FEAT-0020 §"Artifact Bundle"

FEAT-0021 is unambiguous about the *decision*: harness/local executor decides. It is ambiguous about the *record*. FEAT-0021 says decisions are "recorded as run evidence" and the harness "should explain why a decision happened." FEAT-0020 §"Artifact Bundle" lists "approval log" as a run artifact (BFF-stored). So the chain is implicit:

- harness *decides*
- harness *reports* the decision (over the protocol)
- BFF *records* it durably as the audit artifact

What is missing is a **conflict and revocation contract**:

- If the harness reports a decision the BFF can detect violates server/team policy (a stale local policy version, for instance), can the BFF revoke?
- Does the harness wait for BFF acknowledgment before letting the model see the tool result, or does it stream the result and accept retroactive revocation?

**Recommendation:** add to FEAT-0021 §"Audit Trail":

> Permission decisions are made by the harness/local executor (sole authority for local side-effects). The BFF supplies the policy context the harness uses to decide. After each decision, the harness reports the decision (`tool_call_id`, `decision_id`, outcome, policy source, reason) to the BFF, which is the durable audit record. The BFF may detect a policy mismatch (stale local policy version, server-policy override) and reject the report; on rejection, the BFF revokes the decision and the harness must not deliver the tool result to the model. Harness must acknowledge BFF policy version before running tool calls in a run; out-of-version reports are rejected.

### F4 — Workspace lifecycle gaps (cleanup, recovery, remote ownership) — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0015 §"Workspace Policy"; FEAT-0021 §"Workspace-Aware Execution"

FEAT-0015 says: *"The BFF stores workspace metadata on the run. The local harness/executor creates and manages local workspaces because it owns the filesystem."* Five canonical modes are named (`current`, `current_readonly`, `worktree`, `temp_copy`, `remote`). Four ownership questions are not answered:

- **Cleanup.** When a run completes/fails/cancels, who removes `worktree` and `temp_copy` workspaces? The harness, on its own schedule? On BFF instruction? On the next harness reconnect?
- **Reconnect orphans.** If the harness crashes mid-run, are old `worktree`/`temp_copy` directories orphaned? Who finds and cleans them on next harness start?
- **Missing workspace.** If a user manually `rm`s a temp workspace mid-run, who detects and how does the run respond?
- **Remote workspace ownership.** When `remote` mode is in use, the workspace lives in a server-side sandbox. Who owns its lifecycle — BFF (since the sandbox is server-controlled) or harness (consistent with the local-executor pattern)?

**Recommendation:** add a §"Workspace Lifecycle" subsection to FEAT-0015:

> The harness/local executor owns creation, mutation, and cleanup of `current`, `current_readonly`, `worktree`, and `temp_copy` workspaces. Cleanup of `worktree` and `temp_copy` workspaces happens on terminal run states (`completed`, `failed`, `cancelled`) by default; the harness reports cleanup completion to the BFF. On harness reconnect, the harness scans for orphaned workspaces (those whose run is no longer active per BFF) and cleans them with user notification. If a workspace becomes unexpectedly missing during an active run, the harness reports a `workspace_lost` fact and the BFF transitions the run to `failed` with that reason. `remote` mode workspaces are owned by the BFF or the remote sandbox provider; the harness participates only as the policy/permission relay.

### F5 — Disconnected-executor behavior is open-question, not contract — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0015 OQ#7, FEAT-0017 OQ#1, OQ#2; FEAT-0017 §"Background Permission Behavior"

FEAT-0015 OQ#7 and FEAT-0017 OQ#1 ask: *"Can background runs continue local tool execution when no harness/local executor is connected?"* The v0.3.0 plan WU-108 (Run runtime ADR) commits to settling this before implementation, which is correct process. But until the ADR is accepted, the umbrella has no contract for the most consequential ownership question: **what does a run *do* when no executor is available?**

The implicit answer in the current text is:
- BFF-only stages can advance: `preflight`, `context_plan`, `prompt_plan`, `model_call` (without tool calls), routing decisions, validation summarization (over already-captured logs)
- Local-side-effect stages cannot advance: `tool_loop` for any local tool, `validation` if it requires running checks, `artifact_capture` for filesystem evidence

**Recommendation:** add a §"Executor Availability" subsection to FEAT-0015 (or to FEAT-0017 §"Background Permission Behavior") that states the principle even before the ADR fully resolves:

> When a run requires a local side-effect tool and no harness/local executor is connected, the run pauses with status `waiting_user` and a clear "executor disconnected" reason. BFF-only stages may continue: prompt and context planning, model calls whose output is a final assistant message rather than a tool call, routing, and summarization over already-captured artifacts. The exact BFF-safe tool surface (server-side retrieval, BFF-side validation summarization, etc.) is enumerated by the run-runtime ADR. The principle is: the BFF never simulates local side-effects.

This codifies the safe behavior even before WU-108 ADR fills in the precise tool surface.

### F6 — Local-content artifact authority on harness loss — advisory

**Reviewer:** Implementation Readiness
**Severity:** advisory
**Affected sections:** FEAT-0020 §"Artifact Persistence"

FEAT-0020: *"The BFF stores artifact metadata and durable references. Large local files or logs may remain harness/executor-owned if they cannot safely be copied into BFF storage. The artifact record must say where and how the artifact can be read."*

Two missing edges:

- **Harness-loss detection.** If the harness instance that owned local content disappears (uninstall, machine wiped, different machine), the BFF artifact record points to nothing. Spec does not say whether BFF detects this or surfaces it to the user.
- **Multi-harness reconciliation.** If the user attaches from a different harness instance, the local content is unreachable from that one. Does the BFF surface "content unavailable from this client" or attempt some replication?

**Recommendation:** add to FEAT-0020 §"Artifact Persistence":

> BFF metadata is authoritative for artifact existence, identity, and provenance. Content may live locally (harness-owned) or in BFF blob storage. Locally-stored artifacts include a host-fingerprint so the BFF can detect when content is unreachable from the current harness instance and surface a `content_unavailable` state on read. The BFF never silently dereferences artifact records when local content is missing; the artifact remains listed with its metadata and a clear unavailability reason.

### F7 — BFF-side server-safe tool execution boundary unspecified — advisory

**Reviewer:** Architecture Conformance
**Severity:** advisory
**Affected sections:** FEAT-0017 OQ#2; FEAT-0021 §"Solution"

FEAT-0017 OQ#2: *"Should the BFF provide server-side tools for background-safe operations?"* Today the umbrella implies "no" — local-executor authority means local side-effects only happen in the harness. But "server-safe" tools (HTTP fetch, BFF-side retrieval, BFF-side validation summarization that doesn't run a process locally) could legitimately live BFF-side without violating the local-side-effect ownership.

This is partially F5's territory but worth calling out as a boundary question separate from disconnected execution.

**Recommendation:** add a §"Server-Safe Tools" subsection to FEAT-0021 (or FEAT-0017):

> The BFF may host a small set of server-safe tools that produce no local side-effects: BFF-side retrieval, BFF-side citation/quote extraction over already-captured artifacts, BFF-side summarization. Server-safe tools are subject to the same audit-record discipline (`tool_call_id`, `decision_id`, `result_id`) but do not require a connected harness. The set is enumerated by the run-runtime ADR and is intentionally minimal; ambiguous tools default to harness-owned.

### F8 — Prompt visibility to harness — advisory

**Reviewer:** Architecture Conformance
**Severity:** advisory
**Affected sections:** FEAT-0018 §"Budgeting", §"UI / CLI / API Integration"

FEAT-0018 says prompt-layer metadata can be inspected without exposing secrets. The implicit contract: harness sees metadata, BFF holds prompt content. This is the right design but not stated as an authority rule.

**Recommendation:** add a one-sentence statement to FEAT-0018:

> Full prompt content is BFF-owned and is not transmitted to the harness by default. The harness sees prompt-layer metadata (layer name, byte budget, source category, provenance) but not raw prompt text. A user-controlled debug flag may permit prompt content disclosure to the harness for inspection; this is off by default and recorded as a run artifact.

## Recommendations (priority order)

1. **F1** — state run status/stage authority explicitly in FEAT-0016. BFF-authoritative; harness reports facts, BFF emits transitions.
2. **F2** — settle attachment authority in FEAT-0017. BFF grants attach, single attached client, grace-period detach on disconnect.
3. **F3** — define decision record authoring + revocation contract in FEAT-0021.
4. **F4** — add workspace lifecycle subsection to FEAT-0015 covering cleanup, reconnect orphans, missing workspace, and remote ownership.
5. **F5** — codify the disconnected-executor principle in FEAT-0015 even before WU-108 ADR fully resolves.
6. **F6** — add harness-loss detection to FEAT-0020 artifact persistence.
7. **F7** — enumerate server-safe BFF-side tools in FEAT-0021.
8. **F8** — make prompt visibility an explicit BFF-only rule in FEAT-0018.

## Companion artifacts

- Identifier hygiene synthesis: `docs/features/.reviews/syntheses/0015-0022-id-hygiene-claude.md`
- Per-release plan reviews: `docs/releases/v0.3.0…v0.3.4/.reviews/claude-plan-review.md`
- Prior cross-feature review: `docs/features/.reviews/0015-0022-review-kimi.md`
- Per-feature findings: `docs/features/.reviews/0015-0022-*-claude-findings.{md,json}`

## Disposition

Processed on 2026-04-30.

| Finding | Disposition | Notes |
|---|---|---|
| F1 | accepted | Added BFF-authoritative run status/stage transition language to FEAT-0016; harness reports facts and commands are BFF requests. |
| F2 | accepted | Added BFF-authoritative attachment semantics, single attached client, observer clients, and disconnect grace-period behavior to FEAT-0017. |
| F3 | accepted | Added permission decision report/audit authority, policy-version acknowledgment, and rejection/revocation behavior to FEAT-0021. |
| F4 | accepted | Added workspace lifecycle ownership, cleanup, orphan scanning, missing workspace failure, and remote workspace ownership to FEAT-0015. |
| F5 | accepted | Added disconnected-executor rule to FEAT-0015 and FEAT-0017; local side effects pause while BFF-only stages may continue. |
| F6 | accepted | Added BFF metadata authority, host fingerprinting, and `content_unavailable` behavior for local artifact content to FEAT-0020. |
| F7 | accepted | Added server-safe BFF tool boundary to FEAT-0021 and narrowed FEAT-0017 open questions to exact tool-surface selection. |
| F8 | accepted | Added prompt visibility authority to FEAT-0018: full prompt content is BFF-owned and hidden from the harness by default. |
