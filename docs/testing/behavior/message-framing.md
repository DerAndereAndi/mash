# Message Framing Behavior

> Precise specification of wire-level message encoding and parsing

**Status:** Draft
**Created:** 2025-01-25

---

## 1. Overview

MASH messages are transmitted as length-prefixed CBOR payloads over TLS. This document specifies the exact encoding rules, error handling, and compatibility requirements.

**Key principles:**
- Messages are length-prefixed for efficient parsing
- CBOR encoding uses integer keys for compactness
- Unknown keys are ignored (forward compatibility)
- Strict validation with clear error responses

---

## 2. Frame Format

### 2.1 Wire Format

Every MASH message is framed as:

```
┌─────────────────────────────────────────────────────────┐
│ Length (4 bytes) │ CBOR Payload (Length bytes)          │
└─────────────────────────────────────────────────────────┘
```

| Field | Size | Encoding | Description |
|-------|------|----------|-------------|
| Length | 4 bytes | Big-endian unsigned | Payload size (not including length field) |
| Payload | Variable | CBOR | Message content |

### 2.2 Length Field Rules

- **Format:** 32-bit unsigned integer, network byte order (big-endian)
- **Value:** Number of bytes in CBOR payload (0 to 65536)
- **Maximum:** 65536 bytes (64 KB)
- **Minimum:** 1 byte (empty CBOR map is still 1 byte: 0xA0)

**Parsing algorithm:**
```
1. Read exactly 4 bytes
2. If fewer than 4 bytes available: Wait for more data
3. Interpret as big-endian uint32
4. If value > 65536: FATAL ERROR, close connection
5. If value == 0: FATAL ERROR, close connection
6. Read exactly `value` bytes for payload
7. If fewer bytes available: Wait for more data
8. Parse CBOR payload
```

### 2.3 Examples

**Small message (256 bytes):**
```
00 00 01 00 [256 bytes of CBOR]
```

**Typical message (500 bytes):**
```
00 00 01 F4 [500 bytes of CBOR]
```

**Maximum message (65536 bytes):**
```
00 01 00 00 [65536 bytes of CBOR]
```

---

## 3. CBOR Encoding Rules

### 3.1 Supported CBOR Types

| CBOR Type | Supported | MASH Usage |
|-----------|-----------|------------|
| Unsigned integer (0) | Yes | Keys, IDs, enums, positive values |
| Negative integer (1) | Yes | Negative attribute values |
| Byte string (2) | Yes | Binary data, certificates |
| Text string (3) | Yes | Labels, error messages |
| Array (4) | Yes | Lists of IDs, attribute arrays |
| Map (5) | Yes | All structured data |
| Tag (6) | Limited | See 3.6 |
| Simple/Float (7) | Limited | null, bool, float64 only |

### 3.2 Integer Keys

All MASH maps use non-negative integer keys:

```cbor
// Correct
{ 1: "value", 2: 12345 }

// INVALID - string keys not allowed at protocol level
{ "messageId": 1, "operation": 2 }
```

**Key assignment:**
- Keys 1-99: Reserved for protocol-level fields
- Keys 100-255: Reserved for future protocol extension
- Keys >= 256: Available for feature-specific attributes

### 3.3 Key Ordering

**Key ordering is NOT required.**

Parsers MUST accept keys in any order:
```cbor
// Both are valid and equivalent:
{ 1: "a", 2: "b", 3: "c" }
{ 3: "c", 1: "a", 2: "b" }
```

**Recommendation:** Senders SHOULD encode keys in ascending order for:
- Consistent wire format (easier debugging)
- Potential parsing optimization
- Deterministic encoding for signatures

### 3.4 Duplicate Keys

**Duplicate keys are INVALID.**

If a message contains duplicate keys:
```cbor
{ 1: "a", 1: "b" }  // INVALID
```

Receiver MUST:
1. Reject the message with status INVALID_PARAMETER
2. Use the error message: "Duplicate key in message"

### 3.5 Unknown Keys (Forward Compatibility)

**Unknown keys MUST be ignored.**

When a receiver encounters a key it doesn't recognize:
1. Skip the key and its value
2. Continue parsing remaining keys
3. Process known keys normally

