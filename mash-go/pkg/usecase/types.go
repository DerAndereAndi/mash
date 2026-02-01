// Package usecase defines use case definitions (LPC, LPP, MPD, EVC, etc.),
// discovers device capabilities, and matches them against use case requirements.
package usecase

// UseCaseName identifies a use case by its human-readable name.
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

// UseCaseID is the wire identifier for a use case (uint16).
type UseCaseID uint16

const (
	LPCID   UseCaseID = 0x01
	LPPID   UseCaseID = 0x02
	MPDID   UseCaseID = 0x03
	EVCID   UseCaseID = 0x04
	COBID   UseCaseID = 0x05
	FLOAID  UseCaseID = 0x06
	ITPCMID UseCaseID = 0x07
	OHPCFID UseCaseID = 0x08
	PODFID  UseCaseID = 0x09
	POENID  UseCaseID = 0x0A
	TOUTID  UseCaseID = 0x0B
)

// ScenarioBit represents a single scenario bit position.
type ScenarioBit uint8

// ScenarioMap is a bitmap of supported scenarios.
type ScenarioMap uint32

// ScenarioBASE is the BASE scenario (bit 0, always set).
const ScenarioBASE ScenarioMap = 1 << 0

// Has returns true if the given scenario bit is set.
func (s ScenarioMap) Has(bit ScenarioBit) bool {
	return s&(1<<bit) != 0
}

// ScenarioDef describes a single scenario within a use case.
type ScenarioDef struct {
	Bit         ScenarioBit
	Name        string
	Description string
	Features    []FeatureRequirement
}

// UseCaseDef describes the requirements of a single use case.
type UseCaseDef struct {
	Name          UseCaseName
	ID            UseCaseID
	FullName      string
	Description   string
	SpecVersion   string
	Major         uint8    // contract version (breaking changes)
	Minor         uint8    // contract version (compatible refinements)
	EndpointTypes []string // empty = any endpoint type
	Scenarios     []ScenarioDef
	Commands      []string // interactive commands enabled by this use case
}

// AllFeatures returns the union of all scenario features.
func (d *UseCaseDef) AllFeatures() []FeatureRequirement {
	seen := make(map[string]bool)
	var result []FeatureRequirement
	for _, s := range d.Scenarios {
		for _, f := range s.Features {
			if !seen[f.FeatureName] {
				seen[f.FeatureName] = true
				result = append(result, f)
			}
		}
	}
	return result
}

// ScenarioFeatures returns the features required for the given scenario bitmap.
func (d *UseCaseDef) ScenarioFeatures(scenarios ScenarioMap) []FeatureRequirement {
	seen := make(map[string]bool)
	var result []FeatureRequirement
	for _, s := range d.Scenarios {
		if scenarios.Has(s.Bit) {
			for _, f := range s.Features {
				if !seen[f.FeatureName] {
					seen[f.FeatureName] = true
					result = append(result, f)
				}
			}
		}
	}
	return result
}

// BaseScenario returns the BASE scenario definition, or nil if not found.
func (d *UseCaseDef) BaseScenario() *ScenarioDef {
	for i := range d.Scenarios {
		if d.Scenarios[i].Bit == 0 {
			return &d.Scenarios[i]
		}
	}
	return nil
}

// DefinedScenarioMask returns a bitmap with bits set for all defined scenarios.
func (d *UseCaseDef) DefinedScenarioMask() ScenarioMap {
	var mask ScenarioMap
	for _, s := range d.Scenarios {
		mask |= 1 << ScenarioMap(s.Bit)
	}
	return mask
}

// FeatureRequirement describes a feature needed by a use case.
type FeatureRequirement struct {
	FeatureName  string
	FeatureID    uint8 // resolved from spec manifest
	Required     bool
	Attributes   []AttributeRequirement
	Commands     []CommandRequirement
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
	Scenarios       ScenarioMap // which scenarios matched
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

// ScenariosForUseCase returns the scenario bitmap for a matched use case.
func (d *DeviceUseCases) ScenariosForUseCase(name UseCaseName) (ScenarioMap, bool) {
	for _, m := range d.Matches {
		if m.UseCase == name && m.Matched {
			return m.Scenarios, true
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
