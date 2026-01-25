package wire

// Status represents a response status code.
type Status uint8

const (
	// StatusSuccess indicates the operation completed successfully.
	StatusSuccess Status = 0

	// StatusInvalidEndpoint indicates the endpoint doesn't exist.
	StatusInvalidEndpoint Status = 1

	// StatusInvalidFeature indicates the feature doesn't exist on the endpoint.
	StatusInvalidFeature Status = 2

	// StatusInvalidAttribute indicates the attribute doesn't exist.
	StatusInvalidAttribute Status = 3

	// StatusInvalidCommand indicates the command doesn't exist.
	StatusInvalidCommand Status = 4

	// StatusInvalidParameter indicates a parameter value is out of range.
	StatusInvalidParameter Status = 5

	// StatusReadOnly indicates an attempt to write to a read-only attribute.
	StatusReadOnly Status = 6

	// StatusWriteOnly indicates an attempt to read from a write-only attribute.
	StatusWriteOnly Status = 7

	// StatusNotAuthorized indicates the zone doesn't have permission.
	StatusNotAuthorized Status = 8

	// StatusBusy indicates the device is busy; try again later.
	StatusBusy Status = 9

	// StatusUnsupported indicates the operation is not supported.
	StatusUnsupported Status = 10

	// StatusConstraintError indicates a value violates a constraint.
	StatusConstraintError Status = 11

	// StatusTimeout indicates the operation timed out.
	StatusTimeout Status = 12
)

// String returns the status name.
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "SUCCESS"
	case StatusInvalidEndpoint:
		return "INVALID_ENDPOINT"
	case StatusInvalidFeature:
		return "INVALID_FEATURE"
	case StatusInvalidAttribute:
		return "INVALID_ATTRIBUTE"
	case StatusInvalidCommand:
		return "INVALID_COMMAND"
	case StatusInvalidParameter:
		return "INVALID_PARAMETER"
	case StatusReadOnly:
		return "READ_ONLY"
	case StatusWriteOnly:
		return "WRITE_ONLY"
	case StatusNotAuthorized:
		return "NOT_AUTHORIZED"
	case StatusBusy:
		return "BUSY"
	case StatusUnsupported:
		return "UNSUPPORTED"
	case StatusConstraintError:
		return "CONSTRAINT_ERROR"
	case StatusTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

// IsSuccess returns true if the status indicates success.
func (s Status) IsSuccess() bool {
	return s == StatusSuccess
}

// IsError returns true if the status indicates an error.
func (s Status) IsError() bool {
	return s != StatusSuccess
}
