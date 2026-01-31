# MASH Conformance Rules

> Formal specification of valid feature combinations and attribute requirements

**Status:** Draft
**Last Updated:** 2026-01-31

---

## Purpose

This document defines the **conformance rules** for MASH protocol implementations. Conformance rules specify:

1. **Feature bit dependencies** - Which feature flags require other flags
2. **Attribute conformance** - Which attributes are mandatory/optional per feature
3. **Endpoint type constraints** - Which features are valid on which endpoint types
4. **Command conformance** - Which commands must be supported per feature

These rules ensure interoperability by preventing devices from claiming impossible or ambiguous capability combinations.

---

## 1. Feature Map Bit Definitions

```
bit 0  (0x0001): CORE       - EnergyCore basics (always set for energy devices)
bit 1  (0x0002): FLEX       - Flexible power adjustment
bit 2  (0x0004): BATTERY    - Battery-specific attributes
bit 3  (0x0008): EMOB       - E-Mobility/EVSE
bit 4  (0x0010): SIGNALS    - Incentive signals support
bit 5  (0x0020): TARIFF     - Tariff data support
bit 6  (0x0040): PLAN       - Power plan support
bit 7  (0x0080): PROCESS    - Optional process lifecycle
bit 8  (0x0100): FORECAST   - Power forecasting capability
bit 9  (0x0200): ASYMMETRIC - Per-phase asymmetric control
bit 10 (0x0400): V2X        - Vehicle-to-grid/home
```

---

## 2. Feature Bit Dependencies

### 2.1 Dependency Rules

| Feature Bit | Requires | Excludes | Notes |
|-------------|----------|----------|-------|
| CORE | - | - | Base feature, always required for energy endpoints |
| FLEX | CORE | - | Flexibility implies controllable device |
| BATTERY | CORE | EMOB | Battery endpoints are not EV chargers |
| EMOB | CORE | BATTERY | EV charger endpoints are not batteries |
| SIGNALS | CORE | - | Signal reception requires control capability |
| TARIFF | CORE | - | Tariff info requires energy context |
| PLAN | CORE | - | Plan output requires energy context |
| PROCESS | CORE | - | Process management requires control |
| FORECAST | CORE | - | Forecasting requires energy context |
| ASYMMETRIC | CORE | - | Requires `Electrical.phaseCount > 1` |
| V2X | EMOB | - | V2X is specifically for bidirectional EVs |

### 2.2 Dependency Validation Rules

```
RULE: V2X requires EMOB
  IF featureMap.V2X = 1
  THEN featureMap.EMOB MUST = 1

RULE: ASYMMETRIC requires multi-phase
  IF featureMap.ASYMMETRIC = 1
  THEN Electrical.phaseCount MUST > 1

RULE: BATTERY and EMOB are mutually exclusive
  NOT (featureMap.BATTERY = 1 AND featureMap.EMOB = 1)

RULE: V2X requires bidirectional capability
  IF featureMap.V2X = 1
  THEN Electrical.supportedDirections MUST = BIDIRECTIONAL

RULE: CORE is mandatory for energy endpoints
  IF EndpointType IN [EV_CHARGER, INVERTER, BATTERY, PV_STRING,
                      GRID_CONNECTION, HEAT_PUMP, WATER_HEATER, FLEXIBLE_LOAD]
  THEN featureMap.CORE MUST = 1
```

### 2.3 Valid Feature Combinations

| Endpoint Type | Required Bits | Optional Bits | Invalid Bits |
|---------------|---------------|---------------|--------------|
| EV_CHARGER | CORE, EMOB | FLEX, SIGNALS, TARIFF, PLAN, ASYMMETRIC, V2X | BATTERY |
| BATTERY | CORE, BATTERY | FLEX, SIGNALS, TARIFF, PLAN, FORECAST | EMOB, V2X |
| INVERTER | CORE | FLEX, SIGNALS, TARIFF, PLAN, FORECAST, ASYMMETRIC | BATTERY, EMOB, V2X |
| PV_STRING | CORE | FORECAST | BATTERY, EMOB, V2X, FLEX, PROCESS |
| HEAT_PUMP | CORE | FLEX, PROCESS, SIGNALS, TARIFF, PLAN | BATTERY, EMOB, V2X |
| WATER_HEATER | CORE | FLEX, PROCESS, SIGNALS, TARIFF, PLAN | BATTERY, EMOB, V2X |
| GRID_CONNECTION | CORE | - | BATTERY, EMOB, V2X, FLEX, PROCESS |
| DEVICE_ROOT | - | - | All energy features |

