package engine

import (
	"context"
	"testing"
)

// TestChecker_ValueGreaterThan tests the value_greater_than checker.
func TestChecker_ValueGreaterThan(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("count", float64(10))

	tests := []struct {
		expected interface{}
		passed   bool
	}{
		{float64(5), true},   // 10 > 5
		{float64(10), false}, // 10 > 10 (not strictly greater)
		{float64(15), false}, // 10 > 15
	}

	for _, tt := range tests {
		result := CheckerValueGreaterThan("count", tt.expected, state)
		if result.Passed != tt.passed {
			t.Errorf("ValueGreaterThan(10, %v) = %v, want %v", tt.expected, result.Passed, tt.passed)
		}
	}
}

// TestChecker_ValueLessThan tests the value_less_than checker.
func TestChecker_ValueLessThan(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("count", float64(10))

	tests := []struct {
		expected interface{}
		passed   bool
	}{
		{float64(15), true},  // 10 < 15
		{float64(10), false}, // 10 < 10 (not strictly less)
		{float64(5), false},  // 10 < 5
	}

	for _, tt := range tests {
		result := CheckerValueLessThan("count", tt.expected, state)
		if result.Passed != tt.passed {
			t.Errorf("ValueLessThan(10, %v) = %v, want %v", tt.expected, result.Passed, tt.passed)
		}
	}
}

// TestChecker_ValueInRange tests the value_in_range checker.
func TestChecker_ValueInRange(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("value", float64(50))

	tests := []struct {
		expected interface{}
		passed   bool
	}{
		{map[string]interface{}{"min": float64(0), "max": float64(100)}, true},   // 50 in [0, 100]
		{map[string]interface{}{"min": float64(50), "max": float64(100)}, true},  // 50 in [50, 100] (inclusive)
		{map[string]interface{}{"min": float64(0), "max": float64(50)}, true},    // 50 in [0, 50] (inclusive)
		{map[string]interface{}{"min": float64(60), "max": float64(100)}, false}, // 50 not in [60, 100]
		{map[string]interface{}{"min": float64(0), "max": float64(40)}, false},   // 50 not in [0, 40]
	}

	for _, tt := range tests {
		result := CheckerValueInRange("value", tt.expected, state)
		if result.Passed != tt.passed {
			t.Errorf("ValueInRange(50, %v) = %v, want %v", tt.expected, result.Passed, tt.passed)
		}
	}
}

// TestChecker_ValueIsNull tests the value_is_null checker.
func TestChecker_ValueIsNull(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("nil_value", nil)
	state.Set("non_nil_value", "something")

	// Test nil value
	result := CheckerValueIsNull("nil_value", true, state)
	if !result.Passed {
		t.Error("ValueIsNull(nil) should pass")
	}

	// Test non-nil value expecting null
	result = CheckerValueIsNull("non_nil_value", true, state)
	if result.Passed {
		t.Error("ValueIsNull(something) expecting true should fail")
	}

	// Test non-nil value expecting not null
	result = CheckerValueIsNull("non_nil_value", false, state)
	if !result.Passed {
		t.Error("ValueIsNull(something) expecting false should pass")
	}

	// Test missing key (treated as null)
	result = CheckerValueIsNull("missing_key", true, state)
	if !result.Passed {
		t.Error("ValueIsNull(missing) expecting true should pass")
	}
}

// TestChecker_ValueIsMap tests the value_is_map checker.
func TestChecker_ValueIsMap(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("map_value", map[string]interface{}{"key": "value"})
	state.Set("string_value", "not a map")
	state.Set("array_value", []interface{}{1, 2, 3})

	// Test map value
	result := CheckerValueIsMap("map_value", true, state)
	if !result.Passed {
		t.Error("ValueIsMap(map) should pass")
	}

	// Test non-map value
	result = CheckerValueIsMap("string_value", true, state)
	if result.Passed {
		t.Error("ValueIsMap(string) should fail")
	}

	// Test array value (not a map)
	result = CheckerValueIsMap("array_value", true, state)
	if result.Passed {
		t.Error("ValueIsMap(array) should fail")
	}

	// Test expecting not a map
	result = CheckerValueIsMap("string_value", false, state)
	if !result.Passed {
		t.Error("ValueIsMap(string) expecting false should pass")
	}
}