**Example:**
```cbor
// Message with unknown key 99
{
  1: 12345,      // messageId (known)
  2: 1,          // operation (known)
  99: "future",  // UNKNOWN - ignore
  3: 1           // endpointId (known)
}
```

Receiver processes messageId=12345, operation=1, endpointId=1. Key 99 is ignored.

**Rationale:** This enables protocol evolution without breaking old implementations.

### 3.6 CBOR Tags

Only these CBOR tags are recognized:

| Tag | Meaning | Usage |
|-----|---------|-------|
| 0 | DateTime string | Timestamps (ISO 8601) |
| 1 | Epoch timestamp | Timestamps (seconds since epoch) |

**Unrecognized tags:** Remove tag wrapper, use inner value.

### 3.7 Float Handling

**Only float64 (IEEE 754 double) is supported.**

- Float16 and float32: Convert to float64 on receive
- Special values: NaN and Infinity are INVALID

```cbor
// Valid
{ 1: 3.14159265358979 }

// Invalid (will cause error)
{ 1: NaN }
{ 1: Infinity }
```

---

## 4. Null vs Absent vs Zero

### 4.1 Semantic Distinction

| Representation | CBOR Encoding | Meaning |
|----------------|---------------|---------|
| Key absent | Key not in map | Field not relevant to this message |
| Key with null | `key: 0xF6` | Field explicitly has no value |
| Key with zero | `key: 0x00` | Field has value zero |

### 4.2 Examples

**Attribute values:**
```cbor
// Limit is 5000 mW
{ 20: 5000000 }

// Limit is explicitly zero (full stop)
{ 20: 0 }

// Limit is explicitly cleared/unset
{ 20: null }

// Limit unchanged (in delta notification)
{ }  // key 20 absent
```

### 4.3 Required vs Optional Fields

| Field Type | If Absent | Error |
|------------|-----------|-------|
| Required | Error | INVALID_PARAMETER: "Missing required field: X" |
| Optional | Use default | No error |

**Required fields by message type:**

| Message Type | Required Fields |
|--------------|-----------------|
| Request | messageId, operation, endpointId, featureId |
| Response | messageId, status |
| Notification | subscriptionId, endpointId, featureId, changes |
| Ping | type, seq |
| Pong | type, seq |
| Close | type, reason |

---

## 5. Integer Range Handling

### 5.1 Supported Ranges

| Type | Range | CBOR Encoding |
|------|-------|---------------|
| uint8 | 0 to 255 | Major 0, value 0-255 |
| uint16 | 0 to 65535 | Major 0 |
| uint32 | 0 to 4,294,967,295 | Major 0 |
| uint64 | 0 to 2^64-1 | Major 0 |
| int64 | -2^63 to 2^63-1 | Major 0 or 1 |

### 5.2 Range Validation

Each field has a defined range. Values outside the range cause:
- Status: CONSTRAINT_ERROR
- Message: "Value out of range for field X"

**Common field ranges:**

| Field | Type | Valid Range |
|-------|------|-------------|
| messageId | uint32 | 0 to 4,294,967,295 |
| endpointId | uint8 | 0 to 255 |
| featureId | uint8 | 0 to 255 |
| subscriptionId | uint32 | 1 to 4,294,967,295 |
| attributeId | uint16 | 1 to 65535 |
| Power values (mW) | int64 | -2^63 to 2^63-1 |
| Current values (mA) | int64 | -2^63 to 2^63-1 |

### 5.3 JavaScript Safe Integer Consideration

JavaScript's safe integer range is -2^53 to 2^53.

**MASH approach:**
- Protocol supports full int64 range
- Implementations using JavaScript MUST use BigInt for int64 values
- Controllers MAY reject values outside safe integer range if they cannot handle BigInt

---

## 6. Message Size Enforcement

### 6.1 Size Limits

| Limit | Value | Enforcement Point |
|-------|-------|-------------------|
| Maximum frame size | 65536 bytes | Length field parsing |
| Maximum payload size | 65536 bytes | Same as frame size |
| Minimum frame size | 1 byte | Length field parsing |

### 6.2 Oversized Message Handling

**Receiver behavior when length > 65536:**

