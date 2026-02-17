package runner

import "testing"

func TestLifecycle_Transition_DisconnectedToCommissioning(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToCommissioning("main"); err != nil {
		t.Fatalf("ToCommissioning failed: %v", err)
	}
	if got := c.State(); got != LifecycleCommissioning {
		t.Fatalf("state=%v want=%v", got, LifecycleCommissioning)
	}
	if got := c.Authority(); got != "main" {
		t.Fatalf("authority=%q want=%q", got, "main")
	}
}

func TestLifecycle_Transition_CommissioningToOperational(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToCommissioning("main"); err != nil {
		t.Fatalf("ToCommissioning failed: %v", err)
	}
	if err := c.ToOperational("main-zone-1"); err != nil {
		t.Fatalf("ToOperational failed: %v", err)
	}
	if got := c.State(); got != LifecycleOperational {
		t.Fatalf("state=%v want=%v", got, LifecycleOperational)
	}
	if got := c.Authority(); got != "main-zone-1" {
		t.Fatalf("authority=%q want=%q", got, "main-zone-1")
	}
}

func TestLifecycle_Transition_OperationalToDisconnected(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToCommissioning("main"); err != nil {
		t.Fatalf("ToCommissioning failed: %v", err)
	}
	if err := c.ToOperational("main-zone-1"); err != nil {
		t.Fatalf("ToOperational failed: %v", err)
	}
	c.ToDisconnected()
	if got := c.State(); got != LifecycleDisconnected {
		t.Fatalf("state=%v want=%v", got, LifecycleDisconnected)
	}
	if got := c.Authority(); got != "" {
		t.Fatalf("authority=%q want empty", got)
	}
}

func TestLifecycle_InvalidTransitionRejected(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToOperational("main-zone-1"); err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestLifecycle_ControlChannelAuthority_IsUnique(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToCommissioning("main"); err != nil {
		t.Fatalf("ToCommissioning failed: %v", err)
	}
	if err := c.ToCommissioning("suite"); err == nil {
		t.Fatal("expected authority conflict error")
	}
}

func TestLifecycle_ReconnectOperational_FromCommissioning(t *testing.T) {
	c := NewLifecycleController()
	if err := c.ToCommissioning("main"); err != nil {
		t.Fatalf("ToCommissioning failed: %v", err)
	}
	if err := c.ReconnectOperational("suite-zone"); err != nil {
		t.Fatalf("ReconnectOperational failed: %v", err)
	}
	if got := c.State(); got != LifecycleOperational {
		t.Fatalf("state=%v want=%v", got, LifecycleOperational)
	}
	if got := c.Authority(); got != "suite-zone" {
		t.Fatalf("authority=%q want=%q", got, "suite-zone")
	}
}
