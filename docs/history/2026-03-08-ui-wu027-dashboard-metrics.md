# WU-027: Dashboard Frontend — Metrics Display

**Date:** 2026-03-08
**Agent:** UI Implementer
**Status:** Complete

## Summary

Implemented the Metrics view for the modeltap web dashboard, integrating with the existing dashboard structure created by WU-026.

## Files Created

- `internal/dashboard/static/metrics.js` — Metrics view JavaScript (~260 lines)
- `internal/dashboard/static/metrics.css` — Metrics-specific styles (~210 lines)
- `internal/dashboard/static/metrics.html` — Standalone fallback page (for use if index.html is unavailable)

## Files Modified

- `internal/dashboard/static/index.html` — Added metrics.css stylesheet link, replaced metrics placeholder with `metrics-view` container div, added metrics.js script tag
- `internal/dashboard/static/app.js` — Added `metricsView.init()` call when metrics tab is activated via navigation

## Implementation Details

### metrics.js
- IIFE module exposing `window.metricsView` with `init()`, `render()`, and `refresh()` methods
- Fetches `/api/metrics` with query parameters matching the API contract (group_by, since, until)
- **Time range selector:** Today, Last 7 Days, Last 30 Days, Custom Range (with date inputs)
- **Group-by selector:** Day, Hour, Provider, Model
- **Summary cards:** Total Requests, Total Tokens, Total Cost, Avg Latency (weighted average)
- **Bar chart:** CSS-only proportional-width div bars, metric selector buttons (Requests, Tokens, Cost, Latency), limited to 20 bars
- **Data table:** Period, Provider, Model, Requests, Input Tokens, Output Tokens, Cost, Avg Latency, Errors
- Number formatting: commas, currency ($), milliseconds/seconds, K/M suffixes for tokens
- Loading spinner, empty state, and error state handling

### metrics.css
- Uses CSS custom properties from style.css (`--bg`, `--bg-secondary`, `--text`, `--text-muted`, `--border`, `--accent`, `--radius`, `--shadow`, `--row-alt`, `--row-hover`, `--success`, `--danger`) with fallback values
- Dark mode compatible through inherited CSS variables
- Responsive: controls stack on mobile, summary cards collapse to 2-column then 1-column
- Tabular numeric alignment throughout

### Integration with WU-026
- metrics.js is loaded before app.js so `window.metricsView` is available when navigation triggers
- app.js calls `metricsView.init()` when user clicks Metrics tab, which fetches data and renders
- Auto-init on DOMContentLoaded for standalone metrics.html usage

## Verification

- `go build ./...` passes successfully
- No external dependencies (no chart.js or other libraries)
- API field names match `metricsJSON` struct in `internal/dashboard/api.go`
