---
patch: "PATCH-0017"
title: "Session-scoped project context"
status: "approved"
date: "2026-05-07"
related:
  - "FEAT-0008"
  - "FEAT-0023"
branch: "patch/0017-session-scoped-project-context"
release: "v0.3.5"
---

# PATCH-0017: Session-scoped project context

## Problem

The BFF stores project context (`{ root, config_file, config_content }`) at
the connection scope: `internal/bff/capabilities.go:38` holds it on
`CapabilityManager`, and `internal/bff/session.go:169`
(`UpdateProjectContext(req.Project)`) overwrites that single slot on every
`session.resume`. The data model already supports many concurrent
`ActiveSession` rows per connection (`internal/bff/session.go:29`), and each
`ActiveSession` already has its own `Project` field
(`internal/bff/session.go:58`), but every reader of project context goes
through the connection-level `Capabilities().ProjectContext()` accessor.

Result: a client that runs more than one chat on a single connection
clobbers the prompt-assembly project context every time it switches the
focused session. This is invisible today because the only client (the TUI)
is single-active-session, but FEAT-0023 (Desktop GUI Client) needs
concurrent multi-chat over one connection.

This patch is the protocol-internal refactor that unblocks FEAT-0023 and
formalizes Amendment 001 of FEAT-0008.

## Scope

Move project context from connection scope to session scope, audit every
reader, and preserve wire-protocol back-compat for the TUI.

- Remove `project protocol.ProjectContext` from `CapabilityManager`
  (`internal/bff/capabilities.go:38`); remove `ProjectContext()` and
  `UpdateProjectContext(...)` accessors. (Or: keep them as deprecated
  shims that read/write the active session's project — see "Fix Detail.")
- Make `ActiveSession.Project` the canonical store. Promote it from
  `string` to `protocol.ProjectContext` if downstream readers need
  `config_file`/`config_content`; otherwise keep as `string` (root) and
  carry config separately.
- In `session.resume` (`internal/bff/session.go:169`), drop the
  `conn.Capabilities().UpdateProjectContext(req.Project)` call; instead
  populate `active.Project` from `req.Project` directly.
- Add a new `session.open` method (or reuse `session.resume` with a
  no-op-if-absent semantics) that establishes a fresh session and binds
  project context to it, so a client never has to reach through
  `capabilities.register` to set project for a new session.
- Rewrite all readers to take session-scoped project:
  - `internal/bff/session.go:111` (session creation default)
  - `internal/bff/session.go:197`, `:209` (session details)
  - `internal/bff/turn.go:126` (per-turn assembly)
  - `internal/bff/run_controls.go:55` (run controls)
  - `internal/bff/history.go:28`, `:64`, `:71` (history queries)
- Update `capabilities_test.go:79` and `session_test.go:82-83` to
  reflect the new scope (project is no longer asserted via
  `Capabilities()`).
- In `capabilities.register` request handling, accept an optional
  `project` field for back-compat: if provided, store it as a
  per-connection *default* that's applied to any subsequent
  `session.open` / `session.resume` that omits its own project. This
  preserves TUI back-compat without baking project into the
  capabilities scope.
- Update FEAT-0008 §"Planned Amendments" → "Amendment 001" status from
  `planned` to `landed` and timestamp the change.
- Update FEAT-0008 protocol payload schema for `capabilities.register`:
  remove `project` from the canonical example; add it back as an
  optional back-compat field with a deprecation note.
- Update FEAT-0008 protocol payload schema for `session.resume`: no
  change required (it already carries `project`).
- Add `session.open` payload schema to FEAT-0008 protocol section if
  reusing `session.resume` is not preferred.

## Out of Scope

- Multi-chat UI primitives (tab management, per-tab status, per-tab model
  overrides) — those belong to FEAT-0023.
- Authentication or per-session ACLs beyond what FEAT-0010 already
  defines. `session_locked` (MT-CONN-008) keeps its current semantics;
  the patch only clarifies the wording.
- Knowledge-layer scoping changes. The patch passes the same project
  root through to knowledge queries; it does not redesign how knowledge
  scoping works.
- Protocol versioning bump. The change is wire-compatible; the protocol
  version stays.
- Any work that requires breaking the TUI's existing
  `capabilities.register` payload.

## Checklist

- [ ] Remove or deprecate `CapabilityManager.project`,
  `ProjectContext()`, `UpdateProjectContext(...)`
- [ ] Move project storage to `ActiveSession.Project` (promote type if
  needed for `config_file`/`config_content`)
- [ ] Drop `UpdateProjectContext` call in `session.resume`
- [ ] Introduce `session.open` (or document why
  `session.resume` is reused) and its payload
- [ ] Audit and rewrite every reader of
  `Capabilities().ProjectContext()` listed in Scope
- [ ] `capabilities.register` accepts optional `project` as a per-connection
  default for back-compat with the TUI
- [ ] Tests proving two `ActiveSession` rows on one connection can hold
  different project roots without collision
- [ ] Tests proving prompt assembly reads from the *active session's*
  project, not the connection
- [ ] Tests proving the TUI's existing `capabilities.register` flow
  continues to work end-to-end (back-compat smoke)
- [ ] Update FEAT-0008 Amendment 001 status to `landed`
- [ ] Update FEAT-0008 payload schemas
  (`capabilities.register`, `session.open` if added)
- [ ] `go build ./...`, `go vet ./...`, and `go test ./...` pass
- [ ] Lint clean

## Fix Detail

**Deprecation shim option.** Removing `Capabilities().ProjectContext()`
outright touches every reader in one commit. A lighter sequencing is to
keep the accessor but redefine it to return the *active session's*
project for the calling context — and panic (in tests) or log a warning
(in production) if no active session is bound. That lets the migration
land in two commits: one that flips the storage and back-compat shim,
one that deletes the shim after readers are migrated. Recommend the
two-commit path; cleaner blame, easier review.

**Why not bump the protocol version.** A version bump forces every
client to re-handshake. The change here is additive on the server
(accept project in two places, prefer the per-session one) and a
deprecation on the client (stop sending project at registration). Old
clients keep working. New clients drop the field. No version coordination
needed.

**`session.open` vs reusing `session.resume`.** `session.resume` today
implies a session row exists in storage. A multi-chat client opening a
fresh tab needs a method that *creates* the session and binds project in
one round trip. Two options:
- Add `session.open` — clean separation, but a new method to spec.
- Extend `session.resume` to create-on-missing — fewer methods, but the
  name lies.

Recommend `session.open` for clarity. The protocol gets one more method;
the semantics are obvious; `session.resume` keeps its narrow contract.

**Reinterpretation of MT-CONN-008.** The current diagnostic message says
"another harness owns the session" — that wording is already correct.
The scope of this patch is to ensure the *implementation* keys the lock
on `session_id`, not on `connection_id`, so a single client running two
sessions never sees a false-positive `session_locked`. Verify this in
tests.
