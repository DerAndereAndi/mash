package features

import (
	"context"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// ChargingSession attribute IDs.
const (
	// Session state (1-9)
	ChargingSessionAttrState            uint16 = 1
	ChargingSessionAttrSessionID        uint16 = 2
	ChargingSessionAttrSessionStartTime uint16 = 3
	ChargingSessionAttrSessionEndTime   uint16 = 4

	// Session energy (10-19)
	ChargingSessionAttrSessionEnergyCharged    uint16 = 10
	ChargingSessionAttrSessionEnergyDischarged uint16 = 11

	// EV identifications (20-29)
	ChargingSessionAttrEVIdentifications uint16 = 20

	// EV battery state (30-39)
	ChargingSessionAttrEVStateOfCharge   uint16 = 30
	ChargingSessionAttrEVBatteryCapacity uint16 = 31

	// EV energy demands (40-49)
	ChargingSessionAttrEVDemandMode         uint16 = 40
	ChargingSessionAttrEVMinEnergyRequest   uint16 = 41
	ChargingSessionAttrEVMaxEnergyRequest   uint16 = 42
	ChargingSessionAttrEVTargetEnergyRequest uint16 = 43
	ChargingSessionAttrEVDepartureTime      uint16 = 44

	// V2G discharge constraints (50-59)
	ChargingSessionAttrEVMinDischargingRequest           uint16 = 50
	ChargingSessionAttrEVMaxDischargingRequest           uint16 = 51
	ChargingSessionAttrEVDischargeBelowTargetPermitted   uint16 = 52

	// Estimated times (60-69)
	ChargingSessionAttrEstimatedTimeToMinSoC    uint16 = 60
	ChargingSessionAttrEstimatedTimeToTargetSoC uint16 = 61
	ChargingSessionAttrEstimatedTimeToFullSoC   uint16 = 62

	// Charging mode (70-79)
	ChargingSessionAttrChargingMode          uint16 = 70
	ChargingSessionAttrSupportedChargingModes uint16 = 71
	ChargingSessionAttrSurplusThreshold      uint16 = 72

	// Start/stop delays (80-89)
	ChargingSessionAttrStartDelay uint16 = 80
	ChargingSessionAttrStopDelay  uint16 = 81
)

// ChargingSession command IDs.
const (
	ChargingSessionCmdSetChargingMode uint8 = 1
)

// ChargingSessionFeatureRevision is the current revision of the ChargingSession feature.
const ChargingSessionFeatureRevision uint16 = 1

// ChargingState represents the EV charging state.
type ChargingState uint8

const (
	ChargingStateNotPluggedIn       ChargingState = 0x00
	ChargingStatePluggedInNoDemand  ChargingState = 0x01
	ChargingStatePluggedInDemand    ChargingState = 0x02
	ChargingStatePluggedInCharging  ChargingState = 0x03
	ChargingStatePluggedInDischarging ChargingState = 0x04
	ChargingStateSessionComplete    ChargingState = 0x05
	ChargingStateFault              ChargingState = 0x06
)

// String returns the charging state name.
func (c ChargingState) String() string {
	switch c {
	case ChargingStateNotPluggedIn:
		return "NOT_PLUGGED_IN"
	case ChargingStatePluggedInNoDemand:
		return "PLUGGED_IN_NO_DEMAND"
	case ChargingStatePluggedInDemand:
		return "PLUGGED_IN_DEMAND"
	case ChargingStatePluggedInCharging:
		return "PLUGGED_IN_CHARGING"
	case ChargingStatePluggedInDischarging:
		return "PLUGGED_IN_DISCHARGING"
	case ChargingStateSessionComplete:
		return "SESSION_COMPLETE"
	case ChargingStateFault:
		return "FAULT"
	default:
		return "UNKNOWN"
	}
}

// EVDemandMode represents the EV's demand information mode.
type EVDemandMode uint8

const (
	EVDemandModeNone                 EVDemandMode = 0x00
	EVDemandModeSingleDemand         EVDemandMode = 0x01
	EVDemandModeScheduled            EVDemandMode = 0x02
	EVDemandModeDynamic              EVDemandMode = 0x03
	EVDemandModeDynamicBidirectional EVDemandMode = 0x04
)

// String returns the EV demand mode name.
func (m EVDemandMode) String() string {
	switch m {
	case EVDemandModeNone:
		return "NONE"
	case EVDemandModeSingleDemand:
		return "SINGLE_DEMAND"
	case EVDemandModeScheduled:
		return "SCHEDULED"
	case EVDemandModeDynamic:
		return "DYNAMIC"
	case EVDemandModeDynamicBidirectional:
		return "DYNAMIC_BIDIRECTIONAL"
	default:
		return "UNKNOWN"
	}
}

// EVIDType represents the type of EV identification.
type EVIDType uint8

const (
	EVIDTypePCID       EVIDType = 0x00
	EVIDTypeMACEUI48   EVIDType = 0x01
	EVIDTypeMACEUI64   EVIDType = 0x02
	EVIDTypeRFID       EVIDType = 0x03
	EVIDTypeVIN        EVIDType = 0x04
	EVIDTypeContractID EVIDType = 0x05
	EVIDTypeEVCCID     EVIDType = 0x06
	EVIDTypeOther      EVIDType = 0xFF
)

// String returns the EV ID type name.
func (t EVIDType) String() string {
	switch t {
	case EVIDTypePCID:
		return "PCID"
	case EVIDTypeMACEUI48:
		return "MAC_EUI48"
	case EVIDTypeMACEUI64:
		return "MAC_EUI64"
	case EVIDTypeRFID:
		return "RFID"
	case EVIDTypeVIN:
		return "VIN"
	case EVIDTypeContractID:
		return "CONTRACT_ID"
	case EVIDTypeEVCCID:
		return "EVCC_ID"
	case EVIDTypeOther:
		return "OTHER"
	default:
		return "UNKNOWN"
	}
}

// ChargingMode represents the charging optimization strategy.
type ChargingMode uint8

const (
	ChargingModeOff                ChargingMode = 0x00
	ChargingModePVSurplusOnly      ChargingMode = 0x01
	ChargingModePVSurplusThreshold ChargingMode = 0x02
	ChargingModePriceOptimized     ChargingMode = 0x03
	ChargingModeScheduled          ChargingMode = 0x04
)

// String returns the charging mode name.
func (m ChargingMode) String() string {
	switch m {
	case ChargingModeOff:
		return "OFF"
	case ChargingModePVSurplusOnly:
		return "PV_SURPLUS_ONLY"
	case ChargingModePVSurplusThreshold:
		return "PV_SURPLUS_THRESHOLD"
	case ChargingModePriceOptimized:
		return "PRICE_OPTIMIZED"
	case ChargingModeScheduled:
		return "SCHEDULED"
	default:
		return "UNKNOWN"
	}
}

// EVIdentification represents an EV identifier.
type EVIdentification struct {
	Type  EVIDType
	Value string
}

// ChargingSession wraps a Feature with ChargingSession-specific functionality.
type ChargingSession struct {
	*model.Feature

	// Handler callbacks
	onSetChargingMode func(ctx context.Context, mode ChargingMode, surplusThreshold *int64, startDelay, stopDelay *uint32) (ChargingMode, string, error)
}

// NewChargingSession creates a new ChargingSession feature.
func NewChargingSession() *ChargingSession {
	f := model.NewFeature(model.FeatureChargingSession, ChargingSessionFeatureRevision)

	// Session state attributes
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrState,
		Name:        "state",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(ChargingStateNotPluggedIn),
		Description: "Current charging state",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSessionID,
		Name:        "sessionId",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Default:     uint32(0),
		Description: "Unique session identifier",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSessionStartTime,
		Name:        "sessionStartTime",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Timestamp when EV connected",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSessionEndTime,
		Name:        "sessionEndTime",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Timestamp when session ended",
	}))

	// Session energy
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSessionEnergyCharged,
		Name:        "sessionEnergyCharged",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Default:     uint64(0),
		Unit:        "mWh",
		Description: "Energy delivered to EV this session",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSessionEnergyDischarged,
		Name:        "sessionEnergyDischarged",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Default:     uint64(0),
		Unit:        "mWh",
		Description: "Energy returned from EV (V2G) this session",
	}))

	// EV identifications
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVIdentifications,
		Name:        "evIdentifications",
		Type:        model.DataTypeArray,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "List of EV identifiers",
	}))

	// EV battery state
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVStateOfCharge,
		Name:        "evStateOfCharge",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "%",
		Description: "Current EV state of charge",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVBatteryCapacity,
		Name:        "evBatteryCapacity",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "EV battery capacity",
	}))

	// EV energy demands
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVDemandMode,
		Name:        "evDemandMode",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(EVDemandModeNone),
		Description: "EV demand information mode",
	}))

	addEnergyRequestAttr := func(id uint16, name, desc string) {
		f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
			ID:          id,
			Name:        name,
			Type:        model.DataTypeInt64,
			Access:      model.AccessReadOnly,
			Nullable:    true,
			Unit:        "mWh",
			Description: desc,
		}))
	}

	addEnergyRequestAttr(ChargingSessionAttrEVMinEnergyRequest, "evMinEnergyRequest", "Energy to minimum SoC (negative=can discharge)")
	addEnergyRequestAttr(ChargingSessionAttrEVMaxEnergyRequest, "evMaxEnergyRequest", "Energy to full charge")
	addEnergyRequestAttr(ChargingSessionAttrEVTargetEnergyRequest, "evTargetEnergyRequest", "Energy to target SoC")

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVDepartureTime,
		Name:        "evDepartureTime",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Timestamp when EV needs to leave",
	}))

	// V2G discharge constraints
	addEnergyRequestAttr(ChargingSessionAttrEVMinDischargingRequest, "evMinDischargingRequest", "Minimum discharge limit (must be <0)")
	addEnergyRequestAttr(ChargingSessionAttrEVMaxDischargingRequest, "evMaxDischargingRequest", "Maximum discharge limit (must be >=0)")

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrEVDischargeBelowTargetPermitted,
		Name:        "evDischargeBelowTargetPermitted",
		Type:        model.DataTypeBool,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Allow V2G below target SoC",
	}))

	// Estimated times
	addTimeEstimateAttr := func(id uint16, name, desc string) {
		f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
			ID:          id,
			Name:        name,
			Type:        model.DataTypeUint32,
			Access:      model.AccessReadOnly,
			Nullable:    true,
			Unit:        "s",
			Description: desc,
		}))
	}

	addTimeEstimateAttr(ChargingSessionAttrEstimatedTimeToMinSoC, "estimatedTimeToMinSoC", "Estimated time to reach minimum SoC")
	addTimeEstimateAttr(ChargingSessionAttrEstimatedTimeToTargetSoC, "estimatedTimeToTargetSoC", "Estimated time to reach target SoC")
	addTimeEstimateAttr(ChargingSessionAttrEstimatedTimeToFullSoC, "estimatedTimeToFullSoC", "Estimated time to full charge")

	// Charging mode
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrChargingMode,
		Name:        "chargingMode",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(ChargingModeOff),
		Description: "Active optimization strategy",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSupportedChargingModes,
		Name:        "supportedChargingModes",
		Type:        model.DataTypeArray,
		Access:      model.AccessReadOnly,
		Description: "Optimization modes EVSE supports",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrSurplusThreshold,
		Name:        "surplusThreshold",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mW",
		Description: "Threshold for PV_SURPLUS_THRESHOLD mode",
	}))

	// Start/stop delays
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrStartDelay,
		Name:        "startDelay",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Default:     uint32(0),
		Unit:        "s",
		Description: "Delay before (re)starting charge",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ChargingSessionAttrStopDelay,
		Name:        "stopDelay",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Default:     uint32(0),
		Unit:        "s",
		Description: "Delay before pausing charge",
	}))

	cs := &ChargingSession{Feature: f}
	cs.addCommands()

	return cs
}

