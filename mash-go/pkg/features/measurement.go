package features

import (
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Measurement attribute IDs.
const (
	// AC Power (1-9)
	MeasurementAttrACActivePower   uint16 = 1
	MeasurementAttrACReactivePower uint16 = 2
	MeasurementAttrACApparentPower uint16 = 3

	// Per-phase AC power (10-19)
	MeasurementAttrACActivePowerPerPhase   uint16 = 10
	MeasurementAttrACReactivePowerPerPhase uint16 = 11
	MeasurementAttrACApparentPowerPerPhase uint16 = 12

	// AC Current & Voltage (20-29)
	MeasurementAttrACCurrentPerPhase       uint16 = 20
	MeasurementAttrACVoltagePerPhase       uint16 = 21
	MeasurementAttrACVoltagePhaseToPhasePair uint16 = 22
	MeasurementAttrACFrequency             uint16 = 23
	MeasurementAttrPowerFactor             uint16 = 24

	// AC Energy (30-39)
	MeasurementAttrACEnergyConsumed uint16 = 30
	MeasurementAttrACEnergyProduced uint16 = 31

	// DC Measurements (40-49)
	MeasurementAttrDCPower     uint16 = 40
	MeasurementAttrDCCurrent   uint16 = 41
	MeasurementAttrDCVoltage   uint16 = 42
	MeasurementAttrDCEnergyIn  uint16 = 43
	MeasurementAttrDCEnergyOut uint16 = 44

	// Battery State (50-59)
	MeasurementAttrStateOfCharge   uint16 = 50
	MeasurementAttrStateOfHealth   uint16 = 51
	MeasurementAttrStateOfEnergy   uint16 = 52
	MeasurementAttrUseableCapacity uint16 = 53
	MeasurementAttrCycleCount      uint16 = 54

	// Temperature (60-69)
	MeasurementAttrTemperature uint16 = 60
)

// MeasurementFeatureRevision is the current revision of the Measurement feature.
const MeasurementFeatureRevision uint16 = 1

// Measurement wraps a Feature with Measurement-specific functionality.
// It provides real-time telemetry: power, energy, voltage, current readings.
type Measurement struct {
	*model.Feature
}

// NewMeasurement creates a new Measurement feature.
func NewMeasurement() *Measurement {
	f := model.NewFeature(model.FeatureMeasurement, MeasurementFeatureRevision)

	// AC Power (signed: + consumption, - production)
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACActivePower,
		Name:        "acActivePower",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mW",
		Description: "Active/real power",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACReactivePower,
		Name:        "acReactivePower",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mVAR",
		Description: "Reactive power",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACApparentPower,
		Name:        "acApparentPower",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mVA",
		Description: "Apparent power (always positive)",
	}))

	// Per-phase AC power
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACActivePowerPerPhase,
		Name:        "acActivePowerPerPhase",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Active power per phase (Phase -> mW)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACReactivePowerPerPhase,
		Name:        "acReactivePowerPerPhase",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Reactive power per phase (Phase -> mVAR)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACApparentPowerPerPhase,
		Name:        "acApparentPowerPerPhase",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Apparent power per phase (Phase -> mVA)",
	}))

	// AC Current & Voltage
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACCurrentPerPhase,
		Name:        "acCurrentPerPhase",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Current per phase (Phase -> mA, signed)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACVoltagePerPhase,
		Name:        "acVoltagePerPhase",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Phase-to-neutral voltage (Phase -> mV)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACVoltagePhaseToPhasePair,
		Name:        "acVoltagePhaseToPhasePair",
		Type:        model.DataTypeMap,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Phase-to-phase voltage (PhasePair -> mV)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACFrequency,
		Name:        "acFrequency",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mHz",
		Description: "Grid frequency (e.g., 50000 = 50.000 Hz)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrPowerFactor,
		Name:        "powerFactor",
		Type:        model.DataTypeInt16,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		MinValue:    int16(-1000),
		MaxValue:    int16(1000),
		Description: "Power factor (0.001 units, -1.0 to +1.0)",
	}))

	// AC Energy
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACEnergyConsumed,
		Name:        "acEnergyConsumed",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Total energy consumed from grid",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrACEnergyProduced,
		Name:        "acEnergyProduced",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Total energy produced/fed-in to grid",
	}))

	// DC Measurements
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrDCPower,
		Name:        "dcPower",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mW",
		Description: "DC power (+ into device, - out)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrDCCurrent,
		Name:        "dcCurrent",
		Type:        model.DataTypeInt64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mA",
		Description: "DC current (+ into device, - out)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrDCVoltage,
		Name:        "dcVoltage",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mV",
		Description: "DC voltage",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrDCEnergyIn,
		Name:        "dcEnergyIn",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Energy into this component",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrDCEnergyOut,
		Name:        "dcEnergyOut",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Energy out of this component",
	}))

	// Battery State
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrStateOfCharge,
		Name:        "stateOfCharge",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		MinValue:    uint8(0),
		MaxValue:    uint8(100),
		Unit:        "%",
		Description: "State of charge (0-100%)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrStateOfHealth,
		Name:        "stateOfHealth",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		MinValue:    uint8(0),
		MaxValue:    uint8(100),
		Unit:        "%",
		Description: "State of health (0-100%, battery degradation)",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrStateOfEnergy,
		Name:        "stateOfEnergy",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Available energy at current SoC",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrUseableCapacity,
		Name:        "useableCapacity",
		Type:        model.DataTypeUint64,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "mWh",
		Description: "Current useable capacity",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrCycleCount,
		Name:        "cycleCount",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Charge/discharge cycles",
	}))

	// Temperature
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          MeasurementAttrTemperature,
		Name:        "temperature",
		Type:        model.DataTypeInt16,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Unit:        "centi-C",
		Description: "Temperature (e.g., 2500 = 25.00C)",
	}))

	return &Measurement{Feature: f}
}

