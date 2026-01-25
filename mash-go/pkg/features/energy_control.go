package features

import (
	"context"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// EnergyControl attribute IDs.
const (
	// Device type and control state (1-9)
	EnergyControlAttrDeviceType   uint16 = 1
	EnergyControlAttrControlState uint16 = 2
	EnergyControlAttrOptOutState  uint16 = 3

	// Control capabilities (10-19)
	EnergyControlAttrAcceptsLimits          uint16 = 10
	EnergyControlAttrAcceptsCurrentLimits   uint16 = 11
	EnergyControlAttrAcceptsSetpoints       uint16 = 12
	EnergyControlAttrAcceptsCurrentSetpoints uint16 = 13
	EnergyControlAttrIsPausable             uint16 = 14
	EnergyControlAttrIsShiftable            uint16 = 15
	EnergyControlAttrIsStoppable            uint16 = 16

	// Power limits (20-29)
	EnergyControlAttrEffectiveConsumptionLimit uint16 = 20
	EnergyControlAttrMyConsumptionLimit        uint16 = 21
	EnergyControlAttrEffectiveProductionLimit  uint16 = 22
	EnergyControlAttrMyProductionLimit         uint16 = 23

	// Phase current limits - consumption (30-39)
	EnergyControlAttrEffectiveCurrentLimitsConsumption uint16 = 30
	EnergyControlAttrMyCurrentLimitsConsumption        uint16 = 31

	// Phase current limits - production
	EnergyControlAttrEffectiveCurrentLimitsProduction uint16 = 32
	EnergyControlAttrMyCurrentLimitsProduction        uint16 = 33

	// Power setpoints (40-49)
	EnergyControlAttrEffectiveConsumptionSetpoint uint16 = 40
	EnergyControlAttrMyConsumptionSetpoint        uint16 = 41
	EnergyControlAttrEffectiveProductionSetpoint  uint16 = 42
	EnergyControlAttrMyProductionSetpoint         uint16 = 43

	// Phase current setpoints - consumption (50-59)
	EnergyControlAttrEffectiveCurrentSetpointsConsumption uint16 = 50
	EnergyControlAttrMyCurrentSetpointsConsumption        uint16 = 51

	// Phase current setpoints - production
	EnergyControlAttrEffectiveCurrentSetpointsProduction uint16 = 52
	EnergyControlAttrMyCurrentSetpointsProduction        uint16 = 53

	// Failsafe configuration (70-79)
	EnergyControlAttrFailsafeConsumptionLimit uint16 = 70
	EnergyControlAttrFailsafeProductionLimit  uint16 = 71
	EnergyControlAttrFailsafeDuration         uint16 = 72

	// Process management (80-89)
	EnergyControlAttrProcessState    uint16 = 80
	EnergyControlAttrOptionalProcess uint16 = 81
)

// EnergyControl command IDs.
const (
	EnergyControlCmdSetLimit            uint8 = 1
	EnergyControlCmdClearLimit          uint8 = 2
	EnergyControlCmdSetCurrentLimits    uint8 = 3
	EnergyControlCmdClearCurrentLimits  uint8 = 4
	EnergyControlCmdSetSetpoint         uint8 = 5
	EnergyControlCmdClearSetpoint       uint8 = 6
	EnergyControlCmdSetCurrentSetpoints uint8 = 7
	EnergyControlCmdClearCurrentSetpoints uint8 = 8
	EnergyControlCmdPause               uint8 = 9
	EnergyControlCmdResume              uint8 = 10
	EnergyControlCmdStop                uint8 = 11
)

// EnergyControlFeatureRevision is the current revision of the EnergyControl feature.
const EnergyControlFeatureRevision uint16 = 1

// DeviceType represents the type of controllable device.
type DeviceType uint8

const (
	DeviceTypeEVSE         DeviceType = 0x00
	DeviceTypeHeatPump     DeviceType = 0x01
	DeviceTypeWaterHeater  DeviceType = 0x02
	DeviceTypeBattery      DeviceType = 0x03
	DeviceTypeInverter     DeviceType = 0x04
	DeviceTypeFlexibleLoad DeviceType = 0x05
	DeviceTypeOther        DeviceType = 0xFF
)

// String returns the device type name.
func (d DeviceType) String() string {
	switch d {
	case DeviceTypeEVSE:
		return "EVSE"
	case DeviceTypeHeatPump:
		return "HEAT_PUMP"
	case DeviceTypeWaterHeater:
		return "WATER_HEATER"
	case DeviceTypeBattery:
		return "BATTERY"
	case DeviceTypeInverter:
		return "INVERTER"
	case DeviceTypeFlexibleLoad:
		return "FLEXIBLE_LOAD"
	case DeviceTypeOther:
		return "OTHER"
	default:
		return "UNKNOWN"
	}
}

// ControlState represents the control relationship state.
type ControlState uint8

const (
	// ControlStateAutonomous means not under external control.
	ControlStateAutonomous ControlState = 0x00

	// ControlStateControlled means under controller authority, no active limit.
	ControlStateControlled ControlState = 0x01

	// ControlStateLimited means active power limit being applied.
	ControlStateLimited ControlState = 0x02

	// ControlStateFailsafe means connection lost, using failsafe limits.
	ControlStateFailsafe ControlState = 0x03

	// ControlStateOverride means device overriding limits (safety/legal).
	ControlStateOverride ControlState = 0x04
)

// String returns the control state name.
func (c ControlState) String() string {
	switch c {
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

// ProcessState represents the optional task lifecycle state.
type ProcessState uint8

const (
	ProcessStateNone      ProcessState = 0x00
	ProcessStateAvailable ProcessState = 0x01
	ProcessStateScheduled ProcessState = 0x02
	ProcessStateRunning   ProcessState = 0x03
	ProcessStatePaused    ProcessState = 0x04
	ProcessStateCompleted ProcessState = 0x05
	ProcessStateAborted   ProcessState = 0x06
)

// String returns the process state name.
func (p ProcessState) String() string {
	switch p {
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

// OptOutState represents the opt-out state for external control.
type OptOutState uint8

const (
	OptOutNone      OptOutState = 0x00
	OptOutLocal     OptOutState = 0x01
	OptOutGrid      OptOutState = 0x02
	OptOutAll       OptOutState = 0x03
)

// LimitCause represents the cause/reason for a limit.
type LimitCause uint8

const (
	LimitCauseGridEmergency     LimitCause = 0
	LimitCauseGridOptimization  LimitCause = 1
	LimitCauseLocalProtection   LimitCause = 2
	LimitCauseLocalOptimization LimitCause = 3
	LimitCauseUserPreference    LimitCause = 4
)

// SetpointCause represents the cause/reason for a setpoint.
type SetpointCause uint8

const (
	SetpointCauseGridRequest       SetpointCause = 0
	SetpointCauseSelfConsumption   SetpointCause = 1
	SetpointCausePriceOptimization SetpointCause = 2
	SetpointCausePhaseBalancing    SetpointCause = 3
	SetpointCauseUserPreference    SetpointCause = 4
)

// EnergyControl wraps a Feature with EnergyControl-specific functionality.
type EnergyControl struct {
	*model.Feature

	// Handler callbacks for commands
	onSetLimit            func(ctx context.Context, consumptionLimit, productionLimit *int64, cause LimitCause) (int64, int64, error)
	onClearLimit          func(ctx context.Context, direction *Direction) error
	onSetCurrentLimits    func(ctx context.Context, phases map[Phase]int64, direction Direction, cause LimitCause) (map[Phase]int64, error)
	onClearCurrentLimits  func(ctx context.Context, direction *Direction) error
	onSetSetpoint         func(ctx context.Context, consumptionSetpoint, productionSetpoint *int64, cause SetpointCause) (int64, int64, error)
	onClearSetpoint       func(ctx context.Context, direction *Direction) error
	onPause               func(ctx context.Context, duration *uint32) error
	onResume              func(ctx context.Context) error
	onStop                func(ctx context.Context) error
}

// NewEnergyControl creates a new EnergyControl feature.
func NewEnergyControl() *EnergyControl {
	f := model.NewFeature(model.FeatureEnergyControl, EnergyControlFeatureRevision)

	// Device type and control state
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          EnergyControlAttrDeviceType,
		Name:        "deviceType",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(DeviceTypeOther),
		Description: "Type of controllable device",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          EnergyControlAttrControlState,
		Name:        "controlState",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(ControlStateAutonomous),
		Description: "Control relationship state",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          EnergyControlAttrOptOutState,
		Name:        "optOutState",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadWrite,
		Default:     uint8(OptOutNone),
		Description: "Opt-out state for external control",
	}))

	// Control capabilities
	addBoolAttr := func(id uint16, name, desc string) {
		f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
			ID:          id,
			Name:        name,
			Type:        model.DataTypeBool,
			Access:      model.AccessReadOnly,
			Default:     false,
			Description: desc,
		}))
	}

	addBoolAttr(EnergyControlAttrAcceptsLimits, "acceptsLimits", "Accepts SetLimit command")
	addBoolAttr(EnergyControlAttrAcceptsCurrentLimits, "acceptsCurrentLimits", "Accepts SetCurrentLimits command")
	addBoolAttr(EnergyControlAttrAcceptsSetpoints, "acceptsSetpoints", "Accepts SetSetpoint command")
	addBoolAttr(EnergyControlAttrAcceptsCurrentSetpoints, "acceptsCurrentSetpoints", "Accepts SetCurrentSetpoints command")
	addBoolAttr(EnergyControlAttrIsPausable, "isPausable", "Accepts Pause/Resume commands")
	addBoolAttr(EnergyControlAttrIsShiftable, "isShiftable", "Accepts AdjustStartTime command")
	addBoolAttr(EnergyControlAttrIsStoppable, "isStoppable", "Accepts Stop command")

	// Power limits
	addPowerAttr := func(id uint16, name, desc string, writable bool) {
		access := model.AccessReadOnly
		if writable {
			access = model.AccessReadWrite
		}
		f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
			ID:          id,
			Name:        name,
			Type:        model.DataTypeInt64,
			Access:      access,
			Nullable:    true,
			Unit:        "mW",
			Description: desc,
		}))
	}

	addPowerAttr(EnergyControlAttrEffectiveConsumptionLimit, "effectiveConsumptionLimit", "Effective consumption limit (min of all zones)", false)
	addPowerAttr(EnergyControlAttrMyConsumptionLimit, "myConsumptionLimit", "This zone's consumption limit", true)
	addPowerAttr(EnergyControlAttrEffectiveProductionLimit, "effectiveProductionLimit", "Effective production limit (min of all zones)", false)
	addPowerAttr(EnergyControlAttrMyProductionLimit, "myProductionLimit", "This zone's production limit", true)

	// Phase current limits
	addPhaseMapAttr := func(id uint16, name, desc string) {
		f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
			ID:          id,
			Name:        name,
			Type:        model.DataTypeMap,
			Access:      model.AccessReadOnly,
			Nullable:    true,
			Description: desc,
		}))
	}

	addPhaseMapAttr(EnergyControlAttrEffectiveCurrentLimitsConsumption, "effectiveCurrentLimitsConsumption", "Effective per-phase current limits (consumption)")
	addPhaseMapAttr(EnergyControlAttrMyCurrentLimitsConsumption, "myCurrentLimitsConsumption", "This zone's per-phase current limits (consumption)")
	addPhaseMapAttr(EnergyControlAttrEffectiveCurrentLimitsProduction, "effectiveCurrentLimitsProduction", "Effective per-phase current limits (production)")
	addPhaseMapAttr(EnergyControlAttrMyCurrentLimitsProduction, "myCurrentLimitsProduction", "This zone's per-phase current limits (production)")

	// Power setpoints
	addPowerAttr(EnergyControlAttrEffectiveConsumptionSetpoint, "effectiveConsumptionSetpoint", "Effective consumption setpoint", false)
	addPowerAttr(EnergyControlAttrMyConsumptionSetpoint, "myConsumptionSetpoint", "This zone's consumption setpoint", true)
	addPowerAttr(EnergyControlAttrEffectiveProductionSetpoint, "effectiveProductionSetpoint", "Effective production setpoint", false)
	addPowerAttr(EnergyControlAttrMyProductionSetpoint, "myProductionSetpoint", "This zone's production setpoint", true)

	// Phase current setpoints
	addPhaseMapAttr(EnergyControlAttrEffectiveCurrentSetpointsConsumption, "effectiveCurrentSetpointsConsumption", "Effective per-phase current setpoints (consumption)")
	addPhaseMapAttr(EnergyControlAttrMyCurrentSetpointsConsumption, "myCurrentSetpointsConsumption", "This zone's per-phase current setpoints (consumption)")
	addPhaseMapAttr(EnergyControlAttrEffectiveCurrentSetpointsProduction, "effectiveCurrentSetpointsProduction", "Effective per-phase current setpoints (production)")
	addPhaseMapAttr(EnergyControlAttrMyCurrentSetpointsProduction, "myCurrentSetpointsProduction", "This zone's per-phase current setpoints (production)")

	// Failsafe configuration
	addPowerAttr(EnergyControlAttrFailsafeConsumptionLimit, "failsafeConsumptionLimit", "Limit to apply in FAILSAFE state", true)
	addPowerAttr(EnergyControlAttrFailsafeProductionLimit, "failsafeProductionLimit", "Production limit in FAILSAFE state", true)

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          EnergyControlAttrFailsafeDuration,
		Name:        "failsafeDuration",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadWrite,
		Default:     uint32(7200), // 2 hours default
		Unit:        "s",
		Description: "Time in FAILSAFE before returning to AUTONOMOUS (2-24h)",
	}))

	// Process management
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          EnergyControlAttrProcessState,
		Name:        "processState",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(ProcessStateNone),
		Description: "Current process lifecycle state",
	}))

	ec := &EnergyControl{Feature: f}
	ec.addCommands()

	return ec
}

