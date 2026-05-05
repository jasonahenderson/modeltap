# Prompt Pack: Technology-Specific Review and Repair

This pack contains one Markdown file per technology:
- Go
- PostgreSQL
- Python
- TypeScript/Bun

Each technology file includes four prompt variants:
- large-model review
- small-model review
- large-model repair
- small-model repair

## Design intent

These prompts are for:
- critical analysis of code quality
- codebase assessment
- review of engineering decisions
- repair / rewrite with restraint

They are **not** lint prompts.

## Recommended usage

Use review and repair as separate invocations:
1. Review and score the code or codebase
2. Feed the findings into a repair prompt
3. Compare patch quality, minimality, and risk

## Files
- go-code-review-prompts.md
- postgresql-review-prompts.md
- python-review-prompts.md
- typescript-bun-review-prompts.md
