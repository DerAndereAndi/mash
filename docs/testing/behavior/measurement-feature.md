# Measurement Feature Behavior

> Implementation behaviors for the Measurement feature

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

The **Measurement** feature provides real-time telemetry: power, energy, voltage, current, and battery state readings. It is READ-ONLY - controllers observe values but cannot modify them.

**Key concept:** Measurement uses the **passive/load sign convention** - positive values indicate consumption/charging, negative values indicate production/discharging.

**Reference implementation:** `pkg/features/measurement.go`

---

## 2. Sign Convention

### 2.1 Power Flow Direction

| Value | Meaning | Example |
|-------|---------|---------|
| Positive (+) | Consumption/charging | Device drawing power from grid |
| Negative (-) | Production/discharging | Device feeding power to grid |

### 2.2 Attributes Using Sign Convention

| Attribute | Signed | Description |
|-----------|--------|-------------|
| acActivePower | Yes | Active power (mW) |
| acReactivePower | Yes | Reactive power (mVAR) |
| acActivePowerPerPhase | Yes | Per-phase active power |
| dcPower | Yes | DC power (mW) |
| dcCurrent | Yes | DC current (mA) |
| acCurrentPerPhase | Yes | Per-phase current (mA) |

### 2.3 Unsigned Attributes

These are always positive:

| Attribute | Description |
|-----------|-------------|
| acApparentPower | Apparent power (mVA) |
| acEnergyConsumed | Cumulative energy consumed |
| acEnergyProduced | Cumulative energy produced |
| dcVoltage | DC voltage |
| acVoltagePerPhase | Phase-to-neutral voltage |

---

## 3. Nullable Attributes

### 3.1 Behavior

Most Measurement attributes are **nullable**. When a value is not available:
- The attribute exists but has no value
- Getters return `(value, false)` tuple pattern

### 3.2 Getter Pattern

```go
// Returns (value, ok) - ok is false if not set or nil
power, ok := m.ACActivePower()
if !ok {
    // Value not available
}
```

### 3.3 Why Nullable?

- Device may not measure all values
- Sensor may be offline
- Value may be invalid/stale
- Feature may not support all attributes

---

## 4. Helper Methods

### 4.1 IsConsuming()

```go
func (m *Measurement) IsConsuming() bool {
    power, ok := m.ACActivePower()
    return ok && power > 0
}
```

Returns `true` if currently consuming power (positive active power).

**Edge cases:**
- Returns `false` if power is not available
- Returns `false` if power is exactly 0
- Returns `false` if producing (negative power)

### 4.2 IsProducing()

```go
func (m *Measurement) IsProducing() bool {
    power, ok := m.ACActivePower()
    return ok && power < 0
}
```

Returns `true` if currently producing power (negative active power).

### 4.3 ActivePowerKW()

```go
func (m *Measurement) ActivePowerKW() (float64, bool) {
    power, ok := m.ACActivePower()
    if !ok {
        return 0, false
    }
    return float64(power) / 1_000_000.0, true
}
```

Returns active power in kilowatts for convenience.

---

## 5. Cumulative Energy

### 5.1 Rules

| Rule | Description |
|------|-------------|
| Always positive | Energy counters are unsigned (uint64) |
| Monotonic | Values MUST NOT decrease (except on reset) |
| Separate counters | `acEnergyConsumed` and `acEnergyProduced` are independent |

### 5.2 Energy Attributes

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| acEnergyConsumed | uint64 | mWh | Total energy consumed from grid |
| acEnergyProduced | uint64 | mWh | Total energy produced/fed to grid |
| dcEnergyIn | uint64 | mWh | Energy into DC component |
| dcEnergyOut | uint64 | mWh | Energy out of DC component |

### 5.3 Reset Behavior

Energy counters MAY reset to zero on:
- Device power cycle
- Factory reset
- Manual counter reset (device-specific)

Controllers SHOULD detect resets by checking if new value < previous value.

---

## 6. Battery State

### 6.1 State of Charge (SoC)

| Attribute | Type | Range | Unit |
|-----------|------|-------|------|
| stateOfCharge | uint8 | 0-100 | % |

**Constraints:**
- Minimum: 0
- Maximum: 100

### 6.2 State of Health (SoH)

| Attribute | Type | Range | Unit |
|-----------|------|-------|------|
| stateOfHealth | uint8 | 0-100 | % |

Indicates battery degradation: 100% = new, lower = degraded.

### 6.3 Other Battery Attributes

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| stateOfEnergy | uint64 | mWh | Available energy at current SoC |
| useableCapacity | uint64 | mWh | Current useable capacity |
| cycleCount | uint32 | - | Charge/discharge cycles |

---

## 7. Precision and Units

### 7.1 Unit Summary

| Unit | Precision | Example |
|------|-----------|---------|
| mW | milliwatts | 1,000,000 mW = 1 kW |
| mVAR | millivolt-amperes reactive | |
| mVA | millivolt-amperes | |
| mA | milliamps | |
| mV | millivolts | 230,000 mV = 230 V |
| mHz | millihertz | 50,000 mHz = 50 Hz |
| mWh | milliwatt-hours | |
| % | percentage | 0-100 |
| centi-C | centi-degrees Celsius | 2500 = 25.00 C |

