# WU-039 Peer-Review Prompt

**Target reviewer:** any non-Claude model (Codex / GPT-5 / Kimi K2 / Gemini / Llama-3.1-70B local / etc.).
**Scope:** retroactive Tier-C design review of WU-039 (Protocol Core Messages and Framing).
**What's already done:** implementation is merged; a same-model subagent lint has run. See §"What the pre-review already covered" below.
**Output destination:** commit the review at `docs/releases/v0.2.0/.reviews/wu-039/<reviewer>-design-review.md` — use reviewer-first naming (e.g., `codex-design-review.md`, `gpt5-design-review.md`, `kimi-design-review.md`, `gemini-design-review.md`).

---

## How to run this review

1. Read §"Prompt" below — it is the message to send to the external model. It is self-contained; the model needs no other context from this file.
2. Paste it into your chosen model's interface (web UI, API, CLI, whichever).
3. The prompt instructs the model to read specific files. Either:
   - Attach the files directly if the reviewing model supports file upload, OR
   - Copy the file contents inline, OR
   - For web-UI models without file access, give them the prompt first, then paste each referenced file when asked.
4. Save the output as `docs/releases/v0.2.0/.reviews/wu-039/<reviewer>-design-review.md`.
5. Let me know the filename and I'll triage the findings.

---

## Files the reviewer needs

| File | Purpose |
|------|---------|
| `docs/history/2026-04-16-design-wu-039-protocol-core.md` | The design doc under review |
| `internal/protocol/protocol.go` | Implementation: envelope, framing, Mode, constants |
| `internal/protocol/messages.go` | Implementation: 20 request types + shared nested types |
| `internal/protocol/protocol_test.go` | Test coverage |
| `docs/features/0008-bff-server.md` | Source of truth for the protocol (relevant sections: "Protocol Specification", "Protocol Messages", "Protocol Payload Schemas", "Canonical Field Names", "Tool Catalog Schema") |
| `docs/releases/v0.2.0/track-0-shared.md` | WU-039 spec |
| `docs/releases/v0.2.0/.reviews/wu-039/claude-subagent-pre-review.md` | Pre-review lint artifact — findings already identified and dispositioned |

---

## What the pre-review already covered

The subagent lint found 1 Blocking + 7 Attention + 3 Nit. All resolved or dispositioned:

- **Fixed in-WU:** B-01 (missing `ConnectionReady` type), A-03 (`ToolResultRequest` alias freeze-contract godoc), A-06 (nested-type required-field docs), A-07 (package godoc cross-references), N-03 (design-doc copy edit).
- **Deferred to downstream WUs:** A-01 (`Sequence` required-vs-absent → WU-046), A-02 (unknown `Mode` round-trips → WU-046), A-04 (JSON-RPC notification semantics → WU-040), A-05 (`MaxFrameSize` not ratified by FEAT-0008 → WU-049 + spec amendment).
- **Noted, no action:** N-01 (constant ordering), N-02 (byte-at-a-time read performance — defer to WU-095 benchmarking).

The peer reviewer should NOT spend time rediscovering these. They are valid perspective points only if the reviewer has a materially different angle or disagrees with the disposition.

---

## What Tier-C peer review specifically targets

Tier C exists to catch **reasoning blind spots that a same-model reviewer cannot detect**. Focus areas for this review:

1. **Go idiom review.** Package layout, import grouping, error-handling style, naming conventions, godoc quality, use of `encoding/json` / `bufio` / `io` idioms. A Go-specialist reviewer or a model with heavy Go training exposure should spot things Claude may have over-trusted.

