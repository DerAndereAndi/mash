# Clean Architecture TDD Implementation Plan

**Created:** 2026-02-13
**Status:** In Progress
**Goal:** Improve separation of concerns, interface design, and testability across the mash-go stack without over-engineering.

---

## Guiding Principles

1. **TDD**: Write failing tests first, then implement to make them pass
2. **No over-engineering**: Only abstract what has concrete testing or decoupling benefit
3. **Incremental**: Each task is independently mergeable
4. **No hallucination**: Every file path and method name in this plan was verified against the codebase
5. **Preserve behavior**: Zero functional changes -- only structural improvements

---

## Phase 1: Split Wide Interfaces in Test Harness Runner

**Why:** CommissioningOps (24 methods) and ConnPool (22 methods) violate Interface Segregation. Tests mock the full interface even when they use 3-4 methods. Splitting makes tests focused and mocks small.

**Risk:** Low -- additive changes, existing code keeps working.

### Task 1.1: Split CommissioningOps into focused interfaces

**Current:** `runner/coordinator.go:36-76` -- 24 methods in one interface.

**Proposed split based on actual caller analysis:**

```go
// StateAccessor -- read/write commissioning metadata (13 methods)
// Used by: Coordinator.SetupPreconditions, TeardownTest for state queries
type StateAccessor interface {
    PASEState() *PASEState
    SetPASEState(ps *PASEState)
    DeviceStateModified() bool
    SetDeviceStateModified(modified bool)
    WorkingCrypto() CryptoState
    SetWorkingCrypto(crypto CryptoState)
    ClearWorkingCrypto()
    CommissionZoneType() cert.ZoneType
    SetCommissionZoneType(zt cert.ZoneType)
    DiscoveredDiscriminator() uint16
    LastDeviceConnClose() time.Time
    SetLastDeviceConnClose(t time.Time)
    IsSuiteZoneCommission() bool
}

// LifecycleOps -- connection/commissioning transitions (5 methods)
// Used by: Coordinator.SetupPreconditions, TeardownTest for state transitions
type LifecycleOps interface {
    EnsureConnected(ctx context.Context, state *engine.ExecutionState) error
    EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error
    DisconnectConnection()
    EnsureDisconnected()
    ReconnectToZone(state *engine.ExecutionState) error
}

// WireOps -- protocol-level control messages (4 methods)
// Used by: Coordinator.TeardownTest for cleanup
type WireOps interface {
    SendRemoveZone()
    SendRemoveZoneOnConn(conn *Connection, zoneID string)
    SendTriggerViaZone(ctx context.Context, trigger uint64, state *engine.ExecutionState) error
    SendClearLimitInvoke(ctx context.Context) error
}

// DiagnosticsOps -- health and state inspection (2 methods)
// Used by: Coordinator for session probing and device state capture
type DiagnosticsOps interface {
    ProbeSessionHealth() error
    RequestDeviceState(ctx context.Context, state *engine.ExecutionState) DeviceStateSnapshot
}

// PreconditionHandler -- precondition dispatch (2 methods)
// Used by: Coordinator.SetupPreconditions
type PreconditionHandler interface {
    WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error
    HandlePreconditionCases(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState,
        preconds []loader.Condition, needsMultiZone *bool) error
}
```

**Compose for backward compatibility:**
```go
// CommissioningOps preserved as composition of the above
type CommissioningOps interface {
    StateAccessor
    LifecycleOps
    WireOps
    DiagnosticsOps
    PreconditionHandler
}
```

**Files to modify:**
- `runner/coordinator.go` -- add new interfaces, compose CommissioningOps from them
- `runner/coordinator_test.go` -- update stubs to implement smaller interfaces where possible
- `.mockery.yaml` -- add new interface entries for mock generation

**TDD steps:**
1. [ ] Write test for Coordinator using only `StateAccessor` mock (not full CommissioningOps)
2. [ ] Write test for Coordinator using only `LifecycleOps` mock
3. [ ] Define the new interfaces in coordinator.go
4. [ ] Compose CommissioningOps from the new interfaces (backward compatible)
5. [ ] Update .mockery.yaml, run `make generate`
6. [ ] Update coordinator_test.go stubs to use narrow interfaces
7. [ ] Run `go test ./internal/testharness/...` -- all green
8. [ ] Run `go_diagnostics` -- no errors