// addCommands adds the ChargingSession commands.
func (c *ChargingSession) addCommands() {
	c.AddCommand(model.NewCommand(&model.CommandMetadata{
		ID:          ChargingSessionCmdSetChargingMode,
		Name:        "setChargingMode",
		Description: "Set the optimization strategy",
		Parameters: []model.ParameterMetadata{
			{Name: "mode", Type: model.DataTypeUint8, Required: true},
			{Name: "surplusThreshold", Type: model.DataTypeInt64, Required: false},
			{Name: "startDelay", Type: model.DataTypeUint32, Required: false},
			{Name: "stopDelay", Type: model.DataTypeUint32, Required: false},
		},
	}, c.handleSetChargingMode))
}

func (c *ChargingSession) handleSetChargingMode(ctx context.Context, params map[string]any) (map[string]any, error) {
	if c.onSetChargingMode == nil {
		return map[string]any{"success": false, "reason": "not implemented"}, nil
	}

	mode := ChargingModeOff
	if v, ok := params["mode"].(uint8); ok {
		mode = ChargingMode(v)
	}

	var surplusThreshold *int64
	if v, ok := params["surplusThreshold"].(int64); ok {
		surplusThreshold = &v
	}

	var startDelay *uint32
	if v, ok := params["startDelay"].(uint32); ok {
		startDelay = &v
	}

	var stopDelay *uint32
	if v, ok := params["stopDelay"].(uint32); ok {
		stopDelay = &v
	}

	activeMode, reason, err := c.onSetChargingMode(ctx, mode, surplusThreshold, startDelay, stopDelay)
	if err != nil {
		return map[string]any{
			"success":    false,
			"activeMode": uint8(activeMode),
			"reason":     reason,
		}, nil
	}

	return map[string]any{
		"success":    true,
		"activeMode": uint8(activeMode),
	}, nil
}

