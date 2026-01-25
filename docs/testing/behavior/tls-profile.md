# TLS Profile

> Precise specification of TLS 1.3 requirements for MASH

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

MASH uses TLS 1.3 for all transport security. This document consolidates all TLS requirements in one place for testability and interoperability.

**Key principles:**
- TLS 1.3 only (no fallback to 1.2)
- Mutual certificate authentication for operational connections
- Self-signed certificates accepted during commissioning only
- Minimal extension set for constrained devices
- No session resumption (simplicity over optimization)

---

## 2. TLS Version

| Requirement | Value | Rationale |
|-------------|-------|-----------|
| Minimum version | TLS 1.3 | Modern security, simpler handshake |
| Maximum version | TLS 1.3 | Single version for interoperability |
| TLS 1.2 fallback | **Prohibited** | Attack surface reduction |

**Implementation note:** Devices MUST reject connections attempting TLS 1.2 or earlier with `protocol_version` alert.

---

## 3. Cipher Suites

### 3.1 Required Cipher Suites

TLS 1.3 cipher suites (in preference order):

| Cipher Suite | Status | Notes |
|--------------|--------|-------|
| `TLS_AES_128_GCM_SHA256` | **Mandatory** | Must be supported by all implementations |
| `TLS_AES_256_GCM_SHA384` | Recommended | Stronger, slightly higher overhead |
| `TLS_CHACHA20_POLY1305_SHA256` | Optional | Good for devices without AES-NI |

**Note:** TLS 1.3 cipher suites only specify symmetric encryption. Key exchange is negotiated separately via supported_groups.

### 3.2 Prohibited Cipher Suites

The following MUST NOT be offered or accepted:

- Any TLS 1.2 cipher suites
- `TLS_AES_128_CCM_SHA256` (less common, interoperability risk)
- `TLS_AES_128_CCM_8_SHA256` (truncated tag, weaker security)

### 3.3 Cipher Suite Selection

**Server (device) behavior:**
1. Select first cipher from client's list that server supports
2. Prefer AES-128-GCM if client supports it
3. If no common cipher, send `handshake_failure` alert

**Client (controller) behavior:**
1. Offer ciphers in preference order (AES-128-GCM first)
2. Accept any cipher from section 3.1

---

## 4. Key Exchange

### 4.1 Supported Groups (Key Exchange)

| Group | Status | Notes |
|-------|--------|-------|
| `secp256r1` (P-256) | **Mandatory** | Must be supported by all implementations |
| `x25519` | Recommended | Faster, constant-time implementations common |
| `secp384r1` (P-384) | Optional | Higher security margin |

**Prohibited:**
- `secp521r1` (P-521) - Excessive overhead for home automation
- `ffdhe*` groups - DHE not needed with ECDHE available
- Any group < 256 bits

### 4.2 Key Share Extension

Both client and server MUST include `key_share` extension in ClientHello/ServerHello.

**Client behavior:**
- Include key share for `secp256r1` (mandatory)
- MAY include key share for `x25519` (avoids HelloRetryRequest)

**Server behavior:**
- Select from client's offered key shares
- If no acceptable key share, send HelloRetryRequest with supported group
- Maximum one HelloRetryRequest per handshake

---

## 5. Signature Algorithms

### 5.1 For TLS Handshake

Algorithms for CertificateVerify message:

| Algorithm | Status | OID |
|-----------|--------|-----|
| `ecdsa_secp256r1_sha256` | **Mandatory** | Matches P-256 certificates |
| `ecdsa_secp384r1_sha384` | Optional | For P-384 certificates |
| `rsa_pss_rsae_sha256` | **Prohibited** | No RSA in MASH |
| `ed25519` | Optional | If X25519 certificates supported |

### 5.2 For Certificates

Certificate signatures MUST use:

| Algorithm | Status | Notes |
|-----------|--------|-------|
| ECDSA with P-256 + SHA-256 | **Mandatory** | All MASH certificates |
| ECDSA with P-384 + SHA-384 | Optional | Higher security option |

**Prohibited in certificates:**
- RSA signatures (any key size)
- SHA-1 (any algorithm)
- DSA

### 5.3 Signature Algorithm Extension

Both client and server MUST include `signature_algorithms` extension.

```
signature_algorithms: [
    ecdsa_secp256r1_sha256,  // Mandatory
    ecdsa_secp384r1_sha384   // Optional
]
```

