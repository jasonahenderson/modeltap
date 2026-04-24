# Running `kimi-k2.6:cloud` in Claude Code via Ollama

Use an Ollama cloud model as the backend for Claude Code, with tool use, streaming, and thinking blocks working as normal. Ollama exposes a native Anthropic-compatible API on port 11434, so no translation proxy is required.

## Prerequisites

- Ollama installed and recent enough to include the Anthropic-compat endpoint (shipped late 2025).
- An Ollama account (cloud models require sign-in).
- Claude Code CLI installed.

## 1. Sign in and start Ollama

```bash
ollama signin
ollama --version       # confirm a recent build
ollama serve           # or leave the menu-bar/desktop app running
```

## 2. Environment variables

Add to `~/.zshrc`, `~/.bashrc`, or a project `.envrc`:

```bash
export ANTHROPIC_BASE_URL=http://localhost:11434
export ANTHROPIC_AUTH_TOKEN=ollama
export ANTHROPIC_API_KEY=""
```

Notes:
- `ANTHROPIC_API_KEY` must be **exported as an empty string**, not unset. If the variable is missing entirely, Claude Code falls back to its real Anthropic credentials.
- `ANTHROPIC_AUTH_TOKEN=ollama` is the sentinel value Ollama's compat layer expects.

## 3. Launch Claude Code against the model

```bash
claude --model kimi-k2.6:cloud
```

Or use Ollama's one-shot launcher:

```bash
ollama launch claude --model kimi-k2.6:cloud
```

## 4. Raise the context window (required for tools)

Claude Code's system prompt is large — tool schemas, `CLAUDE.md`, memory, and subagent prompts all consume tokens. Ollama recommends **≥ 64k** context; in practice 128k is safer. Without this, tool calls get truncated or malformed.

Create a Modelfile once:

```
FROM kimi-k2.6:cloud
PARAMETER num_ctx 131072
PARAMETER temperature 0.6
PARAMETER top_p 0.95
```

Build and use it:

```bash
ollama create kimi-cc -f Modelfile
claude --model kimi-cc
```

## 5. Verify

Inside a Claude Code session:

1. Run `/model` — it should show `kimi-k2.6:cloud` (or `kimi-cc`).
2. Ask Claude to read a file. If the `Read` tool fires and returns contents, Anthropic ↔ Ollama tool translation is working.
3. Ask a multi-step task to confirm `Task`, `Edit`, `Bash`, etc. all dispatch.

## Recommended inference parameters

| Option        | Value                         | Why                                          |
|---------------|-------------------------------|----------------------------------------------|
| `num_ctx`     | `131072` (up to `262144`)     | Tool schemas + memory eat tokens fast        |
| `temperature` | `0.6`                         | Kimi's tuned default; `0.7` for creative     |
| `top_p`       | `0.95`                        | Standard                                     |
| `num_predict` | `-1`                          | Generate until EOS                           |

## Gotchas

- **Model must be tool-capable.** `kimi-k2.6:cloud` is tagged `vision tools thinking cloud` — it supports tool use. Non-`tools` models will fail when Claude Code sends tool schemas.
- **Rate limits** are on your Ollama account tier, not Anthropic's. Watch for 429s from `localhost:11434`.
- **Subagents (`Task` tool)** work but spawn fresh requests and consume context quickly. Keep `num_ctx` high.
- **Thinking blocks** surface correctly because Kimi is tagged `thinking`.
- **Silent tool failures** usually mean either a schema-translation bug in Ollama or insufficient `num_ctx`. Debug with:
  ```bash
  OLLAMA_DEBUG=1 ollama serve
  ```
- **Switching back to Anthropic**: `unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN ANTHROPIC_API_KEY` (or open a new shell without the exports).
- **Offline use**: `:cloud` variants require network. For fully local, pull the non-`:cloud` tag — but the full weights need serious VRAM.

## Troubleshooting

| Symptom                                | Likely cause                                         |
|----------------------------------------|------------------------------------------------------|
| Claude Code uses real Anthropic API    | `ANTHROPIC_API_KEY` is unset (must be empty string)  |
| "Tool not supported" error             | Model doesn't have `tools` tag, or `num_ctx` too low |
| Truncated / malformed tool calls       | `num_ctx` too low — raise it                         |
| 401/403 from `localhost:11434`         | Not signed in — run `ollama signin`                  |
| 429 errors                             | Ollama account tier rate limit                       |
| Model not found                        | `ollama pull kimi-k2.6:cloud` first                  |

## References

- [Claude Code — Ollama docs](https://docs.ollama.com/integrations/claude-code)
- [Claude Code with Anthropic API compatibility — Ollama blog](https://ollama.com/blog/claude)
- [`kimi-k2.6:cloud` — Ollama library](https://ollama.com/library/kimi-k2.6:cloud)
