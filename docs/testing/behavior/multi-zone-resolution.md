# Multi-Zone Resolution Behavior

> Precise specification of how multiple zones interact

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

When multiple zones set limits or setpoints on the same device, the device must resolve these to a single effective value. This document specifies the exact resolution rules.

---

## 2. Limit Resolution

### 2.1 Core Rule: Most Restrictive Wins

**For consumption limits (charging/consuming):**
```
effectiveConsumptionLimit = min(all zone consumption limits that are set)
```

**For production limits (discharging/producing):**
```
effectiveProductionLimit = min(all zone production limits that are set)
```

### 2.2 Zone Limit Tracking

Device MUST maintain internal state:
```
zoneLimits: map[ZoneID] → {
  consumptionLimit: int64 | null,  // mW, null = not set
  productionLimit: int64 | null,   // mW, null = not set
  consumptionExpiry: timestamp | null,
  productionExpiry: timestamp | null
}
```

### 2.3 Limit Lifecycle

#### Command Parameters vs Attributes

**Important distinction:**

| Type | Example | Behavior |
|------|---------|----------|
| **Attribute** | `myConsumptionLimit` | Stored, readable, subscribable |
| **Command parameter** | `duration` | Not stored, not readable, controls behavior |

The `duration` parameter is NOT an attribute. It:
- Is not stored as a readable value
- Cannot be subscribed to
- Cannot be "deleted" - only the resulting limit can be cleared
- Starts an internal timer that is not exposed

#### Setting a Limit

When zone Z calls `SetLimit(consumptionLimit: X, duration: D, cause: C)`:

1. Store: `zoneLimits[Z].consumptionLimit = X`
2. If D > 0: Start timer, `zoneLimits[Z].consumptionExpiry = now + D`
3. If D == 0 or D omitted: `zoneLimits[Z].consumptionExpiry = null` (indefinite)
4. Recalculate effective limit (see 2.4)
5. Return: `{success: true, effectiveConsumptionLimit: <calculated>}`

**Omitting optional parameters:**

| Parameter | If Omitted | Meaning |
|-----------|------------|---------|
| `consumptionLimit` | No change to consumption limit | Only change production if specified |
| `productionLimit` | No change to production limit | Only change consumption if specified |
| `duration` | Indefinite | Same as `duration: 0` |
| `cause` | Required | Must always specify |

#### Replacing a Limit (Duration Change)

**To change from timed to indefinite (remove timer):**

```
// Initial: limit with 60s duration
SetLimit(consumptionLimit: 5000000, duration: 60, cause: GRID_OPTIMIZATION)

// 30 seconds later: remove the timer, make indefinite
SetLimit(consumptionLimit: 5000000, duration: 0, cause: GRID_OPTIMIZATION)
// OR
SetLimit(consumptionLimit: 5000000, cause: GRID_OPTIMIZATION)  // duration omitted = indefinite
```

**To change from indefinite to timed (add timer):**

```
// Initial: indefinite limit
SetLimit(consumptionLimit: 5000000, cause: GRID_OPTIMIZATION)

// Later: add 60s timer
SetLimit(consumptionLimit: 5000000, duration: 60, cause: GRID_OPTIMIZATION)
```

**To extend/shorten a timer:**

```
// Initial: 60s timer
SetLimit(consumptionLimit: 5000000, duration: 60, cause: GRID_OPTIMIZATION)

// 30s later: extend to 120s from now (not 90s remaining!)
SetLimit(consumptionLimit: 5000000, duration: 120, cause: GRID_OPTIMIZATION)
```

**Each SetLimit REPLACES the previous limit entirely:**
- New value (or same value if unchanged)
- New timer starting NOW (or indefinite if duration omitted/0)
- Previous timer is cancelled

**There is no:**
- "Update duration only" operation
- "Extend by N seconds" operation
- "Query remaining time" attribute

**Design rationale:** Duration is transient control, not persistent state. The controller that set the limit knows when it expires (it set the duration). Other zones don't need to know - they only care about the effective limit value, not when it might change.

