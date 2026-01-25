// Package wire defines the CBOR wire format types for the MASH protocol.
//
// MASH uses CBOR (RFC 8949) with integer keys for efficient encoding.
// All messages are length-prefixed and transmitted over TLS 1.3.
//
// # Message Types
//
// There are three primary message types:
//   - Request: Controller to device (Read, Write, Subscribe, Invoke)
//   - Response: Device to controller (success or error)
//   - Notification: Device to controller (subscription updates)
//
// # CBOR Integer Keys
//
// All maps use integer keys for compactness. The key mappings are
// defined as constants in this package.
//
// # Nullable vs Absent
//
// MASH distinguishes between nullable values and absent keys:
//   - Key absent: Attribute not included in this message
//   - Key with value: Attribute has this value
//   - Key with null: Attribute value is explicitly null (cleared)
package wire
