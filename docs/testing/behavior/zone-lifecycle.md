# Zone Lifecycle Behavior

> Precise specification of zone creation, device management, and certificate lifecycle

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

A **zone** is a trust domain containing one owner (controller) and multiple member devices. This document specifies the complete lifecycle of zones and their members.

**Key concepts:**
- **Zone Owner:** Controller that created the zone (holds Zone CA private key)
- **Zone Admin:** Delegated authority to commission devices (e.g., phone app)
- **Zone Member:** Device commissioned to the zone
- **Zone CA:** Root certificate authority for the zone

---

## 2. Zone Creation

### 2.1 Zone CA Generation

When a zone owner (EMS, SMGW) creates a new zone:

**Step 1: Generate Zone CA Key Pair**
```
Algorithm: ECDSA with P-256 (secp256r1)
Key size: 256 bits
Storage: Secure element or encrypted file (PKCS#8)
```

**Step 2: Create Zone CA Certificate**
```
Subject: CN=<zone-name>, O=<owner-name>, OU=MASH Zone CA
Serial: Random 128-bit
Validity: 99 years (effectively no expiry)
Basic Constraints: CA:TRUE, pathlen:1
Key Usage: Certificate Sign, CRL Sign
```

**Note on Zone CA validity:** Zone CA certificates use very long validity (99 years) because:
- Zone ID is derived from Zone CA fingerprint - if CA changes, zone identity changes
- Expiration doesn't add security for root CAs (compromise requires regeneration anyway)
- Follows Matter model where Fabric Root CAs are effectively permanent
- X.509 requires a notAfter date, so we use maximum practical value

**Step 3: Generate Zone ID**
```
Zone ID = SHA-256(Zone CA Certificate DER)[0:8]  // First 8 bytes = 64 bits
Format: 16 hex characters (e.g., "a1b2c3d4e5f6a7b8")
```

**Step 4: Store Zone State**
```
zoneState = {
  zoneId: <hex string>,
  zoneName: <user-friendly name>,
  zoneType: GRID | LOCAL,
  zonePriority: 1-2,
  caPrivateKey: <encrypted PKCS#8>,
  caCertificate: <DER-encoded X.509>,
  createdAt: <timestamp>,
  members: [],       // List of commissioned devices
  admins: []         // List of authorized admins
}
```

### 2.2 Zone Types and Priorities

| Zone Type | Priority | Typical Owner | Use Case |
|-----------|----------|---------------|----------|
| GRID | 1 | SMGW, DSO, utility | Grid regulation, 14a compliance, external authority |
| LOCAL | 2 | Home EMS, building EMS | Local energy management, optimization |

### 2.3 Zone Metadata

Zone owner maintains:

```cbor
{
  "zoneId": "a1b2c3d4e5f6a7b8",
  "zoneName": "Home Energy",
  "zoneType": 2,                    // LOCAL
  "zonePriority": 2,
  "caCertFingerprint": "<sha256>",
  "createdAt": 1706140800,
  "members": [
    {
      "deviceId": "PEN12345-EVSE001",
      "opCertSerial": "abc123...",
      "commissionedAt": 1706140900,
      "lastSeen": 1706227300,
      "endpoints": [0, 1]
    }
  ],
  "admins": [
    {
      "adminId": "phone-app-001",
      "tokenId": "token-xyz",
      "grantedAt": 1706140850,
      "expiresAt": 1737676850,
      "permissions": 0x03          // commission + remove
    }
  ]
}
```

---

## 3. Adding Devices to Zone

### 3.1 Controller Workflow (Direct Commissioning)

