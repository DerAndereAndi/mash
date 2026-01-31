# Design: Use Cases on the Wire Protocol

**Status:** Proposed
**Date:** 2026-01-31
**Decision:** DEC-055

## Problem Statement

MASH currently treats use cases as a controller-side inference mechanism. The controller probes each endpoint's features via multiple Read operations, builds a capability profile, and locally matches it against a use case registry. This has three problems:

1. **No business logic testing.** We can test protocol conformance ("does SetLimit return success?") but not use case behavior ("when the heartbeat times out, does the device enter failsafe and apply the failsafe power limit?"). For interoperability, both are required.

2. **No explicit contract.** The device never declares what it supports. A device implementing LPC has no obligation beyond implementing EnergyControl generically. There is no certifiable commitment to the LPC behavioral contract (state machine, timing, failsafe).

3. **Unnecessary probing cost.** The controller issues ~34 Read round-trips (or ~11 if batched) to discover what could be declared in a single attribute.

## Prior Art

### Matter

Matter uses a three-layer architecture:

1. **Descriptor Cluster** on every endpoint with `DeviceTypeList: [{deviceTypeID, revision}]` -- explicit wire declaration
2. **Device Library Specification** -- behavioral requirements per device type (the testable business logic layer)
3. **Two-level certification** -- cluster-level conformance tests + device-type-level behavioral tests

Matter versions at three independent levels: protocol version (ALPN), cluster revision (`ClusterRevision` global attribute), and device type revision (in `DeviceTypeList`). Device type revisions pin minimum cluster revisions.

### EEBUS

EEBUS puts use cases on the wire via `NodeManagement.UseCaseInformation` and has per-use-case test specifications (LPC test spec, CEVC test spec, etc.) that test business logic (state machines, timing, failsafe behavior).

**EEBUS weakness:** Use cases and their test specs are released independently. Different implementations run different versions of the same use case, creating a combinatorial compatibility matrix. This is the core interoperability problem MASH must avoid.

## Design Principles

Two principles guide this design:

1. **Bundled releases (anti-EEBUS).** Everything ships together. `specVersion` is the single release identifier. Use case definitions evolve within spec releases, never independently.

2. **REST API versioning for use cases.** Use case names are versioned contracts, following the same compatibility model as REST API versioning:
   - Forward-compatible changes (tighter timing, new optional behaviors, clarifications) increment the minor version
   - Breaking changes (different state machine, changed semantics) increment the major version
   - Different major versions are different contracts that share a lineage
   - A device can declare multiple major versions during a transition period

## Wire Protocol Changes

### Add `useCases` Attribute to DeviceInfo

Add a `useCases` attribute to DeviceInfo on endpoint 0:

```yaml
- id: 21
  name: useCases
  type: "[]UseCaseDecl"
  access: readOnly
  mandatory: true
  description: "Use cases supported by this device"
```

**UseCaseDecl struct (CBOR integer keys):**

| Key | Name | Type | Description |
|-----|------|------|-------------|
| 1 | endpointId | uint8 | Which endpoint implements this use case |
| 2 | name | string | Use case identifier (e.g. "LPC") |
| 3 | major | uint8 | Contract version -- breaking changes |
| 4 | minor | uint8 | Contract version -- compatible refinements |

**Example for an EVSE on specVersion 1.0:**

```cbor
useCases: [
  {1: 1, 2: "LPC", 3: 1, 4: 0},
  {1: 1, 2: "MPD", 3: 1, 4: 0},
  {1: 1, 2: "EVC", 3: 1, 4: 0}
]
```

**Example during a transition period (device supports both contract versions):**

```cbor
useCases: [
  {1: 1, 2: "LPC", 3: 1, 4: 0},
  {1: 1, 2: "LPC", 3: 2, 4: 0},
  {1: 1, 2: "EVC", 3: 1, 4: 0}
]
```

### Versioning Rules

**Major version (breaking):**

- Changes fundamental semantics (power limits -> energy budgets)
- Removes mandatory behavior
- Changes state machine structure incompatibly
- Old controllers cannot safely interact with new behavior

A major bump creates a new contract. `LPC` major 1 and `LPC` major 2 are different contracts that share a lineage. During transition, a device can declare both. Old controllers match on major 1, new controllers prefer major 2.

**Minor version (compatible refinement):**

- Tightens timing constraints (120s -> 60s heartbeat timeout)
- Adds optional behaviors
- Clarifies edge cases
- Adds new optional attributes to the use case definition

