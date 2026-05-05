# Model-Family Overlay Pack

This pack adds family-level overlays for:
- OpenAI / Codex
- Claude
- Gemini
- Qwen
- DeepSeek
- Kimi
- Gemma

These overlays are intended to be prepended to a technology-specific prompt.

## Composition
1. Model-family overlay
2. Technology-specific review or repair prompt
3. Repository / diff / code context

## Why overlays exist

Different model families fail differently:
- some over-rewrite
- some over-explain
- some over-abstract
- some overclaim certainty
- smaller local models often need denser, simpler instructions
