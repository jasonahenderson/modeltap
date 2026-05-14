# 2026-04-16 — Design: Provider Formatting Bundle (WU-042 + WU-043 + WU-044)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Review Tier

**Assigned:** C (bundled)
**Basis:** Shared interface extension + ADR-0006 amendment (both are C triggers per the rules); two concrete implementations (Anthropic, OpenAI) depend on the same interface.
**Plan default:** C (matches — all three WUs tagged C in track-0-shared.md)
**Escalation reason:** n/a

## Scope

Bundled design for three related WUs:

- **WU-042** — ADR-0006 amendment + `Provider` interface extension with canonical `Message` type, `FormatMessages`, `FormatToolDefinitions`. Stub implementations on Anthropic and OpenAI adapters (return `ErrNotImplemented`).
- **WU-043** — Anthropic adapter: implement `FormatMessages` and `FormatToolDefinitions` producing Anthropic Messages API request bodies.
- **WU-044** — OpenAI adapter: same for OpenAI Chat Completions format.

**Out of scope (deferred):**
- Dispatch path (WU-052)
- Conversation state construction (WU-051)
- Streaming relay (WU-053)
- Provider endpoint configuration (WU-057)
- Ollama adapter (WU-066)
- Provider-specific streaming parse (already shipped in WU-001-038)

## Bundle rationale

WU-042 defines the interface; WU-043 and WU-044 are its two reference implementations. Designing them together means the interface shape is stressed against two real provider formats before we commit to it — catching interface mistakes before both adapters re-implement the same workaround.

## Design Decisions

### D1. Canonical `Message` type lives in `internal/provider/message.go`

The canonical form is an internal server concept (WU-051 conversation state assembles it; adapters consume it). It is NOT a wire-format type, so it does not live in `internal/protocol/`.

```go
// Package provider (new file: message.go)

// Message is the canonical representation of a single conversational turn
// or continuation. The BFF assembles []Message from persisted session
// state; each Provider.FormatMessages translates the canonical form into
// the provider's wire format.
type Message struct {
    Role        string         `json:"role"`      // user | assistant | system | tool
    Content     string         `json:"content"`   // primary text content; may be empty if tool_calls/results are the payload
    ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`   // assistant role: tool invocations produced by the model
    ToolResults []ToolResult   `json:"tool_results,omitempty"` // tool role OR user continuations: results of executed tools
    Attachments []Attachment   `json:"attachments,omitempty"`  // files/images attached to this message
    Metadata    map[string]any `json:"metadata,omitempty"`     // optional provenance: turn_id, branch_id, timestamps
}

type ToolCall struct {
    ID    string          `json:"id"`
    Name  string          `json:"name"`
    Input json.RawMessage `json:"input"` // arguments matching the tool's input_schema
}

type ToolResult struct {
    ToolCallID string `json:"tool_call_id"`
    Output     string `json:"output"`
    Status     string `json:"status"` // success | rejected | error — mirrors protocol.ToolResult.Status
    Error      string `json:"error,omitempty"`  // populated when Status == "error"
    Reason     string `json:"reason,omitempty"` // populated when Status == "rejected"
}

type Attachment struct {
    Path        string `json:"path"`          // project-relative file path (required; preserves files-touched tracking)
    Raw         string `json:"raw"`           // base64-encoded original bytes — matches protocol.Attachment.Raw type exactly so convert.go is lossless
    Content     string `json:"content"`       // extracted text representation (what the model sees for text-extract transforms)
    ContentType string `json:"content_type"`
    Transform   string `json:"transform"`
}
```

**Naming note:** these types deliberately share names with `protocol.*` types (`ToolCall`, `ToolResult`, `Attachment`). They are NOT the same types — `provider` types carry server-internal provenance (the `Metadata` field on `Message`); `protocol` types are wire-only. Field shapes (Path, Raw as base64 string, Content, ContentType, Transform for Attachment; full tri-state Status for ToolResult) mirror the wire types exactly so `convert.go` helpers are pure field-copy with no encoding/decoding — the canonical form has the same byte-level payload as the wire form, just with additional `Metadata` bolted on at the `Message` level. Conversion helpers live in `internal/provider/convert.go` and are added as WU-051 (conversation state) needs them.

**On ToolResult tri-state:** keeping `Status` + `Error` + `Reason` at the canonical layer preserves the FEAT-0008 `success / rejected / error` distinction all the way to the adapter. Adapters then map to provider wire format: Anthropic uses `is_error: true` for both "error" and "rejected" (with Output text prefixed to carry the distinction); OpenAI has no is_error concept at all, so rejection/error appears as text prefixed with `[error: ...]` or `[rejected: ...]`. Adapter-level mapping is specified in D5.

### D2. `Provider` interface extension

Add two methods to the existing `Provider` interface (`internal/provider/provider.go`):

```go
type Provider interface {
    // ... existing methods (Name, Detect, ParseRequest, ParseResponse, ReassembleStream) ...

    // FormatMessages translates the canonical conversation into a
    // provider-specific request body. The returned bytes are the complete
    // HTTP request body (JSON-serialized, struct-ordered — see Test note
    // below) ready to send to the provider's API endpoint.
    //
    // Composition with FormatToolDefinitions: FormatMessages calls
    // FormatToolDefinitions internally when opts.Tools is non-empty, splicing
    // the result into the outer request body as a typed field (not a byte
    // splice). Callers of FormatMessages never invoke FormatToolDefinitions
    // themselves for dispatch — the interface method is exposed for
    // logging, debugging, and capability-diff inspection.
    FormatMessages(opts FormatMessagesOpts) ([]byte, error)

    // FormatToolDefinitions translates a canonical tool catalog into the
    // provider-specific tool-definitions wire shape. Returns the bytes of
    // what would be emitted as the "tools" field value (a JSON array), NOT
    // wrapped in an outer request body.
    //
    // Used standalone for: registration logging, capability-diff reporting,
    // UI display of what tool schema was sent. Not used by dispatch — see
    // composition note on FormatMessages.
    FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error)
}

