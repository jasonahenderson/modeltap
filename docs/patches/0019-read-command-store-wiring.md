---
patch: "PATCH-0019"
title: "Wire SQLite store into logs, show, export, metrics commands"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0008 (BFF server)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F5)"
branch: "patch/0019-read-command-store-wiring"
---

# PATCH-0019: Wire SQLite store into logs, show, export, metrics commands

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`modeltap logs`, `modeltap show`, `modeltap export`, and `modeltap
metrics` all return:

```
Error: no store configured
```

Each command's RunE checks a package-level `xxxStore storage.Store`
variable and bails if it is nil:

- `internal/cli/logs.go:58` — `if logsStore == nil { return ... }`
- `internal/cli/show.go:45` — `if showStore == nil { return ... }`
- `internal/cli/export.go` — same pattern
- `internal/cli/metrics.go:60`, `:146` — same pattern

The matching `SetLogsStore`, `SetShowStore`, `SetExportStore`,
`SetMetricsStore` setters exist as a test-injection seam, but no
production code path ever calls them. `internal/cli/root.go` registers
the commands without wiring storage; `cmd/modeltap/main.go` only
loads `.env`.

Net effect: every traffic-inspection command in v0.3.0 is non-functional.
This was caught during the v0.3.0 smoke-test debug session when the
maintainer tried to inspect a captured request body to diagnose an
upstream 400. Recorded as Finding F5 in
`docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Add `internal/cli/store_helper.go`** with a small helper that
   loads config (via `config.LoadWithViper("")`) and opens a SQLite
   store at `cfg.DBPath` using `storage.NewSQLiteStore`. Returns the
   store and an error.

2. **Modify each of the four read commands' `RunE`** so that, when
   the package-level store variable is nil (production path), the
   helper is called and the store is opened lazily for the duration
   of the command. The existing `xxxStore` variable continues to
   serve as the test-injection seam.

3. **Defer `store.Close()`** in the lazy path so the SQLite WAL
   checkpoints cleanly on command exit.

4. **No change to test files.** Tests already inject directly via
   the package-level variable; that path is preserved unchanged.

## Out of Scope

- Removing the `SetXxxStore` test-injection pattern. It is still
  used by `*_test.go` files in the package.
- Refactoring the four commands into a shared base. Each command
  has command-specific flags and output formats; deduplication is
  not warranted by this fix.
- Wiring stores into the BFF/shell path. The shell connects to the
  BFF and does not need a direct store.
- F1 (BFF health-check wiring) and F3 (cloud-probe target). Those
  are separately scoped.

## Checklist

- [ ] `internal/cli/store_helper.go` added with
  `openStoreFromConfig() (storage.Store, error)`
- [ ] `logs.go`, `show.go`, `export.go`, `metrics.go` RunE bodies
  open the store lazily when the package var is nil and defer
  Close
- [ ] Existing `*_test.go` files in `internal/cli` still pass
  (test injection unchanged)
- [ ] `go test ./...` passes
- [ ] Smoke verification: `./bin/modeltap logs --limit 5` returns a
  table (not "no store configured") on a populated DB
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F5 row added
