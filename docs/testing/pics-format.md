# MASH PICS Specification

> Protocol Implementation Conformance Statement format and codes

**Status:** Draft
**Last Updated:** 2026-01-31

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
MASH.<Side>[.<Endpoint>].<Feature>[.<Type><ID>][.<Qualifier>]
```

| Component | Description | Values |
|-----------|-------------|--------|
| MASH | Protocol identifier | Always "MASH" |
| Side | Implementation side | S (Server/Device), C (Client/Controller) |
| Endpoint | Endpoint identifier (DEC-054) | E01-EFF (two hex digits). Required for application features; omitted for transport/pairing features |
| Feature | Feature identifier | ELEC, MEAS, CTRL, STAT, INFO, CHRG, SIG, TAR, PLAN (endpoint-scoped) or TRANS, COMM, CERT, ZONE, etc. (device-level) |
| Type | Item type | F (feature flag), A (attribute), C (command), E (event) |
| ID | Hex ID from spec | 00-FF |
| Qualifier | Additional context | Rsp (accepts), Tx (generates), Opt (optional) |

**Endpoint type declarations** use the form `MASH.S.E<xx>=<EndpointType>`:

```
MASH.S.E01=EV_CHARGER        # Endpoint 1 is an EV charger
MASH.S.E02=BATTERY            # Endpoint 2 is a battery
```

### 2.2 Feature Identifiers

#### Application Layer Features

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

#### Pairing/Connection Layer Features

| ID | Feature | Description |
|----|---------|-------------|
| TRANS | Transport | TLS, framing, keep-alive |
| COMM | Commissioning | SPAKE2+/PASE, setup codes |
| CERT | Certificate | X.509 management, renewal |
| ZONE | Zone | Multi-zone, priority, admin |
| CONN | Connection | Lifecycle, reconnection |
| FAILSAFE | Failsafe | Connection loss handling |
| SUB | Subscription | Notifications, coalescing |
| DURATION | Duration | Timer semantics |
| DISC | Discovery | mDNS, QR codes |

See [Pairing/Connection PICS Registry](pics/pairing-connection-registry.md) for complete code definitions.

### 2.3 Type Identifiers

| Type | Meaning | Example |
|------|---------|---------|
| F | Feature flag (from featureMap) | MASH.S.E01.CTRL.F03 (EMOB flag) |
| A | Attribute | MASH.S.E01.CTRL.A0E (isPausable) |
| C | Command | MASH.S.E01.CTRL.C01 (SetLimit) |
| E | Event | MASH.S.E01.CTRL.E01 (LimitChanged) |
| B | Behavior | MASH.S.E01.CTRL.B_LIMIT_STACKING |

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

# Endpoint Declarations (DEC-054)
MASH.S.E01=EV_CHARGER

# Feature Flags (from featureMap)
MASH.S.E01.CTRL.F00=1       # CORE
MASH.S.E01.CTRL.F03=1       # EMOB
MASH.S.E01.CTRL.F09=1       # ASYMMETRIC

# Attributes
MASH.S.E01.CTRL.A0A=1       # acceptsLimits
MASH.S.E01.CTRL.A0B=1       # acceptsCurrentLimits
MASH.S.E01.CTRL.A0E=1       # isPausable

# Commands
MASH.S.E01.CTRL.C01.Rsp=1   # SetLimit
MASH.S.E01.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.E01.CTRL.C09.Rsp=1   # Pause
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

<pics-code>     ::= "MASH" "." <side> ["." <endpoint>] ["." <feature>] ["." <type> <id>] ["." <qualifier>]
<side>          ::= "S" | "C"
<endpoint>      ::= "E" <hex-digit> <hex-digit>
<feature>       ::= <app-feature> | <transport-feature>
<app-feature>   ::= "ELEC" | "MEAS" | "CTRL" | "STAT" | "INFO" | "CHRG" | "SIG" | "TAR" | "PLAN"
<transport-feature> ::= "TRANS" | "COMM" | "CERT" | "ZONE" | "CONN" | "FAILSAFE" | "SUB" | "DURATION" | "DISC"
<type>          ::= "F" | "A" | "C" | "E" | "B"
<id>            ::= <hex-digit> <hex-digit>
<qualifier>     ::= "Rsp" | "Tx" | "Opt" | "Mfg"
<hex-digit>     ::= "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
                  | "A" | "B" | "C" | "D" | "E" | "F"
                  | "a" | "b" | "c" | "d" | "e" | "f"

<value>         ::= "0" | "1" | <number> | <string> | <endpoint-type>
<endpoint-type> ::= "EV_CHARGER" | "INVERTER" | "BATTERY" | "PV_STRING"
                  | "GRID_CONNECTION" | "HEAT_PUMP" | "WATER_HEATER"
                  | "HVAC" | "APPLIANCE" | "SUB_METER" | "FLEXIBLE_LOAD"
                  | "DEVICE_ROOT"
<number>        ::= <digit>+
<string>        ::= '"' <char>* '"'
<text>          ::= <char>*
<char>          ::= any printable character
<digit>         ::= "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
<newline>       ::= LF | CRLF
```

