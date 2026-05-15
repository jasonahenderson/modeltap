# WU-039 Design Review — Claude Subagent

**Reviewer:** Claude subagent (fresh context, same model as designer — not cross-model)
**Date:** 2026-04-16
**Subject:** `.sdlc/history/2026-04-16-design-wu-039-protocol-core.md`
**Implementation:** `internal/protocol/` at commit `50febd0`
**Review tier:** C (retroactive)

## Caveat on reviewer provenance

This review was produced by a Claude subagent with a fresh context window, reading only the artifacts listed in the review prompt. It is **same-model, different-session**: I share the Designer's training distribution, tokenizer, and reasoning heuristics. This review is most likely to catch: (a) undocumented assumptions the Designer left implicit, (b) drift between the WU spec / feature spec and the design doc, (c) missing cross-WU wiring, and (d) scope gaps. It is least likely to catch: (a) reasoning failures that are characteristic of Claude specifically (e.g., over-trusting a plausible-looking symmetry between types), (b) blind spots baked into the Anthropic training data, and (c) idiom mismatches that a Go-specialist human or a differently-trained model (Codex, Kimi, GPT-5, Gemini) would flag on sight. This review **does not** substitute for a cross-model Tier-C review; it substitutes for the Tier-A self-checklist that should have run at design time.

## Summary

The design is largely solid and faithful to FEAT-0008 for the 19 types it enumerates, with correct JSON-RPC envelope handling, snake_case canonicalization, a defensible `MaxFrameSize` cap, and a clean scope boundary to WU-040/WU-041/WU-093. One **Blocking** gap: the WU-039 spec in `track-0-shared.md` lists a `ConnectionReady` type that the design and implementation both omit, and FEAT-0008 explicitly classifies `connection.ready` as a harness→server message. A handful of **Attention** items flag ambiguous presence semantics (`Sequence int` zero-value; `TurnSubmit.Mode` required-vs-optional in struct form), a protocol-freeze question around `ToolResult.Output`/`ToolResultRequest` alias coupling, and silent round-tripping of unknown `Mode` values that will confuse WU-080 if not policed at the transport edge. Nothing in the implementation strays from the design beyond documented fixes. Overall: ship, but resolve the `ConnectionReady` gap before WU-040 starts, and queue the Attention items for triage.

## Blocking findings

### B-01. `ConnectionReady` message is missing from design and implementation

- **What:** `.sdlc/releases/v0.2.0/track-0-shared.md` line 15 enumerates the types WU-039 delivers; the list explicitly includes `ConnectionReady` (20 types). The design doc (`2026-04-16-design-wu-039-protocol-core.md`) and `internal/protocol/messages.go` both ship 19 types and do not declare `ConnectionReady` / `MethodConnectionReady` / `connection.ready`.
- **Evidence:**
  - `.sdlc/releases/v0.2.0/track-0-shared.md:15` — "… `ConnectionPing`, `ConnectionHealth`, `ConnectionReady`"
  - `.sdlc/features/0008-bff-server.md:211` — "Note: `connection.ping`, `connection.health`, `connection.ready`, and `session.sync` are harness→server messages listed in the harness table above."
  - `.sdlc/features/0008-bff-server.md:411` and `:445` — `connection.ready` is invoked during auto-start probing and is described as a simplified boolean readiness check.
  - `internal/protocol/messages.go:16-36` — only 19 `Method*` constants; no `MethodConnectionReady`.
  - Design doc §"Type Catalog — 19 Harness→Server Request Types" — table ends at row 19 (`ConnectionHealth`).
- **Why blocking:** WU-039 is the protocol freeze. WU-074 (Connection Manager, Track B) auto-start flow polls `connection.ready` with a 10s timeout (FEAT-0008 §"Local service auto-start (solo profile)" step 4), and WU-048 (Connection Lifecycle, Track A) declares that it handles `connection.health, connection.ready` (track-a-bff-server.md:29). Both downstream WUs therefore assume WU-039 provides the type. Adding it later is a protocol-catalog change that contradicts the freeze contract WU-093 is meant to lock down.
- **Suggested fix:** Add `ConnectionReady struct{}` (or a minimal struct — see note below) and `MethodConnectionReady = "connection.ready"` to `messages.go`, a round-trip test to `protocol_test.go`, and a row to the design-doc type catalog. Note: FEAT-0008 describes `connection.ready` as a simplified boolean check; the **response** shape (boolean) belongs in WU-041's `health.go` (`ReadyResponse`). The **request** type is params-less and parallel to `ConnectionPing` / `ConnectionHealth`. If the TPM reading this review prefers to defer the request type to WU-041 (with an explicit spec amendment), the track-0 spec must be updated too — today the spec and design disagree.

