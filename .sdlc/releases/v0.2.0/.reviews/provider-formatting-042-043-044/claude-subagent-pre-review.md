# Provider Formatting Bundle (WU-042 + 043 + 044) Pre-Review Lint — Claude Subagent

**Reviewer:** Claude subagent (fresh context, same-model pre-review — not Tier C peer review)
**Date:** 2026-04-16
**Subject:** `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md`
**Bundle:** WU-042 (Provider interface extension + ADR-0006 amendment) + WU-043 (Anthropic outbound) + WU-044 (OpenAI outbound)

## Reviewer caveat

Same-model lint: shares the Designer's training distribution, tokenizer, and reasoning heuristics. Most likely to catch mechanical drift, missed FEAT-0008 fields, scope gaps, and cross-WU inconsistency. Least likely to catch Claude-characteristic blind spots — particularly around wire-format details that the same training data would have encoded identically on both sides (e.g., if Claude's training data uniformly misremembers an OpenAI field name, this reviewer will not catch it). Does not substitute for a cross-model Tier-C peer review.

## Summary

The design is structurally sound — it correctly identifies the two-function boundary (`FormatMessages` and `FormatToolDefinitions`), takes the correct position that the canonical `Message` is an internal server concept (not wire protocol, so `internal/provider/`), and makes the pragmatic `FormatMessagesOpts` struct deviation explicit. The truncation algorithm's pair-atomicity rule is the right idea. However, there are **five blocking issues**: (B-01) `provider.Attachment` is missing the `Path` field that FEAT-0008 and `protocol.Attachment` require, AND the `Raw` field type (`[]byte`) disagrees with the shipped `protocol.Attachment.Raw string`, which makes the declared `convert.go` helpers structurally impossible to write without lossy conversion; (B-02) two conflicting `ErrNotImplemented` definitions inside the same design document; (B-03) the pair-atomicity rule (D4 step 4) does not handle the realistic multi-assistant-turn interleaving case where ToolResults from an older turn can be dropped while the matching assistant ToolCalls are preserved by the "walked so far" rule; (B-04) the D5 Anthropic mapping elides `tool_use.input` type (it must be a JSON object, so emitting `json.RawMessage` straight through is correct but the table doesn't say so) AND the OpenAI tool_result content-type rules that FEAT-0008 requires (output_type=json, output_type=image) are not addressed; (B-05) `FormatToolDefinitions`'s "returns just the tools field value" contract creates a composition problem because the return type is `[]byte` but `FormatMessages` must then splice that into a JSON body — the design does not spell out whether the splice re-marshals or concatenates. Several attention items flag the ADR amendment front-matter divergence from the existing ADR schema (the repo uses `status: proposed|accepted|...`; no `amends:` or `supersedes:` keys are defined), missing `tool_choice` / `stop_sequences` / `top_p` omissions that FEAT-0008 does not require but real dispatch will need, vision-capability gating that FEAT-0008 line 100 explicitly calls out, and the token-estimation policy leaving tool-result bodies uncounted.

## Blocking findings

### B-01. `provider.Attachment` drops the `Path` field and mistypes `Raw`, breaking both the FEAT-0008 contract and the declared `convert.go` round-trip

