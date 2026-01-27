# EnergyControl Feature

> Limits, setpoints, control commands, and process management

**Feature ID:** 0x0005
**Direction:** IN (controller sends to device)
**Status:** Draft
**Last Updated:** 2025-01-25

---

## Purpose

Provides control capabilities - limits, setpoints, pause/resume, forecasts. The main control interface for energy management. Answers the question: "What SHOULD this device do?"

---

## Attributes

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

  // Optional: Flexibility/Forecast (requires FLEX/FORECAST feature flags)
  60: flexibility,            // FlexibilityStruct (optional)
  61: forecast,               // ForecastStruct (optional)

  // Failsafe configuration (for when controller connection is lost)
  70: failsafeConsumptionLimit,   // int64 mW: limit to apply in FAILSAFE state
  71: failsafeProductionLimit,    // int64 mW: limit to apply in FAILSAFE state
  72: failsafeDuration,           // uint32 s: time in FAILSAFE before AUTONOMOUS (2-24h)

  // Optional process management (requires PROCESS feature flag)
  80: processState,               // ProcessStateEnum: current process lifecycle state
  81: optionalProcess,            // OptionalProcess?: details of available/running process
}
```

---

## Enumerations

### DeviceTypeEnum

```
EVSE              = 0x00  // EV Charger
HEAT_PUMP         = 0x01  // Space heating/cooling
WATER_HEATER      = 0x02
BATTERY           = 0x03  // Home battery storage
INVERTER          = 0x04  // Solar/hybrid inverter
FLEXIBLE_LOAD     = 0x05  // Generic controllable load
OTHER             = 0xFF
```

### ControlStateEnum

```
AUTONOMOUS        = 0x00  // Not under external control (no controller connected)
CONTROLLED        = 0x01  // Under controller authority, no active limit
LIMITED           = 0x02  // Active power limit being applied
FAILSAFE          = 0x03  // Connection lost, using failsafe limits
OVERRIDE          = 0x04  // Device overriding limits (safety/legal/self-protection)
```

**Design rationale:** Device explicitly reports its control relationship state. This replaces implicit heartbeat-based inference (bad: race conditions, debugging difficulty, no single source of truth). Applies to ALL controllable device types: EVSE, battery, heat pump, inverter.

**Connection loss behavior:**
1. Device detects connection loss (TCP/TLS layer)
2. Device transitions to FAILSAFE, applies failsafeConsumptionLimit/failsafeProductionLimit
3. After failsafeDuration expires, device transitions to AUTONOMOUS
4. Device can resume normal operation without controller

### ProcessStateEnum

```
NONE              = 0x00  // No optional process available
AVAILABLE         = 0x01  // Process announced, not yet scheduled
SCHEDULED         = 0x02  // Start time configured, waiting to start
RUNNING           = 0x03  // Process currently executing
PAUSED            = 0x04  // Paused by controller (can resume)
COMPLETED         = 0x05  // Finished successfully
ABORTED           = 0x06  // Stopped/cancelled before completion
```

### LimitCauseEnum

```
GRID_EMERGENCY     = 0   // DSO/SMGW - MUST follow
GRID_OPTIMIZATION  = 1   // Grid balancing request
LOCAL_PROTECTION   = 2   // Fuse/overload protection
LOCAL_OPTIMIZATION = 3   // Cost/self-consumption optimization
USER_PREFERENCE    = 4   // User app request
```

### SetpointCauseEnum

```
GRID_REQUEST       = 0   // Grid operator/aggregator request
SELF_CONSUMPTION   = 1   // Optimize for self-consumption
PRICE_OPTIMIZATION = 2   // Optimize for energy cost
PHASE_BALANCING    = 3   // Balance load across phases (V2H)
USER_PREFERENCE    = 4   // User app request
```

### OptOutEnum

```
NO_OPT_OUT        = 0   // Accept all adjustments
LOCAL_OPT_OUT     = 1   // Reject local optimization only
GRID_OPT_OUT      = 2   // Reject grid requests only
OPT_OUT           = 3   // Reject all external control
```

---

## Data Structures

### FlexibilityStruct (requires FLEX feature flag)

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

### ForecastStruct (requires FORECAST feature flag)

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

### OptionalProcess (requires PROCESS feature flag)

```cbor
{
  // Process identification
  1: processId,              // uint32: unique identifier for this process
  2: description,            // string?: human-readable description

  // Power characteristics
  10: powerEstimate,         // int64 mW?: expected average power
  11: powerMin,              // int64 mW?: minimum operating power
  12: powerMax,              // int64 mW?: maximum operating power

  // Timing constraints
  20: estimatedDuration,     // uint32 s?: expected total duration
  21: minRunDuration,        // uint32 s: minimum time before can pause/stop
  22: minPauseDuration,      // uint32 s?: minimum time between pause/resume

  // Control constraints
  30: isPausable,            // bool: can this process be paused?
  31: isStoppable,           // bool: can this process be stopped/aborted?

  // Energy characteristics
  40: energyEstimate,        // int64 mWh?: expected total energy consumption
  41: resumeEnergyPenalty,   // int64 mWh?: additional energy if resumed after pause

  // Scheduling (set by controller)
  50: scheduledStart,        // timestamp?: when controller scheduled this to start
}
```

---

## Commands

### SetLimit

Set power limits for this zone.

```cbor
Request:
{
  1: consumptionLimit,       // int64 mW (optional - max consumption/charge)
  2: productionLimit,        // int64 mW (optional - max production/discharge)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // LimitCauseEnum
}