// addCommands adds the EnergyControl commands.
func (e *EnergyControl) addCommands() {
	// SetLimit command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdSetLimit,
		Name:        "setLimit",
		Description: "Set power limits for this zone",
		Parameters: []model.ParameterMetadata{
			{Name: "consumptionLimit", Type: model.DataTypeInt64, Required: false},
			{Name: "productionLimit", Type: model.DataTypeInt64, Required: false},
			{Name: "duration", Type: model.DataTypeUint32, Required: false},
			{Name: "cause", Type: model.DataTypeUint8, Required: true},
		},
	}, e.handleSetLimit))

	// ClearLimit command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdClearLimit,
		Name:        "clearLimit",
		Description: "Remove this zone's power limits",
		Parameters: []model.ParameterMetadata{
			{Name: "direction", Type: model.DataTypeUint8, Required: false},
		},
	}, e.handleClearLimit))

	// SetCurrentLimits command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdSetCurrentLimits,
		Name:        "setCurrentLimits",
		Description: "Set per-phase current limits",
		Parameters: []model.ParameterMetadata{
			{Name: "phases", Type: model.DataTypeMap, Required: true},
			{Name: "direction", Type: model.DataTypeUint8, Required: true},
			{Name: "duration", Type: model.DataTypeUint32, Required: false},
			{Name: "cause", Type: model.DataTypeUint8, Required: true},
		},
	}, e.handleSetCurrentLimits))

	// ClearCurrentLimits command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdClearCurrentLimits,
		Name:        "clearCurrentLimits",
		Description: "Remove this zone's per-phase current limits",
		Parameters: []model.ParameterMetadata{
			{Name: "direction", Type: model.DataTypeUint8, Required: false},
		},
	}, e.handleClearCurrentLimits))

	// SetSetpoint command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdSetSetpoint,
		Name:        "setSetpoint",
		Description: "Set power setpoint for this zone",
		Parameters: []model.ParameterMetadata{
			{Name: "consumptionSetpoint", Type: model.DataTypeInt64, Required: false},
			{Name: "productionSetpoint", Type: model.DataTypeInt64, Required: false},
			{Name: "duration", Type: model.DataTypeUint32, Required: false},
			{Name: "cause", Type: model.DataTypeUint8, Required: true},
		},
	}, e.handleSetSetpoint))

	// ClearSetpoint command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdClearSetpoint,
		Name:        "clearSetpoint",
		Description: "Remove this zone's power setpoints",
		Parameters: []model.ParameterMetadata{
			{Name: "direction", Type: model.DataTypeUint8, Required: false},
		},
	}, e.handleClearSetpoint))

	// Pause command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdPause,
		Name:        "pause",
		Description: "Temporarily pause device operation",
		Parameters: []model.ParameterMetadata{
			{Name: "duration", Type: model.DataTypeUint32, Required: false},
		},
	}, e.handlePause))

	// Resume command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdResume,
		Name:        "resume",
		Description: "Resume paused operation",
	}, e.handleResume))

	// Stop command
	e.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          EnergyControlCmdStop,
		Name:        "stop",
		Description: "Abort task completely",
	}, e.handleStop))
}

