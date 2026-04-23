# Certificate Renewal Mechanism: In-Session vs. Reconnect Comparison

**Status:** Research spike (§J of `kiss-implementation-plan.md`)
**Date:** 2026-04-23
**Outcome:** See §6 Recommendation — **keep the current in-session design**.

---

## 1. Motivation

MASH operational certificates renew 30 days before expiry (`docs/security.md §3`). The reference implementation performs renewal **in-session** via a dedicated sub-protocol (MsgTypes 30–33). The v3 KISS analysis flagged this as a candidate for simplification: "is a dedicated renewal subprotocol worth it, or could reconnection-based renewal do the same job with less code?"

This document measures the current design, sketches a reconnect-based alternative, and recommends a direction.

## 2. Current Design — In-Session Renewal

### 2.1 Code footprint (production, hand-written)

| File | LOC | Role |
|---|---:|---|
| `mash-go/pkg/service/controller_renewal.go` | 302 | Controller-side orchestration |
| `mash-go/pkg/service/device_renewal.go` | 183 | Device-side orchestration |
| `mash-go/pkg/service/renewal_tracker.go` | 158 | Per-connection renewal state |
| `mash-go/pkg/commissioning/renewal_messages.go` | 171 | Wire message definitions + nonce hashing |
| **Total production** | **814** | |

Test coverage (unit):
 - `renewal_tracker_test.go` — 265 LOC
 - `device_renewal_test.go` — 454 LOC
 - `controller_renewal_test.go` — 484 LOC

### 2.2 Wire-protocol surface

Four MsgTypes exist solely for renewal:

| MsgType | Name | Direction | Purpose |
|---:|---|---|---|
| 30 | `CertRenewalRequest` | Controller → Device | Initiate renewal; carries random nonce |
| 31 | `CertRenewalCSR` | Device → Controller | CSR (new public key) + NonceHash = SHA256(nonce)[0:16] |
| 32 | `CertRenewalInstall` | Controller → Device | Signed certificate |
| 33 | `CertRenewalAck` | Device → Controller | Installation confirmation |

Nonce-binding (DEC-047): the 16-byte truncated SHA-256 hash in the CSR proves the CSR was generated in response to *this* renewal request, not replayed from a previous one.

### 2.3 Flow

```
T0  Controller checks cert ttl < 30 days
T1  Controller → Device: CertRenewalRequest(nonce)  [over existing operational TLS]
T2  Device generates new keypair; CSR = signed(pub, NonceHash(nonce))
T3  Device → Controller: CertRenewalCSR(csr, nonceHash)
T4  Controller verifies nonceHash, signs cert with Zone CA
T5  Controller → Device: CertRenewalInstall(cert)
T6  Device swaps cert atomically; TLS session continues (session keys unchanged)
T7  Device → Controller: CertRenewalAck
```

Session continuity preserved: no TCP reconnect, no TLS handshake, no PASE.

---

## 3. Alternative — Reconnect-Based Renewal

### 3.1 Concept

Approaching expiry, device closes the operational TLS session, regenerates a keypair locally, and re-establishes a new TLS connection through a commissioning-style channel that can carry a CSR. Certificate issuance rides the commissioning flow.

### 3.2 What deletes

If adopted, the code in §2.1 goes away (all 814 LOC + 1,203 test LOC). MsgTypes 30–33 retire. The `RenewalTracker` state machine disappears. `pkg/service` loses two files outright (`controller_renewal.go`, `device_renewal.go`).

### 3.3 What needs adding

Genuinely new work, not covered by today's commissioning:

1. **Expiry-triggered disconnect orchestration.** Today's reconnect code is reactive (runs on connection loss). Adding a proactive "near-expiry → initiate clean disconnect → reconnect through re-commissioning channel" flow is a state-machine addition on both sides, approximately 100–150 LOC each.
2. **PASE semantics for renewal.** Today PASE runs once per zone using the user-provided setup code. Renewal is not commissioning. We would need one of:
   - Reuse the original setup code (weakens the one-shot posture; now the code is long-lived)
   - Pre-derive a renewal-specific shared secret from the original session (equivalent in spirit to MsgTypes 30–33, just reshuffled)
   - Skip PASE entirely and allow TLS-only re-authentication using the still-valid operational cert (requires an ALPN route that accepts operational certs for CSR submission)

   Each option implies its own design + DEC decisions.
3. **Zone-id and device-id continuity.** The new certificate must preserve the same `zoneId` / `deviceId`. The current flow guarantees this trivially (nonce-bound request). A reconnect flow needs explicit guarantees to prevent a misconfigured device from re-commissioning into a different zone.
4. **Observable downtime.** Zero today; ~200ms–1s with reconnect (TLS handshake + PASE + cert issuance + TLS re-handshake with new cert). Subscription notifications are interrupted. An EMS that issues a `SetLimit` during the reconnect window either races with the disconnect or sees a reject.

Rough code added: ~300–400 LOC (new orchestration on both sides) + new design/DECs for the PASE question (not free).

### 3.4 Flow (option: reuse setup code)

