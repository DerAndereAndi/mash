# Connection Establishment Behavior

> Precise specification of the complete connection flow from discovery to operational

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

This document specifies the complete connection establishment flow, bridging discovery, commissioning, and operational phases. It addresses the transition points between these phases.

**Scope:**
- mDNS record format (complete)
- IPv6 address resolution
- TLS handshake during commissioning
- PASE to operational session transition
- Certificate validation for operational TLS
- Address change handling

---

## 2. mDNS Record Format

MASH uses three separate service types (see `discovery.md` for full details):

| Service Type | Purpose | When Present |
|--------------|---------|--------------|
| `_mashc._udp` | Commissionable discovery | Commissioning window open |
| `_mash._tcp` | Operational discovery | Device has zone(s) |
| `_mashd._udp` | Commissioner discovery | Controller has zone(s) |

### 2.1 Commissionable Discovery Records (`_mashc._udp`)

During commissioning, device publishes:

**PTR Record:**
```
_mashc._udp.local.  PTR  MASH-1234._mashc._udp.local.
```

**SRV Record:**
```
MASH-1234._mashc._udp.local.  SRV  0 0 8443 evse-001.local.
```

**TXT Record:**
```
MASH-1234._mashc._udp.local.  TXT  "D=1234" "VP=1234:5678" "DT=EVSE"
```

**AAAA Record:**
```
evse-001.local.  AAAA  fe80::1234:5678:9abc:def0
evse-001.local.  AAAA  2001:db8::1234:5678:9abc:def0
```

### 2.2 Operational Discovery Records (`_mash._tcp`)

After commissioning, device publishes (one per zone):

**Instance name:** `<zone-id>-<device-id>` where both are fingerprint-derived (64 bits each):
- Zone ID: First 16 hex chars of SHA-256(Zone CA cert DER)
- Device ID: First 16 hex chars of SHA-256(device op cert public key DER)

**PTR Record:**
```
_mash._tcp.local.  PTR  A1B2C3D4E5F6A7B8-F9E8D7C6B5A49382._mash._tcp.local.
```

**SRV Record:**
```
A1B2C3D4E5F6A7B8-F9E8D7C6B5A49382._mash._tcp.local.  SRV  0 0 8443 evse-001.local.
```

**TXT Record:**
```
A1B2C3D4E5F6A7B8-F9E8D7C6B5A49382._mash._tcp.local.  TXT  "ZI=A1B2C3D4E5F6A7B8" "DI=F9E8D7C6B5A49382"
```

| Field | Value | Description |
|-------|-------|-------------|
| Priority | 0 | Single instance, no failover |
| Weight | 0 | No load balancing |
| Port | 8443 | MASH default port |
| Target | hostname.local | Device hostname |

### 2.3 Address Records

Devices MUST publish at least one AAAA record. MAY publish multiple:
- Link-local address (fe80::/10) - always available
- Global/ULA address - if configured

```
evse-001.local.  AAAA  fe80::1234:5678:9abc:def0
evse-001.local.  AAAA  2001:db8::1234:5678:9abc:def0
```

### 2.4 Address Selection

When multiple AAAA records exist, controller selects:

1. **Prefer global/ULA** over link-local (routable across subnets)
2. **Prefer ULA** (fd00::/8) over global (more stable in home networks)
3. **Fall back to link-local** if no others available

**Link-local connection:**
- Requires interface specification (e.g., `fe80::1%eth0`)
- Controller must determine correct interface
- Works without router/DHCP infrastructure

### 2.5 Record TTL (Time-To-Live)

| Record | TTL | Rationale |
|--------|-----|-----------|
| PTR | 4500s (75 min) | Standard DNS-SD |
| SRV | 120s (2 min) | Port changes rare but possible |
| AAAA | 120s (2 min) | Address may change |
| TXT | 4500s (75 min) | Metadata changes infrequently |

### 2.6 Record Updates

**When address changes:**
1. Send mDNS "goodbye" (TTL=0) for old AAAA
2. Send mDNS announcement for new AAAA (3 times)
3. Existing connections will break, clients reconnect

**When entering commissioning mode:**
1. Register `_mashc._udp` service instance
2. Send mDNS announcement (3 times)