### Task 1.2: Split ConnPool into focused interfaces

**Current:** `runner/conn_pool.go:13-82` -- 22 methods in one interface.

**Proposed split based on actual caller analysis:**

```go
// ConnReader -- read-only pool access (7 methods)
// Used by: handlers for sending requests, checking connection state
type ConnReader interface {
    Main() *Connection
    Zone(key string) *Connection
    ZoneID(key string) string
    ZoneCount() int
    ZoneKeys() []string
    NextMessageID() uint32
    Subscriptions() []uint32
}

// ConnWriter -- mutating pool operations (5 methods)
// Used by: handlers for zone lifecycle, subscription tracking
type ConnWriter interface {
    SetMain(conn *Connection)
    TrackZone(key string, conn *Connection, zoneID string)
    UntrackZone(key string)
    TrackSubscription(subID uint32)
    RemoveSubscription(subID uint32)
}

// ConnLifecycle -- connection close/cleanup (3 methods)
// Used by: Coordinator teardown, preconditions
type ConnLifecycle interface {
    CloseZonesExcept(exceptKey string) time.Time
    CloseAllZones() time.Time
    UnsubscribeAll(conn *Connection)
}

// RequestSender -- wire-level request/response (2 methods)
// Used by: handlers for protocol operations
type RequestSender interface {
    SendRequest(data []byte, op string, expectedMsgID uint32) (*wire.Response, error)
    SendRequestWithDeadline(ctx context.Context, data []byte, op string, expectedMsgID uint32) (*wire.Response, error)
}

// NotificationBuffer -- notification queue (4 methods)
// Used by: utility handlers for notification inspection
type NotificationBuffer interface {
    PendingNotifications() [][]byte
    ShiftNotification() ([]byte, bool)
    AppendNotification(data []byte)
    ClearNotifications()
}
```

**Compose for backward compatibility:**
```go
type ConnPool interface {
    ConnReader
    ConnWriter
    ConnLifecycle
    RequestSender
    NotificationBuffer
}
```

**Files to modify:**
- `runner/conn_pool.go` -- add new interfaces, compose ConnPool
- `runner/conn_pool_test.go` -- verify existing tests still pass
- `.mockery.yaml` -- add new interface entries

**TDD steps:**
1. [ ] Write test using only `ConnReader` mock (e.g., handler that reads zone state)
2. [ ] Write test using only `RequestSender` mock (e.g., handler that sends a request)
3. [ ] Define the new interfaces in conn_pool.go
4. [ ] Compose ConnPool from the new interfaces
5. [ ] Update .mockery.yaml, run `make generate`
6. [ ] Run `go test ./internal/testharness/...` -- all green

### Task 1.3: Add ConnectionManager to mockery config

**Current:** ConnectionManager interface exists in `conn_mgr.go:20-56` but is NOT in `.mockery.yaml`.

**TDD steps:**
1. [ ] Add `ConnectionManager: {}` to `.mockery.yaml` under runner interfaces
2. [ ] Run `make generate` to produce MockConnectionManager
3. [ ] Write one test using MockConnectionManager (verify mock compiles)
4. [ ] Run `go test ./internal/testharness/...` -- all green

---

## Phase 2: Extract Context Keys to Shared Package

**Why:** Context keys for zone ID and zone type are defined in `pkg/service/remove_zone_handler.go:51-87`. This forces `pkg/features/limit_resolver.go` to use injected functions instead of importing the context helpers directly. Moving keys to a shared package breaks this coupling.

**Risk:** Low -- moves existing code, no behavior change.

### Task 2.1: Create pkg/zonecontext package

**New file:** `pkg/zonecontext/context.go`

