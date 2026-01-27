# Implementation Plan: Pairing Request Mechanism & Zone Type Constraint

> TDD implementation plan for DEC-042 (Pairing Request) and DEC-043 (Zone Type Constraint)

**Created:** 2026-01-27
**Completed:** 2026-01-27
**Status:** Complete

---

## Overview

This plan implements two key features from commit `714dd2b`:

1. **Pairing Request Mechanism (DEC-042)**: New `_mashp._udp` mDNS service for deferred commissioning scenarios (SMGW, backend-provisioned devices)
2. **Zone Type Constraint (DEC-043)**: Limit devices to max 2 zones (one GRID + one LOCAL)

---

## Implementation Order

### Phase 1: Foundation (Parallel)

| ID | Task | Package | Status |
|----|------|---------|--------|
| 1 | Create `PairingRequestInfo` and `PairingRequestService` types | `pkg/discovery` | Done |
| 2 | Implement `ErrZoneTypeExists` error and zone type enforcement | `pkg/zone` | Done |
| 7 | Add commissioning window expiration event | `pkg/service` | Done |

### Phase 2: Discovery Layer

| ID | Task | Blocked By | Status |
|----|------|------------|--------|
| 3 | Implement `AnnouncePairingRequest` for controllers | #1 | Done |
| 4 | Implement `BrowsePairingRequests` for devices | #1 | Done |
| 8 | Update commissioning error codes for `ZONE_TYPE_EXISTS` | #2 | Done |

### Phase 3: Service Integration

| ID | Task | Blocked By | Status |
|----|------|------------|--------|
| 5 | Integrate pairing request into controller commissioning flow | #3 | Done |
| 6 | Integrate pairing request listening into device service | #4, #7 | Done |

### Phase 4: Validation

| ID | Task | Blocked By | Status |
|----|------|------------|--------|
| 9 | Add integration tests for complete pairing request flow | #5, #6, #8 | Done |

---

## Task Details

### Task 1: Create PairingRequestInfo and PairingRequestService Types

**File:** `pkg/discovery/types.go`

Add new types:

```go
// PairingRequestInfo contains information for announcing a pairing request.
type PairingRequestInfo struct {
    // Discriminator is the target device discriminator (0-4095).
    Discriminator uint16

    // ZoneID is the requesting zone's ID (16 hex chars).
    ZoneID string

    // ZoneName is the optional user-friendly zone name.
    ZoneName string

    // Host is the hostname to advertise.
    Host string

    // Port is set to 0 (signaling only).
    Port uint16
}

// PairingRequestService represents a pairing request found via mDNS.
type PairingRequestService struct {
    // InstanceName is the mDNS instance name (e.g., "A1B2C3D4E5F6A7B8-1234").
    InstanceName string

    // Host is the hostname.
    Host string

    // Port is the service port (always 0).
    Port uint16

    // Addresses contains resolved IP addresses.
    Addresses []string

    // Discriminator is the target discriminator (from TXT "D").
    Discriminator uint16

    // ZoneID is the requesting zone ID (from TXT "ZI").
    ZoneID string

    // ZoneName is the optional zone name (from TXT "ZN").
    ZoneName string
}
```

**Tests:**
- Validate discriminator range (0-4095)
- Validate ZoneID format (16 hex chars)
- Test TXT record building

---

### Task 2: Implement ErrZoneTypeExists and Zone Type Enforcement

**Files:** `pkg/zone/errors.go`, `pkg/zone/manager.go`

Add error:
```go
var ErrZoneTypeExists = errors.New("zone of this type already exists")
```

Update `AddZone()`:
```go
func (m *Manager) AddZone(zoneID string, zoneType ZoneType) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check duplicate ID
    if _, exists := m.zones[zoneID]; exists {
        return ErrZoneExists
    }

    // Check zone type constraint (before capacity check)
    for _, z := range m.zones {
        if z.Type == zoneType {
            return ErrZoneTypeExists
        }
    }

    // Check capacity
    if len(m.zones) >= MaxZones {
        return ErrMaxZonesExceeded
    }

    // ... create zone
}
```

**Tests:**
- Add GRID zone, then add another GRID -> `ErrZoneTypeExists`
- Add GRID zone, then add LOCAL -> success
- Verify error order: type check before capacity check

---

### Task 3: Implement AnnouncePairingRequest

**File:** `pkg/discovery/advertiser.go`

Add methods:
```go
func (a *Advertiser) AnnouncePairingRequest(info PairingRequestInfo) error
func (a *Advertiser) StopPairingRequest(discriminator uint16) error
```

