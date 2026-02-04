# MASH Test Traceability Matrix

> Mapping between features, behaviors, test cases, and implementation

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

This document provides traceability between:
- **Features** - Protocol capabilities (Electrical, Measurement, EnergyControl, etc.)
- **Behaviors** - Precise behavior specifications
- **Test Cases** - YAML test case files
- **PICS Codes** - Capability declarations
- **Implementation** - Go source files

---

## 2. Feature to Test Mapping

### 2.1 Electrical Feature

| Component | Reference |
|-----------|-----------|
| Spec | `docs/features/electrical.md` |
| Behavior | `docs/testing/behavior/electrical-feature.md` |
| Tests | `mash-go/testdata/cases/electrical-tests.yaml` |
| PICS | `mash-go/testdata/pics/electrical-feature.yaml` |
| Implementation | `mash-go/pkg/features/electrical.go` |

**Test Cases (8):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-ELEC-001 | Read phase configuration | MASH.S.ELEC |
| TC-ELEC-002 | IsBidirectional returns correct value | MASH.S.ELEC |
| TC-ELEC-003 | CanConsume for consumption-only device | MASH.S.ELEC |
| TC-ELEC-004 | CanProduce for production-only device | MASH.S.ELEC |
| TC-ELEC-005 | Phase mapping consistency | MASH.S.ELEC |
| TC-ELEC-006 | Power ratings validation | MASH.S.ELEC |
| TC-ELEC-007 | Asymmetric support levels | MASH.S.ELEC |
| TC-ELEC-008 | Energy capacity for storage | MASH.S.ELEC, MASH.S.ELEC.STORAGE |

### 2.2 Measurement Feature

| Component | Reference |
|-----------|-----------|
| Spec | `docs/features/measurement.md` |
| Behavior | `docs/testing/behavior/measurement-feature.md` |
| Tests | `mash-go/testdata/cases/measurement-tests.yaml` |
| PICS | `mash-go/testdata/pics/measurement-feature.yaml` |
| Implementation | `mash-go/pkg/features/measurement.go` |

**Test Cases (10):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-MEAS-001 | Read AC power (positive = consumption) | MASH.S.MEAS |
| TC-MEAS-002 | Read AC power (negative = production) | MASH.S.MEAS |
| TC-MEAS-003 | Per-phase current values | MASH.S.MEAS |
| TC-MEAS-004 | Nullable attribute returns false when unset | MASH.S.MEAS |
| TC-MEAS-005 | IsConsuming helper method | MASH.S.MEAS |
| TC-MEAS-006 | IsProducing helper method | MASH.S.MEAS |
| TC-MEAS-007 | Cumulative energy never decreases | MASH.S.MEAS |
| TC-MEAS-008 | Battery SoC range 0-100 | MASH.S.MEAS, MASH.S.MEAS.BATTERY |
| TC-MEAS-009 | Power factor bounds | MASH.S.MEAS |
| TC-MEAS-010 | Subscribe to measurement changes | MASH.S.MEAS, MASH.S.SUB |

### 2.3 EnergyControl Feature

| Component | Reference |
|-----------|-----------|
| Spec | `docs/features/energy-control.md` |
| Behavior | `docs/testing/behavior/energycontrol-feature.md` |
| Tests | `mash-go/testdata/cases/energycontrol-tests.yaml` |
| PICS | `mash-go/testdata/pics/energycontrol-feature.yaml` |
| Implementation | `mash-go/pkg/features/energy_control.go` |

