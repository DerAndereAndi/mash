# Controller Operational Certificate - TDD Implementation Plan

## Overview

Add controller operational certificate support to MASH, aligning with Matter's design where controllers are full peers with their own identity certificates, separate from their CA signing capability.

**Current State:**
- Controller has Zone CA (for signing device certs)
- Controller uses `InsecureSkipVerify: true` without presenting a client certificate
- No mutual TLS authentication for operational connections

**Target State:**
- Controller has Zone CA (for signing) + Controller Operational Cert (for identity)
- Controller presents operational cert during TLS connections
- Devices verify controller cert is signed by same Zone CA
- `cert` command shows all certificate information

---

## Architecture

```
Controller Certificate Architecture:
┌─────────────────────────────────────────────────────────┐
│  Zone CA (self-signed, 10 years)                        │
│    ├── Controller Operational Cert (1 year)             │
│    └── Device Operational Certs (1 year each)           │
└─────────────────────────────────────────────────────────┘

Storage Layout:
~/.mash-controller/
├── identity/
│   ├── zone-ca.pem          # Zone CA certificate
│   ├── zone-ca.key          # Zone CA private key
│   ├── zone-ca.json         # Zone CA metadata
│   ├── controller.pem       # Controller operational cert  [NEW]
│   └── controller.key       # Controller operational key   [NEW]
└── controller/
    └── devices/
        └── <device-id>/
            └── info.json
```

---

## Phase 0: Specification Updates

### 0.1 Update docs/security.md

Update the certificate hierarchy diagram and add controller operational certificate:

```markdown
## 2. Certificate Hierarchy

┌─────────────────────────────────────────────────────────┐
│  Manufacturer CA (optional)                             │
│    └── Device Attestation Cert (20yr, pre-installed)    │
│                                                         │
│  Zone CA (controller-generated, 10yr)                   │
│    ├── Controller Operational Cert (1yr)     [NEW]      │
│    └── Device Operational Cert (1yr, issued during pairing)│
└─────────────────────────────────────────────────────────┘

### 2.4 Controller Operational Certificate [NEW SECTION]

- **Purpose:** Proves controller identity to devices during operational TLS
- **Issuer:** Zone CA (self-issued by controller)
- **Validity:** 1 year (auto-renewed)
- **Generated:** When Zone CA is created
- **Key Usage:** DigitalSignature, KeyEncipherment
- **Extended Key Usage:** ClientAuth, ServerAuth
```

- [ ] Spec updated
- [ ] Decision logged in decision-log.md

### 0.2 Update docs/testing/behavior/connection-establishment.md

Add section on controller certificate in operational TLS:

```markdown
## 6. Operational TLS Mutual Authentication [NEW SECTION]

### 6.1 Controller Certificate Requirements

Controllers MUST present an operational certificate during operational TLS connections:

| Field | Requirement |
|-------|-------------|
| Issuer | Zone CA |
| Subject CN | Controller ID (fingerprint-derived) |
| Subject O | "MASH Controller" |
| Subject OU | Zone Type, Zone ID |
| Key Usage | digitalSignature, keyEncipherment |
| Extended Key Usage | clientAuth, serverAuth |
| Basic Constraints | CA:FALSE |
| Validity | ≤ 1 year |

### 6.2 Validation by Device

When a controller connects, the device MUST verify:
1. Certificate is not expired
2. Certificate is signed by the Zone CA for this zone
3. AuthorityKeyId matches Zone CA's SubjectKeyId
4. Key usage includes digitalSignature

### 6.3 Certificate Not Used During Commissioning

During PASE commissioning, the controller does NOT present its operational certificate.
Security is provided by SPAKE2+. The device does not yet have the Zone CA to verify against.
```

- [ ] Spec updated

### 0.3 Update docs/testing/behavior/tls-profile.md

Add test cases for controller certificate:

```markdown
### TC-TLS-CTRL-*: Controller Certificate [NEW SECTION]

| ID | Description | Expected |
|----|-------------|----------|
| TC-TLS-CTRL-1 | Controller presents operational cert | TLS handshake includes client certificate |
| TC-TLS-CTRL-2 | Controller cert signed by Zone CA | Device verifies signature succeeds |
| TC-TLS-CTRL-3 | Controller cert expired | Device rejects, TLS alert |
| TC-TLS-CTRL-4 | Controller cert wrong Zone CA | Device rejects, TLS alert |
| TC-TLS-CTRL-5 | Controller cert missing clientAuth | Device rejects, TLS alert |
| TC-TLS-CTRL-6 | Controller cert is CA:TRUE | Device rejects (must be end-entity) |
| TC-TLS-CTRL-7 | Commissioning without controller cert | Handshake succeeds (InsecureSkipVerify) |
```

