# Commissioning (PASE) Behavior

> Precise specification of device commissioning using SPAKE2+

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

Commissioning is the process by which a controller joins a device to its zone. MASH uses SPAKE2+ (Password-Authenticated Secure Password Authenticated Key Exchange) following Matter's proven approach.

**Key principles:**
- Setup code provides shared secret (from QR code or label)
- SPAKE2+ establishes encrypted session without revealing password
- Controller issues operational certificate after successful PASE
- Device can commission to multiple zones (up to 5)

---

## 2. SPAKE2+ Cryptographic Parameters

### 2.1 Algorithm Specification

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Group | P-256 (secp256r1) | Matches TLS cipher preference, widely supported |
| Hash | SHA-256 | Standard, efficient, secure |
| KDF | HKDF-SHA256 | RFC 5869 compliant |
| MAC | HMAC-SHA256 | For key confirmation |

### 2.2 SPAKE2+ Constants

Standard P-256 SPAKE2+ constants (from RFC 9382):

```
M = (base point of P-256) * H("MASH SPAKE2+ M v1")
N = (base point of P-256) * H("MASH SPAKE2+ N v1")
```

Where H is SHA-256 with hash-to-curve per RFC 9380.

### 2.3 Setup Code Processing

**Format:** 8 decimal digits (00000000-99999999)

**Password derivation:**
```
password_bytes = UTF8(setup_code)  // e.g., "12345678" → 8 bytes
w0 = HKDF-Expand(
    HKDF-Extract(salt="", password_bytes),
    info="MASH SPAKE2+ w0",
    length=32
)
w1 = HKDF-Expand(
    HKDF-Extract(salt="", password_bytes),
    info="MASH SPAKE2+ w1",
    length=32
)
```

### 2.4 Session Key Derivation

After SPAKE2+ exchange, both parties derive session keys:

```
K_main = HKDF-Expand(
    HKDF-Extract(salt="", K_shared),
    info="MASH PASE Session Key",
    length=32
)

K_confirm = HKDF-Expand(
    HKDF-Extract(salt="", K_shared),
    info="MASH PASE Confirmation",
    length=32
)

K_encrypt = HKDF-Expand(
    HKDF-Extract(salt="", K_main),
    info="MASH PASE Encryption",
    length=32
)
```

---

## 3. Commissioning States

### 3.1 State Machine

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Commissioning State Machine                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐                                                       │
│   │    IDLE      │  Device not in commissioning mode                     │
│   └──────┬───────┘                                                       │
│          │ Button press / QR display                                     │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ ADVERTISING  │  mDNS with CM=1, waiting for connection               │
│   │  (120s max)  │                                                       │
│   └──────┬───────┘                                                       │
│          │ TLS connection received                                       │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ PASE_WAIT_X  │  Device waiting for controller's X value              │
│   │  (30s max)   │                                                       │
│   └──────┬───────┘                                                       │
│          │ X received                                                    │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ PASE_COMPUTE │  Device computing Y and verifier                      │
│   │  (10s max)   │                                                       │
│   └──────┬───────┘                                                       │
│          │ Computation complete                                          │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ PASE_WAIT_V  │  Device waiting for controller's verifier             │
│   │  (30s max)   │                                                       │
│   └──────┬───────┘                                                       │
│          │ Verifier validated                                            │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ PASE_VERIFIED│  SPAKE2+ complete, session encrypted                  │
│   └──────┬───────┘                                                       │
│          │ CSR request received                                          │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ CSR_GENERATE │  Device generating key pair and CSR                   │
│   │  (10s max)   │                                                       │
│   └──────┬───────┘                                                       │
│          │ CSR sent                                                      │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ CERT_WAIT    │  Waiting for operational certificate                  │
│   │  (30s max)   │                                                       │
│   └──────┬───────┘                                                       │
│          │ Certificate received                                          │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │ CERT_INSTALL │  Installing certificate to zone slot                  │
│   │  (5s max)    │                                                       │
│   └──────┬───────┘                                                       │
│          │ Installation complete                                         │
│          ▼                                                               │
│   ┌──────────────┐                                                       │
│   │  COMPLETE    │  Commissioning successful, transition to OPERATIONAL  │
│   └──────────────┘                                                       │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 State Definitions