Move from `pkg/service/remove_zone_handler.go:51-87`:
- `callerZoneIDKey` type
- `ContextWithCallerZoneID()` function
- `CallerZoneIDFromContext()` function
- `callerZoneTypeKey` type
- `ContextWithCallerZoneType()` function
- `CallerZoneTypeFromContext()` function

**Files to modify:**
- `pkg/zonecontext/context.go` -- new file with extracted types
- `pkg/service/remove_zone_handler.go` -- remove context key definitions, import zonecontext
- `pkg/service/protocol_handler.go` -- update imports to use zonecontext
- `pkg/features/limit_resolver.go` -- import zonecontext directly instead of injected functions
- `cmd/mash-device/main.go` -- update wiring (remove function injection for context extractors)
- `pkg/examples/examples_test.go` -- update imports

**TDD steps:**
1. [ ] Write test: `zonecontext.CallerZoneIDFromContext(zonecontext.ContextWithCallerZoneID(ctx, "z1"))` == "z1"
2. [ ] Write test: `CallerZoneTypeFromContext` round-trips correctly
3. [ ] Write test: empty context returns zero values
4. [ ] Create `pkg/zonecontext/context.go` with the functions
5. [ ] Tests pass
6. [ ] Update `pkg/service/remove_zone_handler.go` -- delegate to zonecontext
7. [ ] Update `pkg/features/limit_resolver.go` -- replace injected functions with direct import
8. [ ] Remove injection wiring in `cmd/mash-device/main.go`
9. [ ] Run `go test ./...` -- all green
10. [ ] Run `go_diagnostics` -- no errors

---

## Phase 3: Add Service-Layer Interfaces

**Why:** `DeviceService` depends on concrete `zone.Manager`, `subscription.Manager`, and `model.Device`. Testing DeviceService requires constructing the full real dependency tree. Adding interfaces at consumption boundaries enables focused unit tests.

**Risk:** Medium -- touches widely-used types, but interfaces are additive.

### Task 3.1: Define ZoneManager interface in pkg/service

**Current:** `pkg/zone/manager.go` exposes 18+ methods on concrete `*zone.Manager`.
DeviceService uses a subset. Define the interface where it's consumed (in pkg/service), not where it's implemented (in pkg/zone). This follows Go's "accept interfaces, return structs" idiom.

**Interface (only methods DeviceService actually calls):**
```go
// pkg/service/interfaces.go
type ZoneManager interface {
    AddZone(zoneID string, zoneType cert.ZoneType) error
    RemoveZone(zoneID string) error
    GetZone(zoneID string) (*zone.Zone, error)
    HasZone(zoneID string) bool
    ListZones() []string
    ZoneCount() int
    SetConnected(zoneID string) error
    SetDisconnected(zoneID string) error
    ConnectedZones() []string
    HighestPriorityConnectedZone() *zone.Zone
    CanRemoveZone(requesterType cert.ZoneType, targetZoneID string) bool
    OnZoneAdded(fn func(zone *zone.Zone))
    OnZoneRemoved(fn func(zoneID string))
    OnConnect(fn func(zoneID string))
    OnDisconnect(fn func(zoneID string))
}
```

**Files to modify:**
- `pkg/service/interfaces.go` -- new file with ZoneManager interface
- `pkg/service/device_service.go` -- change `zoneManager *zone.Manager` to `zoneManager ZoneManager`

**TDD steps:**
1. [ ] Write compile-time check: `var _ ZoneManager = (*zone.Manager)(nil)`
2. [ ] Create `pkg/service/interfaces.go` with ZoneManager
3. [ ] Change DeviceService field type from `*zone.Manager` to `ZoneManager`
4. [ ] Run `go_diagnostics` -- no errors
5. [ ] Run `go test ./pkg/service/...` -- all green

### Task 3.2: Define SubscriptionManager interface in pkg/service

**Current:** `pkg/subscription/manager.go` exposes 11 methods on concrete `*subscription.Manager`.

