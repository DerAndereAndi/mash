# Capability Snapshots: TDD Implementation Plan

**Source:** docs/design/capability-snapshots.md
**Date:** 2026-02-02

This plan is organized into 7 phases. Each phase follows red-green-refactor: write failing tests first, then implement the minimum code to make them pass. Phases are ordered by dependency -- later phases depend on earlier ones compiling.

---

## Phase 1: Log Layer Types (`pkg/log`)

Add the snapshot event types and the new category constant. This is pure data -- no behavior.

### Files Changed

- `pkg/log/event.go` -- Add `CategorySnapshot`, `Snapshot` field on `Event`, and snapshot structs

### Tests (`pkg/log/event_test.go` or `pkg/log/cbor_test.go`)

1. **TestCategorySnapshotString** -- `CategorySnapshot.String()` returns `"SNAPSHOT"`.

2. **TestSnapshotEventCBORRoundTrip** -- Create an `Event` with `Category: CategorySnapshot` and a populated `Snapshot` field containing `Local` and `Remote` `DeviceSnapshot`s with endpoints, features, and use cases. Encode with `EncodeEvent`, decode with `DecodeEvent`, and assert all fields survive the round-trip. This is the pattern already used in `cbor_test.go`.

3. **TestSnapshotEventCBORRoundTrip_NilRemote** -- Same as above but with `Remote: nil`. Verify the `omitempty` tag works correctly and the decoded `Remote` is nil.

4. **TestSnapshotEvent_BackwardCompat** -- Encode an event with a `Snapshot` field, then decode it into an `Event` struct that has no `Snapshot` field (simulate older reader). Verify the decode does not error -- unknown CBOR keys are silently ignored.

### Implementation

Add to `pkg/log/event.go`:

```go
const CategorySnapshot Category = 4
```

Update `Category.String()` to handle the new value.

Add the `Snapshot` field to `Event`:

```go
Snapshot *CapabilitySnapshotEvent `cbor:"15,keyasint,omitempty"`
```

Add the snapshot types (as specified in the design doc):

```go
type CapabilitySnapshotEvent struct {
    Local  *DeviceSnapshot `cbor:"1,keyasint"`
    Remote *DeviceSnapshot `cbor:"2,keyasint,omitempty"`
}

type DeviceSnapshot struct {
    DeviceID    string             `cbor:"1,keyasint"`
    SpecVersion string             `cbor:"2,keyasint,omitempty"`
    Endpoints   []EndpointSnapshot `cbor:"3,keyasint"`
    UseCases    []UseCaseSnapshot  `cbor:"4,keyasint,omitempty"`
}

type EndpointSnapshot struct {
    ID       uint8             `cbor:"1,keyasint"`
    Type     uint8             `cbor:"2,keyasint"`
    Label    string            `cbor:"3,keyasint,omitempty"`
    Features []FeatureSnapshot `cbor:"4,keyasint"`
}

type FeatureSnapshot struct {
    ID            uint16   `cbor:"1,keyasint"`
    FeatureMap    uint32   `cbor:"2,keyasint"`
    AttributeList []uint16 `cbor:"3,keyasint"`
    CommandList   []uint8  `cbor:"4,keyasint,omitempty"`
}

type UseCaseSnapshot struct {
    EndpointID uint8  `cbor:"1,keyasint"`
    ID         uint16 `cbor:"2,keyasint"`
    Major      uint8  `cbor:"3,keyasint"`
    Minor      uint8  `cbor:"4,keyasint"`
    Scenarios  uint32 `cbor:"5,keyasint"`
}
```

### Done When

`go test ./pkg/log/...` passes with all four tests green.

---

## Phase 2: Snapshot Policy (`pkg/service/types.go`)

Add the `SnapshotPolicy` struct, `DefaultSnapshotPolicy()`, and wire it into both config structs.

### Files Changed

- `pkg/service/types.go` -- Add `SnapshotPolicy`, `DefaultSnapshotPolicy()`, add field to `DeviceConfig` and `ControllerConfig`, update both `Default*Config()` functions

### Tests (`pkg/service/types_test.go`)

1. **TestDefaultSnapshotPolicy** -- Call `DefaultSnapshotPolicy()` and assert:
   - `MaxInterval == 30 * time.Minute`
   - `MaxMessages == 1000`
   - `MinMessages == 50`

