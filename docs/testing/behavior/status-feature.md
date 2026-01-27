# Status Feature Behavior

> Implementation behaviors for the Status feature

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

The **Status** feature provides per-endpoint operating state and health information. It is READ-ONLY from the controller perspective - the device sets values based on its internal state.

**Key concepts:**
- **Operating state**: High-level device status
- **Faults**: Error conditions with codes and messages
- **State details**: Vendor-specific extended state information

**Reference implementation:** `pkg/features/status.go`

---

## 2. Operating State

### 2.1 OperatingState Values

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | UNKNOWN | State not known |
| 0x01 | OFFLINE | Not connected / not available |
| 0x02 | STANDBY | Ready but not active |
| 0x03 | STARTING | Powering up / initializing |
| 0x04 | RUNNING | Actively operating |
| 0x05 | PAUSED | Temporarily paused (can resume) |
| 0x06 | SHUTTING_DOWN | Powering down |
| 0x07 | FAULT | Error condition (check faultCode) |
| 0x08 | MAINTENANCE | Under maintenance / firmware update |

### 2.2 State Transitions

```
UNKNOWN ──[device initializes]──> STARTING
STARTING ──[init complete]──> STANDBY | RUNNING
STANDBY ──[operation starts]──> RUNNING
RUNNING ──[pause requested]──> PAUSED
PAUSED ──[resume]──> RUNNING
RUNNING ──[operation complete]──> STANDBY
ANY ──[error detected]──> FAULT
FAULT ──[error cleared]──> (previous state)
ANY ──[maintenance mode]──> MAINTENANCE
MAINTENANCE ──[maintenance ends]──> STANDBY
ANY ──[shutdown requested]──> SHUTTING_DOWN
SHUTTING_DOWN ──[power off]──> OFFLINE
OFFLINE ──[power on]──> STARTING
```

---

## 3. Fault Management

### 3.1 Fault Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| faultCode | uint32 | Numeric fault code (nullable) |
| faultMessage | string | Human-readable description (nullable) |

### 3.2 SetFault Method

```go
func (s *Status) SetFault(code uint32, message string) error {
    s.SetOperatingState(OperatingStateFault)
    s.setFaultCode(code)
    s.setFaultMessage(message)
    return nil
}
```

**Behavior:**
1. Sets operatingState to FAULT
2. Sets faultCode to provided code
3. Sets faultMessage to provided message

### 3.3 ClearFault Method

```go
func (s *Status) ClearFault() error {
    s.setFaultCode(nil)
    s.setFaultMessage(nil)
    return nil
}
```

**Behavior:**
1. Clears faultCode (sets to null)
2. Clears faultMessage (sets to null)
3. Does NOT change operatingState (caller must set appropriate state)

**Important:** After ClearFault(), the device should explicitly transition to an appropriate state (typically STANDBY or RUNNING).

### 3.4 Fault Code Ranges

Fault codes are device-specific. Recommended ranges:

| Range | Category |
|-------|----------|
| 0x0000-0x00FF | General errors |
| 0x0100-0x01FF | Communication errors |
| 0x0200-0x02FF | Hardware errors |
| 0x0300-0x03FF | Configuration errors |
| 0x0400-0x04FF | Safety errors |
| 0x1000-0xFFFF | Vendor-specific |

---

## 4. State Detail

### 4.1 Attribute

| Attribute | Type | Description |
|-----------|------|-------------|
| stateDetail | uint32 | Vendor-specific state detail code (nullable) |

### 4.2 Purpose

Provides additional context beyond operatingState:
- Device in RUNNING might have detail codes for different operating modes
- Device in STANDBY might indicate specific waiting conditions
- Vendor-defined semantics

---

## 5. Helper Methods

### 5.1 IsFaulted()

```go
func (s *Status) IsFaulted() bool {
    return s.OperatingState() == OperatingStateFault
}
```

Returns `true` if device is in FAULT state.

### 5.2 IsRunning()

```go
func (s *Status) IsRunning() bool {
    return s.OperatingState() == OperatingStateRunning
}
```

Returns `true` if device is actively operating.

### 5.3 IsReady()

```go
func (s *Status) IsReady() bool {
    state := s.OperatingState()
    return state == OperatingStateStandby || state == OperatingStateRunning
}
```

Returns `true` if device is ready for operation (STANDBY or RUNNING).

### 5.4 IsOffline()

```go
func (s *Status) IsOffline() bool {
    return s.OperatingState() == OperatingStateOffline
}
```

Returns `true` if device is offline.

---

## 6. Attribute Summary

| ID | Name | Type | Access | Nullable | Description |
|----|------|------|--------|----------|-------------|
| 1 | operatingState | uint8 | RO | No | Current operating state |
| 2 | stateDetail | uint32 | RO | Yes | Vendor-specific detail |
| 3 | faultCode | uint32 | RO | Yes | Fault code when FAULT |
| 4 | faultMessage | string | RO | Yes | Human-readable fault |

---

## 7. PICS Items

```
# Operating states
MASH.S.STATUS.PAUSED             # PAUSED state supported
MASH.S.STATUS.MAINTENANCE        # MAINTENANCE state supported

# Fault handling
MASH.S.STATUS.FAULT_CODES        # Fault codes provided
MASH.S.STATUS.FAULT_MESSAGES     # Fault messages provided

# State detail
MASH.S.STATUS.STATE_DETAIL       # stateDetail attribute used
```

---

## 8. Test Cases

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-STATUS-001 | Read operating state | Read operatingState | Valid enum value |
| TC-STATUS-002 | SetFault transitions to FAULT | Call SetFault | operatingState = FAULT |
| TC-STATUS-003 | SetFault sets code and message | Call SetFault(123, "test") | faultCode=123, faultMessage="test" |
| TC-STATUS-004 | ClearFault clears code and message | SetFault then ClearFault | faultCode=null, faultMessage=null |
| TC-STATUS-005 | IsFaulted helper | Set FAULT state | IsFaulted() = true |
| TC-STATUS-006 | IsRunning helper | Set RUNNING state | IsRunning() = true |
| TC-STATUS-007 | IsReady for STANDBY | Set STANDBY | IsReady() = true |
| TC-STATUS-008 | IsReady for RUNNING | Set RUNNING | IsReady() = true |
| TC-STATUS-009 | IsReady false for FAULT | Set FAULT | IsReady() = false |
| TC-STATUS-010 | Subscribe to state changes | Subscribe, change state | Notification received |
| TC-STATUS-011 | State detail vendor-specific | Set stateDetail | Value preserved |
| TC-STATUS-012 | Nullable attributes return null | Read unset faultCode | (0, false) or null |

---

## 9. Implementation Notes

### 9.1 Default Values

| Attribute | Default |
|-----------|---------|
| operatingState | UNKNOWN (0x00) |
| stateDetail | null |
| faultCode | null |
| faultMessage | null |

### 9.2 Thread Safety

All getters/setters are thread-safe through the underlying Feature implementation.

### 9.3 Nullable Attribute Pattern

For nullable attributes, getters return a tuple:

```go
detail, ok := s.StateDetail()
if !ok {
    // stateDetail is null
}
```

### 9.4 Relationship to EnergyControl.ProcessState

Status.operatingState and EnergyControl.processState are complementary:
- operatingState: Device-level health ("is the device working?")
- processState: Task-level progress ("what is the task doing?")

A device can be RUNNING (operatingState) while a process is PAUSED (processState).
