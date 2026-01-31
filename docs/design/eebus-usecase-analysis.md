# EEBUS Use Case Analysis

> Systematic mapping of EEBUS use cases to MASH protocol features, with gap analysis.

**Status:** Complete
**Last Updated:** 2026-01-31

---

## Batch 1: Grid / Power Management

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 1.1 | LPC | Limit power consumption of a controllable system to stabilize the public grid | ControllableSystem, EnergyGuard (CEM or ControlBox) | LoadControl, DeviceConfiguration, DeviceDiagnosis, ElectricalConnection, Measurement | S1: Control active power consumption limit (M), S2: Failsafe values (M), S3: Heartbeat (M), S4: Constraints (M) | **EnergyControl** - limits, failsafe, controlState | Validated. Good coverage. |
| 1.2 | LPP | Limit power production of a controllable system (mirror of LPC for production side) | ControllableSystem, EnergyGuard | LoadControl, DeviceConfiguration, DeviceDiagnosis, ElectricalConnection, Measurement | S1: Control active power production limit (M), S2: Failsafe values (M), S3: Heartbeat (M), S4: Constraints (M) | **EnergyControl** - production limits, failsafe | Validated. Symmetric with LPC. |
| 1.3 | MPC | Monitor power, current, energy, voltage, frequency of an appliance | MonitoredUnit, MonitoringAppliance | Measurement, ElectricalConnection | S1: Monitor power (M), S2: Monitor energy (O), S3: Monitor current (O), S4: Monitor voltage (O), S5: Monitor frequency (O) | **Measurement** | Validated. Good coverage. |
| 1.4 | MGCP | Monitor total power, energy at the grid connection point | GridConnectionPoint, MonitoringAppliance | Measurement, ElectricalConnection | S1: Monitor phase details (O), S2: Monitor momentary power (M), S3: Monitor total feed-in energy (M), S4: Monitor total consumed energy (M) | **Measurement** on GRID_CONNECTION endpoint | Validated. Good coverage. |
| 1.5 | MHPC | Provide historical (timestamped) power, energy, current, voltage, frequency data | MonitoredUnit, MonitoringAppliance | Measurement, ElectricalConnection, TimeSeries | S1: Monitor historical power (M), S2: Monitor historical energy (O), S3: Monitor historical current (O), S4: Monitor historical voltage (O), S5: Monitor historical frequency (O) | **Measurement** (partial) | **GAP: No historical/time-series data model in MASH.** Measurement only provides current values. |
| 1.6 | PODF | CEM forecasts building's anticipated power consumption/production over time | CEM, TransmissionBroker, EnergyBroker | TimeSeries | S1: CEM's active power forecast (M), S2: Contractual consumption nominal max (O), S3: Contractual production nominal max (O) | **Signals** (FORECAST type) + **Plan** (partial) | **GAP: PODF is CEM->Broker direction (upward). MASH Signals is controller->device (downward). No CEM-outbound forecast mechanism.** |
| 1.7 | POEN | Transmission broker sends power-over-time limit curves (consumption/production min/max, reactive power, power factor) to CEM | TransmissionBroker, CEM | TimeSeries | S1: Set active power consumption limit curve (M), S2: Set active power production limit curve (M), S3: Set reactive power setpoint curve (O), S4: Set power factor setpoint curve (O), S5: Failsafe values (M), S6: Heartbeat (M), S7: Constraints (M) | **Signals** (CONSTRAINT type) | Partial. Active power limits map well. **GAP: No reactive power or power factor in Signals slots. No POEN-specific failsafe in Signals.** |
| 1.8 | TOUT | Energy/transmission broker sends time-based incentive tables (price, CO2, renewable%) to CEM | EnergyBroker, TransmissionBroker, CEM | IncentiveTable | S1: Send incentive table with PoE+TF combined (M/O), S2: Send separate PoE incentive table (M/O), S3: Send separate TF incentive table (M/O) | **Signals** (PRICE type) + **Tariff** | Partial. Simple time-based prices map well. **GAP: TOUT supports multi-tier power-dependent pricing per slot (same as ITPCM structure). MASH Signals has only flat price/priceLevel per slot. Tariff has powerTiers but unclear if per-slot tier variation is supported.** |
| 1.9 | ITPCM | CEM sends power-dependent incentive tables; device responds with power plans (committed/preliminary) | CEM, EnergyConsumer | IncentiveTable, PowerSequences | S1: Monitor power constraints (M), S2: Committed incentive table + power plan (M), S3: Simulation: preliminary incentive table + power plan (O), S4: Unsolicited committed incentive table (O) | **Signals** + **Tariff** + **Plan** | Partial. **GAP: ITPCM has bidirectional negotiation (CEM sends incentive table, device responds with power plan, CEM may iterate). MASH Signals/Plan is unidirectional per feature. No simulation/negotiation protocol. Power tier structure needs per-slot variation.** |
| 1.10 | FLOA | CEM controls flexible load's active power setpoint within constraints; device reports remaining capacity | CEM, EnergyConsumer | DirectControl, ElectricalConnection, Measurement, DeviceConfiguration | S1: Control active power setpoint (M), S2: Monitor power constraints (M), S3: Monitor remaining capacity (M) | **EnergyControl** (setpoints) + **Measurement** | Mostly covered. **GAP: FLOA has explicit controllability indication (device says "you may/may not control me now"), supported value list (not just range), power deviation type (roundUp/roundDown/nearest), and setpoint duration (max 10 min, auto-expires). EnergyControl lacks these.** |

---

### Detailed Analysis

#### 1.1 LPC - Limitation of Power Consumption

**Goal:** Energy Guard (grid operator relay or EMS) limits the active power consumption of a Controllable System to prevent grid overload.

**Actors:**
- Energy Guard (EG) -- sends limits. Can be a ControlBox (DSO device) or a CEM.
- Controllable System (CS) -- receives and enforces limits. Can be a CEM or an appliance.

**Two instances:**
1. EG (ControlBox) -> CS (CEM): limit at grid connection point level
2. EG (CEM) -> CS (appliance): limit on individual device

**SPINE Features:**
- `LoadControl` -- limit value, limit active/inactive flag, limit direction
- `DeviceConfiguration` -- failsafe consumption active power limit, failsafe duration minimum
- `DeviceDiagnosis` -- heartbeat (keep-alive for communication monitoring)
- `ElectricalConnection` -- power consumption nominal max, contractual consumption nominal max
- `Measurement` -- actual power (via MPC dependency)

**State Machine:** Init -> Unlimited/Controlled -> Limited -> Failsafe -> Unlimited/Autonomous (with defined transitions and heartbeat-based liveness)

**MASH Mapping:** EnergyControl covers this well:
- `effectiveConsumptionLimit` / `myConsumptionLimit` = LoadControl limit value
- `controlState` (AUTONOMOUS, CONTROLLED, LIMITED, FAILSAFE) = LPC state machine
- `failsafeConsumptionLimit` + `failsafeDuration` = DeviceConfiguration failsafe
- Heartbeat = MASH Subscribe keep-alive (built into transport)

**Gaps:** None significant. LPC is the canonical use case that shaped EnergyControl's design.

---

#### 1.2 LPP - Limitation of Power Production

**Goal:** Mirror of LPC for production direction. Limits active power production (e.g., solar curtailment).

**Actors:** Same as LPC (EG, CS). Relevant for devices that produce energy (inverters, batteries discharging, V2G EVs).

**SPINE Features:** Same as LPC but for production direction.

**MASH Mapping:** EnergyControl `effectiveProductionLimit` / `myProductionLimit` + production failsafe.

**Gaps:** None. Symmetric design with LPC.

**Note:** LPC and LPP share the `failsafeDuration` data point when both are supported on the same device. MASH already has a single `failsafeDuration` attribute that serves both directions.

---

#### 1.3 MPC - Monitoring of Power Consumption

**Goal:** Monitor real-time electrical values of an appliance (power, energy, current, voltage, frequency).

**Actors:**
- Monitoring Appliance (MA) -- reads/subscribes to measurements
- Monitored Unit (MU) -- provides measurements

**Scenarios:**
1. Monitor power (M) -- total active power, phase-specific power
2. Monitor energy (O) -- consumed energy, produced energy
3. Monitor current (O) -- phase-specific AC current (rms)
4. Monitor voltage (O) -- phase-specific AC voltage
5. Monitor frequency (O) -- AC grid frequency

**SPINE Features:**
- `Measurement` -- measurement values with descriptions, constraints
- `ElectricalConnection` -- phase mapping, parameter descriptions

**MASH Mapping:** Measurement feature covers all scenarios:
- `totalActivePower`, `activePowerL1/L2/L3` = power
- `totalConsumedEnergy`, `totalProducedEnergy` = energy
- `currentL1/L2/L3` = current
- `voltageL1/L2/L3`, `voltageL1L2/L2L3/L3L1` = voltage
- `frequency` = frequency

**Gaps:** None. Well covered.

---

#### 1.4 MGCP - Monitoring of Grid Connection Point

**Goal:** Monitor power and energy at the premises' grid connection point (bidirectional: consumption from grid and feed-in to grid).

**Actors:**
- Monitoring Appliance (MA)
- Grid Connection Point (GCP) -- typically a smart meter or CEM with meter data

**Scenarios:**
1. Monitor phase details (O) -- phase-specific power
2. Monitor momentary power (M) -- total active power (positive = consuming from grid, negative = feeding into grid)
3. Monitor total feed-in energy (M) -- cumulative energy fed into grid
4. Monitor total consumed energy (M) -- cumulative energy consumed from grid

**SPINE Features:** `Measurement`, `ElectricalConnection`

**MASH Mapping:** Measurement on a GRID_CONNECTION endpoint type. All data points map directly.

**Gaps:** None. The GRID_CONNECTION endpoint type exists for this purpose.

---

#### 1.5 MHPC - Monitoring of Historical Power Consumption

**Goal:** Provide timestamped historical measurement values (power, energy, current, voltage, frequency) as a time series.

**Actors:** MonitoringAppliance, MonitoredUnit (same as MPC)

**Scenarios:**
1. Monitor historical power (M) -- total + phase-specific historical active power with timestamps
2. Monitor historical energy (O) -- consumed/produced energy over evaluation periods (start time, end time)
3. Monitor historical current (O) -- historical phase-specific current with timestamps
4. Monitor historical voltage (O) -- historical phase-specific voltage with timestamps
5. Monitor historical frequency (O) -- historical frequency with timestamps

**Key differences from MPC:**
- Each data point has a **timestamp** (when measured)
- Energy values have **start time** and **end time** (evaluation period)
- Multiple historical values are stored (time series, not just current value)
- Shared measurand identifiers with MPC (same measurement, different temporal aspect)

**SPINE Features:** `Measurement` (with TimeSeries capability), `ElectricalConnection`

**MASH Mapping:** Measurement (current values only)

**Gaps:**
- **No time-series / historical data model.** MASH Measurement provides only the latest value. There is no mechanism to store, query, or subscribe to historical data series.
- **Options to address:**
  1. Add a `historyDepth` attribute and array-of-timestamped-values to Measurement
  2. Create a separate "History" or "TimeSeries" feature
  3. Consider this out-of-scope for constrained devices (external logging)
- **Recommendation:** Option 3 for v1 -- historical logging is typically handled by the controller/cloud, not the constrained device. If needed later, a lightweight approach (option 1 with limited depth) could work.

---

#### 1.6 PODF - Power Demand Forecast

**Goal:** CEM forecasts the building's anticipated power consumption/production as a time-slotted curve and sends it to interested parties (Transmission Broker, Energy Broker).

**Actors:**
- CEM -- produces the forecast (server)
- Transmission Broker / Energy Broker -- consumes the forecast (client)

**Scenarios:**
1. CEM's active power forecast (M) -- power-over-time curve (Pavg per slot, optional Pmin/Pmax)
2. Contractual consumption nominal max (O) -- max allowed consumption
3. Contractual production nominal max (O) -- max allowed production

**Key properties:**
- Covers at least next 6 hours, up to 48 hours
- UTC timestamps, relative durations per slot
- No time gaps between slots
- Updated regularly (at least every 24h, typically every 6-12h)

**SPINE Features:** `TimeSeries` (scopeType: activePowerForecast)

**MASH Mapping:** Partially via Signals (FORECAST type has `forecastPower`, `forecastEnergy` per slot) and Plan feature.

**Gaps:**
- **Direction mismatch.** PODF is CEM sending forecasts *upward* to a broker/grid operator. In MASH, Signals flows controller->device (downward) and Plan flows device->controller (upward). PODF would map to Plan if the CEM is treated as a "device" reporting to a higher-tier controller. This works in multi-zone: a HOME_MANAGER could expose a Plan feature to a BUILDING_MANAGER or GRID_OPERATOR zone.
- **Contractual nominal max** is already in EnergyControl (`contractualConsumptionNominalMax`, `contractualProductionNominalMax`).
- **Assessment:** The multi-zone architecture may handle this naturally -- a CEM acting as "device" to a higher-tier controller reports its Plan. No new feature needed, but the direction use case should be validated.

---

#### 1.7 POEN - Power Envelope

**Goal:** Transmission Broker sends power-over-time limit curves to CEM. Up to 4 curves: consumption min/max, production min/max. Plus optional reactive power and power factor setpoint curves.

**Actors:**
- Transmission Broker -- sends envelope (server/writer)
- CEM -- receives and enforces envelope (client/reader)

**Scenarios:**
1. Set active power consumption limit curve (M) -- max consumption over time
2. Set active power production limit curve (M) -- max production over time
3. Set reactive power setpoint curve (O) -- reactive power targets over time
4. Set power factor setpoint curve (O) -- power factor targets over time
5. Failsafe values (M) -- what to do if communication lost
6. Heartbeat (M) -- liveness monitoring
7. Constraints (M) -- nominal max, contractual max

**Key properties:**
- Covers at least next 6 hours, up to 48 hours (same horizon as PODF)
- UTC start times, relative durations per slot
- Failsafe + heartbeat (like LPC/LPP but for time-series curves)

**SPINE Features:** `TimeSeries` (multiple scopeTypes for the different curves)

**MASH Mapping:** Signals (CONSTRAINT type) with `minPower`/`maxPower` per slot.

**Gaps:**
- **Reactive power and power factor** are not modeled in MASH Signals slots. These are relevant for grid stability but may be low priority for residential energy management.
- **POEN-specific failsafe/heartbeat.** Signals doesn't have its own failsafe mechanism. However, MASH transport-level keep-alive and EnergyControl failsafe may suffice since POEN limits feed into the EnergyControl state machine.
- **Four separate curves.** MASH Signals CONSTRAINT slot has `minPower`/`maxPower` but as a single signal. POEN distinguishes consumption vs production min/max (4 curves). This could be modeled as 4 separate signals with appropriate `source` tagging.
- **Assessment:** Core active power limits map well. Reactive power/power factor can be deferred. Multiple curves can use multiple Signal entries.

---

#### 1.8 TOUT - Time of Use Tariff

**Goal:** Energy/Transmission Broker sends time-based incentive tables to CEM. Incentives include price (per kWh), CO2 emission (per kWh), and renewable energy percentage.

**Actors:**
- Energy Broker (ESP) -- sends price of energy (PoE)
- Transmission Broker (DSO) -- sends transmission fee (TF)
- CEM -- receives and optimizes based on incentives

**Scenarios:**
1. Send incentive table with combined PoE + TF (M or O)
2. Send separate PoE incentive table (M or O)
3. Send separate TF incentive table (M or O)

(Scenario 1 OR Scenarios 2+3, not all three simultaneously)

**Key features:**
- Time-slotted: each slot has start time, optional end time
- Up to 5 tiers per slot (power-dependent pricing, same as ITPCM)
- Each tier can have up to 3 incentive types: absolute price, renewable%, CO2
- Tier boundaries define power ranges

**SPINE Features:** `IncentiveTable` (incentiveTableData, incentiveTableDescription, incentiveTableConstraints)

**MASH Mapping:** Signals (PRICE/INCENTIVE type) + Tariff

**Gaps:**
- **Per-slot multi-tier pricing.** TOUT supports power-dependent tiers per time slot (same IncentiveTable structure as ITPCM). MASH Signals has flat `price`/`priceLevel` per slot -- no per-slot power tiers. The Tariff feature has `powerTiers` but these are static, not time-varying.
- **Multiple incentive types per tier.** TOUT supports price + CO2 + renewable% simultaneously per tier. MASH Signals has `price`, `renewablePercent`, `co2Intensity` at slot level but not per-tier.
- **Assessment:** For simple ToU (one price per time slot), MASH works fine. For complex tiered pricing, the Tariff/Signals interaction needs refinement. This is the same gap as ITPCM below.

---

#### 1.9 ITPCM - Incentive Table Power Consumption Management

**Goal:** CEM sends power-dependent incentive tables to a flexible energy consumer. Consumer responds with a power plan. Optional simulation phase for iterative optimization.

**Actors:**
- CEM -- sends incentive tables, receives power plans
- Energy Consumer -- receives incentive tables, produces power plans

**Scenarios:**
1. Monitor power constraints (M) -- consumer reports max tiers, max slots, etc.
2. Committed incentive table + committed power plan (M) -- main operation
3. Preliminary incentive table + preliminary power plan (O) -- simulation/negotiation
4. Unsolicited committed incentive table (O) -- CEM pushes without re-negotiation

**Key features:**
- **Bidirectional negotiation:** CEM sends incentive table -> consumer responds with power plan -> CEM may iterate (especially in simulation phase)
- **Power tiers:** up to 5 tiers per slot with different prices based on power consumption level
- **Three incentive types per tier:** absolute price (currency/Wh), renewable%, CO2 (kg/Wh)
- **Power plan structure:** time-slotted with Pexp, Pmin, Pmax per slot (same structure as PODF forecast)
- **Committed vs preliminary:** committed = device will follow; preliminary = hypothetical "what if"

