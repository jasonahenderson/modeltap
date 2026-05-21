---
name: adr
description: Create or discuss an Architecture Decision Record (ADR) using MADR format with weighted decision drivers and scoring matrices. Use when the user wants to make, document, or evaluate an architectural decision.
argument-hint: [decision-title]
---

# Architecture Decision Record (ADR) Skill

You help create Architecture Decision Records using the MADR (Markdown Any Decision Records) format, extended with a weighted scoring matrix process.

## Process

Every ADR follows this four-step process. Do NOT skip steps. Present each step to the user and get confirmation before proceeding to the next.

**IMPORTANT: Steps A and B must both be presented to the user BEFORE scoring.** Present Step A (decision drivers), then immediately present Step B (options considered) in the same response or the next. Then STOP and wait for the user to review and approve both the drivers and options together before proceeding to Step C (scoring). The user may want to adjust drivers after seeing options, or adjust options after seeing drivers — the two inform each other.

### Step A: Decision Drivers

1. Identify 5–10 decision drivers relevant to this specific decision.
2. Assign each driver a weight from 1–5:
   - **5** = Critical — a wrong choice here would be a project-threatening mistake
   - **4** = Important — significantly affects project success
   - **3** = Moderate — meaningful but not decisive on its own
   - **2** = Minor — nice to have, tiebreaker-level
   - **1** = Negligible — worth noting but rarely decisive
3. Include a brief rationale for each driver explaining why it matters and why it has that weight.
4. Present the drivers as a table.

### Step B: Options Considered

1. Identify 3–5 realistic options. Include the obvious choices and at least one less conventional alternative.
2. Present a brief (1–2 sentence) description of each option.
3. After presenting BOTH Steps A and B, ask the user if they want to adjust any drivers (add, remove, re-weight) or options (add, remove, modify) before proceeding to scoring.

### Step C: Scoring Matrix

1. Score each option against each driver on a 1–5 scale:
   - **5** = Excellent — best-in-class for this driver
   - **4** = Good — strong with minor limitations
   - **3** = Adequate — meets needs but not a strength
   - **2** = Weak — notable limitations
   - **1** = Poor — significant concern or blocker
2. Calculate weighted totals: sum of (weight x score) for each option.
3. Present as a table with a "Weighted Total" row.
4. Provide a brief justification for each score, grouped by option.

### Step D: Decision and Documentation

1. Identify the option with the highest weighted score.
2. If recommending that option: state the decision, note the margin, and explain why the score reflects reality.
3. If recommending a DIFFERENT option than the highest scorer: explicitly state this, explain the quantitative result, and provide clear reasoning for the override (e.g., a low score on a critical driver that the weighting doesn't fully capture, strategic considerations, etc.).
4. Write the full ADR document using the MADR template below.

## MADR Template

Use this exact structure when writing the ADR document. All sections are required unless marked optional.

```markdown
---
status: {proposed | accepted | deprecated | superseded by ADR-NNNN}
date: {YYYY-MM-DD}
decision-makers: {list of people involved}
---

# {Short title describing the decision}

## Context and Problem Statement

{2–4 sentences describing the context, the problem, and why a decision is needed.}

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – {Driver name} ({weight}):** {Rationale for this driver and its weight.}
* **D2 – {Driver name} ({weight}):** {Rationale.}
{...repeat for all drivers}

## Considered Options

* {Option 1}
* {Option 2}
* {Option 3}
{...}

## Decision Outcome

Chosen option: **{Option name}**, because {1–2 sentence summary of why}.

{If the chosen option is NOT the highest weighted scorer, add a paragraph here explicitly stating the weighted score result and the reasoning for overriding it.}

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver | Weight | {Option 1} | {Option 2} | {Option 3} | ... |
|--------|--------|------------|------------|------------|-----|
| D1: {name} | {w} | {score} | {score} | {score} | ... |
| ...    |        |            |            |            |     |
| **Weighted Total** | | **{total}** | **{total}** | **{total}** | ... |

### Scoring Justification

#### {Option 1} ({total})

* **D1 ({score}):** {Why this score.}
* **D2 ({score}):** {Why this score.}
{...}

#### {Option 2} ({total})

{...repeat for each option}

### Consequences

* Good, because {positive consequence}.
* Good, because {another positive consequence}.
* Neutral, because {neutral observation}.
* Bad, because {negative consequence or tradeoff}.
{...}

### Confirmation

{How will we verify this decision was correct? What would trigger revisiting it?}

## More Information

{Any additional context, links, references, or notes about whether the decision aligned with or overrode the scoring matrix.}
```

## File Naming and Location

- Store ADRs in `.sdlc/adr/`
- Name files as `NNNN-short-kebab-case-title.md` (e.g., `0001-programming-language.md`)
- Number sequentially; check the directory for the next available number using: !`ls .sdlc/adr/ 2>/dev/null | sort -r | head -1`
- Set status to `accepted` when writing a final decision, `proposed` when the user wants to review further

## Decision Title

The user wants to create an ADR about: **$ARGUMENTS**

Begin with Step A. If `$ARGUMENTS` is empty, ask the user what decision they need to make.
