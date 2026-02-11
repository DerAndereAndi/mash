package version

import (
	"testing"
)

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		input string
		major uint16
		minor uint16
	}{
		{"1.0", 1, 0},
		{"1.1", 1, 1},
		{"2.0", 2, 0},
		{"10.23", 10, 23},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.input, err)
			}
			if v.Major != tt.major {
				t.Errorf("Major = %d, want %d", v.Major, tt.major)
			}
			if v.Minor != tt.minor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.minor)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []string{
		"",
		"1",
		"abc",
		"1.0.0",
		"1.x",
		"-1.0",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			if err == nil {
				t.Errorf("Parse(%q) should return error", input)
			}
		})
	}
}

func TestSpecVersion_MajorMinor(t *testing.T) {
	v, err := Parse("1.0")
	if err != nil {
		t.Fatal(err)
	}
	if v.Major != 1 {
		t.Errorf("Major = %d, want 1", v.Major)
	}
	if v.Minor != 0 {
		t.Errorf("Minor = %d, want 0", v.Minor)
	}
}

func TestSpecVersion_String(t *testing.T) {
	v, err := Parse("1.0")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "1.0" {
		t.Errorf("String() = %q, want %q", v.String(), "1.0")
	}

	v2, err := Parse("10.23")
	if err != nil {
		t.Fatal(err)
	}
	if v2.String() != "10.23" {
		t.Errorf("String() = %q, want %q", v2.String(), "10.23")
	}
}

func TestCompatible_SameMajor(t *testing.T) {
	v1, _ := Parse("1.0")
	v2, _ := Parse("1.1")

	if !v1.Compatible(v2) {
		t.Error("1.0 should be compatible with 1.1")
	}
	if !v2.Compatible(v1) {
		t.Error("1.1 should be compatible with 1.0")
	}
}

func TestCompatible_DifferentMajor(t *testing.T) {
	v1, _ := Parse("1.0")
	v2, _ := Parse("2.0")

	if v1.Compatible(v2) {
		t.Error("1.0 should NOT be compatible with 2.0")
	}
	if v2.Compatible(v1) {
		t.Error("2.0 should NOT be compatible with 1.0")
	}
}

func TestALPNProtocol(t *testing.T) {
	if got := ALPNProtocol(1); got != "mash/1" {
		t.Errorf("ALPNProtocol(1) = %q, want %q", got, "mash/1")
	}
	if got := ALPNProtocol(2); got != "mash/2" {
		t.Errorf("ALPNProtocol(2) = %q, want %q", got, "mash/2")
	}
}

func TestMajorFromALPN(t *testing.T) {
	tests := []struct {
		input   string
		want    uint16
		wantErr bool
	}{
		{"mash/1", 1, false},
		{"mash/2", 2, false},
		{"mash-comm/1", 1, false},
		{"mash-comm/2", 2, false},
		{"http/1.1", 0, true},
		{"mash/", 0, true},
		{"mash-comm/", 0, true},
		{"", 0, true},
		{"mash/abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := MajorFromALPN(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("MajorFromALPN(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MajorFromALPN(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedALPNProtocols(t *testing.T) {
	protos := SupportedALPNProtocols()
	if len(protos) != 1 {
		t.Fatalf("SupportedALPNProtocols() returned %d protocols, want 1", len(protos))
	}
	if protos[0] != "mash/1" {
		t.Errorf("SupportedALPNProtocols()[0] = %q, want %q", protos[0], "mash/1")
	}
}

func TestCurrent(t *testing.T) {
	v, err := Parse(Current)
	if err != nil {
		t.Fatalf("Parse(Current) returned error: %v", err)
	}
	if v.Major != 1 || v.Minor != 0 {
		t.Errorf("Current version = %s, want 1.0", v)
	}
}
