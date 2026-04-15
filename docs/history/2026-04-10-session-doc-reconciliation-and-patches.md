# 2026-04-10 — Doc Reconciliation and Patches Doc Type

## Summary

Reconciled the modeltap docs structure to align with conventions adopted from the meetingplaceai project: numbered files, identifier prefixes in titles, YAML frontmatter, README index files for each doc type, and `.reviews/` subdirectories with their own READMEs. Introduced a new **patches** doc type for implementation-scoped work, parallel to features and ADRs. Wrote the first patch (PATCH-0001) authorizing OpenAI Responses API support — held for review, not yet implemented.

## Context

Earlier in the day, the user asked how to wire modeltap into Claude Code, Codex, and OpenCode. The usage guide was missing this. While documenting it, a real gap surfaced: Codex sends some traffic to OpenAI's Responses API (`/v1/responses`), which modeltap's v1 OpenAI adapter doesn't detect or parse. Codex traffic still flows through but lands with empty model/token metadata.

The user asked to:

1. Write up the OpenAI Responses API gap as a patch
2. Create a new patches doc type in `docs/`
3. Reference the meetingplaceai project's conventions for ADRs, features, and patches (doc structure, front matter, file naming, `.reviews` handling)
4. Keep modeltap's existing folder layout
5. Reconcile the modeltap project's existing ADRs and features to those conventions

## What Was Done

### Patches doc type (new)

- Created `docs/patches/` with a `README.md` describing when to use a patch vs. a feature spec or ADR, the file-naming convention (`NNNN-title.md`), the `PATCH-NNNN` identifier convention, the inline bold-key metadata format, the lifecycle (proposed → approved → done), and the relationship to other doc types.
- Created `docs/patches/.reviews/README.md` describing the canonical findings layout (`{stem}-findings.md` + `{stem}-findings.json`), the `plan-reviews/` and `syntheses/` subdirs, and the findings JSON schema.
- Wrote `docs/patches/0001-openai-responses-api-support.md` (PATCH-0001) authorizing the OpenAI Responses API extension. Status `proposed`. Includes problem statement, scope (detection by body shape, request/response/streaming parsing), out-of-scope items (tool calls, conversation chaining, Assistants API, Realtime API), checklist of testable deliverables, and a Fix Detail section explaining the body-shape dispatch design and why no `Provider` interface change is needed.

### ADRs (light reconciliation — already mostly aligned)

- Added `docs/adr/README.md` with a current-architecture summary, ADR index table, when-to-write/not-to-write guidance, naming convention, format template, lifecycle, and relationship table.
- Added `docs/adr/.reviews/README.md` with canonical findings layout, findings JSON schema, and when-to-review guidance.
- Edited every ADR (`0001` through `0012`) to add the `ADR-NNNN:` prefix to the H1 title heading. The existing YAML frontmatter (`status`, `date`, `decision-makers`) was already in place and was not changed.

### Features (bigger reshape)

- Renamed all six feature files to use the `NNNN-title.md` convention, in chronological order of git creation:
  - `knowledge-layer.md` → `0001-knowledge-layer.md`
  - `multi-user-support.md` → `0002-multi-user-support.md`
  - `web-dashboard.md` → `0003-web-dashboard.md`
  - `service-management.md` → `0004-service-management.md`
  - `apprenticeship.md` → `0005-apprenticeship.md` (was untracked)
  - `local-inference.md` → `0006-local-inference.md` (was untracked)
- Used `git mv` for the four tracked files to preserve history; `mv` for the two untracked drafts.
- Added YAML frontmatter to every feature file: `feature`, `title`, `status`, `date`, `adr-constraints`. Status values:
  - 0001 knowledge-layer: `proposed` (was no status — interpreted as a forward-looking design sketch)
  - 0002 multi-user-support: `proposed` (same)
  - 0003 web-dashboard: `accepted` (preserved)
  - 0004 service-management: `accepted` (preserved)
  - 0005 apprenticeship: `draft` (preserved)
  - 0006 local-inference: `proposed` (preserved)
