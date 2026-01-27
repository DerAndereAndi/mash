package wire

// ControlState represents the control state of a device endpoint.
// See docs/testing/behavior/state-machines.md for state machine details.
type ControlState uint8

const (
	// ControlStateAutonomous indicates no controller has set limits/setpoints.
	// Device operates according to its own logic.
	ControlStateAutonomous ControlState = 0

	// ControlStateControlled indicates a controller has set setpoints.
	// Device follows controller guidance for optimization.
	ControlStateControlled ControlState = 1

	// ControlStateLimited indicates a controller has set limits.
	// Device is constrained but may operate within limits.
	ControlStateLimited ControlState = 2

	// ControlStateFailsafe indicates connection to all controlling zones was lost.
	// Device applies failsafe limits until reconnection or timer expiry.
	ControlStateFailsafe ControlState = 3

	// ControlStateOverride indicates local override is active.
	// User has taken manual control, ignoring remote commands.
	ControlStateOverride ControlState = 4
)

// String returns the control state name.
func (s ControlState) String() string {
	switch s {
	case ControlStateAutonomous:
		return "AUTONOMOUS"
	case ControlStateControlled:
		return "CONTROLLED"
	case ControlStateLimited:
		return "LIMITED"
	case ControlStateFailsafe:
		return "FAILSAFE"
	case ControlStateOverride:
		return "OVERRIDE"
	default:
		return "UNKNOWN"
	}
}

// ProcessState represents the state of an ongoing process.
// ProcessState is orthogonal to ControlState - a process can be
// RUNNING while the device is in FAILSAFE or LIMITED state.
type ProcessState uint8

const (
	// ProcessStateNone indicates no process is active or pending.
	ProcessStateNone ProcessState = 0

	// ProcessStateAvailable indicates the device is ready to accept a process.
	ProcessStateAvailable ProcessState = 1

	// ProcessStateScheduled indicates a process is scheduled for later.
	ProcessStateScheduled ProcessState = 2

	// ProcessStateRunning indicates a process is actively running.
	ProcessStateRunning ProcessState = 3

	// ProcessStatePaused indicates a running process has been paused.
	ProcessStatePaused ProcessState = 4

	// ProcessStateCompleted indicates a process finished successfully.
	ProcessStateCompleted ProcessState = 5

	// ProcessStateAborted indicates a process was stopped before completion.
	ProcessStateAborted ProcessState = 6
)

// String returns the process state name.
func (s ProcessState) String() string {
	switch s {
	case ProcessStateNone:
		return "NONE"
	case ProcessStateAvailable:
		return "AVAILABLE"
	case ProcessStateScheduled:
		return "SCHEDULED"
	case ProcessStateRunning:
		return "RUNNING"
	case ProcessStatePaused:
		return "PAUSED"
	case ProcessStateCompleted:
		return "COMPLETED"
	case ProcessStateAborted:
		return "ABORTED"
	default:
		return "UNKNOWN"
	}
}

// OperatingState represents the operating state of a device.
type OperatingState uint8

const (
	// OperatingStateNormal indicates the device is operating normally.
	OperatingStateNormal OperatingState = 0

	// OperatingStateStandby indicates the device is in standby/sleep mode.
	OperatingStateStandby OperatingState = 1

	// OperatingStateError indicates the device has an error condition.
	OperatingStateError OperatingState = 2

	// OperatingStateMaintenance indicates the device is in maintenance mode.
	OperatingStateMaintenance OperatingState = 3

	// OperatingStateOffline indicates the device is offline/disconnected.
	OperatingStateOffline OperatingState = 4
)

// String returns the operating state name.
func (s OperatingState) String() string {
	switch s {
	case OperatingStateNormal:
		return "NORMAL"
	case OperatingStateStandby:
		return "STANDBY"
	case OperatingStateError:
		return "ERROR"
	case OperatingStateMaintenance:
		return "MAINTENANCE"
	case OperatingStateOffline:
		return "OFFLINE"
	default:
		return "UNKNOWN"
	}
}

// ZoneType represents the type of zone (controller role).
// Lower priority number = higher authority.
type ZoneType uint8

const (
	// ZoneTypeGrid has the highest priority (1).
	// External/regulatory authority - DSO, smart meter gateway, aggregators.
	ZoneTypeGrid ZoneType = 1

	// ZoneTypeLocal has priority 2.
	// Local energy management - EMS (residential or commercial).
	ZoneTypeLocal ZoneType = 2
)

// String returns the zone type name.
func (z ZoneType) String() string {
	switch z {
	case ZoneTypeGrid:
		return "GRID"
	case ZoneTypeLocal:
		return "LOCAL"
	default:
		return "UNKNOWN"
	}
}

// Priority returns the priority value for this zone type.
// Lower number = higher priority.
func (z ZoneType) Priority() uint8 {
	return uint8(z)
}

// CanOverride returns true if this zone type can override the other zone type.
func (z ZoneType) CanOverride(other ZoneType) bool {
	return z.Priority() < other.Priority()
}

// DeviceType represents the type of energy device.
type DeviceType uint8

