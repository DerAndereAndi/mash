package specparse

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RawConformance defines mandatory and recommended attributes for a feature
// within a specific endpoint type.
type RawConformance struct {
	Mandatory   []string `yaml:"mandatory"`
	Recommended []string `yaml:"recommended"`
}

// RawEndpointConformance represents the full endpoint conformance registry.
type RawEndpointConformance struct {
	SpecVersion   string                                `yaml:"specVersion"`
	EndpointTypes map[string]map[string]*RawConformance `yaml:"endpointTypes"`
}

// ParseEndpointConformance parses endpoint conformance from YAML bytes.
func ParseEndpointConformance(data []byte) (*RawEndpointConformance, error) {
	var ec RawEndpointConformance
	if err := yaml.Unmarshal(data, &ec); err != nil {
		return nil, fmt.Errorf("parsing endpoint conformance: %w", err)
	}
	return &ec, nil
}

// LoadEndpointConformance loads and parses endpoint conformance from a file.
func LoadEndpointConformance(path string) (*RawEndpointConformance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseEndpointConformance(data)
}