**SPINE Features:** `IncentiveTable`, `PowerSequences` (for power plans)

**MASH Mapping:** Signals + Tariff (incentive input) + Plan (power plan output)

**Gaps:**
- **Negotiation protocol.** ITPCM has a commit/simulate cycle: CEM sends preliminary incentive table, device responds with preliminary plan, CEM adjusts, sends committed table, device responds with committed plan. MASH has no negotiation mechanism -- Signals is write-once, Plan is read-only. This could potentially be modeled with Plan's `commitment` enum (PRELIMINARY -> COMMITTED) and the controller re-writing Signals, but the protocol flow isn't explicit.
- **Per-slot power tiers.** Same gap as TOUT -- Tariff has static tiers, Signals has flat per-slot values. The ITPCM tier model is fundamentally per-slot (tiers can change between time slots).
- **Constraints exchange.** ITPCM consumer declares constraints (maxTiers, maxSlots, maxIncentivesPerTier, etc.). MASH has no mechanism for a device to declare signal processing constraints.
- **Assessment:** This is the most complex gap in Batch 1. The core value (device receives price signals, produces a plan) works. The bidirectional negotiation and per-slot tier variation would need protocol-level support or an explicit "negotiation mode."

---

#### 1.10 FLOA - Flexible Load

**Goal:** CEM controls a flexible load's active power setpoint within constraints. Simple, direct control for devices that can adjust power consumption on command.

**Actors:**
- CEM -- sends setpoints
- Energy Consumer -- adjusts power consumption, reports constraints

**Scenarios:**
1. Control active power setpoint (M) -- CEM writes power target
2. Monitor power constraints (M) -- nominal min/max, current max
3. Monitor remaining capacity (M) -- how much more/less the device can consume

**Key features:**
- **Controllability Indication:** device explicitly signals whether CEM may control it right now (true/false)
- **Setpoint properties:** power value + duration (max 10 min, auto-expires) + activation state
- **Value constraints:** supported as range (min/max/step) OR as a discrete value list
- **Power deviation type:** how device rounds non-exact values (roundUp, roundDown, nearest)
- **Remaining capacity:** runtime indication of how much additional power the device can accept

**SPINE Features:** `DirectControl` (setpoint), `ElectricalConnection` (constraints), `Measurement` (linked measurements), `DeviceConfiguration`

**MASH Mapping:** EnergyControl (setpoints) + Measurement (actual power)

**Gaps:**
- **Controllability indication.** FLOA has an explicit flag: "you may/may not control me now." MASH EnergyControl has `controlState` and `optOutState` which partially cover this. `optOutState` could serve as the controllability indicator if the device can opt out.
- **Setpoint auto-expiry.** FLOA setpoints have a mandatory duration (max 10 min) and auto-deactivate. MASH EnergyControl setpoints are persistent until changed. Adding a `setpointDuration` field would address this.
- **Discrete value list.** FLOA allows devices to report supported power values as a list (not just a range). MASH has no equivalent.
- **Power deviation type.** How the device rounds non-exact setpoints. Minor but useful for interop.
- **Remaining capacity.** A dynamic value showing headroom. Could map to a Measurement attribute or an EnergyControl attribute.
- **Assessment:** Most FLOA functionality maps to EnergyControl+Measurement. The controllability indication is covered by existing control state/opt-out. The setpoint auto-expiry is the most notable gap but could be addressed with a single attribute addition. Discrete value lists and deviation types are niche features that could be deferred.

---

### Batch 1 SPINE-to-MASH Feature Type Mapping

| SPINE Feature | Used By | MASH Feature | Notes |
|--------------|---------|-------------|-------|
| LoadControl | LPC, LPP | EnergyControl | Limits + state machine |
| DeviceConfiguration | LPC, LPP, FLOA | EnergyControl | Failsafe values, contractual max |
| DeviceDiagnosis | LPC, LPP | Transport layer | Heartbeat = transport keep-alive |
| ElectricalConnection | LPC, LPP, MPC, MGCP, MHPC, FLOA | Electrical | Phase config, nominal values |
| Measurement | MPC, MGCP, MHPC, FLOA | Measurement | Current telemetry values |
| TimeSeries | MHPC, PODF, POEN | Signals + Plan (partial) | **Historical data gap for MHPC** |
| IncentiveTable | TOUT, ITPCM | Signals + Tariff | **Per-slot tier variation gap** |
| PowerSequences | ITPCM | Plan | Power plans |
| DirectControl | FLOA | EnergyControl | Setpoints |

### Batch 1 Gap Summary

| Priority | Gap | Affected UCs | Recommendation |
|----------|-----|-------------|----------------|
| Low | No historical time-series data model | MHPC | Defer to v2; historical logging is controller responsibility |
| Low | No reactive power / power factor in Signals | POEN (S3, S4) | Defer; niche for residential |
| Medium | CEM-outbound forecast direction | PODF | Validate multi-zone handles this (CEM as "device" to higher tier) |
| Medium | Per-slot power-dependent tiers | TOUT, ITPCM | Refine Tariff+Signals interaction to support time-varying tiers |
| Medium | ITPCM negotiation protocol (commit/simulate) | ITPCM | Design lightweight negotiation using Plan commitment levels |
| Low | Setpoint auto-expiry duration | FLOA | Add optional `setpointDuration` to EnergyControl |
| Low | Discrete value list for setpoints | FLOA | Defer; range is sufficient for most devices |
| Low | Signal processing constraints declaration | ITPCM | Defer; controller can discover via feature attributes |

---

*Batch 1 complete.*

---

## Batch 2: E-Mobility

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 2.1 | CEVC | Coordinate EV charging via incentive tables, Pmax curves, and charging plans | EV, EnergyBroker, EnergyGuard (via EVSE as middlebox) | IncentiveTable, TimeSeries, PowerSequences, ElectricalConnection | S1: EV sends demand curve (M), S2: EG sends Pmax curve (M), S3: EB sends incentive table (M), S4: EV sends charging plan (M) | **Signals** + **Tariff** + **Plan** + **ChargingSession** + **EnergyControl** | Partial. Core CEVC = ITPCM applied to EV. Same negotiation/tier gaps as ITPCM. Demand curve maps to ChargingSession energy requests. |
| 2.2 | EVCC | EV commissioning: connected/disconnected, comm standard, asymmetric support, identification, power limits, operating state | EV, CEM (via EVSE) | DeviceClassification, DeviceConfiguration, DeviceDiagnosis, ElectricalConnection, Identification | S1: EV connected (M), S2: Comm standard (M), S3: Asymmetric support (M), S4: Identification (O), S5: Manufacturer info (O), S6: Charging power limits (O), S7: Operating state (M), S8: EV disconnected (M) | **ChargingSession** (state) + **DeviceInfo** + **Electrical** | Mostly covered. **GAP: Communication standard (ISO 15118-2 vs -20 vs IEC 61851) not modeled as explicit attribute. MASH ChargingSession.evDemandMode implies it but doesn't expose it directly.** |
| 2.3 | EVSECC | EVSE commissioning: manufacturer data, operating/error state, software version, session ID | EVSE, CEM | DeviceClassification, DeviceDiagnosis | S1: Manufacturer data (M), S2: Operating state (M), S3: Software version (O), S4: Session ID (O) | **DeviceInfo** + **Status** | Validated. DeviceInfo covers manufacturer/version. Status covers operating state. ChargingSession.sessionId covers session ID. |
| 2.4 | OSCEV | CEM informs EV about self-produced current available for charging (per-phase or total) | EV, CEM | LoadControl, ElectricalConnection | S1: CEM informs EV about self-produced current per-phase (M), S2: Failsafe (M), S3: Heartbeat (M), S4: CEM informs EV about self-produced current total (O) | **EnergyControl** (current limits/setpoints) | Mostly covered. OSCEV is essentially current-based setpoints with per-phase support. **GAP: OSCEV semantics are "recommended current" (soft setpoint), not "max current" (hard limit). EnergyControl has both limit and setpoint concepts, so this maps to setpoints. Minor: OSCEV auto-deactivates S1 limits when S4 is written; MASH has no such inter-command coupling.** |
| 2.5 | OPEV | Prevent fuse trips by curtailing EV charging current in real-time; per-phase current limits | EV, EnergyGuard | LoadControl, ElectricalConnection, DeviceDiagnosis | S1: EG curtails charging current (M) -- includes per-phase current limits, failsafe, heartbeat | **EnergyControl** (current limits) | Validated. Maps directly to `myCurrentLimitsConsumption` per-phase. Failsafe + heartbeat covered. |
| 2.6 | EVCEM | Measure EV charging current, power, and session energy | EV, CEM | Measurement, ElectricalConnection | S1: Measure charging current (O), S2: Measure charging power (O), S3: Measure charged energy (O) -- at least one mandatory | **Measurement** + **ChargingSession** (sessionEnergy) | Validated. Phase-specific current/power maps to Measurement. Session energy maps to ChargingSession.sessionEnergyCharged. |
| 2.7 | EVSOC | Monitor EV battery state of charge, capacity, range, estimated times | EV, MonitoringAppliance | Measurement | S1: Monitor SoC (M) -- current/min/target SoC, S2: Monitor max useable energy capacity (M), S3: Monitor current travel range (O), S4: Monitor remaining times to SoCs (O) | **ChargingSession** (evStateOfCharge, evBatteryCapacity, estimatedTimes) | Mostly covered. **GAP: `evMinSoC` (minimum SoC to charge ASAP), `evTargetSoC` (user-defined target SoC %), and `evCurrentTravelRange` (meters) are not in ChargingSession. ChargingSession has energy requests (mWh) but not SoC percentages for min/target. Could add `evMinStateOfCharge`, `evTargetStateOfCharge`, `evTravelRange`.** |
| 2.8 | EVCS | Energy Broker sends charging session cost/energy summary (total, self-produced, grid) to EVSE | EnergyBroker, EVSE | Bill | S1: EB sends charging summary to EVSE (M) -- total cost, total energy, self-produced cost/energy, grid cost/energy | None directly | **GAP: No billing/cost summary feature in MASH.** EVCS is informational (not for actual billing). Could be a simple extension to ChargingSession or a new lightweight feature. |
| 2.9 | DBEVC | CEM dynamically controls bidirectional EV charging/discharging with power setpoints; EV reports energy requests and constraints | EV, CEM | DirectControl, ElectricalConnection, Measurement, DeviceConfiguration | S1: Control active power setpoint (M) -- symmetric/asymmetric, charge/discharge, S2: Monitor energy requests (M) -- min/max/target energy, departure time, S3: Monitor power constraints (M) -- nominal min/max charge/discharge | **EnergyControl** (setpoints) + **ChargingSession** (energy demands, V2G) | Good coverage. DBEVC setpoints map to EnergyControl consumption/production setpoints. Energy requests map to ChargingSession evMinEnergyRequest/evMaxEnergyRequest/evTargetEnergyRequest. V2G discharge constraints map to ChargingSession evMinDischargingRequest/evMaxDischargingRequest. **Minor gap: DBEVC has setpoint start/end times (scheduled setpoints) and setpoint changeability flag ("non-changeable by CEM"). EnergyControl lacks these.** |
| 2.10 | SMR | Link charging sessions to their measurement data via identifiers | MonitoringAppliance, MonitoredUnit | Measurement (identifiers) | S1: Latest session measurement relation (M), S2: Previous session measurement relation (O) | **ChargingSession** (implicit via sessionId) | Mostly covered. MASH links measurements to sessions implicitly through temporal correlation (sessionStartTime/sessionEndTime). **GAP: SMR provides explicit measurement-to-session ID mapping, useful for historical lookups. Same gap as MHPC -- historical data is controller responsibility.** |

---

### Detailed Analysis

#### 2.1 CEVC - Coordinated EV Charging

**Goal:** Optimize EV charging via incentive-based negotiation (price/CO2/renewable signals) and power limitation curves, with the EV producing a cost-optimized charging plan.

**Actors:**
- Energy Broker -- sends incentive tables (price signals)
- Energy Guard -- sends Pmax limitation curve
- EV -- receives signals, creates/sends charging plan (via EVSE as middlebox)

**Scenarios:**
1. EV sends demand curve (M) -- departure time, min/max/target energy, min/max power
2. Energy Guard sends Pmax curve (M) -- time-slotted max power limitation
3. Energy Broker sends incentive table (M) -- up to 3 tiers, up to 3 incentive types
4. EV sends charging plan (M) -- time-slotted power plan (Pmin, Pexp, Pmax)

**Key features:**
- Renegotiation: EV, EG, or EB can trigger updates at any time
- Incentive table: identical structure to ITPCM (tiers, boundaries, price/CO2/renewable)
- Charging plan: identical structure to ITPCM power plan (time-slotted Pmin/Pexp/Pmax)
- Demand curve: energy request with departure time

**SPINE Features:** `IncentiveTable`, `TimeSeries`, `PowerSequences`, `ElectricalConnection`

**MASH Mapping:**
- S1 (demand) -> ChargingSession: evMinEnergyRequest, evMaxEnergyRequest, evTargetEnergyRequest, evDepartureTime
- S2 (Pmax curve) -> Signals (CONSTRAINT type): maxPower per slot
- S3 (incentive table) -> Signals (PRICE/INCENTIVE type) + Tariff
- S4 (charging plan) -> Plan: slots with plannedPower, minPower, maxPower

**Gaps:** Same as ITPCM (Batch 1): per-slot power-dependent tiers, negotiation protocol. The demand curve (S1) is well covered by ChargingSession.

---

#### 2.2 EVCC - EV Commissioning and Configuration

**Goal:** Establish baseline EV information when connected: what it is, what it supports, what it can accept.

**Scenarios:**
1. EV connected (M) -- plug-in event
2. Communication standard (M) -- IEC 61851, ISO 15118-2, ISO 15118-20
3. Asymmetric charging support (M) -- can phases be independently controlled
4. Identification (O) -- RFID, ISO 15118 contract certificate, etc.
5. Manufacturer information (O) -- brand, model, serial
6. Charging power limits (O) -- min/max current/power per phase
7. Operating state (M) -- normal, error, sleep mode
8. EV disconnected (M) -- unplug event

**SPINE Features:** `DeviceClassification`, `DeviceConfiguration`, `DeviceDiagnosis`, `ElectricalConnection`, `Identification`

**MASH Mapping:**
- S1/S8 (connected/disconnected) -> ChargingSession.state (NOT_PLUGGED_IN, PLUGGED_IN_*)
- S3 (asymmetric) -> Electrical.supportsAsymmetric
- S4/S5 (identification, manufacturer) -> DeviceInfo, ChargingSession.evIdentifications
- S6 (power limits) -> Electrical nominal values
- S7 (operating state) -> Status.operatingState

**Gaps:**
- Communication standard (IEC 61851 vs ISO 15118-2 vs ISO 15118-20) is not explicitly exposed. ChargingSession.evDemandMode implicitly indicates it (NONE = IEC 61851, SCHEDULED = ISO 15118-2, DYNAMIC_* = ISO 15118-20), but an explicit attribute would be clearer. Low priority.

---

#### 2.3 EVSECC - EVSE Commissioning and Configuration

**Goal:** EVSE reports its identity, operating state, software version, and current session ID to CEM.

**Scenarios:**
1. Manufacturer data (M) -- brand, model, serial number
2. Operating state (M) -- normal operation, error
3. Software version (O)
4. Session ID (O) -- identifier for the current charging session

**MASH Mapping:** DeviceInfo (S1, S3) + Status (S2) + ChargingSession.sessionId (S4). Fully covered.

---

#### 2.4 OSCEV - Optimization of Self-Consumption During EV Charging

**Goal:** CEM informs EV how much self-produced current (e.g., from PV) is available for charging, enabling the EV to maximize self-consumption.

**Scenarios:**
1. CEM informs EV about self-produced current per-phase (M) -- per-phase current limits
2. Failsafe (M) -- what to do if communication lost
3. Heartbeat (M)
4. CEM informs EV about self-produced current total (O) -- consolidated single value

**Key feature:** OSCEV is a "soft recommendation" -- it tells the EV the recommended charging current based on surplus power. It's structured as current limits via LoadControl but with different semantics than OPEV (which is hard safety limits).

**MASH Mapping:** EnergyControl current setpoints (not limits). The distinction between "recommended" (OSCEV) and "mandatory" (OPEV) maps to MASH's setpoint vs limit distinction.

**Gaps:** Minor. OSCEV auto-deactivates per-phase values when total is written. This is implementation behavior, not a protocol gap.

---

#### 2.5 OPEV - Overload Protection by EV Charging Current Curtailment

**Goal:** Prevent fuse trips by dynamically curtailing EV charging current. Safety-critical, real-time.

**Single scenario:** Energy Guard curtails charging current -- per-phase or total current limits with failsafe and heartbeat.

**MASH Mapping:** EnergyControl `myCurrentLimitsConsumption` per-phase. Failsafe + heartbeat covered by transport keep-alive and EnergyControl failsafe state.

**Gaps:** None. Well covered.

---

#### 2.6 EVCEM - EV Charging Electricity Measurement

**Goal:** Measure the EV's charging current, power, and session energy. At least one scenario mandatory.

**Scenarios:**
1. Measure charging current (O) -- phase-specific current
2. Measure charging power (O) -- phase-specific power
3. Measure charged energy (O) -- total session energy

**MASH Mapping:** Measurement (current, power) + ChargingSession (sessionEnergyCharged/Discharged).

**Gaps:** None. Well covered.

---

#### 2.7 EVSOC - EV State of Charge

**Goal:** Monitor EV battery SoC, capacity, travel range, and estimated charging times.

**Scenarios:**
1. Monitor SoC (M) -- current SoC %, minimum SoC (charge ASAP), target SoC
2. Monitor max useable energy capacity (M) -- battery capacity in Wh
3. Monitor current travel range (O) -- estimated range in meters
4. Monitor remaining times (O) -- time to min/max/target SoC