---

## 6. TLS Extensions

### 6.1 Required Extensions

| Extension | Client | Server | Purpose |
|-----------|--------|--------|---------|
| `supported_versions` | MUST send | MUST send | Negotiate TLS 1.3 |
| `supported_groups` | MUST send | N/A | Key exchange groups |
| `signature_algorithms` | MUST send | N/A | Cert verification |
| `key_share` | MUST send | MUST send | ECDHE key exchange |

### 6.2 Recommended Extensions

| Extension | Client | Server | Purpose |
|-----------|--------|--------|---------|
| `server_name` (SNI) | SHOULD send | MAY use | Virtual hosting, logging |
| `application_layer_protocol_negotiation` (ALPN) | MUST send | MUST send | Protocol identification |

### 6.3 Optional Extensions

| Extension | Client | Server | Purpose |
|-----------|--------|--------|---------|
| `certificate_authorities` | MAY send | N/A | Hint which CA to use |
| `post_handshake_auth` | MAY send | N/A | Post-handshake client auth |

### 6.4 Prohibited Extensions

| Extension | Reason |
|-----------|--------|
| `pre_shared_key` | No session resumption (see section 8) |
| `psk_key_exchange_modes` | No session resumption |
| `early_data` | No 0-RTT (see section 8) |
| `status_request` (OCSP) | No certificate revocation checking |
| `signed_certificate_timestamp` | No CT required |
| `compress_certificate` | Complexity, minimal benefit |
| `heartbeat` | Security risk, not needed |

### 6.5 Unknown Extensions

**Client behavior:** Ignore unknown extensions in ServerHello
**Server behavior:** Ignore unknown extensions in ClientHello

Do NOT abort handshake for unknown extensions (forward compatibility).

---

## 7. SNI (Server Name Indication)

### 7.1 Requirements

| Role | Requirement |
|------|-------------|
| Client (controller) | SHOULD send SNI with device ID |
| Server (device) | MAY use SNI for logging, MUST NOT require it |

### 7.2 SNI Value Format

When sent, SNI SHOULD contain the device ID:

```
server_name: <device-id>.local
```

Where `device-id` is the 16 hex character device identifier from mDNS.

**Example:**
```
server_name: F9E8D7C6B5A49382.local
```

### 7.3 SNI Handling

**Device behavior:**
1. If SNI present, log for debugging
2. If SNI matches device ID, proceed
3. If SNI doesn't match, proceed anyway (device ID may be stale)
4. If SNI absent, proceed (optional extension)

**Never reject connection based on SNI mismatch** - the certificate validation is the authoritative check.

---

## 8. Session Resumption and 0-RTT

### 8.1 Session Resumption

**Status:** Prohibited

MASH does NOT support TLS session resumption (PSK-based or session tickets).

| Feature | Status | Rationale |
|---------|--------|-----------|
| Session tickets | Prohibited | Complexity, minimal benefit for persistent connections |
| PSK resumption | Prohibited | Same |
| Session ID | N/A | TLS 1.3 doesn't use session IDs |

**Implementation:**
- Server MUST NOT send NewSessionTicket messages
- Client MUST NOT send `pre_shared_key` extension
- Client MUST NOT send `psk_key_exchange_modes` extension

### 8.2 0-RTT (Early Data)

**Status:** Prohibited

| Feature | Status | Rationale |
|---------|--------|-----------|
| 0-RTT early data | Prohibited | Replay attack risk |
| `early_data` extension | Prohibited | Not applicable |

**Rationale:**
- MASH connections are persistent (minutes to hours)
- Reconnection latency is acceptable (full handshake ~100ms on LAN)
- 0-RTT has replay risks that outweigh benefits for control protocols
- Simpler implementation for constrained devices

---

## 9. Certificate Chain

### 9.1 Chain Depth

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Maximum chain depth | **2** | Zone CA → Device/Controller cert |
| Typical chain depth | 2 | All MASH deployments |

**Chain structure:**
```
Zone CA Certificate (root, self-signed)
    └── Device/Controller Operational Certificate (leaf)
```

**No intermediate CAs** - MASH zones are flat, single CA per zone.

### 9.2 Chain Validation

**Validation algorithm:**

