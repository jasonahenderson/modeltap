# FEAT-0008 / FEAT-0009 Interdependency Review

- Features:
  - `docs/features/0008-bff-server.md`
  - `docs/features/0009-terminal-harness.md`
- Review date: 2026-04-15
- Reviewer: peer review — cross-feature integration
- total_findings: 6
- blocking: 4
- significant: 2
- advisory: 0
- top_line: FEAT-0008 and FEAT-0009 are now directionally aligned on the harness/BFF split, but several protocol payloads, lifecycle behaviors, and responsibility boundaries still need to be locked down from both sides before either feature can be treated as implementation-ready.

## Review Scope

This review covers the dependency boundary between the BFF server and terminal harness:

- What FEAT-0009 assumes FEAT-0008 provides
- What FEAT-0008 assumes FEAT-0009 implements
- Protocol payloads and lifecycle gaps between the specs
- User-facing behavior that depends on BFF state
- Harness-owned behavior that the BFF must be able to represent to models

## Alignment Strengths

- The broad responsibility split is coherent: FEAT-0008 owns provider translation, routing, sessions, prompts, model registry, context, and server-side state; FEAT-0009 owns terminal UI, local tools, permission enforcement, and rendering.
- FEAT-0008 now defines connection lifecycle, health, diagnostics, and in-flight recovery; that gives FEAT-0009 a strong foundation for a resilient harness.
- FEAT-0009 now references FEAT-0008 protocol features directly: `capabilities.register`, `capabilities.update`, `model.selected`, `compact.plan`, `session.list`, `session.details`, and `reviewer_id` for multi-model output.
- The mode boundary is much clearer than earlier drafts: FEAT-0008 injects mode prompts, while FEAT-0009 enforces mode behavior on local tool calls.

## Findings

### F1 — Blocking

**Reviewer:** Connectivity Integration

**Affected sections:** FEAT-0008 Connection Lifecycle and Self-Recovery, FEAT-0008 Diagnostic Taxonomy, FEAT-0009 CLI Integration, FEAT-0009 Success Criteria

**Summary:** FEAT-0009 does not yet accept or render the connectivity lifecycle that FEAT-0008 now requires.

**Detail:** FEAT-0008 now defines managed connection states, auto-start behavior, heartbeat, readiness, `connection.health`, `connection.ready`, `session.sync`, and structured `MT-CONN-*` diagnostics. FEAT-0009 still treats connection as a single success criterion: connect, register capabilities, establish a session. It does not specify how the TUI renders `discovering`, `starting`, `degraded`, `reconnecting`, or `failed`; how it displays automatic repair attempts; how it exposes `modeltap server status`/unlock diagnostics in-session; or how it recovers UI state after `session.sync`.

**Recommendation:** Add a "Connection UX and Recovery" section to FEAT-0009. It should define status bar states, transient banners, retry progress, diagnostic rendering, reconnect behavior during an active stream, and user actions for `MT-CONN-*` failures. Add success criteria proving the harness renders and acts on FEAT-0008 connection lifecycle events.

### F2 — Blocking

**Reviewer:** Protocol Contract

**Affected sections:** FEAT-0008 Capability and tool registration, FEAT-0008 Tool call round-trips, FEAT-0009 Tool Execution, FEAT-0009 Permission Model

**Summary:** The tool catalog contract is not precise enough for FEAT-0009's built-in and MCP tool behavior.

**Detail:** FEAT-0008 requires `capabilities.register` to declare available tools with name, description, input schema, and permission level. FEAT-0009 defines many built-in tools but then says the model sees a unified `Read` tool while the harness internally dispatches to PDF, DOCX, image, spreadsheet, and text readers. FEAT-0009 also says permissions are entirely harness-enforced and the BFF has no knowledge of the current permission level, while FEAT-0008 says permission level is part of capability registration. The specs do not define the canonical tool schema, whether permission metadata is static capability metadata or dynamic policy, how MCP tools are namespaced, or how model capabilities like vision affect the tool catalog.

**Recommendation:** Define a shared tool catalog schema in FEAT-0008 and have FEAT-0009 conform to it. Separate static tool risk metadata from dynamic permission state. Specify namespacing for MCP tools, unified `Read` dispatch semantics, per-tool input/output envelopes, rejection/error payloads, and how capability changes trigger `capabilities.update`.

### F3 — Blocking

**Reviewer:** Pre-Turn Workflow

**Affected sections:** FEAT-0008 Harness Protocol, FEAT-0008 Model Routing Policy, FEAT-0009 Large Paste Handling

**Summary:** FEAT-0009's large-paste summarization path can require BFF model routing before `turn.submit`, but FEAT-0008 has no pre-turn summarization protocol.

