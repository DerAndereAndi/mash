# MASH Protocol Testability Analysis

> Identifying gaps, hidden complexity, and testing requirements

**Status:** Draft Analysis
**Created:** 2025-01-25
**Based on:** Matter SDK testing approach (PICS, Test Harness, Conformance)

---

## Executive Summary

This document analyzes the MASH protocol design for **testability** and **hidden complexity**. The goal is to identify:

1. **Specification gaps** - Ambiguities that would cause interoperability failures
2. **Hidden complexity** - Areas where simple descriptions mask complex behavior
3. **Testing requirements** - What must be specified to enable conformance testing

**Key Finding:** MASH's design is generally well-structured, but lacks the formal conformance framework needed for interoperability. Following Matter's PICS/conformance approach from day one will prevent the "7,000+ implementation variations" problem that plagued EEBUS.

---

## 1. State Machine Complexity

### 1.1 ControlStateEnum + ProcessStateEnum Interaction

**Current Design:**
- ControlStateEnum: AUTONOMOUS, CONTROLLED, LIMITED, FAILSAFE, OVERRIDE (5 states)
- ProcessStateEnum: NONE, AVAILABLE, SCHEDULED, RUNNING, PAUSED, COMPLETED, ABORTED (7 states)
- Documented as "orthogonal" (independent)

**Hidden Complexity:**

| Scenario | ControlState | ProcessState | Behavior? |
|----------|--------------|--------------|-----------|
| Connection lost during running process | FAILSAFE | ??? | Does process pause or continue? |
| User override during scheduled process | OVERRIDE | SCHEDULED | Does scheduled time still apply? |
| Failsafe expires while process running | AUTONOMOUS | RUNNING | Does process continue without controller? |
| New controller connects with running process | CONTROLLED | RUNNING | Who owns the process now? |

**Gap:** The interaction matrix is not specified. Need explicit rules for what happens to ProcessState when ControlState transitions.

**Test Requirement:**
- TC-STATE-1: All valid (ControlState, ProcessState) combinations
- TC-STATE-2: State transitions when connection lost during each ProcessState
- TC-STATE-3: State transitions when new controller connects during each ProcessState

### 1.2 FAILSAFE Timing Precision

**Current Design:**
- Connection loss detected via TCP/TLS layer
- Device enters FAILSAFE, applies failsafe limits
- After `failsafeDuration` expires, transitions to AUTONOMOUS

**Hidden Complexity:**

1. **Detection timing**: How quickly must device detect connection loss?
   - TCP keepalive? TLS layer? Application ping/pong?
   - Current spec says "3 missed pongs" but that's 90+ seconds

2. **Failsafe limit application**: Immediate or gradual?
   - Safety-critical for grid scenarios

3. **Clock accuracy**: What if device clock drifts?
   - 24h failsafe with 1% drift = 14+ minutes error

4. **Reconnection race**: What if controller reconnects just as AUTONOMOUS transition happens?

**Gap:** Need precise timing requirements and tolerance specifications.

**Test Requirement:**
- TC-FAILSAFE-1: Connection loss detection timing (measure actual delay)
- TC-FAILSAFE-2: Failsafe limit application timing
- TC-FAILSAFE-3: Failsafe duration accuracy
- TC-FAILSAFE-4: Reconnection during FAILSAFE→AUTONOMOUS transition

---

## 2. Multi-Zone Resolution Complexity

### 2.1 Limit Resolution Edge Cases

**Current Design:**
- "Most restrictive wins" - min(all zone limits)
- Per-phase resolution for current limits

**Hidden Complexity:**

| Scenario | Zone 1 Limit | Zone 2 Limit | Expected Result | Ambiguity |
|----------|--------------|--------------|-----------------|-----------|
| Same value | 5000W | 5000W | 5000W | Which zone is "active"? |
| One zone clears | 5000W | (cleared) | 5000W? 0? unlimited? | What's the default? |
| All zones clear | (cleared) | (cleared) | ??? | Is device unlimited or 0? |
| Duration expires | 5000W (expired) | 6000W | 6000W? | Auto-clear on expiry? |
| Negative limit? | -3000W | 5000W | ??? | Is negative valid for consumption? |