```python
def validate_certificate_chain(chain: list[X509], zone_ca: X509) -> bool:
    """Validate peer's certificate chain."""

    if len(chain) == 0:
        return False  # No certificate provided

    if len(chain) > 2:
        return False  # Chain too deep

    leaf_cert = chain[0]

    # If chain includes CA, verify it matches our Zone CA
    if len(chain) == 2:
        provided_ca = chain[1]
        if provided_ca != zone_ca:
            return False  # Wrong CA in chain

    # Verify leaf is signed by Zone CA
    if not leaf_cert.verify_signature(zone_ca.public_key):
        return False

    # Verify leaf issuer matches Zone CA subject
    if leaf_cert.issuer != zone_ca.subject:
        return False

    # Verify validity period
    if not is_valid_time(leaf_cert, tolerance=300):
        return False

    return True
```

### 9.3 Certificate Message

**Device (server) sends:**
- Leaf certificate (operational cert)
- Optionally: Zone CA certificate

**Controller (client) sends:**
- Leaf certificate (operational cert)
- Optionally: Zone CA certificate

**Best practice:** Include Zone CA in chain for debugging, but recipient validates against locally stored Zone CA, not the one in the chain.

---

## 10. Certificate Revocation

### 10.1 Status

**Certificate revocation checking is DISABLED in MASH.**

| Mechanism | Status | Rationale |
|-----------|--------|-----------|
| OCSP | Not used | Requires internet connectivity |
| OCSP Stapling | Not used | Complexity for constrained devices |
| CRL | Not used | Distribution challenges on LAN |
| CT (Certificate Transparency) | Not used | Public logs not applicable |

### 10.2 Rationale

MASH operates on local networks without guaranteed internet connectivity:
- Devices may be air-gapped
- OCSP responders unreachable
- CRL distribution impractical

### 10.3 Alternative: Short-Lived Certificates

Instead of revocation, MASH uses:

| Mechanism | Description |
|-----------|-------------|
| Short validity | Certificates valid for 1 year (default) |
| Renewal | Proactive renewal before expiry |
| Zone removal | Remove device from zone (delete cert from device) |
| Zone CA rotation | Create new zone to revoke all old certs |

### 10.4 Emergency Revocation

If a device is compromised:

1. **Remove from zone** - Controller sends removal command, device deletes certs
2. **If device unresponsive** - Other devices in zone will reject it (different Zone CA after rotation)
3. **Factory reset** - Physical access to compromised device

---

## 11. TLS Alerts

### 11.1 Alerts That MUST Be Sent

| Condition | Alert | Action After |
|-----------|-------|--------------|
| TLS 1.2 or earlier requested | `protocol_version` | Close |
| No common cipher suite | `handshake_failure` | Close |
| No common signature algorithm | `handshake_failure` | Close |
| Certificate validation failed | `bad_certificate` | Close |
| Certificate expired | `certificate_expired` | Close |
| Unknown CA (wrong zone) | `unknown_ca` | Close |
| Certificate has wrong key usage | `bad_certificate` | Close |
| Decryption failed | `bad_record_mac` | Close |
| Unexpected message | `unexpected_message` | Close |
| Internal error | `internal_error` | Close |

### 11.2 Alert Handling

**On receiving alert:**

```python
def handle_tls_alert(alert: TLSAlert) -> Action:
    """Handle incoming TLS alert."""

    # Fatal alerts - close immediately
    if alert.level == AlertLevel.FATAL:
        log.error(f"TLS fatal alert: {alert.description}")
        close_connection()

        if alert.description in [
            "certificate_expired",
            "unknown_ca",
            "bad_certificate"
        ]:
            return Action.CERT_ERROR  # May need recommissioning
        else:
            return Action.RECONNECT  # Retry connection

    # Warning alerts - log but continue
    if alert.level == AlertLevel.WARNING:
        log.warning(f"TLS warning alert: {alert.description}")

        if alert.description == "close_notify":
            # Graceful close
            send_close_notify()
            close_connection()
            return Action.RECONNECT

        # Other warnings - continue
        return Action.CONTINUE
```

### 11.3 Close Notify

**Graceful TLS close:**

1. Send `close_notify` alert
2. Wait up to 5 seconds for peer's `close_notify`
3. Close TCP connection

**On receiving `close_notify`:**

1. Send `close_notify` in response
2. Close TCP connection
3. Do NOT treat as error (graceful close)

---

## 12. ALPN (Application-Layer Protocol Negotiation)

