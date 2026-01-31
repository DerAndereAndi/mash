# mash-evse Interactive EVSE Simulator

**Status:** Design
**Date:** 2026-01-28

## Overview

A dedicated interactive EVSE simulator for protocol validation and gap detection. This tool simulates an EVSE device with configurable hardware profiles and EV connections, enabling comprehensive testing of the MASH protocol.

## Goals

1. **Protocol testbed** - Find gaps in the specification, test edge cases, verify all state transitions
2. **Controller testing** - Provide a realistic EVSE for testing `mash-controller` implementations
3. **Scenario coverage** - Support EEBUS use cases: EVCC, EVSECC, CEVC, DBEVC, OSCEV 2.0, OPEV

## Non-Goals

- Not a production EVSE implementation
- Not a controller simulator (use `mash-controller` for that)
- Not implementing Signals/Plan features initially (add later)

## Architecture Decisions

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| CLI | Dedicated `mash-evse` command | Cleaner UX, can grow independently from `mash-device` |
| UI Style | Command-only with `status` command | Simpler to implement, sufficient for testbed |
| Config | YAML files | Human-readable, reproducible test scenarios |
| Mode | Device only | Separation of concerns, use `mash-controller` for testing |
| Features | ChargingSession + EnergyControl first | Core features exist, add Signals/Plan later |

## Priority Scenarios

### 1. Basic Charging
- EV plug-in/unplug lifecycle
- Session state transitions (NOT_PLUGGED_IN → PLUGGED_IN_DEMAND → PLUGGED_IN_CHARGING → SESSION_COMPLETE)
- Charge control via limits and setpoints
- Failsafe behavior on controller disconnect

### 2. Solar Charging Modes
- `PV_SURPLUS_ONLY` - Charge only with excess solar
- `PV_SURPLUS_THRESHOLD` - Charge when solar exceeds threshold, may use grid below
- Phase switching for minimum power optimization
- Start/stop delays to avoid rapid cycling

### 3. V2G Bidirectional (DBEVC)
- Discharge constraints and permissions
- `evMinDischargingRequest`, `evMaxDischargingRequest`
- Session state: `PLUGGED_IN_DISCHARGING`
- Production setpoints via EnergyControl

## Phase Switching Model

Two distinct concepts must be modeled:

### EVSE Installation Type
- **1-phase**: Single phase grid connection (max ~7.4 kW at 32A)
- **3-phase**: Three phase grid connection (max ~22 kW at 32A)

### IEC 61851 Phase Switching
For 3-phase EVSE installations with IEC 61851 (non-ISO 15118) connections, the EVSE can electrically limit the EV to use only 1 phase via hardware relays.

**Minimum power thresholds:**
```
3-phase minimum: 6A × 3 × 230V = 4,140 W (4.14 kW)
1-phase minimum: 6A × 1 × 230V = 1,380 W (1.38 kW)
```

**Solar optimization scenario:**
When available solar power is below 4.14 kW, switching to 1-phase enables charging at lower power levels (down to 1.38 kW), maximizing solar self-consumption.

**Protocol implications:**
- Electrical feature must update dynamically when phases change
- `phaseCount` and `maxConsumptionPower` change at runtime
- Controller receives subscription update when phase switching occurs

## EV Simulation

### Configuration Approach
- Fully manual parameter control via commands
- YAML config files for preset EV profiles
- Hybrid auto-progression: SoC updates automatically when charging, manual override available

### EV Parameters
| Parameter | Description | Example Values |
|-----------|-------------|----------------|
| `batteryCapacity` | Total battery capacity (Wh) | 40000, 77000, 100000 |
| `currentSoC` | Current state of charge (%) | 20, 50, 80 |
| `minSoC` | Minimum acceptable SoC (%) | 20 |
| `targetSoC` | Desired SoC (%) | 80 |
| `maxChargingPower` | Max charging power (W) | 7400, 11000, 22000 |
| `maxDischargingPower` | Max V2G discharge (W) | 0, 7400, 11000 |
| `v2gCapable` | Supports bidirectional | true, false |
| `demandMode` | EV demand mode | SINGLE_DEMAND, DYNAMIC, DYNAMIC_BIDIRECTIONAL |
| `identifiers` | EV identification | VIN, MAC, RFID, contractId |

## Gap Detection

Built-in protocol validation with comprehensive logging:

### State Transition Logging
- Log all ChargingSession state changes with timestamps
- Log all ControlState and ProcessState transitions
- Flag invalid sequences (e.g., CHARGING without DEMAND first)

### Attribute Change Tracking
- Track which attributes change on each operation
- Detect missing updates (e.g., Measurement should update when charging starts)
- Detect unexpected updates

### Constraint Violation Alerts
- Alert when controller sends setpoint outside Electrical limits
- Alert when limit would result in power below minimum
- Alert when phase current limits don't match phase count

## File Structure

```
mash-go/
├── cmd/mash-evse/
│   ├── main.go                 # Entry point, flags, config loading
│   └── interactive/
│       ├── commands.go         # Command registration
│       ├── cmd_evse.go         # EVSE config commands
│       ├── cmd_ev.go           # EV simulation commands
│       ├── cmd_charge.go       # Charging control commands
│       ├── cmd_status.go       # Status and debugging commands
│       └── validation.go       # Protocol gap detection
├── testdata/
│   ├── evse/
│   │   ├── evse-1phase-16a.yaml
│   │   ├── evse-3phase-16a.yaml
│   │   ├── evse-3phase-32a.yaml
│   │   └── evse-3phase-32a-phaseswitching.yaml
│   └── ev/
│       ├── ev-leaf-40kwh.yaml
│       ├── ev-model3-lr.yaml
│       ├── ev-id4-77kwh.yaml
│       ├── ev-ioniq5-v2g.yaml
│       └── ev-generic-basic.yaml
```

