# Connection Cap Behavior

> Transport-level enforcement of the max_zones+1 connection limit (DEC-062)

**Status:** Draft
**Created:** 2026-02-04

---

## 1. Overview

This document specifies the transport-level connection cap behavior. The device MUST enforce a total concurrent connection limit of `max_zones + 1` at TCP accept, before the TLS handshake. This prevents resource exhaustion from flood connections that never send application-layer messages.

**Scope:**
- Atomic counter semantics (increment/decrement points)
- Cap check and rejection behavior
- Interaction with PASE-level guards (DEC-047, DEC-061)

**Related decisions:** DEC-047, DEC-061, DEC-062

---

## 2. Counter Semantics

### 2.1 Increment Point

The active connection counter MUST be incremented in the accept loop, after a successful TCP accept and before spawning the connection handler goroutine.

```
TCP Accept() succeeds
  -> activeConns.Add(1)
  -> spawn handleConnection goroutine
```

### 2.2 Decrement Point

The active connection counter MUST be decremented when the connection handler returns, regardless of the exit reason. This covers:

- TLS handshake failure
- PASE handshake failure
- Normal connection close
- Context cancellation

```
handleConnection entry:
  -> defer activeConns.Add(-1)
  -> ... (TLS, PASE, operational logic)
  -> return (counter decremented by defer)
```

### 2.3 TOCTOU Prevention

The cap check and increment MUST be free of time-of-check-to-time-of-use races. This is achieved by performing both operations in the accept loop, which runs as a single goroutine. No concurrent goroutine can interleave between the check and the increment.

---

## 3. Cap Check Behavior

When the accept loop receives a new TCP connection:

1. Check `activeConns.Load() >= max_zones + 1`
2. If at cap: close the raw TCP connection immediately; do NOT proceed to TLS handshake
3. If below cap: increment counter and spawn handler

The cap applies to ALL connection types uniformly (commissioning and operational). Per-type tracking remains at higher layers (DEC-047).

---

## 4. Test Cases

### TC-CONN-CAP-001: Basic Cap Enforcement

**Preconditions:**
- Device with `max_zones = 2` (cap = 3)

**Steps:**
1. Open 3 TLS connections (all succeed)
2. Attempt 4th connection
3. Verify 4th connection is rejected (TCP close before TLS)
4. Close 1 existing connection
5. Retry connection
6. Verify retry succeeds

**Postconditions:**
- Exactly 3 connections active at peak
- Connection recovers after slot freed

### TC-CONN-CAP-002: Flood Resistance

**Preconditions:**
- Device with `max_zones = 2` (cap = 3)

**Steps:**
1. Launch 100 concurrent connection attempts
2. Wait for all to complete

**Postconditions:**
- At most 3 connections accepted concurrently
- Device remains responsive after flood

### TC-CONN-CAP-003: Counter Decrement on TLS Failure

**Preconditions:**
- Device with `max_zones = 2` (cap = 3)

**Steps:**
1. Connect with invalid TLS configuration (triggers handshake failure)
2. Wait for device to close connection
3. Open a valid connection

**Postconditions:**
- Counter decremented after TLS failure
- Subsequent connection succeeds (slot available)

### TC-CONN-CAP-004: Counter Decrement on Client Close

**Preconditions:**
- Device with `max_zones = 2` (cap = 3)

**Steps:**
1. Open 3 connections (fill cap)
2. Client closes 1 connection
3. Wait briefly for decrement
4. Open new connection

**Postconditions:**
- Counter decremented after client close
- New connection succeeds
