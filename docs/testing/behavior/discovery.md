# Discovery Behavior

> Precise specification of mDNS/DNS-SD and QR code discovery

**Status:** Draft
**Created:** 2025-01-25
**Updated:** 2025-01-25

---

## 1. Overview

MASH discovery uses **three separate mDNS service types** (following the Matter model):

1. **Commissionable Discovery (`_mashc._udp`)** - Find devices ready for pairing
2. **Operational Discovery (`_mash._tcp`)** - Find commissioned devices in zones
3. **Commissioner Discovery (`_mashd._udp`)** - Find zone controllers (EMSs)

**Key principles:**
- DNS-SD compliant (RFC 6763)
- Separate namespaces for commissioning vs operational
- Minimal TXT record size for constrained devices
- QR code contains all commissioning information

---

## 2. Service Types

### 2.1 Commissionable Discovery (`_mashc._udp`)

**Purpose:** Discover devices that are ready to be commissioned (paired).

```
Service Type:  _mashc._udp
Domain:        local
Port:          8443 (UDP for PASE, then TCP for operational)
```

**When advertised:**
- Device is in commissioning mode (button pressed, window open)
- Removed when commissioning window closes (timeout or success)

**Instance name:** `MASH-<discriminator>`

**Example:**
```
_mashc._udp.local.            PTR   MASH-1234._mashc._udp.local.
MASH-1234._mashc._udp.local.  SRV   0 0 8443 evse-001.local.
MASH-1234._mashc._udp.local.  TXT   "D=1234" "VP=1234:5678" "DT=EVSE"
evse-001.local.               AAAA  fe80::1234:5678:9abc:def0
```

### 2.2 Operational Discovery (`_mash._tcp`)

**Purpose:** Discover commissioned devices for operational communication.

```
Service Type:  _mash._tcp
Domain:        local
Port:          8443 (TCP/TLS WebSocket)
```

**When advertised:**
- Device has at least one zone membership
- One service instance per zone membership (device in 2 zones = 2 instances)

**Instance name:** `<zone-id>-<device-id>`

Where:
- `zone-id`: First 8 hex chars of SHA-256(Zone CA certificate DER)
- `device-id`: First 8 hex chars of SHA-256(device operational cert public key DER)

Both IDs are **fingerprint-derived** - no assignment or vendor registration needed.

**Note:** Device ID is zone-specific. Same physical device in two zones has different device IDs (different operational certs per zone). This is intentional - the device has a distinct identity in each zone.

**Example (device in two zones):**
```
_mash._tcp.local.                         PTR   A1B2C3D4-F9E8D7C6._mash._tcp.local.
_mash._tcp.local.                         PTR   E5F6A7B8-B3A29180._mash._tcp.local.
A1B2C3D4-F9E8D7C6._mash._tcp.local.       SRV   0 0 8443 evse-001.local.
A1B2C3D4-F9E8D7C6._mash._tcp.local.       TXT   "ZI=A1B2C3D4" "DI=F9E8D7C6" "VP=1234:5678"
E5F6A7B8-B3A29180._mash._tcp.local.       SRV   0 0 8443 evse-001.local.
E5F6A7B8-B3A29180._mash._tcp.local.       TXT   "ZI=E5F6A7B8" "DI=B3A29180" "VP=1234:5678"
```

### 2.3 Commissioner Discovery (`_mashd._udp`)

**Purpose:** Allow devices to discover zone controllers (EMSs) on the network.

```
Service Type:  _mashd._udp
Domain:        local
Port:          8443
```

**When advertised:**
- Zone controller is ready to accept commissioning requests
- Used for device-initiated pairing (device with screen finds EMSs)

**Instance name:** `<zone-name>` (user-friendly name, max 63 chars)

**Example:**
```
_mashd._udp.local.                   PTR   Home Energy._mashd._udp.local.
Home Energy._mashd._udp.local.       SRV   0 0 8443 ems-controller.local.
Home Energy._mashd._udp.local.       TXT   "ZN=Home Energy" "VP=ABCD:0001" "ZI=A1B2C3D4"
ems-controller.local.                AAAA  2001:db8::1
```

