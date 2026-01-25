# Discovery Behavior

> Precise specification of mDNS/DNS-SD and QR code discovery

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

MASH discovery has two phases:

1. **Pre-commissioning:** mDNS advertising + QR code for pairing
2. **Post-commissioning:** mDNS for operational discovery + capability introspection

**Key principles:**
- DNS-SD compliant (RFC 6763)
- Minimal TXT record size for constrained devices
- QR code contains all commissioning information
- Discovery state changes with device mode

---

## 2. mDNS Service Type

### 2.1 Service Registration

```
Service Type:  _mash._tcp
Domain:        local
```

### 2.2 Instance Naming Rules

| Mode | Instance Name Format | Example |
|------|---------------------|---------|
| Pre-commissioning | `MASH-<discriminator>` | `MASH-1234` |
| Operational | `<device-id>` | `PEN12345-EVSE001` |

**Naming constraints:**
- Maximum length: 63 characters (DNS label limit)
- Allowed characters: A-Z, a-z, 0-9, hyphen (-)
- Must not start or end with hyphen
- Case-insensitive comparison

**Device ID format:**
```
<vendor-prefix><serial>
```
Where:
- `vendor-prefix`: Up to 10 alphanumeric characters
- `serial`: Up to 20 alphanumeric characters + hyphens

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

### 3.2 Pre-Commissioning TXT Records

| Key | Type | Required | Max Len | Description |
|-----|------|----------|---------|-------------|
| `D` | uint16 | Yes | 4 | Discriminator (0-4095 as decimal) |
| `VP` | string | Yes | 11 | Vendor:Product ID (hex:hex, e.g., "1234:5678") |
| `CM` | uint8 | Yes | 1 | Commissioning mode (0=closed, 1=open) |
| `DT` | string | No | 20 | Device type label (e.g., "EVSE", "HeatPump") |
| `DN` | string | No | 32 | Device name (user-friendly) |

**Example:**
```
D=1234
VP=1234:5678
CM=1
DT=EVSE
DN=Garage Charger
```

**Total size:** ~80 bytes typical, 120 bytes maximum

### 3.3 Operational TXT Records

| Key | Type | Required | Max Len | Description |
|-----|------|----------|---------|-------------|
| `DI` | string | Yes | 31 | Device ID |
| `VP` | string | Yes | 11 | Vendor:Product ID |
| `FW` | string | No | 20 | Firmware version (semver) |
| `EP` | uint8 | No | 3 | Endpoint count |
| `FM` | string | No | 10 | Feature map (hex, e.g., "0x001B") |
| `ZC` | uint8 | No | 1 | Zone count (0-5) |

**Example:**
```
DI=PEN12345-EVSE001
VP=1234:5678
FW=1.2.3
EP=2
FM=0x001B
ZC=2
```

**Total size:** ~100 bytes typical, 150 bytes maximum

### 3.4 TXT Record Update Rules

**When to update mDNS:**

| Event | Action |
|-------|--------|
| Enter commissioning mode | Add CM=1, keep other records |
| Exit commissioning mode | Set CM=0 |
| Commissioning complete | Switch from pre-comm to operational format |
| Zone added/removed | Update ZC |
| Firmware update | Update FW |

**Update timing:**
- mDNS change within 1 second of state change
- mDNS goodbye (TTL=0) before removing old record
- mDNS announcement (3 times) for new record

### 3.5 Character Encoding

| Field | Encoding | Allowed Characters |
|-------|----------|--------------------|
| Keys | ASCII | a-z, A-Z, 0-9 |
| Discriminator | Decimal | 0-9 |
| Vendor/Product ID | Hex | 0-9, a-f, A-F, colon |
| Device ID | ASCII | A-Z, a-z, 0-9, hyphen |
| Device Name | UTF-8 | Any Unicode (avoid control chars) |
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
┌───────────────────────────────────────────────────────────────────┐
│                    Discovery State Machine                         │
├───────────────────────────────────────────────────────────────────┤
│                                                                    │
│   ┌─────────────────┐                                              │
│   │ UNREGISTERED    │  Device not advertising (powered off)        │
│   └────────┬────────┘                                              │
│            │ Power on                                              │
│            ▼                                                       │
│   ┌─────────────────┐                                              │
│   │ UNCOMMISSIONED  │  mDNS: MASH-D, CM=0                          │
│   │                 │  (Not open for pairing)                      │
│   └────────┬────────┘                                              │
│            │ Button press                                          │
│            ▼                                                       │
│   ┌─────────────────┐                                              │
│   │ COMMISSIONING   │  mDNS: MASH-D, CM=1                          │
│   │  OPEN (120s)    │  (Ready for pairing)                         │
│   └────────┬────────┘                                              │
│            │                                                       │
│       ┌────┴────┐                                                  │
│       │         │                                                  │
│       ▼         ▼                                                  │
│   Timeout    Success                                               │
│       │         │                                                  │
│       ▼         ▼                                                  │
│   ┌─────────────────┐   ┌─────────────────┐                        │
│   │ UNCOMMISSIONED  │   │   OPERATIONAL   │  mDNS: device-id       │
│   │   (CM=0)        │   │                 │  (Full TXT records)    │
│   └─────────────────┘   └─────────────────┘                        │
│                                                                    │
└───────────────────────────────────────────────────────────────────┘
```

### 6.2 Transition Rules

```
UNREGISTERED → UNCOMMISSIONED  : Power on (no zones)
UNREGISTERED → OPERATIONAL     : Power on (has zones)

