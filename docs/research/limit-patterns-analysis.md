# Limit & Setpoint Patterns Analysis

**Date:** 2025-01-24
**Context:** Research into EEBUS use cases and Matter 1.5 to inform MASH Limit feature design

---

## 1. Overview

This analysis covers all limit/setpoint patterns from EEBUS use cases and Matter 1.5 energy management to identify common patterns and inform a simplified, unified design for MASH.

---

## 2. EEBUS Limit Patterns

### 2.1 Hard Limits (Grid/Safety)

#### LPC - Limitation of Power Consumption
**Purpose:** DSO/SMGW limits power consumption (e.g., German EnWG 14a)

| Data Point | Description |
|------------|-------------|
| Active Power Consumption Limit | Maximum power the device may consume |
| Failsafe Consumption Active Power Limit | Fallback limit if communication lost |
| Failsafe Duration Minimum | How long to stay in failsafe (2-24h) |
| Power Consumption Nominal Max | Device's maximum possible consumption |

**Key behaviors:**
- Heartbeat required every 120s or device enters failsafe
- Device must stay in failsafe for minimum duration
- Limit can be activated/deactivated
- Optional duration on limit

#### LPP - Limitation of Power Production
**Purpose:** DSO limits power feed-in to grid (grid stability, curtailment)

| Data Point | Description |
|------------|-------------|
| Active Power Production Limit | Maximum power device may produce/feed-in |
| Failsafe Production Active Power Limit | Fallback limit if communication lost |
| Power Production Nominal Max | Device's maximum possible production |
| Contractual Production Nominal Max | Contract-limited maximum |

**Key behaviors:**
- Same heartbeat/failsafe model as LPC
- Applies to inverters, PV systems, batteries (when discharging)
- LPP has higher priority than COB setpoints

---

### 2.2 EV Charging Limits

#### OPEV - Overload Protection EV
**Purpose:** Current limits for EV charging (overload protection)

| Data Point | Description |
|------------|-------------|
| Current Limit | Maximum charging current in Amperes |
| Failsafe Current Limit | Fallback current limit |
| Nominal Max Current | Maximum current EVSE can provide |

**Key behaviors:**
- Current-based (not power-based)
- Obligatory limit - EVSE must follow
- No duration - limit stays until changed

#### CEVC - Coordinated EV Charging
**Purpose:** Power curve scheduling for optimization

| Data Point | Description |
|------------|-------------|
| Power Schedule | Time-slotted power values |
| Energy Demand | Requested energy amount |
| Departure Time | When EV needs to be ready |
| Target SoC | Desired state of charge |

**Key behaviors:**
- Schedule-based, not instant limits
- Negotiation between EVCC and CEM
- Slots with power values over time

#### OSCEV - Optimization Self-Consumption EV
**Purpose:** Use excess PV for EV charging

| Data Point | Description |
|------------|-------------|
| Power Range | Min/max power bounds |
| Incentives | Price signals for optimization |

**Key behaviors:**
- Provides flexibility range, not hard limits
- Incentive-driven optimization

#### DBEVC - Dynamic Bidirectional EV Charging
**Purpose:** V2G - EV can charge AND discharge

| Data Point | Description |
|------------|-------------|
| Power Setpoint | Target power (positive=charge, negative=discharge) |
| Power Constraints | Min/max power for charge and discharge |

**Key behaviors:**
- Signed values (+ charge, - discharge)
- Real-time setpoint control
- LPP/LPC limits take priority over setpoints

---

### 2.3 Battery Control

#### COB - Control of Battery
**Purpose:** Control battery charging/discharging

**Control Mode "Power":**
| Data Point | Description |
|------------|-------------|
| Charge/Discharge AC Power Setpoint | Target power for battery |
| Default AC Power | Fallback when setpoint deactivated |