// FormatMessagesOpts groups the inputs to FormatMessages. Tokens is the
// unit for WindowSize and MaxTokens.
type FormatMessagesOpts struct {
    Messages     []Message                  // canonical conversation, in turn order (oldest first)
    SystemPrompt string                     // pre-assembled by WU-055 prompt engine
    WindowSize   int                        // max total tokens for truncation (approximate)
    Model        string                     // provider-specific model identifier
    MaxTokens    int                        // output token cap; Anthropic required, OpenAI optional
    Temperature  *float64                   // optional
    Stream       bool
    Tools        []protocol.ToolDefinition  // optional; empty slice means no tool-use
    Capabilities []string                   // model capabilities (e.g., "vision", "tool_use"); adapter gates attachment types and tool-use accordingly
}
```

**Deferred `FormatMessagesOpts` fields** (added in future WUs without breaking the struct interface):

| Field | Reason deferred | Likely owner |
|-------|-----------------|--------------|
| `StopSequences []string` | Not required by FEAT-0008 for v0.2.0; mode-specific stop markers are a WU-054/055 concern. | WU-055 |
| `TopP *float64` | Not required by FEAT-0008; user-exposed sampling tuning may come later. | future patch |
| `ToolChoice any` | Per-turn tool-use forcing (plan-mode read-only tools per FEAT-0008 line 660). Requires spec amendment for FEAT-0008. | WU-080 |
| `User string` + `Metadata map[string]any` | Provider-side telemetry (OpenAI `user`, Anthropic `metadata.user_id`). FEAT-0010 enterprise auth will need. | WU-057 or FEAT-0010 work |
| `N int` | OpenAI-only multi-completion. Modeltap's multi-model is BFF-managed parallel branches (WU-060), not provider N. Skipped by design. | N/A |

Adding any of these is a zero-breaking-change struct extension.

**Deviation from WU-042 spec:** WU-042 spec writes the signature as `FormatMessages(canonical []Message, systemPrompt string, windowSize int) ([]byte, error)`. This design replaces the loose parameter list with a `FormatMessagesOpts` struct because:
1. Real dispatch requires model name, max_tokens, temperature, stream, and tools — all of which a bare three-argument signature omits.
2. Adding future parameters (e.g., `StopSequences`, `TopP`) is zero-breaking-change with a struct.
3. Matches idiomatic Go for long parameter lists (see also `http.Server`, `tls.Config`).

The spec signature is treated as descriptive (what the function conceptually takes), not prescriptive (exact Go signature). Documented under "Deviations" below; TPM should update the track spec in lockstep.

### D3. Stubs in WU-042 return `ErrNotImplemented`

Before WU-043 and WU-044 fill them in, both adapters have:

```go
func (a *AnthropicProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
    return nil, ErrNotImplemented
}

func (a *AnthropicProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error) {
    return nil, ErrNotImplemented
}
```

The `ErrNotImplemented` sentinel is defined in D8 below (one declaration, one string). The WU-042 DoD is "existing tests still pass (stubs satisfy interface)"; the stubs satisfy the compile-time contract, and no call site invokes them until WU-052 dispatch lands.

### D4. Context window truncation policy

Shared algorithm (lives in `internal/provider/truncate.go` so both adapters share it).

**Stage 1 — budget check.** Compute `systemBudget = EstimateTokens(systemPrompt)`. If `systemBudget >= windowSize`, return `ErrWindowTooSmall` — the prompt engine (WU-055) should have caught this earlier.

**Stage 2 — age-based cutoff.** Walk `Messages` from newest (index N-1) to oldest (index 0). Keep each message while running cumulative message tokens + systemBudget <= windowSize. Let `cutoff` be the oldest index that is kept; everything below `cutoff` is a candidate for dropping.

**Stage 3 — pair reconciliation (the correctness step).** Within the kept prefix `msgs[cutoff:]`, enforce tool-pair invariants. Both providers reject messages arrays where a `tool_result` has no preceding `tool_use`, and messages where a `tool_use` has no later `tool_result` are acceptable to Anthropic but OpenAI may 400:

```
kept = msgs[cutoff:]
seen_tool_use_ids = set()

