# Handoff — Harness Debug & Review (2026-04-21)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

Forward-looking document for the next working session. Purpose: let a fresh Claude context (or a human returning cold) pick up harness debugging and review without re-deriving the layout.

## Goal

Debug and review the v0.2.0 terminal harness — the Bubbletea TUI that launches when `modeltap` is run with no subcommand. Scope is "read the code, run it, identify issues." No implementation commitments yet; that's the outcome of the review.

## Branch & build state

- Branch: `exploration/integrated-harness`. Do not merge to `main` without explicit direction; v0.2.0 is unreleased.
- Working tree is clean as of this handoff (after PATCH-0009 README + PATCH-0010 Makefile commits landed).
- Go toolchain at `/usr/local/go/bin/go` (Go 1.26.1). Makefile uses `GO ?= go`, PATH-resolved (PATCH-0010).
- `make fmt-check vet build` all green. `make lint` unverified — `golangci-lint` not installed on this machine.

## How to launch

```sh
make build                             # produces ./bin/modeltap
./bin/modeltap                         # launches the harness (no subcommand)
./bin/modeltap --resume <session-id>   # resume a prior session
./bin/modeltap harness                 # same as above, explicit subcommand
```

Inside the harness, useful slash commands: `/help`, `/sessions`, `/session {resume|clear|fork}`, `/models`, `/model <name|auto>`, `/mcp status`, `/mcp reconnect <name>`, `/context`, `/plan`, `/build`, `/auto`, `/history {user|project|session}`.

`Ctrl+P` toggles plan↔build mode. Arrow-up/down at the top of the input traverses cross-session command history (served from the BFF).

## Where the code lives

| Area | Path | Entry points |
|------|------|-------------|
| CLI router (no-subcommand → harness) | `internal/cli/root.go` | `NewRootCommand`, `runHarness` |
| Harness app (Bubbletea) | `internal/harness/app.go` | Root model; wire via `app_conn.go` (ConnectionManager surface per PATCH-0003) |
| Connection manager | `internal/harness/connection.go`, `connux.go` | State machine for discovering/starting/registering/reconnecting/failed |
| Protocol client | `internal/harness/client.go` | JSON-RPC client against BFF unix socket |
| Tool dispatcher | `internal/harness/tool_dispatcher.go` | Routes tool-use requests to `internal/harness/tools/` |
| Tools framework | `internal/harness/tools/` | 13 built-in tools: `read.go`, `write.go`, `edit.go`, `bash.go`, `git.go`, `glob.go`, `grep.go`, `webfetch.go`, `websearch.go`, `read_{docx,pdf,xlsx}.go`, plus `framework.go`, `permission.go`, `dangerous.go`, `tracker.go` |
| MCP client | `internal/harness/mcp.go`, `mcp_client.go`, `mcp_tool.go` | Stdio JSON-RPC 2.0 to external MCP servers |
| Paste / viewport / input / statusbar / markdown | `internal/harness/{paste,viewport,input,statusbar,markdown}.go` | UI subcomponents |
| Plan / build mode | `internal/harness/plan.go` | Mode toggle logic |
| Session UI | `internal/harness/sessions.go` | `/sessions` and `/session` handling |
| Model UI | `internal/harness/models.go`, `model.go` | Catalog + override |
| Context / attachments | `internal/harness/context.go` | `@file` resolution, `/context` display |
| History | `internal/harness/history.go` | Cross-session command history client (WU-092) |
| BFF server | `internal/bff/` | JSON-RPC 2.0 over unix socket; one file per concern (`server.go`, `turn.go`, `session.go`, `streaming.go`, `routing.go`, `dispatch.go`, `registry.go`, `providers.go`, `prompt.go`, `cost.go`, `compact.go`, `capabilities.go`, `conversation.go`, `diagnostics.go`, `history.go`, `sync.go`, `transform.go`, `transport.go`) |
| Protocol (wire types) | `internal/protocol/` | Shared JSON message definitions between harness and BFF |
| Providers | `internal/provider/` | `anthropic.go`, `openai.go`, `ollama.go`, plus `registry.go` and `message.go` |

Every production file has a sibling `_test.go`. Tests use the table-driven pattern per `CLAUDE.md`.

## What's already decided / documented

