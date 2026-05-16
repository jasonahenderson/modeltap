---
patch: "PATCH-0010"
title: "Makefile Hygiene — PATH-resolved Go + check-only default"
status: "done"
date: "2026-04-21"
related:
  - "PATCH-0009 (root README — flagged this as a follow-up)"
branch: "exploration/integrated-harness"
---

# PATCH-0010: Makefile Hygiene — PATH-resolved Go + check-only default

## Problem

Today's `Makefile` has two issues that trip up real builds:

1. **Hard-coded Go path.** `GO ?= /usr/local/opt/go/bin/go` points at a Homebrew-managed location that is empty on at least one active developer machine (Go lives at `/usr/local/go/bin/go` there from an official tarball). Result: `make` / `make build` / `make vet` all fail with `No such file or directory` before anything runs.
2. **Default target silently rewrites source.** `all: fmt vet lint test build` — and `fmt` runs `gofmt -s -w .`, which mutates every Go file in the tree in place. A developer typing `make` or `make build` expecting a compile gets a 45-file cross-`internal/` diff dropped into their working tree. This is surprising, race-prone with editor format-on-save, and makes CI runs non-idempotent.

Both issues surfaced when a developer ran `make` after a routine `git pull` and got `/usr/local/opt/go/bin/go: No such file or directory` preceded by a 45-file gofmt cleanup they didn't ask for. The gofmt changes were pure `-s` whitespace simplifications — safe, but not what `make` should be quietly doing.

## Scope

1. **`GO ?= go`** — resolve Go through `PATH` rather than a hard-coded directory. The `?=` operator preserves the override path: `GO=/usr/local/go/bin/go make build` still works, and so does `GO=/custom/path make build`.
2. **Split the `fmt` target:**
   - `fmt` keeps its current behavior (`gofmt -s -w .`) — explicit opt-in when a developer wants to rewrite.
   - `fmt-check` is new: `gofmt -s -l .` piped through a non-empty check, exits non-zero if any file would be reformatted. No mutations.
3. **Rewire `all:`** to `fmt-check vet lint test build`. Default `make` no longer mutates source; it verifies.
4. **`.PHONY:`** updated to include `fmt-check`.
5. **No change** to `build`, `test`, `lint`, `vet`, `clean` target bodies.

## Out of Scope

- Installing or documenting `golangci-lint` — the current `make lint` target already presumes it's installed. Still true after this patch; `make lint` will fail on systems without it. Separate concern.
- Pre-commit hooks (`lefthook`, `pre-commit`). Cheap to add later; not needed for this patch.
- CI-side enforcement. `.github/workflows/ci.yml` already runs its own `go vet` / `go test` steps; wiring `fmt-check` into CI is a follow-up if drift recurs.
- Editor configuration (`.editorconfig`, `.vscode/settings.json`). Out of scope — developers bring their own editor setup.
- The prior gofmt drift itself. Landed separately as `ADMIN: apply gofmt -s to internal/` so this patch is pure Makefile.

## Checklist

- [x] `GO ?= go` (PATH-resolved, override-friendly)
- [x] `fmt-check` target added, using `gofmt -s -l .` with non-empty exit
- [x] `all:` rewired to `fmt-check vet lint test build`
- [x] `.PHONY:` includes `fmt-check`
- [x] `fmt` target unchanged in behavior (still rewrites, still explicit)
- [x] `make fmt-check` passes on the tree post-`ADMIN: apply gofmt -s to internal/`
- [x] `make vet` passes
- [x] `make build` produces `bin/modeltap`
- [x] `make test` runs (race detector on, per existing config)
- [x] Patch registered in `.sdlc/patches/README.md` index and `.sdlc/releases/v0.2.0/changelog.md`
- [ ] `make lint` not verified — `golangci-lint` not installed on the build machine; unchanged from baseline

## Fix Detail

### Default-target behavior before / after

Before:
```make
all: fmt vet lint test build   # fmt rewrites source in place
```

After:
```make
all: fmt-check vet lint test build   # fmt-check is read-only
```

A developer who wants the old "just clean everything up" behavior runs `make fmt` explicitly.

### `fmt-check` implementation

```make
fmt-check:
	@files=$$($(GO_FMT) -s -l .); \
	if [ -n "$$files" ]; then \
	  echo "gofmt -s would rewrite the following files:"; \
	  echo "$$files"; \
	  echo "run 'make fmt' to apply."; \
	  exit 1; \
	fi
```

This is the standard Go-project pattern (used by Kubernetes `hack/verify-gofmt.sh`, HashiCorp projects' `fmtcheck`, etc.) — `gofmt -l` lists files that would be rewritten; if the list is non-empty, fail with a clear instruction.

Using `$(GO_FMT)` (defaulting to `gofmt`, PATH-resolved) rather than calling `$(GO) fmt` because `go fmt` wraps `gofmt` without the `-s` flag.

### Why `?=` not `:=` for `GO`

`?=` means "assign if not already set." That keeps the env-var override path open (`GO=/alt/path make ...`) while giving `go` (PATH-resolved) as the sensible default. `:=` would force the Makefile's choice and shadow any env var.

### Why this doesn't break CI

`.github/workflows/ci.yml` invokes `go` via `actions/setup-go`, which puts `go` on `PATH` before any make invocation. PATH-resolving is what CI was already doing; it's only the Makefile's hard-coded path that diverged.
