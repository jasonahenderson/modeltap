# PostgreSQL: Model-Family Prompt Variants

This file combines model-family guidance with schema design, migrations, queries, and database operations review and repair use cases.

## Usage pattern

Select one model family section, then choose:
- large review
- small review
- large repair
- small repair

---

## OpenAI / Codex
- Best for disciplined review plus patch-oriented repair
- Emphasize evidence, severity, confidence, and minimal safe changes

### Large review
Review schema design, migrations, queries, and database operations with a principal-engineer lens. Prioritize correctness, production risk, maintainability, and explicit evidence.

### Small review
Review schema design, migrations, queries, and database operations concisely. Focus on major issues only. No cosmetic commentary.

### Large repair
Repair schema design, migrations, queries, and database operations with the smallest auditable change set. Preserve interfaces unless a bug fix requires change.

### Small repair
Produce a localized fix or patch sketch for the highest-priority problem.

---

## Claude
- Best for design-quality critique and decision analysis
- Emphasize tradeoffs, hidden coupling, and weak abstractions

### Large review
Assess the quality of the engineering decisions in schema design, migrations, queries, and database operations. Explain where the design choices are weak or fragile.

### Small review
List the top flawed decisions and why they matter.

### Large repair
Repair only the parts whose underlying design decisions are causing risk or instability.

### Small repair
Make a restrained fix and explain the decision improvement.

---

## Gemini
- Best for structured production-readiness analysis
- Emphasize system coherence and safeguards

### Large review
Review schema design, migrations, queries, and database operations as production code. Highlight missing safeguards, hidden assumptions, and maintenance traps.

### Small review
Prioritize the top production risks in schema design, migrations, queries, and database operations.

### Large repair
Improve correctness and maintainability without unnecessary rewrite.

### Small repair
Deliver a small safe patch and state residual risk.

---

## Qwen
- Best for skeptical, economical assessment
- Emphasize consequence-ranked findings

### Large review
Review schema design, migrations, queries, and database operations with sharp prioritization. Focus on risk, fragility, and unnecessary complexity.

### Small review
List the highest-consequence issues first.

### Large repair
Perform minimal robust repair. Avoid abstraction unless it clearly removes risk.

### Small repair
Patch the critical issue with the least disruptive change.

---

## DeepSeek
- Best for careful cause-and-effect reasoning
- Emphasize decision -> consequence -> alternative

### Large review
Assess schema design, migrations, queries, and database operations by identifying concrete design choices and their downstream consequences.

### Small review
List the main flawed choices and the defects or risks they create.

### Large repair
Repair the root cause with a disciplined, auditable patch plan.

### Small repair
Provide a direct fix and note any structural follow-up.

---

## Kimi
- Best for judgment-quality analysis
- Emphasize whether the code is robust or merely plausible

### Large review
Assess whether schema design, migrations, queries, and database operations reflects strong engineering judgment, especially around boundaries, lifecycle, and future change.

### Small review
List where the code takes weak shortcuts.

### Large repair
Repair the code in a way that improves long-term robustness, not just local appearance.

### Small repair
Apply a small fix and note the stronger design choice behind it.

---

## Gemma
- Best for dense, simple instructions
- Emphasize major issues only

### Large review
Review schema design, migrations, queries, and database operations for correctness, reliability, maintainability, and operational risk.

### Small review
Find only the most important problems.

### Large repair
Repair the most important problem with minimal change.

### Small repair
Give a localized patch or patch sketch.