**Endpoint scoping rules (DEC-054):**

- Application features (`<app-feature>`) MUST include an `<endpoint>` segment
- Transport/pairing features (`<transport-feature>`) MUST NOT include an `<endpoint>` segment
- Endpoint type declarations use `MASH.S.E<xx>=<endpoint-type>` (no feature or type segment)

---

## 4. PICS Code Registry

### 4.1 Protocol-Level PICS

| PICS Code | Description | Values |
|-----------|-------------|--------|
| MASH.S | Device implements MASH server | 1 |
| MASH.C | Controller implements MASH client | 1 |
| MASH.S.VERSION | Protocol version (major.minor from specVersion) | 1.0 |
| MASH.S.E*xx* | Endpoint type declaration (derived endpoint count) | EndpointType string |

### 4.2 Electrical Feature (ELEC)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.ELEC | Electrical feature present | M |
| MASH.S.E01.ELEC.A01 | phaseCount | M |
| MASH.S.E01.ELEC.A02 | phaseMapping | M |
| MASH.S.E01.ELEC.A03 | nominalVoltage | M |
| MASH.S.E01.ELEC.A04 | nominalFrequency | M |
| MASH.S.E01.ELEC.A05 | supportedDirections | M |
| MASH.S.E01.ELEC.A0A | nominalMaxConsumption | M if CONSUMPTION/BIDIRECTIONAL |
| MASH.S.E01.ELEC.A0B | nominalMaxProduction | M if PRODUCTION/BIDIRECTIONAL |
| MASH.S.E01.ELEC.A0C | nominalMinPower | O |
| MASH.S.E01.ELEC.A0D | maxCurrentPerPhase | M |
| MASH.S.E01.ELEC.A0E | minCurrentPerPhase | O |
| MASH.S.E01.ELEC.A0F | supportsAsymmetric | M if phaseCount > 1 |
| MASH.S.E01.ELEC.A14 | energyCapacity | M if BATTERY |

