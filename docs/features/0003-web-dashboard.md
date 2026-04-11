---
feature: FEAT-0003
title: Web Dashboard
status: accepted
date: 2026-03-06
adr-constraints:
  - ADR-0001: Go as primary language (embed.FS for assets)
  - ADR-0002: SQLite storage (dashboard reads via Store interface)
  - ADR-0007: Pre-computed aggregation tables
---

# FEAT-0003: Web Dashboard

## Problem

Users need a visual interface to browse captured request/response logs, view usage metrics, and monitor proxy activity. CLI-only access is sufficient for power users but creates a barrier for quick inspection, sharing with teammates, and getting an at-a-glance overview.

## Solution

A web dashboard embedded in the modeltap binary that provides a browser-based UI for viewing logs, metrics, and proxy status. Served by the same Go process — no separate frontend build or deployment.

## Key Capabilities

### Log Viewer
- Browse captured requests/responses with filtering (by provider, model, time range, status code)
- Expand individual entries to see full request/response bodies
- Search logs by content
- Syntax highlighting for JSON request/response bodies
- SSE/streaming responses rendered as complete text

### Metrics Dashboard
- Usage over time (requests, tokens, estimated cost)
- Breakdown by provider and model
- Per-user breakdown (when multi-user is enabled)
- Configurable time ranges (today, 7d, 30d, custom)

### Proxy Status
- Current proxy status (running, upstream URL, port)
- Active connections
- Error rate

## Technical Approach

### Embedded UI
- Static assets (HTML, CSS, JS) embedded in the Go binary via `embed.FS`
- No separate frontend build step for users — `go build` produces everything
- Served on a configurable port (default: same port as proxy, under `/dashboard` path, or separate port)

### Frontend Stack
- Lightweight — no heavy framework required for v1
- Server-rendered HTML with minimal JS for interactivity (htmx or similar)
- Or: small SPA with vanilla JS / lightweight framework (preact, alpine.js)
- Responsive design for desktop and tablet

### API Layer
- Internal REST API endpoints serving JSON for dashboard data
- Endpoints: `/api/logs`, `/api/logs/:id`, `/api/metrics`, `/api/status`
- Same SQLite database as CLI — no data duplication
- Pagination for log listing

### Access Control
- Local-only by default (bind to 127.0.0.1)
- Optional: basic auth or token for non-local access
- When multi-user is enabled, dashboard respects user isolation (users see only their data, admins see aggregates)

## CLI Integration

```
modeltap dashboard              # Open dashboard in default browser
modeltap start --dashboard      # Start proxy with dashboard enabled
modeltap start --dashboard-port 8081  # Dashboard on separate port
```

## Configuration

```yaml
dashboard:
  enabled: true
  port: 8081            # or 0 to share proxy port under /dashboard
  bind: 127.0.0.1       # local only by default
  auth:
    enabled: false
    token: ""            # optional access token
```

## Phasing

- **v1:** Log viewer with filtering, basic metrics display, proxy status
- **v2:** Interactive charts, export from UI, real-time log tailing (websocket)
- **v3+:** Knowledge layer search UI, multi-user admin panel

## Relationship to ADRs

- Uses ADR-0002 (SQLite) as data source
- Integrates with ADR-0003 (Cobra) for CLI commands
- Reads ADR-0004 (Viper) configuration
- Displays ADR-0007 (metrics) aggregation data
- Respects multi-user isolation (feature: multi-user-support)