**MASH Mapping:** ChargingSession covers most:
- `evStateOfCharge` = current SoC
- `evBatteryCapacity` = max useable energy capacity
- `estimatedTimeToMinSoC/TargetSoC/FullSoC` = remaining times
- Energy requests indirectly represent SoC targets (as energy, not %)

**Gaps:**
- **Missing SoC percentage targets.** EVSOC has `evMinSoC` (%) and `evTargetSoC` (%) as user-facing values. ChargingSession has energy-based requests (mWh) but not percentage-based SoC targets. Adding `evMinStateOfCharge` (uint8 %) and `evTargetStateOfCharge` (uint8 %) would complete the mapping.
- **Missing travel range.** `evCurrentTravelRange` (meters) is useful for user display but not for energy management. Low priority.

---

#### 2.8 EVCS - EV Charging Summary

**Goal:** Energy Broker provides a charging session cost/energy summary to the EVSE for user display.

**Data points:** Total cost, total energy, self-produced cost/energy, grid cost/energy.

**SPINE Features:** `Bill`

**MASH Mapping:** No direct equivalent.

**Gaps:**
- **No billing/cost summary.** This is informational (not for actual billing). Could be modeled as:
  1. Extension to ChargingSession (add `sessionCost`, `selfProducedEnergy`, `gridEnergy` attributes)
  2. A lightweight "ChargingSummary" or "SessionSummary" concept
- **Assessment:** Low priority. The data is typically computed by the controller and displayed on the EVSE UI. The controller already has all inputs (Tariff, Measurement, ChargingSession) to compute this itself. Adding it to the protocol mainly serves EVSE display use cases.

---

#### 2.9 DBEVC - Dynamic Bidirectional EV Charging

**Goal:** CEM dynamically controls charge/discharge of a bidirectional EV with power setpoints. EV reports energy requests and power constraints.

**Scenarios:**
1. Control active power setpoint (M) -- symmetric or phase-specific, positive=charge, negative=discharge, with ACK/NACK, optional start/end times
2. Monitor energy requests (M) -- min/max/target energy for charging AND discharging, departure time, "discharge below target permitted" flag
3. Monitor power constraints (M) -- nominal min/max charge/discharge power

**Key features:**
- Bidirectional: positive setpoint = charge, negative = discharge (passive sign convention)
- Setpoint changeability: EV can mark setpoint as "non-changeable" (CEM must not override)
- Scheduled setpoints: optional start/end times for future-effective setpoints
- ACK/NACK: EV explicitly confirms or rejects setpoints

**MASH Mapping:**
- S1 -> EnergyControl: consumption/production setpoints (positive/negative = charge/discharge)
- S2 -> ChargingSession: evMinEnergyRequest, evMaxEnergyRequest, evTargetEnergyRequest, evMinDischargingRequest, evMaxDischargingRequest, evDepartureTime, evDischargeBelowTargetPermitted
- S3 -> Electrical: nominal power limits

**Gaps:**
- **Scheduled setpoints.** DBEVC setpoints can have start/end times (future-effective). EnergyControl setpoints are immediate. Adding optional `setpointStartTime`/`setpointEndTime` would address this, but adds complexity.
- **Setpoint changeability.** The EV can lock its setpoint ("non-changeable"). No MASH equivalent. Could be a simple boolean on EnergyControl.
- **ACK/NACK.** DBEVC explicitly confirms/rejects setpoints. MASH Write operations have response codes, so ACK/NACK is handled at the protocol level.
- **Assessment:** Core bidirectional setpoint control is well covered. Scheduled setpoints and changeability flags are refinements for later.

---

#### 2.10 SMR - Session Measurement Relation

**Goal:** Link charging sessions to their measurement identifiers for historical correlation (billing, logging).

**Scenarios:**
1. Latest session measurement relation (M) -- session ID + list of related measurement IDs
2. Previous session measurement relation (O) -- same for past sessions

**MASH Mapping:** ChargingSession.sessionId + temporal correlation (measurements during session time window).

**Gaps:**
- **Explicit measurement-to-session linking.** MASH relies on temporal correlation rather than explicit ID mapping. This is simpler but less robust for historical queries.
- **Assessment:** Same philosophical question as MHPC -- historical data management is the controller's responsibility. The controller can build its own session-measurement index from sessionStartTime/sessionEndTime + Measurement subscriptions.

---

### Batch 2 SPINE-to-MASH Feature Type Mapping

| SPINE Feature | Used By | MASH Feature | Notes |
|--------------|---------|-------------|-------|
| IncentiveTable | CEVC | Signals + Tariff | Same gap as ITPCM (per-slot tiers) |
| TimeSeries | CEVC | Signals (CONSTRAINT) | Pmax curve |
| PowerSequences | CEVC | Plan | Charging plan output |
| DeviceClassification | EVCC, EVSECC | DeviceInfo | Manufacturer, model, serial |
| DeviceConfiguration | EVCC, DBEVC | ChargingSession + Electrical | Power limits, failsafe |
| DeviceDiagnosis | EVCC, EVSECC, OPEV, OSCEV | Status + transport | Operating state, heartbeat |
| ElectricalConnection | EVCC, OSCEV, OPEV, EVCEM, CEVC, DBEVC | Electrical | Phase config, current limits |
| Identification | EVCC | ChargingSession.evIdentifications | EV identifiers |
| LoadControl | OSCEV, OPEV | EnergyControl | Current limits/setpoints |
| Measurement | EVCEM, EVSOC | Measurement + ChargingSession | Telemetry + SoC |
| DirectControl | DBEVC | EnergyControl | Power setpoints |
| Bill | EVCS | None | **No cost summary feature** |

### Batch 2 Gap Summary

| Priority | Gap | Affected UCs | Recommendation |
|----------|-----|-------------|----------------|
| Medium | Per-slot power-dependent tiers (same as Batch 1) | CEVC | Same as ITPCM recommendation |
| Medium | Negotiation protocol (same as Batch 1) | CEVC | Same as ITPCM recommendation |
| Low | Communication standard attribute | EVCC | Add optional `communicationStandard` to ChargingSession or leave implicit via evDemandMode |
| Low | SoC percentage targets (evMinSoC%, evTargetSoC%) | EVSOC | Add `evMinStateOfCharge` (uint8) and `evTargetStateOfCharge` (uint8) to ChargingSession |
| Low | EV travel range | EVSOC | Add optional `evTravelRange` (uint32 m) to ChargingSession |
| Low | Charging cost summary | EVCS | Defer; controller computes this from existing data |
| Low | Scheduled setpoints (start/end time) | DBEVC | Consider for v2; adds complexity |
| Low | Setpoint changeability flag | DBEVC | Consider adding boolean to EnergyControl |
| Low | Explicit session-measurement linking | SMR | Defer; temporal correlation sufficient |

---

### Batch 2 Key Observations

**EV/EVSE entity split:** In EEBUS, the EV and EVSE are separate SPINE entities. In MASH, this collapses into a single EV_CHARGER endpoint because MASH communicates with the EVSE (not directly with the EV). The EVSE proxies EV data. This is a simplification, not a gap -- MASH's model is correct for the physical communication topology.

**EEBUS has 10 EV-related use cases; MASH handles them with ~4 features:**
- ChargingSession = EVCC + EVSOC + EVCS + SMR (session state, EV data)
- EnergyControl = OSCEV + OPEV + DBEVC (limits, setpoints)
- Signals + Tariff + Plan = CEVC (incentive-based coordination)
- Measurement + Electrical = EVCEM + EVSECC (telemetry, config)

This consolidation is intentional and a core MASH design principle: fewer, more general features vs. EEBUS's per-scenario feature specialization.

*Batch 2 complete.*

---

## Batch 3: HVAC Monitoring

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 3.1 | MoRHSF | Monitor room heating operation mode (auto/on/off/eco) | HVACRoom (nested: HeatingCircuit > HeatingZone > HVACRoom), MonitoringAppliance | HVAC (specialization: RoomHeatingSystemFunctionOperationMode) | S1: Monitor room heating operation mode (M) | None | **GAP: No HVAC operation mode feature. No nested entity hierarchy.** |
| 3.2 | MoRCSF | Monitor room cooling operation mode (auto/on/off/eco) | HVACRoom (same nesting), MonitoringAppliance | HVAC (specialization: RoomCoolingSystemFunctionOperationMode) | S1: Monitor room cooling operation mode (M) | None | **GAP: Same as MoRHSF -- symmetric for cooling.** |
| 3.3 | MoRT | Monitor current room temperature (latest value only) | HVACRoom, MonitoringAppliance | Measurement (specialization: RoomAirTemperature) | S1: Monitor HVAC room temperature (M) | **Measurement** (partial) | **GAP: Measurement only handles electrical quantities. Temperature measurement type not defined.** |
| 3.4 | MoDHWSF | Monitor DHW system function operation mode and overrun status (e.g., one-time DHW loading) | DHWCircuit, MonitoringAppliance | HVAC (specialization: DHWSystemFunctionOperationMode, DHWSystemFunctionOverrun) | S1: Monitor DHW operation mode (M), S2: Monitor DHW overrun (O) | None | **GAP: No HVAC feature, no overrun concept, no DHW entity type.** |
| 3.5 | MoDHWT | Monitor current domestic hot water temperature | DHWCircuit, MonitoringAppliance | Measurement (specialization: DHWTemperature) | S1: Monitor DHW temperature (M) | **Measurement** (partial) | **GAP: Same as MoRT -- temperature measurement type not defined.** |
| 3.6 | MoOT | Monitor current outdoor air temperature | TemperatureSensor, MonitoringAppliance | Measurement (specialization: OutdoorAirTemperature) | S1: Monitor outdoor temperature (M) | **Measurement** (partial) | **GAP: Same as MoRT. Also: no TemperatureSensor endpoint type.** |
| 3.7 | MCSGR | Monitor and control SG-Ready conditions (4 operating states for heat pump grid interaction) | HeatPumpAppliance, CEM | HVAC (specialization: HeatingSystemFunctionOverrunSgReady + _Configuration) | S1: Monitor SG-Ready condition (M), S2: Control SG-Ready condition (M) | **EnergyControl** (partial) | **GAP: SG-Ready is a discrete 4-state operating mode, not a continuous power limit/setpoint. No direct mapping.** |

---

### Detailed Analysis

#### 3.1 MoRHSF - Monitoring of Room Heating System Function

**Goal:** A Monitoring Appliance (CEM, HMI, smartphone app) retrieves the current operation mode of the room heating system function from an HVAC Room.

**Actors:**
- HVAC Room -- the entity being monitored (server of HVAC feature)
- Monitoring Appliance -- CEM or display (client of HVAC feature)

**Entity hierarchy (critical):**
```
Device (deviceType = <any>)
├── Entity: DeviceInformation
│   └── Feature: NodeManagement (special)
├── Entity: HeatingCircuit        ← optional parent
│   └── Entity: HeatingZone       ← parent
│       └── Entity: HVACRoom      ← the actual actor
│           └── Feature: HVAC (server)
│               └── Specialization: HVAC_RoomHeatingSystemFunctionOperationMode
```
The HVACRoom entity can be nested 2-3 levels deep: directly under Device, under HeatingZone, or under HeatingCircuit > HeatingZone. This is the deepest nesting in all EEBUS use cases.

**Scenario 1 -- Monitor room heating operation mode (M):**
- Read HVAC feature to get the current operation mode
- Modes: "auto", "on", "off", "eco" (at least 2 must be supported)
- Exactly one mode active at any time
- Uses: `hvacSystemFunctionOperationModeRelationListData` to discover which modes are available for heating, `hvacOperationModeDescriptionListData` for mode metadata, `hvacSystemFunctionListData` for current active mode

**SPINE Features used:** HVAC (with sub-functions: hvacSystemFunction, hvacSystemFunctionOperationModeRelation, hvacSystemFunctionDescription, hvacOperationModeDescription)

**MASH Mapping:** No direct mapping. MASH has no HVAC-specific feature and no operation mode concept.

**Gaps:**
- **No HVAC operation mode model.** The HVAC feature in SPINE is a complex relational model: system functions have relations to available operation modes, each mode has descriptions, and exactly one is active. MASH has nothing equivalent.
- **No entity nesting.** MASH endpoints are flat (Device > Endpoint). There's no way to express HeatingCircuit > HeatingZone > HVACRoom hierarchy. This is needed to associate rooms with heating zones and circuits.
- **Assessment:** This requires new MASH features -- see gap analysis below.

---

#### 3.2 MoRCSF - Monitoring of Room Cooling System Function

**Goal:** Monitor the current operation mode of the room cooling system function. Symmetric to MoRHSF.

**Actors:** Same as MoRHSF (HVAC Room + Monitoring Appliance).

**Entity hierarchy:** Identical to MoRHSF. Same HVACRoom entity, but with a different HVAC feature specialization (RoomCoolingSystemFunctionOperationMode).

**Scenario 1 -- Monitor room cooling operation mode (M):**
- Same pattern: read HVAC feature, get active cooling mode (auto/on/off/eco)

**SPINE Features used:** HVAC (same sub-functions as MoRHSF, different specialization)

**MASH Mapping:** No direct mapping (same as MoRHSF).

**Gaps:** Identical to MoRHSF. A MASH HVAC feature would handle both heating and cooling modes as different "system functions" on the same feature instance.

---

#### 3.3 MoRT - Monitoring of Room Temperature

**Goal:** Retrieve the current measured room temperature from an HVAC Room. Latest value only, no historical data.

**Actors:**
- HVAC Room -- provides temperature measurement (Measurement feature server)
- Monitoring Appliance -- reads temperature (Measurement feature client)

**Entity hierarchy:** Same as MoRHSF (HVACRoom entity, possibly nested under HeatingZone/HeatingCircuit). The Measurement feature sits on the same HVACRoom entity.

**Scenario 1 -- Monitor HVAC room temperature (M):**
- Read Measurement feature on HVACRoom entity
- Returns: measured temperature value + value state (normal / out of range / error)
- Only latest value exchanged (no history)
- Temperature in degrees Celsius (or Kelvin)

**SPINE Features used:** Measurement (with specialization: temperature measurement type = room air temperature)

**MASH Mapping:** Measurement feature -- partial.

**Gaps:**
- **Measurement feature only defines electrical quantities.** MASH Measurement supports: activePower, reactivePower, apparentPower, activeEnergy, reactiveEnergy, voltage, current, frequency, powerFactor, stateOfCharge, stateOfHealth. Temperature is not in this list.
- **Value state for non-electrical measurements.** The "out of range" / "error" value state concept applies to temperature sensors. MASH Measurement doesn't have per-measurement-value quality indicators (though Status feature has fault reporting).
- **Assessment:** Extending the Measurement feature to support temperature (and potentially other non-electrical measurements like humidity, pressure) would be the natural approach. Alternatively, a new Temperature feature could be created, but that's less compositional.

---

#### 3.4 MoDHWSF - Monitoring of DHW System Function

**Goal:** Monitor the operation mode of the domestic hot water (DHW) system function, plus any active overrun (e.g., one-time DHW loading).

**Actors:**
- DHW Circuit -- the entity being monitored (server of HVAC feature)
- Monitoring Appliance -- CEM or display (client)

**Entity hierarchy:**
```
Device (deviceType = <any>)
├── Entity: DeviceInformation
│   └── Feature: NodeManagement (special)
└── Entity: DHWCircuit              ← the actor
    └── Feature: HVAC (server)
        ├── Specialization: HVAC_DHWSystemFunctionOperationMode
        └── Specialization: HVAC_DHWSystemFunctionOverrun  (S2 only)
```
DHWCircuit is typically a direct child of Device (1 level deep, simpler than room heating).

**Scenario 1 -- Monitor DHW operation mode (M):**
- Same pattern as MoRHSF: read HVAC feature, get active mode (auto/on/off/eco)

**Scenario 2 -- Monitor DHW overrun (O):**
- Read `hvacOverrunListData` from HVAC feature
- Overrun = temporary action that overrides normal operation (e.g., "one-time DHW loading")
- Overrun status: inactive, active, running, finished
- Overrun has a description (label, type)

**SPINE Features used:** HVAC (system function + operation mode for S1; overrun + overrun description for S2)

**MASH Mapping:** No direct mapping.

**Gaps:**
- **No HVAC operation mode model** (same as MoRHSF).
- **No overrun concept.** SPINE's overrun is a temporary action that overrides the normal operation mode until completed or cancelled. This is similar to a "process" but with different semantics (it's a temporary override, not a scheduled process). MASH's ProcessStateEnum (NONE, AVAILABLE, SCHEDULED, RUNNING, PAUSED, COMPLETED, ABORTED) could partially model this, but the override semantics are different.
- **No DHW endpoint type.** MASH has HEAT_PUMP and WATER_HEATER endpoint types but no DHW_CIRCUIT. The DHWCircuit entity type represents a specific subsystem (the DHW loop of a heating system), not the whole appliance.
- **Assessment:** The overrun concept is important for HVAC comfort functions. It could be modeled as a lightweight process on a new HVAC feature, or as a special mode on the Status feature.

---

#### 3.5 MoDHWT - Monitoring of DHW Temperature

**Goal:** Monitor the current domestic hot water temperature. Enables CEM to estimate energy demand by comparing actual temperature to setpoint.

**Actors:**
- DHW Circuit -- provides temperature measurement (same entity as MoDHWSF)
- Monitoring Appliance -- reads temperature

**Entity hierarchy:** Same DHWCircuit entity as MoDHWSF. The Measurement feature sits alongside the HVAC feature on the same entity.

**Scenario 1 -- Monitor DHW temperature (M):**
- Read Measurement feature on DHWCircuit entity
- Returns: measured DHW temperature value + value state
- Latest value only, no history
- The spec notes that comparing actual temp to setpoint enables demand estimation

**SPINE Features used:** Measurement (with specialization: DHW temperature)

**MASH Mapping:** Measurement feature -- partial (same gap as MoRT).

