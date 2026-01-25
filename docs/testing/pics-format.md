# MASH PICS Specification

> Protocol Implementation Conformance Statement format and codes

**Status:** Draft
**Last Updated:** 2025-01-25

---

## 1. Overview

A **PICS** (Protocol Implementation Conformance Statement) is a formal declaration of which protocol features a device implements. PICS enables:

1. **Test selection** - Test harness knows which tests apply to this device
2. **Interoperability** - Controllers know device capabilities before interaction
3. **Certification** - Formal record of claimed compliance
4. **Documentation** - Clear specification of implementation scope

MASH PICS follows Matter's approach but adapted for MASH protocol structure.

---

## 2. PICS Code Format

### 2.1 Code Structure

```
MASH.<Side>.<Feature>[.<Type><ID>][.<Qualifier>]
```

| Component | Description | Values |
|-----------|-------------|--------|
| MASH | Protocol identifier | Always "MASH" |
| Side | Implementation side | S (Server/Device), C (Client/Controller) |
| Feature | Feature identifier | ELEC, MEAS, CTRL, STAT, INFO, CHRG, SIG, TAR, PLAN |
| Type | Item type | F (feature flag), A (attribute), C (command), E (event) |
| ID | Hex ID from spec | 00-FF |
| Qualifier | Additional context | Rsp (accepts), Tx (generates), Opt (optional) |

### 2.2 Feature Identifiers

| ID | Feature | Description |
|----|---------|-------------|
| ELEC | Electrical | Electrical characteristics |
| MEAS | Measurement | Power/energy telemetry |
| CTRL | EnergyControl | Limits, setpoints, control |
| STAT | Status | Operating state, faults |
| INFO | DeviceInfo | Device identity |
| CHRG | ChargingSession | EV charging session |
| SIG | Signals | Time-slotted signals |
| TAR | Tariff | Price structure |
| PLAN | Plan | Device's intended behavior |

### 2.3 Type Identifiers

| Type | Meaning | Example |
|------|---------|---------|
| F | Feature flag (from featureMap) | MASH.S.CTRL.F03 (EMOB flag) |
| A | Attribute | MASH.S.CTRL.A14 (isPausable) |
| C | Command | MASH.S.CTRL.C01 (SetLimit) |
| E | Event | MASH.S.CTRL.E01 (LimitChanged) |
| B | Behavior | MASH.S.CTRL.B_LIMIT_STACKING |

### 2.4 Qualifiers

| Qualifier | Meaning |
|-----------|---------|
| .Rsp | Device accepts/responds to this command |
| .Tx | Device generates/sends this command |
| .Opt | Feature is optional and implemented |
| .Mfg | Manufacturer-specific extension |

---

## 3. PICS File Format

### 3.1 File Structure

PICS files use a simple key-value format:

```
# MASH PICS File
# Device: <vendor> <model>
# Version: <PICS version>
# Date: <creation date>

# Protocol Support
MASH.S=1

# Feature Presence
MASH.S.ELEC=1
MASH.S.MEAS=1
MASH.S.CTRL=1
MASH.S.CHRG=1

# Feature Flags (from featureMap)
MASH.S.CTRL.F00=1       # CORE
MASH.S.CTRL.F03=1       # EMOB
MASH.S.CTRL.F09=1       # ASYMMETRIC

# Attributes
MASH.S.CTRL.A10=1       # acceptsLimits
MASH.S.CTRL.A11=1       # acceptsCurrentLimits
MASH.S.CTRL.A14=1       # isPausable

# Commands
MASH.S.CTRL.C01.Rsp=1   # SetLimit
MASH.S.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.CTRL.C09.Rsp=1   # Pause
```

### 3.2 Syntax Rules

1. Lines starting with `#` are comments
2. Empty lines are ignored
3. Format: `<PICS_CODE>=<VALUE>`
4. Values: `1` (supported), `0` (not supported), or specific value
5. Case-insensitive for PICS codes

### 3.3 BNF Grammar

