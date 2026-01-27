# MASH vs Matter: Certificate Model Comparison

> Comparing PKI architectures and design trade-offs

**Status:** Reference
**Last Updated:** 2025-01-27

---

## Overview

Matter uses a dual-PKI model separating device attestation from operational communication. MASH uses only operational certificates, intentionally omitting device attestation. This document compares the approaches and explains MASH's design choices.

See also: [Device Attestation Considerations](device-attestation-considerations.md)

---

## Certificate Hierarchies

### Matter's Hierarchy

```
ATTESTATION (CSA-controlled):              OPERATIONAL (per fabric):
┌────────────────────────────────┐         ┌────────────────────────────────┐
│ PAA (CSA-approved root)        │         │ RCAC (fabric root, self-signed)│
│   └─► PAI (per product)        │         │   └─► ICAC (intermediate, opt) │
│         └─► DAC (per device)   │         │         └─► NOC (per node)     │
└────────────────────────────────┘         └────────────────────────────────┘
       Manufacturing                              Commissioning
```

### MASH's Hierarchy

```
OPERATIONAL (per zone):
┌────────────────────────────────┐
│ Zone CA (self-signed)          │
│   ├─► Controller Op Cert       │
│   └─► Device Op Certs          │
└────────────────────────────────┘
       Commissioning

(No attestation hierarchy - see device-attestation-considerations.md)
```

---

## Key Concepts Mapping

| Concept | Matter | MASH |
|---------|--------|------|
| Global trust anchor | PAA (CSA-controlled) | None |
| Product-level CA | PAI | None |
| Device identity cert | DAC | None (not included) |
| Operational root | RCAC | Zone CA |
| Operational intermediate | ICAC (optional) | None |
| Node/device cert | NOC | Operational Cert |
| Multi-controller | Multi-fabric (separate RCAC each) | Multi-zone (separate Zone CA each) |
| Trust registry | DCL (blockchain) | None |

---

## Certificate Lifetimes

| Certificate | Matter | MASH |
|-------------|--------|------|
| Root CA (PAA / Zone CA) | ~20 years | 20 years |
| Intermediate (PAI / ICAC) | ~20 years | N/A |
| Device Attestation (DAC) | ~20 years | N/A (not included) |
| Operational (NOC / Op Cert) | **~20 years** | **1 year** |

### Why MASH Uses Short-Lived Operational Certificates

MASH deliberately uses 1-year operational certificates with auto-renewal:

| Benefit | Explanation |
|---------|-------------|
| Cryptographic agility | Rotate keys annually without user intervention |
| Limited compromise window | Stolen key useful for max 1 year |
| No CRL/OCSP needed | Short lifetime = natural expiration handles revocation |
| Fresh keys | Each renewal generates new key pair |

### Matter's Approach

Matter uses long-lived operational certificates (~20 years) with manual `UpdateNOC` command:

- No auto-renewal mechanism
- `UpdateNOC` typically used for fabric migration or admin changes
- Long-lived certs mean compromise has longer impact
- Requires explicit revocation infrastructure or fabric reset

---

## Two Independent PKI Trees

Both protocols use **completely separate** PKI hierarchies for attestation vs operational trust:

```
Attestation:  "Is this device genuine?"     → Verified during commissioning
Operational:  "Does this device belong?"    → Used for all runtime communication
```

The operational root (RCAC/Zone CA) is **NOT derived** from attestation certificates. The flow:

1. Commissioner verifies device's attestation chain (DAC → PAI → PAA)
2. Commissioner creates its own operational root (RCAC/Zone CA) independently
3. Commissioner issues operational certificate (NOC/Op Cert) to the device

---

## Who Creates What

| Entity | Creates (Matter) | Creates (MASH) |
|--------|------------------|----------------|
| CSA | PAA approval, DCL | N/A |
| Manufacturer | DAC, PAI | Device Attestation (optional) |
| Controller/EMS | RCAC, ICAC, NOC | Zone CA, Operational Certs |
| Device | CSR only | CSR only |

---

## Attestation: Mandatory vs Optional

### Matter: Mandatory Attestation

- All devices MUST have DAC signed by CSA-approved PAA
- Verification via Distributed Compliance Ledger (DCL)
- Certification program required to get PAI signed
- Provides supply chain security and counterfeit prevention

### MASH: Optional Attestation

