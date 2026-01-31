# EEBUS Use Case Analysis Plan

Goal: Systematically analyze every EEBUS use case, document its SPINE structure,
and determine how it maps to MASH -- identifying gaps in features, attributes,
endpoint types, and structural capabilities.

## Method

For each use case, extract via CodeAtlas MCP:
1. **Goal** -- what it does in 1-2 sentences
2. **Actors** -- who participates (and SPINE entity types used)
3. **Entity hierarchy** -- nesting depth, sub-entities
4. **SPINE features used** -- feature types and key functions/specializations
5. **Scenarios** -- what operations (read/write/subscribe/notify), mandatory vs optional
6. **MASH mapping** -- which existing MASH features cover it
7. **Gaps** -- new features, attributes, endpoint types, or structural changes needed

Output per use case: a row in the summary table + notes on gaps.

---

## Batch 1: Grid / Power Management

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 1.1 | LPC | Limitation of Power Consumption | [ ] |
| 1.2 | LPP | Limitation of Power Production | [ ] |
| 1.3 | MPC | Monitoring of Power Consumption | [ ] |
| 1.4 | MGCP | Monitoring of Grid Connection Point | [ ] |
| 1.5 | MHPC | Monitoring of Historical Power Consumption | [ ] |
| 1.6 | PODF | Power Demand Forecast | [ ] |
| 1.7 | POEN | Power Envelope | [ ] |
| 1.8 | TOUT | Time of Use Tariff | [ ] |
| 1.9 | ITPCM | Incentive Table Power Consumption Mgmt | [ ] |
| 1.10 | FLOA | Flexible Load | [ ] |

Already mapped in MASH: LPC, LPP, MPC, MGCP, POEN, TOUT, ITPCM.
Focus: validate existing mappings, analyze MHPC/PODF/FLOA gaps.

---

## Batch 2: E-Mobility

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 2.1 | CEVC | Coordinated EV Charging | [ ] |
| 2.2 | EVCC | EV Commissioning and Configuration | [ ] |
| 2.3 | EVSECC | EVSE Commissioning and Configuration | [ ] |
| 2.4 | OSCEV | Optimization of Self-Consumption During EV Charging | [ ] |
| 2.5 | OPEV | Overload Protection by EV Charging Current Curtailment | [ ] |
| 2.6 | EVCEM | EV Charging Electricity Measurement | [ ] |
| 2.7 | EVSOC | EV State of Charge | [ ] |
| 2.8 | EVCS | EV Charging Summary | [ ] |
| 2.9 | DBEVC | Dynamic Bidirectional EV Charging | [ ] |
| 2.10 | SMR | Session Measurement Relation | [ ] |

Already mapped in MASH: CEVC/EVCEM/EVSE partially via ChargingSession+EnergyControl+Signals+Plan.
Focus: EV-specific data (SoC, commissioning, bidirectional), EV vs EVSE entity split.

---

## Batch 3: HVAC Monitoring

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 3.1 | MoRHSF | Monitoring of Room Heating System Function | [ ] |
| 3.2 | MoRCSF | Monitoring of Room Cooling System Function | [ ] |
| 3.3 | MoRT | Monitoring of Room Temperature | [ ] |
| 3.4 | MoDHWSF | Monitoring of DHW System Function | [ ] |
| 3.5 | MoDHWT | Monitoring of DHW Temperature | [ ] |
| 3.6 | MoOT | Monitoring of Outdoor Temperature | [ ] |
| 3.7 | MCSGR | Monitoring and Control of SG-Ready Conditions | [ ] |

Not mapped in MASH at all. These use the HVAC feature, Measurement feature,
nested entity hierarchies (HeatingCircuit > HeatingZone > HVACRoom), and the
HVAC system function / operation mode model.
Focus: what data model is needed, entity hierarchy requirements.

---

## Batch 4: HVAC Configuration

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 4.1 | CoRHSF | Configuration of Room Heating System Function | [ ] |
| 4.2 | CoRCSF | Configuration of Room Cooling System Function | [ ] |
| 4.3 | CoRHT | Configuration of Room Heating Temperature | [ ] |
| 4.4 | CoRCT | Configuration of Room Cooling Temperature | [ ] |
| 4.5 | CoDHWSF | Configuration of DHW System Function | [ ] |
| 4.6 | CoDHWT | Configuration of DHW Temperature | [ ] |
| 4.7 | VHAN | Visualization of Heating Area Name | [ ] |

Not mapped in MASH. These add write operations (setpoints, operation modes)
on top of the monitoring UCs. Uses Setpoint feature, HVAC feature with write.
Focus: how setpoints and mode configuration map to MASH EnergyControl or need new features.

---

## Batch 5: Inverter / Battery / PV

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 5.1 | MOB | Monitoring of Battery | [x] |
| 5.2 | COB | Control of Battery | [x] |
| 5.3 | MOI | Monitoring of Inverter | [x] |
| 5.4 | MPS | Monitoring of PV String | [x] |
| 5.5 | VABD | Visualization of Aggregated Battery Data | [x] |
| 5.6 | VAPD | Visualization of Aggregated Photovoltaic Data | [x] |

