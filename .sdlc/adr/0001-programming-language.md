---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0001: Programming Language Selection

## Context and Problem Statement

Modeltap is a reverse proxy that sits between AI/ML clients (Claude Code CLI, VS Code extensions, and potentially other tools) and model API endpoints, logging requests and responses for observability and usage tracking. The choice of programming language affects distribution, performance, contributor experience, and long-term maintainability.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Distribution simplicity (5):** Users install this alongside dev tools; installation friction kills adoption. Single-binary distribution with no runtime dependency is ideal.
* **D2 – Streaming/SSE handling capability (5):** The core function of the proxy is intercepting and logging server-sent event streams. The language must handle chunked transfer encoding and SSE natively and reliably.
* **D3 – HTTP proxy ecosystem / stdlib support (4):** A reverse proxy is the central component. Strong stdlib or library support for this pattern reduces risk and code volume.
* **D4 – Cross-platform support (4):** Developers use macOS, Linux, and Windows. The proxy must build and run on all three without platform-specific workarounds.
* **D5 – Open source contributor accessibility (4):** Language popularity, learning curve, and the size of the contributor pool for this type of tool matter for community growth.
* **D6 – Runtime resource footprint (3):** The proxy runs in the background alongside an IDE, CLI tools, and the LLM itself. It should not compete for CPU or memory.
* **D7 – Speed of initial development (3):** Reaching a usable v1 quickly matters for validating the concept and attracting early adopters.
* **D8 – Concurrency model (3):** The proxy must handle multiple simultaneous client sessions without complexity or bottlenecks.

## Considered Options

* Go
* TypeScript (Node.js)
* Python
* Rust

## Decision Outcome

Chosen option: **Go**, because it achieves the highest weighted score (147) with a significant margin over the next closest option (Rust, 127). Go scores 4 or 5 on every decision driver with no meaningful weaknesses. It is the dominant language for exactly this category of tool — reverse proxies, CLI tools, and infrastructure software — which means the contributor pool, while smaller than TypeScript or Python overall, is highly relevant.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                          | Weight | Go | TypeScript | Python | Rust |
|---------------------------------|--------|----|------------|--------|------|
| D1: Distribution simplicity     | 5      | 5  | 2          | 1      | 5    |
| D2: Streaming/SSE handling      | 5      | 5  | 4          | 3      | 5    |
| D3: HTTP proxy ecosystem        | 4      | 5  | 3          | 4      | 4    |
| D4: Cross-platform support      | 4      | 5  | 4          | 3      | 5    |
| D5: Contributor accessibility   | 4      | 4  | 5          | 5      | 2    |
| D6: Resource footprint          | 3      | 5  | 3          | 2      | 5    |
| D7: Speed of development        | 3      | 4  | 5          | 5      | 2    |
| D8: Concurrency model           | 3      | 5  | 3          | 2      | 5    |
| **Weighted Total**              |        | **147** | **110** | **97** | **127** |

### Scoring Justification

#### Go (147)

* **D1 (5):** `go build` produces a single static binary. `goreleaser` handles cross-compilation and release packaging. No runtime needed.
* **D2 (5):** `net/http` handles chunked transfer and SSE natively. `io.TeeReader` makes stream tapping trivial.
* **D3 (5):** `httputil.ReverseProxy` is a battle-tested stdlib component purpose-built for this use case.
* **D4 (5):** `GOOS`/`GOARCH` cross-compilation is a one-line environment variable. First-class support for macOS, Linux, and Windows.
* **D5 (4):** Very popular for CLI and infrastructure tooling. Slightly smaller total developer pool than TypeScript or Python, but the right developers for this kind of tool.
* **D6 (5):** Low memory (~10–20 MB for an idle proxy), fast startup, no GC pauses that matter at this scale.
* **D7 (4):** More verbose than TypeScript or Python but the stdlib covers most needs. Less wiring code than Rust.
* **D8 (5):** Goroutines and channels are purpose-built for concurrent request handling.

