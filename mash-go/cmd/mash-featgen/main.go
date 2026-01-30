package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
)

func main() {
	featuresDir := flag.String("features", "", "Base directory for feature YAMLs (docs/features/)")
	sharedPath := flag.String("shared", "", "Path to shared types YAML")
	protocolPath := flag.String("protocol", "", "Path to protocol-versions.yaml")
	version := flag.String("version", "1.0", "Protocol version to generate")
	outputDir := flag.String("output", "", "Output directory for generated Go files")
	modelOutput := flag.String("model-output", "", "Output directory for generated model type files")
	specOutput := flag.String("spec-output", "", "Output path for derived spec manifest")
	flag.Parse()

	if *featuresDir == "" || *sharedPath == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "Usage: mash-featgen -features <dir> -shared <path> -output <dir> [-protocol <path>] [-version <ver>] [-model-output <dir>] [-spec-output <path>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := run(*featuresDir, *sharedPath, *protocolPath, *version, *outputDir, *modelOutput, *specOutput); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(featuresDir, sharedPath, protocolPath, version, outputDir, modelOutput, specOutput string) error {
	// Load shared types
	shared, err := LoadSharedTypes(sharedPath)
	if err != nil {
		return fmt.Errorf("loading shared types: %w", err)
	}

	// Load protocol versions to determine which features/versions to generate
	var featureVersions map[string]string
	var specDescription string
	var ver RawProtocolVersion
	if protocolPath != "" {
		pv, err := LoadProtocolVersions(protocolPath)
		if err != nil {
			return fmt.Errorf("loading protocol versions: %w", err)
		}
		var ok bool
		ver, ok = pv.Versions[version]
		if !ok {
			return fmt.Errorf("protocol version %q not found", version)
		}
		featureVersions = ver.Features
		specDescription = ver.Description
	}

	// Generate model type files if model output directory is specified
	if modelOutput != "" && (len(ver.FeatureTypes) > 0 || len(ver.EndpointTypes) > 0) {
		if err := os.MkdirAll(modelOutput, 0o755); err != nil {
			return fmt.Errorf("creating model output dir: %w", err)
		}

		if len(ver.FeatureTypes) > 0 {
			code, err := GenerateFeatureTypes(ver.FeatureTypes)
			if err != nil {
				return fmt.Errorf("generating feature types: %w", err)
			}
			outPath := filepath.Join(modelOutput, "feature_type_gen.go")
			if err := writeFormatted(outPath, code); err != nil {
				return fmt.Errorf("writing feature_type_gen.go: %w", err)
			}
			fmt.Printf("  generated %s\n", outPath)
		}

		if len(ver.EndpointTypes) > 0 {
			code, err := GenerateEndpointTypes(ver.EndpointTypes)
			if err != nil {
				return fmt.Errorf("generating endpoint types: %w", err)
			}
			outPath := filepath.Join(modelOutput, "endpoint_type_gen.go")
			if err := writeFormatted(outPath, code); err != nil {
				return fmt.Errorf("writing endpoint_type_gen.go: %w", err)
			}
			fmt.Printf("  generated %s\n", outPath)
		}
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// Generate shared enums
	sharedCode, err := GenerateSharedEnums(shared)
	if err != nil {
		return fmt.Errorf("generating shared enums: %w", err)
	}
	sharedOutPath := filepath.Join(outputDir, "shared_gen.go")
	if err := writeFormatted(sharedOutPath, sharedCode); err != nil {
		return fmt.Errorf("writing shared_gen.go: %w", err)
	}
	fmt.Printf("  generated %s\n", sharedOutPath)

	// Load and generate each feature
	var allDefs []*RawFeatureDef

	for featureName, featureVer := range featureVersions {
		// Convert feature name to directory name (e.g., "DeviceInfo" -> "device-info")
		dirName := featureDirName(featureName)
		yamlPath := filepath.Join(featuresDir, dirName, featureVer+".yaml")

		def, err := LoadFeatureDef(yamlPath)
		if err != nil {
			return fmt.Errorf("loading feature %s: %w", featureName, err)
		}
		allDefs = append(allDefs, def)

		// Generate the feature code
		code, err := GenerateFeature(def, shared)
		if err != nil {
			return fmt.Errorf("generating feature %s: %w", featureName, err)
		}

		outFileName := featureFileName(featureName) + "_gen.go"
		outPath := filepath.Join(outputDir, outFileName)
		if err := writeFormatted(outPath, code); err != nil {
			return fmt.Errorf("writing %s: %w", outFileName, err)
		}
		fmt.Printf("  generated %s\n", outPath)
	}

	// Validate feature IDs against registry
	if len(ver.FeatureTypes) > 0 && len(allDefs) > 0 {
		if err := ValidateFeatureIDs(ver.FeatureTypes, allDefs); err != nil {
			return err
		}
	}

	// Derive spec manifest if output path is specified
	if specOutput != "" {
		if specDescription == "" {
			specDescription = fmt.Sprintf("MASH Protocol Specification v%s", version)
		}
		manifest, err := DeriveSpecManifest(allDefs, version, specDescription)
		if err != nil {
			return fmt.Errorf("deriving spec manifest: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(specOutput), 0o755); err != nil {
			return fmt.Errorf("creating spec output dir: %w", err)
		}
		if err := os.WriteFile(specOutput, []byte(manifest), 0o644); err != nil {
			return fmt.Errorf("writing spec manifest: %w", err)
		}
		fmt.Printf("  generated %s\n", specOutput)
	}

	return nil
}

// writeFormatted formats Go source code with goimports and writes it to a file.
func writeFormatted(path string, code string) error {
	formatted, err := imports.Process(path, []byte(code), nil)
	if err != nil {
		// Write unformatted so you can debug the generator output
		_ = os.WriteFile(path+".broken", []byte(code), 0o644)
		return fmt.Errorf("goimports %s: %w", filepath.Base(path), err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

// featureDirName converts "DeviceInfo" to "device-info", "EnergyControl" to "energy-control".
func featureDirName(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('-')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// featureFileName converts "DeviceInfo" to "device_info", "EnergyControl" to "energy_control".
func featureFileName(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
