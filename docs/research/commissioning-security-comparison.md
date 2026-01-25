# Commissioning and Security Model Comparison

**Date:** 2025-01-24
**Context:** Designing MASH security model - learning from SHIP failures and Matter successes

---

## SHIP Trust Model - What's Wrong

### The Trust Level Confusion (0-100)

SHIP has three "trust categories" that combine somehow:

| Category | Mechanisms | Levels |
|----------|------------|--------|
| User Trust | Auto-accept (8), User verify (32) | 0-32 |
| PKI Trust | Certificate validation | 0-16 |
| Second Factor | PIN (16-32) | 0-32 |

**Problems:**
- How do they combine? Addition? Maximum? Unclear.
- What does "trust level 48" mean practically?
- Different features require different levels - but which?
- "PKI trust depends on manufacturer's policy" - vague

### Self-Signed Certificates

```
SHIP Node A                    SHIP Node B
┌─────────────┐               ┌─────────────┐
│ Self-signed │◄─────────────►│ Self-signed │
│ Certificate │   TLS 1.3     │ Certificate │
└─────────────┘               └─────────────┘
```

**Problems:**
- No chain of trust - anyone can generate a certificate
- No attestation - can't verify device is genuine
- SKI (Subject Key Identifier) is the only identity
- Certificate theft = identity theft
- No rotation mechanism defined

### PIN Security Flaws

From the spec:
> "The first communication partner after factory default that sends the SHIP node PIN MAY gain a higher second factor trust level of 32"

**Problems:**
1. **No nonce/challenge** - PIN can be replayed
2. **Brute-forceable** - Only 8-16 hex digits (32-64 bits entropy)
3. **First-wins** - Race condition for who gets "32" trust
4. **One-time advantage** - Factory reset required to change
5. **Trust level 16 vs 32** - Unclear practical difference

### Auto-Accept Window

> "duration for the auto accept time window... MUST be lower than or equal to 2 minutes"

**Problem:** 2 minutes is an eternity for MITM attacks in local networks.

### No Certificate Lifecycle

SHIP doesn't specify:
- Certificate validity period
- Rotation procedure
- Revocation mechanism
- What happens when certificate expires

---

## Matter Commissioning Model - What's Right

### Certificate Chain (Device Attestation)

```
┌─────────────────────────────────────────────────────────┐
│                    Trust Hierarchy                       │
├─────────────────────────────────────────────────────────┤
│  PAA (Product Attestation Authority)                    │
│    └── PAI (Product Attestation Intermediate)           │
│          └── DAC (Device Attestation Certificate)       │
│                └── Device with Attestation Key Pair     │
└─────────────────────────────────────────────────────────┘
```

**Benefits:**
- Manufacturer-signed chain proves device authenticity
- DAC contains Vendor ID + Product ID
- Verifiable against Distributed Compliance Ledger (DCL)
- Can revoke compromised certificates

### Two-Phase Security (PASE → CASE)

**Phase 1: PASE (Password Authenticated Session Establishment)**
```
Commissioner                    Device
     │                            │
     │◄── BLE/WiFi Advertisement ─┤  (with Discriminator)
     │                            │
     │── SPAKE2+ with Passcode ──►│  (from QR code)
     │                            │
     │◄── Secure PASE Session ───►│
     │                            │
     │── Request Attestation ────►│
     │◄── DAC + PAI + Signature ──┤
     │                            │
     │── Verify against DCL ─────►│  (optional)
     │                            │
     │── Install NOC ────────────►│  (Node Operational Cert)
     │                            │
```

**Phase 2: CASE (Certificate Authenticated Session Establishment)**
```
Controller                      Device
     │                            │
     │── mDNS Discovery ─────────►│
     │                            │
     │── CASE with NOC ──────────►│  (mutual cert auth)
     │◄── Verify same Fabric ─────┤
     │                            │
     │◄── Operational Session ───►│
```

### Setup Codes

Matter uses 11-digit numeric passcode + 4-digit discriminator:
- **Passcode**: 27 bits of entropy (8 digits effective)
- **Discriminator**: Helps find the right device
- **QR Code**: Contains passcode + discriminator + vendor info

**Security:**
- SPAKE2+ is a PAKE (Password Authenticated Key Exchange)
- Resistant to offline dictionary attacks
- No PIN transmitted in clear
- Mutual authentication

### Node Operational Certificates (NOC)

After commissioning:
- Device gets a **Fabric-specific identity**
- NOC is signed by Fabric's root CA
- Used for all future CASE sessions
- Can be rotated by administrator

### Certificate Lifecycle

| Event | Mechanism |
|-------|-----------|
| Initial | DAC pre-installed by manufacturer |
| Commissioning | NOC installed by commissioner |
| Rotation | UpdateNOC command |
| Revocation | RemoveFabric command |
| Expiry | NOC has validity period |

---

## Design Options for MASH

### Option A: Simplified Matter-style (Recommended)

**Certificate Hierarchy:**
```
┌─────────────────────────────────────────────┐
│  Manufacturer CA (optional)                 │
│    └── Device Certificate (pre-installed)   │
│                                             │
│  Fabric CA (controller-generated)           │
│    └── Operational Certificate (on pairing) │
└─────────────────────────────────────────────┘
```

