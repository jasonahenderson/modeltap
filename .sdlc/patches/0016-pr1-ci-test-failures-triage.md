---
patch: "PATCH-0016"
title: "Fix v0.2.x test suite failures and lint regressions surfaced by PR #1 CI"
status: "approved"
date: "2026-05-05"
related:
  - "FEAT-0008 (BFF server)"
  - "FEAT-0015 (Harness)"
branch: "patch/0016-pr1-ci-test-failures"
---

# PATCH-0016: Fix v0.2.x test suite failures and lint regressions surfaced by PR #1 CI

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

PR #1 (`spike/scrolling-surface-eval` → `main`, integrating v0.2.0 + v0.2.1 + v0.2.2 + v0.3.x design lockdown) was the first push of the spike branch. CI ran for the first time on Linux ubuntu-latest with `go test -race ./...` and `golangci-lint v2.12.1`, surfacing four test failures and two lint findings. None are caused by the lockdown commits (which only touch `docs/` and `.github/workflows/ci.yml`); they are pre-existing platform/toolchain-sensitive bugs that never had a chance to surface because:

- The branch had never been pushed before, so CI had never run against it.
- All four tests pass on the maintainer's M-series macOS workstation, including under `-race`.
- The `inline` `govet` analyzer that flags the lint failures became active in golangci-lint `v2.12.x`, which the new CI workflow pins to `latest`.

CI run with the failures: https://github.com/jasonahenderson/modeltap/actions/runs/25399762862

| # | Test / Check | File | Symptom |
|---|---|---|---|
| 1 | `TestHandleTurnSubmit_HappyPath` | `internal/bff/turn_test.go:102` | `expected at least 1 persisted turn, got 0` |
| 2 | `internal/harness` panic | `internal/harness/connection_test.go:600` | `SIGSEGV: nil pointer dereference` under `-race`, plus a flagged data race on `mockBFF.ln` |
| 3 | `TestBash_Timeout` | `internal/harness/tools/bash_test.go:107` | `timeout did not kill the process quickly: 10.002...s` |
| 4 | `TestBash_OutputTruncation` | `internal/harness/tools/bash_test.go:125` | `output should announce truncation: "x"` |
| 5 | govet `inline` | `internal/protocol/protocol_test.go:691,694` | `Constant reflect.Ptr should be inlined` |

## Root Causes

Each failure has an independent root cause, all confirmed by reproduction or by reading the failing CI log.

### (1) BFF persistence — `:memory:` SQLite is per-connection without a shared cache

`internal/bff/session_test.go:18` constructs a real-store BFF via `storage.NewSQLiteStore(":memory:")`. `NewSQLiteStore` (`internal/storage/sqlite.go:25`) opens the database with the default `database/sql` connection pool (no `SetMaxOpenConns`, no shared-cache DSN). modernc.org/sqlite treats each pool connection's `:memory:` as an independent database — the codebase already documents this in `upgrade_test.go:574` ("Use a shared-cache in-memory DB so all pool connections see the same data") but the fix never propagated to `NewSQLiteStore`.

`handleTurnSubmit` calls `srv.store.CreateTurn` (user turn) on the main goroutine, then spawns a relay goroutine that calls `srv.store.CreateTurn` (assistant turn) at `internal/bff/streaming.go:312`. When these two calls hit the pool simultaneously, the pool grows to 2 connections and only the first one ever ran `migrate()` — the second is an empty database. By the time the test calls `ListTurns`, it can land on the empty connection and observe zero rows.

**Reproduction (minimal, outside the project tree, modernc.org/sqlite v1.50.0):**

```go
db, _ := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
_, _ = db.Exec("CREATE TABLE t (x INTEGER)")
_, _ = db.Exec("INSERT INTO t VALUES (1)")
c1, _ := db.Conn(ctx)
c2, _ := db.Conn(ctx)
// c1 sees 1 row, c2 sees 0 rows ("no such table: t" then 0 from a retry)
```