**Detail:** FEAT-0009 says large paste summarization is performed harness-side before submission, "using a local model or the BFF's cheap routing target." If the harness calls a local model directly, it violates the core FEAT-0008 premise that the harness never speaks provider/model protocols. If it asks the BFF to summarize before `turn.submit`, FEAT-0008 needs a protocol method for pre-turn transformations that captures raw content, routes to a cheap model, returns transformed content, and avoids polluting the main conversation as a user turn.

**Recommendation:** Add a BFF method such as `content.transform` or `context.summarize` for pre-turn summarization, with capture semantics and cost attribution. Alternatively, change FEAT-0009 so large-paste summarization is deferred until after `turn.submit`, with the BFF owning summarization entirely.

### F4 — Blocking

**Reviewer:** Streaming and Recovery

**Affected sections:** FEAT-0008 Model Transparency, FEAT-0008 In-Flight Turn Recovery, FEAT-0009 Multi-model review display, FEAT-0009 Success Criteria

**Summary:** Multi-model stream display in FEAT-0009 depends on branch-level stream semantics that FEAT-0008 only sketches.

**Detail:** FEAT-0008 says multi-model roles emit interleaved `token.delta` events tagged with `reviewer_id`, and FEAT-0009 describes progressive per-reviewer display. But the shared contract does not define branch lifecycle events, branch completion/error semantics, aggregate `turn.complete` behavior, cancellation of one branch versus all branches, cost updates per branch, or `session.sync` output for partially completed multi-model turns. Without that, FEAT-0009 cannot implement reliable progressive rendering or reconnect recovery.

**Recommendation:** Extend FEAT-0008's protocol event schema for multi-model turns: `branch.started`, branch-tagged `token.delta`, branch-tagged `cost.update`, `branch.complete`, `branch.error`, aggregate `turn.complete`, and branch-aware `session.sync`. FEAT-0009 should reference those events explicitly in the multi-model display section and success criteria.

### F5 — Significant

**Reviewer:** Project Context Boundary

**Affected sections:** FEAT-0008 Project instructions, FEAT-0008 Session List and Details, FEAT-0009 Project Awareness, FEAT-0009 CLI Integration

**Summary:** Project root and project-instruction handoff are not specified in the harness/BFF protocol.

**Detail:** FEAT-0009 says `modeltap --project <path>` scopes the session and file operations are relative to the project root. FEAT-0008 says the server loads `.modeltap.yaml` or `MODELTAP.md` from the harness's project root and includes it in the system prompt. But the protocol tables do not define how the harness transmits the project root, how the BFF validates or stores it, whether it receives file paths as absolute or project-relative, or how project config updates interact with session resume.

**Recommendation:** Add project context fields to `session.resume`, new-session creation, and `turn.submit` as needed. Define canonical project identity, path normalization rules, security boundaries for file access, and whether the server reads project instruction files itself or receives their contents from the harness.

### F6 — Significant

**Reviewer:** Acceptance Alignment

**Affected sections:** FEAT-0008 Success Criteria, FEAT-0009 Success Criteria

**Summary:** FEAT-0009 acceptance criteria exercise behaviors that FEAT-0008 does not yet list as BFF acceptance gates.

**Detail:** FEAT-0009 success criteria require `/models`, session explorer, session details, compaction UI, model routing indicators, model override persistence, MCP capability updates, multi-model review display, and context command behavior. FEAT-0008 supports much of this in body text, but its success criteria still stop at broad connection, turn streaming, persistence, routing, cost, compaction, and concurrent connection behavior. The BFF feature can technically pass while leaving key harness requirements unproven.

**Recommendation:** Add FEAT-0008 success criteria that explicitly cover `model.list`, `model.selected`, `session.list`, `session.details`, `compact.plan`, `compact.apply`, `capabilities.update`, connection health, and branch-aware multi-model stream events. Then FEAT-0009 can depend on those behaviors as tested BFF contracts.

## Required Cross-Feature Contract Additions

Before accepting either feature, define these shared contracts:

1. Connection lifecycle UI contract: FEAT-0008 emits/returns states and diagnostics; FEAT-0009 renders and acts on them.
2. Tool catalog schema: names, descriptions, input schemas, output envelopes, risk metadata, dynamic permission handling, MCP namespaces, and rejection payloads.
3. Pre-turn content transformation: whether summarization is BFF-owned, harness-owned, or explicitly split.
4. Multi-model branch streaming: branch ids, lifecycle events, branch cost, branch error, aggregate completion, and reconnect sync.
5. Project/session initialization: project root, project identity, instruction loading, path normalization, and session resume semantics.
6. Acceptance crosswalk: every FEAT-0009 server-dependent success criterion maps to a FEAT-0008 success criterion or explicit protocol method.

## Bottom Line

FEAT-0008 and FEAT-0009 are close in architectural intent, but the harness is now depending on a richer BFF contract than FEAT-0008's acceptance criteria guarantee. The next useful step is not more product prose; it is a shared protocol appendix or cross-feature contract table that both specs reference.