**Commissioning Flow:**
1. Device has pre-installed device certificate (or self-signed with attestation data)
2. User scans QR code containing setup code
3. PAKE (SPAKE2+ or similar) establishes secure channel
4. Controller verifies device identity/attestation
5. Controller issues Operational Certificate
6. Future sessions use CASE-like mutual TLS

**Pros:**
- Proven security model
- Clear certificate lifecycle
- No trust level confusion
- Attestation possible (but optional)

**Cons:**
- Requires PKI infrastructure for operational certs
- More complex than basic TLS

### Option B: Simplified Self-Signed with Strong Pairing

**Certificate Model:**
```
┌─────────────────────────────────────────────┐
│  Device: Self-signed certificate            │
│    - Unique key pair per device             │
│    - Contains device info (vendor, model)   │
│    - Long validity (10 years)               │
│                                             │
│  Trust: Established via secure pairing      │
│    - PAKE with setup code                   │
│    - Fingerprint stored after pairing       │
└─────────────────────────────────────────────┘
```

**Commissioning Flow:**
1. Device generates self-signed cert at factory/first boot
2. User scans QR code with setup code + cert fingerprint
3. PAKE establishes secure channel
4. Controller verifies cert fingerprint matches QR
5. Controller stores (device_id → fingerprint) as trusted
6. Future sessions verify fingerprint matches stored value

**Pros:**
- Simpler - no PKI needed
- Similar to SSH "trust on first use" but with verification
- Works for small manufacturers without CA infrastructure

**Cons:**
- No attestation chain
- Certificate rotation more complex
- Fingerprint = single point of failure

### Option C: Hybrid - Optional Attestation

Support both:
- **Attested devices**: Full certificate chain (like Matter)
- **Unattested devices**: Self-signed with strong pairing (like Option B)

Controller policy determines what's acceptable.

---

## Key Design Decisions Needed

### 1. Certificate Hierarchy

| Question | Options |
|----------|---------|
| Require manufacturer CA? | Yes (like Matter) / No (self-signed OK) / Optional |
| Operational certificates? | Yes (controller issues) / No (device cert only) |
| Attestation verification? | Required / Optional / Not supported |

### 2. Pairing Protocol

| Question | Options |
|----------|---------|
| PAKE algorithm? | SPAKE2+ (Matter) / SRP / OPAQUE |
| Setup code format? | Numeric (Matter-style) / Alphanumeric / UUID |
| Setup code entropy? | 20 bits (6 digits) / 27 bits (8 digits) / more |

### 3. Certificate Lifecycle

| Question | Options |
|----------|---------|
| Validity period? | 1 year / 5 years / 10 years / no expiry |
| Rotation mechanism? | Controller-initiated / Device-initiated / Manual |
| Revocation? | CRL / OCSP / Fabric removal / Not supported |

### 4. Trust Model

| Question | Options |
|----------|---------|
| Trust levels? | Binary (paired/not) / Priority-based / Role-based |
| Multiple controllers? | Allowed (multi-fabric) / Single controller only |
| Takeover? | Priority-based / User-confirmed / Not allowed |

---

## Recommendation for MASH

### Primary Model: Option A (Simplified Matter-style)

**Rationale:**
1. Proven security in Matter ecosystem
2. Clear lifecycle management
3. Supports both attested and unattested devices
4. Enables multi-controller (multi-fabric) scenarios

**Simplifications vs Matter:**
1. No DCL requirement - attestation optional
2. Simpler setup code - 8 digits sufficient
3. No BLE requirement - mDNS + direct connection OK
4. Single operational cert format (not Matter's complex TLV)

### Proposed Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    MASH Commissioning                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. DISCOVERY                                                │
│     Device advertises via mDNS                               │
│     - Service type: _mash._tcp                               │
│     - TXT: discriminator, vendor, product                    │
│                                                              │
│  2. PAIRING (PASE-like)                                      │
│     Controller connects to device                            │
│     SPAKE2+ with setup code from QR                          │
│     Establishes encrypted session                            │
│                                                              │
│  3. ATTESTATION (optional)                                   │
│     Controller requests device certificate                   │
│     If manufacturer CA present: verify chain                 │
│     If self-signed: verify fingerprint matches QR            │
│                                                              │
│  4. OPERATIONAL CERT INSTALLATION                            │
│     Device generates operational key pair                    │
│     Controller signs CSR with fabric CA                      │
│     Device stores operational cert                           │
│                                                              │
│  5. OPERATIONAL (CASE-like)                                  │
│     Future connections use mutual TLS                        │
│     Both sides present operational certs                     │
│     Verify same fabric (same CA)                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Certificate Validity

| Certificate Type | Validity | Rotation |
|-----------------|----------|----------|
| Device/Attestation Cert | 20 years (device lifetime) | Not rotated |
| Operational Cert | 1 year | Auto-renewed by controller |
| Fabric Root CA | 10 years | Manual, coordinated |

---

## References

- [Matter Commissioning (Google)](https://developers.home.google.com/matter/primer/commissioning)
- [Matter Commissioning (Silicon Labs)](https://docs.silabs.com/matter/latest/matter-overview-guides/matter-commissioning)
- [Matter Attestation (Google)](https://developers.home.google.com/matter/primer/attestation)
- [Matter Security (Silicon Labs)](https://docs.silabs.com/matter/latest/matter-fundamentals-security/)
- [Building a .NET Matter Controller - Commissioning](https://tomasmcguinness.com/2025/04/30/building-a-net-matter-controller-commissioning-flow-case-pt1/)