- [ ] Spec test cases added

### 0.4 Update docs/testing/behavior/zone-lifecycle.md

Add test cases for controller certificate lifecycle:

```markdown
### TC-CTRL-CERT-*: Controller Certificate Lifecycle [NEW SECTION]

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-CTRL-CERT-1 | Auto-generate on zone creation | New controller, no existing cert | Controller cert generated with Zone CA |
| TC-CTRL-CERT-2 | Load existing on restart | Controller restarts | Same controller cert used |
| TC-CTRL-CERT-3 | Renewal within window | Cert expires in 25 days | New cert generated, old replaced |
| TC-CTRL-CERT-4 | Renewal preserves zone membership | Active device connections | Connections continue (session keys unchanged) |
| TC-CTRL-CERT-5 | Controller ID consistency | Multiple cert generations | Controller ID (SKI) remains same |
```

- [ ] Spec test cases added

### 0.5 Add Decision Log Entry

Add to docs/decision-log.md:

```markdown
## DEC-XXX: Controller Operational Certificate

**Date:** 2025-01-27
**Status:** Accepted

### Context

MASH controllers currently use InsecureSkipVerify for operational TLS connections,
providing no mutual authentication. The spec (security.md section 5) states
"Both sides present operational certificates" but this was not implemented.

Matter controllers have their own Node Operational Certificate (NOC) separate from
their CA signing capability, allowing devices to verify controller identity.

### Decision

Controllers SHALL have an operational certificate, signed by their own Zone CA:

1. Generated automatically when Zone CA is created
2. Stored alongside Zone CA in identity/ directory
3. Used for all operational TLS connections (not commissioning)
4. Renewed automatically within 30-day window before expiry
5. Same validity period as device operational certs (1 year)

### Consequences

- Devices can verify controller is legitimate zone owner
- True mutual TLS authentication achieved
- Controller identity is stable (same SKI across renewals)
- Additional certificate to manage (storage, renewal)

### References

- Matter CHIPDeviceController.h:103-105 (controllerNOC)
- Matter CASESession.cpp (Sigma3 with controller NOC)
```

- [ ] Decision logged

---

## Phase 1: Specification Test Cases (docs/testing/)

These test cases define the protocol requirements. Implementation tests verify the code meets these specs.

### TC-TLS-CTRL-*: Controller Certificate (tls-profile.md)

| ID | Description | Expected |
|----|-------------|----------|
| TC-TLS-CTRL-1 | Controller presents operational cert | TLS handshake includes client certificate |
| TC-TLS-CTRL-2 | Controller cert signed by Zone CA | Device verifies signature succeeds |
| TC-TLS-CTRL-3 | Controller cert expired | Device rejects, TLS alert bad_certificate |
| TC-TLS-CTRL-4 | Controller cert wrong Zone CA | Device rejects, TLS alert unknown_ca |
| TC-TLS-CTRL-5 | Controller cert missing clientAuth EKU | Device rejects, TLS alert |
| TC-TLS-CTRL-6 | Controller cert is CA:TRUE | Device rejects (must be end-entity) |
| TC-TLS-CTRL-7 | Commissioning without controller cert | Handshake succeeds (PASE security) |
| TC-TLS-CTRL-8 | Controller cert before notBefore | Device rejects, TLS alert |

- [x] Added to tls-profile.md

### TC-CTRL-CERT-*: Controller Certificate Lifecycle (zone-lifecycle.md)

| ID | Description | Setup | Expected |
|----|-------------|-------|----------|
| TC-CTRL-CERT-1 | Auto-generate on zone creation | New controller | Controller cert generated with Zone CA |
| TC-CTRL-CERT-2 | Load existing on restart | Existing cert | Same controller cert loaded |
| TC-CTRL-CERT-3 | Renewal triggers at 30 days | Cert expires in 25 days | New cert generated |
| TC-CTRL-CERT-4 | Renewal does not disrupt sessions | Active connections | Sessions continue |
| TC-CTRL-CERT-5 | Controller ID stable across renewal | Renew cert | Same SKI/controller ID |
| TC-CTRL-CERT-6 | Cert matches Zone CA | Zone CA rotated | Old controller cert invalid |

- [x] Added to zone-lifecycle.md

### TC-CERT-VAL-CTRL-*: Controller Certificate Validation (connection-establishment.md)

