# Protocol Types Bundle (WU-040 + 041 + 093) Pre-Review Lint — Claude Subagent

**Reviewer:** Claude subagent (fresh context, same-model pre-review — not Tier C peer review)
**Date:** 2026-04-16
**Subject:** .sdlc/history/2026-04-16-design-protocol-types-040-041-093.md
**Bundle:** WU-040 (streaming events) + WU-041 (response types) + WU-093 (conformance fixtures)

## Reviewer caveat

Same-model lint: shares the Designer's training distribution, tokenizer, and reasoning heuristics. Most likely to catch mechanical drift, missed FEAT-0008 fields, scope gaps, and cross-WU inconsistency. Least likely to catch Claude-characteristic reasoning blind spots (e.g., uncritical acceptance of plausible-looking symmetry). Does not substitute for a cross-model Tier-C peer review.

## Summary

The bundled design is substantially complete and materially correct — it accounts for every event and response type that FEAT-0008 and track-0-shared.md enumerate, handles the WU-039 →WU-040/041 seam (envelope, ToolDefinition relocation, Notification wrapping) without regression, and addresses the hardest polymorphism case (`ModelSelected` single-vs-array) pragmatically. Three blocking issues: (B-01) an unspecified JSON-RPC Response payload for `turn.submit` leaves the streaming-initiation shape undefined; (B-02) the design repeatedly invokes "FEAT-0008 ambiguity #4" and "#9" and "the 12 FEAT-0008 ambiguities" but no such enumerated list exists anywhere in the repo, so review gates cannot verify dispositions; (B-03) D7 and the line-190 note describe the `ToolDefinition` / `CompactPlan` relocation using cross-package language ("re-export", "imports from", "`compact.CompactPlan`") — but `internal/protocol/` is a single package, so this wording is either wrong or masks a package-split the design doesn't actually propose. Several attention items flag fields left untyped in the catalog, FEAT-0008 `ServerError.code` semantics vs. `ErrorObject.Code`, the `RoutingPolicy.Resolve` helper crossing the "types-only" scope boundary, and missing explicit enumeration of all 12 `MT-CONN-*` constants.

## Blocking findings

### B-01. `turn.submit` JSON-RPC Response payload is unspecified

- **What:** The design documents Response types for 17 of 20 harness→server methods and documents the "no Response struct — stream behavior" disposition for `turn.cancel` and `tool.result`. It does not document what goes in `Response.Result` for `turn.submit`. FEAT-0008 §"Stream lifecycle" (line 56) says `turn.submit` "initiates a streaming response" terminated by `turn.complete` or `error`, but the envelope itself (JSON-RPC 2.0) is request/response correlated — every `Request` with an ID gets a `Response` with the same ID. WU-046 dispatch must know what to put in that `Result` field.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:268` — "(`turn.cancel` and `tool.result` have no direct response — stream behavior. Documented inline in `messages.go` godoc, not a struct.)" Explicit for two methods; silent on `turn.submit`.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:259-267` — "messages.go — WU-041 response pair additions" table lists only `ContentTransformResponse` and `CapabilitiesUpdateResponse`.
  - `internal/protocol/messages.go:85-98` — `ToolResultRequest` alias already encodes a specific protocol-freeze contract for the paired wire shape; the same treatment is owed to `turn.submit`.
  - FEAT-0008 §"Stream lifecycle" (line 56) and §"Idempotency rules" (line 452) — an idempotent replay of `turn.submit` "returns the existing turn state", implying a non-trivial `Result` payload at least on the replay path.
- **Why blocking:** WU-093 coverage asserts a fixture per exported type. WU-046 dispatch needs to know whether to return `Result: null`, `Result: {}`, `Result: TurnAck{turn_id}`, or the final `TurnComplete` payload on idempotent replay. Downstream Track B (WU-073, WU-074) assumes a stable shape for the initial RPC ack. Leaving this silent repeats exactly the kind of gap that tripped WU-039's `ConnectionReady` omission.
- **Suggested fix:** Add a paragraph to D1 (or a new `D11`) stating explicitly: (a) what `Response.Result` contains for `turn.submit` on first submission, (b) what it contains on idempotent replay (turn in flight vs. turn completed vs. turn errored), and (c) whether a new `TurnSubmitResponse`/`TurnAck` struct is introduced. Apply the same treatment to `turn.cancel` and `tool.result` explicitly — the current prose "stream behavior" is not a wire spec. A fixture covering each Response case follows.