---

## 3. Attribute Conformance

### 3.1 Conformance Notation

| Symbol | Meaning |
|--------|---------|
| **M** | Mandatory - MUST be present |
| **O** | Optional - MAY be present |
| **[F]** | Conditional on feature flag F being set |
| **P** | Provisional - MAY be present, behavior undefined if absent |
| **D** | Deprecated - SHOULD NOT be used |
| **X** | Prohibited - MUST NOT be present |

### 3.2 Electrical Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | phaseCount | M | |
| 2 | phaseMapping | M | |
| 3 | nominalVoltage | M | |
| 4 | nominalFrequency | M | |
| 5 | supportedDirections | M | |
| 10 | nominalMaxConsumption | M | if supportedDirections IN [CONSUMPTION, BIDIRECTIONAL] |
| 11 | nominalMaxProduction | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] |
| 12 | nominalMinPower | O | |
| 13 | maxCurrentPerPhase | M | |
| 14 | minCurrentPerPhase | O | |
| 15 | supportsAsymmetric | M | if phaseCount > 1; otherwise X |
| 20 | energyCapacity | [BATTERY] | Mandatory if BATTERY flag set |

### 3.3 EnergyControl Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | deviceType | M | |
| 2 | controlState | M | |
| 3 | optOutState | O | |
| 10 | acceptsLimits | M | |
| 11 | acceptsCurrentLimits | M | |
| 12 | acceptsSetpoints | M | |
| 13 | acceptsCurrentSetpoints | M | if V2X flag set; otherwise O |
| 14 | isPausable | M | |
| 15 | isShiftable | O | |
| 16 | isStoppable | M | if PROCESS flag set; otherwise O |
| 20 | effectiveConsumptionLimit | M | if acceptsLimits = true |
| 21 | myConsumptionLimit | M | if acceptsLimits = true |
| 22 | effectiveProductionLimit | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] AND acceptsLimits = true |
| 23 | myProductionLimit | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] AND acceptsLimits = true |
| 30 | effectiveCurrentLimitsConsumption | [ASYMMETRIC] | Mandatory if ASYMMETRIC flag set AND acceptsCurrentLimits = true |
| 31 | myCurrentLimitsConsumption | [ASYMMETRIC] | Mandatory if ASYMMETRIC flag set AND acceptsCurrentLimits = true |
| 32 | effectiveCurrentLimitsProduction | [ASYMMETRIC] | Mandatory if ASYMMETRIC flag set AND supportedDirections = BIDIRECTIONAL |
| 33 | myCurrentLimitsProduction | [ASYMMETRIC] | Mandatory if ASYMMETRIC flag set AND supportedDirections = BIDIRECTIONAL |
| 40 | effectiveConsumptionSetpoint | M | if acceptsSetpoints = true |
| 41 | myConsumptionSetpoint | M | if acceptsSetpoints = true |
| 42 | effectiveProductionSetpoint | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] AND acceptsSetpoints = true |
| 43 | myProductionSetpoint | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] AND acceptsSetpoints = true |
| 50 | effectiveCurrentSetpointsConsumption | [V2X] | Mandatory if V2X flag set |
| 51 | myCurrentSetpointsConsumption | [V2X] | Mandatory if V2X flag set |
| 52 | effectiveCurrentSetpointsProduction | [V2X] | Mandatory if V2X flag set |
| 53 | myCurrentSetpointsProduction | [V2X] | Mandatory if V2X flag set |
| 60 | flexibility | [FLEX] | Mandatory if FLEX flag set |
| 61 | forecast | [FORECAST] | Mandatory if FORECAST flag set |
| 70 | failsafeConsumptionLimit | M | |
| 71 | failsafeProductionLimit | M | if supportedDirections IN [PRODUCTION, BIDIRECTIONAL] |
| 72 | failsafeDuration | M | |
| 80 | processState | [PROCESS] | Mandatory if PROCESS flag set |
| 81 | optionalProcess | [PROCESS] | Mandatory if PROCESS flag set AND processState != NONE |

