# Tuning modeltap for Kimi K2.6

This guide covers how to get the best performance from Kimi K2.6 through modeltap's harness, proxy, and configuration. K2.6 is architecturally different from Claude or GPT — it was trained with joint reinforcement learning on agentic tasks and supports distinct *Thinking* and *Instant* modes.

## Quick Start

```bash
# 1. Add your API key
echo 'MOONSHOT_API_KEY=sk-...' >> ~/.modeltap/.env
chmod 600 ~/.modeltap/.env

# 2. Add Moonshot to your config
cat >> ~/.modeltap/config.yaml <<'EOF'
providers:
  moonshot:
    type: moonshot
    api_key: env:MOONSHOT_API_KEY
    host: https://api.moonshot.cn
runtime:
  models:
    kimi-k2-6:
      provider: moonshot
      upstream_model: kimi-k2-6
      context_tokens: 262144
  routing:
    kimi-k2-6: kimi-k2-6
EOF

# 3. Launch the harness
modeltap harness --model kimi-k2-6
```

## Core Concepts: Thinking vs Instant

Kimi K2.6 supports two operational modes at the API level:

| Mode | Temperature | Behavior | Best For |
|---|---|---|---|
| **Thinking** | 1.0 | Full reasoning chain visible; explores alternatives | Planning, architecture, math, debugging |
| **Instant** | 0.6 | Direct response; no reasoning trace | Code generation, file edits,确定性 output |

ModelTap maps these to its own mode system:

| ModelTap Mode | Kimi Mode | Temperature |
|---|---|---|
| **Plan** (`Ctrl+P` from Build) | Thinking | 1.0 |
| **Build** (`Ctrl+P` from Plan) | Instant | 0.6 |
| **Auto** (default) | Thinking | 1.0 |

### Why 1.0 for Thinking?

K2.6 was trained with MuonClip optimizer and joint RL on 15.5T tokens. At temperature 1.0 it exhibits:
- **Coherent long-horizon reasoning** through interleaved thinking
- **Robust agentic planning** (66.1 Tau2-Bench, 76.5 ACEBench)
- **Better SWE-Bench scores** when reasoning is preserved

Lowering it below 0.8 collapses the reasoning chain and degrades performance on agentic benchmarks.

### Why 0.6 for Instant?

At 0.6, K2.6 generates deterministic code with minimal hallucination. This matches its *Instant* mode API setting where reasoning is suppressed entirely. For file edits, shell commands, or API calls, predictability beats creativity.

## Config Reference

```yaml
providers:
  moonshot:
    type: moonshot
    api_key: env:MOONSHOT_API_KEY
    host: https://api.moonshot.cn              # override for vLLM/SGLang local deploy
runtime:
  models:
    kimi-k2-6:
      provider: moonshot
      upstream_model: kimi-k2-6                 # or kimi-k2-5
      context_tokens: 262144
  routing:
    kimi-k2-6: kimi-k2-6
```

### Fields

| Field | Default | Notes |
|---|---|---|
| `type` | required | Must be `moonshot` |
| `host` | `https://api.moonshot.cn` | Set to local endpoint host for vLLM/SGLang |
| `api_key` | required | Use `env:` prefix (PATCH-0004) |
| `runtime.models.<name>.provider` | required | Routes the harness model to the `moonshot` provider endpoint |
| `runtime.models.<name>.upstream_model` | `kimi-k2-6` | Also valid: `kimi-k2-5` |
| `runtime.models.<name>.context_tokens` | `262144` | Kimi K2.6's 256k-token context window |
| `runtime.routing` | required | Maps the requested harness model name to the runtime server model entry |

## Context Window Strategy

K2.6's headline feature is its **256k token context window**. To use it effectively:

### 1. Let the harness send full attachments

Kimi handles long documents natively. Unlike Claude which benefits from chunking, K2.6 performs better with:
- Full source files inline (not chunked)
- Complete documentation pasted directly
- Entire `go.mod`, `package.json`, etc. — not summaries

