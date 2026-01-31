package pics

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/model"
)

func TestGenerateUseCaseCodes_DeviceDecls(t *testing.T) {
	decls := []*model.UseCaseDecl{
		{EndpointID: 1, Name: "LPC", Major: 1, Minor: 0},
		{EndpointID: 1, Name: "MPD", Major: 1, Minor: 0},
		{EndpointID: 1, Name: "EVC", Major: 1, Minor: 0},
	}

	entries := GenerateUseCaseCodes(decls, SideServer)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Should be sorted by code string
	expected := []string{"MASH.S.UC.EVC", "MASH.S.UC.LPC", "MASH.S.UC.MPD"}
	for i, e := range entries {
		if e.Code.String() != expected[i] {
			t.Errorf("entries[%d].Code = %s, want %s", i, e.Code.String(), expected[i])
		}
		if !e.Value.IsTrue() {
			t.Errorf("entries[%d].Value should be true", i)
		}
	}
}

func TestGenerateUseCaseCodes_ControllerDecls(t *testing.T) {
	decls := []*model.UseCaseDecl{
		{EndpointID: 0, Name: "LPC", Major: 1, Minor: 0},
		{EndpointID: 0, Name: "LPP", Major: 1, Minor: 0},
		{EndpointID: 0, Name: "MPD", Major: 1, Minor: 0},
	}

	entries := GenerateUseCaseCodes(decls, SideClient)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	expected := []string{"MASH.C.UC.LPC", "MASH.C.UC.LPP", "MASH.C.UC.MPD"}
	for i, e := range entries {
		if e.Code.String() != expected[i] {
			t.Errorf("entries[%d].Code = %s, want %s", i, e.Code.String(), expected[i])
		}
	}
}

func TestGenerateUseCaseCodes_Deduplication(t *testing.T) {
	// Same UC declared on multiple endpoints -- should produce only one entry
	decls := []*model.UseCaseDecl{
		{EndpointID: 1, Name: "LPC", Major: 1, Minor: 0},
		{EndpointID: 2, Name: "LPC", Major: 1, Minor: 0},
		{EndpointID: 1, Name: "MPD", Major: 1, Minor: 0},
	}

	entries := GenerateUseCaseCodes(decls, SideServer)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (deduplicated), got %d", len(entries))
	}

	expected := []string{"MASH.S.UC.LPC", "MASH.S.UC.MPD"}
	for i, e := range entries {
		if e.Code.String() != expected[i] {
			t.Errorf("entries[%d].Code = %s, want %s", i, e.Code.String(), expected[i])
		}
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