**Interface (only methods DeviceService/ControllerService actually call):**
```go
// pkg/service/interfaces.go (append)
type SubscriptionTracker interface {
    Subscribe(endpointID, featureID uint16, attributeIDs []uint16,
        minInterval, maxInterval time.Duration, currentValues map[uint16]any) (uint32, error)
    Unsubscribe(subscriptionID uint32) error
    NotifyChange(endpointID, featureID, attrID uint16, value any)
    NotifyChanges(endpointID, featureID uint16, changes map[uint16]any)
    ClearAll()
    Count() int
    OnNotification(fn func(subscription.Notification))
}
```

**Files to modify:**
- `pkg/service/interfaces.go` -- add SubscriptionTracker
- `pkg/service/device_service.go` -- change `subscriptionManager *subscription.Manager` to `subscriptionManager SubscriptionTracker`
- `pkg/service/controller_service.go` -- same change if applicable

**TDD steps:**
1. [ ] Write compile-time check: `var _ SubscriptionTracker = (*subscription.Manager)(nil)`
2. [ ] Add SubscriptionTracker to interfaces.go
3. [ ] Change service field types
4. [ ] Run `go_diagnostics` -- no errors
5. [ ] Run `go test ./pkg/service/...` -- all green

### Task 3.3: Define DeviceModel interface in pkg/service

**Current:** `ProtocolHandler` and `DeviceService` use `*model.Device` directly.

**Interface (methods used by service layer):**
```go
// pkg/service/interfaces.go (append)
type DeviceModel interface {
    DeviceID() string
    VendorID() uint32
    ProductID() uint16
    SerialNumber() string
    RootEndpoint() *model.Endpoint
    AddEndpoint(endpoint *model.Endpoint) error
    GetEndpoint(id uint8) (*model.Endpoint, error)
    Endpoints() []*model.Endpoint
    GetFeature(endpointID uint8, featureType model.FeatureType) (*model.Feature, error)
    ReadAttribute(endpointID uint8, featureType model.FeatureType, attrID uint16) (any, error)
    WriteAttribute(endpointID uint8, featureType model.FeatureType, attrID uint16, value any) error
    InvokeCommand(ctx context.Context, endpointID uint8, featureType model.FeatureType, cmdID uint8, params map[string]any) (map[string]any, error)
    Info() *model.DeviceInfo
    FindEndpointsByType(endpointType model.EndpointType) []*model.Endpoint
    FindEndpointsWithFeature(featureType model.FeatureType) []*model.Endpoint
}
```

**Files to modify:**
- `pkg/service/interfaces.go` -- add DeviceModel
- `pkg/service/device_service.go` -- change `device *model.Device` to `device DeviceModel`
- `pkg/service/protocol_handler.go` -- change `device *model.Device` to `device DeviceModel`

**TDD steps:**
1. [ ] Write compile-time check: `var _ DeviceModel = (*model.Device)(nil)`
2. [ ] Add DeviceModel to interfaces.go
3. [ ] Change service and protocol_handler field types
4. [ ] Run `go_diagnostics` -- no errors
5. [ ] Run `go test ./pkg/service/...` -- all green
6. [ ] Run `go test ./...` -- all green (catches any downstream breakage)

### Task 3.4: Generate mocks for new service interfaces

**TDD steps:**
1. [ ] Add ZoneManager, SubscriptionTracker, DeviceModel to `.mockery.yaml`
2. [ ] Run `make generate`
3. [ ] Write one test per mock to verify they compile and work with testify
4. [ ] Run `go test ./pkg/service/...` -- all green

---

## Phase 4: Consolidate Runner State into ConnectionManager

**Why:** Runner owns 8 crypto/commissioning fields that are duplicated in `connMgrImpl`. Handlers access `r.zoneCA`, `r.controllerCert` directly instead of going through `r.connMgr`. This duplication is the main barrier to true decomposition.

**Risk:** Medium -- touches many handler files, but mechanically straightforward.

### Task 4.1: Route crypto state through ConnectionManager

**Current state:** Runner has these fields that duplicate connMgr:
- `r.paseState` (12 direct accesses across handlers)
- `r.zoneCA`, `r.controllerCert`, `r.issuedDeviceCert`, `r.zoneCAPool` (14+ accesses)
- `r.commissionZoneType` (8 accesses)
- `r.deviceStateModified` (7 accesses)
- `r.lastDeviceConnClose` (2 accesses)

