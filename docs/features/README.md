# MASH Feature Definitions

> Feature specifications for the MASH protocol

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Feature Philosophy

MASH features are organized by concern:

| Feature | Question | Update Frequency |
|---------|----------|------------------|
| **Electrical** | "What CAN this do?" | Rarely (hardware/installation) |
| **Measurement** | "What IS it doing?" | Constantly (telemetry) |
| **EnergyControl** | "What SHOULD it do?" | On command (control) |
| **Status** | "Is it working?" | On state change |

---

## Feature Registry

| ID | Name | Direction | Description | Document |
|----|------|-----------|-------------|----------|
| 0x0001 | DeviceInfo | OUT | Device identity and structure | [device-info.md](device-info.md) |
| 0x0002 | Status | OUT | Operating state, faults | [status.md](status.md) |
| 0x0003 | Electrical | - | Dynamic electrical configuration | [electrical.md](electrical.md) |
| 0x0004 | Measurement | OUT | Power, energy, voltage, current telemetry | [measurement.md](measurement.md) |
| 0x0005 | EnergyControl | IN | Limits, setpoints, control commands | [energy-control.md](energy-control.md) |
| 0x0006 | ChargingSession | OUT | EV charging session data | [charging-session.md](charging-session.md) |
| 0x0007 | Tariff | IN | Price structure, components, power tiers | [tariff.md](tariff.md) |
| 0x0008 | Signals | IN | Time-slotted prices, limits, forecasts | [signals.md](signals.md) |
| 0x0009 | Plan | OUT | Device's intended behavior | [plan.md](plan.md) |
| 0x0100+ | (vendor) | - | Vendor-specific features | |

**Direction (data flow):**
- **IN** = Data flows TO device (controller sends)
- **OUT** = Data flows FROM device (device reports)
- **-** = Bidirectional or static configuration

