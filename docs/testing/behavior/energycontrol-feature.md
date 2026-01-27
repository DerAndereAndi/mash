# EnergyControl Feature Behavior

> Implementation behaviors for the EnergyControl feature

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

The **EnergyControl** feature provides control over energy devices through limits, setpoints, and process management. It is the primary interface for zone-based energy management.

**Key concepts:**
- **Limits**: Maximum power constraints ("do not exceed")
- **Setpoints**: Target power values ("aim for this")
- **Process control**: Pause/resume/stop operations
- **Multi-zone**: Each zone's values are tracked separately, with resolution to effective values

**Reference implementation:** `pkg/features/energy_control.go`

---

## 2. State Machines

### 2.1 ControlState

The `controlState` attribute tracks the control relationship:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | AUTONOMOUS | Not under external control |
| 1 | CONTROLLED | Under controller authority, no active limit |
| 2 | LIMITED | Active power limit being applied |
| 3 | FAILSAFE | Connection lost, using failsafe limits |
| 4 | OVERRIDE | Device overriding limits (safety/legal) |

**State transitions:**

```
AUTONOMOUS ──[first command]──> CONTROLLED
CONTROLLED ──[limit active]───> LIMITED
CONTROLLED ──[conn lost]──────> FAILSAFE
LIMITED ────[limit cleared]───> CONTROLLED
LIMITED ────[conn lost]───────> FAILSAFE
FAILSAFE ───[failsafe expires]> AUTONOMOUS
FAILSAFE ───[reconnect]───────> CONTROLLED/LIMITED
ANY ────────[safety/override]─> OVERRIDE
OVERRIDE ───[override ends]───> (previous state)
```

### 2.2 ProcessState

The `processState` attribute tracks optional task lifecycle:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | NONE | No active process |
| 1 | AVAILABLE | Process can be started |
| 2 | SCHEDULED | Start time set, waiting |
| 3 | RUNNING | Actively executing |
| 4 | PAUSED | Temporarily paused |
| 5 | COMPLETED | Successfully finished |
| 6 | ABORTED | Stopped or failed |

**Key behavior:** ControlState and ProcessState are **orthogonal**. A process continues running even in FAILSAFE state.

---

## 3. Control Capabilities

### 3.1 Capability Flags

| Attribute | Type | Description |
|-----------|------|-------------|
| acceptsLimits | bool | Accepts SetLimit command |
| acceptsCurrentLimits | bool | Accepts SetCurrentLimits command |
| acceptsSetpoints | bool | Accepts SetSetpoint command |
| acceptsCurrentSetpoints | bool | Accepts SetCurrentSetpoints command |
| isPausable | bool | Accepts Pause/Resume commands |
| isShiftable | bool | Accepts AdjustStartTime command |
| isStoppable | bool | Accepts Stop command |

### 3.2 Command Validation

Before executing a command, check capability:

```go
// SetLimit requires acceptsLimits = true
if !ec.AcceptsLimits() {
    return error("command not supported")
}
```

---

## 4. Effective vs Own Values

### 4.1 Concept

Each attribute has two forms:
- **my***: This zone's value (what this zone set)
- **effective***: The resolved value across all zones

| My Attribute | Effective Attribute |
|--------------|---------------------|
| myConsumptionLimit | effectiveConsumptionLimit |
| myProductionLimit | effectiveProductionLimit |
| myConsumptionSetpoint | effectiveConsumptionSetpoint |
| myProductionSetpoint | effectiveProductionSetpoint |
| myCurrentLimitsConsumption | effectiveCurrentLimitsConsumption |
| myCurrentLimitsProduction | effectiveCurrentLimitsProduction |
| myCurrentSetpointsConsumption | effectiveCurrentSetpointsConsumption |
| myCurrentSetpointsProduction | effectiveCurrentSetpointsProduction |

### 4.2 Resolution Rules

**Limits:** Most restrictive wins (smallest value)
```go
effectiveConsumptionLimit = min(all zone consumption limits)
```

**Setpoints:** Highest priority wins (lowest zone priority number)
```go
effectiveConsumptionSetpoint = setpoint from highest priority zone
```

See `multi-zone-resolution.md` for detailed resolution behavior.

---

## 5. Commands

### 5.1 SetLimit

