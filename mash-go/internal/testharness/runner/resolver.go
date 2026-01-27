package runner

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/inspect"
)

// Resolver resolves feature and attribute names/IDs for test cases.
// Supports both string names (case-insensitive) and numeric IDs (passthrough).
type Resolver struct{}

// NewResolver creates a new resolver instance.
func NewResolver() *Resolver {
	return &Resolver{}
}

// ResolveFeature resolves a feature name or ID to its numeric ID.
// Accepts:
//   - string: feature name (case-insensitive), e.g., "Measurement", "measurement"
//   - float64: numeric ID (from YAML), e.g., 4.0
//   - int/uint8: numeric ID, e.g., 4
func (r *Resolver) ResolveFeature(nameOrID interface{}) (uint8, error) {
	switch v := nameOrID.(type) {
	case string:
		// Case-insensitive lookup
		id, ok := inspect.ResolveFeatureName(strings.ToLower(v))
		if !ok {
			return 0, fmt.Errorf("unknown feature name: %q", v)
		}
		return id, nil

	case float64:
		// YAML parses numbers as float64
		if v < 0 || v > 255 {
			return 0, fmt.Errorf("feature ID %v out of range [0-255]", v)
		}
		return uint8(v), nil

	case int:
		if v < 0 || v > 255 {
			return 0, fmt.Errorf("feature ID %d out of range [0-255]", v)
		}
		return uint8(v), nil

	case uint8:
		return v, nil

	default:
		return 0, fmt.Errorf("invalid feature type: %T (expected string or number)", nameOrID)
	}
}

// ResolveAttribute resolves an attribute name or ID to its numeric ID.
// Accepts:
//   - string: attribute name (case-insensitive), e.g., "acActivePower"
//   - float64: numeric ID (from YAML), e.g., 1.0
//   - int/uint16: numeric ID, e.g., 1
//
// The feature parameter is used to resolve feature-specific attributes.
func (r *Resolver) ResolveAttribute(feature interface{}, nameOrID interface{}) (uint16, error) {
	// First resolve the feature to get its ID
	featureID, err := r.ResolveFeature(feature)
	if err != nil {
		return 0, fmt.Errorf("resolving feature: %w", err)
	}

	switch v := nameOrID.(type) {
	case string:
		// Case-insensitive lookup
		id, ok := inspect.ResolveAttributeName(featureID, strings.ToLower(v))
		if !ok {
			return 0, fmt.Errorf("unknown attribute %q for feature 0x%02x", v, featureID)
		}
		return id, nil

	case float64:
		// YAML parses numbers as float64
		if v < 0 || v > 65535 {
			return 0, fmt.Errorf("attribute ID %v out of range [0-65535]", v)
		}
		return uint16(v), nil

	case int:
		if v < 0 || v > 65535 {
			return 0, fmt.Errorf("attribute ID %d out of range [0-65535]", v)
		}
		return uint16(v), nil

	case uint16:
		return v, nil

	default:
		return 0, fmt.Errorf("invalid attribute type: %T (expected string or number)", nameOrID)
	}
}

// ResolveEndpoint resolves an endpoint name or ID to its numeric ID.
// Accepts:
//   - string: endpoint type name (case-insensitive), e.g., "EVCharger", "evcharger"
//   - float64: numeric ID (from YAML), e.g., 5.0
//   - int/uint8: numeric ID, e.g., 5
func (r *Resolver) ResolveEndpoint(nameOrID interface{}) (uint8, error) {
	switch v := nameOrID.(type) {
	case string:
		// Case-insensitive lookup
		id, ok := inspect.ResolveEndpointName(strings.ToLower(v))
		if !ok {
			return 0, fmt.Errorf("unknown endpoint name: %q", v)
		}
		return id, nil

	case float64:
		// YAML parses numbers as float64
		if v < 0 || v > 255 {
			return 0, fmt.Errorf("endpoint ID %v out of range [0-255]", v)
		}
		return uint8(v), nil

	case int:
		if v < 0 || v > 255 {
			return 0, fmt.Errorf("endpoint ID %d out of range [0-255]", v)
		}
		return uint8(v), nil

	case uint8:
		return v, nil

	default:
		return 0, fmt.Errorf("invalid endpoint type: %T (expected string or number)", nameOrID)
	}
}
