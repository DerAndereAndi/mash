# Fabric Administrator Model

**Date:** 2025-01-24
**Context:** How apps participate in commissioning without being fragile fabric owners

---

## The Problem

**Fragile App-as-Fabric:**
```
User's Phone Fabric
  └── Devices commissioned to phone

Problem: User loses phone → devices orphaned, can't be controlled
```

**Better: App as Admin of EMS Fabric:**
```
EMS Fabric (EMS is owner)
  ├── Admin: User's Phone App
  ├── Admin: User's Tablet
  └── Devices

Benefit: Lose phone → just authorize new app, devices unaffected
```

---

## Fabric Roles

| Role | Description | Can Do |
|------|-------------|--------|
| **Fabric Owner** | The controller that owns the fabric CA | Issue certs, add/remove admins, full control |
| **Fabric Admin** | Authorized to commission on behalf of owner | Commission devices, manage fabrics on devices |
| **Fabric Member** | Normal participant | Read/Write/Subscribe based on permissions |

---

## App Becoming a Fabric Admin

### Option A: App-EMS Pairing (Recommended)

```
Phase 1: Link App to EMS

User               Phone App              EMS
 │                     │                   │
 │── Open app ────────►│                   │
 │                     │── mDNS discover ─►│
 │                     │◄── Found EMS ─────┤
 │                     │                   │
 │◄── Show EMS list ───┤                   │
 │── Select EMS ──────►│                   │
 │                     │                   │
 │                     │── Request admin ─►│
 │                     │                   │
 │◄─────────────────── Show code on EMS ──┤  (or QR)
 │                     │                   │
 │── Enter code ──────►│                   │
 │                     │── SPAKE2+ ───────►│  (with code)
 │                     │◄── Admin token ───┤
 │                     │                   │
 │◄── "Linked to EMS" ─┤                   │
```

**Result:** App has admin token for EMS fabric. Can now commission devices.

### Option B: QR Code on EMS

```
EMS displays: "Scan to add admin app"
  ┌──────────────┐
  │ [QR CODE]    │  Contains: EMS ID + admin pairing code
  │              │
  └──────────────┘

App scans → SPAKE2+ with EMS → gets admin token
```

### Option C: Cloud-Based Linking

```
User logs into EMS cloud account on phone
Cloud authorizes app as admin for user's EMS
App receives admin credentials via cloud
```

---

## Admin Token

When app becomes admin, it receives:

```cbor
{
  "type": "admin_token",
  "fabric_id": "ems-abc123",
  "fabric_ca_cert": "...",       // EMS's CA cert (to verify device certs)
  "admin_id": "app-xyz789",
  "permissions": ["commission", "manage_fabrics"],
  "issued_at": 1706108400,
  "expires_at": 1737644400,      // Optional: 1 year
  "signature": "..."             // Signed by EMS
}
```

---

## Commissioning Flow with Admin Token

```
User            Phone App           EMS              Device
 │                  │                │                  │
 │── Scan QR ──────►│                │                  │
 │   (device)       │                │                  │
 │                  │                │                  │
 │                  │── TCP connect ─┼─────────────────►│
 │                  │                │                  │
 │                  │── SPAKE2+ ────┼─────────────────►│
 │                  │   (setup code) │                  │
 │                  │◄──────────────┼─── Accept ───────┤
 │                  │                │                  │
 │                  │── Present ────┼─────────────────►│
 │                  │   admin token  │                  │
 │                  │                │                  │
 │                  │◄──────────────┼─── CSR ──────────┤
 │                  │                │                  │
 │                  │── Forward CSR ►│                  │
 │                  │◄── Signed cert ┤  (EMS signs)    │
 │                  │                │                  │
 │                  │── Install cert ┼─────────────────►│
 │                  │                │                  │
 │◄─ "Device added" ┤                │                  │
```

**Key Points:**
1. App initiates connection to device
2. App does SPAKE2+ (has setup code from QR)
3. App presents admin token (proves it's authorized)
4. Device generates CSR
5. **App forwards CSR to EMS** (app can't sign, only EMS can)
6. EMS signs and returns operational cert
7. App installs cert on device

---

## EMS-less Commissioning?

**Question:** What if user doesn't have an EMS, just an app?

**Options:**

### 1. App has minimal fabric capability
- App CAN be a fabric owner (simple cases)
- But recommended to link to EMS if available
- Migration path: transfer devices to EMS later

### 2. Cloud EMS
- App links to cloud-based EMS
- Cloud handles fabric ownership
- Works for users without local EMS

### 3. Device-only mode
- No fabric, just direct device control
- Very limited functionality
- Upgrade path when EMS added

**Recommendation:** Option 1 - App can own fabric for simple cases, but the "happy path" is to be admin of a real EMS.

---

## Multiple Admins

```
EMS Fabric Admins:
  ├── User's Phone (permanent)
  ├── User's Tablet (permanent)
  ├── Spouse's Phone (permanent)
  └── Installer's Tool (temporary, expires in 24h)
```

**Admin Management:**
- EMS can list all admins
- EMS can revoke admin tokens
- Temporary admins for installers
- User can see all admins in EMS UI

---

## SMGW Scenario Revisited

```
SMGW Fabric (Grid Operator):
  Fabric Owner: SMGW
  Admins:
    └── Utility Phone App (delegated commissioning)

EMS Fabric (Home):
  Fabric Owner: EMS
  Admins:
    ├── User's Phone App
    └── User's Tablet

Device (EVSE):
  ├── Fabric 1: SMGW (GRID_OPERATOR)
  └── Fabric 2: EMS (HOME_MANAGER)
```

**Commissioning Flows:**

1. **User commissions to EMS:**
   - Scans device QR with phone app
   - App is EMS admin → commissions to EMS fabric

2. **User registers with utility:**
   - Scans device QR with utility app
   - Utility app uploads to backend
   - Backend forwards to SMGW
   - SMGW commissions (it's fabric owner, no admin needed)

---

## Summary

**Key Decisions:**

1. **Apps are admins, not owners** (recommended path)
2. **EMS is fabric owner** (has CA, issues certs)
3. **App-EMS pairing** establishes admin relationship
4. **Admin token** authorizes app to commission
5. **CSR forwarding** - app can't sign, only forward to EMS
6. **Multiple admins** supported for convenience
7. **Fallback** - app CAN own fabric for simple/no-EMS cases

**Benefits:**
- Losing phone doesn't orphan devices
- Multiple family members can be admins
- Installers get temporary admin rights
- Clear authority hierarchy
- Works with SMGW backend delegation
