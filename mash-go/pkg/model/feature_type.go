package model

// FeatureVendorBase is the start of vendor-specific feature IDs.
// Vendor features use range 0x80-0xFF (128 slots).
const FeatureVendorBase FeatureType = 0x80

// IsVendorFeature returns true if this is a vendor-specific feature.
func (f FeatureType) IsVendorFeature() bool {
	return f >= FeatureVendorBase
}

// FeatureMapBit represents a bit in the featureMap global attribute.
type FeatureMapBit uint32

// FeatureMap bits for quick capability discovery.
const (
	// FeatureMapCore indicates basic energy features (always set).
	FeatureMapCore FeatureMapBit = 0x0001

	// FeatureMapFlex indicates flexible power adjustment support.
	FeatureMapFlex FeatureMapBit = 0x0002

	// FeatureMapBattery indicates battery-specific attributes (SoC, SoH).
	FeatureMapBattery FeatureMapBit = 0x0004

	// FeatureMapEMob indicates E-Mobility/EVSE support.
	FeatureMapEMob FeatureMapBit = 0x0008

	// FeatureMapSignals indicates incentive signals support.
	FeatureMapSignals FeatureMapBit = 0x0010

	// FeatureMapTariff indicates tariff data support.
	FeatureMapTariff FeatureMapBit = 0x0020

	// FeatureMapPlan indicates power plan support.
	FeatureMapPlan FeatureMapBit = 0x0040

	// FeatureMapProcess indicates optional process lifecycle support (OHPCF).
	FeatureMapProcess FeatureMapBit = 0x0080

	// FeatureMapForecast indicates power forecasting capability.
	FeatureMapForecast FeatureMapBit = 0x0100

	// FeatureMapAsymmetric indicates per-phase asymmetric control.
	FeatureMapAsymmetric FeatureMapBit = 0x0200

	// FeatureMapV2X indicates vehicle-to-grid/home (bidirectional EV).
	FeatureMapV2X FeatureMapBit = 0x0400
)

// String returns the feature map bit name.
func (b FeatureMapBit) String() string {
	switch b {
	case FeatureMapCore:
		return "CORE"
	case FeatureMapFlex:
		return "FLEX"
	case FeatureMapBattery:
		return "BATTERY"
	case FeatureMapEMob:
		return "EMOB"
	case FeatureMapSignals:
		return "SIGNALS"
	case FeatureMapTariff:
		return "TARIFF"
	case FeatureMapPlan:
		return "PLAN"
	case FeatureMapProcess:
		return "PROCESS"
	case FeatureMapForecast:
		return "FORECAST"
	case FeatureMapAsymmetric:
		return "ASYMMETRIC"
	case FeatureMapV2X:
		return "V2X"
	default:
		return "UNKNOWN"
	}
}
