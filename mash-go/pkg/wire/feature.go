package wire

// FeatureID represents a MASH feature identifier.
type FeatureID uint8

const (
	// FeatureDeviceInfo provides device identity and structure.
	// Always present on endpoint 0 (DEVICE_ROOT).
	FeatureDeviceInfo FeatureID = 0

	// FeatureElectrical provides static electrical configuration.
	// "What CAN it do?" - phase count, nominal voltage, etc.
	FeatureElectrical FeatureID = 1

	// FeatureMeasurement provides power and energy telemetry.
	// "What IS it doing?" - AC/DC measurements.
	FeatureMeasurement FeatureID = 2

	// FeatureEnergyControl provides limits, setpoints, and control.
	// "What SHOULD it do?" - limits, setpoints, control state.
	FeatureEnergyControl FeatureID = 3

	// FeatureStatus provides operating state and faults.
	// "Is it working?" - operating state, fault codes.
	FeatureStatus FeatureID = 4

	// FeatureChargingSession provides EV charging session information.
	// ISO 15118 integration, session state, energy delivered.
	FeatureChargingSession FeatureID = 5

	// FeatureSignals provides time-slotted prices, limits, and forecasts.
	// Grid signals, tariff schedules, incentive signals.
	FeatureSignals FeatureID = 6

	// FeatureTariff provides price structure and power tiers.
	// Tariff definitions, time-of-use pricing.
	FeatureTariff FeatureID = 7

	// FeaturePlan provides device's intended behavior.
	// Planned consumption/production schedules.
	FeaturePlan FeatureID = 8
)

// String returns the feature name.
func (f FeatureID) String() string {
	switch f {
	case FeatureDeviceInfo:
		return "DeviceInfo"
	case FeatureElectrical:
		return "Electrical"
	case FeatureMeasurement:
		return "Measurement"
	case FeatureEnergyControl:
		return "EnergyControl"
	case FeatureStatus:
		return "Status"
	case FeatureChargingSession:
		return "ChargingSession"
	case FeatureSignals:
		return "Signals"
	case FeatureTariff:
		return "Tariff"
	case FeaturePlan:
		return "Plan"
	default:
		return "Unknown"
	}
}

// ShortCode returns the short code used in PICS files.
func (f FeatureID) ShortCode() string {
	switch f {
	case FeatureDeviceInfo:
		return "INFO"
	case FeatureElectrical:
		return "ELEC"
	case FeatureMeasurement:
		return "MEAS"
	case FeatureEnergyControl:
		return "CTRL"
	case FeatureStatus:
		return "STAT"
	case FeatureChargingSession:
		return "CHRG"
	case FeatureSignals:
		return "SIG"
	case FeatureTariff:
		return "TAR"
	case FeaturePlan:
		return "PLAN"
	default:
		return "UNK"
	}
}
