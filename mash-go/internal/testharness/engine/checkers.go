package engine

import (
	"fmt"
	"time"
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

// CheckerValueGT checks if the "value" output is strictly greater than expected.
// Used in YAML as: value_gt: 0
func CheckerValueGT(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}
	passed := actualNum > expectedNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v > %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerValueGTE checks if the "value" output is >= expected.
// Used in YAML as: value_gte: 1
func CheckerValueGTE(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}
	passed := actualNum >= expectedNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v >= %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerValueMax checks if the "value" output is <= expected (upper bound).
// Used in YAML as: value_max: 5000000
func CheckerValueMax(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}
	passed := actualNum <= expectedNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v <= %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerResponseContains checks if the "result" output map contains the
// expected key(s). Expected can be a string or a list of strings.
// Used in YAML as: response_contains: ["applied"] or response_contains: applied
func CheckerResponseContains(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	result, exists := state.Get("result")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "result"),
		}
	}

	// Collect expected keys from string or list.
	var expectedKeys []string
	switch v := expected.(type) {
	case string:
		expectedKeys = []string{v}
	case []interface{}:
		for _, item := range v {
			expectedKeys = append(expectedKeys, fmt.Sprintf("%v", item))
		}
	default:
		expectedKeys = []string{fmt.Sprintf("%v", expected)}
	}

	// Check against the result map (may have string or any keys).
	var missing []string
	for _, ek := range expectedKeys {
		found := false
		switch m := result.(type) {
		case map[string]any:
			_, found = m[ek]
		case map[any]any:
			for mk := range m {
				if fmt.Sprintf("%v", mk) == ek {
					found = true
					break
				}
			}
		}
		if !found {
			missing = append(missing, ek)
		}
	}

	passed := len(missing) == 0
	msg := fmt.Sprintf("result contains %v", expectedKeys)
	if !passed {
		msg = fmt.Sprintf("result missing keys: %v", missing)
	}
	return &ExpectResult{
		Key: key, Expected: expected, Actual: result, Passed: passed,
		Message: msg,
	}
}

// CheckerValueGTESaved checks if the "value" output is >= a previously saved value.
// Expected is the name under which the reference value was saved.
// Used in YAML as: value_gte_saved: energy_first
func CheckerValueGTESaved(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	savedName, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key: key, Expected: expected, Passed: false,
			Message: fmt.Sprintf("value_gte_saved target must be a string, got %T", expected),
		}
	}
	savedVal, exists := state.Get(savedName)
	if !exists {
		return &ExpectResult{
			Key: key, Expected: savedName, Passed: false,
			Message: fmt.Sprintf("saved value %q not found", savedName),
		}
	}
	actual, actualExists := state.Get("value")
	if !actualExists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	// The saved value may be a full step output map; extract "value" if so.
	ref := savedVal
	if savedMap, isMap := savedVal.(map[string]interface{}); isMap {
		if v, has := savedMap["value"]; has {
			ref = v
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	refNum, ok2 := ToFloat64(ref)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, ref),
		}
	}
	passed := actualNum >= refNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v >= saved(%s) %v = %v", actualNum, savedName, refNum, passed),
	}
}

// CheckerValueMaxRef checks if the "value" output is <= a previously saved value.
// Expected is the name under which the reference value was saved.
// Used in YAML as: value_max_ref: ev_max_discharge
func CheckerValueMaxRef(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	savedName, ok := expected.(string)
	if !ok {
		return &ExpectResult{
			Key: key, Expected: expected, Passed: false,
			Message: fmt.Sprintf("value_max_ref target must be a string, got %T", expected),
		}
	}
	savedVal, exists := state.Get(savedName)
	if !exists {
		return &ExpectResult{
			Key: key, Expected: savedName, Passed: false,
			Message: fmt.Sprintf("saved value %q not found", savedName),
		}
	}
	actual, actualExists := state.Get("value")
	if !actualExists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	ref := savedVal
	if savedMap, isMap := savedVal.(map[string]interface{}); isMap {
		if v, has := savedMap["value"]; has {
			ref = v
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	refNum, ok2 := ToFloat64(ref)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, ref),
		}
	}
	passed := actualNum <= refNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v <= saved(%s) %v = %v", actualNum, savedName, refNum, passed),
	}
}

// CheckerValueLTE checks if the "value" output is <= expected.
// Used in YAML as: value_lte: 100
func CheckerValueLTE(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualNum, ok1 := ToFloat64(actual)
	expectedNum, ok2 := ToFloat64(expected)
	if !ok1 || !ok2 {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot compare non-numeric values: %T and %T", actual, expected),
		}
	}
	passed := actualNum <= expectedNum
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v <= %v = %v", actualNum, expectedNum, passed),
	}
}

// CheckerValueNot checks if the "value" output is not equal to expected.
// Comparison uses fmt.Sprintf for type-agnostic equality.
// Used in YAML as: value_not: CHARGING
func CheckerValueNot(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	passed := fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v != %v = %v", actual, expected, passed),
	}
}