Response:
{
  1: success,                // bool
  2: effectiveConsumptionLimit,  // int64 mW
  3: effectiveProductionLimit    // int64 mW
}
```

### ClearLimit

Remove this zone's power limit(s).

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

### SetCurrentLimits

Set per-phase current limits.

```cbor
Request:
{
  1: phases,                 // map: {PhaseEnum → int64 mA or null}
  2: direction,              // DirectionEnum (CONSUMPTION or PRODUCTION)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // LimitCauseEnum
}

Response:
{
  1: success,                // bool
  2: effectivePhaseCurrents  // map: {PhaseEnum → int64 mA}
}
```

### ClearCurrentLimits

Remove this zone's per-phase current limits.

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

### SetSetpoint

Set power setpoint for this zone (target to achieve).

```cbor
Request:
{
  1: consumptionSetpoint,    // int64 mW (optional - target charge power)
  2: productionSetpoint,     // int64 mW (optional - target discharge power)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // SetpointCauseEnum
}

Response:
{
  1: success,                // bool
  2: effectiveConsumptionSetpoint,  // int64 mW
  3: effectiveProductionSetpoint    // int64 mW
}
```

### ClearSetpoint

Remove this zone's power setpoint(s).

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

### SetCurrentSetpoints

Set per-phase current setpoints (for V2H phase balancing).

```cbor
Request:
{
  1: phases,                 // map: {PhaseEnum → int64 mA or null}
  2: direction,              // DirectionEnum (CONSUMPTION or PRODUCTION)
  3: duration,               // uint32 s (optional, 0 = indefinite)
  4: cause                   // SetpointCauseEnum
}

Response:
{
  1: success,                // bool
  2: effectiveCurrentSetpoints  // map: {PhaseEnum → int64 mA}
}
```

### ClearCurrentSetpoints

Remove this zone's per-phase current setpoints.

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

### Pause

Temporarily pause device operation (requires isPausable = true).

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

### Resume

Resume paused operation.

```cbor
Request: (empty)

Response:
{
  1: success                 // bool
}
```

### Stop

Abort task completely (requires isStoppable = true).

```cbor
Request: (empty)

Response:
{
  1: success                 // bool
}
```

### ScheduleProcess

Schedule an optional process to start (OHPCF-style).

```cbor
Request:
{
  1: processId,              // uint32: which process to schedule
  2: requestedStart,         // timestamp?: when to start (null = start now)
  3: cause                   // SetpointCauseEnum: why scheduling
}