// Handler setters

// OnSetChargingMode sets the handler for SetChargingMode command.
func (c *ChargingSession) OnSetChargingMode(handler func(ctx context.Context, mode ChargingMode, surplusThreshold *int64, startDelay, stopDelay *uint32) (ChargingMode, string, error)) {
	c.onSetChargingMode = handler
}

// Attribute setters - Session state

// SetState sets the charging state.
func (c *ChargingSession) SetState(state ChargingState) error {
	attr, err := c.GetAttribute(ChargingSessionAttrState)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(state))
}

// SetSessionID sets the session identifier.
func (c *ChargingSession) SetSessionID(id uint32) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionID)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(id)
}

// SetSessionStartTime sets the session start timestamp.
func (c *ChargingSession) SetSessionStartTime(timestamp uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionStartTime)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(timestamp)
}

// SetSessionEndTime sets the session end timestamp.
func (c *ChargingSession) SetSessionEndTime(timestamp uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionEndTime)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(timestamp)
}

// ClearSessionEndTime clears the session end timestamp.
func (c *ChargingSession) ClearSessionEndTime() error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionEndTime)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// Attribute setters - Session energy

// SetSessionEnergyCharged sets the energy charged this session.
func (c *ChargingSession) SetSessionEnergyCharged(energy uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionEnergyCharged)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// SetSessionEnergyDischarged sets the energy discharged (V2G) this session.
func (c *ChargingSession) SetSessionEnergyDischarged(energy uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSessionEnergyDischarged)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// Attribute setters - EV identifications

