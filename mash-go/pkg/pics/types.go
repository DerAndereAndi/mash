package pics

import (
	"fmt"
	"strings"
)

// Side represents whether the PICS is for a device (server) or controller (client).
type Side string

const (
	SideServer Side = "S" // Device
	SideClient Side = "C" // Controller
)

// CodeType represents the type of PICS code.
type CodeType string

const (
	CodeTypeFeature   CodeType = ""  // Feature presence (e.g., MASH.S.CTRL=1)
	CodeTypeAttribute CodeType = "A" // Attribute support
	CodeTypeCommand   CodeType = "C" // Command support
	CodeTypeFlag      CodeType = "F" // Feature flag
	CodeTypeEvent     CodeType = "E" // Event support
	CodeTypeBehavior  CodeType = "B" // Behavior option
)

// Qualifier represents a command qualifier.
type Qualifier string

const (
	QualifierNone     Qualifier = ""    // No qualifier
	QualifierResponse Qualifier = "Rsp" // Accepts/responds to command
	QualifierTransmit Qualifier = "Tx"  // Generates/sends command
)

// Code represents a parsed PICS code.
type Code struct {
	// Raw is the original code string.
	Raw string

	// Side is S (server/device) or C (client/controller).
	Side Side

	// Feature is the feature identifier (e.g., "CTRL", "ELEC").
	Feature string

	// Type is the code type (A, C, F, E, B, or empty for feature presence).
	Type CodeType

	// ID is the hex identifier (e.g., "01", "0A").
	ID string

	// Qualifier is Rsp or Tx for commands, empty otherwise.
	Qualifier Qualifier
}

// String returns the canonical code string.
func (c Code) String() string {
	// For behavior codes, the Raw field contains the original string
	// which is the canonical form
	if c.Type == CodeTypeBehavior && c.Raw != "" {
		return c.Raw
	}

	var sb strings.Builder
	sb.WriteString("MASH.")
	sb.WriteString(string(c.Side))

	if c.Feature != "" {
		sb.WriteString(".")
		sb.WriteString(c.Feature)
	}

	if c.Type != "" {
		sb.WriteString(".")
		sb.WriteString(string(c.Type))
		sb.WriteString(c.ID)
	}

	if c.Qualifier != "" {
		sb.WriteString(".")
		sb.WriteString(string(c.Qualifier))
	}

	return sb.String()
}

// Value represents a PICS value.
// Values can be boolean (1/0), integer, or string.
type Value struct {
	// Bool is true if the value is 1 or "true".
	Bool bool

	// Int is the integer value (0 if not an integer).
	Int int64

	// String is the string value.
	String string

	// Raw is the original value string.
	Raw string
}

// IsBool returns true if the value is a boolean (0 or 1).
func (v Value) IsBool() bool {
	return v.Raw == "0" || v.Raw == "1"
}

// IsTrue returns true if the value represents true/enabled.
func (v Value) IsTrue() bool {
	return v.Bool
}

// Entry represents a single PICS entry (code = value).
type Entry struct {
	Code  Code
	Value Value

	// LineNumber is the line number in the source file (1-based).
	LineNumber int
}

// PICS represents a complete PICS file.
type PICS struct {
	// Entries contains all parsed entries.
	Entries []Entry

	// ByCode provides fast lookup by code string.
	ByCode map[string]Entry

	// Side is the primary side (S or C) of this PICS.
	Side Side

	// Version is the protocol version.
	Version int

	// Features lists all features that are enabled.
	Features []string
}

// NewPICS creates a new empty PICS.
func NewPICS() *PICS {
	return &PICS{
		Entries: make([]Entry, 0),
		ByCode:  make(map[string]Entry),
	}
}

// Get returns the value for a PICS code, or false if not present.
func (p *PICS) Get(code string) (Value, bool) {
	entry, ok := p.ByCode[code]
	if !ok {
		return Value{}, false
	}
	return entry.Value, true
}

// Has returns true if the PICS code is present and true.
func (p *PICS) Has(code string) bool {
	v, ok := p.Get(code)
	return ok && v.IsTrue()
}

// GetInt returns the integer value for a PICS code, or 0 if not present.
func (p *PICS) GetInt(code string) int64 {
	v, ok := p.Get(code)
	if !ok {
		return 0
	}
	return v.Int
}

// GetString returns the string value for a PICS code, or empty if not present.
func (p *PICS) GetString(code string) string {
	v, ok := p.Get(code)
	if !ok {
		return ""
	}
	return v.String
}

// HasFeature returns true if the specified feature is enabled.
func (p *PICS) HasFeature(feature string) bool {
	code := fmt.Sprintf("MASH.%s.%s", p.Side, feature)
	return p.Has(code)
}

// HasAttribute returns true if the specified attribute is supported.
func (p *PICS) HasAttribute(feature string, attrID string) bool {
	code := fmt.Sprintf("MASH.%s.%s.A%s", p.Side, feature, attrID)
	return p.Has(code)
}

// HasCommand returns true if the specified command is supported.
func (p *PICS) HasCommand(feature string, cmdID string) bool {
	code := fmt.Sprintf("MASH.%s.%s.C%s.Rsp", p.Side, feature, cmdID)
	return p.Has(code)
}

// HasFeatureFlag returns true if the specified feature flag is set.
func (p *PICS) HasFeatureFlag(feature string, flagID string) bool {
	code := fmt.Sprintf("MASH.%s.%s.F%s", p.Side, feature, flagID)
	return p.Has(code)
}

// IsDevice returns true if this is a device (server) PICS.
func (p *PICS) IsDevice() bool {
	return p.Side == SideServer
}

// IsController returns true if this is a controller (client) PICS.
func (p *PICS) IsController() bool {
	return p.Side == SideClient
}