#### TypeScript / Node.js (110)

* **D1 (2):** Requires Node.js runtime or bundling via `pkg`/`bun compile`, which produces larger binaries with known edge cases.
* **D2 (4):** Good SSE support via `eventsource-parser` and native streams. Slightly more complex to handle backpressure than Go.
* **D3 (3):** `http-proxy` / `node-http-proxy` exist but are less maintained than Go's stdlib. No stdlib reverse proxy.
* **D4 (4):** Node runs everywhere, but native modules (e.g., `better-sqlite3`) require platform-specific compilation.
* **D5 (5):** Largest developer pool. Lowest barrier to contribution. Same ecosystem as Claude Code.
* **D6 (3):** V8 memory overhead (~50–80 MB baseline). Event loop handles I/O concurrency well but is single-threaded by default.
* **D7 (5):** Fastest to prototype. Rich npm ecosystem.
* **D8 (3):** Event loop is good for I/O concurrency but single-threaded. Worker threads add complexity.

#### Python (97)

* **D1 (1):** Requires Python runtime, venv, pip. PyInstaller/Nuitka produce large binaries with compatibility issues. Worst distribution story of the four options.
* **D2 (3):** `aiohttp` and `httpx` support streaming but async Python has ergonomic rough edges. `mitmproxy` handles it well but is a heavy dependency.
* **D3 (4):** `mitmproxy` is excellent and mature, but it is a framework you build on top of rather than a library you integrate — it owns the process.
* **D4 (3):** Python is cross-platform but packaging and distribution differ per OS. Windows is notably worse.
* **D5 (5):** Largest overall developer community. Very accessible language.
* **D6 (2):** Highest memory usage. GIL limits true concurrency without multiprocessing.
* **D7 (5):** Fastest to write. A `mitmproxy` addon could produce a very quick v1.
* **D8 (2):** GIL constrains concurrency. asyncio helps for I/O but adds complexity.

#### Rust (127)

* **D1 (5):** Single static binary, even smaller than Go. Excellent cross-compilation via `cross`.
* **D2 (5):** `hyper` and `tokio` handle streaming with zero-copy efficiency. Best raw performance.
* **D3 (4):** `hyper` is excellent but lower-level than Go's stdlib. More code required to build a reverse proxy.
* **D4 (5):** Excellent cross-compilation. No runtime dependencies.
* **D5 (2):** Smallest contributor pool for this type of tool. Steep learning curve. Borrow checker discourages casual contributions.
* **D6 (5):** Lowest possible resource usage. No garbage collector.
* **D7 (2):** Slowest development speed. Significant boilerplate for async HTTP handling.
* **D8 (5):** Tokio async runtime is best-in-class. Zero-cost abstractions.

### Consequences

* Good, because Go's stdlib `httputil.ReverseProxy` gives us the core proxy component with minimal code.
* Good, because single-binary distribution via `goreleaser` eliminates installation friction.
* Good, because Go is the dominant language for CLI/proxy/infrastructure tools, attracting relevant contributors.
* Good, because goroutines handle concurrent proxy sessions with no additional framework or library.
* Neutral, because Go's verbosity means more lines of code than TypeScript or Python for equivalent logic, but the code is explicit and readable.
* Bad, because Go lacks generics maturity (improving but still less expressive than Rust or TypeScript for some patterns).
* Bad, because Go's error handling is verbose compared to exceptions (TypeScript/Python) or Result types (Rust).

### Confirmation

The decision will be confirmed by successfully implementing the core reverse proxy with SSE stream capture in Go, validating that the stdlib components (`httputil.ReverseProxy`, `io.TeeReader`) work as expected for this use case.

## More Information

The decision aligns with the weighted scoring matrix. No override of the scoring was necessary — Go leads on weighted total with no disqualifying weaknesses.
