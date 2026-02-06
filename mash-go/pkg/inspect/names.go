package inspect

import (
	"strings"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Name tables for resolving human-readable names to IDs.
var (
	// endpointNames maps endpoint type names to their IDs.
	endpointNames = map[string]uint8{
		"deviceroot":     uint8(model.EndpointDeviceRoot),
		"gridconnection": uint8(model.EndpointGridConnection),
		"inverter":       uint8(model.EndpointInverter),
		"pvstring":       uint8(model.EndpointPVString),
		"battery":        uint8(model.EndpointBattery),
		"evcharger":      uint8(model.EndpointEVCharger),
		"heatpump":       uint8(model.EndpointHeatPump),
		"waterheater":    uint8(model.EndpointWaterHeater),
		"hvac":           uint8(model.EndpointHVAC),
		"appliance":      uint8(model.EndpointAppliance),
		"submeter":       uint8(model.EndpointSubMeter),
	}

	// featureNames maps feature type names to their IDs.
	featureNames = map[string]uint8{
		"electrical":      uint8(model.FeatureElectrical),
		"measurement":     uint8(model.FeatureMeasurement),
		"energycontrol":   uint8(model.FeatureEnergyControl),
		"status":          uint8(model.FeatureStatus),
		"deviceinfo":      uint8(model.FeatureDeviceInfo),
		"chargingsession": uint8(model.FeatureChargingSession),
		"signals":         uint8(model.FeatureSignals),
		"tariff":          uint8(model.FeatureTariff),
		"plan":            uint8(model.FeaturePlan),
		"testcontrol":     uint8(model.FeatureTestControl),
	}

	// attributeNames maps attribute names to IDs, keyed by feature ID.
	attributeNames = map[uint8]map[string]uint16{}

	// commandNames maps command names to IDs, keyed by feature ID.
	commandNames = map[uint8]map[string]uint8{}
)

// initNameTables initializes the attribute name tables.
// Called automatically during package init.
func initNameTables() {
	// Global attributes (present on all features)
	globalAttrs := map[string]uint16{
		"featureMap":    model.AttrIDFeatureMap,
		"attributeList": model.AttrIDAttributeList,
		"commandList":   model.AttrIDCommandList,
	}

	// Measurement attributes
	measurementAttrs := map[string]uint16{
		"acActivePower":              features.MeasurementAttrACActivePower,
		"acReactivePower":            features.MeasurementAttrACReactivePower,
		"acApparentPower":            features.MeasurementAttrACApparentPower,
		"acActivePowerPerPhase":      features.MeasurementAttrACActivePowerPerPhase,
		"acReactivePowerPerPhase":    features.MeasurementAttrACReactivePowerPerPhase,
		"acApparentPowerPerPhase":    features.MeasurementAttrACApparentPowerPerPhase,
		"acCurrentPerPhase":          features.MeasurementAttrACCurrentPerPhase,
		"acVoltagePerPhase":          features.MeasurementAttrACVoltagePerPhase,
		"acVoltagePhaseToPhasePair":  features.MeasurementAttrACVoltagePhaseToPhasePair,
		"acFrequency":               features.MeasurementAttrACFrequency,
		"powerFactor":               features.MeasurementAttrPowerFactor,
		"acEnergyConsumed":          features.MeasurementAttrACEnergyConsumed,
		"acEnergyProduced":          features.MeasurementAttrACEnergyProduced,
		"dcPower":                   features.MeasurementAttrDCPower,
		"dcCurrent":                 features.MeasurementAttrDCCurrent,
		"dcVoltage":                 features.MeasurementAttrDCVoltage,
		"dcEnergyIn":                features.MeasurementAttrDCEnergyIn,
		"dcEnergyOut":               features.MeasurementAttrDCEnergyOut,
		"stateOfCharge":             features.MeasurementAttrStateOfCharge,
		"stateOfHealth":             features.MeasurementAttrStateOfHealth,
		"stateOfEnergy":             features.MeasurementAttrStateOfEnergy,
		"useableCapacity":           features.MeasurementAttrUseableCapacity,
		"cycleCount":                features.MeasurementAttrCycleCount,
		"temperature":               features.MeasurementAttrTemperature,
	}
	// Add global attributes to measurement
	for k, v := range globalAttrs {
		measurementAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureMeasurement)] = measurementAttrs

	// EnergyControl attributes
	energyControlAttrs := map[string]uint16{
		"deviceType":                            features.EnergyControlAttrDeviceType,
		"controlState":                          features.EnergyControlAttrControlState,
		"optOutState":                           features.EnergyControlAttrOptOutState,
		"acceptsLimits":                         features.EnergyControlAttrAcceptsLimits,
		"acceptsCurrentLimits":                  features.EnergyControlAttrAcceptsCurrentLimits,
		"acceptsSetpoints":                      features.EnergyControlAttrAcceptsSetpoints,
		"acceptsCurrentSetpoints":               features.EnergyControlAttrAcceptsCurrentSetpoints,
		"isPausable":                            features.EnergyControlAttrIsPausable,
		"isShiftable":                           features.EnergyControlAttrIsShiftable,
		"isStoppable":                           features.EnergyControlAttrIsStoppable,
		"effectiveConsumptionLimit":              features.EnergyControlAttrEffectiveConsumptionLimit,
		"myConsumptionLimit":                    features.EnergyControlAttrMyConsumptionLimit,
		"effectiveProductionLimit":              features.EnergyControlAttrEffectiveProductionLimit,
		"myProductionLimit":                     features.EnergyControlAttrMyProductionLimit,
		"effectiveCurrentLimitsConsumption":     features.EnergyControlAttrEffectiveCurrentLimitsConsumption,
		"myCurrentLimitsConsumption":            features.EnergyControlAttrMyCurrentLimitsConsumption,
		"effectiveCurrentLimitsProduction":      features.EnergyControlAttrEffectiveCurrentLimitsProduction,
		"myCurrentLimitsProduction":             features.EnergyControlAttrMyCurrentLimitsProduction,
		"effectiveConsumptionSetpoint":          features.EnergyControlAttrEffectiveConsumptionSetpoint,
		"myConsumptionSetpoint":                 features.EnergyControlAttrMyConsumptionSetpoint,
		"effectiveProductionSetpoint":           features.EnergyControlAttrEffectiveProductionSetpoint,
		"myProductionSetpoint":                  features.EnergyControlAttrMyProductionSetpoint,
		"effectiveCurrentSetpointsConsumption":  features.EnergyControlAttrEffectiveCurrentSetpointsConsumption,
		"myCurrentSetpointsConsumption":         features.EnergyControlAttrMyCurrentSetpointsConsumption,
		"effectiveCurrentSetpointsProduction":   features.EnergyControlAttrEffectiveCurrentSetpointsProduction,
		"myCurrentSetpointsProduction":          features.EnergyControlAttrMyCurrentSetpointsProduction,
		"failsafeConsumptionLimit":              features.EnergyControlAttrFailsafeConsumptionLimit,
		"failsafeProductionLimit":               features.EnergyControlAttrFailsafeProductionLimit,
		"failsafeDuration":                      features.EnergyControlAttrFailsafeDuration,
		"processState":                          features.EnergyControlAttrProcessState,
		"optionalProcess":                       features.EnergyControlAttrOptionalProcess,
		"controlMode":                           features.EnergyControlAttrControlMode,
		"overrideReason":                        features.EnergyControlAttrOverrideReason,
		"overrideDirection":                     features.EnergyControlAttrOverrideDirection,
		"minRunDuration":                        features.EnergyControlAttrMinRunDuration,
		"minPauseDuration":                      features.EnergyControlAttrMinPauseDuration,
	}
	for k, v := range globalAttrs {
		energyControlAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureEnergyControl)] = energyControlAttrs

	// Electrical attributes
	electricalAttrs := map[string]uint16{
		"phaseCount":            features.ElectricalAttrPhaseCount,
		"phaseMapping":          features.ElectricalAttrPhaseMapping,
		"supportedDirections":   features.ElectricalAttrSupportedDirections,
		"nominalVoltage":        features.ElectricalAttrNominalVoltage,
		"nominalFrequency":      features.ElectricalAttrNominalFrequency,
		"nominalMaxConsumption": features.ElectricalAttrNominalMaxConsumption,
		"nominalMaxProduction":  features.ElectricalAttrNominalMaxProduction,
		"nominalMinPower":       features.ElectricalAttrNominalMinPower,
		"maxCurrentPerPhase":    features.ElectricalAttrMaxCurrentPerPhase,
		"minCurrentPerPhase":    features.ElectricalAttrMinCurrentPerPhase,
		"supportsAsymmetric":    features.ElectricalAttrSupportsAsymmetric,
		"energyCapacity":        features.ElectricalAttrEnergyCapacity,
	}
	for k, v := range globalAttrs {
		electricalAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureElectrical)] = electricalAttrs

	// Status attributes
	statusAttrs := map[string]uint16{
		"operatingState": features.StatusAttrOperatingState,
		"stateDetail":    features.StatusAttrStateDetail,
		"faultCode":      features.StatusAttrFaultCode,
		"faultMessage":   features.StatusAttrFaultMessage,
	}
	for k, v := range globalAttrs {
		statusAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureStatus)] = statusAttrs

	// DeviceInfo attributes
	deviceInfoAttrs := map[string]uint16{
		"deviceId":        features.DeviceInfoAttrDeviceID,
		"vendorName":      features.DeviceInfoAttrVendorName,
		"productName":     features.DeviceInfoAttrProductName,
		"serialNumber":    features.DeviceInfoAttrSerialNumber,
		"vendorId":        features.DeviceInfoAttrVendorID,
		"productId":       features.DeviceInfoAttrProductID,
		"softwareVersion": features.DeviceInfoAttrSoftwareVersion,
		"hardwareVersion": features.DeviceInfoAttrHardwareVersion,
		"specVersion":     features.DeviceInfoAttrSpecVersion,
		"endpoints":       features.DeviceInfoAttrEndpoints,
		"endpointList":    features.DeviceInfoAttrEndpoints,
		"useCases":        features.DeviceInfoAttrUseCases,
		"location":        features.DeviceInfoAttrLocation,
		"label":           features.DeviceInfoAttrLabel,
	}
	for k, v := range globalAttrs {
		deviceInfoAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureDeviceInfo)] = deviceInfoAttrs

	// TestControl attributes
	testControlAttrs := map[string]uint16{
		"testEventTriggersEnabled": features.TestControlAttrTestEventTriggersEnabled,
	}
	for k, v := range globalAttrs {
		testControlAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureTestControl)] = testControlAttrs

	// ChargingSession attributes
	chargingSessionAttrs := map[string]uint16{
		"state":                          features.ChargingSessionAttrState,
		"sessionStartTime":               features.ChargingSessionAttrSessionStartTime,
		"sessionEndTime":                 features.ChargingSessionAttrSessionEndTime,
		"sessionEnergyCharged":           features.ChargingSessionAttrSessionEnergyCharged,
		"sessionEnergyDischarged":        features.ChargingSessionAttrSessionEnergyDischarged,
		"evIdentifications":              features.ChargingSessionAttrEVIdentifications,
		"evStateOfCharge":                features.ChargingSessionAttrEVStateOfCharge,
		"evBatteryCapacity":              features.ChargingSessionAttrEVBatteryCapacity,
		"evMinStateOfCharge":             features.ChargingSessionAttrEVMinStateOfCharge,
		"evTargetStateOfCharge":          features.ChargingSessionAttrEVTargetStateOfCharge,
		"evDemandMode":                   features.ChargingSessionAttrEVDemandMode,
		"evMinEnergyRequest":             features.ChargingSessionAttrEVMinEnergyRequest,
		"evMaxEnergyRequest":             features.ChargingSessionAttrEVMaxEnergyRequest,
		"evTargetEnergyRequest":          features.ChargingSessionAttrEVTargetEnergyRequest,
		"evDepartureTime":                features.ChargingSessionAttrEVDepartureTime,
		"evMinDischargingRequest":        features.ChargingSessionAttrEVMinDischargingRequest,
		"evMaxDischargingRequest":        features.ChargingSessionAttrEVMaxDischargingRequest,
		"evDischargeBelowTargetPermitted": features.ChargingSessionAttrEVDischargeBelowTargetPermitted,
		"estimatedTimeToMinSoC":          features.ChargingSessionAttrEstimatedTimeToMinSoC,
		"estimatedTimeToTargetSoC":       features.ChargingSessionAttrEstimatedTimeToTargetSoC,
		"estimatedTimeToFullSoC":         features.ChargingSessionAttrEstimatedTimeToFullSoC,
		"chargingMode":                   features.ChargingSessionAttrChargingMode,
		"supportedChargingModes":         features.ChargingSessionAttrSupportedChargingModes,
		"surplusThreshold":               features.ChargingSessionAttrSurplusThreshold,
		"startDelay":                     features.ChargingSessionAttrStartDelay,
		"stopDelay":                      features.ChargingSessionAttrStopDelay,
	}
	for k, v := range globalAttrs {
		chargingSessionAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureChargingSession)] = chargingSessionAttrs

	// Signals attributes
	signalsAttrs := map[string]uint16{
		"signalSource":    features.SignalsAttrSignalSource,
		"startTime":       features.SignalsAttrStartTime,
		"validUntil":      features.SignalsAttrValidUntil,
		"priceSlots":      features.SignalsAttrPriceSlots,
		"constraintSlots": features.SignalsAttrConstraintSlots,
		"forecastSlots":   features.SignalsAttrForecastSlots,
		"schedule":        features.SignalsAttrPriceSlots, // alias used in test specs
	}
	for k, v := range globalAttrs {
		signalsAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureSignals)] = signalsAttrs

	// Plan attributes
	planAttrs := map[string]uint16{
		"planId":             features.PlanAttrPlanID,
		"planVersion":        features.PlanAttrPlanVersion,
		"commitment":         features.PlanAttrCommitment,
		"startTime":          features.PlanAttrStartTime,
		"endTime":            features.PlanAttrEndTime,
		"totalEnergyPlanned": features.PlanAttrTotalEnergyPlanned,
		"slots":              features.PlanAttrSlots,
	}
	for k, v := range globalAttrs {
		planAttrs[k] = v
	}
	attributeNames[uint8(model.FeaturePlan)] = planAttrs

	// Tariff attributes
	tariffAttrs := map[string]uint16{
		"tariffId":          features.TariffAttrTariffID,
		"currency":          features.TariffAttrCurrency,
		"priceUnit":         features.TariffAttrPriceUnit,
		"tariffDescription": features.TariffAttrTariffDescription,
	}
	for k, v := range globalAttrs {
		tariffAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureTariff)] = tariffAttrs

	// Command name tables
	commandNames[uint8(model.FeatureTariff)] = map[string]uint8{
		"setTariff": features.TariffCmdSetTariff,
	}
	commandNames[uint8(model.FeatureSignals)] = map[string]uint8{
		"sendPriceSignal":      features.SignalsCmdSendPriceSignal,
		"sendConstraintSignal": features.SignalsCmdSendConstraintSignal,
		"sendForecastSignal":   features.SignalsCmdSendForecastSignal,
		"clearSignals":         features.SignalsCmdClearSignals,
	}
	commandNames[uint8(model.FeatureEnergyControl)] = map[string]uint8{
		"setLimit":              features.EnergyControlCmdSetLimit,
		"clearLimit":            features.EnergyControlCmdClearLimit,
		"setCurrentLimits":      features.EnergyControlCmdSetCurrentLimits,
		"clearCurrentLimits":    features.EnergyControlCmdClearCurrentLimits,
		"setSetpoint":           features.EnergyControlCmdSetSetpoint,
		"clearSetpoint":         features.EnergyControlCmdClearSetpoint,
		"setCurrentSetpoints":   features.EnergyControlCmdSetCurrentSetpoints,
		"clearCurrentSetpoints": features.EnergyControlCmdClearCurrentSetpoints,
		"pause":                 features.EnergyControlCmdPause,
		"resume":                features.EnergyControlCmdResume,
		"stop":                  features.EnergyControlCmdStop,
	}
	commandNames[uint8(model.FeaturePlan)] = map[string]uint8{
		"requestPlan": features.PlanCmdRequestPlan,
		"acceptPlan":  features.PlanCmdAcceptPlan,
	}
	commandNames[uint8(model.FeatureChargingSession)] = map[string]uint8{
		"setChargingMode": features.ChargingSessionCmdSetChargingMode,
	}
	commandNames[uint8(model.FeatureDeviceInfo)] = map[string]uint8{
		"removeZone": features.DeviceInfoCmdRemoveZone,
	}
	commandNames[uint8(model.FeatureTestControl)] = map[string]uint8{
		"triggerTestEvent": features.TestControlCmdTriggerTestEvent,
	}
}

