package features

import (
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Electrical attribute IDs.
const (
	// Phase configuration (1-9)
	ElectricalAttrPhaseCount   uint16 = 1
	ElectricalAttrPhaseMapping uint16 = 2

	// Voltage/frequency
	ElectricalAttrNominalVoltage   uint16 = 3
	ElectricalAttrNominalFrequency uint16 = 4

	// Direction capability
	ElectricalAttrSupportedDirections uint16 = 5

	// Nameplate power ratings (10-19)
	ElectricalAttrNominalMaxConsumption uint16 = 10
	ElectricalAttrNominalMaxProduction  uint16 = 11
	ElectricalAttrNominalMinPower       uint16 = 12

	// Nameplate current ratings
	ElectricalAttrMaxCurrentPerPhase uint16 = 13
	ElectricalAttrMinCurrentPerPhase uint16 = 14

	// Per-phase capabilities
	ElectricalAttrSupportsAsymmetric uint16 = 15

	// Storage (20-29)
	ElectricalAttrEnergyCapacity uint16 = 20
)

// ElectricalFeatureRevision is the current revision of the Electrical feature.
const ElectricalFeatureRevision uint16 = 1

// Electrical wraps a Feature with Electrical-specific functionality.
// It describes the electrical characteristics and capability envelope of an endpoint.
type Electrical struct {
	*model.Feature
}

// NewElectrical creates a new Electrical feature.
func NewElectrical() *Electrical {
	f := model.NewFeature(model.FeatureElectrical, ElectricalFeatureRevision)

	// Phase configuration
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrPhaseCount,
		Name:        "phaseCount",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(1),
		MinValue:    uint8(1),
		MaxValue:    uint8(3),
		Description: "Number of phases (1, 2, or 3)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrPhaseMapping,
		Name:        "phaseMapping",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Description: "Device phase to grid phase mapping",
	}))

	// Voltage/frequency
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrNominalVoltage,
		Name:        "nominalVoltage",
		Type:        model.DataTypeUint16,
		Access:      model.AccessReadOnly,
		Default:     uint16(230),
		Unit:        "V",
		Description: "Nominal voltage",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrNominalFrequency,
		Name:        "nominalFrequency",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(50),
		Unit:        "Hz",
		Description: "Nominal frequency",
	}))

	// Direction capability
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrSupportedDirections,
		Name:        "supportedDirections",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(DirectionConsumption),
		Description: "Power flow direction capability",
	}))

	// Power ratings
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrNominalMaxConsumption,
		Name:        "nominalMaxConsumption",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mW",
		Description: "Maximum consumption power",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrNominalMaxProduction,
		Name:        "nominalMaxProduction",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mW",
		Description: "Maximum production power (0 if N/A)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrNominalMinPower,
		Name:        "nominalMinPower",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mW",
		Description: "Minimum operating point",
	}))

	// Current ratings
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrMaxCurrentPerPhase,
		Name:        "maxCurrentPerPhase",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mA",
		Description: "Maximum current per phase",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrMinCurrentPerPhase,
		Name:        "minCurrentPerPhase",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mA",
		Description: "Minimum current per phase",
	}))

	// Asymmetric support
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrSupportsAsymmetric,
		Name:        "supportsAsymmetric",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(AsymmetricNone),
		Description: "Per-phase asymmetric control support",
	}))

	// Storage
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          ElectricalAttrEnergyCapacity,
		Name:        "energyCapacity",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Default:     int64(0),
		Unit:        "mWh",
		Description: "Battery/storage capacity (0 if N/A)",
	}))

	return &Electrical{Feature: f}
}

// Setters for device implementation

// SetPhaseCount sets the number of phases.
func (e *Electrical) SetPhaseCount(count uint8) error {
	attr, err := e.GetAttribute(ElectricalAttrPhaseCount)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(count)
}

// SetPhaseMapping sets the device phase to grid phase mapping.
// The map keys are Phase (A, B, C) and values are GridPhase (L1, L2, L3).
func (e *Electrical) SetPhaseMapping(mapping map[Phase]GridPhase) error {
	attr, err := e.GetAttribute(ElectricalAttrPhaseMapping)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(mapping)
}

// SetNominalVoltage sets the nominal voltage in V.
func (e *Electrical) SetNominalVoltage(voltage uint16) error {
	attr, err := e.GetAttribute(ElectricalAttrNominalVoltage)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(voltage)
}

// SetNominalFrequency sets the nominal frequency in Hz.
func (e *Electrical) SetNominalFrequency(freq uint8) error {
	attr, err := e.GetAttribute(ElectricalAttrNominalFrequency)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(freq)
}

// SetSupportedDirections sets the power flow direction capability.
func (e *Electrical) SetSupportedDirections(dir Direction) error {
	attr, err := e.GetAttribute(ElectricalAttrSupportedDirections)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(dir))
}