**Gap:** Default values, edge cases, and expiration behavior not specified.

**Test Requirement:**
- TC-LIMIT-1: Tie-breaking when zones set identical limits
- TC-LIMIT-2: Behavior when zone clears limit (explicit vs default)
- TC-LIMIT-3: Behavior when all zones clear limits
- TC-LIMIT-4: Automatic clearing on duration expiry
- TC-LIMIT-5: Invalid limit value handling

### 2.2 Setpoint Resolution

**Current Design:**
- "Highest priority zone wins"
- Priority: GRID > LOCAL

**Hidden Complexity:**

| Scenario | Problem |
|----------|---------|
| Same priority | Two LOCAL zones - who wins? |
| Higher priority clears | Does lower priority's setpoint become active? |
| Setpoint exceeds limit | Limit constrains setpoint - but what's reported? |
| Conflicting directions | Consumption setpoint + production setpoint from different zones |

**Gap:** Priority tie-breaking and setpoint/limit interaction details missing.

**Test Requirement:**
- TC-SETPOINT-1: Same-priority zone tie-breaking
- TC-SETPOINT-2: Setpoint activation when higher priority clears
- TC-SETPOINT-3: Setpoint vs limit interaction (capping behavior)
- TC-SETPOINT-4: Multi-direction setpoint handling

### 2.3 Per-Phase Resolution Complexity

**Current Design:**
- Phase current limits: map {PhaseEnum → int64 mA}
- Resolution: min per phase across all zones

**Hidden Complexity:**
- 3 phases × 2 directions × N zones = 6N values to track
- What if zones specify different subsets of phases?
- Phase mapping (device phase A → grid phase L2) adds translation layer

**Example Problem:**
```
Zone 1: {A: 16A, B: 16A, C: 16A}
Zone 2: {A: 10A} // Only specifies phase A

Result for phase B? Is it:
- 16A (Zone 1's value, Zone 2 didn't specify)
- 0A (Zone 2 implicitly set 0)
- Unlimited (Zone 2 only cares about A)
```

**Gap:** Partial phase specification behavior undefined.

**Test Requirement:**
- TC-PHASE-1: Partial phase specification handling
- TC-PHASE-2: Phase mapping consistency between Electrical and EnergyControl
- TC-PHASE-3: Asymmetric limit combinations

---

## 3. Timing and Ordering Complexity

### 3.1 Subscription Behavior

**Current Design:**
- minInterval: "don't notify faster than this" (coalescing)
- maxInterval: "notify at least this often" (heartbeat)
- Re-establish on reconnect

**Hidden Complexity:**

| Scenario | Question |
|----------|----------|
| Rapid changes | 10 changes in 100ms with minInterval=1000ms - what's reported? |
| No changes | maxInterval heartbeat - send current value or empty notification? |
| Multiple attributes | One changes, others don't - what's in notification? |
| Value bounces back | A→B→A within minInterval - is notification sent? |
| Invalid intervals | minInterval > maxInterval - error or auto-correct? |
| Zero intervals | minInterval=0 - every change? maxInterval=0 - continuous? |

**Gap:** Coalescing semantics, heartbeat content, and interval validation not specified.

**Test Requirement:**
- TC-SUB-1: Change coalescing within minInterval
- TC-SUB-2: Heartbeat notification content at maxInterval
- TC-SUB-3: Multi-attribute notification content
- TC-SUB-4: Value bounce-back handling
- TC-SUB-5: Invalid interval handling
- TC-SUB-6: Subscription re-establishment on reconnect

### 3.2 Duration Parameter Expiration

**Current Design:**
- SetLimit, SetCurrentLimits, SetSetpoint all have optional `duration` parameter
- 0 = indefinite

**Hidden Complexity:**