Response:
{
  1: success,                // bool
  2: actualStart,            // timestamp: when it will actually start
  3: newState                // ProcessStateEnum: SCHEDULED or RUNNING
}
```

### CancelProcess

Cancel a scheduled or running process.

```cbor
Request:
{
  1: processId               // uint32: which process to cancel
}

Response:
{
  1: success,                // bool
  2: newState                // ProcessStateEnum: ABORTED or NONE
}
```

### AdjustStartTime

Request start time shift.

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

---

## Multi-Zone Resolution

### Key Difference

```
LIMITS:    Most restrictive wins (all zones constrain together)
SETPOINTS: Highest priority zone wins (only one controller active)
```

### Power Limits - Most Restrictive Wins

```
Zone 1 (GRID): SetLimit(consumptionLimit: 6000000)
Zone 2 (LOCAL):  SetLimit(consumptionLimit: 5000000)

effectiveConsumptionLimit = min(6000000, 5000000) = 5000000 mW
```

### Phase Current Limits - Most Restrictive Per Phase

```
Zone 1: SetCurrentLimits({A: 20000, B: 20000, C: 20000}, CONSUMPTION)
Zone 2: SetCurrentLimits({A: 16000, B: 10000, C: 16000}, CONSUMPTION)

effectiveCurrentLimitsConsumption = {
  A: min(20000, 16000) = 16000,
  B: min(20000, 10000) = 10000,
  C: min(20000, 16000) = 16000
}
```

### Power Setpoints - Highest Priority Wins

```
Zone 1 (GRID, priority 1): SetSetpoint(consumptionSetpoint: 3000000)
Zone 2 (LOCAL, priority 2):  SetSetpoint(consumptionSetpoint: 5000000)

effectiveConsumptionSetpoint = 3000000 mW (grid operator wins)
```

### Combined Resolution

Limits constrain setpoints:

```
effectiveConsumptionLimit = 5000000 mW (5 kW)
effectiveConsumptionSetpoint = 7000000 mW (7 kW requested)

Device targets: min(7000000, 5000000) = 5000000 mW (limit caps setpoint)
```

---

## State Machines

### ControlStateEnum - Control Relationship State

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
          ▼
   ┌──────────────┐
   │   FAILSAFE   │◄─── (connection lost)
   └──────────────┘
         │
         │ failsafeDuration expires
         ▼
   (back to AUTONOMOUS)
```

### ProcessStateEnum - Optional Task Lifecycle

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

**Orthogonal state machines:** ControlStateEnum and ProcessStateEnum are independent. A device can be `controlState=LIMITED` while `processState=RUNNING`.

---

## EEBUS Use Case Mapping

| EEBUS Use Case | MASH Coverage |
|----------------|---------------|
| LPC (Limit Power Consumption) | SetLimit(consumptionLimit), ControlStateEnum, failsafe* attrs |
| LPP (Limit Power Production) | SetLimit(productionLimit), ControlStateEnum |
| OPEV (Overload Protection) | SetCurrentLimits(phases, CONSUMPTION) |
| OSCEV (Self-Consumption) | FlexibilityStruct + SetSetpoint(cause: SELF_CONSUMPTION) |
| CEVC (Coordinated Charging) | ForecastStruct + FlexibilityStruct |
| DBEVC (Bidirectional EV) | SetSetpoint(consumptionSetpoint, productionSetpoint) |
| COB (Battery Control) | SetSetpoint + ControlStateEnum |
| OHPCF (Heat Pump) | OptionalProcess + ProcessStateEnum + ScheduleProcess |
| POEN (Power Envelope) | Repeated SetLimit |

---

## Related Features

| Feature | Relationship |
|---------|--------------|
| [Electrical](electrical.md) | Provides hardware limits that constrain control |
| [Measurement](measurement.md) | Reports actual values resulting from control |
| [Status](status.md) | Reports operating state (orthogonal to control state) |
| [Signals](signals.md) | Provides time-slotted control inputs |
| [Plan](plan.md) | Device reports its intended response to control |
