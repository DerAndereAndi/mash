package runner

import (
	"fmt"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/pics"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

func TestParseAutoPICSEndpoints(t *testing.T) {
	// Simulate CBOR-decoded endpoints attribute: two endpoints.
	raw := []any{
		map[any]any{
			uint64(1): uint64(0),    // id
			uint64(2): uint64(0x00), // type = DEVICE_ROOT
			uint64(3): "Root",       // label
			uint64(4): []any{uint64(0x01)}, // features: DeviceInfo
		},
		map[any]any{
			uint64(1): uint64(1),    // id
			uint64(2): uint64(0x05), // type = EV_CHARGER
			uint64(3): "Charger",
			uint64(4): []any{uint64(0x03), uint64(0x04), uint64(0x05)}, // Electrical, Measurement, EnergyControl
		},
	}

	eps := parseAutoPICSEndpoints(raw)
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	if eps[0].id != 0 || eps[0].epType != 0x00 || eps[0].label != "Root" {
		t.Errorf("endpoint 0: got id=%d type=%d label=%q", eps[0].id, eps[0].epType, eps[0].label)
	}
	if len(eps[0].features) != 1 || eps[0].features[0] != 0x01 {
		t.Errorf("endpoint 0 features: %v", eps[0].features)
	}

	if eps[1].id != 1 || eps[1].epType != 0x05 {
		t.Errorf("endpoint 1: got id=%d type=%d", eps[1].id, eps[1].epType)
	}
	if len(eps[1].features) != 3 {
		t.Errorf("endpoint 1 features: expected 3, got %d", len(eps[1].features))
	}
}

func TestParseAutoPICSEndpoints_Nil(t *testing.T) {
	eps := parseAutoPICSEndpoints(nil)
	if eps != nil {
		t.Errorf("expected nil for nil input, got %v", eps)
	}
}

func TestParseAutoPICSUseCases(t *testing.T) {
	raw := []any{
		map[any]any{
			uint64(1): uint64(1),    // endpointID
			uint64(2): uint64(0x01), // id = GPL
			uint64(3): uint64(1),    // major
			uint64(4): uint64(0),    // minor
			uint64(5): uint64(0x03), // scenarios: bits 0 and 1 set
		},
		map[any]any{
			uint64(1): uint64(1),
			uint64(2): uint64(0x03), // id = EVC
			uint64(3): uint64(1),
			uint64(4): uint64(0),
			uint64(5): uint64(0x07), // scenarios: bits 0, 1, 2
		},
	}

	ucs := parseAutoPICSUseCases(raw)
	if len(ucs) != 2 {
		t.Fatalf("expected 2 use cases, got %d", len(ucs))
	}

	if ucs[0].id != 0x01 || ucs[0].scenarios != 0x03 {
		t.Errorf("uc[0]: id=0x%02X scenarios=0x%02X", ucs[0].id, ucs[0].scenarios)
	}
	if ucs[1].id != 0x03 || ucs[1].scenarios != 0x07 {
		t.Errorf("uc[1]: id=0x%02X scenarios=0x%02X", ucs[1].id, ucs[1].scenarios)
	}
}

func TestParseReadPayload(t *testing.T) {
	// CBOR decoding typically produces map[any]any with uint64 keys.
	raw := map[any]any{
		uint64(1):      "device-123",
		uint64(12):     "1.0",
		uint64(0xFFFC): uint64(0x0003),
	}

	result := parseReadPayload(raw)
	if v, ok := result[1].(string); !ok || v != "device-123" {
		t.Errorf("attr 1: got %v", result[1])
	}
	if v, ok := result[12].(string); !ok || v != "1.0" {
		t.Errorf("attr 12: got %v", result[12])
	}
	if result[0xFFFC] == nil {
		t.Error("attr 0xFFFC missing")
	}
}

func TestParseReadPayload_Nil(t *testing.T) {
	result := parseReadPayload(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil input, got %v", result)
	}
}

func TestBuildPICSItems_Endpoints(t *testing.T) {
	// Test the PICS code generation for endpoint items.
	// Endpoint 1, type EV_CHARGER (0x05), with features Electrical (0x03) and Measurement (0x04).
	eps := []autoPICSEndpoint{
		{
			id:       1,
			epType:   0x05,
			features: []uint16{0x03, 0x04},
		},
	}

	items := make(map[string]any)
	for _, ep := range eps {
		epCode := "MASH.S.E01"
		items[epCode] = model.EndpointType(ep.epType).String()

		for _, featID := range ep.features {
			picsCode, ok := pics.FeatureTypeToPICSCode[uint8(featID)]
			if !ok {
				t.Errorf("no PICS code for feature 0x%02X", featID)
				continue
			}
			items[epCode+"."+picsCode] = true
		}
	}

	if items["MASH.S.E01"] != "EV_CHARGER" {
		t.Errorf("endpoint type: got %v", items["MASH.S.E01"])
	}
	if items["MASH.S.E01.ELEC"] != true {
		t.Error("expected MASH.S.E01.ELEC = true")
	}
	if items["MASH.S.E01.MEAS"] != true {
		t.Error("expected MASH.S.E01.MEAS = true")
	}
}

func TestBuildPICSItems_UseCases(t *testing.T) {
	ucs := []autoPICSUseCase{
		{id: uint16(usecase.GPLID), scenarios: 0x07}, // bits 0, 1, 2
		{id: uint16(usecase.EVCID), scenarios: 0x01}, // bit 0 only
	}

	items := make(map[string]any)
	for _, uc := range ucs {
		name, ok := usecase.IDToName[usecase.UseCaseID(uc.id)]
		if !ok {
			t.Errorf("unknown use case ID 0x%02X", uc.id)
			continue
		}
		ucKey := "MASH.S.UC." + string(name)
		items[ucKey] = true
		for bit := 0; bit < 32; bit++ {
			if uc.scenarios&(1<<bit) != 0 {
				items[ucKey+".S"+fmt.Sprintf("%02d", bit)] = true
			}
		}
	}

	if items["MASH.S.UC.GPL"] != true {
		t.Error("expected MASH.S.UC.GPL = true")
	}
	if items["MASH.S.UC.GPL.S00"] != true {
		t.Error("expected MASH.S.UC.GPL.S00 = true")
	}
	if items["MASH.S.UC.GPL.S01"] != true {
		t.Error("expected MASH.S.UC.GPL.S01 = true")
	}
	if items["MASH.S.UC.GPL.S02"] != true {
		t.Error("expected MASH.S.UC.GPL.S02 = true")
	}
	if items["MASH.S.UC.EVC"] != true {
		t.Error("expected MASH.S.UC.EVC = true")
	}
	if items["MASH.S.UC.EVC.S00"] != true {
		t.Error("expected MASH.S.UC.EVC.S00 = true")
	}
	// S01 should NOT be set for EVC
	if _, exists := items["MASH.S.UC.EVC.S01"]; exists {
		t.Error("MASH.S.UC.EVC.S01 should not be set")
	}
}

func TestBuildPICSItems_Attributes(t *testing.T) {
	// Verify attribute hex formatting.
	attrList := []uint16{0x01, 0x0A, 0x10, 0xFF}
	featKey := "MASH.S.E01.CTRL"

	items := make(map[string]any)
	for _, attrID := range attrList {
		items[fmt.Sprintf("%s.A%02X", featKey, attrID)] = true
	}

	expected := []string{
		"MASH.S.E01.CTRL.A01",
		"MASH.S.E01.CTRL.A0A",
		"MASH.S.E01.CTRL.A10",
		"MASH.S.E01.CTRL.AFF",
	}
	for _, key := range expected {
		if items[key] != true {
			t.Errorf("expected %s = true", key)
		}
	}
}

func TestBuildPICSItems_Commands(t *testing.T) {
	cmdList := []uint8{0x01, 0x10}
	featKey := "MASH.S.E01.CTRL"

	items := make(map[string]any)
	for _, cmdID := range cmdList {
		items[fmt.Sprintf("%s.C%02X.Rsp", featKey, cmdID)] = true
	}

	if items["MASH.S.E01.CTRL.C01.Rsp"] != true {
		t.Error("expected MASH.S.E01.CTRL.C01.Rsp = true")
	}
	if items["MASH.S.E01.CTRL.C10.Rsp"] != true {
		t.Error("expected MASH.S.E01.CTRL.C10.Rsp = true")
	}
}

func TestBuildPICSItems_FeatureMap(t *testing.T) {
	// featureMap with bits 0 and 3 set (0x09 = CORE + EMOB).
	featMap := uint32(0x09)
	featKey := "MASH.S.E01.CHRG"

	items := make(map[string]any)
	for bit := 0; bit < 32; bit++ {
		if featMap&(1<<bit) != 0 {
			items[fmt.Sprintf("%s.F%02X", featKey, bit)] = true
		}
	}

	if items["MASH.S.E01.CHRG.F00"] != true {
		t.Error("expected MASH.S.E01.CHRG.F00 = true (bit 0)")
	}
	if items["MASH.S.E01.CHRG.F03"] != true {
		t.Error("expected MASH.S.E01.CHRG.F03 = true (bit 3)")
	}
	if _, exists := items["MASH.S.E01.CHRG.F01"]; exists {
		t.Error("MASH.S.E01.CHRG.F01 should not be set")
	}
}
