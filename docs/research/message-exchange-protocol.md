# Message Exchange Protocol

**Date:** 2025-01-24
**Context:** How messages flow over TCP/TLS connection

---

## Transport Recap

```
┌────────────────────────────────────────┐
│ Length (4 bytes, big-endian) │ CBOR    │
└────────────────────────────────────────┘
```

- Single persistent TCP/TLS connection per device pair
- Client (controller) initiates connection
- Bidirectional: both sides can send messages

---

## Message Types

| Type | Direction | Purpose |
|------|-----------|---------|
| `request` | Client → Server | Initiate operation (expects response) |
| `response` | Server → Client | Answer to request |
| `notify` | Server → Client | Subscription update (no response) |
| `ping` | Either → Either | Keep-alive check |
| `pong` | Either → Either | Keep-alive response |

---

## Message Structure

### Common Fields

```cbor
{
  1: <message_id>,      // uint32 - for correlation
  2: <message_type>,    // uint8 - request=1, response=2, notify=3, ping=4, pong=5
  ...                   // type-specific fields
}
```

Using integer keys for compactness (CBOR).

### Request Message

```cbor
{
  1: 42,                // message_id
  2: 1,                 // type: request
  3: 1,                 // operation: read=1, write=2, subscribe=3, invoke=4
  4: [1, 100, 1],       // path: [endpoint_id, cluster_id, attribute_id]
  5: {...}              // payload (operation-specific)
}
```

### Response Message

```cbor
{
  1: 42,                // message_id (matches request)
  2: 2,                 // type: response
  6: 0,                 // status: ok=0, error=1
  5: {...},             // payload (result data)
  7: {...}              // error (if status=1)
}
```

### Notify Message (Subscription Update)

```cbor
{
  1: 0,                 // message_id: 0 for notifications
  2: 3,                 // type: notify
  8: 123,               // subscription_id
  4: [1, 100, 1],       // path
  5: {...}              // payload (new value)
}
```

### Ping/Pong

```cbor
{
  1: 99,                // message_id
  2: 4                  // type: ping
}

{
  1: 99,                // message_id (matches ping)
  2: 5                  // type: pong
}
```

---

## Field Definitions (Integer Keys)

| Key | Name | Type | Description |
|-----|------|------|-------------|
| 1 | message_id | uint32 | Correlation ID |
| 2 | type | uint8 | Message type |
| 3 | operation | uint8 | Operation type |
| 4 | path | array | [endpoint, cluster, attribute/command] |
| 5 | payload | any | Operation data |
| 6 | status | uint8 | Response status |
| 7 | error | map | Error details |
| 8 | subscription_id | uint32 | Subscription reference |

---

## Operations

### Read

**Request:**
```cbor
{
  1: 1,                 // message_id
  2: 1,                 // type: request
  3: 1,                 // operation: read
  4: [1, 100, 1]        // path: endpoint 1, Measurement cluster, power attribute
}
```

**Response:**
```cbor
{
  1: 1,                 // message_id
  2: 2,                 // type: response
  6: 0,                 // status: ok
  5: {                  // payload
    "value": 7400,
    "unit": "W",
    "timestamp": 1706108400
  }
}
```

### Write

**Request:**
```cbor
{
  1: 2,
  2: 1,                 // request
  3: 2,                 // operation: write
  4: [1, 101, 1],       // LoadControl cluster, limit attribute
  5: {
    "value": 3700,
    "duration": 3600
  }
}
```

**Response:**
```cbor
{
  1: 2,
  2: 2,                 // response
  6: 0                  // status: ok
}
```

### Subscribe

**Request:**
```cbor
{
  1: 3,
  2: 1,                 // request
  3: 3,                 // operation: subscribe
  4: [1, 100, 1],       // Measurement.power
  5: {
    "min_interval": 1,    // seconds - don't notify more often than this
    "max_interval": 60    // seconds - notify at least this often
  }
}
```

**Response:**
```cbor
{
  1: 3,
  2: 2,                 // response
  6: 0,                 // status: ok
  8: 456,               // subscription_id
  5: {                  // current value included
    "value": 7400,
    "unit": "W"
  }
}
```

**Subsequent Notifications:**
```cbor
{
  1: 0,                 // no request correlation
  2: 3,                 // type: notify
  8: 456,               // subscription_id
  4: [1, 100, 1],       // path (for context)
  5: {
    "value": 3200,
    "unit": "W",
    "timestamp": 1706108460
  }
}
```

### Unsubscribe

**Request:**
```cbor
{
  1: 4,
  2: 1,                 // request
  3: 5,                 // operation: unsubscribe
  8: 456                // subscription_id
}
```

**Response:**
```cbor
{
  1: 4,
  2: 2,
  6: 0
}
```

### Invoke (Command)

**Request:**
```cbor
{
  1: 5,
  2: 1,                 // request
  3: 4,                 // operation: invoke
  4: [1, 102, 1],       // ChargingSession cluster, StartCharging command
  5: {
    "target_soc": 80,
    "max_power": 11000
  }
}
```

**Response:**
```cbor
{
  1: 5,
  2: 2,
  6: 0,
  5: {
    "session_id": "sess-789",
    "started_at": 1706108500
  }
}
```

---

## Error Handling

