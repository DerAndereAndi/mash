package version

import (
	"embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed specs/*.yaml
var specFS embed.FS

// SpecManifest describes what a MASH spec version requires.
type SpecManifest struct {
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Features    map[string]FeatureSpec `yaml:"features"`
}

// FeatureSpec describes a single feature within a spec version.
type FeatureSpec struct {
	ID         uint8         `yaml:"id"`
	Revision   uint16        `yaml:"revision"`
	Mandatory  bool          `yaml:"mandatory"`
	Attributes AttributeSpec `yaml:"attributes"`
	Commands   CommandSpec   `yaml:"commands"`
}

// AttributeSpec lists the mandatory and optional attributes of a feature.
type AttributeSpec struct {
	Mandatory []AttrDef `yaml:"mandatory"`
	Optional  []AttrDef `yaml:"optional"`
}

// AttrDef is a named attribute with its wire ID.
type AttrDef struct {
	ID   uint16 `yaml:"id"`
	Name string `yaml:"name"`
}

// CommandSpec lists the mandatory and optional commands of a feature.
type CommandSpec struct {
	Mandatory []CmdDef `yaml:"mandatory"`
	Optional  []CmdDef `yaml:"optional"`
}

// CmdDef is a named command with its wire ID.
type CmdDef struct {
	ID   uint8  `yaml:"id"`
	Name string `yaml:"name"`
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*SpecManifest)
)

// LoadSpec loads a spec manifest by version string (e.g. "1.0").
func LoadSpec(ver string) (*SpecManifest, error) {
	cacheMu.RLock()
	if s, ok := cache[ver]; ok {
		cacheMu.RUnlock()
		return s, nil
	}
	cacheMu.RUnlock()

	data, err := specFS.ReadFile("specs/" + ver + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("spec version %q not found: %w", ver, err)
	}

	var m SpecManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", ver, err)
	}

	cacheMu.Lock()
	cache[ver] = &m
	cacheMu.Unlock()

	return &m, nil
}

// LoadCurrentSpec loads the manifest for the current protocol version.
func LoadCurrentSpec() (*SpecManifest, error) {
	return LoadSpec(Current)
}

// AvailableSpecs returns the version strings of all embedded spec manifests.
func AvailableSpecs() ([]string, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	var versions []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			versions = append(versions, strings.TrimSuffix(name, ".yaml"))
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// MandatoryFeatures returns the names of all mandatory features, sorted.
func (s *SpecManifest) MandatoryFeatures() []string {
	var out []string
	for name, fs := range s.Features {
		if fs.Mandatory {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// FeatureByID looks up a feature by its numeric ID.
func (s *SpecManifest) FeatureByID(id uint8) (string, *FeatureSpec, bool) {
	for name, fs := range s.Features {
		if fs.ID == id {
			return name, &fs, true
		}
	}
	return "", nil, false
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// DeviceCapabilities describes what a device actually supports.
type DeviceCapabilities struct {
	SpecVersion string
	Features    map[string]FeatureCapabilities
}

// FeatureCapabilities describes a single feature's actual capabilities.
type FeatureCapabilities struct {
	Revision   uint16
	Attributes []uint16
	Commands   []uint8
}

// ValidationResult holds the outcome of validating a device against a spec.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// ValidateDevice checks whether a device's capabilities satisfy a spec manifest.
func ValidateDevice(spec *SpecManifest, device DeviceCapabilities) ValidationResult {
	var result ValidationResult

	for featureName, featureSpec := range spec.Features {
		devFeature, present := device.Features[featureName]

		if !present {
			if featureSpec.Mandatory {
				result.Errors = append(result.Errors,
					fmt.Sprintf("mandatory feature %s missing", featureName))
			}
			continue
		}

		// Revision check (warning only).
		if devFeature.Revision != featureSpec.Revision {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("feature %s revision mismatch: device has %d, spec expects %d",
					featureName, devFeature.Revision, featureSpec.Revision))
		}

		// Mandatory attributes.
		attrSet := makeUint16Set(devFeature.Attributes)
		for _, attr := range featureSpec.Attributes.Mandatory {
			if !attrSet[attr.ID] {
				result.Errors = append(result.Errors,
					fmt.Sprintf("feature %s missing mandatory attribute %s (ID %d)",
						featureName, attr.Name, attr.ID))
			}
		}

		// Mandatory commands.
		cmdSet := makeUint8Set(devFeature.Commands)
		for _, cmd := range featureSpec.Commands.Mandatory {
			if !cmdSet[cmd.ID] {
				result.Errors = append(result.Errors,
					fmt.Sprintf("feature %s missing mandatory command %s (ID %d)",
						featureName, cmd.Name, cmd.ID))
			}
		}
	}

	result.Valid = len(result.Errors) == 0
	return result
}

func makeUint16Set(ids []uint16) map[uint16]bool {
	s := make(map[uint16]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

func makeUint8Set(ids []uint8) map[uint8]bool {
	s := make(map[uint8]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}