**Control Mode "PCC" (Power at Grid Connection Point):**
| Data Point | Description |
|------------|-------------|
| PCC Power Setpoint | Target power at grid connection |
| Max AC Charge Power | Maximum charging power allowed |
| Max AC Discharge Power | Maximum discharging power allowed |
| Max DC Charge/Discharge Power | For hybrid inverters |

**Key behaviors:**
- Two control modes: direct battery power vs grid connection power
- Setpoints have duration, expire automatically
- Heartbeat required or setpoints deactivated
- LPC/LPP limits have higher priority than COB setpoints
- Passive sign convention: + = charging, - = discharging

---

### 2.4 Heat Pump Control

#### OHPCF - Optimization Heat Pump Compressor Flexibility
**Purpose:** Control heat pump for self-consumption optimization

| Data Point | Description |
|------------|-------------|
| Operating Mode | Normal, boosted, reduced, off |
| SG-Ready Mode | 4 modes per German SG-Ready spec |
| Flexibility Windows | When heat pump can be shifted |

**Key behaviors:**
- Mode-based, not power-based
- SG-Ready: 4 modes (off, normal, on, boost)
- Thermal storage buffer determines flexibility

---

### 2.5 Time-Based Limits

#### POEN - Power Envelope
**Purpose:** Time-based limit curves from DSO/Aggregator

| Curve Type | Description |
|------------|-------------|
| Min Power Consumption | Floor for consumption |
| Max Power Consumption | Ceiling for consumption |
| Min Power Production | Floor for production |
| Max Power Production | Ceiling for production |

**Key behaviors:**
- Up to 4 limit curves (min/max x consumption/production)
- Time slots with values (like a schedule)
- Must cover at least 6 hours ahead
- Maximum 48 hours ahead
- Used by aggregators/virtual power plants

---

### 2.6 Monitoring (Not Control)

#### MGCP - Monitoring of Grid Connection Point
**Purpose:** Monitor power at grid connection (not control)

| Data Point | Description |
|------------|-------------|
| Active Power | Current power at GCP |
| PV Feed-in Limitation Factor | Current curtailment level |
| Phase Powers | Per-phase measurements |

**Not a limit use case - just monitoring.**

---

## 3. Matter 1.5 Energy Management

### DeviceEnergyManagement Cluster
**Purpose:** Unified energy device control

| Attribute | Description |
|-----------|-------------|
| ESAType | Device type (EVSE, heat pump, battery, etc.) |
| ESAState | Current state (online, offline, fault) |
| PowerAdjustmentCapability | What adjustments device supports |
| Forecast | Power/energy forecast with slots |
| OptOutState | User opt-out from control |

**Commands:**
- PowerAdjustRequest(power, duration, cause)
- CancelPowerAdjustRequest()
- StartTimeAdjustRequest(requestedStartTime)
- PauseRequest(duration)
- ResumeRequest()

**Key patterns:**
- Single cluster for all energy devices
- Power adjustment with cause (grid, local optimization, etc.)
- User opt-out capability
- Forecast-based planning

### EVSE Cluster (Matter 1.5)
| Attribute | Description |
|-----------|-------------|
| CircuitCapacity | Max current the circuit can handle |
| MinimumChargeCurrent | Lowest current EVSE can provide |
| MaximumChargeCurrent | Highest current EVSE can provide |
| UserMaximumChargeCurrent | User-set limit |
| ChargingEnabledUntil | Time limit on charging |

**Key patterns:**
- Separates circuit limit from EVSE limit from user limit
- Most restrictive wins
- Time-limited enables

---

## 4. Common Patterns Identified

### Pattern 1: Hard Limits vs Setpoints
```
Hard Limit = "you MUST NOT exceed this" (safety/grid)
Setpoint = "please try to hit this target" (optimization)

Priority: Hard Limits > Setpoints
```

### Pattern 2: Failsafe Behavior
```
All EEBUS limit use cases have:
- Heartbeat requirement (120s timeout)
- Failsafe value (pre-configured fallback)
- Failsafe duration (minimum time in failsafe)
```