**Test Cases (12):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-CTRL-001 | SetLimit command accepted | MASH.S.CTRL, MASH.S.CTRL.C01.Rsp |
| TC-CTRL-002 | ClearLimit removes limit | MASH.S.CTRL, MASH.S.CTRL.C02.Rsp |
| TC-CTRL-003 | SetSetpoint command accepted | MASH.S.CTRL, MASH.S.CTRL.C03.Rsp |
| TC-CTRL-004 | Pause command transitions state | MASH.S.CTRL, MASH.S.CTRL.C09.Rsp |
| TC-CTRL-005 | Resume from paused state | MASH.S.CTRL, MASH.S.CTRL.C0A.Rsp |
| TC-CTRL-006 | Stop aborts process | MASH.S.CTRL, MASH.S.CTRL.C0B.Rsp |
| TC-CTRL-007 | Effective vs own limit values | MASH.S.CTRL, MASH.S.ZONE |
| TC-CTRL-008 | OptOut states respected | MASH.S.CTRL |
| TC-CTRL-009 | Failsafe limits applied | MASH.S.CTRL |
| TC-CTRL-010 | Command to unsupported capability fails | MASH.S.CTRL |
| TC-CTRL-011 | Duration parameter expires limit | MASH.S.CTRL |
| TC-CTRL-012 | Per-phase current limits | MASH.S.CTRL, MASH.S.CTRL.F09 |

### 2.4 ChargingSession Feature

| Component | Reference |
|-----------|-----------|
| Spec | `docs/features/charging-session.md` |
| Behavior | `docs/testing/behavior/chargingsession-feature.md` |
| Tests | `mash-go/testdata/cases/chargingsession-tests.yaml` |
| PICS | `mash-go/testdata/pics/chargingsession-feature.yaml` |
| Implementation | `mash-go/pkg/features/charging_session.go` |

**Test Cases (10):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-CHRG-001 | Session lifecycle transitions | MASH.S.CHRG |
| TC-CHRG-002 | StartSession sets ID and time | MASH.S.CHRG |
| TC-CHRG-003 | EndSession returns to initial state | MASH.S.CHRG |
| TC-CHRG-004 | EV demand modes | MASH.S.CHRG |
| TC-CHRG-005 | CanDischarge logic (all conditions) | MASH.S.CHRG, MASH.S.CTRL.F0A |
| TC-CHRG-006 | CanDischarge false when min >= 0 | MASH.S.CHRG, MASH.S.CTRL.F0A |
| TC-CHRG-007 | Charging mode validation | MASH.S.CHRG |
| TC-CHRG-008 | Start/stop delays | MASH.S.CHRG |
| TC-CHRG-009 | EV identification storage | MASH.S.CHRG |
| TC-CHRG-010 | Session energy tracking | MASH.S.CHRG |

### 2.5 Status Feature

| Component | Reference |
|-----------|-----------|
| Spec | `docs/features/status.md` |
| Behavior | `docs/testing/behavior/status-feature.md` |
| Tests | `mash-go/testdata/cases/status-tests.yaml` |
| PICS | `mash-go/testdata/pics/status-feature.yaml` |
| Implementation | `mash-go/pkg/features/status.go` |

**Test Cases (6):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-STAT-001 | Operating state transitions | MASH.S.STAT |
| TC-STAT-002 | SetFault sets state + code + message | MASH.S.STAT |
| TC-STAT-003 | ClearFault clears all fault fields | MASH.S.STAT |
| TC-STAT-004 | IsFaulted helper method | MASH.S.STAT |
| TC-STAT-005 | IsRunning helper method | MASH.S.STAT |
| TC-STAT-006 | FAULT state requires fault code | MASH.S.STAT |

---

## 3. Protocol Behavior Tests

### 3.1 State Machines

| Component | Reference |
|-----------|-----------|
| Behavior | `docs/testing/behavior/state-machines.md` |
| Tests | `mash-go/testdata/cases/state-machine-tests.yaml` |
| Implementation | `mash-go/pkg/features/energy_control.go` |

**Test Cases (15):**