**Use case flow:**
1. Device with display enters "find controller" mode
2. Device browses `_mashd._udp.local`
3. Device shows list: "Home Energy", "Office EMS", etc.
4. User selects a controller
5. Device initiates commissioning with that controller

### 2.4 Service Type Summary

| Service Type | Purpose | When Present | Instance Name |
|--------------|---------|--------------|---------------|
| `_mashc._udp` | Find devices to commission | Commissioning window open | `MASH-<discriminator>` |
| `_mash._tcp` | Find operational devices | Device has zone(s) | `<zone-id>-<device-id>` |
| `_mashd._udp` | Find commissioners | Controller ready | `<zone-name>` |

### 2.5 Instance Naming Constraints

**All instance names:**
- Maximum length: 63 characters (DNS label limit)
- Allowed characters: A-Z, a-z, 0-9, hyphen (-), space (for zone names)
- Must not start or end with hyphen
- Case-insensitive comparison

**ID derivation:**
```
Zone ID   = hex(SHA-256(Zone CA certificate DER)[0:4])     // 8 hex chars
Device ID = hex(SHA-256(device op cert public key DER)[0:4]) // 8 hex chars
```

**Benefits of fingerprint-derived IDs:**
- No vendor registration required (works for open source)
- No ID assignment/coordination needed
- Cryptographically bound to certificates
- Device can compute its own ID
- Verifiable by anyone with the certificate

---

## 3. TXT Record Specification

### 3.1 TXT Record Format

TXT records follow RFC 6763 key=value format:

```
key=value
```

**Encoding rules:**
- Keys: ASCII only, case-insensitive, 1-9 characters
- Values: UTF-8, maximum 200 bytes per value
- Total TXT record: Maximum 400 bytes (leaves headroom for DNS overhead)

### 3.2 Commissionable TXT Records (`_mashc._udp`)

| Key | Type | Required | Max Len | Description |
|-----|------|----------|---------|-------------|
| `D` | uint16 | Yes | 4 | Discriminator (0-4095 as decimal) |
| `VP` | string | Yes | 11 | Vendor:Product ID (hex:hex, e.g., "1234:5678") |
| `DT` | string | No | 20 | Device type label (e.g., "EVSE", "HeatPump") |
| `DN` | string | No | 32 | Device name (user-friendly) |

**Example:**
```
D=1234
VP=1234:5678
DT=EVSE
DN=Garage Charger
```

**Total size:** ~60 bytes typical, 100 bytes maximum

**Note:** No `CM` flag needed - presence of `_mashc._udp` service indicates commissioning mode.

### 3.3 Operational TXT Records (`_mash._tcp`)

| Key | Type | Required | Max Len | Description |
|-----|------|----------|---------|-------------|
| `ZI` | string | Yes | 8 | Zone ID (first 8 hex chars of SHA-256(Zone CA cert DER)) |
| `DI` | string | Yes | 8 | Device ID (first 8 hex chars of SHA-256(device op cert pubkey DER)) |
| `VP` | string | No | 11 | Vendor:Product ID (optional, for debugging) |
| `FW` | string | No | 20 | Firmware version (semver) |
| `EP` | uint8 | No | 3 | Endpoint count |
| `FM` | string | No | 10 | Feature map (hex, e.g., "0x001B") |

**Example:**
```
ZI=A1B2C3D4
DI=F9E8D7C6
VP=1234:5678
FW=1.2.3
EP=2
FM=0x001B
```

**Total size:** ~60 bytes typical, 100 bytes maximum

**Note:** Both ZI and DI are fingerprint-derived. VP is optional (useful for debugging but not required for open source implementations).

### 3.4 Commissioner TXT Records (`_mashd._udp`)

| Key | Type | Required | Max Len | Description |
|-----|------|----------|---------|-------------|
| `ZN` | string | Yes | 32 | Zone name (user-friendly) |
| `ZI` | string | Yes | 8 | Zone ID (first 8 hex chars of Zone CA fingerprint) |
| `VP` | string | Yes | 11 | Vendor:Product ID of controller |
| `DN` | string | No | 32 | Controller name (user-friendly) |
| `DC` | uint8 | No | 3 | Device count in zone |

**Example:**
```
ZN=Home Energy
ZI=A1B2C3D4
VP=ABCD:0001
DN=Smart EMS Pro
DC=5
```

