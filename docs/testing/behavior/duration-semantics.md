# Duration Timer Semantics

> Behavior specification for command duration parameters

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Purpose

This document specifies the exact behavior of the `duration` parameter available on control commands (SetLimit, SetCurrentLimits, SetSetpoint, SetCurrentSetpoints, Pause). Clear semantics are essential for controllers to predict device behavior.

---

## 1. Overview

### 1.1 Commands with Duration

| Command | Duration Parameter | Default |
|---------|-------------------|---------|
| SetLimit | duration (uint32 s) | 0 (indefinite) |
| SetCurrentLimits | duration (uint32 s) | 0 (indefinite) |
| SetSetpoint | duration (uint32 s) | 0 (indefinite) |
| SetCurrentSetpoints | duration (uint32 s) | 0 (indefinite) |
| Pause | duration (uint32 s) | 0 (indefinite) |

### 1.2 Design Principles

1. **Timer starts on receipt** - Predictable behavior, no ambiguity
2. **Auto-clear on expiry** - Device returns to unconstrained state for that zone
3. **No persistence** - Timers do not survive connection loss or device restart
4. **Per-zone timers** - Each zone's duration is tracked independently
5. **Notification on expiry** - Controllers are informed when values change

---

## 2. Timer Lifecycle

### 2.1 Timer Start

**Rule: Timer starts when device receives the command**

The timer begins when the command message is fully received by the device, NOT when the response is sent. This ensures consistent behavior regardless of response latency.

```
Timeline:
  T+0ms:   Controller sends SetLimit(limit=5000, duration=60)
  T+5ms:   Device receives command → Timer starts (60 seconds)
  T+10ms:  Device sends response
  T+20ms:  Controller receives response
  T+60005ms: Timer expires (60 seconds from T+5ms)
```

### 2.2 Timer Tracking

The device MUST track timers per zone and per command type:

```
Timer Key = (ZoneId, CommandType)

Where CommandType = {
  LIMIT_CONSUMPTION,
  LIMIT_PRODUCTION,
  CURRENT_LIMIT_CONSUMPTION,
  CURRENT_LIMIT_PRODUCTION,
  SETPOINT_CONSUMPTION,
  SETPOINT_PRODUCTION,
  CURRENT_SETPOINT_CONSUMPTION,
  CURRENT_SETPOINT_PRODUCTION,
  PAUSE
}
```

### 2.3 Timer Expiry

When a duration timer expires:

1. Device internally clears the value for that zone and command type
2. Device recalculates effective values (may change if other zones still have values)
3. Device sends subscription notification if effective values changed
4. Device continues normal operation

**Expiry behavior by command:**

| Command | On Expiry |
|---------|-----------|
| SetLimit | Zone's limit is cleared (as if ClearLimit called) |
| SetCurrentLimits | Zone's current limits are cleared |
| SetSetpoint | Zone's setpoint is cleared (as if ClearSetpoint called) |
| SetCurrentSetpoints | Zone's current setpoints are cleared |
| Pause | Device resumes (as if Resume called) |

### 2.4 Timer Replacement

A new command with duration REPLACES any existing timer for the same (ZoneId, CommandType):

```
Example:
  T+0s:   Zone 1 SetLimit(limit=5000, duration=60) → Timer set for T+60s
  T+30s:  Zone 1 SetLimit(limit=3000, duration=90) → Timer replaced, now T+120s
  T+120s: Timer expires, Zone 1 limit cleared
```

### 2.5 Indefinite Duration

When duration = 0 (or omitted), the value persists until:
- Explicitly cleared (ClearLimit, ClearSetpoint, Resume)
- Zone is removed
- Connection is lost (see Section 3)
- Device restarts

---

## 3. Connection Loss Behavior

### 3.1 Timer State on Disconnect

**Rule: Duration timers are NOT persisted across connection loss**

When a zone's connection is lost:

1. All pending duration timers for that zone are cancelled
2. Zone's values remain in effect (for failsafe period)
3. On reconnect, controller must re-establish any timed constraints

**Rationale:** Duration timers represent controller intent for active control. If the controller disconnects, it cannot react to expiry. The failsafe mechanism handles the disconnect case separately.

### 3.2 Interaction with Failsafe

Connection loss triggers FAILSAFE state, which has its own timing:

```
Timeline:
  T+0s:   Zone 1 active, SetLimit(5000, duration=120) timer running
  T+30s:  Connection lost
          → Duration timer cancelled
          → Device enters FAILSAFE
          → failsafeConsumptionLimit applied
  T+(30+failsafeDuration)s: FAILSAFE expires → AUTONOMOUS
```

The duration timer does NOT determine when FAILSAFE ends - that's controlled by `failsafeDuration`.

---

## 4. Multi-Zone Timer Handling

### 4.1 Independent Timers

Each zone's timers are completely independent:

```
Example:
  T+0s:   Zone 1 SetLimit(5000, duration=60)
  T+10s:  Zone 2 SetLimit(3000, duration=30)
  T+40s:  Zone 2 timer expires → Zone 2 limit cleared
          effectiveLimit = 5000 (Zone 1 only)
  T+60s:  Zone 1 timer expires → Zone 1 limit cleared
          effectiveLimit = unlimited (no zones have limits)
```

### 4.2 Effective Value Recalculation

On any timer expiry:

1. Remove expired zone's contribution
2. Recalculate effective value from remaining zones
3. Notify subscribers if effective value changed

```
Example with most-restrictive-wins:
  Zone 1 limit: 5000 mW (no duration - indefinite)
  Zone 2 limit: 3000 mW (duration: 60s)
  effectiveLimit = min(5000, 3000) = 3000 mW

  After Zone 2 expires:
  effectiveLimit = 5000 mW (only Zone 1 remains)
```

---

## 5. Subscription Notifications

### 5.1 Expiry Notification

When a duration timer expires and causes effective values to change:

1. Notification sent within normal subscription timing (minInterval/maxInterval)
2. Notification includes updated effective values
3. Notification includes unchanged my* values (other zones)

### 5.2 Notification Content

The notification does NOT explicitly indicate "timer expired." Controllers infer this from:
- `effectiveConsumptionLimit` changed
- `myConsumptionLimit` unchanged (controller's value wasn't cleared by command)

If the controller that set the duration receives a notification showing their `my*` value cleared, they know their timer expired.

---

## 6. Timer Accuracy

### 6.1 Accuracy Requirements

- Timer accuracy: +/- 1% or +/- 1 second, whichever is greater
- Minimum duration: 1 second
- Maximum duration: 86400 seconds (24 hours)

### 6.2 Clock Considerations

Devices SHOULD use monotonic time for duration tracking, not wall-clock time. This prevents issues with:
- NTP time adjustments
- Daylight saving transitions
- Clock drift

---

## 7. Implementation Notes

### 7.1 Suggested Data Structure

```
struct DurationTimer {
    zoneId: ZoneId,
    commandType: CommandType,
    startTime: MonotonicTime,
    duration: uint32,
    value: CommandValue
}

class TimerManager {
    timers: Map<(ZoneId, CommandType), DurationTimer>

    func setTimer(zone, cmd, duration, value):
        key = (zone, cmd)
        timers[key] = DurationTimer{
            zoneId: zone,
            commandType: cmd,
            startTime: now(),
            duration: duration,
            value: value
        }

    func checkExpired():
        for timer in timers.values():
            if now() >= timer.startTime + timer.duration:
                expireTimer(timer)

    func expireTimer(timer):
        timers.remove((timer.zoneId, timer.commandType))
        recalculateEffectiveValues()
        notifySubscribers()

    func onConnectionLost(zoneId):
        for key in timers.keys():
            if key.zoneId == zoneId:
                timers.remove(key)
}
```

### 7.2 PICS Items

| PICS Code | Description | Values |
|-----------|-------------|--------|
| MASH.S.CTRL.B_DURATION_EXPIRY | Expiry behavior | "clear" (always) |
| MASH.S.CTRL.B_DURATION_MAX | Maximum supported duration | 86400 |
| MASH.S.CTRL.B_DURATION_ACCURACY | Timer accuracy | "1%" |

---

## 8. Test Cases

### TC-DUR-001: Timer Starts on Receipt

**Preconditions:**
- Device in CONTROLLED state
- Zone 1 connected

**Test Steps:**
1. Record current time T0
2. Send SetLimit(consumptionLimit: 5000000, duration: 60)
3. Note response time T1
4. Wait until T0 + 59 seconds
5. Read effectiveConsumptionLimit

**Expected:**
- At T0 + 59s: effectiveConsumptionLimit = 5000000 (still active)
- Shortly after T0 + 60s: effectiveConsumptionLimit changes (timer expired based on receipt time, not response time)