### B-02. Design invokes "FEAT-0008 ambiguities #4 / #9 / the 12 ambiguities" but no such enumerated list exists

- **What:** The design references "FEAT-0008 ambiguity #4" (line 70), "FEAT-0008 ambiguity #9" (line 75), and "the 12 FEAT-0008 ambiguities each resolved or explicitly deferred" (line 375). It also defers numbered ambiguities `#3`, `#5`, `#6`, `#8`, `#10`, `#11`, `#12` in the Risks section (lines 322-329). No numbered-ambiguity list exists in `.sdlc/features/0008-bff-server.md`, in `.sdlc/history/` session notes, in `.reviews/` artifacts, or in `track-0-shared.md`. `FEAT-0008` has only two "Resolved Questions" and two "Open Questions" at the tail (lines 1362-1374), and those are unnumbered across the doc.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:70` — "resolves FEAT-0008 ambiguity #4"
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:75` — "(resolves FEAT-0008 ambiguity #9)"
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:321-329` — cross-refs #9, #3, #5, #6, #8, #10, #11, #12
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:375` — "are the 12 FEAT-0008 ambiguities each resolved or explicitly deferred with the right owner?"
  - `.sdlc/features/0008-bff-server.md:1362-1374` — "Resolved Questions" (2) and "Open Questions" (2); no numbered ambiguity list, no 12 items total.
  - `Grep "ambiguit" in docs/` — only this design doc references them by number. No upstream definition.
- **Why blocking:** Review-gate completeness cannot be verified. The design's risk register and disposition claims point at a list that does not exist, which means (a) reviewers cannot check that each ambiguity is actually resolved or deferred, (b) readers cannot trace dispositions to source, and (c) future WUs inheriting these deferrals (WU-060, WU-061, WU-056, WU-049, WU-050, WU-064, WU-059) have no canonical list to consult. This is the same class of drift the WU-039 Codex review caught (`omitempty` hidden from the spec by symmetry).
- **Suggested fix:** Either (a) add a new section "FEAT-0008 Ambiguities (canonical enumeration)" to `.sdlc/features/0008-bff-server.md` numbering the twelve so the design can cite `FEAT-0008 ambiguity #N` with an authoritative reference, or (b) replace each `#N` reference in this design with the actual ambiguous topic — e.g., `ambiguity #4` becomes `"MT-CONN-* code versioning scheme"`; `ambiguity #9` becomes `"server_capabilities shape"`. Option (a) is cheaper if the twelve are real and just never landed in the spec. Option (b) is cheaper if the numbering was a session artifact and never canonical.

### B-03. D7 (`ToolDefinition` relocation) and line 190 (`CompactPlan` shared type) describe same-package types as if they were cross-package

- **What:** `internal/protocol/` is a single Go package. Files within it share the package scope. D7 says "`messages.go` retains it as a re-export or import" and "`messages.go` imports from `tools.go`" — neither concept exists in Go for same-package files. Line 190 says "event dispatch references `compact.CompactPlan`" — which would be a reference from a hypothetical other package `compact`, but the `compact.go` file lives inside `internal/protocol/` per the D9 and §"File Layout" sections.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:73` — "`messages.go` retains it as a re-export or import. **Decision:** move declaration to `tools.go`, adjust `messages.go` imports."
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:190` — "**Single type `CompactPlan` defined in `compact.go`** is used by both paths. WU-040 `events.go` does not redefine it; event dispatch references `compact.CompactPlan`."
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:119-138` — File Layout box shows all files under `internal/protocol/` — same package.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:336-338` — Cross-Bundle Consistency check #1/#2/#3 repeat the same language ("sessions.go imports from messages.go") that is only meaningful across packages.
- **Why blocking:** A Backend Implementer reading this literally will either (a) create a new `internal/protocol/compact/` sub-package and discover that breaks all the cross-file references, (b) write `import` lines that don't compile, or (c) silently do the right thing and diverge from the design. The type-aliasing remark in D7 ("re-export") is a red herring — you don't need a type alias when two files in the same package declare one type. The correct phrasing is "`ToolDefinition` is declared once in `tools.go`; `messages.go` references it by bare name." This is purely editorial but needs fixing before Phase 3.
- **Suggested fix:** Replace cross-package language with same-package language throughout: (a) D7 → "declaration moves from `messages.go` to `tools.go`; `messages.go` continues to reference `ToolDefinition` by bare name within the same package — no alias or import needed."; (b) Line 190 → "`events.go` references `CompactPlan` directly (same package); no new type."; (c) Cross-Bundle Consistency items #1-#3 → drop "imports from" language.