Partially mapped: MOB, COB, MOI, MPS have entries in MASH mapping table.
Focus: validate mappings are complete, analyze VABD/VAPD aggregation pattern.

---

## Batch 6: Generic + Cross-Cutting Synthesis

| # | UC | Full Name | Status |
|---|-----|-----------|--------|
| 6.1 | NID | Node Identification | [x] |
| 6.2 | -- | OHPCF (Heat Pump Compressor Flexibility) | [x] |
| 6.3 | -- | Cross-cutting: entity hierarchy needs | [x] |
| 6.4 | -- | Cross-cutting: SPINE feature type mapping | [x] |
| 6.5 | -- | Cross-cutting: endpoint type gaps | [x] |
| 6.6 | -- | Cross-cutting: new MASH feature candidates | [x] |
| 6.7 | -- | Final gap summary + recommendations | [x] |

---

## Reference: Current MASH Coverage

### MASH Endpoint Types (11)
- DEVICE_ROOT (0x00), GRID_CONNECTION (0x01), INVERTER (0x02),
  PV_STRING (0x03), BATTERY (0x04), EV_CHARGER (0x05),
  HEAT_PUMP (0x06), WATER_HEATER (0x07), HVAC (0x08),
  APPLIANCE (0x09), SUB_METER (0x0A)

### MASH Feature Types (9)
- DeviceInfo (0x01), Status (0x02), Electrical (0x03),
  Measurement (0x04), EnergyControl (0x05), ChargingSession (0x06),
  Tariff (0x07), Signals (0x08), Plan (0x09)

### EEBUS SPINE Entity Types (48)
- Battery, BatterySystem, CEM, ChargingOutlet, Compressor,
  ControllableSystem, DeviceInformation, DHWCircuit, DHWStorage,
  Dishwasher, Dryer, ElectricalImmersionHeater, ElectricalStorage,
  ElectricityGenerationSystem, ElectricityStorageSystem, EV, EVSE,
  Fan, GasHeatingAppliance, Generic, GridConnectionPointOfPremises,
  GridGuard, HeatingBufferStorage, HeatingCircuit, HeatingObject,
  HeatingZone, HeatPumpAppliance, HeatSinkCircuit, HeatSourceCircuit,
  HeatSourceUnit, Household, HVACController, HVACRoom,
  InstantDHWHeater, Inverter, OilHeatingAppliance, Pump, PV,
  PVESHybrid, PVString, PVSystem, RefrigerantCircuit,
  SmartEnergyAppliance, SolarDHWStorage, SolarThermalCircuit,
  SubMeterElectricity, Surrogate, TemperatureSensor, Washer

### EEBUS SPINE Feature Types (32)
- ActuatorLevel, ActuatorSwitch, Alarm, Bill, DataTunneling,
  DeviceClassification, DeviceConfiguration, DeviceDiagnosis,
  DirectControl, ElectricalConnection, Generic, HVAC,
  Identification, IncentiveTable, LoadControl, Measurement,
  Messaging, NetworkManagement, NodeManagement,
  OperatingConstraints, PowerSequences, Sensing, Setpoint,
  SmartEnergyManagementPs, StateInformation, SupplyCondition,
  Surrogate, TariffInformation, TaskManagement, Threshold,
  TimeInformation, TimeSeries, TimeTable

### Current MASH Mapping Table
| EEBUS Use Case | MASH Features |
|----------------|---------------|
| LPC | EnergyControl |
| LPP | EnergyControl |
| MPC | Measurement |
| MGCP | Measurement (GRID_CONNECTION endpoint) |
| EVSE, EVCEM, CEVC | ChargingSession + EnergyControl + Signals + Plan |
| COB | EnergyControl + Measurement (BATTERY endpoint) |
| MOB | Measurement + Status |
| MOI | Measurement + Status |
| MPS | Measurement |
| TOUT | Signals + Tariff |
| POEN | Signals (CONSTRAINT type) |
| ITPCM | Signals + Tariff |
| OHPCF | EnergyControl (processState, optionalProcess) |

---

## Expected Outputs

After completing all batches:

1. **Per-UC summary table** with columns:
   UC | Goal | SPINE Entities | SPINE Features | Scenarios | MASH Mapping | Gaps

2. **SPINE-to-MASH feature type mapping** showing which SPINE features map
   to which MASH features, and which have no mapping.

3. **Endpoint type gap list** -- SPINE entity types that suggest new MASH
   endpoint types (e.g., DHW_STORAGE, HEATING_CIRCUIT, COMPRESSOR).

4. **Structural gap analysis** -- whether MASH needs:
   - Endpoint hierarchy / parentEndpoint
   - Endpoint labels
   - Temperature / setpoint features
   - HVAC operation mode feature
   - Aggregation patterns
   - Historical data support

5. **Prioritized recommendations** for MASH protocol additions.
