package model

import (
	"errors"
	"fmt"
	"sync"
)

// Attribute ID ranges (convention).
const (
	// AttrIDCore is the start of core identity/type attributes (1-9).
	AttrIDCore uint16 = 1

	// AttrIDCapability is the start of capability flags (10-19).
	AttrIDCapability uint16 = 10

	// AttrIDPrimary is the start of primary data (20-29).
	AttrIDPrimary uint16 = 20

	// AttrIDSecondary is the start of secondary data (30-39).
	AttrIDSecondary uint16 = 30

	// AttrIDTertiary is the start of tertiary data (40-49).
	AttrIDTertiary uint16 = 40

	// AttrIDAdditional is the start of additional data (50-59).
	AttrIDAdditional uint16 = 50

	// AttrIDComplex is the start of complex structures (60-69).
	AttrIDComplex uint16 = 60

	// AttrIDFailsafe is the start of failsafe configuration (70-79).
	AttrIDFailsafe uint16 = 70

	// AttrIDProcess is the start of process management (80-89).
	AttrIDProcess uint16 = 80

	// AttrIDGlobalBase is the start of global attributes (0xFFF0-0xFFFF).
	AttrIDGlobalBase uint16 = 0xFFF0
)

// Global attribute IDs (present on all features).
const (
	// AttrIDClusterRevision is the feature revision number.
	AttrIDClusterRevision uint16 = 0xFFFD

	// AttrIDFeatureMap is the feature capability bitmap.
	AttrIDFeatureMap uint16 = 0xFFFC

	// AttrIDAttributeList is the list of supported attribute IDs.
	AttrIDAttributeList uint16 = 0xFFFB

	// AttrIDCommandList is the list of supported command IDs.
	AttrIDCommandList uint16 = 0xFFFA
)

// Access flags for attributes.
type Access uint8

const (
	// AccessRead allows reading the attribute.
	AccessRead Access = 1 << iota

	// AccessWrite allows writing the attribute.
	AccessWrite

	// AccessSubscribe allows subscribing to changes.
	AccessSubscribe

	// Common access combinations.

	// AccessReadOnly is read and subscribe.
	AccessReadOnly = AccessRead | AccessSubscribe

	// AccessReadWrite is read, write, and subscribe.
	AccessReadWrite = AccessRead | AccessWrite | AccessSubscribe
)

// CanRead returns true if reading is allowed.
func (a Access) CanRead() bool { return a&AccessRead != 0 }

// CanWrite returns true if writing is allowed.
func (a Access) CanWrite() bool { return a&AccessWrite != 0 }

// CanSubscribe returns true if subscribing is allowed.
func (a Access) CanSubscribe() bool { return a&AccessSubscribe != 0 }

// String returns the access flags as a string.
func (a Access) String() string {
	var s string
	if a.CanRead() {
		s += "R"
	}
	if a.CanWrite() {
		s += "W"
	}
	if a.CanSubscribe() {
		s += "S"
	}
	if s == "" {
		return "-"
	}
	return s
}

// DataType represents the type of an attribute value.
type DataType uint8

const (
	DataTypeUnknown DataType = iota
	DataTypeBool
	DataTypeInt8
	DataTypeInt16
	DataTypeInt32
	DataTypeInt64
	DataTypeUint8
	DataTypeUint16
	DataTypeUint32
	DataTypeUint64
	DataTypeFloat32
	DataTypeFloat64
	DataTypeString
	DataTypeBytes
	DataTypeArray
	DataTypeMap
	DataTypeStruct
	DataTypeEnum
	DataTypeNull
)

// String returns the data type name.
func (d DataType) String() string {
	names := []string{
		"unknown", "bool", "int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64", "float32", "float64",
		"string", "bytes", "array", "map", "struct", "enum", "null",
	}
	if int(d) < len(names) {
		return names[d]
	}
	return "unknown"
}

// AttributeMetadata describes an attribute's properties.
type AttributeMetadata struct {
	// ID is the attribute identifier within the feature.
	ID uint16

	// Name is the human-readable attribute name.
	Name string

	// Type is the data type of the attribute value.
	Type DataType

	// Access defines the allowed operations.
	Access Access

	// Nullable indicates if nil/null is a valid value.
	Nullable bool

	// MinValue is the minimum allowed value (for numeric types).
	MinValue any

	// MaxValue is the maximum allowed value (for numeric types).
	MaxValue any

	// Default is the default value.
	Default any

	// Unit is the unit of measurement (e.g., "W", "Wh", "A").
	Unit string

	// Description is a human-readable description.
	Description string
}