Sets power limits for this zone.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| consumptionLimit | int64 | No | Consumption limit (mW) |
| productionLimit | int64 | No | Production limit (mW) |
| duration | uint32 | No | Limit duration (seconds, 0=indefinite) |
| cause | uint8 | Yes | Reason for limit (LimitCause) |

**LimitCause values:**
| Value | Name | Meaning |
|-------|------|---------|
| 0 | GRID_EMERGENCY | Emergency grid condition |
| 1 | GRID_OPTIMIZATION | Grid optimization request |
| 2 | LOCAL_PROTECTION | Local circuit protection |
| 3 | LOCAL_OPTIMIZATION | Local energy optimization |
| 4 | USER_PREFERENCE | User-requested limit |

**Response:**
```go
{
    "success": true,
    "effectiveConsumptionLimit": <int64>,
    "effectiveProductionLimit": <int64>
}
```

### 5.2 ClearLimit

Removes this zone's power limits.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| direction | uint8 | No | Direction to clear (nil=both) |

### 5.3 SetCurrentLimits

Sets per-phase current limits.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| phases | map | Yes | Phase -> current (mA) |
| direction | uint8 | Yes | Consumption or Production |
| duration | uint32 | No | Limit duration (seconds) |
| cause | uint8 | Yes | Reason for limit |

### 5.4 SetSetpoint

Sets power setpoint for this zone.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| consumptionSetpoint | int64 | No | Target consumption (mW) |
| productionSetpoint | int64 | No | Target production (mW) |
| duration | uint32 | No | Setpoint duration (seconds) |
| cause | uint8 | Yes | Reason for setpoint |

**SetpointCause values:**
| Value | Name | Meaning |
|-------|------|---------|
| 0 | GRID_REQUEST | Grid service request |
| 1 | SELF_CONSUMPTION | Optimize self-consumption |
| 2 | PRICE_OPTIMIZATION | Price-based optimization |
| 3 | PHASE_BALANCING | Balance across phases |
| 4 | USER_PREFERENCE | User-requested target |

### 5.5 Pause

Temporarily pauses device operation.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| duration | uint32 | No | Pause duration (seconds) |

**Behavior:**
- Sets processState to PAUSED
- Device stops operation
- Can be resumed with Resume command or duration expiry

### 5.6 Resume

Resumes paused operation.

**Behavior:**
- Sets processState to RUNNING
- Device resumes from where it paused

### 5.7 Stop

Aborts the current process.

**Behavior:**
- Sets processState to ABORTED
- Process cannot be resumed
- New process must be started

---

## 6. OptOut State

### 6.1 Values

The `optOutState` attribute allows devices to opt out of external control:

| Value | Name | Meaning |
|-------|------|---------|
| 0 | NONE | Accept all control |
| 1 | LOCAL | Opt out of local EMS control |
| 2 | GRID | Opt out of grid operator control |
| 3 | ALL | Opt out of all external control |

### 6.2 Behavior

When optOutState is set:
- Commands from opted-out zones are rejected
- Device operates autonomously for those zones
- Safety overrides still apply

---

## 7. Failsafe Configuration

### 7.1 Attributes

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| failsafeConsumptionLimit | int64 | null | Limit to apply in FAILSAFE |
| failsafeProductionLimit | int64 | null | Production limit in FAILSAFE |
| failsafeDuration | uint32 | 7200 | Time in FAILSAFE before AUTONOMOUS (seconds) |

### 7.2 Behavior

When connection to a zone is lost:
1. ControlState transitions to FAILSAFE
2. Failsafe limits are applied
3. After failsafeDuration, transitions to AUTONOMOUS
4. If zone reconnects, exits FAILSAFE immediately

---

## 8. Device Types

The `deviceType` attribute indicates the type of controllable device:

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | EVSE | Electric vehicle supply equipment |
| 0x01 | HEAT_PUMP | Heat pump |
| 0x02 | WATER_HEATER | Water heater |
| 0x03 | BATTERY | Battery storage |
| 0x04 | INVERTER | Solar/battery inverter |
| 0x05 | FLEXIBLE_LOAD | Generic flexible load |
| 0xFF | OTHER | Other device type |

---

## 9. Attribute Summary

### 9.1 State Attributes (IDs 1-9)

| ID | Name | Type | Access | Description |
|----|------|------|--------|-------------|
| 1 | deviceType | uint8 | RO | Type of device |
| 2 | controlState | uint8 | RO | Control relationship state |
| 3 | optOutState | uint8 | RW | Opt-out state |

