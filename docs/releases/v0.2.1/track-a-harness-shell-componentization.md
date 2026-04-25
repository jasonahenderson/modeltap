# Track A: FEAT-0037 Harness Shell Componentization

**Release:** v0.2.1
**WU Range:** WU-097 through WU-102 (6 work units)
**Depends on:** FEAT-0037 accepted; PATCH-0015 approved; shell spike behavior established
**Can parallelize with:** none

## Planning and Design

### WU-097: Refactor Plan and Migration Sequencing
**Size:** Medium | **Dependencies:** —

Define the componentization strategy before detailed package/API design begins.
This WU exists because the extraction has two design layers: first the refactor
plan, then the detailed implementation design under that plan.

Owns:

- package split strategy
- migration sequencing from `internal/harnessspike/`
- behavior-parity constraints
- extraction seams and risk containment
- host/runtime separation goals

**Done:** Accepted refactor plan identifies the package boundary, migration
order, non-goals, and the exact parity rules the later design and
implementation must preserve.

### WU-098: Shell Component API and Package-Boundary Design
**Size:** Medium | **Dependencies:** WU-097 | **Parallelizes with:** WU-099

Design the reusable shell package itself.

Owns:

- exported package shape
- shell-owned state and responsibilities
- action/event contract
- transcript/composer/token/permission state boundaries
- file/preview boundary from shell to host
- serialization-friendly type strategy

**Done:** A detailed design document specifies the component API, package
structure, key types, ownership rules, and invariants required to preserve
`FEAT-0037`.

### WU-099: Modeltap Host Adapter and Integration Design
**Size:** Medium | **Dependencies:** WU-097 | **Parallelizes with:** WU-098

Design the modeltap-specific side of the boundary.

Owns:

- host adapter package layout
- submission / stream / permission / preview integration points
- shell-native vs host-native command routing
- integration with existing harness runtime state
- migration path away from spike/demo behavior

**Done:** A detailed design document specifies how modeltap instantiates the
component, dispatches shell actions, feeds host events back, and preserves
current behavior during migration.

## Implementation

### WU-100: Behavior-Preserving Shell Extraction Implementation
**Size:** Large | **Dependencies:** WU-098, WU-099 | **Parallelizes with:** WU-101, WU-102

Implement the extracted shell component and move the current spike behavior
behind the defined API boundary without redesigning the shell UX.

Owns:

- reusable shell package creation
- movement of shell-local state/rendering logic out of spike package
- replacement of direct runtime/demo coupling with host-driven actions/events
- preservation of transcript, queue, permission, and scroll behavior

**Done:** The reusable shell package exists, current shell behavior matches the
spike contract, and modeltap can drive it through the new API.

### WU-101: Developer Documentation and Embedding Examples
**Size:** Medium | **Dependencies:** WU-098, WU-099 | **Parallelizes with:** WU-100, WU-102

Produce developer-facing documentation for using the component.

Owns:

- package-level developer docs
- ownership/boundary documentation
- minimal embedding example
- host integration examples for submit/stream/permission/preview flows
- extraction guidance for later promotion into its own repository

**Done:** Another engineer can embed the component by following the docs
without reading the old spike implementation.

### WU-102: Parity and Regression Test Sweep
**Size:** Medium | **Dependencies:** WU-100 | **Parallelizes with:** WU-101

Validate that componentization preserved the behavior contract.

Owns:

- parity tests for transcript/composer/queue behavior
- permission-flow regression coverage
- wrapping and scroll behavior coverage
- extraction-boundary integration tests where needed

**Done:** Automated tests cover the preserved shell behavior and guard against
regression during future extraction work.

## Critical Path

```
097 → 098/099 → 100 → 102
```

WU-101 runs alongside implementation once the API and host-adapter designs are
accepted.
