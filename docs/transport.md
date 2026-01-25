# MASH Transport Layer

> TCP/TLS transport with length-prefixed framing

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Protocol Overview](protocol-overview.md) | Vision, architecture, device model |
| [Security](security.md) | TLS certificates, commissioning |
| [Interaction Model](interaction-model.md) | Message semantics |

---

## 1. Protocol Stack

```
┌────────────────────────────────┐
│      CBOR Messages             │
├────────────────────────────────┤
│   Length-Prefix Framing (4B)   │
├────────────────────────────────┤
│         TLS 1.3                │
├────────────────────────────────┤
│           TCP                  │
├────────────────────────────────┤
│         IPv6 only              │
└────────────────────────────────┘
```

---

## 2. Network Layer

### 2.1 IPv6-Only

MASH requires IPv6. No IPv4 support.

**Rationale:**
- Simplifies implementation (single code path)
- Every modern device supports IPv6
- Link-local addresses work without infrastructure
- Thread mesh compatibility for future

### 2.2 Address Types

| Address Type | Range | Use |
|--------------|-------|-----|
| Link-local | fe80::/10 | Commissioning, no router needed |
| Multicast | ff02::fb | mDNS discovery |
| Global/ULA | Site-dependent | Operational communication |

### 2.3 Auto-Configuration

- **SLAAC** (Stateless Address Autoconfiguration) for address assignment
- No DHCP required (but supported)
- Works on isolated networks (link-local only)

---

## 3. TLS 1.3

### 3.1 Requirements

- TLS 1.3 only (no fallback to 1.2)
- Mutual authentication with certificates
- See [Security](security.md) for certificate details

### 3.2 Cipher Suites

Required cipher suites (in preference order):

```
TLS_AES_128_GCM_SHA256        (mandatory)
TLS_AES_256_GCM_SHA384        (recommended)
TLS_CHACHA20_POLY1305_SHA256  (optional, good for constrained devices)
```

### 3.3 Key Exchange

- ECDHE with P-256 (secp256r1) - mandatory
- ECDHE with X25519 - recommended

---

## 4. Frame Format

All messages are length-prefixed:

```
┌─────────────────────────────────────────────┐
│ Length (4 bytes, big-endian) │ CBOR Payload │
└─────────────────────────────────────────────┘
```

### 4.1 Length Field

- 4 bytes, unsigned, big-endian
- Represents payload length (not including the 4-byte length field itself)
- Maximum message size: 65536 bytes (64KB)

### 4.2 Example

A 256-byte CBOR message:
```
00 00 01 00 [256 bytes of CBOR data]
```

### 4.3 Why Length-Prefix?

| Approach | Pros | Cons |
|----------|------|------|
| Length-prefix | Simple parsing, known size upfront | 4 bytes overhead |
| Delimiter-based | No overhead | Complex parsing, escaping needed |
| Self-describing | Flexible | Must parse to know size |

MASH uses length-prefix for simplicity and efficiency on constrained devices.

---

## 5. Connection Model

### 5.1 Client/Server Roles

- **Client** = Controller (EMS, SMGW, app)
- **Server** = Device (EVSE, inverter, heat pump)

**Client always initiates connection.** This eliminates the EEBUS double-connection race condition.

### 5.2 Connection Persistence

- One persistent connection per controller-device pair
- Connection stays open for the session lifetime
- Automatic reconnection on disconnect

### 5.3 Port

- Default port: **8443** (TBD - needs IANA registration consideration)
- Configurable per device

---

## 6. Keep-Alive

### 6.1 Mechanism

Keep-alive uses application-layer ping/pong (not TCP keep-alive):

| Parameter | Value |
|-----------|-------|
| Ping interval | 30 seconds (if no other activity) |
| Pong timeout | 5 seconds |
| Max missed pongs | 3 |

### 6.2 Ping Message

```cbor
{
  "type": "ping",
  "seq": 12345
}
```

### 6.3 Pong Message

```cbor
{
  "type": "pong",
  "seq": 12345
}
```

### 6.4 Connection Closure

After 3 missed pongs (90 seconds without response):
1. Close TCP connection
2. Notify application layer
3. Attempt reconnection (client side)

---

## 7. Reconnection

### 7.1 Client Reconnection Strategy

When connection is lost, client should:

1. Wait initial backoff (1 second)
2. Attempt reconnection
3. On failure, exponential backoff: 2s, 4s, 8s, 16s, 32s, 60s (max)
4. Continue attempting at 60s intervals
5. Reset backoff on successful connection

### 7.2 State Preservation

After reconnection:
- Subscriptions must be re-established
- Device state should be re-read
- No assumption that state is unchanged

---

## 8. Message Size Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max message size | 64 KB | Fits in constrained device memory |
| Typical message | < 2 KB | Most operations are small |
| Max subscription batch | 10 KB | Prevent flooding |

---

## 9. Error Handling

### 9.1 Transport Errors

| Error | Action |
|-------|--------|
| TLS handshake failure | Close connection, log error |
| Invalid length prefix | Close connection |
| Message too large | Close connection |
| CBOR parse error | Send error response, keep connection |
| Timeout | Close connection, reconnect |

### 9.2 Graceful Shutdown

To close connection gracefully:

```cbor
{
  "type": "close",
  "reason": "shutdown"
}
```

Wait for acknowledgment or 5-second timeout, then close TCP.

---

## 10. Comparison with EEBUS SHIP

| Aspect | EEBUS SHIP | MASH Transport |
|--------|------------|----------------|
| Protocol | WebSocket over TLS | Raw TLS over TCP |
| Framing | WebSocket frames | Length-prefix |
| Handshake | Complex SHIP state machine | TLS + simple hello |
| Connection initiation | Either side (race condition) | Client only |
| Keep-alive | SHIP-level + WebSocket | Application ping/pong |
| Message format | JSON | CBOR |

**Key improvements:**
- No WebSocket overhead
- No SHIP state machine complexity
- Deterministic connection ownership
- Binary format for efficiency