### 4.3 EnergyControl Feature (CTRL)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.CTRL | EnergyControl feature present | M |
| MASH.S.E01.CTRL.F00 | CORE feature flag | M |
| MASH.S.E01.CTRL.F01 | FLEX feature flag | O |
| MASH.S.E01.CTRL.F02 | BATTERY feature flag | O |
| MASH.S.E01.CTRL.F03 | EMOB feature flag | O |
| MASH.S.E01.CTRL.F04 | SIGNALS feature flag | O |
| MASH.S.E01.CTRL.F05 | TARIFF feature flag | O |
| MASH.S.E01.CTRL.F06 | PLAN feature flag | O |
| MASH.S.E01.CTRL.F07 | PROCESS feature flag | O |
| MASH.S.E01.CTRL.F08 | FORECAST feature flag | O |
| MASH.S.E01.CTRL.F09 | ASYMMETRIC feature flag | O |
| MASH.S.E01.CTRL.F0A | V2X feature flag | O |
| MASH.S.E01.CTRL.A01 | deviceType | M |
| MASH.S.E01.CTRL.A02 | controlState | M |
| MASH.S.E01.CTRL.A03 | optOutState | O |
| MASH.S.E01.CTRL.A0A | acceptsLimits | M |
| MASH.S.E01.CTRL.A0B | acceptsCurrentLimits | M |
| MASH.S.E01.CTRL.A0C | acceptsSetpoints | M |
| MASH.S.E01.CTRL.A0D | acceptsCurrentSetpoints | M if V2X |
| MASH.S.E01.CTRL.A0E | isPausable | M |
| MASH.S.E01.CTRL.A0F | isShiftable | O |
| MASH.S.E01.CTRL.A10 | isStoppable | M if PROCESS |
| MASH.S.E01.CTRL.A14 | effectiveConsumptionLimit | M if acceptsLimits |
| MASH.S.E01.CTRL.A15 | myConsumptionLimit | M if acceptsLimits |
| MASH.S.E01.CTRL.A16 | effectiveProductionLimit | M if BIDIRECTIONAL & acceptsLimits |
| MASH.S.E01.CTRL.A17 | myProductionLimit | M if BIDIRECTIONAL & acceptsLimits |
| MASH.S.E01.CTRL.A1E | effectiveCurrentLimitsConsumption | M if ASYMMETRIC |
| MASH.S.E01.CTRL.A1F | myCurrentLimitsConsumption | M if ASYMMETRIC |
| MASH.S.E01.CTRL.A20 | effectiveCurrentLimitsProduction | M if ASYMMETRIC & BIDIRECTIONAL |
| MASH.S.E01.CTRL.A21 | myCurrentLimitsProduction | M if ASYMMETRIC & BIDIRECTIONAL |
| MASH.S.E01.CTRL.A3C | flexibility | M if FLEX |
| MASH.S.E01.CTRL.A3D | forecast | M if FORECAST |
| MASH.S.E01.CTRL.A46 | failsafeConsumptionLimit | M |
| MASH.S.E01.CTRL.A47 | failsafeProductionLimit | M if BIDIRECTIONAL |
| MASH.S.E01.CTRL.A48 | failsafeDuration | M |
| MASH.S.E01.CTRL.A50 | processState | M if PROCESS |
| MASH.S.E01.CTRL.A51 | optionalProcess | M if PROCESS |
| MASH.S.E01.CTRL.C01.Rsp | SetLimit | M if acceptsLimits |
| MASH.S.E01.CTRL.C02.Rsp | ClearLimit | M if acceptsLimits |
| MASH.S.E01.CTRL.C03.Rsp | SetSetpoint | M if acceptsSetpoints |
| MASH.S.E01.CTRL.C04.Rsp | ClearSetpoint | M if acceptsSetpoints |
| MASH.S.E01.CTRL.C05.Rsp | SetCurrentLimits | M if acceptsCurrentLimits |
| MASH.S.E01.CTRL.C06.Rsp | ClearCurrentLimits | M if acceptsCurrentLimits |
| MASH.S.E01.CTRL.C07.Rsp | SetCurrentSetpoints | M if V2X |
| MASH.S.E01.CTRL.C08.Rsp | ClearCurrentSetpoints | M if V2X |
| MASH.S.E01.CTRL.C09.Rsp | Pause | M if isPausable |
| MASH.S.E01.CTRL.C0A.Rsp | Resume | M if isPausable |
| MASH.S.E01.CTRL.C0B.Rsp | Stop | M if isStoppable |
| MASH.S.E01.CTRL.C0C.Rsp | ScheduleProcess | M if PROCESS |
| MASH.S.E01.CTRL.C0D.Rsp | CancelProcess | M if PROCESS |
| MASH.S.E01.CTRL.C0E.Rsp | AdjustStartTime | O if isShiftable |

### 4.4 ChargingSession Feature (CHRG)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.CHRG | ChargingSession feature present | M if EMOB |
| MASH.S.E01.CHRG.A01 | state | M |
| MASH.S.E01.CHRG.A02 | sessionId | M |
| MASH.S.E01.CHRG.A03 | sessionStartTime | M |
| MASH.S.E01.CHRG.A04 | sessionEndTime | O |
| MASH.S.E01.CHRG.A0A | sessionEnergyCharged | M |
| MASH.S.E01.CHRG.A0B | sessionEnergyDischarged | M if V2X |
| MASH.S.E01.CHRG.A14 | evIdentifications | O |
| MASH.S.E01.CHRG.A1E | evStateOfCharge | O |
| MASH.S.E01.CHRG.A1F | evBatteryCapacity | O |
| MASH.S.E01.CHRG.A28 | evDemandMode | M |
| MASH.S.E01.CHRG.A29 | evMinEnergyRequest | O |
| MASH.S.E01.CHRG.A2A | evMaxEnergyRequest | O |
| MASH.S.E01.CHRG.A2B | evTargetEnergyRequest | O |
| MASH.S.E01.CHRG.A2C | evDepartureTime | O |
| MASH.S.E01.CHRG.A32 | evMinDischargingRequest | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.E01.CHRG.A33 | evMaxDischargingRequest | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.E01.CHRG.A34 | evDischargeBelowTargetPermitted | M if V2X & DYNAMIC_BIDIRECTIONAL |
| MASH.S.E01.CHRG.A46 | chargingMode | M |
| MASH.S.E01.CHRG.A47 | supportedChargingModes | M |
| MASH.S.E01.CHRG.A48 | surplusThreshold | M if PV_SURPLUS_THRESHOLD supported |
| MASH.S.E01.CHRG.A50 | startDelay | O |
| MASH.S.E01.CHRG.A51 | stopDelay | O |
| MASH.S.E01.CHRG.C01.Rsp | SetChargingMode | M |

