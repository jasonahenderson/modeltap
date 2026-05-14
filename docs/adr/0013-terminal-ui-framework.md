---
status: proposed
date: 2026-04-14
decision-makers: Jason Henderson
---

# ADR-0013: Terminal UI Framework for Harness

## Context and Problem Statement

The integrated harness (EXP-0008) is the user-facing terminal interface for modeltap. It sends conversation turns to the server (runtime server), streams model responses to the terminal, executes tools locally, enforces permissions, and displays session metadata (model, cost, context usage). The harness must render a conversational UI — not a dashboard, not a multi-pane IDE — with streaming markdown output, multi-line input, tool execution blocks, permission prompts, and a status bar.

This decision constrains the harness's rendering approach, language alignment with the Go server, distribution model (single binary vs. multi-runtime), and the complexity ceiling for future UI work. It also has second-order effects on contributor experience, build system, and test infrastructure.

The harness is intentionally the thin half of the product — the intelligence lives in the server/runtime server. The UI framework should match that philosophy: capable enough for the conversational interface, without pulling the harness toward IDE-level complexity.

## Decision Drivers

Drivers are weighted 1-5, where 5 = critical.

* **D1 - Single binary distribution (5):** Modeltap ships as one compiled artifact containing both harness and server (EXP-0008). The UI framework must not introduce a second runtime dependency (Node.js, Python, Bun) that complicates installation, cross-compilation, or distribution via goreleaser.
* **D2 - Streaming token display (5):** Model responses arrive token by token via the runtime server. The framework must render incremental text updates at low latency without visual artifacts (flicker, layout jumps, orphaned escape codes). This is the core UX moment — any framework that cannot do this well is disqualifying.
* **D3 - Language alignment with server (4):** The harness and server share types (protocol messages, session state, tool definitions). A framework in the same language as the server (Go) eliminates a serialization boundary and simplifies the build, test, and contribution story. A different language creates a two-team problem.
* **D4 - Markdown rendering quality (4):** Model responses are markdown. The framework (or its ecosystem) must render headings, code blocks, lists, bold/italic, and inline code in the terminal with reasonable fidelity. Streaming complicates this — partial markdown must render incrementally without waiting for the full response.
* **D5 - Input editing capability (4):** Users compose multi-line messages with cursor movement, backspace, paste, and history recall. The framework must provide or support a text input component beyond basic readline.
* **D6 - Scrollback and viewport (3):** Users need to scroll up through conversation history. The framework must support a scrollable viewport that can be programmatically updated (append new content, scroll to bottom on new output, allow user scroll-up without fighting).
* **D7 - Permission prompt UX (3):** Tool calls require user approval. The framework must support modal-style prompts that interrupt the conversation flow, accept y/n input, and resume streaming. This is a common interaction — it must feel responsive, not hacky.
* **D8 - Ecosystem and community (3):** A larger ecosystem means more reusable components, more Stack Overflow answers, more contributors who already know the framework. Matters for long-term maintenance and contributor onboarding.
* **D9 - Minimal framework complexity (3):** The harness is the thin part of the product. The framework should not impose architectural overhead (large abstractions, complex state management, steep learning curves) disproportionate to the UI's actual complexity.
* **D10 - Async event handling (2):** The harness handles concurrent events: streaming tokens, tool execution results, permission responses, status updates from the runtime server. The framework must support async event delivery without blocking the render loop.

## Considered Options

* Bubbletea (Charm ecosystem)
* tview
* Raw tcell
* TypeScript + Ink (React for terminal)
* Minimal (no framework)

## Decision Outcome

Chosen option: **Bubbletea (Charm ecosystem) from day one**, because it achieves the highest weighted score (119) with a clear margin over all alternatives, and the feature requirements (FEAT-0009) make a minimal prototype impractical.

The original proposal was a phased approach: minimal prototype first, Bubbletea later. This was revised after the feature specifications were developed. The features require interactive UI that a readline + stdout approach cannot render:

- **Plan/build mode indicator** in a persistent status bar with `Ctrl+P` toggle
- **Session explorer** with list navigation, details view, and compact-before-resume
- **Interactive compaction** with categorized context, per-category action selectors, and file-level drill-down
- **Model routing indicator** displayed before every response
- **Streaming markdown** with styled output (headings, code blocks, bold/italic)
- **Scrollable viewport** with auto-scroll and keyboard navigation

A minimal prototype would be a throwaway — it cannot render half the required UX. The phased approach would mean building the conversation loop twice: once in raw stdout, once in Bubbletea. That is wasted effort.

**Precedent**: OpenCode, the most comparable Go terminal AI tool, uses Bubbletea from the start with no phased approach. Their plan/build tab switching, styled markdown, and status bar are all Bubbletea from day one. Claude Code uses TypeScript + Ink (React for terminal), which is the equivalent choice in the TS ecosystem — they also did not phase.