```
T0  Device checks own cert ttl < 30 days
T1  Device → Controller: RenewalNotify (new MsgType, pre-reconnect)  [optional]
T2  Device closes operational TLS session
T3  Device generates new keypair locally
T4  Device connects via commissioning ALPN
T5  PASE with stored setup code
T6  Device → Controller: CSR  (same NonceHash discipline)
T7  Controller → Device: signed cert
T8  Device closes commissioning session, opens operational TLS with new cert
T9  Controller re-admits device; subscription state replayed (DEC-058 snapshot)
```

Note: T6 and T7 look a lot like MsgTypes 31 and 32 from the current design. The in-session sub-protocol is, concretely, a short-circuit of steps T2, T3, T4, T5, T8, T9 — i.e., the current design *is* the reconnect design with everything between T1 and T7 elided.

---

## 4. Side-by-Side Comparison

| Dimension | In-Session (current) | Reconnect-Based |
|---|---|---|
| Production LOC | 814 | ~300–400 (est.) |
| Test LOC | 1,203 | similar (reconnect orchestration needs coverage) |
| Dedicated MsgTypes | 4 (30–33) | 0–1 (depending on PASE option) |
| Downtime per renewal | 0 ms | ~200–1000 ms (TLS + PASE + issuance + TLS) |
| TLS handshakes per renewal | 0 | 2 (commissioning, then new operational) |
| MCU peak memory during renewal | Briefly holds old + new private key (~128 B delta) | Single TLS session state, briefly two if commissioning overlaps |
| Observability | Silent to subscribers (no connection event) | Visible disconnect/reconnect — can be a plus (audit trail) or minus (noise) |
| Security | Session keys unchanged across renewal; relies on nonce-binding (DEC-047) | Fresh handshake + fresh session keys; but reuses setup code OR requires new PASE variant |
| Interacts with EMS operations | Transparent; commands continue | Races with in-flight writes during the reconnect window |
| Spec decisions required | Done (DEC-015, DEC-047) | At least 2 new DECs (PASE-for-renewal semantics, zone-id continuity guarantees) |
| Reversibility | Easy to switch away (delete 814 LOC) | Hard to switch back (reintroduce 4 MsgTypes + renewal state machine + nonce binding) |

---

## 5. Reversibility

**In-session → reconnect:** easy. Delete 814 LOC; controller/device renewal logic is isolated behind `RenewalTracker` interfaces that a reconnect flow could replace. The wire-protocol retirement (MsgTypes 30–33) is a one-way change because the project is pre-1.0 and no deployed devices exist yet.

**Reconnect → in-session:** hard. You'd need to reintroduce the MsgType allocations (can't reuse 30–33 if they were publicly retired), the renewal state machine, nonce binding, and the renewal-specific test infrastructure. If the reconnect flow picked a particular PASE-reuse option, that decision would also need to be unwound.

**Implication:** the asymmetry favors keeping the current design until evidence accumulates that it's costing us. Switching *to* reconnect is a door that stays open; switching *back* is a door that closes quickly.

---

## 6. Recommendation

**Keep the current in-session design.**

Evidence:

1. **Complexity cost is contained.** 814 LOC of production code + 1,203 test LOC is ~2 KLOC in a ~30 KLOC service package. That's not the tall pole.
2. **Zero-downtime matters for energy control.** The whole point of this protocol is load-flexibility coordination. A ~500ms disconnect window in which `SetLimit` writes race against a cert rotation is a fragility the protocol currently doesn't have. Adding it is a downside a spec-text improvement can't recover.
3. **PASE reuse is a design tax.** Reconnect-based renewal is only simpler if the new commissioning channel doesn't need PASE. Any variant that *does* run PASE on every renewal turns the setup code into a long-lived secret, weakening the one-shot commissioning posture (DEC-068).
4. **LOC savings are partly illusory.** Deleting 814 LOC of renewal code also means adding ~300–400 LOC of proactive-reconnect orchestration that today doesn't exist. Net savings: ~400 LOC at best, with more complex semantics.
5. **The in-session sub-protocol *is* the short-circuited reconnect flow.** Steps T6/T7 of the reconnect version (CSR submission, cert install) are essentially MsgTypes 31/32. We've paid the design cost; the remaining wire cost (MsgTypes 30 and 33 for request + ack) is 2 message types, not 4 — the bookkeeping around it is where the LOC lives, and that bookkeeping still exists in a reconnect variant.

**Follow-up within the current design (non-§J, optional future work):**

- Consider shortening the 30-day renewal window to 7 days. Rationale: shorter overlap period reduces the window in which an active attacker with a stolen operational cert can reach renewal. This is a single-constant change plus adjusted tests. Tracked in Watchlist W7 of the implementation plan.
- When §E lands (`pkg/service` split), move the three renewal files into the new `pkg/service/zone/` or `pkg/service/renewal/` sub-package for locality.

**Follow-up if evidence changes:**

- If a concrete operational issue with in-session renewal surfaces (e.g., a dispatcher deadlock during cert rotation, or repeated test flakes specifically in the renewal path), revisit.
- If the service-split (§E) reveals that the renewal state machine has leaked into many call sites, reconsider whether a narrower reconnect-based boundary would actually localize complexity better than attempting an in-place tidy.

---

## 7. DEC Entry

The decision is recorded as **DEC-076: Retain In-Session Certificate Renewal** in `docs/decision-log.md`. DEC-015 (original cert lifecycle decision) is referenced but not modified — DEC-076 is the explicit "we re-examined this and the answer is still in-session."
