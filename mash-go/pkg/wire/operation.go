package wire

// Operation represents a MASH protocol operation.
// MASH has 4 operations (like Matter, not SPINE's 7).
type Operation uint8

const (
	// OpRead gets current attribute values.
	// Direction: Bidirectional (either side can send)
	OpRead Operation = 1

	// OpWrite sets attribute values (full replace).
	// Direction: Bidirectional (either side can send)
	OpWrite Operation = 2

	// OpSubscribe registers for change notifications.
	// Direction: Bidirectional (either side can send)
	OpSubscribe Operation = 3

	// OpInvoke executes a command with parameters.
	// Direction: Bidirectional (either side can send)
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
