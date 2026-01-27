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
make build
# or: go build ./...

# Run unit tests (fast)
make test-unit
# or: go test ./...

# Run all tests including integration tests
make test
```

## Development

### Testing

Tests are separated into unit tests and integration tests using Go build tags.

```bash
# Run unit tests (fast, no network required)
make test-unit
# or: go test ./...

# Run integration tests (requires network, slower)
make test-integration
# or: go test -tags=integration ./...

# Run all tests
make test
```

### Mock Generation

Mocks are generated using [mockery v2](https://github.com/vektra/mockery).

```bash
# Install mockery (one-time setup)
make install-mockery

# Regenerate mocks after interface changes
make mocks
# or: go generate ./...
```

Generated mocks are in `mocks/` subdirectories:
- `pkg/transport/mocks/` - Transport layer mocks
- `pkg/discovery/mocks/` - Discovery layer mocks

## Commands

### mash-device

Reference MASH device implementation supporting multiple device types with mDNS discovery, commissioning, and simulation mode.

```bash
# Start EVSE device with default settings
mash-device -type evse -discriminator 1234 -setup-code 12345678

# Start inverter with custom branding
mash-device -type inverter -brand "Solar Corp" -model "Inverter 10kW"

# Start battery in simulation mode with debug logging
mash-device -type battery -simulate -log-level debug

# Custom port and device name
mash-device -type evse -port 9443 -name "Garage Charger"
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-type` | Device type: `evse`, `inverter`, `battery` | `evse` |
| `-discriminator` | Discriminator for commissioning (0-4095) | `1234` |
| `-setup-code` | 8-digit setup code for commissioning | `12345678` |
| `-port` | Listen port | `8443` |
| `-simulate` | Enable simulation mode with synthetic data | `true` |
| `-log-level` | Log level: `debug`, `info`, `warn`, `error` | `info` |
| `-config` | Configuration file path | - |
| `-serial` | Device serial number (auto-generated if empty) | - |
| `-brand` | Device brand/vendor name | `MASH Reference` |
| `-model` | Device model name (auto-generated if empty) | - |
| `-name` | User-friendly device name | - |

**Device Types:**
- **evse**: EV charger (22kW, 3-phase, supports power limiting)
- **inverter**: PV inverter (10kW, 3-phase, production only)
- **battery**: Battery storage (5kW charge/discharge, bidirectional)

### mash-controller

Reference MASH controller (Energy Management System) with device discovery, commissioning, and interactive command interface.

```bash
# Start controller with interactive mode
mash-controller -zone-name "My Home" -interactive

# Start controller that auto-commissions discovered devices
mash-controller -auto-commission -log-level debug

# Custom zone configuration
mash-controller -zone-name "Building A" -zone-type building
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-zone-name` | Zone name for this controller | `Home Energy` |
| `-zone-type` | Zone type: `grid`, `building`, `home`, `user` | `home` |
| `-interactive` | Enable interactive command mode | `false` |
| `-auto-commission` | Automatically commission discovered devices | `false` |
| `-log-level` | Log level: `debug`, `info`, `warn`, `error` | `info` |
| `-config` | Configuration file path | - |

**Interactive Commands:**
| Command | Description |
|---------|-------------|
| `discover` | Discover commissionable devices |
| `list` | List connected devices |
| `commission <discriminator> <setup-code>` | Commission a device |
| `limit <device-id> <power-kw>` | Set power limit in kW |
| `clear <device-id>` | Clear power limit |
| `pause <device-id>` | Pause device operation |
| `resume <device-id>` | Resume device operation |
| `status` | Show controller status |
| `help` | Show command help |
| `quit` | Exit the controller |

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
│   └── mash-test/          # Conformance test runner
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

## Bidirectional Communication

MASH supports bidirectional communication where both sides of a connection can send requests. This enables advanced scenarios like Smart Meter Gateways exposing grid meter data.

### Controller Exposing Features

A controller can expose a device model to connected devices:

```go
// Create a device model to expose (e.g., grid meter)
exposedDevice := model.NewDevice("smgw-meter", vendorID, productID)
meterEndpoint := model.NewEndpoint(1, model.EndpointGridConnection, "Grid Meter")
measurement := model.NewFeature(model.FeatureMeasurement, 1)
measurement.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
    ID:      1,
    Name:    "ACActivePower",
    Type:    model.DataTypeInt64,
    Access:  model.AccessRead,
    Default: int64(0),
}))
meterEndpoint.AddFeature(measurement)
exposedDevice.AddEndpoint(meterEndpoint)

// After commissioning a device, configure the session to expose features
session := controllerSvc.GetSession(deviceID)
session.SetExposedDevice(exposedDevice)

// Now the device can read/subscribe to controller's meter data
```

### Device Querying Controller

From the device side, query the controller's exposed features:

```go
// Get the zone session for the connected controller
zoneSession := deviceSvc.GetZoneSession(zoneID)

// Read attributes from controller's exposed features
attrs, err := zoneSession.Read(ctx, 1, uint8(model.FeatureMeasurement), nil)

// Subscribe to controller attribute changes
subID, priming, err := zoneSession.Subscribe(ctx, 1, uint8(model.FeatureMeasurement), nil)
```

### Dual-Service Entity (EMS Pattern)

An EMS can act as both device and controller:

```go
// EMS as a device (receives limits from SMGW)
emsDevice := model.NewDevice("ems-001", vendorID, productID)
emsDeviceSvc, _ := service.NewDeviceService(emsDevice, deviceConfig)
emsDeviceSvc.Start(ctx)

// EMS as a controller (manages household devices)
emsControllerSvc, _ := service.NewControllerService(controllerConfig)
emsControllerSvc.Start(ctx)

// Now EMS can:
// - Receive limits from SMGW (via DeviceService)
// - Control EVSE, battery, heat pump (via ControllerService)
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