```bnf
<pics-file>     ::= <line>*
<line>          ::= <comment> | <assignment> | <empty>
<comment>       ::= "#" <text> <newline>
<assignment>    ::= <pics-code> "=" <value> <newline>
<empty>         ::= <newline>

<pics-code>     ::= "MASH" "." <side> ["." <feature>] ["." <type> <id>] ["." <qualifier>]
<side>          ::= "S" | "C"
<feature>       ::= "ELEC" | "MEAS" | "CTRL" | "STAT" | "INFO" | "CHRG" | "SIG" | "TAR" | "PLAN"
<type>          ::= "F" | "A" | "C" | "E" | "B"
<id>            ::= <hex-digit> <hex-digit>
<qualifier>     ::= "Rsp" | "Tx" | "Opt" | "Mfg"
<hex-digit>     ::= "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
                  | "A" | "B" | "C" | "D" | "E" | "F"
                  | "a" | "b" | "c" | "d" | "e" | "f"

<value>         ::= "0" | "1" | <number> | <string>
<number>        ::= <digit>+
<string>        ::= '"' <char>* '"'
<text>          ::= <char>*
<char>          ::= any printable character
<digit>         ::= "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
<newline>       ::= LF | CRLF
```

---

## 4. PICS Code Registry

### 4.1 Protocol-Level PICS

| PICS Code | Description | Values |
|-----------|-------------|--------|
| MASH.S | Device implements MASH server | 1 |
| MASH.C | Controller implements MASH client | 1 |
| MASH.S.VERSION | Protocol version supported | 1 |
| MASH.S.ENDPOINTS | Number of endpoints | 1-255 |

### 4.2 Electrical Feature (ELEC)

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.ELEC | Electrical feature present | M |
| MASH.S.ELEC.A01 | phaseCount | M |
| MASH.S.ELEC.A02 | phaseMapping | M |
| MASH.S.ELEC.A03 | nominalVoltage | M |
| MASH.S.ELEC.A04 | nominalFrequency | M |
| MASH.S.ELEC.A05 | supportedDirections | M |
| MASH.S.ELEC.A0A | nominalMaxConsumption | M if CONSUMPTION/BIDIRECTIONAL |
| MASH.S.ELEC.A0B | nominalMaxProduction | M if PRODUCTION/BIDIRECTIONAL |
| MASH.S.ELEC.A0C | nominalMinPower | O |
| MASH.S.ELEC.A0D | maxCurrentPerPhase | M |
| MASH.S.ELEC.A0E | minCurrentPerPhase | O |
| MASH.S.ELEC.A0F | supportsAsymmetric | M if phaseCount > 1 |
| MASH.S.ELEC.A14 | energyCapacity | M if BATTERY |

### 4.3 EnergyControl Feature (CTRL)

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.CTRL | EnergyControl feature present | M |
| MASH.S.CTRL.F00 | CORE feature flag | M |
| MASH.S.CTRL.F01 | FLEX feature flag | O |
| MASH.S.CTRL.F02 | BATTERY feature flag | O |
| MASH.S.CTRL.F03 | EMOB feature flag | O |
| MASH.S.CTRL.F04 | SIGNALS feature flag | O |
| MASH.S.CTRL.F05 | TARIFF feature flag | O |
| MASH.S.CTRL.F06 | PLAN feature flag | O |
| MASH.S.CTRL.F07 | PROCESS feature flag | O |
| MASH.S.CTRL.F08 | FORECAST feature flag | O |
| MASH.S.CTRL.F09 | ASYMMETRIC feature flag | O |
| MASH.S.CTRL.F0A | V2X feature flag | O |
| MASH.S.CTRL.A01 | deviceType | M |
| MASH.S.CTRL.A02 | controlState | M |
| MASH.S.CTRL.A03 | optOutState | O |
| MASH.S.CTRL.A0A | acceptsLimits | M |
| MASH.S.CTRL.A0B | acceptsCurrentLimits | M |
| MASH.S.CTRL.A0C | acceptsSetpoints | M |
| MASH.S.CTRL.A0D | acceptsCurrentSetpoints | M if V2X |
| MASH.S.CTRL.A0E | isPausable | M |
| MASH.S.CTRL.A0F | isShiftable | O |
| MASH.S.CTRL.A10 | isStoppable | M if PROCESS |
| MASH.S.CTRL.A14 | effectiveConsumptionLimit | M if acceptsLimits |
| MASH.S.CTRL.A15 | myConsumptionLimit | M if acceptsLimits |
| MASH.S.CTRL.A16 | effectiveProductionLimit | M if BIDIRECTIONAL & acceptsLimits |
| MASH.S.CTRL.A17 | myProductionLimit | M if BIDIRECTIONAL & acceptsLimits |
| MASH.S.CTRL.A1E | effectiveCurrentLimitsConsumption | M if ASYMMETRIC |
| MASH.S.CTRL.A1F | myCurrentLimitsConsumption | M if ASYMMETRIC |
| MASH.S.CTRL.A20 | effectiveCurrentLimitsProduction | M if ASYMMETRIC & BIDIRECTIONAL |
| MASH.S.CTRL.A21 | myCurrentLimitsProduction | M if ASYMMETRIC & BIDIRECTIONAL |
| MASH.S.CTRL.A3C | flexibility | M if FLEX |
| MASH.S.CTRL.A3D | forecast | M if FORECAST |
| MASH.S.CTRL.A46 | failsafeConsumptionLimit | M |
| MASH.S.CTRL.A47 | failsafeProductionLimit | M if BIDIRECTIONAL |
| MASH.S.CTRL.A48 | failsafeDuration | M |
| MASH.S.CTRL.A50 | processState | M if PROCESS |
| MASH.S.CTRL.A51 | optionalProcess | M if PROCESS |
| MASH.S.CTRL.C01.Rsp | SetLimit | M if acceptsLimits |
| MASH.S.CTRL.C02.Rsp | ClearLimit | M if acceptsLimits |
| MASH.S.CTRL.C03.Rsp | SetSetpoint | M if acceptsSetpoints |
| MASH.S.CTRL.C04.Rsp | ClearSetpoint | M if acceptsSetpoints |
| MASH.S.CTRL.C05.Rsp | SetCurrentLimits | M if acceptsCurrentLimits |
| MASH.S.CTRL.C06.Rsp | ClearCurrentLimits | M if acceptsCurrentLimits |
| MASH.S.CTRL.C07.Rsp | SetCurrentSetpoints | M if V2X |
| MASH.S.CTRL.C08.Rsp | ClearCurrentSetpoints | M if V2X |
| MASH.S.CTRL.C09.Rsp | Pause | M if isPausable |
| MASH.S.CTRL.C0A.Rsp | Resume | M if isPausable |
| MASH.S.CTRL.C0B.Rsp | Stop | M if isStoppable |
| MASH.S.CTRL.C0C.Rsp | ScheduleProcess | M if PROCESS |
| MASH.S.CTRL.C0D.Rsp | CancelProcess | M if PROCESS |
| MASH.S.CTRL.C0E.Rsp | AdjustStartTime | O if isShiftable |