// Forward pass: collect tool_use ids from assistant messages
for msg in kept:
    if msg.Role == "assistant":
        for call in msg.ToolCalls:
            seen_tool_use_ids.add(call.ID)

// Reconciliation pass: drop orphan tool_results (any ToolResult
// whose ToolCallID is not in seen_tool_use_ids); drop orphan
// tool_calls (any ToolCall with no matching ToolResult later in the
// kept prefix — because the model has no grounding for a call it no
// longer sees the result for).
//
// Note: "drop" here means removing the entry from the message's
// ToolCalls/ToolResults slice. If a message ends up with empty
// Content AND empty ToolCalls AND empty ToolResults, drop the
// message entirely.
```

After reconciliation, if the kept set has zero user messages or is empty, return `ErrTruncationEmpty`. The caller must reduce the conversation some other way (WU-061 compaction) or reject the request.

**Token estimation.** Two helpers:

```go
// Approximate token count for arbitrary text. chars/4 heuristic —
// documented approximate; precise counts come from provider API
// responses after dispatch.
func EstimateTokens(s string) int

// Approximate token count for a full canonical Message: Content +
// json.Marshal(ToolCalls) + json.Marshal(ToolResults) +
// sum(Attachments.Content). Attachments.Raw (base64) is NOT counted —
// it is the transformed Content that the model sees.
func EstimateMessageTokens(m Message) int
```

`Truncate` uses `EstimateMessageTokens` internally.

```go
// In internal/provider/truncate.go
func Truncate(msgs []Message, systemPrompt string, windowSize int) ([]Message, error)