### 4.5 Signals Feature (SIG)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.SIG | Signals feature present | M if SIGNALS |
| MASH.S.E01.SIG.A01 | activeSignals | M |
| MASH.S.E01.SIG.A02 | signalCount | M |
| MASH.S.E01.SIG.A0A | lastReceivedSignalId | M |
| MASH.S.E01.SIG.A0B | signalStatus | M |
| MASH.S.E01.SIG.C01.Rsp | SendSignal | M |
| MASH.S.E01.SIG.C02.Rsp | ClearSignals | M |

### 4.6 Behavior PICS

Application-layer behaviors are endpoint-scoped; transport-layer behaviors remain device-level.

| PICS Code | Description | Values |
|-----------|-------------|--------|
| MASH.S.E01.CTRL.B_LIMIT_DEFAULT | Default when no limit set | "unlimited" or value in mW |
| MASH.S.E01.CTRL.B_PAUSED_AUTO_RESUME | Resume after FAILSAFE when PAUSED | 0, 1 |
| MASH.S.E01.CTRL.B_DURATION_EXPIRY | Behavior on duration expiry | "clear", "notify" |
| MASH.S.SUB.B_HEARTBEAT_CONTENT | Heartbeat notification content (device-level) | "empty", "full" |
| MASH.S.SUB.B_COALESCE | Coalescing strategy (device-level) | "last_value", "all_changes" |

### 4.7 Measurement Feature (MEAS)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).
Per-endpoint-type mandatory attributes are defined in `endpoint-conformance.yaml` (DEC-053).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.MEAS | Measurement feature present | O |
| MASH.S.E01.MEAS.A01 | acActivePower (mW, +consumption/-production) | Per endpoint type |
| MASH.S.E01.MEAS.A02 | acReactivePower (mVAR) | O |
| MASH.S.E01.MEAS.A03 | acApparentPower (mVA) | O |
| MASH.S.E01.MEAS.A0A | acActivePowerPerPhase | O |
| MASH.S.E01.MEAS.A0B | acReactivePowerPerPhase | O |
| MASH.S.E01.MEAS.A0C | acApparentPowerPerPhase | O |
| MASH.S.E01.MEAS.A14 | acCurrentPerPhase (mA) | O |
| MASH.S.E01.MEAS.A15 | acVoltagePerPhase (mV) | O |
| MASH.S.E01.MEAS.A16 | acVoltagePhaseToPhasePair (mV) | O |
| MASH.S.E01.MEAS.A17 | acFrequency (mHz) | O |
| MASH.S.E01.MEAS.A18 | powerFactor (0.001 units) | O |
| MASH.S.E01.MEAS.A1E | acEnergyConsumed (mWh) | Per endpoint type |
| MASH.S.E01.MEAS.A1F | acEnergyProduced (mWh) | Per endpoint type |
| MASH.S.E01.MEAS.A28 | dcPower (mW) | Per endpoint type |
| MASH.S.E01.MEAS.A29 | dcCurrent (mA) | O |
| MASH.S.E01.MEAS.A2A | dcVoltage (mV) | O |
| MASH.S.E01.MEAS.A2B | dcEnergyIn (mWh) | O |
| MASH.S.E01.MEAS.A2C | dcEnergyOut (mWh) | O |
| MASH.S.E01.MEAS.A32 | stateOfCharge (0-100%) | Per endpoint type |
| MASH.S.E01.MEAS.A33 | stateOfHealth (0-100%) | O |
| MASH.S.E01.MEAS.A34 | stateOfEnergy (mWh) | O |
| MASH.S.E01.MEAS.A35 | useableCapacity (mWh) | O |
| MASH.S.E01.MEAS.A36 | cycleCount | O |
| MASH.S.E01.MEAS.A3C | temperature (centi-degrees) | O |
| MASH.S.E01.MEAS.B_SIGN | Sign convention (+consumption/-production) | M |
| MASH.S.E01.MEAS.B_DIRECTION | IsConsuming/IsProducing helpers | M |
| MASH.S.E01.MEAS.B_NULLABLE | Nullable attributes return false when unset | M |
| MASH.S.E01.MEAS.B_CUMULATIVE | Cumulative energy never decreases | M |