### 4.4 ChargingSession Feature (CHRG)

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.CHRG | ChargingSession feature present | M if EMOB |
| MASH.S.CHRG.A01 | state | M |
| MASH.S.CHRG.A02 | sessionId | M |
| MASH.S.CHRG.A03 | sessionStartTime | M |
| MASH.S.CHRG.A04 | sessionEndTime | O |
| MASH.S.CHRG.A0A | sessionEnergyCharged | M |
| MASH.S.CHRG.A0B | sessionEnergyDischarged | M if V2X |
| MASH.S.CHRG.A14 | evIdentifications | O |
| MASH.S.CHRG.A1E | evStateOfCharge | O |
| MASH.S.CHRG.A1F | evBatteryCapacity | O |
| MASH.S.CHRG.A28 | evDemandMode | M |
| MASH.S.CHRG.A29 | evMinEnergyRequest | O |
| MASH.S.CHRG.A2A | evMaxEnergyRequest | O |
| MASH.S.CHRG.A2B | evTargetEnergyRequest | O |
| MASH.S.CHRG.A2C | evDepartureTime | O |
| MASH.S.CHRG.A32 | evMinDischargingRequest | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.CHRG.A33 | evMaxDischargingRequest | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.CHRG.A34 | evDischargeBelowTargetPermitted | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.CHRG.A46 | chargingMode | M |
| MASH.S.CHRG.A47 | supportedChargingModes | M |
| MASH.S.CHRG.A48 | surplusThreshold | M if PV_SURPLUS_THRESHOLD supported |
| MASH.S.CHRG.A50 | startDelay | O |
| MASH.S.CHRG.A51 | stopDelay | O |
| MASH.S.CHRG.C01.Rsp | SetChargingMode | M |

### 4.5 Signals Feature (SIG)

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.SIG | Signals feature present | M if SIGNALS |
| MASH.S.SIG.A01 | activeSignals | M |
| MASH.S.SIG.A02 | signalCount | M |
| MASH.S.SIG.A0A | lastReceivedSignalId | M |
| MASH.S.SIG.A0B | signalStatus | M |
| MASH.S.SIG.C01.Rsp | SendSignal | M |
| MASH.S.SIG.C02.Rsp | ClearSignals | M |

### 4.6 Behavior PICS