```
┌─────────────────────────────────────────────────────────────────────────┐
│              Controller Commissioning Workflow                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌────────────────┐                                                     │
│   │  SCAN_QR_CODE  │  User scans device QR code                          │
│   └───────┬────────┘                                                     │
│           │ Parse MASH:<version>:<D>:<setupcode>                         │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │ BROWSE_MDNS    │  Look for MASH-<D>._mash-comm._tcp                      │
│   │   (10s max)    │                                                     │
│   └───────┬────────┘                                                     │
│           │ Device found with matching discriminator                     │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │  CONNECT       │  TCP + TLS to device (port from SRV)                │
│   └───────┬────────┘                                                     │
│           │ TLS established (no client cert)                             │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │  RUN_PASE      │  SPAKE2+ with setup code                            │
│   │   (30s max)    │  (see commissioning-pase.md)                        │
│   └───────┬────────┘                                                     │
│           │ PASE verified                                                │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │ GET_ATTESTATION│  Request device attestation cert (optional)         │
│   └───────┬────────┘                                                     │
│           │ Verify manufacturer chain (if present)                       │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │  REQUEST_CSR   │  Ask device for CSR                                 │
│   └───────┬────────┘                                                     │
│           │ Receive CSR with device public key                           │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │  SIGN_CSR      │  Sign CSR with Zone CA                              │
│   └───────┬────────┘                                                     │
│           │ Generate operational certificate                             │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │ INSTALL_CERT   │  Send op cert + Zone CA cert                        │
│   └───────┬────────┘                                                     │
│           │ Device acknowledges installation                             │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │ UPDATE_STATE   │  Add device to zone member list                     │
│   └───────┬────────┘                                                     │
│           │                                                              │
│           ▼                                                              │
│   ┌────────────────┐                                                     │
│   │  RECONNECT     │  New TLS with mutual cert auth                      │
│   └───────┬────────┘                                                     │
│           │                                                              │
│           ▼                                                              │
│        SUCCESS                                                           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Admin Workflow (Delegated Commissioning)

When a phone app (admin) commissions on behalf of EMS (owner):

```
App (Admin)                    EMS (Owner)                    Device
    │                              │                             │
    │─── SCAN_QR_CODE ────────────────────────────────────────────►
    │    (gets D, setupcode)       │                             │
    │                              │                             │
    │─── BROWSE + CONNECT ────────────────────────────────────────►
    │                              │                             │
    │─── RUN_PASE (setup code) ───────────────────────────────────►
    │                              │                             │
    │◄─────────────────────────────────────────────────── CSR ────┤
    │                              │                             │
    │─── Forward CSR + admin token ►│                             │
    │                              │                             │
    │    (EMS verifies admin token) │                             │
    │    (EMS signs CSR with Zone CA)                             │
    │                              │                             │
    │◄── Signed op cert + Zone CA ──┤                             │
    │                              │                             │
    │─── INSTALL_CERT ────────────────────────────────────────────►
    │                              │                             │
    │◄─────────────────────────────────────────────────── ACK ────┤
    │                              │                             │
    │─── Notify success ───────────►│                             │
    │                              │─── Update member list ───────