func init() {
	initNameTables()
}

// ResolveEndpointName resolves an endpoint name to its ID (case-insensitive).
func ResolveEndpointName(name string) (uint8, bool) {
	lname := strings.ToLower(name)
	for k, v := range endpointNames {
		if strings.ToLower(k) == lname {
			return v, true
		}
	}
	return 0, false
}

// ResolveFeatureName resolves a feature name to its ID (case-insensitive).
func ResolveFeatureName(name string) (uint8, bool) {
	lname := strings.ToLower(name)
	for k, v := range featureNames {
		if strings.ToLower(k) == lname {
			return v, true
		}
	}
	return 0, false
}

// ResolveAttributeName resolves an attribute name to its ID for a given feature (case-insensitive).
func ResolveAttributeName(featureID uint8, name string) (uint16, bool) {
	if attrNames, ok := attributeNames[featureID]; ok {
		lname := strings.ToLower(name)
		for k, v := range attrNames {
			if strings.ToLower(k) == lname {
				return v, true
			}
		}
	}
	return 0, false
}

// GetEndpointName returns the name for an endpoint type.
func GetEndpointName(id uint8) string {
	epType := model.EndpointType(id)
	return epType.String()
}

// GetFeatureName returns the name for a feature type.
func GetFeatureName(id uint8) string {
	featType := model.FeatureType(id)
	return featType.String()
}

// GetAttributeName returns the name for an attribute ID within a feature.
func GetAttributeName(featureID uint8, attrID uint16) string {
	if attrNames, ok := attributeNames[featureID]; ok {
		for name, id := range attrNames {
			if id == attrID {
				return name
			}
		}
	}
	// Return numeric if not found
	return ""
}

// ResolveCommandName resolves a command name to its ID for a given feature (case-insensitive).
func ResolveCommandName(featureID uint8, name string) (uint8, bool) {
	if cmdNames, ok := commandNames[featureID]; ok {
		lname := strings.ToLower(name)
		for k, v := range cmdNames {
			if strings.ToLower(k) == lname {
				return v, true
			}
		}
	}
	return 0, false
}

// GetCommandName returns the name for a command ID within a feature.
func GetCommandName(featureID uint8, cmdID uint8) string {
	if cmdNames, ok := commandNames[featureID]; ok {
		for name, id := range cmdNames {
			if id == cmdID {
				return name
			}
		}
	}
	return ""
}