| ID | Description | Expected |
|----|-------------|----------|
| TC-CERT-VAL-CTRL-1 | Valid controller cert | Device accepts, session established |
| TC-CERT-VAL-CTRL-2 | Expired controller cert | Device rejects |
| TC-CERT-VAL-CTRL-3 | Wrong Zone CA issuer | Device rejects |
| TC-CERT-VAL-CTRL-4 | Self-signed controller cert | Device rejects (must chain to Zone CA) |
| TC-CERT-VAL-CTRL-5 | Clock skew 200s | Device accepts (within tolerance) |
| TC-CERT-VAL-CTRL-6 | Clock skew 400s | Device rejects (exceeds tolerance) |

- [x] Added to connection-establishment.md

---

## Phase 2: Certificate Generation (Implementation)

### Implementation Test Cases

#### TC-IMPL-CERT-GEN-001: Generate Controller Operational Certificate
```
Given: A valid Zone CA exists
When:  GenerateControllerOperationalCert(zoneCA, controllerID) is called
Then:  Returns OperationalCert with:
       - Certificate signed by Zone CA
       - Subject CN contains controller ID
       - KeyUsage includes DigitalSignature, KeyEncipherment
       - ExtKeyUsage includes ClientAuth, ServerAuth
       - Validity is 1 year (OperationalCertValidity)
       - IsCA is false
       - AuthorityKeyId matches Zone CA's SubjectKeyId
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-GEN-002: Controller Cert Subject Contains Zone Info
```
Given: Zone CA with ZoneID="home-ems" and ZoneType=HOME_MANAGER
When:  GenerateControllerOperationalCert is called
Then:  Certificate Subject includes:
       - CN: controller ID (e.g., "controller-<fingerprint>")
       - O: "MASH Controller"
       - OU: ["HOME_MANAGER", "home-ems"]
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-GEN-003: Controller Cert Uses Fresh Key Pair
```
Given: Zone CA exists
When:  GenerateControllerOperationalCert is called twice
Then:  Each call generates a different key pair
       - Different SubjectKeyId values
       - Different serial numbers
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-GEN-004: Controller Cert Verification
```
Given: Controller operational cert generated from Zone CA
When:  Certificate chain is verified
Then:  Verification succeeds with Zone CA as trust anchor
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

---

## Phase 3: Certificate Storage (Implementation)

#### TC-IMPL-CERT-STORE-001: Save Controller Operational Cert
```
Given: FileControllerStore with Zone CA
When:  SetControllerCert(operationalCert) is called
       And Save() is called
Then:  Files created:
       - identity/controller.pem (certificate)
       - identity/controller.key (private key)
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-STORE-002: Load Controller Operational Cert
```
Given: Controller cert files exist on disk
When:  FileControllerStore.Load() is called
Then:  GetControllerCert() returns valid OperationalCert
       With matching certificate and private key
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-STORE-003: Controller Cert Not Found
```
Given: FileControllerStore with no controller cert saved
When:  GetControllerCert() is called
Then:  Returns ErrCertNotFound
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CERT-STORE-004: Auto-Generate Controller Cert on Start
```
Given: Zone CA exists but no controller cert
When:  ControllerService.Start() is called
Then:  Controller operational cert is auto-generated
       And saved to cert store
```
- [ ] Test implemented
- [ ] Test passing
- [ ] Code implemented

#### TC-IMPL-CERT-STORE-005: Load Existing Controller Cert on Start
```
Given: Zone CA and controller cert both exist
When:  ControllerService.Start() is called
Then:  Existing controller cert is loaded (not regenerated)
```
- [ ] Test implemented
- [ ] Test passing
- [ ] Code implemented

---

## Phase 4: TLS Configuration (Implementation)

#### TC-IMPL-TLS-001: Controller TLS Config Uses Operational Cert
```
Given: Controller with operational cert
When:  Building TLS config for device connection
Then:  tls.Config.Certificates contains controller operational cert
       Not the Zone CA
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-TLS-002: Controller TLS Config Includes Zone CA for Verification
```
Given: Controller with operational cert
When:  Building TLS config for device connection
Then:  tls.Config.RootCAs contains Zone CA
       For verifying device certificates
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-TLS-003: Mutual TLS Handshake Succeeds
```
Given: Controller with operational cert
       Device with operational cert (same Zone CA)
