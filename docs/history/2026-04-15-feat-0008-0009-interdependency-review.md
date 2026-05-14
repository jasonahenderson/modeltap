# 2026-04-15 — FEAT-0008 / FEAT-0009 interdependency review

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Summary

Reviewed `docs/features/0008-bff-server.md` and `docs/features/0009-terminal-harness.md` together for harness/BFF interdependency gaps.

Wrote targeted review artifacts:

- `docs/features/.reviews/plan-reviews/0008-0009-harness-bff-interdependency-review.md`
- `docs/features/.reviews/plan-reviews/0008-0009-harness-bff-interdependency-review.json`

## Findings Summary

- 6 findings total
- 4 blocking
- 2 significant

## Main Issues Raised

- FEAT-0009 does not yet describe how the TUI renders or acts on FEAT-0008's connection lifecycle and diagnostic taxonomy.
- Tool catalog registration needs a shared schema across built-in tools, unified `Read`, dynamic permissions, and MCP tools.
- Large-paste summarization needs a BFF-owned pre-turn transform method or a revised ownership model.
- Multi-model streaming needs branch-level protocol events and branch-aware reconnect semantics.
- Project context handoff needs explicit protocol fields and path rules.
- FEAT-0008 success criteria need to cover the server behaviors FEAT-0009 depends on.