// Command handlers

func (e *EnergyControl) handleSetLimit(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onSetLimit == nil {
		return map[string]any{"success": false}, nil
	}

	var consumptionLimit, productionLimit *int64
	if v, ok := params["consumptionLimit"].(int64); ok {
		consumptionLimit = &v
	}
	if v, ok := params["productionLimit"].(int64); ok {
		productionLimit = &v
	}

	cause := LimitCause(0)
	if v, ok := params["cause"].(uint8); ok {
		cause = LimitCause(v)
	}

	effConsumption, effProduction, err := e.onSetLimit(ctx, consumptionLimit, productionLimit, cause)
	if err != nil {
		return map[string]any{"success": false}, err
	}

	return map[string]any{
		"success":                    true,
		"effectiveConsumptionLimit":  effConsumption,
		"effectiveProductionLimit":   effProduction,
	}, nil
}

func (e *EnergyControl) handleClearLimit(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onClearLimit == nil {
		return map[string]any{"success": false}, nil
	}

	var direction *Direction
	if v, ok := params["direction"].(uint8); ok {
		d := Direction(v)
		direction = &d
	}

	err := e.onClearLimit(ctx, direction)
	return map[string]any{"success": err == nil}, err
}

func (e *EnergyControl) handleSetCurrentLimits(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onSetCurrentLimits == nil {
		return map[string]any{"success": false}, nil
	}

	phases := make(map[Phase]int64)
	if p, ok := params["phases"].(map[any]any); ok {
		for k, v := range p {
			if phase, ok := k.(uint8); ok {
				if current, ok := v.(int64); ok {
					phases[Phase(phase)] = current
				}
			}
		}
	}

	direction := DirectionConsumption
	if v, ok := params["direction"].(uint8); ok {
		direction = Direction(v)
	}

	cause := LimitCause(0)
	if v, ok := params["cause"].(uint8); ok {
		cause = LimitCause(v)
	}

	effective, err := e.onSetCurrentLimits(ctx, phases, direction, cause)
	if err != nil {
		return map[string]any{"success": false}, err
	}

	return map[string]any{
		"success":               true,
		"effectivePhaseCurrents": effective,
	}, nil
}