- **What:** The design's `provider.Attachment` (D1, lines 64-69) has four fields: `ContentType`, `Content`, `Raw` (`[]byte`), `Transform`. The shipped `protocol.Attachment` (`internal/protocol/messages.go:51-57`) has five required fields: `Path`, `Raw` (`string`), `Content`, `ContentType`, `Transform`. FEAT-0008 §"Protocol Payload Schemas" `turn.submit` (line 228-236) explicitly lists `path` as a required attachment field. The design's D1 note says "Conversion helpers between the two live in `internal/provider/convert.go`" — but dropping `path` means conversion is lossy (the file path information that FEAT-0008 §"In-Flight Turn Recovery" line 939 and `files_touched` tracking need) cannot round-trip, and mistyping `Raw` as `[]byte` vs. `string` means every canonical→provider hop does a base64 decode/re-encode when the data is already correctly base64-encoded on the wire.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:64-69` — `provider.Attachment` fields enumerated: `ContentType`, `Content`, `Raw []byte`, `Transform`. No `Path` field. Comment on `Raw`: "NOT base64 — Go []byte; JSON-encodes as base64 automatically."
  - `internal/protocol/messages.go:51-57` — shipped `protocol.Attachment`: `Path string`, `Raw string`, `Content string`, `ContentType string`, `Transform string` (all required — the messages.go doc comment at lines 46-50 explicitly says "All five fields are required... Omitting any field is a spec violation and WU-046 transport validation should reject it.")
  - `.sdlc/features/0008-bff-server.md:228-236` — `turn.submit` payload schema lists `path` first in the attachment object; it is not optional.
  - `.sdlc/features/0008-bff-server.md:1306-1312` — example `protocol.Attachment` struct in FEAT-0008 §"Interface Definition" uses `Raw []byte`, not `Raw string`. This is a pre-existing FEAT-0008 ↔ `internal/protocol/messages.go` drift that WU-039 already resolved in favor of `string`, and the design here re-introduces the inconsistency with a THIRD shape.
- **Why blocking:** (a) The canonical format the BFF assembles from session storage must carry the file path, otherwise session details, files-touched aggregation (FEAT-0008 line 909), and file-staleness detection (context.list line 318 `attached_turn`, `stale`) all break. (b) `convert.go` between the two types is structurally impossible without lossy conversion — either you drop `Path` going protocol→provider (and then cannot round-trip back), or you stash it in `Metadata` (which the design does not specify). (c) The `Raw` type disagreement turns every format path into `base64_decode(protocol.Raw) → []byte → base64_encode(for Anthropic image block)` instead of passing the string through. (d) A reviewer / implementer will trip on this on day one.
- **Suggested fix:** Add `Path string` to `provider.Attachment`. Either (a) align `Raw` type with `protocol.Attachment.Raw string` (recommended — matches the wire shape and avoids double base64) and document that Anthropic image blocks get the base64 string directly without re-encoding, or (b) define a concrete contract for `convert.go` that shows the byte-level round-trip and demonstrates it is not lossy. While editing D1, decide whether `Metadata map[string]any` needs specific documented keys (turn_id, sequence, branch_id, timestamp) — FEAT-0008 §"Session and Turn Storage Model" (line 1029) mentions "conversation history, active model, routing overrides, pinned items, compaction state" as session data, but the per-turn metadata that `Message.Metadata` can carry is under-specified.

### B-02. Two conflicting `ErrNotImplemented` definitions

- **What:** D3 declares `ErrNotImplemented = errors.New("provider: not implemented")`. D8 declares `ErrNotImplemented = errors.New("provider: method not implemented")`. These cannot both be correct; the file layout (line 250) says the sentinel lives in `internal/provider/provider.go` once.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:128` — `ErrNotImplemented = errors.New("provider: not implemented")` — new sentinel in `internal/provider/provider.go`.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:233` — `var ErrNotImplemented = errors.New("provider: method not implemented")`.
- **Why blocking:** Backend Implementer will pick one at random. Error-message-equality tests (the kind track-0 and track-A use) will pass on the wrong side or the right side but not deterministically; this is precisely the kind of mechanical drift that the WU-039 reviews caught and flagged as a systemic class of defect.
- **Suggested fix:** Pick one string — recommend `"provider: method not implemented"` (it is more specific) — and state it in exactly one place in the design (either D3 or D8, with the other referencing by name).

### B-03. Truncation pair-atomicity rule (D4 step 4) does not handle interleaved tool calls across multiple assistant turns

- **What:** D4 step 4 says: "if the first message to be dropped is a `tool` role or a user continuation carrying `ToolResults`, also drop the preceding `assistant` message that contains the matching `ToolCalls`. Walk backward until pair boundary is consistent." This handles a simple case (drop tool-result, then drop its assistant-tool-call parent). It does NOT handle:
  1. **Interleaved tool_calls across turns:** Assistant turn T1 emits tool_call `tc_1`. User turn T2 submits tool_result for `tc_1`. Assistant turn T3 emits tool_call `tc_2`. User turn T4 submits tool_result for `tc_2`. If truncation starts walking newest-to-oldest and stops mid-way such that T4 is kept but T1/T2/T3 are dropped, T4's tool_result references `tc_2` — which was emitted by T3 (dropped), orphaning the result. Both providers reject messages arrays where a tool_result has no prior tool_use. The design's "walk backward until pair boundary is consistent" is ambiguous — does it walk back to drop T3, or walk back further to drop T1/T2 as well?
  2. **Multi-call assistant turns:** A single assistant turn can emit multiple `ToolCalls` (the D1 type allows `[]ToolCall`). If the matching `ToolResults` arrive in a single user turn, they travel together — pair atomicity is easy. But if a later assistant turn emits new tool_calls BEFORE the previous turn's results came back (not typical but representable in the canonical form), the "pair" becomes a DAG, not an edge. D4 does not say.
  3. **Interaction with system prompt budget:** Step 1 reserves `systemBudget` and step 2 walks under `WindowSize - systemBudget`. But step 5 returns `ErrEmptyMessages` if step 4 drops too much. There is no state for "step 4 drops a tool_result whose preceding assistant message is BEFORE the first kept user message" — i.e., pair-drop can force dropping messages that were already past the budget, potentially leaving the kept prefix smaller than intended and un-analyzable.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:134-139` — D4 truncation algorithm.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:269` — `TestTruncate_PairAtomicity` covers "dropping a tool message also drops its paired tool-call assistant" — single-pair only.
  - `.sdlc/features/0008-bff-server.md:60` — "Multiple `tool.call` events may be emitted in sequence within a single turn, each requiring a `tool.result` before the stream continues." — Multi-call-per-turn is explicitly in-scope.
  - Anthropic Messages API contract: `tool_use.id` in an assistant message must precede any `tool_result.tool_use_id` referencing it; violating this returns 400 on dispatch.
  - OpenAI Chat Completions contract: `tool_call_id` on a `role: "tool"` message must match an `id` in a preceding `role: "assistant"` message's `tool_calls[]`; violating this returns 400.
- **Why blocking:** The BFF will silently construct invalid provider requests in realistic multi-turn-with-tools sessions — the exact category of session this product targets. The failure is not caught by `TestTruncate_PairAtomicity` because the test is single-pair. Going live with this would produce opaque 400s at dispatch.
- **Suggested fix:** Rewrite D4 step 4 as an algorithm, not a prose rule: after step 3's cutoff is chosen, iterate the kept prefix forward and drop any `tool_result` whose `tool_call_id` has no matching `tool_use` earlier in the kept prefix. Symmetrically, an assistant `tool_call` with no later matching `tool_result` in the kept prefix should also be dropped (the model's response to a call it no longer sees has nothing to ground). Add two test cases: `TestTruncate_OrphanedToolResult_Dropped` (drop tool_call turn but keep a later tool_result → the tool_result is removed from output) and `TestTruncate_OrphanedToolCall_Dropped` (tool_result turn is dropped, tool_call is kept, no subsequent result → the tool_call is removed). Also document: if pair-cleanup reduces messages below the one-user-message floor, return `ErrWindowTooSmall` (or a new `ErrNoViableTruncation`).

### B-04. D5 Anthropic / OpenAI mapping tables omit wire details the implementer will need

The D5 tables are roughly correct at the "shape" level but miss details that FEAT-0008 and both providers' actual contracts require:

**Anthropic (D5 row on tool_use, line 159):**
- The row reads `{"type": "tool_use", "id": "...", "name": "...", "input": {...}}` — correct, but the row does not say that `input` is a JSON OBJECT (not a string). Since `provider.ToolCall.Input` is `json.RawMessage`, implementers need to write `"input": call.Input` (raw embed), NOT `json.Marshal(string(call.Input))`. Without this note the implementer may mistakenly apply the OpenAI quirk (D6) to Anthropic. Cf. D6 second paragraph "Anthropic has no such quirk — `tool_use.input` is a JSON object directly." — this is correct in D6 but should be in D5 too so the tables are self-contained.
- The row for `tool_result` (line 160) says `{"type": "tool_result", "tool_use_id": "...", "content": "..."}`. Anthropic allows `content` to be either a string OR an array of content blocks (including images). FEAT-0008 `ToolResult.OutputType` is `text|json|binary|image` — but D5 only maps `text` cleanly. The `image` output envelope (FEAT-0008 line 247 and the unified Read dispatch at line 100) requires `content` be an array of blocks. Not handling this means image tool results (a core feature — screenshots from a browser tool, etc.) go through as text.
- The row for images in attachments (line 161): Anthropic's `source.type` can be `base64` (as shown) OR `url` (introduced in late 2024). The design picks base64, which is defensible, but should be stated as an explicit decision. Not blocking.
- The row "System prompt" (line 157) says top-level `system` (string). Anthropic also accepts `system` as an array of content blocks (for cache_control and multi-part system prompts). The design picks string; FEAT-0008 line 652 says "The total is included in the provider request as the system message (Anthropic `system` parameter, OpenAI `system` role message, etc.)" — string form works. Risk register in Risks section correctly defers cache_control. Not blocking.
- **MISSING:** `stop_reason` mapping on outbound is N/A (outbound request has no stop_reason), but `stop_sequences` field (user-configurable stop strings) is commonly needed. Not in FormatMessagesOpts. Noted in A-02 below; can be added without break.

**OpenAI (D5 row on tool message, line 186):**
- Row says `{"role": "tool", "tool_call_id": "...", "content": "..."}`. `content` can now be an array of blocks in the newer OpenAI API (same as user messages). But more importantly, the row says "one message per result" — correct. What it misses: the assistant `tool_calls` message REQUIRES `content: null` OR `content: ""` (not omitted). Several OpenAI client libraries default `content` to omitted if empty, which OpenAI's API rejects for assistant-with-tool-calls-only messages. The row at line 185 (`{"role": "assistant", "tool_calls": [...]}`) must produce `content: null` explicitly when `Message.Content` is empty. The design does not say.
- Row for `Attachment{ContentType: "image/*"}` (line 187): `{"type": "image_url", "image_url": {"url": "data:<type>;base64,<data>"}}` — correct data URL form. But the enclosing `content` must then be an ARRAY not a STRING for that user message. If the user message has both text and an image, content becomes `[{"type": "text", "text": "..."}, {"type": "image_url", ...}]`. The row at line 183 says `{"role": "user", "content": "..."}` for plain text — the adapter needs a branch: if attachments present, content is array; else string. Not stated.
- **MISSING:** `output_type` from `protocol.ToolResult` (values `text|json|binary|image` per FEAT-0008 line 247) is not mapped. The design treats ToolResults as having only an `Output` string and `IsError` bool (D1 line 58-62) — dropping both `OutputType`, `Error`, and `Reason` fields that are on `protocol.ToolResult` (messages.go:77-84). `IsError` alone does not carry "rejected" vs. "error" distinction that FEAT-0008 `turn.submit` payload line 245 requires.
- **MISSING:** Vision capability gating. FEAT-0008 line 100: "if an image is read and the current model lacks `vision`, the server can note this in the response context." The adapter should refuse or downgrade image attachments when the model's capability set does not include `vision`. `FormatMessagesOpts` does not carry capabilities; design does not address.

**Why blocking:** Any one of these individually is a "table polish" item. In aggregate — the three missing OpenAI nulls, the string-vs-array content branch, and the lost `OutputType`/`Error`/`Reason` fields — they make WU-043 and WU-044 implementations ship subtly wrong. `turn.submit` with a `tool_result` carrying `status: "rejected"` should surface to the model as an explicit rejection, not an "error" flag. Discarding that distinction loses the product's tool-rejection UX.

- **Suggested fix:**
  1. Add a note under D1's `ToolResult` type: map `protocol.ToolResult` → `provider.ToolResult` preserving `Status`, `OutputType`, `Error`, `Reason`. Then map these to provider wire format per the appropriate provider's supported tool_result block shapes (Anthropic uses `is_error: true` for error AND rejected; OpenAI does not have a separate rejected state — the adapter synthesizes "User denied: <reason>" text for `status: "rejected"`).
  2. Anthropic D5 row: add explicit note that `tool_use.input` is a JSON object (`json.RawMessage` written raw, no string-wrap). Add note that `tool_result.content` is an array of blocks when OutputType is `image` or `binary`.
  3. OpenAI D5 row: state that assistant-with-tool-calls-only messages MUST include `content: null`. State that user messages with attachments use array-form `content`. Map `output_type == "image"` → `[{"type": "image_url", ...}]` content; map `status: "rejected"` → text prefix.
  4. Add `Capabilities []string` (or pointer to model metadata) to `FormatMessagesOpts`; adapters can gate vision-requiring attachments. Alternatively, explicitly defer vision gating to WU-052 dispatch with a TODO.

### B-05. `FormatToolDefinitions` returns `[]byte` for "just the tools field value" — composition contract undefined

- **What:** D2 (line 92) says `FormatToolDefinitions` returns "the `tools` field value ready to embed in a FormatMessages opts (not the full request body — FormatMessages composes the two)." Return type: `[]byte`. But (a) `FormatMessagesOpts.Tools` is `[]protocol.ToolDefinition` (line 103), not `[]byte` — so the splice point is NOT in the opts struct; (b) if `FormatMessages` internally calls `FormatToolDefinitions` and then embeds the bytes, the embedding must be concatenation into a JSON document — which means: build the outer document as a `map[string]any` and then rescan/merge? Or treat the bytes as `json.RawMessage` and embed? The design does not say. (c) Test plan line 281 says "Tool definitions: canonical `ToolDefinition` → Anthropic `tools` array" — this tests `FormatToolDefinitions` output directly, but there is no test for the splice product.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:89-104` — interface methods and opts struct.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:164-175` — Anthropic "Full wire body" JSON shows `"tools": [...]` embedded inline.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:191-200` — OpenAI wire body same.
- **Why blocking:** Implementers will do one of three things: (1) call `FormatToolDefinitions` from within `FormatMessages` internally (so the external method is dead weight for the dispatch path); (2) treat the opts.Tools field as input and ignore the standalone method; (3) implement both and diverge. Option (1) is cleanest — but then why is `FormatToolDefinitions` on the interface at all? If the answer is "registering with a tool broker / capability diff", that needs to be stated. If the answer is "for WU-055 prompt-engine inspection", that needs to be stated. As written, the two methods' relationship is load-bearing but undocumented.
- **Suggested fix:** Either (a) define `FormatToolDefinitions` as the in-library helper that `FormatMessages` calls internally and document that external callers use it when they need the tools shape in isolation (e.g., for logging or debugging), OR (b) change `FormatMessagesOpts.Tools` type to `json.RawMessage` (or `[]byte`) to create a true splice point — caller formats once, dispatch inserts. Option (a) is simpler. Either way, add an integration test: build opts with `Tools` populated, call `FormatMessages`, unmarshal the output, assert `tools` field present and structurally equals a separate `FormatToolDefinitions` call on the same tools. This is the only test that catches composition drift.