### 4.8 Status Feature (STAT)

Application feature — endpoint-scoped (replace `E01` with actual endpoint ID).

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.E01.STAT | Status feature present | O |
| MASH.S.E01.STAT.A01 | operatingState (OperatingState enum) | M |
| MASH.S.E01.STAT.A02 | stateDetail (vendor-specific code) | O |
| MASH.S.E01.STAT.A03 | faultCode | M if FAULT state |
| MASH.S.E01.STAT.A04 | faultMessage | O |
| MASH.S.E01.STAT.C01 | SetFault (internal) | M |
| MASH.S.E01.STAT.C02 | ClearFault (internal) | M |
| MASH.S.E01.STAT.B_TRANSITIONS | State transition validation | M |
| MASH.S.E01.STAT.B_HELPERS | IsFaulted/IsRunning/IsReady helpers | M |
| MASH.S.E01.STAT.B_FAULT_CODE | FAULT state requires fault code | M |

#### OperatingState Enum Values

| Value | Name | Description |
|-------|------|-------------|
| 0x00 | UNKNOWN | State not determined |
| 0x01 | OFFLINE | Not connected/available |
| 0x02 | STANDBY | Ready but not active |
| 0x03 | STARTING | Powering up/initializing |
| 0x04 | RUNNING | Actively operating |
| 0x05 | PAUSED | Temporarily paused |
| 0x06 | SHUTTING_DOWN | Powering down |
| 0x07 | FAULT | Error condition |
| 0x08 | MAINTENANCE | Under maintenance |

### 4.9 Zone Management (ZONE)

| PICS Code | Description | Conformance |
|-----------|-------------|-------------|
| MASH.S.ZONE | Multi-zone support | M |
| MASH.S.ZONE.MAX | Maximum zones per device (default 5) | M |
| MASH.S.ZONE.GRID | GRID zone (priority 1) - external/regulatory | M |
| MASH.S.ZONE.LOCAL | LOCAL zone (priority 2) - local energy management | M |
| MASH.S.ZONE.ADD | AddZone operation | M |
| MASH.S.ZONE.REMOVE | RemoveZone operation (self-removal) | M |
| MASH.S.ZONE.CONNECT | Connection state tracking | M |
| MASH.S.ZONE.LAST_SEEN | LastSeen timestamp | M |
| MASH.S.ZONE.B_PRIORITY | Highest priority zone wins (setpoints) | M |
| MASH.S.ZONE.B_RESTRICT | Most restrictive wins (limits) | M |
| MASH.S.ZONE.B_SESSION | Per-zone session management | M |
| MASH.S.ZONE.FAILSAFE | Per-zone failsafe tracking | M |
| MASH.S.ZONE.FAILSAFE_DUR | Configurable failsafe duration | M |

#### Zone Priority Hierarchy

| Priority | Zone Type | Description |
|----------|-----------|-------------|
| 1 (highest) | GRID | External/regulatory authority (DSO, utility, SMGW) |
| 2 | LOCAL | Local energy management (home EMS, building EMS) |

#### Multi-Zone Resolution Rules

**Limits (most restrictive wins):**
- Consumption: smaller value is more restrictive
- Production: closer to zero is more restrictive
- Mixed signs: positive (consumption) takes precedence for safety

**Setpoints (highest priority wins):**
- Zone with lowest priority number wins
- GRID (1) > LOCAL (2)

---

## 5. Example PICS Files

### 5.1 Basic V1G EVSE