### TC-DUR-002: Timer Expiry Clears Value

**Preconditions:**
- Device in CONTROLLED state
- No other zones connected

**Test Steps:**
1. Send SetLimit(consumptionLimit: 5000000, duration: 5)
2. Wait 6 seconds
3. Read effectiveConsumptionLimit

**Expected:**
- effectiveConsumptionLimit = unlimited (or device max)
- Subscription notification received showing change

### TC-DUR-003: Timer Replacement

**Preconditions:**
- Device in CONTROLLED state

**Test Steps:**
1. Send SetLimit(consumptionLimit: 5000000, duration: 60)
2. Wait 30 seconds
3. Send SetLimit(consumptionLimit: 3000000, duration: 60)
4. Wait 35 seconds (65 from first command)
5. Read effectiveConsumptionLimit

**Expected:**
- At step 5: effectiveConsumptionLimit = 3000000 (second timer hasn't expired yet)
- Original 60s timer was replaced, not still counting

### TC-DUR-004: Multi-Zone Independent Timers

**Preconditions:**
- Zone 1 and Zone 2 connected

**Test Steps:**
1. Zone 1: SetLimit(consumptionLimit: 5000000, duration: 60)
2. Zone 2: SetLimit(consumptionLimit: 3000000, duration: 30)
3. Wait 35 seconds

**Expected:**
- At step 3: Zone 2 timer expired
- effectiveConsumptionLimit = 5000000 (only Zone 1 limit remains)

### TC-DUR-005: Connection Loss Cancels Timers

**Preconditions:**
- Zone 1 connected
- SetLimit(consumptionLimit: 5000000, duration: 300) active

**Test Steps:**
1. Disconnect Zone 1
2. Wait for failsafe
3. Reconnect Zone 1
4. Read myConsumptionLimit

**Expected:**
- myConsumptionLimit = null/unlimited (timer was cancelled on disconnect)
- Controller must re-establish limit if desired

### TC-DUR-006: Indefinite Duration Persists

**Preconditions:**
- Device in CONTROLLED state

**Test Steps:**
1. Send SetLimit(consumptionLimit: 5000000) // duration omitted = 0 = indefinite
2. Wait 120 seconds
3. Read effectiveConsumptionLimit

**Expected:**
- effectiveConsumptionLimit = 5000000 (still active after 120s)
- Value persists until ClearLimit

### TC-DUR-007: Pause Duration Auto-Resume

**Preconditions:**
- Device pausable (isPausable = true)
- Device actively operating

**Test Steps:**
1. Send Pause(duration: 10)
2. Verify device paused
3. Wait 12 seconds
4. Read device state

**Expected:**
- After ~10 seconds: device automatically resumes
- No explicit Resume command needed

### TC-DUR-008: Expiry Notification Timing

**Preconditions:**
- Subscription active with minInterval=1, maxInterval=60
- SetLimit(consumptionLimit: 5000000, duration: 5) sent

**Test Steps:**
1. Monitor subscription notifications
2. Wait for timer expiry

**Expected:**
- Notification received within minInterval after expiry
- Notification shows effectiveConsumptionLimit changed

### TC-DUR-009: Duration with Setpoint Resolution

**Preconditions:**
- Zone 1 (priority 1) and Zone 2 (priority 2) connected

**Test Steps:**
1. Zone 1: SetSetpoint(consumptionSetpoint: 3000000, duration: 30)
2. Zone 2: SetSetpoint(consumptionSetpoint: 5000000) // indefinite
3. Wait 35 seconds

**Expected:**
- Initially: effectiveSetpoint = 3000000 (Zone 1 priority wins)
- After Zone 1 expires: effectiveSetpoint = 5000000 (Zone 2 now active)

### TC-DUR-010: Duration Accuracy

**Preconditions:**
- Device in CONTROLLED state

**Test Steps:**
1. Send SetLimit(duration: 60)
2. Monitor for expiry
3. Record actual expiry time

**Expected:**
- Expiry within 60s +/- 1% (59.4s to 60.6s) or +/- 1s (59s to 61s)

---

## Related Documents

| Document | Description |
|----------|-------------|
| [EnergyControl](../../features/energy-control.md) | Command definitions |
| [Multi-Zone Resolution](multi-zone-resolution.md) | Limit/setpoint resolution |
| [Feature Interactions](feature-interactions.md) | Cross-feature behavior |
| [Transport](../../transport.md) | Connection handling |