2. **JSON-RPC 2.0 conformance.** Check the `Request` / `Response` / `ErrorObject` envelope types against the JSON-RPC 2.0 specification (https://www.jsonrpc.org/specification) line-by-line. Does our envelope correctly handle:
   - The `id` member semantics (string, number, null, absent-for-notification)?
   - The error code ranges and reserved codes?
   - Batch requests (do we handle them? if not, is that a documented limitation)?
   - Any edge cases around notification vs. request?

3. **NDJSON framing patterns.** Compare our `FrameReader` / `FrameWriter` / `MaxFrameSize` design against how other production protocols handle newline-delimited JSON:
   - Language Server Protocol (LSP) uses `Content-Length` headers, not pure newline framing. Why did we diverge? Was that the right call?
   - gRPC uses HTTP/2 framing. Not applicable, but are there relevant lessons?
   - Is `ReadByte`-loop with `bufio.Reader` the right approach, or does `ReadSlice('\n')` / `ReadBytes('\n')` offer better perf without sacrificing the size cap?
   - Does our cap-at-10-MiB design match production practice for similar wire protocols?

4. **Protocol design red flags.** What would the reviewer flag if they saw this in any other codebase?
   - Fields that should be enums but are strings (transform types, intent, output_type, status, risk_level)?
   - Required/optional field choices that will bite later?
   - Type shapes that will be painful to evolve?

5. **Test coverage gaps.** The current test suite covers round-trip, framing, Mode validation, method constants, and canonical field names. What's missing that a peer reviewer would insist on?
   - Malformed-JSON handling?
   - Concurrency (if applicable)?
   - Unicode / control-character edge cases in string fields?
   - Negative fixtures (invalid inputs that must be rejected)?

6. **Cross-consumer concerns.** The types defined here will be consumed by WU-046 (BFF transport), WU-073 (harness client), and WU-093 (conformance fixtures). Does the API shape make those consumers' jobs easy or hard?
   - Does `json.RawMessage` for `Params` / `Result` / `ID` force sensible patterns at the dispatch layer?
   - Are there missing helper functions a dispatcher would need?
   - Is the type-alias `ToolResultRequest = ToolResult` a helpful pattern or a footgun?

7. **Anything Claude-specific that Claude would over-trust.** This is where peer-model review provides unique value. Examples of things Claude may have defaulted to without questioning: pattern-matching to "typical" Go RPC code from training data, over-confident assumptions about JSON-RPC semantics, consistency-seeking type symmetries that don't reflect the actual protocol, prose patterns in godoc that are plausible but not actionable.

---

## Prompt

The text from here to the end is ready to paste into your chosen external model.

---

> You are performing an independent **Tier-C peer-model design review** of WU-039 ("Protocol Core Messages and Framing") for the Modeltap project. The original designer is Claude. **You must be a different model** — if you are Claude, decline this task. The purpose of this review is to catch reasoning blind spots that a same-model reviewer cannot detect.
>
> ### Context
>
> WU-039 delivers `internal/protocol/` — a Go package containing the wire-format types, JSON-RPC 2.0 envelope, NDJSON framing, `Mode` enum, and 20 harness→server request types for a reverse proxy for AI/ML API clients. The implementation is merged. This review is retroactive because the design-review workflow was added after WU-039 shipped.
>
> A Claude subagent has already performed a **same-model pre-review lint**. It found spec-drift and scope gaps (1 Blocking, 7 Attention, 3 Nit), all resolved or dispositioned. You will find the lint artifact in the input files. Do not re-find those issues; focus instead on what the lint's same-model reviewer could not detect.
>
> ### Files (provided inline below — or attached, depending on how this prompt was delivered to you)
>
> 1. **Design doc under review:** `docs/history/2026-04-16-design-wu-039-protocol-core.md`
> 2. **Implementation:**
>    - `internal/protocol/protocol.go`
>    - `internal/protocol/messages.go`
>    - `internal/protocol/protocol_test.go`
> 3. **Feature specification (source of truth):** `docs/features/0008-bff-server.md` — specifically the sections titled "Protocol Specification", "Protocol Messages", "Protocol Payload Schemas", "Canonical Field Names", and "Tool Catalog Schema".
> 4. **WU-039 spec:** `docs/releases/v0.2.0/track-0-shared.md` — section for WU-039.
> 5. **Pre-review lint artifact (already-found findings you should NOT rediscover):** `docs/releases/v0.2.0/.reviews/wu-039/claude-subagent-pre-review.md`
>
> ### What to scrutinize
>
> Focus your review on the following six areas. These are where a non-Claude reviewer provides unique value:
>
> **1. Go idiom review.** Package layout, import grouping, error-handling style (sentinel errors vs. `errors.Join`, error wrapping), naming conventions, godoc quality, use of `encoding/json` / `bufio` / `io` idioms. Flag anything a Go-specialist would note.
>
> **2. JSON-RPC 2.0 conformance.** Check the `Request` / `Response` / `ErrorObject` envelope types against the JSON-RPC 2.0 specification (https://www.jsonrpc.org/specification). Specifically:
> - `id` handling: string, number, null, absent-for-notification — all correctly representable?
> - Error code ranges: do we reserve the right spaces for pre-defined errors?
> - Batch requests: does our design support or explicitly reject them? Is that documented?
> - Any other conformance issues with the spec?
>
> **3. NDJSON framing patterns.** Compare the `FrameReader` / `FrameWriter` / `MaxFrameSize = 10 MiB` design against how production protocols handle newline-delimited JSON. Specifically:
> - LSP uses `Content-Length` headers — why did we choose newline framing? Trade-offs?
> - Is `ReadByte`-loop correct, or does `bufio.Reader.ReadSlice('\n')` / `ReadBytes('\n')` offer better performance without sacrificing the size cap?
> - Is 10 MiB the right cap? How do similar wire protocols in your training data size their limits?
>
> **4. Protocol design red flags.** What would you flag seeing this code in any other codebase?
> - Fields typed as `string` that should be enums (transform types, intent, output_type, status, risk_level, namespace).
> - Required/optional field shape choices that will cause pain later (e.g., `Sequence int` is required but zero-value is indistinguishable from omitted — this is already deferred to WU-046, but are there others?).
> - Type shapes that will be painful to evolve across protocol versions.
>
> **5. Test coverage gaps.** Examine `protocol_test.go`. What would you add?
> - Malformed JSON handling?
> - Unicode / control-character handling in string fields?
> - Negative fixtures (invalid inputs that must be rejected by the transport layer)?
> - Concurrent use of `FrameReader` / `FrameWriter` from multiple goroutines (are they safe? is that documented)?
>
> **6. Cross-consumer concerns.** These types will be consumed by:
> - WU-046 (BFF transport / dispatch on the server side)
> - WU-073 (harness JSON-RPC client)
> - WU-093 (cross-track conformance fixtures)
>
> Does the API shape make those consumers' jobs easier or harder? Does `json.RawMessage` for `Params` / `Result` force sensible dispatch patterns? Are there missing helpers those consumers will have to reinvent? Is `type ToolResultRequest = ToolResult` a helpful pattern or a footgun?
>
> ### What NOT to scrutinize
>
> The pre-review already covered these. Do not rediscover them unless you have a materially different perspective:
>
> - `ConnectionReady` was missing; now added.
> - `ToolResultRequest = ToolResult` alias needed freeze-contract docs; now added.
> - `Attachment` / `Paste` / `ToolResult` required fields not documented; now documented.
> - Package godoc lacked forward references; now added.
> - `Sequence` required-vs-absent ambiguity; deferred to WU-046.
> - Unknown `Mode` values round-trip silently; deferred to WU-046.
> - JSON-RPC notification semantics; deferred to WU-040.
> - `MaxFrameSize` not ratified by FEAT-0008; deferred to WU-049 + spec amendment.
>
> If you want to opine on any of these (agree, disagree with disposition, propose alternative), do so under "Perspective on already-identified findings" in your output. Otherwise skip them.
>
> ### Output format
>
> Produce a Markdown document with this structure. Do not wrap it in additional formatting.
>
> ```
> # WU-039 Design Review — <Your Model Name>
>
> **Reviewer:** <model name and any relevant version info>
> **Date:** <YYYY-MM-DD>
> **Review tier:** C (peer-model)
>
> ## Summary
>
> <one-paragraph overall verdict>
>
> ## Blocking findings
>
> (items that must be resolved before downstream WUs consume this package)
>
> ### B-01. <title>
> - What:
> - Evidence: <file:line or quote>
> - Why blocking:
> - Suggested fix:
>
> ## Attention findings
>
> (items that should be addressed unless there is documented reason not to)
>
> ### A-01. <title>
> - What:
> - Evidence:
> - Recommended disposition: fix now | defer to WU-NNN | defer to a patch
>
> ## Nit findings
>
> ### N-01. <title>
> - Description:
>
> ## Perspective on already-identified findings
>
> (optional — any disagreement with or addition to the pre-review's dispositions)
>
> ## What I did NOT review
>
> (honest disclosure of things outside your expertise, time budget, or access)
>
> ## Areas where you believe a Claude reviewer would have blind spots
>
> (this is the specific value of peer-model review — your perspective on what Claude's training likely over-trusts or misses; bullet list)
> ```
>
> ### Norms
>
> - Be specific. Cite file names, line numbers, or direct quotes. "The code is sloppy" is not actionable; "protocol.go:142 — bufio.Reader.ReadByte in a loop for 10 MiB frames is ~10M function calls; ReadSlice('\n') would be substantially faster" is actionable.
> - Don't pad. If you find nothing Blocking, say "None." Do not invent issues to appear thorough.
> - Disagree with Claude where you see fit. This review's value depends on your independent judgment.
> - If you lack a file, say so in "What I did NOT review" and skip the dependent sections rather than guess.