| State | Description | Timeout | Exit on Timeout |
|-------|-------------|---------|-----------------|
| IDLE | Not commissioning | None | N/A |
| ADVERTISING | mDNS with CM=1 | 120s | → IDLE |
| PASE_WAIT_X | Waiting for X value | 30s | → ADVERTISING |
| PASE_COMPUTE | Computing Y | 10s | → ADVERTISING |
| PASE_WAIT_V | Waiting for verifier | 30s | → ADVERTISING |
| PASE_VERIFIED | Session established | None | N/A |
| CSR_GENERATE | Key pair generation | 10s | → ADVERTISING |
| CERT_WAIT | Waiting for certificate | 30s | → ADVERTISING |
| CERT_INSTALL | Installing certificate | 5s | → ADVERTISING |
| COMPLETE | Success | None | → OPERATIONAL |

### 3.3 Transition Rules

```
IDLE → ADVERTISING         : Physical action (button, display interaction)
IDLE → IDLE                : API commissioning request (not supported yet)

ADVERTISING → IDLE         : 120s timeout
ADVERTISING → ADVERTISING  : Connection failed, retry allowed
ADVERTISING → PASE_WAIT_X  : TLS connection established (no zone cert)

PASE_WAIT_X → ADVERTISING  : 30s timeout, verification failure
PASE_WAIT_X → PASE_COMPUTE : Valid X received

PASE_COMPUTE → ADVERTISING : Computation error
PASE_COMPUTE → PASE_WAIT_V : Y and verifier sent

PASE_WAIT_V → ADVERTISING  : 30s timeout, verification failure
PASE_WAIT_V → PASE_VERIFIED: Verifier validated

PASE_VERIFIED → CSR_GENERATE: CSR request received

CSR_GENERATE → ADVERTISING : Key generation error
CSR_GENERATE → CERT_WAIT   : CSR sent

CERT_WAIT → ADVERTISING    : 30s timeout
CERT_WAIT → CERT_INSTALL   : Certificate received

CERT_INSTALL → ADVERTISING : Installation error, storage full
CERT_INSTALL → COMPLETE    : Certificate installed

COMPLETE → OPERATIONAL     : (immediate transition)
```

---

## 4. Message Flow

### 4.1 Complete Commissioning Sequence

```
Controller                                    Device
    │                                            │
    │◄─────── mDNS: _mash._tcp (CM=1) ───────────┤  ADVERTISING
    │                                            │
    │─────── TCP Connect ───────────────────────►│
    │                                            │
    │─────── TLS Handshake (no client cert) ────►│
    │◄────── TLS Handshake (self-signed) ────────┤
    │                                            │
    │─────── PASE_X { X, context } ─────────────►│  PASE_WAIT_X
    │                                            │  PASE_COMPUTE
    │◄────── PASE_Y { Y, verifier_device } ──────┤
    │                                            │  PASE_WAIT_V
    │─────── PASE_VERIFY { verifier_ctrl } ─────►│
    │◄────── PASE_CONFIRM { status } ────────────┤  PASE_VERIFIED
    │                                            │
    │─────── ATTESTATION_REQ {} ────────────────►│
    │◄────── ATTESTATION_RSP { device_cert } ────┤  (optional attestation)
    │                                            │
    │─────── CSR_REQ { csr_nonce } ─────────────►│  CSR_GENERATE
    │◄────── CSR_RSP { csr } ────────────────────┤
    │                                            │  CERT_WAIT
    │─────── CERT_INSTALL { op_cert, ca_cert } ─►│  CERT_INSTALL
    │◄────── CERT_ACK { status } ────────────────┤  COMPLETE
    │                                            │
    │◄─────── Connection transitions ────────────┤  → OPERATIONAL
    │         to OPERATIONAL mode                │
```