**Approach:** Replace `r.fieldName` with `r.connMgr.FieldName()` calls. Remove duplicated fields from Runner struct.

**Files to modify (high-touch):**
- `runner/runner.go` -- remove 8 duplicated fields from Runner struct
- `runner/security_handlers.go` -- update ~14 accesses to use connMgr
- `runner/connection_handlers.go` -- update ~8 accesses
- `runner/pase.go` -- update ~12 accesses to paseState, crypto
- `runner/commissioning.go` -- update ~7 accesses
- `runner/zone_handlers.go` -- update ~4 accesses
- `runner/preconditions.go` -- update ~5 accesses
- `runner/cert_handlers.go` -- update accesses
- `runner/conn_mgr.go` -- verify interface covers all needed accessors

**TDD steps:**
1. [ ] Write test: connMgr stores and returns PASEState correctly
2. [ ] Write test: connMgr stores and returns CryptoState correctly
3. [ ] Write test: connMgr tracks commissionZoneType
4. [ ] Write test: connMgr tracks deviceStateModified
5. [ ] Verify ConnectionManager interface has all needed methods (it does per exploration)
6. [ ] Remove duplicated fields from Runner struct one group at a time:
   - [ ] paseState -> r.connMgr.PASEState() / r.connMgr.SetPASEState()
   - [ ] crypto fields -> r.connMgr.WorkingCrypto() / r.connMgr.SetWorkingCrypto()
   - [ ] commissionZoneType -> r.connMgr.CommissionZoneType()
   - [ ] deviceStateModified -> r.connMgr.DeviceStateModified()
   - [ ] lastDeviceConnClose -> r.connMgr.LastDeviceConnClose()
7. [ ] After each group, run `go test ./internal/testharness/...` -- all green
8. [ ] Run `go_diagnostics` on modified files -- no errors
9. [ ] Final: Runner struct has 0 duplicated crypto/commissioning fields

### Task 4.2: Integrate Coordinator into test execution

**Current:** Coordinator is wired in `New()` but never called. The `Run()` method still calls precondition/teardown methods directly on Runner.

**Approach:** Replace direct calls in `Run()` with `r.coordinator.SetupPreconditions()` and `r.coordinator.TeardownTest()`.

**Files to modify:**
- `runner/runner.go` -- in `Run()`, replace direct precondition/teardown calls with coordinator

**TDD steps:**
1. [ ] Write test: Coordinator.SetupPreconditions delegates to CommissioningOps correctly
2. [ ] Write test: Coordinator.TeardownTest sends reset trigger and cleans up
3. [ ] Replace direct calls in Run() with coordinator calls
4. [ ] Run `go test ./internal/testharness/...` -- all green

---

## Phase 5: Generate inspect/names.go from YAML

**Why:** `pkg/inspect/names.go` has hand-maintained tables mapping attribute/command names to IDs. The same data exists in `docs/features/<feature>/1.0.yaml`. Adding a feature attribute to YAML but forgetting names.go is a latent bug class.

**Risk:** Low -- additive code generation, existing manual tables serve as validation.

### Task 5.1: Extend mash-featgen to emit name tables

**Current:** `cmd/mash-featgen/generate.go` (975 lines) generates `*_gen.go` files from YAML.

**Approach:** Add a new output target that generates `pkg/inspect/names_gen.go` from the same YAML parse.

**Files to create/modify:**
- `cmd/mash-featgen/generate_names.go` -- new file: name table generation logic
- `cmd/mash-featgen/generate_names_test.go` -- tests for name generation
- `pkg/inspect/names_gen.go` -- generated output (replaces hand-written names.go)
- `pkg/inspect/names.go` -- keep only the `Resolve*` functions, remove hardcoded tables
- `Makefile` -- ensure `make generate` includes name table generation

