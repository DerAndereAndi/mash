# MASH Protocol Design Document

> **M**inimal **A**pplication-layer **S**mart **H**ome Protocol
>
> A lightweight, streamlined replacement for EEBUS SHIP/SPINE

**Status:** Draft - Initial Exploration
**Created:** 2025-01-24
**Last Updated:** 2025-01-25

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

### 2.2 Transport Layer

**Protocol Stack:**
```
┌────────────────────────────────┐
│      CBOR Messages             │
├────────────────────────────────┤
│   Length-Prefix Framing (4B)   │
├────────────────────────────────┤
│         TLS 1.3                │
├────────────────────────────────┤
│           TCP                  │
├────────────────────────────────┤
│         IPv6 only              │
└────────────────────────────────┘
```

**IPv6-Only Network:**
- No IPv4 support (simplifies implementation)
- Link-local (fe80::/10) for commissioning - works without infrastructure
- Multicast (ff02::fb) for mDNS discovery
- SLAAC for auto-configuration
- Thread-ready for future mesh support

**Frame Format:**
```
┌─────────────────────────────────────────────┐
│ Length (4 bytes, big-endian) │ CBOR Payload │
└─────────────────────────────────────────────┘
```

**Keep-Alive:**
- Ping every 30 seconds if no activity
- Pong expected within 5 seconds
- Connection closed after 3 missed pongs

**Connection Model:**
- Client (controller) initiates connection to server (device)
- One persistent connection per device pair
- Automatic reconnection on disconnect

### 2.3 Key Simplifications vs EEBUS

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

## 3. Core Concepts

### 3.1 Device Model

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

**Hybrid Inverter Example:**
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

**EndpointType Enum:**
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

**Endpoint Discovery Response:**
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

**Topology is implicit from EndpointType:**
- AC endpoints (INVERTER, GRID_CONNECTION, EV_CHARGER) → connect to grid
- DC endpoints (PV_STRING, BATTERY) → internal to device, connect to DC bus
- No explicit parent/child relationships needed

**Addressing:**
```
device_id / endpoint_id / feature_id / attribute_or_command
evse-001  / 1           / Measurement / acActivePower
```

**Endpoint Conventions:**
- Endpoint 0: Reserved for root device metadata (type: DEVICE_ROOT)
- Endpoint 1+: Functional endpoints with appropriate EndpointType

### 3.2 Data Model

**Serialization:** CBOR (RFC 8949) with integer keys for compactness

**Message Size Target:** <2KB for typical messages (vs 4KB+ in SPINE)

**Key Principles:**
- Attributes are typed values (int, float, string, bool, arrays)
- Commands have parameters and optional return values
- Events are timestamped, monotonically-numbered records

### 3.3 Interaction Model

