package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// In real-device mode, enter/exit commissioning actions must not silently
// succeed when trigger delivery fails. Otherwise tests assume the device state
// changed while the device remains unchanged.
func TestEnterCommissioningMode_RealModeFailsWhenTriggerCannotBeSent(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:8443"
	r.config.EnableKey = "00112233445566778899aabbccddeeff"

	// No suite connection and no main connection: trigger delivery cannot work.
	if r.pool.Main() != nil {
		r.pool.Main().state = ConnDisconnected
	}
	r.suite.SetConn(nil)

	state := engine.NewExecutionState(context.Background())
	step := &loader.Step{Action: ActionEnterCommissioningMode}

	_, err := r.handleEnterCommissioningMode(context.Background(), step, state)
	if err == nil {
		t.Fatal("expected error when enter commissioning trigger cannot be delivered")
	}
	if !strings.Contains(err.Error(), "trigger") {
		t.Fatalf("expected trigger-related error, got: %v", err)
	}
	if v, ok := state.Get(StateCommissioningActive); ok && v == true {
		t.Fatal("commissioning state should not be set active when trigger delivery fails")
	}
}

func TestExitCommissioningMode_RealModeFailsWhenTriggerCannotBeSent(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:8443"
	r.config.EnableKey = "00112233445566778899aabbccddeeff"

	// No suite connection and no main connection: trigger delivery cannot work.
	if r.pool.Main() != nil {
		r.pool.Main().state = ConnDisconnected
	}
	r.suite.SetConn(nil)

	state := engine.NewExecutionState(context.Background())
	step := &loader.Step{Action: ActionExitCommissioningMode}

	_, err := r.handleExitCommissioningMode(context.Background(), step, state)
	if err == nil {
		t.Fatal("expected error when exit commissioning trigger cannot be delivered")
	}
	if !strings.Contains(err.Error(), "trigger") {
		t.Fatalf("expected trigger-related error, got: %v", err)
	}
	if v, ok := state.Get(StateCommissioningActive); ok && v == false {
		t.Fatal("commissioning state should not be forced false when trigger delivery fails")
	}
}
