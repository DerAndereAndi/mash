package model

// EndpointType identifies the type/purpose of an endpoint.
type EndpointType uint8

// Endpoint types.
const (
	// EndpointDeviceRoot is device-level metadata (always endpoint 0).
	EndpointDeviceRoot EndpointType = 0x00

	// EndpointGridConnection is an AC grid connection point (smart meter).
	EndpointGridConnection EndpointType = 0x01

	// EndpointInverter is an inverter AC side (grid-facing).
	EndpointInverter EndpointType = 0x02

	// EndpointPVString is a PV string / solar input (DC).
	EndpointPVString EndpointType = 0x03

	// EndpointBattery is battery storage (DC).
	EndpointBattery EndpointType = 0x04

	// EndpointEVCharger is an EVSE / wallbox.
	EndpointEVCharger EndpointType = 0x05

	// EndpointHeatPump is a heat pump.
	EndpointHeatPump EndpointType = 0x06

	// EndpointWaterHeater is a water heater / boiler.
	EndpointWaterHeater EndpointType = 0x07

	// EndpointHVAC is an HVAC system.
	EndpointHVAC EndpointType = 0x08

	// EndpointAppliance is a generic controllable appliance.
	EndpointAppliance EndpointType = 0x09

	// EndpointSubMeter is a sub-meter / circuit monitor.
	EndpointSubMeter EndpointType = 0x0A
)

// String returns the endpoint type name.
func (e EndpointType) String() string {
	switch e {
	case EndpointDeviceRoot:
		return "DEVICE_ROOT"
	case EndpointGridConnection:
		return "GRID_CONNECTION"
	case EndpointInverter:
		return "INVERTER"
	case EndpointPVString:
		return "PV_STRING"
	case EndpointBattery:
		return "BATTERY"
	case EndpointEVCharger:
		return "EV_CHARGER"
	case EndpointHeatPump:
		return "HEAT_PUMP"
	case EndpointWaterHeater:
		return "WATER_HEATER"
	case EndpointHVAC:
		return "HVAC"
	case EndpointAppliance:
		return "APPLIANCE"
	case EndpointSubMeter:
		return "SUB_METER"
	default:
		return "UNKNOWN"
	}
}

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
