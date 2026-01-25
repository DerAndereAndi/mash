# Transport Layer Comparison

**Date:** 2025-01-24
**Context:** Choosing transport layer for MASH protocol
**Target:** 256KB RAM MCU, local network (home/building)

---

## Requirements

| Requirement | Priority | Notes |
|-------------|----------|-------|
| TLS encryption | Must | Security baseline |
| Reliable delivery | Must | Commands must not be lost |
| Bidirectional | Must | Subscriptions need server→client |
| NAT traversal | Should | Works behind home routers |
| Low overhead | Should | Fits in 256KB RAM |
| Reconnection | Must | Handle network blips |
| Library availability | Must | Go + C/embedded |

---

## Option 1: WebSocket over TLS (like SHIP)

```
┌─────────────────────────┐
│     MASH Messages       │
├─────────────────────────┤
│      WebSocket          │
├─────────────────────────┤
│        TLS 1.3          │
├─────────────────────────┤
│         TCP             │
└─────────────────────────┘
```

### Characteristics

| Metric | Value |
|--------|-------|
| **Overhead per message** | 2-14 bytes (WS frame header) |
| **Handshake** | HTTP upgrade + TLS (3-4 RTT) |
| **Reliability** | TCP guarantees (ordered, reliable) |
| **Bidirectional** | Full duplex |
| **NAT** | Works well (outbound TCP) |

### Go Libraries
- `gorilla/websocket` - mature, widely used
- `nhooyr.io/websocket` - modern, context-aware
- stdlib `net/http` + upgrade

### Embedded Libraries
- `libwebsockets` (C) - ~50KB code, well-tested
- `mongoose` (C) - ~40KB, includes HTTP server
- ESP-IDF has built-in WebSocket client

### Pros
- **Proven in SHIP** - known to work for this domain
- **Excellent library support** - mature implementations
- **Works through proxies** - HTTP-based upgrade
- **Easy debugging** - browser dev tools, curl
- **Keep-alive built-in** - ping/pong frames

### Cons
- **HTTP upgrade overhead** - initial handshake heavier
- **WebSocket framing** - slight overhead per message
- **Overkill for local network** - designed for web, not IoT

### Memory Footprint (ESP32)
- TLS: ~40KB
- WebSocket: ~15KB
- Total: ~55KB code

---

## Option 2: Raw TLS with Length-Prefixed Framing

```
┌─────────────────────────┐
│     MASH Messages       │
├─────────────────────────┤
│   Length-Prefix Frame   │  ← 4-byte length + CBOR
├─────────────────────────┤
│        TLS 1.3          │
├─────────────────────────┤
│         TCP             │
└─────────────────────────┘
```

### Frame Format
```
┌──────────────┬─────────────────────────┐
│ Length (4B)  │    CBOR Payload         │
└──────────────┴─────────────────────────┘
```

### Characteristics

| Metric | Value |
|--------|-------|
| **Overhead per message** | 4 bytes (length prefix) |
| **Handshake** | TLS only (2-3 RTT) |
| **Reliability** | TCP guarantees |
| **Bidirectional** | Full duplex |
| **NAT** | Works well (outbound TCP) |

### Go Libraries
- `crypto/tls` - stdlib, excellent
- Just need ~50 lines for length-prefix framing

### Embedded Libraries
- mbedTLS (ESP-IDF default) - ~30KB code
- wolfSSL - ~20KB code
- Custom framing: ~1KB code

### Pros
- **Minimal overhead** - just length prefix
- **Simpler than WebSocket** - less code
- **Faster handshake** - no HTTP upgrade
- **Smaller code size** - no WS library needed

### Cons
- **Custom framing** - must implement ourselves
- **No keep-alive standard** - must define our own
- **No proxy support** - won't work through HTTP proxies
- **Less debugging tools** - no browser support

### Memory Footprint (ESP32)
- TLS: ~40KB
- Framing: ~2KB
- Total: ~42KB code

---

## Option 3: CoAP over DTLS

```
┌─────────────────────────┐
│     MASH Messages       │
├─────────────────────────┤
│         CoAP            │  ← RFC 7252
├─────────────────────────┤
│        DTLS 1.3         │
├─────────────────────────┤
│         UDP             │
└─────────────────────────┘
```

### Characteristics