// In internal/provider/tokens.go
func EstimateTokens(s string) int
func EstimateMessageTokens(m Message) int
```

**Multi-assistant-turn interleaving:** the reconciliation pass handles the case where assistant turn T1 emits `tool_call_A`, user turn T2 submits result for A, assistant turn T3 emits `tool_call_B`, user turn T4 submits result for B. If truncation drops T1/T2 and keeps T3/T4, the reconciliation loop sees `tool_call_B` in T3's kept assistant message and finds T4's matching result — T3/T4 stay intact. If truncation drops T1/T2/T3 and keeps T4 only, T4's result has no matching call in the kept prefix → reconciliation drops T4's orphan result, and if T4 then has no content left, drops T4 entirely → `ErrTruncationEmpty`.

**Multi-call single-turn:** a single assistant message with multiple `ToolCalls` pairs with ToolResults across potentially one or multiple following user messages. Reconciliation is per-call-ID, not per-message, so it handles this correctly.

### D5. Wire format mapping tables

#### Anthropic Messages API (WU-043)

| Canonical | Anthropic wire |
|-----------|----------------|
| `SystemPrompt` | top-level `system` (string form; array-of-blocks form with `cache_control` deferred — see Risks). |
| `Message{Role: "user", Content: "..."}` | `{"role": "user", "content": [{"type": "text", "text": "..."}]}` |
| `Message{Role: "assistant", Content: "..."}` | `{"role": "assistant", "content": [{"type": "text", "text": "..."}]}` |
| `Message{Role: "assistant", ToolCalls: [...]}` | `{"role": "assistant", "content": [{"type": "tool_use", "id": "...", "name": "...", "input": <raw JSON object>}]}`. **`input` is a raw JSON object, NOT a string** — write `call.Input` (a `json.RawMessage`) directly as the field value. Do NOT apply the OpenAI string-wrap (D6). One block per call; text + tool_use blocks may coexist in the same content array if Content is non-empty. |
| `Message{Role: "tool", ToolResults: [...]}` or user continuation | `{"role": "user", "content": [...tool_result blocks...]}` (Anthropic emits tool_result under user role, not tool role). Each block: `{"type": "tool_result", "tool_use_id": result.ToolCallID, "content": <content>, "is_error": <bool>}`. `is_error: true` for BOTH `status: "error"` and `status: "rejected"` (Anthropic has no rejected concept). For `status: "rejected"`, the `content` text is prefixed with `"[rejected: " + result.Reason + "] "` + `result.Output`. For `status: "error"`, prefix with `"[error: " + result.Error + "] "`. |
| `Message.ToolResults` where a result has `OutputType == "image"` or `"binary"` | the `content` field becomes an array of content blocks, not a string. For image: `[{"type": "image", "source": {"type": "base64", "media_type": "...", "data": "..."}}]`. For binary: text block with `[binary: <content_type>, <len> bytes — rendered as base64 in tool logs]`. |
| `Attachment{ContentType: "image/*", Raw: <base64 string>}` | image content block in the owning message's content array: `{"type": "image", "source": {"type": "base64", "media_type": "<contenttype>", "data": "<raw-passthrough>"}}`. Raw is the already-base64-encoded string; pass through without re-encoding. |
| `Attachment{ContentType: "text/*"}` | inlined as text content block using `Attachment.Content` (the extracted text). |
| `Tools` (via `FormatToolDefinitions`) | top-level `tools` (array of `{"name": ..., "description": ..., "input_schema": <JSON Schema object from ToolDefinition.InputSchema raw passthrough>}`). Empty `Tools` slice emits no `tools` field (absent, not empty array). |
| Vision gating | If `opts.Capabilities` does not include `"vision"` and any message carries an image `Attachment` or `OutputType == "image"` tool_result, the adapter substitutes a text placeholder (`"[image omitted: model lacks vision capability]"`) rather than sending unsupported content. Per FEAT-0008 line 100. |

**Full wire body:**
```json
{
  "model": "claude-opus-4-6",
  "max_tokens": 4096,
  "system": "...",
  "tools": [...],
  "messages": [...],
  "temperature": 0.7,
  "stream": true
}
```

#### OpenAI Chat Completions (WU-044)

| Canonical | OpenAI wire |
|-----------|-------------|
| `SystemPrompt` | first message: `{"role": "system", "content": "..."}`. Skipped if `SystemPrompt` is empty. |
| `Message{Role: "user", Content: "..."}` (no attachments) | `{"role": "user", "content": "..."}` (string form). |
| `Message{Role: "user"}` **with attachments** | `{"role": "user", "content": [ {"type": "text", "text": Content}, ...attachment blocks ]}` (array form). Array form is required for any user message with non-text content. |
| `Message{Role: "assistant", Content: "..."}` (no tool_calls) | `{"role": "assistant", "content": "..."}` (string form). |
| `Message{Role: "assistant"}` **with only `ToolCalls`** (empty Content) | `{"role": "assistant", "content": null, "tool_calls": [...]}`. **`content: null` is REQUIRED**, not omitted. Some OpenAI client libraries default to omitting; OpenAI's API rejects such messages. |
| `Message{Role: "assistant"}` with BOTH Content and ToolCalls | `{"role": "assistant", "content": "<content>", "tool_calls": [...]}` (string content alongside tool_calls). |
| `ToolCall` element | `{"id": call.ID, "type": "function", "function": {"name": call.Name, "arguments": <JSON-string>}}`. **`arguments` is a JSON-encoded STRING** (OpenAI quirk — D6 details). `call.Input` is validated as valid JSON first (`json.Valid`); invalid inputs return an error rather than emitting broken wire bytes. |
| `Message{Role: "tool"}` with ToolResults | ONE message per `ToolResult` entry: `{"role": "tool", "tool_call_id": result.ToolCallID, "content": <content>}`. Multiple results from one canonical message expand into multiple tool messages. For `status: "error"`, content is prefixed `"[error: " + result.Error + "] " + result.Output`. For `status: "rejected"`, content is prefixed `"[rejected: " + result.Reason + "] " + result.Output` (OpenAI has no is_error field; text prefix carries the signal). |
| `ToolResult.OutputType == "image"` | content uses array form: `[{"type": "image_url", "image_url": {"url": "data:<content_type>;base64,<data>"}}]` instead of a plain string. |
| `Attachment{ContentType: "image/*"}` | content array block in the parent user message: `{"type": "image_url", "image_url": {"url": "data:<content_type>;base64,<Raw>"}}`. Raw is the already-base64 string; pass through. |
| `Attachment{ContentType: "text/*"}` | content array block: `{"type": "text", "text": Content}`. |
| `Tools` (via `FormatToolDefinitions`) | top-level `tools`, each as `{"type": "function", "function": {"name": ..., "description": ..., "parameters": <InputSchema raw passthrough>}}`. Empty `Tools` slice emits no `tools` field (absent). |
| Vision gating | Same rule as Anthropic: missing `"vision"` capability → substitute text placeholder. |
| `MaxTokens` field name | Emit `max_tokens` for legacy chat models. For o1/o3/o4 reasoning models and GPT-5 family, emit `max_completion_tokens` instead. Model-family detection heuristic lives in the adapter (pattern match on Model name prefix); `internal/provider/openai.go` currently accepts both on the parse side. TODO deferred to WU-052 for a cleaner model-metadata-driven choice. |

**Full wire body:**
```json
{
  "model": "gpt-4-turbo",
  "max_tokens": 4096,
  "messages": [{"role": "system", "content": "..."}, ...],
  "tools": [...],
  "temperature": 0.7,
  "stream": true
}
```

### D6. OpenAI `tool_calls[].function.arguments` JSON-string quirk

OpenAI's API requires `function.arguments` to be a **JSON-encoded string**, not a JSON object. Our canonical `ToolCall.Input` is `json.RawMessage` (raw JSON bytes). The OpenAI adapter must (a) validate the input is well-formed JSON and (b) marshal the raw JSON bytes as a JSON string:

```go
if !json.Valid(call.Input) {
    return nil, fmt.Errorf("openai: tool_call %q has invalid JSON input: %w", call.ID, ErrInvalidToolInput)
}
argsString, err := json.Marshal(string(call.Input))
if err != nil {
    return nil, fmt.Errorf("openai: marshaling tool_call %q arguments: %w", call.ID, err)
}
// argsString is the JSON-string-encoded arguments, ready to embed
```

`string(call.Input)` converts the raw JSON bytes to a Go string (UTF-8 preserved). `json.Marshal` of that string re-encodes it as a JSON string literal with quote-escaping. Round-trip: OpenAI response parsing inverts this (unmarshals the string and then the inner JSON). Inbound parsing already handles this in `openai.go` `ReassembleStream`.

**Edge cases tested:**
- ASCII with embedded quotes: `{"a":"b\"c"}` → escaped correctly.
- UTF-8 non-ASCII: `{"name":"café"}` → emitted as `"{\"name\":\"café\"}"` (Go's json.Marshal preserves UTF-8 printable, no `\u` escape).
- HTML-like content: `{"html":"<script>"}` → Go's default `HTMLEscape` emits `\u003c...\u003e`, which OpenAI re-parses back to `<script>` — functionally equivalent but noisier on the wire. **Decision:** leave default `HTMLEscape` on for consistency with stdlib; acceptable noise.
- Malformed JSON input: rejected up front via `json.Valid` with `ErrInvalidToolInput`.

Anthropic has no such quirk — `tool_use.input` is a JSON object directly. Pass `call.Input` as raw JSON with no re-encoding.

### D7. ADR-0006 amendment strategy

The repo's ADR schema (`docs/adr/README.md`) does not define an "amendment" lifecycle or front-matter keys — it supports `status: proposed | accepted | superseded by ADR-NNNN | deprecated`. Filename convention is `NNNN-short-title.md`, four-digit sequence only. Rather than inventing a parallel convention, this bundle appends the amendment directly to the existing ADR-0006 file as a new section.

**Decision:** amend in place.

- **File:** `docs/adr/0006-multi-provider-support.md` (existing)
- **What changes:** add a new top-level section at the end titled `## Amendment 001 — Outbound Formatting (2026-04-16)`, containing: scope of the change, the two interface method signatures, the canonical Message type sketch, the truncation policy summary, and a reference back to this design doc (`docs/history/2026-04-16-design-provider-formatting-042-043-044.md`) for full detail.
- **Front-matter:** keep existing `status: accepted` and `decision-makers`; bump `date` to 2026-04-16 to reflect the revision.
- **No new file.** No `amends:` key invented. No sub-numbered filename.