## Attention findings

### A-01. `TurnSubmit.Sequence int` loses the distinction between "sequence 0" and "omitted"

- **What:** `TurnSubmit.Sequence` is typed `int` with `json:"sequence"` (no `omitempty`, no pointer). FEAT-0008 §"Protocol Payload Schemas" marks `sequence` required, so this is *formally* consistent: a harness that omits it serializes `"sequence":0`, which decodes identically to "sequence=0 from harness". But downstream dispatch (WU-046, WU-051) will want to reject malformed `turn.submit` that omits `sequence` entirely — FEAT-0008 line 371 declares `sequence` canonical, and idempotency rules rely on `(turn_id, sequence)` correlation.
- **Evidence:**
  - `internal/protocol/messages.go:106` — `Sequence int \`json:"sequence"\``
  - FEAT-0008 §"Canonical Field Names" row `sequence` — "Integer turn order within a session"
  - FEAT-0008 §"Idempotency rules" — duplicate `turn.submit` detected by `(turn_id, sequence)` coupling
- **Recommended disposition:** **defer to WU-046** with a documented expectation. WU-046 is already on the hook for transport-layer validation per the design doc's D7 non-validation stance. Track-A's WU-046 entry criteria (track-a-bff-server.md:11-17) should gain "reject `turn.submit` whose raw JSON omits `sequence`"; a strict decoder is the simplest implementation. Alternatively, change the type to `*int` here; I do not recommend that — it breaks the "plain scalar for required int" rule in D5 and creates two patterns for required ints across the 19 types.

### A-02. `TurnSubmit.Mode` uses typed `Mode` but carries no `omitempty` protection and round-trips unknown values

- **What:** `Mode` is a typed string with `Valid()` that rejects unknown values, but `TurnSubmit.Mode` serializes any `Mode("bogus")` cleanly and the round-trip unmarshal reconstructs the same bogus value. The design D7 section explicitly accepts this ("a bad `Mode` or missing required field round-trips"). Combined with the permissive-unknown-fields decoder (SR-039-03), this means a pre-release harness sending `mode:"plan-with-review"` (an unshipped variant) will be accepted silently by this package.
- **Evidence:**
  - `internal/protocol/messages.go:107` — `Mode Mode \`json:"mode"\``
  - `internal/protocol/protocol_test.go:65-83` — `TestMode_Valid` confirms the rejection semantics exist, but the round-trip test at `:85-102` confirms bogus values survive JSON.
  - Design D4 / D7 — acknowledged as out-of-scope for WU-039.
- **Recommended disposition:** **defer to WU-046** with documented expectation. Add to WU-046 entry criteria: "on decode, reject `turn.submit` frames whose `mode` fails `protocol.Mode.Valid()`." Also worth considering for WU-093 conformance: a negative fixture `turn_submit_invalid_mode.json` that asserts the strict-decode path rejects it. This Attention item is a real gap in test coverage today — nothing in the protocol contract catches mode drift until runtime.

### A-03. `ToolResultRequest = ToolResult` type alias couples two wire concerns

- **What:** The design documents (D6 naming note, §"19 request types" row 3) and messages.go declare `type ToolResultRequest = ToolResult`. FEAT-0008 §"Tool call and result payloads" shows the same shape (tool_call_id, status, output, output_type, error, reason) for both nested and standalone forms **today**, but FEAT-0008's protocol freeze phrasing (line 1254) says only the Protocol Payload Schemas section is frozen. The design doc's Non-risk #4 acknowledges "if WU-041 or later adds fields to the nested `ToolResult` that don't make sense for the standalone request, split the alias."
- **Evidence:**
  - `internal/protocol/messages.go:58-72` — alias declared here.
  - Design doc §"19 request types" table — row 3 calls out the alias.
  - FEAT-0008 §"Tool call and result payloads" — only the standalone form is shown with `status: rejected` and `status: error` variants; the nested form in `turn.submit` shows the same union.
- **Recommended disposition:** **fix now (cheap)** — add a package-level godoc comment on `ToolResultRequest` explicitly stating "This alias encodes a protocol-freeze contract: the standalone `tool.result` request and the nested `turn.submit.tool_results[]` elements MUST remain wire-identical. Any WU-041+ extension to `ToolResult` must apply to both and be reflected here; splitting the alias is a breaking protocol change." Without this note, a later WU author could reasonably add a field to the standalone form only, assuming the alias is a minor implementation detail. This is a five-line doc-comment change, not a code change.

