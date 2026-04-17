# Modeltap Process Structure Adoption Checklist

This checklist compares `modeltap`'s current process structure against the
`keyproxy/alpha` layout and records what should be adopted here.

## Structure to Adopt

- [x] Create a canonical tool-agnostic process file at `.agents/process.md`
- [x] Create a contract directory at `.agents/contracts/`
- [x] Add a base contract for shared expectations
- [x] Add an agent-team contract for role/workflow expectations
- [ ] Add additional contracts only when they have clear ongoing value
- [ ] Decide whether to add `.agents/templates/` now or defer
- [ ] Add a concise precedence rule saying `.agents/*` is canonical

## Instruction File Changes

- [ ] Reduce `AGENTS.md` to a concise entrypoint that points to `.agents/*`
- [ ] Reduce `CLAUDE.md` to tool-specific guidance and precedence only
- [ ] Remove duplicated taxonomy and commit-policy text from top-level files
- [ ] Decide whether `docs/agents.md` becomes a pointer doc or remains a human
      overview

## Process Content to Centralize

- [x] Artifact taxonomy
- [x] Commit prefixes and body requirements
- [x] Release-level phase rules for `v0.2.0`-style execution
- [x] Review artifact placement rules
- [x] History logging expectations
- [ ] Session resumption procedure
- [ ] Review naming conventions where they currently exist only in prose

## Modeltap-Specific Rules That Must Be Preserved

- [x] Release phases are release-level, not WU-level
- [x] Phase 1 requires design coverage for all WUs across all tracks
- [x] Phase 2 is user-directed review only
- [x] Phase 3 begins only after Phase 2 findings are processed
- [x] Phase transitions are explicit `ADMIN:` commits
- [x] Current phase lives in `docs/releases/<version>/plan.md`

## Optional Later Adoption from Keyproxy/Alpha

- [ ] Add document-review contract(s) if review workflows become more formal
- [ ] Add reusable templates for features, ADRs, or reviews if authoring drift
      becomes a problem
- [ ] Add hook-specific companion docs if repo hooks or slash-command flows are
      introduced here
- [ ] Add stronger precedence language for imported skills or external guidance

## Migration Sequence

- [x] Establish `.agents/` structure
- [ ] Repoint `AGENTS.md`
- [ ] Repoint `CLAUDE.md`
- [ ] Reconcile `docs/agents.md`
- [ ] Review for duplication after the first pass
- [ ] Decide whether to expand contracts/templates in a follow-up `ADMIN:` task