| PICS Code | Description | Values |
|-----------|-------------|--------|
| MASH.S.CTRL.B_LIMIT_DEFAULT | Default when no limit set | "unlimited" or value in mW |
| MASH.S.CTRL.B_PAUSED_AUTO_RESUME | Resume after FAILSAFE when PAUSED | 0, 1 |
| MASH.S.CTRL.B_DURATION_EXPIRY | Behavior on duration expiry | "clear", "notify" |
| MASH.S.SUB.B_HEARTBEAT_CONTENT | Heartbeat notification content | "empty", "full" |
| MASH.S.SUB.B_COALESCE | Coalescing strategy | "last_value", "all_changes" |

---

## 5. Example PICS Files

### 5.1 Basic V1G EVSE

```
# MASH PICS File
# Device: ExampleCorp WallBox V1G
# Version: 1.0
# Date: 2025-01-25

# Protocol Support
MASH.S=1
MASH.S.VERSION=1
MASH.S.ENDPOINTS=2

# Features
MASH.S.ELEC=1
MASH.S.MEAS=1
MASH.S.CTRL=1
MASH.S.CHRG=1
MASH.S.STAT=1
MASH.S.INFO=1

# Feature Flags
MASH.S.CTRL.F00=1       # CORE
MASH.S.CTRL.F03=1       # EMOB

# Electrical
MASH.S.ELEC.A01=1       # phaseCount (value: 3)
MASH.S.ELEC.A02=1       # phaseMapping
MASH.S.ELEC.A03=1       # nominalVoltage
MASH.S.ELEC.A04=1       # nominalFrequency
MASH.S.ELEC.A05=1       # supportedDirections (CONSUMPTION)
MASH.S.ELEC.A0A=1       # nominalMaxConsumption
MASH.S.ELEC.A0D=1       # maxCurrentPerPhase
MASH.S.ELEC.A0F=1       # supportsAsymmetric (NONE)

# EnergyControl Attributes
MASH.S.CTRL.A01=1       # deviceType (EVSE)
MASH.S.CTRL.A02=1       # controlState
MASH.S.CTRL.A0A=1       # acceptsLimits = true
MASH.S.CTRL.A0B=1       # acceptsCurrentLimits = true
MASH.S.CTRL.A0C=1       # acceptsSetpoints = false
MASH.S.CTRL.A0D=0       # acceptsCurrentSetpoints = false
MASH.S.CTRL.A0E=1       # isPausable = true
MASH.S.CTRL.A14=1       # effectiveConsumptionLimit
MASH.S.CTRL.A15=1       # myConsumptionLimit
MASH.S.CTRL.A46=1       # failsafeConsumptionLimit
MASH.S.CTRL.A48=1       # failsafeDuration

# EnergyControl Commands
MASH.S.CTRL.C01.Rsp=1   # SetLimit
MASH.S.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.CTRL.C05.Rsp=1   # SetCurrentLimits
MASH.S.CTRL.C06.Rsp=1   # ClearCurrentLimits
MASH.S.CTRL.C09.Rsp=1   # Pause
MASH.S.CTRL.C0A.Rsp=1   # Resume

# ChargingSession
MASH.S.CHRG.A01=1       # state
MASH.S.CHRG.A02=1       # sessionId
MASH.S.CHRG.A03=1       # sessionStartTime
MASH.S.CHRG.A0A=1       # sessionEnergyCharged
MASH.S.CHRG.A28=1       # evDemandMode
MASH.S.CHRG.A46=1       # chargingMode
MASH.S.CHRG.A47=1       # supportedChargingModes
MASH.S.CHRG.C01.Rsp=1   # SetChargingMode

# Behavior
MASH.S.CTRL.B_LIMIT_DEFAULT="unlimited"
MASH.S.CTRL.B_DURATION_EXPIRY="clear"
```

### 5.2 Advanced V2H EVSE with Signals