### Pattern 3: Direction Awareness
```
Consumption limits (power IN to device)
Production limits (power OUT from device)
Bidirectional devices need both
```

### Pattern 4: Stacking/Priority
```
Multiple controllers can set limits
Resolution: most restrictive wins (min of all limits)
Each limit source is tracked
```

### Pattern 5: Duration/Expiry
```
Limits can have duration
Expired limit = no limit (unless failsafe)
Setpoints expire -> use default value
```

### Pattern 6: Time-Based Schedules
```
Simple: single limit value
Complex: schedule with time slots
POEN: up to 4 curves (min/max x in/out)
```

---

## 5. Proposed MASH Simplification

Based on analysis, MASH should support:

### Single "Limit" Feature (per direction)

```
Limit Feature:
  Attributes:
    - effectiveLimit: uint32 (W)     // min(all zone limits) - read only
    - myLimit: uint32 (W)            // this zone's limit (zone-scoped)
    - limitActive: bool              // is any limit active?
    - nominalMax: uint32 (W)         // device's max capability
    - failsafeLimit: uint32 (W)      // fallback if communication lost
    - failsafeDuration: uint32 (s)   // how long to stay in failsafe
    - direction: enum                // CONSUMPTION, PRODUCTION, BIDIRECTIONAL

  Commands:
    - SetLimit(value, duration?)     // set limit, optional duration
    - ClearLimit()                   // remove this zone's limit
```

### Key Simplifications:

| EEBUS Complexity | MASH Simplification |
|------------------|---------------------|
| LPC + LPP separate | Single Limit with direction attribute |
| OPEV uses current (A) | Always use power (W), device converts |
| COB has 2 control modes | Single setpoint model |
| POEN has 4 curves | Single limit value (schedules via Subscribe pattern) |
| Multiple heartbeat specs | Single keep-alive at transport layer |
| Trust levels 0-100 | Binary: zone can set limit or not |

### Bidirectional Devices:

```
For battery/V2G devices with BIDIRECTIONAL direction:
  - positive limit = max consumption (charging)
  - negative limit = max production (discharging)

OR use two separate Limit features on same endpoint:
  - Limit (direction: CONSUMPTION)
  - Limit (direction: PRODUCTION)
```

### Failsafe Behavior:

```
1. Transport keep-alive fails (no pong for 3x ping)
2. Connection marked dead
3. Device starts failsafe timer
4. If no reconnect within failsafeDuration:
   - effectiveLimit = failsafeLimit
5. On reconnect:
   - Zone must re-set limit (subscriptions are gone)
   - Device exits failsafe when valid limit received
```

---

## 6. Open Design Questions

1. **Schedules:** Support time-slotted limits or keep it simple with single values?
   - Recommendation: Start simple, add schedule extension later if needed

2. **Current vs Power:** Some devices (EVSE) naturally work in Amperes
   - Recommendation: Always expose Watts at protocol level, device converts internally

3. **Separate features for consumption/production?**
   - Option A: Single Limit feature with direction
   - Option B: Two features (ConsumptionLimit, ProductionLimit)
   - Recommendation: Option A for simplicity

4. **Default values:** What happens when no limit is set?
   - Recommendation: No limit = nominalMax applies

---

## 7. Summary

EEBUS has 10+ use cases dealing with limits/setpoints, each with slightly different semantics. Matter 1.5 consolidates this into DeviceEnergyManagement cluster.

For MASH, we can simplify to:
- **One Limit feature** with direction (consumption/production/bidirectional)
- **Stacked limits** (most restrictive wins)
- **Failsafe** built into transport layer
- **Power-based** (Watts, not Amps)
- **Optional duration** on limits
- **Zone-scoped** limit tracking

This covers 90% of real-world use cases with 10% of the complexity.
