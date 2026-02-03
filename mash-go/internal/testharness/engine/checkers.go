package engine

import (
	"fmt"
)

// ToFloat64 converts various numeric types to float64 for comparison.
func ToFloat64(v interface{}) (float64, bool) {
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

	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
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

	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
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

	actualNum, ok1 := ToFloat64(actual)
	minNum, ok2 := ToFloat64(minVal)
	maxNum, ok3 := ToFloat64(maxVal)
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

	expectedSize, ok := ToFloat64(expected)
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

// CheckerSaveAs saves the current step's complete output under the given name.
// This allows later steps to compare their output against the saved snapshot
// using the value_equals checker.
func CheckerSaveAs(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	targetKey, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("save_as target must be a string, got %T", expected),
		}
	}

	output, exists := state.Get(InternalStepOutput)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  "no step output to save",
		}
	}

	state.Set(targetKey, output)
	return &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   output,
		Passed:   true,
		Message:  fmt.Sprintf("saved step output as %q", targetKey),
	}
}

// CheckerValueEquals compares the current step's output with a previously saved
// output (stored via save_as). All keys present in the saved map must match.
func CheckerValueEquals(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	savedName, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Passed:   false,
			Message:  fmt.Sprintf("value_equals target must be a string, got %T", expected),
		}
	}

	savedVal, exists := state.Get(savedName)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: savedName,
			Passed:   false,
			Message:  fmt.Sprintf("saved value %q not found", savedName),
		}
	}

	currentOutput, _ := state.Get(InternalStepOutput)

	savedMap, savedIsMap := savedVal.(map[string]interface{})
	currentMap, currentIsMap := currentOutput.(map[string]interface{})

	if savedIsMap && currentIsMap {
		var mismatches []string
		for k, sv := range savedMap {
			cv, has := currentMap[k]
			if !has || fmt.Sprintf("%v", sv) != fmt.Sprintf("%v", cv) {
				mismatches = append(mismatches, fmt.Sprintf("%s: saved=%v current=%v", k, sv, cv))
			}
		}
		passed := len(mismatches) == 0
		msg := "values match"
		if !passed {
			msg = fmt.Sprintf("mismatches: %v", mismatches)
		}
		return &ExpectResult{
			Key:      key,
			Expected: savedVal,
			Actual:   currentOutput,
			Passed:   passed,
			Message:  msg,
		}
	}

	passed := fmt.Sprintf("%v", savedVal) == fmt.Sprintf("%v", currentOutput)
	return &ExpectResult{
		Key:      key,
		Expected: savedVal,
		Actual:   currentOutput,
		Passed:   passed,
		Message:  fmt.Sprintf("saved=%v current=%v", savedVal, currentOutput),
	}
}

// CheckerIssuerFingerprintEquals compares the issuer_fingerprint output of the
// current step against a fingerprint stored in a previously saved output map.
func CheckerIssuerFingerprintEquals(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	savedName, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Passed:   false,
			Message:  fmt.Sprintf("issuer_fingerprint_equals target must be a string, got %T", expected),
		}
	}

	savedVal, exists := state.Get(savedName)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: savedName,
			Passed:   false,
			Message:  fmt.Sprintf("saved value %q not found", savedName),
		}
	}

	// Extract fingerprint from saved value (may be a map or plain string).
	expectedFP := ""
	if savedMap, ok := savedVal.(map[string]interface{}); ok {
		expectedFP, _ = savedMap[KeyFingerprint].(string)
	} else if s, ok := savedVal.(string); ok {
		expectedFP = s
	}

	actualFP, _ := state.Get(KeyIssuerFingerprint)
	actualFPStr := fmt.Sprintf("%v", actualFP)

	passed := actualFPStr == expectedFP && expectedFP != ""
	return &ExpectResult{
		Key:      key,
		Expected: expectedFP,
		Actual:   actualFP,
		Passed:   passed,
		Message:  fmt.Sprintf("issuer_fingerprint=%v expected=%v", actualFP, expectedFP),
	}
}

// RegisterEnhancedCheckers registers all enhanced checkers with the engine.
func RegisterEnhancedCheckers(e *Engine) {
	e.RegisterChecker(CheckerNameValueGreaterThan, CheckerValueGreaterThan)
	e.RegisterChecker(CheckerNameValueLessThan, CheckerValueLessThan)
	e.RegisterChecker(CheckerNameValueInRange, CheckerValueInRange)
	e.RegisterChecker(CheckerNameValueIsNull, CheckerValueIsNull)
	e.RegisterChecker(CheckerNameValueIsMap, CheckerValueIsMap)
	e.RegisterChecker(CheckerNameContains, CheckerContains)
	e.RegisterChecker(CheckerNameMapSizeEquals, CheckerMapSizeEquals)
	e.RegisterChecker(CheckerNameSaveAs, CheckerSaveAs)
	e.RegisterChecker(CheckerNameValueEquals, CheckerValueEquals)
	e.RegisterChecker(CheckerNameIssuerFingerprintEquals, CheckerIssuerFingerprintEquals)
}