func (e *EnergyControl) handleClearCurrentLimits(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onClearCurrentLimits == nil {
		return map[string]any{"success": false}, nil
	}

	var direction *Direction
	if v, ok := params["direction"].(uint8); ok {
		d := Direction(v)
		direction = &d
	}

	err := e.onClearCurrentLimits(ctx, direction)
	return map[string]any{"success": err == nil}, err
}

func (e *EnergyControl) handleSetSetpoint(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onSetSetpoint == nil {
		return map[string]any{"success": false}, nil
	}

	var consumptionSetpoint, productionSetpoint *int64
	if v, ok := params["consumptionSetpoint"].(int64); ok {
		consumptionSetpoint = &v
	}
	if v, ok := params["productionSetpoint"].(int64); ok {
		productionSetpoint = &v
	}

	cause := SetpointCause(0)
	if v, ok := params["cause"].(uint8); ok {
		cause = SetpointCause(v)
	}

	effConsumption, effProduction, err := e.onSetSetpoint(ctx, consumptionSetpoint, productionSetpoint, cause)
	if err != nil {
		return map[string]any{"success": false}, err
	}

	return map[string]any{
		"success":                     true,
		"effectiveConsumptionSetpoint": effConsumption,
		"effectiveProductionSetpoint":  effProduction,
	}, nil
}

