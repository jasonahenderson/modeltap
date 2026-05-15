---
date: 2026-04-19
topic: Session log — Bundles 7 + 13 complete, PATCH-0003, WU-089 CLI launch
---

# 2026-04-19 — Session: Bundle 7 finish, PATCH-0003, Bundle 13 complete, WU-089

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

Long-running execution session. Entered with Bundle 7 at 3/5 (WU-075 +
WU-077 + WU-078 shipped); exited with Bundle 7 complete, Bundle 13
complete, PATCH-0003 for App↔ConnectionManager wiring, and WU-089 CLI
launch path in. v0.2.0 progress: ~85% (50 of 57 actionable WUs).

## What landed (chronological)

### Bundle 7 finish
| WU | Title | Commit |
|----|-------|--------|
| 079 | Glob + Grep + WebSearch + WebFetch | `a92bba2` |
| 076 | Read (text/CSV/image/PDF/DOCX/XLSX) | `efc2cee` |

Bundle 7 total: WU-075, 077, 078, 079, 076 — all 13 built-in harness
tools live under `internal/harness/tools/`. Dep choices deviated from
the original design for ADR-0010 license compliance: DOCX via stdlib
`archive/zip` + `encoding/xml` (no UniDoc/unioffice), PDF via
`ledongthuc/pdf` (BSD-3). Saved a feedback memory about checking
design-named dep licenses against ADR-0010.

### Bundle 13
| WU | Title | Commit |
|----|-------|--------|
| 086 | Connection UX banner translator | `eee3b9e` |
| 083 | Large paste handler + modal overlay | `818a360` |
| 092 | BFF-sourced command history traversal | `9d7c3ca` |
| 085 | /model + /models | `e880992` |
| 084 | /sessions + /session {resume|clear|fork} | `d359bce` |
| 082 | @file resolution + /context | `c1f8bd1` |
| 080 | /plan /build /auto + PlanAccumulator | `ba2f222` |
| 081 | MCP stdio client + manager + tool adapter | `cbff50f` |

All 8 Bundle 13 WUs complete. Slash-command-first surface: every
user-facing operation lands as a banner. The big items — plan-mode
accumulator, MCP subsystem — are scoped as MVP data structures + RPC
wiring so the surface is usable without yet requiring the full
executor↔conn integration (plan interception) or tool deregistration
(MCP reconnect) work.

### PATCH-0003 — App ↔ ConnectionManager wiring
Commit `0afa222`. The missing plumbing from the original plan.
Introduces narrow `ConnSurface` + `ConnProtocolClient` interfaces so
the App depends only on what it uses. `WrapConnectionManager()`
adapts `*ConnectionManager`. `AppOptions.Conn` / `SetConn` inject.
Wired: `PasteSummarizeRequestMsg` → `content.transform`, `SubmitMsg`
(commands `/status`, `/reconnect`; free-form → `turn.submit`). Every
ConnSurface touchpoint has a graceful no-conn fallback.

### WU-089 — modeltap harness CLI
Commit `d6bf0e0`. `modeltap harness` subcommand composes everything:
ConnectionManager (with AutoStart pointing at `os.Executable()`),
ContextManager mounted at `--project`, App with Conn wired. Flags:
`--socket`, `--resume`, `--project`, `--model`, `--no-auto-start`.
`deferredSender` (atomic.Pointer) breaks the manager-needs-sender /
App-needs-conn construction cycle.

## What shipped for real

- 11 WUs + 1 PATCH landed in this session
- ~20 commits total (WU + ADMIN), all DCO signed
- 5 new test files (Paste, History, Model, Session, Context), 4 new
  MCP files (client, client test, manager, manager test, tool
  adapter), 2 new Plan files, 1 new CLI command file
- Full `go test ./...` clean at each commit point; `go vet` clean

## Deferred / queued follow-ups

- **WU-061 compaction** — blocked on design discussion (trim
  heuristic + harness UX flow). No code until that settles.
- **Plan-mode tool interception** — `PlanAccumulator.Append` has no
  callers yet. Requires the harness tool executor to sit in the
  `tool.call` pathway alongside the event bridge.
- **tools.Registry.Deregister** — MCP reconnect currently relies on
  the dup-guard in the launch goroutine. Proper deregistration is a
  small follow-up in the tools package.
- **Default subcommand** — `modeltap` (no args) still prints help.
  Design D2 of track-integration says the default should launch the
  harness. Small change; left explicit for now.

## Not yet started (integration track)

- WU-067 BFF integration tests
- WU-087 harness integration tests
- WU-088 end-to-end harness → BFF → mock provider
- WU-090 documentation sweep (usage guide, config schema, changelog)
- WU-094 security review (OWASP pass, tool framework + SSRF + paths)
- WU-095 performance benchmarks

These are user-judgment-heavy (scope, breadth, acceptance criteria)
and sensible to plan interactively rather than push on autopilot.

## Dep additions

- `github.com/bmatcuk/doublestar/v4` (Apache 2.0) — Glob tool **
- `github.com/xuri/excelize/v2` (BSD-3) — XLSX reading
- `github.com/ledongthuc/pdf` (BSD-3) — PDF text extraction

DOCX stayed stdlib-only per WU-076 scope deviation.

## Resume prompt

> Read `.sdlc/history/2026-04-19-session-bundles-7-13-and-patch-003.md`
> and `.sdlc/releases/v0.2.0/status.md`. Pick next work from the
> remaining track-integration items (WU-067 / WU-087 / WU-088 / WU-090
> / WU-094 / WU-095) or address WU-061 compaction design. Bundle 7,
> Bundle 13, and CLI launch are all complete.
