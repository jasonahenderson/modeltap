---
patch: PATCH-0017
title: Session-scoped project context
status: proposed
date: 2026-05-07
related:
  - FEAT-0008: Runtime Server
  - FEAT-0023: Desktop GUI Client
branch: patch/0017-session-scoped-project-context
release: TBD
---

# PATCH-0017: Session-scoped project context

## Problem

The runtime server currently stores project context at connection scope.
`internal/runtime/capabilities.go` keeps a `protocol.ProjectContext` on
`CapabilityManager`, and handlers refresh that value from `session.create` and
`session.resume`.

That works for the current terminal harness because it has one active chat at a
time. It does not work for a multi-chat client. A GUI with two tabs on one
runtime-server connection could switch between project roots and overwrite the
connection-level project context used by prompt assembly, history filtering,
run controls, and context lookup.

Project context is logically a session attribute. This patch formalizes
Amendment 001 of FEAT-0008 and removes the multi-chat foreclosure identified by
FEAT-0023.

## Scope

- Make `ActiveSession.Project` or an equivalent session-owned field the
  canonical runtime project context.
- Preserve `root`, `config_file`, and `config_content`; do not collapse the
  session context to only a root string if prompt assembly needs config
  content.
- Stop treating `CapabilityManager.ProjectContext()` as the authoritative
  source for per-turn project state.
- Update readers that currently reach through connection capabilities,
  including prompt assembly, history queries, run controls, turn submission,
  session details, and session list filtering.
- Keep `capabilities.register.project` as an optional legacy default for
  compatibility with existing single-session clients.
- Prefer `session.create` or a future `session.open` payload as the canonical
  place to bind project context for a new session.
- Continue accepting project context on `session.resume` so config changes are
  picked up when a session is reopened.
- Add tests proving two active sessions on one connection can hold different
  project roots without collision.
- Add tests proving prompt assembly reads the active session's project context,
  not a mutable connection-level value.
- Update FEAT-0008 Amendment 001 from planned to landed when implemented.

## Out of Scope

- Desktop GUI tab management.
- Authentication or per-session ACL redesign.
- Knowledge-layer scoping redesign.
- Protocol version bump. This is additive and compatibility-preserving.
- Removing existing terminal harness compatibility.

## Checklist

- [ ] Add a session-owned project-context field that carries root, config file,
  and config content.
- [ ] Treat `capabilities.register.project` as a legacy per-connection default,
  not canonical session state.
- [ ] Store project context during `session.create` and `session.resume`.
- [ ] Route prompt assembly through the active session's project context.
- [ ] Route history filtering and run controls through the active session's
  project context.
- [ ] Audit all `Capabilities().ProjectContext()` call sites and remove or
  deprecate them.
- [ ] Tests cover two active sessions on one connection with different project
  roots.
- [ ] Tests cover project config refresh on session resume.
- [ ] Tests cover compatibility for the current terminal harness registration
  flow.
- [ ] FEAT-0008 Amendment 001 is marked landed after implementation.
- [ ] `go build ./...`, `go vet ./...`, and `go test ./...` pass.

## Fix Detail

Use a two-step migration if it keeps review small:

1. Keep `CapabilityManager.ProjectContext()` as a deprecated compatibility
   shim while session-owned context lands.
2. Migrate all readers to explicit session context.
3. Remove the shim once no production reader depends on it.

The protocol does not need a version bump. Old clients can keep sending project
context at registration; the runtime server treats it as a default for
subsequent session creation/resume. New clients send project context per
session and omit it from registration.