**Total size:** ~70 bytes typical, 120 bytes maximum

### 3.5 TXT Record Update Rules

**When to update mDNS:**

| Event | Service | Action |
|-------|---------|--------|
| Enter commissioning mode | `_mashc._udp` | Register service |
| Exit commissioning mode (timeout) | `_mashc._udp` | Deregister service |
| Commissioning complete | `_mashc._udp` | Deregister service |
| Commissioning complete | `_mash._tcp` | Register new zone instance |
| Zone added | `_mash._tcp` | Register additional instance |
| Zone removed | `_mash._tcp` | Deregister that instance |
| All zones removed | `_mash._tcp` | Deregister all instances |
| Firmware update | `_mash._tcp` | Update FW in all instances |
| Device joins zone | `_mashd._udp` | Update DC |

**Update timing:**
- mDNS change within 1 second of state change
- mDNS goodbye (TTL=0) before removing old record
- mDNS announcement (3 times) for new record

### 3.6 Character Encoding

| Field | Encoding | Allowed Characters |
|-------|----------|--------------------|
| Keys | ASCII | a-z, A-Z, 0-9 |
| Discriminator | Decimal | 0-9 |
| Vendor/Product ID | Hex | 0-9, a-f, A-F, colon |
| Zone ID | Hex | 0-9, A-F |
| Device ID | ASCII | A-Z, a-z, 0-9, hyphen |
| Device/Zone Name | UTF-8 | Any Unicode (avoid control chars) |
| Firmware | ASCII | 0-9, period, hyphen |
| Feature Map | Hex | 0-9, a-f, A-F, x |

---

## 4. QR Code Specification

### 4.1 Content Format

```
MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
```

| Field | Format | Range | Example |
|-------|--------|-------|---------|
| version | Decimal | 1-255 | `1` |
| discriminator | Decimal | 0-4095 | `1234` |
| setupcode | Decimal | 00000000-99999999 | `12345678` |
| vendorid | Hex (0x prefix) | 0x0000-0xFFFF | `0x1234` |
| productid | Hex (0x prefix) | 0x0000-0xFFFF | `0x5678` |

**Complete example:**
```
MASH:1:1234:12345678:0x1234:0x5678
```

### 4.2 Numeric Formatting Rules

| Field | Leading Zeros | Prefix |
|-------|---------------|--------|
| version | No | None |
| discriminator | No | None |
| setupcode | **Yes** (8 digits) | None |
| vendorid | **No** | `0x` |
| productid | **No** | `0x` |

**Correct:**
```
MASH:1:1234:00001234:0x1234:0x5678   // setupcode has leading zeros
MASH:1:0:99999999:0x0:0x0           // zero values without leading zeros
```

**Incorrect:**
```
MASH:01:1234:1234:0x1234:0x5678     // version has leading zero
MASH:1:1234:1234:0x001234:0x5678    // vendorid has leading zeros
```

### 4.3 QR Code Parameters

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Error correction | M (15%) | Balance size vs readability |
| Minimum version | 2 | Fits typical content |
| Maximum version | 6 | Keeps code scannable |
| Encoding mode | Alphanumeric | Most efficient for MASH format |

**Recommended size:**
- Minimum: 20mm x 20mm (phone scan at 30cm)
- Recommended: 30mm x 30mm (reliable scanning)

### 4.4 QR Code Generation

The QR code encodes the string directly. No binary encoding.

**Content length:** 38-50 characters typical
- `MASH:` = 5
- Version + colon = 2-4
- Discriminator + colon = 2-5
- Setup code + colon = 9
- Vendor ID + colon = 3-7
- Product ID = 3-7

### 4.5 Parsing Algorithm

