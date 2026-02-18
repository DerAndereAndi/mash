package runner

import "testing"

func TestExtractPASEErrorCode_ParsesParenthesizedCodeFormat(t *testing.T) {
	code, ok := extractPASEErrorCode("cert exchange: CSR: commissioning error (zone type already exists, code 10)")
	if !ok {
		t.Fatal("expected code to be parsed")
	}
	if code != 10 {
		t.Fatalf("expected code 10, got %d", code)
	}
}