| Metric | Value |
|--------|-------|
| **Overhead per message** | 4-20 bytes (CoAP header) |
| **Handshake** | DTLS (2-3 RTT) |
| **Reliability** | CoAP CON messages (app-level ACK) |
| **Bidirectional** | Via Observe extension |
| **NAT** | Tricky - UDP NAT timeouts |

### Go Libraries
- `plgd-dev/go-coap` - mature, CoAP v2
- `dustin/go-coap` - simpler, CoAP v1

### Embedded Libraries
- libcoap (C) - ~40KB code
- Zephyr OS has built-in CoAP
- ESP-IDF: must add library

### Pros
- **Designed for IoT** - RFC 7252, proven in Thread/Matter
- **Low overhead** - binary protocol
- **Observe pattern** - built-in subscription model
- **RESTful** - GET/PUT/POST/DELETE maps well

### Cons
- **UDP complexity** - NAT traversal, retransmission
- **DTLS less common** - fewer library options
- **Home network concerns** - some routers drop UDP
- **Observe limitations** - complex state management

### Memory Footprint (ESP32)
- DTLS: ~45KB
- CoAP: ~15KB
- Total: ~60KB code

---

## Option 4: QUIC

```
┌─────────────────────────┐
│     MASH Messages       │
├─────────────────────────┤
│         QUIC            │  ← RFC 9000
├─────────────────────────┤
│         UDP             │
└─────────────────────────┘
```

### Characteristics

| Metric | Value |
|--------|-------|
| **Overhead per message** | Variable (stream-based) |
| **Handshake** | 1 RTT (0-RTT resumption) |
| **Reliability** | Built-in (per-stream) |
| **Bidirectional** | Multiple streams |
| **NAT** | Good - connection migration |

### Pros
- **Modern design** - 0-RTT, multiplexing
- **Connection migration** - handles network changes
- **Built-in encryption** - TLS 1.3 integrated

### Cons
- **Heavy implementation** - ~200KB+ code
- **Overkill for local IoT** - designed for internet scale
- **Limited embedded support** - few mature C libraries
- **Complexity** - congestion control, etc.

### Memory Footprint (ESP32)
- Unlikely to fit in 256KB RAM comfortably

---

## Comparison Matrix

| Criteria | WebSocket/TLS | Raw TLS | CoAP/DTLS | QUIC |
|----------|--------------|---------|-----------|------|
| **Message overhead** | 2-14B | 4B | 4-20B | Variable |
| **Code size** | ~55KB | ~42KB | ~60KB | ~200KB+ |
| **Library maturity** | Excellent | Good | Good | Limited |
| **NAT traversal** | Excellent | Excellent | Tricky | Good |
| **Debugging** | Excellent | Poor | Medium | Poor |
| **Proxy support** | Yes | No | No | Limited |
| **Subscription model** | App-level | App-level | Observe | Streams |
| **Embedded support** | Good | Excellent | Medium | Poor |

---

## Recommendation

### For MASH: **Option 2 - Raw TLS with Length-Prefixed Framing**

**Primary Rationale:**
1. **Smallest code footprint** - fits easily in 256KB
2. **Minimal overhead** - just 4 bytes per message
3. **Simplest implementation** - TLS + trivial framing
4. **Good library support** - mbedTLS, wolfSSL are excellent
5. **Sufficient for local network** - no proxy needs

**Trade-offs Accepted:**
- Must define our own ping/pong for keep-alive
- No browser debugging (but we'll have CLI tools)
- No proxy traversal (not needed for local energy devices)

**Framing Specification:**
```
┌────────────────────────────────────────────────────────┐
│  Length (4 bytes, big-endian)  │  CBOR Message         │
└────────────────────────────────────────────────────────┘

Max message size: 65535 bytes (16-bit would suffice)
Could use 2-byte length for efficiency
```

### Alternative: WebSocket/TLS

If you value:
- Easier debugging (browser tools)
- Future cloud connectivity (proxies)
- Proven SHIP compatibility

The ~13KB extra code is acceptable for many devices.

---

## Keep-Alive Design (for Raw TLS)

Since we're not using WebSocket, we need our own keep-alive:

```cbor
{
  "type": "ping",
  "ts": 1706108400
}

{
  "type": "pong",
  "ts": 1706108400
}
```

**Timing:**
- Send ping every 30 seconds if no activity
- Expect pong within 5 seconds
- Close connection after 3 missed pongs

---

## Next Steps

1. Decide: Raw TLS vs WebSocket
2. Define framing details (length field size)
3. Define keep-alive protocol
4. Choose TLS library for reference implementation
