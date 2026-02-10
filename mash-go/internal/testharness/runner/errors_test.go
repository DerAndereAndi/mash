package runner

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestErrorCategory(t *testing.T) {
	base := fmt.Errorf("some error")

	infra := Infrastructure(base)
	if Category(infra) != ErrCatInfrastructure {
		t.Fatalf("expected infrastructure, got %v", Category(infra))
	}

	dev := Device(base)
	if Category(dev) != ErrCatDevice {
		t.Fatalf("expected device, got %v", Category(dev))
	}

	proto := Protocol(base)
	if Category(proto) != ErrCatProtocol {
		t.Fatalf("expected protocol, got %v", Category(proto))
	}
}

func TestCategoryDefaultsToProtocol(t *testing.T) {
	plain := fmt.Errorf("unclassified error")
	if Category(plain) != ErrCatProtocol {
		t.Fatalf("expected protocol for unclassified, got %v", Category(plain))
	}
}

func TestErrorUnwrap(t *testing.T) {
	base := fmt.Errorf("base error")
	wrapped := Infrastructure(base)

	if !errors.Is(wrapped, base) {
		t.Fatal("errors.Is should find base error through ClassifiedError")
	}

	var ce *ClassifiedError
	if !errors.As(wrapped, &ce) {
		t.Fatal("errors.As should find ClassifiedError")
	}
	if ce.Category != ErrCatInfrastructure {
		t.Fatalf("expected infrastructure, got %v", ce.Category)
	}
}

func TestClassifyPASEError_Infrastructure(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"EOF", io.EOF},
		{"connection reset", fmt.Errorf("connection reset by peer")},
		{"broken pipe", fmt.Errorf("write: broken pipe")},
		{"cooldown active", fmt.Errorf("commissioning error code 5: cooldown active (4.5s remaining)")},
		{"already in progress", fmt.Errorf("commissioning already in progress")},
		{"error code 5", fmt.Errorf("PASE failed: error code 5: device busy")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classified := classifyPASEError(tt.err)
			if Category(classified) != ErrCatInfrastructure {
				t.Fatalf("expected infrastructure for %q, got %v", tt.err, Category(classified))
			}
			// Unwrap should find original error.
			if !errors.Is(classified, tt.err) {
				t.Fatal("should unwrap to original error")
			}
		})
	}
}

func TestClassifyPASEError_Device(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"auth failed (code 1)", fmt.Errorf("PASE failed: error code 1: authentication failed")},
		{"confirm failed (code 2)", fmt.Errorf("PASE failed: error code 2: confirmation failed")},
		{"CSR failed (code 3)", fmt.Errorf("PASE failed: error code 3: CSR failed")},
		{"cert install (code 4)", fmt.Errorf("PASE failed: error code 4: cert install failed")},
		{"zone type exists (code 10)", fmt.Errorf("commissioning error code 10: zone type already exists")},
		{"zone slots full", fmt.Errorf("zone slots full")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classified := classifyPASEError(tt.err)
			if Category(classified) != ErrCatDevice {
				t.Fatalf("expected device for %q, got %v", tt.err, Category(classified))
			}
		})
	}
}

func TestClassifyPASEError_Protocol(t *testing.T) {
	// Unknown errors default to protocol (conservative).
	err := fmt.Errorf("some unknown PASE error")
	classified := classifyPASEError(err)
	if Category(classified) != ErrCatProtocol {
		t.Fatalf("expected protocol for unknown error, got %v", Category(classified))
	}
}

func TestClassifyPASEError_Nil(t *testing.T) {
	if classifyPASEError(nil) != nil {
		t.Fatal("nil error should return nil")
	}
}

func TestIsTransientError_BackwardCompat(t *testing.T) {
	// Classified infrastructure errors should be detected as transient.
	infraErr := Infrastructure(fmt.Errorf("connection reset"))
	if !isTransientError(infraErr) {
		t.Fatal("classified infrastructure error should be transient")
	}

	// Classified device errors should NOT be transient.
	deviceErr := Device(fmt.Errorf("zone slots full"))
	if isTransientError(deviceErr) {
		t.Fatal("classified device error should not be transient")
	}

	// Unclassified IO errors should still be transient (backward compat).
	plainEOF := io.EOF
	if !isTransientError(plainEOF) {
		t.Fatal("plain EOF should be transient")
	}
}
