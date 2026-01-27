package engine

import (
	"context"
	"testing"
)

// TestInterpolation_Basic tests basic variable interpolation.
func TestInterpolation_Basic(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("device_id", "test-device-123")

	result := Interpolate("Device: {{ device_id }}", state)
	expected := "Device: test-device-123"

	if result != expected {
		t.Errorf("Interpolate() = %q, want %q", result, expected)
	}
}

// TestInterpolation_Multiple tests multiple variables in one string.
func TestInterpolation_Multiple(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("host", "localhost")
	state.Set("port", "8443")

	result := Interpolate("Connect to {{ host }}:{{ port }}", state)
	expected := "Connect to localhost:8443"

	if result != expected {
		t.Errorf("Interpolate() = %q, want %q", result, expected)
	}
}

// TestInterpolation_NoVariables tests strings without variables.
func TestInterpolation_NoVariables(t *testing.T) {
	state := NewExecutionState(context.Background())

	tests := []string{
		"plain string",
		"no variables here",
		"",
	}

	for _, input := range tests {
		result := Interpolate(input, state)
		if result != input {
			t.Errorf("Interpolate(%q) = %q, want %q", input, result, input)
		}
	}
}

// TestInterpolation_UndefinedVariable tests undefined variables are left as-is.
func TestInterpolation_UndefinedVariable(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("defined", "value")

	result := Interpolate("{{ defined }} and {{ undefined }}", state)
	expected := "value and {{ undefined }}"

	if result != expected {
		t.Errorf("Interpolate() = %q, want %q", result, expected)
	}
}

