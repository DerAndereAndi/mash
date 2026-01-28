package log

import (
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Event represents a protocol log event captured at any layer.
// CBOR encoding uses integer keys for compactness.
type Event struct {
	// Timestamp when the event occurred (nanosecond precision).
	Timestamp time.Time `cbor:"1,keyasint"`

	// ConnectionID uniquely identifies the connection (UUID).
	ConnectionID string `cbor:"2,keyasint"`

	// Direction indicates message flow.
	Direction Direction `cbor:"3,keyasint"`

	// Layer where the event was captured.
	Layer Layer `cbor:"4,keyasint"`

	// Category classifies the event type.
	Category Category `cbor:"5,keyasint"`

	// LocalRole indicates whether this is a device or controller.
	LocalRole Role `cbor:"6,keyasint,omitempty"`

	// RemoteAddr is the peer address (IP:port).
	RemoteAddr string `cbor:"7,keyasint,omitempty"`

	// DeviceID is the device identifier (populated after commissioning).
	DeviceID string `cbor:"8,keyasint,omitempty"`

	// ZoneID is the zone identifier (populated after commissioning).
	ZoneID string `cbor:"9,keyasint,omitempty"`

	// Type-specific payload (one of these will be set).
	Frame       *FrameEvent       `cbor:"10,keyasint,omitempty"` // Transport layer
	Message     *MessageEvent     `cbor:"11,keyasint,omitempty"` // Wire layer (decoded)
	StateChange *StateChangeEvent `cbor:"12,keyasint,omitempty"` // Connection/session state
	ControlMsg  *ControlMsgEvent  `cbor:"13,keyasint,omitempty"` // Ping/pong/close
	Error       *ErrorEventData   `cbor:"14,keyasint,omitempty"` // Errors at any layer
}

// Direction indicates the direction of message flow.
type Direction uint8

const (
	// DirectionIn indicates an incoming message.
	DirectionIn Direction = 0
	// DirectionOut indicates an outgoing message.
	DirectionOut Direction = 1
)

// String returns the direction name.
func (d Direction) String() string {
	switch d {
	case DirectionIn:
		return "IN"
	case DirectionOut:
		return "OUT"
	default:
		return "UNKNOWN"
	}
}

// Layer indicates which protocol layer captured the event.
type Layer uint8

const (
	// LayerTransport is the framing layer (raw bytes).
	LayerTransport Layer = 0
	// LayerWire is the message encoding layer (decoded CBOR).
	LayerWire Layer = 1
	// LayerService is the application/service layer.
	LayerService Layer = 2
)

// String returns the layer name.
func (l Layer) String() string {
	switch l {
	case LayerTransport:
		return "TRANSPORT"
	case LayerWire:
		return "WIRE"
	case LayerService:
		return "SERVICE"
	default:
		return "UNKNOWN"
	}
}

// Category classifies the event type.
type Category uint8

const (
	// CategoryMessage indicates a protocol message (request/response/notification).
	CategoryMessage Category = 0
	// CategoryControl indicates a control message (ping/pong/close).
	CategoryControl Category = 1
	// CategoryState indicates a state change.
	CategoryState Category = 2
	// CategoryError indicates an error event.
	CategoryError Category = 3
)

// String returns the category name.
func (c Category) String() string {
	switch c {
	case CategoryMessage:
		return "MESSAGE"
	case CategoryControl:
		return "CONTROL"
	case CategoryState:
		return "STATE"
	case CategoryError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Role indicates whether the local endpoint is a device or controller.
type Role uint8

const (
	// RoleDevice indicates this is a device.
	RoleDevice Role = 0
	// RoleController indicates this is a controller.
	RoleController Role = 1
)

// String returns the role name.
func (r Role) String() string {
	switch r {
	case RoleDevice:
		return "DEVICE"
	case RoleController:
		return "CONTROLLER"
	default:
		return "UNKNOWN"
	}
}

// FrameEvent captures raw frame data at the transport layer.
type FrameEvent struct {
	// Size is the frame size in bytes (including length prefix).
	Size int `cbor:"1,keyasint"`

	// Data is the raw frame bytes (may be truncated for large frames).
	Data []byte `cbor:"2,keyasint,omitempty"`

	// Truncated indicates if Data was truncated.
	Truncated bool `cbor:"3,keyasint,omitempty"`
}

// MessageEvent captures a decoded protocol message at the wire layer.
type MessageEvent struct {
	// Type distinguishes request/response/notification.
	Type MessageType `cbor:"1,keyasint"`

	// MessageID correlates request/response pairs (0 for notifications).
	MessageID uint32 `cbor:"2,keyasint"`

	// For requests: the operation being performed.
	Operation *wire.Operation `cbor:"3,keyasint,omitempty"`

	// For requests: the target endpoint ID.
	EndpointID *uint8 `cbor:"4,keyasint,omitempty"`

	// For requests: the target feature ID.
	FeatureID *uint8 `cbor:"5,keyasint,omitempty"`

	// For responses: the status code.
	Status *wire.Status `cbor:"6,keyasint,omitempty"`

	// For notifications: the subscription ID.
	SubscriptionID *uint32 `cbor:"7,keyasint,omitempty"`

	// Decoded payload (CBOR-compatible representation).
	Payload any `cbor:"8,keyasint,omitempty"`

	// ProcessingTime is the duration from request receipt to response send (response only).
	// Stored as nanoseconds.
	ProcessingTime *time.Duration `cbor:"9,keyasint,omitempty"`
}

// MessageType distinguishes request/response/notification.
type MessageType uint8

const (
	// MessageTypeRequest indicates a request message.
	MessageTypeRequest MessageType = 0
	// MessageTypeResponse indicates a response message.
	MessageTypeResponse MessageType = 1
	// MessageTypeNotification indicates a notification message.
	MessageTypeNotification MessageType = 2
)

// String returns the message type name.
func (m MessageType) String() string {
	switch m {
	case MessageTypeRequest:
		return "REQUEST"
	case MessageTypeResponse:
		return "RESPONSE"
	case MessageTypeNotification:
		return "NOTIFICATION"
	default:
		return "UNKNOWN"
	}
}

// StateChangeEvent captures connection and session lifecycle events.
type StateChangeEvent struct {
	// Entity being changed.
	Entity StateEntity `cbor:"1,keyasint"`

	// OldState is the previous state (may be empty).
	OldState string `cbor:"2,keyasint,omitempty"`

	// NewState is the new state.
	NewState string `cbor:"3,keyasint"`

	// Reason for the change (if available).
	Reason string `cbor:"4,keyasint,omitempty"`
}

// StateEntity indicates what entity changed state.
type StateEntity uint8

const (
	// StateEntityConnection indicates a connection state change.
	StateEntityConnection StateEntity = 0
	// StateEntitySession indicates a session state change.
	StateEntitySession StateEntity = 1
	// StateEntityCommissioning indicates a commissioning state change.
	StateEntityCommissioning StateEntity = 2
)

// String returns the state entity name.
func (s StateEntity) String() string {
	switch s {
	case StateEntityConnection:
		return "CONNECTION"
	case StateEntitySession:
		return "SESSION"
	case StateEntityCommissioning:
		return "COMMISSIONING"
	default:
		return "UNKNOWN"
	}
}

// ControlMsgEvent captures transport-level control messages.
type ControlMsgEvent struct {
	// Type of control message.
	Type ControlMsgType `cbor:"1,keyasint"`

	// CloseReason is the reason code for close messages.
	CloseReason *uint8 `cbor:"2,keyasint,omitempty"`
}

// ControlMsgType indicates the type of control message.
type ControlMsgType uint8

const (
	// ControlMsgPing indicates a ping message.
	ControlMsgPing ControlMsgType = 0
	// ControlMsgPong indicates a pong message.
	ControlMsgPong ControlMsgType = 1
	// ControlMsgClose indicates a close message.
	ControlMsgClose ControlMsgType = 2
)

// String returns the control message type name.
func (c ControlMsgType) String() string {
	switch c {
	case ControlMsgPing:
		return "PING"
	case ControlMsgPong:
		return "PONG"
	case ControlMsgClose:
		return "CLOSE"
	default:
		return "UNKNOWN"
	}
}

// ErrorEventData captures errors at any layer.
type ErrorEventData struct {
	// Layer where the error occurred.
	Layer Layer `cbor:"1,keyasint"`

	// Message is the error message.
	Message string `cbor:"2,keyasint"`

	// Code is the error code (if applicable).
	Code *int `cbor:"3,keyasint,omitempty"`

	// Context describes what operation was being performed.
	Context string `cbor:"4,keyasint,omitempty"`
}
