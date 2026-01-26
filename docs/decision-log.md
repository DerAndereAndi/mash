# MASH Protocol Decision Log

> Tracking what we evaluated, decided, and declined

**Created:** 2025-01-24
**Last Updated:** 2026-01-26

---

## How to Use This Document

Each decision is logged with:
- **Context**: Why we considered this
- **Options Evaluated**: What alternatives we looked at
- **Decision**: What we chose
- **Rationale**: Why we chose it
- **Declined Alternatives**: What we rejected and why

---

## Decisions

### DEC-001: Protocol Naming

**Date:** 2025-01-24
**Status:** Proposed

**Context:** Need a working name for the new protocol.

**Options Evaluated:**
1. MASH (Minimal Application-layer Smart Home)
2. LEAP (Lightweight Energy Application Protocol)
3. SIMPLE (Smart Interoperable Minimal Protocol for Local Energy)

**Decision:** MASH (working name, subject to change)

**Rationale:**
- Memorable and short
- Captures the "minimal" goal
- Works as both acronym and word

**Declined Alternatives:**
- LEAP: Too generic
- SIMPLE: Overused acronym pattern

---

### DEC-002: Design Philosophy - Simplicity over Flexibility

**Date:** 2025-01-24
**Status:** Accepted

**Context:** EEBUS chose maximum flexibility (7 RFE modes, 250+ data structures), leading to 7,000+ implementation variations.

**Options Evaluated:**
1. Keep EEBUS flexibility, improve documentation
2. Moderate flexibility with stricter guidelines
3. Minimal flexibility with fixed operations

**Decision:** Minimal flexibility with fixed operations

**Rationale:**
- EEBUS proves flexibility creates incompatibility
- Matter succeeds with simpler model (Read/Write/Subscribe/Invoke)
- Easier to add features later than remove complexity

**Declined Alternatives:**
- Option 1: Doesn't solve the fundamental problem
- Option 2: Half-measures lead to the same issues

---

### DEC-003: Target Hardware Constraints

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to define minimum hardware requirements.

**Options Evaluated:**
1. High-end only (1MB+ RAM, 32-bit CPU)
2. Mid-range (256KB-512KB RAM, 32-bit CPU)
3. Ultra-low power (64KB RAM, basic MCU)
4. Linux/RTOS only

**Decision:** Mid-range (256KB RAM target, ESP32-class devices)

**Rationale:**
- Balances capability with real-world device constraints
- ESP32 is extremely common in energy devices
- Allows meaningful protocol without extreme compromises
- Still requires thoughtful message size design

**Declined Alternatives:**
- High-end only: Excludes too many embedded devices
- Ultra-low power: Would require extreme simplification, likely separate "lite" profile
- Linux only: Excludes the embedded devices we want to support

---

### DEC-004: Initial Use Case Scope

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to define initial use cases to shape feature design.

**Options Evaluated:**
1. EV Charging only (most common EEBUS use case)
2. Load Control / Power Limits only
3. Measurement Data only
4. All three together

**Decision:** All three together

**Rationale:**
- These three use cases are interconnected in real deployments
- Designing for one would miss patterns needed by others
- EV charging inherently needs measurement and load control
- Better to have coherent design from start

---

### DEC-005: Commissioning Model

**Date:** 2025-01-24
**Status:** Accepted

**Context:** SHIP uses mDNS discovery + complex pairing with trust levels and PINs.

**Options Evaluated:**
1. Matter-style commissioning (QR code / setup code)
2. mDNS discovery + simplified pairing
3. Support both methods

**Decision:** Matter-style commissioning (QR code / setup code)

**Rationale:**
- Proven UX in consumer devices
- Simpler implementation than SHIP's trust level negotiation
- Avoids SHIP's PIN security flaws (no nonce, brute force vulnerable)
- Widely understood by installers and users
- Can still use mDNS for discovery after pairing

**Declined Alternatives:**
- mDNS + pairing: Still requires complex trust negotiation
- Both methods: Adds implementation complexity, unclear benefit

---

### DEC-006: Serialization Format

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to choose message serialization format for 256KB RAM target.

**Options Evaluated:**
1. JSON (156 bytes for test message, human-readable)
2. CBOR (75-108 bytes, IETF standard RFC 8949)
3. MessagePack (105 bytes, no IETF standard)
4. Protocol Buffers (~85 bytes, requires code generation)
5. Custom TLV (~70 bytes, must build ourselves)

**Decision:** CBOR with integer keys for compactness

**Rationale:**
- 52% smaller than JSON without code generation
- IETF standard (RFC 8949) - not proprietary
- Used by Matter, CoAP, Thread - proven in IoT/embedded
- Self-describing - can decode without schema (debugging)
- Streaming parse possible - low RAM usage
- COSE (CBOR security) fits well for auth tokens

**Declined Alternatives:**
- JSON: Too large for constrained devices
- MessagePack: No IETF standard, less IoT adoption
- Protobuf: Code generation adds build complexity
- Custom TLV: High implementation risk, no tooling

**Mitigation for debuggability:**
- Build CLI tool with `cbor2json` conversion
- Use diagnostic notation in docs/logs

---

### DEC-007: Device Model Hierarchy

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to define how devices, capabilities, and data are structured.

