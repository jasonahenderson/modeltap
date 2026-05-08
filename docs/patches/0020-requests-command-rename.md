---
patch: "PATCH-0020"
title: "Rename logs/show/export to requests list/show/export"
status: "approved"
date: "2026-05-08"
related:
  - "PATCH-0019 (read-command store wiring)"
branch: "patch/0020-requests-command-rename"
---

# PATCH-0020: Rename logs/show/export to requests list/show/export

## Problem

The three traffic-inspection top-level commands (`logs`, `show`,
`export`) are misnamed. What they operate on is captured upstream
HTTP request/response exchanges — modeltap's domain primitive per
ADR-0005 — not log lines, and the noun "logs" overloads the term
already used for application/diagnostic logs.

Industry-aligned naming for the same primitive across LLM-proxy and
LLM-observability tooling:

- **Helicone:** `Requests` tab
- **LangSmith:** `runs` / `traces`
- **Langfuse:** `traces` / `generations` / `observations`
- **Phoenix (Arize):** `traces` / `spans`
- **mitmproxy:** `flows`
- **Charles / Fiddler:** `sessions`

Helicone is modeltap's closest functional peer (LLM proxy with
capture); their dashboard uses **Requests**. The modeltap SQLite
table is also already named `requests`, and code comments throughout
the codebase use "captured request" language.

## Scope

1. **Rename top-level commands** `logs` / `show` / `export` to
   subcommands of a new `requests` parent:
   - `modeltap logs`     → `modeltap requests list`
   - `modeltap show <id>`→ `modeltap requests show <id>`
   - `modeltap export`   → `modeltap requests export`

2. **Consolidate the three per-command store package vars** into
   a single `requestsStore` shared by all three subcommands.
   Replace `SetLogsStore` / `SetShowStore` / `SetExportStore` with
   a single `SetRequestsStore`. The lazy-open pattern from
   PATCH-0019 stays intact.

3. **Rename source files** for grep-ability:
   - `internal/cli/logs.go`        → `internal/cli/requests_list.go`
   - `internal/cli/show.go`        → `internal/cli/requests_show.go`
   - `internal/cli/export.go`      → `internal/cli/requests_export.go`
   - `internal/cli/logs_test.go`   → `internal/cli/requests_list_test.go`
   - `internal/cli/show_test.go`   → `internal/cli/requests_show_test.go`
   - `internal/cli/export_test.go` → `internal/cli/requests_export_test.go`

4. **Add `internal/cli/requests.go`** with the parent command
   constructor and the consolidated store + setter.

5. **Update `internal/cli/root.go`** to register
   `newRequestsCommand()` in place of the three previous top-level
   command registrations. Update help-text examples.

6. **Update tests** to reference `requestsStore` and use the new
   command paths (e.g. `SetArgs([]string{"requests", "list"})`).

7. **Update changelog** for the v0.3.0 release with the user-facing
   rename note.

## Out of Scope

- **Backwards-compat aliases.** v0.3.0 has not tagged. Breaking
  the old command names is acceptable; the changelog records the
  rename. No deprecated aliases are added.
- **Renaming `metrics`.** Metrics is its own concept (aggregations
  over requests); the noun does not change.
- **OpenTelemetry alignment / `traces`+`spans` model.** Worth
  considering when modeltap models nested operations (turn → model
  call → tool call). Out of scope for this rename.
- **The SQLite table name.** Already named `requests`; no schema
  change required.

## Checklist

- [ ] `internal/cli/requests.go` added with parent command,
  `requestsStore` var, `SetRequestsStore` setter
- [ ] Three command files renamed; their constructors renamed to
  `newRequestsListCommand` / `newRequestsShowCommand` /
  `newRequestsExportCommand` and their `Use:` set to `list` /
  `show` / `export`
- [ ] All RunE bodies use the consolidated `requestsStore`
- [ ] Three test files renamed; references updated to
  `requestsStore`; CLI invocations updated to use the new path
- [ ] `root.go` registers `newRequestsCommand()`; old top-level
  registrations removed
- [ ] Help-text examples in `root.go` updated to reference
  `modeltap requests list/show/export`
- [ ] `go test ./...` passes
- [ ] Smoke verification: `./bin/modeltap requests list --limit 5`
  returns the captured-request table
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/changelog.md` entry added
