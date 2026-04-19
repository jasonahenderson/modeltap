# PATCH-0003: Harness App ↔ ConnectionManager Wiring

**Status:** approved
**Date:** 2026-04-19
**Related:** v0.2.0 Bundle 6 (WU-073, WU-074), WU-083 (paste summarize), WU-086 (connection UX)
**Branch:** exploration/integrated-harness

## Problem

The v0.2.0 harness has two independent halves: the Bubbletea `App` (Bundle 5 + Bundle 7 tools + WU-083/086 overlays) and the `ConnectionManager` + `ProtocolClient` (Bundle 6). Neither side references the other. The `App` produces `SubmitMsg` on submit and `PasteSummarizeRequestMsg` on paste summarize; nothing consumes them. The `App` has no way to read connection state or to request a reconnect, so the `/status` and `/reconnect` commands called out in the Bundle 13 design (D7) cannot be implemented. Downstream WUs keep running into the same gap: WU-083 summarize, WU-085 model commands, WU-092 history traversal all need a path from the `App` into the connection.

Starting the manager (config → NewConnectionManager → Connect) is the job of the CLI launch command (later WU). This patch is narrower: it defines and wires the **in-process surface** the `App` uses to reach an injected manager, without taking on config / lifecycle ownership.

## Scope

1. Introduce a narrow `ConnSurface` interface in the harness package: `State() string`, `Reconnect() tea.Cmd`, `Client() ConnProtocolClient`. `*ConnectionManager` satisfies it directly.
2. Introduce a companion `ConnProtocolClient` interface covering the subset of methods the `App` invokes: `ContentTransform(ctx, *protocol.ContentTransform) (json.RawMessage, error)` and `SubmitTurn(ctx, *protocol.TurnSubmit) (*TurnSubmitAck, error)`. `*ProtocolClient` satisfies it.
3. Add `App.conn ConnSurface` (nil-safe) plus `AppOptions.Conn` so callers can wire one in at construction, and `(*App).SetConn(c)` for post-construct wiring (tests / deferred wiring).
4. Handle `PasteSummarizeRequestMsg` in `App.Update`: return a `tea.Cmd` that calls `ContentTransform`, then emits `PasteResolvedMsg{Strategy: summarize, Content: <summary>, Original: <original>}`. On error, emit a `BannerMsg` with the failure message and call `a.paste.Complete()` so the overlay clears.
5. Handle `SubmitMsg` in `App.Update`:
   - If `IsCommand && Command == "status"`: produce a `BannerMsg` summarizing `a.conn.State()`.
   - If `IsCommand && Command == "reconnect"`: return `a.conn.Reconnect()` wrapped with a status banner.
   - Otherwise (non-command turn): produce a `tea.Cmd` that calls `SubmitTurn`, emits an ack marker message (reserved for later banner), and relies on the ConnectionManager event bridge for streaming (already wired in Bundle 6).
6. Introduce a `TurnSubmittedMsg{TurnID, SessionID, err}` marker so tests can assert dispatch and so the streaming viewport can flip the spinner on — actual flipping is a later WU.
7. Tests use a fake `ConnSurface` (records calls, returns canned results) to exercise every new path.

## Out of Scope

- Starting / configuring the `ConnectionManager` from an `AppOptions` field — that requires a config surface and lifecycle owner; belongs to the CLI launch WU.
- `/history {user|project|session}` (WU-092): needs a history controller that pages through `history.list` — out of scope here. This patch only handles `/status` and `/reconnect`.
- `/session unlock` and `/mcp ...` commands — separate WUs, each with its own manager.
- Making the harness launchable end-to-end against a running BFF. This patch makes the wiring possible; a later WU will compose it into `modeltap harness` (or similar CLI verb).

## Checklist

- [x] `ConnSurface` + `ConnProtocolClient` interfaces defined in `internal/harness/app_conn.go`
- [x] `*ConnectionManager` / `*ProtocolClient` assertions added (compile-time `var _ ConnSurface = …`)
- [x] `AppOptions.Conn` and `(*App).SetConn(c)` wired
- [x] `PasteSummarizeRequestMsg` handler in `App.Update` calls `ContentTransform` and produces `PasteResolvedMsg`
- [x] `SubmitMsg` handler routes `/status`, `/reconnect`, and free-form turns
- [x] Fake `ConnSurface` test double under `internal/harness/app_conn_test.go`
- [x] `go vet ./...` clean, `go test ./...` green

## Fix Detail

The interface split keeps the App decoupled from the manager's internals (heartbeats, reconnect loop) while exposing exactly what the App needs. The `ConnProtocolClient` split is deliberate: the manager's `Client()` returns a pointer that may be nil during reconnect windows; App code must null-check before every use.

`SubmitTurn` is a synchronous call — the App wraps it in a `tea.Cmd` so it runs off the Update goroutine. The `TurnSubmittedMsg` it emits carries `TurnID` so the streaming viewport can start a spinner keyed to that ID (later WU; this patch only emits the marker).
