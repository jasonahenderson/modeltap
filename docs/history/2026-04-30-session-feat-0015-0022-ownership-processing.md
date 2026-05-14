# 2026-04-30 — FEAT-0015..0022 Ownership Review Processing

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

Processed `docs/features/.reviews/syntheses/0015-0022-ownership-claude.md`.

Changes made:

- Clarified BFF-authoritative run status and stage transitions in FEAT-0016.
- Clarified BFF-authoritative attachment state, single attached client,
  observer clients, and disconnect grace-period behavior in FEAT-0017.
- Clarified permission decision reporting, durable audit authority,
  policy-version checks, and revocation behavior in FEAT-0021.
- Added workspace lifecycle ownership, cleanup, orphan recovery, missing
  workspace handling, and remote workspace ownership in FEAT-0015.
- Added disconnected-executor rules in FEAT-0015 and FEAT-0017.
- Added local artifact content reachability semantics in FEAT-0020.
- Added BFF-side server-safe tool boundary in FEAT-0021.
- Added prompt visibility authority in FEAT-0018.

All eight findings were accepted and dispositioned in the review artifact.