### 4.2 PASE Messages

**PASE_X (Controller → Device):**
```cbor
{
  1: "pase_x",           // type
  2: <bytes[32]>,        // X value (P-256 point, compressed)
  3: <bytes[32]>         // context (session binding data)
}
```

**PASE_Y (Device → Controller):**
```cbor
{
  1: "pase_y",           // type
  2: <bytes[32]>,        // Y value (P-256 point, compressed)
  3: <bytes[32]>         // verifier_device (HMAC over transcript)
}
```

**PASE_VERIFY (Controller → Device):**
```cbor
{
  1: "pase_verify",      // type
  2: <bytes[32]>         // verifier_controller (HMAC over transcript)
}
```

**PASE_CONFIRM (Device → Controller):**
```cbor
{
  1: "pase_confirm",     // type
  2: <uint8>             // status (0 = success)
}
```

### 4.3 Certificate Messages

**ATTESTATION_REQ (Controller → Device):**
```cbor
{
  1: "attestation_req"   // type
}
```

**ATTESTATION_RSP (Device → Controller):**
```cbor
{
  1: "attestation_rsp",  // type
  2: <bytes[]>,          // device_cert (DER-encoded X.509)
  3: <bytes[]>[]         // cert_chain (optional, DER-encoded)
}
```

**Attestation Handling:**

Attestation is OPTIONAL. It allows controller to verify device manufacturer identity.