```
# MASH PICS File
# Device: ExampleCorp WallBox V1G
# Version: 1.0
# Date: 2026-01-31

# Protocol Support
MASH.S=1
MASH.S.VERSION=1.0

# Endpoint Declarations
MASH.S.E01=EV_CHARGER

# Feature Flags
MASH.S.E01.CTRL.F00=1       # CORE
MASH.S.E01.CTRL.F03=1       # EMOB

# Electrical
MASH.S.E01.ELEC.A01=1       # phaseCount (value: 3)
MASH.S.E01.ELEC.A02=1       # phaseMapping
MASH.S.E01.ELEC.A03=1       # nominalVoltage
MASH.S.E01.ELEC.A04=1       # nominalFrequency
MASH.S.E01.ELEC.A05=1       # supportedDirections (CONSUMPTION)
MASH.S.E01.ELEC.A0A=1       # nominalMaxConsumption
MASH.S.E01.ELEC.A0D=1       # maxCurrentPerPhase
MASH.S.E01.ELEC.A0F=1       # supportsAsymmetric (NONE)

# EnergyControl Attributes
MASH.S.E01.CTRL.A01=1       # deviceType (EVSE)
MASH.S.E01.CTRL.A02=1       # controlState
MASH.S.E01.CTRL.A0A=1       # acceptsLimits = true
MASH.S.E01.CTRL.A0B=1       # acceptsCurrentLimits = true
MASH.S.E01.CTRL.A0C=1       # acceptsSetpoints = false
MASH.S.E01.CTRL.A0D=0       # acceptsCurrentSetpoints = false
MASH.S.E01.CTRL.A0E=1       # isPausable = true
MASH.S.E01.CTRL.A14=1       # effectiveConsumptionLimit
MASH.S.E01.CTRL.A15=1       # myConsumptionLimit
MASH.S.E01.CTRL.A46=1       # failsafeConsumptionLimit
MASH.S.E01.CTRL.A48=1       # failsafeDuration

# EnergyControl Commands
MASH.S.E01.CTRL.C01.Rsp=1   # SetLimit
MASH.S.E01.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.E01.CTRL.C05.Rsp=1   # SetCurrentLimits
MASH.S.E01.CTRL.C06.Rsp=1   # ClearCurrentLimits
MASH.S.E01.CTRL.C09.Rsp=1   # Pause
MASH.S.E01.CTRL.C0A.Rsp=1   # Resume

# ChargingSession
MASH.S.E01.CHRG.A01=1       # state
MASH.S.E01.CHRG.A02=1       # sessionId
MASH.S.E01.CHRG.A03=1       # sessionStartTime
MASH.S.E01.CHRG.A0A=1       # sessionEnergyCharged
MASH.S.E01.CHRG.A28=1       # evDemandMode
MASH.S.E01.CHRG.A46=1       # chargingMode
MASH.S.E01.CHRG.A47=1       # supportedChargingModes
MASH.S.E01.CHRG.C01.Rsp=1   # SetChargingMode

# Behavior
MASH.S.E01.CTRL.B_LIMIT_DEFAULT="unlimited"
MASH.S.E01.CTRL.B_DURATION_EXPIRY="clear"
```

### 5.2 Advanced V2H EVSE with Signals

