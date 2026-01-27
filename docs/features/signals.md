# Signals Feature

> Time-slotted incentive signals: prices, limits, forecasts

**Feature ID:** 0x0008
**Direction:** IN (controller sends to device)
**FeatureMap Bit:** SIGNALS (0x0010)
**Status:** Draft
**Last Updated:** 2025-01-25

---

## Purpose

Controller sends time-slotted signals TO devices. Signals are **inputs** that inform device behavior - prices, limits, forecasts, or incentives. The device responds via the [Plan](plan.md) feature.

---

## Design Principles

1. **One structure for all signal types** - unified slot-based format
2. **Direction-agnostic** - same structure for prices, limits, forecasts
3. **Simple slot model** - fixed duration slots, no complex sequences
4. **Relative to now** - slots defined by duration, not absolute time

---

## Attributes

```cbor
Signals Feature:
{
  // Active signals (controller → device)
  1: activeSignals,          // Signal[]: list of active signals
  2: signalCount,            // uint8: number of active signals

  // Signal acknowledgment
  10: lastReceivedSignalId,  // uint32: last signal ID processed
  11: signalStatus,          // SignalStatusEnum: processing status
}

Signal:
{
  1: signalId,               // uint32: unique identifier
  2: type,                   // SignalTypeEnum
  3: source,                 // SignalSourceEnum
  4: priority,               // uint8: 1=highest, 4=lowest
  5: startTime,              // timestamp: when signal starts
  6: slots,                  // Slot[]: time slots (max 96)
}

Slot:
{
  1: duration,               // uint32 s: slot duration

  // Price signals (type: PRICE, INCENTIVE)
  10: price,                 // int32?: price in 0.0001 currency units
  11: priceLevel,            // uint8?: 0-10 relative price level

  // Limit signals (type: CONSTRAINT)
  20: minPower,              // int64 mW?: minimum allowed
  21: maxPower,              // int64 mW?: maximum allowed

  // Forecast signals (type: FORECAST)
  30: forecastPower,         // int64 mW?: expected power
  31: forecastEnergy,        // int64 mWh?: expected energy

  // Renewable/CO2 signals
  40: renewablePercent,      // uint8?: % renewable (0-100)
  41: co2Intensity,          // uint16?: gCO2/kWh
}
```

---

## Enumerations

### SignalTypeEnum

```
PRICE             = 0x00  // Energy price (ToUT)
INCENTIVE         = 0x01  // Non-price incentive (renewable %, CO2)
CONSTRAINT        = 0x02  // Power limits (POEN)
FORECAST          = 0x03  // Expected generation/consumption
```

### SignalSourceEnum

```
GRID              = 0x00  // DSO/TSO
ENERGY_SUPPLIER   = 0x01  // Electricity retailer
AGGREGATOR        = 0x02  // Flexibility aggregator
LOCAL_EMS         = 0x03  // Home/building EMS
```

### SignalStatusEnum

```
NONE              = 0x00  // No signal received
RECEIVED          = 0x01  // Signal received, processing
ACCEPTED          = 0x02  // Signal accepted, will follow
REJECTED          = 0x03  // Signal rejected (reason in Plan)
```

---

## Commands

### SendSignal

Controller sends a new signal to device.

```cbor
Request:
{
  1: signal,               // Signal: the signal to send
  2: replaceExisting,      // bool: replace signals of same type?
}

Response:
{
  1: success,              // bool
  2: signalId,             // uint32: assigned signal ID
}
```

### ClearSignals

Remove active signals.

```cbor
Request:
{
  1: type,                 // SignalTypeEnum?: clear specific type
  2: source,               // SignalSourceEnum?: clear from source
}

Response:
{
  1: success,              // bool
  2: cleared,              // uint8: number cleared
}
```

---

## Examples

### Time-of-Use Tariff (ToUT)

```cbor
{
  signalId: 1001,
  type: PRICE,
  source: ENERGY_SUPPLIER,
  priority: 2,
  startTime: 1706180400,
  slots: [
    { duration: 21600, price: 1500, priceLevel: 2 },   // 6h @ 0.15/kWh (off-peak)
    { duration: 7200,  price: 3500, priceLevel: 7 },   // 2h @ 0.35/kWh (peak)
    { duration: 14400, price: 2000, priceLevel: 4 },   // 4h @ 0.20/kWh (mid)
    { duration: 10800, price: 4000, priceLevel: 9 },   // 3h @ 0.40/kWh (peak)
    { duration: 32400, price: 1200, priceLevel: 1 }    // 9h @ 0.12/kWh (night)
  ]
}
```

### Power Envelope (POEN)

```cbor
{
  signalId: 2001,
  type: CONSTRAINT,
  source: GRID,
  priority: 1,
  startTime: 1706180400,
  slots: [
    { duration: 3600,  minPower: 0, maxPower: 11000000 },  // 1h: up to 11kW
    { duration: 7200,  minPower: 0, maxPower: 3700000 },   // 2h: limited to 3.7kW
    { duration: 3600,  minPower: 0, maxPower: 7400000 },   // 1h: up to 7.4kW
    { duration: 72000, minPower: 0, maxPower: 11000000 }   // 20h: full power
  ]
}
```

### Solar Forecast

```cbor
{
  signalId: 3001,
  type: FORECAST,
  source: LOCAL_EMS,
  priority: 3,
  startTime: 1706180400,
  slots: [
    { duration: 3600, forecastPower: -2500000 },   // 1h: 2.5kW production
    { duration: 3600, forecastPower: -4200000 },   // 1h: 4.2kW peak
    { duration: 3600, forecastPower: -3800000 },   // 1h: 3.8kW
    { duration: 3600, forecastPower: -1500000 }    // 1h: 1.5kW declining
  ]
}
```

---

## EEBUS Use Case Coverage

| EEBUS Use Case | Signals Coverage |
|----------------|------------------|
| ToUT (Time of Use Tariff) | type: PRICE |
| POEN (Power Envelope) | type: CONSTRAINT |
| ITPCM (Incentive Table) | type: INCENTIVE + Tariff feature |
| Load/PV Forecasts | type: FORECAST |

---

## Related Features

| Feature | Relationship |
|---------|--------------|
| [Tariff](tariff.md) | Provides price structure; Signals provides time-slotted values |
| [Plan](plan.md) | Device's response to signals |
| [EnergyControl](energy-control.md) | Immediate control; Signals is scheduled |
