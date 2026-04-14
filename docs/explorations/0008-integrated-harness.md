---
exploration: EXP-0008
title: Integrated Harness — Modeltap as Professional AI Environment
status: exploring
date: 2026-04-14
related:
  - EXP-0001: Knowledge layer is the harness's memory backbone
  - EXP-0007: Orchestration becomes the harness's execution strategy
  - EXP-0002: Multi-user and enterprise auth (prerequisite)
  - EXP-0005: Apprenticeship becomes a harness capture mode
  - ADR-0006: Provider adapters power model routing
  - ADR-0008: sqlite-vec powers knowledge retrieval (amendment proposed — see ADR Amendments)
  - ADR-0009: MCP stdio transport for external knowledge access (scope clarified — see Interface Separation)
  - ADR-0012: Service management for background server
  - ADR-0013: Terminal UI framework (proposed — phased minimal → Bubbletea)
  - PATCH-0002: Local inference as first-class routing targets
---

# EXP-0008: Integrated Harness — Modeltap as Professional AI Environment

## Context

Modeltap was originally conceived as infrastructure — a reverse proxy that sits between AI clients and model providers, passively capturing traffic and computing metrics. The implicit product pitch was "add this to your stack and get observability for free." But this framing has a structural weakness: it positions modeltap as a thing users wire up and forget about, competing with every other observability tool on features they care about more.

Meanwhile, the most compelling capabilities modeltap is building — cross-model memory (EXP-0001), cost-aware routing, context curation, multi-model orchestration (EXP-0007) — are not passive infrastructure features. They are *active intelligence* features. They change how users interact with models, not just how they observe those interactions.

The reframe: modeltap is not a proxy that happens to have a knowledge layer. Modeltap is an **integrated professional AI environment** — a harness and its server, shipped as one product. The server is a BFF (backend for frontend) that provides intelligence, capture, and enterprise features. The harness is the user-facing interface. They are inseparable.

Claude Code's recent open-source release provides a concrete architectural reference for what a serious terminal AI harness looks like: tool use loops, permission models, context compression, streaming UI, MCP extensibility. Modeltap can learn from that architecture while building something fundamentally different — an environment where the intelligence lives in the middleware, not in the client, and where the architecture supports any professional workflow, not just coding.

## Problem

### The Observability Trap

A standalone proxy competes on logging, metrics, and dashboards. These are commodity features. Every cloud provider offers them. The proxy is necessary plumbing, but it is not a product by itself.

### The Clone Trap

Building "another Claude Code" competes on terminal UI polish, tool breadth, and model-specific optimizations. Claude Code has a 512,000-line head start, a dedicated team, and intimate knowledge of their own API. A clone cannot win on those terms.

### The Integration Tax

Keeping the proxy and harness as independent products means two configuration stories, two installation steps, conditional logic everywhere, and users who never discover the full value because they only installed one piece.

### The Actual Opportunity

No existing tool owns the space between the user and *all their models*. Claude Code is deeply coupled to Anthropic. Cursor is an IDE. Aider is git-focused. Every tool assumes a single provider or a single workflow. None of them have:

- Cross-model memory that survives provider switches
- Cost-aware routing that picks the right model for each subtask
- A knowledge layer that makes every conversation smarter based on everything that came before
- Orchestration that can decompose work across models transparently
- Enterprise features: multi-user isolation, centralized credentials, spend governance
- Domain extensibility beyond coding

Modeltap can own this space because it already sits on the traffic boundary. The harness makes that position visible and interactive.

## Design: The Integrated Architecture

### Harness and Server: Separate Processes, One Product

The harness and server are always separate processes. What changes between deployment profiles is where the server runs and how the harness connects. See EXP-0002 for the full deployment profile specification.

The modeltap binary contains both the harness and server code — one compiled artifact for distribution. But they run as separate processes in all profiles:

- **Solo developer**: server runs as a background service or auto-started subprocess on the developer's machine. Harness connects via Unix domain socket. Identity is OS-attested.
- **Team**: server runs on shared infrastructure. Harnesses connect over TLS. Identity via OIDC or tokens.
- **Enterprise**: server is managed infrastructure. Harnesses connect over mTLS or TLS + OIDC. Identity via SPIFFE/SPIRE or enterprise SSO.

The solo developer experience hides the separation behind auto-start — `modeltap` launches and everything works. But the architecture does not change across profiles.

```
Developer machine                          Server (local or remote)
┌──────────────────┐                      ┌──────────────────────────┐
│     Harness      │                      │     Modeltap Server      │
│                  │    modeltap          │                          │
│  - Terminal UI   │    protocol          │  - Provider Router       │
│  - Tool exec     │◄──────────────────►  │  - Capture & Storage     │
│  - Permissions   │    (socket or TLS)   │  - Knowledge Layer       │
│  - MCP client    │                      │  - Metrics & Aggregation │
│  - File handling │                      │  - Session State         │
│                  │                      │  - Auth & Isolation      │
│  No API keys     │                      │  - System Prompt Engine  │
│  No provider     │                      │  - Orchestration         │
│  knowledge       │                      │                          │
└────────┬─────────┘                      └────────────┬─────────────┘
         │                                             │
         │ local tool execution                        │ model API calls
         ▼                                             ▼
   ┌───────────┐                             ┌───────────────────┐
   │ Filesystem │                             │ Anthropic / OpenAI │
   │ Shell      │                             │ Ollama / MLX       │
   │ Git        │                             │ Ollama Cloud       │
   │ MCP servers│                             │ Future providers   │
   └───────────┘                             └───────────────────┘
```

### The BFF Contract

The server serves as a Backend for Frontend with a clear contract:

**What the server (BFF) owns:**
- Provider abstraction and routing — the harness never speaks provider-specific protocols
- Provider API credentials — developers never see or hold model API keys
- Request/response capture and storage
- Knowledge layer — embedding, semantic search, context curation
- Metrics computation and aggregation
- Session state — conversation history, compaction, portable context
- Model selection policy — which model handles which request
- Cost tracking, token accounting, and spend budgets
- Orchestration — task decomposition and multi-model execution (EXP-0007)
- System prompt injection — behavioral specification per domain
- User identity and isolation (EXP-0002)
- Forwarding model tool calls to the harness — the BFF proposes actions but never executes them locally

**What the harness owns:**
- Terminal rendering and user input
- Local tool execution (file read/write, shell commands, git, search) — the harness is the **sole authority** for all local operations
- Permission enforcement — the harness decides whether to execute, prompt, or reject every tool call
- Execution mode (plan/build/auto) — harness-local, not server-controlled
- MCP client connections to external tool servers
- Streaming display of model responses
- File format handling (PDF, DOCX, images, spreadsheets)
- User preferences and local configuration

**What flows between them:**
- The harness sends a conversation turn (user message + tool results + attached files) to the BFF
- The BFF enriches the turn with knowledge context, routes to a model, captures everything
- The BFF streams the model response back to the harness
- If the response contains tool calls, the harness executes them locally and sends results back
- The cycle repeats until the model produces a final response

This is a clean separation: the harness does I/O, the BFF does intelligence.

## The Conversation Loop in Detail

Understanding the full lifecycle of a single user turn reveals where modeltap's value accumulates:

```
User types a message
        │
        ▼
┌─────────────────────────────┐
│  1. Harness receives input  │
│     - Detects file          │
│       attachments (@file,   │
│       drag-drop paths)      │
│     - Detects large paste   │
│       (offer summarize/     │
│       truncate/full)        │
│     - Sends turn to BFF     │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  2. BFF receives turn           │
│     - Checks knowledge layer    │
│     - Retrieves relevant prior  │
│       context (semantic search) │
│     - Checks pinned state       │
│     - Evaluates context budget  │
│     - May compact or summarize  │
│       older conversation turns  │
│     - Injects system prompt     │
│       (domain-specific)         │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  3. BFF selects model           │
│     - Applies routing policy    │
│     - Considers cost, latency,  │
│       capability requirements   │
│     - Checks user's model       │
│       access and budget         │
│     - May decompose into        │
│       subtasks (EXP-0007)       │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  4. BFF sends to provider       │
│     - Enriched prompt with      │
│       knowledge context         │
│     - Provider-specific format  │
│     - Streams response back     │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  5. BFF captures response       │
│     - Full request/response log │
│     - Token/cost accounting     │
│     - Async embedding queue     │
│     - Metadata extraction       │
└──────────┬──────────────────────┘
           │
           ▼
┌─────────────────────────────────┐
│  6. Harness renders response    │
│     - Streams tokens to terminal│
│     - Status bar: model, cost,  │
│       context %, timer          │
│     - If tool calls present:    │
│       - Plan mode: show plan,   │
│         await approval          │
│       - Build mode: check       │
│         permissions, execute    │
│       - Send results to BFF     │
│       - Go to step 2            │
│     - If final response:        │
│       - Display and wait for    │
│         next user input         │
└─────────────────────────────────┘
```

Every turn through this loop enriches the knowledge base. The 50th conversation is smarter than the 1st because the BFF has 49 prior conversations to draw on.

## Harness Capabilities

### File Context Management

Users work with files constantly. The harness must make file context effortless:

**Attachment methods:**
- `@path/to/file` syntax in message input
- `@src/**/*.go` glob patterns for multiple files
- Drag and drop from file manager (terminal emits path on drop)
- Automatic detection of file paths pasted in a burst

**What happens on attachment:**
- Harness reads the file and includes content in the turn sent to the BFF
- BFF token-counts attachments and warns if they'll exceed the context budget
- BFF tracks which files are "in context" across turns — attached once, relevant until dropped or aged out
- BFF cross-references with knowledge layer: "you discussed this file 3 days ago, here's what was decided"

**Context management commands:**

```
> /context
Files in context:
  api.go (3.2 KB, attached turn 2)
  handler.go (1.8 KB, attached turn 2)
  types.go (0.9 KB, auto-included by model)

Knowledge injections:
  Decision: use middleware pattern for auth (2026-04-10)

Total context: 47% of window (38K / 80K tokens)

> /drop handler.go
Removed handler.go from context.
```

**File format support:**
- **Text files**: direct inclusion (Go, Python, JS, YAML, JSON, Markdown, etc.)
- **PDF**: text extraction via Go PDF libraries (pdfcpu, unipdf). Multi-page support. Include as text context or as structured content for vision-capable models.
- **DOCX**: text extraction via Go Office libraries (unioffice). Preserve headings and structure.
- **Images**: base64-encode and include for vision-capable models. BFF routes to a vision model if the current model doesn't support images.
- **Spreadsheets (XLSX/CSV)**: parse structure, include as formatted text or structured data. Critical for finance/accounting workflows.

### Large Paste Handling

When a user pastes more than a configurable threshold (default: 2KB) in a single burst:

```
Large paste detected (347 lines, 14.2 KB)
  1  ERROR: connection refused to database at 10.0.1.45:5432
  2  Stack trace:
  3    at db.Connect (internal/storage/db.go:47)
  4    at proxy.handleRequest (internal/proxy/handler.go:123)
  5    at http.ListenAndServe (net/http/server.go:3285)
  ... 342 more lines

[s]ummarize  [f]ull  [t]runcate (first 50 lines)  [c]ancel
```

If the user chooses "summarize," the BFF routes to a cheap/fast model (local Llama, Haiku-class) for summarization before including it in the main model's context. The full paste is stored in capture for later retrieval, but the live context gets a distilled version. This is a concrete example of cost-aware routing happening transparently for the user.

### Input Editing

The harness provides capable text input:

- **Multi-line composing**: Enter for newline, configurable submit key (Ctrl+Enter, Esc→Enter, or similar)
- **Cursor movement**: arrow keys, Home/End, Ctrl+A/E (emacs bindings), Ctrl+arrows for word-jump
- **Cut/copy/paste**: Ctrl+C/V/X with terminal clipboard integration (OSC 52)
- **Command history**: up/down arrows traverse previous messages. In modeltap, history comes from the BFF — not just this session, but previous sessions for this project.
- **Undo**: Ctrl+Z within the input buffer
- **Multi-line paste detection**: large pastes trigger the summarize/truncate flow rather than submitting on first newline

### Tool Set

Tools are divided between harness-local execution and BFF-mediated capabilities:

**Harness-local tools** (execute on the developer's machine):

| Tool | Description | Permission |
|------|-------------|-----------|
| Read | Read file contents (text, PDF, DOCX, images) | Auto-allow |
| Write | Create new files | Prompt first use |
| Edit | Modify existing files (exact-match diff) | Prompt first use |
| Bash | Execute shell commands | Prompt per command |
| Glob | Find files by pattern | Auto-allow |
| Grep | Search file contents | Auto-allow |
| Git | Git operations | Prompt for mutations |
| WebSearch | Search the web | Prompt first use |
| WebFetch | Fetch URL contents | Prompt per domain |

**BFF-mediated tools** (data lives on server):

| Tool | Description |
|------|-------------|
| KnowledgeSearch | Semantic search over prior interactions |
| CostReport | Token spend, budget utilization |
| ModelSwitch | Change routing target |
| SessionExport | Export session as portable context |

**MCP-discovered tools**: additional tools from configured MCP servers appear automatically. The BFF captures all MCP tool invocations for the knowledge layer. Domain-specific tools (legal, finance, healthcare) are delivered as MCP server packages — see Domain Extensibility below.

**Tool definition quality matters.** Each tool's description includes not just what it does, but when to use it and when NOT to. The Edit tool requires exact string matching (preventing hallucinated file overwrites). The Bash tool description tells the model to prefer dedicated tools over shell commands. The Read tool must be called before Edit (enforced by the harness, not just the prompt). These guardrails prevent common failure modes and are a primary driver of session quality.

### Plan and Build Modes

The harness supports explicit execution modes that control how the **harness** handles tool calls it receives from the BFF. Mode selection is a harness-local setting — the harness is the sole authority for whether a tool call executes or is presented for review.

**Execution boundary principle**: The BFF proposes tool actions (by forwarding the model's tool calls to the harness). The harness decides whether to execute, present for approval, or collect into a plan. The BFF never makes execution decisions about local operations — it does not know or enforce the current mode. This ensures the trust boundary for local actions stays on the machine that will be affected.

**Plan mode** (`/plan`): The harness collects tool calls from the BFF and presents them as a structured plan instead of executing:

```
> /plan
Plan mode active. Model will propose, not execute.

> Refactor the auth middleware to use JWT instead of sessions.

Plan:
  1. Read internal/middleware/auth.go
  2. Read internal/config/config.go
  3. Edit internal/middleware/auth.go
     (replace session logic with JWT validation)
  4. Edit internal/config/config.go
     (add JWT secret and expiry config)
  5. Write internal/middleware/auth_test.go
  6. Bash: go test ./internal/middleware/...

Estimated: ~3 files modified, ~1 new file
[a]pprove all  [s]tep through  [e]dit plan  [c]ancel
```

"Step through" executes one step at a time, letting the user inspect results and approve the next. The harness drives this loop — it sends each approved tool result back to the BFF, which forwards it to the model for the next step.

**Build mode** (`/build` or default): The harness executes tool calls as they arrive, subject to the permission level (default, accept-edits, autonomous). The model reads, writes, runs, and the harness shows progress.

**Auto mode** (`/auto`): The harness auto-approves tool calls within the configured permission level, without per-action prompts. The BFF still captures everything for audit.

Plan vs. build is a harness-local mode that controls the harness's response to tool calls. The BFF is unaware of the mode — it always forwards tool calls the same way. The harness decides what to do with them.

### Context Size Tracking and Call Timer

The status bar displays persistent session metadata:

```
claude-opus-4-6 │ 47% context (38K/80K) │ $0.42 session │ ⏱ 3.2s
```

- **Model name**: currently selected model
- **Context usage**: percentage and absolute tokens, updated per turn
- **Session cost**: running total
- **Timer**: elapsed time for the current model call (starts on send, stops on stream complete)

**Per-turn metadata** (inline after each response):

```
─── claude-opus-4-6 │ 1,247 in / 3,891 out │ $0.08 │ 4.1s ───
```

**Context pressure warnings:**

```
⚠ Context at 78% — consider /compact to free space
```

```
⚠ Context at 92% — auto-compacting older turns
(7 turns summarized, freed 22K tokens, full history in knowledge layer)
```

All data comes from the BFF. The harness renders it but computes none of it. The BFF knows exact token counts (from provider response headers), cost (from pricing tables), context window size (from model metadata), and compaction state.

## Session Quality Stack

Session quality — the feeling of working with a competent collaborator rather than a chatbot — is not one feature. It is a stack of behaviors that compound. This section captures the design principles that drive session quality, informed by what makes the best existing tools (particularly Claude Code) feel natural.

### The System Prompt Is the Product

The single biggest driver of session quality is the behavioral specification injected as the system prompt. This is not "you are a helpful assistant." It is a detailed methodology document that shapes every interaction.

**What the system prompt encodes:**

Engineering methodology:
- Read files before editing them. Do not propose changes to code you haven't read.
- Try the simplest approach first. Don't over-engineer.
- If an approach fails, diagnose why before switching tactics.
- Don't add features, refactor code, or make improvements beyond what was asked.

Negative instructions (prevent over-helpfulness):
- Don't add docstrings, comments, or type annotations to code you didn't change.
- Don't create helpers or abstractions for one-time operations.
- Don't add error handling for scenarios that can't happen.

Output discipline:
- Keep text output brief and direct. Lead with the answer, not the reasoning.
- If you can say it in one sentence, don't use three.

**For modeltap:** The system prompt is configuration, not code. The BFF injects it per-domain (see Domain Extensibility). It works with any capable model. Investing weeks in system prompt engineering yields returns on every session with every user.

### The Autonomy-to-Confirmation Ratio

The permission model determines whether the harness feels like a collaborator or a chatbot that asks for permission to think.

The right ratio: ~80% silent autonomy, ~20% confirmation.

- **Silent (auto-allow)**: Read files, search code, check git status, query knowledge layer. Zero risk, essential for quality. The model does homework without asking.
- **Prompt first use**: Write files, create files. The user approves the pattern once, then it flows.
- **Prompt per action**: Bash commands (especially destructive ones), git push, external API calls.

This gradient matches how a pair programmer behaves: they look at your code without asking, but they don't push to main without saying something. The default permission mode must get this right.

### Proactive Context Gathering

Before answering, the model reads relevant files, checks git state, searches for related code. This proactivity is what makes it feel like it "understands" the codebase.

**Modeltap amplifies this.** Before the model even sees the user's message, the BFF:
- Searches the knowledge layer for relevant prior context
- Injects recent decisions and pinned state
- Includes project metadata (current branch, recent commits, test status)

The model starts every turn with richer context than any single-provider tool can offer. It knows what you decided yesterday, with a different model, in a different session.

### Streaming with Tool Interleaving

The response streams token by token. When the model decides to read a file, you see the tool call happen, see the result, and the response continues. It's watching someone work in real time, not waiting for a batch result.

This interleaving makes the wait feel productive and lets the user interrupt if the model is going the wrong direction.

The internal streaming protocol supports this natively:

```
BFF → Harness stream events:

TokenDelta     { text: string }           // incremental text
ToolCall       { id, name, input }        // model wants to use a tool
ToolResult     { id, output, error }      // tool execution completed
StatusUpdate   { phase, detail }          // "routing to claude-opus-4-6..."
KnowledgeHit   { summary, relevance }     // context injected from knowledge
CostUpdate     { tokens, cost, model }    // running cost for this turn
CompactNotice  { compressed, retained }   // context was compacted
TurnComplete   { usage, model, latency }  // final turn metadata
```

### Error Recovery Without Panic

When the model's first approach fails (test doesn't pass, command errors, file doesn't exist), it adapts: reads the error, adjusts, tries again. It doesn't give up after one failure.

The BFF helps: when a tool call fails, the BFF can include relevant context from the knowledge layer ("you encountered a similar error last week, here's what worked"). The model gets not just the error, but historical resolution context.

### Lossless Compaction

Claude Code's compaction is lossy — compressed content is a summary, the original is gone. Modeltap's compaction is backed by the knowledge layer:

1. The BFF identifies low-value segments in the live context
2. Summarizes them for the active window
3. Retains the full original in the knowledge layer
4. Re-retrieves compressed segments if they become relevant again

Compaction in modeltap is reversible. The user never loses access to anything — it just moves from working memory to searchable long-term memory.

### Cross-Model Memory

The knowledge layer spans every conversation with every model, every provider, every project. "What did I decide about authentication?" works regardless of which model the decision was made with.

| Dimension | Claude Code | Modeltap |
|-----------|------------|----------|
| Memory scope | This project, this tool | All projects, all models, all sessions |
| Memory structure | Flat markdown files | Semantic embeddings + metadata |
| Recall method | File read (exact match) | Semantic search (meaning match) |
| Cross-model | No | Yes |
| Survives model switch | No | Yes |
| Automatic | Partial (auto-memory) | Full (knowledge layer captures everything) |

### Invisible Scaffolding

The BFF handles complexity the user never sees:
- Context compression timing and strategy
- System prompt assembly (domain prompt + project instructions + knowledge injections)
- Tool result formatting and truncation
- Error classification (model mistake vs. tool failure vs. permission denial)
- Status updates during routing and orchestration

This scaffolding lives in one place (the server), maintained once, benefiting every harness session.

### Pace and Rhythm

The harness should feel responsive:
- Streaming makes fast responses feel instant
- The timer shows the model is working during slow responses
- Status updates ("searching knowledge base...", "routing to claude-opus-4-6...") fill gaps that would otherwise feel like dead air
- Per-turn metadata gives a sense of progress and cost
- Responses are concise by default (system prompt discipline), verbose only when depth is warranted

## The Knowledge Layer as Core Differentiator

The integrated architecture transforms the knowledge layer (EXP-0001) from an optional module into the primary differentiator.

**Conflict with ADR-0008**: Accepted ADR-0008 (D1, weight 5, score 5) says the knowledge layer must be "fully optional" so users can run a lightweight proxy without performance, disk, or dependency cost. This exploration argues for changing that default. See the ADR Amendments section below for the proposed amendment and its consequences.

### Transparent Context Enrichment

When the user sends a message, the BFF silently searches the knowledge base for relevant prior context and injects it into the prompt. The user doesn't have to ask "what did I decide about X?" — the system already knows.

**Approach: Relevance-gated injection**

1. BFF embeds the user's message
2. Searches knowledge base for semantically similar prior interactions
3. Scores results by: relevance × recency × importance (pinned items score higher)
4. Injects top results as a "prior context" block in the system prompt
5. Respects a token budget (e.g., max 20% of context window)

The user can see what was injected (`/context`) and manage it (`/forget`, `/pin`, `/unpin`).

### Cross-Model Continuity

```
> /model claude-opus-4-6
Using Claude Opus 4.6 via Anthropic

> Help me design the authentication system for this API.
[... Claude provides a design ...]

> /model llama-3.1-70b
Using Llama 3.1 70B via Ollama

> What are the security weaknesses in the approach we just discussed?
[Llama has full context because the BFF owns the conversation state]
```

This works because the BFF owns the conversation history and translates between provider message formats transparently.

### Cost-Aware Routing

- Simple questions → cheap local model (Ollama, MLX) at $0.00
- Code generation → strong coding model (Claude, GPT-4) at market rate
- Review/critique → different model to avoid same-model blind spots
- Embedding/search → local model for privacy

Local models (PATCH-0002) aren't fallbacks — they're first-class routing targets that make the whole system cheaper.

## Orchestration as Built-In Capability

EXP-0007 (Multi-Model Orchestration) becomes a natural capability of the integrated product. The user doesn't configure workflows — the BFF orchestrates transparently:

**Research task**: decompose into sub-questions, route research to fast/cheap model, route synthesis to strong reasoning model.

**Code review**: send to primary model for structural review, send to a different model for adversarial review, merge findings.

**Build task**: planning runs on a cheaper model, coding routes to the strongest code model, review routes to a different model.

The orchestration is invisible unless the user asks (`/trace` shows the execution graph). For power users who want explicit control:

```
> /route coding=claude-opus-4-6 review=llama-3.1-70b planning=gpt-4
> /pipeline research -> draft -> review
> /compare "Which approach is better?" --models claude,gpt-4,llama
```

## Interface Separation

The product has three distinct interfaces that serve different consumers. They must not be conflated.

### 1. Harness Protocol (internal)

The canonical communication channel between the harness and the server/BFF.

- **Transport**: JSON-RPC over Unix socket (local) or TLS (remote)
- **Consumer**: modeltap harness only
- **Scope**: conversation turns, streaming responses, tool calls/results, status updates, session management, auth negotiation
- **Stability**: versioned but internal. Breaking changes are coordinated between harness and server releases. Not a public API.

This is NOT MCP. It is a purpose-built product protocol optimized for the harness-server interaction. It supports bidirectional streaming, modeltap-specific event types (knowledge hits, cost updates, compaction notices), and auth negotiation — none of which map cleanly onto MCP.

### 2. MCP Stdio Interface (external, per ADR-0009)

The external knowledge access interface for third-party AI clients.

- **Transport**: stdio, per ADR-0009. The MCP server runs as a subprocess of the AI client. No network listener.
- **Consumer**: Claude Code, Cursor, and other MCP-compatible AI clients
- **Scope**: knowledge search, recent interactions, decisions, stats — read-only access to the knowledge layer
- **Stability**: follows MCP protocol versioning. Public, stable, documented.

ADR-0009's decision is unchanged. The MCP stdio interface provides external AI clients with access to modeltap's knowledge base. It is a separate surface from the harness protocol. MCP does not carry conversation turns, tool execution, or session state — those flow through the harness protocol.

### 3. REST API (web dashboard and future web frontends)

The HTTP interface for browser-based access.

- **Transport**: HTTP/HTTPS
- **Consumer**: web dashboard (FEAT-0003), future web-based harness frontends
- **Scope**: log browsing, metrics, status (existing FEAT-0003 endpoints). Future: conversation turns and streaming for web harness.
- **Stability**: versioned. Public for the dashboard; may evolve as web frontends mature.

This is a separate interface from both the harness protocol and MCP. If a web-based harness frontend is built, it consumes the REST API (potentially upgraded to WebSocket for streaming), not the harness protocol or MCP.

### MCP for Tool Extensibility

Separately from the three server interfaces above, the **harness** acts as an MCP client to connect to external tool servers:

- Database MCP server → SQL query tool
- Docker MCP server → container management tool
- GitHub MCP server → PR/issue management tool
- Domain-specific MCP servers → legal, finance, healthcare tools (see Domain Extensibility)

Tool discovery is automatic. The BFF captures all MCP tool invocations for the knowledge layer. Domain tools are delivered as MCP server packages, not harness modifications.

## Domain Extensibility

The architecture is not inherently coding-specific. What makes modeltap "a coding tool" is three things that are all configurable: the tool set, the system prompt, and the knowledge extraction patterns. The core — multi-model routing, knowledge layer, capture, enterprise auth, cost tracking, orchestration — is domain-neutral.

### What Changes Per Domain

**1. The Tool Set (MCP servers)**

Each domain ships as an MCP server package:

**Legal / Patent Processing** (`modeltap-legal-tools`):

| Tool | Capability |
|------|-----------|
| DocumentCompare | Redline two document versions |
| ClauseSearch | Find similar clauses across a contract corpus |
| CitationCheck | Verify case citations, check if overruled |
| FilingCalendar | Jurisdiction-specific deadline tracking |
| RegLookup | Search regulatory databases (SEC, USPTO, CFR) |
| Redact | PII/privilege detection and redaction |

**Accounting / Finance** (`modeltap-finance-tools`):

| Tool | Capability |
|------|-----------|
| SpreadsheetRead | Parse Excel/CSV with structure awareness |
| SpreadsheetWrite | Generate formatted workbooks |
| GLLookup | Query general ledger by account/period/entity |
| TransactionCategorize | Classify against chart of accounts |
| ReportGenerate | Formatted financial statements |
| RegCheck | GAAP/IFRS compliance validation |
| AuditTrail | Immutable record of AI-assisted decisions |

**Healthcare / Clinical** (`modeltap-clinical-tools`):

| Tool | Capability |
|------|-----------|
| ChartReview | Parse clinical notes, lab results |
| CodingAssist | ICD-10/CPT code lookup and suggestion |
| DrugInteraction | Check medication combinations |
| ProtocolSearch | Clinical protocol/guideline search |
| DeIdentify | HIPAA-compliant data stripping |

The harness and BFF don't need to know about legal clauses or general ledgers. They discover tools via MCP and provide them to the model.

**2. The System Prompt**

Each domain has its own behavioral specification:

- **Coding**: read before write, don't over-engineer, try the simplest approach
- **Legal**: never state a conclusion without citing authority, distinguish binding vs. persuasive authority, preserve privilege, check jurisdiction
- **Finance**: verify every number against source data, distinguish GAAP vs. IFRS, flag materiality thresholds, require supporting documentation
- **Healthcare**: flag clinical uncertainty explicitly, cite guidelines, ensure patient safety caveats

The system prompt is server-side configuration. Switching domains means switching the prompt and tool set, not rewriting the product.

**3. Knowledge Extraction Patterns**

The knowledge layer extracts structured metadata from conversations. What's extracted varies by domain:

- **Coding**: decisions, architecture choices, approaches tried, bugs found
- **Legal**: precedents cited, contract terms agreed, filing deadlines, risks identified
- **Finance**: account balances referenced, reconciliation items, audit findings, regulatory interpretations
- **Healthcare**: diagnoses discussed, treatments considered, contraindications flagged

Extraction is model-driven — the BFF prompts a model to extract structured metadata. It adapts to domain context through the extraction prompt, not through code changes.

### What Stays the Same Across Domains

Everything else:

- Server/BFF architecture and protocol
- Identity, auth, and user isolation (EXP-0002)
- Knowledge layer (semantic search, embeddings, context enrichment)
- Multi-model routing and cost-aware selection
- Capture, audit trail, and metrics
- Session management and compaction
- Orchestration engine
- Enterprise features (spend budgets, model access control, policy enforcement)
- Streaming protocol and status tracking

### The Frontend Question

The terminal harness is developer-centric. Other domains may need different frontends:

- **Terminal**: coding, DevOps, data science — professionals who live in the terminal
- **Web**: legal, finance, healthcare — professionals who expect browser-based tools with document previews, formatted output, and domain-specific UI

The BFF protocol (JSON-RPC over TLS) supports multiple frontend types. The terminal harness is the first frontend. When a non-developer customer requires it, a web-based harness can be built that speaks the same protocol. All server-side investment (auth, knowledge, routing, capture, orchestration) carries over. Only the frontend changes.

This is not "make the server work with any client" (which would be a standalone proxy). It is "build purpose-built frontends as part of the product, each speaking the modeltap protocol."

### Business Positioning

This reframes modeltap from "AI coding tool" to **"enterprise AI environment for professional workflows."**

Coding is the first vertical because the team has domain expertise and the market is proven. The architecture supports any knowledge-work vertical where:

1. Professionals interact with AI models repeatedly
2. Institutional memory matters
3. Multiple models are useful for different subtasks
4. Audit and compliance are non-negotiable
5. Cost control matters at organizational scale

Legal, finance, healthcare, consulting, research — they all fit this pattern.

## Implications for Existing Explorations

### EXP-0001 (Knowledge Layer): Core Differentiator

The knowledge layer should be on by default when the harness is the interface, with local embedding models (Ollama/MLX) as the zero-cost default. It can still be disabled (`knowledge.enabled: false`) — the harness works without it, just without context enrichment or semantic search. This preserves ADR-0008's optionality while changing the default. See ADR Amendments below.

### EXP-0002 (Multi-User): Enterprise Prerequisite

Multi-user with pluggable identity (SPIFFE, OIDC, tokens, local socket), per-user isolation, server-owned credentials, and spend governance. See EXP-0002 for the full specification. Per-user knowledge isolation is the baseline; shared knowledge is deferred.

### EXP-0005 (Apprenticeship): Harness Feature

The harness already captures everything. Apprenticeship adds a review and annotation layer on top — a mentor views an apprentice's session (with consent) and annotates it, feeding back into the knowledge layer.

### EXP-0007 (Orchestration): Transparent Intelligence

Orchestration is how the BFF handles complex tasks, not a feature the user configures. `/trace` reveals the internals for users who want visibility.

### PATCH-0002 (Local Inference): Cost Layer

Local models are first-class routing targets. The routing policy sends work to local models whenever they're sufficient, making modeltap cheaper than direct API access for many interactions.

## Technical Threads

### Thread 1: The Harness Protocol

The harness protocol (see Interface Separation above) uses JSON-RPC over Unix socket (local) or TLS (remote). This is distinct from the MCP stdio interface (ADR-0009, external) and the REST API (web dashboard). See EXP-0002 for auth negotiation over this connection.

Key design questions:
- Bidirectional streaming: harness sends tool results while BFF streams model tokens
- Protocol versioning for forward compatibility across server/harness version skew
- Heartbeat and reconnection for service mode
- Whether this protocol eventually stabilizes into a public API is a separate future decision (see Promotion Targets)

### Thread 2: Provider Message Format Translation

The BFF translates between its canonical conversation format and provider-specific formats (Anthropic messages, OpenAI chat completions, OpenAI Responses API, Ollama). This includes message structure, tool call semantics, system prompt conventions, and context window truncation strategies.

The provider adapter interface (ADR-0006) must support both parsing (inbound) and formatting (outbound) for full message histories.

### Thread 3: Embedding Strategy

Local embedding by default (`nomic-embed-text` via Ollama). Cloud embedding as opt-in upgrade. Each conversation turn is one embedding unit. Metadata is embedded as structured text. All vectors in sqlite-vec (ADR-0008).

### Thread 4: Terminal UI Framework

See ADR-0013 (proposed). Phased approach: minimal prototype first (proving the conversation loop), Bubbletea for production (styled markdown via Glamour, scrollable viewport, multi-line input, status bar).

### Thread 5: Graceful Degradation

- No embedding model → keyword search only, backfill when available
- No cloud providers → local models only (valid offline use case)
- No local models → cloud only (more expensive)
- Knowledge database corrupted → rebuild from raw capture log
- Server crash in service mode → harness offers restart or reconnect

### Thread 6: Project Context

Running `modeltap` in a git repository scopes the session to that project. Knowledge queries prefer project-relevant results. Project-level config (`.modeltap.yaml`) overrides global settings. File operations are relative to the project root.

### Thread 7: Security Model

Trust boundaries: user trusts harness → harness trusts server (authenticated) → neither trusts model output (tool calls always gated by harness permissions).

**Execution boundary**: The BFF proposes tool actions by forwarding model tool calls. The harness is the sole authority for local execution — it decides whether to execute, prompt, collect into a plan, or reject. The BFF has no mechanism to execute local operations and does not know the harness's current execution mode (plan/build/auto). This ensures the trust boundary for local actions stays on the machine that will be affected, not on a remote server.

**Tool safety**: The Edit tool requires exact string matching (no hallucinated overwrites). File writes snapshot the original (reversibility). Bash commands display the full command before execution.

In enterprise deployments, server security follows EXP-0002: TLS/mTLS, server-driven auth, user isolation.

### Thread 8: Migration Path from v1

The proxy core, storage, and provider adapters remain unchanged. Existing CLI commands (`modeltap logs`, `modeltap metrics`, `modeltap export`) remain as subcommands. `modeltap` with no subcommand launches the harness (new default). Existing databases work with the new version.

## Session and State Model

### Sessions as First-Class Objects

A session is a unit of work that may span multiple models, multiple tool executions, and multiple sittings.

```
Session
├── Turn 1 (user → claude-opus-4-6 → response with tool calls)
│   ├── Tool: Read file
│   ├── Tool: Grep codebase
│   └── Tool: Read file
├── Turn 2 (user → claude-opus-4-6 → response)
├── Turn 3 (user switches model → llama-3.1-70b → response)
├── [session suspended, user closes terminal]
├── [session resumed next day]
├── Turn 4 (user → claude-opus-4-6 → response)
│   └── Orchestrated subtask → gpt-4 (invisible to user)
└── Turn 5 (user → response, session complete)
```

The BFF persists session state to SQLite. Sessions survive terminal closure, model switches, compaction, and service restarts.

### Session Commands

- `modeltap` — start new session or resume most recent for this project
- `modeltap --resume <id>` — resume specific session
- `modeltap --project <path>` — session scoped to project directory
- `/compact` — compress context, retain full history in knowledge layer
- `/clear` — fresh context within the same session
- `/fork` — branch session into independent continuations
- `/model <name>` — switch models within session
- `/plan` — enter plan mode
- `/build` — enter build mode (default)
- `/auto` — enter autonomous mode
- `/cost` — session cost breakdown
- `/trace` — model routing and orchestration for last turn
- `/context` — show files, knowledge injections, and context budget

## Configuration

### Solo Developer

```yaml
# ~/.config/modeltap/config.yaml

providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
  ollama:
    host: http://localhost:11434

routing:
  default: claude-opus-4-6
  cheap: llama-3.1-8b
  embedding: nomic-embed-text

knowledge:
  enabled: true
  auto_enrich: true
  retention: 90d

permissions:
  mode: default

mcp:
  servers:
    - name: github
      command: gh-mcp-server
      transport: stdio
```

### Enterprise (Server Config)

See EXP-0002 for the full enterprise server configuration including auth providers, role overrides, spend budgets, and model access policies. The harness config for enterprise users is minimal:

```yaml
server:
  address: modeltap.internal.acme.com:8443
```

## What Makes This Different

| Capability | Claude Code | Cursor | Aider | Codex | Modeltap |
|-----------|------------|--------|-------|-------|----------|
| Terminal AI interface | Yes | No (IDE) | Yes | No (cloud) | Yes |
| Multi-provider | No | Yes | Yes | No | Yes |
| Cross-model memory | No | No | No | No | **Yes** |
| Semantic knowledge search | No | No | No | No | **Yes** |
| Cost-aware routing | No | No | No | No | **Yes** |
| Automatic context enrichment | No | No | No | No | **Yes** |
| Lossless compaction | No | No | No | No | **Yes** |
| Multi-model orchestration | No | No | No | No | **Yes** |
| Full interaction capture | No | No | No | No | **Yes** |
| Cross-session continuity | Partial | Partial | No | No | **Yes** |
| Local model first-class | No | Limited | Yes | No | **Yes** |
| Enterprise multi-user | No | No | No | Yes | **Yes** |
| Centralized credentials | No | No | No | Yes | **Yes** |
| Spend governance | No | No | No | Partial | **Yes** |
| Domain extensibility | No | No | No | No | **Yes** |
| Plan/build modes | Partial | No | No | No | **Yes** |
| Single binary (Go) | No (Node) | No (Electron) | No (Python) | No (cloud) | **Yes** |
| MCP extensibility | Yes | Yes | No | No | **Yes** |

The differentiators stem from the same architectural choice: the intelligence is in the middleware, not the client. And the middleware is domain-neutral.

## Promotion Targets

This exploration is a product-thesis umbrella that contains multiple downstream promotion targets. It should not promote as a single feature spec. The near-term and future scopes are:

### Near-Term: Terminal Harness MVP (→ feature spec)

The core conversation loop: harness connects to server, sends turns, streams responses, executes tools, enforces permissions. Includes:
- Harness protocol design and implementation
- Tool set (Read, Edit, Write, Bash, Glob, Grep, Git, WebSearch, WebFetch)
- Permission model (default, accept-edits, autonomous)
- Plan/build/auto modes
- File context management and large paste handling
- Context tracking and call timer
- Session management (resume, compact, clear, model switch)
- Knowledge injection (if knowledge layer is enabled)
- Enterprise auth and per-user isolation (from EXP-0002)
- System prompt engine

This is the promotable scope. It can be implemented and shipped independently.

### Future: Multi-Frontend Protocol Strategy (→ separate exploration or ADR)

The question of whether the harness protocol stabilizes into a public API, how the REST API evolves for web frontends, and when/whether a web-based harness is built. This is a separate product decision from the terminal harness MVP.

### Future: Domain Vertical Expansion (→ separate exploration per domain)

Legal, finance, healthcare, and other domain tool packages. Each domain has its own tool set, system prompt, and knowledge extraction patterns. These are independent product decisions that build on the harness infrastructure but do not block or change it.

## ADR Amendments

This exploration proposes amendments to two accepted ADRs. These amendments must be resolved before downstream implementation artifacts are created.

### ADR-0008 Amendment: Knowledge Layer Default

**Current**: The knowledge layer is "fully optional" — disabled by default, zero cost when disabled (D1, weight 5, score 5).

**Proposed**: The knowledge layer is **on by default** when the harness is the interface, but **can be disabled** via `knowledge.enabled: false`. When disabled, the harness works without context enrichment or semantic search — conversations still function, just without historical knowledge injection.

**Rationale**: In the integrated harness product, the knowledge layer is the primary differentiator. Defaulting to off means most users never discover it. Defaulting to on means the first-run experience demonstrates the product's unique value. The "lightweight proxy" use case is preserved via the config flag.

**Consequences**:
- First-run experience requires either a local embedding model (Ollama) or a cloud embedding key. If neither is available, knowledge features degrade gracefully to keyword search only, with embeddings backfilled when a model becomes available.
- Disk usage increases for default installations (embedding vectors alongside captures).
- ADR-0008's D1 score changes from 5 to 4 — optionality is preserved but the default flips.

**Resolution path**: This should be formalized as `ADR-0008-amendment-001` or, if the scope warrants, a superseding `ADR-0014`.

### ADR-0009 Scope Clarification

**Current**: ADR-0009 chose MCP stdio transport for knowledge access. The scoring context assumed MCP was the primary interface for all knowledge access.

**Proposed**: ADR-0009's scope is **external knowledge access by third-party AI clients** (Claude Code, Cursor, etc.). The harness-to-server protocol is a separate internal interface (see Interface Separation above). ADR-0009 is not superseded — its decision and rationale remain valid for the external MCP surface. The harness protocol is a new interface that did not exist when ADR-0009 was written.

**Resolution path**: Add a clarifying note to ADR-0009 stating its scope is the external MCP surface, and that the harness-to-server protocol is a separate concern addressed by EXP-0008.

## Open Questions

### Architecture

1. **REST API scope**: the web dashboard (FEAT-0003) already uses REST. Does it need to expand, or is the current scope sufficient for the near-term?

2. **How does the harness handle multiple concurrent sessions?** One session per terminal, or tabs/splits?

3. **Harness protocol stability**: the protocol is internal for now. If/when multi-frontend becomes a priority, what is the stabilization process? (This is a future exploration, not a near-term concern.)

### Knowledge Layer

4. **Injection budget tuning**: should the user control how aggressively the BFF injects knowledge, or should the BFF auto-tune based on query complexity?

5. **Negative knowledge**: should the knowledge layer distinguish between "facts I learned" and "approaches that failed"?

6. **Scale**: sqlite-vec brute-force KNN is 50-200ms at 100k vectors. At 200 interactions/day, that's ~500 days. Sufficient, or plan for HNSW indexing?

### Routing and Orchestration

7. **Adaptive routing**: should modeltap track which models produce better results for which task types and adjust over time?

8. **Orchestration visibility**: transparent by default (user sees one response) vs. real-time visibility of subtask execution?

9. **User override vs. routing policy**: when the user explicitly selects a model (`/model`), should subtask routing still apply?

### User Experience

10. **First-run experience**: setup wizard, auto-detect local models, or minimal config?

11. **Long-running tasks**: background orchestration workflows? Work on something else in the same session?

12. **Headless mode**: `echo "fix the bug" | modeltap` for CI/CD and scripting?

### Domain Extensibility

13. **How are domain-specific system prompts managed?** Per-project config? Server-level domain setting? Auto-detected from tool set?

14. **Domain-specific knowledge extraction**: how much domain tuning is needed, vs. generic extraction that works across domains?

15. **Web harness priority**: when does the first non-terminal frontend become necessary? Is it customer-driven or proactive?

### Ecosystem

16. **Relationship to Claude Code's MCP ecosystem**: modeltap consumes the same MCP servers. Alternative, complement, or competitor?

17. **Should modeltap publish domain-specific MCP servers?** Or rely on community/partner ecosystem?

## Proposed Next Steps

1. **Design the internal protocol**: specify JSON-RPC messages between harness and BFF. Minimal set: send turn, stream response, tool call, tool result, status update.

2. **Build the harness MVP (minimal, per ADR-0013)**: terminal UI that connects to the BFF, sends messages, streams responses, executes basic tools (Read, Edit, Bash, Glob, Grep). No knowledge injection or orchestration yet — just the core loop.

3. **Implement knowledge injection**: add semantic search to the knowledge layer and wire it into the conversation loop. Measure whether injected context improves response quality.

4. **Prototype cross-model session state**: build the conversation format translation layer. Test switching between Anthropic and OpenAI mid-session.

5. **Add routing and cost tracking**: implement the routing policy engine and real-time cost display. Test with mixed local + cloud model configurations.

6. **Implement plan/build modes**: add the tool call interceptor that presents plans instead of executing.

7. **Enterprise auth integration**: implement token auth and OIDC providers from EXP-0002, per-user isolation, server-owned credentials.

If this exploration promotes to a feature spec, the scope should be the harness MVP (step 2) plus knowledge injection (step 3) plus enterprise auth (step 7). Routing, orchestration, and domain extensibility are powerful but can follow as subsequent features once the core loop and enterprise foundation are proven.
