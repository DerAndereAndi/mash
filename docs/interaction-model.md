# MASH Interaction Model

> Read, Write, Subscribe, Invoke operations and CBOR serialization

**Status:** Draft
**Last Updated:** 2025-01-25

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Protocol Overview](protocol-overview.md) | Vision, architecture, device model |
| [Transport](transport.md) | Framing, connection model |
| [Features](features/README.md) | Feature definitions |

---

## 1. Overview

MASH has 4 operations (like Matter, not SPINE's 7):

| Operation | Description | Direction |
|-----------|-------------|-----------|
| **Read** | Get current attribute values | Controller → Device |
| **Write** | Set attribute value (full replace) | Controller → Device |
| **Subscribe** | Register for change notifications | Controller → Device |
| **Invoke** | Execute command with parameters | Controller → Device |

---

## 2. Data Model

### 2.1 Serialization

**Format:** CBOR (RFC 8949)

**Why CBOR:**
- Binary format (smaller than JSON)
- Self-describing
- Integer keys for compactness
- Native support for binary data
- Wide language support

### 2.2 Integer Keys

All CBOR maps use integer keys for efficiency:

```cbor
// Instead of:
{ "acActivePower": 5000000, "acReactivePower": 200000 }

// MASH uses:
{ 1: 5000000, 2: 200000 }
```

### 2.3 Message Size Target

| Metric | Target |
|--------|--------|
| Typical message | < 2 KB |
| Maximum message | 64 KB |
| EEBUS comparison | 4 KB+ typical |

---

## 3. Message Format

### 3.1 Request Message

```cbor
{
  1: messageId,        // uint32: unique per connection
  2: operation,        // uint8: 1=Read, 2=Write, 3=Subscribe, 4=Invoke
  3: endpointId,       // uint8: target endpoint
  4: featureId,        // uint8: target feature
  5: payload           // operation-specific data
}
```

### 3.2 Response Message

```cbor
{
  1: messageId,        // uint32: matches request
  2: status,           // uint8: 0=success, or error code
  3: payload           // operation-specific response data (if success)
}
```

### 3.3 Notification Message (from subscriptions)

```cbor
{
  1: 0,                // messageId 0 = notification
  2: subscriptionId,   // uint32: subscription that triggered this
  3: endpointId,       // uint8
  4: featureId,        // uint8
  5: changes           // map of changed attributes
}
```

---

## 4. Read Operation

### 4.1 Read Request

```cbor
{
  1: 12345,            // messageId
  2: 1,                // operation: Read
  3: 1,                // endpointId
  4: 2,                // featureId (e.g., Measurement)
  5: [1, 2, 3]         // attributeIds to read (empty = all)
}
```

### 4.2 Read Response

```cbor
{
  1: 12345,            // messageId
  2: 0,                // status: success
  3: {                 // attribute values
    1: 5000000,        // acActivePower
    2: 200000,         // acReactivePower
    3: 5004000         // acApparentPower
  }
}
```

### 4.3 Read All Attributes

To read all attributes, send empty array:

```cbor
{
  1: 12346,
  2: 1,                // Read
  3: 1,
  4: 2,
  5: []                // empty = read all
}
```

---

## 5. Write Operation

### 5.1 Write Request

```cbor
{
  1: 12347,            // messageId
  2: 2,                // operation: Write
  3: 1,                // endpointId
  4: 3,                // featureId (e.g., EnergyControl)
  5: {                 // attributes to write
    21: 6000000        // myConsumptionLimit = 6kW
  }
}
```

### 5.2 Write Response

```cbor
{
  1: 12347,            // messageId
  2: 0,                // status: success
  3: {                 // resulting values (may differ from requested)
    20: 5000000,       // effectiveConsumptionLimit (min of all zones)
    21: 6000000        // myConsumptionLimit
  }
}
```

### 5.3 Key Simplifications

- **No partial updates** - Write replaces entire attribute value
- **No deleteAll/replaceAll** - Just Write with new value
- **Arrays are replaced entirely** - No append/remove operations

---

## 6. Subscribe Operation

### 6.1 Subscribe Request

```cbor
{
  1: 12348,            // messageId
  2: 3,                // operation: Subscribe
  3: 1,                // endpointId
  4: 2,                // featureId
  5: {
    1: [1, 2, 3],      // attributeIds to subscribe (empty = all)
    2: 1000,           // minInterval (ms) - don't notify faster than this
    3: 60000           // maxInterval (ms) - notify at least this often
  }
}
```

### 6.2 Subscribe Response

```cbor
{
  1: 12348,            // messageId
  2: 0,                // status: success
  3: {
    1: 5001,           // subscriptionId
    2: {               // current values (initial snapshot)
      1: 5000000,
      2: 200000,
      3: 5004000
    }
  }
}
```

### 6.3 Change Notification

When subscribed attributes change:

```cbor
{
  1: 0,                // messageId 0 = notification
  2: 5001,             // subscriptionId
  3: 1,                // endpointId
  4: 2,                // featureId
  5: {                 // changed attributes only
    1: 5500000         // acActivePower changed
  }
}
```

### 6.4 Unsubscribe

```cbor
{
  1: 12349,
  2: 3,                // Subscribe operation
  3: 0,                // endpointId 0 = unsubscribe
  4: 0,                // featureId 0
  5: {
    1: 5001            // subscriptionId to cancel
  }
}
```

### 6.5 Subscription Behavior

| Parameter | Description |
|-----------|-------------|
| minInterval | Minimum time between notifications (coalescing) |
| maxInterval | Maximum time without notification (heartbeat) |
| On reconnect | Subscriptions must be re-established |

---

## 7. Invoke Operation

### 7.1 Invoke Request

```cbor
{
  1: 12350,            // messageId
  2: 4,                // operation: Invoke
  3: 1,                // endpointId
  4: 3,                // featureId (EnergyControl)
  5: {
    1: 1,              // commandId: SetLimit
    2: {               // command parameters
      1: 6000000,      // consumptionLimit
      4: 2             // cause: LOCAL_PROTECTION
    }
  }
}
```

### 7.2 Invoke Response

```cbor
{
  1: 12350,            // messageId
  2: 0,                // status: success
  3: {                 // command response
    1: true,           // success
    2: 5000000,        // effectiveConsumptionLimit
    3: null            // effectiveProductionLimit (unchanged)
  }
}
```

---

## 8. Error Handling

### 8.1 Status Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | SUCCESS | Operation completed successfully |
| 1 | INVALID_ENDPOINT | Endpoint doesn't exist |
| 2 | INVALID_FEATURE | Feature doesn't exist on endpoint |
| 3 | INVALID_ATTRIBUTE | Attribute doesn't exist |
| 4 | INVALID_COMMAND | Command doesn't exist |
| 5 | INVALID_PARAMETER | Parameter value out of range |
| 6 | READ_ONLY | Cannot write to read-only attribute |
| 7 | WRITE_ONLY | Cannot read from write-only attribute |
| 8 | NOT_AUTHORIZED | Zone doesn't have permission |
| 9 | BUSY | Device is busy, try again |
| 10 | UNSUPPORTED | Operation not supported |
| 11 | CONSTRAINT_ERROR | Value violates constraint |
| 12 | TIMEOUT | Operation timed out |

### 8.2 Error Response

```cbor
{
  1: 12345,            // messageId
  2: 5,                // status: INVALID_PARAMETER
  3: {
    1: "consumptionLimit must be >= 0"  // error message (optional)
  }
}
```

---

## 9. Events

### 9.1 Event Model

Events are timestamped, monotonically-numbered records:

```cbor
{
  1: eventNumber,      // uint64: monotonic counter
  2: timestamp,        // uint64: Unix timestamp (ms)
  3: endpointId,       // uint8
  4: featureId,        // uint8
  5: eventId,          // uint8
  6: data              // event-specific payload
}
```

### 9.2 Event Subscription

Events are delivered through the same subscription mechanism:

```cbor
// Subscribe to events
{
  1: 12351,
  2: 3,                // Subscribe
  3: 1,                // endpointId
  4: 4,                // featureId (Status)
  5: {
    4: [1, 2]          // eventIds to subscribe
  }
}
```

### 9.3 Event Properties

- Events are append-only (never modified)
- Event numbers are monotonically increasing
- Missed events can be detected by gaps in numbers
- Events are persisted across reconnections (limited buffer)

---

## 10. Addressing

### 10.1 Address Format

```
device_id / endpoint_id / feature_id / attribute_or_command
evse-001  / 1           / Measurement / acActivePower
```

### 10.2 Wildcards

Not supported. Each request targets specific endpoint/feature.

---

## 11. Comparison with EEBUS SPINE

| Aspect | EEBUS SPINE | MASH |
|--------|-------------|------|
| Operations | 7 RFE modes | 4 operations |
| Serialization | JSON (verbose) | CBOR (compact) |
| Partial updates | Complex partial mechanisms | Full replace only |
| Bindings | Separate from subscriptions | Unified subscription |
| Events | Function-based | Dedicated event model |
| Error codes | Generic status | Specific error codes |

**Key simplifications:**
- 4 operations instead of 7 RFE modes
- No partial updates complexity
- Unified subscription model
- Binary format for efficiency