### 7.2 Power Factor

| Attribute | Type | Range | Precision |
|-----------|------|-------|-----------|
| powerFactor | int16 | -1000 to +1000 | 0.001 units |

Example: 950 = 0.95 power factor

---

## 8. Attribute Summary

### 8.1 AC Power (IDs 1-9)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 1 | acActivePower | int64 | Yes | mW |
| 2 | acReactivePower | int64 | Yes | mVAR |
| 3 | acApparentPower | uint64 | Yes | mVA |

### 8.2 Per-Phase AC Power (IDs 10-19)

| ID | Name | Type | Nullable | Description |
|----|------|------|----------|-------------|
| 10 | acActivePowerPerPhase | map | Yes | Phase -> mW |
| 11 | acReactivePowerPerPhase | map | Yes | Phase -> mVAR |
| 12 | acApparentPowerPerPhase | map | Yes | Phase -> mVA |

### 8.3 AC Current & Voltage (IDs 20-29)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 20 | acCurrentPerPhase | map | Yes | mA |
| 21 | acVoltagePerPhase | map | Yes | mV |
| 22 | acVoltagePhaseToPhasePair | map | Yes | mV |
| 23 | acFrequency | uint32 | Yes | mHz |
| 24 | powerFactor | int16 | Yes | 0.001 |

### 8.4 AC Energy (IDs 30-39)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 30 | acEnergyConsumed | uint64 | Yes | mWh |
| 31 | acEnergyProduced | uint64 | Yes | mWh |

### 8.5 DC Measurements (IDs 40-49)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 40 | dcPower | int64 | Yes | mW |
| 41 | dcCurrent | int64 | Yes | mA |
| 42 | dcVoltage | uint32 | Yes | mV |
| 43 | dcEnergyIn | uint64 | Yes | mWh |
| 44 | dcEnergyOut | uint64 | Yes | mWh |

### 8.6 Battery State (IDs 50-59)

| ID | Name | Type | Nullable | Range |
|----|------|------|----------|-------|
| 50 | stateOfCharge | uint8 | Yes | 0-100% |
| 51 | stateOfHealth | uint8 | Yes | 0-100% |
| 52 | stateOfEnergy | uint64 | Yes | mWh |
| 53 | useableCapacity | uint64 | Yes | mWh |
| 54 | cycleCount | uint32 | Yes | - |

### 8.7 Temperature (IDs 60-69)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 60 | temperature | int16 | Yes | centi-C |

---

## 9. PICS Items

```
# AC Measurements
MASH.S.MEAS.AC_POWER             # acActivePower attribute
MASH.S.MEAS.AC_POWER_PERPHASE    # acActivePowerPerPhase attribute
MASH.S.MEAS.AC_ENERGY            # acEnergyConsumed/Produced attributes

# DC Measurements
MASH.S.MEAS.DC_POWER             # dcPower attribute
MASH.S.MEAS.DC_ENERGY            # dcEnergyIn/Out attributes

# Battery State
MASH.S.MEAS.SOC                  # stateOfCharge attribute
MASH.S.MEAS.SOH                  # stateOfHealth attribute

# Temperature
MASH.S.MEAS.TEMPERATURE          # temperature attribute
```

---

## 10. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-MEAS-001 | Read AC power (positive = consumption) | Set positive power, read | Positive value, IsConsuming=true |
| TC-MEAS-002 | Read AC power (negative = production) | Set negative power, read | Negative value, IsProducing=true |
| TC-MEAS-003 | Per-phase current values | Read acCurrentPerPhase | Map with phase keys |
| TC-MEAS-004 | Nullable attribute returns false when unset | Read unset attribute | (0, false) tuple |
| TC-MEAS-005 | IsConsuming helper method | Power > 0 | Returns true |
| TC-MEAS-006 | IsProducing helper method | Power < 0 | Returns true |
| TC-MEAS-007 | Cumulative energy never decreases | Read, increase, read | Second value >= first |
| TC-MEAS-008 | Battery SoC range 0-100 | Set 50%, read | Returns 50 |
| TC-MEAS-009 | Power factor bounds | Set 950, read | Returns 950 (0.95) |
| TC-MEAS-010 | Subscribe to measurement changes | Subscribe, update value | Notification received |

---

## 11. Implementation Notes

### 11.1 Map Attribute Types

Per-phase attributes use Go maps with type-safe keys:

```go
// Phase -> int64 for power/current
map[Phase]int64

// Phase -> uint32 for voltage
map[Phase]uint32

// PhasePair -> uint32 for phase-to-phase voltage
map[PhasePair]uint32
```

### 11.2 PhasePair Enum

For phase-to-phase voltage measurements:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | PhasePairAB | Between phases A and B |
| 1 | PhasePairBC | Between phases B and C |
| 2 | PhasePairCA | Between phases C and A |

### 11.3 Thread Safety

All getters/setters are thread-safe. The underlying `Feature` implementation handles synchronization.