> **Note:** Direction indicates the typical data flow, not request direction. MASH operations (Read/Write/Subscribe/Invoke) are bidirectional - either side can initiate requests. For example, a device can Read from a controller that exposes features. See [Interaction Model: Bidirectional Communication](../interaction-model.md#12-bidirectional-communication).

---

## Feature Map Bits

The `featureMap` global attribute is a **32-bit bitmap** indicating high-level capability categories. It enables quick capability discovery without reading all feature details.

```
bit 0  (0x0001): CORE       - EnergyCore basics (always set)
bit 1  (0x0002): FLEX       - Flexible power adjustment (FlexibilityStruct)
bit 2  (0x0004): BATTERY    - Battery-specific attributes (SoC, SoH)
bit 3  (0x0008): EMOB       - E-Mobility/EVSE (charging sessions)
bit 4  (0x0010): SIGNALS    - Incentive signals support
bit 5  (0x0020): TARIFF     - Tariff data support
bit 6  (0x0040): PLAN       - Power plan support
bit 7  (0x0080): PROCESS    - Optional process lifecycle (OHPCF)
bit 8  (0x0100): FORECAST   - Power forecasting capability
bit 9  (0x0200): ASYMMETRIC - Per-phase asymmetric control
bit 10 (0x0400): V2X        - Vehicle-to-grid/home (bidirectional EV)
```

### Capability Discovery Pattern

FeatureMap bits indicate **high-level categories**. Detailed capability information is in feature attributes:

| FeatureMap Bit | Quick Check | Details In |
|----------------|-------------|------------|
| EMOB | Has EV charging | ChargingSession: `supportedChargingModes`, `evDemandMode` |
| ASYMMETRIC | Per-phase control | Electrical: `supportsAsymmetric` enum (NONE/CONSUMPTION/PRODUCTION/BIDIRECTIONAL) |
| V2X | Bidirectional EV | Electrical: `supportedDirections` enum; ChargingSession: `evDemandMode` = DYNAMIC_BIDIRECTIONAL |
| BATTERY | Has battery | Electrical: `energyCapacity`; separate BATTERY endpoint |
| FLEX | Flexible power | EnergyControl: `FlexibilityStruct` |
| SIGNALS | Price/limit signals | Signals feature present |
| TARIFF | Tariff structure | Tariff feature present |
| PLAN | Power planning | Plan feature present |
| PROCESS | Process control | EnergyControl: `processState`, `optionalProcess` |
| FORECAST | Power forecasting | EnergyControl: `ForecastStruct` |

**Discovery flow:**
1. Read `featureMap` → quick check what categories are supported
2. Read relevant feature attributes → get specific capability details

**Example:** EVSE with V2X and asymmetric charging but symmetric discharging:
- `featureMap`: CORE | EMOB | ASYMMETRIC | V2X
- Electrical: `supportsAsymmetric = CONSUMPTION` (asymmetric charge only)
- Electrical: `supportedDirections = BIDIRECTIONAL`

---

## Common Patterns

### Attribute Numbering Convention

Each feature uses consistent attribute ID ranges:

| Range | Purpose |
|-------|---------|
| 1-9 | Core identity/type attributes |
| 10-19 | Capability flags |
| 20-29 | Primary data (limits, setpoints, etc.) |
| 30-39 | Secondary data (per-phase variants) |
| 40-49 | Tertiary data |
| 50-59 | Additional data |
| 60-69 | Complex structures (flexibility, forecast) |
| 70-79 | Failsafe configuration |
| 80-89 | Process management |
| 0xFFF0-0xFFFF | Global attributes (reserved) |

### Command Numbering Convention

| Range | Purpose |
|-------|---------|
| 1-4 | Primary commands (Set/Clear for limits) |
| 5-8 | Secondary commands (Set/Clear for currents) |
| 9-12 | Control commands (Pause/Resume/Stop) |
| 13-16 | Process commands (Schedule/Cancel) |

---

## Device Composition

### EVSE Example

```
Endpoint 1 (type: EV_CHARGER)
├── Electrical           ← Phase config, ratings
├── Measurement          ← Power, energy readings
├── EnergyControl        ← Limits, control
├── Status               ← Operating state
└── ChargingSession      ← EV session info
```

### Hybrid Inverter Example

```
Endpoint 1 (type: INVERTER)
├── Electrical
├── Measurement
├── EnergyControl
└── Status

Endpoint 2 (type: PV_STRING)
├── Electrical
├── Measurement
└── Status

Endpoint 3 (type: BATTERY)
├── Electrical
├── Measurement
├── EnergyControl
└── Status
```

### Heat Pump Example

```
Endpoint 1 (type: HEAT_PUMP)
├── Electrical
├── Measurement
├── EnergyControl        ← includes processState, optionalProcess
└── Status
```

---

## EEBUS Use Case Mapping

| EEBUS Use Case | MASH Features |
|----------------|---------------|
| LPC (Load Power Control) | EnergyControl |
| LPP (Load Power Production) | EnergyControl |
| MPC (Measurement Power) | Measurement |
| MGCP (Monitoring Grid Connection) | Measurement (on GRID_CONNECTION endpoint) |
| EVSE, EVCEM, CEVC | ChargingSession + EnergyControl + Signals + Plan |
| COB (Control of Battery) | EnergyControl + Measurement (on BATTERY endpoint) |
| MOB (Monitoring of Battery) | Measurement + Status |
| MOI (Monitoring of Inverter) | Measurement + Status |
| MPS (Monitoring of PV String) | Measurement |
| ToUT (Time of Use Tariff) | Signals + Tariff |
| POEN (Power Envelope) | Signals (CONSTRAINT type) |
| ITPCM (Incentive Table) | Signals + Tariff |
| OHPCF (Heat Pump Flexibility) | EnergyControl (processState, optionalProcess) |

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Protocol Overview](../protocol-overview.md) | Vision, architecture, device model |
| [Discovery](../discovery.md) | Capability discovery, featureMap |
| [Interaction Model](../interaction-model.md) | Read/Write/Subscribe/Invoke |
| [Decision Log](../decision-log.md) | Design decisions |
