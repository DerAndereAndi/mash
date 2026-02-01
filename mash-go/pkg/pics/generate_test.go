package pics

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

func TestGenerateUseCaseCodes_DeviceDecls(t *testing.T) {
	decls := []*model.UseCaseDecl{
		{EndpointID: 1, ID: uint16(usecase.LPCID), Major: 1, Minor: 0, Scenarios: 0x01},
		{EndpointID: 1, ID: uint16(usecase.MPDID), Major: 1, Minor: 0, Scenarios: 0x01},
		{EndpointID: 1, ID: uint16(usecase.EVCID), Major: 1, Minor: 0, Scenarios: 0x01},
	}

	entries := GenerateUseCaseCodes(decls, SideServer)

	// Each use case produces 1 UC entry + 1 scenario entry (BASE only) = 2 per UC = 6 total
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries (3 UCs + 3 BASE scenarios), got %d", len(entries))
	}

	// Verify UC presence entries exist (sorted)
	ucEntries := make(map[string]bool)
	for _, e := range entries {
		ucEntries[e.Code.String()] = true
	}

	for _, expected := range []string{"MASH.S.UC.EVC", "MASH.S.UC.LPC", "MASH.S.UC.MPD"} {
		if !ucEntries[expected] {
			t.Errorf("missing expected entry %s", expected)
		}
	}
	for _, expected := range []string{"MASH.S.UC.EVC.S00", "MASH.S.UC.LPC.S00", "MASH.S.UC.MPD.S00"} {
		if !ucEntries[expected] {
			t.Errorf("missing expected scenario entry %s", expected)
		}
	}
}

func TestGenerateUseCaseCodes_ControllerDecls(t *testing.T) {
	decls := []*model.UseCaseDecl{
		{EndpointID: 0, ID: uint16(usecase.LPCID), Major: 1, Minor: 0, Scenarios: 0x07}, // BASE + S1 + S2
		{EndpointID: 0, ID: uint16(usecase.LPPID), Major: 1, Minor: 0, Scenarios: 0x01}, // BASE only
		{EndpointID: 0, ID: uint16(usecase.MPDID), Major: 1, Minor: 0, Scenarios: 0x01}, // BASE only
	}

	entries := GenerateUseCaseCodes(decls, SideClient)

	// LPC: UC + S00 + S01 + S02 = 4
	// LPP: UC + S00 = 2
	// MPD: UC + S00 = 2
	// Total = 8
	if len(entries) != 8 {
		t.Fatalf("expected 8 entries, got %d", len(entries))
	}

	ucEntries := make(map[string]bool)
	for _, e := range entries {
		ucEntries[e.Code.String()] = true
	}

	for _, expected := range []string{"MASH.C.UC.LPC", "MASH.C.UC.LPP", "MASH.C.UC.MPD"} {
		if !ucEntries[expected] {
			t.Errorf("missing expected entry %s", expected)
		}
	}
	// Check LPC scenarios
	for _, expected := range []string{"MASH.C.UC.LPC.S00", "MASH.C.UC.LPC.S01", "MASH.C.UC.LPC.S02"} {
		if !ucEntries[expected] {
			t.Errorf("missing expected scenario entry %s", expected)
		}
	}
}

func TestGenerateUseCaseCodes_Deduplication(t *testing.T) {
	// Same UC declared on multiple endpoints -- should produce only one set of entries
	decls := []*model.UseCaseDecl{
		{EndpointID: 1, ID: uint16(usecase.LPCID), Major: 1, Minor: 0, Scenarios: 0x01},
		{EndpointID: 2, ID: uint16(usecase.LPCID), Major: 1, Minor: 0, Scenarios: 0x01},
		{EndpointID: 1, ID: uint16(usecase.MPDID), Major: 1, Minor: 0, Scenarios: 0x01},
	}

	entries := GenerateUseCaseCodes(decls, SideServer)

	// LPC (deduped) + MPD = 2 UCs, each with 1 scenario = 4 entries
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (deduplicated), got %d", len(entries))
	}
}

func TestGenerateUseCaseCodes_Empty(t *testing.T) {
	entries := GenerateUseCaseCodes(nil, SideServer)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nil decls, got %d", len(entries))
	}

	entries = GenerateUseCaseCodes([]*model.UseCaseDecl{}, SideServer)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty decls, got %d", len(entries))
	}
}
