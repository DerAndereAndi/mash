// Package features provides MASH feature implementations.
//
// Features are the fundamental building blocks of the MASH device model.
// Each feature represents a specific capability with defined attributes
// and commands.
//
// # Core Features
//
//   - DeviceInfo (0x0006): Device identity and structure (endpoint 0 only)
//   - Electrical (0x0001): Electrical characteristics and capability envelope
//   - Measurement (0x0002): Real-time telemetry (power, energy, voltage, current)
//
// # Usage
//
// Features are created using factory functions and added to endpoints:
//
//	device := model.NewDevice("device-123", vendorID, productID)
//
//	// DeviceInfo is automatically on endpoint 0
//	deviceInfo := features.NewDeviceInfo(device)
//	device.RootEndpoint().AddFeature(deviceInfo)
//
//	// Add Electrical and Measurement to functional endpoints
//	evCharger := model.NewEndpoint(1, model.EndpointEVCharger, "Charger")
//	evCharger.AddFeature(features.NewElectrical())
//	evCharger.AddFeature(features.NewMeasurement())
//	device.AddEndpoint(evCharger)
//
// # Attribute Updates
//
// Feature attribute values are updated through setter methods that handle
// validation and dirty tracking:
//
//	elec := features.NewElectrical()
//	elec.SetPhaseCount(3)
//	elec.SetNominalVoltage(400)
//	elec.SetMaxCurrentPerPhase(32000)  // 32A in mA
//
//	meas := features.NewMeasurement()
//	meas.SetACActivePower(11040000)    // 11.04 kW in mW
//	meas.SetACCurrentPerPhase(map[model.Phase]int64{
//	    model.PhaseA: 16000,
//	    model.PhaseB: 16000,
//	    model.PhaseC: 16000,
//	})
package features
