package inspect

import (
	"strings"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Name tables for resolving human-readable names to IDs.
// Tables are populated by generated code in names_gen.go.
var (
	endpointNames  map[string]uint8
	featureNames   map[string]uint8
	attributeNames = map[uint8]map[string]uint16{}
	commandNames   = map[uint8]map[string]uint8{}
)

func init() {
	endpointNames = generatedEndpointNames()
	featureNames = generatedFeatureNames()
	initGeneratedNameTables()

	// Aliases not present in YAML -- used by test specs and legacy references.
	attributeNames[uint8(model.FeatureDeviceInfo)]["endpointList"] = features.DeviceInfoAttrEndpoints
	attributeNames[uint8(model.FeatureSignals)]["schedule"] = features.SignalsAttrPriceSlots
}

// ResolveEndpointName resolves an endpoint name to its ID (case-insensitive).
func ResolveEndpointName(name string) (uint8, bool) {
	lname := strings.ToLower(name)
	for k, v := range endpointNames {
		if strings.ToLower(k) == lname {
			return v, true
		}
	}
	return 0, false
}

// ResolveFeatureName resolves a feature name to its ID (case-insensitive).
func ResolveFeatureName(name string) (uint8, bool) {
	lname := strings.ToLower(name)
	for k, v := range featureNames {
		if strings.ToLower(k) == lname {
			return v, true
		}
	}
	return 0, false
}

// ResolveAttributeName resolves an attribute name to its ID for a given feature (case-insensitive).
func ResolveAttributeName(featureID uint8, name string) (uint16, bool) {
	if attrNames, ok := attributeNames[featureID]; ok {
		lname := strings.ToLower(name)
		for k, v := range attrNames {
			if strings.ToLower(k) == lname {
				return v, true
			}
		}
	}
	return 0, false
}

// GetEndpointName returns the name for an endpoint type.
func GetEndpointName(id uint8) string {
	epType := model.EndpointType(id)
	return epType.String()
}

// GetFeatureName returns the name for a feature type.
func GetFeatureName(id uint8) string {
	featType := model.FeatureType(id)
	return featType.String()
}

// GetAttributeName returns the name for an attribute ID within a feature.
func GetAttributeName(featureID uint8, attrID uint16) string {
	if attrNames, ok := attributeNames[featureID]; ok {
		for name, id := range attrNames {
			if id == attrID {
				return name
			}
		}
	}
	// Return numeric if not found
	return ""
}

// ResolveCommandName resolves a command name to its ID for a given feature (case-insensitive).
func ResolveCommandName(featureID uint8, name string) (uint8, bool) {
	if cmdNames, ok := commandNames[featureID]; ok {
		lname := strings.ToLower(name)
		for k, v := range cmdNames {
			if strings.ToLower(k) == lname {
				return v, true
			}
		}
	}
	return 0, false
}

// GetCommandName returns the name for a command ID within a feature.
func GetCommandName(featureID uint8, cmdID uint8) string {
	if cmdNames, ok := commandNames[featureID]; ok {
		for name, id := range cmdNames {
			if id == cmdID {
				return name
			}
		}
	}
	return ""
}