```
# MASH PICS File
# Device: ExampleCorp WallBox V2H Pro
# Version: 1.0
# Date: 2025-01-25

# Protocol Support
MASH.S=1
MASH.S.VERSION=1
MASH.S.ENDPOINTS=2

# Features
MASH.S.ELEC=1
MASH.S.MEAS=1
MASH.S.CTRL=1
MASH.S.CHRG=1
MASH.S.SIG=1
MASH.S.PLAN=1
MASH.S.STAT=1
MASH.S.INFO=1

# Feature Flags
MASH.S.CTRL.F00=1       # CORE
MASH.S.CTRL.F01=1       # FLEX
MASH.S.CTRL.F03=1       # EMOB
MASH.S.CTRL.F04=1       # SIGNALS
MASH.S.CTRL.F06=1       # PLAN
MASH.S.CTRL.F09=1       # ASYMMETRIC
MASH.S.CTRL.F0A=1       # V2X

# Electrical
MASH.S.ELEC.A01=1       # phaseCount (value: 3)
MASH.S.ELEC.A02=1       # phaseMapping
MASH.S.ELEC.A03=1       # nominalVoltage
MASH.S.ELEC.A04=1       # nominalFrequency
MASH.S.ELEC.A05=1       # supportedDirections (BIDIRECTIONAL)
MASH.S.ELEC.A0A=1       # nominalMaxConsumption
MASH.S.ELEC.A0B=1       # nominalMaxProduction
MASH.S.ELEC.A0D=1       # maxCurrentPerPhase
MASH.S.ELEC.A0F=1       # supportsAsymmetric (BIDIRECTIONAL)

# EnergyControl Attributes
MASH.S.CTRL.A01=1       # deviceType (EVSE)
MASH.S.CTRL.A02=1       # controlState
MASH.S.CTRL.A0A=1       # acceptsLimits = true
MASH.S.CTRL.A0B=1       # acceptsCurrentLimits = true
MASH.S.CTRL.A0C=1       # acceptsSetpoints = true
MASH.S.CTRL.A0D=1       # acceptsCurrentSetpoints = true (V2X)
MASH.S.CTRL.A0E=1       # isPausable = true
MASH.S.CTRL.A14=1       # effectiveConsumptionLimit
MASH.S.CTRL.A15=1       # myConsumptionLimit
MASH.S.CTRL.A16=1       # effectiveProductionLimit
MASH.S.CTRL.A17=1       # myProductionLimit
MASH.S.CTRL.A1E=1       # effectiveCurrentLimitsConsumption
MASH.S.CTRL.A1F=1       # myCurrentLimitsConsumption
MASH.S.CTRL.A20=1       # effectiveCurrentLimitsProduction
MASH.S.CTRL.A21=1       # myCurrentLimitsProduction
MASH.S.CTRL.A28=1       # effectiveConsumptionSetpoint
MASH.S.CTRL.A29=1       # myConsumptionSetpoint
MASH.S.CTRL.A2A=1       # effectiveProductionSetpoint
MASH.S.CTRL.A2B=1       # myProductionSetpoint
MASH.S.CTRL.A32=1       # effectiveCurrentSetpointsConsumption (V2X)
MASH.S.CTRL.A33=1       # myCurrentSetpointsConsumption (V2X)
MASH.S.CTRL.A34=1       # effectiveCurrentSetpointsProduction (V2X)
MASH.S.CTRL.A35=1       # myCurrentSetpointsProduction (V2X)
MASH.S.CTRL.A3C=1       # flexibility (FLEX)
MASH.S.CTRL.A46=1       # failsafeConsumptionLimit
MASH.S.CTRL.A47=1       # failsafeProductionLimit
MASH.S.CTRL.A48=1       # failsafeDuration

# EnergyControl Commands
MASH.S.CTRL.C01.Rsp=1   # SetLimit
MASH.S.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.CTRL.C03.Rsp=1   # SetSetpoint
MASH.S.CTRL.C04.Rsp=1   # ClearSetpoint
MASH.S.CTRL.C05.Rsp=1   # SetCurrentLimits
MASH.S.CTRL.C06.Rsp=1   # ClearCurrentLimits
MASH.S.CTRL.C07.Rsp=1   # SetCurrentSetpoints (V2X)
MASH.S.CTRL.C08.Rsp=1   # ClearCurrentSetpoints (V2X)
MASH.S.CTRL.C09.Rsp=1   # Pause
MASH.S.CTRL.C0A.Rsp=1   # Resume

# ChargingSession (complete V2X)
MASH.S.CHRG.A01=1       # state
MASH.S.CHRG.A02=1       # sessionId
MASH.S.CHRG.A03=1       # sessionStartTime
MASH.S.CHRG.A04=1       # sessionEndTime
MASH.S.CHRG.A0A=1       # sessionEnergyCharged
MASH.S.CHRG.A0B=1       # sessionEnergyDischarged (V2X)
MASH.S.CHRG.A14=1       # evIdentifications
MASH.S.CHRG.A1E=1       # evStateOfCharge
MASH.S.CHRG.A1F=1       # evBatteryCapacity
MASH.S.CHRG.A28=1       # evDemandMode
MASH.S.CHRG.A29=1       # evMinEnergyRequest
MASH.S.CHRG.A2A=1       # evMaxEnergyRequest
MASH.S.CHRG.A2B=1       # evTargetEnergyRequest
MASH.S.CHRG.A2C=1       # evDepartureTime
MASH.S.CHRG.A32=1       # evMinDischargingRequest
MASH.S.CHRG.A33=1       # evMaxDischargingRequest
MASH.S.CHRG.A34=1       # evDischargeBelowTargetPermitted
MASH.S.CHRG.A46=1       # chargingMode
MASH.S.CHRG.A47=1       # supportedChargingModes
MASH.S.CHRG.A48=1       # surplusThreshold
MASH.S.CHRG.A50=1       # startDelay
MASH.S.CHRG.A51=1       # stopDelay
MASH.S.CHRG.C01.Rsp=1   # SetChargingMode

# Signals
MASH.S.SIG.A01=1        # activeSignals
MASH.S.SIG.A02=1        # signalCount
MASH.S.SIG.A0A=1        # lastReceivedSignalId
MASH.S.SIG.A0B=1        # signalStatus
MASH.S.SIG.C01.Rsp=1    # SendSignal
MASH.S.SIG.C02.Rsp=1    # ClearSignals

# Behavior
MASH.S.CTRL.B_LIMIT_DEFAULT="unlimited"
MASH.S.CTRL.B_PAUSED_AUTO_RESUME=0
MASH.S.CTRL.B_DURATION_EXPIRY="clear"
MASH.S.SUB.B_HEARTBEAT_CONTENT="full"
MASH.S.SUB.B_COALESCE="last_value"
```

