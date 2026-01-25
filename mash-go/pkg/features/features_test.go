package features

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/model"
)

func TestDeviceInfo(t *testing.T) {
	di := NewDeviceInfo()

	t.Run("Type", func(t *testing.T) {
		if di.Type() != model.FeatureDeviceInfo {
			t.Errorf("expected type DeviceInfo, got %v", di.Type())
		}
	})

	t.Run("Revision", func(t *testing.T) {
		if di.Revision() != DeviceInfoFeatureRevision {
			t.Errorf("expected revision %d, got %d", DeviceInfoFeatureRevision, di.Revision())
		}
	})

	t.Run("SetDeviceID", func(t *testing.T) {
		err := di.SetDeviceID("PEN12345.EVSE-001")
		if err != nil {
			t.Fatalf("SetDeviceID failed: %v", err)
		}
		if di.DeviceID() != "PEN12345.EVSE-001" {
			t.Errorf("expected deviceId PEN12345.EVSE-001, got %s", di.DeviceID())
		}
	})

	t.Run("SetVendorInfo", func(t *testing.T) {
		_ = di.SetVendorName("Wallbox")
		_ = di.SetProductName("Pulsar Plus")
		_ = di.SetSerialNumber("WB-2024-001234")
		_ = di.SetVendorID(12345)
		_ = di.SetProductID(100)

		if di.VendorName() != "Wallbox" {
			t.Errorf("expected vendorName Wallbox, got %s", di.VendorName())
		}
		if di.ProductName() != "Pulsar Plus" {
			t.Errorf("expected productName Pulsar Plus, got %s", di.ProductName())
		}
		if di.SerialNumber() != "WB-2024-001234" {
			t.Errorf("expected serialNumber WB-2024-001234, got %s", di.SerialNumber())
		}
		if di.VendorID() != 12345 {
			t.Errorf("expected vendorId 12345, got %d", di.VendorID())
		}
		if di.ProductID() != 100 {
			t.Errorf("expected productId 100, got %d", di.ProductID())
		}
	})

	t.Run("SetSoftwareVersion", func(t *testing.T) {
		_ = di.SetSoftwareVersion("3.2.1")
		if di.SoftwareVersion() != "3.2.1" {
			t.Errorf("expected version 3.2.1, got %s", di.SoftwareVersion())
		}
	})

	t.Run("DefaultSpecVersion", func(t *testing.T) {
		if di.SpecVersion() != "1.0" {
			t.Errorf("expected specVersion 1.0, got %s", di.SpecVersion())
		}
	})

	t.Run("WritableAttributes", func(t *testing.T) {
		// Location and label should be writable by users
		err := di.SetLocation("Garage")
		if err != nil {
			t.Errorf("SetLocation failed: %v", err)
		}
		if di.Location() != "Garage" {
			t.Errorf("expected location Garage, got %s", di.Location())
		}

		err = di.SetLabel("Main Charger")
		if err != nil {
			t.Errorf("SetLabel failed: %v", err)
		}
		if di.Label() != "Main Charger" {
			t.Errorf("expected label Main Charger, got %s", di.Label())
		}
	})

	t.Run("SetEndpoints", func(t *testing.T) {
		endpoints := []*model.EndpointInfo{
			{ID: 0, Type: model.EndpointDeviceRoot, Features: []uint16{0x0006}},
			{ID: 1, Type: model.EndpointEVCharger, Label: "Port 1", Features: []uint16{0x0001, 0x0002}},
		}
		err := di.SetEndpoints(endpoints)
		if err != nil {
			t.Fatalf("SetEndpoints failed: %v", err)
		}
	})
}

