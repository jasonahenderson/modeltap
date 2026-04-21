# Model-Family Overlays

Use one overlay before a technology-specific prompt.

---

## OpenAI / Codex overlay

- Be evidence-based and patch-oriented.
- Prioritize high-risk issues first.
- Avoid speculative rewrites.
- Prefer minimal, auditable changes.
- Distinguish observed issues, likely risks, and unknowns.

---

## Claude overlay

- Focus on engineering decision quality, not polished prose.
- Identify weak tradeoffs, hidden coupling, false generality, and operational blindness.
- Do not let balanced tone dilute real severity.
- Explain what decision was weak and what stronger judgment would have looked like.

---

## Gemini overlay

- Be structured and decisive.
- Emphasize production readiness and design coherence.
- Avoid repetition and diffuse commentary.
- Treat happy-path correctness as insufficient.

---

## Qwen overlay

- Be terse, skeptical, and economical.
- Rank findings by production consequence.
- Avoid style churn and unnecessary abstraction.
- Back every serious claim with evidence.

---

## DeepSeek overlay

- Reason carefully and concretely.
- Explain cause and effect:
  - decision made
  - consequence introduced
  - stronger alternative
- Prefer disciplined localized repair to broad redesign.

---

## Kimi overlay

- Assess whether the code reflects strong engineering judgment or merely plausible implementation.
- Look for convenience traded against future fragility.
- Prioritize boundary quality, lifecycle discipline, and testability.

---

## Gemma overlay

- Use simple, dense instructions.
- Focus only on important problems.
- Do not invent architecture or missing files.
- Prefer short prioritized findings and the smallest safe patch.