**Gaps:**
- **Temperature measurement type not defined** (same as MoRT).
- **Setpoint comparison.** The use case explicitly mentions comparing actual temperature to setpoint for demand estimation. This requires both Measurement (actual) and Setpoint (target) features on the same entity. MASH has no temperature setpoint concept.
- **Assessment:** Same recommendation as MoRT: extend Measurement for temperature, and add a temperature setpoint mechanism (either via EnergyControl extension or new feature).

---

#### 3.6 MoOT - Monitoring of Outdoor Temperature

**Goal:** Monitor the outdoor air temperature from a temperature sensor. Used for energy demand estimation and automation triggers.

**Actors:**
- Outdoor Temperature Sensor -- provides measurement (server)
- Monitoring Appliance -- reads temperature (client)

**Entity hierarchy:**
```
Device (deviceType = <any>)
├── Entity: DeviceInformation
│   └── Feature: NodeManagement (special)
└── Entity: TemperatureSensor         ← the actor
    └── Feature: Measurement (server)
        └── Specialization: Measurement_OutdoorAirTemperature
```
Simple: one level, TemperatureSensor entity with Measurement feature.

**Scenario 1 -- Monitor outdoor temperature (M):**
- Read Measurement feature on TemperatureSensor entity
- Returns: temperature value + value state (normal / out of range / error)
- Latest value only
- Value state rules: "out of range" and "error" values SHALL be ignored by the Monitoring Appliance

**SPINE Features used:** Measurement (outdoor air temperature specialization)

**MASH Mapping:** Measurement feature -- partial.

**Gaps:**
- **Temperature measurement type not defined** (same as MoRT/MoDHWT).
- **No sensor endpoint type.** MASH has no generic "sensor" or "temperature sensor" endpoint type. Current endpoint types are all energy devices (inverter, battery, charger, etc.).
- **Assessment:** A generic SENSOR endpoint type or extending the existing HVAC endpoint type to cover temperature sensors would work. The measurement extension is the same recommendation as MoRT.

---

#### 3.7 MCSGR - Monitoring and Control of SG-Ready Conditions

**Goal:** Digitalize the SG-Ready switching contacts for heat pump grid interaction. CEM monitors and controls which of 4 SG-Ready conditions the heat pump is in.

**Actors:**
- Heat Pump -- provides SG-Ready state via HVAC feature (server)
- CEM -- monitors and controls SG-Ready conditions (client)

**Entity hierarchy:**
```
Device (deviceType = <any>)
├── Entity: DeviceInformation
│   └── Feature: NodeManagement (special)
└── Entity: HeatPumpAppliance          ← the actor
    └── Feature: HVAC (server)
        ├── Specialization: HVAC_HeatingSystemFunctionOverrunSgReady        (S1: read)
        └── Specialization: HVAC_HeatingSystemFunctionOverrunSgReady_Configuration  (S2: write)
```

**SG-Ready conditions:**
1. **Condition 1** (1:0) -- Forced off / grid lock. Max 2h/day. Similar to "off" (frost protection ensured).
2. **Condition 2** (0:0) -- Normal operation. Similar to "auto". Default when no other condition active.
3. **Condition 3** (0:1) -- Intensified operation. Activation recommendation (not a definite start). Similar to "on".
4. **Condition 4** (1:1) -- Definite start-up command with increased temperature setpoints. Most aggressive.

**Scenario 1 -- Monitor SG-Ready condition (M):**
- Read HVAC feature to get `hvacOverrunListData` with 4 overrun entries (one per SG-Ready condition)
- Each has `overrunStatus`: "active", "inactive", or "running"
- Exactly one condition active at any time

**Scenario 2 -- Control SG-Ready condition (M):**
- Write to HVAC feature to set `overrunStatus` = "active" for one condition
- Heat pump automatically sets all others to "inactive"
- If conditions 1, 3, 4 are all inactive, condition 2 (normal) is implicitly active

**Key design feature:** SPINE models this as HVAC overruns on a HeatPumpAppliance entity. The 4 conditions map to 4 overrun entries with mutual exclusion. The CEM writes the overrun status.

**SPINE Features used:** HVAC (hvacOverrun, hvacOverrunDescription, hvacSystemFunction, hvacSystemFunctionDescription)

**MASH Mapping:** Partial -- could map to EnergyControl or Status, but neither is a clean fit.

**Gaps:**
- **No SG-Ready operating mode.** The 4 SG-Ready conditions are a standardized (DIN) interface for heat pump grid interaction. They don't map cleanly to power limits or setpoints. They are discrete operating states with associated behavioral changes (temperature boost, forced off, etc.).
- **Possible MASH mappings:**
  1. **EnergyControl with controlState:** The MASH ControlStateEnum (AUTONOMOUS, CONTROLLED, LIMITED, FAILSAFE, OVERRIDE) partially overlaps: SG1=LIMITED/FAILSAFE, SG2=AUTONOMOUS, SG3=CONTROLLED (setpoint), SG4=OVERRIDE. But this overloads controlState with SG-Ready-specific semantics.
  2. **A new "operating mode" attribute on Status or a new HVAC feature:** An explicit operating mode enum would be cleaner.
  3. **Signals feature:** SG-Ready conditions could be sent as discrete signal types (one slot with a mode indicator). This is creative but doesn't match Signals' time-slotted design.
- **Assessment:** SG-Ready is Germany-specific but increasingly adopted across Europe. A clean mapping would be a new HVAC-focused feature or a discrete operating mode concept on the existing HEAT_PUMP endpoint. The ControlStateEnum approach works pragmatically but conflates protocol state with application mode.

---

### Batch 3 SPINE-to-MASH Feature Type Mapping

| SPINE Feature | Used By | MASH Feature | Notes |
|--------------|---------|-------------|-------|
| HVAC (system function) | MoRHSF, MoRCSF, MoDHWSF, MCSGR | None | **No equivalent. New feature needed.** |
| HVAC (operation mode) | MoRHSF, MoRCSF, MoDHWSF | None | **No equivalent. Part of new HVAC feature.** |
| HVAC (overrun) | MoDHWSF, MCSGR | None / EnergyControl (partial) | **No overrun concept. ProcessState covers some semantics.** |
| Measurement (temperature) | MoRT, MoDHWT, MoOT | Measurement (partial) | **Electrical only. Temperature measurement type needed.** |
| Setpoint (temperature) | (linked from MoDHWT, MoRT for comparison) | None | **No temperature setpoint. EnergyControl only has power/current.** |

### Batch 3 Entity Type Mapping

| SPINE Entity Type | Used By | MASH Endpoint Type | Notes |
|------------------|---------|-------------------|-------|
| HVACRoom | MoRHSF, MoRCSF, MoRT | HVAC (0x08) | Partially. MASH HVAC endpoint exists but has no room semantics. |
| DHWCircuit | MoDHWSF, MoDHWT | WATER_HEATER (0x07) | Partial. WATER_HEATER covers DHW appliances but not circuit-level granularity. |
| TemperatureSensor | MoOT | None | **No sensor endpoint type.** Could use DEVICE_ROOT or a new SENSOR type. |
| HeatPumpAppliance | MCSGR | HEAT_PUMP (0x06) | Good match. |
| HeatingCircuit | (parent entity) | None | **No endpoint hierarchy.** |
| HeatingZone | (parent entity) | None | **No endpoint hierarchy.** |

### Batch 3 Gap Summary

| Priority | Gap | Affected UCs | Recommendation |
|----------|-----|-------------|----------------|
| **High** | **No HVAC operation mode feature** | MoRHSF, MoRCSF, MoDHWSF, MCSGR (+ all Batch 4 config UCs) | New feature: "HvacMode" or extend Status with operation mode attributes. Needs: system function type (heating/cooling/DHW/ventilation), available modes per function, active mode per function. |
| **High** | **No entity hierarchy / parent endpoint** | MoRHSF, MoRCSF, MoRT (+ all HVAC UCs) | Add optional `parentEndpoint` field to endpoint descriptor. Enables: HeatingCircuit > HeatingZone > HVACRoom nesting. Also useful for PV systems (PVSystem > Inverter > PVString). |
| **High** | **Measurement limited to electrical quantities** | MoRT, MoDHWT, MoOT | Extend Measurement feature to support temperature (and potentially humidity, pressure). Add a `measurementDomain` or `measurementCategory` enum: ELECTRICAL, THERMAL, ENVIRONMENTAL. Or simply add temperature measurement types to the existing measurement type enum. |
| Medium | No HVAC overrun / temporary override concept | MoDHWSF, MCSGR | Could extend ProcessStateEnum or add to new HVAC feature. Overrun = temporary action that overrides normal mode until finished. |
| Medium | No temperature setpoint | (linked from MoDHWT, MoRT for demand estimation) | Add temperature setpoint capability -- either via new HVAC feature or by generalizing EnergyControl. Needed for Batch 4 (Configuration UCs). |
| Medium | SG-Ready operating mode mapping | MCSGR | Map to discrete operating mode enum on HEAT_PUMP endpoint, or define SG-Ready as a special case of the new HVAC feature's overrun mechanism. |
| Low | No sensor endpoint type | MoOT | Add SENSOR or TEMPERATURE_SENSOR endpoint type. Or reuse HVAC endpoint type for temperature sensors. |
| Low | Endpoint label / room name | (implied by VHAN in Batch 4) | Add optional `endpointLabel` to endpoint descriptor for human-readable room names. |

---

### Batch 3 Key Observations

**This batch surfaces the most significant structural gaps in MASH:**

1. **MASH was designed for energy management, not HVAC control.** The existing 9 features all deal with electrical quantities (power, energy, current, voltage) and energy management (limits, setpoints, tariffs, plans). HVAC monitoring introduces a fundamentally different data domain: thermal quantities (temperature), mechanical states (operation modes), and HVAC-specific concepts (overruns, system functions).

2. **The entity hierarchy problem is cross-cutting.** SPINE uses deeply nested entities to model physical topology:
   - HeatingCircuit > HeatingZone > HVACRoom (3 levels)
   - This is also used for PV systems: PVSystem > Inverter > PVString
   - MASH's flat endpoint model (Device > Endpoint, 1 level) cannot express these relationships

   MASH could add a simple `parentEndpoint` reference to the endpoint descriptor to enable optional nesting. This is a minimal change that unlocks significant modeling capability.

3. **The HVAC feature is the largest single missing feature.** SPINE's HVAC feature is complex with 8 sub-functions (system function, operation mode, operation mode relation, setpoint relation, power sequence relation, system function description, operation mode description, overrun + description). MASH doesn't need this complexity -- a simplified HVAC feature covering:
   - System function type enum (HEATING, COOLING, DHW, VENTILATION)
   - Operation mode enum (AUTO, ON, OFF, ECO)
   - Active mode per system function
   - Optional overrun state (INACTIVE, ACTIVE, RUNNING, FINISHED)

   ...would cover all 7 use cases in Batches 3 and 4.

4. **Temperature measurement is a natural extension.** Adding temperature to the existing Measurement feature is more compositional than creating a separate Temperature feature. The SPINE approach is identical: same Measurement class, different specialization. MASH can follow this pattern by extending the measurement type enum.

5. **SG-Ready maps pragmatically to EnergyControl but semantically to a mode concept.** If MASH adds an HVAC operation mode feature, SG-Ready conditions could be modeled as a special system function type with 4 predefined modes. This would be cleaner than overloading EnergyControl's controlState.

*Batch 3 complete.*

---

## Batch 4: HVAC Configuration

Batch 4 adds **write operations** on top of Batch 3's monitoring structure. These UCs use the same entities, entity hierarchy, and feature types as Batch 3 -- they just add the ability to change operation modes and temperature setpoints.

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 4.1 | CoRHSF | Set room heating operation mode (auto/on/off/eco) | HVACRoom, ConfigurationAppliance | HVAC (write, specialization: RoomHeatingSystemFunctionOperationMode_Configuration) | S1: Set room heating operation mode (M) | None | **Same as MoRHSF + write.** No HVAC operation mode feature. |
| 4.2 | CoRCSF | Set room cooling operation mode (auto/on/off/eco) | HVACRoom, ConfigurationAppliance | HVAC (write, specialization: RoomCoolingSystemFunctionOperationMode_Configuration) | S1: Set room cooling operation mode (M) | None | **Same as MoRCSF + write.** Symmetric. |
| 4.3 | CoRHT | Set room heating temperature setpoint, with mode-setpoint relations and constraints | HVACRoom, ConfigurationAppliance | Setpoint (write) + HVAC (mode-setpoint relations) | S1: Set room heating temperature setpoint (M) | None | **GAP: No temperature setpoint feature. Mode-setpoint relation model needed.** |
| 4.4 | CoRCT | Set room cooling temperature setpoint, with mode-setpoint relations and constraints | HVACRoom, ConfigurationAppliance | Setpoint (write) + HVAC (mode-setpoint relations) | S1: Set room cooling temperature setpoint (M) | None | **Same as CoRHT for cooling.** |
| 4.5 | CoDHWSF | Set DHW operation mode + start/stop one-time DHW loading overrun | DHWCircuit, ConfigurationAppliance | HVAC (write, specialization: DHWSystemFunction_Configuration + Overrun_Configuration) | S1: Set DHW operation mode (M), S2: Start one-time DHW loading (M), S3: Stop one-time DHW loading (O) | None | **Same as MoDHWSF + write + overrun control.** |
| 4.6 | CoDHWT | Set DHW temperature setpoint, with mode-setpoint relations and constraints | DHWCircuit, ConfigurationAppliance | Setpoint (write) + HVAC (mode-setpoint relations) | S1: Set DHW temperature setpoint (M) | None | **Same as CoRHT pattern applied to DHW.** |
| 4.7 | VHAN | Read human-readable names for heating circuits, heating zones, and HVAC rooms | HeatingCircuit, HeatingZone, HVACRoom, VisualizationAppliance | DeviceConfiguration (read) | S1: Visualize heating circuit name (M), S2: Visualize heating zone name (O), S3: Visualize heating room name (O) | **DeviceInfo** (partial) | **GAP: No endpoint label attribute. Entity hierarchy required for multi-level names.** |

---

### Detailed Analysis

#### 4.1 CoRHSF - Configuration of Room Heating System Function

**Goal:** Configuration Appliance (CEM, app, HMI) sets the heating operation mode on an HVAC Room. This is the write counterpart of MoRHSF (Batch 3.1).

**Actors:**
- HVAC Room -- server of HVAC feature (accepts mode changes)
- Configuration Appliance -- client writing to HVAC feature

**Entity hierarchy:** Identical to MoRHSF: Device > (HeatingCircuit >) HeatingZone > HVACRoom.

**Scenario 1 -- Set room heating operation mode (M):**
- Write to HVAC feature to set the active heating operation mode
- Modes: "auto" (time-schedule based), "on" (always heating), "off" (never), "eco" (reduced/night temperature)
- Exactly one mode active at any time
- The HVAC feature uses `hvacSystemFunctionOperationModeRelationListData` to discover which modes are available, then writes the active mode via `hvacSystemFunctionListData`

**SPINE Features used:** HVAC (same as MoRHSF, but with write access to set the active mode)

**MASH Mapping:** No direct mapping. Same gap as MoRHSF.

**Gaps:** Identical to MoRHSF (Batch 3.1). The same new HVAC feature that supports reading operation modes would need to support writing them. In MASH terms: the HVAC feature would have direction = bidirectional (device reports current mode, controller can set mode).

---

#### 4.2 CoRCSF - Configuration of Room Cooling System Function

**Goal:** Set the cooling operation mode. Symmetric to CoRHSF.

**Scenario 1 -- Set room cooling operation mode (M):**
- Same pattern: write to HVAC feature to change active cooling mode (auto/on/off/eco)
- "auto" = time-schedule based cooling, "on" = always cooling, "off" = never, "eco" = reduced

**MASH Mapping:** No direct mapping. Same gap as MoRCSF / CoRHSF.

**Gaps:** Same as CoRHSF. A MASH HVAC feature would handle heating and cooling as different system function types on the same feature instance.

---

#### 4.3 CoRHT - Configuration of Room Heating Temperature

**Goal:** Set the room temperature setpoint for heating. The setpoint may differ per operation mode (e.g., "on" mode = 21C, "eco" mode = 18C, "auto" mode = multiple setpoints selected by time).

**Actors:**
- HVAC Room -- server of Setpoint feature (accepts setpoint changes)
- Configuration Appliance -- client writing to Setpoint feature

**Entity hierarchy:** Same as MoRHSF. The Setpoint feature sits on the HVACRoom entity alongside the HVAC feature.

**Scenario 1 -- Set room heating temperature setpoint (M):**
- Write to Setpoint feature to set temperature value
- **Mode-setpoint relations (critical complexity):**
  - "auto" mode: relates to 1-4 setpoints (device selects based on time/conditions)
  - "on" / "eco" mode: relates to exactly 1 setpoint each
  - "off" mode: relates to 0-1 setpoints
- **Setpoint constraints:** device reports min, max, step size (or discrete value list) via `setpointConstraintsListData`
- **Setpoint description:** links to a measurement ID (for comparing actual vs desired) and has a label

**Key data model:**
```
Setpoint feature on HVACRoom:
  setpointListData:
    - setpointId: 1, value: 21.0    ← "on" mode temperature
    - setpointId: 2, value: 18.0    ← "eco" mode temperature
    - setpointId: 3, value: 22.0    ← "auto" daytime temperature
    - setpointId: 4, value: 17.0    ← "auto" nighttime temperature

  setpointConstraintsListData:
    - setpointId: 1, rangeMin: 5.0, rangeMax: 30.0, stepSize: 0.5
    - setpointId: 2, rangeMin: 5.0, rangeMax: 30.0, stepSize: 0.5

HVAC feature on same entity (hvacSystemFunctionSetpointRelationListData):
    - systemFunctionId: 1 (heating), operationModeId: "on"  → setpointId: 1
    - systemFunctionId: 1 (heating), operationModeId: "eco" → setpointId: 2
    - systemFunctionId: 1 (heating), operationModeId: "auto" → setpointId: 3, 4
```

