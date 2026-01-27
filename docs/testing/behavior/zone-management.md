# Zone Management Behavior

> Precise specification of zone state management and operations

**Status:** Draft
**Created:** 2025-01-27

---

## 1. Overview

A MASH device can belong to **up to 5 zones** simultaneously. Each zone represents a controller relationship with its own trust domain and priority level. This document specifies the operational behaviors of zone management.

**Related documents:**
- `zone-lifecycle.md` - Certificate and commissioning lifecycle
- `multi-zone-resolution.md` - Limit/setpoint resolution rules

**Reference implementation:** `pkg/zone/manager.go`

---

## 2. Zone Data Model

### 2.1 Zone Structure

Each zone membership tracks:

```go
type Zone struct {
    ID             string         // Unique zone identifier (from Zone CA fingerprint)
    Type           ZoneType       // Determines priority
    Connected      bool           // Active connection to controller
    LastSeen       time.Time      // Last activity timestamp
    CommissionedAt time.Time      // When zone was added
}
```

### 2.2 Zone Types and Priority

Priority is determined by zone type (lower number = higher priority):

| Zone Type | Priority | Constant | Description |
|-----------|----------|----------|-------------|
| GRID | 1 (highest) | `ZoneTypeGrid` | External/regulatory authority (DSO, utility, SMGW) |
| LOCAL | 2 | `ZoneTypeLocal` | Local energy management (home EMS, building EMS) |

**Priority method:**
```go
func (zt ZoneType) Priority() uint8 {
    return uint8(zt)  // Priority equals the type value
}
```

---

## 3. Zone Capacity

### 3.1 Maximum Zones

```go
const MaxZones = 5
```

**Constraint:** A device MUST support up to 5 simultaneous zone memberships.

**Rationale:** This allows for typical deployment scenarios:
- Multiple GRID zones (e.g., DSO + regional grid operator)
- Multiple LOCAL zones (e.g., home EMS + building EMS)

### 3.2 Capacity Enforcement

When `AddZone` is called and the device already has 5 zones:

```go
if len(m.zones) >= MaxZones {
    return ErrMaxZonesExceeded
}
```

**Behavior:** Return error `ErrMaxZonesExceeded` without modifying state.

---

## 4. Zone Operations

### 4.1 AddZone

**Signature:** `AddZone(zoneID string, zoneType ZoneType) error`

**Preconditions:**
- Zone with `zoneID` does not already exist
- Current zone count < MaxZones (5)

**Behavior:**
1. Check for duplicate zone ID
2. Check capacity limit
3. Create zone with:
   - `ID` = provided zoneID
   - `Type` = provided zoneType
   - `Connected` = false
   - `CommissionedAt` = current time
4. Add zone to internal map
5. Invoke `onZoneAdded` callback if registered

**Errors:**
| Error | Condition |
|-------|-----------|
| `ErrZoneExists` | Zone with same ID already exists |
| `ErrMaxZonesExceeded` | Already at 5 zones |

**Note:** `AddZone` does NOT automatically mark the zone as connected. The commissioning flow must call `SetConnected` after certificate installation.

### 4.2 RemoveZone

**Signature:** `RemoveZone(zoneID string) error`

**Preconditions:**
- Zone with `zoneID` exists
- Request comes from the zone being removed (self-removal)

**Behavior:**
1. Verify zone exists
2. Delete zone from internal map
3. Invoke `onZoneRemoved` callback if registered

**Errors:**
| Error | Condition |
|-------|-----------|
| `ErrZoneNotFound` | Zone does not exist |

**Note:** `RemoveZone` is for self-removal only - a zone removing itself from a device.

---

## 5. Connection State Tracking

### 5.1 SetConnected

**Signature:** `SetConnected(zoneID string) error`

**Behavior:**
1. Verify zone exists
2. Set `zone.Connected = true`
3. Update `zone.LastSeen` to current time
4. Invoke `onConnect` callback if registered

**When to call:** After TLS connection established with mutual certificate authentication.

### 5.2 SetDisconnected

**Signature:** `SetDisconnected(zoneID string) error`

**Behavior:**
1. Verify zone exists
2. Set `zone.Connected = false`
3. Invoke `onDisconnect` callback if registered

**When to call:** After TLS connection closes (graceful or abrupt).

**Note:** `SetDisconnected` does NOT update `LastSeen`. The last activity time is preserved.

### 5.3 UpdateLastSeen

**Signature:** `UpdateLastSeen(zoneID string) error`

**Behavior:**
1. Verify zone exists
2. Set `zone.LastSeen` to current time

**When to call:** On any protocol activity (messages received, keep-alive, etc.).

### 5.4 Connection State vs Zone Membership

| State | Zone Exists | Connected | Meaning |
|-------|-------------|-----------|---------|
| Commissioned, disconnected | Yes | No | Zone added but controller offline |
| Commissioned, connected | Yes | Yes | Active communication |
| Not commissioned | No | N/A | No relationship |

**Important:** Zone membership persists across connections. Disconnection does NOT remove the zone.

---

## 6. Query Operations

### 6.1 GetZone

Returns a specific zone by ID.

```go
zone, err := m.GetZone("zone-id")
// Returns ErrZoneNotFound if not found
```

### 6.2 HasZone

Checks if a zone exists (boolean).

```go
if m.HasZone("zone-id") {
    // Zone exists
}
```

### 6.3 ListZones

