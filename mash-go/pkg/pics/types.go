package pics

import (
	"fmt"
	"sort"
	"strings"
)

// Format represents the PICS file format.
type Format int

const (
	// FormatAuto automatically detects the format.
	FormatAuto Format = iota
	// FormatKeyValue is the key=value format (e.g., MASH.S=1).
	FormatKeyValue
	// FormatYAML is the YAML format with device: and items: sections.
	FormatYAML
)

func (f Format) String() string {
	switch f {
	case FormatAuto:
		return "auto"
	case FormatKeyValue:
		return "key-value"
	case FormatYAML:
		return "yaml"
	default:
		return fmt.Sprintf("unknown(%d)", f)
	}
}

// DeviceMetadata contains device identification from YAML PICS files.
type DeviceMetadata struct {
	// Vendor is the device manufacturer.
	Vendor string
	// Product is the product name.
	Product string
	// Model is the model identifier.
	Model string
	// Version is the firmware/software version.
	Version string
}

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

	// EndpointID is the endpoint number (1-255). 0 means device-level.
	EndpointID uint8

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

	if c.EndpointID > 0 {
		sb.WriteString(fmt.Sprintf(".E%02X", c.EndpointID))
	}

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

// EndpointPICS tracks per-endpoint PICS data.
type EndpointPICS struct {
	// ID is the endpoint number (1-255).
	ID uint8
	// Type is the endpoint type (e.g., "EV_CHARGER", "INVERTER", "BATTERY").
	Type string
	// Features lists enabled features on this endpoint.
	Features []string
}

// ApplicationFeatures lists features that must be on endpoints (not device-level).
var ApplicationFeatures = map[string]bool{
	"ELEC": true, "MEAS": true, "CTRL": true, "STAT": true,
	"INFO": true, "CHRG": true, "SIG": true, "TAR": true, "PLAN": true,
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

	// Version is the protocol version (e.g., "1.0").
	Version string

	// Features lists device-level features (transport/pairing: TRANS, COMM, CERT, etc.).
	Features []string

	// Endpoints maps endpoint ID to per-endpoint PICS data.
	Endpoints map[uint8]*EndpointPICS

	// Device contains optional device metadata (from YAML format).
	Device *DeviceMetadata

	// Format is the detected or specified file format.
	Format Format

	// SourceFile is the path to the source file (if parsed from file).
	SourceFile string
}

// NewPICS creates a new empty PICS.
func NewPICS() *PICS {
	return &PICS{
		Entries:   make([]Entry, 0),
		ByCode:    make(map[string]Entry),
		Endpoints: make(map[uint8]*EndpointPICS),
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

// EndpointHas returns true if the given PICS code exists and is true.
func (p *PICS) EndpointHas(epID uint8, code string) bool {
	entry, ok := p.ByCode[code]
	if !ok {
		return false
	}
	return entry.Code.EndpointID == epID && entry.Value.IsTrue()
}

// EndpointType returns the endpoint type for the given endpoint ID, or empty if not found.
func (p *PICS) EndpointType(epID uint8) string {
	ep, ok := p.Endpoints[epID]
	if !ok {
		return ""
	}
	return ep.Type
}

// EndpointIDs returns all endpoint IDs in sorted order.
func (p *PICS) EndpointIDs() []uint8 {
	ids := make([]uint8, 0, len(p.Endpoints))
	for id := range p.Endpoints {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// HasUseCase returns true if the given use case is declared.
// For devices: checks MASH.S.UC.<name>. For controllers: checks MASH.C.UC.<name>.
func (p *PICS) HasUseCase(name string) bool {
	code := fmt.Sprintf("MASH.%s.UC.%s", p.Side, name)
	return p.Has(code)
}

// UseCases returns all declared use case names (those with true values), sorted.
// Scenario sub-codes (e.g., MASH.S.UC.GPL.S00) are excluded -- only top-level
// use case declarations (e.g., MASH.S.UC.GPL) are returned.
func (p *PICS) UseCases() []string {
	prefix := fmt.Sprintf("MASH.%s.UC.", p.Side)
	var names []string
	for code, entry := range p.ByCode {
		if strings.HasPrefix(code, prefix) && entry.Value.IsTrue() {
			name := strings.TrimPrefix(code, prefix)
			// Skip scenario sub-codes (contain a dot, e.g., "GPL.S00")
			if strings.Contains(name, ".") {
				continue
			}
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// EndpointsWithFeature returns all endpoints that have the given feature enabled.
func (p *PICS) EndpointsWithFeature(feature string) []*EndpointPICS {
	var result []*EndpointPICS
	for _, id := range p.EndpointIDs() {
		ep := p.Endpoints[id]
		for _, f := range ep.Features {
			if f == feature {
				result = append(result, ep)
				break
			}
		}
	}
	return result
}
