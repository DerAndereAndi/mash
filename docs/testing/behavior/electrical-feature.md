# Electrical Feature Behavior

> Implementation behaviors for the Electrical feature

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

The **Electrical** feature describes the electrical characteristics and capability envelope of an endpoint. It provides static configuration (nameplate data) that controllers use to understand what the device can do.

**Key concept:** Electrical is a READ-ONLY feature - all attributes describe physical characteristics that cannot be changed via the protocol.

**Reference implementation:** `pkg/features/electrical.go`

---

## 2. Direction Capability

### 2.1 Direction Enum

The `supportedDirections` attribute indicates power flow capability:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | `DirectionConsumption` | Device can only consume power (default) |
| 1 | `DirectionProduction` | Device can only produce power |
| 2 | `DirectionBidirectional` | Device can both consume and produce |

### 2.2 Capability Detection Methods

**IsBidirectional()**
```go
func (e *Electrical) IsBidirectional() bool {
    return e.SupportedDirections() == DirectionBidirectional
}
```
Returns `true` only if the device explicitly supports bidirectional power flow.

**CanConsume()**
```go
func (e *Electrical) CanConsume() bool {
    dir := e.SupportedDirections()
    return dir == DirectionConsumption || dir == DirectionBidirectional
}
```
Returns `true` if the device can consume power (consumption-only OR bidirectional).

**CanProduce()**
```go
func (e *Electrical) CanProduce() bool {
    dir := e.SupportedDirections()
    return dir == DirectionProduction || dir == DirectionBidirectional
}
```
Returns `true` if the device can produce power (production-only OR bidirectional).

### 2.3 Direction by Device Type

| Device Type | Typical Direction | Example |
|-------------|------------------|---------|
| EVSE | Consumption or Bidirectional | V2G-capable charger |
| Heat Pump | Consumption | Heating system |
| Battery | Bidirectional | Home storage |
| Inverter | Production or Bidirectional | Solar inverter |
| PV String | Production | Solar panels |

---

## 3. Phase Configuration

### 3.1 Phase Count

The `phaseCount` attribute indicates the number of electrical phases:

| Value | Meaning |
|-------|---------|
| 1 | Single-phase device |
| 2 | Two-phase (rare) |
| 3 | Three-phase device |

**Constraints:**
- Minimum: 1
- Maximum: 3
- Default: 1

### 3.2 Phase Type

The `Phase` enum represents device phases:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | `PhaseA` | First phase |
| 1 | `PhaseB` | Second phase |
| 2 | `PhaseC` | Third phase |

### 3.3 Grid Phase Type

The `GridPhase` enum represents grid connection points:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | `GridPhaseL1` | Grid line 1 |
| 1 | `GridPhaseL2` | Grid line 2 |
| 2 | `GridPhaseL3` | Grid line 3 |

### 3.4 Phase Mapping

The `phaseMapping` attribute maps device phases to grid phases:

```go
// Example: Device phase A connected to grid L2
mapping := map[Phase]GridPhase{
    PhaseA: GridPhaseL2,
    PhaseB: GridPhaseL3,
    PhaseC: GridPhaseL1,
}
```

**Use cases:**
- Phase-specific current limits
- Load balancing across grid phases
- Phase identification in multi-phase measurements

---

## 4. Power Ratings

### 4.1 Nameplate Values

All power values are in **milliwatts (mW)**.

| Attribute | Type | Description |
|-----------|------|-------------|
| `nominalMaxConsumption` | int64 | Maximum consumption power |
| `nominalMaxProduction` | int64 | Maximum production power (0 if N/A) |
| `nominalMinPower` | int64 | Minimum operating point |

**Sign convention:** Power ratings are always **positive** values representing magnitude.

### 4.2 Current Ratings

All current values are in **milliamps (mA)**.

| Attribute | Type | Description |
|-----------|------|-------------|
| `maxCurrentPerPhase` | int64 | Maximum current per phase |
| `minCurrentPerPhase` | int64 | Minimum current per phase |

---

## 5. Asymmetric Support

### 5.1 AsymmetricSupport Enum