UNCOMMISSIONED → COMMISSIONING_OPEN : Button press / API trigger
UNCOMMISSIONED → UNREGISTERED       : Power off

COMMISSIONING_OPEN → UNCOMMISSIONED : 120s timeout (no zones)
COMMISSIONING_OPEN → OPERATIONAL    : Commissioning success
COMMISSIONING_OPEN → UNREGISTERED   : Power off

OPERATIONAL → COMMISSIONING_OPEN    : Button press (add zone)
OPERATIONAL → UNCOMMISSIONED        : All zones removed
OPERATIONAL → UNREGISTERED          : Power off
```

### 6.3 mDNS Updates per State

| Transition | mDNS Action |
|------------|-------------|
| → UNCOMMISSIONED | Register MASH-D with CM=0 |
| → COMMISSIONING_OPEN | Update CM=1 |
| COMMISSIONING_OPEN timeout | Update CM=0 |
| → OPERATIONAL | Deregister MASH-D, register device-id |
| → UNREGISTERED | Deregister all |

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

**Information in mDNS:**
- Device ID: Unique but not personally identifiable
- Zone count: Reveals number of controllers (minor privacy concern)

**Mitigation options:**
- Omit ZC from TXT records (optional field)
- Use randomized device ID (not serial number)

### 8.3 QR Code Security

**Protect the QR code:**
- Contains setup code (equivalent to password)
- Should not be photographed or shared
- Physical access = commissioning capability

---

## 9. PICS Items

```
# mDNS configuration
MASH.S.DISC.SERVICE_TYPE=_mash._tcp     # Service type
MASH.S.DISC.INSTANCE_MAX_LEN=63         # Maximum instance name length
MASH.S.DISC.TXT_MAX_LEN=400             # Maximum TXT record size

# Pre-commissioning
MASH.S.DISC.PRECOMM_TXT_D=1             # Discriminator in TXT
MASH.S.DISC.PRECOMM_TXT_VP=1            # Vendor:Product in TXT
MASH.S.DISC.PRECOMM_TXT_CM=1            # Commissioning mode in TXT
MASH.S.DISC.PRECOMM_TXT_DT=1            # Device type in TXT (optional)

# Operational
MASH.S.DISC.OPER_TXT_DI=1               # Device ID in TXT
MASH.S.DISC.OPER_TXT_FW=1               # Firmware version in TXT
MASH.S.DISC.OPER_TXT_FM=1               # Feature map in TXT

# QR code
MASH.S.DISC.QR_VERSION=1                # QR code version
MASH.S.DISC.QR_ERROR_CORRECTION=M       # Error correction level
MASH.S.DISC.DISCRIMINATOR_BITS=12       # Discriminator size

# Timing
MASH.S.DISC.MDNS_UPDATE_DELAY=1         # Max delay for mDNS update (seconds)
MASH.S.DISC.BROWSE_TIMEOUT=10           # Browse timeout (seconds)
```

---

## 10. Test Cases

### TC-MDNS-*: mDNS Registration

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-MDNS-1 | Pre-comm registration | Power on device | MASH-D instance registered |
| TC-MDNS-2 | CM update | Enter commissioning mode | CM=1 in TXT |
| TC-MDNS-3 | Operational switch | Complete commissioning | New instance name |
| TC-MDNS-4 | Instance conflict | Two devices same D | Suffix added |
| TC-MDNS-5 | TXT size limit | All fields present | Under 400 bytes |

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
| TC-DISC-1 | Match found | QR D=1234, device D=1234 | Device discovered |
| TC-DISC-2 | No match | QR D=1234, no device D=1234 | Timeout, error |
| TC-DISC-3 | Multiple match | Two devices D=1234 | Both discovered |
| TC-DISC-4 | Collision resolve | Wrong device first | Retry finds correct |

### TC-STATE-*: Discovery State Transitions

| ID | Description | Initial State | Action | Expected State |
|----|-------------|---------------|--------|----------------|
| TC-STATE-1 | Enter comm mode | UNCOMMISSIONED | Button press | COMMISSIONING_OPEN |
| TC-STATE-2 | Comm timeout | COMMISSIONING_OPEN | Wait 120s | UNCOMMISSIONED |
| TC-STATE-3 | Comm success | COMMISSIONING_OPEN | Complete pairing | OPERATIONAL |
| TC-STATE-4 | Add zone | OPERATIONAL | Button press | COMMISSIONING_OPEN |
| TC-STATE-5 | Remove all zones | OPERATIONAL | RemoveZone (last) | UNCOMMISSIONED |

### TC-BROWSE-*: Service Browsing

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-BROWSE-1 | Find all devices | Multiple devices | All discovered |
| TC-BROWSE-2 | Filter by D | QR scan with D=1234 | Only D=1234 returned |
| TC-BROWSE-3 | Browse timeout | No devices | Error after 10s |
| TC-BROWSE-4 | Device appears | Start browse, then power on | Device found |

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