- **FEAT-0008** (`.sdlc/features/0008-bff-server.md`) — BFF server feature spec, **accepted**. Covers protocol framing, session lifecycle, turn dispatch, streaming, routing, capabilities handshake, cost updates.
- **FEAT-0009** (`.sdlc/features/0009-terminal-harness.md`) — terminal harness feature spec, **accepted**. Covers Bubbletea UI, tool framework, permission model, MCP client, `@file` context, session commands.
- **ADR-0013** (`.sdlc/adr/0013-terminal-ui-framework.md`) — Bubbletea chosen over alternatives. Status: proposed (consult before changing framework assumptions).
- **ADR-0009** (`.sdlc/adr/0009-mcp-server-for-knowledge-access.md`) — MCP stdio transport; constrains the harness MCP client shape.
- **PATCH-0003** (`.sdlc/patches/0003-harness-app-conn-mgr-wiring.md`) — `ConnSurface` / `deferredSender` wiring between Bubbletea App and ConnectionManager. Read this before changing how the app talks to the connection.
- **PATCH-0005** — harness provider traffic routed through the local proxy by default. Cloud providers default to `http://127.0.0.1:<port>` unless `host:` is explicitly set.
- **Release status:** `.sdlc/releases/v0.2.0/status.md` — Phase 3 (implementation). All blocking and cross-flow findings resolved. See the "Completed" section for what shipped in each bundle (4–11 on BFF side, 5–7 on harness side, Bundle 13 harness UX).
- **Release changelog:** `.sdlc/releases/v0.2.0/changelog.md` — headline features narrative + artifacts index.

## Known open issues / pending review

- **PATCH-0008 (Moonshot provider adapter)** — draft doc exists; readiness review found **4 blocking + 2 significant issues** (missing BFF endpoint integration, unsupported config fields in examples, missing mode propagation to providers, incomplete Thinking/Instant request behavior). Findings: `.sdlc/patches/.reviews/0008-moonshot-provider-adapter-findings.md`. **Not yet addressed.**
- **WU-094 Medium / Low follow-ups** — 5 Mediums + 19 Lows from the security review remain open. Tracked in `status.md` and `.reviews/security/`.
- **`/drop <file>` and full ContextManager** — per-session attachment bookkeeping (drop, pin, stale detection) isn't implemented. `/context` currently shows the server's view only.
- **Multi-model branching (WU-060)** — explicitly deferred to FEAT-0013. `turn.submit` rejects array routes with a clear error.
- **OpenAI Responses API** — PATCH-0001 proposed; not landed. Codex sends traffic there and currently gets passthrough capture without model/token metadata.
- **FEAT-0010 multi-user** — TLS/mTLS listener + `user_id` schema hooks are in; full user management, auth flow, and session isolation remain.
- **`golangci-lint` not installed locally** — `make lint` will fail on this host; unchanged from baseline.

## Suggested first moves for the debug / review session

1. **Build and launch.** `make build && ./bin/modeltap`. Confirm the harness comes up, connects to the BFF, renders the status bar and input.
2. **Exercise happy paths.** Enter a prompt, watch streaming render. Try `/sessions`, `/model`, `/mcp status`, `@file <path>`, `/plan` toggle.
3. **Read the entry points in order:** `cmd/modeltap/main.go` → `internal/cli/root.go` (`runHarness`) → `internal/harness/app.go` → `internal/harness/connection.go` / `connux.go` → `internal/bff/server.go`. The full conversation flow is worth tracing end-to-end once.
4. **Check the review queue.** Open `.sdlc/patches/.reviews/0008-moonshot-provider-adapter-findings.md` and decide whether to address the blockers now or defer.
5. **Scan test coverage.** `grep -l "^func Test" internal/harness/*_test.go internal/bff/*_test.go` for a list; focus review on untested or thinly-tested paths.

## Conventions to honor

- Per `CLAUDE.md` §Commit Policy: every change traces to a feature / patch / ADR / exploration authoring, or is `ADMIN:`. Harness changes discovered during review that need code would typically be `PATCH-NNNN` for small fixes, `FEAT-0009` amendments (rare) for behavior changes.
- Per `.agents/process.md`: v0.2.0 is Phase 3. Don't do new design work; revise explicitly if implementation reveals a flaw.
- `git commit -s` for DCO sign-off on all commits.
- Do not merge `exploration/integrated-harness` to `main` without explicit direction.

## Today's just-landed work (context for the first 5 minutes)

Last four commits on this branch, most recent first:

1. `ADMIN: log session for 2026-04-21 Makefile hygiene`
2. `PATCH-0010: Makefile hygiene — PATH-resolved Go + check-only default`
3. `ADMIN: apply gofmt -s to internal/` (one-shot cleanup — 45 files, whitespace only)
4. `ADMIN: log session for 2026-04-21 root README`
5. `PATCH-0009: add root README and usage-guide cross-link`
6. `ADMIN: split README quick start into harness and proxy paths` *(this batch, alongside this handoff)*

None of those touched harness or BFF code semantics. Whitespace only in the gofmt commit; tree-surface changes everywhere else.