**Rationale for append-in-place over a new ADR (ADR-0014):**
- The change is an extension of ADR-0006's "Provider adapter interface from day one" decision, not a reversal or a distinct architectural choice. A new ADR would force the reader to read two files to understand one decision.
- ADR-0006's "More Information" section already sketches a 5-method interface and anticipates growth ("the interface must be designed well upfront. A poor interface will require breaking changes when edge cases emerge in later providers."). Extending that same interface with two methods is consistent with that note.
- The existing ADR-0006 scoring matrix still applies unchanged — the scoring compares adapter-interface-from-day-one vs. alternatives, and adding methods to the interface does not move any weights.

**Consequences:** every provider adapter (Anthropic, OpenAI, Ollama via WU-066, future) must implement the two new methods. `ErrNotImplemented` stubs permit incremental rollout per WU-042.

**Confirmation criteria** (unchanged from original ADR-0006 confirmation, plus the new methods):
1. Anthropic and OpenAI adapters implement `FormatMessages` and `FormatToolDefinitions` producing wire-correct bodies.
2. The BFF dispatch (WU-052) calls `FormatMessages` end-to-end.
3. `FormatToolDefinitions` output validates against each provider's tool schema.

The amendment ships in the same commit as the WU-042 interface extension.

### D8. Error sentinels and helpers

New `internal/provider/provider.go` exports (single definition each — these are the canonical values):

```go
var ErrNotImplemented    = errors.New("provider: method not implemented")
var ErrWindowTooSmall    = errors.New("provider: context window too small even for system prompt")
var ErrEmptyMessages     = errors.New("provider: messages slice is empty")
var ErrTruncationEmpty   = errors.New("provider: truncation produced no viable messages")
var ErrInvalidToolInput  = errors.New("provider: tool_call input is not valid JSON")
var ErrUnsupportedOutputType = errors.New("provider: tool_result output_type is not supported by provider")
```