// SetEVIdentifications sets the EV identifiers.
func (c *ChargingSession) SetEVIdentifications(ids []EVIdentification) error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVIdentifications)
	if err != nil {
		return err
	}
	if ids == nil {
		return attr.SetValueInternal(nil)
	}
	// Convert to serializable format
	data := make([]map[string]any, len(ids))
	for i, id := range ids {
		data[i] = map[string]any{
			"type":  uint8(id.Type),
			"value": id.Value,
		}
	}
	return attr.SetValueInternal(data)
}

// Attribute setters - EV battery state

// SetEVStateOfCharge sets the EV's current SoC.
func (c *ChargingSession) SetEVStateOfCharge(soc uint8) error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVStateOfCharge)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(soc)
}

// ClearEVStateOfCharge clears the EV's SoC (not available).
func (c *ChargingSession) ClearEVStateOfCharge() error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVStateOfCharge)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// SetEVBatteryCapacity sets the EV's battery capacity.
func (c *ChargingSession) SetEVBatteryCapacity(capacity uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVBatteryCapacity)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(capacity)
}

// ClearEVBatteryCapacity clears the EV's battery capacity.
func (c *ChargingSession) ClearEVBatteryCapacity() error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVBatteryCapacity)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// Attribute setters - EV energy demands

// SetEVDemandMode sets the EV's demand information mode.
func (c *ChargingSession) SetEVDemandMode(mode EVDemandMode) error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVDemandMode)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(mode))
}

