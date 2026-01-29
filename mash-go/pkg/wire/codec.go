package wire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"
)

// encMode is the CBOR encoder mode for MASH messages.
// Configured for deterministic encoding with integer keys.
var encMode cbor.EncMode

// decMode is the CBOR decoder mode for MASH messages.
var decMode cbor.DecMode

func init() {
	var err error

	// Configure encoder for deterministic output
	encOpts := cbor.EncOptions{
		Sort:          cbor.SortCanonical, // Deterministic key ordering
		IndefLength:   cbor.IndefLengthForbidden,
		NilContainers: cbor.NilContainerAsNull,
		Time:          cbor.TimeUnix, // Unix timestamps
	}
	encMode, err = encOpts.EncMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create CBOR encoder mode: %v", err))
	}

	// Configure decoder to be lenient for forward compatibility
	decOpts := cbor.DecOptions{
		DupMapKey:         cbor.DupMapKeyQuiet, // Ignore duplicate keys (last wins)
		IndefLength:       cbor.IndefLengthAllowed,
		ExtraReturnErrors: cbor.ExtraDecErrorNone,
	}
	decMode, err = decOpts.DecMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create CBOR decoder mode: %v", err))
	}
}

// Marshal encodes a value to CBOR bytes.
func Marshal(v any) ([]byte, error) {
	return encMode.Marshal(v)
}

// Unmarshal decodes CBOR bytes into a value.
func Unmarshal(data []byte, v any) error {
	return decMode.Unmarshal(data, v)
}

// NewEncoder creates a new CBOR encoder that writes to w.
func NewEncoder(w io.Writer) *cbor.Encoder {
	return encMode.NewEncoder(w)
}

// NewDecoder creates a new CBOR decoder that reads from r.
func NewDecoder(r io.Reader) *cbor.Decoder {
	return decMode.NewDecoder(r)
}

// EncodeRequest encodes a request message to CBOR bytes.
func EncodeRequest(req *Request) ([]byte, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	return Marshal(req)
}

// DecodeRequest decodes CBOR bytes into a request message.
func DecodeRequest(data []byte) (*Request, error) {
	var req Request
	if err := Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to decode request: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	return &req, nil
}

// EncodeResponse encodes a response message to CBOR bytes.
func EncodeResponse(resp *Response) ([]byte, error) {
	return Marshal(resp)
}

// DecodeResponse decodes CBOR bytes into a response message.
func DecodeResponse(data []byte) (*Response, error) {
	var resp Response
	if err := Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}

// EncodeNotification encodes a notification message to CBOR bytes.
// Notifications have messageId=0 which is handled automatically.
func EncodeNotification(notif *Notification) ([]byte, error) {
	// Create the wire format with messageId=0
	wireMsg := struct {
		MessageID      uint32         `cbor:"1,keyasint"`
		SubscriptionID uint32         `cbor:"2,keyasint"`
		EndpointID     uint8          `cbor:"3,keyasint"`
		FeatureID      uint8          `cbor:"4,keyasint"`
		Changes        map[uint16]any `cbor:"5,keyasint"`
	}{
		MessageID:      NotificationMessageID,
		SubscriptionID: notif.SubscriptionID,
		EndpointID:     notif.EndpointID,
		FeatureID:      notif.FeatureID,
		Changes:        notif.Changes,
	}
	return Marshal(wireMsg)
}

// DecodeNotification decodes CBOR bytes into a notification message.
func DecodeNotification(data []byte) (*Notification, error) {
	// First decode to check messageId
	var wireMsg struct {
		MessageID      uint32         `cbor:"1,keyasint"`
		SubscriptionID uint32         `cbor:"2,keyasint"`
		EndpointID     uint8          `cbor:"3,keyasint"`
		FeatureID      uint8          `cbor:"4,keyasint"`
		Changes        map[uint16]any `cbor:"5,keyasint"`
	}
	if err := Unmarshal(data, &wireMsg); err != nil {
		return nil, fmt.Errorf("failed to decode notification: %w", err)
	}
	if wireMsg.MessageID != NotificationMessageID {
		return nil, fmt.Errorf("not a notification message: messageId=%d", wireMsg.MessageID)
	}
	return &Notification{
		SubscriptionID: wireMsg.SubscriptionID,
		EndpointID:     wireMsg.EndpointID,
		FeatureID:      wireMsg.FeatureID,
		Changes:        wireMsg.Changes,
	}, nil
}

// EncodeControlMessage encodes a control message (ping/pong/close) to CBOR bytes.
func EncodeControlMessage(msg *ControlMessage) ([]byte, error) {
	return Marshal(msg)
}

// DecodeControlMessage decodes CBOR bytes into a control message.
func DecodeControlMessage(data []byte) (*ControlMessage, error) {
	var msg ControlMessage
	if err := Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode control message: %w", err)
	}
	return &msg, nil
}

// MessageType represents the type of a decoded message.
type MessageType int

const (
	MessageTypeUnknown MessageType = iota
	MessageTypeRequest
	MessageTypeResponse
	MessageTypeNotification
	MessageTypeControl
)