Usage:
- `ErrNotImplemented` — WU-042 stubs; removed when WU-043/044 fill in.
- `ErrWindowTooSmall` — system prompt alone exceeds WindowSize.
- `ErrEmptyMessages` — `FormatMessages` called with `len(opts.Messages) == 0` or post-reconciliation empty (see ErrTruncationEmpty for the explicit truncation case).
- `ErrTruncationEmpty` — `Truncate` ran, reconciled pairs, and had no user messages left; caller needs a different strategy (compaction / reject turn).
- `ErrInvalidToolInput` — OpenAI adapter guard; `json.Valid(call.Input)` returned false.
- `ErrUnsupportedOutputType` — e.g., OpenAI adapter receives a `binary` output_type that has no provider equivalent; adapter chooses to degrade (text placeholder) or reject based on implementation.

Truncation and format functions return these where appropriate.

## File Layout

**New (WU-042):**
- `internal/provider/message.go` — canonical `Message`, `ToolCall`, `ToolResult`, `Attachment` types
- `internal/provider/truncate.go` — shared `Truncate` function + `internal/provider/truncate_test.go` tests
- `internal/provider/tokens.go` — `EstimateTokens` + `EstimateMessageTokens` + tests
- `internal/provider/convert.go` — helpers between `protocol.*` and `provider.*` analogues (consumed by WU-051)

**Modified (WU-042):**
- `docs/adr/0006-multi-provider-support.md` — append "Amendment 001 — Outbound Formatting (2026-04-16)" section; bump front-matter date.
- `internal/provider/provider.go` — extend `Provider` interface, add `FormatMessagesOpts`, add error sentinels
- `internal/provider/anthropic.go` — stub methods returning `ErrNotImplemented`
- `internal/provider/openai.go` — stub methods returning `ErrNotImplemented`
- `internal/provider/anthropic_test.go` — no new tests (stubs)
- `internal/provider/openai_test.go` — no new tests (stubs)

**Modified (WU-043):**
- `internal/provider/anthropic.go` — full `FormatMessages` + `FormatToolDefinitions` implementations
- `internal/provider/anthropic_format_test.go` — table-driven tests

**Modified (WU-044):**
- `internal/provider/openai.go` — full `FormatMessages` + `FormatToolDefinitions` implementations
- `internal/provider/openai_format_test.go` — table-driven tests

## Test Plan Outline

### WU-042 (stub phase)
- `TestProvider_FormatMessages_Stub_Anthropic` — zero-value opts → `(nil, ErrNotImplemented)`.
- `TestProvider_FormatMessages_Stub_OpenAI` — same.
- `TestProvider_FormatToolDefinitions_Stub_Anthropic` — empty tool slice → `(nil, ErrNotImplemented)`.
- `TestProvider_FormatToolDefinitions_Stub_OpenAI` — same.
- `TestTruncate_Basic` — happy-path truncation drops oldest until fits.
- `TestTruncate_SystemTooLarge` — returns `ErrWindowTooSmall`.
- `TestTruncate_PairAtomicity_SinglePair` — single tool_use/tool_result pair dropped together.
- `TestTruncate_OrphanedToolResult_Dropped` — tool_call turn dropped; later tool_result has no matching id in kept prefix; reconciliation removes the orphan result.
- `TestTruncate_OrphanedToolCall_Dropped` — tool_result turn dropped; assistant tool_call is kept; reconciliation removes the orphan call from the assistant's ToolCalls slice.
- `TestTruncate_MultiCallSingleTurn` — one assistant message with two ToolCalls, followed by two ToolResults across one or two user messages. Truncation keeps or drops the whole group atomically based on matching ids.
- `TestTruncate_TruncationEmpty` — reconciliation leaves zero user messages → `ErrTruncationEmpty`.
- `TestTruncate_EmptyInput` — returns `ErrEmptyMessages`.
- `TestEstimateTokens_ApproxCorrect` — chars/4 yields expected values on known samples.
- `TestEstimateMessageTokens_Decomposition` — Message with Content + ToolCalls + Attachments is counted as Content + json.Marshal(ToolCalls) + json.Marshal(ToolResults) + sum(Attachments.Content); Raw bytes NOT counted.

### WU-043 (Anthropic)
Table-driven against golden-JSON fixtures (inline in test, not separate files). **Comparison:** unmarshal output into `map[string]any` and compare against unmarshalled golden — NOT byte-for-byte. Go's `json.Marshal` of a struct is stable (field declaration order), but maps are not, so all adapter-level assembly goes through typed structs with explicit field order matching D5's "Full wire body" example.

