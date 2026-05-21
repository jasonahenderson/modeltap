# 2026-04-30 — v0.3.x Design Readiness Check

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

After processing the v0.3.x tech-lead review findings, the release plans now
allow v0.3.0 design to proceed only through an explicit release-open `ADMIN:`
commit that satisfies the new gates.

Current readiness:

- v0.3.0 Phase 1 is still not opened.
- FEAT-0015/0016/0017 remain draft unless separately promoted.
- If design starts before feature acceptance, the release-open commit must
  record an explicit design-against-draft exception.
- The release-open commit must reconcile the v0.2.x release-status mismatch or
  name the committed BFF/harness contracts v0.3.0 design may depend on.
- Phase 3 remains blocked until the relevant feature scope and run-runtime ADR
  are accepted.

Recommended next step: open v0.3.0 Phase 1 with an ADMIN commit that records the
design-against-draft exception and names the stable v0.2.x contracts, then draft
all WU-108 through WU-117 designs before Phase 2 review.