**4 Operations (like Matter, not SPINE's 7):**

| Operation | Description | Example |
|-----------|-------------|---------|
| **Read** | Get current attribute values | Read power measurement |
| **Write** | Set attribute value (full replace) | Write power limit |
| **Subscribe** | Register for change notifications | Subscribe to power changes |
| **Invoke** | Execute command with parameters | StartCharging(targetSoC: 80) |

**Key Simplifications:**
- No partial updates - Write replaces entire value
- No separate "notify" - handled by Subscribe
- No deleteAll/replaceAll distinction - just Write

**Connection Model:**
- **Client initiates** - controller connects to device (no race conditions)
- One connection per device pair
- Persistent connection with reconnect

### 3.4 Multi-Zone Model

**Devices support multiple independent controllers (zones):**

```
┌─────────────────────────────────────────────────────────────┐
│                      Device (EVSE)                           │
├─────────────────────────────────────────────────────────────┤
│  Zone 1: GRID_OPERATOR                                       │
│    └── Operational Cert from SMGW                           │
│    └── Priority 1 for LoadControl                           │
│                                                              │
│  Zone 2: HOME_MANAGER                                        │
│    └── Operational Cert from EMS                            │
│    └── Priority 3 for LoadControl                           │
│                                                              │
│  Max Zones: 5                                                │
└─────────────────────────────────────────────────────────────┘
```

**Zone Types (priority order):**
```
GRID_OPERATOR = 1     // DSO, SMGW - highest priority
BUILDING_MANAGER = 2  // Commercial building EMS
HOME_MANAGER = 3      // Residential EMS
USER_APP = 4          // Mobile apps, voice - lowest priority
```

**Priority Resolution:**
- Per-feature, not global (SMGW has priority for Limit, not everything)
- Higher priority can override lower priority's settings
- Lower priority is notified of override
- Device tracks which zone set current value
- User physical override always possible (button on device)

**Zone Roles:**
```
┌────────────────────────────────────────────────────────────┐
│  Zone Owner    │ Has Zone CA, issues certs   │ EMS, SMGW  │
│  Zone Admin    │ Can commission devices      │ Phone App  │
│  Zone Member   │ Normal participant          │ Devices    │
└────────────────────────────────────────────────────────────┘
```

- Apps are admins, NOT owners (losing phone doesn't break anything)
- Apps always need an EMS (no standalone app zones)
- Multiple admins supported (family, installers)

**App-EMS Admin Authorization:**
```
User             Phone App           EMS Web UI
 │                   │                   │
 │── "Add admin" ────┼──────────────────►│
 │◄── Temp QR (5min) ┼───────────────────┤
 │── Scan QR ───────►│── SPAKE2+ ───────►│
 │◄── "Confirm?" ────┼───────────────────┤
 │── Yes ────────────┼──────────────────►│
 │                   │◄── Admin token ───┤
```

**App Commissioning Device (as Admin):**
```
Phone App              EMS                 Device
    │                   │                    │
    │── SPAKE2+ ───────┼───────────────────►│  (app has setup code)
    │◄─────────────────┼───────── CSR ──────┤
    │── Forward CSR ──►│                    │
    │◄── Signed cert ──┤                    │
    │── Install cert ──┼───────────────────►│  (device in EMS zone)
```

**Delegated Commissioning (SMGW via backend):**
```
User           Phone App       DSO Backend       SMGW          Device
 │                 │               │               │              │
 │── Scan QR ─────►│── Upload ────►│── Forward ───►│              │
 │                 │               │               │── SPAKE2+ ──►│
 │                 │               │               │◄── Accept ───┤
```

### 3.5 Security Model

#### Certificate Hierarchy

```
┌─────────────────────────────────────────────────────────┐
│  Manufacturer CA (optional)                             │
│    └── Device Attestation Cert (20yr, pre-installed)    │
│                                                         │
│  Zone CA (controller-generated)                         │
│    └── Operational Cert (1yr, issued during pairing)    │
└─────────────────────────────────────────────────────────┘
```

- **Device Attestation**: Optional - supports manufacturer CA if present, allows self-signed
- **Operational Certs**: Controller-issued, enables rotation and clean revocation

#### Commissioning Flow (PASE-like)

```
Controller                         Device
     │                               │
     │◄── mDNS: _mash._tcp ──────────┤  Advertising
     │                               │
     │── TCP Connect ───────────────►│
     │                               │
     │◄── SPAKE2+ (setup code) ─────►│  From QR code (8 digits)
     │                               │
     │── Request Device Cert ───────►│
     │◄── Device Cert (+ chain) ─────┤  Verify if CA present
     │                               │
     │── Request CSR ───────────────►│
     │◄── CSR with new key pair ─────┤
     │                               │
     │── Install Operational Cert ──►│  Signed by Zone CA
     │                               │
     │── Commissioning Complete ────►│
     │                               │
```

#### Operational Sessions (CASE-like)

```
Controller                         Device
     │                               │
     │── TLS with Operational Cert ─►│  Mutual authentication
     │◄── Verify same Zone ──────────┤
     │                               │
     │◄── Encrypted Session ────────►│  Read/Write/Subscribe/Invoke
```

#### Certificate Lifecycle

| Certificate | Validity | Renewal | Revocation |
|-------------|----------|---------|------------|
| Device/Attestation | 20 years | Never | N/A |
| Operational | 1 year | Auto by controller | RemoveZone command |
| Zone CA | 10 years | Manual | Zone dissolution |

#### Setup Code Format

- **8 decimal digits** (00000000-99999999)
- ~27 bits entropy
- QR code contains: setup code + discriminator + vendor ID + product ID
- Optionally printed on device label for manual entry

**Trust Model:**
- Binary: paired or not paired (no SHIP trust levels 0-100)
- Pairing grants full access to announced capabilities
- Priority level determines takeover rights

### 3.6 Discovery

**Pre-Commissioning (device advertising):**
- mDNS service type: `_mash._tcp.local`
- TXT records: discriminator, vendor, product, commissioning mode

**Post-Commissioning (operational):**
- mDNS with zone-specific instance name
- TXT records: capabilities, endpoints, firmware version

**QR Code Content:**
```
MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
Example: MASH:1:1234:12345678:0x1234:0x5678
```

### 3.7 Capability Discovery

**Problem:** MASH has multiple features with optional attributes. Controllers need to know:
- Which features a device implements
- Which optional attributes are present within each feature
- Which commands the device accepts
- What the device's capabilities are (without reading each attribute)

**Solution:** Matter-style global attributes present on every device/endpoint.

#### 3.7.1 Global Attributes

Every endpoint MUST implement these global attributes (reserved IDs 0xFFF0-0xFFFF):

| Attribute | ID | Type | Description |
|-----------|-----|------|-------------|
| `clusterRevision` | 0xFFFD | uint16 | MASH protocol version this endpoint implements |
| `featureMap` | 0xFFFC | bitmap32 | Bit flags indicating supported optional features |
| `attributeList` | 0xFFFB | array[uint16] | IDs of all implemented attributes |
| `acceptedCommandList` | 0xFFFA | array[uint8] | Command IDs the endpoint accepts |
| `generatedCommandList` | 0xFFF9 | array[uint8] | Command IDs the endpoint generates (responses) |
| `eventList` | 0xFFF8 | array[uint8] | Event IDs the endpoint can emit |

```cbor
// Example: Reading global attributes from an EVSE endpoint
{
  0xFFFD: 1,                           // clusterRevision: v1
  0xFFFC: 0x001B,                      // featureMap: CORE|FLEX|EMOB|SIGNALS
  0xFFFB: [1, 2, 3, 10, 11, 14, 20, 21, 60, ...],  // attributeList
  0xFFFA: [1, 2, 5, 6, 10, 11],        // acceptedCommandList
  0xFFF9: [1, 2, 5, 6, 10, 11],        // generatedCommandList
  0xFFF8: [1, 2]                       // eventList
}
```

#### 3.7.2 Feature Map

The `featureMap` bitmap indicates which optional feature sets the device supports:

```
FeatureMapBits:
  bit 0  (0x0001): CORE      - EnergyCore basics (always set for energy devices)
  bit 1  (0x0002): FLEX      - Flexible power adjustment (FlexibilityStruct)
  bit 2  (0x0004): BATTERY   - Battery-specific attributes (SoC, SoH, capacity)
  bit 3  (0x0008): EMOB      - E-Mobility/EVSE (charging sessions, EV state)
  bit 4  (0x0010): SIGNALS   - Incentive signals support
  bit 5  (0x0020): TARIFF    - Tariff data support
  bit 6  (0x0040): PLAN      - Power plan support
  bit 7  (0x0080): PROCESS   - Optional process lifecycle (OHPCF-style)
  bit 8  (0x0100): FORECAST  - Power forecasting capability
  bit 9  (0x0200): ASYMMETRIC - Per-phase asymmetric control
  bit 10 (0x0400): V2X       - Vehicle-to-grid/home (bidirectional EV)
```

**Feature-Dependent Attribute Conformance:**

| Attribute | Mandatory If | Optional If |
|-----------|-------------|-------------|
| `stateOfCharge`, `stateOfHealth`, `batteryCapacity` | BATTERY | - |
| `sessionEnergy`, `evseState`, `connectedVehicle` | EMOB | - |
| `flexibility` | - | FLEX |
| `forecast` | - | FORECAST |
| `processState`, `optionalProcess` | PROCESS | - |
| `effectiveCurrentSetpointsConsumption/Production` | ASYMMETRIC | - |
| Signals feature attributes | SIGNALS | - |
| Tariff feature attributes | TARIFF | - |
| Plan feature attributes | PLAN | - |

#### 3.7.3 Discovery Flow

When a controller connects to a device:

```
1. Read endpoint list (discover device structure)
   → Get: [{id: 0, type: DEVICE_ROOT}, {id: 1, type: EV_CHARGER}, ...]

2. For each endpoint, read global attributes:
   a. featureMap       → Which feature sets are supported
   b. attributeList    → Which specific attributes are implemented
   c. acceptedCommandList → Which commands can be invoked
   d. clusterRevision  → Protocol version for compatibility

3. Based on featureMap, controller knows:
   - If EMOB (0x0008) is set → ChargingSession attributes available
   - If BATTERY (0x0004) is set → Battery attributes available
   - If PROCESS (0x0080) is set → OHPCF-style scheduling available
   - etc.

4. attributeList provides exact attribute IDs for fine-grained discovery
```

#### 3.7.4 Example Configurations

**Basic EVSE (V1G, no flexibility):**
```cbor
{
  featureMap: 0x0009,        // CORE | EMOB
  attributeList: [1, 2, 3, 10, 11, 14, 20, 21, 0xFFF8, 0xFFF9, 0xFFFA, 0xFFFB, 0xFFFC, 0xFFFD],
  acceptedCommandList: [1, 2, 5, 6],     // SetLimit, ClearLimit, SetCurrentLimits, ClearCurrentLimits
  generatedCommandList: [1, 2, 5, 6]
}
```

**Advanced V2H EVSE (bidirectional, asymmetric, flexibility):**
```cbor
{
  featureMap: 0x060B,        // CORE | FLEX | EMOB | ASYMMETRIC | V2X
  attributeList: [1, 2, 3, 10-16, 20-23, 30-33, 40-43, 50-53, 60, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],  // All limit/setpoint commands + Pause/Resume
  generatedCommandList: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
}
```

**Heat Pump with Optional Process (OHPCF):**
```cbor
{
  featureMap: 0x0083,        // CORE | FLEX | PROCESS
  attributeList: [1, 2, 3, 10, 14, 16, 20, 21, 60, 70-72, 80, 81, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 9, 10, 11, 12, 13],  // SetLimit, ClearLimit, Pause, Resume, Stop, ScheduleProcess, CancelProcess
  generatedCommandList: [1, 2, 9, 10, 11, 12, 13]
}
```

**Battery Storage:**
```cbor
{
  featureMap: 0x0107,        // CORE | FLEX | BATTERY | FORECAST
  attributeList: [1, 2, 3, 10-16, 20-23, 40-43, 50-53, 60, 61, 70-72, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 3, 4, 7, 8, 9, 10],  // Limits + Setpoints + Pause/Resume
  generatedCommandList: [1, 2, 3, 4, 7, 8, 9, 10]
}
```

#### 3.7.5 Benefits

1. **Self-describing**: Controller knows exactly what's available without trial/error
2. **Version-safe**: `clusterRevision` enables graceful protocol evolution
3. **Fine-grained**: `attributeList` gives exact attribute availability
4. **Compact**: Bitmap `featureMap` is efficient for quick capability checks
5. **Predictable**: No implicit assumptions about what "EVSE" means

---

## 4. Core Features

**Feature Separation Philosophy:**

| Feature | Question | Update Frequency |
|---------|----------|------------------|
| **Electrical** | "What CAN this do?" | Rarely (hardware/installation) |
| **Measurement** | "What IS it doing?" | Constantly (telemetry) |
| **EnergyControl** | "What SHOULD it do?" | On command (control) |

**Device Composition Example:**
```
Device (evse-001)
├── Endpoint 0 (Root)
│   └── DeviceInfo
└── Endpoint 1 (Charger Port)
    ├── Electrical          ← Phase config, ratings
    ├── Measurement         ← Power, energy, per-phase readings
    ├── EnergyControl       ← Limits, control state
    └── ChargingSession     ← EV session info (EVSE only)
```

---

### 4.1 Electrical Feature

**Purpose:** Describes the static electrical characteristics of an endpoint. Read once at discovery, rarely changes.

#### 4.1.1 Attributes

```cbor
Electrical Feature Attributes:
{
  // Phase configuration
  1: phaseCount,              // uint8: 1, 2, or 3
  2: phaseMapping,            // map: {DevicePhase → GridPhase}

  // Voltage/frequency
  3: nominalVoltage,          // uint16 V (e.g., 230, 400)
  4: nominalFrequency,        // uint8 Hz (e.g., 50, 60)

  // Direction capability
  5: supportedDirections,     // DirectionEnum

  // Nameplate power ratings (hardware limits)
  10: nominalMaxConsumption,  // int64 mW (max charge/consumption power)
  11: nominalMaxProduction,   // int64 mW (max discharge/production power, 0 if N/A)
  12: nominalMinPower,        // int64 mW (minimum operating point)

  // Nameplate current ratings (per-phase)
  13: maxCurrentPerPhase,     // int64 mA (e.g., 32000 = 32A)
  14: minCurrentPerPhase,     // int64 mA (e.g., 6000 = 6A)

  // Per-phase capabilities
  15: supportsAsymmetric,     // AsymmetricSupportEnum (which directions support different values per phase)

  // For storage devices
  20: energyCapacity          // int64 mWh (battery size, 0 if N/A)
}
```

#### 4.1.2 Enumerations

**DirectionEnum:**
```
CONSUMPTION       = 0x00  // Can only consume power
PRODUCTION        = 0x01  // Can only produce power
BIDIRECTIONAL     = 0x02  // Can consume and produce
```

**AsymmetricSupportEnum:**
```
NONE              = 0x00  // Symmetric only (all phases must have same value)
CONSUMPTION       = 0x01  // Asymmetric consumption (different values per phase when charging)
PRODUCTION        = 0x02  // Asymmetric production (different values per phase when discharging)
BIDIRECTIONAL     = 0x03  // Asymmetric both directions
```

**PhaseEnum (Device Phase):**
```
A                 = 0x00
B                 = 0x01
C                 = 0x02
```

**GridPhaseEnum:**
```
L1                = 0x00
L2                = 0x01
L3                = 0x02
```

#### 4.1.3 Phase Mapping Examples

**3-Phase EVSE (standard rotation):**
```cbor
{
  phaseCount: 3,
  phaseMapping: {A: L1, B: L2, C: L3},
  maxCurrentPerPhase: 32000,
  supportsAsymmetric: CONSUMPTION    // Can charge asymmetrically
}
```

**3-Phase V2H (bidirectional per-phase, asymmetric both ways):**
```cbor
{
  phaseCount: 3,
  phaseMapping: {A: L1, B: L2, C: L3},
  maxCurrentPerPhase: 32000,
  supportsAsymmetric: BIDIRECTIONAL  // Can charge AND discharge asymmetrically
}
```

**Battery inverter (balanced phases):**
```cbor
{
  phaseCount: 3,
  phaseMapping: {A: L1, B: L2, C: L3},
  maxCurrentPerPhase: 25000,
  supportsAsymmetric: NONE           // Always symmetric, inverter balances phases
}
```

**1-Phase EVSE (connected to L3):**
```cbor
{
  phaseCount: 1,
  phaseMapping: {A: L3},
  maxCurrentPerPhase: 32000,
  supportsAsymmetric: NONE           // Single phase, N/A
}
```

**3-Phase with rotated installation:**
```cbor
{
  phaseCount: 3,
  phaseMapping: {A: L2, B: L3, C: L1},
  // Device's Phase A is connected to grid's L2
}
```

---

### 4.2 Measurement Feature

**Purpose:** Provides real-time telemetry - power, energy, voltage, current readings. Subscribe for continuous updates. Supports both AC (grid-facing) and DC (internal) measurements.

#### 4.2.1 Attributes

```cbor
Measurement Feature Attributes:
{
  // ═══════════════════════════════════════════════════════════
  // AC POWER (signed: + consumption, - production)
  // Used by: INVERTER, GRID_CONNECTION, EV_CHARGER, etc.
  // ═══════════════════════════════════════════════════════════

  // Total AC power
  1: acActivePower,            // int64 mW (active/real power)
  2: acReactivePower,          // int64 mVAR (reactive power)
  3: acApparentPower,          // uint64 mVA (apparent power, always positive)

  // Per-phase AC power (optional)
  // map: {PhaseEnum → int64 mW}
  10: acActivePowerPerPhase,
  11: acReactivePowerPerPhase,
  12: acApparentPowerPerPhase,

  // ═══════════════════════════════════════════════════════════
  // AC CURRENT & VOLTAGE (per-phase)
  // ═══════════════════════════════════════════════════════════

  // map: {PhaseEnum → int64 mA} (signed: + consumption, - production)
  20: acCurrentPerPhase,

  // map: {PhaseEnum → uint32 mV} (phase-to-neutral voltage, RMS)
  21: acVoltagePerPhase,

  // map: {PhasePairEnum → uint32 mV} (phase-to-phase voltage, optional)
  22: acVoltagePhaseToPhasePair,

  // Grid frequency
  23: acFrequency,             // uint32 mHz (e.g., 50000 = 50.000 Hz)

  // Power factor (cos φ)
  24: powerFactor,             // int16 (0.001 units, -1000 to +1000 = -1.0 to +1.0)

  // ═══════════════════════════════════════════════════════════
  // AC ENERGY (cumulative since install/reset)
  // ═══════════════════════════════════════════════════════════

  30: acEnergyConsumed,        // uint64 mWh (total consumed from grid)
  31: acEnergyProduced,        // uint64 mWh (total produced/fed-in to grid)

  // ═══════════════════════════════════════════════════════════
  // DC MEASUREMENTS (for PV strings, batteries)
  // Used by: PV_STRING, BATTERY endpoints
  // ═══════════════════════════════════════════════════════════

  40: dcPower,                 // int64 mW (+ into device, - out of device)
  41: dcCurrent,               // int64 mA (+ into device, - out of device)
  42: dcVoltage,               // uint32 mV

  // DC energy (cumulative)
  43: dcEnergyIn,              // uint64 mWh (energy into this component)
  44: dcEnergyOut,             // uint64 mWh (energy out of this component)

  // ═══════════════════════════════════════════════════════════
  // BATTERY STATE (for BATTERY endpoints)
  // ═══════════════════════════════════════════════════════════

  50: stateOfCharge,           // uint8 % (0-100)
  51: stateOfHealth,           // uint8 % (0-100, battery degradation)
  52: stateOfEnergy,           // uint64 mWh (available energy at current SoC)
  53: useableCapacity,         // uint64 mWh (current useable capacity)
  54: cycleCount,              // uint32 (charge/discharge cycles)

  // ═══════════════════════════════════════════════════════════
  // TEMPERATURE
  // ═══════════════════════════════════════════════════════════

  60: temperature,             // int16 centi-°C (e.g., 2500 = 25.00°C)
}
```

#### 4.2.2 Enumerations

**PhasePairEnum (for phase-to-phase voltage):**
```
AB                = 0x00  // Voltage between phase A and B
BC                = 0x01  // Voltage between phase B and C
CA                = 0x02  // Voltage between phase C and A
```

#### 4.2.3 Sign Convention

**Passive sign convention (load convention):**
- **Positive** = power/current flowing INTO the component (consumption, charging)
- **Negative** = power/current flowing OUT of the component (production, discharging)

| Component | Positive (+) | Negative (-) |
|-----------|--------------|--------------|
| Inverter AC | Consuming from grid | Feeding to grid |
| PV String | N/A (always produces) | Producing |
| Battery DC | Charging | Discharging |
| EV Charger | Charging EV | V2G discharge |

#### 4.2.4 Examples by Endpoint Type

**Inverter AC endpoint (feeding 5kW to grid):**
```cbor
{
  acActivePower: -5000000,                  // -5 kW (producing)
  acReactivePower: 200000,                  // 200 VAR
  acApparentPower: 5004000,                 // 5.004 kVA
  acCurrentPerPhase: {A: -7200, B: -7300, C: -7100},  // ~7A per phase out
  acVoltagePerPhase: {A: 230500, B: 231000, C: 229800},
  acFrequency: 50020,                       // 50.02 Hz
  powerFactor: -998,                        // -0.998 (producing)
  acEnergyConsumed: 125000000,              // 125 kWh lifetime from grid
  acEnergyProduced: 8450000000              // 8450 kWh lifetime to grid
}
```

**PV String endpoint (producing 3.2kW):**
```cbor
{
  dcPower: -3200000,                        // -3.2 kW (producing)
  dcCurrent: -8000,                         // -8 A out
  dcVoltage: 400000,                        // 400 V
  dcEnergyOut: 4200000000                   // 4200 kWh total yield
}
```

**Battery endpoint (charging at 2kW):**
```cbor
{
  dcPower: 2000000,                         // +2 kW (charging)
  dcCurrent: 4000,                          // +4 A in
  dcVoltage: 500000,                        // 500 V
  dcEnergyIn: 850000000,                    // 850 kWh total charged
  dcEnergyOut: 780000000,                   // 780 kWh total discharged
  stateOfCharge: 65,                        // 65%
  stateOfHealth: 97,                        // 97%
  stateOfEnergy: 6500000,                   // 6.5 kWh available
  useableCapacity: 10000000,                // 10 kWh useable
  cycleCount: 342,                          // 342 cycles
  temperature: 2850                         // 28.5°C
}
```

**3-Phase EVSE (charging EV at 11kW):**
```cbor
{
  acActivePower: 11040000,                  // +11.04 kW (consuming)
  acCurrentPerPhase: {A: 16000, B: 16000, C: 16000},
  acVoltagePerPhase: {A: 230000, B: 231000, C: 229000},
  acFrequency: 50000,                       // 50.00 Hz
  acEnergyConsumed: 2500000000              // 2500 kWh lifetime
}
```

**Grid Connection Point / Smart Meter (house importing 3kW):**
```cbor
{
  acActivePower: 3000000,                   // +3 kW import
  acReactivePower: 150000,                  // 150 VAR
  acCurrentPerPhase: {A: 5200, B: 4100, C: 3800},
  acVoltagePerPhase: {A: 230200, B: 230800, C: 229500},
  acFrequency: 49980,                       // 49.98 Hz
  acEnergyConsumed: 45000000000,            // 45000 kWh imported
  acEnergyProduced: 12000000000             // 12000 kWh exported
}
```

#### 4.2.5 EEBUS Use Case Coverage

| EEBUS Use Case | Measurement Attributes Used |
|----------------|----------------------------|
| **MPC** (Power Consumption) | acActivePower, acActivePowerPerPhase, acCurrentPerPhase, acVoltagePerPhase, acFrequency, acEnergyConsumed/Produced |
| **MGCP** (Grid Connection) | Same as MPC (on GRID_CONNECTION endpoint) |
| **EVCEM** (EV Measurement) | acActivePower, acCurrentPerPhase, acEnergyConsumed |
| **MOI** (Inverter) | All AC power types, powerFactor, acEnergy, temperature |
| **MOB** (Battery) | dcPower, dcCurrent, dcVoltage, dcEnergy, stateOfCharge/Health/Energy, cycleCount, temperature |
| **MPS** (PV String) | dcPower, dcCurrent, dcVoltage, dcEnergyOut |

---

### 4.3 EnergyControl Feature

**Purpose:** Provides control capabilities - limits, setpoints, pause/resume, forecasts. The main control interface.

#### 4.3.1 Attributes

```cbor
EnergyControl Feature Attributes:
{
  // Device type and control state
  1: deviceType,              // DeviceTypeEnum
  2: controlState,            // ControlStateEnum (explicit control relationship status)
  3: optOutState,             // OptOutEnum

  // Control capabilities - what commands this device accepts
  10: acceptsLimits,          // bool - accepts SetLimit (total power)
  11: acceptsCurrentLimits,   // bool - accepts SetCurrentLimits (per-phase current)
  12: acceptsSetpoints,       // bool - accepts SetSetpoint (total power)
  13: acceptsCurrentSetpoints,// bool - accepts SetCurrentSetpoints (per-phase, for V2H)
  14: isPausable,             // bool - accepts Pause/Resume
  15: isShiftable,            // bool - accepts AdjustStartTime
  16: isStoppable,            // bool - accepts Stop (abort task completely)

  // Power LIMITS (hard constraint: "do not exceed")
  // Resolution: most restrictive wins (min of all zones)
  20: effectiveConsumptionLimit,  // int64 mW (min of all zones)
  21: myConsumptionLimit,         // int64 mW (this zone's limit)
  22: effectiveProductionLimit,   // int64 mW (min of all zones)
  23: myProductionLimit,          // int64 mW (this zone's limit)

  // Phase current LIMITS - consumption direction
  // map: {PhaseEnum → int64 mA}
  30: effectiveCurrentLimitsConsumption,
  31: myCurrentLimitsConsumption,

  // Phase current LIMITS - production direction
  // map: {PhaseEnum → int64 mA}
  32: effectiveCurrentLimitsProduction,
  33: myCurrentLimitsProduction,

  // Power SETPOINTS (target: "please try to achieve")
  // Resolution: highest priority zone wins (only one active)
  40: effectiveConsumptionSetpoint,  // int64 mW (from controlling zone)
  41: myConsumptionSetpoint,         // int64 mW (this zone's setpoint)
  42: effectiveProductionSetpoint,   // int64 mW (from controlling zone)
  43: myProductionSetpoint,          // int64 mW (this zone's setpoint)

  // Phase current SETPOINTS - consumption direction (for V2H phase balancing)
  // map: {PhaseEnum → int64 mA}
  50: effectiveCurrentSetpointsConsumption,
  51: myCurrentSetpointsConsumption,

  // Phase current SETPOINTS - production direction (for V2H phase balancing)
  // map: {PhaseEnum → int64 mA}
  52: effectiveCurrentSetpointsProduction,
  53: myCurrentSetpointsProduction,

  // Optional: Flexibility/Forecast
  60: flexibility,            // FlexibilityStruct (optional)
  61: forecast,               // ForecastStruct (optional)

  // Failsafe configuration (for when controller connection is lost)
  70: failsafeConsumptionLimit,   // int64 mW: limit to apply in FAILSAFE state
  71: failsafeProductionLimit,    // int64 mW: limit to apply in FAILSAFE state
  72: failsafeDuration,           // uint32 s: time in FAILSAFE before AUTONOMOUS (2-24h)

  // Optional process management (for OHPCF-style optional tasks)
  80: processState,               // ProcessStateEnum: current process lifecycle state
  81: optionalProcess,            // OptionalProcess?: details of available/running process
}
```

#### 4.3.2 Enumerations

**DeviceTypeEnum:**
```
EVSE              = 0x00  // EV Charger
HEAT_PUMP         = 0x01  // Space heating/cooling
WATER_HEATER      = 0x02
BATTERY           = 0x03  // Home battery storage
INVERTER          = 0x04  // Solar/hybrid inverter
FLEXIBLE_LOAD     = 0x05  // Generic controllable load
OTHER             = 0xFF
```

**ControlStateEnum:**
```
AUTONOMOUS        = 0x00  // Not under external control (no controller connected)
CONTROLLED        = 0x01  // Under controller authority, no active limit
LIMITED           = 0x02  // Active power limit being applied
FAILSAFE          = 0x03  // Connection lost, using failsafe limits
OVERRIDE          = 0x04  // Device overriding limits (safety/legal/self-protection)
```

**Design rationale:** Device explicitly reports its control relationship state. This replaces
implicit heartbeat-based inference (bad: race conditions, debugging difficulty, no single
source of truth). Applies to ALL controllable device types: EVSE, battery, heat pump, inverter.

When connection to controller is lost:
1. Device detects connection loss (TCP/TLS layer)
2. Device transitions to FAILSAFE, applies failsafeConsumptionLimit/failsafeProductionLimit
3. After failsafeDuration expires, device transitions to AUTONOMOUS
4. Device can resume normal operation without controller

**ProcessStateEnum (for OHPCF-style optional task lifecycle):**
```
NONE              = 0x00  // No optional process available
AVAILABLE         = 0x01  // Process announced, not yet scheduled
SCHEDULED         = 0x02  // Start time configured, waiting to start
RUNNING           = 0x03  // Process currently executing
PAUSED            = 0x04  // Paused by controller (can resume)
COMPLETED         = 0x05  // Finished successfully
ABORTED           = 0x06  // Stopped/cancelled before completion
```

**LimitCauseEnum:**
```
GRID_EMERGENCY     = 0   // DSO/SMGW - MUST follow
GRID_OPTIMIZATION  = 1   // Grid balancing request
LOCAL_PROTECTION   = 2   // Fuse/overload protection
LOCAL_OPTIMIZATION = 3   // Cost/self-consumption optimization
USER_PREFERENCE    = 4   // User app request
```

**SetpointCauseEnum:**
```
GRID_REQUEST       = 0   // Grid operator/aggregator request
SELF_CONSUMPTION   = 1   // Optimize for self-consumption
PRICE_OPTIMIZATION = 2   // Optimize for energy cost
PHASE_BALANCING    = 3   // Balance load across phases (V2H)
USER_PREFERENCE    = 4   // User app request
```

**OptOutEnum:**
```
NO_OPT_OUT        = 0   // Accept all adjustments
LOCAL_OPT_OUT     = 1   // Reject local optimization only
GRID_OPT_OUT      = 2   // Reject grid requests only
OPT_OUT           = 3   // Reject all external control
```

#### 4.3.3 Data Structures

**FlexibilityStruct (OPTIONAL):**
```cbor
{
  1: earliestStart,          // timestamp (optional)
  2: latestEnd,              // timestamp (optional)
  3: energyMin,              // int64 mWh (optional)
  4: energyMax,              // int64 mWh (optional)
  5: energyTarget,           // int64 mWh (optional)
  6: powerRangeMin,          // int64 mW
  7: powerRangeMax,          // int64 mW
  8: minRunDuration,         // uint32 s (optional)
  9: maxPauseDuration        // uint32 s (optional)
}
```

**ForecastStruct (OPTIONAL):**
```cbor
{
  1: forecastId,             // uint32
  2: startTime,              // timestamp
  3: endTime,                // timestamp
  4: slots                   // array of ForecastSlot (max 10)
}

ForecastSlot:
{
  1: duration,               // uint32 s
  2: nominalPower,           // int64 mW
  3: minPower,               // int64 mW (optional)
  4: maxPower,               // int64 mW (optional)
  5: isPausable              // bool (optional)
}
```

**OptionalProcess (for OHPCF-style optional tasks):**
```cbor
{
  // Process identification
  1: processId,              // uint32: unique identifier for this process
  2: description,            // string?: human-readable description (e.g., "Hot water heating")

  // Power characteristics
  10: powerEstimate,         // int64 mW?: expected average power during execution
  11: powerMin,              // int64 mW?: minimum operating power
  12: powerMax,              // int64 mW?: maximum operating power

  // Timing constraints
  20: estimatedDuration,     // uint32 s?: expected total duration
  21: minRunDuration,        // uint32 s: minimum time before can pause/stop
  22: minPauseDuration,      // uint32 s?: minimum time between pause/resume cycles

  // Control constraints
  30: isPausable,            // bool: can this process be paused?
  31: isStoppable,           // bool: can this process be stopped/aborted?

  // Energy characteristics
  40: energyEstimate,        // int64 mWh?: expected total energy consumption
  41: resumeEnergyPenalty,   // int64 mWh?: additional energy needed if resumed after pause

  // Scheduling (set by controller)
  50: scheduledStart,        // timestamp?: when controller scheduled this to start
}
```

**Usage:** Device announces optional process by setting `processState = AVAILABLE` and populating
`optionalProcess`. Controller schedules it via `ScheduleProcess` command. Device reports state
transitions through `processState`. Used for heat pump compressor runs, water heater cycles,
dishwasher programs, or any deferrable task the controller can schedule.

#### 4.3.4 Commands

**SetLimit** - Set power limits for this zone
```cbor
Request:
{
  1: consumptionLimit,       // int64 mW (optional - max consumption/charge)
  2: productionLimit,        // int64 mW (optional - max production/discharge)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // LimitCauseEnum
}
// At least one of consumptionLimit or productionLimit must be set

Response:
{
  1: success,                // bool
  2: effectiveConsumptionLimit,  // int64 mW
  3: effectiveProductionLimit    // int64 mW
}
```

**ClearLimit** - Remove this zone's power limit(s)
```cbor
Request:
{
  1: direction               // DirectionEnum (optional - if omitted, clears both)
}

Response:
{
  1: success                 // bool
}
```

**SetCurrentLimits** - Set per-phase current limits
```cbor
Request:
{
  1: phases,                 // map: {PhaseEnum → int64 mA or null}
  2: direction,              // DirectionEnum (CONSUMPTION or PRODUCTION)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // LimitCauseEnum
}
// phases map semantics:
//   Key present with value: set that phase to value
//   Key present with null: clear that phase
//   Key absent: leave unchanged

Response:
{
  1: success,                // bool
  2: effectivePhaseCurrents  // map: {PhaseEnum → int64 mA}
}
```

**ClearCurrentLimits** - Remove this zone's per-phase current limits
```cbor
Request:
{
  1: direction               // DirectionEnum (optional - if omitted, clears both)
}

Response:
{
  1: success                 // bool
}
```

**SetSetpoint** - Set power setpoint for this zone (target to achieve)
```cbor
Request:
{
  1: consumptionSetpoint,    // int64 mW (optional - target charge power)
  2: productionSetpoint,     // int64 mW (optional - target discharge power)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // SetpointCauseEnum
}
// At least one of consumptionSetpoint or productionSetpoint must be set
// Setpoint is constrained by any active limits

Response:
{
  1: success,                // bool
  2: effectiveConsumptionSetpoint,  // int64 mW
  3: effectiveProductionSetpoint    // int64 mW
}
```

**ClearSetpoint** - Remove this zone's power setpoint(s)
```cbor
Request:
{
  1: direction               // DirectionEnum (optional - if omitted, clears both)
}

Response:
{
  1: success                 // bool
}
```

**SetCurrentSetpoints** - Set per-phase current setpoints (for V2H phase balancing)
```cbor
Request:
{
  1: phases,                 // map: {PhaseEnum → int64 mA or null}
  2: direction,              // DirectionEnum (CONSUMPTION or PRODUCTION)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // SetpointCauseEnum
}
// phases map semantics:
//   Key present with value: set that phase to value
//   Key present with null: clear that phase
//   Key absent: leave unchanged
// Requires: acceptsCurrentSetpoints = true AND supportsAsymmetric includes direction

Response:
{
  1: success,                // bool
  2: effectiveCurrentSetpoints  // map: {PhaseEnum → int64 mA}
}
```

**ClearCurrentSetpoints** - Remove this zone's per-phase current setpoints
```cbor
Request:
{
  1: direction               // DirectionEnum (optional - if omitted, clears both)
}

Response:
{
  1: success                 // bool
}
```

**Pause** - Temporarily pause device operation
```cbor
Request:
{
  1: duration                // uint32 s (optional, 0 = indefinite)
}

Response:
{
  1: success                 // bool
}
```

**Resume** - Resume paused operation
```cbor
Request: (empty)

Response:
{
  1: success                 // bool
}
```

**Stop** - Abort task completely (requires isStoppable = true)
```cbor
Request: (empty)

Response:
{
  1: success                 // bool
}
```

**ScheduleProcess** - Schedule an optional process to start (OHPCF-style)
```cbor
Request:
{
  1: processId,              // uint32: which process to schedule
  2: requestedStart,         // timestamp?: when to start (null = start now)
  3: cause                   // SetpointCauseEnum: why scheduling
}
// Requires: processState = AVAILABLE

Response:
{
  1: success,                // bool
  2: actualStart,            // timestamp: when it will actually start
  3: newState                // ProcessStateEnum: new process state (SCHEDULED or RUNNING)
}
```

**CancelProcess** - Cancel a scheduled or running process
```cbor
Request:
{
  1: processId               // uint32: which process to cancel
}
// Requires: processState in (SCHEDULED, RUNNING, PAUSED)

Response:
{
  1: success,                // bool
  2: newState                // ProcessStateEnum: new process state (ABORTED or NONE)
}
```

**AdjustStartTime** - Request start time shift
```cbor
Request:
{
  1: requestedStart,         // timestamp
  2: cause                   // LimitCauseEnum
}

Response:
{
  1: success,                // bool
  2: actualStart             // timestamp
}
```

#### 4.3.5 Limit and Setpoint Resolution (Multi-Zone)

**Key difference:**
```
LIMITS:    Most restrictive wins (all zones constrain together)
SETPOINTS: Highest priority zone wins (only one controller active)
```

**Power limits** - most restrictive wins:
```
Zone 1 (GRID_OPERATOR): SetLimit(consumptionLimit: 6000000)
Zone 2 (HOME_MANAGER):  SetLimit(consumptionLimit: 5000000)

effectiveConsumptionLimit = min(6000000, 5000000) = 5000000 mW
```

**Phase current limits** - most restrictive per phase:
```
Zone 1: SetCurrentLimits({A: 20000, B: 20000, C: 20000}, CONSUMPTION)
Zone 2: SetCurrentLimits({A: 16000, B: 10000, C: 16000}, CONSUMPTION)

effectiveCurrentLimitsConsumption = {
  A: min(20000, 16000) = 16000,
  B: min(20000, 10000) = 10000,
  C: min(20000, 16000) = 16000
}
```

**Power setpoints** - highest priority zone wins:
```
Zone 1 (GRID_OPERATOR, priority 1): SetSetpoint(consumptionSetpoint: 3000000)
Zone 2 (HOME_MANAGER, priority 2):  SetSetpoint(consumptionSetpoint: 5000000)

effectiveConsumptionSetpoint = 3000000 mW (grid operator wins)
```

**Per-phase current setpoints** - highest priority zone wins:
```
Zone 2 (HOME_MANAGER): SetCurrentSetpoints({A: 10000, B: 2000, C: 5000}, PRODUCTION)

effectiveCurrentSetpointsProduction = {A: 10000, B: 2000, C: 5000}
```

**Combined resolution** - limits constrain setpoints:
```
effectiveConsumptionLimit = 5000000 mW (5 kW)
effectiveConsumptionSetpoint = 7000000 mW (7 kW requested)

Device targets: min(7000000, 5000000) = 5000000 mW (limit caps setpoint)
```

**V2H phase balancing example:**
```
Scenario: House consumption L1=20A, L2=5A, L3=12A at 230V
          Grid limit 25A per phase
          EMS wants EV to discharge asymmetrically to balance

1. Grid operator sets limit:
   SetCurrentLimits({A: 25000, B: 25000, C: 25000}, PRODUCTION)

2. Home EMS sets asymmetric discharge setpoint:
   SetCurrentSetpoints({A: 10000, B: 2000, C: 5000}, PRODUCTION)
   cause: PHASE_BALANCING

3. V2H EV receives:
   effectiveCurrentLimitsProduction = {A: 25000, B: 25000, C: 25000}
   effectiveCurrentSetpointsProduction = {A: 10000, B: 2000, C: 5000}

4. EV discharges: 10A on L1, 2A on L2, 5A on L3
   Result: Net house import = L1: 10A, L2: 3A, L3: 7A (balanced)
```

#### 4.3.6 State Machines

**ControlStateEnum - Control Relationship State:**
```
                    ┌──────────────┐
                    │  AUTONOMOUS  │◄──── failsafeDuration expired
                    └──────┬───────┘
                           │ controller connects
                           ▼
                    ┌──────────────┐
          ┌─────────│  CONTROLLED  │◄─────────┐
          │         └──────┬───────┘          │
          │                │ SetLimit()       │ ClearLimit() / expires
          │                ▼                  │
          │         ┌──────────────┐          │
          │         │   LIMITED    │──────────┘
          │         └──────┬───────┘
          │                │
          │ connection     │ self-protection/safety
          │ lost           │
          │                ▼
          │         ┌──────────────┐
          │         │   OVERRIDE   │── condition cleared ──►(back to LIMITED)
          │         └──────────────┘
          │
          │
          │                │
          ▼                │
   ┌──────────────┐        │
   │   FAILSAFE   │◄───────┘ (connection lost)
   └──────────────┘
         │
         │ failsafeDuration expires
         ▼
   (back to AUTONOMOUS)
```

**ProcessStateEnum - Optional Task Lifecycle:**
```
                    ┌──────────────┐
     device has  ──►│     NONE     │◄── task unavailable / completed
     no task        └──────┬───────┘
                           │ device announces optional task
                           ▼
                    ┌──────────────┐
                    │  AVAILABLE   │◄──────────────────┐
                    └──────┬───────┘                   │
                           │ ScheduleProcess()        │ CancelProcess()
                           ▼                          │
                    ┌──────────────┐                   │
                    │  SCHEDULED   │───────────────────┤
                    └──────┬───────┘                   │
                           │ scheduled time reached   │
                           ▼                          │
                    ┌──────────────┐                   │
     ┌──────────────│   RUNNING    │───────────────────┤
     │ Pause()      └──────┬───────┘                   │
     ▼                     │ task finishes            │
┌──────────┐               ▼                          │
│  PAUSED  │        ┌──────────────┐                   │
└────┬─────┘        │  COMPLETED   │                   │
     │ Resume()     └──────────────┘                   │
     │                                                │
     └────────────────────────────────────────────────┘
                     Stop() / CancelProcess()
                           │
                           ▼
                    ┌──────────────┐
                    │   ABORTED    │
                    └──────────────┘
```

**Orthogonal state machines:** ControlStateEnum and ProcessStateEnum are independent.
A device can be `controlState=LIMITED` while `processState=RUNNING` (executing an
optional task under power constraints). Or `controlState=FAILSAFE` with
`processState=PAUSED` (lost controller, task paused waiting for reconnection).

#### 4.3.7 EEBUS Use Case Mapping

| EEBUS Use Case | MASH Coverage |
|----------------|---------------|
| LPC (Limit Power Consumption) | SetLimit(consumptionLimit), ControlStateEnum (LIMITED/FAILSAFE), failsafe* attrs |
| LPP (Limit Power Production) | SetLimit(productionLimit), ControlStateEnum |
| OPEV (Overload Protection) | SetCurrentLimits(phases, CONSUMPTION) |
| OPEV asymmetric | SetCurrentLimits({A: x, B: y, C: z}, CONSUMPTION) |
| OSCEV (Self-Consumption) | FlexibilityStruct + SetSetpoint(cause: SELF_CONSUMPTION) |
| CEVC (Coordinated Charging) | ForecastStruct + FlexibilityStruct |
| DBEVC (Bidirectional EV) | SetSetpoint(consumptionSetpoint, productionSetpoint) |
| COB (Battery Control) | SetSetpoint + ControlStateEnum (failsafe for battery safety) |
| OHPCF (Heat Pump) | OptionalProcess + ProcessStateEnum + ScheduleProcess + Pause/Resume/Stop |
| POEN (Power Envelope) | Repeated SetLimit (schedule extension TBD) |

**Explicit state reporting improvements over EEBUS:**

| EEBUS Approach | MASH Improvement |
|----------------|------------------|
| LPC heartbeat-based state inference | `controlState` explicitly reported by device |
| LPC implicit failsafe detection | Device reports `controlState=FAILSAFE` directly |
| OHPCF implicit process status | `processState` explicitly tracks task lifecycle |
| No unified control state across use cases | Same `ControlStateEnum` for LPC, COB, EVSE, OHPCF |

**MASH Extends Beyond EEBUS:**

| New Capability | MASH Coverage | Use Case |
|----------------|---------------|----------|
| Asymmetric V2H discharge | SetCurrentSetpoints(phases, PRODUCTION) | Phase balancing |
| Per-phase production limits | SetCurrentLimits(phases, PRODUCTION) | Grid feed-in per phase |
| Bidirectional per-phase setpoints | SetCurrentSetpoints(phases, BIDIRECTIONAL) | Full V2H optimization |

---

### 4.4 Status Feature

**Purpose:** Per-endpoint operating state and health. Any endpoint can have this feature, including measurement-only endpoints that aren't controllable.

#### 4.4.1 Attributes

```cbor
Status Feature Attributes:
{
  1: operatingState,         // OperatingStateEnum
  2: stateDetail,            // uint32 (vendor-specific detail code, optional)
  3: faultCode,              // uint32 (fault/error code when state=FAULT, optional)
  4: faultMessage,           // string (human-readable fault description, optional)
}
```

#### 4.4.2 Enumerations

**OperatingStateEnum:**
```
UNKNOWN           = 0x00  // State not known
OFFLINE           = 0x01  // Not connected / not available
STANDBY           = 0x02  // Ready but not active
STARTING          = 0x03  // Powering up / initializing
RUNNING           = 0x04  // Actively operating
PAUSED            = 0x05  // Temporarily paused (can resume)
SHUTTING_DOWN     = 0x06  // Powering down
FAULT             = 0x07  // Error condition (check faultCode)
MAINTENANCE       = 0x08  // Under maintenance / firmware update
```

#### 4.4.3 Examples by Endpoint Type

**Inverter AC endpoint (normal operation):**
```cbor
{
  operatingState: RUNNING
}
```

**PV String endpoint (fault condition):**
```cbor
{
  operatingState: FAULT,
  faultCode: 1001,
  faultMessage: "String overcurrent protection"
}
```

**Battery endpoint (standby):**
```cbor
{
  operatingState: STANDBY,
  stateDetail: 0x0001           // Vendor: waiting for charge command
}
```

**EVSE endpoint (no vehicle connected):**
```cbor
{
  operatingState: STANDBY
}
```

**EVSE endpoint (actively charging):**
```cbor
{
  operatingState: RUNNING
}
```

**Smart meter (communication issue):**
```cbor
{
  operatingState: FAULT,
  faultCode: 2001,
  faultMessage: "Meter communication timeout"
}
```

**Device root (firmware updating):**
```cbor
{
  operatingState: MAINTENANCE,
  stateDetail: 0x0010           // Firmware update in progress
}
```

#### 4.4.4 State Transitions

```
                    ┌──────────────────────────────────────────┐
                    │                                          │
                    ▼                                          │
              ┌─────────┐                                      │
    power ───►│ OFFLINE │◄─── power off / disconnect          │
              └────┬────┘                                      │
                   │ connect                                   │
                   ▼                                           │
              ┌──────────┐                                     │
              │ STARTING │                                     │
              └────┬─────┘                                     │
                   │ ready                                     │
                   ▼                                           │
              ┌─────────┐    start     ┌─────────┐            │
              │ STANDBY │─────────────►│ RUNNING │            │
              └────┬────┘◄─────────────└────┬────┘            │
                   │         stop           │                  │
                   │                        │ pause            │
                   │                        ▼                  │
                   │                   ┌────────┐              │
                   │                   │ PAUSED │              │
                   │                   └────┬───┘              │
                   │                        │ resume           │
                   │                        ▼                  │
                   │              (back to RUNNING)            │
                   │                                           │
                   │ shutdown                                  │
                   ▼                                           │
              ┌──────────────┐                                 │
              │ SHUTTING_DOWN│─────────────────────────────────┘
              └──────────────┘

  Any state ──► FAULT (on error)
  Any state ──► MAINTENANCE (for updates)
  FAULT ──► STANDBY (after recovery)
  MAINTENANCE ──► STANDBY (after completion)
```

#### 4.4.5 Usage Notes

- **Per-endpoint**: Each endpoint has its own Status feature with independent state
- **Subscribe for updates**: Controllers should subscribe to status changes
- **Fault handling**: When `operatingState = FAULT`, check `faultCode` and `faultMessage` for details
- **Vendor extensions**: Use `stateDetail` for vendor-specific sub-states
- **Not for control**: Status is read-only; use EnergyControl.Pause/Resume for control

#### 4.4.6 Relationship to Other Features

| Feature | Responsibility |
|---------|---------------|
| **Status** | Operating state, faults, health (read-only) |
| **EnergyControl** | Control commands, limits, setpoints (write) |
| **Measurement** | Electrical readings, energy values (read-only) |

**Example: EVSE with all three features:**
```
Status:        operatingState = RUNNING
EnergyControl: effectiveConsumptionLimit = 11000000 mW
Measurement:   acActivePower = 10500000 mW, acEnergyConsumed = 25000000 mWh
```

---

### 4.5 ChargingSession Feature

**Purpose:** EV charging session state, energy demands, battery state, and vehicle identification. EV_CHARGER endpoints only.

**Scope:** This feature exists only on endpoints with `EndpointType = EV_CHARGER` and provides:
1. Charging session state and energy tracking
2. EV identification (multiple IDs: RFID, MAC, PCID, VIN, etc.)
3. EV battery state (from vehicle via ISO 15118)
4. EV energy demands for charging optimization
5. V2G discharge constraints (for bidirectional vehicles)

#### 4.5.1 Protocol Context

EV charging involves different communication protocols with varying capabilities:

| Protocol | Smart Comm | SoC Info | Energy Demands | V2G |
|----------|------------|----------|----------------|-----|
| IEC 61851-1 | No (PWM only) | No | No | No |
| ISO 15118-2 | Yes | Yes | Scheduled mode | No |
| ISO 15118-20 | Yes | Yes | Dynamic + Scheduled | Yes |

The ChargingSession feature accommodates all levels - fields are nullable when the EV cannot provide that information.

#### 4.5.2 Attributes

```cbor
ChargingSession Feature (Feature ID: 0x0007)
{
  // SESSION STATE
  1: state,                    // ChargingStateEnum
  2: sessionId,                // uint32: unique session identifier
  3: sessionStartTime,         // timestamp: when EV connected
  4: sessionEndTime,           // timestamp?: when session ended (null if ongoing)

  // SESSION ENERGY (cumulative for this session)
  10: sessionEnergyCharged,    // uint64 mWh: energy delivered to EV this session
  11: sessionEnergyDischarged, // uint64 mWh: energy returned from EV (V2G)

  // EV IDENTIFICATIONS (multiple possible)
  20: evIdentifications,       // EvIdentification[]: list of EV identifiers

  // EV BATTERY STATE (from vehicle via ISO 15118)
  30: evStateOfCharge,         // uint8?: current SoC % (null if unknown)
  31: evBatteryCapacity,       // uint64 mWh?: EV battery capacity

  // EV ENERGY DEMANDS (ISO 15118-2/20)
  40: evDemandMode,            // EvDemandModeEnum: what level of demand info available
  41: evMinEnergyRequest,      // int64 mWh?: energy to reach min SoC (negative = can discharge)
  42: evMaxEnergyRequest,      // int64 mWh?: energy to reach full (remaining capacity)
  43: evTargetEnergyRequest,   // int64 mWh?: energy to reach target SoC by departure
  44: evDepartureTime,         // timestamp?: when EV needs to be ready

  // V2G DISCHARGE CONSTRAINTS (ISO 15118-20 bidirectional)
  50: evMinDischargingRequest, // int64 mWh?: must be <0 for discharge to be allowed
  51: evMaxDischargingRequest, // int64 mWh?: must be >=0 for discharge to be allowed
  52: evDischargeBelowTargetPermitted, // bool?: user preference for V2G below target

  // ESTIMATED TIMES (from EV)
  60: estimatedTimeToMinSoC,   // uint32 s?: seconds to reach min SoC
  61: estimatedTimeToTargetSoC,// uint32 s?: seconds to reach target SoC
  62: estimatedTimeToFullSoC,  // uint32 s?: seconds to reach full charge
}
```

**Attribute Details:**

| ID | Name | Type | M/O | Description |
|----|------|------|-----|-------------|
| 1 | state | enum | M | Current charging state |
| 2 | sessionId | uint32 | M | Unique identifier for this session |
| 3 | sessionStartTime | timestamp | M | When EV was connected |
| 4 | sessionEndTime | timestamp | O | When session ended (null if active) |
| 10 | sessionEnergyCharged | uint64 | M | Energy delivered this session (mWh) |
| 11 | sessionEnergyDischarged | uint64 | M | Energy returned V2G this session (mWh) |
| 20 | evIdentifications | array | O | List of EV identifiers |
| 30 | evStateOfCharge | uint8 | O | EV battery SoC % (0-100) |
| 31 | evBatteryCapacity | uint64 | O | EV battery capacity (mWh) |
| 40 | evDemandMode | enum | M | Level of demand information available |
| 41 | evMinEnergyRequest | int64 | O | Energy to min SoC (mWh, negative = dischargeable) |
| 42 | evMaxEnergyRequest | int64 | O | Energy to full (mWh) |
| 43 | evTargetEnergyRequest | int64 | O | Energy to target SoC (mWh) |
| 44 | evDepartureTime | timestamp | O | When EV needs to leave |
| 50 | evMinDischargingRequest | int64 | O | V2G: min discharge energy (mWh) |
| 51 | evMaxDischargingRequest | int64 | O | V2G: max discharge energy (mWh) |
| 52 | evDischargeBelowTargetPermitted | bool | O | V2G: allow discharge below target |
| 60 | estimatedTimeToMinSoC | uint32 | O | Seconds to reach min SoC |
| 61 | estimatedTimeToTargetSoC | uint32 | O | Seconds to reach target SoC |
| 62 | estimatedTimeToFullSoC | uint32 | O | Seconds to reach full |

#### 4.5.3 Enumerations

**ChargingStateEnum:**
```
NOT_PLUGGED_IN        = 0x00  // No EV connected
PLUGGED_IN_NO_DEMAND  = 0x01  // EV connected, not requesting charge
PLUGGED_IN_DEMAND     = 0x02  // EV requesting charge, waiting to start
PLUGGED_IN_CHARGING   = 0x03  // Actively charging
PLUGGED_IN_DISCHARGING= 0x04  // V2G: actively discharging
SESSION_COMPLETE      = 0x05  // Charging finished, still plugged in
FAULT                 = 0x06  // Error state (check Status feature)
```

**EvDemandModeEnum:**
```
NONE                  = 0x00  // IEC 61851: no demand info available
SINGLE_DEMAND         = 0x01  // Basic: just energy amount requested
SCHEDULED             = 0x02  // ISO 15118: EV plans based on incentives
DYNAMIC               = 0x03  // ISO 15118-20: CEM controls directly
DYNAMIC_BIDIRECTIONAL = 0x04  // ISO 15118-20: dynamic with V2G
```

**EvIdTypeEnum:**
```
PCID                  = 0x00  // Provisioning Certificate ID (ISO 15118)
MAC_EUI48             = 0x01  // MAC address 6 bytes (AA:BB:CC:DD:EE:FF)
MAC_EUI64             = 0x02  // Extended MAC 8 bytes
RFID                  = 0x03  // RFID tag identifier
VIN                   = 0x04  // Vehicle Identification Number (17 chars)
CONTRACT_ID           = 0x05  // eMI3 Contract ID (EMAID)
EVCC_ID               = 0x06  // EVCC ID from ISO 15118
OTHER                 = 0xFF  // Vendor-specific
```

#### 4.5.4 Data Structures

**EvIdentification:**
```cbor
EvIdentification {
  1: type,    // EvIdTypeEnum: what kind of identifier
  2: value,   // string: the identifier value
}
```

#### 4.5.5 Energy Request Semantics

Energy requests follow ISO 15118-20 conventions - they are **deltas from current SoC**, not absolute values:

```
evMinEnergyRequest = (minSoC - currentSoC) * batteryCapacity
evTargetEnergyRequest = (targetSoC - currentSoC) * batteryCapacity
evMaxEnergyRequest = (100% - currentSoC) * batteryCapacity
```

**Sign convention:**
- **Positive** = energy needs to be charged
- **Negative** = energy can be discharged (below current SoC)
- **Zero** = SoC level reached

**Example - EV at 60% SoC, 80kWh battery:**
```
evStateOfCharge = 60
evBatteryCapacity = 80000000 mWh (80 kWh)
evMinEnergyRequest = -16000000 mWh  // Can discharge to 40% (min SoC)
evTargetEnergyRequest = 16000000 mWh  // Needs 16 kWh to reach 80% (target)
evMaxEnergyRequest = 32000000 mWh  // Needs 32 kWh to reach 100%
```

#### 4.5.6 V2G Discharge Rules

From ISO 15118-20 / EEBUS DBEVC, discharging is only permitted when:

1. `evMinDischargingRequest < 0` (there's energy available to discharge)
2. `evMaxDischargingRequest >= 0` (discharge cycle limit not exceeded)
3. If discharging would put `evTargetEnergyRequest > 0`, check `evDischargeBelowTargetPermitted`

```
Can discharge = (evMinDischargingRequest < 0)
              AND (evMaxDischargingRequest >= 0)
              AND (evTargetEnergyRequest <= 0 OR evDischargeBelowTargetPermitted)
```

#### 4.5.7 Examples

**IEC 61851 Basic Charger (no smart communication):**
```cbor
{
  1: 0x03,                    // state: PLUGGED_IN_CHARGING
  2: 12345,                   // sessionId
  3: 1706180400,              // sessionStartTime (Unix timestamp)
  10: 5500000,                // sessionEnergyCharged: 5.5 kWh
  11: 0,                      // sessionEnergyDischarged: 0
  40: 0x00,                   // evDemandMode: NONE
  // No EV info available - all other fields null/absent
}
```

**ISO 15118-2 Smart Charger:**
```cbor
{
  1: 0x03,                    // state: PLUGGED_IN_CHARGING
  2: 12346,                   // sessionId
  3: 1706180400,              // sessionStartTime
  10: 12000000,               // sessionEnergyCharged: 12 kWh
  11: 0,                      // sessionEnergyDischarged: 0
  20: [                       // evIdentifications
    { 1: 0x03, 2: "04E57CD2A1B3" },  // RFID
    { 1: 0x01, 2: "AA:BB:CC:DD:EE:FF" }  // MAC
  ],
  30: 65,                     // evStateOfCharge: 65%
  31: 75000000,               // evBatteryCapacity: 75 kWh
  40: 0x02,                   // evDemandMode: SCHEDULED
  41: -18750000,              // evMinEnergyRequest: -18.75 kWh (can discharge to 40%)
  42: 26250000,               // evMaxEnergyRequest: 26.25 kWh (to full)
  43: 11250000,               // evTargetEnergyRequest: 11.25 kWh (to 80% target)
  44: 1706223600,             // evDepartureTime: tomorrow 8am
}
```

**ISO 15118-20 V2G Bidirectional:**
```cbor
{
  1: 0x04,                    // state: PLUGGED_IN_DISCHARGING (V2G active)
  2: 12347,                   // sessionId
  3: 1706180400,              // sessionStartTime
  10: 8000000,                // sessionEnergyCharged: 8 kWh
  11: 3500000,                // sessionEnergyDischarged: 3.5 kWh (V2G)
  20: [                       // evIdentifications
    { 1: 0x00, 2: "PCID-VW-2024-ABC123" },  // PCID
    { 1: 0x04, 2: "WVWZZZ3CZWE123456" },    // VIN
    { 1: 0x03, 2: "04E57CD2A1B3" }          // RFID
  ],
  30: 72,                     // evStateOfCharge: 72%
  31: 82000000,               // evBatteryCapacity: 82 kWh
  40: 0x04,                   // evDemandMode: DYNAMIC_BIDIRECTIONAL
  41: -26240000,              // evMinEnergyRequest: -26.24 kWh (can go to 40%)
  42: 22960000,               // evMaxEnergyRequest: 22.96 kWh (to full)
  43: -8200000,               // evTargetEnergyRequest: -8.2 kWh (target 62%, below current)
  44: 1706223600,             // evDepartureTime
  50: -16400000,              // evMinDischargingRequest: -16.4 kWh available
  51: 8200000,                // evMaxDischargingRequest: 8.2 kWh (cycle limit ok)
  52: true,                   // evDischargeBelowTargetPermitted
  60: 0,                      // estimatedTimeToMinSoC: already above
  61: 0,                      // estimatedTimeToTargetSoC: already above target
  62: 5400,                   // estimatedTimeToFullSoC: 90 min
}
```

**Dual-Port EVSE (two endpoints):**
```
Endpoint 1 (Port 1): ChargingSession
  state: PLUGGED_IN_CHARGING
  sessionId: 1001
  evStateOfCharge: 45%

Endpoint 2 (Port 2): ChargingSession
  state: NOT_PLUGGED_IN
  sessionId: 0 (no active session)
```

#### 4.5.8 EEBUS Use Case Coverage

| EEBUS Use Case | Data Points | ChargingSession Mapping |
|----------------|-------------|------------------------|
| **EVCEM** | Charging power/energy | Measurement feature (not here) |
| **EVSOC** | Current SoC, Min/Target SoC, Battery capacity | evStateOfCharge, evMinEnergyRequest, evTargetEnergyRequest, evBatteryCapacity |
| **EVCS** | Session summary, costs | sessionEnergyCharged (costs = future Tariff feature) |
| **SMR** | Session-measurement relation | sessionId links to Measurement data |
| **EVCC/EVSECC** | EV identification, session ID | evIdentifications, sessionId |
| **CEVC** | Energy demands (Emin, Eopt, Emax), departure time | evMinEnergyRequest, evTargetEnergyRequest, evMaxEnergyRequest, evDepartureTime |
| **DBEVC** | Dynamic setpoints, V2G constraints | evDemandMode, evMin/MaxDischargingRequest, evDischargeBelowTargetPermitted |

**Not covered (future features):**
- CEVC incentive tables → Tariff/Incentive feature
- CEVC charging plan curves → Schedule/Forecast feature
- CEVC/DBEVC power schedules → EnergyControl schedule extension

#### 4.5.9 Relationship to Other Features

```
ChargingSession: "What's the EV telling us?"
  ├── Session state and energy tracking
  ├── EV battery state (SoC, capacity)
  ├── EV energy demands (for optimization)
  └── EV identification

Measurement: "What are the actual electrical values?"
  └── Power, voltage, current, cumulative energy

EnergyControl: "What can we tell the EVSE to do?"
  ├── SetLimit (current/power limits)
  └── SetSetpoint (target power for V2H/V2G)

Status: "Is the EVSE working?"
  └── Operating state, faults
```

---

### 4.6 DeviceInfo Feature

**Purpose:** Device identity, manufacturer information, and complete device structure. Endpoint 0 only.

**Scope:** This feature exists **only on endpoint 0** (DEVICE_ROOT) and provides:
1. Device identification (unique ID, vendor, product, serial number)
2. Version information (software, hardware)
3. Complete device structure (all endpoints with their types and features)

#### 4.6.1 Device ID Format

Devices are identified by a globally unique string in one of two formats:

| Format | Pattern | Example | Use Case |
|--------|---------|---------|----------|
| **IANA PEN** | `i:<PEN>:<unique>` | `i:46925:ABC123-XYZ` | Vendors with registered IANA Private Enterprise Number |
| **Vendor Name** | `n:<vendor>:<unique>` | `n:acme:ABC123-XYZ` | Vendors without IANA PEN |

**Rules:**
- `<PEN>`: IANA Private Enterprise Number (decimal)
- `<vendor>`: Lowercase alphanumeric, no spaces, max 32 chars
- `<unique>`: Vendor-assigned unique ID, alphanumeric and `-_`, max 64 chars
- Total deviceId max length: 100 characters
- Vendors are responsible for ensuring uniqueness within their namespace

#### 4.6.2 Attributes

```cbor
DeviceInfo Feature (Feature ID: 0x0006)
{
  // DEVICE IDENTIFICATION
  1: deviceId,           // string: unique ID (format: i:<PEN>:<id> or n:<vendor>:<id>)
  2: vendorName,         // string: human-readable vendor name
  3: productName,        // string: human-readable product name
  4: productId,          // string: vendor-defined product/model identifier
  5: serialNumber,       // string: serial number (as printed on device)
  6: brandName,          // string?: brand name (optional, if different from vendor)

  // VERSION INFORMATION
  10: softwareVersion,   // string: software/firmware version
  11: hardwareVersion,   // string: hardware revision

  // DEVICE STRUCTURE (complete endpoint list)
  20: endpoints,         // EndpointDescriptor[]: all endpoints with features
}
```

**Attribute Details:**

| ID | Name | Type | M/O | Description |
|----|------|------|-----|-------------|
| 1 | deviceId | string | M | Globally unique device identifier |
| 2 | vendorName | string | M | Human-readable vendor/manufacturer name |
| 3 | productName | string | M | Human-readable product/model name |
| 4 | productId | string | M | Vendor's product identifier (for programmatic matching) |
| 5 | serialNumber | string | M | Device serial number |
| 6 | brandName | string | O | Brand name if different from vendor |
| 10 | softwareVersion | string | M | Current software/firmware version |
| 11 | hardwareVersion | string | M | Hardware revision |
| 20 | endpoints | array | M | Complete list of endpoints and their features |

#### 4.6.3 Data Structures

**EndpointDescriptor:**
```cbor
EndpointDescriptor {
  1: id,        // uint8: endpoint ID (0-255)
  2: type,      // EndpointType: what this endpoint represents
  3: label,     // string?: manufacturer label (e.g., "Roof South", "Port 1")
  4: features,  // uint16[]: list of feature IDs supported on this endpoint
}
```

**Example - Hybrid Inverter:**
```cbor
{
  1: "i:12345:INV-2024-001",           // deviceId
  2: "SolarTech GmbH",                 // vendorName
  3: "HybridMax 10000",                // productName
  4: "HM10K-EU",                       // productId
  5: "SN2024ABC123",                   // serialNumber
  10: "2.1.0",                         // softwareVersion
  11: "1.0",                           // hardwareVersion
  20: [                                // endpoints
    {
      1: 0,                            // id: endpoint 0
      2: 0x00,                         // type: DEVICE_ROOT
      4: [0x0006]                      // features: DeviceInfo
    },
    {
      1: 1,                            // id: endpoint 1
      2: 0x02,                         // type: INVERTER
      3: "Grid Connection",            // label
      4: [0x0001, 0x0002, 0x0003, 0x0005]  // features: Electrical, Measurement, EnergyControl, Status
    },
    {
      1: 2,                            // id: endpoint 2
      2: 0x03,                         // type: PV_STRING
      3: "Roof South",                 // label
      4: [0x0002, 0x0005]              // features: Measurement, Status
    },
    {
      1: 3,                            // id: endpoint 3
      2: 0x03,                         // type: PV_STRING
      3: "Roof West",                  // label
      4: [0x0002, 0x0005]              // features: Measurement, Status
    },
    {
      1: 4,                            // id: endpoint 4
      2: 0x04,                         // type: BATTERY
      3: "LG Chem RESU",               // label
      4: [0x0002, 0x0003, 0x0005]      // features: Measurement, EnergyControl, Status
    }
  ]
}
```

**Example - Simple EVSE:**
```cbor
{
  1: "n:wallbox:WB-2024-XYZ",          // deviceId (no IANA PEN)
  2: "WallBox Inc",                    // vendorName
  3: "ChargePoint 22",                 // productName
  4: "CP22-EU",                        // productId
  5: "WB123456",                       // serialNumber
  10: "1.5.2",                         // softwareVersion
  11: "2.0",                           // hardwareVersion
  20: [                                // endpoints
    {
      1: 0,                            // id: endpoint 0
      2: 0x00,                         // type: DEVICE_ROOT
      4: [0x0006]                      // features: DeviceInfo
    },
    {
      1: 1,                            // id: endpoint 1
      2: 0x05,                         // type: EV_CHARGER
      4: [0x0001, 0x0002, 0x0003, 0x0005]  // Electrical, Measurement, EnergyControl, Status
    }
  ]
}
```

#### 4.6.4 Feature ID Registry

| Feature ID | Name | Description |
|------------|------|-------------|
| 0x0001 | Electrical | Static electrical configuration |
| 0x0002 | Measurement | Power, energy, voltage, current telemetry |
| 0x0003 | EnergyControl | Limits, setpoints, control commands |
| 0x0004 | (reserved) | |
| 0x0005 | Status | Operating state, faults |
| 0x0006 | DeviceInfo | Device identity and structure |
| 0x0007 | ChargingSession | EV charging session data (future) |
| 0x0100+ | (vendor) | Vendor-specific features |

#### 4.6.5 Usage Notes

**Single Read for Complete Device Understanding:**
- Reading DeviceInfo provides everything needed to understand the device
- No additional discovery requests required
- Controllers can immediately know which endpoints exist and what features they support

**Endpoint 0 Convention:**
- Endpoint 0 always exists and always has DeviceInfo
- Endpoint 0 type is always DEVICE_ROOT
- Other features on endpoint 0 are device-wide (not endpoint-specific)

**No User-Configurable Fields:**
- DeviceInfo is read-only
- User labels/locations are managed by the EMS, not stored on the device
- Simplifies device implementation (no writable storage needed for labels)

#### 4.6.6 EEBUS NID Use Case Coverage

| EEBUS NID Data Point | MASH DeviceInfo | Notes |
|---------------------|-----------------|-------|
| Device Name | productName | Human-readable name |
| Device Code | productId | Model identifier |
| Serial Number | serialNumber | Direct mapping |
| Software Revision | softwareVersion | Direct mapping |
| Hardware Revision | hardwareVersion | Direct mapping |
| Vendor Name | vendorName | Direct mapping |
| Vendor Code | deviceId (PEN part) | Embedded in deviceId format |
| Brand Name | brandName | Optional field |
| Power Source | - | Not included (derivable from Electrical feature) |
| Node Identification | deviceId | Globally unique ID |
| Label | - | EMS-managed, not on device |
| Description | - | EMS-managed, not on device |

---

### 4.7 Signals Feature

**Purpose:** Time-slotted INPUT data from controllers to devices: prices, power constraints, targets, and forecasts. Data flows IN to inform device behavior.

**Scope:** This feature can exist on:
- Endpoint 0 (DEVICE_ROOT): Device-wide signals (e.g., site tariff, household constraints)
- Specific endpoints: Endpoint-specific signals (e.g., EV charging constraints, PV production forecast)

#### 4.7.1 Design Principles

1. **Input direction** - Signals flow FROM controllers TO devices (never from device)
2. **Multiple sources** - Different controllers can provide signals simultaneously
3. **Use what you need** - All slot fields optional; presence indicates what's relevant
4. **Tariff separation** - Price structure defined in Tariff feature; Signals carries time-varying values
5. **Plan separation** - Device responses go in Plan feature (separate output channel)

#### 4.7.2 Attributes

```cbor
Signals Feature (Feature ID: 0x0008)
{
  // ACTIVE SIGNALS (from controllers/services)
  1: signals,                     // Signal[]: all active signals received

  // CURRENT STATE (convenience - what's active right now)
  10: currentPrice,               // int64?: current total consumption price
  11: currentProductionPrice,     // int64?: current production/feed-in price
  12: currentMaxConsumption,      // int64 mW?: effective consumption limit
  13: currentMaxProduction,       // int64 mW?: effective production limit
  14: currentCo2Intensity,        // uint16 g/kWh?: current grid CO2
  15: currentRenewablePercent,    // uint8 %?: current renewable content

  // CAPABILITIES
  20: maxSlots,                   // uint16: max slots per signal (min 24)
  21: maxSignals,                 // uint8: max concurrent signals (min 2)
  22: supportedSignalTypes,       // SignalTypeEnum[]: what types understood
}
```

#### 4.7.3 Data Structures

**Signal:**
```cbor
Signal {
  // METADATA
  1: signalId,                    // uint32: unique identifier
  2: source,                      // SignalSourceEnum: who provided this
  3: priority,                    // uint8: for conflict resolution (higher wins)
  4: validFrom,                   // timestamp: when signal becomes active
  5: validUntil,                  // timestamp?: when signal expires
  6: signalType,                  // SignalTypeEnum: purpose of this signal

  // TARIFF REFERENCE (for price signals)
  10: tariffId,                   // uint32?: which Tariff structure applies

  // TIME SLOTS
  20: slots,                      // SignalSlot[]: the signal data
}
```

**SignalSlot:**
```cbor
SignalSlot {
  // TIME
  1: duration,                    // uint32 s: slot duration

  // PRICE DATA (for PRICE signals)
  10: componentPrices,            // ComponentPrice[]?: prices by component
  11: totalPrice,                 // int64?: total price if no breakdown
  12: productionPrice,            // int64?: feed-in/production price
  13: tierMultiplier,             // int16?: power tier multiplier (100 = 1.0x)

  // ENVIRONMENTAL SIGNALS
  15: co2Intensity,               // uint16 g/kWh?: grid carbon intensity
  16: renewablePercent,           // uint8 %?: renewable energy content

  // POWER LIMITS (for CONSTRAINT signals)
  20: minConsumption,             // int64 mW?: must consume at least
  21: maxConsumption,             // int64 mW?: cannot consume more than
  22: minProduction,              // int64 mW?: must produce at least
  23: maxProduction,              // int64 mW?: cannot produce more than

  // POWER TARGETS (for TARGET signals)
  30: targetConsumption,          // int64 mW?: suggested consumption
  31: targetProduction,           // int64 mW?: suggested production

  // FORECASTS (for FORECAST signals)
  35: forecastConsumption,        // int64 mW?: expected consumption
  36: forecastProduction,         // int64 mW?: expected production
  37: forecastConfidence,         // uint8 %?: prediction confidence (0-100)
}
```

**ComponentPrice:**
```cbor
ComponentPrice {
  1: componentId,                 // uint8: references TariffComponent
  2: price,                       // int64: price for this slot (scaled per Tariff)
}
```

#### 4.7.4 Enumerations

**SignalTypeEnum:**
```
PRICE             = 0x00  // Price/tariff schedule (ToUT)
CONSTRAINT        = 0x01  // Power limits (POEN)
TARGET            = 0x02  // Power setpoints (optimization guidance)
FORECAST          = 0x03  // Power prediction (solar, load, CO2)
COMBINED          = 0x04  // Mix of price + constraints (CEVC input)
```

**SignalSourceEnum:**
```
GRID_OPERATOR     = 0x00  // DSO, TSO - highest authority for limits
ENERGY_SUPPLIER   = 0x01  // Utility, retailer - price source
AGGREGATOR        = 0x02  // VPP, flexibility provider
HOME_EMS          = 0x03  // Local energy manager
USER              = 0x04  // Manual user input
FORECAST_SERVICE  = 0x05  // Weather service, prediction provider
SPOT_MARKET       = 0x06  // Direct spot price feed
```

#### 4.7.5 Commands

**SetSignal** - Provide a signal to the device
```cbor
Request:
{
  1: signal,                      // Signal: the signal to set
  2: replaceExisting,             // bool?: replace signal with same source/type
}

Response:
{
  1: success,                     // bool
  2: signalId,                    // uint32: assigned/confirmed ID
}
```

**ClearSignal** - Remove a signal
```cbor
Request:
{
  1: signalId,                    // uint32?: specific signal (null = all from zone)
}

Response:
{
  1: success,                     // bool
}
```

#### 4.7.6 Signal Resolution

**For limits (CONSTRAINT signals):**
- Most restrictive wins across all signals
- `effectiveMaxConsumption = min(all maxConsumption values)`
- `effectiveMinConsumption = max(all minConsumption values)`
- Limits from higher priority sources take precedence if conflict

**For prices (PRICE signals):**
- Highest priority source wins
- If same priority, most recent signal wins
- Device may aggregate prices from Tariff components

**For forecasts (FORECAST signals):**
- Multiple forecasts can coexist (from different sources)
- Consumer (EMS/device) decides which to trust based on source/confidence

#### 4.7.7 Slot Field Usage by Signal Type

| Field | PRICE | CONSTRAINT | TARGET | FORECAST | COMBINED |
|-------|-------|------------|--------|----------|----------|
| duration | Yes | Yes | Yes | Yes | Yes |
| componentPrices | Yes | - | - | - | Yes |
| totalPrice | Yes | - | - | - | Yes |
| productionPrice | Yes | - | - | - | Yes |
| tierMultiplier | Yes | - | - | - | Yes |
| co2Intensity | Opt | - | - | Yes | Opt |
| renewablePercent | Opt | - | - | Yes | Opt |
| minConsumption | - | Yes | - | - | Opt |
| maxConsumption | - | Yes | - | - | Yes |
| minProduction | - | Yes | - | - | Opt |
| maxProduction | - | Yes | - | - | Yes |
| targetConsumption | - | - | Yes | - | Opt |
| targetProduction | - | - | Yes | - | Opt |
| forecastConsumption | - | - | - | Yes | Opt |
| forecastProduction | - | - | - | Yes | Opt |
| forecastConfidence | - | - | - | Yes | - |

#### 4.7.8 Examples

**ToUT - Time-of-Use Price Signal:**
```cbor
{
  1: 1001,                        // signalId
  2: 0x01,                        // source: ENERGY_SUPPLIER
  3: 100,                         // priority
  4: 1706140800,                  // validFrom: midnight
  6: 0x00,                        // signalType: PRICE
  10: 1,                          // tariffId: references Tariff
  20: [
    { 1: 7200, 10: [{ 1: 1, 2: 2800 }] },   // 00-02: 0.28€ energy
    { 1: 7200, 10: [{ 1: 1, 2: 1500 }] },   // 02-04: 0.15€ (off-peak)
    { 1: 7200, 10: [{ 1: 1, 2: 1500 }] },   // 04-06: 0.15€
    { 1: 7200, 10: [{ 1: 1, 2: 2500 }] },   // 06-08: 0.25€
    { 1: 7200, 10: [{ 1: 1, 2: 3200 }] },   // 08-10: 0.32€
    // ... etc
  ]
}
```

**POEN - Grid Power Envelope:**
```cbor
{
  1: 2001,                        // signalId
  2: 0x00,                        // source: GRID_OPERATOR
  3: 200,                         // priority (high - must respect)
  4: 1706140800,                  // validFrom
  5: 1706227200,                  // validUntil: +24h
  6: 0x01,                        // signalType: CONSTRAINT
  20: [
    { 1: 3600, 21: 15000000 },    // 00-01: max 15kW consume
    { 1: 3600, 21: 12000000 },    // 01-02: max 12kW
    { 1: 3600, 21: 10000000, 23: 5000000 }, // 02-03: max 10kW in, 5kW out
    // ... etc
  ]
}
```

**Solar Production Forecast:**
```cbor
{
  1: 3001,                        // signalId
  2: 0x05,                        // source: FORECAST_SERVICE
  3: 50,                          // priority
  4: 1706140800,                  // validFrom
  6: 0x03,                        // signalType: FORECAST
  20: [
    { 1: 3600, 36: 0, 37: 95 },             // 00-01: 0kW, 95% confident
    { 1: 3600, 36: 0, 37: 95 },             // 01-02: 0kW
    { 1: 3600, 36: 0, 37: 95 },             // 02-03: 0kW
    { 1: 3600, 36: 0, 37: 95 },             // 03-04: 0kW
    { 1: 3600, 36: 0, 37: 95 },             // 04-05: 0kW
    { 1: 3600, 36: 0, 37: 95 },             // 05-06: 0kW
    { 1: 3600, 36: 500000, 37: 80 },        // 06-07: 0.5kW
    { 1: 3600, 36: 2000000, 37: 75 },       // 07-08: 2kW
    { 1: 3600, 36: 4500000, 37: 70 },       // 08-09: 4.5kW
    { 1: 3600, 36: 6000000, 37: 65 },       // 09-10: 6kW
    { 1: 3600, 36: 7000000, 37: 60 },       // 10-11: 7kW (peak)
    { 1: 3600, 36: 7500000, 37: 55 },       // 11-12: 7.5kW
    { 1: 3600, 36: 7000000, 37: 60 },       // 12-13: 7kW
    // ... afternoon decline
  ]
}
```

**CEVC Input - Prices + Limits + PV Forecast (Combined Signal):**
```cbor
{
  1: 4001,                        // signalId
  2: 0x03,                        // source: HOME_EMS
  3: 80,                          // priority
  4: 1706194800,                  // validFrom: 18:00
  5: 1706238000,                  // validUntil: 06:00 next day
  6: 0x04,                        // signalType: COMBINED
  10: 1,                          // tariffId
  20: [
    { 1: 7200, 10: [{ 1: 1, 2: 3500 }], 21: 11000000, 36: 0 },
    // 18-20: 0.35€, max 11kW, no PV
    { 1: 7200, 10: [{ 1: 1, 2: 1500 }], 21: 11000000, 36: 0 },
    // 20-22: 0.15€ (cheap!), max 11kW, no PV
    { 1: 7200, 10: [{ 1: 1, 2: 1200 }], 21: 11000000, 36: 0 },
    // 22-00: 0.12€ (cheapest), max 11kW, no PV
    { 1: 7200, 10: [{ 1: 1, 2: 1500 }], 21: 11000000, 36: 0 },
    // 00-02: 0.15€, max 11kW, no PV
    { 1: 7200, 10: [{ 1: 1, 2: 2000 }], 21: 11000000, 36: 0 },
    // 02-04: 0.20€, max 11kW, no PV
    { 1: 7200, 10: [{ 1: 1, 2: 2500 }], 21: 11000000, 36: 3000000 },
    // 04-06: 0.25€, max 11kW, 3kW PV expected
  ]
}
```

#### 4.7.9 Relationship to Other Features

| Feature | Relationship |
|---------|--------------|
| **Plan** | Device responds to Signals by providing Plan (output direction) |
| **Tariff** | Signals references Tariff for price structure; Signals carries time-varying values |
| **EnergyControl** | Signals provides context; EnergyControl executes (SetLimit/SetSetpoint based on Signals) |
| **ChargingSession** | ChargingSession reports EV state; Signals carries CEVC optimization input |
| **Measurement** | Measurement reports actual values; Signals carries expected/planned values |

---

### 4.8 Tariff Feature

**Purpose:** Define price structure, components, and power-based tiers. Separates stable tariff structure from time-varying prices (which go in Signals).

**Scope:** This feature can exist on:
- Endpoint 0 (DEVICE_ROOT): Site-wide tariffs
- Specific endpoints: Endpoint-specific tariffs (rare)

#### 4.8.1 Design Principles

1. **Structure vs Values** - Tariff defines WHAT components exist; Schedule defines WHEN prices apply
2. **Component breakdown** - Separates energy, grid fees, taxes, levies for transparency
3. **Power tiers** - Supports demand-based pricing (different rates above power thresholds)
4. **Multiple tariffs** - Consumption and production (feed-in) can have different structures

#### 4.8.2 Attributes

```cbor
Tariff Feature (Feature ID: 0x0009)
{
  // TARIFF DEFINITIONS
  1: tariffs,                     // TariffDefinition[]: all defined tariffs

  // ACTIVE STATE
  10: activeTariffIds,            // uint32[]: which tariffs currently apply
}
```

#### 4.8.3 Data Structures

**TariffDefinition:**
```cbor
TariffDefinition {
  // IDENTITY
  1: tariffId,                    // uint32: unique identifier
  2: name,                        // string?: human-readable name
  3: source,                      // TariffSourceEnum: who provided this
  4: scope,                       // TariffScopeEnum: consumption, production, or both

  // CURRENCY & UNITS
  10: currency,                   // string: ISO 4217 (EUR, USD, SEK)
  11: priceScale,                 // int8: decimal scale (-4 = 0.0001)
  12: priceUnit,                  // PriceUnitEnum: PER_KWH, PER_MWH, PER_HOUR

  // COMPONENTS (the price elements)
  20: components,                 // TariffComponent[]: price breakdown

  // POWER TIERS (optional - for demand-based pricing)
  30: powerTiers,                 // PowerTierDefinition[]?
}
```

**TariffComponent:**
```cbor
TariffComponent {
  1: componentId,                 // uint8: unique within tariff
  2: type,                        // ComponentTypeEnum: what this component represents
  3: name,                        // string?: human-readable name
  4: basePrice,                   // int64?: default price (scaled)
  5: isVariable,                  // bool: does price vary by time (via Schedule)?
}
```

**PowerTierDefinition:**
```cbor
PowerTierDefinition {
  1: tierId,                      // uint8: unique within tariff
  2: thresholdPower,              // int64 mW: tier applies above this power
  3: priceMultiplier,             // int16?: percentage multiplier (100 = 1.0x)
  4: priceOffset,                 // int64?: additive offset (scaled)
  5: label,                       // string?: "Base rate", "Peak rate"
}
```

#### 4.8.4 Enumerations

**TariffSourceEnum:**
```
ENERGY_SUPPLIER   = 0x00  // Utility/retailer
GRID_OPERATOR     = 0x01  // DSO/TSO network charges
GOVERNMENT        = 0x02  // Tax authority
AGGREGATOR        = 0x03  // Flexibility provider
USER              = 0x04  // Manual configuration
```

**TariffScopeEnum:**
```
CONSUMPTION       = 0x00  // Applies when consuming/charging
PRODUCTION        = 0x01  // Applies when producing/discharging (feed-in)
BIDIRECTIONAL     = 0x02  // Applies to both directions
```

**ComponentTypeEnum:**
```
ENERGY            = 0x00  // Commodity/wholesale energy cost
GRID_FEE          = 0x01  // Network/transmission/distribution charges
TAX               = 0x02  // Government taxes (VAT, electricity tax)
LEVY              = 0x03  // Surcharges (EEG, renewable levies)
CO2               = 0x04  // Carbon price component
CREDIT            = 0x05  // Discounts, rebates (typically negative)
OTHER             = 0x06  // Other charges
```

**PriceUnitEnum:**
```
PER_KWH           = 0x00  // Price per kilowatt-hour
PER_MWH           = 0x01  // Price per megawatt-hour
PER_HOUR          = 0x02  // Price per hour (demand charges)
```

#### 4.8.5 Commands

**SetTariff** - Define or update a tariff
```cbor
Request:
{
  1: tariff,                      // TariffDefinition: the tariff to set
  2: activate,                    // bool?: also add to activeTariffIds
}

Response:
{
  1: success,                     // bool
  2: tariffId,                    // uint32: assigned/confirmed ID
}
```

**ClearTariff** - Remove a tariff definition
```cbor
Request:
{
  1: tariffId,                    // uint32: tariff to remove
}

Response:
{
  1: success,                     // bool
}
```

**SetActiveTariffs** - Change which tariffs are active
```cbor
Request:
{
  1: tariffIds,                   // uint32[]: tariffs to activate
}

Response:
{
  1: success,                     // bool
}
```

#### 4.8.6 Power Tier Semantics

Power tiers implement demand-based pricing where the rate depends on instantaneous power draw.

**Rules:**
- Tiers ordered by `thresholdPower` (ascending)
- First tier typically has `thresholdPower = 0`
- Price for a tier = `basePrice × priceMultiplier/100 + priceOffset`
- Current power determines which SINGLE tier applies (not cumulative)

**Example - German demand-based grid fee:**
```cbor
powerTiers: [
  { 1: 1, 2: 0,        3: 100, 5: "Standard" },      // 0-4.6kW: 1.0x
  { 1: 2, 2: 4600000,  3: 130, 5: "Elevated" },      // 4.6-10kW: 1.3x
  { 1: 3, 2: 10000000, 3: 160, 5: "Peak" },          // >10kW: 1.6x
]
```

Drawing 8kW → Tier 2 applies → all consumption priced at 1.3× base rate.

#### 4.8.7 Examples

**German Household Consumption Tariff:**
```cbor
{
  1: 1,                           // tariffId
  2: "Stadtwerke Berlin Flex",    // name
  3: 0x00,                        // source: ENERGY_SUPPLIER
  4: 0x00,                        // scope: CONSUMPTION
  10: "EUR",                      // currency
  11: -4,                         // priceScale: 0.0001 EUR
  12: 0x00,                       // priceUnit: PER_KWH
  20: [
    { 1: 1, 2: 0x00, 3: "Energy", 5: true },           // variable via Schedule
    { 1: 2, 2: 0x01, 3: "Grid fee", 4: 800, 5: false },    // 0.08€ fixed
    { 1: 3, 2: 0x03, 3: "EEG surcharge", 4: 350, 5: false }, // 0.035€
    { 1: 4, 2: 0x03, 3: "KWKG levy", 4: 50, 5: false },    // 0.005€
    { 1: 5, 2: 0x02, 3: "Electricity tax", 4: 205, 5: false }, // 0.0205€
    { 1: 6, 2: 0x02, 3: "VAT 19%", 5: true },              // calculated
  ],
  30: [
    { 1: 1, 2: 0,        3: 100 },     // 0-6kW: standard rate
    { 1: 2, 2: 6000000,  3: 125 },     // >6kW: 1.25× multiplier
  ]
}
```

**Feed-in Tariff (PV surplus):**
```cbor
{
  1: 2,                           // tariffId
  2: "Feed-in Compensation",      // name
  3: 0x01,                        // source: GRID_OPERATOR
  4: 0x01,                        // scope: PRODUCTION (feed-in)
  10: "EUR",
  11: -4,
  12: 0x00,
  20: [
    { 1: 1, 2: 0x00, 3: "Feed-in rate", 4: 820, 5: true }, // 0.082€/kWh base
  ]
  // No power tiers - flat rate for all production
}
```

**Nordic Spot-Based Tariff (simple):**
```cbor
{
  1: 3,
  2: "Nordpool Spot",
  3: 0x07,                        // source: (would map to SPOT_MARKET if in TariffSourceEnum)
  4: 0x02,                        // scope: BIDIRECTIONAL
  10: "SEK",
  11: -2,                         // öre (1/100 SEK)
  12: 0x00,
  20: [
    { 1: 1, 2: 0x00, 3: "Spot price", 5: true },          // 100% variable
    { 1: 2, 2: 0x01, 3: "Network fee", 4: 45, 5: false }, // 0.45 SEK fixed
    { 1: 3, 2: 0x02, 3: "Energy tax", 4: 36, 5: false },  // 0.36 SEK
    { 1: 4, 2: 0x02, 3: "VAT 25%", 5: true },             // calculated
  ]
}
```

#### 4.8.8 Price Calculation

Total price for a slot is calculated as:

```
For each active TariffComponent:
  If isVariable AND componentPrices has entry:
    componentPrice = componentPrices[componentId].price
  Else:
    componentPrice = basePrice

  If powerTiers defined AND current power > 0:
    Find applicable tier (highest threshold below current power)
    componentPrice = componentPrice × tier.priceMultiplier/100 + tier.priceOffset

Sum all component prices for total price.
```

#### 4.8.9 Relationship to Signals

| Aspect | Tariff | Signals |
|--------|--------|---------|
| **Content** | Structure, components, tiers | Time-varying values |
| **Changes** | Rarely (contract change) | Frequently (hourly/daily) |
| **Referenced by** | Signal.tariffId | - |
| **Provides** | Component IDs, base prices | componentPrices per slot |

**Typical flow:**
1. Energy supplier provisions Tariff (structure rarely changes)
2. Daily: supplier sends Signal with hourly componentPrices
3. Device calculates total price: Tariff structure + Signal values + power tier

#### 4.8.10 EEBUS/Matter Coverage

| Source Concept | MASH Mapping |
|----------------|--------------|
| EEBUS IncentiveTable.Tariff | TariffDefinition |
| EEBUS IncentiveTable.Tier | PowerTierDefinition |
| EEBUS IncentiveTable.Incentive | TariffComponent + Signal.componentPrices |
| EEBUS IncentiveTypeType | ComponentTypeEnum + co2Intensity/renewablePercent |
| Matter CommodityTariff | TariffDefinition + Signals |
| Matter CommodityPrice | Signals (PRICE type) |
| Matter TariffComponent | TariffComponent |
| Matter PowerThreshold | PowerTierDefinition |

---

### 4.9 Plan Feature

**Purpose:** Time-slotted OUTPUT data from devices: what the device intends to do. Data flows OUT to inform controllers/EMS of device behavior.

**Scope:** This feature can exist on:
- Endpoint 0 (DEVICE_ROOT): Device-wide plan (aggregated)
- Specific endpoints: Endpoint-specific plans (e.g., EV charging plan, battery discharge plan)

#### 4.9.1 Design Principles

1. **Output direction** - Plans flow FROM devices TO controllers (never to device)
2. **Response to Signals** - Plan is device's response to received Signals (prices, limits)
3. **Respect constraints** - Plan must respect active CONSTRAINT signals
4. **ISO 15118 alignment** - For EVSE, plan reflects EV's optimization result communicated via ISO 15118

#### 4.9.2 Attributes

```cbor
Plan Feature (Feature ID: 0x000A)
{
  // CURRENT PLAN
  1: currentPlan,                 // Plan?: device's current plan (if any)

  // CAPABILITIES
  10: maxSlots,                   // uint16: max slots in plan (min 24)
  11: supportsOnDemand,           // bool: supports RequestPlan command?
  12: autoPublish,                // bool: automatically publishes plan on Signal change?
}
```

#### 4.9.3 Data Structures

**Plan:**
```cbor
Plan {
  // METADATA
  1: planId,                      // uint32: unique identifier
  2: validFrom,                   // timestamp: when plan starts
  3: validUntil,                  // timestamp?: when plan ends
  4: signalIds,                   // uint32[]?: signals this plan responds to

  // TIME SLOTS
  10: slots,                      // PlanSlot[]: the planned behavior
}
```

**PlanSlot:**
```cbor
PlanSlot {
  // TIME
  1: duration,                    // uint32 s: slot duration

  // PLANNED POWER
  10: plannedConsumption,         // int64 mW?: what device will consume
  11: plannedProduction,          // int64 mW?: what device will produce
  12: plannedEnergy,              // int64 mWh?: energy this slot (if known)

  // STATE (for storage/EV)
  20: expectedSoC,                // uint8 %?: expected SoC at slot end
  21: expectedEnergy,             // int64 mWh?: expected stored energy at slot end
}
```

#### 4.9.4 Commands

**RequestPlan** - Ask device for its planned response to current Signals
```cbor
Request:
{
  1: startTime,                   // timestamp?: plan start (default: now)
  2: duration,                    // uint32 s?: plan duration (default: 24h)
}

Response:
{
  1: success,                     // bool
  2: plan,                        // Plan?: device's plan
  3: reason,                      // string?: why no plan (if !success)
}
```

**AcknowledgePlan** - Controller acknowledges receipt of plan
```cbor
Request:
{
  1: planId,                      // uint32: the plan being acknowledged
  2: accepted,                    // bool: whether plan is acceptable
  3: feedback,                    // string?: why not accepted (if !accepted)
}

Response:
{
  1: success,                     // bool
}
```

#### 4.9.5 Plan Events

The Plan feature publishes events when the plan changes:

**PlanUpdated** - Device has new/updated plan
```cbor
{
  1: planId,                      // uint32: new plan identifier
  2: reason,                      // PlanUpdateReasonEnum: why plan changed
}
```

**PlanUpdateReasonEnum:**
```
NEW_SIGNALS       = 0x00  // New Signals received, plan recalculated
STATE_CHANGE      = 0x01  // Device state changed (e.g., EV plugged in)
USER_REQUEST      = 0x02  // User requested plan update
CONSTRAINT_CHANGE = 0x03  // Constraint signal changed
MANUAL            = 0x04  // Manual trigger
```

#### 4.9.6 CEVC Flow Example

This shows how Signals and Plan work together for coordinated EV charging:

```
EMS                          EVSE                         EV (via ISO 15118)
 |                            |                            |
 |-- SetSignal (COMBINED) --->|                            |
 |   (prices, limits, PV)     |                            |
 |                            |-- SA_ScheduleTuple ------->|
 |                            |   (prices, limits)         |
 |                            |                            |
 |                            |<-- ChargingProfile --------|
 |                            |   (EV's optimized plan)    |
 |                            |                            |
 |<-- PlanUpdated event ------|                            |
 |                            |                            |
 |-- RequestPlan ------------>|                            |
 |                            |                            |
 |<-- Plan response ----------|                            |
 |   (slots with planned      |                            |
 |    consumption, SoC)       |                            |
```

**Key insight:** The EVSE acts as a translator:
- MASH Signals → ISO 15118 SA_ScheduleTuple (to EV)
- ISO 15118 ChargingProfile → MASH Plan (to EMS)

#### 4.9.7 Example Plan (EV Charging)

```cbor
{
  1: 5001,                        // planId
  2: 1706194800,                  // validFrom: 18:00
  3: 1706238000,                  // validUntil: 06:00 next day
  4: [4001],                      // signalIds: responding to signal 4001
  10: [
    { 1: 7200, 10: 0, 20: 30 },               // 18-20: no charge, stay 30%
    { 1: 7200, 10: 11000000, 20: 50 },        // 20-22: 11kW, reach 50%
    { 1: 7200, 10: 11000000, 20: 70 },        // 22-00: 11kW, reach 70%
    { 1: 7200, 10: 11000000, 20: 90 },        // 00-02: 11kW, reach 90%
    { 1: 7200, 10: 0, 20: 90 },               // 02-04: done, maintain
    { 1: 7200, 10: 0, 20: 90 },               // 04-06: done
  ]
}
```

#### 4.9.8 Relationship to Other Features

| Feature | Relationship |
|---------|--------------|
| **Signals** | Plan is device's response to received Signals |
| **Tariff** | Plan may reference Tariff for cost optimization context |
| **EnergyControl** | EnergyControl enforces limits; Plan shows intended behavior within limits |
| **ChargingSession** | ChargingSession shows EV state; Plan shows future charging intent |
| **Measurement** | Measurement shows actuals; Plan shows intended/expected values |

---

### 4.10 Feature ID Registry (Updated)

| ID | Name | Direction | Description |
|----|------|-----------|-------------|
| 0x0001 | Electrical | - | Static electrical configuration |
| 0x0002 | Measurement | OUT | Power, energy, voltage, current telemetry |
| 0x0003 | EnergyControl | IN | Limits, setpoints, control commands |
| 0x0004 | (reserved) | - | |
| 0x0005 | Status | OUT | Operating state, faults |
| 0x0006 | DeviceInfo | OUT | Device identity and structure |
| 0x0007 | ChargingSession | OUT | EV charging session data |
| 0x0008 | Signals | IN | Time-slotted prices, limits, forecasts (TO device) |
| 0x0009 | Tariff | IN | Price structure, components, power tiers |
| 0x000A | Plan | OUT | Device's intended behavior (FROM device) |
| 0x0100+ | (vendor) | - | Vendor-specific features |

---

## 5. Use Case Coverage

### 5.1 Initial Target Use Cases

- [x] Load Control / Power Limitation - **EnergyControl feature**
- [x] Measurement Data Exchange - **Measurement feature**
- [x] Device Status and Diagnostics - **Status feature**
- [x] Device Identification - **DeviceInfo feature**
- [x] EV Charging Optimization (CEVC) - **ChargingSession + Signals + Plan features**
- [x] Grid Incentives / Tariffs (ToUT) - **Signals + Tariff features**
- [x] Power Envelopes (POEN) - **Signals feature (CONSTRAINT type)**
- [x] Solar/Load Forecasts - **Signals feature (FORECAST type)**
- [x] Device Planning / Response - **Plan feature**

### 5.2 Future Use Cases

- [ ] HVAC Control
- [ ] Battery Management (beyond basic EnergyControl)
- [ ] Recurring Schedule Patterns (weekly, seasonal)

---

## 6. Open Questions

> These will be resolved through iterative discussion

1. ~~**Transport**: WebSocket over TLS vs simpler alternatives (CoAP)?~~ → **Resolved: TCP/TLS (DEC-011)**
2. ~~**Serialization**: JSON vs CBOR vs Protobuf?~~ → **Resolved: CBOR (DEC-006)**
3. ~~**Device Model Depth**: How many hierarchy levels?~~ → **Resolved: 3-level (DEC-007)**
4. **Feature Extensibility**: Fixed spec or vendor extensions?
5. ~~**Pairing Flow**: How simple can we make it?~~ → **Resolved: SPAKE2+ (DEC-014)**
6. ~~**Multi-controller**: Support or explicitly single-controller?~~ → **Resolved: Multi-zone (DEC-016)**
7. ~~**Scheduled Limits**: Should EnergyControl support time-slotted limits (like POEN)?~~ → **Resolved: Signals + Plan features (DEC-033)**

---

## 7. References

### 7.1 EEBUS Analysis Documents

- [SHIP 1.0.1 Analysis](../../enbility/ship-go/analysis-docs/detailed-analysis/SHIP_1.0.1_ANALYSIS.md)
- [SPINE Promise vs Reality](../../enbility/spine-go/analysis-docs/UNDERSTANDING_SPINE_PROMISE_VS_REALITY.md)
- [SHIP Executive Summary](../../enbility/ship-go/analysis-docs/EXECUTIVE_SUMMARY.md)
- [SPINE Executive Summary](../../enbility/spine-go/analysis-docs/EXECUTIVE_SUMMARY.md)

### 7.2 Matter Protocol Resources

- [Matter Documentation](https://project-chip.github.io/connectedhomeip-doc/index.html)
- [Matter Data Model (Silicon Labs)](https://docs.silabs.com/matter/latest/matter-fundamentals-data-model/)
- [Matter Primer (Google)](https://developers.home.google.com/matter/primer/device-data-model)

---

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2025-01-24 | 0.1 | Initial structure and problem statement |
| 2025-01-24 | 0.2 | Added core decisions: CBOR, 3-level hierarchy, 4 ops, TCP/TLS, multi-controller model |
| 2025-01-24 | 0.3 | Added complete security model: SPAKE2+, operational certs, certificate lifecycle |
| 2025-01-24 | 0.4 | Added multi-zone model, zone types, priority resolution, delegated commissioning |
| 2025-01-24 | 0.5 | Added zone roles (owner/admin/member), app-EMS authorization, CSR forwarding |
| 2025-01-24 | 0.6 | Added IPv6-only network layer (no IPv4 support) |
| 2025-01-24 | 0.7 | Renamed "fabric" to "zone" (DEC-023) |
| 2025-01-24 | 0.8 | Renamed "cluster" to "feature", "LoadControl" to "Limit" (DEC-025) |
| 2025-01-25 | 0.9 | Added EnergyControl feature specification (DEC-026): capability-first design, CBOR structures, commands, state machine, EEBUS mapping |
| 2025-01-25 | 0.10 | Feature separation (DEC-027): Split into Electrical (static config), Measurement (telemetry), EnergyControl (control). Added per-phase current limits, bidirectional support, phase mapping |
| 2025-01-25 | 0.11 | Setpoints and V2H support (DEC-028): Added SetSetpoint, SetCurrentSetpoints commands. AsymmetricSupportEnum for direction-aware asymmetric capability. Extended EEBUS coverage with V2H phase balancing |
| 2025-01-25 | 0.12 | Measurement feature and EndpointType (DEC-029): Comprehensive AC/DC measurement support. EndpointType enum for device topology. Hybrid inverter multi-endpoint example. Covers MPC, MGCP, EVCEM, MOI, MOB, MPS use cases |
| 2025-01-25 | 0.13 | Status feature (DEC-030): Per-endpoint operating state for all devices. Separates state observation from control (EnergyControl). OperatingStateEnum with fault reporting |
| 2025-01-25 | 0.14 | DeviceInfo feature (DEC-031): Device identification with IANA PEN-based deviceId format. Complete device structure in single read. Feature ID registry. Covers EEBUS NID use case |
| 2025-01-25 | 0.15 | ChargingSession feature (DEC-032): EV session state, energy demands, V2G constraints. Multiple EV identifications. Covers EVSOC, EVCS, SMR, CEVC, DBEVC use cases. Protocol-aware (IEC 61851, ISO 15118-2/20) |
| 2025-01-25 | 0.16 | Schedule + Tariff features (DEC-033): Unified time-slotted schedules for ToUT, POEN, CEVC, forecasts. Tariff feature for price structure with components and power tiers. Covers Matter CommodityPrice/CommodityTariff and EEBUS IncentiveTable |
