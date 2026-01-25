# State Machine Behavior

> Precise specification of ControlState and ProcessState interactions

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

MASH devices have two orthogonal state machines:

- **ControlStateEnum**: Relationship with external controller(s)
- **ProcessStateEnum**: Lifecycle of optional tasks (e.g., heat pump compressor)

While these are logically independent, certain events (connection loss, commands) affect both. This document specifies the precise interaction rules.

---

## 2. ControlStateEnum Transitions

### 2.1 State Definitions

| State | Meaning | Entry Condition |
|-------|---------|-----------------|
| AUTONOMOUS | No external control | Initial state; failsafeDuration expires |
| CONTROLLED | Controller connected, no active limit | Controller connects; all limits cleared |
| LIMITED | Active limit being applied | SetLimit() called with value |
| FAILSAFE | Connection lost, using failsafe limits | Connection loss detected |
| OVERRIDE | Device overriding for safety/legal | Device self-protection triggered |

### 2.2 State Transition Rules

```
AUTONOMOUS → CONTROLLED    : First controller connects (commissioning complete)
AUTONOMOUS → FAILSAFE      : [INVALID - no connection to lose]
AUTONOMOUS → LIMITED       : [INVALID - must be CONTROLLED first]
AUTONOMOUS → OVERRIDE      : [INVALID - no limit to override]

CONTROLLED → AUTONOMOUS    : [INVALID - must go through FAILSAFE]
CONTROLLED → LIMITED       : SetLimit() with non-null limit value
CONTROLLED → FAILSAFE      : Connection loss detected
CONTROLLED → OVERRIDE      : [INVALID - no limit to override]

LIMITED → AUTONOMOUS       : [INVALID - must go through FAILSAFE]
LIMITED → CONTROLLED       : ClearLimit() AND no other zone has limits
LIMITED → FAILSAFE         : Connection loss detected
LIMITED → OVERRIDE         : Device self-protection triggered

FAILSAFE → AUTONOMOUS      : failsafeDuration expires
FAILSAFE → CONTROLLED      : Controller reconnects AND no limits were active
FAILSAFE → LIMITED         : Controller reconnects AND limits restore
FAILSAFE → OVERRIDE        : Device self-protection triggered

OVERRIDE → AUTONOMOUS      : [INVALID - must go through FAILSAFE]
OVERRIDE → CONTROLLED      : Override condition cleared AND no limits active
OVERRIDE → LIMITED         : Override condition cleared AND limits still apply
OVERRIDE → FAILSAFE        : Connection loss detected
```

### 2.3 Connection Loss Detection

**Detection mechanism:**
1. TCP layer detects connection loss (FIN, RST, or timeout)
2. TLS layer failure (handshake failure on reconnect)
3. Application ping/pong timeout (3 consecutive missed pongs)

**Timing requirements:**

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Ping interval | 30 seconds | Balance between overhead and detection speed |
| Pong timeout | 5 seconds | Allow for network latency |
| Missed pongs before FAILSAFE | 3 | Avoid false positives |
| **Maximum detection delay** | **95 seconds** | 3 * 30s + 5s |
| Recommended detection delay | 35 seconds | TCP keepalive + 1 missed pong |

**Device MUST:**
- Enter FAILSAFE within 95 seconds of actual connection loss
- Apply failsafe limits immediately upon entering FAILSAFE (not gradually)

### 2.4 FAILSAFE Behavior

Upon entering FAILSAFE:

1. **Immediately apply:**
   - `failsafeConsumptionLimit` as `effectiveConsumptionLimit`
   - `failsafeProductionLimit` as `effectiveProductionLimit`
   - Per-phase limits remain unchanged (per-phase failsafe not specified)

2. **Start failsafe timer:**
   - Timer duration: `failsafeDuration` (configured, 2-24 hours)
   - Timer accuracy: +/- 1% (e.g., 24h +/- 14 minutes)
   - Timer MUST survive device restart during FAILSAFE

3. **Preserve zone limit state:**
   - Zone limits remain stored (not cleared)
   - Will be used to calculate effective limits on reconnect

4. **Continue operation:**
   - Device continues operating within failsafe limits
   - Device MAY continue optional processes (see Section 4)

### 2.5 FAILSAFE → AUTONOMOUS Transition

When failsafeDuration expires:

1. Clear all zone limits (they are considered stale)
2. Set `effectiveConsumptionLimit = null` (no external limit)
3. Set `effectiveProductionLimit = null`
4. Transition controlState to AUTONOMOUS
5. Device operates according to internal logic only

### 2.6 Reconnection During FAILSAFE

When controller reconnects during FAILSAFE:

1. **If reconnection is within failsafeDuration:**
   - Cancel failsafe timer
   - Restore zone limits from stored state
   - Recalculate effective limits
   - Transition to LIMITED if any limits exist, else CONTROLLED

