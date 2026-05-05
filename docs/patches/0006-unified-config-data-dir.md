---
patch: "PATCH-0006"
title: "Unified `~/.modeltap/` Config & Data Directory"
status: "done"
date: "2026-04-20"
related:
  - "PATCH-0004 (secret prefix resolver)"
  - "PATCH-0005 (BFF→proxy routing)"
  - "ADR-0004 (Viper configuration)"
branch: "exploration/integrated-harness"
---

# PATCH-0006: Unified `~/.modeltap/` Config & Data Directory

## Problem

Today modeltap scatters its state across two XDG-style directories plus one quirk:

| Artifact | Current default |
|----------|-----------------|
| Config file | `~/.config/modeltap/config.yaml` |
| SQLite DB | `~/.config/modeltap/modeltap.db` *(XDG-wrong: data under CONFIG)* |
| BFF socket | `~/.local/share/modeltap/server.sock` *(XDG-right)* |
| Service log (macOS) | `~/.config/modeltap/modeltap.log` *(XDG-wrong)* |

Users running `modeltap` for the first time then have to hunt across `~/.config/modeltap/` and `~/.local/share/modeltap/` to understand "where does modeltap keep its stuff." XDG compliance has value on Linux but is largely invisible ergonomics on macOS (where nothing else honors it). The split also produces the real-world surprise of an `XDG_DATA_HOME`-aware socket path but an XDG-ignoring DB path in the same config.

A single `~/.modeltap/` directory:
- One place to look for config, data, and logs.
- One place to back up / migrate / delete.
- Simpler docs (no "XDG_X_HOME or ~/.Y/Z" two-case explanations).

XDG users who explicitly set `$XDG_CONFIG_HOME` or `$XDG_DATA_HOME` still get routed there — the default changes, not the override behavior.

## Scope

1. **New canonical default dir:** `~/.modeltap/`. All four artifacts (config, DB, socket, log) default under it:
   - `~/.modeltap/config.yaml`
   - `~/.modeltap/modeltap.db`
   - `~/.modeltap/server.sock`
   - `~/.modeltap/modeltap.log`
2. **XDG as explicit opt-in:**
   - If `$XDG_CONFIG_HOME` is set, config defaults to `$XDG_CONFIG_HOME/modeltap/config.yaml`.
   - If `$XDG_DATA_HOME` is set, DB / socket / log default under `$XDG_DATA_HOME/modeltap/`.
   - If neither is set, everything goes to `~/.modeltap/`.
3. **Legacy fallback (one release of grace):**
   - Config resolution: if the new default `~/.modeltap/config.yaml` does not exist but `~/.config/modeltap/config.yaml` does, load the legacy path and emit a single-line deprecation notice on stderr. The notice fires exactly once per process.
   - When running in legacy mode, default the DB / socket / log paths to their legacy locations (`~/.config/modeltap/modeltap.db`, `~/.local/share/modeltap/server.sock`, `~/.config/modeltap/modeltap.log`) so the existing install keeps working without edits.
   - Explicit `--config` / `MODELTAP_CONFIG` always wins; no fallback search when the user points at a specific file.
4. **Centralized helpers in `internal/config/`:**
   - `DefaultConfigPath()` returns the resolved path (new default, XDG, or legacy fallback).
   - `DefaultDataDir()` (new) returns `~/.modeltap/` / `$XDG_DATA_HOME/modeltap/` / legacy equivalent.
   - Socket / DB / log helpers compose on top of `DefaultDataDir()`.
5. **Migrate touchpoints:**
   - `internal/config/config.go`: `DefaultConfigDir`, `DefaultConfigPath`, `DefaultBFFSocketPath`, the `db_path` viper default (both Load and LoadWithViper paths).
   - `internal/service/status.go`: `LogFilePath()`.
   - `internal/cli/service.go`: the doc comment on `ServiceLogsCmd`.
   - `docs/sample-config.yaml`: comment header and path examples.
   - Test fixtures with hardcoded legacy paths (`internal/cli/status_test.go`, `internal/service/service_test.go`, `internal/protocol/fixtures/*.json`) updated to the new canonical default for new-install assertions, with a separate test case exercising the legacy-fallback branch.
6. **Dashboard help HTML** (`internal/dashboard/static/help.html`) updated to reference `~/.modeltap/`.

## Out of Scope

- Automatic *file* migration (copying an existing DB / config into `~/.modeltap/`). A user running `modeltap` on an existing install keeps using legacy paths until they move the file themselves. Too much risk to silently move databases.
- Deprecation removal timeline. Legacy fallback stays in for at least one full release cycle (v0.2.0 ships with it, v0.3.0 decides).
- A `modeltap migrate` subcommand. Cheap to add later if demand warrants; not part of this patch.
- Windows path conventions (`%APPDATA%` / `%LOCALAPPDATA%`). Not supported today; this patch doesn't introduce the problem or pretend to solve it.

## Checklist

- [x] `DefaultConfigPath()` resolution: new default, XDG overrides, legacy fallback
- [x] `DefaultDataDir()` helper + `DefaultBFFSocketPath` / DB / log use it
- [x] Legacy-mode detection propagates to DB / socket / log defaults so legacy installs keep working
- [x] One-shot deprecation warning on legacy config load
- [x] Viper `db_path` default updated in Load + LoadWithViper
- [x] `internal/service/status.go` `LogFilePath()` updated (with local `dataDir()` helper to avoid an import cycle)
- [x] `internal/service/launchd.go` `LogDir` template value updated
- [x] `internal/cli/service.go` doc comment updated
- [x] `docs/sample-config.yaml` header + paths updated
- [x] Dashboard help HTML updated
- [x] Fixture JSONs in `internal/protocol/fixtures/` updated to new canonical default
- [x] Tests: new default, XDG overrides, legacy fallback (stderr captured), explicit --config wins
- [x] `go vet ./...` clean, `go test ./...` green

## Fix Detail

The legacy-fallback branch is the only subtle piece. A user with an existing v0.1/v0.2.0-pre install has:

```
~/.config/modeltap/config.yaml      ← they know about this
~/.config/modeltap/modeltap.db      ← contains their capture history
~/.local/share/modeltap/server.sock ← created by `modeltap start`
```

If we naively flip the default, their next `modeltap` run quietly creates a *new* DB at `~/.modeltap/modeltap.db` and their capture history looks "gone" until they realize. So:

- On config load, if `--config` isn't explicit and the new default is absent but the legacy config exists, **all** data-path defaults switch to legacy equivalents. The install keeps working with zero edits.
- The deprecation warning tells them what's going on:
  ```
  modeltap: using legacy config at ~/.config/modeltap/config.yaml
            move to ~/.modeltap/config.yaml to silence this warning
            (and move ~/.config/modeltap/modeltap.db + ~/.local/share/modeltap/server.sock alongside it)
  ```
- Users who set explicit paths in the legacy config (e.g., `db_path: /mnt/data/modeltap.db`) are unaffected — the override wins as it always did.

Precedence (unchanged from today, just explicit):

1. CLI flag
2. `MODELTAP_*` env var
3. Config file value
4. Computed default (new `~/.modeltap/` / XDG override / legacy fallback)
