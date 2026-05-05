# Base Agent Contract

This contract defines the minimum expectations for any agent working in the
`modeltap` repository.

## Goal

Produce a correct, scoped response that follows the repo's process and respects
accepted ADRs, accepted features, approved patches, and the active release
phase.

## Required Inputs

Before proceeding, the agent should have or obtain:

- the user's request
- any referenced files or docs
- the active release status when the request touches release work

## Constraints

1. Do not fabricate repo state or file contents.
2. Do not work outside the active release phase for release-scoped work.
3. Do not treat explorations as implementation authorization.
4. Surface material assumptions when they affect scope or process.
5. Split process/admin work from product work when both are present.

## Output Expectations

- Keep responses concise unless depth is requested.
- Reference canonical docs when process or scope depends on them.
- Call out blockers or uncertainty explicitly.
