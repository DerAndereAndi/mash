# ChargingSession Feature Behavior

> Implementation behaviors for the ChargingSession feature

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

The **ChargingSession** feature manages EV charging sessions on EVSE endpoints. It tracks session lifecycle, EV demands, and V2G discharge capabilities.

**Key concepts:**
- **Session**: From plug-in to plug-out
- **Demand modes**: How the EV communicates its needs
- **V2G**: Vehicle-to-grid discharge capability
- **Charging modes**: Optimization strategies

**Reference implementation:** `pkg/features/charging_session.go`

---

## 2. Session Lifecycle

### 2.1 ChargingState

The `state` attribute tracks the current charging state:

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | NOT_PLUGGED_IN | No EV connected |
| 0x01 | PLUGGED_IN_NO_DEMAND | EV connected, no charge request |
| 0x02 | PLUGGED_IN_DEMAND | EV requesting charge |
| 0x03 | PLUGGED_IN_CHARGING | Actively charging |
| 0x04 | PLUGGED_IN_DISCHARGING | V2G discharging |
| 0x05 | SESSION_COMPLETE | Session ended, EV still plugged |
| 0x06 | FAULT | Error condition |

### 2.2 State Transitions

```
NOT_PLUGGED_IN ──[EV connects]──> PLUGGED_IN_NO_DEMAND
PLUGGED_IN_NO_DEMAND ──[demand signal]──> PLUGGED_IN_DEMAND
PLUGGED_IN_DEMAND ──[charge starts]──> PLUGGED_IN_CHARGING
PLUGGED_IN_CHARGING ──[charge ends]──> PLUGGED_IN_NO_DEMAND
PLUGGED_IN_CHARGING ──[V2G request]──> PLUGGED_IN_DISCHARGING
PLUGGED_IN_DISCHARGING ──[discharge ends]──> PLUGGED_IN_CHARGING
ANY_PLUGGED_IN ──[target reached]──> SESSION_COMPLETE
ANY_PLUGGED_IN ──[EV disconnects]──> NOT_PLUGGED_IN
ANY ──[error]──> FAULT
```

### 2.3 Session Management

**StartSession(sessionID, startTime)**
```go
func (c *ChargingSession) StartSession(sessionID uint32, startTime uint64) error {
    c.SetSessionID(sessionID)
    c.SetSessionStartTime(startTime)
    c.ClearSessionEndTime()
    c.SetSessionEnergyCharged(0)
    c.SetSessionEnergyDischarged(0)
    c.SetState(ChargingStatePluggedInNoDemand)
}
```

**EndSession(endTime)**
```go
func (c *ChargingSession) EndSession(endTime uint64) error {
    c.SetSessionEndTime(endTime)
    c.SetState(ChargingStateNotPluggedIn)
}
```

---

## 3. EV Demand Modes

### 3.1 EVDemandMode

The `evDemandMode` attribute indicates how the EV communicates its needs:

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | NONE | No demand information |
| 0x01 | SINGLE_DEMAND | Simple "charge me" request |
| 0x02 | SCHEDULED | Departure time specified |
| 0x03 | DYNAMIC | Real-time demand updates |
| 0x04 | DYNAMIC_BIDIRECTIONAL | Dynamic with V2G support |

### 3.2 Demand Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| evMinEnergyRequest | int64 | Energy to minimum SoC (can be negative for V2G) |
| evMaxEnergyRequest | int64 | Energy to full charge |
| evTargetEnergyRequest | int64 | Energy to target SoC |
| evDepartureTime | uint64 | When EV needs to leave (Unix timestamp) |

### 3.3 Energy Request Sign Convention

| Value | Meaning |
|-------|---------|
| Positive | Energy needed (charge required) |
| Zero | At target |
| Negative | Can discharge this much energy (V2G) |

---

## 4. V2G (Vehicle-to-Grid)

### 4.1 Discharge Constraints

| Attribute | Type | Description |
|-----------|------|-------------|
| evMinDischargingRequest | int64 | Minimum discharge limit (must be < 0) |
| evMaxDischargingRequest | int64 | Maximum discharge limit (must be >= 0) |
| evDischargeBelowTargetPermitted | bool | Allow V2G below target SoC |

### 4.2 CanDischarge Logic