| ID | Name | Behavior |
|----|------|----------|
| TC-STATE-001 | AUTONOMOUS to CONTROLLED | First command transitions state |
| TC-STATE-002 | CONTROLLED to LIMITED | Active limit constrains device |
| TC-STATE-003 | Transition to FAILSAFE | Connection loss |
| TC-STATE-004 | FAILSAFE to AUTONOMOUS | After failsafe duration |
| TC-STATE-005 | Any to OVERRIDE | Local user action |
| TC-STATE-006 | ProcessState NONE to AVAILABLE | Process becomes available |
| TC-STATE-007 | SCHEDULED to RUNNING | At scheduled time |
| TC-STATE-008 | RUNNING to PAUSED | Pause command |
| TC-STATE-009 | PAUSED to RUNNING | Resume command |
| TC-STATE-010 | Process to COMPLETED | Successful completion |
| TC-STATE-011 | Process to ABORTED | Stop command |
| TC-STATE-012 | Orthogonality - Process continues in FAILSAFE | Independence |
| TC-STATE-013 | Scheduled process starts despite FAILSAFE | Independence |
| TC-STATE-014 | OVERRIDE does not affect ProcessState | Independence |
| TC-STATE-015 | Multiple transitions in sequence | Chain verification |

### 3.2 Multi-Zone Resolution

| Component | Reference |
|-----------|-----------|
| Behavior | `docs/testing/behavior/multi-zone-resolution.md` |
| Tests | `mash-go/testdata/cases/zone-limit-tests.yaml` |
| Implementation | `mash-go/pkg/zone/manager.go`, `mash-go/pkg/features/energy_control.go` |

**Test Cases (13):**

| ID | Name | Behavior |
|----|------|----------|
| TC-ZONE-001 | Single zone sets limit | Basic limit |
| TC-ZONE-002 | Two zones, lower value wins | Most restrictive |
| TC-ZONE-003 | Production limit closer to zero wins | Negative resolution |
| TC-ZONE-004 | Mixed consumption/production limits | Positive precedence |
| TC-ZONE-005 | Limit with duration expires | Duration semantics |
| TC-ZONE-006 | Higher priority zone's setpoint wins | Priority resolution |
| TC-ZONE-007 | Zone disconnect affects resolution | Connected state |
| TC-ZONE-008 | Force remove lower priority zone | Priority enforcement |
| TC-ZONE-009 | Cannot force remove higher priority | Priority protection |
| TC-ZONE-010 | Per-phase limit resolution | Phase-specific |
| TC-ZONE-011 | Zone reconnect restores values | State persistence |
| TC-ZONE-012 | Maximum 2 zones enforcement | Capacity limit |
| TC-ZONE-013 | All zones clear limit | Unlimited behavior |

### 3.3 Subscriptions

| Component | Reference |
|-----------|-----------|
| Behavior | `docs/testing/behavior/subscription-semantics.md` |
| Tests | `mash-go/testdata/cases/subscription-tests.yaml` |
| Implementation | `mash-go/pkg/service/subscription.go` |

**Test Cases (12):**

| ID | Name | Behavior |
|----|------|----------|
| TC-SUB-001 | Priming report contains all attributes | Initial values |
| TC-SUB-002 | Delta notification on change | Only changed |
| TC-SUB-003 | No notification if unchanged | Suppression |
| TC-SUB-004 | Heartbeat at maxInterval | Periodic full state |
| TC-SUB-005 | Coalescing within minInterval | Rate limiting |
| TC-SUB-006 | Multiple subscriptions same feature | Independence |
| TC-SUB-007 | Unsubscribe stops notifications | Cleanup |
| TC-SUB-008 | Subscription survives zone changes | Persistence |
| TC-SUB-009 | Subscription lost on disconnect | Session binding |
| TC-SUB-010 | Re-subscribe after reconnect | Recovery |
| TC-SUB-011 | Subscribe to multiple features | Multi-feature |
| TC-SUB-012 | Subscription ID uniqueness | Correlation |

---

## 4. Infrastructure Tests

### 4.1 Connection

| Component | Reference |
|-----------|-----------|
| Behavior | `docs/testing/behavior/connection-state-machine.md` |
| Tests | `mash-go/testdata/cases/connection-tests.yaml` |
| Implementation | `mash-go/pkg/transport/` |

**Test Cases (5):**

