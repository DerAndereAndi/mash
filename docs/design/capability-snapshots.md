# Capability Snapshots in Protocol Logs

**Status:** Proposed
**Date:** 2026-02-02
**Related:** DEC-051 (attributeList immutability), DEC-052 (feature-level subscriptions)

## Problem

On long-lived TLS/TCP connections, device capability metadata -- endpoints, features, use cases, scenarios, featureMap, attributeList -- is exchanged once at connection establishment and never repeated. If protocol logs are rotated, truncated, or begin recording mid-session, the initial discovery data is lost.

This is already a known pain point with EEBUS/SPINE, where SPINE's `NodeManagementDetailedDiscovery` is exchanged once and then relied upon for the life of the connection.

The problem affects both sides of the connection (device and controller) and is compounded by the fact that third-party devices control their own logging infrastructure -- we cannot mandate log retention policies.

### What is lost

Without the initial discovery exchange, a log analyst cannot:

- Map endpoint IDs to endpoint types (EV_CHARGER, INVERTER, etc.)
- Know which features exist on which endpoints
- Interpret featureMap bitmaps (which optional feature sets are active)
- Know the complete attributeList (which attributes the device supports)
- Determine declared use cases and scenario bitmaps
- Identify the device (vendor, product, serial, firmware version)
- Distinguish "attribute not supported" from "attribute unchanged"

### What cannot be reliably inferred

Remaining protocol messages (notifications, reads, writes) reveal endpoint/feature/attribute IDs that were actively used, but:

- Endpoints and features with no traffic during the log window are invisible
- The complete attributeList cannot be reconstructed from partial observations
- Use case declarations are never repeated in any existing message
- Device identity information is only in DeviceInfo

### Why Matter's approach does not apply

Matter mitigates this through timeout-forced re-subscriptions (a consequence of UDP transport), which generate fresh priming reports every MaxInterval. MASH runs over TCP with persistent connections, so re-subscriptions do not occur naturally. Matter's Descriptor cluster and wildcard subscriptions are structurally equivalent to MASH's endpoint 0 DeviceInfo -- the fundamental gap is the same.

## Design

### Overview

The application configures the MASH stack with a **snapshot policy** that controls when capability snapshot events are emitted into the protocol log. Each snapshot contains the complete device model of both the local device and the connected remote peer. Snapshots are emitted on a hybrid trigger: whichever of a time interval or a message count threshold is reached first.

### Snapshot Policy

```go
// SnapshotPolicy controls when capability snapshots are emitted to the protocol log.
type SnapshotPolicy struct {
    // MaxInterval is the maximum time between snapshots. The time trigger
    // only fires if at least MinMessages have been logged since the last
    // snapshot. Zero disables time-based triggering.
    MaxInterval time.Duration

    // MaxMessages is the maximum number of protocol messages (requests,
    // responses, notifications) logged for a connection before a snapshot
    // is emitted. Zero disables count-based triggering.
    MaxMessages int

    // MinMessages is the minimum number of messages that must have been
    // logged since the last snapshot for the time-based trigger to fire.
    // This prevents snapshots from dominating the log on near-idle
    // connections where very few messages are exchanged. The message-count
    // trigger (MaxMessages) is not affected by this floor.
    // Zero means the time trigger fires unconditionally.
    MinMessages int
}
```

Default policy:

```go
func DefaultSnapshotPolicy() SnapshotPolicy {
    return SnapshotPolicy{
        MaxInterval:  30 * time.Minute,
        MaxMessages:  1000,
        MinMessages:  50,
    }
}
```

Both MaxInterval and MaxMessages can be independently disabled by setting their value to zero. Setting both to zero disables capability snapshots entirely (except the initial snapshot on session establishment, which always fires).

### Configuration

The policy is set globally on the service configs:

```go
// In DeviceConfig:
SnapshotPolicy SnapshotPolicy

// In ControllerConfig:
SnapshotPolicy SnapshotPolicy
```

The default configs (`DefaultDeviceConfig()`, `DefaultControllerConfig()`) include `DefaultSnapshotPolicy()`.

### Trigger Semantics

The hybrid trigger adapts naturally to the forensic risk:

- **Busy connection** (high message rate, fast log growth, higher rotation risk): MaxMessages threshold triggers frequently, producing snapshots proportional to log volume.
- **Normal connection** (moderate message rate): MaxInterval fires when enough messages have accumulated (above MinMessages floor), providing periodic checkpoints.
- **Quiet connection** (few messages, slow log growth): MaxInterval elapses but MinMessages floor is not met, so the time trigger is suppressed. Snapshots only fire when MaxMessages eventually accumulates. Since the log is barely growing, there is little risk of losing context.
- **Idle connection** (no messages at all): neither trigger fires. The only snapshot is the initial one emitted at session establishment.