### 12.1 Protocol Identifier

| Protocol | ALPN String | Usage |
|----------|-------------|-------|
| MASH v1 | `mash/1` | All MASH connections |

### 12.2 Requirements

| Role | Requirement |
|------|-------------|
| Client | MUST send ALPN with `mash/1` |
| Server | MUST send ALPN with `mash/1` if client offered it |

### 12.3 ALPN Mismatch

If server doesn't support `mash/1`:
- Server sends `no_application_protocol` alert
- Client closes connection
- Client reports "Incompatible protocol version"

If client doesn't send ALPN:
- Server MAY accept connection (permissive)
- Server SHOULD log warning
- Recommended: Server rejects with `no_application_protocol`

---

## 13. Timing Requirements

### 13.1 Handshake Timeouts

| Phase | Timeout | Notes |
|-------|---------|-------|
| TCP connect | 5 seconds | Per address attempt |
| TLS handshake (total) | 15 seconds | ClientHello to Finished |
| Certificate validation | Included in handshake | No external calls |

### 13.2 Handshake Performance

**Expected handshake time on LAN:**

| Scenario | Time |
|----------|------|
| Full handshake (P-256) | 50-150ms |
| Full handshake (X25519) | 30-100ms |
| Constrained device (ESP32) | 200-500ms |

---

## 14. Implementation Notes

### 14.1 Recommended Libraries

| Platform | Library | Notes |
|----------|---------|-------|
| Go | `crypto/tls` | Excellent TLS 1.3 support |
| Rust | `rustls` | Modern, safe implementation |
| C/C++ | `mbedTLS` | Good for constrained devices |
| C/C++ | `wolfSSL` | Commercial-friendly license |
| Python | `ssl` (OpenSSL) | Standard library |

### 14.2 Configuration Checklist

```
☐ TLS 1.3 only (no 1.2 fallback)
☐ Cipher suites: AES-128-GCM mandatory
☐ Key exchange: P-256 mandatory
☐ Signature algorithms: ECDSA P-256 + SHA-256
☐ ALPN: mash/1
☐ No session tickets
☐ No 0-RTT
☐ Certificate chain depth ≤ 2
☐ No OCSP/CRL checking
☐ Mutual authentication for operational
☐ Self-signed accepted for commissioning only
```

---

## 15. PICS Items

```
# TLS version
MASH.S.TLS.VERSION=1.3                    # TLS version
MASH.S.TLS.FALLBACK_DISABLED=1            # No TLS 1.2 fallback

# Cipher suites
MASH.S.TLS.CS_AES_128_GCM=1               # TLS_AES_128_GCM_SHA256 (mandatory)
MASH.S.TLS.CS_AES_256_GCM=1               # TLS_AES_256_GCM_SHA384 (recommended)
MASH.S.TLS.CS_CHACHA20=0                  # TLS_CHACHA20_POLY1305_SHA256 (optional)

# Key exchange
MASH.S.TLS.KX_P256=1                      # secp256r1 (mandatory)
MASH.S.TLS.KX_X25519=1                    # x25519 (recommended)
MASH.S.TLS.KX_P384=0                      # secp384r1 (optional)

# Signature algorithms
MASH.S.TLS.SIG_ECDSA_P256_SHA256=1        # Mandatory
MASH.S.TLS.SIG_ECDSA_P384_SHA384=0        # Optional

# Extensions
MASH.S.TLS.EXT_SNI=1                      # SNI support
MASH.S.TLS.EXT_ALPN=1                     # ALPN support (mandatory)
MASH.S.TLS.EXT_SESSION_TICKET=0           # Session tickets (prohibited)
MASH.S.TLS.EXT_EARLY_DATA=0               # 0-RTT (prohibited)

# Certificate
MASH.S.TLS.CERT_CHAIN_MAX_DEPTH=2         # Maximum chain depth
MASH.S.TLS.CERT_REVOCATION=0              # No OCSP/CRL

# Authentication
MASH.S.TLS.MUTUAL_AUTH_OPERATIONAL=1      # Mutual auth for operational
MASH.S.TLS.SELF_SIGNED_COMMISSIONING=1    # Self-signed for commissioning

# Timing
MASH.S.TLS.HANDSHAKE_TIMEOUT=15           # Handshake timeout (seconds)
```

---

## 16. Test Cases