- Simple text turn: single user message → correct Anthropic body.
- Multi-turn: 3 user + 3 assistant → 6-element messages array, alternating.
- Tool call round-trip: assistant emits tool_use (input as JSON object, raw passthrough) → user tool_result → assistant final text.
- ToolResult tri-state: `status: "error"` → `is_error: true` with `[error: ...]` prefix; `status: "rejected"` → `is_error: true` with `[rejected: ...]` prefix.
- ToolResult.OutputType == "image": content becomes array with image block.
- System prompt: set at top level, not inline.
- Attachments: PDF (text transform), image (base64 passthrough).
- Vision gating: missing "vision" capability → image attachments substituted with placeholder text.
- Context window truncation: 10 turns with small window → older turns dropped.
- Tool definitions: canonical `ToolDefinition` → Anthropic `tools` array.
- Empty tools: absent `tools` field (not empty array).
- Composition test: opts with Tools populated → unmarshal output, assert `tools` field present and structurally equal to separate `FormatToolDefinitions` call on same tools.

### WU-044 (OpenAI)
Parallel table-driven tests with OpenAI shapes:
- Simple text turn → `messages` with system message.
- Multi-turn: alternating user/assistant.
- Tool call round-trip: arguments as JSON-string (D6).
- Tool call JSON-string edge cases: non-ASCII UTF-8, embedded double-quotes, backslashes, HTML-like content (expect `HTMLEscape` noise), malformed input → `ErrInvalidToolInput`.
- Assistant-with-only-tool-calls: emits `content: null` literal.
- Images: `image_url` with data URL; content becomes array form.
- User-with-attachments: content array with text block + image_url block.
- Attachments: non-image text inlined as text block.
- ToolResult tri-state: `status: "error"` prefix, `status: "rejected"` prefix in content text.
- ToolResult.OutputType == "image": content becomes array with image_url block.
- Vision gating: no "vision" in capabilities → image attachments substituted with placeholder text.
- Reasoning-model MaxTokens: model name matching `o1-*` / `o3-*` / `o4-*` / `gpt-5*` → emits `max_completion_tokens` instead of `max_tokens`.
- Context window truncation: same scenario as Anthropic, different wire format.
- Tool definitions: canonical → OpenAI `{type: function, function: {...}}` wrapping.

### Shared truncation tests
Live in `internal/provider/truncate_test.go`; both adapter tests reuse them implicitly by invoking `FormatMessages` with adversarial inputs.

## Risks and Open Items

- **Risk — deferred:** Token estimation accuracy. chars/4 is a well-known approximation (off by ~15-20% depending on content). For truncation, this is acceptable — we truncate a bit more aggressively than strictly necessary. Real token counts come back in provider responses (`usage.input_tokens`). Later WU (WU-061 compaction) may add tokenizer integration if accuracy bites.
- **Risk — deferred:** Anthropic prompt caching (`cache_control`) not modeled. FEAT-0008 does not require it for v0.2.0. Can be added as a separate field on `FormatMessagesOpts` without interface break.
- **Risk — deferred:** OpenAI function calling vs. tools API. OpenAI deprecated `functions` in favor of `tools`. WU-044 targets the new `tools` format only. Documented in the adapter; older API shape is out of scope.
- **Risk — deferred:** Streaming request format differences. Both providers set `stream: true` in the request body but their stream wire protocols differ (already handled in WU-001-038 `ReassembleStream`). The format direction is simpler: `stream` is just a boolean passed through.
- **Non-risk:** Separate `provider.*` types vs. `protocol.*` types with similar fields. Documented in D1; convert helpers live in `convert.go`.
- **Non-risk:** `FormatMessagesOpts` struct instead of bare signature. Idiomatic Go; deviation documented.

## Deviations from track-0-shared.md