// TestInterpolation_WhitespaceVariants tests different whitespace in templates.
func TestInterpolation_WhitespaceVariants(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("value", "test")

	tests := []struct {
		input    string
		expected string
	}{
		{"{{value}}", "test"},
		{"{{ value }}", "test"},
		{"{{  value  }}", "test"},
		{"{{ value}}", "test"},
		{"{{value }}", "test"},
	}

	for _, tt := range tests {
		result := Interpolate(tt.input, state)
		if result != tt.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestInterpolation_NumericValues tests interpolation of numeric values.
func TestInterpolation_NumericValues(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("count", 42)
	state.Set("price", 19.99)

	tests := []struct {
		input    string
		expected string
	}{
		{"Count: {{ count }}", "Count: 42"},
		{"Price: {{ price }}", "Price: 19.99"},
	}

	for _, tt := range tests {
		result := Interpolate(tt.input, state)
		if result != tt.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestInterpolateParams_Map tests recursive map parameter interpolation.
func TestInterpolateParams_Map(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("endpoint", float64(1))
	state.Set("feature", "Measurement")
	state.Set("attr", "acActivePower")

	params := map[string]interface{}{
		"endpoint":  "{{ endpoint }}",
		"feature":   "{{ feature }}",
		"attribute": "{{ attr }}",
		"literal":   42,
	}

	result := InterpolateParams(params, state)

	// Check endpoint was resolved (should be numeric value, not string)
	if ep, ok := result["endpoint"].(float64); !ok || ep != 1 {
		t.Errorf("endpoint = %v (%T), want 1 (float64)", result["endpoint"], result["endpoint"])
	}

	// Check feature was resolved to string
	if feat, ok := result["feature"].(string); !ok || feat != "Measurement" {
		t.Errorf("feature = %v (%T), want 'Measurement' (string)", result["feature"], result["feature"])
	}

	// Check attribute was resolved
	if attr, ok := result["attribute"].(string); !ok || attr != "acActivePower" {
		t.Errorf("attribute = %v (%T), want 'acActivePower' (string)", result["attribute"], result["attribute"])
	}

	// Check literal was preserved
	if lit, ok := result["literal"].(int); !ok || lit != 42 {
		t.Errorf("literal = %v (%T), want 42 (int)", result["literal"], result["literal"])
	}
}

// TestInterpolateParams_NestedMap tests interpolation in nested maps.
func TestInterpolateParams_NestedMap(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("value", "nested-value")

	params := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "{{ value }}",
		},
	}

	result := InterpolateParams(params, state)

	outer, ok := result["outer"].(map[string]interface{})
	if !ok {
		t.Fatalf("outer is not a map: %T", result["outer"])
	}

	if inner, ok := outer["inner"].(string); !ok || inner != "nested-value" {
		t.Errorf("outer.inner = %v (%T), want 'nested-value' (string)", outer["inner"], outer["inner"])
	}
}

// TestInterpolateParams_Array tests interpolation in arrays.
func TestInterpolateParams_Array(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("item1", "first")
	state.Set("item2", "second")

	params := map[string]interface{}{
		"items": []interface{}{
			"{{ item1 }}",
			"{{ item2 }}",
			"literal",
		},
	}

	result := InterpolateParams(params, state)

	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatalf("items is not an array: %T", result["items"])
	}

	if len(items) != 3 {
		t.Fatalf("items length = %d, want 3", len(items))
	}

	if items[0] != "first" {
		t.Errorf("items[0] = %v, want 'first'", items[0])
	}
	if items[1] != "second" {
		t.Errorf("items[1] = %v, want 'second'", items[1])
	}
	if items[2] != "literal" {
		t.Errorf("items[2] = %v, want 'literal'", items[2])
	}
}

// TestInterpolateParams_PureVariableReference tests that pure {{ var }} returns actual value type.
func TestInterpolateParams_PureVariableReference(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("numeric", float64(42))
	state.Set("boolean", true)
	state.Set("object", map[string]interface{}{"key": "value"})

	params := map[string]interface{}{
		"num":  "{{ numeric }}",
		"bool": "{{ boolean }}",
		"obj":  "{{ object }}",
	}

	result := InterpolateParams(params, state)

	// Pure variable reference should preserve type
	if num, ok := result["num"].(float64); !ok || num != 42 {
		t.Errorf("num = %v (%T), want 42 (float64)", result["num"], result["num"])
	}

	if b, ok := result["bool"].(bool); !ok || b != true {
		t.Errorf("bool = %v (%T), want true (bool)", result["bool"], result["bool"])
	}

	if obj, ok := result["obj"].(map[string]interface{}); !ok {
		t.Errorf("obj = %T, want map[string]interface{}", result["obj"])
	} else if obj["key"] != "value" {
		t.Errorf("obj[key] = %v, want 'value'", obj["key"])
	}
}

// TestInterpolateParams_MixedContent tests that mixed content becomes string.
func TestInterpolateParams_MixedContent(t *testing.T) {
	state := NewExecutionState(context.Background())
	state.Set("name", "test")
	state.Set("count", 5)

	params := map[string]interface{}{
		"message": "Name: {{ name }}, Count: {{ count }}",
	}

	result := InterpolateParams(params, state)

	expected := "Name: test, Count: 5"
	if msg, ok := result["message"].(string); !ok || msg != expected {
		t.Errorf("message = %v (%T), want %q (string)", result["message"], result["message"], expected)
	}
}

// TestInterpolateParams_NilState tests handling of nil state.
func TestInterpolateParams_NilState(t *testing.T) {
	params := map[string]interface{}{
		"key": "{{ value }}",
	}

	result := InterpolateParams(params, nil)

	// Should return params unchanged when state is nil
	if result["key"] != "{{ value }}" {
		t.Errorf("key = %v, want '{{ value }}'", result["key"])
	}
}

// TestInterpolateParams_EmptyParams tests handling of empty params.
func TestInterpolateParams_EmptyParams(t *testing.T) {
	state := NewExecutionState(context.Background())

	result := InterpolateParams(nil, state)
	if result != nil {
		t.Errorf("InterpolateParams(nil, state) = %v, want nil", result)
	}

	result = InterpolateParams(map[string]interface{}{}, state)
	if len(result) != 0 {
		t.Errorf("InterpolateParams({}, state) has %d items, want 0", len(result))
	}
}