### TC-TLS-VERSION-*: TLS Version

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-VERSION-1 | TLS 1.3 accepted | Client offers 1.3 | Handshake succeeds |
| TC-TLS-VERSION-2 | TLS 1.2 rejected | Client offers only 1.2 | `protocol_version` alert |
| TC-TLS-VERSION-3 | TLS 1.3 selected | Client offers 1.2 + 1.3 | Server selects 1.3 |

### TC-TLS-CIPHER-*: Cipher Suites

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-CIPHER-1 | AES-128-GCM accepted | Offer AES-128-GCM | Handshake succeeds |
| TC-TLS-CIPHER-2 | AES-256-GCM accepted | Offer AES-256-GCM | Handshake succeeds |
| TC-TLS-CIPHER-3 | No common cipher | Offer only CCM | `handshake_failure` alert |
| TC-TLS-CIPHER-4 | Preference order | Offer 256 before 128 | Server selects client's first |

### TC-TLS-KX-*: Key Exchange

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-KX-1 | P-256 accepted | Offer P-256 | Handshake succeeds |
| TC-TLS-KX-2 | X25519 accepted | Offer X25519 | Handshake succeeds (if supported) |
| TC-TLS-KX-3 | HelloRetryRequest | Offer only X25519, server needs P-256 | HRR sent, handshake succeeds |
| TC-TLS-KX-4 | No common group | Offer only P-521 | `handshake_failure` alert |

### TC-TLS-ALPN-*: ALPN

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-ALPN-1 | mash/1 accepted | Offer mash/1 | Handshake succeeds, ALPN confirmed |
| TC-TLS-ALPN-2 | Wrong protocol | Offer http/1.1 only | `no_application_protocol` alert |
| TC-TLS-ALPN-3 | No ALPN | Client sends no ALPN | Server rejects or warns |

### TC-TLS-CHAIN-*: Certificate Chain

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-CHAIN-1 | Valid chain depth 2 | Zone CA + leaf | Validation succeeds |
| TC-TLS-CHAIN-2 | Chain too deep | 3 certs in chain | Validation fails |
| TC-TLS-CHAIN-3 | Leaf only | No CA in chain | Validation succeeds (local CA used) |
| TC-TLS-CHAIN-4 | Wrong CA in chain | Different CA cert | Validation fails |

### TC-TLS-RESUME-*: Session Resumption

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-RESUME-1 | No session ticket | Complete handshake | No NewSessionTicket sent |
| TC-TLS-RESUME-2 | PSK rejected | Client sends PSK | Server ignores, full handshake |
| TC-TLS-RESUME-3 | Early data rejected | Client sends early_data | Server ignores, full handshake |

### TC-TLS-ALERT-*: Alert Handling

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-TLS-ALERT-1 | Close notify | Send close_notify | Peer responds, connection closes |
| TC-TLS-ALERT-2 | Bad certificate | Invalid cert | `bad_certificate` alert |
| TC-TLS-ALERT-3 | Expired certificate | Expired cert | `certificate_expired` alert |
| TC-TLS-ALERT-4 | Unknown CA | Wrong zone cert | `unknown_ca` alert |

---

## 17. Security Considerations

### 17.1 Downgrade Attacks

TLS 1.3's `supported_versions` extension prevents downgrade attacks. The handshake transcript hash includes all negotiated parameters.

### 17.2 Weak Cipher Suites

By prohibiting CCM-8 and legacy ciphers, MASH avoids known weaknesses in truncated authentication tags.

### 17.3 Forward Secrecy

All MASH connections use ephemeral ECDHE key exchange, providing forward secrecy. Compromise of long-term certificate keys does not expose past session keys.

### 17.4 Certificate Pinning

Controllers MAY implement certificate pinning:
- Pin Zone CA certificate (recommended)
- Pin device operational certificate (optional, stricter)

Pinning protects against compromised Zone CA issuing rogue certificates.

---

## 18. References

- [RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446) - TLS 1.3
- [RFC 8447](https://datatracker.ietf.org/doc/html/rfc8447) - TLS Cipher Suite Registry Updates
- [RFC 7919](https://datatracker.ietf.org/doc/html/rfc7919) - Negotiated Finite Field DH for TLS
- [RFC 8422](https://datatracker.ietf.org/doc/html/rfc8422) - ECC Cipher Suites for TLS 1.2+