| Question | Answer needed |
|----------|---------------|
| When does timer start? | On command receipt? On response? |
| What happens on expiry? | Auto-clear? Revert to previous? Notification? |
| Timer persistence | Does timer survive reconnection? |
| Multiple durations | Zone 1: 60s, Zone 2: 30s - how tracked? |

**Gap:** Timer semantics completely unspecified.

**Test Requirement:**
- TC-DURATION-1: Timer start point
- TC-DURATION-2: Expiry behavior (clearing mechanism)
- TC-DURATION-3: Expiry notification
- TC-DURATION-4: Timer behavior across reconnection
- TC-DURATION-5: Multiple concurrent duration timers

### 3.3 Event Ordering

**Current Design:**
- Events have monotonic eventNumber
- Missed events detectable by gaps

**Hidden Complexity:**

| Scenario | Question |
|----------|----------|
| Event buffer overflow | What happens when buffer full? Drop oldest? Newest? |
| Buffer size | How many events must device buffer? |
| Cross-reconnect | Are event numbers preserved? |
| Multi-endpoint | Are event numbers global or per-endpoint? |

**Gap:** Event buffer behavior and numbering scope undefined.

**Test Requirement:**
- TC-EVENT-1: Event buffer overflow handling
- TC-EVENT-2: Minimum buffer size requirement
- TC-EVENT-3: Event number persistence across reconnection
- TC-EVENT-4: Event number scope (global vs per-endpoint)

---

## 4. Feature Dependency Complexity

### 4.1 FeatureMap Bit Dependencies

**Current Design:**
```
bit 0: CORE       bit 4: SIGNALS    bit 8: FORECAST
bit 1: FLEX       bit 5: TARIFF     bit 9: ASYMMETRIC
bit 2: BATTERY    bit 6: PLAN       bit 10: V2X
bit 3: EMOB       bit 7: PROCESS
```

**Hidden Complexity:**

| Combination | Valid? | Why? |
|-------------|--------|------|
| V2X without EMOB | ??? | V2X is EV-specific, but EMOB is also EV-specific |
| ASYMMETRIC without phase info | ??? | Needs Electrical.phaseCount > 1 |
| FORECAST without FLEX | ??? | Forecasts inform flexibility |
| PROCESS without control | ??? | Process scheduling needs EnergyControl |
| BATTERY on non-BATTERY endpoint | ??? | EndpointType constraint? |

**Gap:** No conformance rules specifying valid feature combinations.

**Test Requirement:**
- Define conformance rules for each feature bit
- TC-CONFORM-1: Validate featureMap consistency with EndpointType
- TC-CONFORM-2: Validate feature bit dependencies
- TC-CONFORM-3: Validate attribute presence matches feature bits

### 4.2 Attribute Conformance

**Current Design:**
- Discovery doc mentions "Feature-Dependent Attribute Conformance"
- Table shows some dependencies

**Hidden Complexity:**
- Missing: Complete conformance table for ALL attributes
- Missing: Conditional conformance (e.g., "mandatory if X > 0")
- Missing: Choice conformance (e.g., "at least one of A, B, C")

**Example from EnergyControl:**
```
Attribute 60: flexibility    - Optional if FLEX
Attribute 61: forecast       - Optional if FORECAST
Attribute 80: processState   - ??? if PROCESS
Attribute 81: optionalProcess - ??? if PROCESS
```

Is processState mandatory or optional when PROCESS flag is set?

**Gap:** Need complete conformance specification per Matter's model.

