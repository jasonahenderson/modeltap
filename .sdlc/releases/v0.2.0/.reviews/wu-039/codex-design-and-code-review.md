# WU-039 Design Review — Codex

## Findings

### 1. High — Request envelope permits missing `id` even though FEAT-0008 requires every harness request to carry one

**Location:** [.sdlc/features/0008-bff-server.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/features/0008-bff-server.md:54), [.sdlc/history/2026-04-16-design-wu-039-protocol-core.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/history/2026-04-16-design-wu-039-protocol-core.md:41), [internal/protocol/protocol.go](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/internal/protocol/protocol.go:80)

FEAT-0008 says “every request from the harness includes a `id` field.” WU-039 freezes the shared `Request` envelope with `ID json.RawMessage \`json:"id,omitempty"\``, which silently permits request frames with no `id` at all. That is contract drift in the foundational protocol package, not just a transport-layer implementation choice.

This matters because WU-039 is supposed to be the shared source of truth for Track A and Track B. Leaving `id` optional in the only shared request envelope makes it easy for later code to emit a harness request that violates the feature spec while still type-checking and round-tripping cleanly. The later WU-040 note about deciding whether notifications are represented as JSON-RPC notifications or as a separate type makes the ambiguity explicit, but WU-039 has already frozen a request shape that relaxes the spec.

**Recommended fix:** Either make harness-side requests require `id`, or introduce a separate notification/event envelope and reserve missing `id` for server-initiated notifications only. In either case, the WU-039 design artifact and Track 0 contract should state the rule explicitly before more downstream code is built on the permissive envelope.

### 2. Medium — Tests do not pin the “request `id` is required” invariant, so the contract drift can persist unnoticed

**Location:** [internal/protocol/protocol_test.go](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/internal/protocol/protocol_test.go:320), [.sdlc/features/0008-bff-server.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/features/0008-bff-server.md:54)

The envelope test coverage only exercises a happy-path request with an `id` present. There is no test asserting that a harness request without `id` is forbidden by the shared contract, and no negative test around the missing-`id` case. For a protocol-freeze WU, this leaves a spec-required invariant completely unpinned.

**Recommended fix:** Add a contract-level test or conformance check that distinguishes harness requests from notifications and fails if a harness request can be serialized without an `id`.

### 3. Low — The WU-039 design artifact still contains stale “19 request types” wording after `ConnectionReady` was added

**Location:** [.sdlc/history/2026-04-16-design-wu-039-protocol-core.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/history/2026-04-16-design-wu-039-protocol-core.md:12), [.sdlc/history/2026-04-16-design-wu-039-protocol-core.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/history/2026-04-16-design-wu-039-protocol-core.md:26), [.sdlc/history/2026-04-16-design-wu-039-protocol-core.md](/Users/jasonhenderson/Projects/jasonahenderson/modeltap/.sdlc/history/2026-04-16-design-wu-039-protocol-core.md:186)

The retroactive review already corrected the actual type catalog to 20 types, but the top-level scope/package-layout text still says WU-039 delivers 19 harness→server request types. This is not a runtime defect, but it weakens the protocol-freeze audit trail and creates avoidable confusion for later readers.

**Recommended fix:** Update the stale 19-count references so the design artifact is internally consistent.

## Open Questions

- If the intended model is “all harness→server messages are JSON-RPC requests with `id`; server→harness streaming uses notifications,” should WU-040 introduce a dedicated notification envelope instead of relying on `Request` with omitted `id`?
- If the intended model is “missing `id` is allowed anywhere JSON-RPC allows notifications,” FEAT-0008 should be amended because its current wording is stricter than the frozen WU-039 envelope.

## Change Summary

The core message catalog, `ConnectionReady` addition, field tags, and frame-size handling look broadly sound. The main issue is the request-envelope contract: WU-039 currently documents and ships a shared `Request` type that is looser than FEAT-0008 on request correlation.

---

## Disposition (2026-04-16)

Appended by the Designer after triage.

- **Finding 1 (High) — `Request.ID omitempty` permits missing id:** **FIXED in-WU.** Removed `omitempty` from `Request.ID`; every `Request` now emits an `id` key on the wire. Introduced a separate `Notification` envelope (no `id` field, by design) for server→harness streaming events. Harness→server frames MUST use `Request`; this is documented on both types. Nil `ID` still marshals as `"id":null`, which is a legal JSON-RPC value but will be rejected by WU-046 transport validation because it defeats correlation. The type-layer fix eliminates the accidental-notification case; the validation-layer fix is deferred to WU-046 (aligned with pre-review finding A-02's deferral pattern).
- **Finding 2 (Medium) — Tests do not pin the id-required invariant:** **FIXED in-WU.** Added `TestRequest_IDAlwaysEmitted` asserting that marshaled `Request` output always contains the `"id"` key (covering zero-value `ID` and explicit `ID` cases) and `TestNotification_RoundTrip_NoID` asserting that `Notification` never emits an `id` key. Any future refactor that reintroduces `omitempty` on `Request.ID` or elides the field will fail these tests.
- **Finding 3 (Low) — Stale "19 request types" wording:** **FIXED in-WU.** Updated all five residual "19" references in the design doc (scope, layout, section header, type-catalog intro, test plan description) and two in the implementation file headers (`protocol.go` package godoc, `messages.go` file-level comments). Catalog is now consistently "20 harness→server request types" everywhere.

## Related Spec Change

With `Notification` introduced in WU-039, the pre-review lint's A-04 deferred question ("Request with optional id" vs. "dedicated Notification type") is now resolved in favor of the latter. The WU-040 spec in `.sdlc/releases/v0.2.0/track-0-shared.md` has been updated to state that streaming events wrap their payloads in `protocol.Notification`.

All tests pass (`go test -race ./internal/protocol/...`). `go build ./...` green.

Reviewer acknowledgment: the "High" finding was a real contract-drift catch that the same-model pre-review missed. This is exactly the kind of cross-model value Tier-C peer review exists to provide.