```go
func (c *ChargingSession) CanDischarge() bool {
    // Check evMinDischargingRequest < 0
    minDischarge := c.evMinDischargingRequest
    if minDischarge >= 0 {
        return false
    }

    // Check evMaxDischargingRequest >= 0
    maxDischarge := c.evMaxDischargingRequest
    if maxDischarge < 0 {
        return false
    }

    // Check target energy or below target permission
    if c.evTargetEnergyRequest <= 0 {
        return true  // Already at or above target
    }

    // Check if discharge below target is permitted
    return c.evDischargeBelowTargetPermitted
}
```

**Summary of conditions:**
1. `evMinDischargingRequest < 0` (can discharge some amount)
2. `evMaxDischargingRequest >= 0` (has a valid max)
3. Either:
   - `evTargetEnergyRequest <= 0` (at or above target), OR
   - `evDischargeBelowTargetPermitted = true`

---

## 5. Charging Modes

### 5.1 ChargingMode Values

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | OFF | No optimization |
| 0x01 | PV_SURPLUS_ONLY | Charge only with solar surplus |
| 0x02 | PV_SURPLUS_THRESHOLD | Charge when surplus exceeds threshold |
| 0x03 | PRICE_OPTIMIZED | Charge during low price periods |
| 0x04 | SCHEDULED | Follow explicit schedule |

### 5.2 Mode Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| chargingMode | uint8 | Active optimization strategy |
| supportedChargingModes | array | Modes the EVSE supports |
| surplusThreshold | int64 | Threshold for PV_SURPLUS_THRESHOLD mode (mW) |

### 5.3 SetChargingMode Command

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| mode | uint8 | Yes | Desired mode |
| surplusThreshold | int64 | No | Threshold for PV_SURPLUS_THRESHOLD |
| startDelay | uint32 | No | Delay before starting charge (s) |
| stopDelay | uint32 | No | Delay before stopping charge (s) |

**Response:**
```go
{
    "success": true,
    "activeMode": <uint8>  // May differ if requested mode not supported
}
```

### 5.4 SupportsMode Helper

```go
func (c *ChargingSession) SupportsMode(mode ChargingMode) bool {
    modes := c.supportedChargingModes
    for _, m := range modes {
        if m == mode {
            return true
        }
    }
    return mode == ChargingModeOff  // OFF always supported
}
```

---

## 6. EV Identification

### 6.1 EVIDType Values

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | PCID | Protocol Connection ID |
| 0x01 | MAC_EUI48 | MAC address (48-bit) |
| 0x02 | MAC_EUI64 | MAC address (64-bit) |
| 0x03 | RFID | RFID tag |
| 0x04 | VIN | Vehicle Identification Number |
| 0x05 | CONTRACT_ID | Contract identifier |
| 0x06 | EVCC_ID | EV Communication Controller ID |
| 0xFF | OTHER | Other identifier |

### 6.2 EVIdentification Structure

```go
type EVIdentification struct {
    Type  EVIDType
    Value string
}
```

Multiple identifiers may be present (e.g., VIN + RFID).

---

## 7. Session Energy Tracking

### 7.1 Energy Attributes

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| sessionEnergyCharged | uint64 | mWh | Energy delivered to EV |
| sessionEnergyDischarged | uint64 | mWh | Energy returned from EV (V2G) |

### 7.2 Rules

- Both counters reset to 0 when session starts
- Counters are cumulative within session
- Net energy = charged - discharged

---

## 8. Timing Attributes

### 8.1 Delays

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| startDelay | uint32 | s | Delay before (re)starting charge |
| stopDelay | uint32 | s | Delay before pausing charge |

**Purpose:** Prevent rapid cycling (e.g., when tracking solar surplus).

### 8.2 Estimated Times

| Attribute | Type | Unit | Description |
|-----------|------|------|-------------|
| estimatedTimeToMinSoC | uint32 | s | Time to reach minimum SoC |
| estimatedTimeToTargetSoC | uint32 | s | Time to reach target SoC |
| estimatedTimeToFullSoC | uint32 | s | Time to full charge |

---

## 9. Helper Methods

| Method | Returns | Description |
|--------|---------|-------------|
| IsPluggedIn() | bool | state != NOT_PLUGGED_IN |
| IsCharging() | bool | state == PLUGGED_IN_CHARGING |
| IsDischarging() | bool | state == PLUGGED_IN_DISCHARGING |
| CanDischarge() | bool | V2G discharge permitted (see section 4.2) |
| SupportsMode(mode) | bool | Charging mode supported |