```
# MASH PICS File
# Device: ExampleCorp WallBox V2H Pro
# Version: 1.0
# Date: 2026-01-31

# Protocol Support
MASH.S=1
MASH.S.VERSION=1.0

# Endpoint Declarations
MASH.S.E01=EV_CHARGER

# Feature Flags
MASH.S.E01.CTRL.F00=1       # CORE
MASH.S.E01.CTRL.F01=1       # FLEX
MASH.S.E01.CTRL.F03=1       # EMOB
MASH.S.E01.CTRL.F04=1       # SIGNALS
MASH.S.E01.CTRL.F06=1       # PLAN
MASH.S.E01.CTRL.F09=1       # ASYMMETRIC
MASH.S.E01.CTRL.F0A=1       # V2X

# Electrical
MASH.S.E01.ELEC.A01=1       # phaseCount (value: 3)
MASH.S.E01.ELEC.A02=1       # phaseMapping
MASH.S.E01.ELEC.A03=1       # nominalVoltage
MASH.S.E01.ELEC.A04=1       # nominalFrequency
MASH.S.E01.ELEC.A05=1       # supportedDirections (BIDIRECTIONAL)
MASH.S.E01.ELEC.A0A=1       # nominalMaxConsumption
MASH.S.E01.ELEC.A0B=1       # nominalMaxProduction
MASH.S.E01.ELEC.A0D=1       # maxCurrentPerPhase
MASH.S.E01.ELEC.A0F=1       # supportsAsymmetric (BIDIRECTIONAL)

# EnergyControl Attributes
MASH.S.E01.CTRL.A01=1       # deviceType (EVSE)
MASH.S.E01.CTRL.A02=1       # controlState
MASH.S.E01.CTRL.A0A=1       # acceptsLimits = true
MASH.S.E01.CTRL.A0B=1       # acceptsCurrentLimits = true
MASH.S.E01.CTRL.A0C=1       # acceptsSetpoints = true
MASH.S.E01.CTRL.A0D=1       # acceptsCurrentSetpoints = true (V2X)
MASH.S.E01.CTRL.A0E=1       # isPausable = true
MASH.S.E01.CTRL.A14=1       # effectiveConsumptionLimit
MASH.S.E01.CTRL.A15=1       # myConsumptionLimit
MASH.S.E01.CTRL.A16=1       # effectiveProductionLimit
MASH.S.E01.CTRL.A17=1       # myProductionLimit
MASH.S.E01.CTRL.A1E=1       # effectiveCurrentLimitsConsumption
MASH.S.E01.CTRL.A1F=1       # myCurrentLimitsConsumption
MASH.S.E01.CTRL.A20=1       # effectiveCurrentLimitsProduction
MASH.S.E01.CTRL.A21=1       # myCurrentLimitsProduction
MASH.S.E01.CTRL.A28=1       # effectiveConsumptionSetpoint
MASH.S.E01.CTRL.A29=1       # myConsumptionSetpoint
MASH.S.E01.CTRL.A2A=1       # effectiveProductionSetpoint
MASH.S.E01.CTRL.A2B=1       # myProductionSetpoint
MASH.S.E01.CTRL.A32=1       # effectiveCurrentSetpointsConsumption (V2X)
MASH.S.E01.CTRL.A33=1       # myCurrentSetpointsConsumption (V2X)
MASH.S.E01.CTRL.A34=1       # effectiveCurrentSetpointsProduction (V2X)
MASH.S.E01.CTRL.A35=1       # myCurrentSetpointsProduction (V2X)
MASH.S.E01.CTRL.A3C=1       # flexibility (FLEX)
MASH.S.E01.CTRL.A46=1       # failsafeConsumptionLimit
MASH.S.E01.CTRL.A47=1       # failsafeProductionLimit
MASH.S.E01.CTRL.A48=1       # failsafeDuration

# EnergyControl Commands
MASH.S.E01.CTRL.C01.Rsp=1   # SetLimit
MASH.S.E01.CTRL.C02.Rsp=1   # ClearLimit
MASH.S.E01.CTRL.C03.Rsp=1   # SetSetpoint
MASH.S.E01.CTRL.C04.Rsp=1   # ClearSetpoint
MASH.S.E01.CTRL.C05.Rsp=1   # SetCurrentLimits
MASH.S.E01.CTRL.C06.Rsp=1   # ClearCurrentLimits
MASH.S.E01.CTRL.C07.Rsp=1   # SetCurrentSetpoints (V2X)
MASH.S.E01.CTRL.C08.Rsp=1   # ClearCurrentSetpoints (V2X)
MASH.S.E01.CTRL.C09.Rsp=1   # Pause
MASH.S.E01.CTRL.C0A.Rsp=1   # Resume

# ChargingSession (complete V2X)
MASH.S.E01.CHRG.A01=1       # state
MASH.S.E01.CHRG.A02=1       # sessionId
MASH.S.E01.CHRG.A03=1       # sessionStartTime
MASH.S.E01.CHRG.A04=1       # sessionEndTime
MASH.S.E01.CHRG.A0A=1       # sessionEnergyCharged
MASH.S.E01.CHRG.A0B=1       # sessionEnergyDischarged (V2X)
MASH.S.E01.CHRG.A14=1       # evIdentifications
MASH.S.E01.CHRG.A1E=1       # evStateOfCharge
MASH.S.E01.CHRG.A1F=1       # evBatteryCapacity
MASH.S.E01.CHRG.A28=1       # evDemandMode
MASH.S.E01.CHRG.A29=1       # evMinEnergyRequest
MASH.S.E01.CHRG.A2A=1       # evMaxEnergyRequest
MASH.S.E01.CHRG.A2B=1       # evTargetEnergyRequest
MASH.S.E01.CHRG.A2C=1       # evDepartureTime
MASH.S.E01.CHRG.A32=1       # evMinDischargingRequest
MASH.S.E01.CHRG.A33=1       # evMaxDischargingRequest
MASH.S.E01.CHRG.A34=1       # evDischargeBelowTargetPermitted
MASH.S.E01.CHRG.A46=1       # chargingMode
MASH.S.E01.CHRG.A47=1       # supportedChargingModes
MASH.S.E01.CHRG.A48=1       # surplusThreshold
MASH.S.E01.CHRG.A50=1       # startDelay
MASH.S.E01.CHRG.A51=1       # stopDelay
MASH.S.E01.CHRG.C01.Rsp=1   # SetChargingMode

# Signals
MASH.S.E01.SIG.A01=1        # activeSignals
MASH.S.E01.SIG.A02=1        # signalCount
MASH.S.E01.SIG.A0A=1        # lastReceivedSignalId
MASH.S.E01.SIG.A0B=1        # signalStatus
MASH.S.E01.SIG.C01.Rsp=1    # SendSignal
MASH.S.E01.SIG.C02.Rsp=1    # ClearSignals

# Behavior (endpoint-scoped)
MASH.S.E01.CTRL.B_LIMIT_DEFAULT="unlimited"
MASH.S.E01.CTRL.B_PAUSED_AUTO_RESUME=0
MASH.S.E01.CTRL.B_DURATION_EXPIRY="clear"

# Behavior (device-level)
MASH.S.SUB.B_HEARTBEAT_CONTENT="full"
MASH.S.SUB.B_COALESCE="last_value"
```