| ID | Name |
|----|------|
| TC-CONN-001 | TLS handshake success |
| TC-CONN-002 | Keep-alive ping/pong |
| TC-CONN-003 | Reconnect with backoff |
| TC-CONN-004 | Graceful close |
| TC-CONN-005 | Connection timeout |

### 4.2 Connection Busy Response (DEC-063)

| Component | Reference |
|-----------|-----------|
| Spec | `docs/transport.md` Section 5.4, `docs/security.md` Section 11.8 |
| Tests | `mash-go/testdata/cases/connection-busy-tests.yaml` |
| Implementation | `mash-go/pkg/service/device_service.go`, `mash-go/pkg/commissioning/session.go` |

**Test Cases (3):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-CONN-BUSY-001 | Busy Response Contains RetryAfter | MASH.S.COMM.ERR_BUSY, MASH.S.COMM.ERR_BUSY_RETRY_AFTER |
| TC-CONN-BUSY-002 | Busy Response During Cooldown | MASH.S.COMM.ERR_BUSY, MASH.S.COMM.CONN_COOLDOWN |
| TC-CONN-BUSY-003 | Retry After Busy Succeeds | MASH.S.COMM.ERR_BUSY, MASH.S.COMM.ERR_BUSY_RETRY_AFTER |

### 4.3 Connection Reaper (DEC-064)

| Component | Reference |
|-----------|-----------|
| Spec | `docs/transport.md` Section 5.4, `docs/security.md` Section 11.9 |
| Tests | `mash-go/testdata/cases/connection-reaper-tests.yaml` |
| Implementation | `mash-go/pkg/service/conn_tracker.go`, `mash-go/pkg/service/device_service.go` |

**Test Cases (3):**

| ID | Name | PICS Required |
|----|------|---------------|
| TC-CONN-REAP-001 | Idle Connection Reaped | MASH.S.CONN.STALE_REAPER, MASH.S.CONN.STALE_TIMEOUT |
| TC-CONN-REAP-002 | Active Session Not Reaped | MASH.S.CONN.STALE_REAPER, MASH.S.CONN.STALE_TIMEOUT |
| TC-CONN-REAP-003 | Reaper Frees Connection Slot | MASH.S.CONN.STALE_REAPER, MASH.S.TRANS.CONN_CAP |

### 4.4 Error Handling

| Component | Reference |
|-----------|-----------|
| Tests | `mash-go/testdata/cases/error-handling-tests.yaml` |
| Implementation | `mash-go/pkg/wire/errors.go` |

**Test Cases (6):**

| ID | Name |
|----|------|
| TC-ERR-001 | Invalid endpoint returns error |
| TC-ERR-002 | Invalid feature returns error |
| TC-ERR-003 | Invalid attribute returns error |
| TC-ERR-004 | Write to read-only fails |
| TC-ERR-005 | Invalid command parameter |
| TC-ERR-006 | Constraint violation |

---

## 5. PICS Code Index

### 5.1 Feature Presence PICS

| PICS Code | Description | Required For |
|-----------|-------------|--------------|
| MASH.S | Device implements MASH | All tests |
| MASH.S.ELEC | Electrical feature | TC-ELEC-* |
| MASH.S.MEAS | Measurement feature | TC-MEAS-* |
| MASH.S.CTRL | EnergyControl feature | TC-CTRL-*, TC-STATE-* |
| MASH.S.CHRG | ChargingSession feature | TC-CHRG-* |
| MASH.S.STAT | Status feature | TC-STAT-* |
| MASH.S.ZONE | Multi-zone support | TC-ZONE-* |
| MASH.S.SUB | Subscription support | TC-SUB-* |

### 5.2 Feature Flag PICS

| PICS Code | Description | Required For |
|-----------|-------------|--------------|
| MASH.S.CTRL.F00 | CORE flag | All CTRL tests |
| MASH.S.CTRL.F03 | EMOB flag | TC-CHRG-* |
| MASH.S.CTRL.F07 | PROCESS flag | TC-STATE-006..011 |
| MASH.S.CTRL.F09 | ASYMMETRIC flag | TC-CTRL-012 |
| MASH.S.CTRL.F0A | V2X flag | TC-CHRG-005, TC-CHRG-006 |