When:  Controller connects to device
Then:  TLS handshake succeeds
       Both sides verify peer certificate
       ConnectionState.PeerCertificates is populated
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-TLS-004: Mutual TLS Rejects Wrong Zone
```
Given: Controller with cert from Zone A
       Device with cert from Zone B
When:  Controller connects to device
Then:  TLS handshake fails
       Certificate verification error
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-TLS-005: Commissioning Still Uses Insecure TLS
```
Given: Controller with operational cert
When:  Commissioning a new device (PASE)
Then:  TLS uses InsecureSkipVerify (security from SPAKE2+)
       Controller does NOT present operational cert
       (Device doesn't have Zone CA yet)
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

---

## Phase 5: Certificate Inspection CLI (Implementation)

#### TC-IMPL-CLI-CERT-001: Show Controller Zone CA
```
Given: Controller with Zone CA
When:  User runs "cert" command
Then:  Output shows Zone CA info:
       - Zone ID
       - Zone Type
       - Subject CN
       - Validity period (NotBefore - NotAfter)
       - Days until expiry
       - Serial number
       - Is CA: true
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CLI-CERT-002: Show Controller Operational Cert
```
Given: Controller with operational cert
When:  User runs "cert" command
Then:  Output shows Controller Operational Cert info:
       - Subject CN (controller ID)
       - Issuer (Zone CA)
       - Validity period
       - Days until expiry
       - Serial number
       - Key type (ECDSA P-256)
       - Is CA: false
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CLI-CERT-003: Show Device Certificate
```
Given: Connected device with TLS session
When:  User runs "cert <device-id>" command
Then:  Output shows device's peer certificate:
       - Subject CN (device ID)
       - Issuer (Zone CA)
       - Validity period
       - Days until expiry
       - SHA256 fingerprint
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CLI-CERT-004: Show All Certificates
```
Given: Controller with Zone CA, operational cert, and 2 connected devices
When:  User runs "cert --all" command
Then:  Output shows summary table:
       - Zone CA (1 entry)
       - Controller cert (1 entry)
       - Device certs (2 entries)
       With columns: Type, Subject, Issuer, Expiry, Status
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CLI-CERT-005: Device Not Connected
```
Given: Known device that is not currently connected
When:  User runs "cert <device-id>" command
Then:  Output shows error: "Device not connected - certificate not available"
       (Cannot inspect cert without active TLS session)
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-CLI-CERT-006: Unknown Device
```
Given: Device ID that doesn't exist
When:  User runs "cert <unknown-id>" command
Then:  Output shows error: "Device not found: <unknown-id>"
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

---

## Phase 6: Controller Cert Renewal (Implementation)

#### TC-IMPL-RENEWAL-001: Controller Cert Needs Renewal Check
```
Given: Controller cert expiring in 25 days
When:  NeedsRenewal() is called
Then:  Returns true (within 30-day renewal window)
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

#### TC-IMPL-RENEWAL-002: Controller Cert Auto-Renewal
```
Given: Controller cert expiring in 25 days
When:  ControllerService periodic check runs
Then:  New controller cert is generated
       Signed by same Zone CA
       Old cert replaced
       No service interruption
```
- [x] Test implemented
- [x] Test passing
- [x] Code implemented

---

## Implementation Order

### Step 0: Specification Updates
- [ ] Update `docs/security.md` - Add section 2.4 Controller Operational Certificate
- [ ] Update `docs/testing/behavior/tls-profile.md` - Add TC-TLS-CTRL-* test cases
- [ ] Update `docs/testing/behavior/zone-lifecycle.md` - Add TC-CTRL-CERT-* test cases
- [ ] Update `docs/testing/behavior/connection-establishment.md` - Add TC-CERT-VAL-CTRL-* test cases
- [ ] Add decision to `docs/decision-log.md`

### Step 1: Certificate Generation (pkg/cert/generate.go)
- [ ] Add `GenerateControllerOperationalCert(zoneCA *ZoneCA, controllerID string) (*OperationalCert, error)`
- [ ] Reuse existing `OperationalCert` type (same as device certs)
- [ ] Tests: TC-IMPL-CERT-GEN-001 through TC-IMPL-CERT-GEN-004

### Step 2: Certificate Storage (pkg/cert/store_file_controller.go)
- [ ] Add `controllerCert *OperationalCert` field to `FileControllerStore`
- [ ] Add `GetControllerCert() (*OperationalCert, error)` method
- [ ] Add `SetControllerCert(cert *OperationalCert) error` method
- [ ] Update `Save()` to persist controller cert
- [ ] Update `Load()` to load controller cert
- [ ] Tests: TC-IMPL-CERT-STORE-001 through TC-IMPL-CERT-STORE-003

### Step 3: Service Integration (pkg/service/controller_service.go)
- [ ] Update `Start()` to generate/load controller operational cert
- [ ] Store controller cert alongside Zone CA
- [ ] Add `GetControllerCert() *OperationalCert` getter
- [ ] Tests: TC-IMPL-CERT-STORE-004, TC-IMPL-CERT-STORE-005

### Step 4: TLS Configuration (pkg/transport/tls.go, pkg/service/controller_service.go)
- [ ] Create `NewOperationalTLSConfig(controllerCert, zoneCA)` function
- [ ] Update `attemptReconnection()` to use operational TLS config
- [ ] Keep commissioning using `NewCommissioningTLSConfig()` (unchanged)
- [ ] Tests: TC-IMPL-TLS-001 through TC-IMPL-TLS-005

### Step 5: Certificate Access from Session (pkg/service/)
- [ ] Add `TLSConnectionState()` method to `framedConnection`
- [ ] Add `TLSConnectionState()` method to `DeviceSession`
- [ ] Expose via `ControllerService.GetDeviceTLSState(deviceID)`

### Step 6: CLI Command (cmd/mash-controller/interactive/)
- [ ] Create `cmd_cert.go` with `cmdCert()` function
- [ ] Implement certificate formatting helpers
- [ ] Register command in controller.go
- [ ] Tests: TC-IMPL-CLI-CERT-001 through TC-IMPL-CLI-CERT-006

### Step 7: Controller Cert Renewal (optional, can defer)
- [ ] Add renewal check to service startup
- [ ] Add periodic renewal check
- [ ] Tests: TC-IMPL-RENEWAL-001, TC-IMPL-RENEWAL-002

---

## Files to Create/Modify

### Specification Files

| File | Action | Description |
|------|--------|-------------|
| `docs/security.md` | Modify | Add section 2.4 Controller Operational Certificate |
| `docs/decision-log.md` | Modify | Add DEC-XXX for controller cert decision |
| `docs/testing/behavior/tls-profile.md` | Modify | Add TC-TLS-CTRL-* test cases |
| `docs/testing/behavior/zone-lifecycle.md` | Modify | Add TC-CTRL-CERT-* test cases |
| `docs/testing/behavior/connection-establishment.md` | Modify | Add TC-CERT-VAL-CTRL-* test cases |

### Implementation Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/cert/generate.go` | Modify | Add `GenerateControllerOperationalCert()` |
| `pkg/cert/generate_test.go` | Modify | Add tests for controller cert generation |
| `pkg/cert/store.go` | Modify | Add `GetControllerCert`/`SetControllerCert` to `ControllerStore` interface |
| `pkg/cert/store_file_controller.go` | Modify | Implement controller cert storage |
| `pkg/cert/store_file_controller_test.go` | Create | Tests for controller cert storage |
| `pkg/transport/tls.go` | Modify | Add `NewOperationalTLSConfig()` |
| `pkg/transport/tls_test.go` | Modify | Add tests for operational TLS config |
| `pkg/service/controller_service.go` | Modify | Generate/load controller cert on Start() |
| `pkg/service/controller_service_test.go` | Modify | Add tests for cert lifecycle |
| `pkg/service/framed_connection.go` | Modify | Add `TLSConnectionState()` method |
| `pkg/service/device_session.go` | Modify | Add `TLSConnectionState()` method |
| `cmd/mash-controller/interactive/cmd_cert.go` | Create | New `cert` command |
| `cmd/mash-controller/interactive/controller.go` | Modify | Register `cert` command |

---

## Progress Tracking

### Overall Status: COMPLETE

| Phase | Status | Tests Passing | Notes |
|-------|--------|---------------|-------|
| Phase 0: Spec Updates | [x] | N/A | docs/security.md updated with 2.4, 3.2 |
| Phase 1: Spec Test Cases | [x] | N/A | 20 spec test cases added |
| Phase 2: Certificate Generation | [x] | 4/4 | GenerateControllerOperationalCert() |
| Phase 3: Certificate Storage | [x] | 5/5 | Get/SetControllerCert(), Save/Load, auto-gen on Start() |
| Phase 4: TLS Configuration | [x] | 5/5 | NewOperationalClientTLSConfig(), buildOperationalTLSConfig() |
| Phase 5: CLI Command | [x] | 6/6 | cert command with Zone CA, controller, device cert display |
| Phase 6: Renewal | [x] | 2/2 | RenewControllerCert(), StartRenewalChecking() |

---

## References

- Matter `CHIPDeviceController.h:103-105` - Controller NOC parameters
- Matter `CASESession.cpp` - Sigma3 message with controller NOC
- MASH `docs/security.md` - "Both sides present operational certificates"
- MASH `pkg/cert/types.go` - Existing `OperationalCert` type
