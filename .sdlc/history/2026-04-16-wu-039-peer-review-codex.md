# 2026-04-16 — WU-039 Peer Review (Codex)

Performed a retroactive design-first peer review of WU-039, followed by a code check for implementation-specific risk.

Review artifact:
- `.sdlc/releases/v0.2.0/.reviews/wu-039/codex-design-and-code-review.md`

Primary findings:
- High: WU-039 freezes `protocol.Request` with optional `id`, but FEAT-0008 says every harness request includes `id`.
- Medium: tests do not pin that invariant, so the contract drift can persist unnoticed.
- Low: the WU-039 design doc still contains stale “19 request types” wording after `ConnectionReady` brought the catalog to 20.

Recommended next step:
- Resolve the request-vs-notification envelope rule before more Track A / Track B work depends on the current shared request type.
