# FEAT-0022 Findings (Claude)

- Feature: `.sdlc/features/0022-memory-routing-and-workflow-extensions.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 3
- blocking: 1
- significant: 1
- advisory: 1
- top_line: The "use run artifacts as the bridge between memory, routing, and workflow extensions" thesis is the right framing. Two material issues: acceptance is gated on three `proposed` (not `accepted`) features, and §Workflow Extensions reframes how skills and agent teams behave without coordinating revisions to FEAT-0012/0013. The first is process-blocking per CLAUDE.md; the second risks two specs disagreeing about what skills and teams are.

## Findings

### F1 — Blocking

**Reviewer:** Acceptance Sequencing

**Affected sections:** Frontmatter (depends-on), Success Criteria

**Summary:** FEAT-0022 depends on three `proposed`-status features that have not been accepted.

**Detail:** FEAT-0022:0010-0015 lists `depends-on: FEAT-0011, FEAT-0012, FEAT-0013, FEAT-0016, FEAT-0020`. FEAT-0011 (Knowledge Integration), FEAT-0012 (Skills), and FEAT-0013 (Agent Teams) are all `status: proposed` per `.sdlc/features/README.md`. CLAUDE.md states explicitly: "only `accepted` features drive work." That makes FEAT-0022's path to acceptance contingent on accepting three other specs first. The dependency is logically correct; the issue is that FEAT-0022 cannot be accepted (and therefore cannot drive work) until that ordering resolves.

**Recommendation:** Either (a) explicitly note in FEAT-0022 §Open Questions or §Success Criteria that acceptance is gated on FEAT-0011/0012/0013 acceptance and that the umbrella plan should sequence those first, or (b) scope FEAT-0022 down to a slice that only requires already-accepted features (e.g. memory and routing decisions tied to run artifacts, deferring the Workflow Extensions section until 0012/0013 are accepted).

### F2 — Significant

**Reviewer:** Charter Coordination

**Affected sections:** Workflow Extensions, Non-Goals

**Summary:** §Workflow Extensions reframes how skills and agent teams operate without coordinating revisions to FEAT-0012/0013.

**Detail:** FEAT-0022:0086-0096 says "skills, hooks, slash commands, and agent teams should align with workflow contracts" and "agent teams execute as runs with multiple coordinated agents." That is a meaningful reframing of what skills and agent teams *are*, not just an addition. FEAT-0012 currently treats skills as a harness-side capability; FEAT-0013 treats agent teams as a multi-agent orchestration concept. Under FEAT-0022, both become specializations of the durable-run model. The non-goals say "this feature does not replace FEAT-0012 or FEAT-0013," but the relationship is more than additive — it's a re-anchoring. If FEAT-0012/0013 land before FEAT-0022 and use the older framing, downstream design will conflict.

**Recommendation:** Add a §Cross-Feature Impact subsection naming exactly which sections of FEAT-0012 and FEAT-0013 are reframed by this spec, and either revise those specs in lockstep or sequence FEAT-0022 to be accepted before FEAT-0012/0013 are accepted. The peer-review findings on FEAT-0012 (`.sdlc/features/.reviews/0012-skills-and-agent-teams-findings.md` F1) already flag that FEAT-0012 should be split — that work and FEAT-0022 should converge on the same model.

### F3 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** See FEAT-0015 review F4.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Moved FEAT-0011/0012/0013 from hard dependencies to related and added an acceptance-gating/phasing note. |
| F2 | accepted | Added Cross-Feature Impact for FEAT-0012 skills and FEAT-0013 teams. |
| F3 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
