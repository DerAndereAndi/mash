package model

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestUseCaseDecl_CBORRoundTrip(t *testing.T) {
	decl := UseCaseDecl{
		EndpointID: 1,
		Name:       "LPC",
		Major:      1,
		Minor:      0,
	}

	data, err := cbor.Marshal(decl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got UseCaseDecl
	if err := cbor.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.EndpointID != 1 {
		t.Errorf("EndpointID = %d, want 1", got.EndpointID)
	}
	if got.Name != "LPC" {
		t.Errorf("Name = %q, want LPC", got.Name)
	}
	if got.Major != 1 {
		t.Errorf("Major = %d, want 1", got.Major)
	}
	if got.Minor != 0 {
		t.Errorf("Minor = %d, want 0", got.Minor)
	}
}

func TestUseCaseDecl_CBORIntegerKeys(t *testing.T) {
	decl := UseCaseDecl{
		EndpointID: 2,
		Name:       "EVC",
		Major:      1,
		Minor:      3,
	}

	data, err := cbor.Marshal(decl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Decode into raw map to verify integer keys
	var raw map[uint64]any
	if err := cbor.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// Key 1 = endpointId
	if v, ok := raw[1]; !ok {
		t.Error("missing key 1 (endpointId)")
	} else if v != uint64(2) {
		t.Errorf("key 1 = %v, want 2", v)
	}

	// Key 2 = name
	if v, ok := raw[2]; !ok {
		t.Error("missing key 2 (name)")
	} else if v != "EVC" {
		t.Errorf("key 2 = %v, want EVC", v)
	}

	// Key 3 = major
	if v, ok := raw[3]; !ok {
		t.Error("missing key 3 (major)")
	} else if v != uint64(1) {
		t.Errorf("key 3 = %v, want 1", v)
	}

	// Key 4 = minor
	if v, ok := raw[4]; !ok {
		t.Error("missing key 4 (minor)")
	} else if v != uint64(3) {
		t.Errorf("key 4 = %v, want 3", v)
	}
}

func TestUseCaseDecl_MinorZeroEncoded(t *testing.T) {
	// minor=0 must still appear in CBOR (no omitempty)
	decl := UseCaseDecl{
		EndpointID: 1,
		Name:       "MPD",
		Major:      1,
		Minor:      0,
	}

	data, err := cbor.Marshal(decl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[uint64]any
	if err := cbor.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := raw[4]; !ok {
		t.Error("key 4 (minor) missing -- zero values must be encoded")
	}
}