**Service format:**
- Instance name: `<zone-id>-<discriminator>._mashp._udp.local`
- SRV port: 0 (signaling only)
- TXT records: `D=<discriminator>`, `ZI=<zone-id>`, `ZN=<zone-name>`

**Tests:**
- Correct mDNS service creation
- TXT record format matches spec
- Stop removes announcement
- Multiple concurrent requests supported

---

### Task 4: Implement BrowsePairingRequests

**File:** `pkg/discovery/browser.go`

Add method:
```go
func (b *Browser) BrowsePairingRequests(ctx context.Context, callback func(PairingRequestService)) error
```

**Tests:**
- Parse valid TXT records
- Filter by discriminator
- Callback invoked on discovery
- Handle malformed TXT records gracefully

---

### Task 5: Integrate Pairing Request into Controller

**File:** `pkg/service/controller_service.go`

Update `CommissionDevice()`:
1. Browse `_mashc._udp` for device
2. If not found: announce `_mashp._udp` with target discriminator
3. Re-announce every 2 minutes (PairingRequestTTL)
4. When device appears: stop pairing request, proceed with PASE
5. Configurable timeout (default: 1 hour interactive, 7 days SMGW)

Add method:
```go
func (c *ControllerService) CancelCommissioning(discriminator uint16) error
```

**Tests:**
- Device already advertising -> direct commission
- Device not advertising -> pairing request -> device appears -> commission
- Timeout triggers cancel
- CancelCommissioning stops pairing request

---

### Task 6: Integrate Pairing Request Listening into Device

**File:** `pkg/service/device_service.go`

Update device to:
1. Listen for `_mashp._udp` when uncommissioned OR accepting additional zones
2. On matching discriminator: open commissioning window (3h default)
3. Start advertising `_mashc._udp`
4. Rate limit: ignore requests while window already open
5. Stop listening when at max zones (2)

**Tests:**
- Uncommissioned device receives request -> opens window
- Device with one zone receives request -> opens window
- Device with both zones ignores requests
- Rate limiting works
- Window duration configurable (1-24h)

---

### Task 7: Add Commissioning Window Expiration Event

**File:** `pkg/service/device_service.go`

Add handling:
1. Timer for window expiration
2. Emit `EventCommissioningClosed` with reason ("timeout" or "commissioned")
3. Stop `_mashc._udp` advertisement
4. Return to pairing request listening state if applicable

**Tests:**
- Expiration emits event
- Reason field correct
- mDNS stops on expiration
- Returns to listening state

---

### Task 8: Update Commissioning Error Codes

**File:** `pkg/commissioning/errors.go`

Add error code:
```go
const (
    // ... existing codes
    ErrorZoneTypeExists = 10  // Device already has a zone of this type
)
```

Update CERT_INSTALL handling to check zone type and return appropriate error.

**Tests:**
- Device with GRID rejects second GRID
- Error code 10 in CERT_ACK
- Controller reports error appropriately

---

### Task 9: Integration Tests

**File:** `pkg/service/commissioning_integration_test.go`

Test scenarios:

**Scenario A: Standard commissioning**
1. Device opens window
2. Controller browses, finds, commissions

**Scenario B: Deferred commissioning**
1. Controller has QR data, device not advertising
2. Controller announces pairing request
3. Device sees request, opens window
4. Controller discovers, commissions

**Scenario C: SMGW-style**
1. Controller receives credentials
2. Device offline for extended period
3. Device powers on, sees request
4. Commissioning completes

---

## Files to Create/Modify

| Package | Files |
|---------|-------|
| `pkg/discovery` | `types.go`, `advertiser.go`, `browser.go`, `*_test.go` |
| `pkg/zone` | `errors.go`, `manager.go`, `*_test.go` |
| `pkg/service` | `controller_service.go`, `device_service.go`, `commissioning_integration_test.go` |
| `pkg/commissioning` | `errors.go` |

---

## Test Cases from Spec

| ID | Description | Expected |
|----|-------------|----------|
| TC-ZONE-2 | Commission second GRID zone | `ZONE_TYPE_EXISTS` error |
| TC-ZONE-3 | Commission when both slots full | `ZONE_FULL` error |
| TC-ZONE-1 | Commission LOCAL after GRID | Both zones active |

---

## Related Documents

- [DEC-042: Pairing Request Mechanism](decision-log.md#dec-042-pairing-request-mechanism-for-deferred-commissioning)
- [DEC-043: One Zone Per Zone Type Constraint](decision-log.md#dec-043-one-zone-per-zone-type-constraint)
- [Discovery Specification](discovery.md)
- [Commissioning Behavior](testing/behavior/commissioning-pase.md)
- [Zone Management Behavior](testing/behavior/zone-management.md)