**SPINE Features used:** Setpoint (write), HVAC (relations, read-only)

**MASH Mapping:** No direct mapping. EnergyControl has power/current setpoints but no temperature setpoints. The mode-setpoint relation model has no MASH equivalent.

**Gaps:**
- **No temperature setpoint feature.** MASH EnergyControl's setpoints (`effectiveConsumptionSetpoint`, `effectiveProductionSetpoint`) are power values, not temperatures. A temperature setpoint mechanism is needed.
- **Mode-setpoint relations.** The SPINE model links setpoints to operation modes (each mode can have a different target temperature). This is a relational model that's more complex than a single setpoint attribute.
- **Setpoint constraints (min/max/step).** SPINE's `setpointConstraintsListData` provides range and step size. MASH would need similar constraint reporting.
- **Assessment:** This could be modeled as part of a new HVAC feature with embedded temperature setpoints per mode, or as a generalized Setpoint feature. The former is simpler for HVAC-specific use; the latter is more extensible.

---

#### 4.4 CoRCT - Configuration of Room Cooling Temperature

**Goal:** Set the room temperature setpoint for cooling. Mirror of CoRHT.

**Scenario 1 -- Set room cooling temperature setpoint (M):**
- Identical pattern to CoRHT but for cooling system function
- Same mode-setpoint relations: "auto" = 1-4 setpoints, "on"/"eco" = 1 each, "off" = 0-1
- Same constraints model

**SPINE Features used:** Setpoint (write) + HVAC (relations)

**MASH Mapping:** No direct mapping. Same gap as CoRHT.

**Gaps:** Identical to CoRHT. The same temperature setpoint mechanism covers both heating and cooling.

---

#### 4.5 CoDHWSF - Configuration of DHW System Function

**Goal:** Set the DHW operation mode, plus start/stop one-time DHW loading overruns. This is the write counterpart of MoDHWSF (Batch 3.4).

**Actors:**
- DHW Circuit -- server of HVAC feature (accepts mode changes and overrun commands)
- Configuration Appliance -- client writing to HVAC feature

**Scenario 1 -- Set DHW operation mode (M):**
- Same as CoRHSF pattern: write to HVAC feature to change active DHW mode (auto/on/off/eco)
- "auto" = time-schedule based, "on" = always producing DHW, "off" = no DHW, "eco" = reduced

**Scenario 2 -- Start one-time DHW loading (M):**
- Write to HVAC overrun: set `overrunStatus` = "active" for the "one-time DHW" overrun
- The overrun temporarily overrides the normal operation mode
- Heat pump starts heating DHW regardless of current mode
- Overrun transitions: inactive -> active -> running -> finished (or inactive if stopped)

**Scenario 3 -- Stop one-time DHW loading (O):**
- Write to HVAC overrun: set `overrunStatus` = "inactive" for the "one-time DHW" overrun
- Cancels the running overrun, device returns to normal operation mode

**SPINE Features used:** HVAC (write: system function mode, overrun status)

**MASH Mapping:** No direct mapping.

**Gaps:**
- **Same as MoDHWSF** (Batch 3.4): no HVAC feature, no overrun concept.
- **Overrun write control.** The start/stop overrun pattern is a command-like operation (trigger a temporary action). In MASH, this could map to:
  1. An Invoke command on the new HVAC feature ("startOverrun" / "stopOverrun")
  2. Write-to-attribute pattern (write overrunStatus = ACTIVE/INACTIVE)
- **Assessment:** The Invoke approach is cleaner for command semantics. The attribute-write approach is simpler but conflates state and command.

---

#### 4.6 CoDHWT - Configuration of DHW Temperature

**Goal:** Set the DHW temperature setpoint. Mirror of CoRHT applied to DHW.

**Scenario 1 -- Set DHW temperature setpoint (M):**
- Write to Setpoint feature on DHWCircuit entity
- Same mode-setpoint relation model as CoRHT: "auto" = 1-4 setpoints, "on"/"eco" = 1, "off" = 0-1
- Same constraints (min, max, step size or discrete values)
- Configuration Appliance may adjust setpoint based on user input or energy optimization algorithms (e.g., reduce DHW temp during low-demand periods)

**SPINE Features used:** Setpoint (write) + HVAC (relations)

**MASH Mapping:** No direct mapping. Same gap as CoRHT/CoRCT.

**Gaps:** Identical to CoRHT. The temperature setpoint mechanism covers room heating, room cooling, and DHW uniformly.

---

#### 4.7 VHAN - Visualization of Heating Area Name

**Goal:** Read human-readable names for the heating system topology: heating circuits, heating zones, and HVAC rooms.

**Actors:**
- Heating Circuit -- provides its name (server)
- Heating Zone -- provides its name (server)
- HVAC Room -- provides its name (server)
- Visualization Appliance -- reads names (client)

**Entity hierarchy (critical -- same as Batch 3):**
```
Device
├── Entity: HeatingCircuit (name: "Floor Heating Ground Floor")
│   ├── Entity: HeatingZone (name: "Living Area")
│   │   ├── Entity: HVACRoom (name: "Living Room")
│   │   └── Entity: HVACRoom (name: "Kitchen")
│   └── Entity: HeatingZone (name: "Sleeping Area")
│       ├── Entity: HVACRoom (name: "Master Bedroom")
│       └── Entity: HVACRoom (name: "Kids Room")
└── Entity: HeatingCircuit (name: "Radiator Circuit Upstairs")
    └── Entity: HeatingZone (name: "Upper Floor")
        └── Entity: HVACRoom (name: "Office")
```

**Scenario 1 -- Visualize heating circuit name (M):**
- Read DeviceConfiguration feature on HeatingCircuit entity to get label/description

**Scenario 2 -- Visualize heating zone name (O):**
- Read DeviceConfiguration feature on HeatingZone entity

**Scenario 3 -- Visualize heating room name (O):**
- Read DeviceConfiguration feature on HVACRoom entity

**SPINE Features used:** DeviceConfiguration (read, for entity labels/descriptions stored as key-value pairs)

**MASH Mapping:** DeviceInfo (partial). MASH DeviceInfo has device-level metadata but no per-endpoint labels.

**Gaps:**
- **No endpoint label.** MASH endpoints don't have a human-readable name/label field. Adding an optional `endpointLabel` (string) to the endpoint descriptor would address this.
- **Entity hierarchy required.** The names are only meaningful in context of the hierarchy (which room belongs to which zone belongs to which circuit). Without `parentEndpoint`, the flat endpoint list can't express this topology.
- **Assessment:** This is the simplest UC in Batch 4 but it powerfully demonstrates why `parentEndpoint` and `endpointLabel` are both needed. These are small additions with high utility.

---

### Batch 4 SPINE-to-MASH Feature Type Mapping

| SPINE Feature | Used By | MASH Feature | Notes |
|--------------|---------|-------------|-------|
| HVAC (system function write) | CoRHSF, CoRCSF, CoDHWSF | None | **Same new feature as Batch 3, with write access** |
| HVAC (overrun write) | CoDHWSF | None | **Overrun start/stop = command or attribute-write** |
| HVAC (mode-setpoint relation) | CoRHT, CoRCT, CoDHWT | None | **Relational model linking modes to setpoints** |
| Setpoint (write) | CoRHT, CoRCT, CoDHWT | None | **No temperature setpoint feature in MASH** |
| Setpoint (constraints) | CoRHT, CoRCT, CoDHWT | None | **No setpoint constraint reporting** |
| DeviceConfiguration (labels) | VHAN | DeviceInfo (partial) | **No per-endpoint label** |

### Batch 4 Gap Summary

| Priority | Gap | Affected UCs | Recommendation |
|----------|-----|-------------|----------------|
| **High** | **HVAC operation mode write** (extends Batch 3 read-only gap) | CoRHSF, CoRCSF, CoDHWSF | New HVAC feature must be bidirectional: device reports current mode, controller can set mode. Same feature as Batch 3 recommendation. |
| **High** | **Temperature setpoint mechanism** | CoRHT, CoRCT, CoDHWT | Add temperature setpoint to new HVAC feature (per-mode setpoints with constraints). Or create a generalized Setpoint feature for non-electrical values. |
| Medium | Mode-setpoint relation model | CoRHT, CoRCT, CoDHWT | Simplify SPINE's relational model: each operation mode has one writable temperature setpoint (eliminate the 1-4 setpoints for "auto" mode complexity). |
| Medium | Overrun start/stop command | CoDHWSF | Add to HVAC feature as Invoke commands: `StartOverrun(overrunType)` / `StopOverrun(overrunType)`. Or model as attribute write to overrun status. |
| Medium | Setpoint constraints (min/max/step) | CoRHT, CoRCT, CoDHWT | Include setpoint bounds in the HVAC feature: `setpointMin`, `setpointMax`, `setpointStep`. |
| Low | Endpoint label | VHAN | Add optional `endpointLabel` (string) to endpoint descriptor in DeviceInfo. |
| Low | Entity hierarchy (reinforced from Batch 3) | VHAN | `parentEndpoint` field. VHAN makes the strongest case: names are only meaningful in hierarchy context. |

---

### Batch 4 Key Observations

**1. Batch 4 adds no new structural gaps -- it reinforces Batch 3.**

All 7 Batch 4 UCs use the same entities, hierarchy, and features as Batch 3. The only difference is write access. This validates that the Batch 3 gap analysis is complete and correct: if MASH closes the Batch 3 gaps (HVAC feature, entity hierarchy, temperature measurement), Batch 4 comes "for free" by making those features bidirectional and adding temperature setpoints.

**2. The temperature setpoint model is the main new complexity.**

SPINE uses a separate Setpoint feature with a relational model: operation modes relate to setpoints, each mode can have different setpoints, and setpoints have constraints. This is powerful but complex. MASH can simplify this significantly:

- **Option A: Embedded setpoints in HVAC feature.** Each system function (heating/cooling/DHW) has one temperature setpoint per mode. Constraints are attributes. No separate Setpoint feature needed.
- **Option B: Generalized Setpoint feature.** A new `Setpoint` feature type for any writable target value (temperature, power, etc.). More extensible but requires the relational model.
- **Recommendation: Option A** for HVAC-specific use cases. It's simpler, handles all 14 HVAC UCs (Batches 3+4), and avoids the relational complexity. Power/current setpoints remain in EnergyControl.

**3. The mode-setpoint relation can be simplified.**

SPINE allows "auto" mode to have 1-4 setpoints (selected by time or other vendor-specific conditions). This is unnecessarily complex for interoperability -- the Configuration Appliance can't know which setpoint is currently active. MASH can simplify: each mode has exactly one writeable setpoint. If the device internally uses time-based selection, that's an implementation detail.

**4. Overrun control has clean MASH semantics.**

The start/stop one-time DHW loading maps naturally to Invoke commands on the HVAC feature. This is more explicit than SPINE's attribute-write pattern and aligns with MASH's Invoke operation for commands that trigger state changes.

**5. Combined Batch 3+4 feature design sketch:**

```
HVAC Feature (new, ID: 0x000A or similar):
  Direction: Bidirectional

  Attributes:
    // System function state (per function type)
    1: systemFunctions[]           // HvacSystemFunction[]
       - functionType              // enum: HEATING, COOLING, DHW, VENTILATION
       - operationMode             // enum: AUTO, ON, OFF, ECO (read/write)
       - supportedModes[]          // enum[]: which modes are available
       - temperatureSetpoint       // int16 (0.1 C): target temp for current mode (read/write)
       - setpointMin               // int16 (0.1 C): minimum allowed
       - setpointMax               // int16 (0.1 C): maximum allowed
       - setpointStep              // uint16 (0.1 C): step size

    // Overrun state
    10: overruns[]                 // HvacOverrun[]
       - overrunType               // enum: ONE_TIME_DHW, SG_READY_1, SG_READY_2, SG_READY_3, SG_READY_4
       - overrunStatus             // enum: INACTIVE, ACTIVE, RUNNING, FINISHED
       - overrunLabel              // string?: human-readable name

  Commands:
    1: SetOperationMode            // Set mode for a system function
    2: SetTemperatureSetpoint      // Set temp target for a mode
    3: StartOverrun                // Trigger an overrun
    4: StopOverrun                 // Cancel an overrun
```

This single feature covers: MoRHSF, MoRCSF, CoRHSF, CoRCSF, MoDHWSF, CoDHWSF, CoRHT, CoRCT, CoDHWT, and MCSGR (SG-Ready). That's 10 of the 14 HVAC use cases. The remaining 4 (MoRT, MoDHWT, MoOT, MoDHWT) are covered by extending Measurement with temperature types.

*Batch 4 complete.*

---

## Batch 5: Inverter / Battery / PV

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 5.1 | MOB | Monitor battery state (SoC, SoH, energy), DC measurements, identification, capabilities, and temperature | Battery (child of Inverter), MonitoringAppliance | Measurement, ElectricalConnection, DeviceClassification, DeviceDiagnosis | S1: Identification (M), S2: Battery state (M), S3: DC power (R), S4: DC current (M), S5: DC voltage (M), S6: DC energy (M), S7: Additional details (R), S8: Capabilities (R), S9: Internal data (O) | **Measurement** + **Status** + **DeviceInfo** + **Electrical** | Mostly covered. Minor: max cycle count/day, DC nominal power split. |
| 5.2 | COB | Control battery charge/discharge via two modes: direct power setpoint or PCC (grid connection point) setpoint, with failsafe and heartbeat | Inverter (addresses aggregated battery function), CEM | DirectControl (Setpoint), DeviceConfiguration, DeviceDiagnosis, ElectricalConnection | S1: Control mode "Power" (M), S2: Control mode "PCC" (M), S3: Configuration parameters (M), S4: Failsafe values (M), S5: Heartbeat (M) | **EnergyControl** + **Measurement** | Partial. **GAP: No PCC control mode (setpoint targets GCP, not device). No explicit control mode selector. No default setpoints (distinct from failsafe).** |
| 5.3 | MOI | Monitor inverter identification (type, manufacturer), operating state, AC power/energy details, power factor, capabilities, temperature | Inverter, MonitoringAppliance | DeviceClassification, DeviceDiagnosis, Measurement, ElectricalConnection | S1: Identification (M), S2: State (M), S3: AC power details (R), S4: AC energy details (R), S5: AC additional details (R), S6: Capabilities (R), S7: Internal data (O) | **DeviceInfo** + **Status** + **Measurement** + **Electrical** | Well covered. Minor: inverter type enum (PV/hybrid/battery) not explicit, monthly/yearly yield not modeled. |
| 5.4 | MPS | Monitor PV string DC power, current, voltage, energy, and capabilities | PVString (child of Inverter), MonitoringAppliance | Measurement, ElectricalConnection, DeviceClassification | S1: DC power (M), S2: DC current (M), S3: DC voltage (M), S4: DC energy (M), S5: Capabilities (R) | **Measurement** + **Electrical** + **DeviceInfo** | Well covered. MASH Measurement has all DC measurement types needed. |
| 5.5 | VABD | Visualize aggregated battery system data: (dis)charge power, cumulated charge/discharge energy, SoC | BatterySystem, VisualizationAppliance | Measurement, ElectricalConnection | S1: (Dis)charge power (M), S2: Cumulated charge energy (M), S3: Cumulated discharge energy (M), S4: SoC (M) | **Measurement** (partial) | **GAP: No aggregation entity type (BatterySystem). Controller must aggregate from individual BATTERY endpoints.** |
| 5.6 | VAPD | Visualize aggregated PV system data: nominal peak power, current production, cumulated yield | PVSystem, VisualizationAppliance | Measurement, ElectricalConnection | S1: Nominal peak power (M), S2: Current power production (M), S3: Cumulated yield (M) | **Measurement** + **Electrical** (partial) | **GAP: No aggregation entity type (PVSystem). Same pattern as VABD.** |

---

### Detailed Analysis

#### 5.1 MOB - Monitoring of Battery

**Goal:** Monitor a battery's operational state, DC-side electrical measurements, identification, capabilities, and internal data (temperature). The battery is always a child of an inverter in the SPINE device hierarchy.

**Actors:**
- Battery -- provides data (server). Always a child entity of an Inverter.
- Monitoring Appliance -- reads/subscribes to data (client). Typically the CEM.

**Entity hierarchy (critical):**
```
Device
├── Entity: Inverter (parent)
│   ├── Entity: Battery 1          ← this actor
│   │   └── Features: Measurement, ElectricalConnection, DeviceClassification, DeviceDiagnosis
│   ├── Entity: Battery 2 (optional)
│   └── Entity: PV String (if hybrid)
```
The Inverter's parent entity type classifies it as "Hybrid Inverter" or "Battery Inverter". Battery is always a child of Inverter, never a top-level entity.

**Scenarios (9 total):**

1. **Monitor Battery identification (M/R)** -- brand, model, serial number via DeviceClassification
2. **Monitor Battery state (M)** -- SoC (%), SoH (%), stateOfEnergy (Wh), usableCapacity (Wh), batteryState (normal/standby/notReady/off/failure)
3. **Monitor Battery DC power (R/M)** -- DC power (charge=positive, discharge=negative)
4. **Monitor Battery DC current (M)** -- DC current
5. **Monitor Battery DC voltage (M)** -- DC voltage
6. **Monitor Battery DC energy (M/R)** -- total DC charge energy, total DC discharge energy (cumulative)
7. **Monitor Battery additional details (R)** -- component temperature
8. **Monitor Battery capabilities (R/M)** -- nominal capacity (Wh), nominal max DC charge/discharge power, max charge/discharge cycle count per day
9. **Monitor Battery internal data (O)** -- vendor-specific internal values

