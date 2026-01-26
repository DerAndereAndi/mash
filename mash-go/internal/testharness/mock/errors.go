package mock

import "errors"

// Mock package errors.
var (
	// ErrDeviceNotConnected is returned when operating on a disconnected device.
	ErrDeviceNotConnected = errors.New("device not connected")

	// ErrEndpointNotFound is returned when an endpoint doesn't exist.
	ErrEndpointNotFound = errors.New("endpoint not found")

	// ErrFeatureNotFound is returned when a feature doesn't exist.
	ErrFeatureNotFound = errors.New("feature not found")

	// ErrAttributeNotFound is returned when an attribute doesn't exist.
	ErrAttributeNotFound = errors.New("attribute not found")

	// ErrInvalidPath is returned for malformed paths.
	ErrInvalidPath = errors.New("invalid path")

	// ErrReadOnly is returned when writing to a read-only attribute.
	ErrReadOnly = errors.New("attribute is read-only")

	// ErrNotSubscribable is returned for non-subscribable attributes.
	ErrNotSubscribable = errors.New("attribute is not subscribable")

	// ErrNotInvokable is returned when invoke is not supported.
	ErrNotInvokable = errors.New("feature does not support invoke")
)
