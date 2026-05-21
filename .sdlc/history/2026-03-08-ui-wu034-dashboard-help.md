# WU-034: Dashboard Help Page

**Date:** 2026-03-08
**Agent:** UI/Designer
**Status:** Complete

## Summary

Created a help/documentation page for the web dashboard that provides searchable, collapsible help content covering all major modeltap features.

## Changes

### New Files

- `internal/dashboard/static/help.html` -- Help page with five content sections:
  - Quick Start guide
  - CLI Commands Reference (all commands with flag tables)
  - Configuration Options (settings table, example config, env var overrides)
  - Provider Support (Anthropic and OpenAI detection, routing, streaming)
  - Dashboard Usage Tips (enabling, log viewer, metrics, status, theme, time filters)
- `internal/dashboard/static/help.js` -- Client-side search/filter functionality:
  - Debounced text input filters sections by matching text content
  - Shows/hides sections based on search query with result count
  - Collapsible sections with click-to-toggle
  - Escape key clears search
- `internal/dashboard/static/help.css` -- Styling consistent with existing dashboard theme:
  - Card-based section layout matching status page patterns
  - Styled code blocks, tables, and lists
  - Responsive design for mobile viewports
  - Search input with focus styling

### Modified Files

- `internal/dashboard/static/index.html` -- Added "Help" link in the navbar navigation, linking to `/static/help.html`

## Design Decisions

- Used a standalone page pattern (like `status.html` and `metrics.html`) rather than embedding as a view in `index.html`, since help content is static and does not require API calls.
- Navigation in help page includes links back to Logs, Metrics, and Status for consistency.
- Search filtering is client-side only with no external dependencies -- sections are shown/hidden based on text content matching.
- Content is derived from the usage guide (`docs/usage-guide.md`) to ensure consistency between CLI docs and dashboard help.
- Sections are collapsible to let users focus on the topic they need.
- The help page is automatically served by the existing `embed.go` static file embedding (`//go:embed static/*`), requiring no backend changes.