```

### 3.3 Device State After Commissioning

Device stores per zone:

```
zoneSlot[n] = {
  zoneId: "a1b2c3d4e5f6a7b8",
  zoneCACert: <DER bytes>,         // For verifying zone members
  opCertPrivateKey: <encrypted>,   // Generated during CSR
  opCertificate: <DER bytes>,      // Signed by Zone CA
  zoneType: 2,                     // LOCAL
  zonePriority: 2,
  commissionedAt: <timestamp>
}
```

---

## 4. Certificate Renewal

### 4.1 Renewal Timeline

```
Certificate Lifecycle:
│
├── Day 0: Certificate issued (commissionedAt)
│
├── Day 335: Renewal window opens (30 days before expiry)
│   └── Controller should initiate renewal
│
├── Day 358: Warning period (7 days before expiry)
│   └── Device sends CERT_EXPIRING notification
│
├── Day 365: Certificate expires
│   └── See section 4.4 for expiry handling
│
└── Day 366+: Grace period (if configured)
```

### 4.2 Renewal Protocol

**Controller-initiated renewal (recommended):**

```
Controller                                    Device
    │                                            │
    │   (Controller tracks cert expiry locally)  │
    │                                            │
    │─── CERT_RENEWAL_REQ ──────────────────────►│
    │    { nonce: <32 bytes> }                   │
    │                                            │
    │◄── CERT_RENEWAL_CSR ──────────────────────┤
    │    { csr: <PKCS#10> }                      │
    │                                            │
    │   (Controller signs CSR with Zone CA)      │
    │                                            │
    │─── CERT_RENEWAL_INSTALL ─────────────────►│
    │    { newOpCert: <DER>, sequence: N }       │
    │                                            │
    │◄── CERT_RENEWAL_ACK ─────────────────────┤
    │    { status: 0, activeSequence: N }        │
    │                                            │
    │   (TLS session continues uninterrupted)    │
    │                                            │
```

**Message formats:**

```cbor
// CERT_RENEWAL_REQ (Controller → Device)
{
  1: "cert_renewal_req",
  2: <bytes[32]>              // nonce (anti-replay)
}

// CERT_RENEWAL_CSR (Device → Controller)
{
  1: "cert_renewal_csr",
  2: <bytes[]>                // CSR (PKCS#10, DER)
}

// CERT_RENEWAL_INSTALL (Controller → Device)
{
  1: "cert_renewal_install",
  2: <bytes[]>,               // new operational certificate (DER)
  3: <uint32>                 // sequence number (for ordering)
}

// CERT_RENEWAL_ACK (Device → Controller)
{
  1: "cert_renewal_ack",
  2: <uint8>,                 // status (0=success)
  3: <uint32>                 // active sequence number
}
```

### 4.3 Renewal Behavior

**Device behavior during renewal:**
1. Generate new key pair when CSR requested
2. Keep old certificate active until new one installed
3. Store new certificate alongside old one
4. Switch to new certificate on CERT_RENEWAL_INSTALL
5. Delete old certificate after successful switch
6. Acknowledge with new sequence number

**Session continuity:**
- TLS session remains open during renewal
- No reconnection required
- Subscription state preserved
- Commands continue normally

### 4.4 Certificate Expiry Handling

**If certificate expires before renewal:**

| Scenario | Controller Behavior | Device Behavior |
|----------|--------------------| ----------------|
| Controller cert expires | Close connection, regenerate cert, reconnect | Accept reconnection with new cert |
| Device cert expires | Detect during TLS, close connection | Close connection, wait for recommissioning |
| Both expire | Recommission device | Wait for recommissioning |

**Grace period (optional, device-specific):**
- Device MAY accept expired certs for up to 7 days
- Only for existing sessions (not new connections)
- PICS item: `MASH.S.CERT.GRACE_PERIOD_DAYS`

**Device expiry notification:**

When device cert is 7 days from expiry, device sends:

```cbor
// Event notification
{
  1: 0,                       // notification
  2: <subscriptionId>,
  3: 0,                       // endpoint 0 (device root)
  4: <DeviceInfo feature>,
  5: {
    "event": "cert_expiring",
    "zoneId": "a1b2c3d4e5f6a7b8",
    "expiresAt": 1737676850,
    "daysRemaining": 7
  }
}
```

---

## 5. Removing Devices from Zone

### 5.1 RemoveZone Command

**Initiated by controller:**

```cbor
// RemoveZone Request (Controller → Device)
{
  1: <messageId>,
  2: 4,                       // operation: Invoke
  3: 0,                       // endpoint 0 (device root)
  4: <DeviceInfo feature>,
  5: {
    1: 0x10,                  // commandId: RemoveZone
    2: {
      1: "a1b2c3d4e5f6a7b8"   // zoneId to remove
    }
  }
}

// RemoveZone Response (Device → Controller)
{
  1: <messageId>,
  2: 0,                       // status: success
  3: {
    1: true                   // removed
  }
}
```

### 5.2 Device Behavior on RemoveZone

1. **Validate request:**
   - Command must come from the zone being removed (self-removal only)

2. **Clean up zone state:**
   ```
   - Delete zoneSlot[n].opCertPrivateKey (secure erase)
   - Delete zoneSlot[n].opCertificate
   - Delete zoneSlot[n].zoneCACert
   - Clear zoneSlot[n] = null
   ```

3. **Close connection:**
   - Send RemoveZone response
   - Send graceful close message
   - Close TLS connection
   - Remove all subscriptions for that zone

4. **Update mDNS:**
   - Decrement zone count (ZC)
   - If last zone removed, switch to pre-commissioning mDNS

5. **Update ControlState:**
   - If this was the only zone: CONTROLLED → AUTONOMOUS
   - If other zones remain: No change

### 5.3 Controller Behavior on RemoveZone

1. **Send RemoveZone command**
2. **Wait for response**
3. **Update local state:**
   ```
   - Remove device from zone.members[]
   - Delete stored device info
   - Clear any cached data
   ```
4. **Connection will be closed by device**

---

## 6. Device-to-Device Zone Verification

### 6.1 Same-Zone Verification

Two devices in the same zone can verify each other:

**Verification algorithm:**
```python
def verify_same_zone(local_zone_ca: Certificate, peer_op_cert: Certificate) -> bool:
    """Verify peer is in the same zone."""
    # 1. Check peer cert is signed by our Zone CA
    if not peer_op_cert.verify_signature(local_zone_ca.public_key):
        return False

    # 2. Check peer cert is not expired
    if peer_op_cert.not_after < now():
        return False

    # 3. Check peer cert issuer matches our Zone CA
    if peer_op_cert.issuer != local_zone_ca.subject:
        return False

    return True
```

### 6.2 Peer Discovery Within Zone

Devices can discover other zone members via mDNS:

**Zone-specific mDNS record (optional):**
```
TXT ZI=a1b2c3d4...    // Zone ID (first 8 bytes of Zone CA fingerprint)
```

**Verification on connection:**
1. Device A connects to Device B
2. Both present operational certificates
3. Both verify certificates are signed by same Zone CA
4. If verification succeeds, peer communication allowed

### 6.3 Use Cases for Device-to-Device

| Use Case | Example |
|----------|---------|
| Local coordination | Two EVSEs sharing limited circuit capacity |
| Redundancy | Backup controller on same zone |
| Mesh networking | Future: devices relay messages |

---

## 7. QR Code Generation

### 7.1 When QR Code is Generated

| Method | When | Storage |
|--------|------|---------|
| Manufacturing | Factory provisioning | Printed label, stored in firmware |
| First boot | Device initialization | Generated, displayed on screen |
| Reset | Factory reset | Regenerated with new setup code |
| Manual | User request | Displayed temporarily |

### 7.2 QR Code Generation Algorithm

```python
def generate_qr_content(device: Device) -> str:
    """Generate MASH QR code content."""
    # Fixed at manufacturing or boot
    version = 1

    # Generated at manufacturing or reset
    discriminator = random.randint(0, 4095)
    setup_code = f"{random.randint(0, 99999999):08d}"

    # Store for PASE verification
    device.store_setup_code(setup_code)
    device.store_discriminator(discriminator)

    # Format QR content (simplified - no vendor/product IDs)
    return f"MASH:{version}:{discriminator}:{setup_code}"
```

**Note:** Device identification (brand, model, serial) is provided via mDNS TXT records, not in the QR code. This simplifies the QR code and removes dependency on vendor ID registration.

### 7.3 Setup Code Security

**Generation requirements:**
- Cryptographically random (not pseudo-random from predictable seed)
- Generated per-device (not shared across production batch)
- Stored securely (encrypted or in secure element)

**Regeneration events:**
- Factory reset
- Manual "reset setup code" command
- After N failed commissioning attempts (optional, prevents brute force)

### 7.4 QR Code Display

**Physical label:**
- Permanent: Printed during manufacturing
- Readable after installation
- Protected from casual viewing (inside panel, under cover)

**Digital display:**
- Shown only in commissioning mode
- Auto-hide after timeout (120s)
- User confirmation to display

---

## 8. Certificate Storage Requirements

### 8.1 Device Storage

| Item | Count | Size | Total |
|------|-------|------|-------|
| Zone CA cert | 5 max | ~500 bytes | 2.5 KB |
| Operational cert | 5 max | ~500 bytes | 2.5 KB |
| Operational private key | 5 max | ~150 bytes | 750 bytes |
| Device attestation cert | 1 | ~500 bytes | 500 bytes |
| Device attestation key | 1 | ~150 bytes | 150 bytes |
| **Total** | | | **~6.5 KB** |

### 8.2 Storage Security

| Item | Protection Level |
|------|-----------------|
| Private keys | Secure element or encrypted storage |
| Certificates | Plain storage (public data) |
| Setup code | Encrypted or secure element |
| Zone CA fingerprints | Plain storage |

### 8.3 Controller Storage

| Item | Per Zone | Notes |
|------|----------|-------|
| Zone CA private key | 1 | Must be highly protected |
| Zone CA certificate | 1 | Public |
| Device op certs | N (members) | For verification |
| Admin tokens | M (admins) | For delegation |

---

## 9. Revocation Scenarios

### 9.1 Normal Revocation (RemoveZone)

Controller can reach device:
1. Send RemoveZone command
2. Device deletes credentials
3. Immediate effect

### 9.2 Offline Device Revocation

Controller cannot reach device (offline, network issue):

**Controller behavior:**
1. Mark device as "pending removal" locally
2. Add device cert serial to local revocation list
3. On next connection from device, send RemoveZone
4. If device never reconnects, credentials expire naturally (1 year)

**Local revocation list:**
```cbor
{
  "revokedCerts": [
    {
      "serial": "abc123...",
      "revokedAt": 1706140800,
      "reason": "user_requested"
    }
  ]
}
```

### 9.3 Compromised Zone CA

If Zone CA private key is compromised:

**Recovery steps:**
1. Generate new Zone CA
2. Recommission all devices to new zone
3. Remove old zone from all devices
4. Destroy old Zone CA key

**No CRL/OCSP:** MASH intentionally omits traditional revocation infrastructure. Instead:
- Short cert lifetime (1 year)
- Direct revocation via RemoveZone
- Zone isolation prevents cross-zone attacks

### 9.4 Compromised Device

If device private key is compromised:

**Recovery:**
1. Factory reset device (generates new keys)
2. Recommission device (gets new operational cert)
3. Old cert expires naturally or is rejected by controller

---

## 10. PICS Items

```
# Zone capacity
MASH.S.ZONE.MAX_ZONES=5               # Maximum zone slots

# Certificate lifecycle
MASH.S.CERT.VALIDITY_DAYS=365         # Operational cert validity
MASH.S.CERT.RENEWAL_WINDOW_DAYS=30    # Days before expiry to renew
MASH.S.CERT.WARNING_DAYS=7            # Days before expiry warning
MASH.S.CERT.GRACE_PERIOD_DAYS=0       # Grace period (0=none)
MASH.S.CERT.SUPPORTS_IN_SESSION_RENEWAL=1  # Renew without disconnect

# Storage
MASH.S.STORE.SECURE_ELEMENT=1         # Has secure element
MASH.S.STORE.ENCRYPTED_KEYS=1         # Keys encrypted at rest

# QR code
MASH.S.QR.GENERATED_AT=MANUFACTURING  # MANUFACTURING | BOOT | RESET
MASH.S.QR.DISPLAY_CAPABLE=1           # Can display QR on screen
MASH.S.QR.REGENERATE_ON_RESET=1       # New code on factory reset

# Device-to-device
MASH.S.D2D.SUPPORTS_PEER_VERIFY=1     # Can verify same-zone peers
MASH.S.D2D.PUBLISHES_ZONE_ID=0        # Zone ID in mDNS TXT
```

---

## 11. Test Cases

### TC-ZONE-CREATE-*: Zone Creation

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ZONE-CREATE-1 | Create zone | Generate Zone CA | Valid CA cert, zone ID |
| TC-ZONE-CREATE-2 | Zone ID uniqueness | Create two zones | Different zone IDs |
| TC-ZONE-CREATE-3 | Zone metadata | Create with name/type | Metadata stored correctly |

### TC-ZONE-ADD-*: Adding Devices

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ZONE-ADD-1 | Direct commissioning | Full workflow | Device in zone.members |
| TC-ZONE-ADD-2 | Admin commissioning | Delegated flow | Device commissioned |
| TC-ZONE-ADD-3 | Wrong setup code | QR setupcode != device | PASE VERIFICATION_FAILED |
| TC-ZONE-ADD-4 | Discriminator collision | Two devices same D | Correct device found |
| TC-ZONE-ADD-5 | Second zone | Add to already-commissioned device | Both zones active |

### TC-CERT-RENEW-*: Certificate Renewal

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-CERT-RENEW-1 | Normal renewal | Renew before expiry | New cert installed |
| TC-CERT-RENEW-2 | Session continuity | Renew during operation | No disconnect |
| TC-CERT-RENEW-3 | Expiry warning | 7 days before expiry | Event notification |
| TC-CERT-RENEW-4 | Expired cert | Let cert expire | Connection closed |
| TC-CERT-RENEW-5 | Grace period | Expired + grace | Grace period works |

### TC-ZONE-REMOVE-*: Removing Devices

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-ZONE-REMOVE-1 | Self removal | RemoveZone from zone | Device removed |
| TC-ZONE-REMOVE-2 | Last zone | Remove only zone | Device uncommissioned |
| TC-ZONE-REMOVE-3 | Partial removal | Remove one of two | Other zone unaffected |
| TC-ZONE-REMOVE-4 | Offline removal | Device offline | Pending, expires |

### TC-D2D-*: Device-to-Device

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-D2D-1 | Same zone verify | Two devices, same zone | Verification succeeds |
| TC-D2D-2 | Different zone | Two devices, different zones | Verification fails |
| TC-D2D-3 | Expired peer cert | Peer cert expired | Verification fails |

### TC-QR-GEN-*: QR Code Generation

| ID | Description | Steps | Expected |
|----|-------------|-------|----------|
| TC-QR-GEN-1 | Factory QR | Check at boot | QR present |
| TC-QR-GEN-2 | Reset regenerates | Factory reset | New setup code |
| TC-QR-GEN-3 | Unique per device | Two devices | Different codes |
| TC-QR-GEN-4 | Display mode | Enter commissioning | QR shown (if capable) |

### TC-CTRL-CERT-*: Controller Certificate Lifecycle

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-CTRL-CERT-1 | Auto-generate on zone creation | New controller, no existing cert | Controller cert generated with Zone CA |
| TC-CTRL-CERT-2 | Load existing on restart | Controller restarts with existing cert | Same controller cert loaded (not regenerated) |
| TC-CTRL-CERT-3 | Renewal triggers at 30 days | Cert expires in 25 days | New cert generated automatically |
| TC-CTRL-CERT-4 | Renewal does not disrupt sessions | Active device connections during renewal | Sessions continue uninterrupted |
| TC-CTRL-CERT-5 | Controller ID stable across renewal | Renew controller cert | Same SKI/controller ID maintained |
| TC-CTRL-CERT-6 | Cert matches Zone CA | Zone CA rotated | Old controller cert invalid, new one generated |

---

## 12. Security Considerations

### 12.1 Zone Isolation

- Zones are cryptographically isolated (different CAs)
- No cross-zone communication without explicit multi-zone membership
- Compromised zone cannot affect other zones

### 12.2 Principle of Least Authority

- Zone Owner: Full authority (create, destroy, sign)
- Zone Admin: Limited (commission devices only)
- Zone Member: No authority (participate only)

### 12.3 Key Protection

| Key | Protection | Consequence if Compromised |
|-----|------------|---------------------------|
| Zone CA private | Highest | Entire zone compromised |
| Device operational | High | Single device impersonation |
| Device attestation | Medium | Device identity spoofing |
| Setup code | Medium | Unauthorized commissioning |

### 12.4 Attack Mitigation

| Attack | Mitigation |
|--------|------------|
| Setup code brute force | Rate limiting, commissioning window |
| Zone CA theft | Secure storage, regeneration procedure |
| Replay attacks | Nonces in renewal, timestamps in tokens |
| Man-in-the-middle | SPAKE2+ prevents during commissioning |
