# Device Attestation: Considerations and Trade-offs

> Why MASH does not include device attestation, and what would be needed to add it

**Status:** Design Note
**Last Updated:** 2025-01-27

---

## Overview

Device attestation is a mechanism where devices prove their authenticity using manufacturer-issued certificates. Matter requires this; MASH does not. This document explains the reasoning and trade-offs.

---

## What Is Device Attestation?

Device attestation uses a PKI hierarchy to prove a device is genuine:

```
Manufacturer Root CA (or industry authority like CSA)
  └─► Product CA (per product line)
        └─► Device Attestation Certificate (unique per device, pre-installed)
```

During commissioning, the device presents its attestation certificate chain. The controller verifies:
1. The chain is valid (signatures check out)
2. The root CA is trusted (known manufacturer or industry authority)
3. The device wasn't revoked

If verification passes, the controller has cryptographic proof the device is a genuine product from a known manufacturer.

---

## Why Matter Requires Device Attestation

Matter's context makes attestation valuable:

| Factor | Matter Reality |
|--------|----------------|
| **Purchase channel** | Consumers buy from Amazon, retail stores |
| **Physical access** | Often set up remotely, device shipped directly |
| **Counterfeit risk** | Cheap consumer IoT devices easily counterfeited |
| **Brand protection** | Manufacturers want to prevent fake products |
| **Liability** | Fake devices could be fire/safety hazards |
| **Central authority** | CSA exists to operate trust infrastructure (DCL) |

For Matter, attestation answers: "Is this device I bought online actually a genuine Philips Hue bulb, or a counterfeit that might catch fire?"

---

## Why MASH Does Not Include Device Attestation

### 1. Different Threat Model

MASH targets energy management devices (EV chargers, inverters, batteries, heat pumps):

| Factor | MASH Reality |
|--------|--------------|
| **Purchase channel** | Professional installation, utility programs |
| **Physical access** | Installer on-site, can visually verify device |
| **Counterfeit risk** | Low - expensive equipment, regulated market |
| **Device cost** | High - counterfeiting less economically attractive |
| **Installation** | Requires electrician, permits, inspections |

When an electrician installs a 22kW EV charger, they verify the device visually, check certifications, and the installation is inspected. Cryptographic attestation adds little value.

### 2. No Central Authority

Matter has the Connectivity Standards Alliance (CSA) operating:
- Distributed Compliance Ledger (DCL) - blockchain for trust anchors
- Certification program - devices must be certified
- PAA approval process - manufacturers must be vetted

The energy sector has no equivalent:
- No single industry body for EV chargers, inverters, heat pumps
- Multiple regional standards (EU, US, Asia)
- No existing PKI infrastructure for device identity

Building this infrastructure would require industry coordination that doesn't exist today.

### 3. Manufacturer Readiness

Device attestation requires manufacturers to:

| Requirement | Challenge |
|-------------|-----------|
| Generate unique key pairs per device | Secure key generation in factory |
| Store private keys securely | Hardware security module or secure element |
| Sign certificates during manufacturing | PKI integration into production line |
| Manage certificate lifecycle | Revocation infrastructure |
| Protect root CA | Offline HSM, strict access controls |

Many energy device manufacturers, especially smaller ones, lack the PKI expertise and infrastructure to implement this securely. Poorly implemented attestation (weak keys, leaked CAs) is worse than no attestation.

### 4. Cost-Benefit Analysis

| Cost | Benefit |
|------|---------|
| Manufacturing complexity | Prevents commissioning fake devices |
| Per-device secure element (~$1-5) | Proves device provenance |
| PKI infrastructure investment | Enables remote attestation |
| Ongoing CA management | Supply chain security |

For energy devices with professional installation, the costs outweigh the benefits. The installer's physical presence provides equivalent assurance at lower complexity.

### 5. MASH's Security Model Works Without It

MASH provides security through:

| Mechanism | Security Property |
|-----------|-------------------|
| **SPAKE2+ commissioning** | Requires physical access (QR code) |
| **Zone CA** | Controller is root of trust for its zone |
| **Short-lived operational certs** | 1-year validity, auto-renewed |
| **Mutual TLS** | All communication encrypted and authenticated |

An attacker would need:
1. Physical access to scan the QR code
2. To be present during the commissioning window
3. The setup code (8 digits, ~27 bits entropy)

This is sufficient for the target deployment scenarios.

---

## What Would Be Needed to Add Attestation

If the energy device ecosystem matures and attestation becomes valuable, MASH would need:

### Industry Infrastructure

1. **Trust anchor authority** - Organization to approve manufacturer CAs
2. **Compliance ledger** - Registry of approved CAs and revocations
3. **Certification program** - Process for manufacturers to get approved
4. **Revocation infrastructure** - Way to invalidate compromised devices

### Manufacturer Requirements

1. **Secure manufacturing** - Key generation in controlled environment
2. **Hardware security** - Secure element or TPM for key storage
3. **PKI operations** - Certificate issuance and management
4. **Audit compliance** - Proving security practices

### Protocol Changes

1. **Attestation message** - Device sends certificate chain during commissioning
2. **Verification logic** - Controller checks chain against trust store
3. **Trust store management** - Way to update approved root CAs
4. **Revocation checking** - Query ledger for revoked certificates

### Open Questions

| Question | Considerations |
|----------|---------------|
| Who operates the trust anchor? | Industry consortium? Regional authorities? |
| How are manufacturers vetted? | Certification requirements? Audits? |
| What's the cost model? | Per-device fees? Annual membership? |
| How to handle legacy devices? | Grandfather existing deployments? |
| Regional fragmentation? | EU vs US vs Asia requirements? |

---

## Comparison: With vs Without Attestation

### Without Attestation (Current MASH)

```
Commissioning:
  User scans QR code → SPAKE2+ → Zone CA issues operational cert

Trust basis:
  Physical access + setup code + Zone CA

Attack surface:
  Must have physical access during commissioning window
```

### With Attestation (Hypothetical)

```
Commissioning:
  User scans QR code → Device proves attestation → SPAKE2+ → Zone CA issues operational cert

Trust basis:
  Manufacturer CA + physical access + setup code + Zone CA

Attack surface:
  Must have genuine device + physical access during commissioning window
```

The additional security from attestation:
- Prevents commissioning counterfeit/malicious hardware
- Enables remote/delegated commissioning (no physical access needed)
- Provides audit trail of device provenance

---

## Recommendations

### For Now

MASH should **not include device attestation** because:
1. The target ecosystem lacks necessary infrastructure
2. Manufacturers aren't ready to implement it properly
3. Physical installation provides equivalent assurance
4. Added complexity without proportional benefit

### For the Future

Consider adding attestation when:
1. An industry body emerges to operate trust infrastructure
2. Manufacturers commonly include secure elements
3. Remote/delegated commissioning becomes important
4. Counterfeit energy devices become a real problem

### Design Principle

MASH follows the principle: **don't specify what can't be implemented well**.

Poorly implemented attestation (self-signed, leaked keys, no revocation) provides false assurance. Better to be honest that MASH relies on physical commissioning than to include security theater.

---

## References

- [Matter Device Attestation](https://csa-iot.org/) - CSA specification for attestation
- [MASH Security Model](security.md) - How MASH provides security without attestation
- [Matter Comparison](matter-comparison.md) - Full comparison of MASH vs Matter PKI