## Attention findings

### A-01. `ServerError.code` semantics vs. `ErrorObject.Code` are not disambiguated

- **What:** The `ServerError` event (events.go) has a `code (R)` field (type unspecified in the catalog table). The JSON-RPC envelope `ErrorObject.Code` (protocol.go, WU-039) is `int`. FEAT-0008 §"Diagnostic Taxonomy" line 516 says the structured `error` event carries `code` as the MT-CONN-* diagnostic code (a typed string). These are two distinct `code` fields on two different payloads, but the design table doesn't state `ServerError.code` as `DiagnosticCode` explicitly.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:186` — `ServerError` row: "`turn_id` (O, omitempty), `code` (R), `message` (R), `diagnostic` (R, `Diagnostic` from errors.go)"
  - `internal/protocol/protocol.go:134-138` — `ErrorObject.Code int`
  - `.sdlc/features/0008-bff-server.md:516` — "structured `error` event with fields: `code`, `category`, `cause`, ..." (code is the MT-CONN-* string)
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:248` — `Diagnostic.Code DiagnosticCode` already carries the code; why does `ServerError` also carry one at the top level?
- **Recommended disposition:** **fix in design now** — (a) explicitly type `ServerError.Code DiagnosticCode` in the catalog, and (b) decide whether `ServerError.code` is redundant with `Diagnostic.code` (which lives at `ServerError.diagnostic.code`). If redundant, drop one. If different (e.g., outer is coarse category, inner is the specific MT-CONN-*), say so. Either way, name the Go type in the catalog so the Backend Implementer doesn't guess.

### A-02. `RoutingPolicy.Resolve` helper crosses the "types-only" scope boundary

- **What:** `models.go` row declares `RoutingPolicy` with helper `Resolve(role string) ([]string, error)`. WU-039 DoD states the package contains "only types and serialization — no business logic." WU-040 and WU-041 inherit that DoD (track-0-shared.md lines 18, 33, 50). A `Resolve` that parses dot-path role names and walks the fallback tree is logic, not types/serialization. FEAT-0008 §"Routing Policy" (line 788-798) specifies the resolution algorithm — that's a handler concern, not a protocol-types concern. Precedent: `Mode.Valid()` is the only behavior in WU-039 and is explicitly called out as trivial.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:229` — "`RoutingPolicy` | Dot-path role name → model name or array. Represented as `map[string]json.RawMessage` to allow string-or-array values. Helpers: `Resolve(role string) ([]string, error)`."
  - `.sdlc/releases/v0.2.0/track-0-shared.md:18,33,50` — WU-039/040/041 all carry "only types and serialization — no business logic."
  - WU-039 D7 precedent: `Mode.Valid()` is the only tolerated helper, and even that is one trivial switch.
- **Recommended disposition:** **fix in design now** — either (a) remove `Resolve` from the WU-041 scope and defer it to a later handler WU (WU-059 or WU-060 own model-selection logic), or (b) justify why resolution logic belongs in the shared protocol package. Option (a) is aligned with precedent; option (b) needs an explicit carve-out because WU-059/WU-060 consumers would otherwise have no canonical place to look. Same-model-specific note: the design's inclusion of `Resolve` alongside `IsMulti`/`SingleModel`/`MultiModels` helpers for `ModelSelected` shows a pattern of "add a helper where the wire shape is awkward." `ModelSelected` helpers are arguably JSON-polymorphism ergonomics (defensible). `RoutingPolicy.Resolve` is policy algorithm (not defensible here).