**TDD steps:**
1. [ ] Write test: generated name tables match current hand-written tables exactly
2. [ ] Write test: all features in YAML produce entries in generated tables
3. [ ] Write test: attributes and commands are lowercased correctly
4. [ ] Implement `generateNameTables()` in generate_names.go
5. [ ] Run generator, compare output against current names.go
6. [ ] Replace hand-written tables with generated ones
7. [ ] Delete hardcoded tables from names.go (keep resolver functions)
8. [ ] Run `go test ./pkg/inspect/...` -- all green
9. [ ] Run `go test ./internal/testharness/...` -- all green (resolver still works)

---

## Phase 6: Minor Cleanups

**Risk:** Low -- each task is small and isolated.

### Task 6.1: Rename service.SubscriptionManager to avoid confusion

**Current:** Two types named "SubscriptionManager":
- `pkg/subscription.Manager` -- coalescing/heartbeat logic
- `pkg/service.SubscriptionManager` -- per-session inbound/outbound tracking

**Rename:** `pkg/service.SubscriptionManager` -> `pkg/service.SessionSubscriptionTracker`

**TDD steps:**
1. [ ] Use gopls rename: `SubscriptionManager` -> `SessionSubscriptionTracker` in pkg/service
2. [ ] Run `go_diagnostics` -- no errors
3. [ ] Run `go test ./pkg/service/...` -- all green

### Task 6.2: Extract simulation logic from mash-device

**Current:** `cmd/mash-device/main.go` has simulation logic at lines 766-836.

**Approach:** Move to `cmd/mash-device/simulation.go` (same package, just file separation).

**TDD steps:**
1. [ ] Create `cmd/mash-device/simulation.go` with extracted functions
2. [ ] Remove from main.go
3. [ ] Run `go build ./cmd/mash-device/` -- compiles
4. [ ] Run `go_diagnostics` -- no errors

---

## Progress Tracking

| Phase | Task | Status | Notes |
|-------|------|--------|-------|
| 1 | 1.1 Split CommissioningOps | [x] Done | StateAccessor, LifecycleOps, WireOps, DiagnosticsOps, PreconditionHandler |
| 1 | 1.2 Split ConnPool | [x] Done | ConnReader, ConnWriter, ConnLifecycle, RequestSender, NotificationBuffer |
| 1 | 1.3 Add ConnectionManager to mockery | [x] Done | MockConnectionManager generated |
| 2 | 2.1 Create pkg/zonecontext | [x] Done | Context keys extracted, LimitResolver uses direct import, injection wiring removed |
| 3 | 3.1 ZoneManager interface | [ ] Not started | |
| 3 | 3.2 SubscriptionTracker interface | [ ] Not started | |
| 3 | 3.3 DeviceModel interface | [ ] Not started | |
| 3 | 3.4 Generate mocks | [ ] Not started | |
| 4 | 4.1 Consolidate Runner state | [ ] Not started | |
| 4 | 4.2 Integrate Coordinator | [ ] Not started | |
| 5 | 5.1 Generate names.go from YAML | [ ] Not started | |
| 6 | 6.1 Rename SubscriptionManager | [ ] Not started | |
| 6 | 6.2 Extract simulation logic | [ ] Not started | |

## Dependencies

```
Phase 1 ──> Phase 4 (Phase 4 uses the split interfaces from Phase 1)
Phase 2 ──> (independent, can run in parallel with Phase 1)
Phase 3 ──> (independent, can run in parallel with Phase 1/2)
Phase 5 ──> (independent, can run any time)
Phase 6 ──> Phase 3 (6.1 depends on 3.2 for naming clarity)
```

## What This Plan Does NOT Include

Intentionally excluded to avoid over-engineering:
- **Splitting DeviceService into sub-packages** -- the 13-dependency coupling is legitimate orchestration coupling. The interfaces added in Phase 3 address testability without structural upheaval.
- **Channel-based event bus for Runner** -- the callback pattern works; fixing the state duplication (Phase 4) is sufficient.
- **Unified error type across the stack** -- current per-package error handling is adequate.
- **Splitting handler files into sub-packages** -- handler files are large but cohesive; file-level organization suffices.
