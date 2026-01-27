# Multi-Controller Scenario: SMGW + EMS + Devices

**Date:** 2025-01-24
**Context:** Real-world deployment with grid operator (SMGW) and home energy manager (EMS)

---

## The Scenario

```
┌──────────────────────────────────────────────────────────────┐
│                         DSO Backend                           │
│                    (Utility Cloud System)                     │
└──────────────────────────┬───────────────────────────────────┘
                           │ Internet
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                          SMGW                                 │
│               (Smart Meter Gateway)                           │
│                                                               │
│  - No user interface                                          │
│  - Commissioned via backend                                   │
│  - Grid operator priority for load control                    │
│  - Regulatory authority                                       │
└──────────────────────────┬───────────────────────────────────┘
                           │ Local Network
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                          EMS                                  │
│              (Energy Management System)                       │
│                                                               │
│  - User interface (app, web)                                  │
│  - Home optimization                                          │
│  - User preferences                                           │
│  - Lower priority than SMGW for grid functions                │
└──────────────────────────┬───────────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         ┌────────┐   ┌────────┐   ┌────────┐
         │  EVSE  │   │ HeatPump│  │Battery │
         │        │   │        │   │        │
         └────────┘   └────────┘   └────────┘
              │
         QR Code on device
```

---

## Key Questions

### 1. How does SMGW get commissioned without UI?

**Problem:** SMGW can't scan QR codes. Setup code is on the device.

**Solution: Delegated Commissioning**

```
User                Phone App           DSO Backend         SMGW              Device
 │                      │                    │               │                  │
 │── Scan QR Code ─────►│                    │               │                  │
 │                      │                    │               │                  │
 │                      │── Upload setup ───►│               │                  │
 │                      │   info to backend  │               │                  │
 │                      │                    │               │                  │
 │                      │                    │── Send setup ─►│                 │
 │                      │                    │   info to SMGW │                  │
 │                      │                    │               │                  │
 │                      │                    │               │── Commission ───►│
 │                      │                    │               │   with setup     │
 │                      │                    │               │   code           │
 │                      │                    │               │◄── Accept ───────┤
```

**QR Code Contains:**
- Setup code (for SPAKE2+)
- Discriminator (to find device)
- Device ID / Serial number
- Vendor ID + Product ID

**Backend Workflow:**
1. User installs device, scans QR with utility app
2. App uploads: setup code + device info to DSO backend
3. Backend securely forwards to SMGW
4. SMGW uses setup code to commission device
5. No user interaction needed on device

---

### 2. How do multiple controllers coexist?

**Option A: Single Fabric with Roles**

```
┌─────────────────────────────────────────┐
│            Single Fabric                 │
│                                          │
│  Fabric CA (owned by SMGW or EMS?)      │
│    │                                     │
│    ├── SMGW (GridOperator role)         │
│    ├── EMS (HomeManager role)           │
│    └── Devices                          │
└─────────────────────────────────────────┘
```

**Problems:**
- Who owns the Fabric CA?
- If SMGW owns it, what happens when SMGW is replaced?
- If EMS owns it, can SMGW trust the certs?
- Tight coupling between operators

---

**Option B: Multiple Fabrics (Recommended)**

```
┌─────────────────────────────────────────┐
│              Device (EVSE)               │
│                                          │
│  Fabric 1: Grid Operator                 │
│    └── Operational Cert from SMGW       │
│    └── Access: LoadControl, Measurement │
│                                          │
│  Fabric 2: Home Automation               │
│    └── Operational Cert from EMS        │
│    └── Access: All clusters             │
│                                          │
│  Priority: Fabric 1 > Fabric 2          │
│            for LoadControl cluster       │
└─────────────────────────────────────────┘
```

**Benefits:**
- Each controller is independent
- Device has separate operational cert per fabric
- No coupling between SMGW and EMS
- SMGW replacement doesn't affect EMS
- Clear authority boundaries

**How it works:**
1. Device can be in multiple fabrics simultaneously
2. Each fabric has its own CA, issues its own operational certs
3. Device stores multiple (fabric_id → operational_cert) mappings
4. Connections authenticated by fabric membership
5. Priority resolved per-cluster based on fabric type

---

### 3. How is priority enforced?

**Per-Cluster Priority Matrix:**

| Cluster | Grid (SMGW) | Local (EMS) |
|---------|-------------|-------------|
| LoadControl | **Priority 1** | Priority 2 |
| Measurement | Read-only | Read + Subscribe |
| ChargingSession | Override only | Full control |
| DeviceInfo | Read-only | Read-only |
| UserPreferences | No access | Full control |

**Fabric Types (announced during commissioning):**
```
enum FabricType {
  GRID = 1,   // DSO, utility, SMGW - external/regulatory authority
  LOCAL = 2,  // Residential/building EMS - local energy management
}
```

**Priority Resolution:**
1. When multiple fabrics want to control same cluster
2. Device checks fabric type of each requester
3. Lower fabric type number = higher priority
4. Higher priority can override lower priority's settings
5. Lower priority is notified of override

---

### 4. Commissioning Flow for Multi-Fabric

