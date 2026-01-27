# Feature Interaction Semantics

> How MASH features interact when multiple features affect the same behavior

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Purpose

This document specifies the **interaction semantics** between MASH features when multiple features could affect the same device behavior. These rules ensure deterministic, predictable behavior across implementations.

---

## 1. Interaction Principles

### 1.1 General Rules

1. **Safety first**: When in doubt, the more restrictive constraint wins
2. **Explicit over implicit**: Direct commands take precedence over inferred behavior
3. **Hardware limits are absolute**: No software command can exceed Electrical limits
4. **Layer separation**: Each feature has a defined scope; interactions occur at boundaries

### 1.2 Feature Layers

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 1: Hardware Capability (Electrical)                    │
│   - Absolute physical limits                                 │
│   - Dynamic updates when devices connect/disconnect          │
│   - Cannot be exceeded by any software command               │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ Layer 2: Policy Constraints (EnergyControl limits)           │
│   - Multi-zone limit resolution (most restrictive wins)      │
│   - Immediate effect on receipt                              │
│   - Constrained by Layer 1                                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: Scheduled Constraints (Signals CONSTRAINT type)     │
│   - Time-slotted limits from grid/aggregator                 │
│   - Applied per slot, constrained by Layer 2                 │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ Layer 4: Operational Targets (Setpoints, ChargingMode)       │
│   - Best-effort targets within constraint layers             │
│   - Device decides how to achieve within bounds              │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Signals + EnergyControl Interaction

### 2.1 Limit Precedence

When both Signals (CONSTRAINT type) and EnergyControl provide limits:

**Rule: Most restrictive wins across all sources**

```
effectiveLimit = min(
    Electrical.nominalMaxConsumption,           // Hardware limit
    EnergyControl.effectiveConsumptionLimit,    // Immediate limit (all zones)
    Signals.activeSlot.maxPower                 // Scheduled limit
)
```

**Example:**
```
Electrical.nominalMaxConsumption = 11000000 mW (11 kW hardware)
EnergyControl.effectiveConsumptionLimit = 7400000 mW (7.4 kW from LOCAL)
Signals.activeSlot.maxPower = 5000000 mW (5 kW from GRID)

Device operates at: max 5000000 mW (most restrictive wins)
```

### 2.2 Slot Boundary Behavior

When a Signal slot boundary is reached:

1. Device evaluates new slot's constraints
2. Device recalculates effective limits/setpoints
3. Device notifies subscribed controllers via subscription update
4. Device adjusts behavior to comply with new effective values

**Timing:**
- Slot transition is evaluated at slot boundary timestamp
- Effective values update within 1 second of boundary
- Subscription notifications sent within minInterval

### 2.3 Signal Expiry

When the last Signal slot ends and no subsequent slot exists:

**Rule: Signal constraint is removed, EnergyControl limits still apply**

```
Before last slot ends:
  effectiveLimit = min(EnergyControl.limit, Signals.slot.maxPower)

After last slot ends:
  effectiveLimit = EnergyControl.limit  // Signals no longer constrains
```

The device does NOT revert to unlimited - it reverts to whatever limits remain from EnergyControl.

### 2.4 Multi-Zone Signal Handling

When multiple zones send Signals:

**Rule: Zone priority applies, same as setpoints (highest priority wins)**

```
Zone 1 (GRID): Signal with maxPower = 3000000 mW
Zone 2 (LOCAL): Signal with maxPower = 5000000 mW

Active Signal: Zone 1's signal (GRID priority wins)
effectiveLimit from Signals = 3000000 mW
```

**Note:** This is different from EnergyControl limits where all zones constrain together. Signals represent time-slotted policies, and only the highest-priority policy is active.

---

## 3. Dynamic Electrical Updates

### 3.1 When Electrical Updates

The Electrical feature updates its values when:

| Event | Update Timing | Attributes Affected |
|-------|---------------|---------------------|
| EV connects (IEC 61851 PWM) | Within 5 seconds of PWM negotiation | nominalMaxConsumption, maxCurrentPerPhase |
| EV connects (ISO 15118) | When ChargeParameterDiscovery completes | All capability attributes |
| EV disconnects | Within 2 seconds of disconnect | Revert to EVSE hardware limits |
| ISO 15118 renegotiation | When new parameters confirmed | Affected capability attributes |
| Hardware fault | Immediately | Affected attributes (may reduce to 0) |

