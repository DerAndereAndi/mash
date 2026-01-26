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
// - Control: key 1 is 1-3 (valid ControlMessageType) AND keys 3,4 are absent/zero
// - Request: has endpoint (key 3) or feature (key 4) with meaningful values
// - Response: default for other cases
func PeekMessageType(data []byte) (MessageType, error) {
	// Decode all relevant fields at once for efficiency
	var fullPeek struct {
		Field1     uint32 `cbor:"1,keyasint"` // messageId, or controlType
		Field2     uint8  `cbor:"2,keyasint"` // operation, status, or sequence
		EndpointID uint8  `cbor:"3,keyasint,omitempty"`
		FeatureID  uint8  `cbor:"4,keyasint,omitempty"`
	}
	if err := Unmarshal(data, &fullPeek); err != nil {
		return MessageTypeUnknown, fmt.Errorf("failed to peek message: %w", err)
	}

	// Check for notification (messageId = 0)
	if fullPeek.Field1 == NotificationMessageID {
		return MessageTypeNotification, nil
	}

	// Check for control message: Type (key 1) is 1-3 AND no endpoint/feature
	// Control messages only have keys 1 and 2, while requests have keys 3 and 4
	if fullPeek.Field1 >= 1 && fullPeek.Field1 <= 3 &&
		fullPeek.EndpointID == 0 && fullPeek.FeatureID == 0 {
		return MessageTypeControl, nil
	}

	// Check for request: has endpoint or feature ID
	// Requests always have featureId > 0 for meaningful operations
	if fullPeek.Field2 >= 1 && fullPeek.Field2 <= 4 &&
		(fullPeek.EndpointID > 0 || fullPeek.FeatureID > 0) {
		return MessageTypeRequest, nil
	}

	// Default to response
	return MessageTypeResponse, nil
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
