package usecase

import "testing"

// Helper to build a test DeviceProfile.
func buildProfile(deviceID string, endpoints ...*EndpointProfile) *DeviceProfile {
	p := &DeviceProfile{
		DeviceID:  deviceID,
		Endpoints: make(map[uint8]*EndpointProfile),
	}
	for _, ep := range endpoints {
		p.Endpoints[ep.EndpointID] = ep
	}
	return p
}

func buildEndpoint(id uint8, epType string, feats ...*FeatureProfile) *EndpointProfile {
	ep := &EndpointProfile{
		EndpointID:   id,
		EndpointType: epType,
		Features:     make(map[uint8]*FeatureProfile),
	}
	for _, f := range feats {
		ep.Features[f.FeatureID] = f
	}
	return ep
}

func buildFeature(id uint8, attrIDs []uint16, cmdIDs []uint8, caps map[uint16]any) *FeatureProfile {
	if caps == nil {
		caps = make(map[uint16]any)
	}
	return &FeatureProfile{
		FeatureID:    id,
		AttributeIDs: attrIDs,
		CommandIDs:   cmdIDs,
		Attributes:   caps,
	}
}

func TestMatchAll_LPC_FullMatch(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05, // EnergyControl
				[]uint16{1, 2, 10, 20, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true}, // acceptsLimits = true
			),
			buildFeature(0x03, // Electrical
				[]uint16{1, 5, 10},
				nil, nil,
			),
			buildFeature(0x04, // Measurement
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(LPC) {
		t.Error("expected LPC to match")
	}
	if len(du.MatchedUseCases()) == 0 {
		t.Error("expected at least one matched use case")
	}
}

func TestMatchAll_LPC_MissingElectrical(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			// No Electrical feature
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if du.HasUseCase(LPC) {
		t.Error("LPC should not match without Electrical")
	}

	// Check MissingRequired
	for _, m := range du.Matches {
		if m.UseCase == LPC && !m.Matched {
			found := false
			for _, mr := range m.MissingRequired {
				if mr == "Electrical" {
					found = true
				}
			}
			if !found {
				t.Errorf("MissingRequired should contain 'Electrical', got %v", m.MissingRequired)
			}
		}
	}
}

func TestMatchAll_LPC_AcceptsLimitsFalse(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20},
				[]uint8{1, 2},
				map[uint16]any{10: false}, // acceptsLimits = false
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if du.HasUseCase(LPC) {
		t.Error("LPC should not match when acceptsLimits=false")
	}
}

func TestMatchAll_LPC_MeasurementOptional(t *testing.T) {
	// EnergyControl + Electrical present, Measurement absent
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10},
				nil, nil,
			),
			// No Measurement
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(LPC) {
		t.Error("LPC should match even without optional Measurement")
	}

	// Check OptionalMissing
	for _, m := range du.Matches {
		if m.UseCase == LPC && m.Matched {
			found := false
			for _, om := range m.OptionalMissing {
				if om == "Measurement" {
					found = true
				}
			}
			if !found {
				t.Errorf("OptionalMissing should contain 'Measurement', got %v", m.OptionalMissing)
			}
		}
	}
}

func TestMatchAll_LPC_WrongEndpointType(t *testing.T) {
	// GRID_CONNECTION is not in LPC's endpoint types
	profile := buildProfile("dev-1",
		buildEndpoint(1, "GRID_CONNECTION",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10},
				nil, nil,
			),
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if du.HasUseCase(LPC) {
		t.Error("LPC should not match GRID_CONNECTION endpoint")
	}
}

func TestMatchAll_LPP_Matched(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 22, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10, 11}, // has nominalMaxProduction (11)
				nil, nil,
			),
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(LPP) {
		t.Error("expected LPP to match INVERTER with nominalMaxProduction")
	}
}

func TestMatchAll_MPD_Matched(t *testing.T) {
	// MPD matches endpoints in its type list with Measurement
	profile := buildProfile("dev-1",
		buildEndpoint(1, "GRID_CONNECTION",
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(MPD) {
		t.Error("expected MPD to match GRID_CONNECTION with Measurement")
	}
}

func TestMatchAll_MPD_NoCommands(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "PV_STRING",
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	cmds := du.SupportedCommands()
	if len(cmds) != 0 {
		t.Errorf("MPD-only device should have no commands, got %v", cmds)
	}
}

func TestSupportedCommands_Union(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10},
				nil, nil,
			),
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)

	// Should have LPC + MPD (MPD has no commands)
	cmds := du.SupportedCommands()
	for _, cmd := range []string{"limit", "clear", "capacity", "override", "lpc-demo"} {
		if !cmds[cmd] {
			t.Errorf("expected command %q in union", cmd)
		}
	}
}

func TestEndpointForUseCase_FromMatch(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 22, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10, 11},
				nil, nil,
			),
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)

	epID, ok := du.EndpointForUseCase(LPC)
	if !ok || epID != 1 {
		t.Errorf("EndpointForUseCase(LPC) = (%d, %v), want (1, true)", epID, ok)
	}

	epID, ok = du.EndpointForUseCase(MPD)
	if !ok || epID != 1 {
		t.Errorf("EndpointForUseCase(MPD) = (%d, %v), want (1, true)", epID, ok)
	}
}

func TestMatchAll_MultiEndpoint(t *testing.T) {
	profile := buildProfile("dev-1",
		// Endpoint 1: INVERTER with full LPC/LPP/MPD support
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 22, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03,
				[]uint16{1, 5, 10, 11},
				nil, nil,
			),
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
		// Endpoint 2: PV_STRING with only Measurement (MPD only)
		buildEndpoint(2, "PV_STRING",
			buildFeature(0x04,
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)

	if !du.HasUseCase(LPC) {
		t.Error("expected LPC to match on endpoint 1")
	}
	if !du.HasUseCase(LPP) {
		t.Error("expected LPP to match on endpoint 1")
	}
	if !du.HasUseCase(MPD) {
		t.Error("expected MPD to match")
	}

	// Check LPC is on endpoint 1
	epID, _ := du.EndpointForUseCase(LPC)
	if epID != 1 {
		t.Errorf("LPC endpoint = %d, want 1", epID)
	}
}