### A-03. All 12 `MT-CONN-*` constants are not explicitly enumerated in the design

- **What:** D6 shows one constant (`DiagServiceNotRunning DiagnosticCode = "MT-CONN-001"`) and comments `// ... MT-CONN-002 through MT-CONN-012`. FEAT-0008 §"Diagnostic Taxonomy" lines 503-514 enumerates all 12 with category names (`service_not_running`, `stale_socket`, `socket_permission`, `version_mismatch`, `tls_untrusted`, `auth_expired`, `storage_unready`, `session_locked`, `provider_unavailable`, `capability_registration_failed`, `model_unavailable`, `heartbeat_timeout`). A type catalog aimed at Tester + Backend Implementer should list every constant so the test list is mechanical.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:62-69` — D6 shows one constant, elides the rest.
  - `.sdlc/features/0008-bff-server.md:503-514` — full table with all 12.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:316` — "Pin a fixture for each of the 12 diagnostic codes so the `DiagnosticCode` typed enum catches any wire-value drift." — but the 12 constant Go names are not written out.
- **Recommended disposition:** **fix in design now (cheap)** — replace the `// ... MT-CONN-002 through MT-CONN-012` comment with the full 12-constant block. Suggested names following the shipped `DiagServiceNotRunning` pattern: `DiagStaleSocket`, `DiagSocketPermission`, `DiagVersionMismatch`, `DiagTLSUntrusted`, `DiagAuthExpired`, `DiagStorageUnready`, `DiagSessionLocked`, `DiagProviderUnavailable`, `DiagCapabilityRegistrationFailed`, `DiagModelUnavailable`, `DiagHeartbeatTimeout`. FEAT-0008 line 1337-1341 already seeds this in the example code block. Precedent: WU-039 Codex review caught missing-constant enumeration as a class of defect (the 19→20 "`ConnectionReady`" omission).

### A-04. Field Go types are inconsistently specified in the catalog

- **What:** Some rows specify Go types (e.g., `input (R, json.RawMessage — tool's input_schema)`, `tokens_freed (O, int)`, `relevance (R, float 0-1)`, `cancelled (R, bool)`), some imply the type from context, and others give only `(R)` or `(O)`. A Tester writing round-trip instances and a Backend Implementer writing structs both need the Go type.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:184` — `TurnComplete` has `cancelled (R, bool)` explicit, but `final_input_tokens`, `final_output_tokens`, `latency_ms` are just `(R)`.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:181` — `CostUpdate.input_tokens/output_tokens` no type; `input_cost/output_cost/total_cost` no type.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:186` — `ServerError.code (R), message (R), diagnostic (R, Diagnostic from errors.go)` — `code` and `message` are untyped (see A-01).
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:206-221` — sessions.go rows almost universally omit Go types.
- **Recommended disposition:** **fix in design now** — add a single "field types are Go-typed; see next column" column OR a per-row type annotation wherever the type is ambiguous (int vs. int64, float vs. float64, string vs. typed enum, bool vs. *bool). Cost-related numeric fields specifically need float64 (dollars) vs. int (tokens) disambiguation. This is a mechanical pass but meaningfully reduces reviewer-and-implementer effort.

### A-05. `CompactNotice.triggered_by` enum is narrowed to one value