func (e *EnergyControl) handleClearSetpoint(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onClearSetpoint == nil {
		return map[string]any{"success": false}, nil
	}

	var direction *Direction
	if v, ok := params["direction"].(uint8); ok {
		d := Direction(v)
		direction = &d
	}

	err := e.onClearSetpoint(ctx, direction)
	return map[string]any{"success": err == nil}, err
}

func (e *EnergyControl) handlePause(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onPause == nil {
		return map[string]any{"success": false}, nil
	}

	var duration *uint32
	if v, ok := params["duration"].(uint32); ok {
		duration = &v
	}

	err := e.onPause(ctx, duration)
	return map[string]any{"success": err == nil}, err
}

func (e *EnergyControl) handleResume(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onResume == nil {
		return map[string]any{"success": false}, nil
	}

	err := e.onResume(ctx)
	return map[string]any{"success": err == nil}, err
}

func (e *EnergyControl) handleStop(ctx context.Context, params map[string]any) (map[string]any, error) {
	if e.onStop == nil {
		return map[string]any{"success": false}, nil
	}

	err := e.onStop(ctx)
	return map[string]any{"success": err == nil}, err
}

// Handler setters

// OnSetLimit sets the handler for SetLimit command.
func (e *EnergyControl) OnSetLimit(handler func(ctx context.Context, consumptionLimit, productionLimit *int64, cause LimitCause) (int64, int64, error)) {
	e.onSetLimit = handler
}

