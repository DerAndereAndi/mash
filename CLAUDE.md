# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MASH (Minimal Application-layer Smart Home Protocol) is a lightweight protocol designed as a replacement for EEBUS SHIP/SPINE. This repository contains both the protocol specification and a Go reference implementation.

**Key design goals:**
- Simple and deterministic (no ambiguities, no race conditions)
- Lightweight (targets 256KB RAM MCUs like ESP32)
- Matter-inspired but independent (4 operations: Read, Write, Subscribe, Invoke)

## Repository Structure

```
mash/
├── docs/                        # Protocol specification
│   ├── protocol-overview.md       # Canonical architecture guide
│   ├── decision-log.md            # Design decisions (DEC-XXX)
│   ├── transport.md
│   ├── security.md
│   ├── discovery.md
│   ├── interaction-model.md
│   ├── multi-zone.md              # Multi-zone architecture
│   ├── stack-architecture.md
│   ├── features/                  # Feature specifications
│   │   ├── README.md                # Feature registry, numbering, composition
│   │   ├── <feature>/1.0.yaml      # Machine-readable definitions (code gen source)
│   │   ├── <feature>.md            # Human-readable specs
│   │   └── protocol-versions.yaml  # Feature/endpoint type registries
│   ├── usecases/                  # Use case definitions
│   │   └── 1.0/                     # YAML use case files (LPC, LPP, MPC, etc.)
│   ├── testing/                   # Test specifications
│   └── design/                    # Design documents
└── mash-go/                     # Go reference implementation
    ├── cmd/                       # CLI tools
    │   ├── mash-device/             # Reference device (EVSE, inverter, battery)
    │   ├── mash-controller/         # Reference controller (EMS)
    │   ├── mash-test/               # Conformance test runner
    │   ├── mash-featgen/            # Feature code generator (YAML -> Go)
    │   ├── mash-ucgen/              # Use case code generator (YAML -> Go)
    │   ├── mash-log/                # Protocol log analyzer
    │   └── mash-pics/               # PICS file tools
    ├── pkg/                       # Public packages
    ├── internal/                   # Private packages (test harness)
    └── testdata/                   # Test cases and PICS files
```

## Go Implementation

See [mash-go/README.md](mash-go/README.md) for detailed documentation (commands, flags, examples).

### Quick Reference

```bash
cd mash-go

# Build and test
go build ./...
go test ./...

# Generate all code (features, use cases, mocks)
make generate

# Generate feature code from YAML definitions
make features

# Generate use case definitions from YAML
make usecases
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/wire` | CBOR message encoding |
| `pkg/transport` | TLS 1.3 server/client with framing |
| `pkg/discovery` | mDNS advertising and browsing |
| `pkg/commissioning` | PASE (SPAKE2+) session establishment |
| `pkg/service` | Device/controller service orchestration |
| `pkg/model` | Device model types (endpoints, features) -- partially generated |
| `pkg/features` | Feature implementations -- partially generated from YAML |
| `pkg/usecase` | Use case definitions -- generated from YAML |
| `internal/testharness` | Test infrastructure for conformance testing |

### Code Generation Pipeline

Feature and model code is generated from YAML definitions in `docs/features/`:

```
docs/features/<feature>/1.0.yaml   -->  mash-featgen  -->  pkg/features/<feature>_gen.go
docs/features/protocol-versions.yaml  -->  mash-featgen  -->  pkg/model/*_gen.go
docs/features/_shared/1.0.yaml    -->  mash-featgen  -->  pkg/features/shared_gen.go
docs/usecases/1.0/*.yaml          -->  mash-ucgen    -->  pkg/usecase/definitions_gen.go
```

**Important:** `*_gen.go` files are generated -- edit the YAML source or generator templates, not the generated files. Hand-written helpers coexist alongside generated code in the same packages.

## Key Concepts

### Device Model
3-level hierarchy: Device > Endpoint > Feature
- Endpoint 0: Always DEVICE_ROOT with DeviceInfo
- Endpoint 1+: Functional endpoints (EV_CHARGER, INVERTER, BATTERY, PV_STRING, etc.)

### Core Features

See [docs/features/README.md](docs/features/README.md) for the full registry, numbering conventions, and device composition examples.

| Feature | Question | Key Decisions |
|---------|----------|---------------|
| Electrical | "What CAN it do?" | Static config, phase mapping |
| Measurement | "What IS it doing?" | AC/DC telemetry |
| EnergyControl | "What SHOULD it do?" | Limits/setpoints, ControlStateEnum |
| Status | "Is it working?" | OperatingStateEnum, faults |
| ChargingSession | "What's the EV doing?" | Session lifecycle, SoC |
| Tariff / Signals / Plan | Price/forecast input/output | See feature specs |

### Multi-Zone Architecture
- Devices support max 1 zone per type (max 1 GRID + max 1 LOCAL = 2 zones, DEC-043)
- Zone types: GRID > LOCAL > TEST
- Limits: Most restrictive wins (safety)
- Setpoints: Highest priority wins
- Details: [docs/multi-zone.md](docs/multi-zone.md)

### State Machines
- **ControlStateEnum**: AUTONOMOUS, CONTROLLED, LIMITED, FAILSAFE, OVERRIDE
- **ProcessStateEnum**: NONE, AVAILABLE, SCHEDULED, RUNNING, PAUSED, COMPLETED, ABORTED
- These are orthogonal -- process continues during FAILSAFE

### Certificate Renewal
Controllers renew device certificates in-session (no TLS reconnection). MsgTypes 30-33. Key files: `pkg/commissioning/renewal_messages.go`, `pkg/service/renewal_tracker.go`, `pkg/service/device_renewal.go`, `pkg/service/controller_renewal.go`. Details: [docs/security.md](docs/security.md)

## Documentation Conventions

### Decision References
Design decisions are tracked in `docs/decision-log.md` as DEC-XXX (e.g., DEC-026 for EnergyControl design).

### EEBUS Comparison
See [docs/features/README.md](docs/features/README.md) for the full EEBUS use case mapping table. Key replacements: LPC/LPP -> EnergyControl, MPC/MGCP -> Measurement, CEVC/OSCEV -> ChargingSession + Signals + Plan, OHPCF -> EnergyControl with ProcessStateEnum.

### Matter Alignment
Protocol takes inspiration from Matter for:
- 4-operation interaction model (Read/Write/Subscribe/Invoke)
- FeatureMap bitmap for capability discovery
- SPAKE2+ for commissioning
- Global attributes pattern (clusterRevision, featureMap, attributeList)

## Working with This Repository

### Specification Work
1. Read existing specifications in `docs/`
2. Feature definitions: human-readable `.md` + machine-readable `.yaml` in `docs/features/`
3. Add design decisions to `docs/decision-log.md`
4. When changing feature attributes/enums, update the YAML source and run `make features`

### Implementation Work
1. Follow TDD: write tests first in `*_test.go`
2. Use `go test ./...` to run all tests
3. Never edit `*_gen.go` files directly -- edit YAML or generator templates
4. After YAML changes: `cd mash-go && make generate`
5. Use `mash-test` CLI to run conformance tests
6. Add YAML test cases in `mash-go/testdata/cases/`

### CBOR Serialization
The protocol uses CBOR with integer keys for compactness:
```cbor
{
  1: deviceId,        // string
  2: endpoints,       // array
}
```

### Sign Convention
Power values use passive/load convention:
- Positive (+) = consumption/charging
- Negative (-) = production/discharging