- **What:** Design row says `triggered_by (R — "threshold_exceeded")`. FEAT-0008 §"Auto-compaction" (line 1015) says "at 92% context usage, the server runs the same analysis but applies its default recommendations automatically" — implying threshold_exceeded is the primary trigger, but the events table entry (line 200) just says "Auto-compaction applied (what was compressed, what was retained)" without specifying a closed enum. Nothing in FEAT-0008 forbids additional triggers (e.g., `manual_apply`, `forced_by_admin`).
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:183` — `CompactNotice` row: `triggered_by (R — "threshold_exceeded")`
  - `.sdlc/features/0008-bff-server.md:200,1015` — only auto-threshold is described as a trigger, but no closed enum stated.
- **Recommended disposition:** **fix in design now** — either (a) call out `triggered_by` as a free-form string with `"threshold_exceeded"` as one known value, or (b) enumerate a proper closed set if the Designer believes that's the spec intent. Option (a) matches FEAT-0008's current looseness; option (b) requires a FEAT-0008 amendment to ratify the closed set.

### A-06. `CapabilitiesRequestEvent.reason` enum extends FEAT-0008 without spec update

- **What:** Design row: `reason (O, omitempty) — "reconnection" / "server_restart" / "tool_schema_drift"`. FEAT-0008 line 208 says `capabilities.request` is "triggered on reconnect or when server detects tool schema drift" — only two triggers. The design adds `server_restart` as a third value.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:187` — three enum values.
  - `.sdlc/features/0008-bff-server.md:208` — two triggers mentioned.
- **Recommended disposition:** **defer to FEAT-0008 amendment or drop the third** — either amend the spec or remove `"server_restart"` from the design catalog until ratified. This kind of "design expands enum under pressure" is exactly what the protocol-freeze contract is meant to prevent.

### A-07. `ActiveTurnState.status` and `ReviewerState.status` enums include values FEAT-0008 does not show

- **What:** Design has `ActiveTurnState.status` enum = `pending_tool_result/streaming/complete/error/cancelled` and `ReviewerState.status` enum = `complete/streaming/failed/pending`. FEAT-0008 §"In-Flight Turn Recovery" only shows `pending_tool_result` (line 467) and `streaming` (line 481) by example; the rest are design-inferred.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:212,215` — both enums listed in design.
  - `.sdlc/features/0008-bff-server.md:460-494` — only `pending_tool_result` and `streaming` appear literally.
- **Recommended disposition:** **fix in design now (cheap)** — either (a) annotate each added value with "design-inferred, not in FEAT-0008" and flag for spec amendment, or (b) justify each inferred value inline. The values themselves look reasonable, but uninspected additions are how specs drift from types.

### A-08. `SessionDetail` name breaks the `*Response` suffix pattern

- **What:** Every other response type follows `<Verb>Response` (SessionListResponse, SessionResumeResponse, ContextListResponse, ModelListResponse, ModelSwitchResponse, etc.). `SessionDetail` has no suffix. The design does not name a `SessionDetailsResponse`; the `SessionDetail` type IS the response to `session.details`.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:208` — `SessionDetail | Response to session.details.`
- **Recommended disposition:** **fix in design now** — either rename to `SessionDetailsResponse` for consistency, or document the naming decision explicitly (e.g., "SessionDetail is the data type; it's also the shape returned from session.details — no separate wrapper"). The WU-041 spec in track-0-shared.md line 43 says "`SessionSummary`, `SessionDetail`, `TurnSummary`, `ServerEvent` payloads" — so the spec itself uses `SessionDetail`. Consistency lint still calls for an explicit note.

### A-09. Design renames spec types without flagging the rename in the spec update log

- **What:** The design renames two types from the WU-041 track spec:
  - Track-0 WU-041 spec line 43: "`ServerEvent` payloads" → Design uses `ServerSessionEvent` (to avoid clash with `protocol.ServerError`; explicitly called out in design line 210).
  - Track-0 WU-041 spec line 48: "`CompactPlanResponse`" → Design uses `CompactPlan` (unified event+response, called out at line 190 and again in compact.go row line 256).
  - Track-0 WU-041 spec line 44: "`ModelSelectedEvent` payloads" listed under `models.go` → Design keeps `ModelSelected` in `events.go` (WU-040) (implicit, not called out).