```python
def parse_qr(content: str) -> dict:
    """Parse MASH QR code content."""
    if not content.startswith("MASH:"):
        raise ValueError("Invalid prefix")

    parts = content.split(":")
    if len(parts) != 6:
        raise ValueError("Invalid field count")

    _, version, discriminator, setupcode, vendorid, productid = parts

    # Validate version
    version = int(version)
    if version < 1 or version > 255:
        raise ValueError("Version out of range")

    # Validate discriminator
    discriminator = int(discriminator)
    if discriminator < 0 or discriminator > 4095:
        raise ValueError("Discriminator out of range")

    # Validate setup code (must be 8 digits)
    if len(setupcode) != 8 or not setupcode.isdigit():
        raise ValueError("Invalid setup code format")

    # Validate vendor/product IDs
    if not vendorid.startswith("0x") or not productid.startswith("0x"):
        raise ValueError("Missing 0x prefix")

    vendorid = int(vendorid, 16)
    productid = int(productid, 16)

    if vendorid < 0 or vendorid > 0xFFFF:
        raise ValueError("Vendor ID out of range")
    if productid < 0 or productid > 0xFFFF:
        raise ValueError("Product ID out of range")

    return {
        "version": version,
        "discriminator": discriminator,
        "setupcode": setupcode,  # Keep as string (preserve leading zeros)
        "vendorid": vendorid,
        "productid": productid
    }
```

---

## 5. Discriminator Handling

### 5.1 Purpose

The discriminator helps filter multiple devices during discovery:
- 12-bit value (0-4095)
- Encoded in QR code and mDNS TXT record
- Used to match QR scan to discovered device

### 5.2 Assignment

| Method | Description |
|--------|-------------|
| Random | Generated at manufacturing (recommended) |
| Serial-derived | Hash of serial number mod 4096 |
| Configurable | User-configurable (less common) |

### 5.3 Collision Handling

If multiple devices have the same discriminator:
1. Controller discovers multiple devices with matching D value
2. Controller attempts commissioning with each
3. SPAKE2+ verification fails for wrong device (wrong setup code)
4. Controller retries with next matching device

**Probability:** With random assignment, ~0.02% collision chance per additional device.

### 5.4 Discriminator in Instance Name

Pre-commissioning instance name includes discriminator:

```
MASH-<discriminator>._mash._tcp.local
```

This allows DNS-SD PTR query filtering:
```
; Query for specific discriminator
MASH-1234._mash._tcp.local
```

---

## 6. Discovery State Machine

### 6.1 Device States

```
┌────────────────────────────────────────────────────────────────────────────┐
│                        Discovery State Machine                              │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────┐                                                       │
│   │ UNREGISTERED    │  No mDNS services (powered off)                       │
│   └────────┬────────┘                                                       │
│            │ Power on                                                       │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │ UNCOMMISSIONED  │  No mDNS services                                     │
│   │                 │  (Waiting for user action)                            │
│   └────────┬────────┘                                                       │
│            │ Button press                                                   │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │ COMMISSIONING   │  _mashc._udp: MASH-<D>                                │
│   │  OPEN (120s)    │  (Ready for pairing)                                  │
│   └────────┬────────┘                                                       │
│            │                                                                │
│       ┌────┴────┐                                                           │
│       │         │                                                           │
│       ▼         ▼                                                           │
│   Timeout    Success                                                        │
│       │         │                                                           │
│       ▼         ▼                                                           │
│   ┌─────────────────┐   ┌─────────────────────────────────────────┐         │
│   │ UNCOMMISSIONED  │   │            OPERATIONAL                   │         │
│   │ (no services)   │   │  _mash._tcp: <zone-id>-<device-id>      │         │
│   └─────────────────┘   │  (one instance per zone membership)      │         │
│                         └────────────────┬────────────────────────┘         │
│                                          │                                  │
│                                          │ Button press (add zone)          │
│                                          ▼                                  │
│                         ┌─────────────────────────────────────────┐         │
│                         │   OPERATIONAL + COMMISSIONING           │         │
│                         │  _mash._tcp: existing zones             │         │
│                         │  _mashc._udp: MASH-<D> (for new zone)   │         │
│                         └─────────────────────────────────────────┘         │
│                                                                             │
└────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Transition Rules

```
UNREGISTERED → UNCOMMISSIONED  : Power on (no zones)
UNREGISTERED → OPERATIONAL     : Power on (has zones)

UNCOMMISSIONED → COMMISSIONING_OPEN : Button press / API trigger
UNCOMMISSIONED → UNREGISTERED       : Power off

COMMISSIONING_OPEN → UNCOMMISSIONED : 120s timeout (no zones)
COMMISSIONING_OPEN → OPERATIONAL    : Commissioning success (first zone)
COMMISSIONING_OPEN → UNREGISTERED   : Power off

