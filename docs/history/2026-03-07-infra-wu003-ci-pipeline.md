# WU-003: CI Pipeline (GitHub Actions)

**Date:** 2026-03-07
**Role:** Infrastructure Engineer
**Status:** Complete

## Summary

Created GitHub Actions CI workflows for modeltap: a main CI pipeline for lint, test, and build; a DCO sign-off check for pull requests; and a standard Go .gitignore file.

## Files Created

- `.github/workflows/ci.yml` — CI pipeline triggered on push and pull_request to main. Three parallel jobs:
  - **lint** — Uses golangci/golangci-lint-action to run `golangci-lint run ./...`
  - **test** — Runs `go test -race ./...`
  - **build** — Runs `go build ./...` to verify compilation
- `.github/workflows/dco.yml` — DCO sign-off check triggered on pull_request to main. Iterates all commits in the PR and verifies each contains a `Signed-off-by:` line, per ADR-0011.
- `.gitignore` — Standard Go gitignore covering build output (`bin/`, `*.exe`), test/coverage files (`*.out`, `coverage.*`), vendor directory, and OS/IDE artifacts (`.DS_Store`, `.idea/`, `.vscode/`).

## Design Decisions

- Used official actions: actions/checkout@v4, actions/setup-go@v5, golangci/golangci-lint-action@v6.
- Go version set to `1.25.x` to match the project's Go 1.25.6 requirement.
- DCO check implemented as a shell script rather than a third-party action to minimize external dependencies and provide clear error messages.
- All jobs use `ubuntu-latest` runner.
- Permissions scoped to minimum required (`contents: read`, `pull-requests: read`).

## Verification

- `ci.yml` — valid YAML, contains correct triggers, three jobs with expected steps.
- `dco.yml` — valid YAML, contains correct trigger, DCO validation logic checks all PR commits.
- `.gitignore` — covers all required patterns (bin/, *.exe, .DS_Store, vendor/, coverage output).

## Notes

- CI jobs run in parallel (lint, test, build are independent).
- DCO check uses `fetch-depth: 0` to access full commit history for the PR range.
- No changes were committed; files are staged for review.
