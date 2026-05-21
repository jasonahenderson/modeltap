# WU-095 — v0.2.0 Performance Baseline

**Date:** 2026-04-19
**Hardware:** Apple M5 Max, macOS 25.4.0
**Go:** 1.25.6
**Notes:** Single-iteration numbers below use `-benchtime=1x`; percentile numbers use `n=50` samples (25 for turn.submit). Rerun for stable means with `-count=5 -benchtime=3s` on CI hardware.

## Methodology

**Part A — micro-benchmarks** live with the code they exercise:
- `internal/harness/tools/benchmarks_test.go`
- `internal/protocol/benchmarks_test.go`
- `internal/storage/benchmarks_test.go`

Run:
```
go test ./internal/harness/tools/ ./internal/protocol/ ./internal/storage/ \
    -bench=. -benchmem -run=^$ -count=5
```

**Part B — end-to-end latency** lives at `internal/integration/latency_test.go`. These are regular `go test` tests that sample N iterations of a live BFF↔harness round-trip and report p50 / p95 / p99.

Run:
```
go test ./internal/integration/ -run LatencyE2E -v -count=3
```

## Baseline numbers

### A. Micro-benchmarks

| Benchmark | ns/op | B/op | allocs/op | Notes |
|-----------|-------|------|-----------|-------|
| `Grep_FilesWithMatches` (200f × 50L) | 1,389,542 | 1,934,328 | 2,059 | Full walk + match |
| `Grep_ContentMode` (100f × 50L) | 1,481,000 | 2,018,832 | 2,538 | Same walk, different output format |
| `Glob_DoubleStar` (500f) | 836,333 | 109,616 | 814 | doublestar/v4 |
| `ReadText_SmallFile` | 114,208 | 39,544 | 353 | 200-line text |
| `ReadText_LargeFile` | 2,000,084 | 5,987,800 | 39,910 | ~512 KiB text |
| `PermissionCheck_ReadOnly` | 500 | 0 | 0 | Zero-alloc hot path — good |
| `DangerousPatternMatch` | 8,583 | 39,736 | 5 | Regex catalog walk |
| `FrameReader_1MBFrame` | 4,991,667 | 3,211,360 | 19 | **210 MB/s** — WU-094 P-6 flagged this as slower than a scanner; baseline for future improvement |
| `FrameReader_SmallFrames` (1000×) | 253,375 | 193,568 | 2,002 | **205 MB/s** |
| `FrameWriter_1KBFrame` | 8,458 | 3,136 | 4 | **121 MB/s** |
| `CreateTurn` | 49,292 | 2,336 | 57 | SQLite insert incl. JSON marshal |
| `ListTurns` (200 rows) | 622,000 | 446,016 | 10,341 | WU-094 H-8 flagged this as needing a default limit |
| `AppendCommandHistory` | 32,042 | 440 | 13 | Cheap, hot path on every turn |

### B. End-to-end latency

| Operation | p50 | p95 | p99 | min | max | n |
|-----------|-----|-----|-----|-----|-----|---|
| `dial + handshake` | 86 µs | 290 µs | 653 µs | 60 µs | 653 µs | 50 |
| `connection.ping` | 17 µs | 25 µs | 33 µs | 12 µs | 33 µs | 50 |
| `session.list` | 62 µs | 114 µs | 275 µs | 56 µs | 275 µs | 50 |
| `turn.submit → first token` | 2.71 ms | 2.92 ms | 3.30 ms | 0.21 ms | 3.30 ms | 25 |

`turn.submit → first token` includes: harness `SubmitTurn` RPC → BFF `handleTurnSubmit` → session create / resume → `TurnDispatcher` HTTP to mock upstream → `SSEParser.Next()` → `StreamRelay` decode → `token.delta` notification → harness `ConnectionManager.HandleEvent` → `StreamTokenMsg` delivered to sender. The 2–3 ms floor is dominated by the httptest mock upstream response time + the BFF streaming relay; the harness-side framing is in the tens of microseconds (see `connection.ping`).

## Observations worth tracking

1. **FrameReader 210 MB/s** — WU-094 P-6 flagged the byte-by-byte `ReadByte` path as ~10× slower than a scanner. The number here (210 MB/s, 3.2 MB allocated per 1 MB frame = ~3× overhead) is the "before" baseline. If someone rewrites to `bufio.Scanner` we'd expect ~1 GB/s and ~1 MB/MB. Leaving as-is for v0.2.0 — not a user-felt bottleneck at typical frame sizes.

2. **ListTurns allocs** — 10,341 allocs for 200 rows is ~50 allocs per row. JSON unmarshal of each row (`Content`, `ToolCalls`, `FilesTouched`, `FilesModified`, `OriginalTurns`) is the bulk. Combined with WU-094 H-8's missing default limit, a long session pulls tens of thousands of rows × 50 allocs = hundreds of thousands of allocs per list call. Default-limit fix lands with H-8.

3. **ReadText_LargeFile ~40k allocs** — the line-numbering output builds a `strings.Builder` that grows as it fills. Reasonable for a 512 KiB file but scales linearly. Not a user-facing regression target — Read is invoked deliberately, not in a tight loop.

4. **connection.ping 17 µs p50** — the framing layer is cheap. Anything that shows up slower than this in production is not framing's fault.

5. **turn.submit p99 under 4 ms on mock upstream** — the BFF streaming relay is not the limiting factor. Real provider round-trips land in the 500 ms – 5 s range per token stream. This baseline is what "infinitely fast provider" looks like.

## Regression budgets

No budgets asserted in test code yet — these are baselines, not gates. Reasonable starting budgets (to revisit after field signal):

- Micro-benchmarks: no regression worse than **1.5×** the baseline ns/op without explicit justification.
- `connection.ping` p99 < **100 µs** (current: 33 µs).
- `session.list` p99 < **1 ms** (current: 275 µs).
- `turn.submit → first token` p99 (mock upstream) < **10 ms** (current: 3.3 ms).

Real provider latency is out of our control and not budgeted here.

## Hot paths explicitly not yet benchmarked

Worth adding in a follow-up:
- MCP `tools/call` round-trip latency (needs a fake MCP subprocess)
- Compaction plan build + apply on 1000-turn session
- Context.list with realistic file set + knowledge injections
- Provider adapter `ParseStreamEvent` on adversarial SSE (pairs with WU-094 H-4 bound)
- Concurrent turn dispatch (N parallel harness clients on one BFF)

## Scripts

A simple shell wrapper for recording a full run:

```bash
#!/bin/sh
# record baseline — pipe output to .sdlc/releases/v0.2.0/.reviews/wu-095-perf/
go test ./internal/harness/tools/ ./internal/protocol/ ./internal/storage/ \
    -bench=. -benchmem -run=^$ -count=5 > bench-a.txt
go test ./internal/integration/ -run LatencyE2E -v -count=3 > bench-b.txt
```