func TestElectrical(t *testing.T) {
	elec := NewElectrical()

	t.Run("Type", func(t *testing.T) {
		if elec.Type() != model.FeatureElectrical {
			t.Errorf("expected type Electrical, got %v", elec.Type())
		}
	})

	t.Run("DefaultValues", func(t *testing.T) {
		if elec.PhaseCount() != 1 {
			t.Errorf("expected default phaseCount 1, got %d", elec.PhaseCount())
		}
		if elec.NominalVoltage() != 230 {
			t.Errorf("expected default voltage 230, got %d", elec.NominalVoltage())
		}
		if elec.NominalFrequency() != 50 {
			t.Errorf("expected default frequency 50, got %d", elec.NominalFrequency())
		}
		if elec.SupportedDirections() != DirectionConsumption {
			t.Errorf("expected default direction CONSUMPTION, got %v", elec.SupportedDirections())
		}
	})

	t.Run("SetPhaseConfig", func(t *testing.T) {
		_ = elec.SetPhaseCount(3)
		_ = elec.SetPhaseMapping(map[Phase]GridPhase{
			PhaseA: GridPhaseL1,
			PhaseB: GridPhaseL2,
			PhaseC: GridPhaseL3,
		})

		if elec.PhaseCount() != 3 {
			t.Errorf("expected phaseCount 3, got %d", elec.PhaseCount())
		}

		mapping := elec.PhaseMapping()
		if mapping == nil || len(mapping) != 3 {
			t.Error("expected 3-phase mapping")
		}
	})

	t.Run("SetPowerRatings", func(t *testing.T) {
		_ = elec.SetNominalMaxConsumption(22_000_000) // 22 kW
		_ = elec.SetNominalMaxProduction(0)
		_ = elec.SetNominalMinPower(1_400_000) // 1.4 kW

		if elec.NominalMaxConsumption() != 22_000_000 {
			t.Errorf("expected maxConsumption 22000000, got %d", elec.NominalMaxConsumption())
		}
		if elec.NominalMinPower() != 1_400_000 {
			t.Errorf("expected minPower 1400000, got %d", elec.NominalMinPower())
		}
	})

	t.Run("SetCurrentRatings", func(t *testing.T) {
		_ = elec.SetMaxCurrentPerPhase(32_000) // 32A
		_ = elec.SetMinCurrentPerPhase(6_000)  // 6A

		if elec.MaxCurrentPerPhase() != 32_000 {
			t.Errorf("expected maxCurrent 32000, got %d", elec.MaxCurrentPerPhase())
		}
		if elec.MinCurrentPerPhase() != 6_000 {
			t.Errorf("expected minCurrent 6000, got %d", elec.MinCurrentPerPhase())
		}
	})

	t.Run("BidirectionalSupport", func(t *testing.T) {
		_ = elec.SetSupportedDirections(DirectionBidirectional)
		_ = elec.SetSupportsAsymmetric(AsymmetricConsumption)

		if !elec.IsBidirectional() {
			t.Error("expected bidirectional support")
		}
		if !elec.CanConsume() {
			t.Error("expected consumption capability")
		}
		if !elec.CanProduce() {
			t.Error("expected production capability")
		}
		if elec.SupportsAsymmetric() != AsymmetricConsumption {
			t.Errorf("expected asymmetric CONSUMPTION, got %v", elec.SupportsAsymmetric())
		}
	})

	t.Run("SetEnergyCapacity", func(t *testing.T) {
		_ = elec.SetEnergyCapacity(10_000_000) // 10 kWh

		if elec.EnergyCapacity() != 10_000_000 {
			t.Errorf("expected capacity 10000000, got %d", elec.EnergyCapacity())
		}
	})
}

