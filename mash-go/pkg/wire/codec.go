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
func PeekMessageType(data []byte) (MessageType, error) {
	// Decode just enough to check key indicators
	var peek struct {
		MessageID uint32 `cbor:"1,keyasint"`
		Field2    uint8  `cbor:"2,keyasint"` // Could be operation, status, or subscriptionId
	}
	if err := Unmarshal(data, &peek); err != nil {
		return MessageTypeUnknown, fmt.Errorf("failed to peek message: %w", err)
	}

	// Check for notification (messageId = 0)
	if peek.MessageID == NotificationMessageID {
		return MessageTypeNotification, nil
	}

	// Check for control message by looking at the structure
	// Control messages have a different shape (type field instead of messageId)
	var ctrl struct {
		Type uint8 `cbor:"1,keyasint"`
	}
	if err := Unmarshal(data, &ctrl); err == nil {
		if ctrl.Type >= 1 && ctrl.Type <= 3 {
			// Might be control message, verify by checking if it has seq field
			var ctrlCheck struct {
				Type uint8  `cbor:"1,keyasint"`
				Seq  uint32 `cbor:"2,keyasint,omitempty"`
			}
			if Unmarshal(data, &ctrlCheck) == nil && ctrlCheck.Type >= 1 && ctrlCheck.Type <= 3 {
				// Check if this could be a valid request/response instead
				// by checking for endpoint/feature fields
				var reqCheck struct {
					EndpointID uint8 `cbor:"3,keyasint,omitempty"`
				}
				if Unmarshal(data, &reqCheck) == nil && reqCheck.EndpointID > 0 {
					// Has endpoint, not a control message
				} else if peek.MessageID == 0 && peek.Field2 == 0 {
					// MessageId=0, Field2=0 could be notification or control
					// Control messages don't have endpoint/feature
					return MessageTypeControl, nil
				}
			}
		}
	}

	// Determine if request or response by checking if field2 is valid operation or status
	// Operations are 1-4, status codes can be 0-12
	// We need additional context - check for payload structure
	var fullPeek struct {
		MessageID  uint32 `cbor:"1,keyasint"`
		Field2     uint8  `cbor:"2,keyasint"`
		EndpointID uint8  `cbor:"3,keyasint,omitempty"`
		FeatureID  uint8  `cbor:"4,keyasint,omitempty"`
	}
	if err := Unmarshal(data, &fullPeek); err != nil {
		return MessageTypeUnknown, err
	}

	// Requests have endpointId and featureId, responses typically don't
	if fullPeek.Field2 >= 1 && fullPeek.Field2 <= 4 && (fullPeek.EndpointID > 0 || fullPeek.FeatureID > 0) {
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
