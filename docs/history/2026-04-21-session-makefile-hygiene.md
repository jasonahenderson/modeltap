# 2026-04-21 Session — Makefile Hygiene (PATCH-0010)

Continuation of the same working session that produced PATCH-0009 (root README). Split into its own log because the Makefile work is a distinct patch with a different scope.

## What was discussed

User ran `make` after the PATCH-0009 commits and hit two issues:

1. `make build` → `/bin/sh: /usr/local/opt/go/bin/go: No such file or directory`. The Makefile hard-codes Go at a Homebrew-managed path that was empty on this machine; actual Go is at `/usr/local/go/bin/go`.
2. Before the Go error surfaced, `make` (with no args) ran `all: fmt vet lint test build`. The `fmt` step's `gofmt -s -w .` rewrote 45 files across `internal/` in place — pure whitespace simplifications, but surprising mutations coming from a build command.

User asked whether having `gofmt` in a Makefile default target is standard. Short answer: having `make fmt` as a convenience is common; having `gofmt -w` wired into the default build target is non-standard and the cause of the surprise. Modern Go projects use check-only default (`gofmt -s -l` with non-empty exit) plus editor format-on-save for the inner loop.

## Decisions

- Split the 45-file gofmt drift cleanup into its own `ADMIN:` commit so PATCH-0010 stays a Makefile-only diff.
- PATCH-0010 makes two changes: `GO ?= go` (PATH-resolved, env-var override preserved), and a new `fmt-check` target that `all:` uses instead of `fmt`. `make fmt` stays as the explicit rewrite command.
- Not landing: pre-commit hooks, editor configuration, CI `fmt-check` step, `golangci-lint` install docs. All out of scope; cheap to add later.
- `make lint` still requires `golangci-lint` to be installed; that's unchanged from baseline and not addressed here.

## Actions taken

- Drafted `docs/patches/0010-makefile-hygiene.md` (status: done, same-session approval).
- Registered PATCH-0010 in `docs/patches/README.md` and `docs/releases/v0.2.0/changelog.md`.
- Committed `ADMIN: apply gofmt -s to internal/` — the 45-file cleanup pass.
- Rewrote `Makefile`: `GO ?= go`, new `fmt-check` target, `all:` rewired, `.PHONY:` extended.
- Verified `make fmt-check`, `make vet`, `make build` on the post-ADMIN tree — all green. `make lint` skipped (no `golangci-lint`).
- Committed `PATCH-0010: Makefile hygiene — PATH-resolved Go + check-only default`.
- Commits this half of the session:
  1. `ADMIN: apply gofmt -s to internal/`
  2. `PATCH-0010: Makefile hygiene — PATH-resolved Go + check-only default`
  3. `ADMIN: log session for 2026-04-21 Makefile hygiene` (this log)

## Files created

- `docs/patches/0010-makefile-hygiene.md`
- `docs/history/2026-04-21-session-makefile-hygiene.md`

## Files modified

- `Makefile` — `GO ?= go`, split `fmt` / `fmt-check`, rewired `all:`
- `docs/patches/README.md` — PATCH-0010 registered
- `docs/releases/v0.2.0/changelog.md` — PATCH-0010 row added to Patches table
- 45 `.go` files under `internal/` — gofmt cleanup (separate ADMIN commit)

## What's next / open items

- **`golangci-lint` not installed on the build machine.** `make lint` will fail until the user installs it (`brew install golangci-lint` on this host, or the official install script). Out of scope for PATCH-0010; can be a tools-bootstrap patch later if worth it.
- **Wiring `fmt-check` into CI.** `.github/workflows/ci.yml` does its own vet/test; adding `gofmt -s -l` enforcement there is a small follow-up that prevents tree drift from accumulating again.
- **Pre-commit hook.** Standard convention for teams; would run `gofmt -s -w` on staged Go files only. Not yet.
- **Homebrew Go symlink at `/usr/local/opt/go`.** Stale on this machine — `brew --prefix go` reports it, but the directory is empty; actual Go is the official tarball at `/usr/local/go`. Not a project concern; user-machine cleanup if/when it matters.
