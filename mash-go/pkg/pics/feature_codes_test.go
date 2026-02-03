package pics

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/model"
)

func TestFeatureTypeToPICSCode_AllMapped(t *testing.T) {
	// Every known (non-vendor) FeatureType constant should have a PICS code.
	knownFeatures := []struct {
		ft   model.FeatureType
		code string
	}{
		{model.FeatureDeviceInfo, "INFO"},
		{model.FeatureStatus, "STAT"},
		{model.FeatureElectrical, "ELEC"},
		{model.FeatureMeasurement, "MEAS"},
		{model.FeatureEnergyControl, "CTRL"},
		{model.FeatureChargingSession, "CHRG"},
		{model.FeatureTariff, "TAR"},
		{model.FeatureSignals, "SIG"},
		{model.FeaturePlan, "PLAN"},
		{model.FeatureTestControl, "TCTRL"},
	}

	for _, kf := range knownFeatures {
		code, ok := FeatureTypeToPICSCode[uint8(kf.ft)]
		if !ok {
			t.Errorf("FeatureType %s (0x%02X) has no PICS code mapping", kf.ft, uint8(kf.ft))
			continue
		}
		if code != kf.code {
			t.Errorf("FeatureType %s: got PICS code %q, want %q", kf.ft, code, kf.code)
		}
	}
}

func TestPICSCodeToFeatureType_ReverseMapping(t *testing.T) {
	// Verify the reverse mapping is consistent.
	for id, code := range FeatureTypeToPICSCode {
		reverseID, ok := PICSCodeToFeatureType[code]
		if !ok {
			t.Errorf("PICS code %q has no reverse mapping", code)
			continue
		}
		if reverseID != id {
			t.Errorf("reverse mapping for %q: got 0x%02X, want 0x%02X", code, reverseID, id)
		}
	}
}

func TestFeatureTypeToPICSCode_Count(t *testing.T) {
	// Ensure we haven't accidentally lost mappings.
	if len(FeatureTypeToPICSCode) != len(PICSCodeToFeatureType) {
		t.Errorf("forward map has %d entries, reverse has %d", len(FeatureTypeToPICSCode), len(PICSCodeToFeatureType))
	}
}