**If remaining time is needed:** Controller should track expiry locally:
```python
# Controller-side tracking
expiry_time = time.now() + duration
# Later
remaining = expiry_time - time.now()
```

#### Clearing a Limit

When zone Z calls `ClearLimit()`:

1. Store: `zoneLimits[Z].consumptionLimit = null`
2. Store: `zoneLimits[Z].productionLimit = null`
3. Store: `zoneLimits[Z].consumptionExpiry = null`
4. Store: `zoneLimits[Z].productionExpiry = null`
5. Recalculate effective limit (see 2.4)
6. Return: `{success: true}`

#### Duration Expiry

When a limit's expiry time is reached:

1. Store: `zoneLimits[Z].consumptionLimit = null` (or productionLimit)
2. Recalculate effective limit (see 2.4)
3. Send notification to all subscribed zones (effectiveConsumptionLimit changed)

### 2.4 Effective Limit Calculation

```python
def calculate_effective_consumption_limit():
    active_limits = [
        zoneLimits[z].consumptionLimit
        for z in zones
        if zoneLimits[z].consumptionLimit is not None
    ]

    if len(active_limits) == 0:
        return None  # No limit - device operates freely
    else:
        return min(active_limits)
```

**Critical:** When no zones have active limits, `effectiveConsumptionLimit = null` (not 0, not unlimited constant). This means "no external limit" - device uses its own internal limits (from Electrical feature).

### 2.5 My Limit vs Effective Limit

Each zone sees two values:

| Attribute | Meaning | Scope |
|-----------|---------|-------|
| `myConsumptionLimit` | The limit THIS zone set | Zone-specific |
| `effectiveConsumptionLimit` | The actual limit being applied | Same for all zones |

When reading `myConsumptionLimit`:
- Return the value zone Z most recently set via SetLimit
- Return `null` if zone Z has not set a limit or has cleared it

### 2.6 Limit Active Indicator

A zone can determine if its limit is the effective one:

```
limitActive = (myConsumptionLimit == effectiveConsumptionLimit)
              AND (myConsumptionLimit != null)
```

This is NOT an attribute - zones calculate it from the two limit values.

---

## 3. Setpoint Resolution

### 3.1 Core Rule: Highest Priority Wins

Unlike limits, setpoints are mutually exclusive - only one zone's setpoint is active.

**Resolution:**
```
effectiveConsumptionSetpoint = setpoint from zone with highest priority
                               (lowest priority number)
```

### 3.2 Priority Order

| Zone Type | Priority | Wins Against |
|-----------|----------|--------------|
| GRID | 1 | LOCAL |
| LOCAL | 2 | None |

### 3.3 Same Priority Tie-Breaking

If two zones have the same priority (e.g., two LOCAL zones):

**Rule:** Most recently set setpoint wins.

```
Zone 1 (LOCAL): SetSetpoint(5000000) at T=100
Zone 2 (LOCAL): SetSetpoint(3000000) at T=200

effectiveConsumptionSetpoint = 3000000 (Zone 2, more recent)
```

### 3.4 Setpoint Clearing

When a zone clears its setpoint:

1. Remove zone's setpoint from tracking
2. Find next highest priority zone with active setpoint
3. That zone's setpoint becomes effective
4. If no zones have setpoints, `effectiveConsumptionSetpoint = null`

**Example:**
```
Zone 1 (GRID): 3000000 → effective
Zone 2 (LOCAL):  5000000 → inactive

Zone 1 clears setpoint.

Zone 2 (LOCAL):  5000000 → now effective
```

### 3.5 Setpoint vs Limit Interaction

Limits constrain setpoints. The device targets:

```
actual_target = min(effectiveConsumptionSetpoint, effectiveConsumptionLimit)
```

If `effectiveConsumptionSetpoint = 7000000` and `effectiveConsumptionLimit = 5000000`:
- Device targets 5000000 (limit caps the setpoint)
- `effectiveConsumptionSetpoint` still reports 7000000 (the requested setpoint)
- Device behavior: operate at limit, not setpoint

---

## 4. Per-Phase Current Resolution

### 4.1 Phase Limit Resolution

Per-phase current limits follow the same "most restrictive wins" rule, applied per-phase:

```
effectiveCurrentLimitsConsumption[phase] = min(
    zoneLimits[z].currentLimitsConsumption[phase]
    for z in zones
    if phase in zoneLimits[z].currentLimitsConsumption
)
```

### 4.2 Partial Phase Specification

When a zone specifies only some phases:

**Rule:** Unspecified phases are not constrained by that zone.

```
Zone 1: SetCurrentLimits({A: 16000, B: 16000, C: 16000})
Zone 2: SetCurrentLimits({A: 10000})  // Only phase A

effective = {
  A: min(16000, 10000) = 10000,  // Both zones contribute
  B: 16000,                       // Only Zone 1
  C: 16000                        // Only Zone 1
}
```

### 4.3 Clearing Phase Limits

`ClearCurrentLimits()` clears ALL phases for that zone.

To clear only specific phases, use `SetCurrentLimits({A: null, B: null})` with explicit nulls.

---

## 5. Notification Behavior

### 5.1 When to Notify

Device MUST send subscription notifications when:

1. `effectiveConsumptionLimit` changes (including null → value or value → null)
2. `effectiveProductionLimit` changes
3. `effectiveConsumptionSetpoint` changes
4. `effectiveProductionSetpoint` changes
5. `effectiveCurrentLimits*` changes (any phase)
6. `effectiveCurrentSetpoints*` changes (any phase)

### 5.2 Notification Content

Notification contains only changed attributes:

```cbor
{
  1: 0,                    // messageId 0 = notification
  2: 5001,                 // subscriptionId
  3: 1,                    // endpointId
  4: 3,                    // featureId (EnergyControl)
  5: {                     // changed attributes only
    20: 5000000            // effectiveConsumptionLimit changed
  }
}
```

### 5.3 Coalescing

If multiple changes occur within subscription's `minInterval`, device MAY coalesce into single notification containing all changed attributes.

---

## 6. Edge Cases

### 6.1 All Zones Clear Limits

When the last zone clears its limit:
- `effectiveConsumptionLimit = null`
- Device operates according to its internal limits (Electrical.nominalMaxConsumption)

### 6.2 Zone Disconnects

When a zone's connection is lost:
- Zone's limits/setpoints remain active until:
  - Their duration expires, OR
  - The zone reconnects and clears them, OR
  - Another zone with sufficient priority clears them

**Rationale:** Grid operator limits must persist even if connection is temporarily lost.

### 6.3 Negative Limit Values

Negative values for consumptionLimit are INVALID. Device MUST return error:
- Status: INVALID_PARAMETER
- Message: "consumptionLimit must be >= 0"

Production limits follow the same rule.

### 6.4 Zero Limit Value

`consumptionLimit = 0` is VALID. It means "do not consume any power."
- Device MUST stop consumption (not charging, not heating, etc.)
- Device MAY continue essential functions (communication, safety systems)

### 6.5 Limit Exceeds Electrical Capacity

If zone sets limit higher than device can achieve:
- Device accepts the limit (no error)
- Device operates at its actual maximum
- `effectiveConsumptionLimit` reports the set value, not the actual capability

**Rationale:** Zone doesn't need to know device's exact capability to set a limit.

---

## 7. PICS Items

```
# Limit behavior
MASH.S.CTRL.B_LIMIT_PERSIST_DISCONNECT=1  # Limits persist when zone disconnects
MASH.S.CTRL.B_LIMIT_CLEAR_ALL_NULL=1      # effectiveLimit=null when all cleared

# Setpoint behavior
MASH.S.CTRL.B_SETPOINT_TIEBREAK_RECENT=1  # Same-priority: most recent wins

# Phase limit behavior
MASH.S.CTRL.B_PHASE_PARTIAL_OK=1          # Partial phase specification allowed
```

---

## 8. Test Cases

See:
- `TC-ZONE-LIMIT-*`: Limit resolution tests
- `TC-ZONE-SETPOINT-*`: Setpoint resolution tests
- `TC-ZONE-PHASE-*`: Per-phase resolution tests
- `TC-ZONE-EDGE-*`: Edge case tests