### A-04. `json.RawMessage` for `ID` does not actually preserve JSON-RPC `null`

- **What:** `Request.ID` and `Response.ID` are typed `json.RawMessage` with `omitempty`. `Request.ID` has `omitempty`; `Response.ID` does not. JSON-RPC 2.0 permits `id` to be `null`, a string, or a number. The Designer's rationale (D2) correctly cites the string|number|null requirement. But `omitempty` on a `json.RawMessage` treats a nil slice as absent — and a `json.RawMessage` unmarshaled from wire `"id":null` will be the 4-byte slice `[]byte("null")`, not nil, so round-tripping `null` works. However, **an uninitialized `Request` with zero-value `ID` (nil slice) is emitted with no `id` field at all** — which is JSON-RPC 2.0's "notification" semantics, not "request with id: null". This is a semantic footgun for any caller who forgets to set `ID`.
- **Evidence:**
  - `internal/protocol/protocol.go:74` — `ID json.RawMessage \`json:"id,omitempty"\``
  - JSON-RPC 2.0 spec §4.1 — notifications are specifically "a Request object without an ‘id’ member"
  - `internal/protocol/protocol_test.go:314-349` — `TestRequest_RoundTrip` uses `json.RawMessage(\`"req-1"\`)`, never tests the nil-ID case.
- **Recommended disposition:** **defer to WU-040** (which introduces server→harness events that are arguably notifications) with a pre-decision: the design doc should state explicitly whether WU-039's `Request` is usable as a JSON-RPC notification (omit `id`) or whether a dedicated `Notification` type will be introduced in WU-040. If the latter, add a doc comment on `Request.ID` warning that callers **must** set `ID` for non-notification requests and linking to the forthcoming `Notification` type. This is not a wire-incompatible decision yet — it's a code-clarity decision that will affect WU-040 and WU-046.

### A-05. `MaxFrameSize = 10 MiB` is stated in the design but FEAT-0008 does not ratify it

- **What:** The design D3 justifies 10 MiB as "covers typical docs, images, and spreadsheets without allowing trivial memory exhaustion." FEAT-0008 §"Protocol Specification" §"Framing" says only "Each message is a complete JSON object terminated by `\n`" — no size cap is stated, normative or otherwise. A harness that attempts to attach a 15 MiB PDF (well within `.docx` sizes for spec documents; real estate, legal, and financial domains routinely exceed 10 MiB per attachment) will be rejected by the reader with no advance warning in the capabilities handshake.
- **Evidence:**
  - `internal/protocol/protocol.go:51` — `const MaxFrameSize = 10 * 1024 * 1024`
  - `.sdlc/features/0008-bff-server.md` §"Framing" — no cap stated (grepped).
  - `.sdlc/releases/v0.2.0/track-0-shared.md` WU-039 spec — no cap stated.
  - Design D3 rationale — states the cap is caller-facing policy.
- **Recommended disposition:** **defer to a FEAT-0008 amendment or an ADR**, not a fix in WU-039. The cap is a reasonable default, but (a) FEAT-0008 should ratify it (or document it as implementation-configurable), (b) WU-049 (capability registration) should expose the current cap in the capability handshake so the harness can refuse oversize attachments before serializing, and (c) WU-041 (errors.go) should define a diagnostic code for "attachment too large" so the harness renders an actionable message rather than a generic connection drop. File this as a patch candidate or add to WU-049's entry criteria. Note: SR-039-01 already shifted "close the connection on ErrFrameTooLarge" to WU-046 — that handling is correct for malicious input, but a legitimate large-attachment case deserves a cleaner error path.

### A-06. `Attachment.Raw` and `Paste.Raw` lack size bounds and have no `omitempty`

- **What:** `Attachment.Raw`, `Attachment.Content`, `Paste.Raw`, `Paste.Content` are all plain `string` fields with no `omitempty`. FEAT-0008 §"Protocol Payload Schemas" shows the JSON examples but does not state whether `raw` is required. For a harness that only transmits extracted text (e.g., `transform: "pdf_text_extract"` with the raw bytes already captured server-side), emitting `"raw":""` is wasteful but not wrong. More concerning: a zero-field `Attachment{Path: "x.pdf"}` serializes to a frame with `"raw":"", "content":"", "content_type":"", "transform":""`, which is indistinguishable at this layer from "harness intended to send a real attachment but forgot to populate its fields."
- **Evidence:**
  - `internal/protocol/messages.go:43-56` — all nested-struct string fields are plain strings, no `omitempty`.
  - FEAT-0008 §"Protocol Payload Schemas" — JSON examples show all fields populated, but no explicit required/optional table for nested types.