## Attention findings

### A-01. ADR amendment front-matter uses keys (`supersedes`, `amends`) not defined in the repo's ADR schema

- **What:** D7 specifies amendment front-matter `status: accepted, date: 2026-04-16, supersedes: none, amends: 0006-multi-provider-support.md`. But `.sdlc/adr/README.md:54-58` defines the schema as `status: proposed | accepted | superseded by ADR-NNNN | deprecated; date: YYYY-MM-DD; decision-makers: Name, Name (optional)`. The schema does NOT define `amends` as a front-matter key; it handles supersession via `status: superseded by ADR-NNNN` in the body. No existing amendment files exist in `.sdlc/adr/`, so the design is proposing a novel convention.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:219-220` — front-matter design.
  - `.sdlc/adr/README.md:54-58` — schema (no `amends`, no `supersedes` as bare key).
  - `.sdlc/adr/README.md:41-47` — "Naming: Files: `NNNN-short-title.md` — four-digit zero-padded sequence." Amendment filename `0006-amendment-001-outbound-formatting.md` is non-standard (introduces the suffix `-amendment-NNN-`).
  - `.sdlc/adr/README.md:80-86` — Lifecycle covers "Propose / Review / Accept / Supersede" — no Amend lifecycle.
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md` (protocol bundle design, reviewed separately) did not take a stance on amendment format.
- **Recommended disposition:** **fix in design now** — either (a) promote this to a full new ADR (ADR-0014) that supersedes ADR-0006, per the lifecycle the README actually defines; (b) add the amendment convention to `.sdlc/adr/README.md` as an `ADMIN:` change, then write the amendment under that new convention (define `amends:` key, the `NNNN-amendment-NNN-title.md` filename, and the relationship to the parent ADR's status); or (c) append the amendment to the existing `0006-multi-provider-support.md` as a new section ("Amendment 001: Outbound formatting, 2026-04-16") and update the parent ADR's front-matter date. Option (b) is the most work but aligns with the doc taxonomy. Option (c) is cheapest. Option (a) is heavy-handed for what is genuinely an extension, not a reversal, of ADR-0006. The design's current proposal is closer to (b) but skips the README update step. Pick one and commit to it.

### A-02. `FormatMessagesOpts` is missing fields real dispatch will need

- **What:** The opts struct (D2, lines 95-104) has: Messages, SystemPrompt, WindowSize, Model, MaxTokens, Temperature, Stream, Tools. Missing:
  - `StopSequences []string` (both providers support; Anthropic `stop_sequences`, OpenAI `stop`). FEAT-0008 does not require but FEAT-0008 §"System Prompt Engine" Layer 5 mode prompts (plan/build/auto) can benefit from mode-specific stop markers.
  - `TopP *float64` (both providers). Called out in D2 as a future-friendly reason to use a struct, which is fine, but listing it in the deferred set would be clearer.
  - `ToolChoice` (`"auto"` / `"none"` / `"any"` / `{tool_name: ...}` on Anthropic; `"auto"` / `"none"` / `{type: function, function: {name}}` on OpenAI). Without this, the server cannot force tool-use or forbid it for mode-specific control (e.g., plan mode line 660 wants only read-only tools; ToolChoice could enforce).
  - `Capabilities []string` — see B-04 for vision gating.
  - `User` / `Metadata` / `RequestID` — provider-side telemetry field (OpenAI `user`, Anthropic `metadata.user_id`). FEAT-0010 enterprise auth (referenced in FEAT-0008) may require.
  - `N int` (OpenAI only, number of completions). Multi-model in FEAT-0008 is BFF-managed parallel branches, not provider-side N — so skipping is defensible but should be stated.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:95-104` — opts struct.
  - `.sdlc/features/0008-bff-server.md:660` — plan-mode read-only tool expectation.
  - `.sdlc/features/0008-bff-server.md:100` — vision-capability gating.
- **Recommended disposition:** **fix in design now (cheap)** — add a "Deferred opts fields" note to D2 explicitly listing StopSequences, TopP, ToolChoice, Capabilities, User/Metadata as acknowledged-deferred, with a one-line rationale per entry. This converts the "future parameters are zero-break" promise into a concrete roadmap and prevents the next WU from rediscovering them.

### A-03. Token estimation (D4) does not define what is counted

- **What:** D4 says `EstimateTokens(s string) int` using chars/4. But a `Message` has Content (text), ToolCalls (structured JSON), ToolResults (structured output), Attachments (file bytes). Truncation walks "Messages" and sums the budget. What does per-message estimation count? Just `Content`? `Content + serialized tool_calls + serialized tool_results`? `Content + Attachments.Content`? The chars/4 heuristic only applies to natural language; tool_call JSON schemas and image bytes have very different token density.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:140-148` — D4 token estimation section; chars/4 formula; no per-message breakdown.
  - `.sdlc/features/0008-bff-server.md:976` — "the server maintains a running token count for the full conversation (system prompt + history + knowledge injections + current turn)" — mentions aggregate, not per-field decomposition.
  - `.sdlc/features/0008-bff-server.md:329-330` — `session.context.list` response has `system_prompt_tokens: 4200, knowledge_injection_tokens: 1800` — the product exposes per-category breakdowns to the user, so the server must track them somewhere.
- **Recommended disposition:** **fix in design now** — define `EstimateTokens(Message)` as a separate helper (or wrap `EstimateTokens(string)` to handle a Message via `Content + json.Marshal(ToolCalls) + json.Marshal(ToolResults) + sum(Attachments.Content)`) and explicitly state that `Attachment.Raw` (binary/base64) is NOT counted — it is the transformed `Content` that the model sees. Add a `TestEstimateTokens_MessageDecomposition` test.

### A-04. `FormatMessages` return convention — wire-ready bytes vs. body struct — is under-specified

- **What:** D2 says the method returns "the complete HTTP request body (JSON-serialized) ready to send to the provider's API endpoint." But: (a) provider SDK integration work in the future (if the adapter uses a Go SDK rather than raw HTTP) would benefit from returning a typed struct; (b) capture/logging (ADR-0005) wants the exact bytes sent, so wire-ready bytes is good for fidelity — but dispatch middleware (rate limiting, retry, request signing) may want the struct; (c) tests compare against golden JSON — but byte-level comparison is brittle to `json.Marshal` field ordering. The design picks the bytes path but does not call out why (logging fidelity?) or address the test brittleness (a golden-JSON comparison should normalize field order).
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:84-87` — return type `[]byte`, comment "complete HTTP request body."
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:272-273` — test plan "Table-driven against golden-JSON fixtures (inline in test, not separate files)" — no normalization mentioned.
- **Recommended disposition:** **fix in design now** — state the rationale (ADR-0005 capture-fidelity / straight pass-through to http.Request.Body). State the test comparison approach: unmarshal the output into `map[string]any`, compare against unmarshalled golden — NOT byte-for-byte. Field ordering in `json.Marshal` of a struct is stable (by struct field declaration order) but field ordering of a `map` is NOT. This matters because D5 Anthropic wire shows `model, max_tokens, system, tools, messages, temperature, stream` ordering — if the implementation uses a struct with that field order, golden bytes match. If it uses `map[string]any`, they don't. Pick the struct path and document it.

### A-05. Deviation-from-track-spec log omits the Tool / canonical type renames

- **What:** The "Deviations from track-0-shared.md" section (lines 306-310) lists (1) the opts struct, (2) Message location, and (3) the explicit pair-atomicity rule. It does NOT list:
  - `WU-042` spec (track-0-shared.md line 61) says the canonical Message carries `(role, content, tool_calls, tool_results, attachments, metadata)`. Design adds `ToolCalls`, `ToolResults`, `Attachments`, `Metadata` — aligned — but also creates new types `provider.ToolCall`, `provider.ToolResult`, `provider.Attachment` that share names with `protocol.*` counterparts. The track spec does not specify these named types exist in the provider package; they could have been aliases. The design's "they're deliberately named-identically" decision (D1 note) is a real deviation worth calling out.
  - `WU-042` spec says the canonical Message `metadata` — design restricts to `map[string]any`. Fine, but the reserved keys (turn_id, branch_id, timestamps) are design-new.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:306-310` — Deviations list.
  - `.sdlc/releases/v0.2.0/track-0-shared.md:60-66` — WU-042 spec.
- **Recommended disposition:** **fix in design now (cheap)** — add the two missing deviations to the Deviations section so TPM updates the track spec in lockstep.

### A-06. `provider.ToolResult.IsError` drops FEAT-0008 `status` tri-state (`success / rejected / error`)

- **What:** D1 `provider.ToolResult` (lines 58-62) has `IsError bool`. Shipped `protocol.ToolResult` (messages.go:77-84) has `Status string` (`success|rejected|error`), plus `Error` (required when status=error) and `Reason` (required when status=rejected). Collapsing to a bool loses the rejected-vs-error distinction that FEAT-0008 line 131 (`"status": "rejected", "reason": "user_denied"`) makes visible to the user and the model.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:58-62`
  - `internal/protocol/messages.go:77-84`
  - `.sdlc/features/0008-bff-server.md:113-141` — success/rejected/error examples.
- **Recommended disposition:** **fix in design now** — `provider.ToolResult` should be `{ ToolCallID string; Output string; Status string; Error string; Reason string }` (mirroring protocol, minus `OutputType` which is wire-only). Adapters then map Status to is_error: Anthropic `is_error: true` for both "error" and "rejected" with Output text including the reason/error; OpenAI has no is_error field, so rejection/error becomes text content prefixed with "[error]" or "[rejected: <reason>]". See B-04.

### A-07. Missing ErrNoViableTruncation / ErrToolPairOrphan error cases

- **What:** D8 lists `ErrNotImplemented`, `ErrWindowTooSmall`, `ErrEmptyMessages`. D4 step 5 says "If step 4 leaves fewer than one user message, return error." What error? `ErrEmptyMessages`? `ErrWindowTooSmall`? Neither is semantically right ("empty input" vs. "truncated to empty"). And once B-03 is fixed, the orphan-tool case is a third distinct failure mode.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:138-139` — step 5 error mention.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:232-236` — D8 sentinel list.
- **Recommended disposition:** **fix in design now (cheap)** — add `ErrTruncationEmpty` (truncation produced no messages) and, once B-03 is adopted, `ErrTruncationOrphan` (could not preserve tool-call pair atomicity). Or collapse them into a single wrapped error with typed reasons.

### A-08. Token replay / continuation of partial turns is not considered

- **What:** FEAT-0008 §"In-Flight Turn Recovery" line 469 includes `token_replay_available: false` — a capability for resuming streams mid-turn after reconnection. `FormatMessages` is called to dispatch a new turn; but what about resuming a previously-started assistant turn that was cut off mid-stream? The design does not address whether a partial assistant message (`Message{Role: "assistant", Content: "Hello I was about to sa"}`) gets formatted as final assistant content, dropped, or becomes a new system prompt. Both Anthropic and OpenAI have different conventions for continuation.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:14` — WU-042/043/044 scope.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:17-24` — Out of scope: "Conversation state construction (WU-051)".
  - `.sdlc/features/0008-bff-server.md:469` — `token_replay_available` flag.
- **Recommended disposition:** **defer explicitly** — state in the Risks section that partial-turn continuation is WU-064's responsibility and `FormatMessages` assumes complete turns only. Or add a minimal note that incomplete assistant turns get dropped by WU-051 before the canonical list reaches `FormatMessages`. Not blocking, but the design should say so.

### A-09. OpenAI `max_completion_tokens` vs. `max_tokens` divergence not handled

- **What:** The shipped `openai.go` parser already handles both fields (`internal/provider/openai.go:45-46, 59-62`) — OpenAI deprecated `max_tokens` for newer chat models in favor of `max_completion_tokens`. The D5 OpenAI full-wire-body (line 191-200) shows only `"max_tokens": 4096`. The design does not specify which field to emit, nor whether the adapter should switch based on model.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:191-200` — wire body.
  - `internal/provider/openai.go:44-46` — struct `openaiRequest` fields both present.
  - `internal/provider/openai.go:59-62` — parser prefers `MaxTokens` over `MaxCompletionTokens`.
- **Recommended disposition:** **fix in design now (cheap)** — state the outbound policy: emit `max_tokens` for legacy chat models, `max_completion_tokens` for o1/o3/o4/gpt-5 reasoning models, or emit both for forward/backward compat. Alternatively, defer with a TODO citing the model-metadata-driven choice at WU-052.

### A-10. The D6 OpenAI JSON-string quirk has a subtle encoding bug potential

- **What:** D6 says `argsBytes, _ := json.Marshal(string(call.Input))`. Suppose `call.Input` is `{"path":"foo bar", "content":"café"}` (contains a space, embedded double-quotes, and UTF-8 non-ASCII). `string(call.Input)` is the raw bytes as a Go string (UTF-8 since `json.RawMessage` is UTF-8 JSON). `json.Marshal` of that string re-encodes it as a JSON string with quote-escaping and `\u` for non-ASCII if Go's default policy applies. Result is a correctly escaped JSON string literal — which OpenAI then parses back and re-parses as JSON. That round-trip is correct for ASCII but non-ASCII characters may be emitted as `\u` escapes (Go's encoder defaults to escaping `<`, `>`, `&` via `HTMLEscape`, and will `\u`-escape control chars, but NOT emit `\u` for printable UTF-8). So "café" round-trips as `"café"` inside the JSON string (good). But `<script>` inside a tool input round-trips as `\u003cscript\u003e` (because `HTMLEscape` is on by default on `json.Marshal`), which when OpenAI re-parses will collapse back to `<script>` — functionally correct. **Edge case:** what if `call.Input` is the literal bytes `{"a":1}` (trailing whitespace-free, well-formed)? `string(call.Input) = "{\"a\":1}"`, Marshal → `"{\"a\":1}"` — correct. **Actual failure case:** if `call.Input` is NOT valid JSON (e.g., truncated: `{"a":1`), `string()` passes through fine but OpenAI's parse on the receiver side fails. The design does not say whether `FormatMessages` validates that `ToolCall.Input` is well-formed JSON before embedding.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:205-211` — D6 code snippet with ignored error.
- **Recommended disposition:** **fix in design now** — (a) validate `call.Input` with `json.Valid(call.Input)` before using; return an error if invalid; (b) the ignored `_` in `json.Marshal(string(call.Input))` on line 208 should be checked — it can fail on non-UTF-8 bytes, which is a real failure mode when upstream schema drift lets bytes leak through. Use `jsonEncoder.SetEscapeHTML(false)` if the HTML-escape round-trip matters (usually doesn't for tool arguments). Add a test for: non-ASCII (UTF-8), embedded double-quotes, embedded backslashes, embedded `<script>`, malformed input (expect error).

### A-11. Stubs test plan (line 266) asserts stubs return `ErrNotImplemented` but doesn't specify opts

- **What:** `TestProvider_FormatMessages_Stub` will invoke the method; with what opts? Empty zero value? A minimal valid opts? If zero-value, the test is trivial; if minimal valid, the test is half of WU-043/044 fixtures. Does `FormatToolDefinitions` get a corresponding stub test?
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:266` — test listed without opts detail.
  - `.sdlc/history/2026-04-16-design-provider-formatting-042-043-044.md:121-126` — both methods are stubbed; test covers FormatMessages only.
- **Recommended disposition:** **fix in design now (cheap)** — expand to `TestProvider_FormatMessages_Stub_Anthropic`, `TestProvider_FormatMessages_Stub_OpenAI`, `TestProvider_FormatToolDefinitions_Stub_Anthropic`, `TestProvider_FormatToolDefinitions_Stub_OpenAI`; each calls with `FormatMessagesOpts{}` / `[]protocol.ToolDefinition{}` respectively, asserts `(nil, ErrNotImplemented)`.

## Nit findings

### N-01. `Message.Role` should be a typed constant set or enum

The D1 `Message.Role string` comment "user | assistant | system | tool" is closed. `protocol.Mode` (messages.go) uses a typed-string pattern. Precedent suggests `type Role string; const (RoleUser Role = "user"; ...)`. Minor editorial.

### N-02. Package comment for `internal/provider/message.go` not specified

The design specifies the package comment in `provider.go` (via the shipped code, line 1-4) but does not say whether `message.go` gets its own file-level doc. Go convention is one per package; since `message.go` is in `package provider`, no new package comment is needed. Mention or it looks like an oversight.

### N-03. D2 "idiomatic Go" reference list

D2 cites `http.Server` and `tls.Config` as precedent for struct-opts. `http.Server` is a receiver type (you set fields then call methods), not a function-opts pattern. `tls.Config` is similar. The closer precedent for function-opts is functional-options or dedicated-opts-struct — e.g., `http.Request{}` passed to `http.Client.Do`. Cosmetic; the deviation rationale is sound.

### N-04. The CompactPlan / stream format hand-off to WU-053 is implicit

Test plan line 279 "Context window truncation: 10 turns with small window → older turns dropped" implicitly covers the truncation output, but the Risks section doesn't note that WU-053 (streaming relay) consumes the same formatted body via different transport (SSE vs. buffered). Neither affects this bundle; mentioning keeps continuity clean.

### N-05. ADR amendment commit point omitted from track-0 spec

Track-0-shared.md line 58-66 (WU-042 DoD) lists "ADR amendment written" as a single deliverable. It does not specify a separate commit point — the design's "Ships in the same commit as WU-042 interface extension" (line 226) is reasonable but the track spec should mirror the commit-point discipline from other WUs. TPM-level nit.

### N-06. `FormatMessagesOpts.WindowSize int` unit is implicit

Field comment line 98: "max total tokens for truncation." The unit is tokens, not bytes or chars. Adequate but explicit typing (e.g., `WindowSizeTokens int`) or a package-level constant like `const MaxWindowSize = 1_000_000` would document the unit better.

## Coverage table

| FEAT-0008 section / spec element | Design doc coverage | Notes |
|----------------------------------|---------------------|-------|
| Provider Message Format Translation (line 542-558) | partial | Core translation covered; vision capability gating missing (B-04); tool_choice absent (A-02); Responses API / Ollama explicitly deferred (scope list). |
| Tool call round-trips (line 60) | partial | Single-pair case covered; multi-assistant-turn interleaving not handled (B-03). |
| Tool catalog schema (line 66-98) | complete | `FormatToolDefinitions` consumes `protocol.ToolDefinition`; shape matches. Composition contract undefined (B-05). |
| `turn.submit` attachments (line 228-236) | blocked | `provider.Attachment` drops `Path`; mistypes `Raw`; see B-01. |
| `turn.submit` tool_results (line 242-251) | blocked | `provider.ToolResult.IsError bool` drops tri-state (success/rejected/error); see A-06 and B-04. |
| System Prompt Engine (line 560-654) | complete (delegated) | `SystemPrompt string` assumed pre-assembled by WU-055. Delegation is correct; design states so (D2 rationale, scope list). |
| Context Window Management (line 972-1017) | partial | Truncation algorithm present (D4); orphan-pair edge cases missing (B-03); token estimation decomposition unclear (A-03); compaction delegated to WU-061 correctly. |
| Canonical Field Names (line 361-373) | N/A | Provider layer below the canonical-field surface. |
| Multi-Model Roles (line 795-802) | N/A | BFF-level parallelism, not provider-level; correctly out of scope. |
| Model Transparency (line 808-886) | partial | `model.selected` / capability metadata NOT in `FormatMessagesOpts` (A-02 vision gating). |
| In-Flight Turn Recovery (line 447-495) | partial | Turn-continuation not considered (A-08). |
| ADR-0006 amendment shape | blocked | Front-matter schema does not match `.sdlc/adr/README.md` convention; see A-01. |
| Track-0-shared.md WU-042 spec (line 54-66) | complete (with declared deviations) | Three deviations declared; two additional deviations missing from the log (A-05). |
| Track-0-shared.md WU-043 spec (line 70-83) | complete | All eight DoD sub-items addressed in test plan; wire-table gaps (B-04) affect correctness but not coverage. |
| Track-0-shared.md WU-044 spec (line 86-98) | complete | Same as WU-043, with the OpenAI-specific quirks. |

## What I did NOT review

- **FEAT-0008 spec correctness itself.** Treated FEAT-0008 as source of truth for product intent; did not audit internal consistency. If FEAT-0008 is wrong about (for example) `protocol.Attachment.Raw` being a string, that drift is out of scope.
- **Wire-format correctness against the actual Anthropic and OpenAI API specs.** I have reasonable confidence in the shape of both mappings (and I called out the specific omissions I could verify from memory), but I did not pull up the provider docs to line-by-line check e.g. `tool_use` vs. `tool_use[]` semantics, the exact `image` block `source.media_type` spelling, or the `max_tokens` default. A cross-model reviewer with provider-doc search (Codex / Kimi / GPT-5 agents) should verify D5 against the providers' current schemas. Same-model blind spot.
- **Go idiom review in depth.** Did not check whether `FormatMessagesOpts` with `Temperature *float64` + `Tools []protocol.ToolDefinition` mixed-pointer style is idiomatic vs. all-value or all-pointer.
- **Round-trip correctness of the type-separation decision (D1).** The design claims `provider.*` and `protocol.*` types are meaningfully different. I did NOT attempt to enumerate every field that would need to cross the boundary and verify a lossless conversion helper can be written. B-01 is one clear break; there may be others.
- **Benchmark / performance.** `EstimateTokens` chars/4 is O(n) per message × every truncation call; on a 100-turn session that's fine. Not measured. Not reviewed.
- **Interaction with WU-051 (conversation state) and WU-052 (dispatch).** Those are declared out of scope; I did not verify that the hand-off shape (what WU-051 produces and passes to `FormatMessages`) is captured by `FormatMessagesOpts`. Re-read after WU-051 design lands.
- **Security / prompt-injection through tool results.** `provider.ToolResult.Output` passes through verbatim; the design does not address whether malicious tool output could construct adversarial prompt fragments that get formatted into a provider body. FEAT-0008 does not require this be at the provider layer — but WU-044 especially, with its arguments-as-JSON-string quirk, is a place where injection via crafted JSON strings could be a future concern.
- **Cross-bundle conflicts with the protocol-types bundle (WU-040/041/093)** and the storage bundle (WU-045/091/096). Reviewed separately by the same sub-agent chain. The `protocol.Attachment` / `provider.Attachment` mismatch (B-01) is within-bundle; if the protocol bundle's design revises the shape, this review's B-01 may be absorbed or intensified.