// OnClearLimit sets the handler for ClearLimit command.
func (e *EnergyControl) OnClearLimit(handler func(ctx context.Context, direction *Direction) error) {
	e.onClearLimit = handler
}

// OnSetCurrentLimits sets the handler for SetCurrentLimits command.
func (e *EnergyControl) OnSetCurrentLimits(handler func(ctx context.Context, phases map[Phase]int64, direction Direction, cause LimitCause) (map[Phase]int64, error)) {
	e.onSetCurrentLimits = handler
}

// OnClearCurrentLimits sets the handler for ClearCurrentLimits command.
func (e *EnergyControl) OnClearCurrentLimits(handler func(ctx context.Context, direction *Direction) error) {
	e.onClearCurrentLimits = handler
}

// OnSetSetpoint sets the handler for SetSetpoint command.
func (e *EnergyControl) OnSetSetpoint(handler func(ctx context.Context, consumptionSetpoint, productionSetpoint *int64, cause SetpointCause) (int64, int64, error)) {
	e.onSetSetpoint = handler
}

// OnClearSetpoint sets the handler for ClearSetpoint command.
func (e *EnergyControl) OnClearSetpoint(handler func(ctx context.Context, direction *Direction) error) {
	e.onClearSetpoint = handler
}

// OnPause sets the handler for Pause command.
func (e *EnergyControl) OnPause(handler func(ctx context.Context, duration *uint32) error) {
	e.onPause = handler
}

// OnResume sets the handler for Resume command.
func (e *EnergyControl) OnResume(handler func(ctx context.Context) error) {
	e.onResume = handler
}

// OnStop sets the handler for Stop command.
func (e *EnergyControl) OnStop(handler func(ctx context.Context) error) {
	e.onStop = handler
}

// Attribute setters

// SetDeviceType sets the device type.
func (e *EnergyControl) SetDeviceType(dt DeviceType) error {
	attr, err := e.GetAttribute(EnergyControlAttrDeviceType)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(dt))
}

// SetControlState sets the control state.
func (e *EnergyControl) SetControlState(state ControlState) error {
	attr, err := e.GetAttribute(EnergyControlAttrControlState)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(state))
}