Returns all zone IDs.

```go
ids := m.ListZones()  // []string
```

### 6.4 AllZones

Returns all zone objects.

```go
zones := m.AllZones()  // []*Zone
```

### 6.5 ConnectedZones

Returns only connected zone IDs.

```go
ids := m.ConnectedZones()  // []string of connected zones only
```

### 6.6 ZoneCount

Returns total number of zones.

```go
count := m.ZoneCount()  // 0-5
```

### 6.7 HighestPriorityZone

Returns the zone with highest priority (lowest number).

```go
zone := m.HighestPriorityZone()
// Returns nil if no zones exist
```

### 6.8 HighestPriorityConnectedZone

Returns the connected zone with highest priority.

```go
zone := m.HighestPriorityConnectedZone()
// Returns nil if no zones are connected
```

---

## 7. Event Callbacks

### 7.1 Available Callbacks

| Callback | Signature | Trigger |
|----------|-----------|---------|
| `OnZoneAdded` | `func(zone *Zone)` | After zone added |
| `OnZoneRemoved` | `func(zoneID string)` | After zone removed |
| `OnConnect` | `func(zoneID string)` | After SetConnected |
| `OnDisconnect` | `func(zoneID string)` | After SetDisconnected |

### 7.2 Callback Invocation

Callbacks are invoked:
- **Synchronously** (within the operation, under lock)
- **After state change** (state is consistent when callback runs)
- **Only if registered** (nil callbacks are not invoked)

### 7.3 Use Cases

| Callback | Typical Use |
|----------|-------------|
| `OnZoneAdded` | Update mDNS zone count, initialize zone-specific state |
| `OnZoneRemoved` | Clean up subscriptions, update mDNS |
| `OnConnect` | Start failsafe timer reset, notify other features |
| `OnDisconnect` | Start failsafe timer, pause certain operations |

---

## 8. Thread Safety

### 8.1 Synchronization

All Manager methods are thread-safe. The implementation uses:
- `sync.RWMutex` for protecting the zones map
- Read operations use `RLock`/`RUnlock`
- Write operations use `Lock`/`Unlock`

### 8.2 Callback Safety

Callbacks are invoked while holding the lock. Callback implementations:
- MUST NOT call back into the Manager (would deadlock)
- SHOULD be quick (blocks other operations)
- MAY spawn goroutines for longer work

---

## 9. PICS Items

```
# Zone capacity
MASH.S.ZONE                      # Multi-zone support present
MASH.S.ZONE.MAX=5                # Maximum zones per device

# Zone types supported
MASH.S.ZONE.GRID                 # GRID zone (priority 1) - external/regulatory
MASH.S.ZONE.LOCAL                # LOCAL zone (priority 2) - local energy management

# Zone operations
MASH.S.ZONE.ADD                  # AddZone operation
MASH.S.ZONE.REMOVE               # RemoveZone operation (self-removal)

# Connection tracking
MASH.S.ZONE.CONNECT              # Connection state tracking
MASH.S.ZONE.LAST_SEEN            # LastSeen timestamp tracking

# Priority behaviors
MASH.S.ZONE.B_PRIORITY           # Highest priority zone wins (setpoints)
MASH.S.ZONE.B_RESTRICT           # Most restrictive wins (limits)
```

---

## 10. Test Cases

| ID | Name | Description | Expected |
|----|------|-------------|----------|
| TC-ZONE-MGT-001 | AddZone success | Add first zone | Zone added, Connected=false |
| TC-ZONE-MGT-002 | AddZone duplicate | Add same zone twice | ErrZoneExists |
| TC-ZONE-MGT-003 | AddZone at capacity | Add 6th zone | ErrMaxZonesExceeded |
| TC-ZONE-MGT-004 | RemoveZone success | Remove existing zone | Zone removed |
| TC-ZONE-MGT-005 | RemoveZone not found | Remove non-existent | ErrZoneNotFound |
| TC-ZONE-MGT-006 | SetConnected | Mark zone connected | Connected=true, LastSeen updated |
| TC-ZONE-MGT-007 | SetDisconnected | Mark zone disconnected | Connected=false, LastSeen preserved |
| TC-ZONE-MGT-008 | HighestPriorityZone | Query with multiple zones | Returns lowest priority number |
| TC-ZONE-MGT-009 | HighestPriorityConnectedZone | Query with mixed connection | Returns connected zone with lowest priority |

---

## 11. Error Definitions

```go
var (
    ErrZoneNotFound       = errors.New("zone not found")
    ErrZoneExists         = errors.New("zone already exists")
    ErrMaxZonesExceeded   = errors.New("maximum zones exceeded")
    ErrInsufficientPriority = errors.New("insufficient priority")
    ErrZoneNotConnected   = errors.New("zone not connected")
)
```

---

## 12. Implementation Notes

### 12.1 Zone ID Source

Zone ID is derived from the Zone CA certificate fingerprint:
```
Zone ID = SHA-256(Zone CA Certificate DER)[0:8]
```
This provides a unique, verifiable identifier tied to the zone's cryptographic identity.

### 12.2 Zone Type from Certificate

Zone type is embedded in the Zone CA certificate and extracted during commissioning. It cannot be changed after the zone is created.

### 12.3 Persistence

Zone state should be persisted to survive device restart:
- Zone ID, type, commissioned timestamp
- Connection state is transient (start disconnected on boot)
- LastSeen is transient (reset on boot)