// AC Power setters

// SetACActivePower sets the active power in mW.
func (m *Measurement) SetACActivePower(power int64) error {
	attr, err := m.GetAttribute(MeasurementAttrACActivePower)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetACReactivePower sets the reactive power in mVAR.
func (m *Measurement) SetACReactivePower(power int64) error {
	attr, err := m.GetAttribute(MeasurementAttrACReactivePower)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetACApparentPower sets the apparent power in mVA.
func (m *Measurement) SetACApparentPower(power uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrACApparentPower)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetACActivePowerPerPhase sets the per-phase active power.
func (m *Measurement) SetACActivePowerPerPhase(powers map[Phase]int64) error {
	attr, err := m.GetAttribute(MeasurementAttrACActivePowerPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(powers)
}

// SetACReactivePowerPerPhase sets the per-phase reactive power.
func (m *Measurement) SetACReactivePowerPerPhase(powers map[Phase]int64) error {
	attr, err := m.GetAttribute(MeasurementAttrACReactivePowerPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(powers)
}

// SetACApparentPowerPerPhase sets the per-phase apparent power.
func (m *Measurement) SetACApparentPowerPerPhase(powers map[Phase]uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrACApparentPowerPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(powers)
}

// AC Current & Voltage setters

// SetACCurrentPerPhase sets the per-phase current in mA.
func (m *Measurement) SetACCurrentPerPhase(currents map[Phase]int64) error {
	attr, err := m.GetAttribute(MeasurementAttrACCurrentPerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(currents)
}

// SetACVoltagePerPhase sets the per-phase voltage in mV.
func (m *Measurement) SetACVoltagePerPhase(voltages map[Phase]uint32) error {
	attr, err := m.GetAttribute(MeasurementAttrACVoltagePerPhase)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(voltages)
}

// SetACVoltagePhaseToPhasePair sets the phase-to-phase voltages in mV.
func (m *Measurement) SetACVoltagePhaseToPhasePair(voltages map[PhasePair]uint32) error {
	attr, err := m.GetAttribute(MeasurementAttrACVoltagePhaseToPhasePair)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(voltages)
}

// SetACFrequency sets the grid frequency in mHz.
func (m *Measurement) SetACFrequency(freq uint32) error {
	attr, err := m.GetAttribute(MeasurementAttrACFrequency)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(freq)
}

// SetPowerFactor sets the power factor (0.001 units, -1000 to +1000).
func (m *Measurement) SetPowerFactor(pf int16) error {
	attr, err := m.GetAttribute(MeasurementAttrPowerFactor)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(pf)
}

// AC Energy setters

// SetACEnergyConsumed sets the total consumed energy in mWh.
func (m *Measurement) SetACEnergyConsumed(energy uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrACEnergyConsumed)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// SetACEnergyProduced sets the total produced energy in mWh.
func (m *Measurement) SetACEnergyProduced(energy uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrACEnergyProduced)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// DC Measurement setters

// SetDCPower sets the DC power in mW.
func (m *Measurement) SetDCPower(power int64) error {
	attr, err := m.GetAttribute(MeasurementAttrDCPower)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(power)
}

// SetDCCurrent sets the DC current in mA.
func (m *Measurement) SetDCCurrent(current int64) error {
	attr, err := m.GetAttribute(MeasurementAttrDCCurrent)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(current)
}

// SetDCVoltage sets the DC voltage in mV.
func (m *Measurement) SetDCVoltage(voltage uint32) error {
	attr, err := m.GetAttribute(MeasurementAttrDCVoltage)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(voltage)
}

// SetDCEnergyIn sets the energy into this component in mWh.
func (m *Measurement) SetDCEnergyIn(energy uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrDCEnergyIn)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// SetDCEnergyOut sets the energy out of this component in mWh.
func (m *Measurement) SetDCEnergyOut(energy uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrDCEnergyOut)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(energy)
}

// Battery State setters

// SetStateOfCharge sets the battery SoC (0-100%).
func (m *Measurement) SetStateOfCharge(soc uint8) error {
	attr, err := m.GetAttribute(MeasurementAttrStateOfCharge)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(soc)
}

// SetStateOfHealth sets the battery SoH (0-100%).
func (m *Measurement) SetStateOfHealth(soh uint8) error {
	attr, err := m.GetAttribute(MeasurementAttrStateOfHealth)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(soh)
}

// SetStateOfEnergy sets the available energy in mWh.
func (m *Measurement) SetStateOfEnergy(soe uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrStateOfEnergy)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(soe)
}

// SetUseableCapacity sets the current useable capacity in mWh.
func (m *Measurement) SetUseableCapacity(capacity uint64) error {
	attr, err := m.GetAttribute(MeasurementAttrUseableCapacity)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(capacity)
}

// SetCycleCount sets the charge/discharge cycle count.
func (m *Measurement) SetCycleCount(count uint32) error {
	attr, err := m.GetAttribute(MeasurementAttrCycleCount)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(count)
}

// SetTemperature sets the temperature in centi-degrees Celsius.
func (m *Measurement) SetTemperature(temp int16) error {
	attr, err := m.GetAttribute(MeasurementAttrTemperature)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(temp)
}

// Getters for reading values

// ACActivePower returns the active power in mW.
func (m *Measurement) ACActivePower() (int64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACActivePower)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// ACReactivePower returns the reactive power in mVAR.
func (m *Measurement) ACReactivePower() (int64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACReactivePower)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// ACApparentPower returns the apparent power in mVA.
func (m *Measurement) ACApparentPower() (uint64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACApparentPower)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint64); ok {
		return v, true
	}
	return 0, false
}

// ACCurrentPerPhase returns the per-phase current in mA.
func (m *Measurement) ACCurrentPerPhase() (map[Phase]int64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACCurrentPerPhase)
	if err != nil || val == nil {
		return nil, false
	}
	if v, ok := val.(map[Phase]int64); ok {
		return v, true
	}
	return nil, false
}