// PeekMessageType examines CBOR data to determine the message type
// without fully decoding it.
//
// Message type detection logic:
// - Notification: messageId (key 1) = 0
// - Control: key 1 is 1-3 (valid ControlMessageType) AND no payload at key 3
// - Request: has endpoint (key 3) and feature (key 4) as integers
// - Response: has payload at key 3 as a map, or status > 4
func PeekMessageType(data []byte) (MessageType, error) {
	// Decode into a generic map to inspect field types
	var rawMsg map[uint64]any
	if err := Unmarshal(data, &rawMsg); err != nil {
		return MessageTypeUnknown, fmt.Errorf("failed to peek message: %w", err)
	}

	// Get field 1 (messageId / controlType)
	field1, _ := toUint32(rawMsg[1])

	// Check for notification (messageId = 0)
	if field1 == NotificationMessageID {
		return MessageTypeNotification, nil
	}

	// Get field 2 (operation / status / sequence)
	field2, _ := toUint8(rawMsg[2])

	// Check field 3 to distinguish Request from Response
	// Request has uint8 EndpointID at key 3
	// Response has map or nil payload at key 3
	field3 := rawMsg[3]

	// If field 3 is a map or absent with status value > 4, it's a Response
	if field3 != nil {
		switch field3.(type) {
		case map[any]any, map[uint64]any, map[string]any:
			// Field 3 is a map -> Response payload
			return MessageTypeResponse, nil
		}
	}

	// Check for control message: Type (key 1) is 1-3 AND no endpoint/feature fields
	// Must check this BEFORE the field2 > 4 check because control messages use
	// field 2 as a sequence number which can be any uint32 value
	if field1 >= 1 && field1 <= 3 {
		_, hasField3 := rawMsg[3]
		_, hasField4 := rawMsg[4]
		if !hasField3 && !hasField4 {
			return MessageTypeControl, nil
		}
	}

	// If field 2 is a valid status (0-13) and > 4 (not a valid operation), it's Response
	// Status values 5-13 are not valid Operation values
	if field2 > 4 {
		return MessageTypeResponse, nil
	}

	// Check for request: field 2 is valid operation (1-4) and has endpoint/feature
	if field2 >= 1 && field2 <= 4 {
		endpointID, _ := toUint8(field3)
		featureID, _ := toUint8(rawMsg[4])
		if endpointID > 0 || featureID > 0 {
			return MessageTypeRequest, nil
		}
		// EndpointID can be 0 (DEVICE_ROOT), so also check if field 4 exists
		if _, hasField4 := rawMsg[4]; hasField4 {
			return MessageTypeRequest, nil
		}
	}

	// Default to response
	return MessageTypeResponse, nil
}

// toUint32 converts various numeric types to uint32.
func toUint32(v any) (uint32, bool) {
	switch n := v.(type) {
	case uint64:
		return uint32(n), true
	case uint32:
		return n, true
	case int64:
		return uint32(n), true
	case int:
		return uint32(n), true
	default:
		return 0, false
	}
}

// toUint8 converts various numeric types to uint8.
func toUint8(v any) (uint8, bool) {
	switch n := v.(type) {
	case uint64:
		return uint8(n), true
	case uint32:
		return uint8(n), true
	case int64:
		return uint8(n), true
	case int:
		return uint8(n), true
	default:
		return 0, false
	}
}

// Clone creates a deep copy of the CBOR data by re-encoding.
// Useful for copying messages without shared references.
func Clone[T any](v T) (T, error) {
	var result T
	data, err := Marshal(v)
	if err != nil {
		return result, err
	}
	err = Unmarshal(data, &result)
	return result, err
}

// Equal compares two values by their CBOR encoding.
func Equal(a, b any) bool {
	dataA, errA := Marshal(a)
	dataB, errB := Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return bytes.Equal(dataA, dataB)
}

// ToInt64 safely converts any numeric type to int64.
// CBOR decoding into `any` may produce different integer types depending on value size,
// so this helper handles all cases. Returns (0, false) if v is not a numeric type.
func ToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int16:
		return int64(n), true
	case int8:
		return int64(n), true
	case uint64:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint8:
		return int64(n), true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	default:
		return 0, false
	}
}

// ToUint32 safely converts any numeric type to uint32.
// CBOR decoding into `any` may produce different integer types depending on value size.
func ToUint32(v any) (uint32, bool) {
	switch n := v.(type) {
	case uint32:
		return n, true
	case uint64:
		return uint32(n), true
	case uint16:
		return uint32(n), true
	case uint8:
		return uint32(n), true
	case int64:
		return uint32(n), true
	case int32:
		return uint32(n), true
	case int16:
		return uint32(n), true
	case int8:
		return uint32(n), true
	case int:
		return uint32(n), true
	default:
		return 0, false
	}
}

// ToUint8Public safely converts any numeric type to uint8.
// This is the public version of toUint8 for use by notification handlers.
func ToUint8Public(v any) (uint8, bool) {
	switch n := v.(type) {
	case uint8:
		return n, true
	case uint64:
		return uint8(n), true
	case uint32:
		return uint8(n), true
	case uint16:
		return uint8(n), true
	case int64:
		return uint8(n), true
	case int32:
		return uint8(n), true
	case int16:
		return uint8(n), true
	case int8:
		return uint8(n), true
	case int:
		return uint8(n), true
	default:
		return 0, false
	}
}

// ToStringMap normalizes a CBOR-decoded map to map[string]any.
// CBOR decodes maps into map[any]any when the target is any.
// This converts both map[any]any and map[string]any to map[string]any.
// Returns nil if the value is not a map type.
func ToStringMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("%v", k)] = val
		}
		return result
	default:
		return nil
	}
}
