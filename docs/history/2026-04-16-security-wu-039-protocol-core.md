# 2026-04-16 — Security Review: WU-039 Protocol Core

**Work Unit:** WU-039 (Track 0)
**Agent role:** Security Reviewer
**Scope of review:** `internal/protocol/protocol.go`, `internal/protocol/messages.go` at commits `28213eb` (red phase), `1aa3830` (green phase), and the fix applied here.

## Review Areas

1. Secrets / credential exposure in declared fields
2. JSON tag correctness vs. canonical snake_case
3. NDJSON framing — size bounds, DoS resistance, resynchronization
4. Deserialization safety
5. Information leakage via error envelopes
6. Unknown-field handling

## Findings

### SR-039-01 (FIXED) — Unbounded drain on oversize frame

**Severity:** Medium
**Component:** `FrameReader.ReadFrame` / previously `drainUntilNewline`
**Commits affected:** `1aa3830`
**Status:** Fixed in this WU (same-session)

The green-phase implementation of `ReadFrame` called a helper `drainUntilNewline` after detecting an oversize frame, reading bytes until the next newline before returning `ErrFrameTooLarge`. Because the attacker controls when (or whether) a newline appears in the stream, this helper could loop indefinitely reading arbitrarily many bytes, defeating the purpose of `MaxFrameSize`.

**Attack:** Peer sends a 10 MiB + 1 byte payload followed by no newline; `ReadFrame` enters drain, blocks on subsequent reads, holding a goroutine and buffered socket state until the connection is forcibly closed from elsewhere.

**Fix:** Removed the drain. `ReadFrame` now returns `ErrFrameTooLarge` the moment the oversize threshold is reached, consuming at most `MaxFrameSize + 1` bytes total. Documented that the caller MUST close the connection on this error (the underlying reader is left mid-frame and cannot be resynchronized without risking interpretation of attacker-controlled bytes as a fresh frame).

**Reviewer note:** This fix shifts responsibility to the transport layer (WU-046, WU-047). That WU's DoD must include: "on `ErrFrameTooLarge`, close the connection immediately; do not attempt to resynchronize." Will flag in WU-046 entry criteria.

### SR-039-02 — User-controlled content fields may carry secrets

**Severity:** Informational (not fixable at this layer)
**Fields:**
- `CapabilitiesRegister.Project.ConfigContent` — full `.modeltap.yaml` or equivalent content; may contain API keys if the user has embedded them
- `Attachment.Raw` — base64-encoded file bytes; could be `.env`, private keys, any user-selected file
- `ContentTransform.RawContent` — user-pasted content
- `Paste.Raw` — user paste

**Assessment:** These fields are intentionally user-supplied content; the protocol's job is to carry them. Redaction cannot happen at the type layer — an `Attachment.Raw` of a PDF looks identical to `Attachment.Raw` of a secrets file.

**Action:** No change needed here. Downstream layers (capture store, log exporter, session-details API, MCP tool responses) MUST apply the ADR-0005 secret-header redaction pattern plus content-type-aware redaction for known sensitive file types. These are **WU-094 scope** (security review suite, OWASP pass).

### SR-039-03 — Unknown-field handling uses default (permissive) decoder

**Severity:** Informational
**Assessment:** `json.Unmarshal` without `DisallowUnknownFields` silently accepts extra fields. For a versioned protocol this is usually desirable (forward compatibility with newer harness/server pairs).

**Action:** No change at this layer. Cross-track conformance fixtures (**WU-093**) will assert that fixtures contain only the documented fields, catching drift in either direction. If the team later prefers strict decoding, a helper `DecodeStrict[T]` can be added without changing the types themselves.

### SR-039-04 — `ErrorObject.Data` is raw JSON

**Severity:** Informational
**Field:** `ErrorObject.Data`
**Assessment:** Typed as `json.RawMessage` so upper layers can put arbitrary structured diagnostic data in it. That freedom lets them accidentally include internal state (stack frames, file paths, SQL statements).

**Action:** No change at this layer. **WU-063** (diagnostic taxonomy) and **WU-094** must specify the allowed `Data` shapes and enforce that no internal state leaks. The diagnostic codes (MT-CONN-*) defined in WU-041 should constrain what `Data` may contain.

### SR-039-05 — `ToolDefinition.InputSchema` is `json.RawMessage`

**Severity:** Informational
**Assessment:** `InputSchema` passes through as raw JSON so that JSON Schema payloads survive unmodified. This means a harness-registered tool definition could include a malicious or oversized schema. At this layer, protocol types impose no schema-content constraints.

**Action:** **WU-049** (capability registration) must:
- Apply a size cap on per-tool schema
- Validate that `InputSchema` parses as JSON Schema draft 2020-12 (or whichever version is declared)
- Reject tools whose schema allows arbitrary code / eval

Covered in WU-094 security suite.

## Checked and Clean

- JSON tags — all 19 request types + 5 nested types use snake_case; enforced by `TestCanonicalFieldNames` and a CamelCase negative-leak test.
- `ProtocolVersion = "1"` is a non-empty constant; `TestProtocolVersion` asserts.
- `MaxFrameSize = 10 MiB` is within the 1 MiB–100 MiB sanity band; `TestMaxFrameSize_IsPositive` asserts.
- `FrameWriter.WriteFrame` rejects embedded newlines (would otherwise allow frame splitting/injection); `TestFraming_WriteRejectsEmbeddedNewline` asserts.
- `Mode.Valid` is strict: only `plan`/`build`/`auto` pass; empty string rejected.
- `ToolResultRequest` as an alias preserves wire-shape equivalence; type-level check via `var _ ToolResultRequest = ToolResult{}`.
- No `fmt.Sprintf` or string concatenation into JSON anywhere; all encoding via `encoding/json`.
- No `unsafe`, no `cgo`, no `reflect`-based marshaling beyond standard library JSON.

## Verdict

**Pass with one fixed finding.** WU-039 is secure to the extent the type layer can be. Downstream WUs (WU-046, WU-049, WU-063, WU-094) carry the remaining responsibilities documented above. Flagged items will be tracked in their entry criteria.

## Follow-ups Queued for Later WUs

- **WU-046 (JSON-RPC transport):** On `ErrFrameTooLarge`, close the connection immediately; document in that WU's DoD. (From SR-039-01.)
- **WU-049 (capability registration):** Size-cap and validate `ToolDefinition.InputSchema`. (From SR-039-05.)
- **WU-063 (diagnostic taxonomy):** Enumerate allowed `ErrorObject.Data` shapes; enforce no internal-state leakage. (From SR-039-04.)
- **WU-094 (security review suite):** Content-type-aware attachment redaction; API-key scanning in config-content paths; schema-injection tests for tool catalog. (From SR-039-02, SR-039-04, SR-039-05.)
