package loader

import (
	"testing"
)

func TestLoadRenewalTestCases(t *testing.T) {
	files := []string{
		"../../../testdata/cases/cert-renewal-normal.yaml",
		"../../../testdata/cases/cert-renewal-session.yaml",
		"../../../testdata/cases/cert-renewal-warning.yaml",
		"../../../testdata/cases/cert-renewal-expired.yaml",
		"../../../testdata/cases/cert-renewal-grace.yaml",
	}

	for _, f := range files {
		tc, err := LoadTestCase(f)
		if err != nil {
			t.Errorf("Failed to load %s: %v", f, err)
			continue
		}

		// Basic validation
		if tc.ID == "" {
			t.Errorf("%s: missing ID", f)
		}
		if tc.Name == "" {
			t.Errorf("%s: missing Name", f)
		}
		if len(tc.Steps) == 0 {
			t.Errorf("%s: no steps defined", f)
		}
		if len(tc.PICSRequirements) == 0 {
			t.Errorf("%s: no PICS requirements", f)
		}

		t.Logf("Loaded %s: %s (%d steps)", tc.ID, tc.Name, len(tc.Steps))
	}
}