// SetNominalMaxConsumption sets the maximum consumption power in mW.
func (e *Electrical) SetNominalMaxConsumption(power int64) error {
	attr, err := e.GetAttribute(ElectricalAttrNominalMaxConsumption)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetNominalMaxProduction sets the maximum production power in mW.
func (e *Electrical) SetNominalMaxProduction(power int64) error {
	attr, err := e.GetAttribute(ElectricalAttrNominalMaxProduction)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetNominalMinPower sets the minimum operating point in mW.
func (e *Electrical) SetNominalMinPower(power int64) error {
	attr, err := e.GetAttribute(ElectricalAttrNominalMinPower)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetMaxCurrentPerPhase sets the maximum current per phase in mA.
func (e *Electrical) SetMaxCurrentPerPhase(current int64) error {
	attr, err := e.GetAttribute(ElectricalAttrMaxCurrentPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(current)
}

// SetMinCurrentPerPhase sets the minimum current per phase in mA.
func (e *Electrical) SetMinCurrentPerPhase(current int64) error {
	attr, err := e.GetAttribute(ElectricalAttrMinCurrentPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(current)
}

// SetSupportsAsymmetric sets the asymmetric control capability.
func (e *Electrical) SetSupportsAsymmetric(support AsymmetricSupport) error {
	attr, err := e.GetAttribute(ElectricalAttrSupportsAsymmetric)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(support))
}

// SetEnergyCapacity sets the storage capacity in mWh.
func (e *Electrical) SetEnergyCapacity(capacity int64) error {
	attr, err := e.GetAttribute(ElectricalAttrEnergyCapacity)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(capacity)
}

// Getters for reading values

// PhaseCount returns the number of phases.
func (e *Electrical) PhaseCount() uint8 {
	val, _ := e.ReadAttribute(ElectricalAttrPhaseCount)
	if v, ok := val.(uint8); ok {
		return v
	}
	return 1
}

// PhaseMapping returns the phase mapping.
func (e *Electrical) PhaseMapping() map[Phase]GridPhase {
	val, _ := e.ReadAttribute(ElectricalAttrPhaseMapping)
	if m, ok := val.(map[Phase]GridPhase); ok {
		return m
	}
	return nil
}

// NominalVoltage returns the nominal voltage in V.
func (e *Electrical) NominalVoltage() uint16 {
	val, _ := e.ReadAttribute(ElectricalAttrNominalVoltage)
	if v, ok := val.(uint16); ok {
		return v
	}
	return 230
}

// NominalFrequency returns the nominal frequency in Hz.
func (e *Electrical) NominalFrequency() uint8 {
	val, _ := e.ReadAttribute(ElectricalAttrNominalFrequency)
	if v, ok := val.(uint8); ok {
		return v
	}
	return 50
}

// SupportedDirections returns the power flow direction capability.
func (e *Electrical) SupportedDirections() Direction {
	val, _ := e.ReadAttribute(ElectricalAttrSupportedDirections)
	if v, ok := val.(uint8); ok {
		return Direction(v)
	}
	return DirectionConsumption
}

// NominalMaxConsumption returns the maximum consumption power in mW.
func (e *Electrical) NominalMaxConsumption() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrNominalMaxConsumption)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// NominalMaxProduction returns the maximum production power in mW.
func (e *Electrical) NominalMaxProduction() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrNominalMaxProduction)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// NominalMinPower returns the minimum operating point in mW.
func (e *Electrical) NominalMinPower() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrNominalMinPower)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// MaxCurrentPerPhase returns the maximum current per phase in mA.
func (e *Electrical) MaxCurrentPerPhase() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrMaxCurrentPerPhase)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// MinCurrentPerPhase returns the minimum current per phase in mA.
func (e *Electrical) MinCurrentPerPhase() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrMinCurrentPerPhase)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// SupportsAsymmetric returns the asymmetric control capability.
func (e *Electrical) SupportsAsymmetric() AsymmetricSupport {
	val, _ := e.ReadAttribute(ElectricalAttrSupportsAsymmetric)
	if v, ok := val.(uint8); ok {
		return AsymmetricSupport(v)
	}
	return AsymmetricNone
}

// EnergyCapacity returns the storage capacity in mWh.
func (e *Electrical) EnergyCapacity() int64 {
	val, _ := e.ReadAttribute(ElectricalAttrEnergyCapacity)
	if v, ok := val.(int64); ok {
		return v
	}
	return 0
}

// IsBidirectional returns true if the device supports both consumption and production.
func (e *Electrical) IsBidirectional() bool {
	return e.SupportedDirections() == DirectionBidirectional
}

// CanConsume returns true if the device can consume power.
func (e *Electrical) CanConsume() bool {
	dir := e.SupportedDirections()
	return dir == DirectionConsumption || dir == DirectionBidirectional
}

// CanProduce returns true if the device can produce power.
func (e *Electrical) CanProduce() bool {
	dir := e.SupportedDirections()
	return dir == DirectionProduction || dir == DirectionBidirectional
}