**Key data relationships:**
- usableCapacity = nominalCapacity * SoH
- stateOfEnergy = usableCapacity * SoC
- Battery state enum: Normal Operation, Standby, Temporarily Not Ready, Off, Failure

**SPINE Features used:** Measurement (DC values, SoC/SoH), ElectricalConnection (nominal constraints), DeviceClassification (identification), DeviceDiagnosis (operating state)

**MASH Mapping:**
- S1 (identification) -> DeviceInfo: vendorName, productName, serialNumber
- S2 (state) -> Measurement: stateOfCharge(50), stateOfHealth(51), stateOfEnergy(52), useableCapacity(53) + Status: operatingState
- S3 (DC power) -> Measurement: dcPower(40)
- S4 (DC current) -> Measurement: dcCurrent(41)
- S5 (DC voltage) -> Measurement: dcVoltage(42)
- S6 (DC energy) -> Measurement: dcEnergyIn(43), dcEnergyOut(44)
- S7 (temperature) -> Measurement: temperature(60)
- S8 (capabilities) -> Electrical: energyCapacity(20), nominalMaxConsumption(10), nominalMaxProduction(11)

**Gaps:**
- **Battery state enum mapping.** MOB's battery states (Normal, Standby, Temporarily Not Ready, Off, Failure) map to MASH Status.operatingState (RUNNING, STANDBY, STARTING/PAUSED, OFFLINE, FAULT). Adequate coverage.
- **Max cycle count per day (MOB-084).** MASH has cycleCount in Measurement (cumulative count) but no "max permitted per day" constraint. This is a capacity degradation limit. Low priority, could be added to Electrical.
- **DC nominal power values.** MOB reports nominal max DC charge/discharge power. MASH Electrical has `nominalMaxConsumption`/`nominalMaxProduction` which are generic and work for both AC and DC depending on the endpoint type. A BATTERY endpoint would use these for DC values. Sufficient.
- **Assessment:** Very well covered. MASH Measurement was clearly designed with battery monitoring in mind -- all 9 scenarios have direct attribute mappings.

---

#### 5.2 COB - Control of Battery

**Goal:** CEM controls the charging/discharging behavior of a battery inverter via two control modes: "Power" (direct battery power setpoint) and "PCC" (grid connection point power setpoint). Includes failsafe mechanism, heartbeat, and configurable default/failsafe setpoints.

**Actors:**
- CEM -- sends setpoints and configuration (client)
- Inverter -- receives control commands, manages batteries (server). Addresses the aggregated battery function, not individual batteries.

**Two control modes (critical distinction):**
1. **Power mode:** CEM sets a direct charge/discharge power setpoint for the battery. The inverter charges or discharges at the specified power (AC for battery inverters, DC for hybrid inverters).
2. **PCC mode:** CEM sets a target power at the Grid Connection Point. The inverter self-regulates its battery charge/discharge (and PV contribution in hybrid mode) to achieve the target GCP power. PCC=0 means zero grid exchange (maximize self-consumption).

**State machine:**
```
Init -> Power Control State / PCC Control State / Auto Control State
     -> Auto Uncontrolled State (no CEM communication within 120s)

Power/PCC Control State -> Failsafe State (no heartbeat for 120s)
                        -> Auto Control State (inverter decides)
Failsafe State -> Power/PCC Control (CEM resumes) / Auto Uncontrolled (timeout)
```

**Scenarios:**
1. **Control mode "Power" (M)** -- AC or DC power setpoint (battery inverter: AC, hybrid: DC). Includes setpoint constraints (min/max/step), activation/deactivation, optional duration (auto-expiry), ACK/NACK.
2. **Control mode "PCC" (M)** -- PCC power setpoint + max AC/DC charge/discharge power limits. PCC setpoint is "soft" (exceptions allowed for physical limits).
3. **Configuration parameters (M)** -- Active Control Mode (Power/PCC), Default AC Power, Default DC Power, Default PCC Power. These are the values used when no active setpoint exists.
4. **Failsafe values (M)** -- Failsafe AC/DC/PCC power setpoints (used when heartbeat lost), Failsafe Duration Minimum (2-24 hours).
5. **Heartbeat (M)** -- Bidirectional, at least every 60 seconds. Missing for 120s triggers failsafe.

**Key features:**
- **Active Control Mode selector:** Explicit attribute (Power/PCC) that determines which setpoints apply. Written by CEM.
- **Default setpoints (distinct from failsafe):** Used when CEM has active communication but no active setpoint. Example: CEM switches to Power mode but hasn't sent a setpoint yet -> Default AC/DC Power is used.
- **Failsafe setpoints:** Used when heartbeat is lost. Different values from defaults.
- **Setpoint duration:** Optional, auto-deactivates when expired. Duration counts down regardless of activation state.
- **Max charge/discharge limits in PCC mode:** CEM can cap the battery charge/discharge power while letting the inverter self-regulate the GCP target.

**SPINE Features used:** DirectControl (Setpoint), DeviceConfiguration (active control mode, defaults, failsafe), DeviceDiagnosis (heartbeat), ElectricalConnection (nominal constraints)

**MASH Mapping:**
- S1 (Power mode setpoint) -> EnergyControl: consumptionSetpoint (charge) / productionSetpoint (discharge). SetSetpoint command with duration parameter.
- S2 (PCC mode) -> **No direct mapping.** PCC setpoint targets a different entity (grid connection point), not the device itself.
- S3 (Configuration) -> EnergyControl: controlState partially maps Active Control Mode.
- S4 (Failsafe) -> EnergyControl: failsafeConsumptionLimit(70), failsafeProductionLimit(71), failsafeDuration(72).
- S5 (Heartbeat) -> Transport-level keep-alive + EnergyControl FAILSAFE state.

**Gaps:**
- **PCC control mode has no MASH equivalent.** This is the most significant gap. In PCC mode, the setpoint doesn't target the battery's power -- it targets the power at the Grid Connection Point. The inverter autonomously regulates its battery + PV output to achieve the GCP target. MASH EnergyControl setpoints target the device itself. Possible approaches:
  1. **Add a controlMode attribute to EnergyControl** with values like POWER, PCC, AUTO. When mode=PCC, the consumptionSetpoint/productionSetpoint are interpreted as GCP targets rather than device targets.
  2. **Use a separate signal/feature** for PCC targets (e.g., a Signal slot targeting the GRID_CONNECTION endpoint).
  3. **Make PCC mode implicit** in the inverter type -- battery inverter uses AC setpoints directly, hybrid inverter interprets setpoints differently.
  - **Recommendation:** Option 1 is cleanest. A simple `controlMode` enum on EnergyControl allows the CEM to select how the inverter interprets setpoints.