1. Read the 4-byte length field
2. Detect value > 65536
3. DO NOT attempt to read payload
4. Close connection immediately (FATAL)
5. Log error: "Message too large: {length} bytes"

**Rationale:** Reading a huge payload would exhaust memory on constrained devices.

### 6.3 Sender Responsibility

Senders MUST NOT:
- Send messages larger than 65536 bytes
- Send arrays with more than 1000 elements
- Send maps with more than 500 keys
- Send strings longer than 10000 bytes

If data exceeds limits, sender should:
- Split into multiple messages (if applicable)
- Return error to calling application

---

## 7. Error Handling

### 7.1 Parsing Errors

| Error | Severity | Action |
|-------|----------|--------|
| Length > 65536 | FATAL | Close connection |
| Length == 0 | FATAL | Close connection |
| CBOR parse failure | FATAL | Close connection |
| Missing required field | RECOVERABLE | Error response |
| Duplicate key | RECOVERABLE | Error response |
| Invalid value type | RECOVERABLE | Error response |
| Value out of range | RECOVERABLE | Error response |

### 7.2 Fatal vs Recoverable

**FATAL errors:**
- Indicate protocol corruption or attack
- Connection is closed immediately
- No error response is sent
- Trigger reconnection (client side)

**RECOVERABLE errors:**
- Indicate invalid request content
- Error response is sent
- Connection remains open
- Client can retry with corrected request

### 7.3 Error Response Format

```cbor
{
  1: <messageId>,           // From request
  2: <statusCode>,          // Non-zero error code
  3: {
    1: "<error message>"    // Human-readable description
  }
}
```

---

## 8. Message Type Identification

### 8.1 Request/Response Correlation

| Field | Request | Response | Notification |
|-------|---------|----------|--------------|
| messageId | > 0 | Same as request | 0 |
| operation | Present | Absent | Absent |
| subscriptionId | Absent | Absent | Present |

**Identification algorithm:**
```
if messageId == 0:
    → Notification
else if 'operation' key present:
    → Request
else:
    → Response
```

### 8.2 Control Messages

| Type | Identification |
|------|----------------|
| Ping | `{ "type": "ping", "seq": N }` |
| Pong | `{ "type": "pong", "seq": N }` |
| Close | `{ "type": "close", "reason": "...", "code": N }` |
| Close Ack | `{ "type": "close_ack" }` |

Control messages use string keys for the `type` field to distinguish from data messages.

---

## 9. Backwards Compatibility

### 9.1 Version Negotiation

MASH uses ALPN (Application-Layer Protocol Negotiation) in TLS:

| ALPN String | Meaning |
|-------------|---------|
| "mash/1" | MASH protocol version 1 |

Future versions: "mash/2", etc.

### 9.2 Forward Compatibility Rules

To ensure implementations remain compatible with future protocol versions:

1. **Ignore unknown keys** - Already specified in 3.5
2. **Ignore unknown enum values** - Treat as default/unknown
3. **Accept additional array elements** - Process known elements
4. **Accept larger maps** - Process known keys

### 9.3 Breaking vs Non-Breaking Changes

**Non-breaking (minor version):**
- Adding new optional fields
- Adding new enum values
- Adding new features
- Increasing maximum limits

**Breaking (major version):**
- Removing required fields
- Changing field types
- Changing semantic meaning
- Decreasing limits below existing usage

---

## 10. PICS Items

```
# Frame format
MASH.S.FRAME.MAX_SIZE=65536           # Maximum message size in bytes
MASH.S.FRAME.LENGTH_BYTES=4           # Length field size
MASH.S.FRAME.BYTE_ORDER=BIG_ENDIAN    # Length field byte order

# CBOR support
MASH.S.CBOR.IGNORE_UNKNOWN_KEYS=1     # Unknown keys are ignored
MASH.S.CBOR.KEY_ORDER_REQUIRED=0      # Key order is not required
MASH.S.CBOR.DUPLICATE_KEYS_ERROR=1    # Duplicate keys cause error
MASH.S.CBOR.FLOAT64_ONLY=1            # Only float64 supported
MASH.S.CBOR.TAG_0_DATETIME=1          # Tag 0 for datetime supported
MASH.S.CBOR.TAG_1_EPOCH=1             # Tag 1 for epoch supported

# Integer handling
MASH.S.INT.MAX_INT64=1                # Full int64 range supported
MASH.S.INT.BIGINT_REQUIRED=0          # BigInt not required if within int53

# Limits
MASH.S.LIMIT.MAX_ARRAY_ELEMENTS=1000  # Maximum array elements
MASH.S.LIMIT.MAX_MAP_KEYS=500         # Maximum map keys
MASH.S.LIMIT.MAX_STRING_BYTES=10000   # Maximum string length
```