// SetEVEnergyRequests sets the EV's energy requests.
func (c *ChargingSession) SetEVEnergyRequests(min, max, target *int64) error {
	setAttr := func(id uint16, val *int64) error {
		attr, err := c.GetAttribute(id)
		if err != nil {
			return err
		}
		if val == nil {
			return attr.SetValueInternal(nil)
		}
		return attr.SetValueInternal(*val)
	}

	if err := setAttr(ChargingSessionAttrEVMinEnergyRequest, min); err != nil {
		return err
	}
	if err := setAttr(ChargingSessionAttrEVMaxEnergyRequest, max); err != nil {
		return err
	}
	return setAttr(ChargingSessionAttrEVTargetEnergyRequest, target)
}

// SetEVDepartureTime sets the EV's departure time.
func (c *ChargingSession) SetEVDepartureTime(timestamp uint64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVDepartureTime)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(timestamp)
}

// ClearEVDepartureTime clears the EV's departure time.
func (c *ChargingSession) ClearEVDepartureTime() error {
	attr, err := c.GetAttribute(ChargingSessionAttrEVDepartureTime)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// Attribute setters - V2G constraints

// SetEVDischargeConstraints sets the V2G discharge constraints.
func (c *ChargingSession) SetEVDischargeConstraints(minDischarge, maxDischarge *int64, belowTargetPermitted *bool) error {
	setAttrInt := func(id uint16, val *int64) error {
		attr, err := c.GetAttribute(id)
		if err != nil {
			return err
		}
		if val == nil {
			return attr.SetValueInternal(nil)
		}
		return attr.SetValueInternal(*val)
	}

	if err := setAttrInt(ChargingSessionAttrEVMinDischargingRequest, minDischarge); err != nil {
		return err
	}
	if err := setAttrInt(ChargingSessionAttrEVMaxDischargingRequest, maxDischarge); err != nil {
		return err
	}

	attr, err := c.GetAttribute(ChargingSessionAttrEVDischargeBelowTargetPermitted)
	if err != nil {
		return err
	}
	if belowTargetPermitted == nil {
		return attr.SetValueInternal(nil)
	}
	return attr.SetValueInternal(*belowTargetPermitted)
}

// Attribute setters - Estimated times

// SetEstimatedTimes sets the estimated times to various SoC levels.
func (c *ChargingSession) SetEstimatedTimes(toMin, toTarget, toFull *uint32) error {
	setAttr := func(id uint16, val *uint32) error {
		attr, err := c.GetAttribute(id)
		if err != nil {
			return err
		}
		if val == nil {
			return attr.SetValueInternal(nil)
		}
		return attr.SetValueInternal(*val)
	}

	if err := setAttr(ChargingSessionAttrEstimatedTimeToMinSoC, toMin); err != nil {
		return err
	}
	if err := setAttr(ChargingSessionAttrEstimatedTimeToTargetSoC, toTarget); err != nil {
		return err
	}
	return setAttr(ChargingSessionAttrEstimatedTimeToFullSoC, toFull)
}

// Attribute setters - Charging mode

// SetChargingMode sets the active charging mode.
func (c *ChargingSession) SetChargingMode(mode ChargingMode) error {
	attr, err := c.GetAttribute(ChargingSessionAttrChargingMode)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(mode))
}

