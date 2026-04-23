package specparse

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RawSharedTypes represents shared type definitions loaded from YAML.
type RawSharedTypes struct {
	Version string       `yaml:"version"`
	Enums   []RawEnumDef `yaml:"enums"`
}

// ParseSharedTypes parses shared type definitions from YAML bytes.
func ParseSharedTypes(data []byte) (*RawSharedTypes, error) {
	var shared RawSharedTypes
	if err := yaml.Unmarshal(data, &shared); err != nil {
		return nil, fmt.Errorf("parsing shared types: %w", err)
	}
	return &shared, nil
}

// LoadSharedTypes loads and parses shared types from a file.
func LoadSharedTypes(path string) (*RawSharedTypes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseSharedTypes(data)
}