func TestMeasurement(t *testing.T) {
	meas := NewMeasurement()

	t.Run("Type", func(t *testing.T) {
		if meas.Type() != model.FeatureMeasurement {
			t.Errorf("expected type Measurement, got %v", meas.Type())
		}
	})

	t.Run("ACPower", func(t *testing.T) {
		_ = meas.SetACActivePower(11_040_000) // 11.04 kW consuming
		_ = meas.SetACReactivePower(200_000)
		_ = meas.SetACApparentPower(11_050_000)

		power, ok := meas.ACActivePower()
		if !ok || power != 11_040_000 {
			t.Errorf("expected activePower 11040000, got %d (ok=%v)", power, ok)
		}

		reactive, ok := meas.ACReactivePower()
		if !ok || reactive != 200_000 {
			t.Errorf("expected reactivePower 200000, got %d (ok=%v)", reactive, ok)
		}

		apparent, ok := meas.ACApparentPower()
		if !ok || apparent != 11_050_000 {
			t.Errorf("expected apparentPower 11050000, got %d (ok=%v)", apparent, ok)
		}
	})

	t.Run("ACPerPhase", func(t *testing.T) {
		currents := map[Phase]int64{
			PhaseA: 16_000,
			PhaseB: 16_000,
			PhaseC: 16_000,
		}
		_ = meas.SetACCurrentPerPhase(currents)

		voltages := map[Phase]uint32{
			PhaseA: 230_000,
			PhaseB: 231_000,
			PhaseC: 229_000,
		}
		_ = meas.SetACVoltagePerPhase(voltages)

		readCurrents, ok := meas.ACCurrentPerPhase()
		if !ok || len(readCurrents) != 3 {
			t.Errorf("expected 3 phase currents, got %d", len(readCurrents))
		}

		readVoltages, ok := meas.ACVoltagePerPhase()
		if !ok || len(readVoltages) != 3 {
			t.Errorf("expected 3 phase voltages, got %d", len(readVoltages))
		}
	})

	t.Run("ACFrequencyAndPowerFactor", func(t *testing.T) {
		_ = meas.SetACFrequency(50_020) // 50.02 Hz
		_ = meas.SetPowerFactor(998)    // 0.998

		freq, ok := meas.ACFrequency()
		if !ok || freq != 50_020 {
			t.Errorf("expected frequency 50020, got %d", freq)
		}

		pf, ok := meas.PowerFactor()
		if !ok || pf != 998 {
			t.Errorf("expected powerFactor 998, got %d", pf)
		}
	})

	t.Run("ACEnergy", func(t *testing.T) {
		_ = meas.SetACEnergyConsumed(2_500_000_000) // 2500 kWh
		_ = meas.SetACEnergyProduced(0)

		consumed, ok := meas.ACEnergyConsumed()
		if !ok || consumed != 2_500_000_000 {
			t.Errorf("expected consumed 2500000000, got %d", consumed)
		}
	})

	t.Run("DCMeasurements", func(t *testing.T) {
		_ = meas.SetDCPower(-3_200_000) // -3.2 kW producing
		_ = meas.SetDCCurrent(-8_000)   // -8A out
		_ = meas.SetDCVoltage(400_000)  // 400V

		power, ok := meas.DCPower()
		if !ok || power != -3_200_000 {
			t.Errorf("expected dcPower -3200000, got %d", power)
		}

		current, ok := meas.DCCurrent()
		if !ok || current != -8_000 {
			t.Errorf("expected dcCurrent -8000, got %d", current)
		}

		voltage, ok := meas.DCVoltage()
		if !ok || voltage != 400_000 {
			t.Errorf("expected dcVoltage 400000, got %d", voltage)
		}
	})

	t.Run("BatteryState", func(t *testing.T) {
		_ = meas.SetStateOfCharge(65)
		_ = meas.SetStateOfHealth(97)
		_ = meas.SetStateOfEnergy(6_500_000) // 6.5 kWh
		_ = meas.SetCycleCount(342)
		_ = meas.SetTemperature(2850) // 28.5C

		soc, ok := meas.StateOfCharge()
		if !ok || soc != 65 {
			t.Errorf("expected soc 65, got %d", soc)
		}

		soh, ok := meas.StateOfHealth()
		if !ok || soh != 97 {
			t.Errorf("expected soh 97, got %d", soh)
		}

		soe, ok := meas.StateOfEnergy()
		if !ok || soe != 6_500_000 {
			t.Errorf("expected soe 6500000, got %d", soe)
		}

		temp, ok := meas.Temperature()
		if !ok || temp != 2850 {
			t.Errorf("expected temp 2850, got %d", temp)
		}
	})

	t.Run("IsConsumingProducing", func(t *testing.T) {
		_ = meas.SetACActivePower(5_000_000) // 5kW consuming
		if !meas.IsConsuming() {
			t.Error("expected IsConsuming to be true")
		}
		if meas.IsProducing() {
			t.Error("expected IsProducing to be false")
		}

		_ = meas.SetACActivePower(-5_000_000) // -5kW producing
		if meas.IsConsuming() {
			t.Error("expected IsConsuming to be false")
		}
		if !meas.IsProducing() {
			t.Error("expected IsProducing to be true")
		}
	})

	t.Run("ActivePowerKW", func(t *testing.T) {
		_ = meas.SetACActivePower(11_040_000) // 11.04 kW
		kw, ok := meas.ActivePowerKW()
		if !ok {
			t.Error("expected ActivePowerKW to return ok")
		}
		if kw < 11.039 || kw > 11.041 {
			t.Errorf("expected ~11.04 kW, got %f", kw)
		}
	})
}