// CheckerValueIn checks if the "value" output is one of the expected values.
// Expected should be an array of valid values.
// Used in YAML as: value_in: [1, 2, 3]
func CheckerValueIn(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualStr := fmt.Sprintf("%v", actual)
	switch vals := expected.(type) {
	case []interface{}:
		for _, v := range vals {
			if fmt.Sprintf("%v", v) == actualStr {
				return &ExpectResult{
					Key: key, Expected: expected, Actual: actual, Passed: true,
					Message: fmt.Sprintf("value %v in %v", actual, expected),
				}
			}
		}
	}
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: false,
		Message: fmt.Sprintf("value %v not in %v", actual, expected),
	}
}

// CheckerValueNonNegative checks if the "value" output is >= 0.
// Used in YAML as: value_non_negative: true
func CheckerValueNonNegative(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualNum, ok := ToFloat64(actual)
	if !ok {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: false,
			Message: fmt.Sprintf("cannot convert to numeric: %T", actual),
		}
	}
	passed := actualNum >= 0
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %v >= 0 = %v", actualNum, passed),
	}
}

// CheckerValueIsArray checks if the "value" output is an array/slice.
// Used in YAML as: value_is_array: true
func CheckerValueIsArray(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	_, isArray := actual.([]interface{})
	expectArray, _ := expected.(bool)
	passed := isArray == expectArray
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value is array = %v (expected %v)", isArray, expectArray),
	}
}

// CheckerValueIsNotNull checks if the "value" output is not nil.
// Used in YAML as: value_is_not_null: true
func CheckerValueIsNotNull(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	isNull := !exists || actual == nil
	passed := !isNull
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value is not null = %v", passed),
	}
}

// CheckerValueIsRecent checks if the "value" output is a recent timestamp
// (within a configurable window, default 30 seconds).
// Used in YAML as: value_is_recent: true
func CheckerValueIsRecent(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	// Accept numeric (unix seconds or millis) timestamps.
	if num, ok := ToFloat64(actual); ok {
		now := float64(time.Now().Unix())
		// If the value looks like milliseconds (> year 2000 in seconds), convert.
		if num > 1e12 {
			num = num / 1000
		}
		diff := now - num
		if diff < 0 {
			diff = -diff
		}
		passed := diff < 30
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: passed,
			Message: fmt.Sprintf("timestamp age %.0fs (threshold 30s)", diff),
		}
	}
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: false,
		Message: fmt.Sprintf("value is not a numeric timestamp: %T", actual),
	}
}

// CheckerValueTreatedAsUnknown checks that the "value" output is treated as
// unknown by the device (nil, 0, or "unknown"). Domain-specific check.
// Used in YAML as: value_treated_as_unknown: true
func CheckerValueTreatedAsUnknown(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists || actual == nil {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: actual, Passed: true,
			Message: "value is nil (treated as unknown)",
		}
	}
	actualStr := fmt.Sprintf("%v", actual)
	passed := actualStr == "0" || actualStr == "" || actualStr == "unknown" || actualStr == "<nil>"
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("value %q treated as unknown = %v", actualStr, passed),
	}
}

// CheckerValueType checks the Go type name of the "value" output.
// Used in YAML as: value_type: string
func CheckerValueType(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get("value")
	if !exists {
		return &ExpectResult{
			Key: key, Expected: expected, Actual: nil, Passed: false,
			Message: fmt.Sprintf("output key %q not found", "value"),
		}
	}
	actualType := fmt.Sprintf("%T", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	passed := actualType == expectedStr
	return &ExpectResult{
		Key: key, Expected: expected, Actual: actual, Passed: passed,
		Message: fmt.Sprintf("type = %s (expected %s)", actualType, expectedStr),
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

	// Short-form checkers that operate on the "value" output key.
	e.RegisterChecker(CheckerNameValueGT, CheckerValueGT)
	e.RegisterChecker(CheckerNameValueGTE, CheckerValueGTE)
	e.RegisterChecker(CheckerNameValueMax, CheckerValueMax)
	e.RegisterChecker(CheckerNameValueLTE, CheckerValueLTE)
	e.RegisterChecker(CheckerNameValueNot, CheckerValueNot)
	e.RegisterChecker(CheckerNameValueNotEqual, CheckerValueNot)           // alias
	e.RegisterChecker(CheckerNameValueDifferentFrom, CheckerValueNot)      // alias
	e.RegisterChecker(CheckerNameValueAtLeast, CheckerValueGTE)            // alias
	e.RegisterChecker(CheckerNameValueGreaterOrEqual, CheckerValueGTE)     // alias
	e.RegisterChecker(CheckerNameValueIn, CheckerValueIn)
	e.RegisterChecker(CheckerNameValueNonNegative, CheckerValueNonNegative)
	e.RegisterChecker(CheckerNameValueIsArray, CheckerValueIsArray)
	e.RegisterChecker(CheckerNameValueIsNotNull, CheckerValueIsNotNull)
	e.RegisterChecker(CheckerNameValueIsRecent, CheckerValueIsRecent)
	e.RegisterChecker(CheckerNameValueTreatedAsUnknown, CheckerValueTreatedAsUnknown)
	e.RegisterChecker(CheckerNameValueType, CheckerValueType)
	e.RegisterChecker(CheckerNameResponseContains, CheckerResponseContains)
	e.RegisterChecker(CheckerNameValueGTESaved, CheckerValueGTESaved)
	e.RegisterChecker(CheckerNameValueMaxRef, CheckerValueMaxRef)
}
