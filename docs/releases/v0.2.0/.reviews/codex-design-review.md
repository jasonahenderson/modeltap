# v0.2.0 Design Review

Reviewed:

- `docs/releases/v0.2.0/designs/2026-04-16-design-protocol-types-040-041-093.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-provider-formatting-042-043-044.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-storage-045-091-096.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-bff-foundation-046-047-048-049.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-sessions-conversation-050-051-052.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-model-config-routing-057-058-059-060.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-context-diagnostics-recovery-061-062-063-064.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-cli-ollama-history-065-066-091.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-bubbletea-scaffold-068-069-070-071-072.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-protocol-client-073-074.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-tool-framework-075-076-077-078-079.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-harness-features-080-086-092.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-track-integration-tests-067-087.md`
- `docs/releases/v0.2.0/designs/2026-04-16-design-integration-track-088-090-094-095.md`

Cross-checked against:

- `docs/features/0008-bff-server.md`
- `docs/features/0009-terminal-harness.md`
- `docs/adr/0006-multi-provider-support.md`
- `docs/adr/0013-terminal-ui-framework.md`

## Findings Summary

- 5 findings total
- 5 blocking

## Findings

### 1. Blocking — the storage/recovery design breaks the accepted reconnect contract for multi-model turns

The storage bundle explicitly chooses to keep multi-model branch state in memory only and to report a cancelled turn after a BFF restart (`docs/releases/v0.2.0/designs/2026-04-16-design-storage-045-091-096.md:215`, `:223`, `:229`). That conflicts with the accepted FEAT-0008 contract that reconnect after a BFF crash preserves the final in-flight turn state and that `session.sync` for multi-model turns returns per-branch completed/failed/pending status (`docs/features/0008-bff-server.md:457`, `:495`, `:837`, `:1225`, `:1227`).

As designed, a network drop can recover branch state, but a BFF restart cannot. The feature contract does not make that distinction; it explicitly names BFF crash/restart as a supported recovery case. This is a release-level behavioral regression, not an implementation detail.

### 2. Blocking — `capabilities.request` cannot work as written because bundle 4 both requires and forbids re-registration on the same connection

Bundle 4 says `capabilities.register` may only be called once per connection and that any later call is rejected outside `ConnRegistering` (`docs/releases/v0.2.0/designs/2026-04-16-design-bff-foundation-046-047-048-049.md:538`). The same bundle then defines `capabilities.request` as a server-initiated request for the harness to send `capabilities.register` again, with the server replacing the old catalog atomically (`:544`, `:564`).

Those two rules are mutually exclusive. Without resolving the state-machine rule, either `capabilities.request` is dead on arrival or replay prevention is wrong. Because capability refresh is part of the accepted protocol surface, this needs a single legal path before Phase 3.

### 3. Blocking — the protocol client expects the wrong wire shape for `capabilities.register`

The protocol-types and BFF-foundation bundles agree that `capabilities.register` returns `protocol.CapabilitiesRegisterResponse` with `registered`, `server_capabilities`, and optional `rejected` fields (`docs/releases/v0.2.0/designs/2026-04-16-design-protocol-types-040-041-093.md:283`, `docs/releases/v0.2.0/designs/2026-04-16-design-bff-foundation-046-047-048-049.md:516`). But the harness protocol-client bundle defines a different `RegisterResponse` with top-level `negotiated_version`, `server_version`, `max_frame_size`, and `max_attachment_size` (`docs/releases/v0.2.0/designs/2026-04-16-design-protocol-client-073-074.md:185`).

That is not a harmless local alias; it is a different JSON shape. If implemented as written, the harness will fail to decode the server response correctly and will drop protocol data the shared contract requires, including rejected-tool reporting and the full `server_capabilities` object.

### 4. Blocking — bundle 4 invents `MT-CONN-013`, but the shared diagnostics contract only defines `MT-CONN-001` through `MT-CONN-012`

The BFF foundation design says oversize attachments are rejected with diagnostic `MT-CONN-013` (`docs/releases/v0.2.0/designs/2026-04-16-design-bff-foundation-046-047-048-049.md:589`). The protocol-types bundle and the diagnostics bundle both define the taxonomy as exactly 12 codes, `MT-CONN-001` through `MT-CONN-012` (`docs/releases/v0.2.0/designs/2026-04-16-design-protocol-types-040-041-093.md:328`, `docs/releases/v0.2.0/designs/2026-04-16-design-context-diagnostics-recovery-061-062-063-064.md:212`). FEAT-0008 also enumerates only those 12 (`docs/features/0008-bff-server.md:503`).

This is a cross-bundle contract break. Either attachment-too-large must map onto an existing diagnostic model, or the shared taxonomy has to be amended everywhere. Leaving it split guarantees inconsistent tests and user-visible codes.

### 5. Blocking — the provider interface is not actually stable across bundles; bundle 10 adds `ParseStreamEvent` outside the shared contract bundle

The provider-formatting bundle is the shared contract bundle for the `Provider` interface and its ADR-0006 amendment. Its interface design includes `FormatMessages` and `FormatToolDefinitions`, but not stream-event parsing (`docs/releases/v0.2.0/designs/2026-04-16-design-provider-formatting-042-043-044.md:84`). Bundle 10 later adds a new `ParseStreamEvent` interface method and says it “will be added as an amendment” (`docs/releases/v0.2.0/designs/2026-04-16-design-streaming-prompts-cost-053-054-055-056.md:128`, `:509`).

That means the release is not actually done with Phase 1 interface design on a shared contract consumed by multiple providers. Phase 3 would otherwise discover a new mandatory provider method mid-implementation, which is exactly the silent contract drift the phased process is meant to prevent.

## Residual Risk

I did not find another issue as severe as the five above. The remaining designs are mostly coherent once these cross-bundle contract breaks are resolved.