Bubbletea's streaming weakness (D2: 3) is a solvable engineering challenge, not an architectural risk. The debounced redraw approach (buffer tokens, re-render every 50ms) is well-understood and used by OpenCode.

### Scoring Matrix

Scale: 1 (poor) - 5 (excellent). Weighted total = sum of (weight x score).

| Driver | Weight | Bubbletea | tview | Raw tcell | TS + Ink | Minimal |
|--------|--------|-----------|-------|-----------|----------|---------|
| D1: Single binary distribution | 5 | 5 | 5 | 5 | 1 | 5 |
| D2: Streaming token display | 5 | 3 | 4 | 5 | 5 | 4 |
| D3: Language alignment | 4 | 5 | 5 | 5 | 1 | 5 |
| D4: Markdown rendering quality | 4 | 4 | 2 | 2 | 5 | 2 |
| D5: Input editing capability | 4 | 5 | 4 | 2 | 5 | 3 |
| D6: Scrollback and viewport | 3 | 5 | 5 | 4 | 5 | 1 |
| D7: Permission prompt UX | 3 | 4 | 4 | 3 | 5 | 2 |
| D8: Ecosystem and community | 3 | 5 | 3 | 2 | 5 | 1 |
| D9: Minimal framework complexity | 3 | 3 | 3 | 1 | 2 | 5 |
| D10: Async event handling | 2 | 5 | 3 | 5 | 5 | 2 |
| **Weighted Total** | | **119** | **108** | **103** | **101** | **99** |

### Scoring Justification

#### Bubbletea (119) — Production Target