OPERATIONAL → OPERATIONAL+COMMISSIONING : Button press (add zone)
OPERATIONAL → UNCOMMISSIONED            : All zones removed
OPERATIONAL → UNREGISTERED              : Power off

OPERATIONAL+COMMISSIONING → OPERATIONAL : Timeout or success
```

### 6.3 mDNS Updates per State

| Transition | `_mashc._udp` | `_mash._tcp` |
|------------|---------------|--------------|
| → UNCOMMISSIONED | - | - |
| → COMMISSIONING_OPEN | Register MASH-D | - |
| COMMISSIONING_OPEN timeout | Deregister | - |
| COMMISSIONING_OPEN success | Deregister | Register zone-device instance |
| → OPERATIONAL+COMMISSIONING | Register MASH-D | Keep existing |
| Add zone (operational) | - | Add new instance |
| Remove zone (operational) | - | Remove that instance |
| → UNREGISTERED | Deregister | Deregister all |

### 6.4 Zone Controller (Commissioner) States

```
┌────────────────────────────────────────────────────────────────────────────┐
│                   Zone Controller Discovery States                          │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────┐                                                       │
│   │ INACTIVE        │  No mDNS services (powered off / no zone)             │
│   └────────┬────────┘                                                       │
│            │ Zone created                                                   │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │ ACTIVE          │  _mashd._udp: <zone-name>                             │
│   │                 │  (Accepting commissioning requests)                   │
│   └────────┬────────┘                                                       │
│            │                                                                │
│            │ Zone deleted                                                   │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │ INACTIVE        │  Deregister _mashd._udp                               │
│   └─────────────────┘                                                       │
│                                                                             │
└────────────────────────────────────────────────────────────────────────────┘
```

**Multiple zones:** Controller with multiple zones registers one `_mashd._udp` instance per zone.

---

## 7. Error Handling

### 7.1 mDNS Errors

| Error | Cause | Recovery |
|-------|-------|----------|
| Name conflict | Another device using same instance name | Append suffix (-2, -3, etc.) |
| Network unavailable | No link-local address | Retry every 5 seconds |
| mDNS responder failure | System mDNS crashed | Restart mDNS responder |

### 7.2 QR Code Errors

| Error | Cause | User Action |
|-------|-------|-------------|
| Invalid prefix | Not a MASH QR code | Scan correct QR code |
| Parse error | Malformed content | Report to manufacturer |
| Wrong version | Incompatible protocol | Update app or device |
| Discriminator not found | No device with D value | Ensure device in commissioning mode |

### 7.3 Discovery Timeouts

| Operation | Timeout | Action on Timeout |
|-----------|---------|-------------------|
| mDNS browse | 10 seconds | Report "No devices found" |
| Discriminator match | 30 seconds | Report "Device not found" |
| Commission attempt | 60 seconds | Report "Commissioning timeout" |

---

## 8. Privacy Considerations

### 8.1 Pre-Commissioning

**Minimal information exposure:**
- Discriminator: Essentially random, reveals nothing
- Vendor/Product: Type of device (acceptable for discovery)
- No personal information in mDNS records

### 8.2 Operational

**Information in `_mash._tcp`:**
- Zone ID: Partial fingerprint of Zone CA (reveals zone membership)
- Device ID: Unique but not personally identifiable
- Multiple instances reveal multi-zone membership

**Mitigation options:**
- Use randomized device ID (not serial number)
- Zone ID is truncated (8 chars) - reduces fingerprinting

### 8.3 Commissioner Discovery

**Information in `_mashd._udp`:**
- Zone name: User-chosen name (may reveal personal info)
- Device count: Reveals number of devices in zone

**Mitigation options:**
- Use generic zone names ("Zone 1" instead of "John's Home")
- Omit DC from TXT records (optional field)

### 8.4 QR Code Security

**Protect the QR code:**
- Contains setup code (equivalent to password)
- Should not be photographed or shared
- Physical access = commissioning capability

---

## 9. PICS Items

```
# Service types
MASH.S.DISC.SVC_COMMISSION=_mashc._udp  # Commissionable discovery service
MASH.S.DISC.SVC_OPERATIONAL=_mash._tcp  # Operational discovery service
MASH.S.DISC.SVC_COMMISSIONER=_mashd._udp # Commissioner discovery service (controllers only)

