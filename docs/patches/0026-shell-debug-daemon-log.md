---
patch: "PATCH-0026"
title: "Capture auto-spawned daemon stdio to a log file via flag/env"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0008 (BFF server)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F4, sub-item 10b)"
branch: "patch/0026-shell-debug-daemon-log"
---

# PATCH-0026: Capture auto-spawned daemon stdio to a log file via flag/env

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

When `modeltap shell` auto-spawns a daemon (because the BFF socket
is absent), the daemon's stdin / stdout / stderr are explicitly nilled
in `internal/harness/connection.go:541-543`:

```go
cmd.Stdin = nil
cmd.Stdout = nil
cmd.Stderr = nil
```

A nil stdio in `os/exec` connects the child to `/dev/null`. The
intent is correct for production — daemon log lines must not corrupt
the Bubble Tea alt-screen — but it eliminates the only diagnostic
channel for daemon startup failures. F1, F2, F3 in this retrospective
were all delayed in diagnosis by this dead zone.

This is sub-item 10b of Finding F4 (BFF/TUI lifecycle fragility) in
`docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Add `MODELTAP_DAEMON_LOG=<path>` environment variable.** When
   set on the shell process, the auto-start path opens the file
   for append and assigns it as the daemon's `cmd.Stdout` and
   `cmd.Stderr`. `cmd.Stdin` stays nil. The shell prints a
   one-line confirmation to its own stderr before entering the
   TUI: `daemon log → <path>`. (This pre-TUI line is fine because
   alt-screen has not been entered yet.)

2. **Add `--debug-daemon-log <path>` flag to `modeltap shell`.**
   When the flag is provided, it propagates as
   `MODELTAP_DAEMON_LOG` for the auto-start environment. Flag
   wins over env when both are set; explicit nullification is
   not supported.

3. **Update `defaultStartServer` in
   `internal/harness/connection.go`** to consult the env var
   (or the configured `cfg.DaemonLogPath` if added) and switch
   between `nil` and a `*os.File` open in append mode.

4. **No behavior change when neither flag nor env is set.** The
   existing `nil` → `/dev/null` redirection remains the default
   to preserve the TERM-corruption fix.

5. **Tests**:
   - `internal/harness/connection_test.go` — fake binary or shim
     that produces stderr; assert the log file contains the
     expected output when env is set; assert no leak when env
     is unset (existing behavior).

## Out of Scope

- **Sub-item 10a (stale-daemon detection).** Defer to v0.3.1;
  needs a protocol addition (binary path on
  `connection.health`) and shell-side warn UX.
- **Sub-item 10c (reliable socket cleanup).** Already largely
  handled by `Server.startSocketListener`'s probe-and-remove
  flow plus PATCH-0021's `Shutdown` integration. No additional
  fix scoped here.
- **Sub-item 10d (`modeltap status` probes daemon).** Defer to
  v0.3.1; needs a CLI rework that connects to the socket.
- **Sub-item 10e (single-terminal mode).** Defer to v0.3.1;
  larger UX change.
- **Sub-item 10f (binary mismatch warning).** Defer to v0.3.1;
  protocol-additive (binary path on `connection.health`).

## Checklist

- [ ] `MODELTAP_DAEMON_LOG` env honored in
  `defaultStartServer` (or the configured equivalent)
- [ ] `--debug-daemon-log` flag added to `modeltap shell`
  (propagates as env)
- [ ] Pre-TUI confirmation line printed when daemon log path is
  set
- [ ] Existing default (`nil` stdio) preserved when neither
  flag nor env is set
- [ ] Tests cover the env-set case (file populated) and the
  env-unset case (no leak / no panic)
- [ ] `go test ./...` passes
- [ ] Smoke verification: launch `modeltap shell
  --debug-daemon-log /tmp/d.log`, observe daemon startup output
  in `/tmp/d.log`
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F4 updated
  to reflect 10b fixed; 10a/10c/10d/10e/10f deferred to v0.3.1
