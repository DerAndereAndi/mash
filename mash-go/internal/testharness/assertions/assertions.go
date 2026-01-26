// Package assertions provides test assertion helpers for the MASH test harness.
package assertions

import (
	"fmt"
	"reflect"
	"time"
)

// Result represents the outcome of an assertion.
type Result struct {
	// Passed indicates if the assertion passed.
	Passed bool

	// Message describes the assertion result.
	Message string

	// Expected is the expected value (for error messages).
	Expected interface{}

	// Actual is the actual value (for error messages).
	Actual interface{}
}

// Pass creates a passing result.
func Pass(message string) *Result {
	return &Result{Passed: true, Message: message}
}

// Fail creates a failing result.
func Fail(message string, expected, actual interface{}) *Result {
	return &Result{
		Passed:   false,
		Message:  message,
		Expected: expected,
		Actual:   actual,
	}
}

// Equal asserts that two values are equal.
func Equal(expected, actual interface{}) *Result {
	if reflect.DeepEqual(expected, actual) {
		return Pass(fmt.Sprintf("values are equal: %v", expected))
	}
	return Fail("values are not equal", expected, actual)
}

// NotEqual asserts that two values are not equal.
func NotEqual(expected, actual interface{}) *Result {
	if !reflect.DeepEqual(expected, actual) {
		return Pass("values are not equal")
	}
	return Fail("values should not be equal", "different", actual)
}

// True asserts that a value is true.
func True(value bool) *Result {
	if value {
		return Pass("value is true")
	}
	return Fail("expected true", true, false)
}

// False asserts that a value is false.
func False(value bool) *Result {
	if !value {
		return Pass("value is false")
	}
	return Fail("expected false", false, true)
}

// Nil asserts that a value is nil.
func Nil(value interface{}) *Result {
	if value == nil {
		return Pass("value is nil")
	}
	// Check for typed nil
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return Pass("value is nil")
	}
	return Fail("expected nil", nil, value)
}

// NotNil asserts that a value is not nil.
func NotNil(value interface{}) *Result {
	if value == nil {
		return Fail("expected non-nil", "non-nil", nil)
	}
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return Fail("expected non-nil", "non-nil", nil)
	}
	return Pass("value is not nil")
}