func TestEnums(t *testing.T) {
	t.Run("Direction", func(t *testing.T) {
		tests := []struct {
			d    Direction
			want string
		}{
			{DirectionConsumption, "CONSUMPTION"},
			{DirectionProduction, "PRODUCTION"},
			{DirectionBidirectional, "BIDIRECTIONAL"},
			{Direction(99), "UNKNOWN"},
		}
		for _, tt := range tests {
			if got := tt.d.String(); got != tt.want {
				t.Errorf("Direction(%d).String() = %s, want %s", tt.d, got, tt.want)
			}
		}
	})

	t.Run("AsymmetricSupport", func(t *testing.T) {
		tests := []struct {
			a    AsymmetricSupport
			want string
		}{
			{AsymmetricNone, "NONE"},
			{AsymmetricConsumption, "CONSUMPTION"},
			{AsymmetricProduction, "PRODUCTION"},
			{AsymmetricBidirectional, "BIDIRECTIONAL"},
		}
		for _, tt := range tests {
			if got := tt.a.String(); got != tt.want {
				t.Errorf("AsymmetricSupport(%d).String() = %s, want %s", tt.a, got, tt.want)
			}
		}
	})

	t.Run("Phase", func(t *testing.T) {
		tests := []struct {
			p    Phase
			want string
		}{
			{PhaseA, "A"},
			{PhaseB, "B"},
			{PhaseC, "C"},
		}
		for _, tt := range tests {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("Phase(%d).String() = %s, want %s", tt.p, got, tt.want)
			}
		}
	})

	t.Run("GridPhase", func(t *testing.T) {
		tests := []struct {
			g    GridPhase
			want string
		}{
			{GridPhaseL1, "L1"},
			{GridPhaseL2, "L2"},
			{GridPhaseL3, "L3"},
		}
		for _, tt := range tests {
			if got := tt.g.String(); got != tt.want {
				t.Errorf("GridPhase(%d).String() = %s, want %s", tt.g, got, tt.want)
			}
		}
	})

	t.Run("PhasePair", func(t *testing.T) {
		tests := []struct {
			p    PhasePair
			want string
		}{
			{PhasePairAB, "AB"},
			{PhasePairBC, "BC"},
			{PhasePairCA, "CA"},
		}
		for _, tt := range tests {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("PhasePair(%d).String() = %s, want %s", tt.p, got, tt.want)
			}
		}
	})
}

func TestFeatureIntegration(t *testing.T) {
	// Test using features with the device model
	device := model.NewDevice("test-device", 0x1234, 0x5678)

	// Add DeviceInfo to root endpoint
	deviceInfo := NewDeviceInfo()
	_ = deviceInfo.SetDeviceID("test-device")
	_ = deviceInfo.SetVendorName("Test Vendor")
	_ = deviceInfo.SetProductName("Test Product")
	_ = deviceInfo.SetSerialNumber("SN12345")
	_ = deviceInfo.SetSoftwareVersion("1.0.0")
	device.RootEndpoint().AddFeature(deviceInfo.Feature)

	// Create an EV charger endpoint with Electrical and Measurement
	charger := model.NewEndpoint(1, model.EndpointEVCharger, "Main Charger")

	electrical := NewElectrical()
	_ = electrical.SetPhaseCount(3)
	_ = electrical.SetNominalVoltage(400)
	_ = electrical.SetMaxCurrentPerPhase(32_000)
	_ = electrical.SetNominalMaxConsumption(22_000_000)
	_ = electrical.SetSupportedDirections(DirectionConsumption)
	charger.AddFeature(electrical.Feature)

	measurement := NewMeasurement()
	_ = measurement.SetACActivePower(11_000_000)
	_ = measurement.SetACCurrentPerPhase(map[Phase]int64{
		PhaseA: 16_000,
		PhaseB: 16_000,
		PhaseC: 16_000,
	})
	charger.AddFeature(measurement.Feature)

	_ = device.AddEndpoint(charger)

	// Verify device structure
	if device.EndpointCount() != 2 {
		t.Errorf("expected 2 endpoints, got %d", device.EndpointCount())
	}

	// Read attributes through device
	power, err := device.ReadAttribute(1, model.FeatureMeasurement, MeasurementAttrACActivePower)
	if err != nil {
		t.Fatalf("ReadAttribute failed: %v", err)
	}
	if power != int64(11_000_000) {
		t.Errorf("expected power 11000000, got %v", power)
	}

	phases, err := device.ReadAttribute(1, model.FeatureElectrical, ElectricalAttrPhaseCount)
	if err != nil {
		t.Fatalf("ReadAttribute failed: %v", err)
	}
	if phases != uint8(3) {
		t.Errorf("expected 3 phases, got %v", phases)
	}

	// Update endpoint info in DeviceInfo
	endpoints := make([]*model.EndpointInfo, 0)
	for _, ep := range device.Endpoints() {
		endpoints = append(endpoints, ep.Info())
	}
	_ = deviceInfo.SetEndpoints(endpoints)
}

func TestNullableAttributes(t *testing.T) {
	meas := NewMeasurement()

	// Without setting a value, nullable attributes should return not-ok
	_, ok := meas.ACActivePower()
	if ok {
		t.Error("expected ACActivePower to return not-ok when not set")
	}

	_, ok = meas.StateOfCharge()
	if ok {
		t.Error("expected StateOfCharge to return not-ok when not set")
	}

	// After setting, should return ok
	_ = meas.SetACActivePower(1000)
	_, ok = meas.ACActivePower()
	if !ok {
		t.Error("expected ACActivePower to return ok after setting")
	}
}
