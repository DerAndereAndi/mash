package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// variablePattern matches {{ variable }} templates.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// Interpolate replaces {{ variable }} placeholders in a string with values from state.
// If a variable is not found in state, the placeholder is left unchanged.
// Returns the interpolated string.
func Interpolate(template string, state *ExecutionState) string {
	if state == nil {
		return template
	}

	return variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable name from {{ name }}
		submatches := variablePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		varName := submatches[1]

		// Look up value in state
		value, exists := state.Outputs[varName]
		if !exists {
			return match // Leave undefined variables as-is
		}

		// Convert value to string
		return valueToString(value)
	})
}

// InterpolateParams recursively interpolates all string values in a params map.
// For pure variable references (string is exactly "{{ var }}"), preserves the original type.
// For mixed content (string contains text + variables), converts to string.
// Returns a new map with interpolated values.
func InterpolateParams(params map[string]interface{}, state *ExecutionState) map[string]interface{} {
	if params == nil {
		return nil
	}

	if state == nil {
		// Return a copy to avoid modifying original
		result := make(map[string]interface{}, len(params))
		for k, v := range params {
			result[k] = v
		}
		return result
	}

	result := make(map[string]interface{}, len(params))
	for key, value := range params {
		result[key] = interpolateValue(value, state)
	}
	return result
}

// interpolateValue recursively interpolates a single value.
func interpolateValue(value interface{}, state *ExecutionState) interface{} {
	switch v := value.(type) {
	case string:
		return interpolateString(v, state)

	case map[string]interface{}:
		// Recursively interpolate nested maps
		result := make(map[string]interface{}, len(v))
		for k, val := range v {
			result[k] = interpolateValue(val, state)
		}
		return result

	case []interface{}:
		// Recursively interpolate arrays
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = interpolateValue(val, state)
		}
		return result

	default:
		// Non-string values pass through unchanged
		return value
	}
}

// interpolateString handles string interpolation with type preservation.
// If the string is purely a single variable reference "{{ var }}", returns the actual value type.
// If it contains text mixed with variables, returns interpolated string.
func interpolateString(s string, state *ExecutionState) interface{} {
	trimmed := strings.TrimSpace(s)

	// Check if this is a pure variable reference (exactly "{{ var }}")
	if isPureVariableRef(trimmed) {
		// Extract variable name
		submatches := variablePattern.FindStringSubmatch(trimmed)
		if len(submatches) >= 2 {
			varName := submatches[1]
			if value, exists := state.Outputs[varName]; exists {
				return value // Return actual type, not string
			}
		}
		return s // Variable not found, return original
	}

	// Mixed content - interpolate as string
	return Interpolate(s, state)
}

// isPureVariableRef checks if a string is exactly a single variable reference.
func isPureVariableRef(s string) bool {
	matches := variablePattern.FindAllStringIndex(s, -1)
	if len(matches) != 1 {
		return false
	}
	// Check if the match spans the entire string
	return matches[0][0] == 0 && matches[0][1] == len(s)
}

// valueToString converts a value to its string representation.
func valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		// Format without trailing zeros for whole numbers
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