---

## 6. PICS Validation

### 6.1 Validation Rules

A valid PICS file MUST satisfy:

1. **Protocol declaration**: `MASH.S=1` or `MASH.C=1` must be present
2. **Endpoint declarations**: Each application feature code must be on a declared endpoint (`MASH.S.E<xx>=<type>`)
3. **Feature consistency**: For each endpoint with a feature, all mandatory attributes for that feature must be declared
4. **Flag dependencies**: Feature flag dependencies must be satisfied per endpoint (e.g., V2X requires EMOB)
5. **Command consistency**: If attribute `acceptsFoo=true`, corresponding `SetFoo` command must be declared on that endpoint
6. **Endpoint type conformance** (EPT-001): Each endpoint's attributes must satisfy the mandatory/recommended requirements from `endpoint-conformance.yaml` (DEC-053)

### 6.2 Validation Algorithm

```python
def validate_pics(pics: dict) -> list[str]:
    errors = []

    # Check protocol declaration
    if not pics.get("MASH.S") and not pics.get("MASH.C"):
        errors.append("Missing protocol declaration (MASH.S or MASH.C)")

    # Iterate per endpoint (DEC-054)
    for ep_id, ep_type in pics.endpoints():
        prefix = f"MASH.S.E{ep_id:02X}"

        # Check feature flag dependencies
        if pics.get(f"{prefix}.CTRL.F0A"):  # V2X
            if not pics.get(f"{prefix}.CTRL.F03"):  # EMOB
                errors.append(f"Endpoint {ep_id} ({ep_type}): V2X requires EMOB")

        # Check mandatory attributes per feature
        if pics.get(f"{prefix}.CTRL"):
            mandatory_ctrl = ["A01", "A02", "A0A", "A0B", "A0C", "A0E", "A46", "A48"]
            for attr in mandatory_ctrl:
                if not pics.get(f"{prefix}.CTRL.{attr}"):
                    errors.append(f"Endpoint {ep_id} ({ep_type}): missing {prefix}.CTRL.{attr}")

        # Check command consistency
        if pics.get(f"{prefix}.CTRL.A0A"):  # acceptsLimits
            if not pics.get(f"{prefix}.CTRL.C01.Rsp"):  # SetLimit
                errors.append(f"Endpoint {ep_id} ({ep_type}): acceptsLimits requires SetLimit")

        # EPT-001: Endpoint type conformance (DEC-053)
        conformance = ENDPOINT_CONFORMANCE.get(ep_type)
        if conformance:
            for feature, reqs in conformance.items():
                if pics.get(f"{prefix}.{feature}"):
                    for attr in reqs.mandatory:
                        if not pics.get(f"{prefix}.{feature}.{attr}"):
                            errors.append(f"Endpoint {ep_id} ({ep_type}): "
                                          f"missing mandatory {feature}.{attr}")

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

Each test case declares its PICS requirements using endpoint-scoped codes:

```yaml
test_case: TC-CTRL-001
description: Verify SetLimit command
pics_requirements:
  - MASH.S.E01.CTRL=1
  - MASH.S.E01.CTRL.A0A=1       # acceptsLimits
  - MASH.S.E01.CTRL.C01.Rsp=1   # SetLimit
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
| [Pairing/Connection PICS Registry](pics/pairing-connection-registry.md) | PICS codes for pairing and connection layer |

### Example PICS Files

| Example | Description |
|---------|-------------|
| [Minimal Device](pics/examples/minimal-device-pairing.pics) | Constrained device with mandatory features only |
| [Full-Featured Device](pics/examples/full-featured-device-pairing.pics) | High-end device with all optional features |
| [Local Controller](pics/examples/home-manager-controller.pics) | LOCAL zone controller |
| [Grid Controller](pics/examples/grid-operator-controller.pics) | GRID zone controller (highest priority) |