The `supportsAsymmetric` attribute indicates per-phase control capability:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | `AsymmetricNone` | All phases must be controlled together |
| 1 | `AsymmetricReadOnly` | Per-phase values readable but not controllable |
| 2 | `AsymmetricFull` | Per-phase control supported |

### 5.2 Implications

| Support Level | SetLimit | SetCurrentLimits |
|---------------|----------|------------------|
| None | Single value applies to all phases | Not supported |
| ReadOnly | Single value applies to all phases | Not supported |
| Full | Single value applies to all phases | Per-phase values accepted |

---

## 6. Storage Attributes

For battery/storage devices:

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| `energyCapacity` | int64 | mWh | Battery/storage capacity (0 if N/A) |

---

## 7. Voltage and Frequency

| Attribute | Type | Unit | Default | Description |
|-----------|------|------|---------|-------------|
| `nominalVoltage` | uint16 | V | 230 | Nominal voltage |
| `nominalFrequency` | uint8 | Hz | 50 | Nominal frequency |

---

## 8. Attribute Summary

| ID | Name | Type | Access | Nullable | Unit |
|----|------|------|--------|----------|------|
| 1 | phaseCount | uint8 | RO | No | - |
| 2 | phaseMapping | map | RO | No | - |
| 3 | nominalVoltage | uint16 | RO | No | V |
| 4 | nominalFrequency | uint8 | RO | No | Hz |
| 5 | supportedDirections | uint8 | RO | No | - |
| 10 | nominalMaxConsumption | int64 | RO | No | mW |
| 11 | nominalMaxProduction | int64 | RO | No | mW |
| 12 | nominalMinPower | int64 | RO | No | mW |
| 13 | maxCurrentPerPhase | int64 | RO | No | mA |
| 14 | minCurrentPerPhase | int64 | RO | No | mA |
| 15 | supportsAsymmetric | uint8 | RO | No | - |
| 20 | energyCapacity | int64 | RO | No | mWh |

---

## 9. PICS Items

```
# Direction capability
MASH.S.ELEC.BIDIR                # supportedDirections = Bidirectional
MASH.S.ELEC.CONSUME              # supportedDirections includes consumption
MASH.S.ELEC.PRODUCE              # supportedDirections includes production

# Phase capability
MASH.S.ELEC.3PHASE               # phaseCount = 3
MASH.S.ELEC.ASYMMETRIC           # supportsAsymmetric = Full

# Storage
MASH.S.ELEC.STORAGE              # energyCapacity > 0
```

---

## 10. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ELEC-001 | Read phase configuration | Read phaseCount, phaseMapping | Valid values returned |
| TC-ELEC-002 | IsBidirectional returns correct value | Read supportedDirections | Matches expected capability |
| TC-ELEC-003 | CanConsume for consumption-only device | Check IsBidirectional, CanConsume | IsBidirectional=false, CanConsume=true |
| TC-ELEC-004 | CanProduce for production-only device | Check IsBidirectional, CanProduce | IsBidirectional=false, CanProduce=true |
| TC-ELEC-005 | Phase mapping consistency | Read phaseCount, phaseMapping | Mapping covers all phases |
| TC-ELEC-006 | Power ratings validation | Read nominalMaxConsumption, nominalMaxProduction | Non-negative values |
| TC-ELEC-007 | Asymmetric support levels | Read supportsAsymmetric | Valid enum value |
| TC-ELEC-008 | Energy capacity for storage | Read energyCapacity | Value > 0 for battery devices |

---

## 11. Implementation Notes

### 11.1 Default Values

All getters return sensible defaults if the attribute value is not set or invalid:
- `PhaseCount()`: returns 1
- `NominalVoltage()`: returns 230
- `NominalFrequency()`: returns 50
- `SupportedDirections()`: returns DirectionConsumption
- `SupportsAsymmetric()`: returns AsymmetricNone

### 11.2 Setter Methods

Setters are provided for device implementations to configure values during initialization. All setters use `SetValueInternal()` which bypasses access control (appropriate for device-side configuration).

### 11.3 Thread Safety

The underlying `Feature` implementation handles thread safety. All getter/setter methods are safe for concurrent use.