* **D1 (5):** Pure Go. Compiles into the same binary as the server. No runtime dependencies. Works with goreleaser cross-compilation.
* **D2 (3):** This is Bubbletea's main weakness for this use case. Bubbletea redraws the full view on every Update cycle. Streaming tokens means frequent redraws. The standard approach is to buffer tokens and debounce redraws (e.g., every 50ms), which works but adds implementation complexity. Not as natural as "just print to stdout." Scored 3 because the workaround is well-understood but requires deliberate engineering.
* **D3 (5):** Same language as the server. Protocol message types, session state structs, and tool definitions are shared directly — no serialization boundary, no code generation, no type drift.
* **D4 (4):** Glamour (Charm's markdown renderer) produces high-quality styled terminal output for complete markdown. The streaming challenge: Glamour is batch-oriented (render a full string), not incremental. The workaround is to re-render the accumulated response buffer on each debounced cycle. For typical response lengths (< 10KB), this is fast enough. For very long responses, chunked re-rendering may be needed. Scored 4 because the quality is excellent and the streaming workaround is viable, but not zero-effort.
* **D5 (5):** The `textarea` bubble provides multi-line input with cursor movement, selection, paste, and configurable key bindings. Best-in-class for Go terminal input.
* **D6 (5):** The `viewport` bubble provides a scrollable region with programmatic content updates, keyboard-driven scrolling, and auto-scroll-to-bottom behavior. Exactly what the conversation view needs.
* **D7 (4):** Permission prompts can be implemented as a state change in the Model — swap the active view component from the conversation to a prompt, capture the response, swap back. Not a built-in modal widget, but the pattern is straightforward in Elm architecture. Scored 4 because it works cleanly but requires manual implementation.
* **D8 (5):** Largest ecosystem in the Go TUI space. ~18,000 dependent repositories. Charm actively maintains Bubbletea, Bubbles (components), Lipgloss (styling), and Glamour (markdown). Community contributions add components regularly. Contributors are likely to have encountered Bubbletea before.
* **D9 (3):** Elm architecture (Model-Update-View) is clean but adds structural overhead. Every interaction requires defining a message type, handling it in Update, and reflecting it in View. For a simple conversation UI this is manageable; the architecture earns its weight if the UI grows more complex. Scored 3 — appropriate complexity for a production harness, slightly heavy for a prototype.
* **D10 (5):** Bubbletea's Cmd system maps naturally to Go concurrency. Long-running operations (runtime server communication, tool execution) run in goroutines and deliver results as messages. The Update loop is single-threaded — no race conditions in state management. Excellent fit for the harness's concurrent event streams.

#### tview (108)

* **D1 (5):** Pure Go. Same distribution story as Bubbletea.
* **D2 (4):** tview's `TextView` widget supports `Write()` for streaming content. Tokens can be written directly as they arrive. Better streaming ergonomics than Bubbletea's full-redraw model. Scored 4 because streaming is natural but rendering customization is more limited.
* **D3 (5):** Same language as the server. Same benefits as Bubbletea.
* **D4 (2):** No built-in markdown rendering. Would need to integrate a separate markdown-to-ANSI library or build custom rendering. tview's `TextView` supports some inline styling tags, but they are not markdown. Scored 2 because this is a significant gap for a tool that primarily displays markdown.
* **D5 (4):** `InputField` and `TextArea` widgets handle text input. Less customizable than Bubbletea's textarea bubble, but functional for message composition.
* **D6 (5):** `TextView` has built-in scrolling. Widget-based layout handles viewport management.
* **D7 (4):** `Modal` widget exists for dialogs. Permission prompts map directly.
* **D8 (3):** Active project but smaller community than Bubbletea. Fewer third-party components. Contributors less likely to have used it.
* **D9 (3):** Widget-based architecture is familiar from GUI toolkits but can feel heavy for a conversational UI. Callback-based event handling can get tangled for complex interaction flows.
* **D10 (3):** Application-level event loop with `QueueUpdateDraw` for goroutine-safe updates. Works but less ergonomic than Bubbletea's message-passing model.

#### Raw tcell (103)

* **D1 (5):** Pure Go. tcell is the terminal abstraction both Bubbletea and tview build on.
* **D2 (5):** Maximum control over rendering. Can update individual cells without redrawing. Lowest-latency streaming possible in a terminal.
* **D3 (5):** Same language as the server.
* **D4 (2):** No markdown rendering. No text styling abstractions. Every rendering decision is manual cell-by-cell. Would need to build or integrate a full markdown renderer.
* **D5 (2):** No input widgets. Would need to build text input from scratch — cursor management, key handling, clipboard, scrolling within the input area.
* **D6 (4):** No viewport widget, but possible to implement. Full control means full responsibility.
* **D7 (3):** No modal or prompt abstraction. Implementable but entirely custom.
* **D8 (2):** tcell is foundational but not something most developers use directly. Small community of direct users. Contributors would need to learn the low-level API.
* **D9 (1):** Using tcell directly means building a UI framework. The harness is supposed to be the thin part of the product — building a framework contradicts that philosophy. Scored 1 because the complexity is disproportionate to the UI requirements.
* **D10 (5):** Direct control over the event loop. Goroutines and channels work naturally. No framework abstractions in the way.

#### TypeScript + Ink (101)

* **D1 (1):** Requires Node.js or Bun. Cannot compile into the Go binary. Distribution becomes either: ship two binaries (Go server + JS harness), bundle a JS runtime, or require users to install Node.js. All options are significantly worse than a single binary. Scored 1 because this is the most heavily weighted driver and Ink fails it completely.
* **D2 (5):** React's reconciler naturally handles incremental updates. Ink re-renders only changed components. Streaming tokens update a text component; React diffs and applies minimal terminal updates. This is what Claude Code does — it is proven at scale.
* **D3 (1):** Different language. Requires a serialization boundary (JSON-RPC over socket) between harness and server. Two build systems, two test suites, two dependency trees, two contribution paths. Type definitions drift unless shared via code generation.
* **D4 (5):** React ecosystem has excellent markdown rendering (react-markdown, marked, remark). Streaming markdown is well-solved — render partial markdown as it arrives, update the component tree incrementally.
* **D5 (5):** Ink's `useInput` hook and custom text input components handle multi-line editing. Claude Code's implementation proves this works.
* **D6 (5):** React component composition handles scrollable viewports naturally. Ink provides layout primitives (Box, Text) with flexbox-style layout via Yoga.
* **D7 (5):** React's component model handles modal prompts cleanly — conditionally render a prompt component, capture input, unmount. Claude Code does this for every permission check.
* **D8 (5):** Massive ecosystem. React is the most widely known UI framework. Ink has a healthy community. Claude Code's open source provides a reference implementation.
* **D9 (2):** React + Ink is a powerful but complex stack. JSX compilation, React reconciler, Yoga layout engine, hook-based state management — significant abstraction layers for a terminal UI. Claude Code's 512k lines are partly a consequence of this complexity.
* **D10 (5):** React's state management and effect hooks handle async naturally. Promises, async/await, and event emitters map well to concurrent runtime server communication.

#### Minimal — No Framework (99)

* **D1 (5):** Pure Go. No dependencies beyond the standard library and a readline library (e.g., `peterh/liner` or `chzyer/readline`).
* **D2 (4):** `fmt.Print` streams tokens directly to stdout. The terminal handles scrolling. No framework redraw cycle to manage. Slight deduction because raw ANSI output can produce artifacts with complex markdown (partial escape sequences at token boundaries). Scored 4 because simple streaming is excellent, edge cases require care.
* **D3 (5):** Same language. Shared types. No serialization boundary.
* **D4 (2):** No markdown rendering. Output is unstyled plaintext or requires manual ANSI code generation. Could integrate Glamour for batch rendering of completed responses, but streaming rendering is unstyled.
* **D5 (3):** Readline libraries provide single-line input with history, editing, and completion. Multi-line input requires either a workaround (backslash continuation, paste detection) or a more capable library. Scored 3 because basic input works but multi-line composition is limited.
* **D6 (1):** No programmatic scrollback. The terminal's native scroll buffer is the only option. Cannot implement "scroll up through conversation, then snap back to bottom on new output." This is the approach's most significant UX limitation.
* **D7 (2):** Permission prompts are inline — print the prompt, read y/n, continue. Cannot overlay a prompt on top of streaming output. If a tool call arrives mid-stream, the prompt interrupts the visual flow. Functional but rough.
* **D8 (1):** No framework community. Every UI pattern is custom to this project. No reusable components, no external documentation, no contributor familiarity.
* **D9 (5):** Minimal complexity by definition. No abstractions, no framework learning curve, no architectural constraints. The harness is exactly as complex as the code you write.
* **D10 (2):** Async handling is manual. Goroutines writing to stdout concurrently require explicit synchronization (mutex on output). No event loop, no message passing — just goroutines and channels wired by hand.

### Consequences

* Good, because Bubbletea's Elm architecture (Model-Update-View) provides a clean state management model that scales as the UI grows. The harness has enough interactive UI (session explorer, compaction selector, plan/build modes, status bar) to justify the framework.
* Good, because Glamour produces high-quality markdown rendering, and the Charm ecosystem provides reusable components (textarea, viewport, spinner, table) that accelerate development.
* Good, because the single-binary story is preserved — no runtime dependencies, simple installation, clean goreleaser integration.
* Good, because Go alignment means shared types with the runtime server, one build, one test suite, and a single-language contribution path.
* Good, because OpenCode proves this stack works for this exact use case — plan/build modes, styled markdown, status bar, all in Bubbletea.
* Bad, because streaming token display requires a debounced redraw workaround rather than direct `Write()` calls. This is the primary engineering challenge. The workaround is well-understood (buffer tokens, re-render every 50ms, final clean render on completion) but requires deliberate implementation.
* Bad, because Glamour's batch markdown rendering means long responses may show brief render pauses during streaming. Mitigation: chunk the response buffer and only re-render the latest chunk during streaming; render the full response on completion.
* Bad, because the framework imposes Elm architecture from the start — every interaction requires message types, Update handlers, and View rendering. This is more upfront work than raw stdout. The tradeoff is accepted because the feature requirements demand it.
* Neutral, because the Bubbletea learning curve is real but bounded. The ecosystem is well-documented, and OpenCode's open source provides a direct reference for this use case.

### Confirmation

The decision is confirmed when: the Bubbletea harness can connect to the runtime server, render a session explorer with list navigation, display a persistent status bar with mode/model/context/cost, stream a model response with styled markdown (Glamour), execute tools with permission prompts that don't disrupt the conversation flow, and render the interactive compaction category selector. Streaming token display latency must be under 100ms perceived delay.

### Why Not Phased

The original proposal (2026-04-14) was to start with a minimal prototype and migrate to Bubbletea when interaction patterns stabilized. This was revised (2026-04-15) after feature specifications (FEAT-0009) revealed that the required UI — plan/build mode indicator with keyboard toggle, session explorer, interactive compaction, model routing display — cannot be rendered with stdout + readline. A minimal prototype would be throwaway code, and the migration would mean building the rendering layer twice. OpenCode's successful direct adoption of Bubbletea for the same UI patterns confirms that the framework risk is manageable.

## More Information

**TypeScript + Ink** scored competitively on most drivers (101) but its failure on D1 (single binary distribution, weight 5, score 1) is disqualifying. The 18-point deficit versus Bubbletea is almost entirely explained by D1 and D3. Claude Code uses Ink because they are a TypeScript shop. In a Go project, the two-language tax is too high.

**tview** (108) is a credible alternative with better streaming ergonomics (D2: 4 vs. 3) but weaker markdown rendering (D4: 2 vs. 4) and a smaller ecosystem (D8: 3 vs. 5). If Bubbletea's streaming workaround proves more problematic than expected, tview is the first fallback to evaluate.

**Raw tcell** (103) is rejected because building a UI framework contradicts the harness's design philosophy as the thin part of the product.

**Minimal** (99) was originally chosen for a prototype phase but is no longer part of the decision. The feature requirements make a minimal prototype impractical — the UI demands (mode indicators, session explorer, compaction selectors, model display) exceed what stdout + readline can render. See "Why Not Phased" above.

**Comparable tools**: Claude Code (TypeScript + Ink), OpenCode (Go + Bubbletea). Both went straight to their framework without a phased approach. OpenCode is the most direct precedent for modeltap's technology and UI choices.
