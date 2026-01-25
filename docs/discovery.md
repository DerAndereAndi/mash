# MASH Discovery

> mDNS/DNS-SD discovery and capability introspection

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Protocol Overview](protocol-overview.md) | Vision, architecture, device model |
| [Security](security.md) | Commissioning, QR codes |
| [Features](features/README.md) | Feature definitions |

---

## 1. Discovery Overview

MASH uses two discovery mechanisms:

| Phase | Mechanism | Purpose |
|-------|-----------|---------|
| Pre-commissioning | mDNS + QR code | Find and pair new devices |
| Post-commissioning | Capability discovery | Learn device features |

---

## 2. mDNS/DNS-SD Discovery

### 2.1 Service Type

```
_mash._tcp.local
```

### 2.2 Pre-Commissioning (Device Advertising)

When a device is not commissioned (or in commissioning mode):

**Service Instance Name:**
```
MASH-<discriminator>._mash._tcp.local
```

**TXT Records:**

| Key | Description | Example |
|-----|-------------|---------|
| `D` | Discriminator (12-bit) | `1234` |
| `VP` | Vendor:Product ID | `1234:5678` |
| `CM` | Commissioning mode | `1` (open) |
| `DT` | Device type | `EVSE` |

**Example:**
```
MASH-1234._mash._tcp.local
  SRV 0 0 8443 evse-001.local
  TXT D=1234 VP=1234:5678 CM=1 DT=EVSE
```

### 2.3 Post-Commissioning (Operational)

After commissioning, device updates mDNS:

**Service Instance Name:**
```
<device-id>._mash._tcp.local
```

**TXT Records:**

| Key | Description | Example |
|-----|-------------|---------|
| `DI` | Device ID | `PEN12345.EVSE001` |
| `VP` | Vendor:Product ID | `1234:5678` |
| `FW` | Firmware version | `1.2.3` |
| `EP` | Endpoint count | `2` |
| `FM` | Feature map (hex) | `0x001B` |

**Example:**
```
PEN12345-EVSE001._mash._tcp.local
  SRV 0 0 8443 evse-001.local
  TXT DI=PEN12345.EVSE001 VP=1234:5678 FW=1.2.3 EP=2 FM=0x001B
```

---

## 3. QR Code Format

### 3.1 Content Structure

```
MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
```

| Field | Bits | Description |
|-------|------|-------------|
| version | 8 | Protocol version (1) |
| discriminator | 12 | Device discriminator |
| setupcode | 27 | 8-digit setup code |
| vendorid | 16 | Vendor ID |
| productid | 16 | Product ID |

### 3.2 Example

```
MASH:1:1234:12345678:0x1234:0x5678
```

### 3.3 Physical Placement

QR code should be:
- Printed on device label
- Visible after installation
- Optionally in packaging/manual
- Scannable by phone camera

### 3.4 Manual Entry Fallback

If QR scanning fails, user can manually enter:
- 8-digit setup code
- Discriminator (if multiple devices)

---

## 4. Capability Discovery

After connecting to a device, controllers discover capabilities through global attributes.

### 4.1 Global Attributes

Every endpoint MUST implement these attributes (reserved IDs 0xFFF0-0xFFFF):

| Attribute | ID | Type | Description |
|-----------|-----|------|-------------|
| `clusterRevision` | 0xFFFD | uint16 | MASH protocol version |
| `featureMap` | 0xFFFC | bitmap32 | Supported optional features |
| `attributeList` | 0xFFFB | array[uint16] | Implemented attribute IDs |
| `acceptedCommandList` | 0xFFFA | array[uint8] | Accepted command IDs |
| `generatedCommandList` | 0xFFF9 | array[uint8] | Response command IDs |
| `eventList` | 0xFFF8 | array[uint8] | Supported event IDs |

### 4.2 Reading Global Attributes

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

---

## 5. Feature Map

The `featureMap` is a **32-bit bitmap** indicating which optional feature sets the device supports. This is aligned with Matter's BITMAP32 type.

### 5.1 Feature Map Bits

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

### 5.2 Feature-Dependent Attribute Conformance

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

### 5.3 Two-Level Capability Discovery

FeatureMap bits indicate **high-level categories**. Detailed capability information is in feature attributes:

| FeatureMap Bit | Quick Check | Details In |
|----------------|-------------|------------|
| EMOB | Has EV charging | ChargingSession: `supportedChargingModes`, `evDemandMode` |
| ASYMMETRIC | Per-phase control | Electrical: `supportsAsymmetric` enum |
| V2X | Bidirectional EV | Electrical: `supportedDirections` enum |
| BATTERY | Has battery | Electrical: `energyCapacity` |

**Why two levels?**
- FeatureMap for quick filtering ("show me all V2X devices")
- Attributes for accurate matching ("can it do asymmetric V2G?")
- Prevents combinatorial explosion of featureMap bits
- Detailed enums are more expressive than boolean bits

**Example - V2H EVSE with asymmetric charging but symmetric discharging:**
```
featureMap: 0x060B (CORE | FLEX | EMOB | ASYMMETRIC | V2X)
  → Quick check: "yes, it does V2X and asymmetric"

Electrical.supportsAsymmetric = CONSUMPTION
  → Detail: "asymmetric only for charging"

Electrical.supportedDirections = BIDIRECTIONAL
  → Detail: "can charge and discharge"
```

---

## 6. Discovery Flow

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

---

## 7. Example Configurations

### 7.1 Basic EVSE (V1G, no flexibility)

```cbor
{
  featureMap: 0x0009,        // CORE | EMOB
  attributeList: [1, 2, 3, 10, 11, 14, 20, 21, 0xFFF8, 0xFFF9, 0xFFFA, 0xFFFB, 0xFFFC, 0xFFFD],
  acceptedCommandList: [1, 2, 5, 6],     // SetLimit, ClearLimit, SetCurrentLimits, ClearCurrentLimits
  generatedCommandList: [1, 2, 5, 6]
}
```

### 7.2 Advanced V2H EVSE (bidirectional, asymmetric, flexibility)

```cbor
{
  featureMap: 0x060B,        // CORE | FLEX | EMOB | ASYMMETRIC | V2X
  attributeList: [1, 2, 3, 10-16, 20-23, 30-33, 40-43, 50-53, 60, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],  // All limit/setpoint commands + Pause/Resume
  generatedCommandList: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
}
```

### 7.3 Heat Pump with Optional Process (OHPCF)

```cbor
{
  featureMap: 0x0083,        // CORE | FLEX | PROCESS
  attributeList: [1, 2, 3, 10, 14, 16, 20, 21, 60, 70-72, 80, 81, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 9, 10, 11, 12, 13],  // SetLimit, ClearLimit, Pause, Resume, Stop, ScheduleProcess, CancelProcess
  generatedCommandList: [1, 2, 9, 10, 11, 12, 13]
}
```

### 7.4 Battery Storage

```cbor
{
  featureMap: 0x0107,        // CORE | FLEX | BATTERY | FORECAST
  attributeList: [1, 2, 3, 10-16, 20-23, 40-43, 50-53, 60, 61, 70-72, 0xFFF8-0xFFFD],
  acceptedCommandList: [1, 2, 3, 4, 7, 8, 9, 10],  // Limits + Setpoints + Pause/Resume
  generatedCommandList: [1, 2, 3, 4, 7, 8, 9, 10]
}
```

---

## 8. Benefits of This Approach

| Benefit | Explanation |
|---------|-------------|
| **Self-describing** | Controller knows exactly what's available without trial/error |
| **Version-safe** | `clusterRevision` enables graceful protocol evolution |
| **Fine-grained** | `attributeList` gives exact attribute availability |
| **Compact** | Bitmap `featureMap` is efficient for quick capability checks |
| **Predictable** | No implicit assumptions about what "EVSE" means |

---

## 9. Comparison with EEBUS Discovery

| Aspect | EEBUS | MASH |
|--------|-------|------|
| Network discovery | mDNS with SHIP service | mDNS with MASH service |
| Capability discovery | nodeManagementUseCaseData | featureMap + attributeList |
| Granularity | Use case level | Attribute level |
| Version info | Per use case scenario | clusterRevision |
| Required attributes | Inferred from use case | Explicit in attributeList |

**Key improvements:**
- Fine-grained attribute-level discovery
- No need to lookup spec to know what attributes exist
- Clear version negotiation
- Self-describing devices
