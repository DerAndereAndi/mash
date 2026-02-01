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

func TestMatchAll_GPL_ConsumptionOnly(t *testing.T) {
	// EV_CHARGER with EnergyControl + Electrical (consumption) + Measurement
	// Should match GPL BASE + CONSUMPTION + MEASUREMENT
	// (PRODUCTION should not match: EV_CHARGER is not in PRODUCTION's endpointTypes)
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05, // EnergyControl
				[]uint16{1, 2, 10, 20, 70, 72, 73, 75, 76},
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
	if !du.HasUseCase(GPL) {
		t.Error("expected GPL to match")
	}

	scenarios, ok := du.ScenariosForUseCase(GPL)
	if !ok {
		t.Fatal("expected ScenariosForUseCase(GPL) to return ok")
	}
	if !scenarios.Has(0) {
		t.Error("expected BASE scenario (bit 0) to be set")
	}
	if !scenarios.Has(1) {
		t.Error("expected CONSUMPTION scenario (bit 1) to be set")
	}
	if scenarios.Has(2) {
		t.Error("PRODUCTION scenario (bit 2) should NOT be set for EV_CHARGER")
	}
	if !scenarios.Has(3) {
		t.Error("expected MEASUREMENT scenario (bit 3) to be set")
	}
}

func TestMatchAll_GPL_BothDirections(t *testing.T) {
	// INVERTER with both consumption and production attributes
	profile := buildProfile("dev-1",
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05, // EnergyControl
				[]uint16{1, 2, 10, 20, 70, 71, 72, 73, 74, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03, // Electrical
				[]uint16{1, 5, 10, 11}, // nominalMaxConsumption + nominalMaxProduction
				nil, nil,
			),
			buildFeature(0x04, // Measurement
				[]uint16{1},
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(GPL) {
		t.Error("expected GPL to match")
	}

	scenarios, ok := du.ScenariosForUseCase(GPL)
	if !ok {
		t.Fatal("expected ScenariosForUseCase(GPL) to return ok")
	}
	if !scenarios.Has(0) {
		t.Error("expected BASE (bit 0)")
	}
	if !scenarios.Has(1) {
		t.Error("expected CONSUMPTION (bit 1)")
	}
	if !scenarios.Has(2) {
		t.Error("expected PRODUCTION (bit 2) for INVERTER")
	}
	if !scenarios.Has(3) {
		t.Error("expected MEASUREMENT (bit 3)")
	}
}

func TestMatchAll_GPL_MissingEnergyControl(t *testing.T) {
	// Electrical + Measurement but no EnergyControl -> BASE cannot match
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
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
	if du.HasUseCase(GPL) {
		t.Error("GPL should not match without EnergyControl")
	}
}

func TestMatchAll_GPL_AcceptsLimitsFalse(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 72},
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
	if du.HasUseCase(GPL) {
		t.Error("GPL should not match when acceptsLimits=false")
	}
}

func TestMatchAll_GPL_MeasurementOptional(t *testing.T) {
	// EnergyControl + Electrical present, Measurement absent
	// BASE + CONSUMPTION should match, but MEASUREMENT should not
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 72, 73, 75, 76},
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
	if !du.HasUseCase(GPL) {
		t.Error("GPL should match even without optional Measurement scenario")
	}

	scenarios, ok := du.ScenariosForUseCase(GPL)
	if !ok {
		t.Fatal("expected ScenariosForUseCase(GPL) to return ok")
	}
	if !scenarios.Has(0) {
		t.Error("expected BASE scenario to be set")
	}
	if !scenarios.Has(1) {
		t.Error("expected CONSUMPTION scenario to be set")
	}
	if scenarios.Has(3) {
		t.Error("MEASUREMENT scenario should NOT be set when Measurement is absent")
	}
}

func TestMatchAll_GPL_RequiresAny_StripsBaseWithoutDirection(t *testing.T) {
	// EnergyControl present (BASE features match) but no Electrical
	// -> Neither CONSUMPTION nor PRODUCTION can match
	// -> requiresAny strips BASE
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 72},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			// No Electrical -> CONSUMPTION can't match
		),
	)

	du := MatchAll(profile, Registry)
	if du.HasUseCase(GPL) {
		t.Error("GPL should not match when neither CONSUMPTION nor PRODUCTION is present (requiresAny)")
	}
}

func TestMatchAll_GPL_Requires_ConsumptionNeedsBase(t *testing.T) {
	// Test that if BASE features don't match, CONSUMPTION doesn't match either
	// because CONSUMPTION requires BASE
	def := &UseCaseDef{
		Name: "TEST",
		Scenarios: []ScenarioDef{
			{
				Bit: 0, Name: "BASE",
				RequiresAny: []string{"S1"},
				Features:    []FeatureRequirement{{FeatureName: "A", FeatureID: 1}},
			},
			{
				Bit: 1, Name: "S1",
				Requires: []string{"BASE"},
				Features: []FeatureRequirement{{FeatureName: "B", FeatureID: 2}},
			},
		},
	}

	// Only feature B present -> S1 matches on features, BASE does not
	// But S1 requires BASE, so S1 is stripped
	// Then BASE's requiresAny(S1) fails too
	ep := buildEndpoint(1, "TEST",
		buildFeature(2, nil, nil, nil),
	)

	result := matchEndpoint(def, ep)
	if result.Matched {
		t.Error("should not match: circular dependency means nothing survives")
	}
}

