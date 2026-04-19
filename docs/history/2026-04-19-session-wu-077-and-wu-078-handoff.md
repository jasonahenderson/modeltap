---
date: 2026-04-19
topic: Handoff — v0.2.0 Bundle 7, 3/5 tool WUs complete
---

# 2026-04-19 — Session Handoff: Bundle 7 tools, mid-flight

## Where we are

**Release:** v0.2.0 (`docs/releases/v0.2.0/`)
**Phase:** 3 — Implementation
**Branch:** `exploration/integrated-harness`
**Current bundle:** Bundle 7 (Tools), **3 of 5 WUs complete**

All five WUs in Bundle 7 parallelize on WU-075. Framework and three
tool-pair WUs have landed; two tool WUs remain.

```
Bundle 7
├── WU-075 Framework + permission model   ✅ 2026-04-18
├── WU-076 Read (text/PDF/DOCX/image/xlsx) ⬜ Large
├── WU-077 Write + Edit                    ✅ 2026-04-19
├── WU-078 Bash + Git                      ✅ 2026-04-19
└── WU-079 Glob + Grep + WebSearch + WebFetch ⬜ Medium
```

## What landed today

| Commit    | Title |
|-----------|-------|
| `00727d5` | ADMIN: session log + status for Bundle 6 and WU-075 (catch-up) |
| `797f278` | WU-077: Write and Edit tools |
| `aedf269` | ADMIN: status — WU-077 complete (2/5) |
| `96ec3b7` | WU-078: Bash and Git tools |
| `a19a9b4` | ADMIN: status — WU-078 complete (3/5) |

Session log: `docs/history/2026-04-19-session-wu-077-and-wu-078.md`.
Catch-up log (Bundle 6 + WU-075): `docs/history/2026-04-19-session-bundle-6-and-wu-075.md`.

## What changed in already-shipped code

Two carry-over fixes to WU-075's surface that came out of wiring
WU-078:

1. **`dangerous.go` push short-flag regex.** The prefix
   `\bpush\s+` greedily consumed the single space before `-f` when
   `-f` was the first argument, so `push -f origin main` didn't
   match. Rewritten to `\bpush\b.*(^|\s)-f(\s|$)`. Paired
   `--force` regex tightened to `\bpush\b.*--force\b` for symmetry.
   All pre-existing dangerous-Git tests still pass.

2. **`permission.go` Check() + alwaysPrompt() refactor.** Added a
   Git-read fast path in the `RiskExecute` branch so `git status`
   / `log` / `diff` auto-allow in every mode. Extracted
   `gitCommandFromInput` helper so the same input-string
   extraction serves both the fast path and the dangerous-prompt
   path. `TestPermissionEnforcer_GitDangerous` (which uses the
   legacy `args[]` input shape) still passes.

## Design deviations (deliberate, conservative)

- **`gitReadCommands` narrowed.** `remote` and `config` were in
  the design's read list but have positional-arg mutations
  (`remote add foo`, `config user.email foo@bar.com`). Both
  removed; first use prompts normally.
- **`positionalMutationSubs`** added for `branch` and `tag`
  — bare `git branch` lists, but `git branch newname` creates.
  Any non-flag positional on these subs classifies as
  `RiskExecute`.

These do not change the wire protocol or the tool catalog; they
only tighten which inputs auto-allow.

## Pre-existing flake (not blocking)

`internal/integration.TestMetricsAggregation` fails on a clean
tree with `SQLITE_BUSY` (timing-sensitive). Verified reproduces
before any of today's changes. Unrelated to Bundle 7.

## Next step options

The remaining Bundle 7 work, in priority-of-impact order:

### Option A — WU-079 (Glob + Grep + WebSearch + WebFetch)

**Size:** Medium. Four tools but each is contained.
**Scope:**
- `glob.go` — `github.com/bmatcuk/doublestar/v4` for `**`, results
  sorted by modification time.
- `grep.go` — stdlib `regexp` + `filepath.WalkDir`, output modes
  (`content` / `files_with_matches` / `count`).
- `websearch.go` — Brave or SerpAPI, chosen by config.
- `webfetch.go` — `net/http` + HTML-to-text. SSRF block
  (loopback / RFC1918 / link-local) already lives in
  `permission.go`'s `alwaysPrompt`.
**New deps:** `doublestar` (small). WebSearch needs an API key
from config — design D13 allows "no key configured → error".
**Done in one session:** likely.

### Option B — WU-076 (Read)

**Size:** Large. Five file types plus auto-detection.
**Scope:**
- Text: `os.ReadFile` + line numbering.
- PDF: `github.com/pdfcpu/pdfcpu`.
- DOCX: `github.com/unidoc/unioffice`.
- Images: base64 + MIME via `net/http.DetectContentType`.
- Spreadsheets: `github.com/xuri/excelize/v2` for XLSX,
  `encoding/csv` for CSV.
**New deps:** three heavy parsers. Worth a dedicated session
because each has its own quirks and test fixtures.
**Done in one session:** possible but tight.

### Recommendation

Do **WU-079 next** (Option A). Smaller scope, keeps Bundle 7
progressing, and finishes the "simple tools" set in one session.
WU-076 deserves its own session for the dep audit and fixture
management.

## Outside Bundle 7 (unchanged from previous handoff)

- **App ↔ ConnectionManager wiring** — the App already handles
  `ConnStateMsg` / `StreamTokenMsg` / etc., but nothing
  instantiates the `ConnectionManager` on startup. Probably a
  small WU or part of Bundle 15 integration.
- **WU-061 compaction** — still waiting on the trim-heuristic +
  harness-UX design pass.
- **WU-067** BFF integration tests — now easy to write with
  `internal/harness.ProtocolClient` as a test driver.
- **WU-087** harness integration tests — now easy to write
  against a mock BFF using the Protocol client / ConnectionManager
  already in place.
- **WU-060** multi-model branching: deferred → FEAT-0013
  sub-agents.

## Good resume prompt

> Read `docs/history/2026-04-19-session-wu-077-and-wu-078-handoff.md`
> and continue v0.2.0 Bundle 7. Pick WU-079 (Glob + Grep +
> WebSearch + WebFetch) unless there's a reason to go WU-076 Read
> first. Design reference:
> `docs/releases/v0.2.0/designs/2026-04-16-design-tool-framework-075-076-077-078-079.md`.

## Files to check first on resume

- `docs/releases/v0.2.0/status.md` — authoritative progress
- `internal/harness/tools/` — existing tools + framework
- `docs/releases/v0.2.0/designs/2026-04-16-design-tool-framework-075-076-077-078-079.md` — Bundle 7 design (WU-079 spec is D11–D14; test list lines 738+)
- `internal/harness/tools/permission.go` — see if you need to add
  new `ToolNameX` constants for WU-079 tools
