# MASH Use Case Definitions

This directory contains YAML definitions of EEBUS-style use cases mapped to MASH features.

## Structure

```
usecases/
  1.0/           # Use cases for spec version 1.0
    lpc.yaml     # Limit Power Consumption
    lpp.yaml     # Limit Power Production
    mpd.yaml     # Monitor Power Device
    evc.yaml     # EV Charging
    ...          # See 1.0/ directory for full list
```

## Format

Each YAML file defines one use case:

```yaml
name: LPC                     # Short identifier
fullName: Limit Power Consumption
specVersion: "1.0"            # MASH spec version for name resolution
description: >
  Human-readable description.

endpointTypes:                # Endpoint types that support this use case
  - EV_CHARGER                # Empty list = any endpoint type
  - INVERTER

features:                     # Required and optional features
  - feature: EnergyControl    # Feature name (from spec manifest)
    required: true
    attributes:
      - name: acceptsLimits   # Attribute name (from spec manifest)
        requiredValue: true   # Optional: attribute must have this value
    commands:
      - setLimit              # Command name (from spec manifest)
    subscribe: all            # Subscribe to all feature attributes (DEC-052)

commands:                     # Interactive controller commands this enables
  - limit
  - clear
```

## Code Generation

Use case YAML files are processed by `mash-ucgen` to generate Go code:

```bash
cd mash-go
go run ./cmd/mash-ucgen -input ../docs/usecases/1.0 -output pkg/usecase/definitions_gen.go -version 1.0
```

The generated `definitions_gen.go` contains a `Registry` map with all use case
definitions, pre-resolved to numeric feature/attribute/command IDs.

## Adding Use Cases

1. Create a new YAML file in the appropriate version directory
2. Reference only features, attributes, and commands defined in the spec manifest
3. Run `go generate ./pkg/usecase/...` to regenerate the Go code
4. Add validation tests in `definitions_gen_test.go`