func TestMatchAll_GPL_PerScenarioEndpointTypes(t *testing.T) {
	// EV_CHARGER is NOT in PRODUCTION's endpoint types (INVERTER, BATTERY, GRID_CONNECTION)
	// Even if features match, PRODUCTION should be filtered out
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05, // EnergyControl
				[]uint16{1, 2, 10, 20, 70, 71, 72, 73, 74, 75, 76},
				[]uint8{1, 2},
				map[uint16]any{10: true},
			),
			buildFeature(0x03, // Electrical
				[]uint16{1, 5, 10, 11}, // both consumption and production attrs
				nil, nil,
			),
		),
	)

	du := MatchAll(profile, Registry)
	if !du.HasUseCase(GPL) {
		t.Error("expected GPL to match")
	}

	scenarios, _ := du.ScenariosForUseCase(GPL)
	if scenarios.Has(2) {
		t.Error("PRODUCTION should NOT match on EV_CHARGER (per-scenario endpointTypes filter)")
	}
	if !scenarios.Has(1) {
		t.Error("CONSUMPTION should match on EV_CHARGER")
	}
}

func TestMatchAll_GPL_GridConnection(t *testing.T) {
	// GRID_CONNECTION is in GPL's use-case-level endpoint types
	// and in PRODUCTION's per-scenario endpoint types
	profile := buildProfile("dev-1",
		buildEndpoint(1, "GRID_CONNECTION",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 71, 72, 73, 74, 75, 76},
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
	if !du.HasUseCase(GPL) {
		t.Error("GPL should match GRID_CONNECTION endpoint")
	}

	scenarios, _ := du.ScenariosForUseCase(GPL)
	if !scenarios.Has(2) {
		t.Error("PRODUCTION should match on GRID_CONNECTION")
	}
}

func TestMatchAll_MPD_Matched(t *testing.T) {
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
	// Build a profile that matches GPL (with CONSUMPTION) + MPD
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 72, 73, 75, 76},
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

	cmds := du.SupportedCommands()
	for _, cmd := range []string{"limit", "clear", "capacity", "override", "failsafe"} {
		if !cmds[cmd] {
			t.Errorf("expected command %q in union", cmd)
		}
	}
}

func TestEndpointForUseCase_FromMatch(t *testing.T) {
	profile := buildProfile("dev-1",
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 71, 72, 73, 74, 75, 76},
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

	epID, ok := du.EndpointForUseCase(GPL)
	if !ok || epID != 1 {
		t.Errorf("EndpointForUseCase(GPL) = (%d, %v), want (1, true)", epID, ok)
	}

	epID, ok = du.EndpointForUseCase(MPD)
	if !ok || epID != 1 {
		t.Errorf("EndpointForUseCase(MPD) = (%d, %v), want (1, true)", epID, ok)
	}
}

func TestMatchAll_MultiEndpoint(t *testing.T) {
	profile := buildProfile("dev-1",
		// Endpoint 1: INVERTER with full GPL/MPD support
		buildEndpoint(1, "INVERTER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 71, 72, 73, 74, 75, 76},
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

	if !du.HasUseCase(GPL) {
		t.Error("expected GPL to match on endpoint 1")
	}
	if !du.HasUseCase(MPD) {
		t.Error("expected MPD to match")
	}

	// Check GPL is on endpoint 1
	epID, _ := du.EndpointForUseCase(GPL)
	if epID != 1 {
		t.Errorf("GPL endpoint = %d, want 1", epID)
	}
}

func TestMatchAll_ScenarioBitmap(t *testing.T) {
	// Build a profile that matches BASE + CONSUMPTION + MEASUREMENT for GPL
	// but not OVERRIDE (since we don't set override-related attributes)
	profile := buildProfile("dev-1",
		buildEndpoint(1, "EV_CHARGER",
			buildFeature(0x05,
				[]uint16{1, 2, 10, 20, 70, 72, 73},
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
	scenarios, ok := du.ScenariosForUseCase(GPL)
	if !ok {
		t.Fatal("expected GPL to match")
	}

	if !scenarios.Has(0) {
		t.Error("expected BASE (bit 0) to be set")
	}
}

func TestEnforceScenarioConstraints_RequiresAny(t *testing.T) {
	def := &UseCaseDef{
		Scenarios: []ScenarioDef{
			{Bit: 0, Name: "BASE", RequiresAny: []string{"A", "B"}},
			{Bit: 1, Name: "A"},
			{Bit: 2, Name: "B"},
		},
	}

	// BASE + A -> both survive
	result := enforceScenarioConstraints(def, 0x03)
	if result != 0x03 {
		t.Errorf("BASE+A: got 0x%02X, want 0x03", result)
	}

	// BASE + B -> both survive
	result = enforceScenarioConstraints(def, 0x05)
	if result != 0x05 {
		t.Errorf("BASE+B: got 0x%02X, want 0x05", result)
	}

	// BASE only -> BASE stripped (no A or B)
	result = enforceScenarioConstraints(def, 0x01)
	if result != 0x00 {
		t.Errorf("BASE alone: got 0x%02X, want 0x00", result)
	}
}

func TestEnforceScenarioConstraints_Requires(t *testing.T) {
	def := &UseCaseDef{
		Scenarios: []ScenarioDef{
			{Bit: 0, Name: "BASE"},
			{Bit: 1, Name: "S1", Requires: []string{"BASE"}},
			{Bit: 2, Name: "S2", Requires: []string{"BASE", "S1"}},
		},
	}

	// All present -> all survive
	result := enforceScenarioConstraints(def, 0x07)
	if result != 0x07 {
		t.Errorf("all: got 0x%02X, want 0x07", result)
	}

	// S2 without S1 -> S2 stripped
	result = enforceScenarioConstraints(def, 0x05)
	if result != 0x01 {
		t.Errorf("BASE+S2: got 0x%02X, want 0x01", result)
	}

	// S1 without BASE -> S1 stripped
	result = enforceScenarioConstraints(def, 0x02)
	if result != 0x00 {
		t.Errorf("S1 alone: got 0x%02X, want 0x00", result)
	}
}
