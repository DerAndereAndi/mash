package wire

// Operation represents a MASH protocol operation.
// MASH has 4 operations (like Matter, not SPINE's 7).
type Operation uint8

const (
	// OpRead gets current attribute values from a device.
	// Direction: Controller -> Device
	OpRead Operation = 1

	// OpWrite sets attribute values on a device (full replace).
	// Direction: Controller -> Device
	OpWrite Operation = 2

	// OpSubscribe registers for change notifications.
	// Direction: Controller -> Device
	OpSubscribe Operation = 3

	// OpInvoke executes a command with parameters.
	// Direction: Controller -> Device
	OpInvoke Operation = 4
)

// String returns the operation name.
func (o Operation) String() string {
	switch o {
	case OpRead:
		return "Read"
	case OpWrite:
		return "Write"
	case OpSubscribe:
		return "Subscribe"
	case OpInvoke:
		return "Invoke"
	default:
		return "Unknown"
	}
}

// IsValid returns true if the operation is a valid MASH operation.
func (o Operation) IsValid() bool {
	return o >= OpRead && o <= OpInvoke
}
