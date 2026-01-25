# Device Model Hierarchy Comparison

**Date:** 2025-01-24
**Context:** Choosing hierarchy depth for MASH device model

---

## Real Device Examples

### Example 1: Simple EV Charger (Single Port)

**2-Level Model (Device > Cluster):**
```yaml
Device: evse-001
  Clusters:
    - Measurement (power, energy, current, voltage)
    - LoadControl (power limits, active limit)
    - DeviceInfo (manufacturer, model, serial)
    - ChargingSession (state, connected, SoC)
```
Address: `evse-001/Measurement/power`

**3-Level Model (Device > Endpoint > Cluster):**
```yaml
Device: evse-001
  Endpoint 0 (Root):
    - DeviceInfo
  Endpoint 1 (Charger):
    - Measurement
    - LoadControl
    - ChargingSession
```
Address: `evse-001/1/Measurement/power`

**Assessment:** 2-level is sufficient. Single endpoint adds no value.

---

### Example 2: Dual-Port EV Charger

**2-Level Model (Device > Cluster):**
```yaml
Device: evse-dual-001
  Clusters:
    - Measurement[0] (port 1 power, energy...)
    - Measurement[1] (port 2 power, energy...)
    - LoadControl[0] (port 1 limits)
    - LoadControl[1] (port 2 limits)
    - DeviceInfo
```
Address: `evse-dual-001/Measurement[0]/power`

**3-Level Model (Device > Endpoint > Cluster):**
```yaml
Device: evse-dual-001
  Endpoint 0 (Root):
    - DeviceInfo
  Endpoint 1 (Port 1):
    - Measurement
    - LoadControl
    - ChargingSession
  Endpoint 2 (Port 2):
    - Measurement
    - LoadControl
    - ChargingSession
```
Address: `evse-dual-001/1/Measurement/power`

**Assessment:** 3-level is cleaner here. Avoids array indices in cluster names.

---

### Example 3: Hybrid Inverter (PV + Battery + Grid)

**2-Level Model:**
```yaml
Device: inverter-001
  Clusters:
    - Measurement[pv] (PV production)
    - Measurement[battery] (battery charge/discharge)
    - Measurement[grid] (grid import/export)
    - BatteryControl (charge/discharge commands)
    - DeviceInfo
```

**3-Level Model:**
```yaml
Device: inverter-001
  Endpoint 0 (Root):
    - DeviceInfo
  Endpoint 1 (PV):
    - Measurement
  Endpoint 2 (Battery):
    - Measurement
    - BatteryControl
  Endpoint 3 (Grid):
    - Measurement
```

**Assessment:** 3-level provides better semantic separation.

---

### Example 4: Energy Manager (Controller)

**2-Level Model:**
```yaml
Device: hems-001
  Clusters:
    - DeviceInfo
    - Optimization (strategies, preferences)
    - GridTariff (current rates, schedules)
```

**3-Level Model:**
```yaml
Device: hems-001
  Endpoint 0 (Root):
    - DeviceInfo
    - Optimization
    - GridTariff
```

**Assessment:** 2-level is sufficient. Energy manager is single-purpose.

---

## Comparison Summary

| Scenario | 2-Level | 3-Level |
|----------|---------|---------|
| Single-port EVSE | Clean | Unnecessary overhead |
| Dual-port EVSE | Array indices needed | Clean separation |
| Hybrid inverter | Array indices needed | Clean separation |
| Energy manager | Clean | Unnecessary overhead |
| Simple sensor | Clean | Unnecessary overhead |

---

## Design Options

### Option A: Fixed 2-Level (Device > Cluster)
- Clusters can have instance IDs: `Measurement[0]`, `Measurement[1]`
- Simpler addressing: `device/cluster[instance]/attribute`
- Works for all cases, less semantic clarity for multi-function devices

### Option B: Fixed 3-Level (Device > Endpoint > Cluster)
- Endpoints represent logical sub-devices
- Cleaner for multi-function devices
- Slight overhead for single-function devices (always use endpoint 0)

### Option C: Flexible Depth (1-3 levels)
- Simple devices: `device/cluster/attribute`
- Multi-function: `device/endpoint/cluster/attribute`
- Most flexibility, but complicates parsing

### Option D: 2-Level with Composite IDs
- Device ID can be composite: `evse-001:port1`, `evse-001:port2`
- Technically 2 levels, but allows sub-device addressing
- Discovery returns all composite IDs

---

## Recommendation

### For MASH: **Option B - Fixed 3-Level (Device > Endpoint > Cluster)**

**Rationale:**
1. **Handles all cases cleanly** - multi-port, hybrid inverters, etc.
2. **Matter-aligned** - familiar to developers
3. **Fixed depth = simple parsing** - no variable-length path handling
4. **Small overhead** - single-function devices just use endpoint 1
5. **Future-proof** - won't need redesign for complex devices

**Addressing scheme:**
```
device_id / endpoint_id / cluster_id / attribute_or_command

Examples:
- evse-001/1/Measurement/power
- evse-001/1/LoadControl/SetLimit
- inverter-001/2/BatteryControl/SetChargeTarget
```

**Endpoint conventions:**
- Endpoint 0: Reserved for root device info (like Matter)
- Endpoint 1+: Functional endpoints

---

## Alternative: 2-Level with Instance Keys

If we want maximum simplicity:

```
device_id / cluster_type:instance / attribute_or_command

Examples:
- evse-001/Measurement:port1/power
- evse-001/LoadControl:port1/SetLimit
- inverter-001/BatteryControl:main/SetChargeTarget
```

This keeps parsing simple while allowing logical grouping.

---

## Memory Impact

On 256KB RAM ESP32:

| Approach | Routing Code | Per-Device RAM |
|----------|--------------|----------------|
| 2-Level | ~2KB | ~100B per cluster |
| 3-Level | ~3KB | ~50B per endpoint + 80B per cluster |
| Variable | ~5KB | ~150B per node |

Difference is minimal - choose based on semantic clarity, not memory.
