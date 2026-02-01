package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/specparse"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

// DocModel holds the cross-referenced documentation data.
type DocModel struct {
	Version            string
	Features           []*specparse.RawFeatureDef   // sorted by ID
	SharedTypes        *specparse.RawSharedTypes
	UseCases           []*usecase.RawUseCaseDef     // sorted by ID
	EndpointTypes      []specparse.RawModelTypeDef
	FeatureTypes       []specparse.RawModelTypeDef
	UseCaseTypes       []specparse.RawModelTypeDef
	Conformance        *specparse.RawEndpointConformance
	FeatureByName      map[string]*specparse.RawFeatureDef
	FeatureUseCaseRefs map[string][]string // feature name -> []use case names
}

// BuildDocModel loads all specification data and builds the cross-referenced model.
func BuildDocModel(docsRoot, version string) (*DocModel, error) {
	featuresDir := filepath.Join(docsRoot, "features")

	// Load protocol versions
	pvPath := filepath.Join(featuresDir, "protocol-versions.yaml")
	pv, err := specparse.LoadProtocolVersions(pvPath)
	if err != nil {
		return nil, fmt.Errorf("loading protocol versions: %w", err)
	}

	ver, ok := pv.Versions[version]
	if !ok {
		return nil, fmt.Errorf("protocol version %q not found", version)
	}

	// Load shared types
	sharedPath := filepath.Join(featuresDir, "_shared", ver.Shared+".yaml")
	shared, err := specparse.LoadSharedTypes(sharedPath)
	if err != nil {
		return nil, fmt.Errorf("loading shared types: %w", err)
	}

	// Load all features
	var features []*specparse.RawFeatureDef
	featureByName := make(map[string]*specparse.RawFeatureDef)

	for featureName, featureVer := range ver.Features {
		dirName := specparse.FeatureDirName(featureName)
		yamlPath := filepath.Join(featuresDir, dirName, featureVer+".yaml")
		def, err := specparse.LoadFeatureDef(yamlPath)
		if err != nil {
			return nil, fmt.Errorf("loading feature %s: %w", featureName, err)
		}
		features = append(features, def)
		featureByName[def.Name] = def
	}

	sort.Slice(features, func(i, j int) bool {
		return features[i].ID < features[j].ID
	})

	// Load use cases
	usecasesDir := filepath.Join(docsRoot, "usecases", version)
	var useCases []*usecase.RawUseCaseDef

	for ucName := range ver.UseCases {
		yamlPath := filepath.Join(usecasesDir, strings.ToLower(ucName)+".yaml")
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			return nil, fmt.Errorf("loading use case %s: %w", ucName, err)
		}
		ucDef, err := usecase.ParseRawUseCaseDef(data)
		if err != nil {
			return nil, fmt.Errorf("parsing use case %s: %w", ucName, err)
		}
		useCases = append(useCases, ucDef)
	}

	sort.Slice(useCases, func(i, j int) bool {
		return useCases[i].ID < useCases[j].ID
	})

	// Load endpoint conformance
	confPath := filepath.Join(featuresDir, "endpoint-conformance.yaml")
	var conformance *specparse.RawEndpointConformance
	if _, err := os.Stat(confPath); err == nil {
		conformance, err = specparse.LoadEndpointConformance(confPath)
		if err != nil {
			return nil, fmt.Errorf("loading endpoint conformance: %w", err)
		}
	}

	// Build cross-reference: feature name -> use case names
	featureUseCaseRefs := make(map[string][]string)
	for _, uc := range useCases {
		seen := make(map[string]bool)
		for _, sc := range uc.Scenarios {
			for _, fr := range sc.Features {
				if !seen[fr.Feature] {
					featureUseCaseRefs[fr.Feature] = append(featureUseCaseRefs[fr.Feature], uc.Name)
					seen[fr.Feature] = true
				}
			}
		}
		// Also check legacy flat features
		for _, fr := range uc.Features {
			if !seen[fr.Feature] {
				featureUseCaseRefs[fr.Feature] = append(featureUseCaseRefs[fr.Feature], uc.Name)
				seen[fr.Feature] = true
			}
		}
	}

	return &DocModel{
		Version:            version,
		Features:           features,
		SharedTypes:        shared,
		UseCases:           useCases,
		EndpointTypes:      ver.EndpointTypes,
		FeatureTypes:       ver.FeatureTypes,
		UseCaseTypes:       ver.UseCaseTypes,
		Conformance:        conformance,
		FeatureByName:      featureByName,
		FeatureUseCaseRefs: featureUseCaseRefs,
	}, nil
}
