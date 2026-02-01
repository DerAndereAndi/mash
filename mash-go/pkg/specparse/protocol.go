package specparse

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

// RawUseCaseRef represents a use case reference in protocol-versions.yaml.
type RawUseCaseRef struct {
	ID    int `yaml:"id"`
	Major int `yaml:"major"`
	Minor int `yaml:"minor"`
}

// RawProtocolVersion maps feature names to version strings and includes model type registries.
type RawProtocolVersion struct {
	Description   string                   `yaml:"description"`
	Features      map[string]string        `yaml:"features"`
	Shared        string                   `yaml:"shared"`
	FeatureTypes  []RawModelTypeDef        `yaml:"feature_types"`
	EndpointTypes []RawModelTypeDef        `yaml:"endpoint_types"`
	UseCases      map[string]RawUseCaseRef `yaml:"usecases"`
	UseCaseTypes  []RawModelTypeDef        `yaml:"use_case_types"`
}

// ParseProtocolVersions parses protocol version mappings from YAML bytes.
func ParseProtocolVersions(data []byte) (*RawProtocolVersions, error) {
	var pv RawProtocolVersions
	if err := yaml.Unmarshal(data, &pv); err != nil {
		return nil, fmt.Errorf("parsing protocol versions: %w", err)
	}
	return &pv, nil
}

// LoadProtocolVersions loads and parses protocol version mappings from a file.
func LoadProtocolVersions(path string) (*RawProtocolVersions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseProtocolVersions(data)
}
