# 2026-04-21 Session — Root README (PATCH-0009)

## What was discussed

User asked where run/deploy/update instructions live. I inventoried the repo: `Makefile` has `build/test/lint/fmt/vet/clean` (and hard-codes `GO ?= /usr/local/opt/go/bin/go`); `docs/usage-guide.md` is the canonical end-user doc (install, service, config, CLI reference); `CONTRIBUTING.md` covers contributor workflow; CI lives in `.github/workflows/`. Two gaps flagged: no repo-root `README.md`, and no release/update pipeline checked in.

User decided to add a root README and asked whether it was a feature. I recommended a patch rather than a feature — scope is bounded, a checklist defines "done," and there are no personas/success criteria to weigh. User agreed.

## Decisions

- README belongs in `docs/patches/` as PATCH-0009, not `docs/features/`. Patches are implementation-scoped; a docs addition with a clear checklist fits that shape.
- Frame the README around two labels: "Today" (v0.1 shipped + v0.2.0 in development on `exploration/integrated-harness`) and "Direction" (FEAT-0010/0011/0012/0013 as proposed, not shipped). Avoid overclaiming unshipped features.
- Encourage forking explicitly — Apache-2.0 per ADR-0010, and the harness substrate is genuinely useful fork material.
- Cross-link `docs/usage-guide.md` back to the README so readers who land in the guide first find the overview.

## Actions taken

- Drafted `docs/patches/0009-root-readme.md` (status: proposed → approved → done in one session).
- Registered PATCH-0009 in `docs/patches/README.md` index.
- Wrote `README.md` at the repo root.
- Added a pointer at the top of `docs/usage-guide.md` back to the README.
- Updated `docs/releases/v0.2.0/changelog.md` Patches table with PATCH-0009 (and backfilled PATCH-0006 and PATCH-0007 rows, which were missing).
- Commits logically grouped:
  1. `PATCH-0009: add root README and usage-guide cross-link`
  2. `PATCH-0009: mark done; register in changelog`
  3. `ADMIN: session log for 2026-04-21 root README`

## Files created

- `README.md`
- `docs/patches/0009-root-readme.md`
- `docs/history/2026-04-21-session-root-readme.md`

## Files modified

- `docs/patches/README.md` — PATCH-0009 registered, status set to done
- `docs/usage-guide.md` — README pointer at top
- `docs/releases/v0.2.0/changelog.md` — Patches table updated with PATCH-0006, PATCH-0007, PATCH-0009

## What's next / open items

- Badges (CI, license, Go report card) are intentionally deferred — follow-up patch if/when wanted.
- Dashboard screenshots/GIFs deferred — not load-bearing for v1 of the README.
- No release/update pipeline is checked in yet. Separate concern from this patch; raised in the initial session discussion but not addressed.
- `Makefile` hard-codes `GO ?= /usr/local/opt/go/bin/go`. Works for the project owner; may trip up contributors on systems where `go` is on `PATH` at a different location. Not in scope for PATCH-0009.
