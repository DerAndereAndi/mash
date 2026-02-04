package service

import (
	"testing"
	"time"
)

func TestDefaultSnapshotPolicy(t *testing.T) {
	policy := DefaultSnapshotPolicy()

	if policy.MaxInterval != 30*time.Minute {
		t.Errorf("MaxInterval: got %v, want %v", policy.MaxInterval, 30*time.Minute)
	}
	if policy.MaxMessages != 1000 {
		t.Errorf("MaxMessages: got %d, want 1000", policy.MaxMessages)
	}
	if policy.MinMessages != 50 {
		t.Errorf("MinMessages: got %d, want 50", policy.MinMessages)
	}
}

func TestDefaultDeviceConfigIncludesSnapshotPolicy(t *testing.T) {
	cfg := DefaultDeviceConfig()
	expected := DefaultSnapshotPolicy()

	if cfg.SnapshotPolicy != expected {
		t.Errorf("DeviceConfig.SnapshotPolicy: got %+v, want %+v", cfg.SnapshotPolicy, expected)
	}
}

func TestDefaultDeviceConfig_StaleConnectionTimeout(t *testing.T) {
	cfg := DefaultDeviceConfig()
	if cfg.StaleConnectionTimeout != 90*time.Second {
		t.Errorf("StaleConnectionTimeout: got %v, want 90s", cfg.StaleConnectionTimeout)
	}
}

func TestDefaultDeviceConfig_ReaperInterval(t *testing.T) {
	cfg := DefaultDeviceConfig()
	if cfg.ReaperInterval != 10*time.Second {
		t.Errorf("ReaperInterval: got %v, want 10s", cfg.ReaperInterval)
	}
}

func TestDefaultControllerConfigIncludesSnapshotPolicy(t *testing.T) {
	cfg := DefaultControllerConfig()
	expected := DefaultSnapshotPolicy()

	if cfg.SnapshotPolicy != expected {
		t.Errorf("ControllerConfig.SnapshotPolicy: got %+v, want %+v", cfg.SnapshotPolicy, expected)
	}
}