// SetSupportedChargingModes sets the supported charging modes.
func (c *ChargingSession) SetSupportedChargingModes(modes []ChargingMode) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSupportedChargingModes)
	if err != nil {
		return err
	}
	data := make([]uint8, len(modes))
	for i, m := range modes {
		data[i] = uint8(m)
	}
	return attr.SetValueInternal(data)
}

// SetSurplusThreshold sets the surplus threshold for PV_SURPLUS_THRESHOLD mode.
func (c *ChargingSession) SetSurplusThreshold(threshold int64) error {
	attr, err := c.GetAttribute(ChargingSessionAttrSurplusThreshold)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(threshold)
}

// ClearSurplusThreshold clears the surplus threshold.
func (c *ChargingSession) ClearSurplusThreshold() error {
	attr, err := c.GetAttribute(ChargingSessionAttrSurplusThreshold)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// Attribute setters - Delays

// SetStartDelay sets the start delay.
func (c *ChargingSession) SetStartDelay(seconds uint32) error {
	attr, err := c.GetAttribute(ChargingSessionAttrStartDelay)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(seconds)
}

// SetStopDelay sets the stop delay.
func (c *ChargingSession) SetStopDelay(seconds uint32) error {
	attr, err := c.GetAttribute(ChargingSessionAttrStopDelay)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(seconds)
}

// Getters

// State returns the current charging state.
func (c *ChargingSession) State() ChargingState {
	val, _ := c.ReadAttribute(ChargingSessionAttrState)
	if v, ok := val.(uint8); ok {
		return ChargingState(v)
	}
	return ChargingStateNotPluggedIn
}

// SessionID returns the session identifier.
func (c *ChargingSession) SessionID() uint32 {
	val, _ := c.ReadAttribute(ChargingSessionAttrSessionID)
	if v, ok := val.(uint32); ok {
		return v
	}
	return 0
}

// SessionEnergyCharged returns the energy charged this session.
func (c *ChargingSession) SessionEnergyCharged() uint64 {
	val, _ := c.ReadAttribute(ChargingSessionAttrSessionEnergyCharged)
	if v, ok := val.(uint64); ok {
		return v
	}
	return 0
}

// SessionEnergyDischarged returns the energy discharged this session.
func (c *ChargingSession) SessionEnergyDischarged() uint64 {
	val, _ := c.ReadAttribute(ChargingSessionAttrSessionEnergyDischarged)
	if v, ok := val.(uint64); ok {
		return v
	}
	return 0
}

// EVStateOfCharge returns the EV's current SoC.
func (c *ChargingSession) EVStateOfCharge() (uint8, bool) {
	val, err := c.ReadAttribute(ChargingSessionAttrEVStateOfCharge)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint8); ok {
		return v, true
	}
	return 0, false
}

// EVBatteryCapacity returns the EV's battery capacity.
func (c *ChargingSession) EVBatteryCapacity() (uint64, bool) {
	val, err := c.ReadAttribute(ChargingSessionAttrEVBatteryCapacity)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint64); ok {
		return v, true
	}
	return 0, false
}

// EVDemandMode returns the EV's demand information mode.
func (c *ChargingSession) EVDemandMode() EVDemandMode {
	val, _ := c.ReadAttribute(ChargingSessionAttrEVDemandMode)
	if v, ok := val.(uint8); ok {
		return EVDemandMode(v)
	}
	return EVDemandModeNone
}

