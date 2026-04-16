# v0.2.0 Status

## Last Updated
2026-04-16

## Current Phase
Planning revised after review; implementation not yet started

## Planned
See `plan.md`, `track-0-shared.md`, `track-a-bff-server.md`, `track-b-terminal-harness.md`, `track-integration.md`.

54 work units total: WU-039 through WU-092.

## Completed
(none yet)

## In Progress
(none yet)

## Up Next
WU-039: Protocol types — core messages and framing (Track 0)

## Blocked
(none)

## Notes
- Tracks A and B can run in parallel or be serialized (server-first recommended)
- Track 0 gate for Track A: all of WU-039–WU-045 must complete
- Track 0 gate for Track B (relaxed): harness-local WUs (068-072, 075-079) may start after WU-039; protocol-client and session-aware WUs additionally require WU-040/041
- Integration track (WU-088-090) runs after both tracks complete
- WU-091 and WU-092 were added 2026-04-16 from the plan review to cover cross-session command history (FEAT-0009)

## Plan Review History
- 2026-04-16 — first round of plan reviews processed; see `.reviews/codex-plan-review.md` and `.reviews/kimi-plan-review.md`, and `docs/history/2026-04-16-release-v0.2.0-plan-review-processed.md`
- 2026-04-16 — feature-gaps review processed; see `.reviews/kimi-plan-review-feat-gaps.md` and `docs/history/2026-04-16-release-v0.2.0-feat-gaps-review-processed.md` (4 blocking items already fixed; 3 attention items applied to WU-048, WU-050, WU-061, WU-081)