| Scenario | Controller Behavior |
|----------|---------------------|
| Device has no attestation cert | ATTESTATION_RSP with empty fields; controller proceeds |
| Valid attestation chain | Log manufacturer info; proceed with commissioning |
| Invalid attestation chain | Log warning; proceed with commissioning (don't reject) |
| Attestation chain required (policy) | Reject if missing/invalid (controller-specific policy) |

**Validation algorithm:**
```python
def handle_attestation(response: AttestationResponse, policy: Policy) -> AttestationResult:
    """Handle device attestation response."""

    if not response.device_cert:
        # No attestation - device may be open-source/DIY
        if policy.require_attestation:
            return AttestationResult(
                valid=False,
                error="Attestation required but not provided"
            )
        return AttestationResult(valid=True, manufacturer=None)

    # Validate certificate chain
    try:
        chain = [response.device_cert] + (response.cert_chain or [])
        verify_chain(chain, trusted_roots=policy.trusted_manufacturer_cas)

        # Extract manufacturer info from cert
        manufacturer = extract_manufacturer(response.device_cert)

        return AttestationResult(
            valid=True,
            manufacturer=manufacturer,
            cert_fingerprint=sha256(response.device_cert)
        )
    except ChainValidationError as e:
        # Log but don't necessarily reject
        log.warning(f"Attestation chain invalid: {e}")

        if policy.require_valid_attestation:
            return AttestationResult(valid=False, error=str(e))

        # Proceed anyway - attestation is informational
        return AttestationResult(valid=True, manufacturer=None, warning=str(e))
```

**Default policy:** Accept devices without attestation. Log attestation info when present. This supports both commercial devices (with manufacturer certs) and open-source/DIY devices (without).

**CSR_REQ (Controller → Device):**
```cbor
{
  1: "csr_req",          // type
  2: <bytes[32]>         // csr_nonce (anti-replay)
}
```

**CSR_RSP (Device → Controller):**
```cbor
{
  1: "csr_rsp",          // type
  2: <bytes[]>           // csr (PKCS#10, DER-encoded)
}
```

**CERT_INSTALL (Controller → Device):**
```cbor
{
  1: "cert_install",     // type
  2: <bytes[]>,          // operational_cert (DER-encoded X.509)
  3: <bytes[]>,          // zone_ca_cert (DER-encoded X.509)
  4: <uint8>,            // zone_type (GRID_OPERATOR, HOME_MANAGER, etc.)
  5: <uint8>             // zone_priority (1-4)
}
```

**CERT_ACK (Device → Controller):**
```cbor
{
  1: "cert_ack",         // type
  2: <uint8>,            // status (0 = success)
  3: <string>            // error_message (if status != 0)
}
```

### 4.4 Operational Certificate Format

Controller creates the operational certificate from the device's CSR:

**Certificate Structure:**

```
Certificate:
    Version: 3 (0x2)
    Serial Number: <random 128-bit>
    Signature Algorithm: ecdsa-with-SHA256
    Issuer: <Zone CA Subject>
    Validity:
        Not Before: <current time - 5 minutes>  (clock skew allowance)
        Not After: <current time + 365 days>
    Subject:
        CN = <device-id>                        (16 hex chars, from CSR public key)
        O = <zone-name>                         (optional, user-friendly)
        OU = MASH Device
    Subject Public Key Info:
        Algorithm: id-ecPublicKey (P-256)
        Public Key: <from CSR>
    Extensions:
        Basic Constraints: critical
            CA: FALSE
        Key Usage: critical
            Digital Signature
            Key Encipherment
        Extended Key Usage:
            TLS Web Server Authentication (1.3.6.1.5.5.7.3.1)
            TLS Web Client Authentication (1.3.6.1.5.5.7.3.2)
        Subject Key Identifier:
            <SHA-1 of public key>
        Authority Key Identifier:
            keyIdentifier: <Zone CA SKI>
```

**Device ID derivation:**
```python
def derive_device_id(csr: PKCS10) -> str:
    """Derive device ID from CSR public key."""
    public_key_der = csr.public_key.to_der()  # SubjectPublicKeyInfo DER
    hash = SHA256(public_key_der)
    return hash[0:8].hex().upper()  # 16 hex chars (64 bits)
```

**Certificate generation algorithm:**
```python
def create_operational_cert(
    csr: PKCS10,
    zone_ca_cert: X509,
    zone_ca_key: PrivateKey,
    zone_name: str
) -> X509:
    """Create operational certificate from CSR."""

    # Derive device ID from CSR public key
    device_id = derive_device_id(csr)

    # Build certificate
    cert = X509Certificate()
    cert.version = 3
    cert.serial_number = random_bytes(16)
    cert.issuer = zone_ca_cert.subject
    cert.not_before = now() - timedelta(minutes=5)  # Clock skew
    cert.not_after = now() + timedelta(days=365)

    # Subject DN
    cert.subject = X509Name()
    cert.subject.CN = device_id
    cert.subject.O = zone_name  # Optional
    cert.subject.OU = "MASH Device"

    # Copy public key from CSR
    cert.public_key = csr.public_key

    # Extensions
    cert.add_extension(BasicConstraints(ca=False), critical=True)
    cert.add_extension(KeyUsage(
        digital_signature=True,
        key_encipherment=True
    ), critical=True)
    cert.add_extension(ExtendedKeyUsage([
        OID_SERVER_AUTH,
        OID_CLIENT_AUTH
    ]))
    cert.add_extension(SubjectKeyIdentifier(cert.public_key))
    cert.add_extension(AuthorityKeyIdentifier(zone_ca_cert))

    # Sign with Zone CA
    cert.sign(zone_ca_key, algorithm=SHA256)

    return cert
```

**Validity period:**
- Default: 365 days (1 year)
- Controller MAY use shorter validity for higher security
- Device MUST accept any validity up to 10 years
- See `zone-lifecycle.md` for renewal procedures

---

## 5. Timing Requirements

### 5.1 Commissioning Window

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Window duration | 120 seconds | User has time to scan QR |
| Window activation | Physical action | Prevents remote commissioning |
| Extension | None | Window does not extend on activity |
| Cooldown | 10 seconds | Before re-opening window |

### 5.2 Phase Timeouts

| Phase | Timeout | Cumulative Max |
|-------|---------|----------------|
| Advertising | 120s | 120s |
| PASE handshake | 30s | 150s |
| CSR generation | 10s | 160s |
| Certificate wait | 30s | 190s |
| Certificate install | 5s | 195s |

**Note:** Phases overlap with advertising. Total commissioning must complete within 120s window.

### 5.3 SPAKE2+ Computation Time

| Device Class | Expected Time | Maximum |
|--------------|---------------|---------|
| Constrained (ESP32) | 2-5 seconds | 10 seconds |
| Standard (RPi) | < 1 second | 5 seconds |
| Powerful (x86) | < 100 ms | 1 second |

---

## 6. Error Handling

### 6.1 PASE Verification Failure

**Cause:** Wrong setup code entered.

**Device behavior:**
1. Send PASE_CONFIRM with status = 1 (VERIFICATION_FAILED)
2. Close TLS connection
3. Return to ADVERTISING state
4. Accept new commissioning attempts

**Controller behavior:**
1. Receive PASE_CONFIRM with failure status
2. Report error to user: "Incorrect setup code"
3. Allow user to retry

**Retry policy:**
- No retry limit (user can keep trying)
- No backoff between attempts
- No lockout mechanism

### 6.2 Error Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | SUCCESS | Operation successful |
| 1 | VERIFICATION_FAILED | SPAKE2+ verification failed |
| 2 | TIMEOUT | Operation timed out |
| 3 | ZONE_FULL | Device has 5 zones, cannot add more |
| 4 | ALREADY_COMMISSIONED | Device already commissioned to this zone |
| 5 | INVALID_CERT | Certificate validation failed |
| 6 | STORAGE_ERROR | Cannot store certificate |
| 7 | KEY_GENERATION_ERROR | Cannot generate key pair |
| 8 | INVALID_FORMAT | Message format error |
| 9 | INTERNAL_ERROR | Device internal error |

### 6.3 Connection Loss During Commissioning

**If connection lost in any state:**
1. Cancel current commissioning attempt
2. Release any generated keys (if not installed)
3. Return to ADVERTISING state (if window still open)
4. Return to IDLE state (if window expired)

**Controller behavior:**
1. Detect connection loss
2. Report to user: "Connection lost during commissioning"
3. User can retry from start

### 6.4 Zone Capacity

When device already has 5 zones:
1. Device accepts TLS connection
2. Device sends PASE_CONFIRM with status = 3 (ZONE_FULL)
3. Connection closed
4. Controller reports: "Device has maximum zones"

**To add a 6th zone:**
1. Controller must remove existing zone first (RemoveZone command)
2. Then retry commissioning

---

## 7. Multi-Zone Commissioning

### 7.1 Zone Slots

Device maintains up to 5 zone slots:

```
zoneSlots[5] = {
  [0]: { zoneCA: <cert>, opCert: <cert>, zoneType: GRID_OPERATOR, priority: 1 },
  [1]: { zoneCA: <cert>, opCert: <cert>, zoneType: HOME_MANAGER, priority: 3 },
  [2]: null,  // empty slot
  [3]: null,  // empty slot
  [4]: null   // empty slot
}
```

### 7.2 Concurrent Commissioning

**Rule:** Only one commissioning session at a time.

**Behavior:**
- If device is in any commissioning state (PASE_*, CSR_*, CERT_*):
- New commissioning connection is rejected
- Existing session continues
- New controller receives: "Device busy commissioning"

**Queue behavior:** No queue. Rejected controllers must retry.

### 7.3 Existing Zones During Commissioning

**Rule:** Existing zone connections remain OPERATIONAL.

**Behavior:**
- Controller A (HOME_MANAGER) is connected and OPERATIONAL
- Controller B (GRID_OPERATOR) starts commissioning
- Controller A continues normal operation
- Controller B completes commissioning
- Controller B establishes OPERATIONAL connection
- Both controllers now connected

### 7.4 Same Zone Re-Commissioning

**Scenario:** Controller tries to commission device that's already in its zone.

**Behavior:**
1. Device checks if certificate issuer matches existing zone CA
2. If match: Return ALREADY_COMMISSIONED error
3. Controller should use existing operational connection

**To re-commission (e.g., after key compromise):**
1. Controller sends RemoveZone command (existing connection)
2. Device removes zone slot
3. Controller initiates new commissioning

---

## 8. Delegated Commissioning (Admin Flow)

### 8.1 Admin Authorization

Apps act as admins, not zone owners. Admin flow:

```
User              App              EMS (Zone Owner)        Device
 │                 │                     │                    │
 │─ "Add admin" ──►│                     │                    │
 │                 │── Request admin ───►│                    │
 │                 │◄── Temp QR (5min) ──┤                    │
 │◄── Display QR ──┤                     │                    │
 │── Scan QR ─────►│── SPAKE2+ ─────────►│                    │
 │                 │◄── Admin token ─────┤                    │
```

### 8.2 Admin Token Format

```cbor
{
  1: <bytes[32]>,        // token_id
  2: <uint64>,           // issued_at (Unix timestamp)
  3: <uint64>,           // expires_at (Unix timestamp)
  4: <bytes[]>,          // zone_ca_fingerprint (SHA-256)
  5: <uint8>,            // permissions (bitmap)
  6: <bytes[64]>         // signature (ECDSA over fields 1-5)
}
```

**Permissions bitmap:**

| Bit | Permission |
|-----|------------|
| 0 | Can commission devices |
| 1 | Can remove devices |
| 2 | Can view device list |
| 3-7 | Reserved |

### 8.3 Admin Commissioning Flow

```
App (Admin)                              EMS (Owner)              Device
    │                                        │                       │
    │───── PASE with setup code ────────────────────────────────────►│
    │◄───────────────────────────────────────────────────── CSR ─────┤
    │                                        │                       │
    │─── Forward CSR + admin token ─────────►│                       │
    │◄── Signed operational cert ────────────┤                       │
    │                                        │                       │
    │─── CERT_INSTALL ──────────────────────────────────────────────►│
    │◄──────────────────────────────────────────────────── ACK ──────┤
```

### 8.4 Admin Token Validation

Device validates admin token during CERT_INSTALL:

1. Verify signature using zone CA public key
2. Check expires_at > current time
3. Check permissions include "can commission"
4. Check zone_ca_fingerprint matches received zone CA cert

**If validation fails:**
- Return CERT_ACK with status = 5 (INVALID_CERT)
- Error message: "Invalid admin token"

---

## 9. Certificate Profile

### 9.1 Operational Certificate

| Field | Value |
|-------|-------|
| Version | v3 |
| Serial | Random 128-bit |
| Issuer | Zone CA DN |
| Subject | Device ID + Zone ID |
| Validity | 1 year |
| Key Usage | Digital Signature, Key Encipherment |
| Extended Key Usage | Client Auth, Server Auth |
| Subject Alt Name | URI: mash://device/{device_id} |

### 9.2 Zone CA Certificate

| Field | Value |
|-------|-------|
| Version | v3 |
| Serial | Random 128-bit |
| Issuer | Self-signed (Zone CA) |
| Subject | Zone name + Zone ID |
| Validity | 10 years |
| Key Usage | Certificate Sign, CRL Sign |
| Basic Constraints | CA:TRUE, pathlen:1 |

### 9.3 Device Attestation Certificate (Optional)

| Field | Value |
|-------|-------|
| Version | v3 |
| Issuer | Manufacturer CA |
| Subject | Vendor ID + Product ID + Device ID |
| Validity | 20 years |
| Key Usage | Digital Signature |

---

## 10. PICS Items

```
# SPAKE2+ parameters
MASH.S.PASE.GROUP=P256               # SPAKE2+ group
MASH.S.PASE.HASH=SHA256              # Hash algorithm
MASH.S.PASE.KDF=HKDF_SHA256          # Key derivation

# Timing
MASH.S.PASE.WINDOW_DURATION=120      # Commissioning window (seconds)
MASH.S.PASE.PASE_TIMEOUT=30          # PASE handshake timeout
MASH.S.PASE.CSR_TIMEOUT=10           # CSR generation timeout
MASH.S.PASE.CERT_TIMEOUT=30          # Certificate wait timeout

# Capacity
MASH.S.PASE.MAX_ZONES=5              # Maximum zone slots
MASH.S.PASE.MAX_CONCURRENT=1         # Concurrent commissioning sessions

# Features
MASH.S.PASE.ATTESTATION=1            # Device attestation supported
MASH.S.PASE.ADMIN_DELEGATION=1       # Admin delegation supported

# Error behavior
MASH.S.PASE.RETRY_LIMIT=0            # 0 = unlimited retries
MASH.S.PASE.RETRY_BACKOFF=0          # No backoff between retries
```

---

## 11. Test Cases

### TC-PASE-*: SPAKE2+ Protocol

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-PASE-1 | Successful PASE | Correct setup code | PASE_VERIFIED state |
| TC-PASE-2 | Wrong setup code | Incorrect setup code | VERIFICATION_FAILED |
| TC-PASE-3 | PASE timeout | No X sent | Return to ADVERTISING after 30s |
| TC-PASE-4 | Invalid X value | Malformed X | Close connection |
| TC-PASE-5 | Replay X | Same X twice | Fresh Y each time |

### TC-COMM-*: Commissioning Flow

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-COMM-1 | Complete commissioning | Full flow | OPERATIONAL state |
| TC-COMM-2 | Window timeout | Wait 120s | Return to IDLE |
| TC-COMM-3 | CSR timeout | No CSR_REQ | Return to ADVERTISING |
| TC-COMM-4 | Invalid certificate | Malformed cert | INVALID_CERT error |
| TC-COMM-5 | Storage full | Max certs stored | STORAGE_ERROR |

### TC-ZONE-*: Multi-Zone

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ZONE-1 | Second zone | Commission after first | Both zones active |
| TC-ZONE-2 | Fifth zone | Commission 5th | Success |
| TC-ZONE-3 | Sixth zone | Commission 6th | ZONE_FULL error |
| TC-ZONE-4 | Same zone | Re-commission | ALREADY_COMMISSIONED |
| TC-ZONE-5 | Concurrent | Two controllers | Second rejected |

### TC-ADMIN-*: Delegated Commissioning

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ADMIN-1 | Valid admin token | Admin commissions | Success |
| TC-ADMIN-2 | Expired token | Expired admin token | INVALID_CERT error |
| TC-ADMIN-3 | Invalid signature | Tampered token | INVALID_CERT error |
| TC-ADMIN-4 | Wrong permissions | No commission perm | INVALID_CERT error |

### TC-CERT-*: Certificate Handling

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-CERT-1 | Valid chain | Full cert chain | Success |
| TC-CERT-2 | Self-signed device | No attestation | Success (optional) |
| TC-CERT-3 | Invalid chain | Broken chain | INVALID_CERT error |
| TC-CERT-4 | Expired cert | Expired op cert | INVALID_CERT error |

---

## 12. Security Considerations

### 12.1 Setup Code Protection

- **Display security:** QR should only be shown during commissioning window
- **Physical security:** Label should not be visible during normal operation
- **Entropy:** 27 bits sufficient for 120-second window (online attack only)
- **Brute force:** ~100 million combinations, no lockout

### 12.2 SPAKE2+ Security

- **Offline attack resistance:** w0/w1 derivation prevents offline dictionary attack
- **Forward secrecy:** Ephemeral X/Y values, no persistent session keys
- **MITM resistance:** Verifier exchange authenticates both parties
- **Replay resistance:** Random X/Y per session

### 12.3 Certificate Security

- **Zone isolation:** Each zone has independent CA
- **Rotation:** 1-year validity with 30-day renewal window
- **Revocation:** Immediate via RemoveZone command
- **Path validation:** Maximum path length = 2 (Zone CA → Op Cert)
