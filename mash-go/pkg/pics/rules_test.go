package pics

import (
	"testing"
)

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityInfo, "info"},
		{Severity(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.expected {
				t.Errorf("Severity.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestViolation_String(t *testing.T) {
	tests := []struct {
		name     string
		v        Violation
		contains []string
	}{
		{
			name: "basic violation",
			v: Violation{
				RuleID:   "TEST-001",
				Severity: SeverityError,
				Message:  "test message",
			},
			contains: []string{"TEST-001", "error", "test message"},
		},
		{
			name: "with PICS codes",
			v: Violation{
				RuleID:    "TEST-002",
				Severity:  SeverityWarning,
				Message:   "missing dependency",
				PICSCodes: []string{"MASH.S.CTRL.F0A", "MASH.S.CTRL.F03"},
			},
			contains: []string{"TEST-002", "warning", "MASH.S.CTRL.F0A", "MASH.S.CTRL.F03"},
		},
		{
			name: "with line numbers",
			v: Violation{
				RuleID:      "TEST-003",
				Severity:    SeverityError,
				Message:     "invalid value",
				LineNumbers: []int{10, 15},
			},
			contains: []string{"TEST-003", "10", "15"},
		},
		{
			name: "with suggestion",
			v: Violation{
				RuleID:     "TEST-004",
				Severity:   SeverityInfo,
				Message:    "consider adding version",
				Suggestion: "Add MASH.S.VERSION=1",
			},
			contains: []string{"TEST-004", "info", "Add MASH.S.VERSION=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.v.String()
			for _, substr := range tt.contains {
				if !containsString(s, substr) {
					t.Errorf("Violation.String() = %q, expected to contain %q", s, substr)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name       string
		violations []Violation
		expected   bool
	}{
		{
			name:       "empty",
			violations: nil,
			expected:   false,
		},
		{
			name: "only warnings",
			violations: []Violation{
				{Severity: SeverityWarning},
				{Severity: SeverityInfo},
			},
			expected: false,
		},
		{
			name: "has error",
			violations: []Violation{
				{Severity: SeverityWarning},
				{Severity: SeverityError},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasErrors(tt.violations); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFilterBySeverity(t *testing.T) {
	violations := []Violation{
		{RuleID: "1", Severity: SeverityError},
		{RuleID: "2", Severity: SeverityWarning},
		{RuleID: "3", Severity: SeverityInfo},
	}

	tests := []struct {
		name        string
		minSeverity Severity
		expectedIDs []string
	}{
		{
			name:        "errors only",
			minSeverity: SeverityError,
			expectedIDs: []string{"1"},
		},
		{
			name:        "errors and warnings",
			minSeverity: SeverityWarning,
			expectedIDs: []string{"1", "2"},
		},
		{
			name:        "all",
			minSeverity: SeverityInfo,
			expectedIDs: []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterBySeverity(violations, tt.minSeverity)
			if len(filtered) != len(tt.expectedIDs) {
				t.Errorf("FilterBySeverity() returned %d violations, want %d", len(filtered), len(tt.expectedIDs))
				return
			}
			for i, v := range filtered {
				if v.RuleID != tt.expectedIDs[i] {
					t.Errorf("FilterBySeverity()[%d].RuleID = %s, want %s", i, v.RuleID, tt.expectedIDs[i])
				}
			}
		})
	}
}

func TestBaseRule(t *testing.T) {
	rule := NewBaseRule("TEST-001", "Test Rule", "test", SeverityWarning)

	if rule.ID() != "TEST-001" {
		t.Errorf("ID() = %s, want TEST-001", rule.ID())
	}
	if rule.Name() != "Test Rule" {
		t.Errorf("Name() = %s, want Test Rule", rule.Name())
	}
	if rule.Category() != "test" {
		t.Errorf("Category() = %s, want test", rule.Category())
	}
	if rule.DefaultSeverity() != SeverityWarning {
		t.Errorf("DefaultSeverity() = %v, want SeverityWarning", rule.DefaultSeverity())
	}
}

// testRule is a simple rule implementation for testing
type testRule struct {
	*BaseRule
	checkFunc func(*PICS) []Violation
}

func (r *testRule) Check(pics *PICS) []Violation {
	if r.checkFunc != nil {
		return r.checkFunc(pics)
	}
	return nil
}

func TestRuleRegistry_Register(t *testing.T) {
	registry := NewRuleRegistry()

	rule := &testRule{
		BaseRule: NewBaseRule("TEST-001", "Test", "test", SeverityError),
	}

	registry.Register(rule)

	if registry.Count() != 1 {
		t.Errorf("Count() = %d, want 1", registry.Count())
	}

	if !registry.IsEnabled("TEST-001") {
		t.Error("expected TEST-001 to be enabled by default")
	}

	if registry.GetRule("TEST-001") != rule {
		t.Error("GetRule() returned wrong rule")
	}
}

func TestRuleRegistry_EnableDisable(t *testing.T) {
	registry := NewRuleRegistry()
	rule := &testRule{
		BaseRule: NewBaseRule("TEST-001", "Test", "test", SeverityError),
	}
	registry.Register(rule)

	// Disable
	registry.Disable("TEST-001")
	if registry.IsEnabled("TEST-001") {
		t.Error("expected TEST-001 to be disabled")
	}
	if registry.EnabledCount() != 0 {
		t.Errorf("EnabledCount() = %d, want 0", registry.EnabledCount())
	}

	// Enable
	registry.Enable("TEST-001")
	if !registry.IsEnabled("TEST-001") {
		t.Error("expected TEST-001 to be enabled")
	}
	if registry.EnabledCount() != 1 {
		t.Errorf("EnabledCount() = %d, want 1", registry.EnabledCount())
	}
}

func TestRuleRegistry_SetSeverity(t *testing.T) {
	registry := NewRuleRegistry()
	rule := &testRule{
		BaseRule: NewBaseRule("TEST-001", "Test", "test", SeverityError),
	}
	registry.Register(rule)

	// Default severity
	if registry.GetSeverity("TEST-001") != SeverityError {
		t.Errorf("GetSeverity() = %v, want SeverityError", registry.GetSeverity("TEST-001"))
	}

	// Override severity
	registry.SetSeverity("TEST-001", SeverityWarning)
	if registry.GetSeverity("TEST-001") != SeverityWarning {
		t.Errorf("GetSeverity() = %v, want SeverityWarning", registry.GetSeverity("TEST-001"))
	}
}

func TestRuleRegistry_Categories(t *testing.T) {
	registry := NewRuleRegistry()
	registry.Register(&testRule{BaseRule: NewBaseRule("DEP-001", "Dep 1", "dependency", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("DEP-002", "Dep 2", "dependency", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("MAN-001", "Man 1", "mandatory", SeverityError)})

	categories := registry.Categories()
	if len(categories) != 2 {
		t.Errorf("Categories() returned %d categories, want 2", len(categories))
	}

	depRules := registry.RulesByCategory("dependency")
	if len(depRules) != 2 {
		t.Errorf("RulesByCategory(dependency) returned %d rules, want 2", len(depRules))
	}
}

func TestRuleRegistry_RunRules(t *testing.T) {
	registry := NewRuleRegistry()

	// Add a rule that always returns a violation
	rule := &testRule{
		BaseRule: NewBaseRule("TEST-001", "Test", "test", SeverityError),
		checkFunc: func(pics *PICS) []Violation {
			return []Violation{
				{RuleID: "TEST-001", Severity: SeverityError, Message: "test violation"},
			}
		},
	}
	registry.Register(rule)

	pics := NewPICS()
	violations := registry.RunRules(pics)

	if len(violations) != 1 {
		t.Errorf("RunRules() returned %d violations, want 1", len(violations))
	}
	if violations[0].RuleID != "TEST-001" {
		t.Errorf("violation.RuleID = %s, want TEST-001", violations[0].RuleID)
	}
}

func TestRuleRegistry_EnableDisableCategory(t *testing.T) {
	registry := NewRuleRegistry()
	registry.Register(&testRule{BaseRule: NewBaseRule("DEP-001", "Dep 1", "dependency", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("DEP-002", "Dep 2", "dependency", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("MAN-001", "Man 1", "mandatory", SeverityError)})

	// Disable dependency category
	registry.DisableCategory("dependency")
	if registry.IsEnabled("DEP-001") || registry.IsEnabled("DEP-002") {
		t.Error("dependency rules should be disabled")
	}
	if !registry.IsEnabled("MAN-001") {
		t.Error("mandatory rule should still be enabled")
	}

	// Enable dependency category
	registry.EnableCategory("dependency")
	if !registry.IsEnabled("DEP-001") || !registry.IsEnabled("DEP-002") {
		t.Error("dependency rules should be enabled")
	}
}

func TestRuleRegistry_EnableDisableAll(t *testing.T) {
	registry := NewRuleRegistry()
	registry.Register(&testRule{BaseRule: NewBaseRule("TEST-001", "Test 1", "test", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("TEST-002", "Test 2", "test", SeverityError)})

	// Disable all
	registry.DisableAll()
	if registry.EnabledCount() != 0 {
		t.Errorf("EnabledCount() after DisableAll() = %d, want 0", registry.EnabledCount())
	}

	// Enable all
	registry.EnableAll()
	if registry.EnabledCount() != 2 {
		t.Errorf("EnabledCount() after EnableAll() = %d, want 2", registry.EnabledCount())
	}
}

func TestRuleRegistry_AllRules(t *testing.T) {
	registry := NewRuleRegistry()
	registry.Register(&testRule{BaseRule: NewBaseRule("TEST-001", "Test 1", "test", SeverityError)})
	registry.Register(&testRule{BaseRule: NewBaseRule("TEST-002", "Test 2", "test", SeverityError)})
	registry.Disable("TEST-002")

	all := registry.AllRules()
	if len(all) != 2 {
		t.Errorf("AllRules() returned %d rules, want 2", len(all))
	}

	enabled := registry.EnabledRules()
	if len(enabled) != 1 {
		t.Errorf("EnabledRules() returned %d rules, want 1", len(enabled))
	}
}