### 5.3 Command PICS

| PICS Code | Description | Required For |
|-----------|-------------|--------------|
| MASH.S.CTRL.C01.Rsp | SetLimit | TC-CTRL-001, TC-ZONE-* |
| MASH.S.CTRL.C02.Rsp | ClearLimit | TC-CTRL-002 |
| MASH.S.CTRL.C03.Rsp | SetSetpoint | TC-CTRL-003 |
| MASH.S.CTRL.C09.Rsp | Pause | TC-CTRL-004, TC-STATE-008 |
| MASH.S.CTRL.C0A.Rsp | Resume | TC-CTRL-005, TC-STATE-009 |
| MASH.S.CTRL.C0B.Rsp | Stop | TC-CTRL-006, TC-STATE-011 |

---

## 6. Implementation Coverage

### 6.1 Features Package

| File | Tests | Coverage |
|------|-------|----------|
| `pkg/features/electrical.go` | TC-ELEC-001..008 | 8 tests |
| `pkg/features/measurement.go` | TC-MEAS-001..010 | 10 tests |
| `pkg/features/energy_control.go` | TC-CTRL-*, TC-STATE-* | 27 tests |
| `pkg/features/charging_session.go` | TC-CHRG-001..010 | 10 tests |
| `pkg/features/status.go` | TC-STAT-001..006 | 6 tests |

### 6.2 Zone Package

| File | Tests | Coverage |
|------|-------|----------|
| `pkg/zone/manager.go` | TC-ZONE-001..013 | 13 tests |
| `pkg/zone/zone.go` | TC-ZONE-* | Indirect |

### 6.3 Service Package

| File | Tests | Coverage |
|------|-------|----------|
| `pkg/service/subscription.go` | TC-SUB-001..012 | 12 tests |
| `pkg/service/device_service.go` | Multiple | Integration |
| `pkg/service/controller_service.go` | Multiple | Integration |

### 6.4 Transport Package

| File | Tests | Coverage |
|------|-------|----------|
| `pkg/transport/client.go` | TC-CONN-* | 5 tests |
| `pkg/transport/server.go` | TC-CONN-* | 5 tests |

---

## 7. Test File Summary

| File | Test Count | Category |
|------|------------|----------|
| `state-machine-tests.yaml` | 15 | State machines |
| `zone-limit-tests.yaml` | 13 | Multi-zone |
| `subscription-tests.yaml` | 12 | Subscriptions |
| `energycontrol-tests.yaml` | 12 | Feature |
| `measurement-tests.yaml` | 10 | Feature |
| `chargingsession-tests.yaml` | 10 | Feature |
| `electrical-tests.yaml` | 8 | Feature |
| `status-tests.yaml` | 6 | Feature |
| `error-handling-tests.yaml` | 6 | Error handling |
| `connection-tests.yaml` | 5 | Connection |
| `connection-busy-tests.yaml` | 3 | Busy Response (DEC-063) |
| `connection-reaper-tests.yaml` | 3 | Stale Reaper (DEC-064) |
| **Total (new Phase 3)** | **103** | |
| Existing tests | 13 | Discovery, commissioning, bidirectional, cert renewal |
| **Grand Total** | **116** | |

---

## 8. Running Tests

### By Feature

```bash
# Run specific feature tests
go run ./cmd/mash-test -target localhost:8443 -suite electrical
go run ./cmd/mash-test -target localhost:8443 -suite measurement
go run ./cmd/mash-test -target localhost:8443 -suite energycontrol
```

### With PICS Filtering

```bash
# Only tests matching device capabilities
go run ./cmd/mash-test -target localhost:8443 -pics testdata/pics/ev-charger.yaml
```

### Verbose Output

```bash
go run ./cmd/mash-test -target localhost:8443 -verbose
```
