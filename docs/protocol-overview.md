# MASH Protocol Overview

> **M**inimal **A**pplication-layer **S**mart **H**ome Protocol
>
> A lightweight, streamlined replacement for EEBUS SHIP/SPINE

**Status:** Draft - Initial Exploration
**Created:** 2025-01-24
**Last Updated:** 2025-01-25

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Transport](transport.md) | TCP/TLS, framing, keep-alive, connection model |
| [Security](security.md) | Certificates, commissioning, zones, pairing |
| [Discovery](discovery.md) | mDNS, capability discovery, QR codes |
| [Interaction Model](interaction-model.md) | Read/Write/Subscribe/Invoke semantics |
| [Features](features/README.md) | Feature definitions and registry |
| [Decision Log](decision-log.md) | Design decisions and rationale |

---

## 1. Vision & Goals

### 1.1 Problem Statement

EEBUS (SHIP + SPINE) has fundamental design flaws that make it:
- **Complex**: 50+ specification ambiguities, 7,000+ implementation variations
- **Hard to implement**: Race conditions, undefined behaviors, no test specs
- **Resource-heavy**: Designed for flexibility over efficiency
- **Lacking orchestration**: Communication without coordination

### 1.2 Design Goals

| Priority | Goal | Description |
|----------|------|-------------|
| P0 | **Simple** | Implementable in days, not months |
| P0 | **Deterministic** | No ambiguities, no race conditions |
| P0 | **Lightweight** | Runs on 256KB RAM MCUs |
| P1 | **Secure** | Modern security without complexity |
| P1 | **Testable** | Reference implementation + test suite from day one |
| P2 | **Extensible** | New use cases without breaking changes |

### 1.3 Non-Goals

- Full backwards compatibility with EEBUS
- Support for every conceivable future use case
- Complex trust hierarchies
- Cloud-first architecture

---

## 2. Architecture Overview

### 2.1 Layer Model (Inspired by Matter)

```
┌─────────────────────────────────────────────┐
│           Application / Use Cases           │  <- EV Charging, Load Control, etc.
├─────────────────────────────────────────────┤
│              Data Model Layer               │  <- Features, Attributes, Commands
│              (CBOR serialization)           │
├─────────────────────────────────────────────┤
│           Interaction Model Layer           │  <- Read, Write, Subscribe, Invoke
├─────────────────────────────────────────────┤
│         Transport Layer (TCP/TLS)           │  <- Length-prefixed framing
├─────────────────────────────────────────────┤
│            Discovery Layer                  │  <- mDNS/DNS-SD + QR commissioning
└─────────────────────────────────────────────┘
```

### 2.2 Key Simplifications vs EEBUS

| EEBUS Complexity | MASH Simplification |
|------------------|---------------------|
| SHIP + SPINE as separate specs | Single unified specification |
| 7 RFE operation modes | 4 operations: Read, Write, Subscribe, Invoke |
| Features with Functions | Features with Attributes + Commands |
| Bindings + Subscriptions (separate) | Unified subscription model |
| Double connection race condition | Single connection per device pair |
| Trust levels 0-100 | Binary trust: paired or not |
| 250+ data structures | Small set of core features |

---

## 3. Device Model

**Hierarchy:** Device > Endpoint > Feature (3-level)

```
Device (evse-001)
├── Endpoint 0 (type: DEVICE_ROOT)
│   └── DeviceInfo
├── Endpoint 1 (type: EV_CHARGER, label: "Port 1")
│   ├── Electrical           ← Phase config, ratings (static)
│   ├── Measurement          ← Power, energy readings (telemetry)
│   ├── EnergyControl        ← Limits, control (commands)
│   └── ChargingSession      ← EV session info
└── Endpoint 2 (type: EV_CHARGER, label: "Port 2")  [optional for dual-port]
    ├── Electrical
    ├── Measurement
    ├── EnergyControl
    └── ChargingSession
```

### 3.1 Hybrid Inverter Example