**Phase 1: User commissions device to EMS**
```
User                    EMS                  Device
 │                       │                     │
 │── Scan QR code ──────►│                     │
 │   (via EMS app)       │                     │
 │                       │── SPAKE2+ ─────────►│
 │                       │◄── Accept ──────────┤
 │                       │── Install OpCert ──►│  (Fabric: LOCAL)
 │                       │◄── Done ────────────┤
```

**Phase 2: SMGW commissioned via backend**
```
User              Phone App        Backend         SMGW           Device
 │                    │              │              │               │
 │── Scan QR ────────►│              │              │               │
 │   (utility app)    │              │              │               │
 │                    │── Upload ───►│              │               │
 │                    │              │── Forward ──►│               │
 │                    │              │              │── SPAKE2+ ───►│
 │                    │              │              │◄── Accept ────┤
 │                    │              │              │── OpCert ────►│  (Fabric: GRID)
```

**Result: Device has two fabrics**
```
Device State:
  Fabrics:
    - fabric_1: {type: GRID, ca: smgw_ca, cert: op_cert_1}
    - fabric_2: {type: LOCAL, ca: ems_ca, cert: op_cert_2}
```

---

## Proposed Architecture

### Device Data Model

```
Device
├── Endpoint 0 (Root)
│   ├── DeviceInfo cluster
│   ├── FabricManagement cluster  ◄── NEW: manages multi-fabric
│   │   ├── fabrics: []
│   │   ├── AddFabric command
│   │   ├── RemoveFabric command
│   │   └── ListFabrics command
│   └── AccessControl cluster     ◄── NEW: per-fabric permissions
│       ├── entries: []
│       └── DefaultPolicy
│
├── Endpoint 1 (Charger)
│   ├── Measurement cluster
│   ├── LoadControl cluster
│   │   ├── attribute: activeLimits
│   │   ├── attribute: limitSource  ◄── Which fabric set this?
│   │   └── command: SetLimit
│   └── ChargingSession cluster
```

### Access Control Entry

```cbor
{
  "fabric": 1,                    // Which fabric this applies to
  "cluster": "LoadControl",       // Which cluster
  "permissions": ["read", "write", "invoke"],
  "priority": 1                   // Override priority
}
```

### Limit Source Tracking

```cbor
{
  "limitId": 1,
  "value": 7400,
  "unit": "W",
  "source": {
    "fabric": 1,                  // Grid operator set this
    "timestamp": 1706108400,
    "reason": "GridOverload"
  }
}
```

---

## Manageable Setup for Users

### Ideal User Experience

1. **Install device** (EVSE, heat pump, etc.)

2. **Scan QR with EMS app** (if user has EMS)
   - Device joins home fabric
   - EMS can optimize charging

3. **Scan QR with utility app** (optional, or installer does it)
   - Device info sent to backend
   - SMGW auto-commissions device
   - No user action needed on device

4. **Done** - device works with both controllers
   - EMS optimizes based on solar, tariffs
   - SMGW can override for grid emergencies
   - User sees both in their respective apps

### What the User Sees

**In EMS App:**
```
Devices:
  ✓ EV Charger (Wallbox)
    Status: Charging at 7.4 kW
    Controlled by: Home Optimization
    Grid Override: None
```

**In Utility App:**
```
Smart Grid Devices:
  ✓ EV Charger (registered)
    Grid Status: Available for flex
    Current Limit: None active
```

**During Grid Event:**
```
EMS App:
  ⚠️ EV Charger
    Status: Limited to 3.7 kW
    Reason: Grid operator override
    Expected duration: 30 min
```

---

## Open Design Questions

### Q1: Maximum number of fabrics per device?

**Options:**
- Fixed: 3 fabrics max (grid, building, user)
- Configurable: Device announces max in DeviceInfo
- Unlimited: Limited only by storage

**Recommendation:** 5 fabrics max (covers multiple GRID and LOCAL zones)

### Q2: Can fabrics see each other?

**Options:**
- Isolated: Each fabric only sees its own state
- Visible: Fabrics can see who else controls device
- Partial: Can see fabric types but not details

**Recommendation:** Visible fabric types for coordination

### Q3: What if SMGW and EMS both set limits?

**Resolution:**
1. Most restrictive limit applies (min of all limits)
2. OR Priority-based (higher priority wins)
3. OR Per-fabric limits tracked separately

**Recommendation:** Priority-based - SMGW limit overrides EMS limit

### Q4: Setup code reuse across fabrics?

**Options:**
- Single-use: New setup code needed per fabric (more secure)
- Reusable: Same code works for multiple fabrics (easier)
- Time-limited: Code valid for 24 hours after first use

**Recommendation:** Time-limited (24 hours) for practical deployment

---

## Summary

**Key Design Decisions Needed:**

1. **Multi-fabric support** - Device can be in multiple fabrics simultaneously
2. **Fabric types** - Enum defining priority (GRID > LOCAL)
3. **Per-cluster priority** - Not global, different clusters may have different priority
4. **Delegated commissioning** - QR code info can be uploaded to backend
5. **Limit source tracking** - Device tracks which fabric set each limit
6. **Setup code validity** - Time-limited reuse for practical deployment
