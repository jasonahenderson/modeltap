---
patch: "PATCH-0024"
title: "Support short-id prefix lookup in requests show"
status: "approved"
date: "2026-05-08"
related:
  - "PATCH-0019 (read-command store wiring)"
  - "PATCH-0020 (requests command rename)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F6)"
branch: "patch/0024-show-short-id-prefix"
---

# PATCH-0024: Support short-id prefix lookup in requests show

## Problem

`modeltap requests show <id>` claims (in its help text) to accept
"the short 8-character prefix shown in the list table output", but
the underlying `storage.GetRequest` does an exact `WHERE id = ?`
lookup. Short prefixes always return "request not found." Users
who copy a short id from `modeltap requests list` cannot inspect
that capture without first looking up the full UUID via SQL.

Recorded as Finding F6 in `docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Enhance `SQLiteStore.GetRequest`** in
   `internal/storage/sqlite.go` to fall back to a prefix lookup
   when the exact-match query returns no row. The fallback uses
   `WHERE id LIKE ? || '%'` with `LIMIT 2`. If the prefix matches
   exactly one row, return it; if zero, return nil (existing "not
   found" behavior); if two or more, return nil and an "ambiguous
   prefix" error so the user can disambiguate.

2. **No `Store` interface change.** Only the SQLite implementation
   is updated; the prefix-fallback semantics are additive to the
   existing contract (exact match always wins). Other Store
   consumers that call `GetRequest` with a full UUID see no
   behavior change.

3. **Tests** in `internal/storage/sqlite_test.go` covering:
   - exact-id match (no regression)
   - 8-char prefix match → success
   - prefix match with multiple results → ambiguous error
   - prefix match with zero results → nil (no error)

## Out of Scope

- **Length-based dispatch.** Some implementations special-case
  full-UUID-length input vs. shorter input. We instead always
  try exact first then prefix; the cost of the extra query is
  negligible and the semantics are simpler.
- **Multi-character ambiguity disambiguation UI.** The CLI
  currently surfaces the storage error as-is; a future patch
  could list candidate IDs.
- **Other `Get*` methods** (`GetSession`, `GetRun`, etc.). They
  may have similar issues but are not driven by user-typed
  prefixes today; out of scope.

## Checklist

- [ ] `SQLiteStore.GetRequest` falls back to prefix LIKE when
  exact match returns no rows
- [ ] Ambiguous-prefix case returns nil + error
- [ ] Storage tests cover exact, prefix-unique, prefix-ambiguous,
  prefix-empty
- [ ] `go test ./...` passes
- [ ] Smoke verification: `modeltap requests show <8-char-prefix>`
  returns the captured detail
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F6 status
  updated to "Fixed in PATCH-0024"