**Error Response:**
```cbor
{
  1: 6,
  2: 2,                 // response
  6: 1,                 // status: error
  7: {
    "code": 404,        // error code
    "message": "Attribute not found",
    "path": [1, 100, 99]
  }
}
```

**Error Codes:**

| Code | Name | Description |
|------|------|-------------|
| 400 | BadRequest | Malformed request |
| 401 | Unauthorized | Not authorized for this operation |
| 403 | Forbidden | Operation not allowed (e.g., read-only) |
| 404 | NotFound | Path doesn't exist |
| 409 | Conflict | Operation conflicts with current state |
| 429 | TooManyRequests | Rate limited |
| 500 | InternalError | Server error |
| 503 | Unavailable | Temporarily unavailable |

---

## Subscription Behavior

### Min/Max Intervals

```
min_interval: 1s   - Don't send updates more often than every 1s
max_interval: 60s  - Always send update at least every 60s (even if unchanged)
```

### Change Detection

- Notify when value changes by more than threshold (configurable)
- Always notify when value crosses important boundaries
- Max interval ensures client knows connection is alive

### Subscription Lifecycle

1. **Subscribe** - Client requests, gets subscription_id + current value
2. **Notify** - Server pushes updates
3. **Unsubscribe** - Client explicitly ends subscription
4. **Connection lost** - All subscriptions for that connection are cancelled
5. **Reconnect** - Client must re-subscribe

---

## Connection Management

### Keep-Alive

```
Idle timeout: 90 seconds
Ping interval: 30 seconds (if no other traffic)
Pong timeout: 5 seconds
Max missed pongs: 3
```

**Flow:**
```
Client                              Server
  │                                    │
  │  (no traffic for 30s)              │
  │                                    │
  │── Ping (id: 100) ─────────────────►│
  │◄── Pong (id: 100) ─────────────────┤
  │                                    │
  │  (30s passes, still idle)          │
  │                                    │
  │── Ping (id: 101) ─────────────────►│
  │   (no pong within 5s)              │
  │── Ping (id: 102) ─────────────────►│
  │   (no pong within 5s)              │
  │── Ping (id: 103) ─────────────────►│
  │   (no pong within 5s)              │
  │                                    │
  │  Connection considered dead        │
  │  Close and reconnect               │
```

### Reconnection

1. Close existing connection
2. Wait: exponential backoff (1s, 2s, 4s, 8s... max 5min)
3. Reconnect
4. Re-authenticate (CASE with operational cert)
5. Re-subscribe to all previous subscriptions

---

## Multiplexing

Multiple requests can be in-flight simultaneously:

```
Client                              Server
  │                                    │
  │── Request id:1 (read power) ──────►│
  │── Request id:2 (read voltage) ────►│
  │── Request id:3 (write limit) ─────►│
  │                                    │
  │◄── Response id:2 ──────────────────┤  (responses can arrive
  │◄── Response id:1 ──────────────────┤   out of order)
  │◄── Response id:3 ──────────────────┤
```

- Message IDs correlate requests to responses
- No head-of-line blocking
- Server processes requests independently

---

## Size Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max message size | 64 KB | Fits in RAM on 256KB device |
| Max in-flight requests | 16 | Prevent resource exhaustion |
| Max subscriptions | 32 | Per connection |
| Max path depth | 3 | endpoint/cluster/attribute |

---

## Example Session

```
Client                              Server (EVSE)
  │                                    │
  │══ TLS Handshake (CASE) ═══════════►│
  │◄══════════════════════════════════ │
  │                                    │
  │── Read DeviceInfo ────────────────►│
  │◄── {vendor, model, serial} ────────┤
  │                                    │
  │── Subscribe Measurement.power ────►│
  │◄── {sub_id:1, value:0} ────────────┤
  │                                    │
  │── Subscribe ChargingSession.state ►│
  │◄── {sub_id:2, value:"idle"} ───────┤
  │                                    │
  │  (EV plugs in)                     │
  │                                    │
  │◄── Notify sub:2 {state:"connected"}┤
  │                                    │
  │── Invoke StartCharging ───────────►│
  │◄── {session_id:"sess-1"} ──────────┤
  │                                    │
  │◄── Notify sub:2 {state:"charging"} ┤
  │◄── Notify sub:1 {value:7400} ──────┤
  │                                    │
  │  (30s idle)                        │
  │                                    │
  │── Ping ───────────────────────────►│
  │◄── Pong ───────────────────────────┤
  │                                    │
```

---

## Comparison to Alternatives

### vs WebSocket
- Similar bidirectional capability
- No HTTP upgrade overhead
- Custom framing more efficient

### vs HTTP/2
- Simpler (no stream management)
- Lower overhead
- Sufficient for our use case

### vs CoAP Observe
- TCP reliability vs UDP+CON
- Simpler subscription model
- No blockwise transfer needed (messages small)

### vs MQTT
- No broker needed
- Direct device-to-device
- Simpler topic model (just paths)

---

## Summary

**Message flow is simple:**
1. Client sends requests, server sends responses
2. Server pushes notifications for subscriptions
3. Both can send ping/pong
4. Message IDs correlate requests to responses
5. Subscriptions identified by subscription_id

**Key design choices:**
- Integer keys in CBOR for compactness
- Single connection, multiplexed requests
- Server-initiated push for subscriptions
- Explicit unsubscribe (not just timeout)
- Reconnect = resubscribe