2. **TestDefaultDeviceConfigIncludesSnapshotPolicy** -- Call `DefaultDeviceConfig()` and assert `SnapshotPolicy` equals `DefaultSnapshotPolicy()`.

3. **TestDefaultControllerConfigIncludesSnapshotPolicy** -- Call `DefaultControllerConfig()` and assert `SnapshotPolicy` equals `DefaultSnapshotPolicy()`.

### Implementation

Add to `pkg/service/types.go`:

```go
type SnapshotPolicy struct {
    MaxInterval time.Duration
    MaxMessages int
    MinMessages int
}

func DefaultSnapshotPolicy() SnapshotPolicy {
    return SnapshotPolicy{
        MaxInterval: 30 * time.Minute,
        MaxMessages: 1000,
        MinMessages: 50,
    }
}
```

Add `SnapshotPolicy SnapshotPolicy` field to both `DeviceConfig` and `ControllerConfig`. Set `DefaultSnapshotPolicy()` in both default config functions.

### Done When

`go test ./pkg/service/... -run TestDefault` passes.

---

## Phase 3: Snapshot Builder (`pkg/service/snapshot_builder.go`)

A pure function that converts a `*model.Device` into a `*log.DeviceSnapshot`. This is the data extraction logic, separate from trigger logic, making it easy to test.

### Files Changed

- `pkg/service/snapshot_builder.go` -- New file

### Tests (`pkg/service/snapshot_builder_test.go`)

1. **TestBuildDeviceSnapshot_Basic** -- Create a test device (using the existing `createTestDevice()` pattern: root endpoint + 1 EVSE endpoint with Electrical and Measurement features). Call `buildDeviceSnapshot(device)`. Assert:
   - `DeviceID` matches device ID
   - `len(Endpoints) == 2` (root + EVSE)
   - Endpoint 0 has type `0x00` (DeviceRoot), Endpoint 1 has type `0x05` (EVCharger)
   - Each feature snapshot has correct feature type ID, featureMap, attributeList, and commandList

2. **TestBuildDeviceSnapshot_EmptyDevice** -- Device with only the root endpoint (no functional endpoints). Assert `len(Endpoints) == 1` and root endpoint has at least DeviceInfo feature.

3. **TestBuildDeviceSnapshot_WithUseCases** -- Create a device, set use case declarations on DeviceInfo. Assert `UseCases` field is populated with correct values.

4. **TestBuildDeviceSnapshot_WithLabel** -- Create an endpoint with a label. Assert `EndpointSnapshot.Label` is populated.

5. **TestBuildDeviceSnapshot_SpecVersion** -- Set specVersion on DeviceInfo. Assert `DeviceSnapshot.SpecVersion` is populated.

### Implementation

```go
func buildDeviceSnapshot(device *model.Device) *log.DeviceSnapshot {
    // Read DeviceInfo for specVersion and useCases
    // Iterate endpoints -> features -> collect featureMap, attributeList, commandList
    // Return populated snapshot
}
```

Key details:
- Access `device.Endpoints()` to iterate all endpoints
- For each endpoint, iterate `endpoint.Features()` to get feature list
- For each feature: `feature.Type()` (cast to uint16 for ID), `feature.FeatureMap()`, `feature.AttributeList()`, `feature.CommandList()`
- For specVersion: read from DeviceInfo feature on endpoint 0 (`features.DeviceInfoAttrSpecVersion`)
- For useCases: read from DeviceInfo feature on endpoint 0 (`features.DeviceInfoAttrUseCases`)

### Done When

`go test ./pkg/service/... -run TestBuildDeviceSnapshot` passes.

---

## Phase 4: Snapshot Tracker (`pkg/service/snapshot_tracker.go`)

The core trigger logic. The tracker is a struct with `onMessageLogged()` and `shouldEmit()` methods. It uses a `timeNow` function field for testability (avoids `time.Sleep` in tests).

### Files Changed

- `pkg/service/snapshot_tracker.go` -- New file

### Tests (`pkg/service/snapshot_tracker_test.go`)

Use a fake clock (`timeNow func() time.Time`) injected into the tracker to make time-based tests deterministic.