- **WU-042 `FormatMessages` signature.** Spec: `FormatMessages(canonical []Message, systemPrompt string, windowSize int) ([]byte, error)`. Design: `FormatMessages(opts FormatMessagesOpts) ([]byte, error)` where `FormatMessagesOpts` contains 9 fields (see D2). Rationale in D2. TPM should update spec to match.
- **Message type location.** Spec implies it lives somewhere in the provider package (doesn't pin); design puts it in `internal/provider/message.go`.
- **Truncation policy is explicit, with reconciliation.** Spec says "Context window truncation (drop oldest turns, preserve system prompt + recent)." Design adds D4 reconciliation pass for tool-pair orphan handling across multi-assistant-turn interleaving. TPM should update spec to include reconciliation.
- **`provider.*` types (Message, ToolCall, ToolResult, Attachment) are named identically to `protocol.*` analogues and share wire-level byte shape.** WU-042 spec says the canonical Message has fields `(role, content, tool_calls, tool_results, attachments, metadata)` but does not prescribe new types. Design creates explicit `provider.*` types so server-internal metadata (Message.Metadata) has a home. TPM should update spec to reflect this split.
- **`Message.Metadata` reserves specific keys.** `turn_id`, `branch_id`, `sequence`, `timestamp` are documented as the canonical keys. Spec does not call these out.
- **ADR-0006 amended in place, not a new file.** Spec wording ("ADR amendment document: `docs/adr/0006-amendment-001-outbound-formatting.md`") implies a standalone file. Design appends to the existing ADR-0006 file because the repo's ADR schema (`docs/adr/README.md`) does not define an amendment lifecycle or filename convention. TPM should update spec to reflect in-place amendment.

## Pre-Review Lint Disposition (2026-04-16)

Subagent pre-review lint ran against the initial draft. Review artifact: `docs/releases/v0.2.0/.reviews/provider-formatting-042-043-044/claude-subagent-pre-review.md`. Findings and dispositions:

**Blocking (all fixed in-WU):**
- B-01 — `provider.Attachment` missing `Path` field + `Raw` type mismatch → **FIXED.** D1 rewritten: `Attachment` now has all five fields mirroring `protocol.Attachment` exactly (`Path`, `Raw` as base64 string, `Content`, `ContentType`, `Transform`). `convert.go` is field-copy, lossless.
- B-02 — Two conflicting `ErrNotImplemented` strings → **FIXED.** D3 now references D8; D8 has the single canonical definition (`"provider: method not implemented"`).
- B-03 — Pair atomicity rule didn't handle interleaved multi-turn tool calls → **FIXED.** D4 rewritten with explicit 3-stage algorithm: budget check → age-based cutoff → **reconciliation pass** that walks the kept prefix forward and drops orphan results and orphan calls per-id. Multi-call and multi-turn cases called out with specific examples.
- B-04 — D5 wire tables missed wire details → **FIXED.** Both tables now include: Anthropic `tool_use.input` as raw JSON object (not string-wrapped); OpenAI assistant-with-only-tool-calls requires `content: null`; OpenAI user-with-attachments uses array-form content; `ToolResult` tri-state (success/rejected/error) preserved in canonical type and mapped distinctly on both providers; `OutputType == "image"` handled; vision gating added to `FormatMessagesOpts.Capabilities`.
- B-05 — `FormatToolDefinitions` composition contract undocumented → **FIXED.** D2 now states: `FormatMessages` calls `FormatToolDefinitions` internally; the standalone method is exposed for logging / debugging / capability-diff; composition test added to WU-043 and WU-044 test plans.

**Attention (all fixed in-WU):**
- A-01 ADR amendment front-matter invented keys → **FIXED.** D7 changed to amend-in-place on existing ADR-0006 file; no new file, no invented keys.
- A-02 Missing `FormatMessagesOpts` fields → **FIXED.** Added `Capabilities []string` (used for vision gating). Other fields (`StopSequences`, `TopP`, `ToolChoice`, `User`, `Metadata`, `N`) documented as "Deferred opts fields" with owners.
- A-03 Token estimation doesn't define what's counted → **FIXED.** D4 now defines `EstimateMessageTokens(m Message) int` explicitly; `Attachment.Raw` NOT counted; bench test added.
- A-04 Return convention under-specified → **FIXED.** D2 states rationale (ADR-0005 capture fidelity + http.Request.Body pass-through) and test comparison approach (unmarshal both sides to `map[string]any`, not byte-for-byte). WU-043 test plan preamble notes this.
- A-05 Missing deviations in the log → **FIXED.** Added `provider.*` types, Metadata reserved keys, ADR amend-in-place as deviations.
- A-06 `IsError bool` drops tri-state → **FIXED** (with B-04). `provider.ToolResult` now has `Status / Error / Reason` mirroring `protocol.ToolResult`.
- A-07 Missing `ErrTruncationEmpty` / `ErrInvalidToolInput` → **FIXED.** D8 expanded.
- A-08 Partial-turn continuation not considered → **Deferred explicitly.** Risks section notes WU-051 is responsible for producing complete turns; `FormatMessages` assumes complete turns; WU-064 in-flight recovery is the owner of partial-turn handling.
- A-09 OpenAI `max_completion_tokens` vs. `max_tokens` → **FIXED.** D5 OpenAI table states model-family heuristic for field selection.
- A-10 D6 JSON-string quirk edge cases → **FIXED.** D6 rewritten with `json.Valid` guard, error handling, and explicit test cases. `ErrInvalidToolInput` added in D8.
- A-11 Stub test plan underspecified → **FIXED.** WU-042 test list expanded to four named stub tests (Anthropic/OpenAI × FormatMessages/FormatToolDefinitions).

**Nits:**
- N-01 `Message.Role` typed constants → **deferred.** Not fixing; plain-string consistent with protocol.* analogues and low-value change.
- N-02 `message.go` package comment → No action needed (same package as provider.go; no new comment required).
- N-03 D2 precedent list → cosmetic; left as-is.
- N-04 WU-053 streaming continuity note → not in this bundle's scope; streaming relay consumes the same formatted body, no change needed.
- N-05 Track-0 spec commit-point for amendment → TPM-level nit, outside this design's scope.
- N-06 `WindowSize int` unit → documented as tokens in D2.

All dispositions complete. Design ready for Phase 2 opt-in decision.
