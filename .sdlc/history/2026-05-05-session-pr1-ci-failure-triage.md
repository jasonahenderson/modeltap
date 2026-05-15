# 2026-05-05 — Session: Triage PR #1 CI Failures (Investigate-Only)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

Resumed from `.sdlc/history/2026-05-05-handoff-v0.2.x-test-failure-triage.md`. PR #1 (`spike/scrolling-surface-eval` → `main`) was still open. Per the handoff's "still open" guidance, the user picked **option 3: investigate-only** — reproduce, root-cause, and draft the patch doc, but commit nothing until PR #1 merges.

## What I Did

1. Re-read the handoff and confirmed PR #1 state via `gh pr view 1` — open, not merged.
2. Pulled the latest failed CI log via `gh run view 25399762862 --log-failed` to get exact failure traces (the handoff summary was correct but light on detail).
3. Read the four failing tests and their underlying production code:
   - `internal/bff/turn.go` + `turn_test.go`
   - `internal/harness/connection_test.go` (around `mockBFF`, lines 574-606)
   - `internal/harness/tools/bash.go` + `bash_test.go`
   - `internal/storage/sqlite.go` (`NewSQLiteStore`)
4. Reproduced each failure mentally; ran the suite locally with `-race -count=50` to confirm none reproduce on macOS (as the handoff predicted).
5. Built a minimal off-tree repro for the BFF persistence failure that proved modernc.org/sqlite gives each `:memory:` pool connection its own independent database — c1 saw 1 row, c2 saw 0 rows on the same `*sql.DB`.
6. Drafted `.sdlc/patches/0016-pr1-ci-test-failures-triage.md` covering all four test failures plus a fifth lint regression (`reflect.Ptr` → `reflect.Pointer`) that came from the same CI run. Status `proposed`. Not yet committed.

## Root Causes (one-line each)

1. **`TestHandleTurnSubmit_HappyPath`** — `:memory:` SQLite is per-connection; `NewSQLiteStore` doesn't pin the pool, and the relay goroutine's concurrent `CreateTurn` grows it. Fix: `db.SetMaxOpenConns(1)` when path is `:memory:`.
2. **`internal/harness` SIGSEGV** — `mockBFF.close()` nils `s.ln` while `acceptLoop()` reads it; race + nil deref. Fix: drop the `s.ln = nil` line.
3. **`TestBash_Timeout`** — Linux `dash` forks `sleep` as a child holding the stdout/stderr pipes; SIGKILL on dash leaves sleep alive and `CombinedOutput` blocks on EOF. Fix: `cmd.WaitDelay = 100ms`.
4. **`TestBash_OutputTruncation`** — `printf 'x%.0s' {1..500}` uses bash brace expansion; dash treats `{1..500}` literally and printf prints `x` once. Fix: replace with `awk 'BEGIN{for(i=0;i<500;i++)printf "x"}'`.
5. **lint** — golangci-lint v2.12.x's `inline` analyzer flags deprecated `reflect.Ptr` at `internal/protocol/protocol_test.go:691,694`. Fix: rename to `reflect.Pointer`.

## Files Created (uncommitted)

- `.sdlc/patches/0016-pr1-ci-test-failures-triage.md` — full patch with per-fix sketches and rationale
- `.sdlc/history/2026-05-05-session-pr1-ci-failure-triage.md` — this log

## After PR #1 Merged

User confirmed PR #1 merged. Pulled `main` (via one-shot HTTPS fetch since the
local SSH agent had no key registered for this account; remote config left
untouched), branched `patch/0016-pr1-ci-test-failures`, and landed the work as
seven commits:

1. `PATCH-0016: add patch doc for PR #1 CI test failure triage`
2. `PATCH-0016: pin SQLite pool to a single connection for :memory: databases`
3. `PATCH-0016: remove racy s.ln=nil from harness mockBFF.close`
4. `PATCH-0016: bound BashTool timeout cleanup with cmd.WaitDelay`
5. `PATCH-0016: replace bash brace expansion with portable awk in test`
6. `PATCH-0016: rename reflect.Ptr to reflect.Pointer for govet inline check`
7. `PATCH-0016: update README index, v0.2.0 changelog, patch checklist, session log`

Local validation: `go test -race -count=1 ./...` green across all packages
(macOS arm64). golangci-lint not installed locally — deferred to CI.

## What's Next

- Push `patch/0016-pr1-ci-test-failures` and open a PR against `main`.
- Watch CI on Linux (race + golangci-lint v2.12.x) for green.
- After CI green: flip patch status `approved` → `done` in a follow-up commit
  and update the index/changelog rows accordingly.