Any controller that handles major X works with any minor within that major. The minor version enables precise test selection ("run LPC 1.1 test suite") without requiring controller-side branching.

**Compatibility rules (mirror protocol versioning):**

| Situation | Behavior |
|-----------|----------|
| Same major, any minor | Always compatible. Controller handles any minor within a major. |
| Different major, device declares both | Controller picks highest major it understands |
| Different major, device declares only new | Old controller sees no match, falls back to probing |

### Relationship to specVersion

Use case versions are **bundled with spec releases**, not independent:

```
MASH 1.0:
  Features:                    Use Cases:
    DeviceInfo       rev 1       LPC  1.0
    Electrical       rev 1       LPP  1.0
    Measurement      rev 1       EVC  1.0
    EnergyControl    rev 1       COB  1.0
    Status           rev 1       MPD  1.0
    ChargingSession  rev 1       ...

MASH 1.1 (hypothetical):
  Features:                    Use Cases:
    DeviceInfo       rev 1       LPC  1.1  (tightened timing)
    Electrical       rev 2       LPP  1.0
    EnergyControl    rev 2       EVC  1.0
    ...                          COB  1.0
```

A device on specVersion 1.0 declares use case versions from the 1.0 manifest. A device on specVersion 1.1 declares versions from the 1.1 manifest. The use case version does not float independently of the spec release.

### Feature Revisions: No Change (DEC-050 Stands)

DEC-050 removed per-feature `clusterRevision` from the wire in favor of `specVersion` + `attributeList` discovery. This decision is unaffected. Features evolve additively (new optional attributes, null for deprecated). The `attributeList` global attribute handles feature-level capability discovery. Use case versions handle business logic contracts. These are orthogonal concerns.

## Use Case YAML Changes

Add `major` and `minor` fields to use case definitions:

```yaml
name: LPC
fullName: Limit Power Consumption
specVersion: "1.0"
major: 1
minor: 0
description: >
  Controller limits active power consumption of a device.
```

These fields are used by:
- `mash-ucgen` to generate the registry with version information
- `mash-test` to select the correct test suite
- The device to populate the `useCases` attribute at startup

## Business Logic Specifications

Each use case needs a behavioral specification beyond "which features are required." This is what makes the declaration meaningful and testable:

- **State machine** -- for use cases that have one (e.g. LPC: AUTONOMOUS -> CONTROLLED -> LIMITED -> FAILSAFE)
- **Timing requirements** -- failsafe durations, timeout behavior, response windows
- **Error handling** -- invalid limits, rejected commands, connection loss
- **Orchestration rules** -- multi-zone resolution, priority, override behavior

Format: prose in the use case `.md` files, with structured timing/threshold values in the YAML (machine-readable for test generation).

## Impact on Implementations

### Device

1. Evaluate which use cases it supports at startup (from feature configuration against use case definitions)
2. Populate `useCases` attribute in DeviceInfo
3. Implement use-case-specific business logic (state machines, timing, failsafe) -- required for interoperability regardless of wire presence

For 256KB MCUs: the attribute is a short CBOR array (~11 entries max). The real cost is implementing business logic, which is required regardless.

### Controller

1. Read `useCases` from DeviceInfo in a single Read (replaces ~34 probing reads)
2. Match on name + highest supported major version
3. Optionally verify the declaration by probing features (defense in depth)
4. Use the declaration for subscription setup

Existing `pkg/usecase/` discovery code remains as verification/fallback.

### Test Infrastructure

1. `mash-test` reads `useCases` from device to select test suites
2. Use case version determines which behavioral test suite to run (LPC 1.0 vs LPC 1.1)
3. PICS still needed for optional feature declarations within a use case
4. `mash-pics` can auto-generate from device's `useCases` + `attributeList`

## Resolved Questions

1. **Use cases per-endpoint or per-device?** Per-device (on endpoint 0 DeviceInfo) with `endpointId` field per entry. Keeps the device model simple -- one Read gives the full picture.

2. **Feature revisions on wire?** No. DEC-050 stands. `attributeList` handles feature capability discovery. Use case versions handle business logic contracts. Orthogonal.

3. **Independent use case releases?** No. All use case versions are bundled into spec releases. The version on the wire is a contract declaration, not an independent release identifier.

## Open Questions

1. **Behavioral spec format.** How to define use case business logic: prose in markdown, structured YAML state machines, or both? Structured YAML enables test auto-generation but adds spec authoring complexity.

2. **Backward compatibility.** Devices running current MASH 1.0 without `useCases` attribute. Controllers should fall back to probing when the attribute is absent.
