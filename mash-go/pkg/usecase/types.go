// Package usecase defines use case definitions (LPC, LPP, MPD, EVC, etc.),
// discovers device capabilities, and matches them against use case requirements.
package usecase

// UseCaseName identifies a use case.
type UseCaseName string

const (
	COB   UseCaseName = "COB"   // Control of Battery
	EVC   UseCaseName = "EVC"   // EV Charging
	FLOA  UseCaseName = "FLOA"  // Flexible Load
	ITPCM UseCaseName = "ITPCM" // Incentive Table-based Power Consumption Management
	LPC   UseCaseName = "LPC"   // Limit Power Consumption
	LPP   UseCaseName = "LPP"   // Limit Power Production
	MPD   UseCaseName = "MPD"   // Monitor Power Device
	OHPCF UseCaseName = "OHPCF" // Optimized Heat Pump Control Flow
	PODF  UseCaseName = "PODF"  // Power-on Demand Forecast
	POEN  UseCaseName = "POEN"  // Power-on Energy Negotiation
	TOUT  UseCaseName = "TOUT"  // Time of Use Tariff
)

// UseCaseDef describes the requirements of a single use case.
type UseCaseDef struct {
	Name          UseCaseName
	FullName      string
	Description   string
	SpecVersion   string
	EndpointTypes []string             // empty = any endpoint type
	Features      []FeatureRequirement // features needed for this use case
	Commands      []string             // interactive commands enabled by this use case
}

// FeatureRequirement describes a feature needed by a use case.
type FeatureRequirement struct {
	FeatureName   string
	FeatureID     uint8 // resolved from spec manifest
	Required      bool
	Attributes    []AttributeRequirement
	Commands      []CommandRequirement
	SubscribeAll bool // true = subscribe to all attributes (DEC-052)
}

// ShouldSubscribe returns true if this feature requires subscription.
func (f *FeatureRequirement) ShouldSubscribe() bool {
	return f.SubscribeAll
}

// AttributeRequirement describes an attribute needed by a use case.
type AttributeRequirement struct {
	Name          string
	AttrID        uint16 // resolved from spec manifest
	RequiredValue *bool  // nil = presence only, non-nil = must match
}

// CommandRequirement describes a command needed by a use case.
type CommandRequirement struct {
	Name      string
	CommandID uint8 // resolved from spec manifest
}

// --- Discovery result types ---

// DeviceProfile captures what a device actually supports.
type DeviceProfile struct {
	DeviceID  string
	Endpoints map[uint8]*EndpointProfile
}

// EndpointProfile captures a single endpoint's capabilities.
type EndpointProfile struct {
	EndpointID   uint8
	EndpointType string
	Features     map[uint8]*FeatureProfile
}

// FeatureProfile captures a single feature's capabilities.
type FeatureProfile struct {
	FeatureID    uint8
	FeatureMap   uint32
	AttributeIDs []uint16
	CommandIDs   []uint8
	Attributes   map[uint16]any // capability booleans read here
}

// --- Match result types ---

// MatchResult describes whether a use case matched a specific endpoint.
type MatchResult struct {
	UseCase         UseCaseName
	Matched         bool
	EndpointID      uint8
	MissingRequired []string
	OptionalMissing []string
}

// DeviceUseCases holds the discovery and match results for a device.
type DeviceUseCases struct {
	DeviceID string
	Profile  *DeviceProfile
	Matches  []MatchResult
	registry map[UseCaseName]*UseCaseDef // reference to registry for command lookup
}

// SupportedCommands returns the union of commands from all matched use cases.
func (d *DeviceUseCases) SupportedCommands() map[string]bool {
	cmds := make(map[string]bool)
	for _, m := range d.Matches {
		if !m.Matched {
			continue
		}
		if def, ok := d.registry[m.UseCase]; ok {
			for _, cmd := range def.Commands {
				cmds[cmd] = true
			}
		}
	}
	return cmds
}

// HasUseCase returns true if the device matched the given use case.
func (d *DeviceUseCases) HasUseCase(name UseCaseName) bool {
	for _, m := range d.Matches {
		if m.UseCase == name && m.Matched {
			return true
		}
	}
	return false
}

// EndpointForUseCase returns the endpoint ID for a matched use case.
func (d *DeviceUseCases) EndpointForUseCase(name UseCaseName) (uint8, bool) {
	for _, m := range d.Matches {
		if m.UseCase == name && m.Matched {
			return m.EndpointID, true
		}
	}
	return 0, false
}

// MatchedUseCases returns the names of all matched use cases.
func (d *DeviceUseCases) MatchedUseCases() []UseCaseName {
	var names []UseCaseName
	for _, m := range d.Matches {
		if m.Matched {
			names = append(names, m.UseCase)
		}
	}
	return names
}