- **Evidence:**
  - `.sdlc/releases/v0.2.0/track-0-shared.md:43-48`
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:190,210,256`
- **Recommended disposition:** **fix in design now (cheap)** — add a short section "Deviations from track-0-shared.md WU-041 type names" listing these three renames/relocations so the TPM can update the track spec in lockstep and downstream WUs don't grep for the wrong name.

### A-10. No fixture policy for nested-only types (`Attachment`, `Paste`, `ToolResult`, `ProjectContext`, `Diagnostic`, etc.)

- **What:** D10 point 4 (coverage) says "reflection walks every exported type in `internal/protocol/`" and asserts each has "at least one fixture." Nested types like `Attachment` ship only inside `TurnSubmit.attachments[]`. Will `TestFixtureCoverage` accept a fixture for a parent type as covering the nested type? Or does it require a standalone top-level fixture?
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:113-114` — "Coverage registry: reflection walks every exported type ... asserts each has at least one fixture."
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:298-306` — coverage test snippet asserts per-type; does not say whether nested-within-parent counts.
- **Recommended disposition:** **fix in design now (cheap)** — clarify in D10 or the coverage test pseudocode: option (a) nested types are implicitly covered by any parent fixture (reflection walks fields); option (b) nested types need standalone fixtures; option (c) `_covered.json` explicitly marks nested types as "nested_only" with the parent fixture path. Option (c) gives the clearest audit trail. This matters for ~8 nested types in the catalog.

### A-11. The WU-093 four-check strategy (D10) has a subtle self-test gap

- **What:** D10 lists "Forward (unmarshal→re-marshal→equal)", "Reverse (synthesize→marshal→equal)", "Strict schema (DisallowUnknownFields)", "Coverage." Forward and Reverse together pin wire shape. Strict-schema pins that unknown fields are rejected. But none of the four catches: a field being declared in Go but accidentally omitted from the fixture (the Go type adds a field, but the fixture doesn't include it; Forward still passes because JSON unmarshal is permissive-on-missing, and Reverse passes because the synthesized instance uses Go defaults). The "missing required field" case needs a negative fixture.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:109-114` — four-check list.
  - Comparison: the WU-093 spec in track-0-shared.md line 184 explicitly says "Schema conformance: fixture fails validation if it has unknown fields **or is missing required fields**."
- **Recommended disposition:** **fix in design now** — add a fifth check or strengthen check 3: "strict-schema validates on a fixture that was intentionally pruned of a required field and asserts the decoder rejects it." Equivalent wording: "A negative fixture per required-field class must exist and assert rejection." This aligns the design with the track-0 WU-093 spec.

## Nit findings

### N-01. Design D6's inline code comment `// MT-CONN-002 through MT-CONN-012` implies a range, but all 12 are discrete

FEAT-0008 §"Diagnostic Taxonomy" assigns semantic categories to each code. The `// ... MT-CONN-002 through MT-CONN-012` elision reads as if they were machine-generated rather than hand-picked. Once A-03 is resolved this disappears.

### N-02. Fixture count estimate arithmetic

`~70 files total (20 WU-039 requests + 16 events + ~20 responses + 12 diagnostics + variants)` sums to 68 minimum plus variants. Track-0 WU-093 spec line 188 says "one happy-path variant per type plus one error/diagnostic variant where applicable." The "+ variants" hand-wave could hide real coverage gaps. A target of 80-90 fixtures is more honest given: session list (empty + 1-item + multi-item = 3 fixtures for one type), multi-model variants of 5 events (+5), minimal + full request variants (+20), and the per-diagnostic-code ServerError context fixtures.

### N-03. "method-name constants colocated" — which file holds `EventCompactPlanNotice`?

Design line 159: `EventCompactPlanNotice = "compact.plan"` is listed under events.go's method-name constants block. But line 190 says `CompactPlan` (the type) lives in compact.go and events.go references it. So the method constant is in events.go while the type is in compact.go. That split works in a single package but is worth calling out explicitly — it is the one case where the file-level colocation pattern breaks. Minor editorial.

### N-04. `SessionListResponse` does not include pagination / total-count fields

FEAT-0008 §"Session List and Details" shows only `{sessions: [...]}`. But §"Configuration" mentions `sessions.max_per_user: 100` — with 100 sessions per user and no pagination, clients will paginate client-side or pull all. This is a FEAT-0008 gap more than a design gap; do not block on it. Worth mentioning so downstream WUs (FEAT-0009 harness UI for session browse) don't assume the field is missing by oversight.

## Coverage table

