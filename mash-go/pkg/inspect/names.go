package inspect

import (
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
	}

	// attributeNames maps attribute names to IDs, keyed by feature ID.
	attributeNames = map[uint8]map[string]uint16{}
)

// initNameTables initializes the attribute name tables.
// Called automatically during package init.
func initNameTables() {
	// Global attributes (present on all features)
	globalAttrs := map[string]uint16{
		"featuremap":    model.AttrIDFeatureMap,
		"attributelist": model.AttrIDAttributeList,
		"commandlist":   model.AttrIDCommandList,
	}

	// Measurement attributes
	measurementAttrs := map[string]uint16{
		"acactivepower":            features.MeasurementAttrACActivePower,
		"acreactivepower":          features.MeasurementAttrACReactivePower,
		"acapparentpower":          features.MeasurementAttrACApparentPower,
		"acactivepowerperphase":    features.MeasurementAttrACActivePowerPerPhase,
		"acreactivepowerperphase":  features.MeasurementAttrACReactivePowerPerPhase,
		"acapparentpowerperphase":  features.MeasurementAttrACApparentPowerPerPhase,
		"accurrentperphase":        features.MeasurementAttrACCurrentPerPhase,
		"acvoltageperphase":        features.MeasurementAttrACVoltagePerPhase,
		"acvoltagephasetophasepair": features.MeasurementAttrACVoltagePhaseToPhasePair,
		"acfrequency":              features.MeasurementAttrACFrequency,
		"powerfactor":              features.MeasurementAttrPowerFactor,
		"acenergyconsumed":         features.MeasurementAttrACEnergyConsumed,
		"acenergyproduced":         features.MeasurementAttrACEnergyProduced,
		"dcpower":                  features.MeasurementAttrDCPower,
		"dccurrent":                features.MeasurementAttrDCCurrent,
		"dcvoltage":                features.MeasurementAttrDCVoltage,
		"dcenergyin":               features.MeasurementAttrDCEnergyIn,
		"dcenergyout":              features.MeasurementAttrDCEnergyOut,
		"stateofcharge":            features.MeasurementAttrStateOfCharge,
		"stateofhealth":            features.MeasurementAttrStateOfHealth,
		"stateofenergy":            features.MeasurementAttrStateOfEnergy,
		"useablecapacity":          features.MeasurementAttrUseableCapacity,
		"cyclecount":               features.MeasurementAttrCycleCount,
		"temperature":              features.MeasurementAttrTemperature,
	}
	// Add global attributes to measurement
	for k, v := range globalAttrs {
		measurementAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureMeasurement)] = measurementAttrs

	// EnergyControl attributes
	energyControlAttrs := map[string]uint16{
		"devicetype":                         features.EnergyControlAttrDeviceType,
		"controlstate":                       features.EnergyControlAttrControlState,
		"optoutstate":                        features.EnergyControlAttrOptOutState,
		"acceptslimits":                      features.EnergyControlAttrAcceptsLimits,
		"acceptscurrentlimits":               features.EnergyControlAttrAcceptsCurrentLimits,
		"acceptssetpoints":                   features.EnergyControlAttrAcceptsSetpoints,
		"acceptscurrentsetpoints":            features.EnergyControlAttrAcceptsCurrentSetpoints,
		"ispausable":                         features.EnergyControlAttrIsPausable,
		"isshiftable":                        features.EnergyControlAttrIsShiftable,
		"isstoppable":                        features.EnergyControlAttrIsStoppable,
		"effectiveconsumptionlimit":          features.EnergyControlAttrEffectiveConsumptionLimit,
		"myconsumptionlimit":                 features.EnergyControlAttrMyConsumptionLimit,
		"effectiveproductionlimit":           features.EnergyControlAttrEffectiveProductionLimit,
		"myproductionlimit":                  features.EnergyControlAttrMyProductionLimit,
		"effectivecurrentlimitsconsumption":  features.EnergyControlAttrEffectiveCurrentLimitsConsumption,
		"mycurrentlimitsconsumption":         features.EnergyControlAttrMyCurrentLimitsConsumption,
		"effectivecurrentlimitsproduction":   features.EnergyControlAttrEffectiveCurrentLimitsProduction,
		"mycurrentlimitsproduction":          features.EnergyControlAttrMyCurrentLimitsProduction,
		"effectiveconsumptionsetpoint":       features.EnergyControlAttrEffectiveConsumptionSetpoint,
		"myconsumptionsetpoint":              features.EnergyControlAttrMyConsumptionSetpoint,
		"effectiveproductionsetpoint":        features.EnergyControlAttrEffectiveProductionSetpoint,
		"myproductionsetpoint":               features.EnergyControlAttrMyProductionSetpoint,
		"effectivecurrentsetpointsconsumption": features.EnergyControlAttrEffectiveCurrentSetpointsConsumption,
		"mycurrentsetpointsconsumption":      features.EnergyControlAttrMyCurrentSetpointsConsumption,
		"effectivecurrentsetpointsproduction": features.EnergyControlAttrEffectiveCurrentSetpointsProduction,
		"mycurrentsetpointsproduction":       features.EnergyControlAttrMyCurrentSetpointsProduction,
		"failsafeconsumptionlimit":           features.EnergyControlAttrFailsafeConsumptionLimit,
		"failsafeproductionlimit":            features.EnergyControlAttrFailsafeProductionLimit,
		"failsafeduration":                   features.EnergyControlAttrFailsafeDuration,
		"processstate":                       features.EnergyControlAttrProcessState,
		"optionalprocess":                    features.EnergyControlAttrOptionalProcess,
	}
	for k, v := range globalAttrs {
		energyControlAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureEnergyControl)] = energyControlAttrs

	// Electrical attributes
	electricalAttrs := map[string]uint16{
		"phasecount":              features.ElectricalAttrPhaseCount,
		"phasemapping":            features.ElectricalAttrPhaseMapping,
		"supporteddirections":     features.ElectricalAttrSupportedDirections,
		"nominalvoltage":          features.ElectricalAttrNominalVoltage,
		"nominalfrequency":        features.ElectricalAttrNominalFrequency,
		"nominalmaxconsumption":   features.ElectricalAttrNominalMaxConsumption,
		"nominalmaxproduction":    features.ElectricalAttrNominalMaxProduction,
		"nominalminpower":         features.ElectricalAttrNominalMinPower,
		"maxcurrentperphase":      features.ElectricalAttrMaxCurrentPerPhase,
		"mincurrentperphase":      features.ElectricalAttrMinCurrentPerPhase,
		"supportedasymmetric":     features.ElectricalAttrSupportsAsymmetric,
		"energycapacity":          features.ElectricalAttrEnergyCapacity,
	}
	for k, v := range globalAttrs {
		electricalAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureElectrical)] = electricalAttrs

	// Status attributes
	statusAttrs := map[string]uint16{
		"operatingstate": features.StatusAttrOperatingState,
		"statedetail":    features.StatusAttrStateDetail,
		"faultcode":      features.StatusAttrFaultCode,
		"faultmessage":   features.StatusAttrFaultMessage,
	}
	for k, v := range globalAttrs {
		statusAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureStatus)] = statusAttrs

	// DeviceInfo attributes
	deviceInfoAttrs := map[string]uint16{
		"deviceid":        features.DeviceInfoAttrDeviceID,
		"vendorname":      features.DeviceInfoAttrVendorName,
		"productname":     features.DeviceInfoAttrProductName,
		"serialnumber":    features.DeviceInfoAttrSerialNumber,
		"vendorid":        features.DeviceInfoAttrVendorID,
		"productid":       features.DeviceInfoAttrProductID,
		"softwareversion": features.DeviceInfoAttrSoftwareVersion,
		"hardwareversion": features.DeviceInfoAttrHardwareVersion,
		"specversion":     features.DeviceInfoAttrSpecVersion,
		"endpoints":       features.DeviceInfoAttrEndpoints,
		"location":        features.DeviceInfoAttrLocation,
		"label":           features.DeviceInfoAttrLabel,
	}
	for k, v := range globalAttrs {
		deviceInfoAttrs[k] = v
	}
	attributeNames[uint8(model.FeatureDeviceInfo)] = deviceInfoAttrs
}

func init() {
	initNameTables()
}

// ResolveEndpointName resolves an endpoint name to its ID.
func ResolveEndpointName(name string) (uint8, bool) {
	id, ok := endpointNames[name]
	return id, ok
}

// ResolveFeatureName resolves a feature name to its ID.
func ResolveFeatureName(name string) (uint8, bool) {
	id, ok := featureNames[name]
	return id, ok
}

// ResolveAttributeName resolves an attribute name to its ID for a given feature.
func ResolveAttributeName(featureID uint8, name string) (uint16, bool) {
	if attrNames, ok := attributeNames[featureID]; ok {
		id, found := attrNames[name]
		return id, found
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