- **Default setpoints (distinct from failsafe).** COB has three layers: active setpoint > default setpoint > failsafe setpoint. MASH EnergyControl has: active setpoint > failsafe limit. There's no "default" value for when communication is active but no setpoint has been sent. This could be addressed by:
  1. Adding `defaultConsumptionSetpoint`/`defaultProductionSetpoint` attributes
  2. Treating null setpoint as "device decides" (equivalent to COB's Auto Control State)
  - **Assessment:** MASH's approach of null=autonomous is simpler and sufficient. The CEM can always write a setpoint. The COB "default" is mainly a safety net for the transition period between mode selection and setpoint activation, which MASH handles by having the device remain in its current state until a setpoint arrives.

- **Max charge/discharge limits in PCC mode.** COB-022 to COB-025 are PCC-mode-specific limits on battery charge/discharge power. These are constraints on the device's internal behavior, not grid-facing limits. MASH EnergyControl has power limits but they're grid-facing. If PCC mode were added, these could be modeled as device-internal constraints on a BATTERY sub-endpoint.

- **ACK/NACK for setpoints.** COB requires explicit acceptance/rejection of setpoints. MASH's SetSetpoint command already returns a response with `applied` boolean and `rejectReason`. Covered at protocol level.

- **Setpoint duration (auto-expiry).** COB setpoints can have durations. MASH's SetSetpoint command already has a `duration` parameter. Covered.

---

#### 5.3 MOI - Monitoring of Inverter

**Goal:** Monitor an inverter's identification (type, manufacturer), operating state, AC-side power and energy details, power factor, capabilities (nominal min/max power), and internal temperature.

**Actors:**
- Inverter -- provides data (server). Is the parent entity for Batteries and PV Strings.
- Monitoring Appliance -- reads/subscribes (client). CEM or UI.

**Entity hierarchy:**
```
Device
├── Parent entity type: "Photovoltaic Inverter" / "Hybrid Inverter" / "Battery Inverter"
│   └── Entity: Inverter
│       ├── Entity: PV String 1..n (if PV or hybrid)
│       └── Entity: Battery 1..m (if battery or hybrid)
```
The inverter's parent entity type (ElectricityGenerationSystem / PVESHybrid / ElectricalStorage) classifies whether it's a PV-only, hybrid, or battery-only inverter.

**Scenarios (7 total):**

1. **Monitor Inverter identification (M)** -- vendor name, model, serial number, **inverter type** (PV/hybrid/battery)
2. **Monitor Inverter state (M)** -- operating state (normal/standby/error/failure), last error code
3. **Monitor Inverter AC power details (R)** -- apparent power, reactive power (total + phase-specific). Note: basic AC active power is covered by MPC, not MOI.
4. **Monitor Inverter AC energy details (R)** -- consumed energy, produced energy, monthly yield, yearly yield
5. **Monitor Inverter AC additional details (R)** -- power factor, grid frequency (if not covered by MPC)
6. **Monitor Inverter capabilities (R)** -- nominal max production, nominal max consumption, nominal min production, nominal min consumption
7. **Monitor Inverter internal data (O)** -- internal component temperature

**MASH Mapping:**
- S1 (identification) -> DeviceInfo: vendorName, productName, serialNumber
- S2 (state) -> Status: operatingState, faultCode
- S3 (AC power details) -> Measurement: acReactivePower(2), acApparentPower(3), acReactivePowerPerPhase(11), acApparentPowerPerPhase(12)
- S4 (AC energy details) -> Measurement: acEnergyConsumed(30), acEnergyProduced(31)
- S5 (AC additional details) -> Measurement: powerFactor(24), acFrequency(23)
- S6 (capabilities) -> Electrical: nominalMaxConsumption(10), nominalMaxProduction(11), nominalMinPower(12)
- S7 (internal data) -> Measurement: temperature(60)

**Gaps:**
- **Inverter type (PV/hybrid/battery).** SPINE uses the parent entity type to classify the inverter. MASH has a single INVERTER endpoint type. The inverter's child endpoints implicitly reveal the type (has PV_STRING children = PV or hybrid; has BATTERY children = battery or hybrid). An explicit `inverterType` attribute could be added to Electrical or DeviceInfo, but it's not strictly necessary -- the device model structure conveys this. Low priority.
- **Monthly/yearly yield.** MOI has acYieldMonth and acYieldYear. MASH Measurement only has cumulative totals (acEnergyConsumed, acEnergyProduced). Period-based yield is a historical/aggregation feature. Same gap as MHPC. Low priority -- the controller can compute these from cumulative values.
- **Assessment:** Very well covered. All core monitoring scenarios have direct MASH attribute mappings.

---

#### 5.4 MPS - Monitoring of PV String

**Goal:** Monitor a PV string's DC power, current, voltage, energy, and capabilities. The PV string is always a child of an inverter.

**Actors:**
- PV String -- provides DC-side measurements (server). Always a child entity of an Inverter.
- Monitoring Appliance -- reads/subscribes (client).

**Entity hierarchy:**
```
Device
├── Entity: Inverter (parent)
│   ├── Entity: PV String 1         ← this actor
│   │   └── Features: Measurement, ElectricalConnection, DeviceClassification
│   └── Entity: PV String 2 (optional)
```
PV Strings are children of the Inverter, similar to Batteries.

**Scenarios (5 total):**

1. **Monitor PV String DC power (M)** -- DC power production
2. **Monitor PV String DC current (M)** -- DC current
3. **Monitor PV String DC voltage (M)** -- DC voltage
4. **Monitor PV String DC energy (M/R)** -- total produced energy (cumulative since install/reset)
5. **Monitor PV String capabilities (R)** -- nominal max DC power (peak power)

**MASH Mapping:**
- S1 -> Measurement: dcPower(40)
- S2 -> Measurement: dcCurrent(41)
- S3 -> Measurement: dcVoltage(42)
- S4 -> Measurement: dcEnergyOut(44) (energy produced by PV)
- S5 -> Electrical: nominalMaxProduction(11)

**Gaps:** None significant. MASH Measurement and Electrical fully cover all MPS scenarios. The PV_STRING endpoint type already exists.

---

#### 5.5 VABD - Visualization of Aggregated Battery Data

**Goal:** A Visualization Appliance reads aggregated battery system data for user display: total (dis)charge power, cumulated charge/discharge energy, and overall SoC. The BatterySystem entity aggregates data from one or more battery inverters.

**Actors:**
- Battery System -- aggregates data from multiple inverter+battery combinations (server)
- Visualization Appliance -- reads aggregated data (client)

**Entity hierarchy:**
```
Device
├── Entity: BatterySystem            ← aggregation entity
│   └── Features: Measurement, ElectricalConnection
```
BatterySystem is a **system-level aggregation entity**. It's not a physical device but a logical entity that combines data from multiple inverters and batteries into a single view.

**Scenarios (4 total):**

1. **Monitor current Battery System (dis)charge power (M)** -- aggregated AC power (positive=charging, negative=discharging)
2. **Monitor cumulated Battery System charge energy (M)** -- total energy charged since reset
3. **Monitor cumulated Battery System discharge energy (M)** -- total energy discharged since reset
4. **Monitor current state of charge of Battery System (M)** -- percentage (0-100%)

**SPINE Features used:** Measurement, ElectricalConnection

**MASH Mapping:** Measurement feature covers all data points (acActivePower, dcEnergyIn, dcEnergyOut, stateOfCharge).

**Gaps:**
- **No aggregation entity type.** MASH has no BatterySystem or equivalent system-level endpoint type. MASH endpoints represent physical functional units (BATTERY, INVERTER), not system-level aggregations.
- **Assessment:** This is an architectural question, not a feature gap. Two approaches:
  1. **Controller-side aggregation (recommended for v1).** The CEM reads data from individual BATTERY endpoints and computes the aggregated view. This keeps the protocol simple and avoids redundant data.
  2. **Optional aggregation endpoint.** A device with multiple inverters could expose a virtual endpoint (e.g., type BATTERY with a flag indicating it's aggregated) that reports pre-computed totals. This helps simpler visualization appliances.
- The data itself (power, energy, SoC) is fully supported by existing Measurement attributes.

---

#### 5.6 VAPD - Visualization of Aggregated Photovoltaic Data

**Goal:** A Visualization Appliance reads aggregated PV system data: nominal peak power, current power production, and cumulated energy yield. The PVSystem entity aggregates data from the AC side of one or more PV inverters.

**Actors:**
- PV System -- aggregates data from multiple PV inverters (server)
- Visualization Appliance -- reads aggregated data (client)

**Entity hierarchy:**
```
Device
├── Entity: PVSystem                  ← aggregation entity
│   └── Features: Measurement, ElectricalConnection
```
PVSystem is a system-level aggregation entity similar to BatterySystem.

**Scenarios (3 total):**

1. **Monitor nominal peak power (M)** -- total installed PV peak power (Wp) across all inverters
2. **Monitor current photovoltaic power production (M)** -- aggregated AC power production
3. **Monitor cumulated photovoltaic yield (M)** -- total energy produced since installation/reset

**SPINE Features used:** Measurement, ElectricalConnection

**MASH Mapping:**
- S1 -> Electrical: nominalMaxProduction(11)
- S2 -> Measurement: acActivePower(1) (negative = production)
- S3 -> Measurement: acEnergyProduced(31)

**Gaps:**
- **No aggregation entity type.** Same as VABD. No PVSystem endpoint type in MASH.
- **Assessment:** Same recommendation as VABD: controller-side aggregation for v1. The individual PV_STRING and INVERTER endpoints provide all raw data needed.

---

### Batch 5 SPINE-to-MASH Feature Type Mapping

| SPINE Feature | Used By | MASH Feature | Notes |
|--------------|---------|-------------|-------|
| Measurement (DC values) | MOB, MPS | Measurement | DC power, current, voltage, energy fully covered (attrs 40-44) |
| Measurement (battery state) | MOB, VABD | Measurement | SoC, SoH, stateOfEnergy, useableCapacity, cycleCount (attrs 50-54) |
| Measurement (temperature) | MOB (S7), MOI (S7) | Measurement | temperature(60) -- covered for component temp |
| Measurement (AC values) | MOI, VABD, VAPD | Measurement | Active/reactive/apparent power, energy (attrs 1-31) |
| ElectricalConnection | All | Electrical | Nominal min/max power, capacity |
| DeviceClassification | MOB, MOI, MPS | DeviceInfo | Manufacturer, model, serial |
| DeviceDiagnosis | MOB, MOI, COB | Status + Transport | Operating state, heartbeat |
| DirectControl (Setpoint) | COB | EnergyControl | Power setpoints with duration |
| DeviceConfiguration | COB | EnergyControl (partial) | **Active control mode, default setpoints -- partially mapped** |

### Batch 5 Entity Type Mapping

| SPINE Entity Type | Used By | MASH Endpoint Type | Notes |
|------------------|---------|-------------------|-------|
| Inverter | MOI, COB | INVERTER (0x02) | Good match |
| Battery | MOB | BATTERY (0x04) | Good match |
| PVString | MPS | PV_STRING (0x03) | Good match |
| BatterySystem | VABD | None | **Aggregation entity -- controller responsibility** |
| PVSystem | VAPD | None | **Aggregation entity -- controller responsibility** |
| ElectricalStorage | (parent of battery inverter) | None | Implicit via device model (INVERTER with BATTERY children) |
| PVESHybrid | (parent of hybrid inverter) | None | Implicit via device model (INVERTER with both children) |
| ElectricityGenerationSystem | (parent of PV inverter) | None | Implicit via device model (INVERTER with PV_STRING children) |

### Batch 5 Gap Summary

| Priority | Gap | Affected UCs | Recommendation |
|----------|-----|-------------|----------------|
| **Medium** | **PCC control mode (setpoint targets GCP, not device)** | COB | Add `controlMode` enum (POWER, PCC, AUTO) to EnergyControl. When mode=PCC, setpoints are interpreted as GCP power targets. |
| Low | Default setpoints (distinct from failsafe) | COB | MASH's null=autonomous approach is sufficient. CEM can always explicitly write setpoints. No change needed. |
| Low | Max charge/discharge limits in PCC mode | COB | Can be modeled as EnergyControl limits on the BATTERY sub-endpoint if PCC mode is added. |
| Low | Inverter type classification (PV/hybrid/battery) | MOI | Implicitly conveyed by device model structure (which child endpoints exist). No change needed for v1. |
| Low | Monthly/yearly yield values | MOI | Same historical data gap as MHPC. Controller computes from cumulative values. |
| Low | Max cycle count per day | MOB | Could add to Electrical if needed. Low priority niche parameter. |
| Low | No BatterySystem/PVSystem aggregation entity | VABD, VAPD | Controller-side aggregation for v1. No protocol change needed. |
| Low | Entity hierarchy (parent-child for Inverter>Battery/PVString) | MOB, MPS, COB | Same `parentEndpoint` recommendation as Batch 3. Useful here to express Inverter>Battery/PVString nesting. |

---

### Batch 5 Key Observations

**1. MASH was designed for this batch.**

The Measurement feature's attribute layout (IDs 40-60: DC measurements, battery state, temperature) and the existing BATTERY/INVERTER/PV_STRING endpoint types demonstrate that inverter/battery/PV monitoring was a primary design target. Of all 6 batches, this one has the fewest gaps.

**2. COB's PCC mode is the only significant gap.**

PCC (Point of Common Coupling) mode is a fundamentally different control paradigm: instead of the CEM telling the battery "charge at 5 kW," the CEM tells the inverter "target 0 W at the grid connection point and figure out the battery charge/discharge yourself." This is valuable for self-consumption optimization because the inverter can react faster to PV/load changes than the CEM's control loop.

MASH can support this with a simple `controlMode` enum on EnergyControl. When mode=PCC, the existing setpoint attributes are reinterpreted as GCP targets. The inverter's EnergyControl response already includes `applied` and `effectiveConsumptionLimit`/`effectiveProductionLimit`, which can report the actual battery behavior.

**3. Aggregation is a controller concern, not a protocol concern.**

VABD and VAPD use system-level aggregation entities (BatterySystem, PVSystem) that combine data from multiple physical devices. This is a fundamentally different pattern from MASH's device-centric model. In MASH, the controller (EMS) reads individual device endpoints and performs aggregation itself. This is the correct architectural split:
- **Device protocol:** reports raw device data
- **Controller/UI:** aggregates, visualizes, and acts on the data

Adding aggregation endpoints would create redundant data paths and add complexity to the constrained-device protocol. If a device manufacturer wants to expose pre-aggregated data, they can use a virtual endpoint with existing features.

**4. Entity hierarchy reinforcement.**

The Inverter > Battery/PVString parent-child relationship provides another use case for the `parentEndpoint` field recommended in Batch 3. While MASH can work without it (a device with one INVERTER, two BATTERY, and three PV_STRING endpoints is unambiguous), multi-inverter devices need explicit parent-child links to associate batteries/strings with their inverter.

**5. The three-tier setpoint model (COB) vs. MASH's two-tier model.**

COB has: Active Setpoint > Default Setpoint > Failsafe Setpoint
MASH has: Active Setpoint > Failsafe Limit

The "default setpoint" in COB is the value used when communication is active but no setpoint has been explicitly activated. In MASH, this state is simply "no active setpoint" (null), and the device operates autonomously. This is a simpler model that achieves the same effect -- the CEM writes a setpoint when it wants control, and the device self-manages otherwise. No change needed.

*Batch 5 complete.*

---

## Batch 6: Generic + Cross-Cutting Synthesis

### Summary Table

| # | UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps |
|---|-----|------|---------------|---------------|-----------|-------------|------|
| 6.1 | NID | Identify a node: manufacturer data (name, brand, serial, software/hardware revision, vendor code, power source, label, description) | DeviceInformation (any node) | DeviceClassification | S1: Get manufacturer data (M) | **DeviceInfo** | Minor: `powerSource` enum and `brandName` not explicit; `nodeIdentification` (UUID) covered by `deviceId`. |
| 6.2 | OHPCF | CEM monitors and controls heat pump compressor's optional power consumption process (availability, power, state, pause/resume/stop) | Compressor, CEM | SmartEnergyManagementPs (optional sequence-based immediate control), OperatingConstraints | S1: Monitor compressor flexibility (M), S2: Control compressor flexibility (M) | **EnergyControl** (processState, optionalProcess, isPausable, isStoppable, pause/resume/stop) | Partial: **GAP: No operating constraints (min on/off durations). No power approximation indicator. No explicit optional-process power value.** |

---

### Detailed Analysis

#### 6.1 NID - Node Identification

**Goal:** Retrieve manufacturer data from any SPINE node to identify the device: what it is, who made it, and how to distinguish it from other nodes.

**Actors:**
- Identifiable Node -- any device providing identification data (server)
- Identifying Appliance -- CEM, UI, or commissioning tool (client)

**Entity hierarchy:** Simple. The DeviceInformation entity sits at the device root level.

**Scenario 1 -- Get manufacturer data (M):**

| Data point | Support | Description |
|-----------|---------|-------------|
| Device Name | M | Name as indicated on housing or manual |
| Device Code | O | Unique model identifier |
| Serial Number | M | Unique serial number per manufacturer |
| Software Revision | O | Current software version |
| Hardware Revision | O | Current hardware version |
| Vendor Name | O | Manufacturer name |
| Vendor Code | O | Unique vendor identifier |
| Brand Name | M | Brand name (may differ from vendor) |
| Power Source | O | Electrical power source: unknown, mains-1ph, mains-3ph, battery, DC |
| Node Identification | O | Globally unique UUID (manufacturer-assigned) |
| Label | O | Short label defined by manufacturer |
| Description | O | Description defined by manufacturer |

**SPINE Features used:** DeviceClassification (deviceClassificationManufacturerData)

**MASH Mapping:**
- Device Name -> DeviceInfo: `productName` (3)
- Device Code -> DeviceInfo: `productId` (6)
- Serial Number -> DeviceInfo: `serialNumber` (4)
- Software Revision -> DeviceInfo: `softwareVersion` (10)
- Hardware Revision -> DeviceInfo: `hardwareVersion` (11)
- Vendor Name -> DeviceInfo: `vendorName` (2)
- Vendor Code -> DeviceInfo: `vendorId` (5)
- Brand Name -> No direct mapping. `vendorName` covers manufacturer but not brand.
- Power Source -> No direct mapping. Not in DeviceInfo.
- Node Identification (UUID) -> DeviceInfo: `deviceId` (1) -- globally unique identifier
- Label -> DeviceInfo: `label` (31)
- Description -> No direct mapping. No `description` field in DeviceInfo.

**Gaps:**
- **Brand Name vs Vendor Name.** NID distinguishes brand (marketing name) from vendor (manufacturer). MASH has only `vendorName`. In most cases these are identical. For white-label products they differ (e.g., vendor="Stiebel Eltron", brand="Bosch"). Could add optional `brandName` to DeviceInfo. Low priority.
- **Power Source.** NID reports the device's electrical power source (mains 1-phase, mains 3-phase, battery, DC). This information is partially available from Electrical feature (`phaseCount` implies 1-ph/3-ph, `supportedDirections` implies whether it's a source or sink). An explicit `powerSource` enum could be useful for discovery but is not critical.
- **Description.** NID has a manufacturer-defined description field. MASH DeviceInfo has `label` (user-assigned) but no manufacturer description. Low priority -- metadata that rarely affects energy management.
- **Assessment:** NID maps very well to MASH DeviceInfo. All mandatory data points (Device Name, Serial Number, Brand Name) have close equivalents. The gaps are minor optional enrichment fields. NID v1.1 adds no new mandatory data points.

---

#### 6.2 OHPCF - Optimization of Self-Consumption by Heat Pump Compressor Flexibility

**Goal:** CEM monitors whether a heat pump compressor has an optional power consumption process available (e.g., DHW pre-heating when PV surplus exists), and if so, can schedule, start, pause, resume, or stop that process to optimize self-consumption.

**Actors:**
- Compressor -- announces optional power consumption availability, executes process (server)
- CEM -- decides whether/when to activate the process (client)

**Entity hierarchy:**
```
Device
├── Entity: DeviceInformation
│   └── Feature: NodeManagement
└── Entity: Compressor                    ← the actor
    └── Feature: SmartEnergyManagementPs (server)
        └── Specialization: OptionalSequenceBasedImmediateControl
```
Simple one-level hierarchy. The Compressor entity is a direct child of Device.

**Scenario 1 -- Monitor compressor flexibility (M):**

Phase A -- Announcement of optional process:
1. Availability of an optional consumption (boolean: process is available or not)
2. Power value -- "good approximation" or "maximum power" of the process
3. The consumption will NOT be started autonomously (CEM decides)
4. Duration of consumption is unknown (cannot be predicted)
5. Whether CEM may stop the process (isStoppable)
6. Whether CEM may pause and resume (isPausable)
7. At least one of stop/pause must be supported

Phase B -- Report of active process state:
- State transitions: inactive -> scheduled -> running -> paused -> running -> completed
- CEM observes state changes via subscription

**Scenario 2 -- Control compressor flexibility (M):**
- CEM writes schedule start time (immediate or future)
- CEM can pause a running process
- CEM can resume a paused process
- CEM can stop (abort) a running or paused process
- All subject to operating constraints:
  - `activeDurationMin` -- minimum time the compressor must run before it can be stopped
  - `activeDurationMax` -- maximum time the compressor can run
  - `pauseDurationMin` -- minimum pause duration before restart
  - `pauseDurationMax` -- maximum pause duration

**Key design features:**
- **Optional process model:** The compressor doesn't *need* to run -- it *can* run if the CEM decides it's beneficial (e.g., PV surplus available). This is fundamentally different from mandatory processes.
- **No energy forecast:** Heat pump compressors cannot predict how long they'll run or how much energy they'll consume. The process has unknown duration.
- **Constraints protect the device:** Min on/off durations prevent damage to the compressor (short-cycling).

**SPINE Features used:** SmartEnergyManagementPs (complex class wrapping PowerSequences + OperatingConstraints + Setpoint functionality)

**MASH Mapping:**
- Process availability -> EnergyControl: `optionalProcess` (81) = true, `processState` (80) = AVAILABLE
- Process state -> EnergyControl: `processState` -- AVAILABLE, SCHEDULED, RUNNING, PAUSED, COMPLETED, ABORTED
- isPausable -> EnergyControl: `isPausable` (14)
- isStoppable -> EnergyControl: `isStoppable` (16)
- Pause command -> EnergyControl: `pause` command (9)
- Resume command -> EnergyControl: `resume` command (10)
- Stop command -> EnergyControl: `stop` command (11)
- Schedule start -> No direct mapping. EnergyControl lacks `AdjustStartTime`-like scheduling. However, `isShiftable` (15) exists.

**Gaps:**
- **Operating constraints (min on/off durations).** OHPCF requires `activeDurationMin`, `pauseDurationMin`, `activeDurationMax`, `pauseDurationMax` to prevent compressor short-cycling. MASH EnergyControl has no equivalent. These are critical for device protection.
  - **Recommendation:** Add operating constraint attributes to EnergyControl:
    - `minRunDuration` (uint32, seconds) -- minimum run time before pause/stop
    - `minPauseDuration` (uint32, seconds) -- minimum pause before restart
    - `maxRunDuration` (uint32, seconds, nullable) -- maximum run time
    - `maxPauseDuration` (uint32, seconds, nullable) -- maximum pause time

- **Optional process power value.** OHPCF announces the expected power consumption when the process runs. MASH has no way for a device to advertise "if you start this process, it will consume approximately X watts." This could be a new attribute on EnergyControl or Electrical.
  - **Recommendation:** Add optional `optionalProcessPower` (int64, mW) to EnergyControl -- the expected power consumption of the optional process.

- **Power approximation indicator.** OHPCF distinguishes "good approximation" from "maximum power" for the announced value. Minor -- the CEM can treat any value as approximate.

- **Schedule start time.** OHPCF allows the CEM to write a start time for the process. MASH's `isShiftable` (15) attribute exists but there's no explicit `AdjustStartTime` command. The CEM could set the start time via a hypothetical command parameter.
  - **Assessment:** The pause/resume/stop commands cover runtime control. Schedule start could be added as a parameter to a new `start` command or as an attribute. Medium priority.

- **Assessment:** MASH EnergyControl was designed with OHPCF in mind (the ProcessStateEnum and optionalProcess attributes are direct mappings). The main gap is operating constraints -- min/max on/off durations that protect the compressor from short-cycling. This is a small but important addition.

---

### 6.3 Cross-Cutting: Entity Hierarchy Needs

Entity hierarchy (`parentEndpoint`) was identified as a gap in multiple batches:

| Batch | Use Cases | Hierarchy Needed | Depth |
|-------|----------|-----------------|-------|
| 3 | MoRHSF, MoRCSF, MoRT | HeatingCircuit > HeatingZone > HVACRoom | 3 levels |
| 4 | All config UCs + VHAN | Same as Batch 3 + names at each level | 3 levels |
| 5 | MOB, MPS, COB | Inverter > Battery, Inverter > PVString | 2 levels |

**Summary of hierarchy patterns:**

1. **HVAC topology:** HeatingCircuit > HeatingZone > HVACRoom (deepest: 3 levels). Also: DHWCircuit as direct child of Device (1 level). This models the physical heating system topology: circuits carry thermal fluid to zones, zones contain rooms.

2. **Inverter/battery topology:** Inverter > Battery(1..n), Inverter > PVString(1..n). This models the electrical topology: inverter manages batteries and PV strings on its DC bus. Multi-inverter devices need explicit parent links to associate DC components with their inverter.

3. **Aggregation (not hierarchy):** BatterySystem, PVSystem, GridConnectionPointOfPremises. These are system-level aggregation entities that combine data from multiple physical devices. Handled by controller-side aggregation, not endpoint hierarchy.

**Recommendation:** Add optional `parentEndpoint` (uint16, nullable) to the endpoint descriptor in DeviceInfo's `endpoints` array. This is a minimal change:
- If null: endpoint is a direct child of Device root (current behavior)
- If set: endpoint is a child of the referenced endpoint
- Maximum practical depth: 3 (HVAC), typically 1-2

This single field enables all hierarchy use cases above. No MASH protocol changes beyond adding one optional field to the endpoint descriptor.

---

### 6.4 Cross-Cutting: SPINE-to-MASH Feature Type Mapping

Comprehensive mapping of all 32 SPINE feature types encountered across all use cases:

| SPINE Feature Type | MASH Feature | Coverage | Notes |
|-------------------|-------------|----------|-------|
| **LoadControl** | EnergyControl | **Good** | Limits + state machine. Core mapping. |
| **DirectControl** | EnergyControl | **Good** | Power/current setpoints. |
| **DeviceConfiguration** | EnergyControl + Electrical | **Partial** | Failsafe values -> EnergyControl. Contractual max -> EnergyControl. Labels -> DeviceInfo. Some config has no mapping. |
| **DeviceDiagnosis** | Status + Transport | **Good** | Operating state -> Status. Heartbeat -> transport keep-alive. |
| **DeviceClassification** | DeviceInfo | **Good** | Manufacturer data, identification. |
| **ElectricalConnection** | Electrical | **Good** | Phase config, nominal values, power ranges. |
| **Measurement** | Measurement | **Partial** | Electrical quantities: full coverage. Temperature: **gap**. |
| **TimeSeries** | Signals + Plan | **Partial** | Forecast/constraint curves -> Signals. Historical data: **gap**. |
| **IncentiveTable** | Signals + Tariff | **Partial** | Simple pricing: good. Per-slot power tiers: **gap**. |
| **PowerSequences** | Plan | **Partial** | Power plans: good. Negotiation: **gap**. |
| **SmartEnergyManagementPs** | EnergyControl | **Partial** | Process state/control: good. Operating constraints (min on/off): **gap**. |
| **OperatingConstraints** | EnergyControl (partial) | **Gap** | Min/max run/pause durations not modeled. |
| **HVAC** | None | **No mapping** | Operation modes, system functions, overruns. **Needs new feature.** |
| **Setpoint** | EnergyControl (power only) | **Partial** | Power setpoints: good. Temperature setpoints: **gap**. |
| **Identification** | ChargingSession | **Good** | EV identification (RFID, contract cert). |
| **Bill** | None | **No mapping** | Charging cost summary. Low priority. |
| **TariffInformation** | Tariff | **Good** | Static tariff structure. |
| **Alarm** | Status (partial) | **N/A** | Not directly used by analyzed UCs. Faults map to Status. |
| **NodeManagement** | Transport layer | **N/A** | Connection management, not a feature concern. |
| **NetworkManagement** | Transport layer | **N/A** | Network-level concern. |
| **Surrogate** | N/A | **N/A** | SPINE-internal proxy mechanism. Not applicable to MASH. |
| **StateInformation** | Status | **Partial** | General state reporting. |
| **SupplyCondition** | Signals | **Partial** | Grid condition signals. |
| **Threshold** | None | **N/A** | Not directly used by analyzed UCs. |
| **TimeInformation** | None | **N/A** | Time sync. MASH assumes NTP or similar external sync. |
| **TimeTable** | None | **N/A** | Schedule tables. Not directly used. |
| **TaskManagement** | None | **N/A** | SPINE-internal. |
| **ActuatorLevel** | None | **N/A** | Generic level actuator. Not used by analyzed UCs. |
| **ActuatorSwitch** | None | **N/A** | Generic switch actuator. Not used by analyzed UCs. |
| **DataTunneling** | None | **N/A** | Vendor-specific tunneling. Not applicable. |
| **Generic** | None | **N/A** | SPINE catch-all. |
| **Messaging** | None | **N/A** | User messaging. Not applicable. |
| **Sensing** | None | **N/A** | Generic sensing. Temperature covered by Measurement extension. |

**Key findings:**
- **9 of 32 SPINE features have good or full MASH coverage** (LoadControl, DirectControl, DeviceClassification, DeviceDiagnosis, ElectricalConnection, Identification, TariffInformation, and partial Measurement/TimeSeries).
- **5 SPINE features have partial MASH coverage** needing extensions (Measurement for temperature, IncentiveTable for per-slot tiers, PowerSequences for negotiation, SmartEnergyManagementPs for constraints, Setpoint for temperature).
- **1 SPINE feature has no MASH equivalent and needs a new feature** (HVAC).
- **17 SPINE features are not directly used** by the analyzed use cases or are infrastructure-level (NodeManagement, NetworkManagement, etc.).

**MASH feature utilization (9 features -> 32 SPINE features):**

| MASH Feature | Covers SPINE Features |
|-------------|----------------------|
| DeviceInfo | DeviceClassification, DeviceConfiguration (labels) |
| Status | DeviceDiagnosis, StateInformation, Alarm (partial) |
| Electrical | ElectricalConnection, DeviceConfiguration (power ranges) |
| Measurement | Measurement (electrical), Measurement (thermal -- gap) |
| EnergyControl | LoadControl, DirectControl, DeviceConfiguration (failsafe), SmartEnergyManagementPs, Setpoint (power) |
| ChargingSession | Identification (EV), Bill (partial -- gap) |
| Tariff | TariffInformation, IncentiveTable (partial) |
| Signals | TimeSeries, IncentiveTable (partial), SupplyCondition |
| Plan | PowerSequences |

MASH achieves approximately 9:1 compression (9 MASH features cover the functionality of 15 actively-used SPINE features), which aligns with the design goal of fewer, more general features.

---

### 6.5 Cross-Cutting: Endpoint Type Gaps

**Current MASH endpoint types (11):** DEVICE_ROOT, GRID_CONNECTION, INVERTER, PV_STRING, BATTERY, EV_CHARGER, HEAT_PUMP, WATER_HEATER, HVAC, APPLIANCE, SUB_METER

**SPINE entity types used by analyzed use cases, grouped by mapping status:**

**Well-mapped (SPINE -> MASH):**

| SPINE Entity | MASH Endpoint | Used By |
|-------------|--------------|---------|
| Inverter | INVERTER | MOI, COB |
| Battery | BATTERY | MOB |
| PVString | PV_STRING | MPS |
| EVSE | EV_CHARGER | EVSECC, EVCEM, CEVC |
| HeatPumpAppliance | HEAT_PUMP | MCSGR |
| GridConnectionPointOfPremises | GRID_CONNECTION | MGCP |
| ControllableSystem | (any endpoint with EnergyControl) | LPC, LPP |
| SmartEnergyAppliance | (any endpoint with EnergyControl) | ITPCM, FLOA |
| DeviceInformation | DEVICE_ROOT | NID, all UCs |

**Mapped with caveats:**

| SPINE Entity | MASH Endpoint | Caveat |
|-------------|--------------|--------|
| HVACRoom | HVAC | MASH HVAC exists but has no room semantics yet. Needs HVAC feature. |
| DHWCircuit | WATER_HEATER | WATER_HEATER is appliance-level; DHWCircuit is subsystem-level. Adequate for simple cases; hierarchy needed for complex systems. |
| EV | (proxied via EV_CHARGER) | MASH communicates with EVSE, not directly with EV. EV data is proxied. Correct for physical topology. |

**Entities requiring new endpoint types (candidates):**

| SPINE Entity | Proposed MASH Type | Priority | Justification |
|-------------|-------------------|----------|---------------|
| HeatingCircuit | (use HVAC with hierarchy) | Low | Not a separate endpoint type -- use HVAC endpoint with `parentEndpoint`. Heating circuits are parents in the hierarchy, not functional endpoints. |
| HeatingZone | (use HVAC with hierarchy) | Low | Same as HeatingCircuit -- a structural level, not a functional endpoint. |
| TemperatureSensor | SENSOR or reuse HVAC | Low | Only used by MoOT. Could be a generic SENSOR endpoint type or handled by attaching Measurement to an HVAC endpoint. |
| Compressor | (use HEAT_PUMP) | Low | Compressor is a sub-component of heat pump. OHPCF operates on HEAT_PUMP endpoint with process control features. No separate type needed. |

**Entities that are NOT endpoint types (controller-side or structural):**

| SPINE Entity | Reason |
|-------------|--------|
| BatterySystem | Aggregation entity. Controller responsibility. |
| PVSystem | Aggregation entity. Controller responsibility. |
| CEM / EnergyGuard / MonitoringAppliance | Controller roles, not device endpoint types. |
| Compressor | Sub-component, not a separate endpoint. |
| ElectricalStorage / ElectricityGenerationSystem / PVESHybrid | Parent entity type classifiers for inverters. Implicit via device model (children reveal type). |
| Household / Surrogate | SPINE-specific concepts. Not applicable. |

**Recommendation:** No new endpoint types needed for v1. The current 11 types cover all functional use cases when combined with:
1. `parentEndpoint` for hierarchy (HVAC nesting, Inverter>Battery/PVString)
2. `endpointLabel` for human-readable names (VHAN)
3. The existing HVAC endpoint type repurposed with the new HVAC feature for room-level entities

If demand arises, a generic SENSOR endpoint type (0x0B) could be added later for standalone temperature/humidity sensors. But for now, an HVAC or APPLIANCE endpoint with a Measurement feature carrying temperature values is sufficient.

---

### 6.6 Cross-Cutting: New MASH Feature Candidates

Based on all 6 batches, these are the identified candidates for new or extended MASH features:

#### Candidate 1: HVAC Feature (new -- HIGH priority)

**Purpose:** Model HVAC system functions, operation modes, temperature setpoints, and overruns.

**Covers:** MoRHSF, MoRCSF, CoRHSF, CoRCSF, MoDHWSF, CoDHWSF, CoRHT, CoRCT, CoDHWT, MCSGR (10 use cases across Batches 3+4)

**Design sketch (from Batch 4 analysis):**
```
HVAC Feature (new, suggested ID: 0x0A):
  Attributes:
    systemFunctions[]     -- array of {functionType, operationMode, supportedModes[],
                             temperatureSetpoint, setpointMin, setpointMax, setpointStep}
    overruns[]            -- array of {overrunType, overrunStatus, overrunLabel}

  Commands:
    SetOperationMode      -- set mode for a system function
    SetTemperatureSetpoint -- set temperature for current mode
    StartOverrun          -- trigger a temporary override
    StopOverrun           -- cancel a temporary override
```

**Rationale:** This is the largest feature gap. Without it, 14 of 42 analyzed use cases (all HVAC monitoring and configuration) have no MASH mapping. A single feature with ~10 attributes and 4 commands covers all of them.

#### Candidate 2: Measurement Feature Extension (extend -- HIGH priority)

**Purpose:** Add temperature measurement types to existing Measurement feature.

**Covers:** MoRT, MoDHWT, MoOT (3 use cases in Batch 3)

**Required additions:**
```
Measurement feature (extend):
  New attributes:
    61: roomTemperature      -- int16, centi-C, nullable (room air temperature)
    62: outdoorTemperature   -- int16, centi-C, nullable (outdoor air temperature)
    63: waterTemperature     -- int16, centi-C, nullable (DHW / water temperature)
    64: supplyTemperature    -- int16, centi-C, nullable (heating supply flow)
    65: returnTemperature    -- int16, centi-C, nullable (heating return flow)
```

**Rationale:** Extending the existing Measurement feature is more compositional than creating a separate Temperature feature. The existing `temperature` attribute (ID 60) covers component/internal temperature. Adding domain-specific temperature types (room, outdoor, water, supply, return) follows the same pattern as AC/DC power types.

#### Candidate 3: EnergyControl Operating Constraints (extend -- MEDIUM priority)

**Purpose:** Add operating constraints for process-capable devices (min on/off durations).

**Covers:** OHPCF (Batch 6), potentially other process-based use cases.

**Required additions:**
```
EnergyControl feature (extend):
  New attributes:
    82: minRunDuration       -- uint32, seconds, nullable (min time before pause/stop)
    83: minPauseDuration     -- uint32, seconds, nullable (min pause before restart)
    84: maxRunDuration       -- uint32, seconds, nullable (max run time)
    85: maxPauseDuration     -- uint32, seconds, nullable (max pause time)
    86: optionalProcessPower -- int64, mW, nullable (expected power when process runs)
```

**Rationale:** These constraints protect physical devices (compressor short-cycling). The existing ProcessStateEnum and pause/resume/stop commands handle process lifecycle; constraints tell the CEM what timing rules to respect.

#### Candidate 4: PCC Control Mode on EnergyControl (extend -- MEDIUM priority)

**Purpose:** Add control mode selector for battery inverters (direct power vs GCP target).

**Covers:** COB (Batch 5)

**Required additions:**
```
EnergyControl feature (extend):
  New enum: ControlMode {DIRECT, PCC, AUTO}
  New attribute:
    87: controlMode -- uint8, enum: ControlMode, nullable (how setpoints are interpreted)
```

**Rationale:** When `controlMode=PCC`, the existing consumption/production setpoints are reinterpreted as Grid Connection Point power targets instead of device-level targets. The inverter self-regulates to achieve the GCP target. This is a single-attribute addition that enables a fundamentally different control paradigm.

#### Candidate 5: Endpoint Metadata Extensions (extend DeviceInfo -- LOW priority)

**Purpose:** Add per-endpoint metadata for hierarchy and labeling.

**Covers:** VHAN (Batch 4), all hierarchy use cases (Batches 3-5)

**Required additions:**
```
Endpoint descriptor in DeviceInfo.endpoints[]:
  parentEndpoint  -- uint16, nullable (parent endpoint index for hierarchy)
  endpointLabel   -- string, nullable (human-readable name: "Living Room", "DHW Circuit")
```

**Rationale:** Small additions to the endpoint descriptor that unlock significant modeling capability. `parentEndpoint` enables HVAC nesting and Inverter>Battery/PVString hierarchy. `endpointLabel` enables VHAN and improves device discovery/UI.

#### Not Recommended for v1

| Candidate | Why Defer |
|-----------|-----------|
| Historical/time-series data model | Controller-side responsibility. Constrained devices shouldn't store history. |
| Per-slot power-dependent pricing tiers | Complex refinement to Tariff+Signals. Simple per-slot pricing covers 90% of use cases. |
| ITPCM negotiation protocol | Complex bidirectional protocol. Plan's commitment levels (PRELIMINARY, COMMITTED) may be sufficient. |
| Bill/cost summary feature | Controller computes from existing data (Tariff + Measurement + ChargingSession). |
| Reactive power / power factor in Signals | Niche for residential. Add when grid operator requirements clarify. |
| Scheduled setpoints (start/end time) | Adds complexity for a rare use case. CEM can schedule by timing its writes. |

---

### 6.7 Final Gap Summary + Recommendations

#### Complete Gap Inventory (42 use cases analyzed)

| Priority | Gap | Affected UCs | Batch | Recommendation |
|----------|-----|-------------|-------|----------------|
| **HIGH** | **No HVAC operation mode feature** | MoRHSF, MoRCSF, MoDHWSF, MoDHWT(setpoint), CoRHSF, CoRCSF, CoDHWSF, CoRHT, CoRCT, CoDHWT, MCSGR | 3, 4 | New HVAC feature (Candidate 1) |
| **HIGH** | **No entity hierarchy (parentEndpoint)** | MoRHSF, MoRCSF, MoRT, VHAN, MOB, MPS, COB | 3, 4, 5 | Add `parentEndpoint` to endpoint descriptor (Candidate 5) |
| **HIGH** | **Measurement limited to electrical quantities** | MoRT, MoDHWT, MoOT | 3 | Extend Measurement with temperature types (Candidate 2) |
| **MEDIUM** | **No operating constraints (min on/off durations)** | OHPCF | 6 | Extend EnergyControl (Candidate 3) |
| **MEDIUM** | **No PCC control mode** | COB | 5 | Add `controlMode` enum to EnergyControl (Candidate 4) |
| **MEDIUM** | **No temperature setpoint mechanism** | CoRHT, CoRCT, CoDHWT | 4 | Part of new HVAC feature (embedded setpoints per mode) |
| **MEDIUM** | **SG-Ready mode mapping** | MCSGR | 3 | Part of new HVAC feature (overrun mechanism) |
| **MEDIUM** | **Per-slot power-dependent pricing tiers** | TOUT, ITPCM, CEVC | 1, 2 | Defer to v2. Simple per-slot pricing is adequate for v1. |
| **MEDIUM** | **ITPCM negotiation protocol** | ITPCM, CEVC | 1, 2 | Defer to v2. Plan commitment levels may suffice. |
| **MEDIUM** | **CEM-outbound forecast direction** | PODF | 1 | Validate multi-zone handles this (CEM as "device" to higher tier). |
| Low | No endpoint label | VHAN | 4 | Add `endpointLabel` to endpoint descriptor (Candidate 5) |
| Low | No HVAC overrun concept | MoDHWSF, CoDHWSF | 3, 4 | Part of new HVAC feature (overrun commands) |
| Low | Optional process power value | OHPCF | 6 | Add `optionalProcessPower` to EnergyControl (Candidate 3) |
| Low | SoC percentage targets (evMinSoC%, evTargetSoC%) | EVSOC | 2 | Add `evMinStateOfCharge`, `evTargetStateOfCharge` to ChargingSession |
| Low | Setpoint auto-expiry duration | FLOA, DBEVC | 1, 2 | EnergyControl SetSetpoint already has `duration` parameter |
| Low | Brand name (distinct from vendor) | NID | 6 | Add optional `brandName` to DeviceInfo |
| Low | Power source enum | NID | 6 | Add optional `powerSource` to DeviceInfo or infer from Electrical |
| Low | Communication standard attribute | EVCC | 2 | Leave implicit via evDemandMode |
| Low | EV travel range | EVSOC | 2 | Add optional `evTravelRange` to ChargingSession |
| Low | Charging cost summary | EVCS | 2 | Defer; controller computes from existing data |
| Low | Scheduled setpoints (start/end time) | DBEVC | 2 | Defer to v2 |
| Low | Setpoint changeability flag | DBEVC | 2 | Consider adding boolean to EnergyControl |
| Low | Explicit session-measurement linking | SMR | 2 | Defer; temporal correlation sufficient |
| Low | Inverter type classification | MOI | 5 | Implicit via device model structure |
| Low | Monthly/yearly yield | MOI | 5 | Controller computes from cumulative values |
| Low | Max cycle count per day | MOB | 5 | Add to Electrical if needed |
| Low | Aggregation entities (BatterySystem, PVSystem) | VABD, VAPD | 5 | Controller-side aggregation |
| Low | Sensor endpoint type | MoOT | 3 | Reuse HVAC or APPLIANCE endpoint type |
| Low | Historical time-series data | MHPC | 1 | Controller-side logging |
| Low | Reactive power / power factor in Signals | POEN | 1 | Defer to v2 |
| Low | Discrete value list for setpoints | FLOA | 1 | Defer; range is sufficient |
| Low | Signal processing constraints | ITPCM | 1 | Defer |

#### Coverage Statistics

- **42 use cases analyzed** across 6 batches
- **20 use cases (48%)** have good or full MASH coverage with existing features
- **10 use cases (24%)** have partial coverage needing minor extensions
- **12 use cases (28%)** have no coverage and need the new HVAC feature (10) or operating constraint extensions (2)

#### Prioritized Recommendations

**Phase 1: Structural foundations (enable HVAC + hierarchy)**
1. Add `parentEndpoint` and `endpointLabel` to endpoint descriptor
2. Create new HVAC feature (operation modes, temperature setpoints, overruns)
3. Extend Measurement with temperature types (room, outdoor, water, supply, return)

These 3 changes close the gaps for all 14 HVAC use cases (Batches 3+4) and improve modeling for Batch 5 hierarchy.

**Phase 2: Process control + battery enhancements**
4. Add operating constraints to EnergyControl (min/max run/pause durations)
5. Add `controlMode` enum to EnergyControl (DIRECT/PCC/AUTO for battery inverters)
6. Add `optionalProcessPower` to EnergyControl

These close the remaining OHPCF and COB gaps.

**Phase 3: Minor enrichments (as needed)**
7. Add `evMinStateOfCharge`, `evTargetStateOfCharge` to ChargingSession
8. Add `brandName`, `powerSource` to DeviceInfo
9. Validate multi-zone handles PODF direction

**Deferred to v2:**
- Per-slot power-dependent pricing tiers (complex Tariff+Signals refinement)
- ITPCM negotiation protocol
- Historical time-series data model
- Reactive power / power factor in Signals
- Scheduled setpoints with start/end times

#### Final Assessment

MASH's 9-feature, 11-endpoint-type design covers **48% of analyzed EEBUS use cases with no changes** and **72% with minor extensions**. The remaining 28% (HVAC monitoring and configuration) require a single new feature.

The protocol's design philosophy is validated: fewer, more general features achieve broad coverage. The main architectural gap is the HVAC domain -- unsurprising, as MASH was originally designed for electrical energy management. Adding the HVAC feature, temperature measurement extension, and endpoint hierarchy transforms MASH from an energy-management-only protocol into a comprehensive home energy and comfort protocol.

*Batch 6 complete. All batches finished.*
