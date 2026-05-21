---
patch: "PATCH-0036"
title: "Correlate proxy captures with durable runs"
status: "done"
date: "2026-05-12"
related:
  - "FEAT-0015 (Professional Harness Runtime)"
  - "FEAT-0017 (Durable Runs and Background Agents)"
  - "ADR-0015 (Run Runtime Ownership and Semantics)"
branch: "patch/0036-run-proxy-correlation"
parent: "FEAT-0015"
series: "Professional Harness Runtime"
series-role: "member"
series-order: 6
---

# PATCH-0036: Correlate proxy captures with durable runs

## Problem

Durable runs provide the user-facing task boundary for harness work, while the
existing proxy capture log remains request-centric. Run-native tables can show
run events, checkpoints, model-call accounting, and tool-result accounting, but
raw captured provider requests do not yet carry a first-class `run_id`.

That leaves a gap in history and debugging workflows:

- a user can inspect a run but cannot directly filter raw proxy captures by run
- provider request/response bodies are harder to connect to the run timeline
- dashboard and CLI history views must infer relationships from surrounding
  events instead of querying a stable correlation field
- low-level failures may carry BFF run context in structured logs while the raw
  capture record remains detached from that context

ADR-0015 already defines durable run identity and trace identity. This patch
threads those identifiers into request capture where modeltap knows them.

## Scope

1. **Add nullable request correlation fields.** Extend captured request storage
   with nullable or empty-string `run_id` and `trace_id` fields. Existing
   transparent proxy traffic that is not associated with a durable run remains
   valid and stores empty correlation fields.

2. **Migrate SQLite safely.** Add a schema migration that preserves existing
   request rows and initializes new correlation columns to empty values. Add
   indexes suitable for filtering request history by `run_id` and `trace_id`.

3. **Propagate run correlation from BFF-owned dispatch.** When a provider
   request is dispatched for a known run, pass `run_id` and `trace_id` into the
   capture path so the saved request record can be queried from the run.

4. **Expose request filtering by run.** Add CLI filtering for captured request
   history, for example `modeltap requests list --run <run-id>`, and include
   run/trace fields in detailed request output and export formats where
   applicable.

5. **Preserve metrics behavior.** Existing aggregate usage metrics remain
   provider/model/time scoped. This patch does not add run-level aggregate
   metrics tables unless required for correctness; run totals continue to come
   from run-native accounting.

## Out of Scope

- Creating durable runs for arbitrary transparent proxy clients.
- Inferring sessions or runs from raw proxy traffic that did not originate from
  the BFF run runtime.
- New cross-client task-history UX beyond request filtering and detail/export
  visibility.
- Durable memory promotion, routing decisions, or context curation based on raw
  captures.
- Changing run lifecycle semantics, attachment semantics, or checkpoint schema.

## Checklist

- [x] Storage `Request` model includes `RunID` and `TraceID`
- [x] SQLite migration adds request correlation columns without data loss
- [x] Request list filter supports optional run and trace filtering
- [x] Capture path can receive run/trace correlation metadata
- [x] BFF provider dispatch passes known `run_id` and `trace_id` to capture
- [x] `requests list` supports `--run <run-id>`
- [x] `requests show` includes run/trace fields when present
- [x] `requests export` includes run/trace fields in JSONL and CSV output
- [x] Tests cover migration from prior schema with existing request rows
- [x] Tests cover run-filtered request listing
- [x] Tests cover uncorrelated transparent proxy traffic
- [x] Tests cover BFF-owned provider dispatch saving run-correlated captures
- [x] `docs/patches/README.md` index updated

## Implementation Notes

The preferred storage shape is empty string rather than SQL `NULL` if that
matches the existing request schema conventions. The user-facing behavior should
still be nullable in practice: no run correlation means the field is omitted or
rendered blank.

`run_id` is the user-facing grouping key for task history. `trace_id` is the
lower-level observability key for linking BFF logs, harness messages, provider
dispatch, retries, and captured requests.

This patch should avoid changing usage aggregation semantics unless a future
run-level metrics feature explicitly requires it. The immediate goal is stable
history correlation, not a new analytics model.
