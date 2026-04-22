# PATCH-0012: Remove `lint` from the Makefile default target

**Status:** proposed
**Date:** 2026-04-22
**Related:** PATCH-0010 (Makefile hygiene — left this as an open checklist item)
**Branch:** exploration/integrated-harness

## Problem

`make` (the default `all:` target) still invokes `lint`, which calls `golangci-lint run ./...`. On any machine without `golangci-lint` installed, `make` fails with:

```
make: golangci-lint: No such file or directory
make: *** [lint] Error 1
```

This hits anyone building from a fresh clone who hasn't separately installed `golangci-lint`. It also regressed the user during normal harness-debug work today. PATCH-0010 explicitly left `make lint` unverified on the build machine because `golangci-lint` wasn't installed; that unresolved state has now surfaced as a real blocker.

## Scope

1. **Drop `lint` from `all:`**. New default becomes `all: fmt-check vet test build`.
2. **Keep `make lint` as an explicit target**. Its body (`golangci-lint run ./...`) is unchanged. A developer who wants linting still runs `make lint` explicitly; CI still runs it explicitly (see below). If the binary is missing, `make lint` still fails — that failure is now opt-in rather than a side effect of `make`.
3. **No change** to `build`, `test`, `fmt`, `fmt-check`, `vet`, `clean`.

## Out of Scope

- **Auto-installing `golangci-lint`.** Not this patch. Developers bring their own toolchain.
- **Graceful no-op when `golangci-lint` is missing.** Considered; rejected. Silent-skip hides real setup gaps. `make lint` should fail loudly when the tool isn't installed so the gap is visible rather than invisible.
- **CI behavior.** `.github/workflows/ci.yml` runs `make lint` (or its own equivalent) as a separate step; this patch does not touch CI. If CI was relying on `make` default to catch lint issues, it already covers that via dedicated steps.
- **Pre-commit hooks.** Out of scope — separate concern.

## Checklist

- [ ] `all:` target becomes `fmt-check vet test build` (drop `lint`)
- [ ] `lint:` target body unchanged
- [ ] `.PHONY:` line unchanged (already lists `lint`)
- [ ] `make` runs cleanly on a machine without `golangci-lint`
- [ ] `make lint` still runs cleanly on a machine with `golangci-lint` installed
- [ ] `make lint` still fails clearly on a machine without `golangci-lint`
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.2.0/changelog.md` entry added

## Fix Detail

### Default-target behavior before / after

Before:
```make
all: fmt-check vet lint test build
```

After:
```make
all: fmt-check vet test build
```

### Rationale for opt-in-but-strict (not silent skip)

The alternative — making `make lint` a no-op with a warning when `golangci-lint` isn't on `PATH` — was considered and rejected. Reasons:

- **Visibility.** If CI and contributor machines silently skip lint on missing binary, nobody notices when the binary stops being installed somewhere it should be. Loud failures surface environment drift.
- **Scope of this patch.** This patch is about the default `make` target. Changing `make lint` semantics (skip vs. fail) is a policy call about how strict the project is with developer environments; PATCH-0010's precedent is "developers bring their own toolchain, CI enforces." This patch sticks with that.
- **Installation is cheap.** `brew install golangci-lint` on macOS or `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` are both one-liners. Developers who want to run `make lint` locally install it once.

### Why this doesn't break CI

`.github/workflows/ci.yml` invokes steps that install `golangci-lint` before running it. CI was never relying on `make` default to include `lint`; it runs its own explicit lint step. Dropping `lint` from `make` default is a dev-experience fix, not a CI policy change.
