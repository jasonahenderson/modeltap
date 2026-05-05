# 2026-04-29 — Patch Frontmatter and Artifact Grouping

## Context

Standardized patch metadata with the rest of the repo artifact system and added
a universal grouping mechanism for feature, patch, ADR, and exploration
families.

## Work Completed

- Updated `.agents/process.md` with artifact grouping metadata:
  `parent`, `series`, `series-role`, and `series-order`.
- Converted all existing patch docs from inline bold metadata to YAML
  frontmatter.
- Updated `docs/patches/README.md` to define YAML frontmatter for patches.
- Updated feature, exploration, and ADR README docs with the shared grouping
  fields.
- Kept artifact IDs monotonic and hierarchy-free; grouping now lives in
  metadata instead of suffixes such as `FEAT-0015a`.

## Notes

- This was an admin/process/doc-format change only.
- Product feature drafting was committed separately under `FEAT-0015`.