// ACVoltagePerPhase returns the per-phase voltage in mV.
func (m *Measurement) ACVoltagePerPhase() (map[Phase]uint32, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACVoltagePerPhase)
	if err != nil || val == nil {
		return nil, false
	}
	if v, ok := val.(map[Phase]uint32); ok {
		return v, true
	}
	return nil, false
}

// ACFrequency returns the grid frequency in mHz.
func (m *Measurement) ACFrequency() (uint32, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACFrequency)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint32); ok {
		return v, true
	}
	return 0, false
}

// PowerFactor returns the power factor (0.001 units).
func (m *Measurement) PowerFactor() (int16, bool) {
	val, err := m.ReadAttribute(MeasurementAttrPowerFactor)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int16); ok {
		return v, true
	}
	return 0, false
}

// ACEnergyConsumed returns the total consumed energy in mWh.
func (m *Measurement) ACEnergyConsumed() (uint64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACEnergyConsumed)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint64); ok {
		return v, true
	}
	return 0, false
}

// ACEnergyProduced returns the total produced energy in mWh.
func (m *Measurement) ACEnergyProduced() (uint64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrACEnergyProduced)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint64); ok {
		return v, true
	}
	return 0, false
}

// DCPower returns the DC power in mW.
func (m *Measurement) DCPower() (int64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrDCPower)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// DCCurrent returns the DC current in mA.
func (m *Measurement) DCCurrent() (int64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrDCCurrent)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int64); ok {
		return v, true
	}
	return 0, false
}

// DCVoltage returns the DC voltage in mV.
func (m *Measurement) DCVoltage() (uint32, bool) {
	val, err := m.ReadAttribute(MeasurementAttrDCVoltage)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint32); ok {
		return v, true
	}
	return 0, false
}

// StateOfCharge returns the battery SoC (0-100%).
func (m *Measurement) StateOfCharge() (uint8, bool) {
	val, err := m.ReadAttribute(MeasurementAttrStateOfCharge)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint8); ok {
		return v, true
	}
	return 0, false
}

// StateOfHealth returns the battery SoH (0-100%).
func (m *Measurement) StateOfHealth() (uint8, bool) {
	val, err := m.ReadAttribute(MeasurementAttrStateOfHealth)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint8); ok {
		return v, true
	}
	return 0, false
}

// StateOfEnergy returns the available energy in mWh.
func (m *Measurement) StateOfEnergy() (uint64, bool) {
	val, err := m.ReadAttribute(MeasurementAttrStateOfEnergy)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint64); ok {
		return v, true
	}
	return 0, false
}

// Temperature returns the temperature in centi-degrees Celsius.
func (m *Measurement) Temperature() (int16, bool) {
	val, err := m.ReadAttribute(MeasurementAttrTemperature)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(int16); ok {
		return v, true
	}
	return 0, false
}

// Helper methods

// IsConsuming returns true if currently consuming power (positive active power).
func (m *Measurement) IsConsuming() bool {
	power, ok := m.ACActivePower()
	return ok && power > 0
}

// IsProducing returns true if currently producing power (negative active power).
func (m *Measurement) IsProducing() bool {
	power, ok := m.ACActivePower()
	return ok && power < 0
}

// ActivePowerKW returns the active power in kW (for convenience).
func (m *Measurement) ActivePowerKW() (float64, bool) {
	power, ok := m.ACActivePower()
	if !ok {
		return 0, false
	}
	return float64(power) / 1_000_000.0, true
}
