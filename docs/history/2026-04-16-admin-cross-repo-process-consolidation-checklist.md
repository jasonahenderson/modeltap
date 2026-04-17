# Cross-Repo Process Consolidation Checklist

This checklist tracks the specific work needed to align `modeltap`,
`keyproxy/alpha`, and `meetingplaceai/alpha` around one shared process core.

## Shared Core

- [ ] Define the shared core source location
- [ ] Standardize shared contracts
- [ ] Standardize shared templates
- [ ] Standardize hook utility code
- [ ] Define a shared process config schema
- [ ] Define sync/update workflow for downstream repos

## Common Format Rules

- [ ] ADR format unified
- [ ] Feature spec format unified
- [ ] Patch format unified
- [ ] Exploration front matter unified
- [ ] Review artifact naming unified
- [ ] Peer-review identity convention unified

## Hook Guardrails

- [ ] Move hook path logic to config
- [ ] Fix `docs/implementation/patches` drift in copied patch hooks
- [ ] Decide which validations are warning-only vs blocking
- [ ] Separate repo-specific gating from shared structural validation

## Modeltap

- [x] Establish `.agents/process.md`
- [x] Establish minimal `.agents/contracts/`
- [ ] Add `.agents/templates/`
- [ ] Reduce `AGENTS.md` to router/summary role
- [ ] Reduce `CLAUDE.md` to tool-specific overlay
- [ ] Reconcile `docs/agents.md`
- [ ] Decide when to add hooks
- [ ] Keep release-level phase gating as repo-local overlay

## Keyproxy/Alpha

- [x] Canonical `.agents/` structure exists
- [ ] Fix patch hook path/config drift
- [ ] Move semver release/design-review companion rules into canonical process or
      keep them as explicit addendum
- [ ] Verify contracts/templates match the shared core once defined
- [ ] Trim any duplicated process text that survives in top-level instructions

## Meetingplaceai/Alpha

- [ ] Create `.agents/process.md`
- [ ] Create `.agents/contracts/`
- [ ] Create `.agents/templates/`
- [ ] Migrate `.claude/skills/*` content into `.agents/*`
- [ ] Update hook config to point at `.agents/*`
- [ ] Preserve existing release/review practice during migration
- [ ] Reduce `CLAUDE.md` to overlay role

## Release Process Alignment

- [ ] Decide which release rules are universal
- [ ] Keep `modeltap` release-level phase gating repo-specific unless adopted by
      the others explicitly
- [ ] Standardize release-local review placement under release `.reviews/`
- [ ] Standardize release `plan.md` and `status.md` templates where applicable

## Completion Definition

- [ ] Shared-core source exists
- [ ] All three repos consume the same template/contract vocabulary
- [ ] Hook utilities are config-driven
- [ ] No known path drift remains in hook validation
- [ ] Repo-specific differences are documented as overlays, not hidden forks
