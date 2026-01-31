package usecase

import (
	"fmt"

	"github.com/mash-protocol/mash-go/pkg/version"
	"gopkg.in/yaml.v3"
)

// RawUseCaseDef is the YAML-level representation before name resolution.
type RawUseCaseDef struct {
	Name          string              `yaml:"name"`
	FullName      string              `yaml:"fullName"`
	SpecVersion   string              `yaml:"specVersion"`
	Major         uint8               `yaml:"major"`
	Minor         uint8               `yaml:"minor"`
	Description   string              `yaml:"description"`
	EndpointTypes []string            `yaml:"endpointTypes"`
	Features      []RawFeatureReq     `yaml:"features"`
	Commands      []string            `yaml:"commands"`
}

// RawFeatureReq is a feature requirement before name resolution.
type RawFeatureReq struct {
	Feature       string            `yaml:"feature"`
	Required      bool              `yaml:"required"`
	Attributes    []RawAttrReq      `yaml:"attributes"`
	Commands      []string          `yaml:"commands"`
	Subscribe string `yaml:"subscribe"` // "all" = subscribe to all attributes (DEC-052)
}

// RawAttrReq is an attribute requirement before name resolution.
type RawAttrReq struct {
	Name          string `yaml:"name"`
	RequiredValue *bool  `yaml:"requiredValue"`
}

// ParseRawUseCaseDef parses YAML bytes into a RawUseCaseDef.
func ParseRawUseCaseDef(data []byte) (*RawUseCaseDef, error) {
	var raw RawUseCaseDef
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing use case YAML: %w", err)
	}
	return &raw, nil
}

// ResolveUseCaseDef resolves symbolic names in a RawUseCaseDef to numeric IDs
// using the spec manifest for the referenced spec version.
func ResolveUseCaseDef(raw *RawUseCaseDef) (*UseCaseDef, error) {
	specVer := raw.SpecVersion
	if specVer == "" {
		specVer = version.Current
	}

	spec, err := version.LoadSpec(specVer)
	if err != nil {
		return nil, fmt.Errorf("loading spec %q: %w", specVer, err)
	}

	def := &UseCaseDef{
		Name:          UseCaseName(raw.Name),
		FullName:      raw.FullName,
		Description:   raw.Description,
		SpecVersion:   specVer,
		Major:         raw.Major,
		Minor:         raw.Minor,
		EndpointTypes: raw.EndpointTypes,
		Commands:      raw.Commands,
	}

	for _, rf := range raw.Features {
		fr, err := resolveFeatureReq(spec, rf)
		if err != nil {
			return nil, fmt.Errorf("use case %s: %w", raw.Name, err)
		}
		def.Features = append(def.Features, *fr)
	}

	return def, nil
}

func resolveFeatureReq(spec *version.SpecManifest, rf RawFeatureReq) (*FeatureRequirement, error) {
	featureSpec, ok := spec.Features[rf.Feature]
	if !ok {
		return nil, fmt.Errorf("unknown feature %q", rf.Feature)
	}

	fr := &FeatureRequirement{
		FeatureName: rf.Feature,
		FeatureID:   featureSpec.ID,
		Required:    rf.Required,
	}

	// Build lookup maps for attributes and commands
	attrByName := buildAttrNameMap(&featureSpec)
	cmdByName := buildCmdNameMap(&featureSpec)

	// Resolve attributes
	for _, ra := range rf.Attributes {
		attrID, ok := attrByName[ra.Name]
		if !ok {
			return nil, fmt.Errorf("feature %s: unknown attribute %q", rf.Feature, ra.Name)
		}
		fr.Attributes = append(fr.Attributes, AttributeRequirement{
			Name:          ra.Name,
			AttrID:        attrID,
			RequiredValue: ra.RequiredValue,
		})
	}

	// Resolve commands
	for _, cmdName := range rf.Commands {
		cmdID, ok := cmdByName[cmdName]
		if !ok {
			return nil, fmt.Errorf("feature %s: unknown command %q", rf.Feature, cmdName)
		}
		fr.Commands = append(fr.Commands, CommandRequirement{
			Name:      cmdName,
			CommandID: cmdID,
		})
	}

	// Resolve subscriptions
	if rf.Subscribe == "all" {
		fr.SubscribeAll = true
	} else if rf.Subscribe != "" {
		return nil, fmt.Errorf("feature %s: invalid subscribe value %q (expected \"all\")", rf.Feature, rf.Subscribe)
	}

	return fr, nil
}

func buildAttrNameMap(fs *version.FeatureSpec) map[string]uint16 {
	m := make(map[string]uint16)
	for _, a := range fs.Attributes.Mandatory {
		m[a.Name] = a.ID
	}
	for _, a := range fs.Attributes.Optional {
		m[a.Name] = a.ID
	}
	return m
}

func buildCmdNameMap(fs *version.FeatureSpec) map[string]uint8 {
	m := make(map[string]uint8)
	for _, c := range fs.Commands.Mandatory {
		m[c.Name] = c.ID
	}
	for _, c := range fs.Commands.Optional {
		m[c.Name] = c.ID
	}
	return m
}