### 9.2 Capability Attributes (IDs 10-19)

| ID | Name | Type | Access | Description |
|----|------|------|--------|-------------|
| 10 | acceptsLimits | bool | RO | Accepts SetLimit |
| 11 | acceptsCurrentLimits | bool | RO | Accepts SetCurrentLimits |
| 12 | acceptsSetpoints | bool | RO | Accepts SetSetpoint |
| 13 | acceptsCurrentSetpoints | bool | RO | Accepts SetCurrentSetpoints |
| 14 | isPausable | bool | RO | Accepts Pause/Resume |
| 15 | isShiftable | bool | RO | Accepts AdjustStartTime |
| 16 | isStoppable | bool | RO | Accepts Stop |

### 9.3 Power Limits (IDs 20-29)

| ID | Name | Type | Nullable | Unit |
|----|------|------|----------|------|
| 20 | effectiveConsumptionLimit | int64 | Yes | mW |
| 21 | myConsumptionLimit | int64 | Yes | mW |
| 22 | effectiveProductionLimit | int64 | Yes | mW |
| 23 | myProductionLimit | int64 | Yes | mW |

### 9.4 Failsafe Attributes (IDs 70-79)

| ID | Name | Type | Unit | Default |
|----|------|------|------|---------|
| 70 | failsafeConsumptionLimit | int64 | mW | null |
| 71 | failsafeProductionLimit | int64 | mW | null |
| 72 | failsafeDuration | uint32 | s | 7200 |

### 9.5 Process Attributes (IDs 80-89)

| ID | Name | Type | Description |
|----|------|------|-------------|
| 80 | processState | uint8 | Process lifecycle state |
| 81 | optionalProcess | bool | Process is optional |

---

## 10. PICS Items

```
# Control capabilities
MASH.S.CTRL.LIMITS               # acceptsLimits = true
MASH.S.CTRL.CURRENT_LIMITS       # acceptsCurrentLimits = true
MASH.S.CTRL.SETPOINTS            # acceptsSetpoints = true
MASH.S.CTRL.PAUSABLE             # isPausable = true
MASH.S.CTRL.STOPPABLE            # isStoppable = true

# State machines
MASH.S.CTRL.FAILSAFE             # Failsafe behavior supported
MASH.S.CTRL.PROCESS              # Process state machine supported

# Multi-zone
MASH.S.CTRL.MULTIZONE            # Multi-zone resolution supported
```

---

## 11. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-CTRL-001 | SetLimit command accepted | Send SetLimit, read effective | Limit applied |
| TC-CTRL-002 | ClearLimit removes limit | Set then clear limit | effectiveLimit = null |
| TC-CTRL-003 | SetSetpoint command accepted | Send SetSetpoint | Setpoint applied |
| TC-CTRL-004 | Pause command transitions state | Send Pause | processState = PAUSED |
| TC-CTRL-005 | Resume from paused state | Pause then Resume | processState = RUNNING |
| TC-CTRL-006 | Stop aborts process | Send Stop | processState = ABORTED |
| TC-CTRL-007 | Effective vs own limit values | Two zones set limits | Effective = min |
| TC-CTRL-008 | OptOut states respected | Set optOutState, send command | Command rejected |
| TC-CTRL-009 | Failsafe limits applied | Simulate disconnect | Failsafe limits active |
| TC-CTRL-010 | Command to unsupported capability | Send SetLimit when acceptsLimits=false | Error returned |
| TC-CTRL-011 | Duration parameter expires limit | Set limit with duration | Limit clears after duration |
| TC-CTRL-012 | Per-phase current limits | Send SetCurrentLimits | Per-phase limits applied |

---

## 12. Implementation Notes

### 12.1 Handler Pattern

Commands use handler callbacks for device-specific implementation:

```go
ec.OnSetLimit(func(ctx context.Context, consumptionLimit, productionLimit *int64, cause LimitCause) (int64, int64, error) {
    // Device-specific implementation
    return effectiveConsumption, effectiveProduction, nil
})
```

### 12.2 Default Behavior

If no handler is set, commands return `{success: false}`.

### 12.3 Helper Methods

| Method | Returns | Description |
|--------|---------|-------------|
| IsLimited() | bool | controlState == LIMITED |
| IsFailsafe() | bool | controlState == FAILSAFE |
| AcceptsLimits() | bool | acceptsLimits attribute value |
| IsPausable() | bool | isPausable attribute value |