```
Device (hybrid-inverter-001)
├── Endpoint 0 (type: DEVICE_ROOT)
│   └── DeviceInfo
├── Endpoint 1 (type: INVERTER, label: "Grid Connection")
│   ├── Electrical           ← AC phase config, ratings
│   ├── Measurement          ← AC power, energy, voltage, current
│   └── EnergyControl        ← Grid limits (LPC/LPP)
├── Endpoint 2 (type: PV_STRING, label: "Roof South")
│   ├── Electrical           ← DC ratings
│   └── Measurement          ← DC power, voltage, current, yield
├── Endpoint 3 (type: PV_STRING, label: "Roof West")
│   ├── Electrical
│   └── Measurement
└── Endpoint 4 (type: BATTERY, label: "LG Chem RESU")
    ├── Electrical           ← DC ratings, capacity
    ├── Measurement          ← DC power, SoC, SoH, temperature
    └── EnergyControl        ← Battery control (COB)
```

### 3.2 EndpointType Enum

```
DEVICE_ROOT       = 0x00  // Device-level info (always endpoint 0)
GRID_CONNECTION   = 0x01  // AC grid connection point (smart meter)
INVERTER          = 0x02  // Inverter AC side (grid-facing)
PV_STRING         = 0x03  // PV string / solar input (DC)
BATTERY           = 0x04  // Battery storage (DC)
EV_CHARGER        = 0x05  // EVSE / wallbox
HEAT_PUMP         = 0x06  // Heat pump
WATER_HEATER      = 0x07  // Water heater / boiler
HVAC              = 0x08  // HVAC system
APPLIANCE         = 0x09  // Generic controllable appliance
SUB_METER         = 0x0A  // Sub-meter / circuit monitor
```

### 3.3 Endpoint Discovery Response

```cbor
{
  endpoints: [
    { id: 0, type: DEVICE_ROOT, features: [DeviceInfo] },
    { id: 1, type: INVERTER, label: "Grid", features: [Electrical, Measurement, EnergyControl] },
    { id: 2, type: PV_STRING, label: "South", features: [Electrical, Measurement] },
    { id: 3, type: BATTERY, label: "Battery", features: [Electrical, Measurement, EnergyControl] }
  ]
}
```

### 3.4 Topology

**Topology is implicit from EndpointType:**
- AC endpoints (INVERTER, GRID_CONNECTION, EV_CHARGER) → connect to grid
- DC endpoints (PV_STRING, BATTERY) → internal to device, connect to DC bus
- No explicit parent/child relationships needed

### 3.5 Addressing

```
device_id / endpoint_id / feature_id / attribute_or_command
evse-001  / 1           / Measurement / acActivePower
```

**Endpoint Conventions:**
- Endpoint 0: Reserved for root device metadata (type: DEVICE_ROOT)
- Endpoint 1+: Functional endpoints with appropriate EndpointType

---

## 4. Use Case Coverage

### 4.1 Initial Target Use Cases

| Use Case | EEBUS Equivalent | Status |
|----------|------------------|--------|
| EV Charging Control | EVSE, EVCEM, CEVC | Defined |
| Load/Production Control | LPC, LPP | Defined |
| Battery Control | COB | Defined |
| Grid Signals | ToUT, POEN, ITPCM | Defined |
| Heat Pump Flexibility | OHPCF | Defined |
| Device Monitoring | MPC, MGCP, MOB, MOI, MPS | Defined |

### 4.2 Future Use Cases

- HVAC Control
- Smart Appliances (washing machine, dishwasher)
- Water Heater Control
- Demand Response Programs

---

## 5. References

### 5.1 EEBUS Analysis Documents

- [SHIP Analysis](../analysis/) - Transport layer issues
- [SPINE Analysis](../analysis/) - Data model issues
- [Use Case Analysis](../analysis/) - Use case coverage gaps

### 5.2 Matter Protocol Resources

- [Matter Specification](https://csa-iot.org/developer-resource/specifications-download-request/)
- [Matter SDK](https://github.com/project-chip/connectedhomeip)