// SetCapabilities sets the control capabilities.
func (e *EnergyControl) SetCapabilities(limits, currentLimits, setpoints, currentSetpoints, pausable, shiftable, stoppable bool) {
	setAttr := func(id uint16, val bool) {
		if attr, err := e.GetAttribute(id); err == nil {
			_ = attr.SetValueInternal(val)
		}
	}
	setAttr(EnergyControlAttrAcceptsLimits, limits)
	setAttr(EnergyControlAttrAcceptsCurrentLimits, currentLimits)
	setAttr(EnergyControlAttrAcceptsSetpoints, setpoints)
	setAttr(EnergyControlAttrAcceptsCurrentSetpoints, currentSetpoints)
	setAttr(EnergyControlAttrIsPausable, pausable)
	setAttr(EnergyControlAttrIsShiftable, shiftable)
	setAttr(EnergyControlAttrIsStoppable, stoppable)
}

// SetEffectiveConsumptionLimit sets the effective consumption limit.
func (e *EnergyControl) SetEffectiveConsumptionLimit(limit *int64) error {
	attr, err := e.GetAttribute(EnergyControlAttrEffectiveConsumptionLimit)
	if err != nil {
		return err
	}
	if limit == nil {
		return attr.SetValueInternal(nil)
	}
	return attr.SetValueInternal(*limit)
}

// SetEffectiveProductionLimit sets the effective production limit.
func (e *EnergyControl) SetEffectiveProductionLimit(limit *int64) error {
	attr, err := e.GetAttribute(EnergyControlAttrEffectiveProductionLimit)
	if err != nil {
		return err
	}
	if limit == nil {
		return attr.SetValueInternal(nil)
	}
	return attr.SetValueInternal(*limit)
}

// SetProcessState sets the process state.
func (e *EnergyControl) SetProcessState(state ProcessState) error {
	attr, err := e.GetAttribute(EnergyControlAttrProcessState)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(state))
}

// Getters

// DeviceType returns the device type.
func (e *EnergyControl) DeviceType() DeviceType {
	val, _ := e.ReadAttribute(EnergyControlAttrDeviceType)
	if v, ok := val.(uint8); ok {
		return DeviceType(v)
	}
	return DeviceTypeOther
}

// ControlState returns the control state.
func (e *EnergyControl) ControlState() ControlState {
	val, _ := e.ReadAttribute(EnergyControlAttrControlState)
	if v, ok := val.(uint8); ok {
		return ControlState(v)
	}
	return ControlStateAutonomous
}

// ProcessState returns the process state.
func (e *EnergyControl) ProcessState() ProcessState {
	val, _ := e.ReadAttribute(EnergyControlAttrProcessState)
	if v, ok := val.(uint8); ok {
		return ProcessState(v)
	}
	return ProcessStateNone
}

// EffectiveConsumptionLimit returns the effective consumption limit.
func (e *EnergyControl) EffectiveConsumptionLimit() (int64, bool) {
	val, err := e.ReadAttribute(EnergyControlAttrEffectiveConsumptionLimit)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// EffectiveProductionLimit returns the effective production limit.
func (e *EnergyControl) EffectiveProductionLimit() (int64, bool) {
	val, err := e.ReadAttribute(EnergyControlAttrEffectiveProductionLimit)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// AcceptsLimits returns true if the device accepts SetLimit command.
func (e *EnergyControl) AcceptsLimits() bool {
	val, _ := e.ReadAttribute(EnergyControlAttrAcceptsLimits)
	if v, ok := val.(bool); ok {
		return v
	}
	return false
}

// IsPausable returns true if the device can be paused.
func (e *EnergyControl) IsPausable() bool {
	val, _ := e.ReadAttribute(EnergyControlAttrIsPausable)
	if v, ok := val.(bool); ok {
		return v
	}
	return false
}

// IsLimited returns true if currently in LIMITED state.
func (e *EnergyControl) IsLimited() bool {
	return e.ControlState() == ControlStateLimited
}

// IsFailsafe returns true if in FAILSAFE state.
func (e *EnergyControl) IsFailsafe() bool {
	return e.ControlState() == ControlStateFailsafe
}