**Test Requirement:**
- Create full attribute conformance table (like Matter's XML conformance)
- TC-ATTR-*: Validate each attribute's presence against conformance rules

---

## 5. Protocol Encoding Complexity

### 5.1 CBOR Encoding Precision

**Current Design:**
- Integer keys for compactness
- Optional fields represented as nullable

**Hidden Complexity:**

| Scenario | Question |
|----------|----------|
| Missing vs null | Is `{1: 5000}` different from `{1: 5000, 2: null}`? |
| Unknown keys | Device receives key 99 - ignore? error? |
| Key ordering | Must keys be in order? |
| Integer range | int64 specified - what about values > 2^53 (JS safe integer)? |
| Floating point | Any attributes use float? Precision requirements? |

**Gap:** CBOR encoding rules not fully specified.

**Test Requirement:**
- TC-CBOR-1: Missing vs null field handling
- TC-CBOR-2: Unknown key handling (forward compatibility)
- TC-CBOR-3: Key ordering requirements
- TC-CBOR-4: Integer boundary handling
- TC-CBOR-5: Message size limit enforcement

### 5.2 Array Size Limits

**Current Design:**
- ForecastSlot: "max 10"
- endpoints array: no stated limit
- Per-phase maps: implicitly max 3

**Hidden Complexity:**
- What happens if device sends 11 ForecastSlots?
- What's the maximum number of endpoints?
- What's the maximum number of subscriptions?

**Gap:** Resource limits not comprehensively specified.

**Test Requirement:**
- TC-LIMIT-1: Array overflow handling
- TC-LIMIT-2: Maximum endpoint count
- TC-LIMIT-3: Maximum subscription count
- TC-LIMIT-4: Maximum message size handling

---

## 6. Security and Commissioning Complexity

### 6.1 SPAKE2+ Implementation

**Current Design:**
- Uses SPAKE2+ for commissioning
- 8-digit setup code (~27 bits entropy)

**Hidden Complexity:**
- SPAKE2+ has parameters (group, hash function) - which ones?
- Iteration count for key derivation?
- Salt handling?
- Error handling on verification failure?
- Timeout on idle commissioning session?

**Gap:** Cryptographic parameters not specified.

**Test Requirement:**
- TC-SPAKE-1: Parameter negotiation/validation
- TC-SPAKE-2: Failed verification handling
- TC-SPAKE-3: Commissioning session timeout
- TC-SPAKE-4: Replay attack resistance

### 6.2 Certificate Chain Validation

**Current Design:**
- Optional manufacturer CA
- Zone CA issues operational certs
- 1-year validity, auto-renewal

**Hidden Complexity:**
- Certificate format (X.509 v3? custom?)
- Which extensions required/forbidden?
- Path length constraints?
- Key usage constraints?
- Clock skew tolerance for validity checking?

**Gap:** Certificate profile not specified.

**Test Requirement:**
- TC-CERT-1: Certificate format validation
- TC-CERT-2: Chain validation rules
- TC-CERT-3: Renewal timing and process
- TC-CERT-4: Revocation handling (RemoveZone)

### 6.3 Multi-Zone Commissioning

**Current Design:**
- Device supports up to 2 zones (one per zone type)
- Each zone has independent CA

**Hidden Complexity:**
- What happens when 6th zone tries to commission?
- Can existing zone be removed without device consent?
- What if two zones try to commission simultaneously?

**Gap:** Zone management edge cases not specified.

**Test Requirement:**
- TC-ZONE-1: Maximum zone limit enforcement
- TC-ZONE-2: Concurrent commissioning handling
- TC-ZONE-3: Zone removal procedure
- TC-ZONE-4: Zone priority conflicts

---

## 7. Feature Interaction Complexity

### 7.1 Signals + EnergyControl Interaction

**Current Design:**
- Signals: time-slotted prices, limits, forecasts (IN)
- EnergyControl: immediate limits, setpoints (IN)
- Plan: device's intended behavior (OUT)

**Hidden Complexity:**

| Scenario | Question |
|----------|----------|
| Immediate limit + Signal limit | Which takes precedence? |
| Signal slot boundary | What happens at slot transition? Notification? |
| Overlapping signals | Two zones send different signals - merge? override? |
| Signal expiry | Slot ends, no next slot - what limit applies? |

**Gap:** Signal/EnergyControl precedence rules undefined.

**Test Requirement:**
- TC-SIGNAL-1: Immediate vs scheduled limit precedence
- TC-SIGNAL-2: Slot boundary behavior
- TC-SIGNAL-3: Multi-zone signal handling
- TC-SIGNAL-4: Signal expiry handling

### 7.2 Dynamic Electrical Feature

**Current Design:**
- Electrical updates when EV connects (reflects system capability)
- EnergyControl reflects policy

**Hidden Complexity:**
- When exactly does Electrical update? (plug-in? communication start? PWM negotiation?)
- Does this trigger subscription notification?
- What if EV capabilities change mid-session? (ISO 15118 renegotiation)
- Order of updates: Electrical first, then EnergyControl?

**Gap:** Dynamic update timing and notification behavior undefined.

**Test Requirement:**
- TC-DYN-1: Electrical update timing on EV connect
- TC-DYN-2: Subscription notification on dynamic update
- TC-DYN-3: Mid-session capability change handling

### 7.3 ChargingSession + EnergyControl

**Current Design:**
- ChargingSession: EV demands, session state
- EnergyControl: limits from controller

**Hidden Complexity:**
- evMinEnergyRequest vs effectiveConsumptionLimit - which constrains?
- chargingMode (PV_SURPLUS_ONLY) vs explicit limit - interaction?
- startDelay/stopDelay vs immediate Pause command - conflict resolution?

**Gap:** Feature interaction semantics not fully specified.

**Test Requirement:**
- TC-EV-1: EV demand vs controller limit interaction
- TC-EV-2: Charging mode vs explicit limit precedence
- TC-EV-3: Delay vs immediate command conflict

---

## 8. Error Handling Complexity

### 8.1 Error Code Semantics

**Current Design:**
12 error codes defined (SUCCESS through TIMEOUT)

**Hidden Complexity:**

| Error Code | When exactly used? | Connection impact? | Retriable? |
|------------|-------------------|-------------------|------------|
| INVALID_ENDPOINT | Endpoint 99 requested | Keep open | Yes (with valid EP) |
| INVALID_ATTRIBUTE | Attribute 999 read | Keep open | Yes |
| CONSTRAINT_ERROR | Value out of range | Keep open | Yes (with valid value) |
| BUSY | Device processing | Keep open | Yes (after delay) |
| TIMEOUT | ??? | Close? | ??? |

**Gap:** Error semantics, connection behavior, and retry guidance missing.

**Test Requirement:**
- TC-ERROR-*: Each error code's trigger conditions
- TC-ERROR-CONN: Connection state after each error
- TC-ERROR-RETRY: Retriability of each error

### 8.2 Reconnection Behavior

**Current Design:**
- Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s, 60s (max)
- Reset on successful connection

**Hidden Complexity:**
- What constitutes "successful connection"? TLS handshake? First message?
- What if connection succeeds but then fails immediately?
- Jitter recommended to avoid thundering herd?

**Gap:** Reconnection success criteria and jitter not specified.

**Test Requirement:**
- TC-RECONN-1: Backoff timing accuracy
- TC-RECONN-2: Success criteria definition
- TC-RECONN-3: Rapid reconnect/disconnect handling

---

## 9. Discovery Complexity

### 9.1 mDNS Record Format

**Current Design:**
- Service type: `_mash._tcp.local`
- TXT records with various keys

**Hidden Complexity:**
- TXT record size limit (DNS 255 bytes per string)
- Multiple TXT records vs single concatenated?
- Character encoding (UTF-8? ASCII only?)
- Update frequency when state changes?

**Gap:** mDNS implementation details incomplete.

**Test Requirement:**
- TC-MDNS-1: TXT record format compliance
- TC-MDNS-2: Record update on state change
- TC-MDNS-3: Character encoding handling

### 9.2 QR Code Format

**Current Design:**
```
MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
```

**Hidden Complexity:**
- Numeric vs hex format for each field?
- Leading zeros preserved?
- Maximum total length?
- Error correction level for QR?

**Gap:** QR code format not precisely specified.

**Test Requirement:**
- TC-QR-1: Format parsing edge cases
- TC-QR-2: Invalid format handling
- TC-QR-3: QR error correction requirements

---

## 10. Recommended PICS Structure for MASH

Following Matter's approach, MASH should define PICS codes:

### 10.1 PICS Code Format

```
MASH.<Side>.<Type><ID>[.<Qualifier>]

Side: S (Server/Device), C (Client/Controller)
Type: F (Feature), A (Attribute), C (Command), E (Event)
ID: Hex ID from spec
Qualifier: Rsp (accepts), Tx (generates)
```

### 10.2 Example PICS Codes

```
# Feature presence
MASH.S=1                      # Device supports MASH protocol
MASH.S.ELEC=1                 # Electrical feature present
MASH.S.MEAS=1                 # Measurement feature present
MASH.S.CTRL=1                 # EnergyControl feature present

# Feature flags
MASH.S.CTRL.F00=1             # CORE flag set
MASH.S.CTRL.F03=1             # EMOB flag set
MASH.S.CTRL.F09=1             # ASYMMETRIC flag set

# Attributes
MASH.S.CTRL.A0001=1           # deviceType attribute present
MASH.S.CTRL.A0014=1           # isPausable attribute present

# Commands
MASH.S.CTRL.C01.Rsp=1         # SetLimit command accepted
MASH.S.CTRL.C09.Rsp=1         # Pause command accepted

# Events
MASH.S.CTRL.E01=1             # LimitChanged event supported
```

### 10.3 Conformance Rules

Define in XML format (like Matter):

```xml
<feature bit="9" name="ASYMMETRIC">
  <mandatoryConform>
    <attribute name="phaseCount">
      <greaterTerm value="1"/>
    </attribute>
  </mandatoryConform>
</feature>

<attribute id="0x001E" name="effectiveCurrentLimitsConsumption">
  <mandatoryConform>
    <feature name="ASYMMETRIC"/>
  </mandatoryConform>
</attribute>
```

---

## 11. Recommended Test Categories

### 11.1 Conformance Tests
- Validate PICS declarations match actual device capabilities
- Validate attribute presence matches feature flags
- Validate command acceptance matches declared capabilities

### 11.2 Interoperability Tests
- Multi-vendor controller + device communication
- Multi-zone scenarios with different controller types
- Protocol version negotiation (future)

### 11.3 State Machine Tests
- All valid state transitions
- Invalid transition handling
- State persistence across reconnection

### 11.4 Timing Tests
- Subscription interval compliance
- Failsafe duration accuracy
- Reconnection backoff compliance

### 11.5 Edge Case Tests
- Resource exhaustion (max subscriptions, max zones)
- Invalid input handling
- Concurrent operation handling

### 11.6 Security Tests
- SPAKE2+ protocol compliance
- Certificate validation
- Zone isolation

---

## 12. Next Steps

### 12.1 Immediate Actions

1. **Define PICS format** - Create MASH PICS specification
2. **Write conformance rules** - XML conformance for each feature
3. **Resolve specification gaps** - Address each "Gap" identified above
4. **Create test framework** - Python-based, similar to Matter TH

### 12.2 Testing Infrastructure

1. **Reference implementation** - Canonical MASH implementation
2. **Test harness** - Automated test execution
3. **PICS generator** - Auto-fill from device capabilities
4. **PICS validator** - Verify PICS matches actual behavior

### 12.3 Documentation

1. **Test specification** - Formal test case definitions
2. **Implementer's guide** - How to implement MASH correctly
3. **Conformance guide** - How to complete PICS

---

## Appendix: Gap Summary

| Area | Gap Count | Severity |
|------|-----------|----------|
| State Machines | 4 | High |
| Multi-Zone Resolution | 6 | High |
| Timing/Ordering | 8 | Medium |
| Feature Dependencies | 3 | High |
| Protocol Encoding | 5 | Medium |
| Security | 4 | High |
| Feature Interaction | 5 | Medium |
| Error Handling | 3 | Medium |
| Discovery | 3 | Low |
| **Total** | **41** | |

**Recommendation:** Address all "High" severity gaps before implementation begins. "Medium" gaps can be resolved during implementation with test-driven refinement.