- Devices MAY have manufacturer-signed attestation
- Self-signed attestation allowed for hobbyist/DIY devices
- No central authority or certification program
- Physical commissioning (QR code) provides sufficient trust

### Why MASH Makes Attestation Optional

| Factor | Matter Context | MASH Context |
|--------|----------------|--------------|
| Scale | Billions of consumer IoT devices | Thousands of energy devices |
| Purchase channel | Retail (Amazon, etc.) | Professional installation |
| Counterfeit risk | High (cheap consumer goods) | Low (expensive energy equipment) |
| Certification body | CSA exists, has resources | No equivalent for energy sector |
| Physical access | Often remote setup | Installer on-site with QR code |

---

## Complexity Comparison

| Aspect | Matter | MASH |
|--------|--------|------|
| Certificate types | 6 (PAA, PAI, DAC, RCAC, ICAC, NOC) | 4 (Mfr CA, Attestation, Zone CA, Op Cert) |
| Chain depth (attestation) | 3 levels | 1-2 levels |
| Chain depth (operational) | 2-3 levels | 2 levels |
| Intermediate CAs | Required (PAI), Optional (ICAC) | None |
| Certificate format | X.509 DER + Matter TLV | X.509 DER only |
| Global infrastructure | DCL blockchain | None |
| Revocation | DCL + fabric reset | Zone removal + natural expiry |

---

## Renewal Mechanisms

### Matter: UpdateNOC Command

```
Controller                              Device
    │                                     │
    │── CSRRequest ─────────────────────►│
    │◄── CSRResponse (new public key) ───┤
    │                                     │
    │── UpdateNOC (new NOC) ────────────►│
    │◄── NOCResponse ────────────────────┤
```

- Manual process, no auto-renewal
- Typically used for admin changes, not routine renewal
- Can change node ID during update
- Requires fail-safe context (atomic commit)

### MASH: Automatic Certificate Renewal

```
Controller                              Device
    │                                     │
    │── CertRenewalRequest (nonce) ─────►│
    │◄── CertRenewalCSR (new key pair) ──┤
    │                                     │
    │── CertRenewalInstall (new cert) ──►│
    │◄── CertRenewalAck ─────────────────┤
```

- Automatic, controller-initiated
- 30 days before expiry
- In-session (no TLS reconnection)
- Device generates fresh key pair each time

---

## Trust Model Summary

```
Matter:
┌─────────────────────────────────────────────────────────────────┐
│  CSA (Connectivity Standards Alliance)                          │
│    ├─► Approves PAAs (manufacturers)                            │
│    ├─► Operates DCL (trust registry)                            │
│    └─► Certifies products                                       │
│                                                                 │
│  Commissioner/Controller                                        │
│    ├─► Verifies DAC chain via DCL                              │
│    ├─► Creates RCAC (fabric root)                              │
│    └─► Issues NOCs to devices                                  │
└─────────────────────────────────────────────────────────────────┘

MASH:
┌─────────────────────────────────────────────────────────────────┐
│  No central authority                                           │
│                                                                 │
│  Controller/EMS                                                 │
│    ├─► Optionally verifies attestation (if manufacturer CA)    │
│    ├─► Creates Zone CA (zone root)                             │
│    ├─► Issues operational certs to devices                     │
│    └─► Auto-renews certificates before expiry                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Design Rationale

MASH's simpler model is appropriate for its target domain:

1. **Energy devices are expensive** - Less counterfeit risk than cheap IoT gadgets
2. **Professional installation** - Installer physically present, can verify device
3. **No CSA equivalent** - No central authority exists for energy sector
4. **Operational security via short-lived certs** - 1-year operational certs with auto-renewal provides better security than 20-year certs with manual renewal
5. **Resource constraints** - Targeting 256KB RAM MCUs, simpler PKI reduces code size

Matter's complex model is appropriate for consumer IoT:

1. **Retail purchase** - Users buy devices sight-unseen from Amazon
2. **Counterfeit risk** - Cheap devices easily counterfeited
3. **CSA infrastructure** - Organization exists to operate DCL and certification
4. **Interoperability** - Many vendors, need certification for compatibility
5. **Brand protection** - Manufacturers want to prevent fake products

---

## References

- [Matter Specification](https://csa-iot.org/developer-resource/specifications-download-request/) - Device Attestation, Operational Credentials
- [MASH Security Model](security.md) - Certificates, commissioning, zones
- [connectedhomeip SDK](https://github.com/project-chip/connectedhomeip) - Matter reference implementation
