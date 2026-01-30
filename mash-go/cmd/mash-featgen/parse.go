package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RawFeatureDef represents a feature definition loaded from YAML.
type RawFeatureDef struct {
	Name        string            `yaml:"name"`
	ID          uint8             `yaml:"id"`
	Revision    uint16            `yaml:"revision"`
	Mandatory   bool              `yaml:"mandatory"`
	Description string            `yaml:"description"`
	Enums       []RawEnumDef      `yaml:"enums"`
	Attributes  []RawAttributeDef `yaml:"attributes"`
	Commands    []RawCommandDef   `yaml:"commands"`
}

// RawEnumDef represents an enum type definition.
type RawEnumDef struct {
	Name        string         `yaml:"name"`
	Type        string         `yaml:"type"` // "uint8"
	Description string         `yaml:"description"`
	Values      []RawEnumValue `yaml:"values"`
}

// RawEnumValue represents a single enum value.
type RawEnumValue struct {
	Name        string `yaml:"name"`
	Value       int    `yaml:"value"`
	Description string `yaml:"description"`
}

// RawAttributeDef represents an attribute definition.
type RawAttributeDef struct {
	ID           uint16           `yaml:"id"`
	Name         string           `yaml:"name"`
	Type         string           `yaml:"type"`         // "uint8", "int64", "map", "string", "bool", "array", etc.
	Enum         string           `yaml:"enum"`         // Optional: references enum name
	MapKeyType   string           `yaml:"mapKeyType"`   // For map types: "Phase", "PhasePair"
	MapValueType string           `yaml:"mapValueType"` // For map types: "int64", "uint32"
	Items        *RawArrayItemDef `yaml:"items"`        // For typed array attributes
	Access       string           `yaml:"access"`       // "readOnly", "readWrite"
	Mandatory    bool             `yaml:"mandatory"`
	Nullable     bool             `yaml:"nullable"`
	Default      any              `yaml:"default"`
	Min          any              `yaml:"min"`
	Max          any              `yaml:"max"`
	Unit         string           `yaml:"unit"`
	Description  string           `yaml:"description"`
}

// RawArrayItemDef describes the items of a typed array attribute.
type RawArrayItemDef struct {
	Type       string             `yaml:"type"`       // "uint8", "string", "object"
	Enum       string             `yaml:"enum"`       // for enum arrays
	StructName string             `yaml:"structName"` // for object arrays
	Fields     []RawArrayFieldDef `yaml:"fields"`     // for object arrays
}

// RawArrayFieldDef describes a field within an object array item.
type RawArrayFieldDef struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Enum string `yaml:"enum"`
}

// RawCommandDef represents a command definition.
type RawCommandDef struct {
	ID          uint8             `yaml:"id"`
	Name        string            `yaml:"name"`
	Mandatory   bool              `yaml:"mandatory"`
	Description string            `yaml:"description"`
	Parameters  []RawParameterDef `yaml:"parameters"`
	Response    []RawParameterDef `yaml:"response"`
}

// RawParameterDef represents a command parameter or response field.
type RawParameterDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Enum        string `yaml:"enum"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// RawSharedTypes represents shared type definitions loaded from YAML.
type RawSharedTypes struct {
	Version string       `yaml:"version"`
	Enums   []RawEnumDef `yaml:"enums"`
}

// RawProtocolVersions represents the protocol version mapping.
type RawProtocolVersions struct {
	Versions map[string]RawProtocolVersion `yaml:"versions"`
}

// RawModelTypeDef represents a type definition in the model registry (feature or endpoint types).
type RawModelTypeDef struct {
	Name        string `yaml:"name"`
	ID          int    `yaml:"id"`
	Description string `yaml:"description"`
}

// RawProtocolVersion maps feature names to version strings and includes model type registries.
type RawProtocolVersion struct {
	Description   string            `yaml:"description"`
	Features      map[string]string `yaml:"features"`
	Shared        string            `yaml:"shared"`
	FeatureTypes  []RawModelTypeDef `yaml:"feature_types"`
	EndpointTypes []RawModelTypeDef `yaml:"endpoint_types"`
}

// ParseFeatureDef parses a feature definition from YAML bytes.
func ParseFeatureDef(data []byte) (*RawFeatureDef, error) {
	var def RawFeatureDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing feature def: %w", err)
	}
	if def.Name == "" {
		return nil, fmt.Errorf("feature definition missing name")
	}
	return &def, nil
}

// ParseSharedTypes parses shared type definitions from YAML bytes.
func ParseSharedTypes(data []byte) (*RawSharedTypes, error) {
	var shared RawSharedTypes
	if err := yaml.Unmarshal(data, &shared); err != nil {
		return nil, fmt.Errorf("parsing shared types: %w", err)
	}
	return &shared, nil
}

// ParseProtocolVersions parses protocol version mappings from YAML bytes.
func ParseProtocolVersions(data []byte) (*RawProtocolVersions, error) {
	var pv RawProtocolVersions
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil, fmt.Errorf("parsing protocol versions: %w", err)
	}
	return &pv, nil
}

// LoadFeatureDef loads and parses a feature definition from a file.
func LoadFeatureDef(path string) (*RawFeatureDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseFeatureDef(data)
}

// LoadSharedTypes loads and parses shared types from a file.
func LoadSharedTypes(path string) (*RawSharedTypes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseSharedTypes(data)
}

// LoadProtocolVersions loads and parses protocol version mappings from a file.
func LoadProtocolVersions(path string) (*RawProtocolVersions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseProtocolVersions(data)
}
