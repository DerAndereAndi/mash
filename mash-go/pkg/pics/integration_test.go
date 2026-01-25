package pics

import (
	"path/filepath"
	"runtime"
	"testing"
)

// getTestdataPath returns the path to the testdata directory.
func getTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "pics")
}

func TestParseRealPICSFile(t *testing.T) {
	path := filepath.Join(getTestdataPath(), "minimal-device-pairing.pics")

	pics, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify basic structure
	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}

	if pics.Side != SideServer {
		t.Errorf("expected Side=S, got %v", pics.Side)
	}

	if pics.Version != 1 {
		t.Errorf("expected Version=1, got %d", pics.Version)
	}

	// Check transport features
	if !pics.Has("MASH.S.TRANS") {
		t.Error("expected TRANS feature to be present")
	}

	if !pics.Has("MASH.S.TRANS.TLS13") {
		t.Error("expected TLS13 to be present")
	}

	// Check commissioning features
	if !pics.Has("MASH.S.COMM") {
		t.Error("expected COMM feature to be present")
	}

	if !pics.Has("MASH.S.COMM.PASE") {
		t.Error("expected PASE to be present")
	}

	// Validate the PICS
	result := ValidatePICS(pics)
	if !result.Valid {
		t.Errorf("PICS validation failed: %v", result.Errors)
	}

	// Log some info
	t.Logf("Parsed %d entries, %d features", len(pics.Entries), len(pics.Features))
}