The MinMessages floor prevents snapshots from dominating the log on near-idle connections. Without it, a connection exchanging 1 message every 10 minutes would produce a snapshot every 30 minutes -- meaning every third logged event would be a 500-byte snapshot alongside perhaps 300 bytes of actual protocol data. With `MinMessages: 50`, the time trigger is suppressed until meaningful traffic has occurred.

| Traffic pattern | Trigger | Snapshot frequency |
|---|---|---|
| 10 msg/sec (busy) | MaxMessages=1000 | Every ~100 seconds |
| 1 msg/sec (normal) | MaxInterval=30min (1800 msgs > 50 floor) | Every 30 minutes |
| 1 msg/10min (quiet) | MaxInterval suppressed (3 msgs < 50 floor) | Only when 1000 msgs accumulate (~7 days) |
| 0 msgs (idle) | Neither fires | Initial snapshot only |

### Snapshot Event

A new log event category is added to the protocol logging system.

#### Log event structure

```go
// CategorySnapshot is a new event category for capability snapshots.
const CategorySnapshot Category = 4

// CapabilitySnapshotEvent is logged periodically and contains the complete
// device model for both sides of the connection.
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

#### CBOR encoding

The snapshot event is embedded in the standard `log.Event` structure as a new optional field. It uses the same CBOR integer key encoding as all other log events for compactness.

### Snapshot Tracker

Each session (ZoneSession on the device side, DeviceSession on the controller side) contains a snapshot tracker that monitors message flow and triggers emission:

```go
type snapshotTracker struct {
    policy       SnapshotPolicy
    messageCount int
    lastSnapshot time.Time
    logger       log.Logger
    connID       string

    // Capability data sources
    localDevice  *model.Device
    remoteCache  *DeviceSnapshot
}
```

The tracker hooks into the existing protocol logging path. Each time `logRequest`, `logResponse`, or `logNotification` is called in the protocol handler or session, the tracker increments its counter and checks whether a snapshot should be emitted.

```go
func (t *snapshotTracker) onMessageLogged() {
    if t.logger == nil {
        return
    }
    t.messageCount++
    if t.shouldEmit() {
        t.emitSnapshot()
    }
}

func (t *snapshotTracker) shouldEmit() bool {
    // Message count trigger: fires unconditionally when threshold is reached.
    if t.policy.MaxMessages > 0 && t.messageCount >= t.policy.MaxMessages {
        return true
    }
    // Time trigger: fires only if enough messages have been logged since the
    // last snapshot (MinMessages floor). This prevents snapshots from
    // dominating the log on near-idle connections.
    if t.policy.MaxInterval > 0 && time.Since(t.lastSnapshot) >= t.policy.MaxInterval {
        if t.messageCount >= t.policy.MinMessages {
            return true
        }
    }
    return false
}