## Interactive Commands

### EVSE Configuration
```
evse load <config.yaml>     # Load EVSE hardware config
evse info                   # Show current EVSE configuration
phases <1|3>                # Set operational phase count (phase switching)
```

### EV Simulation
```
ev plug <ev-config.yaml>    # Simulate EV plug-in with profile
ev plug --soc 30 --capacity 77000 --max-power 11000  # Manual params
ev unplug                   # Disconnect EV
ev soc <percent>            # Manually set SoC
ev demand <mode>            # Set demand mode
ev identify <type> <value>  # Set EV identifier (VIN, RFID, etc.)
```

### Charging Control
```
charge start                # Start charging (device-side decision)
charge stop                 # Stop charging
charge mode <mode>          # Set charging mode (OFF, PV_SURPLUS_ONLY, etc.)
charge power <kw>           # Override power output (testing)
```

### Status & Debugging
```
status                      # Current session state, power, SoC, limits
limits                      # Show active limits from all zones
electrical                  # Show Electrical feature state
energy-control              # Show EnergyControl feature state
transitions                 # Show state transition history
violations                  # Show protocol violations detected
```

### Zone Management
```
zones                       # List connected controller zones
zone <id> info              # Show zone details
```

## Example EVSE Config (YAML)

```yaml
# testdata/evse/evse-3phase-32a-phaseswitching.yaml
name: "3-Phase 22kW with Phase Switching"
installation:
  phaseCount: 3
  phaseMapping: [L1, L2, L3]
  voltage: 230
  frequency: 50
electrical:
  maxConsumptionPower: 22000      # 22 kW
  minConsumptionPower: 1380       # 1.38 kW (1-phase minimum)
  maxCurrentPerPhase: 32
  minCurrentPerPhase: 6
capabilities:
  phaseSwitching: true            # Can switch between 1 and 3 phases
  v2gSupport: false               # Hardware doesn't support V2G
chargingModes:
  - OFF
  - FAST
  - PV_SURPLUS_ONLY
  - PV_SURPLUS_THRESHOLD
defaults:
  startDelay: 60                  # seconds
  stopDelay: 300                  # seconds
  surplusThreshold: 200           # watts
```

## Example EV Config (YAML)

```yaml
# testdata/ev/ev-ioniq5-v2g.yaml
name: "Hyundai Ioniq 5 (V2G)"
battery:
  capacity: 77400                 # Wh
  defaultSoC: 50                  # %
  minSoC: 20                      # %
  targetSoC: 80                   # %
charging:
  maxPower: 11000                 # W (on-board charger limit)
  phases: 3
discharging:
  capable: true
  maxPower: 11000                 # W
demandMode: DYNAMIC_BIDIRECTIONAL
identifiers:
  vin: "KMHK8XXXXXXXXXXXX"
```

## EEBUS Use Case Mapping

| EEBUS Use Case | mash-evse Coverage |
|----------------|-------------------|
| EVCC | EV identification, session management |
| EVSECC | Session states, charging control |
| CEVC | Energy requests, departure time (partial - needs Signals/Plan) |
| DBEVC | V2G discharge, bidirectional demand modes |
| OSCEV 2.0 | Charging modes, start/stop delays, surplus threshold |
| OPEV | Per-phase current limits via EnergyControl |
| MPC | Measurement feature telemetry |

## Implementation Phases

### Phase 1: Basic Infrastructure
- [ ] Create `cmd/mash-evse/` structure
- [ ] YAML config loading for EVSE and EV profiles
- [ ] Basic interactive command loop
- [ ] Integration with existing DeviceService

### Phase 2: Core Simulation
- [ ] EV plug/unplug with state transitions
- [ ] Charging simulation with SoC progression
- [ ] Electrical feature dynamic updates
- [ ] ChargingSession state machine

### Phase 3: Solar Charging
- [ ] Phase switching commands and logic
- [ ] Charging mode handling (PV_SURPLUS_ONLY, etc.)
- [ ] Start/stop delay enforcement
- [ ] Minimum power threshold validation

### Phase 4: V2G Support
- [ ] Discharge state and power tracking
- [ ] Discharge constraints and permissions
- [ ] Bidirectional demand mode handling

### Phase 5: Gap Detection
- [ ] State transition logging
- [ ] Attribute change tracking
- [ ] Constraint violation detection
- [ ] Test scenario recording/playback

## Open Questions

1. Should phase switching trigger a subscription update or require controller re-read?
2. How should `startDelay` interact with controller setpoints during the delay period?
3. Should the simulator support multiple EVs (dual-port EVSE) or single EV only?
4. What level of Measurement simulation is needed (just power, or full AC telemetry)?

## References

- [ChargingSession Feature Spec](../features/charging-session.md)
- [EnergyControl Feature Spec](../features/energy-control.md)
- [Electrical Feature Spec](../features/electrical.md)
- [EEBUS Use Case Mapping](../features/README.md)
