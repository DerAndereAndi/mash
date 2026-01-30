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
├── docs/                   # Protocol specification
│   ├── protocol-overview.md  # Canonical architecture guide
│   ├── decision-log.md
│   ├── transport.md
│   ├── security.md
│   ├── discovery.md
│   ├── interaction-model.md
│   ├── multi-zone.md        # Multi-zone architecture
│   ├── features/           # Feature specifications
│   └── testing/            # Test specifications
└── mash-go/                # Go reference implementation
    ├── cmd/                # CLI tools (mash-device, mash-controller, mash-test)
    ├── pkg/                # Public packages
    ├── internal/           # Private packages (test harness)
    └── testdata/           # Test cases and PICS files
```

## Go Implementation

See [mash-go/README.md](mash-go/README.md) for detailed documentation.

### Quick Reference

```bash
cd mash-go

# Build and test
go build ./...
go test ./...

# Run conformance tests against a device
go run ./cmd/mash-test -target localhost:8443 -verbose

# Run with PICS filtering
go run ./cmd/mash-test -target localhost:8443 -pics testdata/pics/ev-charger.yaml
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/wire` | CBOR message encoding |
| `pkg/transport` | TLS 1.3 server/client with framing |
| `pkg/discovery` | mDNS advertising and browsing |
| `pkg/commissioning` | PASE (SPAKE2+) session establishment |
| `pkg/service` | Device/controller service orchestration |
| `internal/testharness` | Test infrastructure for conformance testing |

## Key Concepts

### Device Model
3-level hierarchy: Device > Endpoint > Feature
- Endpoint 0: Always DEVICE_ROOT with DeviceInfo
- Endpoint 1+: Functional endpoints (EV_CHARGER, INVERTER, BATTERY, PV_STRING, etc.)

### Core Features
| Feature | Purpose | Key Decisions |
|---------|---------|---------------|
| Electrical | "What CAN it do?" | Static config, phase mapping |
| Measurement | "What IS it doing?" | AC/DC telemetry |
| EnergyControl | "What SHOULD it do?" | Limits/setpoints, ControlStateEnum |
| Status | "Is it working?" | OperatingStateEnum, faults |

### Multi-Zone Architecture
- Devices support up to 5 zones (controllers)
- Zone types: GRID_OPERATOR > BUILDING_MANAGER > HOME_MANAGER > USER_APP
- Limits: Most restrictive wins (safety)
- Setpoints: Highest priority wins

### State Machines
- **ControlStateEnum**: AUTONOMOUS, CONTROLLED, LIMITED, FAILSAFE, OVERRIDE
- **ProcessStateEnum**: NONE, AVAILABLE, SCHEDULED, RUNNING, PAUSED, COMPLETED, ABORTED
- These are orthogonal - process continues during FAILSAFE

### Certificate Renewal
Controllers can renew device certificates without disconnecting the TLS session:

**Protocol Flow:**
1. Controller sends `CertRenewalRequest` (MsgType 30) with 32-byte nonce
2. Device generates new key pair and responds with `CertRenewalCSR` (MsgType 31)
3. Controller signs CSR with Zone CA, sends `CertRenewalInstall` (MsgType 32)
4. Device installs new cert atomically, responds with `CertRenewalAck` (MsgType 33)

**Key Design Points:**
- Renewal happens in-session (no TLS reconnection required)
- Subscriptions and session state are preserved
- Controller-initiated, typically 30 days before expiry
- Device generates new key pair for each renewal (not reused)

**CLI Commands (mash-controller):**
```
renew <device-id>    # Renew specific device certificate
renew --all          # Renew all devices needing renewal
renew --status       # Show certificate expiry status
```

**Key Files:**
| File | Purpose |
|------|---------|
| `pkg/commissioning/renewal_messages.go` | Wire protocol messages (30-33) |
| `pkg/service/renewal_tracker.go` | Expiry tracking |
| `pkg/service/device_renewal.go` | Device-side handler |
| `pkg/service/controller_renewal.go` | Controller-side handler |
| `cmd/mash-controller/interactive/cmd_renew.go` | CLI command |

## Documentation Conventions

### Decision References
Design decisions are tracked in `docs/decision-log.md` as DEC-XXX (e.g., DEC-026 for EnergyControl design).

### EEBUS Comparison
Documentation frequently references EEBUS use cases being replaced:
- LPC/LPP (power limits) → EnergyControl
- MPC/MGCP (monitoring) → Measurement
- CEVC/OSCEV (EV charging) → ChargingSession + Signals + Plan
- OHPCF (heat pump) → EnergyControl with ProcessStateEnum

### Matter Alignment
Protocol takes inspiration from Matter for:
- 4-operation interaction model (Read/Write/Subscribe/Invoke)
- FeatureMap bitmap for capability discovery
- SPAKE2+ for commissioning
- Global attributes pattern (clusterRevision, featureMap, attributeList)

## Working with This Repository

### Specification Work
1. Read existing specifications in `docs/`
2. Review behavior specs in `docs/testing/behavior/`
3. Add design decisions to `docs/decision-log.md`
4. Create/update feature specs in `docs/features/`

### Implementation Work
1. Follow TDD: write tests first in `*_test.go`
2. Use `go test ./... -v` to run all tests
3. Use `mash-test` CLI to run conformance tests
4. Add YAML test cases in `mash-go/testdata/cases/`

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