const (
	DeviceTypeGeneric        DeviceType = 0
	DeviceTypeEVSE           DeviceType = 1  // EV Charger
	DeviceTypeHeatPump       DeviceType = 2  // Heat Pump
	DeviceTypeBattery        DeviceType = 3  // Battery Storage
	DeviceTypeInverter       DeviceType = 4  // PV/Battery Inverter
	DeviceTypePVString       DeviceType = 5  // PV String
	DeviceTypeSmartAppliance DeviceType = 6  // Dishwasher, Washer, etc.
	DeviceTypeHVAC           DeviceType = 7  // Heating/Cooling
	DeviceTypeWaterHeater    DeviceType = 8  // Water Heater
	DeviceTypeMeter          DeviceType = 9  // Energy Meter
	DeviceTypeGridConnection DeviceType = 10 // Grid Connection Point
)

// String returns the device type name.
func (d DeviceType) String() string {
	switch d {
	case DeviceTypeGeneric:
		return "GENERIC"
	case DeviceTypeEVSE:
		return "EVSE"
	case DeviceTypeHeatPump:
		return "HEAT_PUMP"
	case DeviceTypeBattery:
		return "BATTERY"
	case DeviceTypeInverter:
		return "INVERTER"
	case DeviceTypePVString:
		return "PV_STRING"
	case DeviceTypeSmartAppliance:
		return "SMART_APPLIANCE"
	case DeviceTypeHVAC:
		return "HVAC"
	case DeviceTypeWaterHeater:
		return "WATER_HEATER"
	case DeviceTypeMeter:
		return "METER"
	case DeviceTypeGridConnection:
		return "GRID_CONNECTION"
	default:
		return "UNKNOWN"
	}
}

// EndpointType represents the type of endpoint within a device.
type EndpointType uint8

const (
	// EndpointTypeDeviceRoot is always endpoint 0.
	// Contains DeviceInfo feature with device-level information.
	EndpointTypeDeviceRoot EndpointType = 0

	// EndpointTypeEVCharger represents an EV charging connector.
	EndpointTypeEVCharger EndpointType = 1

	// EndpointTypeInverter represents a power inverter.
	EndpointTypeInverter EndpointType = 2

	// EndpointTypeBattery represents a battery storage system.
	EndpointTypeBattery EndpointType = 3

	// EndpointTypePVString represents a PV string/array.
	EndpointTypePVString EndpointType = 4

	// EndpointTypeHeatPump represents a heat pump.
	EndpointTypeHeatPump EndpointType = 5

	// EndpointTypeMeter represents an energy meter.
	EndpointTypeMeter EndpointType = 6

	// EndpointTypeGridConnection represents a grid connection point.
	EndpointTypeGridConnection EndpointType = 7
)

// String returns the endpoint type name.
func (e EndpointType) String() string {
	switch e {
	case EndpointTypeDeviceRoot:
		return "DEVICE_ROOT"
	case EndpointTypeEVCharger:
		return "EV_CHARGER"
	case EndpointTypeInverter:
		return "INVERTER"
	case EndpointTypeBattery:
		return "BATTERY"
	case EndpointTypePVString:
		return "PV_STRING"
	case EndpointTypeHeatPump:
		return "HEAT_PUMP"
	case EndpointTypeMeter:
		return "METER"
	case EndpointTypeGridConnection:
		return "GRID_CONNECTION"
	default:
		return "UNKNOWN"
	}
}

// EnergyDirection represents the direction of energy flow.
type EnergyDirection uint8

const (
	// EnergyDirectionConsumption means the device consumes energy (positive).
	EnergyDirectionConsumption EnergyDirection = 0

	// EnergyDirectionProduction means the device produces energy (negative).
	EnergyDirectionProduction EnergyDirection = 1

	// EnergyDirectionBidirectional means the device can both consume and produce.
	EnergyDirectionBidirectional EnergyDirection = 2
)

// String returns the energy direction name.
func (d EnergyDirection) String() string {
	switch d {
	case EnergyDirectionConsumption:
		return "CONSUMPTION"
	case EnergyDirectionProduction:
		return "PRODUCTION"
	case EnergyDirectionBidirectional:
		return "BIDIRECTIONAL"
	default:
		return "UNKNOWN"
	}
}

// LimitCause represents the reason for setting a limit.
type LimitCause uint8

const (
	LimitCauseUnspecified        LimitCause = 0
	LimitCauseGridProtection     LimitCause = 1 // Grid stability/protection
	LimitCauseLocalProtection    LimitCause = 2 // Local installation protection
	LimitCauseTariffOptimization LimitCause = 3 // Cost optimization
	LimitCauseSelfConsumption    LimitCause = 4 // Self-consumption optimization
	LimitCauseUserRequest        LimitCause = 5 // User-initiated
	LimitCauseSchedule           LimitCause = 6 // Scheduled limit
	LimitCauseEmergency          LimitCause = 7 // Emergency situation
)

// String returns the limit cause name.
func (c LimitCause) String() string {
	switch c {
	case LimitCauseUnspecified:
		return "UNSPECIFIED"
	case LimitCauseGridProtection:
		return "GRID_PROTECTION"
	case LimitCauseLocalProtection:
		return "LOCAL_PROTECTION"
	case LimitCauseTariffOptimization:
		return "TARIFF_OPTIMIZATION"
	case LimitCauseSelfConsumption:
		return "SELF_CONSUMPTION"
	case LimitCauseUserRequest:
		return "USER_REQUEST"
	case LimitCauseSchedule:
		return "SCHEDULE"
	case LimitCauseEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}