### 3.4 Measurement Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | powerTotal | M | |
| 2 | powerPerPhase | M | if Electrical.phaseCount > 1 |
| 3 | energyConsumed | M | |
| 4 | energyProduced | M | if Electrical.supportedDirections IN [PRODUCTION, BIDIRECTIONAL] |
| 10 | voltagePerPhase | O | |
| 11 | currentPerPhase | O | |
| 12 | frequencyHz | O | |
| 20 | powerFactor | O | |
| 21 | powerFactorPerPhase | O | if powerFactor present AND Electrical.phaseCount > 1 |

### 3.5 ChargingSession Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | state | M | |
| 2 | sessionId | M | |
| 3 | sessionStartTime | M | |
| 4 | sessionEndTime | O | |
| 10 | sessionEnergyCharged | M | |
| 11 | sessionEnergyDischarged | M | if V2X flag set; otherwise O |
| 20 | evIdentifications | O | |
| 30 | evStateOfCharge | O | ISO 15118 only |
| 31 | evBatteryCapacity | O | ISO 15118 only |
| 40 | evDemandMode | M | |
| 41-44 | evMinEnergyRequest, evMaxEnergyRequest, evTargetEnergyRequest, evDepartureTime | O | ISO 15118 only |
| 50-52 | evMinDischargingRequest, evMaxDischargingRequest, evDischargeBelowTargetPermitted | [V2X] | Mandatory if V2X flag set AND evDemandMode = DYNAMIC_BIDIRECTIONAL |
| 60-62 | estimatedTimeToMinSoC, estimatedTimeToTargetSoC, estimatedTimeToFullSoC | O | |
| 70 | chargingMode | M | |
| 71 | supportedChargingModes | M | |
| 72 | surplusThreshold | M | if PV_SURPLUS_THRESHOLD in supportedChargingModes |
| 80 | startDelay | O | |
| 81 | stopDelay | O | |

### 3.6 Signals Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | activeSignals | M | |
| 2 | signalCount | M | |
| 10 | lastReceivedSignalId | M | |
| 11 | signalStatus | M | |

### 3.7 Status Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | operatingState | M | |
| 2 | faultCode | M | |
| 3 | faultMessage | O | |
| 10 | lastStateChange | O | |

### 3.8 DeviceInfo Feature Attributes

| ID | Attribute | Conformance | Condition |
|----|-----------|-------------|-----------|
| 1 | deviceId | M | |
| 2 | vendorId | M | |
| 3 | productId | M | |
| 4 | serialNumber | M | |
| 5 | firmwareVersion | M | |
| 6 | hardwareVersion | O | |
| 10 | endpoints | M | |

---

## 4. Command Conformance

### 4.1 EnergyControl Commands

| ID | Command | Conformance | Condition |
|----|---------|-------------|-----------|
| 1 | SetLimit | M | if acceptsLimits = true |
| 2 | ClearLimit | M | if acceptsLimits = true |
| 3 | SetSetpoint | M | if acceptsSetpoints = true |
| 4 | ClearSetpoint | M | if acceptsSetpoints = true |
| 5 | SetCurrentLimits | M | if acceptsCurrentLimits = true |
| 6 | ClearCurrentLimits | M | if acceptsCurrentLimits = true |
| 7 | SetCurrentSetpoints | [V2X] | Mandatory if V2X flag set |
| 8 | ClearCurrentSetpoints | [V2X] | Mandatory if V2X flag set |
| 9 | Pause | M | if isPausable = true |
| 10 | Resume | M | if isPausable = true |
| 11 | Stop | M | if isStoppable = true |
| 12 | ScheduleProcess | [PROCESS] | Mandatory if PROCESS flag set |
| 13 | CancelProcess | [PROCESS] | Mandatory if PROCESS flag set |
| 14 | AdjustStartTime | O | if isShiftable = true |

### 4.2 Signals Commands

| ID | Command | Conformance | Condition |
|----|---------|-------------|-----------|
| 1 | SendSignal | M | |
| 2 | ClearSignals | M | |

### 4.3 ChargingSession Commands

| ID | Command | Conformance | Condition |
|----|---------|-------------|-----------|
| 1 | SetChargingMode | M | |