---

## 6. PICS Validation

### 6.1 Validation Rules

A valid PICS file MUST satisfy:

1. **Protocol declaration**: `MASH.S=1` or `MASH.C=1` must be present
2. **Feature consistency**: If `MASH.S.<FEATURE>=1`, all mandatory attributes for that feature must be declared
3. **Flag dependencies**: Feature flag dependencies must be satisfied (e.g., V2X requires EMOB)
4. **Command consistency**: If attribute `acceptsFoo=true`, corresponding `SetFoo` command must be declared

### 6.2 Validation Algorithm

```python
def validate_pics(pics: dict) -> list[str]:
    errors = []

    # Check protocol declaration
    if not pics.get("MASH.S") and not pics.get("MASH.C"):
        errors.append("Missing protocol declaration (MASH.S or MASH.C)")

    # Check feature flag dependencies
    if pics.get("MASH.S.CTRL.F0A"):  # V2X
        if not pics.get("MASH.S.CTRL.F03"):  # EMOB
            errors.append("V2X (F0A) requires EMOB (F03)")

    # Check mandatory attributes per feature
    if pics.get("MASH.S.CTRL"):
        mandatory_ctrl = ["A01", "A02", "A0A", "A0B", "A0C", "A0E", "A46", "A48"]
        for attr in mandatory_ctrl:
            if not pics.get(f"MASH.S.CTRL.{attr}"):
                errors.append(f"Missing mandatory attribute MASH.S.CTRL.{attr}")

    # Check command consistency
    if pics.get("MASH.S.CTRL.A0A"):  # acceptsLimits
        if not pics.get("MASH.S.CTRL.C01.Rsp"):  # SetLimit
            errors.append("acceptsLimits requires SetLimit command")

    return errors
```

---

## 7. Test Harness Integration

### 7.1 Test Selection

The test harness uses PICS to select applicable tests:

```python
def select_tests(pics: dict, all_tests: list[Test]) -> list[Test]:
    applicable = []
    for test in all_tests:
        if test.pics_requirements_met(pics):
            applicable.append(test)
    return applicable
```

### 7.2 Test Case PICS Requirements

Each test case declares its PICS requirements:

```yaml
test_case: TC-CTRL-001
description: Verify SetLimit command
pics_requirements:
  - MASH.S.CTRL=1
  - MASH.S.CTRL.A0A=1       # acceptsLimits
  - MASH.S.CTRL.C01.Rsp=1   # SetLimit
steps:
  - send: SetLimit(consumptionLimit: 5000000)
  - verify: effectiveConsumptionLimit <= 5000000
```

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Conformance Rules](../conformance/README.md) | Attribute conformance |
| [Testing README](README.md) | Test approach overview |
| [Testability Analysis](../testability-analysis.md) | Gap analysis |
