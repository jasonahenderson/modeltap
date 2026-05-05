# 2026-05-05 — FEAT-0015..0022 Architect/SRE Review Creation

Generated architect-and-SRE-perspective review findings for the entire
Professional Harness Runtime series (FEAT-0015 umbrella plus seven members)
plus a cross-feature continuity synthesis. Companion to the
`*-review-processing` session logs that captured the user's acceptance pass.

## Discussion

Started with a chat-only architectural Q&A on FEAT-0015: what does the
umbrella runtime contract leave unspecified from architect and SRE lenses?
Identified umbrella-level gaps that downstream FEATs cannot pin without an
upstream commitment: event delivery semantics, cancellation, idempotency,
multi-attach lease, deadline/budget propagation across run families, schema
versioning, observability surface, heartbeat/liveness, durability boundaries,
provider resilience, retention/GC.

User then asked for a written review doc, then expanded to the full series and
a continuity review. Per memory rule, all reviews are markdown-only (no JSON
findings). Reviews were authored independently — each looks at one feature on
its own terms, with cross-references to related findings only where the
specific gap manifests in another feature.

## Findings Created

All under `docs/features/.reviews/`:

- `0015-professional-harness-runtime-architect-sre-findings.md` — 12 findings
  (8 sig, 4 adv). Headline: cross-cutting runtime guarantees not committed —
  events, cancel, idempotency, lease, budget propagation, schema versioning,
  observability, liveness, durability, budget stop-behavior, provider
  resilience, retention.
- `0016-managed-codegen-run-pipeline-architect-sre-findings.md` — 12 findings
  (8 sig, 4 adv). Headline: pipeline reads as a linear sequence but is a
  graph; concurrency between `model_call` and `tool_loop` unmodeled;
  checkpoint as boundary not stage; `interrupt` vs `cancel` vocabulary;
  per-stage cost attribution; `tool_loop` partial-failure semantics.
- `0017-durable-runs-and-background-agents-architect-sre-findings.md` —
  12 findings (8 sig, 4 adv). Headline: state-model self-collision (`blocked`
  is both attachment state and UI grouping); three-way disjunction in resume
  contract; in-flight tool fate at disconnect; queue fairness/notification
  SLA.
- `0018-context-planner-and-project-rules-architect-sre-findings.md` —
  10 findings (6 sig, 4 adv). Headline: rule precedence undefined; plan
  immutability across stages unstated; repo snapshot point unspecified;
  budget overflow chooses among summarize/trim/reject without per-category
  rules.
- `0019-validation-and-repair-loop-architect-sre-findings.md` — 11 findings
  (7 sig, 4 adv). Headline: plan authority split BFF/harness; risk
  classification undefined; "same fix" detection only catches literal loops;
  no cost envelope on repair amplification.
- `0020-patch-evidence-and-run-artifacts-architect-sre-findings.md` —
  12 findings (8 sig, 4 adv). Headline: artifact bundle is a list not a
  schema; `content_unavailable` has no recovery; patch evidence timing
  relative to validation/repair unspecified; threshold features ship
  dormant without defaults.
- `0021-policy-grade-tool-runtime-architect-sre-findings.md` — 12 findings
  (8 sig, 4 adv). Headline: layer precedence deferred; policy version
  semantics undefined; mismatch revoke can't unwind side effects; audit
  lives inside mutable run records.
- `0022-memory-routing-and-workflow-extensions-architect-sre-findings.md` —
  12 findings (8 sig, 4 adv). Headline: scope precedence missing;
  durable/ephemeral separation criteria absent; routing roles overlap with
  FEAT-0016 stages while claiming orthogonality; extension trust model
  undefined.
- `syntheses/0015-0022-architect-sre-continuity.md` — 12 findings (9 sig,
  3 adv). Top-line: each member is internally reasonable but the series
  doesn't yet compose. C3 (event stream) and C2 (identity schema) are the
  foundational unresolved contracts; C1 (vocabulary) is the cheapest win;
  C9 (budget envelope) the most likely production bite. Includes a
  per-finding cross-reference map and a suggested resolution order.

Total: 9 review documents, 93 findings (62 significant, 31 advisory, 0
blocking).

## Cross-Feature Patterns

Themes that recurred across multiple reviews:

- **Schema and identity:** ~10 ID concepts in play across the series with no
  central glossary; schema versioning deferred everywhere.
- **Event stream:** four features depend on a contract that is pinned in none.
- **Permission flow:** three owners (status in FEAT-0015, inbox in FEAT-0017,
  decision in FEAT-0021) without a synchronization story.
- **Retention coordination:** five features have retention knobs with no
  precedence; artifacts can outlive runs or be GC'd before them.
- **Cost/budget envelope:** four features touch budget with no compositional
  rule across siblings, repair loops, and routing-driven model upgrades.
- **Operability:** no member commits to fleet metrics, traces, or
  operator-facing signals — operability surface is UI-only.
- **Vocabulary drift:** `cancel` vs `interrupt`, `blocked` as status grouping
  vs attachment state, `checkpoint` as stage vs boundary.

## Disposition

User processed each review immediately upon delivery. All 93 findings were
accepted across the per-feature reviews and the continuity synthesis;
disposition columns flipped from `null` to `accepted` for every entry.
Acceptance changes to FEAT-0015..0022 are captured in the companion
processing logs:

- `docs/history/2026-05-04-session-feat-0015-architect-sre-review-processing.md`
- `docs/history/2026-05-04-session-feat-0016-architect-sre-review-processing.md`
- `docs/history/2026-05-04-session-feat-0017-0022-architect-sre-review-processing.md`

## Open Items

Following the continuity synthesis's suggested order, post-acceptance work
that should land before Phase 3:

1. Run Runtime ADR — covers C2 (identity schema), C3 (event stream),
   FEAT-0015 A1/A3 (event delivery + idempotency).
2. Project rules / prompt layering ADR — covers FEAT-0018 A1, C5
   (workspace lifecycle ownership consolidation).
3. Validation and repair artifacts ADR — covers FEAT-0019 contracts.
4. Artifact storage and redaction ADR — covers FEAT-0020 schema, retention
   envelope (C6), redaction application timing.
5. Policy and workspace boundaries ADR — covers FEAT-0021 layer precedence
   (A1), version semantics (A2), audit independence (S2).
6. Memory, routing, and extension trust ADR — covers FEAT-0022 trust tiers
   (A4), retention defaults (S1), dataset boundary (S3).

The continuity synthesis's resolution order remains: vocabulary lock first
(cheapest), then identity + event stream foundation, then independent
contracts in parallel.
