package model

// FeatureType identifies the type of a feature.
// Uses uint8 to match wire protocol and save bytes in messages.
type FeatureType uint8

// Feature types from the MASH feature registry.
// Ordered from most fundamental to most specialized.
// Standard features: 0x01-0x7F (127 max)
// Vendor features: 0x80-0xFF (128 max)
const (
	// FeatureDeviceInfo provides device identity and structure information.
	// Present on endpoint 0 (DEVICE_ROOT) of every device.
	FeatureDeviceInfo FeatureType = 0x01

	// FeatureStatus provides operating state and fault information.
	// "Is it working?" - Operational status and diagnostics.
	FeatureStatus FeatureType = 0x02

	// FeatureElectrical provides static electrical configuration.
	// "What CAN this do?" - Phase config, ratings, capabilities.
	FeatureElectrical FeatureType = 0x03

	// FeatureMeasurement provides power, energy, voltage, current telemetry.
	// "What IS it doing?" - Real-time telemetry data.
	FeatureMeasurement FeatureType = 0x04

	// FeatureEnergyControl provides limits, setpoints, and control commands.
	// "What SHOULD it do?" - Control interface for energy management.
	FeatureEnergyControl FeatureType = 0x05

	// FeatureChargingSession provides EV charging session data.
	// Used on EV_CHARGER endpoints.
	FeatureChargingSession FeatureType = 0x06

	// FeatureTariff provides price structure, components, and power tiers.
	// Defines the structure that Signals references.
	FeatureTariff FeatureType = 0x07

	// FeatureSignals provides time-slotted prices, limits, and forecasts.
	// Receives incentive/constraint signals from controllers.
	FeatureSignals FeatureType = 0x08

	// FeaturePlan provides the device's intended behavior/schedule.
	// Reports planned power consumption/production in response to Signals.
	FeaturePlan FeatureType = 0x09

	// FeatureVendorBase is the start of vendor-specific feature IDs.
	// Vendor features use range 0x80-0xFF (128 slots).
	FeatureVendorBase FeatureType = 0x80
)

// String returns the feature type name.
func (f FeatureType) String() string {
	switch f {
	case FeatureElectrical:
		return "Electrical"
	case FeatureMeasurement:
		return "Measurement"
	case FeatureEnergyControl:
		return "EnergyControl"
	case FeatureStatus:
		return "Status"
	case FeatureDeviceInfo:
		return "DeviceInfo"
	case FeatureChargingSession:
		return "ChargingSession"
	case FeatureSignals:
		return "Signals"
	case FeatureTariff:
		return "Tariff"
	case FeaturePlan:
		return "Plan"
	default:
		if f >= FeatureVendorBase {
			return "Vendor"
		}
		return "Unknown"
	}
}

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