- **Recommended disposition:** **fix in same WU** (cheap) — add `omitempty` to `Attachment.Raw`, `Attachment.Content`, `Paste.Raw`, `Paste.Content` if FEAT-0008 classifies them as optional; otherwise leave as-is and **add a godoc comment clarifying the required-set explicitly**. This matters for WU-093 fixtures, which otherwise cannot faithfully encode "attachment metadata only, no content."

### A-07. Documentation quality: package godoc is good, but a cross-track designer still needs FEAT-0008

- **What:** Scope boundaries, canonical field names, and types are well-documented at the file level. However, for a WU-073 (harness protocol client) designer or a WU-046 (BFF transport) designer approaching `internal/protocol/` for the first time, the package godoc does **not** include: (a) a link or reference to FEAT-0008's anchor sections, (b) the list of method names expected in WU-040/WU-041 (forward pointers), (c) the JSON-RPC 2.0 spec reference URL. A designer relying only on `go doc ./internal/protocol` will not know that WU-041 adds response types whose names mirror these request types.
- **Evidence:**
  - `internal/protocol/protocol.go:1-29` — package doc comment.
- **Recommended disposition:** **fix in same WU** (cheap) — add three bullet references in the package godoc: (a) `// Feature spec: .sdlc/features/0008-bff-server.md (Protocol Specification section)`; (b) `// JSON-RPC 2.0: https://www.jsonrpc.org/specification`; (c) a one-line forward pointer to the `*_test.go` conformance work in WU-093. This is a doc-only change, low risk.

## Nit findings

### N-01. Constant block ordering vs. table ordering

`messages.go:16-36` lists the 19 method constants in a sensible pragmatic order (turn → tool → content → session → model → context → capabilities → connection). The design-doc catalog table orders them identically. The test table in `protocol_test.go:22-42` matches. Consistent. If future WUs adopt alphabetical sorting, keep all three in sync. No action.

### N-02. `FrameReader` reads byte-at-a-time

`ReadFrame` uses `br.ReadByte()` in a loop. For a 10 MiB legitimate attachment, this is ~10 million function calls through `bufio.Reader`. `bufio.Reader.ReadSlice('\n')` or `ReadBytes('\n')` would be substantially faster and still bounded by `MaxFrameSize` if wrapped correctly. That said: the current implementation is *correct* and the byte-at-a-time choice makes the size-cap check obvious. For WU-039's DoD this is fine. If a later WU surfaces throughput as a pain point, profile first. No action.

### N-03. `MaxFrameSize` comment says "10 MB" but the constant is 10 MiB

Design doc D3 says "10 MB cap" (decimal); the code comment at `protocol.go:51` says "10 MiB" (binary). The test at `:487-492` uses 1 MiB / 100 MiB as the sanity band. The code is consistent; the design doc is off by one unit prefix. Trivial copy-edit in the design doc.

## What I did NOT review

- **WU-040 / WU-041 scope specifics.** I verified that WU-039 defers events, tool payloads, session payloads, and diagnostics to those WUs but did not audit the later specs in depth. That is their own Tier-C review's job.
- **Go idiom review in depth.** Package name, file split, import grouping, and error construction look correct to me, but a Go-specialist or differently-trained model may spot idioms I missed.
- **Cryptographic concerns** beyond what SR-039-01 and SR-039-02 already covered. TLS wiring is WU-047.
- **Performance of the framing path under concurrent load.** I noted N-02 but did not benchmark.
- **Fixture design for WU-093.** That WU has its own Tier-C gate.
- **FEAT-0008 correctness itself.** I treated FEAT-0008 as the source of truth. If FEAT-0008 is wrong about `connection.ready` being a harness→server request, the finding B-01 is a cross-doc inconsistency rather than a code bug — but then the inconsistency still blocks WU-039's freeze.
- **Whether `Sequence` should be `uint`.** The FEAT-0008 spec says "integer"; Go convention prefers `int` for JSON numeric fields because `encoding/json` unmarshals numbers as `float64`/`int` by default. Left as-is.

