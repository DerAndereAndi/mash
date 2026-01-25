package features

// Direction indicates power flow capability.
type Direction uint8

const (
	// DirectionConsumption indicates the device can only consume power.
	DirectionConsumption Direction = 0x00

	// DirectionProduction indicates the device can only produce power.
	DirectionProduction Direction = 0x01

	// DirectionBidirectional indicates the device can both consume and produce.
	DirectionBidirectional Direction = 0x02
)

// String returns the direction name.
func (d Direction) String() string {
	switch d {
	case DirectionConsumption:
		return "CONSUMPTION"
	case DirectionProduction:
		return "PRODUCTION"
	case DirectionBidirectional:
		return "BIDIRECTIONAL"
	default:
		return "UNKNOWN"
	}
}

// AsymmetricSupport indicates per-phase asymmetric control capability.
type AsymmetricSupport uint8

const (
	// AsymmetricNone indicates symmetric only (all phases must have same value).
	AsymmetricNone AsymmetricSupport = 0x00

	// AsymmetricConsumption indicates asymmetric consumption support.
	AsymmetricConsumption AsymmetricSupport = 0x01

	// AsymmetricProduction indicates asymmetric production support.
	AsymmetricProduction AsymmetricSupport = 0x02

	// AsymmetricBidirectional indicates asymmetric support in both directions.
	AsymmetricBidirectional AsymmetricSupport = 0x03
)

// String returns the asymmetric support name.
func (a AsymmetricSupport) String() string {
	switch a {
	case AsymmetricNone:
		return "NONE"
	case AsymmetricConsumption:
		return "CONSUMPTION"
	case AsymmetricProduction:
		return "PRODUCTION"
	case AsymmetricBidirectional:
		return "BIDIRECTIONAL"
	default:
		return "UNKNOWN"
	}
}

// Phase represents a device phase (internal to the device).
type Phase uint8

const (
	PhaseA Phase = 0x00
	PhaseB Phase = 0x01
	PhaseC Phase = 0x02
)

// String returns the phase name.
func (p Phase) String() string {
	switch p {
	case PhaseA:
		return "A"
	case PhaseB:
		return "B"
	case PhaseC:
		return "C"
	default:
		return "?"
	}
}

// GridPhase represents a grid phase (L1, L2, L3).
type GridPhase uint8

const (
	GridPhaseL1 GridPhase = 0x00
	GridPhaseL2 GridPhase = 0x01
	GridPhaseL3 GridPhase = 0x02
)

// String returns the grid phase name.
func (g GridPhase) String() string {
	switch g {
	case GridPhaseL1:
		return "L1"
	case GridPhaseL2:
		return "L2"
	case GridPhaseL3:
		return "L3"
	default:
		return "?"
	}
}

// PhasePair represents a pair of phases for phase-to-phase voltage.
type PhasePair uint8

const (
	PhasePairAB PhasePair = 0x00
	PhasePairBC PhasePair = 0x01
	PhasePairCA PhasePair = 0x02
)

// String returns the phase pair name.
func (p PhasePair) String() string {
	switch p {
	case PhasePairAB:
		return "AB"
	case PhasePairBC:
		return "BC"
	case PhasePairCA:
		return "CA"
	default:
		return "??"
	}
}
