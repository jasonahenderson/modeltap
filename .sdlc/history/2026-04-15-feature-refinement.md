# 2026-04-15: Feature Spec Refinement and Protocol Contract

## Session Summary

Continuation of the integrated harness work. Focused on refining FEAT-0008 and FEAT-0009 through iterative discussion and multiple peer reviews. Major additions: interactive compaction, session explorer, model transparency, system prompt engine, plan/build mode UX, three-layer model config, connection lifecycle, diagnostics, and the shared protocol contract.

## Key Decisions

1. **ADR-0013 revised**: Bubbletea from day one, no phased approach. Feature requirements (plan/build indicator, session explorer, compaction UI) make a minimal prototype throwaway. OpenCode uses Bubbletea directly for the same patterns.
2. **Interactive compaction**: server categorizes context semantically, user selects per-category actions (summarize/keep/drop/pin) with file-level granularity. Not opaque auto-summarization.
3. **Session explorer on launch**: users see recent sessions with summaries, context usage, cost, and can view details or compact-before-resume.
4. **Model transparency always**: model.selected event before every response. User override with /model, clear with /model auto. /models shows full routing tree.
5. **Seven-layer system prompt engine**: core behavioral (ships with product), tool-use instructions, domain, project, mode, knowledge, session state. Assembled per-turn.
6. **Plan/build mode with Ctrl+P toggle**: plan mode instructs model to propose + harness intercepts writes. Build mode executes normally. Visual indicator always visible.
7. **Three-layer model config**: provider endpoints → model registry (auto-discovery) → hierarchical routing policy with dot-path resolution.
8. **Multi-model parallel review as background threads**: progressive completion with spinners, branch_id event tagging.
9. **Session conflict: reject, not collaborate**: one harness per session. Observer mode deferred to post-FEAT-0010.
10. **All 13 tools required from day one**: no phased tool rollout. MCP required for extensibility.
11. **Shared protocol Go package required**: internal/protocol/ with struct types imported by both server and harness. Created before implementation begins.
12. **FEAT-0008 and FEAT-0009 can be built in parallel** against test doubles, with protocol contract frozen.
13. **JIT user provisioning**: auto-create on first OIDC login. OS keychain for token storage.

## Peer Reviews Processed

- FEAT-0008 connectivity review (4 blocking, 1 significant): connection lifecycle, heartbeat/health, in-flight recovery, diagnostic taxonomy, local bootstrap
- FEAT-0008/0009 interdependency review (4 blocking, 2 significant): connection UX, tool catalog schema, pre-turn summarization, multi-model branching, project context, acceptance crosswalk

## Artifacts Modified

- **ADR-0013**: revised from phased to Bubbletea from day one
- **FEAT-0008**: interactive compaction, session explorer, model transparency, system prompt engine, three-layer model config, connection lifecycle, diagnostic taxonomy, protocol payload schemas, canonical field names, tool catalog schema, interface definition requirement, parallel build strategy
- **FEAT-0009**: connection UX and recovery, plan/build mode with Ctrl+P, session explorer, model commands, full tool set, multi-model branch display, project awareness protocol, parallel build strategy
- **FEAT-0010**: observer mode future epic, JIT provisioning, OS keychain for tokens
- **FEAT-0012**: renamed to Skills only (agent teams split to FEAT-0013 in prior session)

## What's Next

- Implementation planning: work unit breakdown for FEAT-0008 and FEAT-0009
- ADR-0006 amendment (outbound message formatting)
- internal/protocol/ package as first work unit