// TestChecker_Contains tests the contains checker (for arrays/maps).
func TestChecker_Contains(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("array", []interface{}{"apple", "banana", "cherry"})
	state.Set("map", map[string]interface{}{"name": "test", "count": 42})

	// Test array contains
	result := CheckerContains("array", "banana", state)
	if !result.Passed {
		t.Error("Contains(array, banana) should pass")
	}

	result = CheckerContains("array", "grape", state)
	if result.Passed {
		t.Error("Contains(array, grape) should fail")
	}

	// Test map contains key
	result = CheckerContains("map", "name", state)
	if !result.Passed {
		t.Error("Contains(map, name) should pass")
	}

	result = CheckerContains("map", "missing", state)
	if result.Passed {
		t.Error("Contains(map, missing) should fail")
	}
}

// TestChecker_MapSizeEquals tests the map_size_equals checker.
func TestChecker_MapSizeEquals(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("map2", map[string]interface{}{"a": 1, "b": 2})
	state.Set("map0", map[string]interface{}{})
	state.Set("array3", []interface{}{1, 2, 3})

	// Test map size equals
	result := CheckerMapSizeEquals("map2", float64(2), state)
	if !result.Passed {
		t.Errorf("MapSizeEquals(map2, 2) = %v, want true. Message: %s", result.Passed, result.Message)
	}

	result = CheckerMapSizeEquals("map2", float64(3), state)
	if result.Passed {
		t.Error("MapSizeEquals(map2, 3) should fail")
	}

	// Test empty map
	result = CheckerMapSizeEquals("map0", float64(0), state)
	if !result.Passed {
		t.Error("MapSizeEquals(map0, 0) should pass")
	}

	// Test array size
	result = CheckerMapSizeEquals("array3", float64(3), state)
	if !result.Passed {
		t.Error("MapSizeEquals(array3, 3) should pass (works for arrays too)")
	}
}

// TestChecker_SaveAs tests the save_as checker (stores value for later use).
func TestChecker_SaveAs(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("original", "test-value")

	// SaveAs should copy value to new key
	result := CheckerSaveAs("original", "copied", state)
	if !result.Passed {
		t.Errorf("SaveAs(original, copied) failed: %s", result.Message)
	}

	// Check that value was copied
	copied, exists := state.Get("copied")
	if !exists {
		t.Error("SaveAs did not create 'copied' key")
	}
	if copied != "test-value" {
		t.Errorf("copied = %v, want 'test-value'", copied)
	}
}

// TestChecker_NotFound tests checker behavior when key is not found.
func TestChecker_NotFound(t *testing.T) {
	state := NewExecutionState(context.Background())

	// All checkers should fail gracefully when key not found
	checkers := []struct {
		name   string
		fn     ExpectChecker
		expect interface{}
	}{
		{"value_greater_than", CheckerValueGreaterThan, float64(5)},
		{"value_less_than", CheckerValueLessThan, float64(5)},
		{"value_in_range", CheckerValueInRange, map[string]interface{}{"min": 0, "max": 10}},
		{"value_is_map", CheckerValueIsMap, true},
		{"contains", CheckerContains, "value"},
		{"map_size_equals", CheckerMapSizeEquals, float64(1)},
	}

	for _, tc := range checkers {
		result := tc.fn("nonexistent", tc.expect, state)
		if result.Passed {
			t.Errorf("%s should fail for nonexistent key", tc.name)
		}
	}
}

// TestChecker_TypeConversion tests numeric type handling.
func TestChecker_TypeConversion(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("int_value", int(10))   // Set as int
	state.Set("float_value", 10.5)    // Set as float64

	// Should work with both int and float64
	result := CheckerValueGreaterThan("int_value", float64(5), state)
	if !result.Passed {
		t.Error("Should handle int value with float64 comparison")
	}

	result = CheckerValueGreaterThan("float_value", float64(5), state)
	if !result.Passed {
		t.Error("Should handle float64 value")
	}
}