1. **TestSnapshotTracker_MaxMessagesTriggersEmission** -- Set policy to `MaxMessages: 10, MaxInterval: 0`. Call `onMessageLogged()` 10 times. Verify the logger received exactly 1 snapshot event. Verify `messageCount` resets to 0 after emission.

2. **TestSnapshotTracker_MaxIntervalTriggersEmission** -- Set policy to `MaxInterval: 5min, MaxMessages: 0, MinMessages: 0`. Advance fake clock by 5 minutes. Log 1 message. Verify snapshot emitted.

3. **TestSnapshotTracker_MinMessagesFloor** -- Set policy to `MaxInterval: 5min, MaxMessages: 0, MinMessages: 50`. Advance clock by 5 minutes. Log only 3 messages. Verify NO snapshot emitted (below floor). Then log 47 more messages (total 50). Advance clock 5 more minutes. Log 1 more. Verify snapshot emitted.

4. **TestSnapshotTracker_BothDisabled** -- Set both `MaxInterval: 0, MaxMessages: 0`. Log 10000 messages, advance time by hours. Verify no snapshot emitted.

5. **TestSnapshotTracker_NilLoggerIsNoop** -- Create tracker with `logger: nil`. Call `onMessageLogged()` multiple times. Verify no panic, no emission.

6. **TestSnapshotTracker_MessageCountResetsAfterEmission** -- After a snapshot triggers via MaxMessages, verify the counter resets and a second snapshot requires another MaxMessages count.

7. **TestSnapshotTracker_TimeResetsAfterEmission** -- After a snapshot triggers via MaxInterval, advance time by MaxInterval again. Verify second snapshot fires.

8. **TestSnapshotTracker_EmittedEventStructure** -- Trigger a snapshot and inspect the logged event. Verify:
   - `Category == log.CategorySnapshot`
   - `Layer == log.LayerService`
   - `Snapshot != nil`
   - `Snapshot.Local != nil`
   - `ConnectionID` is set correctly

9. **TestSnapshotTracker_EmitInitialSnapshot** -- Call `emitInitialSnapshot()`. Verify a snapshot is emitted regardless of message count or time. Verify it sets `lastSnapshot` time to prevent immediate re-trigger.

### Implementation

```go
type snapshotTracker struct {
    policy       SnapshotPolicy
    messageCount int
    lastSnapshot time.Time
    logger       log.Logger
    connID       string
    localRole    log.Role

    localDevice  *model.Device
    remoteCache  *log.DeviceSnapshot

    // For testing
    timeNow func() time.Time
}

func newSnapshotTracker(policy SnapshotPolicy, device *model.Device, logger log.Logger, connID string, role log.Role) *snapshotTracker

func (t *snapshotTracker) onMessageLogged()
func (t *snapshotTracker) shouldEmit() bool
func (t *snapshotTracker) emitSnapshot()
func (t *snapshotTracker) emitInitialSnapshot()
func (t *snapshotTracker) setRemoteCache(snapshot *log.DeviceSnapshot)
```

### Done When

`go test ./pkg/service/... -run TestSnapshotTracker` passes.

---

## Phase 5: Session Integration (`pkg/service/zone_session.go`, `pkg/service/device_session.go`)

Wire the tracker into both session types. The tracker is created when `SetProtocolLogger` is called and emits an initial snapshot immediately.

### Files Changed

- `pkg/service/zone_session.go` -- Add `snapshotTracker` field, initialize in `SetProtocolLogger`, call `onMessageLogged()` in logging paths
- `pkg/service/device_session.go` -- Same pattern

### Tests

#### ZoneSession (`pkg/service/zone_session_test.go`)

1. **TestZoneSession_EmitsInitialSnapshot** -- Create a ZoneSession with a test device. Call `SetProtocolLogger(capturingLogger, connID)`. Verify the logger received exactly 1 event with `Category == CategorySnapshot` and `Snapshot.Local` is populated.

2. **TestZoneSession_SnapshotOnMessageCount** -- Set a policy with `MaxMessages: 5`. After `SetProtocolLogger`, clear the logger. Send 5 messages through the session (simulate via `OnMessage` with encoded requests). Verify a snapshot event appears.

