package inspect_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/inspect"
	"github.com/mash-protocol/mash-go/pkg/model"
)

func TestResolveCommandName(t *testing.T) {
	tests := []struct {
		name      string
		feature   uint8
		command   string
		wantID    uint8
		wantFound bool
	}{
		{"tariff SetTariff", uint8(model.FeatureTariff), "settariff", 1, true},
		{"signals SendPriceSignal", uint8(model.FeatureSignals), "sendpricesignal", 1, true},
		{"signals ClearSignals", uint8(model.FeatureSignals), "clearsignals", 4, true},
		{"energycontrol SetLimit", uint8(model.FeatureEnergyControl), "setlimit", 1, true},
		{"energycontrol Stop", uint8(model.FeatureEnergyControl), "stop", 11, true},
		{"plan RequestPlan", uint8(model.FeaturePlan), "requestplan", 1, true},
		{"chargingsession SetChargingMode", uint8(model.FeatureChargingSession), "setchargingmode", 1, true},
		{"deviceinfo RemoveZone", uint8(model.FeatureDeviceInfo), "removezone", 16, true},
		{"testcontrol TriggerTestEvent", uint8(model.FeatureTestControl), "triggertestevent", 1, true},
		{"nonexistent command", uint8(model.FeatureTariff), "nonexistent", 0, false},
		{"wrong feature for command", uint8(model.FeatureMeasurement), "settariff", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, found := inspect.ResolveCommandName(tt.feature, tt.command)
			if found != tt.wantFound {
				t.Errorf("ResolveCommandName(0x%02x, %q) found = %v, want %v", tt.feature, tt.command, found, tt.wantFound)
			}
			if id != tt.wantID {
				t.Errorf("ResolveCommandName(0x%02x, %q) = %d, want %d", tt.feature, tt.command, id, tt.wantID)
			}
		})
	}
}

func TestGetCommandName(t *testing.T) {
	tests := []struct {
		name     string
		feature  uint8
		cmdID    uint8
		wantName string
	}{
		{"tariff cmd 1", uint8(model.FeatureTariff), 1, "settariff"},
		{"signals cmd 4", uint8(model.FeatureSignals), 4, "clearsignals"},
		{"unknown cmd", uint8(model.FeatureTariff), 99, ""},
		{"unknown feature", 0xFF, 1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := inspect.GetCommandName(tt.feature, tt.cmdID)
			if name != tt.wantName {
				t.Errorf("GetCommandName(0x%02x, %d) = %q, want %q", tt.feature, tt.cmdID, name, tt.wantName)
			}
		})
	}
}