// Contains asserts that a slice/map/string contains a value.
func Contains(container, element interface{}) *Result {
	cv := reflect.ValueOf(container)

	switch cv.Kind() {
	case reflect.String:
		s := cv.String()
		e := fmt.Sprintf("%v", element)
		if contains(s, e) {
			return Pass(fmt.Sprintf("string contains %q", e))
		}
		return Fail(fmt.Sprintf("string does not contain %q", e), e, s)

	case reflect.Slice, reflect.Array:
		for i := 0; i < cv.Len(); i++ {
			if reflect.DeepEqual(cv.Index(i).Interface(), element) {
				return Pass(fmt.Sprintf("slice contains %v", element))
			}
		}
		return Fail("slice does not contain element", element, container)

	case reflect.Map:
		if cv.MapIndex(reflect.ValueOf(element)).IsValid() {
			return Pass(fmt.Sprintf("map contains key %v", element))
		}
		return Fail("map does not contain key", element, "not found")

	default:
		return Fail("container must be string, slice, array, or map", "container", cv.Kind().String())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// InRange asserts that a numeric value is within a range [min, max].
func InRange(value, min, max interface{}) *Result {
	vf, vok := toFloat64(value)
	minf, minok := toFloat64(min)
	maxf, maxok := toFloat64(max)

	if !vok || !minok || !maxok {
		return Fail("values must be numeric", "[min, max]", value)
	}

	if vf >= minf && vf <= maxf {
		return Pass(fmt.Sprintf("%v is in range [%v, %v]", value, min, max))
	}
	return Fail(fmt.Sprintf("%v is not in range [%v, %v]", value, min, max),
		fmt.Sprintf("[%v, %v]", min, max), value)
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// GreaterThan asserts that a value is greater than another.
func GreaterThan(value, threshold interface{}) *Result {
	vf, vok := toFloat64(value)
	tf, tok := toFloat64(threshold)

	if !vok || !tok {
		return Fail("values must be numeric", "> threshold", value)
	}

	if vf > tf {
		return Pass(fmt.Sprintf("%v > %v", value, threshold))
	}
	return Fail(fmt.Sprintf("%v is not greater than %v", value, threshold),
		fmt.Sprintf("> %v", threshold), value)
}

// LessThan asserts that a value is less than another.
func LessThan(value, threshold interface{}) *Result {
	vf, vok := toFloat64(value)
	tf, tok := toFloat64(threshold)

	if !vok || !tok {
		return Fail("values must be numeric", "< threshold", value)
	}

	if vf < tf {
		return Pass(fmt.Sprintf("%v < %v", value, threshold))
	}
	return Fail(fmt.Sprintf("%v is not less than %v", value, threshold),
		fmt.Sprintf("< %v", threshold), value)
}

// TimingResult represents a timing assertion result.
type TimingResult struct {
	*Result
	Duration  time.Duration
	Tolerance time.Duration
}

// WithinDuration asserts that a duration is within expected +/- tolerance.
func WithinDuration(actual, expected, tolerance time.Duration) *TimingResult {
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}

	tr := &TimingResult{
		Duration:  actual,
		Tolerance: tolerance,
	}

	if diff <= tolerance {
		tr.Result = Pass(fmt.Sprintf("duration %v is within %v +/- %v", actual, expected, tolerance))
	} else {
		tr.Result = Fail(
			fmt.Sprintf("duration %v is not within %v +/- %v (diff: %v)", actual, expected, tolerance, diff),
			fmt.Sprintf("%v +/- %v", expected, tolerance),
			actual,
		)
	}

	return tr
}

// StateResult represents a state assertion result.
type StateResult struct {
	*Result
	State string
}

// HasState asserts that an entity has a specific state.
func HasState(actualState, expectedState string) *StateResult {
	sr := &StateResult{State: actualState}

	if actualState == expectedState {
		sr.Result = Pass(fmt.Sprintf("state is %s", expectedState))
	} else {
		sr.Result = Fail("state mismatch", expectedState, actualState)
	}

	return sr
}

// ErrorCodeResult represents an error code assertion result.
type ErrorCodeResult struct {
	*Result
	Code uint8
}

// HasErrorCode asserts that an error has a specific code.
func HasErrorCode(actualCode, expectedCode uint8) *ErrorCodeResult {
	er := &ErrorCodeResult{Code: actualCode}

	if actualCode == expectedCode {
		er.Result = Pass(fmt.Sprintf("error code is %d", expectedCode))
	} else {
		er.Result = Fail("error code mismatch", expectedCode, actualCode)
	}

	return er
}

// NoError asserts that an error is nil.
func NoError(err error) *Result {
	if err == nil {
		return Pass("no error")
	}
	return Fail("expected no error", nil, err.Error())
}

// Error asserts that an error is not nil.
func Error(err error) *Result {
	if err != nil {
		return Pass(fmt.Sprintf("error: %v", err))
	}
	return Fail("expected an error", "error", nil)
}

// ErrorContains asserts that an error message contains a substring.
func ErrorContains(err error, substr string) *Result {
	if err == nil {
		return Fail("expected an error", "error containing "+substr, nil)
	}
	if contains(err.Error(), substr) {
		return Pass(fmt.Sprintf("error contains %q", substr))
	}
	return Fail(fmt.Sprintf("error does not contain %q", substr), substr, err.Error())
}

// Len asserts that a collection has a specific length.
func Len(collection interface{}, expectedLen int) *Result {
	cv := reflect.ValueOf(collection)

	var actualLen int
	switch cv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String, reflect.Chan:
		actualLen = cv.Len()
	default:
		return Fail("value must be a collection", "collection", cv.Kind().String())
	}

	if actualLen == expectedLen {
		return Pass(fmt.Sprintf("length is %d", expectedLen))
	}
	return Fail("length mismatch", expectedLen, actualLen)
}

// Empty asserts that a collection is empty.
func Empty(collection interface{}) *Result {
	return Len(collection, 0)
}

// NotEmpty asserts that a collection is not empty.
func NotEmpty(collection interface{}) *Result {
	cv := reflect.ValueOf(collection)

	var actualLen int
	switch cv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String, reflect.Chan:
		actualLen = cv.Len()
	default:
		return Fail("value must be a collection", "collection", cv.Kind().String())
	}

	if actualLen > 0 {
		return Pass(fmt.Sprintf("collection has %d elements", actualLen))
	}
	return Fail("expected non-empty collection", "> 0", 0)
}