## Scorecard

Using the Tier-A self-checklist as a floor:

1. **Every input (ADR, feature spec, dependency WU) referenced and constraints captured:** **PARTIAL** — FEAT-0008 anchor sections cited; `track-0-shared.md` WU-039 spec cited; but the spec's explicit `ConnectionReady` enumeration (line 15) was missed, producing B-01. ADR-0005 is referenced obliquely via a comment in `ContentTransform`. ADR-0002 (SQLite) is out of this WU's scope and correctly omitted.

2. **Every field enumerated; required/optional match source:** **PARTIAL** — 19 of the 20 spec'd types are enumerated exhaustively with required/optional labels that match FEAT-0008. `ConnectionReady` is entirely absent (B-01). Nested-type required/optional labels are not explicit in the design doc (A-06). `TurnSubmit.Sequence` required-vs-absent ambiguity at the Go-type level (A-01) is documented but not mitigated here.

3. **Cross-WU type consistency:** **PARTIAL** — `ToolDefinition` (WU-049 consumer), `ProjectContext` (WU-049 consumer), `Mode` (WU-080 consumer) match FEAT-0008. Method-name strings match. But `ConnectionReady` (consumed by WU-048 and WU-074) is missing. `ToolResultRequest` alias (A-03) lacks a contract comment that would prevent WU-041+ drift.

4. **Conventions followed (gofmt, effective Go, snake_case):** **PASS** — every field carries an explicit JSON tag, snake_case everywhere, CamelCase Go identifiers. Negative-leak test (`TestCanonicalFieldNames` at `protocol_test.go:603`) enforces this. No `unsafe`, `cgo`, or reflection beyond stdlib JSON.

5. **At least one "what could go wrong" case with mitigation:** **PASS** — design §"Risks and Non-Risks" lists four, and SR-039-01 is a real find-and-fix. Coverage could be deeper (e.g., the oversize-legitimate-attachment case from A-05 is not in the risks list) but the self-checklist floor is met.

6. **Scope boundaries explicit:** **PASS** — §"Scope" in the design doc enumerates what is in and what defers to WU-040 / WU-041 / WU-093. The only in-vs-out ambiguity is `ConnectionReady`, which the WU-039 spec claims is in-scope — so the scope statement is *clear*, it just *disagrees with the spec*.

---

## Disposition (2026-04-16)

Appended by the Designer after triage.

- **B-01 `ConnectionReady` missing** — **FIXED** in-WU. Added `ConnectionReady struct{}`, `MethodConnectionReady = "connection.ready"`, round-trip test, and design-doc catalog entry. Catalog updated from 19 to 20 types. Package godoc updated.
- **A-01 `Sequence` required-vs-absent ambiguity** — **DEFERRED to WU-046.** Added to WU-046 DoD: reject `turn.submit` whose raw JSON omits `sequence`.
- **A-02 Unknown `Mode` round-trips silently** — **DEFERRED to WU-046.** Added to WU-046 DoD: reject `turn.submit` frames whose `mode` fails `protocol.Mode.Valid()`.
- **A-03 `ToolResultRequest` alias freeze contract** — **FIXED** in-WU. Added godoc on `ToolResultRequest` documenting wire-identity guarantee and what would constitute a breaking change.
- **A-04 `Request.ID` JSON-RPC notification semantics** — **DEFERRED to WU-040.** Added decision-required note to WU-040 spec in track-0-shared.md: WU-040 must choose between shared `Request` with optional ID vs. dedicated `Notification` type.
- **A-05 `MaxFrameSize = 10 MiB` not ratified** — **DEFERRED to WU-049 + FEAT-0008 amendment.** Added to WU-049 DoD: expose max frame size and max attachment size in capability handshake; coordinate a FEAT-0008 amendment to ratify the cap.
- **A-06 Nested-type required-set not documented** — **FIXED** in-WU. Added godoc on `Attachment`, `Paste`, and `ToolResult` documenting which fields are required.
- **A-07 Package godoc lacks forward references** — **FIXED** in-WU. Added FEAT-0008 spec path, JSON-RPC 2.0 URL, and WU-093 forward pointer to package godoc.
- **N-01 Constant ordering** — noted, no action.
- **N-02 Byte-at-a-time read** — noted, defer to perf work (WU-095 will benchmark and may revisit).
- **N-03 "10 MB" vs "10 MiB" copy edit** — **FIXED** in design doc.

All green-phase tests still pass after the fixes. `go build ./...` green.
