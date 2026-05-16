# WU-030: Dashboard Security Review

**Date:** 2026-03-08
**Agent:** Security
**Status:** Complete

## Summary

Performed a security review of the dashboard code (`internal/dashboard/`) covering XSS, Content-Security-Policy, CORS, path traversal, sensitive data exposure, and input validation.

## Findings and Fixes

### 1. XSS: Unescaped Values in innerHTML (Fixed)

**Severity:** Medium
**Files:** `internal/dashboard/static/app.js`

Several values were inserted into `innerHTML` without escaping:

- `response_status` in `renderTable()` and `renderDetail()` -- numeric from API but should always be escaped when inserted as HTML.
- `proxy.port` and `retention.days` in `renderStatus()` -- same concern.
- Boolean (`true`/`false`) and `null` values in `highlightJSON()` -- inserted without `escapeHtml()`.

**Fix:** Wrapped all unescaped values with `escapeHtml()` and `String()` conversion where needed.

### 2. Missing Security Headers (Fixed)

**Severity:** High
**Files:** `internal/dashboard/api.go`

No `Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`, or `Referrer-Policy` headers were set on any response.

**Fix:** Added a `securityHeaders()` helper that sets:
- `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

Applied to all API JSON responses (via `writeJSON`), HTML page serving, and the static file server.

### 3. No Maximum Limit on Pagination (Fixed)

**Severity:** Low
**Files:** `internal/dashboard/api.go`

The `limit` query parameter on `/api/logs` accepted any non-negative integer, allowing a client to request an unbounded number of records in a single response, potentially causing denial of service.

**Fix:** Capped `limit` at 1000.

### 4. Path Traversal in Static File Serving (No Issue)

The static files are served from an `embed.FS` which is inherently safe against path traversal. The `fs.Sub` call restricts to the `static/` subdirectory, and Go's `http.FileServer` with `embed.FS` does not allow escaping the embedded filesystem boundary. Additionally, the root handler explicitly checks `r.URL.Path != "/"`.

### 5. CORS Restrictions (No Issue)

The dashboard is served on the same origin as its API endpoints. No `Access-Control-Allow-Origin` headers are set, which means the browser's same-origin policy applies by default. This is the correct secure default -- no cross-origin access is permitted.

### 6. Sensitive Data Exposure (Acceptable)

The `/api/status` endpoint exposes proxy port and upstream URL. This is intentional dashboard functionality. Request/response headers in log detail are expected to have sensitive values (e.g., API keys) redacted by the capture middleware before storage, which is the correct layer for that concern.

### 7. Input Validation (Adequate)

All API query parameters are validated:
- `limit` and `offset` reject non-numeric and negative values.
- `since` and `until` require RFC3339 format.
- `status` requires a numeric value.
- `group_by` is validated against an allowlist.
- `provider` and `model` are passed as string filters without injection risk (parameterized queries in SQLite store).

## Existing Positive Patterns

- `escapeHtml()` helper already used extensively in `app.js` for user-sourced strings.
- `escapeHTML()` helper in `status.js` properly escapes all rendered values.
- `metrics.js` uses `textContent` (via the `el()` helper) rather than `innerHTML` for all data rendering -- the most secure approach.
- `encodeURIComponent()` used for log detail ID in fetch URL.
- `CSS.escape()` used for attribute selector queries.

## Files Modified

- `internal/dashboard/api.go` -- security headers, limit cap
- `internal/dashboard/static/app.js` -- XSS fixes for unescaped values