// Attribute represents an attribute instance with its current value.
type Attribute struct {
	mu       sync.RWMutex
	metadata *AttributeMetadata
	value    any
	dirty    bool // True if value changed since last report
}

// Attribute errors.
var (
	ErrAttributeNotWritable = errors.New("attribute is not writable")
	ErrAttributeNotNullable = errors.New("attribute does not accept null")
	ErrAttributeValueType   = errors.New("invalid value type for attribute")
	ErrAttributeOutOfRange  = errors.New("value out of range")
)

// NewAttribute creates a new attribute with the given metadata.
func NewAttribute(meta *AttributeMetadata) *Attribute {
	return &Attribute{
		metadata: meta,
		value:    meta.Default,
	}
}

// ID returns the attribute ID.
func (a *Attribute) ID() uint16 {
	return a.metadata.ID
}

// Metadata returns the attribute metadata.
func (a *Attribute) Metadata() *AttributeMetadata {
	return a.metadata
}

// Value returns the current attribute value.
func (a *Attribute) Value() any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.value
}

// SetValue sets the attribute value.
// Returns an error if the attribute is not writable or the value is invalid.
func (a *Attribute) SetValue(value any) error {
	if !a.metadata.Access.CanWrite() {
		return ErrAttributeNotWritable
	}
	return a.setValueInternal(value)
}

// SetValueInternal sets the value without checking write access.
// Used by the device implementation to update read-only attributes.
func (a *Attribute) SetValueInternal(value any) error {
	return a.setValueInternal(value)
}

func (a *Attribute) setValueInternal(value any) error {
	// Check nullable
	if value == nil && !a.metadata.Nullable {
		return ErrAttributeNotNullable
	}

	// Validate type and range
	if value != nil {
		if err := a.validateValue(value); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if value actually changed
	if a.value != value {
		a.value = value
		a.dirty = true
	}

	return nil
}

// validateValue checks if the value matches the expected type and range.
func (a *Attribute) validateValue(value any) error {
	// Type checking based on DataType
	switch a.metadata.Type {
	case DataTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: expected bool", ErrAttributeValueType)
		}
	case DataTypeInt8, DataTypeInt16, DataTypeInt32, DataTypeInt64:
		if !isIntegerType(value) {
			return fmt.Errorf("%w: expected integer", ErrAttributeValueType)
		}
	case DataTypeUint8, DataTypeUint16, DataTypeUint32, DataTypeUint64:
		if !isIntegerType(value) {
			return fmt.Errorf("%w: expected unsigned integer", ErrAttributeValueType)
		}
	case DataTypeFloat32, DataTypeFloat64:
		if !isNumericType(value) {
			return fmt.Errorf("%w: expected float", ErrAttributeValueType)
		}
	case DataTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: expected string", ErrAttributeValueType)
		}
	case DataTypeBytes:
		if _, ok := value.([]byte); !ok {
			return fmt.Errorf("%w: expected bytes", ErrAttributeValueType)
		}
	}

	// Range checking for numeric types
	if a.metadata.MinValue != nil || a.metadata.MaxValue != nil {
		if err := a.checkRange(value); err != nil {
			return err
		}
	}

	return nil
}

// checkRange validates numeric range constraints.
func (a *Attribute) checkRange(value any) error {
	v, ok := toFloat64(value)
	if !ok {
		return nil // Not a numeric type
	}

	if a.metadata.MinValue != nil {
		min, _ := toFloat64(a.metadata.MinValue)
		if v < min {
			return fmt.Errorf("%w: %v < %v", ErrAttributeOutOfRange, value, a.metadata.MinValue)
		}
	}

	if a.metadata.MaxValue != nil {
		max, _ := toFloat64(a.metadata.MaxValue)
		if v > max {
			return fmt.Errorf("%w: %v > %v", ErrAttributeOutOfRange, value, a.metadata.MaxValue)
		}
	}

	return nil
}

// IsDirty returns true if the value changed since the last ClearDirty call.
func (a *Attribute) IsDirty() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dirty
}

// ClearDirty clears the dirty flag.
func (a *Attribute) ClearDirty() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dirty = false
}

// Helper functions for type checking.

func isIntegerType(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func isNumericType(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
