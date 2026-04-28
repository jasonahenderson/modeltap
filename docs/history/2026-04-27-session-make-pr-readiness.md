# 2026-04-27 — Session: Makefile and lint PR readiness

## Scope

Cleaned up the repository's local PR-readiness path after checking
the Makefile targets.

## Changes

- Ran `make fmt` to clear existing `gofmt -s` drift.
- Implemented PATCH-0012 by removing `lint` from the default
  `all` target while keeping `make lint` explicit and strict.
- Migrated golangci-lint config to v2 (`.golangci.yaml`) and kept
  the selected lint stack: errcheck, govet, ineffassign,
  staticcheck, and unused.
- Updated `make lint` to resolve either `golangci-lint` on `PATH`
  or the standard `$(go env GOPATH)/bin/golangci-lint` install path.
- Made CLI config-loading tests self-contained by isolating config
  paths and provider key env vars.
- Cleaned lint findings across legacy test helpers, unchecked test
  writes/reads, unused extraction leftovers, and minor staticcheck
  simplifications.

## Verification

- `make fmt-check`
- `make lint`
- `make all`
