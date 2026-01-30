package service

import (
	"errors"
	"testing"
)

func TestVerifySpecVersion_Compatible(t *testing.T) {
	err := checkVersionCompatibility("1.0")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestVerifySpecVersion_ForwardCompat(t *testing.T) {
	// Device is newer minor, same major -> compatible
	err := checkVersionCompatibility("1.1")
	if err != nil {
		t.Errorf("expected nil for 1.1, got %v", err)
	}
}

func TestVerifySpecVersion_BackwardCompat(t *testing.T) {
	// Device is older minor, same major -> compatible
	err := checkVersionCompatibility("1.0")
	if err != nil {
		t.Errorf("expected nil for 1.0, got %v", err)
	}
}

func TestVerifySpecVersion_Incompatible(t *testing.T) {
	err := checkVersionCompatibility("2.0")
	if err == nil {
		t.Fatal("expected error for major version mismatch")
	}
	if !errors.Is(err, ErrIncompatibleVersion) {
		t.Errorf("expected ErrIncompatibleVersion, got %v", err)
	}
}

func TestVerifySpecVersion_Default(t *testing.T) {
	// Empty specVersion -> assume compatible (backward compat with pre-versioning devices)
	err := checkVersionCompatibility("")
	if err != nil {
		t.Errorf("expected nil for empty version, got %v", err)
	}
}

func TestVerifySpecVersion_Malformed(t *testing.T) {
	err := checkVersionCompatibility("abc")
	if err == nil {
		t.Fatal("expected error for malformed version")
	}
	if !errors.Is(err, ErrIncompatibleVersion) {
		t.Errorf("expected ErrIncompatibleVersion, got %v", err)
	}
}
