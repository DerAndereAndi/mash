package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// variablePattern matches {{ variable }} or {{ variable + N }} templates for state variables.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*([+\-])\s*(\d+))?\s*\}\}`)

// picsPattern matches ${PICS_ITEM} or ${PICS_ITEM + N} templates for PICS values.
// Examples: ${MASH.S.COMM.BACKOFF_TIER2}, ${MASH.S.ZONE.MAX + 1}
var picsPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_.]+)(?:\s*([+\-])\s*(\d+))?\}`)

// Interpolate replaces {{ variable }} placeholders in a string with values from state.
// If a variable is not found in state, the placeholder is left unchanged.
// Returns the interpolated string.
func Interpolate(template string, state *ExecutionState) string {
	if state == nil {
		return template
	}

	return variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable name and optional arithmetic from {{ name [+/- N] }}
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

		// Apply arithmetic if present
		if len(submatches) >= 4 && submatches[2] != "" {
			operator := submatches[2]
			operand, _ := strconv.Atoi(submatches[3])
			numValue := toNumeric(value)
			switch operator {
			case "+":
				return valueToString(numValue + int64(operand))
			case "-":
				return valueToString(numValue - int64(operand))
			}
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
// If the string is purely a single variable reference "{{ var }}" or "{{ var + N }}",
// returns the actual value type (with arithmetic applied if present).
// If it contains text mixed with variables, returns interpolated string.
func interpolateString(s string, state *ExecutionState) interface{} {
	trimmed := strings.TrimSpace(s)

	// Check if this is a pure variable reference (exactly "{{ var }}" or "{{ var + N }}")
	if isPureVariableRef(trimmed) {
		// Extract variable name and optional arithmetic
		submatches := variablePattern.FindStringSubmatch(trimmed)
		if len(submatches) >= 2 {
			varName := submatches[1]
			if value, exists := state.Outputs[varName]; exists {
				// Apply arithmetic if present
				if len(submatches) >= 4 && submatches[2] != "" {
					operator := submatches[2]
					operand, _ := strconv.Atoi(submatches[3])
					numValue := toNumeric(value)
					switch operator {
					case "+":
						return numValue + int64(operand)
					case "-":
						return numValue - int64(operand)
					}
				}
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

// ============================================================================
// PICS Value Substitution (DEC-047)
// ============================================================================

// InterpolateWithPICS replaces both {{ variable }} and ${PICS_ITEM} placeholders.
// This is the preferred interpolation function when PICS values are available.
func InterpolateWithPICS(template string, state *ExecutionState, pics *loader.PICSFile) string {
	if template == "" {
		return template
	}

	// First interpolate state variables
	result := Interpolate(template, state)

	// Then interpolate PICS values
	if pics != nil {
		result = interpolatePICS(result, pics)
	}

	return result
}

// InterpolateParamsWithPICS recursively interpolates params with PICS support.
// For pure variable references, preserves the original type.
// For PICS references, returns numeric types when appropriate.
func InterpolateParamsWithPICS(params map[string]interface{}, state *ExecutionState, pics *loader.PICSFile) map[string]interface{} {
	if params == nil {
		return nil
	}

	result := make(map[string]interface{}, len(params))
	for key, value := range params {
		result[key] = interpolateValueWithPICS(value, state, pics)
	}
	return result
}

// interpolateValueWithPICS recursively interpolates a single value with PICS support.
func interpolateValueWithPICS(value interface{}, state *ExecutionState, pics *loader.PICSFile) interface{} {
	switch v := value.(type) {
	case string:
		return interpolateStringWithPICS(v, state, pics)

	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for k, val := range v {
			result[k] = interpolateValueWithPICS(val, state, pics)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = interpolateValueWithPICS(val, state, pics)
		}
		return result

	default:
		return value
	}
}

// interpolateStringWithPICS handles string interpolation with PICS and type preservation.
func interpolateStringWithPICS(s string, state *ExecutionState, pics *loader.PICSFile) interface{} {
	trimmed := strings.TrimSpace(s)

	// Check if this is a pure state variable reference
	if isPureVariableRef(trimmed) && state != nil {
		submatches := variablePattern.FindStringSubmatch(trimmed)
		if len(submatches) >= 2 {
			varName := submatches[1]
			if value, exists := state.Outputs[varName]; exists {
				// Apply arithmetic if present
				if len(submatches) >= 4 && submatches[2] != "" {
					operator := submatches[2]
					operand, _ := strconv.Atoi(submatches[3])
					numValue := toNumeric(value)
					switch operator {
					case "+":
						return numValue + int64(operand)
					case "-":
						return numValue - int64(operand)
					}
				}
				return value
			}
		}
		return s
	}

	// Check if this is a pure PICS reference
	if isPurePICSRef(trimmed) && pics != nil {
		return interpolatePICSValue(trimmed, pics)
	}

	// Mixed content - interpolate as string
	result := Interpolate(s, state)
	if pics != nil {
		result = interpolatePICS(result, pics)
	}
	return result
}

// isPurePICSRef checks if a string is exactly a single PICS reference.
func isPurePICSRef(s string) bool {
	matches := picsPattern.FindAllStringIndex(s, -1)
	if len(matches) != 1 {
		return false
	}
	return matches[0][0] == 0 && matches[0][1] == len(s)
}

// interpolatePICS replaces all ${PICS_ITEM} placeholders in a string.
func interpolatePICS(template string, pics *loader.PICSFile) string {
	return picsPattern.ReplaceAllStringFunc(template, func(match string) string {
		value := interpolatePICSValue(match, pics)
		return valueToString(value)
	})
}

// interpolatePICSValue extracts and evaluates a single PICS reference.
// Returns the actual value type (int, bool, string) for type preservation.
func interpolatePICSValue(match string, pics *loader.PICSFile) interface{} {
	submatches := picsPattern.FindStringSubmatch(match)
	if len(submatches) < 2 {
		return match
	}

	picsKey := submatches[1]
	operator := ""
	operand := 0

	if len(submatches) >= 4 && submatches[2] != "" {
		operator = submatches[2]
		operand, _ = strconv.Atoi(submatches[3])
	}

	// Look up PICS value
	value, exists := pics.Items[picsKey]
	if !exists {
		return match // Leave undefined PICS refs as-is
	}

	// Apply arithmetic if present
	if operator != "" {
		numValue := toNumeric(value)
		switch operator {
		case "+":
			return numValue + int64(operand)
		case "-":
			return numValue - int64(operand)
		}
	}

	return value
}

// toNumeric converts a PICS value to numeric for arithmetic operations.
func toNumeric(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
		return 0
	default:
		return 0
	}
}