# General mDNS
MASH.S.DISC.INSTANCE_MAX_LEN=63         # Maximum instance name length
MASH.S.DISC.TXT_MAX_LEN=400             # Maximum TXT record size

# Commissionable (_mashc._udp)
MASH.S.DISC.COMM_TXT_D=1                # Discriminator in TXT
MASH.S.DISC.COMM_TXT_VP=1               # Vendor:Product in TXT
MASH.S.DISC.COMM_TXT_DT=1               # Device type in TXT (optional)
MASH.S.DISC.COMM_TXT_DN=1               # Device name in TXT (optional)

# Operational (_mash._tcp)
MASH.S.DISC.OPER_TXT_ZI=1               # Zone ID in TXT
MASH.S.DISC.OPER_TXT_DI=1               # Device ID in TXT
MASH.S.DISC.OPER_TXT_VP=1               # Vendor:Product in TXT
MASH.S.DISC.OPER_TXT_FW=1               # Firmware version in TXT (optional)
MASH.S.DISC.OPER_TXT_FM=1               # Feature map in TXT (optional)

# Commissioner (_mashd._udp) - controllers only
MASH.C.DISC.COMMR_TXT_ZN=1              # Zone name in TXT
MASH.C.DISC.COMMR_TXT_ZI=1              # Zone ID in TXT
MASH.C.DISC.COMMR_TXT_VP=1              # Vendor:Product in TXT
MASH.C.DISC.COMMR_TXT_DN=1              # Controller name in TXT (optional)
MASH.C.DISC.COMMR_TXT_DC=1              # Device count in TXT (optional)

# QR code
MASH.S.DISC.QR_VERSION=1                # QR code version
MASH.S.DISC.QR_ERROR_CORRECTION=M       # Error correction level
MASH.S.DISC.DISCRIMINATOR_BITS=12       # Discriminator size