3. **TestZoneSession_NoSnapshotWithoutLogger** -- Create a ZoneSession without calling `SetProtocolLogger`. Process messages normally. Verify no panic.

#### DeviceSession (`pkg/service/device_session_test.go`)

4. **TestDeviceSession_EmitsInitialSnapshot** -- Same pattern as ZoneSession test. Create DeviceSession, set protocol logger, verify initial snapshot.

5. **TestDeviceSession_SnapshotOnMessageCount** -- Same pattern as ZoneSession test.

6. **TestDeviceSession_RemoteCacheInSnapshot** -- Create DeviceSession, set protocol logger. Then call `setRemoteCache(remoteSnapshot)`. Trigger another snapshot. Verify `Snapshot.Remote` is populated.

### Implementation

Both sessions get:
- A `snapshot *snapshotTracker` field
- In `SetProtocolLogger`: create the tracker, call `emitInitialSnapshot()`
- In each logging call (`logResponse`, `logNotification`, `logRequest` in the handler): after the existing `logger.Log()` call, call `t.snapshot.onMessageLogged()`

The `SnapshotPolicy` needs to be passed to the session. Two approaches:
- **Option A**: Add `SetSnapshotPolicy(policy)` method on both sessions, called before `SetProtocolLogger`
- **Option B**: Pass the policy as a parameter to `SetProtocolLogger`

Option A is cleaner since it doesn't change the existing `SetProtocolLogger` signature. The service layer (DeviceService / ControllerService) sets the policy from its config before setting the logger.

### Done When

`go test ./pkg/service/... -run TestZoneSession_.*Snapshot` and `TestDeviceSession_.*Snapshot` pass.

---

## Phase 6: Controller Remote Capability Read (`pkg/service/controller_service.go`)

Extend the post-commissioning flow to read the remote device's full capabilities and cache them on the DeviceSession's snapshot tracker.

### Files Changed

- `pkg/service/controller_service.go` -- Add `readRemoteCapabilities()` method, call it after `checkDeviceVersion()`
- `pkg/service/version_check.go` -- (May be renamed or extended, or new file `pkg/service/capability_read.go`)

### Tests (`pkg/service/capability_read_test.go`)

1. **TestReadRemoteCapabilities_Basic** -- Create a mock device with known endpoints/features. Create a DeviceSession connected to it (using the existing `mockResponseConnection` pattern). Call `readRemoteCapabilities(ctx, session)`. Assert the returned `*log.DeviceSnapshot` has correct endpoints, features, featureMap, attributeList.

2. **TestReadRemoteCapabilities_WithUseCases** -- Same but device has use case declarations. Assert `UseCases` field is populated.

3. **TestReadRemoteCapabilities_ReadFailure** -- Mock connection returns an error for the read. Assert `readRemoteCapabilities` returns nil (graceful degradation, not a fatal error).

4. **TestReadRemoteCapabilities_EmptyDevice** -- Device has only endpoint 0. Assert snapshot has 1 endpoint.

### Implementation

```go
func (s *ControllerService) readRemoteCapabilities(ctx context.Context, session *DeviceSession) *log.DeviceSnapshot {
    // 1. Read DeviceInfo attributes: specVersion, endpoints, useCases
    // 2. For each endpoint, read featureMap, attributeList, commandList from each feature
    // 3. Assemble DeviceSnapshot
    // 4. Cache on session's snapshot tracker
}
```

This is called in `Commission()` and `attemptReconnection()`, after `checkDeviceVersion()` succeeds. The result is stored via `session.snapshot.setRemoteCache(snapshot)`.

The read uses the existing `session.Read()` method:
- Read endpoint 0, feature DeviceInfo, attributes: specVersion (12), endpoints (20), useCases (21)
- Parse the endpoints list to discover endpoint IDs and types
- For each endpoint's features, read global attributes (0xFFFC = featureMap, 0xFFFB = attributeList, 0xFFFA = commandList)

### Done When

`go test ./pkg/service/... -run TestReadRemoteCapabilities` passes.

---

## Phase 7: mash-log Integration (`cmd/mash-log`)

Extend the view and stats commands to understand snapshot events.

### Files Changed

- `cmd/mash-log/commands/view.go` -- Add `formatSnapshotDetails()`, handle `CategorySnapshot` in `formatEvent()`
- `cmd/mash-log/commands/stats.go` -- Track last snapshot per connection, report snapshot count

