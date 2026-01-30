package usecase

import "testing"

func TestDeviceUseCases_SupportedCommands(t *testing.T) {
	du := &DeviceUseCases{
		DeviceID: "test-device",
		Matches: []MatchResult{
			{
				UseCase:    LPC,
				Matched:    true,
				EndpointID: 1,
			},
			{
				UseCase:    MPC,
				Matched:    true,
				EndpointID: 1,
			},
			{
				UseCase: LPP,
				Matched: false,
			},
		},
	}

	// Provide a registry for the test
	du.registry = map[UseCaseName]*UseCaseDef{
		LPC: {
			Name:     LPC,
			Commands: []string{"limit", "clear", "capacity", "override", "lpc-demo"},
		},
		LPP: {
			Name:     LPP,
			Commands: []string{"limit", "clear"},
		},
		MPC: {
			Name:     MPC,
			Commands: []string{},
		},
	}

	cmds := du.SupportedCommands()

	// LPC is matched, so its commands should be present
	for _, cmd := range []string{"limit", "clear", "capacity", "override", "lpc-demo"} {
		if !cmds[cmd] {
			t.Errorf("expected command %q to be supported", cmd)
		}
	}

	// MPC has no commands
	if len(cmds) != 5 {
		t.Errorf("expected 5 commands, got %d: %v", len(cmds), cmds)
	}
}

func TestDeviceUseCases_SupportedCommands_Empty(t *testing.T) {
	du := &DeviceUseCases{
		DeviceID: "test-device",
		Matches:  []MatchResult{},
		registry: map[UseCaseName]*UseCaseDef{},
	}

	cmds := du.SupportedCommands()
	if len(cmds) != 0 {
		t.Errorf("expected empty commands, got %v", cmds)
	}
}

func TestDeviceUseCases_HasUseCase(t *testing.T) {
	du := &DeviceUseCases{
		DeviceID: "test-device",
		Matches: []MatchResult{
			{UseCase: LPC, Matched: true, EndpointID: 1},
			{UseCase: MPC, Matched: true, EndpointID: 1},
			{UseCase: LPP, Matched: false},
		},
	}

	if !du.HasUseCase(LPC) {
		t.Error("expected HasUseCase(LPC) to be true")
	}
	if !du.HasUseCase(MPC) {
		t.Error("expected HasUseCase(MPC) to be true")
	}
	if du.HasUseCase(LPP) {
		t.Error("expected HasUseCase(LPP) to be false")
	}
	if du.HasUseCase("CEVC") {
		t.Error("expected HasUseCase(CEVC) to be false")
	}
}

func TestDeviceUseCases_EndpointForUseCase(t *testing.T) {
	du := &DeviceUseCases{
		DeviceID: "test-device",
		Matches: []MatchResult{
			{UseCase: LPC, Matched: true, EndpointID: 1},
			{UseCase: MPC, Matched: true, EndpointID: 2},
			{UseCase: LPP, Matched: false},
		},
	}

	epID, ok := du.EndpointForUseCase(LPC)
	if !ok || epID != 1 {
		t.Errorf("expected EndpointForUseCase(LPC) = (1, true), got (%d, %v)", epID, ok)
	}

	epID, ok = du.EndpointForUseCase(MPC)
	if !ok || epID != 2 {
		t.Errorf("expected EndpointForUseCase(MPC) = (2, true), got (%d, %v)", epID, ok)
	}

	_, ok = du.EndpointForUseCase(LPP)
	if ok {
		t.Error("expected EndpointForUseCase(LPP) to return false")
	}

	_, ok = du.EndpointForUseCase("CEVC")
	if ok {
		t.Error("expected EndpointForUseCase(CEVC) to return false")
	}
}

func TestDeviceUseCases_MatchedUseCases(t *testing.T) {
	du := &DeviceUseCases{
		DeviceID: "test-device",
		Matches: []MatchResult{
			{UseCase: LPC, Matched: true, EndpointID: 1},
			{UseCase: MPC, Matched: true, EndpointID: 1},
			{UseCase: LPP, Matched: false},
		},
	}

	names := du.MatchedUseCases()
	if len(names) != 2 {
		t.Fatalf("expected 2 matched use cases, got %d", len(names))
	}

	found := map[UseCaseName]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found[LPC] || !found[MPC] {
		t.Errorf("expected LPC and MPC in matched use cases, got %v", names)
	}
}
