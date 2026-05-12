# 2026-05-08 - Implementation Review Guidelines

## Summary

Added canonical implementation-review guidance for post-implementation review
and release-readiness evidence.

## Changes

- Created `.agents/reviews/implementation-review.md`.
- Linked the guideline from `.agents/process.md`.
- Added a short human-readable pointer in `docs/agents.md`.

## Notes

The guideline distinguishes static conformance review from runtime evidence
review. Runtime evidence may come from automated E2E tests, smoke tests,
manual launch checks, logs, probes, or CI jobs, but static review alone should
not be treated as operational release validation.

## Validation

Documentation-only process change; no code tests run.