---

## 10. Attribute Summary

### 10.1 Session State (IDs 1-9)

| ID | Name | Type | Nullable |
|----|------|------|----------|
| 1 | state | uint8 | No |
| 2 | sessionId | uint32 | No |
| 3 | sessionStartTime | uint64 | Yes |
| 4 | sessionEndTime | uint64 | Yes |

### 10.2 Session Energy (IDs 10-19)

| ID | Name | Type | Unit |
|----|------|------|------|
| 10 | sessionEnergyCharged | uint64 | mWh |
| 11 | sessionEnergyDischarged | uint64 | mWh |

### 10.3 EV Identification (IDs 20-29)

| ID | Name | Type | Description |
|----|------|------|-------------|
| 20 | evIdentifications | array | List of EV identifiers |

### 10.4 EV Battery State (IDs 30-39)

| ID | Name | Type | Unit |
|----|------|------|------|
| 30 | evStateOfCharge | uint8 | % |
| 31 | evBatteryCapacity | uint64 | mWh |

### 10.5 EV Energy Demands (IDs 40-49)

| ID | Name | Type | Unit |
|----|------|------|------|
| 40 | evDemandMode | uint8 | - |
| 41 | evMinEnergyRequest | int64 | mWh |
| 42 | evMaxEnergyRequest | int64 | mWh |
| 43 | evTargetEnergyRequest | int64 | mWh |
| 44 | evDepartureTime | uint64 | Unix timestamp |

### 10.6 V2G Constraints (IDs 50-59)

| ID | Name | Type | Description |
|----|------|------|-------------|
| 50 | evMinDischargingRequest | int64 | Minimum discharge (mWh, < 0) |
| 51 | evMaxDischargingRequest | int64 | Maximum discharge (mWh, >= 0) |
| 52 | evDischargeBelowTargetPermitted | bool | Allow V2G below target |

### 10.7 Charging Mode (IDs 70-79)

| ID | Name | Type | Description |
|----|------|------|-------------|
| 70 | chargingMode | uint8 | Active mode |
| 71 | supportedChargingModes | array | Supported modes |
| 72 | surplusThreshold | int64 | Threshold for PV mode (mW) |

### 10.8 Delays (IDs 80-89)

| ID | Name | Type | Unit |
|----|------|------|------|
| 80 | startDelay | uint32 | s |
| 81 | stopDelay | uint32 | s |

---

## 11. PICS Items

```
# Session management
MASH.S.CHRG.SESSION              # Session tracking supported

# V2G capabilities
MASH.S.CHRG.V2G                  # V2G discharge supported
MASH.S.CHRG.V2G_BELOW_TARGET     # V2G below target supported

# Charging modes
MASH.S.CHRG.MODE_PV_SURPLUS      # PV surplus mode supported
MASH.S.CHRG.MODE_PRICE           # Price optimization supported
MASH.S.CHRG.MODE_SCHEDULED       # Scheduled mode supported

# EV demand modes
MASH.S.CHRG.DEMAND_SCHEDULED     # Scheduled demand mode supported
MASH.S.CHRG.DEMAND_DYNAMIC       # Dynamic demand mode supported
```

---

## 12. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-CHRG-001 | Session lifecycle | StartSession, charge, EndSession | State transitions correctly |
| TC-CHRG-002 | StartSession sets ID and time | Call StartSession | sessionId and sessionStartTime set |
| TC-CHRG-003 | EndSession returns to NOT_PLUGGED_IN | Call EndSession | state = NOT_PLUGGED_IN |
| TC-CHRG-004 | EV demand modes | Set evDemandMode | Mode reflected in attribute |
| TC-CHRG-005 | CanDischarge logic (all conditions) | Set V2G constraints | CanDischarge returns correct value |
| TC-CHRG-006 | CanDischarge false when min >= 0 | evMinDischargingRequest = 0 | CanDischarge = false |
| TC-CHRG-007 | Charging mode validation | SetChargingMode | Only supported modes accepted |
| TC-CHRG-008 | Start/stop delays | Set delays, change mode | Delays applied |
| TC-CHRG-009 | EV identification storage | Set multiple IDs | All IDs stored |
| TC-CHRG-010 | Session energy tracking | Charge, discharge | Counters updated correctly |