- Added `FEAT-NNNN:` prefix to the H1 of every feature.
- Created `docs/features/README.md` with an index table of all features, when-to-write guidance, naming convention, YAML frontmatter format, lifecycle, commit convention, and promotion path from patches.
- Created `docs/features/.reviews/README.md` mirroring the patches `.reviews` README.

### Cross-references and instructions

- Updated `docs/agents.md` line 85 to reference `docs/features/0003-web-dashboard.md` (the only live cross-reference to a feature filename outside of frozen history logs).
- Updated `CLAUDE.md` Key References section to point at the new README files and to add `docs/patches/` as a referenced location.
- Added a new "Doc Type Taxonomy" table to `CLAUDE.md` showing the four doc types (ADR, feature spec, patch, work unit) with their scope, location, and identifier convention.
- Did not touch `docs/history/` log files — they are frozen records.

## Files Created

- `docs/adr/README.md`
- `docs/adr/.reviews/README.md`
- `docs/features/README.md`
- `docs/features/.reviews/README.md`
- `docs/patches/README.md`
- `docs/patches/.reviews/README.md`
- `docs/patches/0001-openai-responses-api-support.md`
- `docs/history/2026-04-10-session-doc-reconciliation-and-patches.md` (this log)

## Files Renamed

- `docs/features/knowledge-layer.md` → `docs/features/0001-knowledge-layer.md`
- `docs/features/multi-user-support.md` → `docs/features/0002-multi-user-support.md`
- `docs/features/web-dashboard.md` → `docs/features/0003-web-dashboard.md`
- `docs/features/service-management.md` → `docs/features/0004-service-management.md`
- `docs/features/apprenticeship.md` → `docs/features/0005-apprenticeship.md`
- `docs/features/local-inference.md` → `docs/features/0006-local-inference.md`

## Files Modified

- `CLAUDE.md` — added doc taxonomy table, updated Key References
- `docs/agents.md` — fixed feature filename reference
- `docs/adr/0001-programming-language.md` through `docs/adr/0012-background-execution-strategy.md` — added `ADR-NNNN:` prefix to H1
- `docs/features/0001-knowledge-layer.md` — added YAML frontmatter, `FEAT-0001:` H1 prefix
- `docs/features/0002-multi-user-support.md` — added YAML frontmatter, `FEAT-0002:` H1 prefix
- `docs/features/0003-web-dashboard.md` — added YAML frontmatter (replacing bare `status:` line), `FEAT-0003:` H1 prefix
- `docs/features/0004-service-management.md` — added YAML frontmatter, `FEAT-0004:` H1 prefix
- `docs/features/0005-apprenticeship.md` — added YAML frontmatter, `FEAT-0005:` H1 prefix
- `docs/features/0006-local-inference.md` — added YAML frontmatter, `FEAT-0006:` H1 prefix
- `docs/usage-guide.md` — earlier in the session: added "Connecting AI Coding Tools" section for Claude Code, Codex, OpenCode

## Held for Review

- **PATCH-0001 (OpenAI Responses API Support)** — status `proposed`, awaiting human review before implementation. The patch design (body-shape dispatch inside the existing OpenAI adapter, no Provider interface change, reuses StreamChunk.EventType for typed SSE events) is the main thing to validate before code lands.

## What's Next / Open Items

1. Review PATCH-0001 and either accept it (status → `approved`) so implementation can begin, or push back on the design.
2. Decide whether the no-status features (FEAT-0001 knowledge-layer, FEAT-0002 multi-user-support) should stay at `proposed` or move to `draft`. They were marked `proposed` in this reconciliation but neither has been formally evaluated against the ADR baseline.
3. Decide whether the four-digit numbering convention should also retroactively apply to existing work units (`WU-NNN` are three-digit). Out of scope for this session — work units are tracked in `status.md` and renaming would invalidate every existing commit message reference.
4. Consider whether `docs/features/0001-knowledge-layer.md` and `docs/features/0006-local-inference.md` need formal `.reviews/` artifacts before any of their content drives implementation work.
5. The earlier conversation flagged that the env-var/config behavior described for Claude Code, Codex, and OpenCode in the usage guide was stated from memory and not verified against current versions of those tools. Worth a spot-check before the next release.
