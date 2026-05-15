---
patch: "PATCH-0028"
title: "Add session.create RPC and harness auto-call on Ready"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0008 (BFF server)"
  - "FEAT-0014 (Harness Conversation Shell)"
  - "PATCH-0023 (Dispatch host-native slash commands)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F10)"
branch: "patch/0028-session-create-rpc"
---

# PATCH-0028: Add session.create RPC and harness auto-call on Ready

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

Slash commands that depend on an active session (e.g., `/model X`,
`/context`, `/sessions clear`) fail with
`-32602 session_id is required` when the user types them before
submitting their first turn. The only paths that establish a
session_id today are:

- `turn.submit` — auto-creates a session lazily if `submit.SessionID`
  is empty (turn.go:82-83 mints `uuid.NewString()` and turn.go:122-138
  persists it on first call), or
- `session.resume` — requires an existing session_id.

Result: the user runs `modeltap shell`, types `/model qwen3.5:35b`
expecting to switch the active model, and gets an `InvalidParams`
error. To recover, they have to submit a throwaway turn against the
default-routed model so the session gets created, then re-issue
`/model`. This is the failure path captured by Finding F10 in
`.sdlc/releases/v0.3.0/retrospective.md`.

## Scope

1. **Add a `session.create` JSON-RPC method** in
   `internal/protocol/messages.go`:

   - `MethodSessionCreate = "session.create"` constant.
   - `SessionCreate` request type carrying a `Project ProjectContext`.
   - `SessionCreateResponse` response type carrying the
     server-assigned `SessionID` and the project echo.

2. **Add fixtures + conformance coverage**:
   `internal/protocol/fixtures/requests/session_create.json` and
   `internal/protocol/fixtures/responses/session_create.json`,
   registered in `conformance_test.go`'s `allFixtureCases` and the
   request-method registry table.

3. **Implement `handleSessionCreate`** in `internal/bff/session.go`.
   The handler:
   - Mints a new `uuid.NewString()` session id.
   - Persists a `storage.Session` (mirrors the lazy-create branch of
     `turn.go:122-138`).
   - Acquires the session lock for the connection (matches
     `session.resume` semantics).
   - Updates the connection's project context with the request's
     `Project`.
   - Binds the connection (`conn.SetSessionID(...)`) and ensures the
     active session entry exists.
   - Returns `SessionCreateResponse` with the new id and the project
     echo.

4. **Register the handler** in `internal/bff/server.go` alongside
   the existing session method registrations.

5. **Auto-call from the harness on `ConnStateReady`**. In
   `internal/harnesshost/production_runtime.go`'s
   `observeRuntimeMessage`, when the connection enters Ready, kick
   off a background `bootstrapSession` call that issues
   `session.create` and stores the returned id via
   `r.mode.SetSessionID`. Best-effort: a failure does not abort the
   shell — `turn.submit` will still create a session implicitly the
   first time it runs.

6. **Tests**:
   - BFF: a new `internal/bff/session_create_test.go` that calls the
     handler and verifies a session row lands in storage with the
     returned id, that the session lock is acquired, and that the
     connection's `SessionID()` reflects the new id.
   - Harness: extend existing production-runtime integration coverage
     so the BFF stub responds to `session.create` and the harness
     observes a non-empty `SessionID()` after the connection reaches
     Ready (no `model.switch` in the test path because the stub does
     not need to honor the override; the goal is to verify the
     auto-call wiring lands).

## Out of Scope

- **Session-resume preference.** When the user wants to attach to a
  prior session, `--resume <id>` (already wired via shell flags) is
  the right path; this patch's auto-create only fires when no
  `--resume` flag is present.
- **Connection-scoped model override.** The alternative direction
  ("loose `model.switch` and let the override apply to the next
  session created on first turn") is not pursued. It muddies the
  contract: every other RPC that reads session-state would also need
  a connection-fallback path. Auto-creating a session on Ready is
  the simpler invariant.
- **Compact / fork on session.create.** Initial session is empty;
  no transcript, no override, no compaction. Those land via their
  existing methods if the user invokes them.

## Checklist

- [ ] `MethodSessionCreate`, `SessionCreate`, `SessionCreateResponse`
  in protocol
- [ ] Request and response fixtures registered in
  `conformance_test.go`
- [ ] `handleSessionCreate` registered and exercised by a unit test
  in `internal/bff`
- [ ] `production_runtime.bootstrapSession` runs on `ConnStateReady`
  when `r.mode.SessionID()` is empty
- [ ] BFFStub in `internal/harnesshost/testutil` answers
  `session.create` with a stub session id (existing tests need this
  to pass once the auto-call lands)
- [ ] `go test ./...` passes
- [ ] Smoke verification: in a fresh shell, `/model qwen3.5:35b`
  succeeds before any turn has been submitted; subsequent
  `turn.submit` reuses the auto-created session id
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` Finding F10 status
  updated to "Fixed in PATCH-0028"
