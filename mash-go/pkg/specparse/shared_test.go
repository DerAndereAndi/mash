package specparse

import (
	"path/filepath"
	"testing"
)

func TestParseSharedTypes(t *testing.T) {
	yaml := `
version: "1.0"
enums:
  - name: Phase
    type: uint8
    description: "Device phase"
    values:
      - { name: A, value: 0x00 }
      - { name: B, value: 0x01 }
      - { name: C, value: 0x02 }
  - name: Direction
    type: uint8
    values:
      - { name: CONSUMPTION, value: 0x00 }
      - { name: PRODUCTION, value: 0x01 }
`
	shared, err := ParseSharedTypes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSharedTypes failed: %v", err)
	}
	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}
	if len(shared.Enums) != 2 {
		t.Fatalf("len(enums) = %d, want 2", len(shared.Enums))
	}
	if shared.Enums[0].Name != "Phase" || len(shared.Enums[0].Values) != 3 {
		t.Errorf("enums[0] = %+v, want Phase with 3 values", shared.Enums[0])
	}
}

func TestParseSharedTypesFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "_shared", "1.0.yaml")
	shared, err := LoadSharedTypes(path)
	if err != nil {
		t.Fatalf("LoadSharedTypes failed: %v", err)
	}

	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}

	// Expect 5 shared enums: Phase, GridPhase, PhasePair, Direction, AsymmetricSupport
	if len(shared.Enums) != 5 {
		t.Fatalf("len(enums) = %d, want 5", len(shared.Enums))
	}

	enumNames := make(map[string]int)
	for _, e := range shared.Enums {
		enumNames[e.Name] = len(e.Values)
	}

	expected := map[string]int{
		"Phase":             3,
		"GridPhase":         3,
		"PhasePair":         3,
		"Direction":         3,
		"AsymmetricSupport": 4,
	}
	for name, count := range expected {
		if got, ok := enumNames[name]; !ok {
			t.Errorf("missing enum %s", name)
		} else if got != count {
			t.Errorf("enum %s has %d values, want %d", name, got, count)
		}
	}
}