| FEAT-0008 section | Design doc coverage | Notes |
|-------------------|---------------------|-------|
| Protocol Messages (line 162-211) | complete | All 14 streaming + 2 non-streaming + 17 harness-side response pairs covered. `turn.submit` response gap is B-01. |
| Protocol Payload Schemas (line 213-360) | partial | `turn.submit`, `session.resume`, `session.clear`, `session.fork`, `context.list`, `status.update`, `content.transform` all covered. `turn.submit` Response gap (B-01). |
| Canonical Field Names (line 361-373) | complete | D3 inherits WU-039's snake_case contract; `TestCanonicalFieldNames` extended per D3. |
| Tool Catalog Schema (line 66-98) | complete | `ToolDefinition` moved to tools.go (D7); wire shape unchanged. Relocation wording needs B-03 fix. |
| Diagnostic Taxonomy (line 497-531) | partial | All 12 codes referenced (D6); all 12 constants not enumerated (A-03); `Diagnostic` struct matches (line 248). `ServerError.code` vs. `Diagnostic.code` ambiguity is A-01. |
| Session Management (line 888-949) | complete | `SessionSummary`, `SessionDetail`, `TurnSummary`, `ServerSessionEvent` all covered with fields. Naming deviation (A-08, A-09). |
| In-Flight Turn Recovery (line 447-495) | partial | `SessionSyncResponse`, `ActiveTurnState`, `PendingToolCall`, `MultiModelState`, `ReviewerState` all covered. Enum extensions are A-07. Token replay deferred to WU-064 correctly (design line 323). |
| Compaction (line 972-1017) | complete | `CompactCategory`, `CompactFileBreakdown`, `CompactPlan` (shared event+response per line 190), `CompactApplyResponse` covered. Same-package relocation wording is B-03. |
| Model Transparency + Multi-Model (line 808-886) | complete | `ModelSelected` polymorphism handled pragmatically (D5); `ModelInfo`, `ModelListResponse`, `RoutingPolicy`, `ModelSwitchResponse` covered. `RoutingPolicy.Resolve` helper scope issue is A-02. |
| Streaming events (line 184-203) | complete | All 14 streaming events (token.delta, branch.*, tool.call, status.update, knowledge.hit, cost.update, compact.*, turn.complete, model.selected, error) + 2 non-streaming (capabilities.request, connection.pong) present. |
| Health / Readiness (line 422-445) | complete | `HealthResponse`, `ReadyResponse`, `DependencyStatus`, `ProviderStatus`, `ActiveSessionInfo`, `ServerCapabilities` (D8) covered. |
| FEAT-0008 ambiguity register | blocked | B-02: the "12 ambiguities" referenced do not exist as a canonical list. |

## What I did NOT review

- **FEAT-0008 spec correctness itself.** I treated FEAT-0008 as the source of truth and did not audit its internal consistency.
- **Go idiom review in depth.** Same-model blind spot; a Go-specialist reviewer (or Codex / Kimi / GPT-5) should lint idioms (e.g., `json.RawMessage` ergonomics for `ModelSelected.Model`, or whether `RoutingPolicy` as `map[string]json.RawMessage` is the right representation).
- **Performance of reflection-based coverage test** on ~60 exported types. Likely fine but not measured.
- **Fixture file-naming conventions** (e.g., `token_delta.json` vs. `TokenDelta.json`). Design implies snake_case filenames and this is consistent.
- **Whether `CompactPlan` sharing between event and response can round-trip cleanly** through both `Notification.Params` and `Response.Result` envelopes. The wire bytes are identical JSON objects, so this should work, but I did not trace the exact test path.
- **Storage-side compatibility.** WU-045 session storage schema (`session_events` table) and its relation to the wire `ServerSessionEvent` shape are out of scope for this bundle (separate bundle, per the TPM task list).
- **Interaction with WU-046 dispatch.** WU-046 is referenced throughout the design's deferral register. I did not verify WU-046 is actually scheduled to accept these deferrals or has entry criteria updated.
- **Cross-bundle conflicts with the provider-formatting bundle (WU-042/043/044)** and the storage bundle (WU-045/091/096). Those are reviewed separately by the same sub-agent chain.