2. **If reconnection is after failsafeDuration (device is AUTONOMOUS):**
   - Treat as new controller connection
   - Zone must re-issue any limits
   - Transition to CONTROLLED (no limits) until SetLimit() called

### 2.7 OVERRIDE State

Device enters OVERRIDE when it must ignore external limits for:
- Safety (overheating protection)
- Legal requirements (minimum EV charging for departure)
- Self-protection (battery SoC limits)

**OVERRIDE rules:**
- Device MUST log reason for override
- Device SHOULD notify subscribed zones
- Device operates according to internal logic, ignoring effective limits
- External limits remain stored, will be re-applied when override clears

**Override clearing:**
- When override condition is no longer present
- Device automatically transitions back to LIMITED (if limits exist) or CONTROLLED

---

## 3. ProcessStateEnum Transitions

### 3.1 State Definitions

| State | Meaning | Entry Condition |
|-------|---------|-----------------|
| NONE | No optional process available | Default; process completed/cancelled |
| AVAILABLE | Process announced, awaiting scheduling | Device announces new process |
| SCHEDULED | Start time set, waiting | ScheduleProcess() accepted |
| RUNNING | Process currently executing | Scheduled time reached; or immediate start |
| PAUSED | Paused by controller | Pause() command |
| COMPLETED | Finished successfully | Process natural completion |
| ABORTED | Stopped before completion | Stop() or CancelProcess() command |

### 3.2 State Transition Rules

```
NONE → AVAILABLE           : Device announces new optional process
NONE → SCHEDULED           : [INVALID - must be AVAILABLE first]
NONE → RUNNING             : [INVALID - must be AVAILABLE first]

AVAILABLE → NONE           : Process becomes unavailable (timeout, user action)
AVAILABLE → SCHEDULED      : ScheduleProcess(requestedStart > now)
AVAILABLE → RUNNING        : ScheduleProcess(requestedStart = null or <= now)
AVAILABLE → ABORTED        : CancelProcess()

SCHEDULED → NONE           : [INVALID - must go through ABORTED]
SCHEDULED → AVAILABLE      : [INVALID - must go through ABORTED]
SCHEDULED → RUNNING        : Scheduled time reached
SCHEDULED → ABORTED        : CancelProcess()

RUNNING → NONE             : [INVALID - must go through COMPLETED/ABORTED]
RUNNING → PAUSED           : Pause() command (requires isPausable)
RUNNING → COMPLETED        : Process natural completion
RUNNING → ABORTED          : Stop() command (requires isStoppable)

PAUSED → RUNNING           : Resume() command
PAUSED → ABORTED           : Stop() or CancelProcess()
PAUSED → COMPLETED         : [INVALID - must resume first]

COMPLETED → NONE           : Device clears completed process
COMPLETED → AVAILABLE      : Device announces new process (same or different)

ABORTED → NONE             : Device clears aborted process
ABORTED → AVAILABLE        : Device re-announces same process (if still valid)
```

### 3.3 Process Timing

**Scheduling precision:**
- Device MUST start within +/- 5 seconds of `scheduledStart`
- If device cannot meet scheduled time, it MUST notify with updated `actualStart`

**Pause constraints:**
- `minRunDuration`: Pause() rejected if process hasn't run this long
- `minPauseDuration`: Resume() rejected if paused less than this

---

## 4. ControlState + ProcessState Interaction

### 4.1 Interaction Matrix

This table defines what happens to ProcessState when ControlState changes:

| ControlState Change | NONE | AVAILABLE | SCHEDULED | RUNNING | PAUSED |
|---------------------|------|-----------|-----------|---------|--------|
| → FAILSAFE | No change | No change | See 4.2 | See 4.3 | See 4.4 |
| → OVERRIDE | No change | No change | No change | No change | No change |
| → AUTONOMOUS | No change | No change | See 4.5 | Continue | Continue |
| Reconnect | No change | No change | See 4.6 | No change | No change |

### 4.2 Connection Lost While SCHEDULED

When connection is lost (FAILSAFE) and ProcessState = SCHEDULED:

**Rule:** Process remains SCHEDULED if scheduledStart is in the future.

**Behavior:**
1. Process proceeds to RUNNING at scheduledStart (device-driven)
2. No controller confirmation needed
3. Process runs under failsafe limits

**Rationale:** User expectation is that scheduled task runs. Grid safety is maintained by failsafe limits.

### 4.3 Connection Lost While RUNNING

When connection is lost (FAILSAFE) and ProcessState = RUNNING:

**Rule:** Process continues running under failsafe limits.

**Behavior:**
1. Process continues executing
2. Power constrained to failsafeConsumptionLimit
3. Process may complete (→ COMPLETED) during FAILSAFE
4. Process may be slower due to reduced power