// EVTargetEnergyRequest returns the energy needed to reach target SoC.
func (c *ChargingSession) EVTargetEnergyRequest() (int64, bool) {
	val, err := c.ReadAttribute(ChargingSessionAttrEVTargetEnergyRequest)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// CurrentChargingMode returns the active charging mode.
func (c *ChargingSession) CurrentChargingMode() ChargingMode {
	val, _ := c.ReadAttribute(ChargingSessionAttrChargingMode)
	if v, ok := val.(uint8); ok {
		return ChargingMode(v)
	}
	return ChargingModeOff
}

// Helper methods

// IsPluggedIn returns true if an EV is connected.
func (c *ChargingSession) IsPluggedIn() bool {
	state := c.State()
	return state != ChargingStateNotPluggedIn
}

// IsCharging returns true if actively charging.
func (c *ChargingSession) IsCharging() bool {
	return c.State() == ChargingStatePluggedInCharging
}

// IsDischarging returns true if V2G discharging.
func (c *ChargingSession) IsDischarging() bool {
	return c.State() == ChargingStatePluggedInDischarging
}

// CanDischarge returns true if V2G discharge is permitted.
func (c *ChargingSession) CanDischarge() bool {
	// Check evMinDischargingRequest < 0
	minVal, err := c.ReadAttribute(ChargingSessionAttrEVMinDischargingRequest)
	if err != nil || minVal == nil {
		return false
	}
	minDischarge, ok := minVal.(int64)
	if !ok || minDischarge >= 0 {
		return false
	}

	// Check evMaxDischargingRequest >= 0
	maxVal, err := c.ReadAttribute(ChargingSessionAttrEVMaxDischargingRequest)
	if err != nil || maxVal == nil {
		return false
	}
	maxDischarge, ok := maxVal.(int64)
	if !ok || maxDischarge < 0 {
		return false
	}

	// Check target energy or below target permission
	targetVal, err := c.ReadAttribute(ChargingSessionAttrEVTargetEnergyRequest)
	if err == nil && targetVal != nil {
		if target, ok := targetVal.(int64); ok && target <= 0 {
			return true // Already at or above target
		}
	}

	// Check if discharge below target is permitted
	permVal, err := c.ReadAttribute(ChargingSessionAttrEVDischargeBelowTargetPermitted)
	if err != nil || permVal == nil {
		return false
	}
	if perm, ok := permVal.(bool); ok {
		return perm
	}
	return false
}

// SupportsMode returns true if the given charging mode is supported.
func (c *ChargingSession) SupportsMode(mode ChargingMode) bool {
	val, err := c.ReadAttribute(ChargingSessionAttrSupportedChargingModes)
	if err != nil || val == nil {
		return mode == ChargingModeOff // OFF is always supported
	}
	if modes, ok := val.([]uint8); ok {
		for _, m := range modes {
			if ChargingMode(m) == mode {
				return true
			}
		}
	}
	return false
}

// StartSession begins a new charging session.
func (c *ChargingSession) StartSession(sessionID uint32, startTime uint64) error {
	if err := c.SetSessionID(sessionID); err != nil {
		return err
	}
	if err := c.SetSessionStartTime(startTime); err != nil {
		return err
	}
	if err := c.ClearSessionEndTime(); err != nil {
		return err
	}
	if err := c.SetSessionEnergyCharged(0); err != nil {
		return err
	}
	if err := c.SetSessionEnergyDischarged(0); err != nil {
		return err
	}
	return c.SetState(ChargingStatePluggedInNoDemand)
}

// EndSession ends the current charging session.
func (c *ChargingSession) EndSession(endTime uint64) error {
	if err := c.SetSessionEndTime(endTime); err != nil {
		return err
	}
	return c.SetState(ChargingStateNotPluggedIn)
}