---

## 11. Test Cases

### TC-FRAME-*: Frame Parsing

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-FRAME-1 | Valid small message | 00 00 00 05 + 5 bytes | Parse success |
| TC-FRAME-2 | Valid large message | 00 01 00 00 + 65536 bytes | Parse success |
| TC-FRAME-3 | Oversized length | 00 01 00 01 | FATAL, close |
| TC-FRAME-4 | Zero length | 00 00 00 00 | FATAL, close |
| TC-FRAME-5 | Truncated length | 00 00 00 | Wait for more data |
| TC-FRAME-6 | Truncated payload | 00 00 00 10 + 5 bytes | Wait for more data |

### TC-CBOR-*: CBOR Parsing

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-CBOR-1 | Valid map with int keys | {1: "a", 2: "b"} | Parse success |
| TC-CBOR-2 | Keys out of order | {2: "b", 1: "a"} | Parse success |
| TC-CBOR-3 | Duplicate keys | {1: "a", 1: "b"} | Error INVALID_PARAMETER |
| TC-CBOR-4 | Unknown key ignored | {1: "a", 99: "x"} | Parse success, 99 ignored |
| TC-CBOR-5 | String key | {"a": 1} | Error (protocol level) |
| TC-CBOR-6 | NaN value | {1: NaN} | Error INVALID_PARAMETER |
| TC-CBOR-7 | Infinity value | {1: Infinity} | Error INVALID_PARAMETER |

### TC-NULL-*: Null Handling

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-NULL-1 | Explicit null | {1: null} | Field value is null |
| TC-NULL-2 | Key absent | {} | Field uses default |
| TC-NULL-3 | Value zero | {1: 0} | Field value is zero |
| TC-NULL-4 | Required field missing | {} for required | Error INVALID_PARAMETER |

### TC-INT-*: Integer Handling

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-INT-1 | Max uint32 | {1: 4294967295} | Parse success |
| TC-INT-2 | Max int64 | {1: 9223372036854775807} | Parse success |
| TC-INT-3 | Negative int64 | {1: -9223372036854775808} | Parse success |
| TC-INT-4 | Value exceeds range | endpointId: 256 | Error CONSTRAINT_ERROR |

### TC-COMPAT-*: Compatibility

| ID | Description | Input | Expected |
|----|-------------|-------|----------|
| TC-COMPAT-1 | Future key ignored | Request with key 200 | Process normally |
| TC-COMPAT-2 | Extra array element | [1, 2, 3, 4] when expecting 3 | Process first 3 |
| TC-COMPAT-3 | Unknown enum value | status: 255 | Treat as unknown |

---

## 12. Implementation Recommendations

### 12.1 CBOR Libraries

Recommended libraries with MASH-compatible features:

| Language | Library | Notes |
|----------|---------|-------|
| Go | fxamacker/cbor | Supports all features |
| Rust | ciborium | Good performance |
| Python | cbor2 | Pure Python, easy to use |
| JavaScript | cbor-x | Supports BigInt for int64 |
| C | tinycbor | Suitable for constrained devices |

### 12.2 Constrained Device Considerations

For devices with limited memory:

1. **Streaming parser:** Parse CBOR incrementally, don't buffer entire message
2. **Fixed buffers:** Pre-allocate message buffer at max size
3. **Early validation:** Check length field before allocating
4. **Reject oversize:** Fail fast on oversized messages

### 12.3 Security Considerations

1. **Length validation:** Always validate length before reading payload
2. **Recursion limit:** Limit CBOR nesting depth (recommended: 16)
3. **String limits:** Enforce string length limits before allocation
4. **Array limits:** Enforce array size limits before allocation
