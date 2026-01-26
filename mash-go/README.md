# MASH Go Implementation

Go reference implementation of the MASH (Minimal Application-layer Smart Home Protocol).

## Overview

This implementation provides:
- Complete protocol stack (wire format, transport, discovery)
- Device and controller services
- Commissioning with PASE (SPAKE2+)
- Test harness for conformance testing

## Quick Start

```bash
# Build all commands
go build ./...

# Run tests
go test ./...
```

## Commands

### mash-device

Run a MASH device that advertises via mDNS and accepts controller connections.

```bash
go run ./cmd/mash-device -port 8443 -name "My Device"
```

### mash-controller

Run a MASH controller that discovers and connects to devices.

```bash
go run ./cmd/mash-controller
```

### mash-test

Protocol conformance test runner. Tests devices or controllers against the MASH specification.

```bash
# Basic usage - test a device
mash-test -target localhost:8443 -mode device

# With PICS filtering (skip tests for unsupported features)
mash-test -target localhost:8443 -pics testdata/pics/ev-charger.yaml

# Run specific test patterns
mash-test -target localhost:8443 "TC-DISC*"    # Discovery tests only
mash-test -target localhost:8443 "*EnergyControl*"

# Verbose output for debugging
mash-test -target localhost:8443 -verbose

# CI-friendly output formats
mash-test -target localhost:8443 -json          # JSON output
mash-test -target localhost:8443 -junit > results.xml  # JUnit XML

# Skip TLS verification (for testing)
mash-test -target localhost:8443 -insecure
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-target` | Target address (host:port) | *required* |
| `-mode` | Test mode: `device` or `controller` | `device` |
| `-pics` | Path to PICS file for capability filtering | - |
| `-tests` | Path to test cases directory | `./testdata/cases` |
| `-timeout` | Per-test timeout | `30s` |
| `-verbose` | Show detailed step output | `false` |
| `-json` | Output results as JSON | `false` |
| `-junit` | Output results as JUnit XML | `false` |
| `-insecure` | Skip TLS certificate verification | `false` |

## Project Structure

```
mash-go/
├── cmd/                    # Command-line applications
│   ├── mash-device/        # Device reference implementation
│   ├── mash-controller/    # Controller reference implementation
│   ├── mash-test/          # Conformance test runner
│   ├── evse-example/       # EV charger example
│   └── cem-example/        # CEM (controller) example
├── pkg/                    # Public packages
│   ├── wire/               # CBOR message encoding
│   ├── transport/          # TLS server/client, framing
│   ├── discovery/          # mDNS advertising and browsing
│   ├── commissioning/      # PASE (SPAKE2+), session management
│   ├── subscription/       # Subscription lifecycle
│   ├── failsafe/           # Failsafe timer and state
│   ├── zone/               # Multi-zone coordination
│   ├── service/            # Device/controller service orchestration
│   ├── model/              # Device model (endpoints, features)
│   ├── features/           # Feature implementations
│   └── ...
├── internal/               # Private packages
│   └── testharness/        # Test infrastructure
│       ├── loader/         # YAML test case parsing
│       ├── engine/         # Test execution
│       ├── assertions/     # Type-safe assertions
│       ├── mock/           # Device/controller mocks
│       ├── reporter/       # Output formatting
│       └── runner/         # CLI integration
└── testdata/               # Test data files
    ├── cases/              # YAML test cases
    └── pics/               # PICS profiles
```

## Test Harness

The test harness supports YAML-based declarative test cases:

```yaml
# testdata/cases/example.yaml
id: TC-EXAMPLE-001
name: Example Test
description: Demonstrates test case format

pics_requirements:
  - D.COMM.SC    # Device supports secure connection

steps:
  - name: Connect to device
    action: connect
    params:
      insecure: true
    expect:
      connection_established: true

  - name: Read device info
    action: read
    params:
      endpoint: 0
      feature: 1
    expect:
      read_success: true

timeout: "10s"
tags:
  - basic
  - connectivity
```

### PICS Files

PICS (Protocol Implementation Conformance Statement) files describe device capabilities:

```yaml
# testdata/pics/ev-charger.yaml
device:
  vendor: "Example Corp"
  product: "Smart Charger"

items:
  D.COMM.SC: true       # Secure connection
  D.COMM.PASE: true     # PASE commissioning
  D.ELEC.PHASES: 3      # Three-phase
  D.ELEC.MAX_CURRENT: 32
  D.CTRL.LIMIT: true    # Power limiting
```

## Dependencies

- [fxamacker/cbor/v2](https://github.com/fxamacker/cbor) - CBOR encoding
- [grandcat/zeroconf](https://github.com/grandcat/zeroconf) - mDNS
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML parsing

## Test Coverage

352 tests covering:
- Wire format encoding/decoding
- TLS server/client
- mDNS discovery
- PASE commissioning (SPAKE2+)
- Subscription management
- Failsafe behavior
- Multi-zone coordination
- Test harness infrastructure

Run with coverage:
```bash
go test ./... -cover
```
