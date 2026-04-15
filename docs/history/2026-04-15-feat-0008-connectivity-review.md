# 2026-04-15 — FEAT-0008 connectivity re-review

## Summary

Re-reviewed `docs/features/0008-bff-server.md` specifically for harness-to-BFF/proxy connectivity reliability, self-recovery, and user-facing diagnostics after the feature changed again.

Overwrote the targeted review artifacts:

- `docs/features/.reviews/plan-reviews/0008-bff-server-connectivity-review.md`
- `docs/features/.reviews/plan-reviews/0008-bff-server-connectivity-review.json`

## Findings Summary

- 5 findings total
- 4 blocking
- 1 significant

## Main Issues Raised

- FEAT-0008 is stronger as a BFF/server spec, but still needs a first-class connection lifecycle state machine.
- Heartbeat timeout is mentioned, but heartbeat/readiness/dependency-health protocol primitives are not specified.
- Reconnect behavior still needs idempotent in-flight turn, tool-result, and multi-model stream semantics.
- User-facing connection failures need stable diagnostic codes and concrete remediation commands.
- Local service bootstrap and session unlock behavior should be tied explicitly to CLI/configuration semantics and FEAT-0004/ADR-0012.