**When commissioning completes:**
1. Deregister `_mashc._udp` service instance
2. Register `_mash._tcp` service instance for new zone
3. Send mDNS announcements

---

## 3. TLS During Commissioning

### 3.1 The Problem

During commissioning, the device has no operational certificate yet. What certificate does it present for TLS?

### 3.2 Solution: Temporary Self-Signed Certificate

**Device behavior on boot (if no operational certs):**

1. Generate temporary key pair (P-256)
2. Create self-signed certificate:
   ```
   Subject: CN=MASH-<discriminator>
   Issuer: CN=MASH-<discriminator> (self-signed)
   Validity: 1 day
   Key Usage: Digital Signature, Key Encipherment
   ```
3. Use this cert for TLS during commissioning

**Certificate lifecycle:**
- Generated at boot if no operational certs exist
- Regenerated on factory reset
- Deleted after first operational cert is installed
- Never used for operational connections

### 3.3 Commissioning TLS Handshake

```
Controller                                    Device
    │                                            │
    │──── ClientHello ──────────────────────────►│
    │     - TLS 1.3                               │
    │     - ALPN: "mash/1"                        │
    │     - No client cert (no Certificate msg)  │
    │                                            │
    │◄─── ServerHello ──────────────────────────┤
    │     - TLS 1.3                               │
    │     - Selected cipher                       │
    │                                            │
    │◄─── Certificate (self-signed) ────────────┤
    │     - CN=MASH-<discriminator>              │
    │                                            │
    │◄─── CertificateVerify ────────────────────┤
    │                                            │
    │◄─── Finished ─────────────────────────────┤
    │                                            │
    │──── Finished ─────────────────────────────►│
    │                                            │
    │     [TLS session established]              │
    │     [PASE messages now encrypted]          │
```

### 3.4 Controller Certificate Validation (Commissioning)

During commissioning, controller MUST:

1. **Accept self-signed certificate** - device has no Zone CA cert yet
2. **Verify CN matches discriminator** - CN=MASH-<D> where D is from QR code
3. **Skip chain validation** - no CA to verify against
4. **Proceed to PASE** - real authentication happens via SPAKE2+

**Security note:** The TLS handshake during commissioning does NOT authenticate the device. SPAKE2+ provides authentication via the setup code. TLS only provides encryption.

### 3.5 Why This Is Secure

