package model

// IsACEndpoint returns true if this endpoint type represents an AC-connected device.
func (e EndpointType) IsACEndpoint() bool {
	switch e {
	case EndpointGridConnection, EndpointInverter, EndpointEVCharger,
		EndpointHeatPump, EndpointWaterHeater, EndpointHVAC, EndpointAppliance:
		return true
	default:
		return false
	}
}

// IsDCEndpoint returns true if this endpoint type represents a DC-connected device.
func (e EndpointType) IsDCEndpoint() bool {
	switch e {
	case EndpointPVString, EndpointBattery:
		return true
	default:
		return false
	}
}

// SupportsEnergyControl returns true if this endpoint type typically supports EnergyControl.
func (e EndpointType) SupportsEnergyControl() bool {
	switch e {
	case EndpointInverter, EndpointBattery, EndpointEVCharger,
		EndpointHeatPump, EndpointWaterHeater, EndpointHVAC, EndpointAppliance:
		return true
	default:
		return false
	}
}