### 3.2 Subscription Notification

When Electrical values change:

1. Device updates Electrical attributes
2. Device sends subscription notification to all subscribers
3. Notification includes all changed attributes
4. Controllers adjust their limits/setpoints if needed

**Order guarantee:** Electrical notification is sent BEFORE any automatic EnergyControl adjustments.

### 3.3 EnergyControl Response to Electrical Changes

When Electrical values decrease (e.g., EV with lower capability connects):

**Rule: Limits that exceed new Electrical values are auto-capped**

```
Before EV connects:
  Electrical.nominalMaxConsumption = 22000000 mW (22 kW EVSE hardware)
  EnergyControl.myConsumptionLimit = 11000000 mW (11 kW from controller)

After EV connects (7.4 kW EV):
  Electrical.nominalMaxConsumption = 7400000 mW (now limited by EV)
  EnergyControl.myConsumptionLimit = 11000000 mW (unchanged - controller's intent)
  EnergyControl.effectiveConsumptionLimit = 7400000 mW (capped to Electrical)
```

**Important:** The controller's limit value (myConsumptionLimit) is NOT modified. Only the effective value is capped. If Electrical later increases, the original controller limit applies again.

### 3.4 Auto-Adjustment Notification

When Electrical changes cause effectiveLimit to change:

1. Device calculates new effective limits
2. Device sends EnergyControl subscription notification
3. Notification includes updated effective values
4. Notification includes unchanged my* values (controller's intent preserved)

---

## 4. ChargingSession + EnergyControl Interaction

### 4.1 ChargingMode vs Explicit Limits

When ChargingSession.chargingMode conflicts with EnergyControl limits:

**Rule: EnergyControl limits are hard constraints; chargingMode is advisory**

| Scenario | Behavior |
|----------|----------|
| chargingMode = PV_SURPLUS_ONLY, limit = 7400W | Charge only when surplus exists, max 7.4kW |
| chargingMode = OFF, limit = 7400W | Charge at max rate up to 7.4kW |
| chargingMode = PV_SURPLUS_ONLY, no surplus | Do not charge, even if limit allows |
| chargingMode = OFF, limit = 0W | Do not charge (limit takes precedence) |

**Explanation:** ChargingMode tells the EVSE *how* to optimize within the allowed envelope. EnergyControl limits define the envelope itself.

### 4.2 Start/Stop Delays vs Pause Command

When startDelay/stopDelay conflict with immediate Pause/Resume commands:

**Rule: Explicit commands override delays, but EVSE may enforce minimum delay for EV protection**

| Scenario | Behavior |
|----------|----------|
| Pause command received, stopDelay = 120s | EVSE stops within EVSE-determined safe time (may be < 120s) |
| Resume command, startDelay = 60s | EVSE starts within EVSE-determined safe time (may be < 60s) |
| Limit drops below minimum, stopDelay = 120s | EVSE waits stopDelay before stopping |
| Limit rises above minimum, startDelay = 60s | EVSE waits startDelay before starting |

**Rationale:** Delays are primarily for PV surplus optimization (avoiding chatter). Direct Pause/Resume commands indicate controller intent to change state, which takes priority. However, EVSE may enforce minimum EV-safe timing.

### 4.3 EV Energy Demands vs Controller Limits

When EV demands exceed controller limits:

**Rule: Controller limits constrain delivery; EV demands inform planning**

```
evTargetEnergyRequest = 28000000 mWh (28 kWh to reach 80%)
effectiveConsumptionLimit = 3700000 mW (3.7 kW limit)
Electrical.nominalMaxConsumption = 7400000 mW (7.4 kW EV max)

Result: Charge at 3.7 kW (limited by controller)
        EV will take longer to reach target
        Plan feature should reflect actual expected delivery
```

### 4.4 V2G Discharge Constraints

When V2G discharge is requested but EV constraints prevent it:

**Rule: EV constraints are absolute; controller cannot override EV limits**

```
Controller requests: productionSetpoint = 5000000 mW (5 kW discharge)
EV reports: evMaxDischargingRequest = 3000000 mWh (can only discharge 3 kWh total)
EV reports: evDischargeBelowTargetPermitted = false

Result: Device limits discharge to respect EV constraints
        Plan feature should reflect actual discharge capability
```

---

## 5. Process + Control State Interaction

### 5.1 Orthogonal State Machines

ControlStateEnum and ProcessStateEnum are **orthogonal** - they track different concerns:

- **ControlStateEnum**: Relationship with external controller
- **ProcessStateEnum**: Internal task lifecycle

A device can be in any combination of (ControlState, ProcessState).

### 5.2 Process Behavior During FAILSAFE

When connection is lost (ControlState → FAILSAFE) and a process is active:

| ProcessState | Behavior | Rationale |
|--------------|----------|-----------|
| NONE | No change | No process to affect |
| AVAILABLE | No change | Process not started |
| SCHEDULED | Process starts at scheduled time | User intent preserved |
| RUNNING | Process continues | Safety: don't interrupt mid-task |
| PAUSED | Implementation-defined (PICS) | See below |
| COMPLETED | No change | Already finished |
| ABORTED | No change | Already aborted |

**PAUSED during FAILSAFE:** Behavior depends on PICS item `MASH.S.CTRL.B_PAUSED_AUTO_RESUME`:
- If true: Device automatically resumes after failsafeDuration expires
- If false: Device remains paused until new controller connects

### 5.3 Process Behavior During OVERRIDE

When device enters OVERRIDE (self-protection):

| ProcessState | Behavior |
|--------------|----------|
| RUNNING | May continue or pause depending on override reason |
| SCHEDULED | Schedule preserved; start may be delayed |

The override reason determines process handling:
- **Thermal protection**: Process likely pauses
- **Grid frequency event**: Process may continue at reduced power
- **User safety override**: Process stops

### 5.4 New Controller Connects with Running Process

When a new controller connects and a process is already running:

1. Controller receives processState = RUNNING in initial attribute read
2. Controller receives optionalProcess with current process details
3. Controller MAY issue CancelProcess if it disagrees with the running process
4. Controller MAY issue Pause if it wants to temporarily stop
5. Controller cannot "take over" an existing process - it was scheduled by the previous controller or device autonomy

---

## 6. Plan Feature Interactions

### 6.1 Plan Reflects All Constraints

The Plan feature (device's intended behavior) MUST reflect all active constraints:

```
Plan output considers:
  - Electrical limits (hardware envelope)
  - EnergyControl limits (all zones, most restrictive)
  - Signals constraints (active slot)
  - ChargingSession mode (optimization strategy)
  - EV demands (target, departure time)
  - Process state (if OHPCF-style)
```

### 6.2 Plan Update Triggers

Device SHOULD update Plan when any of these change:
- EnergyControl limits change
- Signals slot boundary reached
- Electrical capabilities change (EV connect/disconnect)
- ChargingMode changes
- Process state changes
- EV demands update (ISO 15118 renegotiation)

---

## 7. Test Cases

### TC-FI-001: Signals + EnergyControl Limit Stacking

**Preconditions:**
- Device with SIGNALS and CORE flags
- EnergyControl.effectiveConsumptionLimit = 7400000 mW
- Signal with maxPower = 5000000 mW active

**Expected:**
- Device operates at max 5000000 mW
- Subscription reports effectiveConsumptionLimit accounting for both

### TC-FI-002: Signal Slot Boundary Transition

**Preconditions:**
- Active signal with slots: [5000W for 1h, 10000W for 1h]
- Device operating at 5000W

**Test:**
- Wait for slot boundary

**Expected:**
- Within 1 second of boundary, device may operate up to 10000W
- Subscription notification sent within minInterval

### TC-FI-003: Signal Expiry Returns to EnergyControl

**Preconditions:**
- EnergyControl.effectiveConsumptionLimit = 7400000 mW
- Signal with single slot maxPower = 3700000 mW, duration 60s

**Test:**
- Wait 60+ seconds for signal expiry

**Expected:**
- After expiry, effectiveConsumptionLimit returns to 7400000 mW
- Subscription notification sent

### TC-FI-004: Electrical Update Caps Effective Limit

**Preconditions:**
- EVSE with 22kW hardware
- EnergyControl.myConsumptionLimit = 11000000 mW
- No EV connected

**Test:**
- Connect 7.4kW EV

**Expected:**
- Electrical.nominalMaxConsumption updates to 7400000 mW
- EnergyControl.myConsumptionLimit unchanged at 11000000 mW
- EnergyControl.effectiveConsumptionLimit = 7400000 mW (capped)

### TC-FI-005: ChargingMode PV_SURPLUS_ONLY with Limit

**Preconditions:**
- chargingMode = PV_SURPLUS_ONLY
- effectiveConsumptionLimit = 7400000 mW
- No PV surplus available

**Test:**
- Read charging state

**Expected:**
- state = PLUGGED_IN_DEMAND (not charging)
- Device waits for PV surplus despite limit allowing charging

### TC-FI-006: Pause Command Overrides stopDelay

**Preconditions:**
- EV charging at 7.4kW
- stopDelay = 120s

**Test:**
- Send Pause command

**Expected:**
- Device stops charging (EVSE may enforce brief safety delay, but not full 120s)
- state transitions to PLUGGED_IN_DEMAND or equivalent

### TC-FI-007: Process Continues During FAILSAFE

**Preconditions:**
- processState = RUNNING
- controlState = CONTROLLED

**Test:**
- Disconnect controller (simulate connection loss)

**Expected:**
- controlState transitions to FAILSAFE
- processState remains RUNNING
- Process continues until completion or failsafeDuration expires

### TC-FI-008: Multi-Zone Signal Priority

**Preconditions:**
- Zone 1 (GRID): Signal maxPower = 3000000 mW
- Zone 2 (LOCAL): Signal maxPower = 5000000 mW

**Test:**
- Read effective signal constraint

**Expected:**
- Active signal is Zone 1's (GRID priority wins)
- Effective constraint from signals = 3000000 mW

### TC-FI-009: EnergyControl Limit vs Signal - Both Apply

**Preconditions:**
- Zone 1 (GRID): EnergyControl.myConsumptionLimit = 6000000 mW
- Zone 2 (LOCAL): Signal maxPower = 4000000 mW

**Test:**
- Calculate effective operating envelope

**Expected:**
- Effective limit = min(6000000, 4000000) = 4000000 mW
- Both constraints apply (EnergyControl from all zones, Signal from highest priority zone)

### TC-FI-010: V2G Discharge Constrained by EV

**Preconditions:**
- Controller: productionSetpoint = 5000000 mW
- EV: evMaxDischargingRequest = 2000000 mWh (energy limit)
- Current SoC allows discharge

**Test:**
- Monitor discharge behavior

**Expected:**
- Device respects EV energy limit
- Actual discharge may be less than setpoint
- Plan reflects actual discharge capability

### TC-FI-011: Electrical Increase Restores Original Limit

**Preconditions:**
- EV with 7.4kW max connected
- myConsumptionLimit = 11000000 mW
- effectiveConsumptionLimit = 7400000 mW (capped)

**Test:**
- EV disconnects

**Expected:**
- Electrical.nominalMaxConsumption returns to EVSE hardware (e.g., 22kW)
- effectiveConsumptionLimit = 11000000 mW (controller's original limit now applies)

### TC-FI-012: SCHEDULED Process Starts During FAILSAFE

**Preconditions:**
- processState = SCHEDULED
- scheduledStart = T+30s
- controlState = CONTROLLED

**Test:**
- At T+10s: Disconnect controller
- Wait until T+35s

**Expected:**
- controlState = FAILSAFE
- processState transitions SCHEDULED → RUNNING at scheduled time
- Process runs despite FAILSAFE state

### TC-FI-013: Signal Slot with No Limit Falls Through

**Preconditions:**
- Signal with slot: { duration: 3600, price: 1500 } // No maxPower
- EnergyControl.effectiveConsumptionLimit = 7400000 mW

**Test:**
- Read effective operating limit

**Expected:**
- Effective limit = 7400000 mW (EnergyControl)
- Signal slot has no maxPower, so doesn't constrain

---

## Related Documents

| Document | Description |
|----------|-------------|
| [EnergyControl](../../features/energy-control.md) | Limit/setpoint commands |
| [Signals](../../features/signals.md) | Time-slotted signals |
| [ChargingSession](../../features/charging-session.md) | EV charging session |
| [Plan](../../features/plan.md) | Device's intended behavior |
| [Conformance Rules](../../conformance/README.md) | Feature requirements |