**How:** The harness's `@file` resolver already sends file contents. No config needed unless you're proxying non-harness clients.

### 2. Use Plan mode for context comprehension

Switch to Plan mode (`Ctrl+P`) when:
- Onboarding to a large codebase
- Reviewing architecture documents
- Analyzing multi-file changes

The Thinking temperature (1.0) lets K2.6 explore cross-file relationships that Instant mode would miss.

### 3. Compaction thresholds

The runtime server sends `EventCompactSuggest` when context pressure hits a threshold. For K2.6:
- **Raise the threshold** — 256k means you can tolerate 80–100% context before compacting
- **Prefer `/compact` manually** over auto-compaction to preserve reasoning chains

### 4. Preserve Thinking across turns

PATCH-0008 intentionally keeps reasoning persistence out of scope. A future
provider option could preserve thinking across turns with a request body similar
to:
```yaml
extra_body:
  thinking:
    type: enabled
    keep: all
```

This retains the full reasoning chain across turns, which K2.6's evaluation showed improves coding task performance.

## Mode Workflow

Recommended harness workflow for K2.6:

```
# Step 1: Understand the task (Plan mode)
[Plan] User: "Review this codebase and suggest a refactor"
        → K2.6 thinks aloud, explores alternatives
        → You read the reasoning, ask clarifying questions

# Step 2: Generate the change (Build mode)
[Build] User: "Implement the refactor from turn 3"
        → K2.6 outputs deterministic code
        → You review diff, apply with @file attachments

# Step 3: Iterate in Build mode
[Build] User: "Fix the error in line 42"
        → Fast, direct fixes
```

### When to stay in Plan mode

- Architecture decisions
- Debugging complex race conditions
- Understanding third-party library behavior
- Designing data models or APIs

### When to switch to Build mode

- Writing functions, methods, tests
- Editing configuration files
- Shell commands (`bash` tool calls)
- Refactoring with clear scope

## Performance Tuning

### Latency vs quality tradeoffs

| Goal | Setting | Impact |
|---|---|---|
| Fastest response | Build mode | Lower latency; less thorough |
| Best reasoning | Plan mode | Default; balanced |
| Deep analysis | Plan mode with larger provider-side output allowance | Slowest; but complete |

### Streaming behavior

K2.6 streams reasoning content separately from final content in Thinking mode. The Moonshot adapter buffers reasoning silently; you will see:
1. **Visible output** — the actual response
2. **Reasoning trace** — captured in the proxy log but not shown in the harness viewport

Future harness updates may display reasoning traces inline (toggleable with `/reasoning`).

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| "Provider not found" on startup | Registry missing moonshot | Ensure PATCH-0008 applied |
| Connection timeouts | CN mainland endpoint from outside China | Set `host` to a local vLLM/SGLang endpoint |
| Gibberish at high temperature | `temperature > 1.2` in Thinking mode | Remove explicit `temperature`; let mode default apply |
| Overly terse code output | Build mode with `temperature: 0.3` | Remove explicit `temperature`; 0.6 is the Instant sweet spot |
| Long first-token latency | Large context (256k) | Normal for K2.6; latency improves with subsequent tokens |

## Benchmarks Reference

Kimi K2.6 scores with Thinking mode enabled (temperature=1.0):

| Benchmark | Score |
|---|---|
| SWE-Bench Verified | 80.2 |
| LiveCodeBench v6 | 89.6 |
| SWE-Bench Pro | 58.6 |
| Terminal-Bench 2.0 | 66.7 |
| AIME 2026 | 96.4 |
| GPQA-Diamond | 90.5 |

Instant mode (temperature=0.6) scores slightly lower on reasoning benchmarks but produces more deterministic code.

## Further Reading

- [Moonshot API Docs](https://platform.moonshot.cn/docs)
- [Kimi K2 Paper (arXiv:2507.20534)](https://arxiv.org/abs/2507.20534)
- [HuggingFace Model Card](https://huggingface.co/moonshotai/Kimi-K2.6)
- `docs/patches/0008-moonshot-provider-adapter.md` — implementation details