| Attack | Mitigation |
|--------|------------|
| MITM during TLS | SPAKE2+ will fail (attacker doesn't know setup code) |
| Rogue device | SPAKE2+ will fail (different setup code) |
| Replay | SPAKE2+ uses random X/Y, fresh each session |
| Eavesdropping | TLS encrypts PASE messages |

---

## 4. PASE to Operational Transition

### 4.1 The Transition Problem

After PASE completes and operational cert is installed:
- Current TLS session used self-signed cert (not authenticated)
- Need to establish mutually authenticated session
- How do we transition?

### 4.2 Solution: Close and Reconnect

**After CERT_INSTALL succeeds:**

```
Controller                                    Device
    │                                            │
    │◄─── CERT_ACK {status: 0} ─────────────────┤
    │                                            │
    │──── Close {reason: "commissioning_complete"}►│
    │                                            │
    │◄─── Close_ACK ────────────────────────────┤
    │                                            │
    │     [TCP connection closed]                │
    │                                            │
    │     [Controller waits 1 second]            │
    │                                            │
    │──── New TCP Connect ──────────────────────►│
    │                                            │
    │──── TLS with operational certs ───────────►│
    │     (mutual authentication)                │
    │                                            │
    │     [OPERATIONAL session established]      │
```

### 4.3 Why Reconnect (Not Upgrade)?

| Approach | Pros | Cons |
|----------|------|------|
| **Reconnect** (chosen) | Clean state, simple implementation | Brief disconnect |
| TLS renegotiation | No disconnect | TLS 1.3 doesn't support renegotiation |
| Session upgrade | Continuous | Complex state management |

**Reconnect is cleanest:** The commissioning connection was unauthenticated. Starting fresh with mutual TLS is clearer and simpler.

### 4.4 Device Behavior During Transition

1. Receive CERT_ACK with success
2. Store operational cert and Zone CA cert
3. Receive Close message
4. Send Close_ACK
5. Close TCP connection
6. Update mDNS:
   - Deregister `_mashc._udp` service instance
   - Compute device ID: `hex(SHA-256(op cert public key DER)[0:8])` (16 hex chars)
   - Register `_mash._tcp` service instance with `<zone-id>-<device-id>` name
   - Update TXT records for operational mode (ZI, DI)
7. Listen for new connection
8. On new TLS connection, require mutual cert auth

### 4.5 Controller Behavior During Transition

1. Send Close after receiving CERT_ACK
2. Wait for Close_ACK
3. Close TCP connection
4. Wait 1 second (allow device to update state)
5. Browse mDNS for new instance name (or use same address)
6. Connect with operational TLS (present Zone CA-signed cert)
7. Verify device cert is signed by same Zone CA

---

## 5. Operational TLS Handshake

### 5.1 Mutual Authentication

```
Controller                                    Device
    │                                            │
    │──── ClientHello ──────────────────────────►│
    │     - TLS 1.3                               │
    │     - ALPN: "mash/1"                        │
    │     - SNI: <device-id> (optional)          │
    │                                            │
    │◄─── ServerHello ──────────────────────────┤
    │                                            │
    │◄─── CertificateRequest ───────────────────┤
    │     (device requests client cert)          │
    │                                            │
    │◄─── Certificate (device op cert) ─────────┤
    │                                            │
    │◄─── CertificateVerify ────────────────────┤
    │                                            │
    │◄─── Finished ─────────────────────────────┤
    │                                            │
    │──── Certificate (controller op cert) ─────►│
    │                                            │
    │──── CertificateVerify ────────────────────►│
    │                                            │
    │──── Finished ─────────────────────────────►│
    │                                            │
    │     [Mutual TLS established]               │
    │     [Both certs verified against Zone CA]  │
```

### 5.2 Certificate Validation Algorithm

**Controller validates device certificate:**

```python
def validate_device_cert(device_cert: X509, zone_ca_cert: X509) -> bool:
    """Validate device's operational certificate."""

    # 1. Check certificate is not expired
    if device_cert.not_before > now() or device_cert.not_after < now():
        return False  # Certificate expired or not yet valid

    # 2. Verify signature by Zone CA
    if not device_cert.verify_signature(zone_ca_cert.public_key):
        return False  # Not signed by our Zone CA

    # 3. Check issuer matches Zone CA subject
    if device_cert.issuer != zone_ca_cert.subject:
        return False  # Issuer mismatch

    # 4. Check key usage
    if not device_cert.has_key_usage(KEY_USAGE_DIGITAL_SIGNATURE):
        return False  # Invalid key usage

    # 5. Check extended key usage (if present)
    if device_cert.has_ext_key_usage():
        if not device_cert.has_ext_key_usage(EXT_KEY_USAGE_SERVER_AUTH):
            return False  # Wrong extended key usage

    return True
```

**Device validates controller certificate:**

```python
def validate_controller_cert(controller_cert: X509, zone_ca_cert: X509) -> bool:
    """Validate controller's operational certificate."""

    # 1. Check certificate is not expired
    if controller_cert.not_before > now() or controller_cert.not_after < now():
        return False

    # 2. Verify signature by Zone CA
    if not controller_cert.verify_signature(zone_ca_cert.public_key):
        return False  # Controller not in our zone

    # 3. Check issuer matches Zone CA subject
    if controller_cert.issuer != zone_ca_cert.subject:
        return False

    # 4. Check key usage
    if not controller_cert.has_key_usage(KEY_USAGE_DIGITAL_SIGNATURE):
        return False

    # 5. Check extended key usage (if present)
    if controller_cert.has_ext_key_usage():
        if not controller_cert.has_ext_key_usage(EXT_KEY_USAGE_CLIENT_AUTH):
            return False

    return True
```

### 5.3 Zone Membership Verification

Both parties verify they're in the same zone:

```
Zone membership = cert.issuer == local_zone_ca.subject
                  AND cert.verify_signature(local_zone_ca.public_key)
```

If verification fails:
- TLS handshake fails with certificate_unknown alert
- Connection closed
- No MASH messages exchanged

### 5.4 Clock Skew Handling

Certificate validation requires synchronized clocks.

**Tolerance:** Accept certificates within +/- 300 seconds of stated validity times.

```python
CLOCK_SKEW_TOLERANCE = 300  # seconds

def is_cert_valid(cert: X509) -> bool:
    now = current_time()
    return (cert.not_before - CLOCK_SKEW_TOLERANCE <= now <=
            cert.not_after + CLOCK_SKEW_TOLERANCE)
```

---

## 6. IPv6 Address Handling

### 6.1 Address Types

| Address Type | Range | Use Case |
|--------------|-------|----------|
| Link-local | fe80::/10 | Commissioning, same subnet |
| ULA | fd00::/8 | Home networks, stable |
| Global | 2000::/3 | Routable, may change |

### 6.2 Address Selection Priority

Controller should prefer addresses in this order:

1. **ULA (fd00::/8)** - Most stable in residential
2. **Global unicast** - Routable but may change
3. **Link-local (fe80::/10)** - Always available, same subnet only

### 6.3 Address Change Handling

**Device behavior when address changes:**

1. Update AAAA record in mDNS
2. Keep existing connections (TCP doesn't break immediately)
3. If connection breaks, wait for controller to reconnect

**Controller behavior when connection breaks:**

1. Detect connection loss (keep-alive failure)
2. Enter RECONNECTING state
3. Re-resolve device via mDNS (get new AAAA)
4. Connect to new address
5. Re-authenticate with mutual TLS
6. Re-establish subscriptions

### 6.4 Link-Local Interface Binding

For link-local addresses, controller must specify interface:

```
Connect to: fe80::1234:5678:9abc:def0%eth0
                                    ^^^^^^
                                    Interface identifier
```

**Controller determines interface:**
1. mDNS response arrives on specific interface
2. Use that interface for link-local connection
3. Store interface with device record

---

## 7. Complete Connection Sequence

### 7.1 First-Time Commissioning (End-to-End)

```
┌────────────────────────────────────────────────────────────────────────────┐
│                    COMPLETE FIRST-TIME CONNECTION SEQUENCE                  │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Controller                          Network                      Device   │
│      │                                  │                            │     │
│  1.  │ ─── Scan QR code ─────────────────────────────────────────────────  │
│      │     Parse: D=1234, setupcode, VP                              │     │
│      │                                  │                            │     │
│  2.  │ ─── mDNS Query ─────────────────►│                            │     │
│      │     _mashc._udp.local PTR?       │                            │     │
│      │                                  │                            │     │
│      │ ◄── mDNS Response ───────────────┤◄──────────────────────────┤     │
│      │     PTR MASH-1234._mashc._udp    │                            │     │
│      │     SRV 0 0 8443 evse.local      │                            │     │
│      │     AAAA fe80::1234              │                            │     │
│      │     TXT D=1234 VP=...            │                            │     │
│      │                                  │                            │     │
│  3.  │     [Verify: D matches, VP matches]                           │     │
│      │                                  │                            │     │
│  4.  │ ═══ TCP Connect ════════════════════════════════════════════►│     │
│      │     [fe80::1234]:8443            │                            │     │
│      │                                  │                            │     │
│  5.  │ ─── TLS ClientHello ────────────────────────────────────────►│     │
│      │     ALPN: mash/1, no client cert │                            │     │
│      │                                  │                            │     │
│      │ ◄── TLS ServerHello + Cert ─────────────────────────────────┤     │
│      │     Cert: CN=MASH-1234 (self-signed)                         │     │
│      │                                  │                            │     │
│      │     [Accept self-signed, verify CN=MASH-<D>]                 │     │
│      │                                  │                            │     │
│      │ ─── TLS Finished ───────────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │     [TLS ESTABLISHED - encrypted but not authenticated]      │     │
│      │                                  │                            │     │
│  6.  │ ─── PASE_X {X, context} ────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── PASE_Y {Y, verifier_device} ────────────────────────────┤     │
│      │                                  │                            │     │
│      │ ─── PASE_VERIFY {verifier_ctrl} ────────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── PASE_CONFIRM {status: 0} ───────────────────────────────┤     │
│      │                                  │                            │     │
│      │     [PASE VERIFIED - device authenticated via setup code]    │     │
│      │                                  │                            │     │
│  7.  │ ─── ATTESTATION_REQ ────────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── ATTESTATION_RSP {device_cert, chain} ───────────────────┤     │
│      │                                  │  (optional)                │     │
│      │                                  │                            │     │
│  8.  │ ─── CSR_REQ {nonce} ────────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── CSR_RSP {csr} ──────────────────────────────────────────┤     │
│      │                                  │                            │     │
│      │     [Sign CSR with Zone CA]      │                            │     │
│      │                                  │                            │     │
│  9.  │ ─── CERT_INSTALL {op_cert, zone_ca} ────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── CERT_ACK {status: 0} ───────────────────────────────────┤     │
│      │                                  │                            │     │
│      │     [COMMISSIONING COMPLETE]     │                            │     │
│      │                                  │                            │     │
│ 10.  │ ─── Close {reason: commissioning_complete} ─────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── Close_ACK ──────────────────────────────────────────────┤     │
│      │                                  │                            │     │
│      │     [TCP CLOSED]                 │                            │     │
│      │                                  │                            │     │
│      │     [Wait 1 second]              │      [Update mDNS]         │     │
│      │                                  │      [_mashc gone,         │     │
│      │                                  │       _mash._tcp added]    │     │
│      │                                  │                            │     │
│ 11.  │ ═══ TCP Connect (new) ══════════════════════════════════════►│     │
│      │                                  │                            │     │
│ 12.  │ ─── TLS ClientHello ────────────────────────────────────────►│     │
│      │     ALPN: mash/1                 │                            │     │
│      │                                  │                            │     │
│      │ ◄── CertificateRequest ─────────────────────────────────────┤     │
│      │                                  │                            │     │
│      │ ◄── Certificate (device op cert) ───────────────────────────┤     │
│      │                                  │                            │     │
│      │     [Validate: signed by Zone CA, not expired]               │     │
│      │                                  │                            │     │
│      │ ─── Certificate (controller op cert) ───────────────────────►│     │
│      │                                  │                            │     │
│      │     [Device validates: signed by Zone CA]                    │     │
│      │                                  │                            │     │
│      │ ─── TLS Finished ───────────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │     [MUTUAL TLS ESTABLISHED - both authenticated]            │     │
│      │                                  │                            │     │
│ 13.  │ ─── Read {DeviceInfo} ──────────────────────────────────────►│     │
│      │                                  │                            │     │
│      │ ◄── Response {attributes} ──────────────────────────────────┤     │
│      │                                  │                            │     │
│      │     [OPERATIONAL - ready for commands]                       │     │
│      │                                  │                            │     │
└────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Reconnection (After Disconnect)

```
Controller                                    Device
    │                                            │
    │     [Detect connection loss]               │
    │     [Enter RECONNECTING state]             │
    │                                            │
    │──── mDNS re-resolve (if needed) ──────────►│
    │                                            │
    │◄─── AAAA response ────────────────────────┤
    │                                            │
    │═══ TCP Connect ═══════════════════════════►│
    │                                            │
    │──── TLS with mutual certs ────────────────►│
    │     [No PASE needed - already commissioned]│
    │                                            │
    │     [MUTUAL TLS ESTABLISHED]               │
    │                                            │
    │──── Re-subscribe ─────────────────────────►│
    │                                            │
    │◄─── Priming report ───────────────────────┤
    │                                            │
    │     [OPERATIONAL]                          │
```

---

## 8. PICS Items

```
# mDNS records
MASH.S.MDNS.SRV_RECORD=1              # Publishes SRV record
MASH.S.MDNS.AAAA_RECORD=1             # Publishes AAAA record
MASH.S.MDNS.LINK_LOCAL=1              # Supports link-local
MASH.S.MDNS.GLOBAL_ADDR=1             # Supports global address

# TLS commissioning
MASH.S.TLS.SELF_SIGNED_COMM=1         # Uses self-signed for commissioning
MASH.S.TLS.ALPN=mash/1                # ALPN protocol identifier
MASH.S.TLS.MUTUAL_AUTH=1              # Requires mutual auth for operational

# Transition
MASH.S.TRANS.RECONNECT_AFTER_COMM=1   # Reconnects after commissioning
MASH.S.TRANS.WAIT_TIME=1              # Wait time before reconnect (seconds)

# Certificate validation
MASH.S.CERT.CLOCK_SKEW_TOLERANCE=300  # Clock skew tolerance (seconds)
MASH.S.CERT.VALIDATES_CHAIN=1         # Validates certificate chain
MASH.S.CERT.CHECKS_KEY_USAGE=1        # Checks key usage extension

# IPv6
MASH.S.IPV6.LINK_LOCAL_INTERFACE=1    # Handles link-local interface binding
MASH.S.IPV6.ADDRESS_CHANGE=1          # Handles address changes
```

---

## 9. Test Cases

### TC-MDNS-REC-*: mDNS Records

| ID | Description | Expected |
|----|-------------|----------|
| TC-MDNS-REC-1 | SRV record present | Port 8443, valid hostname |
| TC-MDNS-REC-2 | AAAA record present | Valid IPv6 address |
| TC-MDNS-REC-3 | Multiple AAAA | ULA and link-local |
| TC-MDNS-REC-4 | Address change | New AAAA announced |

### TC-TLS-COMM-*: TLS During Commissioning

| ID | Description | Expected |
|----|-------------|----------|
| TC-TLS-COMM-1 | Self-signed cert | CN=MASH-<D> accepted |
| TC-TLS-COMM-2 | Wrong CN | CN doesn't match D, warning |
| TC-TLS-COMM-3 | No client cert | Connection accepted |
| TC-TLS-COMM-4 | ALPN mash/1 | Protocol selected |

### TC-TRANS-*: PASE to Operational Transition

| ID | Description | Expected |
|----|-------------|----------|
| TC-TRANS-1 | Normal transition | Close, reconnect, mutual TLS |
| TC-TRANS-2 | mDNS update | `_mashc._udp` removed, `_mash._tcp` added |
| TC-TRANS-3 | Service type switch | `_mash._tcp` instance name: `<zone-id>-<device-id>` |
| TC-TRANS-4 | Reconnect timeout | 10 seconds max |

### TC-CERT-VAL-*: Certificate Validation

| ID | Description | Expected |
|----|-------------|----------|
| TC-CERT-VAL-1 | Valid cert | Verification succeeds |
| TC-CERT-VAL-2 | Expired cert | Verification fails |
| TC-CERT-VAL-3 | Wrong Zone CA | Verification fails |
| TC-CERT-VAL-4 | Clock skew 200s | Verification succeeds |
| TC-CERT-VAL-5 | Clock skew 400s | Verification fails |

### TC-IPV6-*: IPv6 Address Handling

| ID | Description | Expected |
|----|-------------|----------|
| TC-IPV6-1 | Link-local connect | Connection succeeds |
| TC-IPV6-2 | Global address | Preferred over link-local |
| TC-IPV6-3 | Address change | Reconnect to new address |
| TC-IPV6-4 | Interface binding | Correct interface used |

### TC-E2E-*: End-to-End Flow

| ID | Description | Expected |
|----|-------------|----------|
| TC-E2E-1 | First commissioning | Full flow succeeds |
| TC-E2E-2 | Reconnection | Mutual TLS, no PASE |
| TC-E2E-3 | Second zone | Additional commissioning |
| TC-E2E-4 | Device reboot | Reconnection works |

---

## 10. Security Considerations

### 10.1 Commissioning TLS Security

The self-signed certificate during commissioning provides:
- **Encryption:** PASE messages are encrypted
- **NOT authentication:** Device identity not verified

Authentication comes from SPAKE2+:
- Setup code proves device possession
- Verifier exchange proves both sides know code

### 10.2 Certificate Pinning

After commissioning, controller MAY pin:
- Device operational certificate
- Zone CA certificate

This prevents:
- Certificate substitution attacks
- Compromised Zone CA (if pinning op cert directly)

### 10.3 Address Spoofing

mDNS responses can be spoofed. Mitigations:
- SPAKE2+ authenticates during commissioning
- Mutual TLS authenticates during operation
- Address spoofing only causes DoS, not impersonation