# Timing
MASH.S.DISC.MDNS_UPDATE_DELAY=1         # Max delay for mDNS update (seconds)
MASH.S.DISC.BROWSE_TIMEOUT=10           # Browse timeout (seconds)
MASH.S.DISC.COMMISSION_WINDOW=120       # Commissioning window timeout (seconds)
```

---

## 10. Test Cases

### TC-MASHC-*: Commissionable Discovery (`_mashc._udp`)

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-MASHC-1 | Register on button press | Press commissioning button | `_mashc._udp` MASH-D instance appears |
| TC-MASHC-2 | Deregister on timeout | Wait 120s in commissioning | `_mashc._udp` instance removed |
| TC-MASHC-3 | Deregister on success | Complete commissioning | `_mashc._udp` instance removed |
| TC-MASHC-4 | TXT record format | Enter commissioning | D, VP fields present |
| TC-MASHC-5 | Instance conflict | Two devices same D | Suffix added (-2) |
| TC-MASHC-6 | Already operational | Device in zone, press button | `_mashc._udp` added, `_mash._tcp` kept |

### TC-MASHO-*: Operational Discovery (`_mash._tcp`)

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-MASHO-1 | Register on commission | Complete commissioning | `_mash._tcp` zone-device instance appears |
| TC-MASHO-2 | Instance name format | Inspect instance | `<zone-id>-<device-id>` format |
| TC-MASHO-3 | TXT record format | Inspect TXT | ZI, DI, VP fields present |
| TC-MASHO-4 | Multi-zone instances | Device in 2 zones | Two `_mash._tcp` instances |
| TC-MASHO-5 | Zone removed | Remove from one zone | That instance removed, other kept |
| TC-MASHO-6 | All zones removed | Remove from all zones | No `_mash._tcp` instances |

### TC-MASHD-*: Commissioner Discovery (`_mashd._udp`)

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-MASHD-1 | Register on zone create | Create zone | `_mashd._udp` zone-name instance appears |
| TC-MASHD-2 | TXT record format | Inspect TXT | ZN, ZI, VP fields present |
| TC-MASHD-3 | Multiple zones | Controller has 2 zones | Two `_mashd._udp` instances |
| TC-MASHD-4 | Device count update | Add device to zone | DC field updated |
| TC-MASHD-5 | Deregister on zone delete | Delete zone | That instance removed |
| TC-MASHD-6 | Device browses commissioners | Device enters find mode | Discovers `_mashd._udp` instances |

### TC-QR-*: QR Code Parsing

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-QR-1 | Valid QR | `MASH:1:1234:12345678:0x1234:0x5678` | Parse success |
| TC-QR-2 | Leading zeros setupcode | `MASH:1:0:00000001:0x0:0x0` | Parse success, setupcode="00000001" |
| TC-QR-3 | Invalid prefix | `EEBUS:1:1234:...` | Error: Invalid prefix |
| TC-QR-4 | Short setupcode | `MASH:1:1234:1234:0x1234:0x5678` | Error: Invalid setup code |
| TC-QR-5 | Missing 0x prefix | `MASH:1:1234:12345678:1234:5678` | Error: Missing 0x prefix |
| TC-QR-6 | Discriminator overflow | `MASH:1:9999:12345678:0x1234:0x5678` | Error: Discriminator out of range |

### TC-DISC-*: Discriminator Handling

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-DISC-1 | Match found | QR D=1234, device D=1234 | Device discovered via `_mashc._udp` |
| TC-DISC-2 | No match | QR D=1234, no device D=1234 | Timeout, error |
| TC-DISC-3 | Multiple match | Two devices D=1234 | Both discovered |
| TC-DISC-4 | Collision resolve | Wrong device first | PASE fails, retry finds correct |

### TC-DSTATE-*: Discovery State Transitions

| ID | Description | Initial State | Action | Expected |
|----|-------------|---------------|--------|----------|
| TC-DSTATE-1 | Enter comm mode | UNCOMMISSIONED | Button press | `_mashc._udp` registered |
| TC-DSTATE-2 | Comm timeout | COMMISSIONING_OPEN | Wait 120s | `_mashc._udp` deregistered |
| TC-DSTATE-3 | Comm success | COMMISSIONING_OPEN | Complete pairing | `_mashc._udp` removed, `_mash._tcp` added |
| TC-DSTATE-4 | Add zone (operational) | OPERATIONAL | Button press | `_mashc._udp` added, `_mash._tcp` kept |
| TC-DSTATE-5 | Second zone added | OPERATIONAL+COMM | Complete | New `_mash._tcp` instance added |
| TC-DSTATE-6 | Remove all zones | OPERATIONAL | RemoveZone (last) | All `_mash._tcp` removed |

### TC-BROWSE-*: Service Browsing

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-BROWSE-1 | Browse commissionable | Multiple commissioning devices | All `_mashc._udp` found |
| TC-BROWSE-2 | Browse operational | Multiple operational devices | All `_mash._tcp` found |
| TC-BROWSE-3 | Browse commissioners | Multiple controllers | All `_mashd._udp` found |
| TC-BROWSE-4 | Filter by discriminator | QR scan D=1234 | Only MASH-1234 in `_mashc._udp` |
| TC-BROWSE-5 | Filter by zone | Zone ID A1B2C3D4 | Only A1B2C3D4-* in `_mash._tcp` |
| TC-BROWSE-6 | Browse timeout | No services | Error after 10s |

---

## 11. Implementation Notes

### 11.1 mDNS Libraries

| Platform | Library | Notes |
|----------|---------|-------|
| Linux | Avahi | Default on most distros |
| macOS | mDNSResponder | Built-in |
| Windows | Bonjour SDK | From Apple |
| Embedded | lwIP mDNS | Lightweight |
| Go | github.com/grandcat/zeroconf | Cross-platform |
| Python | zeroconf | Pure Python |

### 11.2 QR Code Libraries

| Platform | Library | Notes |
|----------|---------|-------|
| Go | github.com/skip2/go-qrcode | Generation |
| Python | qrcode | Generation |
| JavaScript | qrcode.js | Browser generation |
| Scanning | Platform camera APIs | OS-specific |

### 11.3 Constrained Device Considerations

- Pre-compute QR code at manufacturing (save at runtime)
- Use minimal TXT records (omit optional fields)
- Share mDNS state with main application (single responder)
