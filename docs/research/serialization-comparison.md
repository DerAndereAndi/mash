# Serialization Format Comparison

**Date:** 2025-01-24
**Context:** Choosing serialization format for MASH protocol
**Target:** 256KB RAM MCU (ESP32-class)

---

## Test Message: Load Control Command

A realistic message for EV charging power limit control:

```
Device: energy-manager-001
Target: evse-wallbox-042
Command: SetPowerLimit
Parameters:
  - limitId: 1
  - value: 7400 (watts)
  - duration: 3600 (seconds)
  - isActive: true
Timestamp: 1706108400
MessageId: 42
```

---

## Format Comparison

### 1. JSON

```json
{"d":"energy-manager-001","t":"evse-wallbox-042","c":"SetPowerLimit","p":{"limitId":1,"value":7400,"duration":3600,"isActive":true},"ts":1706108400,"id":42}
```

| Metric | Value |
|--------|-------|
| **Size** | 156 bytes |
| **Parse complexity** | Low (standard libraries) |
| **Human readable** | Yes |
| **Schema validation** | External (JSON Schema) |
| **Go library** | `encoding/json` (stdlib) |
| **C library** | cJSON, jsmn (~10KB code) |
| **Streaming parse** | Difficult |

**Pros:**
- Human-readable for debugging
- Universal tooling (curl, jq, browsers)
- No code generation needed
- Everyone knows it

**Cons:**
- Largest message size
- String keys repeated in every message
- No native binary data (must Base64 encode)
- Parsing allocates memory unpredictably

---

### 2. CBOR (Concise Binary Object Representation)

```
Binary (hex): A6 61 64 71 65 6E 65 72 67 79 2D 6D 61 6E 61 67 65 72 2D 30 30 31 61 74 70 65 76 73 65 2D 77 61 6C 6C 62 6F 78 2D 30 34 32 61 63 6D 53 65 74 50 6F 77 65 72 4C 69 6D 69 74 61 70 A4 67 6C 69 6D 69 74 49 64 01 65 76 61 6C 75 65 19 1C E8 68 64 75 72 61 74 69 6F 6E 19 0E 10 68 69 73 41 63 74 69 76 65 F5 62 74 73 1A 65 B2 E4 90 62 69 64 18 2A
```

| Metric | Value |
|--------|-------|
| **Size** | 108 bytes (31% smaller than JSON) |
| **Parse complexity** | Low-Medium |
| **Human readable** | No (but tools exist) |
| **Schema validation** | CDDL schema language |
| **Go library** | `github.com/fxamacker/cbor` |
| **C library** | tinycbor (~8KB code) |
| **Streaming parse** | Yes |

**Pros:**
- Compact binary format
- Self-describing (like JSON)
- Native binary data support
- Streaming/incremental parsing
- Used by Matter, CoAP, COSE (security tokens)
- No code generation required

**Cons:**
- Not human-readable in raw form
- Less universal tooling than JSON
- Still has string key overhead (can use integer keys)

**With integer keys (optimized):**
```
Size: ~75 bytes (52% smaller than JSON)
```

---

### 3. MessagePack

```
Binary similar to CBOR, slightly different encoding
```

| Metric | Value |
|--------|-------|
| **Size** | 105 bytes (33% smaller than JSON) |
| **Parse complexity** | Low |
| **Human readable** | No |
| **Schema validation** | None built-in |
| **Go library** | `github.com/vmihailenco/msgpack` |
| **C library** | msgpack-c (~15KB code) |
| **Streaming parse** | Yes |

**Pros:**
- Very similar to CBOR
- Widely adopted (Redis, Fluentd)
- Simple spec

**Cons:**
- Less standardized extensions than CBOR
- Not used by major IoT standards
- No IETF standardization (CBOR is RFC 8949)

---

### 4. Protocol Buffers

```protobuf
// Requires schema definition
message LoadControlCommand {
  string device = 1;
  string target = 2;
  string command = 3;
  LoadControlParams params = 4;
  uint64 timestamp = 5;
  uint32 message_id = 6;
}
```

| Metric | Value |
|--------|-------|
| **Size** | ~85 bytes (45% smaller than JSON) |
| **Parse complexity** | Low (generated code) |
| **Human readable** | No |
| **Schema validation** | Built-in (proto files) |
| **Go library** | `google.golang.org/protobuf` |
| **C library** | nanopb (~5KB code) |
| **Streaming parse** | Limited |

**Pros:**
- Most compact format
- Excellent tooling (protoc, grpc)
- Strong schema evolution guarantees
- Generated code = type safety

**Cons:**
- Requires code generation step
- Schema must be shared ahead of time
- Not self-describing (need schema to decode)
- More complex build process
- Adds dependency on protoc toolchain

---

### 5. Custom TLV (Matter-style)

Matter uses a custom Type-Length-Value encoding optimized for their needs.

| Metric | Value |
|--------|-------|
| **Size** | ~70-80 bytes |
| **Parse complexity** | Medium (custom parser) |
| **Human readable** | No |
| **Schema validation** | Custom |
| **Library availability** | Must write ourselves |
| **Streaming parse** | Yes |

**Pros:**
- Maximum control over encoding
- Can optimize for specific use cases

**Cons:**
- Must write and maintain parser/encoder
- No standard tooling
- Higher implementation risk
- Harder for third parties to implement

---

## Summary Matrix

| Format | Size | Parse RAM | Complexity | Tooling | Std? |
|--------|------|-----------|------------|---------|------|
| JSON | 156B | High | Low | Excellent | IETF |
| CBOR | 75-108B | Low | Low-Med | Good | IETF RFC 8949 |
| MsgPack | 105B | Low | Low | Good | No |
| Protobuf | 85B | Low | Low* | Excellent | Google |
| Custom TLV | 70B | Lowest | High | None | No |

*Low complexity for users, high for implementers

---

## Recommendation

### For 256KB RAM target: **CBOR with integer keys**

**Rationale:**

1. **Size/Complexity Balance**: 52% smaller than JSON without code generation
2. **IETF Standard**: RFC 8949, not proprietary
3. **IoT Adoption**: Used by Matter, CoAP, Thread - proven in embedded
4. **Self-Describing**: Can decode without schema (debugging)
5. **Integer Keys**: Use numeric field IDs for compactness when needed
6. **Security Fit**: COSE (CBOR Object Signing and Encryption) for security tokens
7. **Streaming**: Can parse without loading entire message into RAM

**Trade-off accepted:**
- Less human-readable than JSON
- Mitigation: Build CLI tool with `cbor2json` conversion for debugging

### Alternative consideration: **Protocol Buffers**

If you value:
- Maximum compactness
- Strong typing guarantees
- Excellent tooling

The code generation requirement is manageable for a well-defined protocol.

---

## Next Steps

1. Decide: CBOR vs Protobuf (or JSON if debugging priority)
2. Define message schema/structure
3. Create reference encoder/decoder
4. Benchmark on target hardware (ESP32)

---

## References

- [RFC 8949 - CBOR](https://datatracker.ietf.org/doc/html/rfc8949)
- [CBOR Go library](https://github.com/fxamacker/cbor)
- [tinycbor for embedded](https://github.com/nickcona/tinycbor)
- [Protocol Buffers](https://protobuf.dev/)
- [nanopb - Protobuf for embedded](https://github.com/nanopb/nanopb)