macOS happens to win the contention almost always (the relay goroutine's CreateTurn lands on the same conn as the main goroutine before the pool grows), so the bug never surfaced locally. Linux + race detector slows scheduling enough that the race lands the other way.

### (2) `mockBFF` listener race + nil deref under `-race`

`internal/harness/connection_test.go:574-606` defines a test-only `mockBFF` whose `close()` method nils out `s.ln`:

```go
func (s *mockBFF) close() {
    if s.ln != nil {
        _ = s.ln.Close()
        s.ln = nil       // <-- racy write
    }
}

func (s *mockBFF) acceptLoop() {
    for {
        conn, err := s.ln.Accept()   // <-- racy read; nil deref after close()
        if err != nil {
            return
        }
        go s.handle(conn)
    }
}
```

When `t.Cleanup(srv.close)` fires, the close goroutine runs concurrently with the still-spinning `acceptLoop` goroutine. The race detector flags the unsynchronized `s.ln` access (CI log shows `WARNING: DATA RACE` between `acceptLoop()` at line 600 and `close()` at line 594), and on Linux the loop's next iteration dereferences nil and SIGSEGVs at `pc=0x9567a8 addr=0x18`. Closing the listener alone is sufficient to unblock `Accept` with `net.ErrClosed`; the `s.ln = nil` line is the bug.

### (3) `TestBash_Timeout` — orphaned child holds output pipes after `sh` is killed

`internal/harness/tools/bash.go:86` runs `exec.CommandContext(execCtx, "sh", "-c", *in.Command)` and reads via `cmd.CombinedOutput()`. When the test passes `sleep 10` with a 100ms timeout:

- macOS: `/bin/sh` (bash) `exec`s into the simple final command, so the kernel-level process *is* `sleep`. SIGKILL hits sleep directly, the pipes close, `CombinedOutput` returns immediately.
- Linux ubuntu-latest: `/bin/sh` is `dash`. dash's behavior for a single trailing simple command varies, and on the CI runner sleep is forked as a separate child of dash. SIGKILL kills dash; sleep is reparented to PID 1 with the stdout/stderr pipes still open. `CombinedOutput`'s I/O reader goroutines wait for EOF, which only arrives when sleep exits naturally (10 seconds later).

This is the canonical Go `os/exec` pipe-orphan issue. The supported fix as of Go 1.20 is `cmd.WaitDelay`, which forces the I/O reader goroutines to give up at a deadline. Process-group + group kill (`Setpgid: true`, `Kill(-pid, SIGKILL)`) is the older approach.

### (4) `TestBash_OutputTruncation` — bash brace expansion on dash

`bash_test.go:117` passes the command:

```bash
printf 'x%.0s' {1..500}
```

Brace expansion (`{1..500}`) is a bash/zsh feature, not POSIX. Ubuntu's `/bin/sh` is `dash`, which treats `{1..500}` as a literal argument. `printf 'x%.0s' '{1..500}'` therefore evaluates to one format-arg cycle that prints `x` once. The 50-byte truncation cap is never reached and the test asserts on a 1-byte output containing no truncation marker.

### (5) `govet` `inline` — `reflect.Ptr` is a deprecated alias

golangci-lint `v2.12.x` enables a `govet` analyzer named `inline`. `reflect.Ptr` is a typed constant identical to `reflect.Pointer`; the Go standard library deprecated `reflect.Ptr` in favor of `reflect.Pointer` (Go 1.18+). The analyzer requires that the deprecated form be replaced. Two call sites at `internal/protocol/protocol_test.go:691` and `:694` need the rename.

## Scope

One fix per failure, one commit per fix. All commits prefixed `PATCH-0016:` and DCO-signed.

1. **Pin the SQLite connection pool to a single connection when the path is `:memory:`.** Add `db.SetMaxOpenConns(1)` in `NewSQLiteStore` immediately after `sql.Open` when `dbPath == ":memory:"`. Production never opens an in-memory database, so this only affects tests; serializing access in tests is acceptable and keeps every store instance fully isolated. (An alternative — switch to `file:<unique>?mode=memory&cache=shared` — is more invasive and requires generating a unique URI per Store.)
2. **Remove `s.ln = nil` from `mockBFF.close()` and rely on `ln.Close()` to unblock `Accept`.** This eliminates the race entirely; no nil deref, no `ln`-field synchronization needed because the field is set once at construction and never reassigned.
3. **Set `cmd.WaitDelay = 100 * time.Millisecond` in `BashTool.Execute`.** Go's `os/exec` uses WaitDelay (Go 1.20+) to time-bound the I/O reader goroutines after the process is killed; orphaned children no longer hold the runner past the configured delay. Pair with `cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }` (optional, polite) followed by the SIGKILL that `CommandContext` already issues. WaitDelay alone is sufficient to fix the test; the Cancel hook is a quality nicety.
4. **Replace `printf 'x%.0s' {1..500}` with a POSIX-portable command** in `TestBash_OutputTruncation`. Recommended replacement: `awk 'BEGIN{for(i=0;i<500;i++)printf "x"}'`. Alternatives that also work: `head -c 500 /dev/zero | tr '\0' x`, `yes x | tr -d '\n' | head -c 500`. Pick the awk form for readability.
5. **Rename `reflect.Ptr` → `reflect.Pointer`** at `internal/protocol/protocol_test.go:691` and `:694`.

## Out of Scope

- No new ADRs or feature changes; this is purely test-suite and lint hygiene.
- No change to BFF connection handling, relay semantics, or session lifecycle.
- No upgrade of modernc.org/sqlite or any other dependency.
- No retroactive auditing of every other `:memory:` test in the repo. The fix in `NewSQLiteStore` automatically covers any future caller; existing callers already work because they don't hit the concurrency pattern that flushes out the per-connection bug.
- No CI workflow change to pin golangci-lint to a specific version. The lint fix removes the offending pattern; future analyzer additions can be addressed when they appear.
- No introduction of process groups for `BashTool`. WaitDelay achieves the same observable behavior with substantially less platform-specific code; if a future caller needs to reliably kill the child *tree* (not just stop reading from it), that's a separate scope.

## Checklist

- [x] `internal/storage/sqlite.go`: `NewSQLiteStore` calls `db.SetMaxOpenConns(1)` when `dbPath == ":memory:"`, with a one-line comment explaining why
- [x] `internal/harness/connection_test.go`: drop `s.ln = nil` from `mockBFF.close()`
- [x] `internal/harness/tools/bash.go`: `cmd.WaitDelay = 100 * time.Millisecond` (or named constant) before `cmd.CombinedOutput()`
- [x] `internal/harness/tools/bash_test.go:117`: replace bash brace expansion with portable equivalent
- [x] `internal/protocol/protocol_test.go:691,694`: `reflect.Ptr` → `reflect.Pointer`
- [x] `go test -race ./...` passes locally on macOS
- [ ] `golangci-lint run ./...` passes locally (with `latest` matching CI) — golangci-lint not installed locally; deferred to CI
- [ ] CI passes on the patch branch (Linux + race + golangci-lint)
- [x] One commit per fix, all prefixed `PATCH-0016:`, all DCO-signed
- [x] `.sdlc/patches/README.md` index updated
- [x] `.sdlc/releases/v0.2.0/changelog.md` entry added

## Fix Detail

### Per-fix code sketches

**(1) `internal/storage/sqlite.go` (after `sql.Open`)**

```go
db, err := sql.Open("sqlite", dsn)
if err != nil {
    return nil, fmt.Errorf("opening database: %w", err)
}
// :memory: gives each pool connection its own independent database, so
// schema and data written via one connection are invisible to the next.
// Pin the pool to a single connection when in-memory; production paths
// are file-backed and unaffected.
if dbPath == ":memory:" {
    db.SetMaxOpenConns(1)
}
```

**(2) `internal/harness/connection_test.go`**

```go
func (s *mockBFF) close() {
    if s.ln != nil {
        _ = s.ln.Close()
        // Do NOT set s.ln = nil — acceptLoop reads s.ln in another
        // goroutine. Close() is sufficient to unblock Accept with
        // net.ErrClosed, and the field is never reassigned, so leaving
        // it pointing at the closed listener is race-free.
    }
}
```

**(3) `internal/harness/tools/bash.go`**

```go
cmd := exec.CommandContext(execCtx, "sh", "-c", *in.Command)
cmd.Dir = b.projectRoot
// On Linux, sh may fork the simple command rather than exec'ing into
// it; SIGKILL on sh leaves the child holding the stdout/stderr pipes
// and CombinedOutput blocks on EOF. WaitDelay forces the I/O reader
// goroutines to give up after the deadline so timeout actually returns
// promptly.
cmd.WaitDelay = 100 * time.Millisecond
output, runErr := cmd.CombinedOutput()
```

**(4) `internal/harness/tools/bash_test.go`**

```go
res, err := b.Execute(context.Background(),
    bashInput(t, `awk 'BEGIN{for(i=0;i<500;i++)printf "x"}'`, 0))
```

**(5) `internal/protocol/protocol_test.go`**

```go
if inVal.Kind() == reflect.Pointer {
    inVal = inVal.Elem()
}
if outVal.Kind() == reflect.Pointer {
    outVal = outVal.Elem()
}
```

### Why one patch, not five

The five fixes share a triggering event (PR #1 turning on first-time CI on Linux + race + golangci-lint v2.12.x) and a single owner (the maintainer working through the failure list in one sitting). Bundling them under a single patch doc keeps the rationale together for the reviewer, and the per-commit `PATCH-0016:` prefix preserves the one-fix-per-commit discipline that `CLAUDE.md` requires.

### Why no new tests

Each existing failure *is* the regression test. After the fix, running `go test -race ./...` on Linux (or via CI on the patch branch) is the verification. Adding new tests would duplicate coverage. The patch checklist requires CI to pass before flipping status to `done`.

### Notes for the implementer

- PR #1 is still open at the time this patch is drafted (2026-05-05). Per the handoff doc, the user prefers merging PR #1 first and then implementing this patch on `main`. This patch should land on `main` after PR #1 merges, on a `patch/0016-pr1-ci-test-failures` branch.
- `pre-dco-rewrite` (`9cedf88`) is the rollback anchor for the DCO sign-off rewrite, in case PR #1 needs to be undone.
- `design-locked-v0.3.x` (`41b56fe` post-rewrite) is the v0.3.x design lockdown anchor.