---

## 5. Conformance Validation Algorithm

### 5.1 Device Self-Validation

Devices MUST validate their own conformance at startup:

```
function validateConformance(device):
    // Check feature dependencies
    for each endpoint in device.endpoints:
        featureMap = endpoint.featureMap

        // V2X requires EMOB
        if featureMap.V2X and not featureMap.EMOB:
            return ERROR("V2X requires EMOB")

        // BATTERY and EMOB are mutually exclusive
        if featureMap.BATTERY and featureMap.EMOB:
            return ERROR("BATTERY and EMOB are mutually exclusive")

        // ASYMMETRIC requires multi-phase
        if featureMap.ASYMMETRIC and endpoint.electrical.phaseCount <= 1:
            return ERROR("ASYMMETRIC requires phaseCount > 1")

        // V2X requires bidirectional
        if featureMap.V2X and endpoint.electrical.supportedDirections != BIDIRECTIONAL:
            return ERROR("V2X requires BIDIRECTIONAL supportedDirections")

    // Check attribute presence (feature-level mandatory from YAML)
    for each endpoint in device.endpoints:
        for each attribute in CONFORMANCE_TABLE:
            if attribute.isMandatory(endpoint.featureMap):
                if attribute not in endpoint.attributeList:
                    return ERROR("Missing mandatory attribute: " + attribute.name)

    // Check endpoint-type-aware conformance (DEC-053)
    // See docs/features/endpoint-conformance.yaml for the registry
    for each endpoint in device.endpoints:
        conformance = ENDPOINT_CONFORMANCE[endpoint.type]
        for each feature in endpoint.features:
            if feature.name in conformance:
                for each attr in conformance[feature.name].mandatory:
                    if attr not in endpoint.attributeList:
                        return ERROR("Missing mandatory attribute for " + endpoint.type + ": " + attr)
                for each attr in conformance[feature.name].recommended:
                    if attr not in endpoint.attributeList:
                        WARN("Missing recommended attribute for " + endpoint.type + ": " + attr)

    return SUCCESS
```

### 5.2 Controller Validation

Controllers SHOULD validate device conformance on connection, using the endpoint-aware
PICS format (DEC-054) where each application feature is scoped to an endpoint:

```
function validateDeviceConformance(device):
    for each endpoint in device.endpoints:
        // Read featureMap for this endpoint
        featureMap = device.read(endpoint.id, 0xFFFC)

        // Read attributeList for this endpoint
        attributeList = device.read(endpoint.id, 0xFFFB)

        // Validate feature dependencies per endpoint
        if not validateFeatureDependencies(featureMap):
            return CONFORMANCE_ERROR

        // Validate attribute presence per endpoint
        for each attribute in CONFORMANCE_TABLE:
            if attribute.isMandatory(featureMap):
                if attribute.id not in attributeList:
                    return CONFORMANCE_ERROR

        // EPT-001: Validate endpoint type conformance (DEC-053)
        conformance = ENDPOINT_CONFORMANCE[endpoint.type]
        for each feature in endpoint.features:
            if feature.name in conformance:
                for each attr in conformance[feature.name].mandatory:
                    if attr not in attributeList:
                        return CONFORMANCE_ERROR

    return SUCCESS
```

---

## 6. PICS Mapping

Each conformance rule maps to a PICS code. Application feature codes are endpoint-scoped (DEC-054). See [pics-format.md](../testing/pics-format.md) for the complete PICS specification.

| Conformance Rule | PICS Code |
|------------------|-----------|
| CORE flag set | MASH.S.E01.CTRL.F00 |
| EMOB flag set | MASH.S.E01.CTRL.F03 |
| V2X flag set | MASH.S.E01.CTRL.F0A |
| acceptsLimits attribute | MASH.S.E01.CTRL.A0A |
| SetLimit command | MASH.S.E01.CTRL.C01.Rsp |

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Discovery](../discovery.md) | Feature map and capability discovery |
| [Features README](../features/README.md) | Feature definitions |
| [Endpoint Conformance](../features/endpoint-conformance.yaml) | Per-endpoint-type mandatory/recommended attributes (DEC-053) |
| [PICS Format](../testing/pics-format.md) | PICS specification format |