func (t *snapshotTracker) emitSnapshot() {
    t.messageCount = 0
    t.lastSnapshot = time.Now()

    event := log.Event{
        Timestamp:    time.Now(),
        ConnectionID: t.connID,
        Layer:        log.LayerService,
        Category:     log.CategorySnapshot,
        // ... other standard fields ...
        Snapshot: &log.CapabilitySnapshotEvent{
            Local:  buildDeviceSnapshot(t.localDevice),
            Remote: t.remoteCache,
        },
    }
    t.logger.Log(event)
}
```

### Emission Points

1. **Session establishment**: The first snapshot is always emitted immediately when a session is created and the protocol logger is set. This initial snapshot is exempt from the MinMessages floor -- it fires unconditionally to ensure every log file starts with context.

2. **After remote capability discovery**: On the controller side, after reading the remote device's DeviceInfo and per-endpoint global attributes, the remote cache is populated and a snapshot is emitted (this may be the same as or shortly after point 1).

3. **Ongoing**: The hybrid trigger fires on whichever threshold is reached first.

4. **Reconnection**: When a connection is re-established, the remote cache is refreshed (capabilities may have changed if the device rebooted with new firmware) and a new snapshot is emitted immediately.

### Remote Capability Cache

#### Controller side

After commissioning or reconnection, the controller reads the remote device's capabilities. This extends the existing `checkDeviceVersion()` pattern in `controller_service.go`:

1. Read all DeviceInfo attributes (endpoint 0, feature 1) -- gets endpoints, useCases, specVersion, identity info
2. For each endpoint in the device's endpoint list, read featureMap, attributeList, and commandList from each feature

The result is cached on the DeviceSession as `remoteCache *DeviceSnapshot`. Since capabilities are immutable per connection (DEC-051), this cache is valid for the lifetime of the session.

#### Device side

The device always knows its own model (in memory). For the remote side (the connected controller):

- Zone ID and zone type are known from the commissioning certificate
- If the controller exposes a device model (bidirectional communication), the device can read it and cache it
- Otherwise, the remote snapshot contains the zone identity and any information observable from the controller's behavior (subscribed features, etc.)

The remote snapshot on the device side will be less complete than on the controller side, which is acceptable -- the controller typically holds the richer view.

### Wire Overhead

None. This is purely a logging-layer mechanism. No additional protocol messages are exchanged for snapshot emission. The only wire cost is the initial capability read on the controller side, which extends the existing version check by reading additional attributes.

### Log Overhead

A typical snapshot for an EVSE device (2 endpoints, 5 features):
- Local snapshot: ~200-300 bytes CBOR
- Remote snapshot: ~200-300 bytes CBOR
- Total per emission: ~400-600 bytes

With default policy (MaxInterval=30min, MaxMessages=1000, MinMessages=50):
- Busy connection (10 msg/sec): snapshot every ~100 sec, ~22 KB/hour
- Normal connection (1 msg/sec): snapshot every 30 min, ~1.2 KB/hour
- Quiet connection (1 msg/10min): snapshot only every ~7 days (when 1000 msgs accumulate), negligible
- Idle connection: initial snapshot only (~500 bytes total)

On busy connections where log loss is most likely, snapshots are frequent. On quiet connections where the log barely grows, snapshots are rare. The overhead is always proportional to the forensic risk.

### mash-log Integration

The `mash-log` tool is extended to understand snapshot events:

- **`mash-log view`**: Formats snapshot events with a device tree display showing endpoints, features, and capabilities.
- **`mash-log stats`**: Reports "last capability snapshot at: ..." per connection, and the gap since the last snapshot.
- **`mash-log view --annotate`**: Uses the nearest preceding snapshot to annotate subsequent events with human-readable names (endpoint types, feature names, attribute names) alongside raw numeric IDs.

### Backward Compatibility

- Snapshot events use a new `CategorySnapshot` value. Older versions of `mash-log` that do not recognize this category will skip these events gracefully (the CBOR decoder ignores unknown fields).
- The `.mlog` file format remains append-only CBOR events. No structural changes to the format.
- Devices and controllers that do not configure a snapshot policy simply never emit snapshot events. The protocol logger interface (`log.Logger`) is unchanged.

## Files Affected

| File | Change |
|------|--------|
| `pkg/log/event.go` | Add `CategorySnapshot`, `CapabilitySnapshotEvent`, and related snapshot types |
| `pkg/service/types.go` | Add `SnapshotPolicy` struct, add field to `DeviceConfig` and `ControllerConfig`, add `DefaultSnapshotPolicy()` |
| `pkg/service/snapshot_tracker.go` | New file: snapshot tracker with hybrid trigger logic |
| `pkg/service/zone_session.go` | Integrate snapshot tracker, emit on session start |
| `pkg/service/device_session.go` | Integrate snapshot tracker, emit on session start and after discovery |
| `pkg/service/controller_service.go` | Read remote device capabilities after commissioning/reconnection, populate cache |
| `cmd/mash-log/commands/view.go` | Format snapshot events |
| `cmd/mash-log/commands/stats.go` | Report snapshot statistics per connection |

## Alternatives Considered

### Protocol-level metadata snapshot message

A new control message type emitted periodically on the wire. Rejected because:
- Requires a protocol spec change affecting all implementations
- The problem is fundamentally about log observability, not protocol correctness
- Wire overhead (even if small) is unnecessary when the data is already in memory locally

### Enhanced subscription heartbeats

Include global attributes (featureMap, attributeList) in every heartbeat notification. Partially rejected because:
- Per-feature only -- no single message captures the full device model
- Does not include endpoint structure or use case declarations
- The current implementation already includes global attributes in subscribe-all heartbeats (DEC-052), so the gap is smaller than it appears

Note: Enhanced heartbeats remain a valid complementary improvement but do not replace snapshots for full device model reconstruction.

### Capability inference from partial logs

A `mash-log reconstruct` command that infers capabilities from protocol traffic patterns. Rejected as a primary solution because:
- Best-effort only -- cannot discover endpoints/features with no traffic
- Cannot reconstruct use case declarations or device identity
- High implementation complexity for uncertain results

May be valuable as a supplementary tool for analyzing third-party logs where snapshots are unavailable.

### Sidecar index file

A `.mlog.idx` file alongside the `.mlog` that stores the latest snapshot per connection. Not included in this design but could be added as a future optimization for deployments with active log rotation.