**Rationale:** Safer to complete process (e.g., dishwasher cycle) than leave it mid-operation.

### 4.4 Connection Lost While PAUSED

When connection is lost (FAILSAFE) and ProcessState = PAUSED:

**Rule:** Process remains PAUSED until failsafeDuration expires.

**Behavior:**
1. Process stays PAUSED during FAILSAFE
2. If failsafeDuration expires (→ AUTONOMOUS), device MAY auto-resume
3. Device-specific policy: conservative devices stay PAUSED, others resume

**PICS item:** `MASH.S.CTRL.B_PAUSED_AUTO_RESUME` indicates if device auto-resumes.

### 4.5 FAILSAFE Expires While Process Active

When transitioning FAILSAFE → AUTONOMOUS with active process:

| ProcessState | Behavior |
|--------------|----------|
| SCHEDULED | Process starts at scheduledStart (device-driven) |
| RUNNING | Process continues to completion |
| PAUSED | Device-specific: resume or stay paused |

### 4.6 Reconnection with Scheduled/Running Process

When controller reconnects and ProcessState = SCHEDULED or RUNNING:

**Rule:** Controller must re-subscribe and acknowledge state.

**Behavior:**
1. Device sends current processState and optionalProcess in subscription priming
2. Controller sees process in progress
3. Controller can: let it continue, Pause(), or CancelProcess()
4. If controller takes no action, process continues

---

## 5. Multi-Zone Process Control

### 5.1 Process Ownership

Only **one zone** can control a process at a time.

**Rule:** First zone to ScheduleProcess() owns the process.

**Ownership transfer:**
- Owner zone can CancelProcess()
- Higher-priority zone can "steal" with ScheduleProcess() (owner notified)
- Owner zone disconnecting does NOT cancel process (see 4.3)

### 5.2 Pause Authority

**Rule:** Any zone with priority >= owner can Pause().

**Resume authority:**
- Zone that issued Pause() can Resume()
- Higher priority zone can Resume()
- Owner zone can Resume() (if it didn't pause)

### 5.3 Priority Conflicts

| Scenario | Resolution |
|----------|------------|
| Zone A schedules, Zone B (higher priority) schedules | Zone B wins, Zone A notified |
| Zone A running, Zone B (higher priority) pauses | Process pauses, Zone A notified |
| Zone A paused it, Zone B (lower priority) resumes | Rejected (Zone B insufficient priority) |

---

## 6. Timing Edge Cases

### 6.1 Clock Synchronization

**Requirement:** Device clock SHOULD be synchronized via NTP or similar.

**Tolerance:** Device MUST accept scheduled times within +/- 300 seconds of its clock.

**Recommendation:** Controllers should use relative offsets when possible:
```cbor
{ "requestedStart": null, "startDelay": 3600 }  // Start in 1 hour
```

### 6.2 Reconnection Race

**Scenario:** Controller reconnects exactly as failsafeDuration is expiring.

**Rule:** If reconnection completes before AUTONOMOUS transition, FAILSAFE→CONTROLLED/LIMITED.

**Implementation:**
1. Reconnection attempt starts
2. If failsafe timer fires during handshake: wait for handshake to complete/fail
3. If handshake completes within 5 seconds of timer: reconnection wins
4. If handshake fails or takes > 5 seconds: AUTONOMOUS transition proceeds

### 6.3 Rapid State Changes

**Scenario:** Multiple commands arrive in rapid succession.

**Rule:** Commands processed in order received.

**Implementation:**
- Commands queued and processed sequentially
- Each command completes before next starts
- State notifications may be coalesced per subscription minInterval

---

## 7. PICS Items

```
# Connection loss behavior
MASH.S.CTRL.B_FAILSAFE_IMMEDIATE=1      # Failsafe limits applied immediately
MASH.S.CTRL.B_FAILSAFE_TIMER_PERSIST=1  # Timer survives restart

# Process behavior during connection loss
MASH.S.CTRL.B_PROCESS_CONTINUE_FAILSAFE=1  # Running process continues
MASH.S.CTRL.B_PAUSED_AUTO_RESUME=0          # Paused process does NOT auto-resume

# Reconnection behavior
MASH.S.CTRL.B_RECONNECT_RESTORE_LIMITS=1   # Zone limits restored on reconnect
MASH.S.CTRL.B_RECONNECT_RACE_WINDOW=5      # Reconnection race window in seconds
```

---

## 8. Test Cases

See:
- `TC-STATE-*`: ControlState transition tests
- `TC-PROCESS-*`: ProcessState transition tests
- `TC-FAILSAFE-*`: Failsafe timing and behavior tests
- `TC-INTERACTION-*`: ControlState + ProcessState interaction tests
