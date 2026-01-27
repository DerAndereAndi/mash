package engine

import (
	"fmt"
)

// toFloat64 converts various numeric types to float64 for comparison.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint32:
		return float64(n), true
	default:
		return 0, false
	}
}

// CheckerValueGreaterThan checks if the actual value is greater than expected.
func CheckerValueGreaterThan(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	actualNum, ok1 := toFloat64(actual)
	expectedNum, ok2 := toFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}

	passed := actualNum > expectedNum
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("%v > %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerValueLessThan checks if the actual value is less than expected.
func CheckerValueLessThan(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	actualNum, ok1 := toFloat64(actual)
	expectedNum, ok2 := toFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}

	passed := actualNum < expectedNum
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("%v < %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerValueInRange checks if the actual value is within a range [min, max].
// Expected should be a map with "min" and "max" keys.
func CheckerValueInRange(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	// Expected should be a map with min and max
	rangeMap, ok := expected.(map[string]interface{})
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  "expected must be a map with 'min' and 'max' keys",
		}
	}

	minVal, hasMin := rangeMap["min"]
	maxVal, hasMax := rangeMap["max"]
	if !hasMin || !hasMax {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  "expected must have both 'min' and 'max' keys",
		}
	}

	actualNum, ok1 := toFloat64(actual)
	minNum, ok2 := toFloat64(minVal)
	maxNum, ok3 := toFloat64(maxVal)
	if !ok1 || !ok2 || !ok3 {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  "cannot compare non-numeric values",
		}
	}

	passed := actualNum >= minNum && actualNum <= maxNum
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("%v in [%v, %v] = %v", actualNum, minNum, maxNum, passed),
	}
}

// CheckerValueIsNull checks if the actual value is nil/null.
func CheckerValueIsNull(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)

	expectNull, _ := expected.(bool)
	if !expectNull {
		// Expecting not null
		isNull := !exists || actual == nil
		passed := !isNull
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   passed,
			Message:  fmt.Sprintf("value is null = %v (expected not null)", isNull),
		}
	}

	// Expecting null
	isNull := !exists || actual == nil
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   isNull,
		Message:  fmt.Sprintf("value is null = %v", isNull),
	}
}

// CheckerValueIsMap checks if the actual value is a map.
func CheckerValueIsMap(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	_, isMap := actual.(map[string]interface{})
	expectMap, _ := expected.(bool)

	passed := isMap == expectMap
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("value is map = %v (expected %v)", isMap, expectMap),
	}
}

// CheckerContains checks if an array contains a value or a map contains a key.
func CheckerContains(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	// Check if it's an array
	if arr, ok := actual.([]interface{}); ok {
		for _, item := range arr {
			if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", expected) {
				return &ExpectResult{
					Key:      key,
					Expected: expected,
					Actual:   actual,
					Passed:   true,
					Message:  fmt.Sprintf("array contains %v", expected),
				}
			}
		}
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("array does not contain %v", expected),
		}
	}

	// Check if it's a map
	if m, ok := actual.(map[string]interface{}); ok {
		keyStr := fmt.Sprintf("%v", expected)
		_, hasKey := m[keyStr]
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   hasKey,
			Message:  fmt.Sprintf("map contains key %q = %v", keyStr, hasKey),
		}
	}

	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   false,
		Message:  fmt.Sprintf("value is neither array nor map: %T", actual),
	}
}

// CheckerMapSizeEquals checks if the size of a map or array equals expected.
func CheckerMapSizeEquals(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	expectedSize, ok := toFloat64(expected)
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("expected size must be numeric, got %T", expected),
		}
	}

	var actualSize int

	// Check if it's a map
	if m, ok := actual.(map[string]interface{}); ok {
		actualSize = len(m)
	} else if arr, ok := actual.([]interface{}); ok {
		actualSize = len(arr)
	} else {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("value is neither map nor array: %T", actual),
		}
	}

	passed := float64(actualSize) == expectedSize
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("size = %d (expected %v)", actualSize, expectedSize),
	}
}

// CheckerSaveAs copies a value to another key (always passes).
// This is a side-effect checker that stores the actual value under the expected key name.
func CheckerSaveAs(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found, cannot save", key),
		}
	}

	targetKey, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("save_as target must be a string, got %T", expected),
		}
	}

	state.Set(targetKey, actual)
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   true,
		Message:  fmt.Sprintf("saved %q to %q", key, targetKey),
	}
}

// RegisterEnhancedCheckers registers all enhanced checkers with the engine.
func RegisterEnhancedCheckers(e *Engine) {
	e.RegisterChecker("value_greater_than", CheckerValueGreaterThan)
	e.RegisterChecker("value_less_than", CheckerValueLessThan)
	e.RegisterChecker("value_in_range", CheckerValueInRange)
	e.RegisterChecker("value_is_null", CheckerValueIsNull)
	e.RegisterChecker("value_is_map", CheckerValueIsMap)
	e.RegisterChecker("contains", CheckerContains)
	e.RegisterChecker("map_size_equals", CheckerMapSizeEquals)
	e.RegisterChecker("save_as", CheckerSaveAs)
}
