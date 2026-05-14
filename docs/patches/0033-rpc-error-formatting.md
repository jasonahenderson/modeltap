---
patch: "PATCH-0033"
title: "Unwrap RPCError framing in shell statusError; friendlier terminal-run reject"
status: "proposed"
date: "2026-05-11"
related:
  - "docs/releases/v0.3.0/retrospective.md (Finding F15)"
branch: "patch/0033-rpc-error-formatting"
---

# PATCH-0033: Unwrap RPCError framing in shell statusError; friendlier terminal-run reject

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

Every slash command in the production shell that hits a BFF error
surfaces the error as `<op> failed: rpc error -<code>: <message>` — the
raw JSON-RPC wire format leaks into user-facing status text.

Example surfaced during smoke step 10 (`/attach <completed-run-id>`):

```
run.attach failed: rpc error -32602: cannot attach terminal run
```

The "rpc error -32602:" prefix is internal framing the user does not
need to see; the same leak affects `model.list`, `model.switch`,
`session.list`, `session.resume`, `run.detach`, `run.cancel`, etc.

The terminal-run attach path additionally lacks any guidance — the
spec (`docs/releases/v0.3.0/smoke-test.md:84`) accepts the rejection,
but the user has no hint that they can still inspect the completed
run with `/run <run-id>`.

Recorded as Finding F15 in `docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Unwrap `*harness.RPCError` in
   `internal/harnesshost/production_runtime.go:statusError`.** When
   the underlying error implements the RPCError shape, surface just
   the `Message` field instead of `Error()` (which formats `rpc
   error -%d: %s`). Non-RPC errors are surfaced as today.

2. **Friendlier message for the terminal-run attach reject.** When
   the operation is `run.attach` and the RPCError message is
   `cannot attach terminal run`, replace it with:
   `run.attach failed: run is already complete — use /run <run-id>
   to inspect it`. The run id is taken from the operation context
   (the args passed to `handleRunAttachCommand`).

3. **Test the unwrap.** A new test in
   `internal/harnesshost/production_runtime_test.go` injects a
   stub `*harness.RPCError` into `statusError` and asserts the
   emitted `HostStatusEvent.Status` contains the inner message but
   not the `rpc error -` framing.

## Out of Scope

- **Read-only attach to terminal runs.** Surfacing the run's
  events for viewing via attach would require a new `replay-only`
  attachment state in storage; defer to v0.3.1 or FEAT-0024 work.

- **Translating other RPC error codes to friendlier messages.**
  Out of scope; this patch only handles the formatting leak and
  the specific terminal-run case the smoke test surfaced.

- **`statusError` callers in tests / non-production runtimes.**
  The fix lives in the production runtime; demo/fake runtimes do
  not exhibit the leak.

## Checklist

- [ ] `statusError` unwraps `*harness.RPCError` and surfaces its
  `Message` only
- [ ] `handleRunAttachCommand` substitutes the friendlier reject
  text when the RPCError carries the terminal-run message
- [ ] Unit test asserts the unwrap removes the `rpc error -`
  framing
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` F15 entry references
  this patch as fix

## Fix Detail

The core change in `statusError`:

```go
func (r *ProductionRuntime) statusError(op string, err error) error {
    msg := err.Error()
    var rpcErr *harness.RPCError
    if errors.As(err, &rpcErr) {
        msg = rpcErr.Message
    }
    r.sender.Send(harnessshell.HostStatusEvent{
        Status: op + " failed: " + msg,
        Kind:   harnessshell.StatusError,
    })
    return nil
}
```

The terminal-run friendly message is applied at the call site in
`handleRunAttachCommand` so the run id is in scope:

```go
if harness.IsRPCError(err, protocol.CodeInvalidParams) &&
    strings.Contains(err.Error(), "cannot attach terminal run") {
    r.sender.Send(harnessshell.HostStatusEvent{
        Status: "run.attach failed: run " + runID +
            " is already complete — use /run " + runID + " to inspect it",
        Kind:   harnessshell.StatusError,
    })
    return nil
}
return r.statusError("run.attach", err)
```