### Tests

#### View (`cmd/mash-log/commands/view_test.go`)

1. **TestFormatSnapshotEvent** -- Create an `Event` with a populated snapshot. Call `formatEvent()` into a buffer. Assert the output contains:
   - "SNAPSHOT" in the header line
   - Device ID
   - Endpoint listing with types
   - Feature listing with featureMap

2. **TestFormatSnapshotEvent_NoRemote** -- Same but `Remote: nil`. Assert only "Local:" section appears.

3. **TestParseCategoryFlag_Snapshot** -- `parseCategory("snapshot")` returns `log.CategorySnapshot`.

4. **TestFilterBySnapshotCategory** -- Filtering by `CategorySnapshot` only returns snapshot events.

#### Stats (`cmd/mash-log/commands/stats_test.go`)

5. **TestStatsCountsSnapshots** -- Create events including snapshot events. Run stats accumulation. Assert snapshot count is reported in the "Events by Category" section.

6. **TestStatsLastSnapshotPerConnection** -- Create events for 2 connections, each with 1+ snapshots. Assert stats report "Last snapshot:" per connection.

### Implementation

In `view.go`, add to the `formatEvent` switch:

```go
case event.Snapshot != nil:
    typeLabel = "Snapshot"
```

And in the detail switch:

```go
case event.Snapshot != nil:
    formatSnapshotDetails(w, event.Snapshot)
```

```go
func formatSnapshotDetails(w io.Writer, snap *log.CapabilitySnapshotEvent) {
    // Print Local device tree
    // Print Remote device tree (if present)
}
```

Format example:
```
  Local: device-001
    Endpoint 0 (DEVICE_ROOT)
      DeviceInfo [featureMap=0x0001, attrs=12, cmds=0]
    Endpoint 1 (EV_CHARGER) "Wallbox"
      Electrical [featureMap=0x0001, attrs=8, cmds=0]
      Measurement [featureMap=0x0001, attrs=6, cmds=0]
    UseCases: GPL(1.0) scenarios=0x07
  Remote: controller-001
    ...
```

Add `"snapshot"` case to `parseCategory()` in `view.go`.

In `stats.go`, add snapshot tracking to `ConnectionStats`:
```go
SnapshotCount   int
LastSnapshotAt  time.Time
```

### Done When

`go test ./cmd/mash-log/...` passes.

---

## Implementation Order Summary

| Phase | Package | New Files | Key Test Count |
|-------|---------|-----------|----------------|
| 1 | `pkg/log` | -- | 4 |
| 2 | `pkg/service` | -- | 3 |
| 3 | `pkg/service` | `snapshot_builder.go`, `snapshot_builder_test.go` | 5 |
| 4 | `pkg/service` | `snapshot_tracker.go`, `snapshot_tracker_test.go` | 9 |
| 5 | `pkg/service` | -- | 6 |
| 6 | `pkg/service` | `capability_read.go`, `capability_read_test.go` | 4 |
| 7 | `cmd/mash-log` | -- | 6 |
| **Total** | | **4 new files** | **~37 tests** |

## What This Plan Does NOT Include

- **`--annotate` flag for mash-log view**: This is a display enhancement that can be added in a follow-up. It requires snapshot-based name resolution logic that adds complexity without affecting the core snapshot mechanism.
- **Device-side remote cache population**: The design doc notes this is less complete than the controller side. The initial implementation can leave `Remote` nil on the device side and add controller model reading later.
- **Sidecar index file**: Explicitly deferred in the design doc.

## Design Decisions for Implementation

1. **`timeNow` injection on tracker**: Avoids flaky tests. The field defaults to `time.Now` in production.
2. **`buildDeviceSnapshot` as a standalone function**: Testable in isolation without needing a tracker or logger.
3. **`SetSnapshotPolicy` on sessions rather than changing `SetProtocolLogger` signature**: Avoids changing existing callers. Policy is set once from config before the logger is attached.
4. **Graceful degradation in `readRemoteCapabilities`**: If any read fails, the remote cache is nil or partial. Snapshots still emit with whatever data is available. The feature is observability-only and must never break protocol flow.
