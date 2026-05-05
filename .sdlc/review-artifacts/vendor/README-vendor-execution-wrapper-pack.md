# Vendor Execution Wrapper Pack

This pack contains final-layer execution wrappers for:
- OpenAI / Codex
- Claude
- Gemini
- Qwen
- DeepSeek
- Kimi
- Gemma

These wrappers tune review and repair behavior for each model family.

## Composition order
1. Vendor execution wrapper
2. Technology-specific review or repair prompt
3. Code / diff / repo context

## Design goal

Reduce model-specific failure modes such as:
- hallucinated architecture
- overconfident claims
- needless rewrites
- style chatter replacing substance
- speculative performance or security findings