**Options Evaluated:**
1. 2-Level: Device > Feature (simpler, but needs array indices for multi-port)
2. 3-Level: Device > Endpoint > Feature (like Matter's Device > Endpoint > Cluster)
3. Variable depth (flexible but complex parsing)

**Decision:** 3-Level hierarchy: Device > Endpoint > Feature

**Rationale:**
- Cleanly handles multi-function devices (dual-port EVSEs, hybrid inverters)
- Matter-aligned - familiar to developers
- Fixed depth = simple, predictable parsing
- Minimal overhead for single-function devices (use endpoint 1)
- Future-proof for complex device types

**Addressing Scheme:**
```
device_id / endpoint_id / feature_id / attribute_or_command
```

**Endpoint Conventions:**
- Endpoint 0: Root device info (manufacturer, model, etc.)
- Endpoint 1+: Functional endpoints (charger port, inverter section, etc.)

**Declined Alternatives:**
- 2-Level: Requires array indices (`Measurement[0]`) - less semantic clarity
- Variable depth: Complicates parsing, unclear benefit

### DEC-008: Interaction Model

**Date:** 2025-01-24
**Status:** Accepted

**Context:** SPINE has 7 RFE operation modes creating 7,000+ implementation variations.

**Options Evaluated:**
1. 7 Operations (like SPINE: replaceAll, updateAll, partial, delete, deleteAll, notify, read)
2. 5 Operations (Read, Write, Subscribe, Invoke + Partial Write)
3. 4 Operations (like Matter: Read, Write, Subscribe, Invoke)
4. 3 Operations (Read, Write, Invoke - no subscriptions)

**Decision:** 4 Operations (Read, Write, Subscribe, Invoke)

**Rationale:**
- Write replaces the entire value - no partial update complexity
- Subscribe handles notifications (no separate "notify" mode)
- Invoke for commands with parameters
- Read for current state
- Dramatically reduces implementation complexity

**Declined Alternatives:**
- Partial updates: Adds significant complexity for marginal benefit
- 3 operations (no subscribe): Forces polling, inefficient

---

### DEC-009: Connection Initiation

**Date:** 2025-01-24
**Status:** Accepted

**Context:** SHIP allows both peers to initiate connections, causing race conditions.

**Options Evaluated:**
1. Client initiates only (controller → device)
2. Either can initiate (like SHIP)
3. Server initiates (push model)

**Decision:** Client initiates only

**Rationale:**
- Deterministic - no race conditions
- Natural model: energy manager (client) connects to EVSE (server)
- Simpler implementation
- Eliminates SHIP's "double connection" problem entirely

---

### DEC-010: Multi-Controller and Pairing Model

**Date:** 2025-01-24
**Status:** Accepted

**Context:** When multiple controllers exist, need clear rules for who controls what.

**Options Evaluated:**
1. First wins, others rejected
2. Priority-based takeover
3. Shared control allowed
4. Capability-based routing with user confirmation

**Decision:** Hybrid capability-routing with priority override

**Key Principles:**
1. **Discovery**: Controllers discover devices and their capabilities
2. **Suggestion**: System suggests pairings based on capability matching
3. **User Confirmation**: User confirms pairing (required)
4. **Priority Takeover**: Higher-priority controller can request takeover
5. **User Override**: User can always override priority (e.g., disconnect grid operator)

**Priority Levels (example):**
- Level 1: Grid Operator / DSO (highest)
- Level 2: Building/Commercial Energy Manager
- Level 3: Home Energy Manager
- Level 4: User App (lowest)

**Takeover Flow:**
1. New controller requests control with its priority level
2. Device notifies current controller of takeover request
3. If higher priority: takeover succeeds (current controller notified)
4. User can physically interact with device to reject takeover

**Rationale:**
- Avoids SPINE's undefined "appropriate client"
- Enables grid override scenarios
- User always has final say
- Clear, deterministic behavior

---

## Pending Decisions

### DEC-011: Transport Layer

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to choose transport for 256KB devices. SHIP uses WebSocket/TLS, Matter uses UDP+MRP or TCP.

**Options Evaluated:**
1. WebSocket over TLS (like SHIP) - ~55KB code
2. Raw TCP/TLS with length-prefix framing - ~42KB code
3. CoAP over DTLS - ~60KB code, UDP NAT issues
4. UDP with custom reliability (like Matter MRP)
5. QUIC - ~200KB+, too heavy

**Decision:** TCP/TLS with simple length-prefix framing

**Rationale:**
- **TCP reliability built-in** - no need to implement MRP ourselves
- **Smallest code footprint** - ~42KB vs 55KB for WebSocket
- **Minimal overhead** - 4 bytes per message vs 2-14 for WebSocket
- **Sufficient for target devices** - EVSEs, inverters are always-on, not battery
- **Simple implementation** - TLS library + trivial framing code

**Framing Format:**
```
┌────────────────────────────────────────┐
│ Length (4B, big-endian) │ CBOR Payload │
└────────────────────────────────────────┘
```

**Keep-Alive Protocol (to be defined):**
- Ping/pong messages in CBOR
- 30-second interval
- Connection closed after 3 missed pongs

**Declined Alternatives:**
- WebSocket: Extra 13KB code, HTTP upgrade overhead not needed
- CoAP/DTLS: UDP NAT issues in home networks
- UDP+MRP: Unnecessary complexity for always-on devices
- QUIC: Won't fit in 256KB RAM

---

### DEC-012: Device Attestation Model

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to decide if devices must prove authenticity via manufacturer certificates.

**Options Evaluated:**
1. Required attestation (all devices need manufacturer CA)
2. Optional attestation (support CA if present, allow self-signed)
3. No attestation (self-signed only)

**Decision:** Optional attestation

**Rationale:**
- Supports large manufacturers with CA infrastructure
- Doesn't exclude small vendors or DIY devices
- Trust ultimately established through pairing, not certificate chain
- Pragmatic for energy device ecosystem

---

### DEC-013: Operational Certificates

**Date:** 2025-01-24
**Status:** Accepted

**Context:** After pairing, should devices get controller-issued certificates?

**Options Evaluated:**
1. Operational certs (controller issues during pairing)
2. Device cert only (keep using original)

**Decision:** Operational certificates

**Rationale:**
- Enables certificate rotation (1-year validity)
- Supports multi-controller (each controller issues own zone cert)
- Clean revocation via cert deletion
- Like Matter's NOC model

**Lifecycle:**
- Validity: 1 year
- Renewal: Auto-renewed by controller before expiry
- Revocation: Controller sends delete command to device

---

### DEC-014: Pairing Protocol

**Date:** 2025-01-24
**Status:** Accepted

**Context:** How to establish initial trust between controller and device.

**Options Evaluated:**
1. SPAKE2+ with numeric code (like Matter PASE)
2. Simple pre-shared key
3. Certificate fingerprint in QR

**Decision:** SPAKE2+ with 8-digit numeric setup code

**Rationale:**
- Proven PAKE protocol (used by Matter)
- No secrets transmitted in clear
- Resistant to offline dictionary attacks
- 8 digits provides ~27 bits entropy (sufficient for local pairing)
- Easy to type manually if needed

**Setup Code Format:**
- 8 decimal digits (00000000-99999999)
- Encoded in QR code with discriminator + vendor info
- Optional: printed on device label

---

### DEC-015: Certificate Lifecycle

**Date:** 2025-01-24
**Status:** Accepted

**Context:** How long are certificates valid and how are they managed.

**Decision:**

| Certificate | Validity | Renewal | Revocation |
|-------------|----------|---------|------------|
| Device/Attestation | 20 years | Never | N/A |
| Operational | 1 year | Auto by controller | Controller deletes from device |
| Zone CA | 99 years | N/A | Zone removal |

**Note:** Zone CA uses very long validity (99 years) because:
- Zone ID is derived from CA certificate fingerprint
- Changing CA would require re-commissioning all devices
- Expiry doesn't add security (zone removal is the revocation mechanism)
- Effectively "permanent" for practical purposes

**Revocation Flow:**
1. Controller decides to remove device
2. Controller sends "RemoveZone" command
3. Device deletes operational certificate
4. Device returns to unpaired state

---

### DEC-016: Multi-Zone Support

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Real deployments have multiple controllers (SMGW, EMS, apps) needing access.

**Options Evaluated:**
1. Single zone with roles (one CA, shared)
2. Multiple zones (each controller independent)

**Decision:** Multiple zones

**Rationale:**
- Each controller (SMGW, EMS) is independent with own Zone CA
- Device holds multiple operational certs (one per zone)
- No coupling between controllers
- SMGW replacement doesn't affect EMS
- Like Matter's multi-admin model

**Implementation:**
- Device supports up to 5 zones
- Each zone has: type, CA, operational cert
- ZoneManagement feature for add/remove
- Zones can see each other's types for coordination

---

### DEC-017: Per-Feature Priority

**Date:** 2025-01-24
**Status:** Accepted

**Context:** When multiple controllers want to control the same feature, who wins?

**Options Evaluated:**
1. Zone type priority (GRID > BUILDING > HOME > USER)
2. Most restrictive wins (min of all limits)
3. Separate limits tracked per controller

**Decision:** Zone type priority

**Zone Types (priority order):**
```
GRID_OPERATOR = 1     // DSO, SMGW - highest
BUILDING_MANAGER = 2  // Commercial EMS
HOME_MANAGER = 3      // Residential EMS
USER_APP = 4          // Mobile apps - lowest
```

**Behavior:**
- Higher priority can clear/remove lower priority's settings
- Lower priority is notified of changes
- User physical override always possible (button on device)

**Important Clarification (see DEC-024):**
- For **limits** specifically, priority does NOT override
- All zone limits are tracked, most restrictive (min) wins
- This ensures safety constraints (fuses, wiring) are respected
- Priority matters for: access control, removing others' settings, notifications

---

### DEC-018: Setup Code Reusability

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Can the same setup code be used for multiple zone commissionings?

**Decision:** Static setup code, reusable for all zones

**Rationale (like Matter):**
- Setup code is factory-programmed, printed on device
- Same code used for all zone commissionings
- Security provided by:
  1. Physical access to QR code required
  2. SPAKE2+ protocol prevents eavesdropping
  3. User scanning implies consent
- No benefit to rolling codes - physical access is the gate

---

### DEC-019: Delegated Commissioning (Backend)

**Date:** 2025-01-24
**Status:** Accepted

**Context:** SMGW has no UI - can't scan QR codes directly.

**Decision:** Support delegated commissioning via backend

**Flow:**
1. User scans device QR with utility app (phone)
2. App uploads setup code + device info to DSO backend
3. Backend securely forwards to SMGW
4. SMGW uses setup code to commission device via SPAKE2+
5. Device gets SMGW's operational cert (GRID_OPERATOR zone)

**QR Code Content (sufficient for backend delegation):**
```
MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
```

**Security:**
- Backend must securely store/transmit setup codes
- TLS between all components
- Setup code only valuable with network access to device

### DEC-020: Zone Roles (Owner vs Admin)

**Date:** 2025-01-24
**Status:** Accepted

**Context:** How do apps participate in commissioning without being fragile zone owners?

**Decision:** Three-tier role model

| Role | Description | Example |
|------|-------------|---------|
| **Zone Owner** | Owns Zone CA, issues certs | EMS, SMGW |
| **Zone Admin** | Authorized to commission | Phone App, Installer Tool |
| **Zone Member** | Normal participant | Devices |

**Key Principles:**
- Apps are admins, NOT owners (losing phone doesn't orphan devices)
- EMS is always zone owner (has CA)
- Apps cannot own zones - always need an EMS
- Multiple admins supported (family members, installers)

**Commissioning with Admin:**
1. App has admin token from EMS
2. App does SPAKE2+ with device (has setup code)
3. App gets CSR from device
4. App **forwards CSR to EMS** (app can't sign)
5. EMS signs, returns operational cert
6. App installs cert on device

---

### DEC-021: App-EMS Admin Authorization

**Date:** 2025-01-24
**Status:** Accepted

**Context:** How does an app become a zone admin for an EMS?

**Decision:** QR code + confirmation via EMS web UI

**Flow:**
```
User             Phone App           EMS Web UI
 │                   │                   │
 │── Open web UI ────┼──────────────────►│
 │── "Add admin" ────┼──────────────────►│
 │◄──────────────────┼── Show temp QR ───┤  (5 min expiry)
 │                   │                   │
 │── Scan QR ───────►│                   │
 │                   │── SPAKE2+ ───────►│
 │                   │                   │
 │◄──────────────────┼── "Add 'Phone'?" ─┤
 │── Confirm ────────┼──────────────────►│
 │                   │◄── Admin token ───┤
```

**Security:**
- QR is temporary (5 min expiry)
- User must confirm in EMS UI (proves access to EMS)
- App name displayed for verification

**Pre-requisite:**
- EMS must have web UI (local or cloud-based)
- User must have access to EMS UI to add admins

---

### DEC-022: IPv6-Only Network Layer

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Need to decide on IP version for the network layer.

**Options Evaluated:**
1. IPv4 only (legacy)
2. Dual-stack IPv4/IPv6 (two code paths)
3. IPv6 mandatory with IPv4 fallback
4. IPv6 only (no IPv4)

**Decision:** IPv6 only - no IPv4 support

**Rationale:**
- **Simpler implementation** - one code path, not two
- **Link-local (fe80::) always works** - even without DHCP/router
- **Linux support since 1999** - kernel 2.2, over 25 years ago
- **All modern embedded platforms support it** - ESP32, etc.
- **Matter is IPv6-only** - proven approach
- **Thread ready** - IPv6-only mesh, door open for future Thread support
- **No NAT issues** - every device gets unique address
- **Multicast discovery** - native support (ff02::fb for mDNS)

**IPv6 Features Used:**
```
Link-Local (fe80::/10)
├── Always available without configuration
├── Used for commissioning
└── Works without router/DHCP

Multicast (ff02::)
├── ff02::fb - mDNS discovery
└── Efficient group communication

SLAAC
└── Auto-configuration without DHCP
```

**Declined Alternatives:**
- IPv4 only: Legacy, NAT issues, limited addresses
- Dual-stack: Doubles implementation complexity, testing burden
- IPv4 fallback: No realistic scenario where needed
  - Old routers: Even cheap routers from 5+ years ago support IPv6
  - Old networks: We target new devices, not retrofitting
  - Corporate networks: Smart home devices aren't on corporate networks

---

### DEC-024: Limit Resolution - Stacked Limits (Most Restrictive Wins)

**Date:** 2025-01-24
**Status:** Accepted

**Context:** When multiple zones set limits on the same feature, how are they resolved?

**Options Evaluated:**
1. Priority override (higher priority zone's limit wins, others ignored)
2. Most restrictive wins (min of all limits, all tracked)
3. Subscription-based (notify on removal, let zones re-apply)

**Decision:** Most restrictive wins (stacked limits)

**Critical Example - Why Priority Override Fails:**
```
SMGW (GRID_OPERATOR, priority 1): "Grid allows 6 kW"
EMS (HOME_MANAGER, priority 3):   "House fuse is 5 kW"

Priority-based: 6 kW → exceeds fuse capacity → dangerous
Stacked limits: min(6, 5) = 5 kW → safe
```

**Lower priority controllers MUST be able to impose stricter limits** for physical safety constraints (fuses, wiring, device ratings).

**Internal vs Exposed State:**
```
Device internal state (not exposed):
  zoneLimits: map[zoneID] → limit
    Zone 1 (GRID_OPERATOR): 6000 W
    Zone 2 (HOME_MANAGER):  5000 W

Exposed via API (per-zone scoped):
  effectiveLimit: min(all) = 5000 W    // same for all zones
  myLimit: 6000 W or 5000 W            // depends on requesting zone
  limitActive: true/false              // is my limit the effective one?
```

**API Design (simple, zone-isolated):**
```
Limit feature:
  Attributes:
    - effectiveLimit: uint32 (W)   // min(all zone limits)
    - myLimit: uint32 (W)          // this zone's limit (zone-scoped)
    - limitActive: bool            // is my limit the effective one?

  Commands:
    - SetLimit(value, duration?)
    - ClearLimit()
```

**Why NOT expose per-zone limits:**
- Each zone knows its own limit (it set it)
- Each zone can see effective limit
- Zone isolation / privacy between controllers
- Simpler API, less data in messages
- Device handles complexity internally

**What Priority IS Used For:**
| Aspect | Priority-based? |
|--------|-----------------|
| Setting limits | No - all zones can set |
| Effective limit | No - min(all) wins |
| Removing OTHER zone's limits | Yes - higher can clear lower |
| Access to other features | Yes - per DEC-017 |

**Subscription for Awareness:**
- Zones subscribe to Limit.effectiveLimit
- Get notified when effective limit changes
- Can infer: if effectiveLimit < myLimit, someone else is more restrictive
- EMS knows when constraint lifts (can adjust strategy)

**Declined Alternatives:**
- Priority override: Dangerous - ignores physical constraints
- Subscription re-apply: Extra complexity, race conditions, still doesn't solve fuse scenario

---

### DEC-025: Terminology - "Feature" instead of "Cluster"

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Matter uses "cluster" for groupings of attributes and commands. Should we use the same term?

**Options Evaluated:**
1. Cluster (Matter terminology)
2. Feature (functional building block)
3. Capability
4. Service
5. Component

**Decision:** Use "Feature" as our terminology

**Rationale:**
- **Intuitive** - "device supports Limit feature" reads naturally
- **Functional focus** - describes what it does, not structure
- **Reusable** - features are building blocks composed into endpoints
- **Distinct from Matter** - avoids implying compatibility

**Terminology Mapping:**
| Matter | MASH |
|--------|------|
| Cluster | Feature |
| Cluster ID | Feature ID |

**Updated Hierarchy:**
```
Device > Endpoint > Feature > Attributes/Commands
```

**Core Features:**
| Feature | Purpose | Used By |
|---------|---------|---------|
| `Limit` | Power constraints, set/clear | EVSE, Inverter, Heat Pump |
| `Measurement` | Power, energy, voltage, current | All devices |
| `ChargingSession` | EV session state, SoC | EVSE only |
| `DeviceInfo` | Identity, firmware, capabilities | All (endpoint 0) |

**Device Composition Example:**
```
EVSE
├── Endpoint 0: DeviceInfo
└── Endpoint 1: Measurement + Limit + ChargingSession

Inverter
├── Endpoint 0: DeviceInfo
└── Endpoint 1: Measurement + Limit
```

**Declined Alternatives:**
- Cluster: Matter terminology, implies compatibility
- Capability: Too vague
- Service: Overloaded (web services, systemd, etc.)
- Component: Implies UI/frontend

---

### DEC-023: Terminology - "Zone" instead of "Fabric"

**Date:** 2025-01-24
**Status:** Accepted

**Context:** Matter uses "fabric" for trust boundaries / controller domains. Should we adopt the same term or choose our own?

**Options Evaluated:**
1. Fabric (Matter terminology)
2. Authority
3. Realm
4. Trust
5. Domain
6. Zone
7. Scope
8. Sector

**Decision:** Use "Zone" as our terminology

**Rationale:**
- **Simple and clear** - no explanation needed
- **No baggage** - unlike "domain" (DNS/AD) or "realm" (Kerberos)
- **Industrial/energy feel** - fits the smart energy context
- **Distinct from Matter** - avoids implying compatibility
- **Works naturally in context:**
  - "Device supports up to 5 zones"
  - "Add device to EMS zone"
  - "Zone owner vs zone admin"
  - "GRID_OPERATOR zone has priority"

**Terminology Mapping:**
| Matter | MASH |
|--------|------|
| Fabric | Zone |
| Fabric ID | Zone ID |
| Fabric CA | Zone CA |
| Multi-fabric | Multi-zone |
| AddFabric | AddZone |
| RemoveFabric | RemoveZone |

**Declined Alternatives:**
- Fabric: Implies Matter compatibility, borrowed terminology
- Authority: Slightly redundant with "zone owner"
- Realm: Fantasy/Kerberos connotations
- Domain: DNS/Active Directory confusion
- Trust: Grammatically awkward in some contexts

---

### DEC-026: EnergyControl Feature Design (Capability-First, Forecast-Optional)

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Need to define the Limit/EnergyControl feature that covers all EEBUS limit use cases (LPC, LPP, OPEV, OSCEV, CEVC, DBEVC, COB, OHPCF, POEN) while being simpler. Matter's DeviceEnergyManagement cluster is forecast-centric, but not all devices can provide forecasts (e.g., basic wallbox doesn't know EV's plan).

**Options Evaluated:**
1. EEBUS approach: Separate use cases for each device type (10+ specs)
2. Matter approach: Forecast-centric with PowerForecast/StateForecast required
3. Capability-first: Device announces what it CAN do, forecast optional

**Decision:** Capability-first, forecast-optional design

**Key Principle:**
```
Every device announces: "What I CAN do" (capabilities)  ← MANDATORY
Some devices announce:  "What I PLAN to do" (forecast)  ← OPTIONAL
Controller sends:       "What I WANT you to do" (limit) ← CORE FUNCTION
```

**Rationale:**
- **All devices can announce capabilities** - even the simplest EVSE knows its max power
- **Not all devices can forecast** - wallbox without ISO 15118-20 doesn't know EV plans
- **Forecasts add value when available** - smart devices can optimize better
- **Single feature covers all EEBUS use cases** - no separate LPC/LPP/OPEV/etc.
- **Matter-inspired simplicity** - feature flags, not separate protocols

**Feature Structure:**

```
EnergyControl Feature
├── MANDATORY (all controllable devices):
│   ├── deviceType: DeviceTypeEnum
│   ├── state: StateEnum
│   ├── capabilities: CapabilityStruct
│   ├── effectiveLimit: int64 (mW)
│   └── optOutState: OptOutEnum
│
├── OPTIONAL (for flexibility announcement):
│   ├── flexibility: FlexibilityStruct
│   └── forecast: ForecastStruct
│
└── COMMANDS:
    ├── SetLimit(power, duration?, cause)
    ├── ClearLimit()
    ├── Pause(duration?)  [if isPausable]
    ├── Resume()          [if isPausable]
    └── AdjustStartTime(newStart)  [if isShiftable]
```

**DeviceTypeEnum (like Matter's ESAType):**
```
EVSE              = 0x00  // EV Charger
HEAT_PUMP         = 0x01  // Space heating/cooling
WATER_HEATER      = 0x02
BATTERY           = 0x03  // Home battery storage
INVERTER          = 0x04  // Solar/hybrid inverter
FLEXIBLE_LOAD     = 0x05  // Generic controllable load
OTHER             = 0xFF
```

**StateEnum:**
```
OFFLINE           = 0x00  // Not controllable
ONLINE            = 0x01  // Normal operation
FAULT             = 0x02  // Error state
LIMITED           = 0x03  // Limit active
PAUSED            = 0x04  // Temporarily paused
FAILSAFE          = 0x05  // Communication lost, failsafe active
```

**CapabilityStruct (MANDATORY):**
```
{
  direction: enum { CONSUMPTION, PRODUCTION, BIDIRECTIONAL }

  // Power boundaries (signed: + consumption, - production)
  nominalMaxConsumption: int64 (mW)
  nominalMaxProduction: int64 (mW)     // 0 if can't produce
  currentMaxConsumption: int64 (mW)    // Currently available
  currentMaxProduction: int64 (mW)
  currentMinPower: int64 (mW)          // Minimum operating point

  // For storage devices
  energyCapacity: int64? (mWh)
  stateOfCharge: uint8? (%)

  // Feature flags
  isPausable: bool
  isShiftable: bool
  acceptsLimits: bool
  acceptsSetpoints: bool               // Can track a target, not just limit
}
```

**FlexibilityStruct (OPTIONAL):**
```
{
  // Time flexibility
  earliestStart: timestamp?
  latestEnd: timestamp?

  // Energy flexibility
  energyMin: int64? (mWh)
  energyMax: int64? (mWh)
  energyTarget: int64? (mWh)

  // Power operating range
  powerRangeMin: int64 (mW)
  powerRangeMax: int64 (mW)

  // For interruptible loads
  minRunDuration: uint32? (s)
  maxPauseDuration: uint32? (s)
}
```

**ForecastStruct (OPTIONAL):**
```
{
  forecastId: uint32
  startTime: timestamp
  endTime: timestamp
  slots: ForecastSlot[] (max 10)
}

ForecastSlot {
  duration: uint32 (s)
  nominalPower: int64 (mW)
  minPower: int64? (mW)
  maxPower: int64? (mW)
  isPausable: bool?
}
```

**LimitCauseEnum (why the limit):**
```
GRID_EMERGENCY     = 0   // DSO/SMGW - highest priority, MUST follow
GRID_OPTIMIZATION  = 1   // DSO request for grid balancing
LOCAL_PROTECTION   = 2   // Fuse protection, overload prevention
LOCAL_OPTIMIZATION = 3   // Home optimization, cost savings
USER_PREFERENCE    = 4   // User app request
```

**OptOutEnum (user override, from Matter):**
```
NO_OPT_OUT        = 0   // Accept all adjustments
LOCAL_OPT_OUT     = 1   // Reject local optimization
GRID_OPT_OUT      = 2   // Reject grid requests
OPT_OUT           = 3   // Reject all external control
```

**Commands:**
```
SetLimit(
  power: int64 (mW),        // + consumption limit, - production limit
  duration: uint32? (s),    // Optional (0 = indefinite)
  cause: LimitCauseEnum
) → { success: bool, effectiveLimit: int64 }

ClearLimit() → { success: bool }

Pause(duration: uint32? (s)) → { success: bool }
Resume() → { success: bool }

AdjustStartTime(
  requestedStart: timestamp,
  cause: LimitCauseEnum
) → { success: bool, actualStart: timestamp }
```

**EEBUS Use Case Mapping:**

| EEBUS Use Case | MASH Coverage |
|----------------|---------------|
| LPC (Limit Power Consumption) | SetLimit with CONSUMPTION direction |
| LPP (Limit Power Production) | SetLimit with PRODUCTION direction |
| OPEV (Overload Protection EV) | SetLimit with LOCAL_PROTECTION cause |
| OSCEV (Self-Consumption EV) | Flexibility + SetLimit with LOCAL_OPTIMIZATION |
| CEVC (Coordinated EV Charging) | Forecast + Flexibility |
| DBEVC (Bidirectional EV) | BIDIRECTIONAL direction + Forecast |
| COB (Control of Battery) | BIDIRECTIONAL direction + SetLimit |
| OHPCF (Heat Pump Flexibility) | isPausable + isShiftable + Pause/Resume |
| POEN (Power Envelope) | Repeated SetLimit calls (or future schedule extension) |

**Failsafe Behavior (inherited from transport):**
1. Keep-alive fails (3 missed pongs)
2. Device state → FAILSAFE
3. Device applies failsafe limit (from capabilities)
4. On reconnect: zone must re-set limit
5. Device exits FAILSAFE when valid limit received

**Declined Alternatives:**
- EEBUS approach: 10+ use cases too complex, testing nightmare
- Pure forecast approach: Not all devices can forecast (basic wallbox)
- Current-based limits (OPEV style): Always use power (W), device converts

**Resolves:** OPEN-001 (Feature Definitions) - partially

---

### DEC-027: Feature Separation (Electrical, Measurement, EnergyControl)

**Date:** 2025-01-25
**Status:** Accepted

**Context:** During EnergyControl feature design, we realized phase configuration and electrical ratings are needed both for control (EnergyControl) and for interpreting measurement data. Similarly, measurement data (power readings, SoC) is distinct from control capabilities.

**Options Evaluated:**
1. Single monolithic feature with all electrical data
2. Two features: Electrical + EnergyControl (measurements in both)
3. Three features: Electrical (config), Measurement (telemetry), EnergyControl (control)

**Decision:** Three separate features with clear responsibilities

**Feature Responsibilities:**

| Feature | Question | Examples | Update Frequency |
|---------|----------|----------|------------------|
| **Electrical** | "What is this device?" | Phase config, voltage ratings, nominal power | Read once at discovery |
| **Measurement** | "What is it doing now?" | Power readings, energy totals, SoC | Subscribe for real-time |
| **EnergyControl** | "What can I tell it to do?" | Capabilities, limits, commands | Subscribe for state changes |

**Electrical Feature (Static Configuration):**
```
- phaseCount: 1, 2, or 3
- phaseMapping: {DevicePhase → GridPhase} for coordination
- nominalVoltage, supportedDirections
- nominalMaxConsumption, nominalMaxProduction (power ratings)
- maxCurrentPerPhase (for OPEV-style limits)
- supportsAsymmetric, phaseCurrentDirections
- energyCapacity (for batteries)
```

**Measurement Feature (Telemetry):**
```
- activePower, reactivePower (total, signed)
- energyConsumed, energyProduced (cumulative)
- stateOfCharge (for batteries)
- perPhaseReadings: {phase → {power, current, voltage}}
```

**EnergyControl Feature (Control):**
```
- deviceType, deviceState (capability flags)
- isPausable, isShiftable, canForecast
- consumptionLimit, productionLimit (active limits)
- currentLimits (per-phase, for OPEV)
- forecast (optional)
- Commands: SetLimit, SetCurrentLimits, ClearLimit, Pause, Resume
```

**Rationale:**
- **Separation of concerns**: Static config vs telemetry vs control have different update patterns
- **Efficiency**: Subscribe only to what you need (measurements or control state)
- **Reusability**: Measurement feature useful without EnergyControl (e.g., pure monitoring)
- **Phase consistency**: Phase mapping defined once in Electrical, used by both Measurement and EnergyControl
- **Per-phase current limits**: Supports OPEV-style asymmetric current limits + bidirectional (EEBUS gap)

**Phase Mapping Design:**
```
Device declares in Electrical:
  phaseMapping: {A: L1, B: L2, C: L3}  // or rotated: {A: L2, B: L3, C: L1}

Controller can now:
  - Interpret per-phase measurements correctly
  - Set per-phase current limits on correct grid phases
```

**Declined Alternatives:**
- Single feature: Too large, inefficient subscriptions, mixed update frequencies
- Two features: Unclear where measurements belong, phase config duplicated

**Resolves:** OPEN-001 (Feature Definitions) - Electrical and Measurement features now defined

---

### DEC-028: Setpoints and V2H Phase Balancing

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Limits ("do not exceed") are not sufficient for all control scenarios. Battery systems, solar charging, and V2H bidirectional EVs need setpoints ("please target this value"). Additionally, V2H with asymmetric phase support requires per-phase current setpoints to balance household load across phases.

**Options Evaluated:**
1. Limits only - devices figure out their own targets
2. Single setpoint command (total power only)
3. Separate setpoint commands for power and per-phase current (mirrors limit structure)

**Decision:** Symmetric command structure for limits and setpoints

**New Commands:**
| | Total Power (mW) | Per-Phase Current (mA) |
|--|------------------|------------------------|
| **Hard constraint** | SetLimit | SetCurrentLimits |
| **Target** | SetSetpoint | SetCurrentSetpoints |
| **Clear constraint** | ClearLimit | ClearCurrentLimits |
| **Clear target** | ClearSetpoint | ClearCurrentSetpoints |

**New Capabilities:**
```
acceptsLimits: bool           // accepts SetLimit
acceptsCurrentLimits: bool    // accepts SetCurrentLimits
acceptsSetpoints: bool        // accepts SetSetpoint
acceptsCurrentSetpoints: bool // accepts SetCurrentSetpoints (V2H)
```

**AsymmetricSupportEnum (replaces bool):**
```
NONE          = 0x00  // Symmetric only
CONSUMPTION   = 0x01  // Asymmetric when charging
PRODUCTION    = 0x02  // Asymmetric when discharging
BIDIRECTIONAL = 0x03  // Asymmetric both directions
```

**Resolution Difference:**
```
LIMITS:    Most restrictive wins (all zones constrain together)
SETPOINTS: Highest priority zone wins (only one controller active)
```

**V2H Use Case:**
```
House consumption: L1=20A, L2=5A, L3=12A
EMS sends: SetCurrentSetpoints({A: 10000, B: 2000, C: 5000}, PRODUCTION, PHASE_BALANCING)
V2H EV discharges asymmetrically to balance phases
Result: Net import L1=10A, L2=3A, L3=7A (balanced)
```

**Rationale:**
- Symmetric structure is easier to learn and implement
- Per-phase current setpoints enable V2H phase balancing (EEBUS gap)
- AsymmetricSupportEnum clarifies which directions support different values per phase
- Limits constrain setpoints (device can't target beyond its limit)

**Extends EEBUS:**
- DBEVC only supports total power setpoints, not per-phase
- No EEBUS use case covers asymmetric discharge for phase balancing
- MASH fills this gap with SetCurrentSetpoints

---

### DEC-029: Measurement Feature and EndpointType

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Need comprehensive measurement support covering all EEBUS monitoring use cases (MPC, MGCP, EVCEM, MOI, MOB, MPS). Also need a way to identify what each endpoint represents in multi-component devices like hybrid inverters.

**Options Evaluated for Measurements:**
1. Single feature with AC-only measurements
2. Separate AcMeasurement and DcMeasurement features
3. Single Measurement feature with both AC and DC attributes

**Decision:** Single Measurement feature with comprehensive AC and DC support

**Measurement Attributes:**

| Category | Attributes | Used By |
|----------|------------|---------|
| **AC Power** | acActivePower, acReactivePower, acApparentPower, acActivePowerPerPhase | INVERTER, GRID_CONNECTION, EV_CHARGER |
| **AC Current/Voltage** | acCurrentPerPhase, acVoltagePerPhase, acVoltagePhaseToPhasePair, acFrequency, powerFactor | All AC endpoints |
| **AC Energy** | acEnergyConsumed, acEnergyProduced | All AC endpoints |
| **DC Measurements** | dcPower, dcCurrent, dcVoltage, dcEnergyIn, dcEnergyOut | PV_STRING, BATTERY |
| **Battery State** | stateOfCharge, stateOfHealth, stateOfEnergy, useableCapacity, cycleCount | BATTERY |
| **Temperature** | temperature | BATTERY, INVERTER |

**EndpointType Enum:**

```
DEVICE_ROOT       = 0x00  // Device-level info (always endpoint 0)
GRID_CONNECTION   = 0x01  // AC grid connection point (smart meter)
INVERTER          = 0x02  // Inverter AC side (grid-facing)
PV_STRING         = 0x03  // PV string / solar input (DC)
BATTERY           = 0x04  // Battery storage (DC)
EV_CHARGER        = 0x05  // EVSE / wallbox
HEAT_PUMP         = 0x06  // Heat pump
WATER_HEATER      = 0x07  // Water heater / boiler
HVAC              = 0x08  // HVAC system
APPLIANCE         = 0x09  // Generic controllable appliance
SUB_METER         = 0x0A  // Sub-meter / circuit monitor
```

**Hybrid Inverter Example:**

```
Device (hybrid-inverter)
├── Endpoint 0: type=DEVICE_ROOT
├── Endpoint 1: type=INVERTER, label="Grid Connection"
│   └── Measurement: AC power, energy, voltage, current
├── Endpoint 2: type=PV_STRING, label="Roof South"
│   └── Measurement: DC power, voltage, current, yield
├── Endpoint 3: type=PV_STRING, label="Roof West"
│   └── Measurement: DC power, voltage, current, yield
└── Endpoint 4: type=BATTERY, label="LG Chem"
    └── Measurement: DC power, SoC, SoH, temperature
```

**Topology is Implicit:**
- AC endpoints (INVERTER, GRID_CONNECTION) → connect to grid
- DC endpoints (PV_STRING, BATTERY) → internal, connect to inverter DC bus
- No explicit parent/child relationships needed (grid is never DC)

**Sign Convention (Passive/Load):**
- Positive (+) = power flowing INTO component (consumption, charging)
- Negative (-) = power flowing OUT of component (production, discharging)

**EEBUS Use Case Coverage:**

| EEBUS | MASH Coverage |
|-------|---------------|
| MPC | acActivePower, acCurrentPerPhase, acVoltagePerPhase, acFrequency, acEnergy |
| MGCP | Same as MPC on GRID_CONNECTION endpoint |
| EVCEM | acActivePower, acCurrentPerPhase, acEnergyConsumed |
| MOI | All AC power types, powerFactor, temperature |
| MOB | dcPower/Current/Voltage, stateOfCharge/Health, cycleCount, temperature |
| MPS | dcPower, dcCurrent, dcVoltage, dcEnergyOut |

**Rationale:**
- Single feature simplifies subscription (one subscription for all measurements)
- EndpointType identifies component role without complex relationships
- AC/DC distinction covers all real-world scenarios
- Comprehensive coverage of 6+ EEBUS monitoring use cases

**Resolves:** OPEN-001 (Feature Definitions) - Measurement feature now fully defined

---

### DEC-030: Status Feature for Operating State

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Operating state (running, standby, fault, etc.) was initially considered part of EnergyControl. However, non-controllable devices (e.g., monitoring-only devices, smart meters) also have operating states but don't support EnergyControl.

**Options Evaluated:**
1. Keep operating state in EnergyControl (requires EnergyControl for all devices)
2. Create separate Status feature (applicable to any endpoint)
3. Put operating state in DeviceInfo (but state is per-endpoint, not per-device)

**Decision:** Separate Status feature, per-endpoint

**Status Feature Attributes:**
```
{
  1: operatingState,    // OperatingStateEnum
  2: stateDetail,       // uint32 (vendor-specific sub-state)
  3: faultCode,         // uint32 (0 = no fault)
  4: faultMessage,      // string (optional human-readable)
}
```

**OperatingStateEnum:**
```
UNKNOWN       = 0x00  // State cannot be determined
OFFLINE       = 0x01  // Not connected / powered off
STANDBY       = 0x02  // Ready but not active
STARTING      = 0x03  // Transitioning to running
RUNNING       = 0x04  // Normal operation
PAUSED        = 0x05  // Temporarily suspended
SHUTTING_DOWN = 0x06  // Transitioning to offline
FAULT         = 0x07  // Error state (check faultCode)
MAINTENANCE   = 0x08  // Service/maintenance mode
```

**Per-Endpoint Scope:**
- Each endpoint has its own Status feature instance
- Hybrid inverter: PV_STRING endpoints can be RUNNING while BATTERY is STANDBY
- EVSE: Port 1 can be RUNNING (charging) while Port 2 is STANDBY

**Relationship to Other Features:**
```
Status Feature: "What's happening?" (state observation)
  └── operatingState: RUNNING, STANDBY, FAULT, etc.

EnergyControl Feature: "What can I tell it to do?" (control)
  └── Pause/Resume commands → affect operatingState
```

**Rationale:**
- Operating state is universally applicable (monitoring-only devices need it)
- Per-endpoint allows fine-grained status for multi-component devices
- Separates observation from control (EnergyControl is optional)
- Fault reporting tied to specific endpoint (not device-global)

**Declined Alternatives:**
- State in EnergyControl: Forces controllable interface on monitoring devices
- State in DeviceInfo: DeviceInfo is device-level (endpoint 0), state is per-endpoint

**Resolves:** OPEN-001 (Feature Definitions) - Status feature now defined

---

### DEC-031: DeviceInfo Feature with Complete Device Structure

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Need a feature for device identification and discovery. EEBUS uses DeviceClassification (NID use case) with multiple function types. Matter uses Basic Information cluster on endpoint 0. Key question: how to minimize round-trips while providing complete device understanding.

**Options Evaluated:**
1. Minimal DeviceInfo (identity only) + separate Descriptor feature per endpoint
2. DeviceInfo with device identity + complete endpoint/feature structure
3. DeviceInfo + UseCaseInformation (like EEBUS) for capability discovery

**Decision:** DeviceInfo with complete device structure in single read

**Device ID Format:**
```
i:<IANA_PEN>:<unique>   - For vendors with IANA Private Enterprise Number
n:<vendor>:<unique>     - For vendors without IANA PEN
```

Examples:
- `i:46925:ABC123-XYZ` (using IANA PEN 46925)
- `n:acme:WB-2024-001` (using vendor name "acme")

**DeviceInfo Attributes:**
```
Identification: deviceId, vendorName, productName, productId, serialNumber, brandName?
Versions:       softwareVersion, hardwareVersion
Structure:      endpoints[] (id, type, label?, features[])
```

**Key Design Choices:**

| Choice | Decision | Rationale |
|--------|----------|-----------|
| Single read | Yes | One request gets everything needed to understand device |
| Endpoint structure | Included | No separate discovery needed |
| Feature list | Per-endpoint | Know exactly what each endpoint supports |
| User labels | Not included | EMS manages labels, simplifies device |
| Protocol version | Not included | Handled at connection level |
| Numeric vendor ID | Not required | String-based deviceId format sufficient |

**Feature ID Registry:**
```
0x0001  Electrical
0x0002  Measurement
0x0003  EnergyControl
0x0005  Status
0x0006  DeviceInfo
0x0007  ChargingSession (future)
0x0100+ Vendor-specific
```

**EEBUS NID Coverage:**
- All mandatory NID data points covered (Device Name, Serial Number, Brand Name)
- Optional fields mapped (Device Code, Software/Hardware Revision, Vendor Name/Code)
- User-configurable fields (Label, Description) not on device - EMS responsibility

**Rationale:**
- Single read minimizes round-trips (vs EEBUS's multiple requests)
- Complete structure enables immediate understanding of device capabilities
- IANA PEN format provides globally unique IDs without central registry for all
- Manufacturer labels on endpoints helpful for multi-component devices
- Read-only simplifies device implementation

**Declined Alternatives:**
- Separate Descriptor feature: Extra round-trips, unnecessary complexity
- UseCaseInformation: Too EEBUS-specific, features list is sufficient
- Numeric vendor/product IDs: Would require central registry

**Resolves:** OPEN-001 (Feature Definitions) - DeviceInfo feature now defined

---

### DEC-032: ChargingSession Feature for EV Charging

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Need a feature for EV charging session data that accommodates different protocol capabilities (IEC 61851, ISO 15118-2, ISO 15118-20) and supports V2G bidirectional charging. Must cover EEBUS use cases: EVSOC, EVCS, SMR, EVCC/EVSECC, CEVC, DBEVC.

**Key Design Decisions:**

**1. Protocol-Aware Design:**
```
IEC 61851-1:  No smart comm - only session state/energy
ISO 15118-2:  SoC, energy demands, scheduled mode
ISO 15118-20: Dynamic control, V2G discharge constraints
```
All fields nullable to accommodate capability differences.

**2. Multiple EV Identifications:**
```cbor
evIdentifications: [
  { type: RFID, value: "04E57CD2..." },
  { type: MAC_EUI48, value: "AA:BB:CC:DD:EE:FF" },
  { type: VIN, value: "WVWZZZ3CZWE123456" },
  { type: PCID, value: "PCID-VW-2024-ABC" }
]
```
EVs can be identified via multiple mechanisms simultaneously.

**3. Energy Requests as Deltas (ISO 15118-20 style):**
```
evMinEnergyRequest = (minSoC - currentSoC) * capacity
evTargetEnergyRequest = (targetSoC - currentSoC) * capacity
evMaxEnergyRequest = (100% - currentSoC) * capacity
```
Positive = needs charging, Negative = can discharge, Zero = level reached.

**4. V2G Discharge Constraints:**
```
Can discharge = (evMinDischargingRequest < 0)
              AND (evMaxDischargingRequest >= 0)
              AND (evTargetEnergyRequest <= 0 OR evDischargeBelowTargetPermitted)
```
From EEBUS DBEVC / ISO 15118-20 cycle protection rules.

**5. Demand Mode Indicator:**
```
NONE                  - IEC 61851 (no demand info)
SINGLE_DEMAND         - Basic energy request
SCHEDULED             - ISO 15118 scheduled mode (EV plans)
DYNAMIC               - ISO 15118-20 dynamic mode (CEM controls)
DYNAMIC_BIDIRECTIONAL - ISO 15118-20 with V2G
```
Tells EMS what level of optimization is possible.

**EEBUS Use Case Coverage:**

| Use Case | Mapping |
|----------|---------|
| EVSOC | evStateOfCharge, evBatteryCapacity, evMin/Target/MaxEnergyRequest |
| EVCS | sessionEnergyCharged, sessionId |
| SMR | sessionId links to Measurement data |
| EVCC/EVSECC | evIdentifications, sessionId |
| CEVC | evMin/Target/MaxEnergyRequest, evDepartureTime, evDemandMode |
| DBEVC | evMin/MaxDischargingRequest, evDischargeBelowTargetPermitted |

**Now Covered by DEC-033:**
- CEVC incentive tables → **Signals (COMBINED) + Tariff features**
- CEVC charging plan curves → **Plan feature**
- CEVC/DBEVC power schedules → **Signals (CONSTRAINT type)**

**Rationale:**
- Accommodates all EV charging protocols in single feature
- Multiple identifications reflect real-world where EVs provide several IDs
- Energy request semantics match ISO 15118-20 for consistency
- V2G constraints essential for bidirectional charging use cases
- Demand mode indicator enables protocol-appropriate optimization

**Resolves:** OPEN-001 (Feature Definitions) - ChargingSession feature now defined, completing all initial features

---

### DEC-033: Signals, Tariff, and Plan Features

**Date:** 2025-01-25
**Status:** Accepted (Updated)

**Context:** Need to support time-based pricing (ToUT), power envelopes (POEN), coordinated EV charging (CEVC), and power forecasts. These scenarios share common data patterns (time slots) but differ in purpose and data direction.

**Research Findings:**

*EEBUS:*
- IncentiveTable with Tariff, Tiers, TierBoundaries, Incentives
- IncentiveTypes: absoluteCost, relativeCost, renewableEnergyPercentage, co2Emission
- Separate Bill feature for cost component breakdown

*Matter:*
- CommodityTariff: Structure (DayPatterns, CalendarPeriods, TariffComponents, PowerThresholds)
- CommodityPrice: Current price + forecast (up to 56 slots)
- DeviceEnergyManagement: Forecast with SlotStruct, PowerAdjustRequest, ConstraintBasedForecast

*ISO 15118 insight:*
- In CEVC, the EV is the optimizer (not the EMS)
- EMS sends incentives/limits TO EVSE via MASH
- EVSE translates to SA_ScheduleTuple via ISO 15118 TO EV
- EV returns ChargingProfile via ISO 15118 TO EVSE
- EVSE translates to Plan via MASH TO EMS

*Real-world tariffs:*
- Components vary independently: energy (~31%), grid fees (~28%), taxes/levies (~41%)
- Power-based tiers common (different rate above X kW)
- Feed-in tariffs separate from consumption tariffs
- CO2 intensity and renewable percentage increasingly important

**Options Evaluated:**
1. Single Schedule feature with all fields (bidirectional)
2. Separate features: PriceSchedule, PowerSchedule, Forecast
3. Two features: Schedule (time-varying values) + Tariff (stable structure)
4. Three features: Signals (IN) + Plan (OUT) + Tariff (stable structure)

**Decision:** Three features - Signals, Tariff, and Plan

The key insight is **data direction**:
- **Signals** = data flowing IN to device (from controllers)
- **Plan** = data flowing OUT from device (to controllers)
- **Tariff** = stable structure (rarely changes)

**Signals Feature (0x0008):**
- Time-slotted INPUT data from controllers
- Five signal types: PRICE, CONSTRAINT, TARGET, FORECAST, COMBINED
- Supports multiple concurrent signals from different sources
- Resolution rules: most restrictive for limits, highest priority for prices
- SignalSlot fields: prices, limits, targets, forecasts, environmental signals

**Tariff Feature (0x0009):**
- Defines price structure separately from time-varying values
- Components: ENERGY, GRID_FEE, TAX, LEVY, CO2, CREDIT
- Power tiers for demand-based pricing
- Separate consumption and production (feed-in) tariffs
- Signals references Tariff for component IDs

**Plan Feature (0x000A):**
- Time-slotted OUTPUT data from device
- Device's intended behavior in response to Signals
- For EVSE: reflects EV's ISO 15118 ChargingProfile
- PlanSlot fields: plannedConsumption/Production, expectedSoC
- Events notify controllers when plan changes

**Use Case Coverage:**

| Use Case | Feature(s) | Direction | Key Data |
|----------|------------|-----------|----------|
| ToUT (time-of-use tariff) | Signals (PRICE) + Tariff | IN | componentPrices per slot |
| POEN (power envelope) | Signals (CONSTRAINT) | IN | min/maxConsumption/Production |
| CEVC input | Signals (COMBINED) + Tariff | IN | prices + limits + forecast |
| CEVC output (EV plan) | Plan | OUT | plannedConsumption, expectedSoC |
| Solar forecast | Signals (FORECAST) | IN | forecastProduction, confidence |
| Load forecast | Signals (FORECAST) | IN | forecastConsumption |
| Grid CO2 forecast | Signals (FORECAST) | IN | co2Intensity |

**CEVC Flow:**
```
EMS --> SetSignal(COMBINED) --> EVSE --> SA_ScheduleTuple --> EV
                                     <-- ChargingProfile <--
    <-- Plan (via RequestPlan) <--
```

**EEBUS/Matter Coverage:**

| Source | MASH Mapping |
|--------|--------------|
| EEBUS ToUT | Signals (PRICE) + Tariff |
| EEBUS POEN | Signals (CONSTRAINT) |
| EEBUS CEVC IncentiveTable | Signals (COMBINED) + Tariff |
| EEBUS CEVC ChargingPlan | Plan |
| Matter CommodityTariff | Tariff |
| Matter CommodityPrice | Signals (PRICE) |
| Matter DeviceEnergyManagement.Forecast | Signals (FORECAST) / Plan |

**Rationale:**
- Direction-based split (IN vs OUT) clarifies data flow semantics
- Matches ISO 15118 model where EV optimizes, not EMS
- Separating structure (Tariff) from values (Signals) matches real-world contracts
- Power tiers essential for demand-based pricing (common in Europe)
- Plan feature enables EMS to integrate device intentions into overall planning

**Declined Alternatives:**
- Single bidirectional Schedule feature: Confuses input vs output semantics
- Extending EnergyControl: Conflates control with information
- Per-type schedule features: Redundant structures

**Resolves:** Open Question #7 (Scheduled Limits)

---

### DEC-034: Explicit Control State and Optional Process Lifecycle

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Analyzing EEBUS OHPCF (Optimization of Self-Consumption by Heat Pump Compressor Flexibility) and LPC (Limitation of Power Consumption) revealed two design problems in EEBUS:

1. **LPC's heartbeat-based state inference is bad**: EEBUS LPC uses heartbeats and implicit state transitions - the controller infers device state (Init, Limited, Failsafe, Unlimited) rather than reading it directly. This causes race conditions, debugging difficulty, and no single source of truth.

2. **OHPCF requires optional process lifecycle management**: Heat pumps, water heaters, and similar devices have "optional" tasks (e.g., "I could run the compressor now") that controllers schedule. This needs explicit task lifecycle tracking.

**Options Evaluated:**

*For control state:*
1. Keep EEBUS implicit heartbeat model (controller infers state)
2. Report operational state only (OFFLINE/ONLINE/FAULT)
3. Report explicit control relationship state (device tells you its control status)

*For process lifecycle:*
1. Extend Pause/Resume commands with implicit state
2. Add explicit ProcessStateEnum with full lifecycle tracking
3. Separate status/control features

**Decision:**

1. **ControlStateEnum** - explicit control relationship state reported by device:
   - `AUTONOMOUS` (0x00): Not under external control
   - `CONTROLLED` (0x01): Under controller authority, no active limit
   - `LIMITED` (0x02): Active power limit being applied
   - `FAILSAFE` (0x03): Connection lost, using failsafe limits
   - `OVERRIDE` (0x04): Device overriding for safety/legal/self-protection

2. **ProcessStateEnum** - explicit optional task lifecycle:
   - `NONE` (0x00): No optional process available
   - `AVAILABLE` (0x01): Process announced, not scheduled
   - `SCHEDULED` (0x02): Start time configured, waiting
   - `RUNNING` (0x03): Process executing
   - `PAUSED` (0x04): Paused by controller
   - `COMPLETED` (0x05): Finished successfully
   - `ABORTED` (0x06): Stopped/cancelled

3. **Failsafe configuration** - device knows what to do when controller disappears:
   - `failsafeConsumptionLimit`: limit to apply in FAILSAFE
   - `failsafeProductionLimit`: limit to apply in FAILSAFE
   - `failsafeDuration`: time in FAILSAFE before transitioning to AUTONOMOUS (2-24h)

4. **OptionalProcess struct** - for announcing optional tasks:
   - Process identification (id, description)
   - Power characteristics (estimate, min, max)
   - Timing constraints (duration, minRunDuration, minPauseDuration)
   - Control constraints (isPausable, isStoppable)
   - Energy characteristics (estimate, resumeEnergyPenalty)

5. **New commands**:
   - `Stop`: Abort task completely (vs Pause which can resume)
   - `ScheduleProcess`: Schedule an optional process to start
   - `CancelProcess`: Cancel a scheduled/running process

**Rationale:**

*Why explicit state reporting:*
- Single source of truth: device state is what device reports, not inferred
- No race conditions: no ambiguity about which state applies
- Debuggable: inspect state directly, no need to trace heartbeat history
- Universal: same ControlStateEnum works for LPC, EVSE, battery, heat pump, inverter

*Why separate ControlStateEnum from OperatingStateEnum (Status feature):*
- ControlStateEnum = "Am I being externally controlled?"
- OperatingStateEnum = "Am I working correctly?"
- Orthogonal concerns: device can be RUNNING (operational) but AUTONOMOUS (not controlled)

*Why ProcessStateEnum is orthogonal to ControlStateEnum:*
- Device can be LIMITED (control state) while process is RUNNING (task lifecycle)
- Or FAILSAFE (control state) with process PAUSED (task lifecycle)
- Different concerns: connection status vs task execution

*Failsafe rationale:*
- Devices MUST know what to do when controller disappears
- Pre-configured failsafe limits ensure safety
- Duration prevents devices being stuck in FAILSAFE forever
- Applicable to ALL device types: EVSE (stop charging after X hours), battery (discharge limits), heat pump (min heating)

**EEBUS Use Case Coverage:**

| EEBUS | MASH | Improvement |
|-------|------|-------------|
| LPC heartbeat state machine | ControlStateEnum | Explicit, not inferred |
| LPC failsafe by timeout | failsafeDuration → AUTONOMOUS | Same behavior, explicit |
| LPC "limited" state | controlState = LIMITED | Direct reading |
| OHPCF SmartEnergyManagementPs | OptionalProcess + ProcessStateEnum | Cleaner model |
| OHPCF power sequence state | processState | Explicit lifecycle |
| COB failsafe state | ControlStateEnum.FAILSAFE | Unified across device types |

**Declined Alternatives:**

- Heartbeat-based inference: Fundamentally flawed - no single source of truth
- Combined state enum: Conflates control relationship with task lifecycle
- Per-device-type state handling: Redundant, same concept applies universally

---

### DEC-035: Matter-Style Capability Discovery

**Date:** 2025-01-25
**Status:** Accepted

**Context:** MASH has multiple features (EnergyControl, Measurement, Signals, Tariff, Plan, etc.) with many optional attributes. Controllers need to know:
- Which features a device implements
- Which optional attributes are present within each feature
- Which commands the device accepts
- Device capabilities without reading each attribute individually

**Options Evaluated:**

1. **EEBUS approach**: nodeManagementUseCaseData to advertise use cases by name
2. **Trial and error**: Controller reads attributes and handles errors for missing ones
3. **Matter-style global attributes**: FeatureMap bitmap + attribute/command lists

**Decision:** Matter-style global attributes on every endpoint.

**Global Attributes (reserved IDs 0xFFF0-0xFFFF):**

| Attribute | ID | Type | Description |
|-----------|-----|------|-------------|
| `clusterRevision` | 0xFFFD | uint16 | MASH spec version |
| `featureMap` | 0xFFFC | bitmap32 | Supported optional features |
| `attributeList` | 0xFFFB | array[uint16] | Implemented attribute IDs |
| `acceptedCommandList` | 0xFFFA | array[uint8] | Accepted command IDs |
| `generatedCommandList` | 0xFFF9 | array[uint8] | Response command IDs |
| `eventList` | 0xFFF8 | array[uint8] | Supported event IDs |

**FeatureMap Bits:**

```
bit 0  (0x0001): CORE      - EnergyCore basics (always set)
bit 1  (0x0002): FLEX      - FlexibilityStruct support
bit 2  (0x0004): BATTERY   - Battery attributes (SoC, SoH)
bit 3  (0x0008): EMOB      - E-Mobility/EVSE
bit 4  (0x0010): SIGNALS   - Incentive signals
bit 5  (0x0020): TARIFF    - Tariff data
bit 6  (0x0040): PLAN      - Power plan
bit 7  (0x0080): PROCESS   - Optional process lifecycle (OHPCF)
bit 8  (0x0100): FORECAST  - Power forecasting
bit 9  (0x0200): ASYMMETRIC - Per-phase asymmetric control
bit 10 (0x0400): V2X       - Vehicle-to-grid/home
```

**Discovery Flow:**
1. Read endpoint list to discover device structure
2. For each endpoint, read `featureMap` for quick capability check
3. Read `attributeList` for fine-grained attribute discovery
4. Read `acceptedCommandList` to know which commands work

**Rationale:**

*Why Matter-style:*
- Self-describing: Controller knows exactly what's available without trial/error
- Version-safe: `clusterRevision` enables graceful protocol evolution
- Fine-grained: `attributeList` gives exact attribute availability
- Compact: Bitmap `featureMap` is efficient for quick checks
- Predictable: No implicit assumptions about device type implications

*Why not EEBUS nodeManagementUseCaseData:*
- EEBUS is coarse-grained (use case level, not attribute level)
- Use case "support" is ambiguous (which scenarios? which functions?)
- No version negotiation per use case
- Requires out-of-band spec lookup to know what attributes exist

*Why not trial and error:*
- Wastes roundtrips on error responses
- Error handling complexity
- No way to show user what device can do before trying

**Feature-Dependent Conformance:**

| Attribute | Mandatory If | Optional If |
|-----------|-------------|-------------|
| `stateOfCharge`, `stateOfHealth` | BATTERY | - |
| `sessionEnergy`, `evseState` | EMOB | - |
| `flexibility` | - | FLEX |
| `forecast` | - | FORECAST |
| `processState`, `optionalProcess` | PROCESS | - |
| Phase setpoint attributes | ASYMMETRIC | - |

**Declined Alternatives:**

- EEBUS use case discovery: Too coarse, no version handling, requires spec lookup
- Trial and error: Inefficient, poor UX, no proactive capability display
- Capability negotiation at connection time: Adds connection complexity

---

### DEC-036: Charging Mode and Responsibility Model

**Date:** 2025-01-25
**Status:** Accepted

**Context:** OSCEV 2.0 adds PV charging mode support - letting users choose optimization strategies (PV surplus only, PV with threshold, fast charging). This raises questions about:
- Where does charging mode configuration belong?
- Who controls what between CEM and EVSE?
- How to handle EV-specific timing constraints?

**Options Evaluated:**

1. **CEM controls everything**: CEM sets mode and timing, EVSE just executes
2. **EVSE controls everything**: EVSE handles optimization based on hints from CEM
3. **Split responsibility**: CEM sets goals, EVSE implements using domain knowledge

**Decision:** Split responsibility with "CEM suggests, EVSE decides within safe bounds" pattern.

**Additions to ChargingSession Feature:**

```cbor
// CHARGING MODE (optimization strategy)
70: chargingMode,            // ChargingModeEnum: active optimization strategy
71: supportedChargingModes,  // ChargingModeEnum[]: modes EVSE supports
72: surplusThreshold,        // int64 mW?: threshold for PV_SURPLUS_THRESHOLD

// START/STOP DELAYS (CEM can override, EVSE enforces)
80: startDelay,              // uint32 s: delay before (re)starting charge
81: stopDelay,               // uint32 s: delay before pausing charge
```

**ChargingModeEnum:**
```
OFF                   = 0x00  // No optimization, charge at maximum rate
PV_SURPLUS_ONLY       = 0x01  // Only self-produced energy, no grid
PV_SURPLUS_THRESHOLD  = 0x02  // Allow grid if surplus >= surplusThreshold
PRICE_OPTIMIZED       = 0x03  // Optimize based on price signals
SCHEDULED             = 0x04  // Follow time-based schedule/plan
```

**Responsibility Model:**

| Domain | Owner | Responsibility |
|--------|-------|----------------|
| System optimization | CEM/EMS | Goals, prices, grid constraints, PV forecasts |
| EV behavior | EVSE | Protocol handling, timing, hardware limits |

- **CEM sets** charging mode and can override delays
- **EVSE validates** requests against EV/hardware constraints
- **EVSE enforces** behavior using its domain knowledge
- **EVSE reports** active mode, constraints, and deviations
- EVSE may **reject** values that would harm the EV

**Rationale:**

*Why split responsibility:*
- Each party contributes expertise from their domain
- CEM doesn't need to know EV-specific timing details
- EVSE can protect EV from harmful requests
- Similar to failsafe pattern already in EnergyControl

*Why start/stop delays:*
- Prevents EVs from stopping completely due to frequent PV-induced interruptions
- EVSE knows connected EV's requirements
- CEM can override if system needs demand, EVSE still enforces safety

**EEBUS Coverage:**

| OSCEV 2.0 Scenario | MASH Mapping |
|-------------------|--------------|
| Scenario 5 - PV Charge Mode | chargingMode, surplusThreshold |
| Scenario 6 - Start/Stop Delays | startDelay, stopDelay |

**Declined Alternatives:**

- CEM controls everything: CEM lacks EV-specific knowledge
- EVSE controls everything: Misses system-wide optimization opportunities
- Separate feature for charging mode: Overcomplicates; logically part of session

---

### DEC-036b: Dynamic Electrical Feature for Connected Devices

**Date:** 2025-01-25
**Status:** Accepted

**Context:** When an EV connects to an EVSE, the system's effective capability changes (e.g., 22kW EVSE + 7.4kW EV = 7.4kW effective max). Where should EV constraints (min/max power, min/max current) be reported?

**Options Evaluated:**

1. **ChargingSession**: Add `evMinChargingPower`, `evMaxChargingPower`, etc.
2. **EnergyControl**: Dynamic constraint fields
3. **Electrical**: Existing fields update when EV connects

**Decision:** Electrical feature is dynamic and reflects current system capability.

**Design:**

```
┌─────────────────────────────────────────────────────────────┐
│ Electrical (capability)                                     │
│   → "What CAN this endpoint do right now?"                  │
│   → Dynamic: updates when connected device changes          │
│   → nominalMaxConsumption, maxCurrentPerPhase, etc.         │
├─────────────────────────────────────────────────────────────┤
│ EnergyControl (policy)                                      │
│   → "What SHOULD this endpoint do?"                         │
│   → CEM-set limits and setpoints                            │
│   → Must be within Electrical's envelope                    │
└─────────────────────────────────────────────────────────────┘
```

**Rationale:**

*Why Electrical:*
- It's truly a capability change, not a policy change
- "What CAN this endpoint do" naturally includes connected devices
- No field duplication across features
- CEM subscribes to Electrical, sees capability changes automatically
- Clean separation: capability vs policy

*Why not ChargingSession:*
- Would duplicate min/max fields that already exist in Electrical
- CEM would have to calculate effective range itself
- Mixes session state with system capability

*Why not EnergyControl:*
- EnergyControl is for CEM-set policy, not physical capability
- Would blur the line between "what's possible" and "what's allowed"

**Example flow:**
1. EVSE reports Electrical with hardware limits (22kW, 32A)
2. EV connects with 7.4kW/16A max, 1.4kW/6A min
3. EVSE updates Electrical: `nominalMaxConsumption=7400000`, `maxCurrentPerPhase=16000`, `nominalMinPower=1400000`
4. CEM (subscribed) sees change, adjusts limits accordingly
5. EV disconnects → Electrical returns to EVSE hardware limits

---

### DEC-037: Two-Level Capability Discovery Pattern

**Date:** 2025-01-25
**Status:** Accepted

**Context:** With multiple capability flags across features (asymmetric support, V2X, bidirectional, charging modes), controllers need efficient discovery. Should all details be in featureMap bits, or distributed between featureMap and attributes?

**Options Evaluated:**

1. **Fine-grained featureMap**: Separate bits for every capability variant (ASYMMETRIC_CHARGING, ASYMMETRIC_DISCHARGING, ASYMMETRIC_BIDIRECTIONAL, etc.)
2. **Coarse-grained featureMap**: High-level bits (ASYMMETRIC) with details in attributes
3. **No featureMap**: All discovery via attribute reading

**Decision:** Two-level capability discovery with 32-bit featureMap.

**Pattern:**
- **FeatureMap** (32-bit bitmap) = high-level category check ("does it support this?")
- **Feature attributes** = detailed capability information ("how exactly?")

**FeatureMap Bits (high-level):**
```
ASYMMETRIC (0x0200) → "Supports per-phase control"
V2X (0x0400)        → "Supports bidirectional EV"
EMOB (0x0008)       → "Has EV charging"
```

**Detailed Information (in attributes):**
```
Electrical.supportsAsymmetric: NONE/CONSUMPTION/PRODUCTION/BIDIRECTIONAL
Electrical.supportedDirections: CONSUMPTION/PRODUCTION/BIDIRECTIONAL
ChargingSession.supportedChargingModes: [OFF, PV_SURPLUS_ONLY, ...]
ChargingSession.evDemandMode: DYNAMIC_BIDIRECTIONAL
```

**Discovery Flow:**
1. Read `featureMap` → quick check what categories are supported
2. Read relevant feature attributes → get specific capability details

**Example - V2H EVSE with asymmetric charging but symmetric discharging:**
- `featureMap`: CORE | EMOB | ASYMMETRIC | V2X (quick: "yes it does V2X and asymmetric")
- `Electrical.supportsAsymmetric = CONSUMPTION` (detail: "but only for charging")
- `Electrical.supportedDirections = BIDIRECTIONAL` (detail: "can charge and discharge")

**Rationale:**

*Why 32-bit:*
- Aligned with Matter (BITMAP32)
- CBOR encodes small values efficiently anyway
- Room for future expansion (currently 11 bits used)

*Why two levels:*
- FeatureMap for quick filtering ("show me all V2X capable devices")
- Attributes for accurate capability matching ("can it do asymmetric V2G?")
- Prevents combinatorial explosion of featureMap bits
- Detailed enums are more expressive than boolean bits

*Why not all details in featureMap:*
- ASYMMETRIC × V2X × directions = many combinations
- Bits are limited (32), enums are unlimited
- Easier to add new enum values than new featureMap bits

**Declined Alternatives:**

- Fine-grained featureMap: Combinatorial explosion, quickly runs out of bits
- No featureMap: Requires reading many attributes for basic filtering
- 16-bit featureMap: Works for now but limits future expansion

---

### DEC-038: Command Parameters vs Stored Attributes

**Date:** 2025-01-25
**Status:** Accepted

**Context:** Commands like `SetLimit` have parameters like `duration` that control behavior. Question: Are these stored and readable like attributes, or transient like Matter's command parameters?

**Protocol Comparison:**

| Protocol | Approach |
|----------|----------|
| **EEBUS** | All function data is stored and readable. Writing to a function persists all fields. No concept of "transient parameters." |
| **Matter** | Commands have parameters that control execution but aren't stored. `transitionTime` in LevelControl is passed at invocation but not readable as an attribute. |

**Options Evaluated:**

1. **EEBUS model**: Store all command parameters as readable attributes (e.g., `limitDuration`, `limitCause`)
2. **Matter model**: Command parameters are transient - control behavior but aren't persisted
3. **Hybrid**: Some parameters stored, some transient

**Decision:** Matter model - command parameters are transient, not stored attributes.

**Design:**

| Concept | Behavior | Example |
|---------|----------|---------|
| **Attribute** | Stored, readable, subscribable | `myConsumptionLimit`, `effectiveConsumptionLimit` |
| **Command parameter** | Transient, not readable, controls execution | `duration`, `cause` |

**After SetLimit(consumptionLimit: 5000000, duration: 60, cause: LOCAL_PROTECTION):**
- `myConsumptionLimit` = 5000000 (readable, subscribable)
- `duration` = not accessible (internal timer running)
- `cause` = not accessible (logged but not queryable)

**Implications:**

1. **No "remaining duration" attribute**: Timer is internal, not exposed
2. **Controller tracks expiry locally**: If controller needs to know, it calculates `now + duration`
3. **To change duration**: Re-send entire SetLimit command
4. **To remove timer**: Send SetLimit with `duration: 0` or omit duration
5. **null invalid for command parameters**: Use omission for defaults, not null

**Omitting Optional Parameters:**

| In command | Meaning |
|------------|---------|
| Key absent | Use default value |
| Key with value | Use this value |
| Key with `null` | Invalid - don't use null for command parameters |

**Rationale:**

*Why Matter model over EEBUS model:*
- **Simpler device implementation**: Don't need to store/manage every parameter
- **Clearer semantics**: Attributes are state, commands are actions
- **Less state to synchronize**: Transient parameters don't need notification
- **Controller owns scheduling context**: Controller set the duration, controller knows when it expires
- **Matter-aligned**: Familiar pattern for developers

*Why no remaining duration attribute:*
- Would require continuous updates or on-demand calculation
- Adds complexity for marginal benefit
- Controller can track locally (it set the duration)
- Other zones don't need to know when a limit expires - they only care about the effective limit value

**Test Implications:**
- TC-ZONE-LIMIT-6c: Verify duration is NOT readable as an attribute
- TC-ZONE-LIMIT-6a: Verify re-sending command replaces timer

**Declined Alternatives:**

- EEBUS model (all stored): Adds complexity, unclear benefit for transient control parameters
- Hybrid model: Complicates the mental model - either it's state or it's control, not both
- Adding `remainingDuration` attribute: Continuous change or on-demand calculation adds complexity

---

### DEC-039: State Machine Interaction Rules

**Date:** 2025-01-25
**Status:** Accepted

**Context:** MASH has two orthogonal state machines: ControlStateEnum (controller relationship) and ProcessStateEnum (optional task lifecycle). The testability analysis identified that interaction rules were unspecified - what happens to ProcessState when ControlState changes?

**Key Scenarios:**

| Scenario | Question |
|----------|----------|
| Connection lost during RUNNING process | Does process pause or continue? |
| Connection lost during PAUSED process | Does it stay paused or auto-resume on failsafe expiry? |
| Failsafe expires during SCHEDULED process | Does scheduled process still start? |
| Reconnection during active process | Who owns the process? |

**Decision:** Process lifecycle continues independently of ControlState transitions.

**Rules:**

1. **RUNNING processes continue during FAILSAFE**
   - Process operates under failsafe limits (more restrictive)
   - Process can complete during FAILSAFE
   - Rationale: Safer to complete (e.g., dishwasher cycle) than leave mid-operation

2. **SCHEDULED processes start as planned during FAILSAFE**
   - Device starts process at scheduledStart without controller confirmation
   - Process runs under failsafe limits
   - Rationale: User expectation is scheduled task runs

3. **PAUSED processes remain paused during FAILSAFE**
   - Device-specific behavior on failsafe expiry (PICS item)
   - Conservative devices: stay paused
   - Aggressive devices: auto-resume
   - PICS: `MASH.S.CTRL.B_PAUSED_AUTO_RESUME`

4. **Reconnection restores control, doesn't cancel**
   - Controller sees process state in subscription priming
   - Controller can interact (Pause, Cancel) but process continues by default
   - Process ownership persists across disconnection

**FAILSAFE Timing Precision:**

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Maximum detection delay | 95 seconds | 3 missed pongs @ 30s + 5s timeout |
| Failsafe timer accuracy | +/- 1% | 24h failsafe = +/- 14 minutes |
| Failsafe limit application | Immediate | Safety-critical, no gradual ramp |
| Reconnection race window | 5 seconds | If handshake completes within 5s of timer, reconnection wins |

**Rationale:**

*Why continue processes:*
- User scheduled the task expecting it to run
- Mid-operation abort can cause problems (wet laundry, half-heated water)
- Failsafe limits provide grid safety regardless
- Controller regains control on reconnection

*Why PAUSED is device-specific:*
- Paused implies intentional stop - conservative to maintain that
- But some devices may want to resume after extended disconnect
- Make it explicit via PICS so controllers know what to expect

**Test Cases:**
- TC-INTERACTION-1 through TC-INTERACTION-15 cover all interaction scenarios
- TC-STATE-5 through TC-STATE-9 cover FAILSAFE behavior
- TC-PROCESS-* cover ProcessState transitions

**Documentation:** See `docs/testing/behavior/state-machines.md` for complete specification.

---

### DEC-040: Device Identity via Certificate Fingerprint

**Date:** 2026-01-26
**Status:** Proposed

**Context:** Need a stable, verifiable device identity for:
- Reconnection after restarts (controller matches device)
- Certificate pinning/validation
- Persistence of commissioned device relationships

**Options Evaluated:**
1. Subject Key Identifier (SKI) - 20-byte hash of public key
2. Full certificate fingerprint - SHA-256 hash of entire certificate
3. PASE-derived ID - hash of SPAKE2+ shared secret

**Decision:** Full certificate fingerprint (SHA-256)

**Rationale:**
- More collision-resistant than SKI (32 bytes vs 20 bytes)
- Covers entire certificate content, not just public key
- Standard practice for certificate pinning
- If certificate changes (renewal, rotation), identity changes appropriately
- Easy to compute: `sha256(certificate.Raw)` in Go

**Implementation:**
- Device generates/loads persistent TLS certificate on startup
- Device ID = hex-encoded SHA-256 of DER-encoded certificate
- Controller stores device's certificate fingerprint during commissioning
- On reconnection, controller verifies fingerprint matches

**Declined Alternatives:**
- SKI: Smaller but less collision-resistant, only covers public key
- PASE-derived: Different for each commissioning session, not tied to certificate

**Related:** OPEN-002 (Certificate and Session Details)

---

## Open Questions (To Be Addressed)

### OPEN-001: Feature Definitions (RESOLVED)

**Context:** Need to define specific features for our three initial use cases.

**Status: ALL COMPLETE**
- [x] Electrical feature - **DEC-027**
- [x] Measurement feature - **DEC-027**, **DEC-029** (comprehensive AC/DC)
- [x] EnergyControl/Limit feature - **DEC-026**, **DEC-027**, **DEC-028**
- [x] EndpointType enum - **DEC-029**
- [x] Status feature - **DEC-030** (per-endpoint operating state)
- [x] DeviceInfo feature - **DEC-031** (identity + complete device structure)
- [x] ChargingSession feature - **DEC-032** (EV session, demands, V2G)
- [x] Signals feature - **DEC-033** (time-slotted input: ToUT, POEN, forecasts)
- [x] Tariff feature - **DEC-033** (price structure, components, power tiers)
- [x] Plan feature - **DEC-033** (time-slotted output: device's intended behavior)

---

### OPEN-002: Certificate and Session Details

**Context:** Security model needs implementation details.

**Questions:**
- Certificate format (X.509? Custom?)
- Session key derivation
- Certificate rotation
- TLS cipher suite requirements

---

### OPEN-003: Error Handling

**Context:** Need to define error response format.

**Questions:**
- Error codes taxonomy
- Retry semantics
- Connection error vs command error

---

### OPEN-004: Version Negotiation

**Context:** SPINE lacks version negotiation, causing interop issues.

**Questions:**
- How to negotiate protocol version?
- How to handle feature version mismatches?
- Backwards compatibility strategy

---

### OPEN-005: Discovery Details

**Context:** mDNS/DNS-SD for discovery, QR code for commissioning.

**Questions:**
- mDNS TXT record format
- Service type naming
- QR code content format (like Matter's setup payload?)

---

## Declined Proposals

*(None yet - document declined ideas here with reasons)*

---

## Research Notes

### Matter Protocol Insights

**Source:** [Matter Documentation](https://project-chip.github.io/connectedhomeip-doc/)

Key learnings:
- Matter has 4 interaction types: Read, Write, Subscribe, Invoke
- Clusters are well-defined with mandatory/optional attributes
- Server is stateful, Client is stateless
- Events are timestamped journal entries (not just current state)

### EEBUS Failure Analysis

**Source:** ship-go and spine-go analysis documents

Key learnings:
- 50+ specification ambiguities in SHIP alone
- 7,000+ potential RFE combinations create testing nightmare
- "Appropriate client" never defined - security hole
- No test specifications or reference implementations
- Double connection "most recent" rule causes race conditions

---

## Revision History

| Date | Changes |
|------|---------|
| 2025-01-24 | Initial creation with first decisions |
| 2025-01-25 | Added DEC-026: EnergyControl feature design (capability-first, forecast-optional) |
| 2025-01-25 | Added DEC-027: Feature separation (Electrical, Measurement, EnergyControl) with per-phase current limits and phase mapping |
| 2025-01-25 | Added DEC-028: Setpoints and V2H phase balancing - SetSetpoint, SetCurrentSetpoints commands, AsymmetricSupportEnum |
| 2025-01-25 | Added DEC-029: Measurement feature (comprehensive AC/DC) and EndpointType enum for multi-component devices |
| 2025-01-25 | Added DEC-030: Status feature for per-endpoint operating state (separates observation from control) |
| 2025-01-25 | Added DEC-031: DeviceInfo feature with IANA PEN-based deviceId, complete device structure in single read |
| 2025-01-25 | Added DEC-032: ChargingSession feature for EV charging with ISO 15118 support |
| 2025-01-25 | Added DEC-033: Signals + Tariff + Plan features for ToUT, POEN, CEVC, forecasts. Direction-based split: Signals (IN), Plan (OUT), Tariff (structure) |
| 2025-01-25 | Added DEC-034: Explicit ControlStateEnum and ProcessStateEnum - replaces implicit heartbeat-based state inference with explicit device-reported state. Universal across LPC, EVSE, battery, heat pump. Adds failsafe config and OptionalProcess for OHPCF-style task scheduling. |
| 2025-01-25 | Added DEC-035: Matter-style capability discovery with global attributes (featureMap, attributeList, acceptedCommandList). Defines 11 feature flags for optional capability sets. Enables self-describing devices without trial-and-error attribute reading. |
| 2025-01-25 | Added DEC-036: Charging mode and responsibility model. ChargingModeEnum for optimization strategy (PV surplus, price optimized). "CEM suggests, EVSE decides" pattern with start/stop delays. Covers OSCEV 2.0 use cases. |
| 2025-01-25 | Updated DEC-036: Electrical feature is dynamic - reflects current system capability including connected devices (e.g., EV). No separate EV constraint fields needed; Electrical updates when EV connects. Clean two-layer model: Electrical (capability) + EnergyControl (policy). |
| 2025-01-25 | Added DEC-037: Two-level capability discovery. FeatureMap is 32-bit bitmap (aligned with Matter) for high-level category checks. Detailed capabilities in feature attributes (supportsAsymmetric, supportedDirections, supportedChargingModes). Pattern: featureMap for quick check, attributes for specifics. |
| 2025-01-25 | Added DEC-038: Command parameters vs stored attributes. Duration, cause are transient command parameters (like Matter), not stored attributes (unlike EEBUS). No "remaining duration" attribute. |
| 2025-01-25 | Added DEC-039: State machine interaction rules. Process continues during FAILSAFE, scheduled processes start as planned, PAUSED behavior is device-specific (PICS). Connection loss detection max 95s, failsafe timer accuracy +/- 1%. |
