package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/specparse"
)

// DeriveSpecManifest produces the spec manifest YAML from parsed feature definitions.
// Features are sorted by ID. The output matches the structure of pkg/version/specs/1.0.yaml.
func DeriveSpecManifest(features []*specparse.RawFeatureDef, version, description string) (string, error) {
	// Sort features by ID
	sorted := make([]*specparse.RawFeatureDef, len(features))
	copy(sorted, features)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	var b strings.Builder

	fmt.Fprintf(&b, "version: %q\n", version)
	fmt.Fprintf(&b, "description: %q\n", description)
	b.WriteString("\nfeatures:\n")

	for _, def := range sorted {
		writeFeatureSpec(&b, def)
	}

	return b.String(), nil
}

func writeFeatureSpec(b *strings.Builder, def *specparse.RawFeatureDef) {
	fmt.Fprintf(b, "  %s:\n", def.Name)
	fmt.Fprintf(b, "    id: 0x%02X\n", def.ID)
	fmt.Fprintf(b, "    revision: %d\n", def.Revision)
	fmt.Fprintf(b, "    mandatory: %v\n", def.Mandatory)

	// Partition attributes into mandatory and optional
	var mandatory, optional []specparse.RawAttributeDef
	for _, attr := range def.Attributes {
		if attr.Mandatory {
			mandatory = append(mandatory, attr)
		} else {
			optional = append(optional, attr)
		}
	}

	if len(mandatory) > 0 || len(optional) > 0 {
		b.WriteString("    attributes:\n")
		if len(mandatory) > 0 {
			b.WriteString("      mandatory:\n")
			for _, attr := range mandatory {
				fmt.Fprintf(b, "        - { id: %s, name: %s }\n", formatAttrID(attr.ID), attr.Name)
			}
		}
		if len(optional) > 0 {
			b.WriteString("      optional:\n")
			for _, attr := range optional {
				fmt.Fprintf(b, "        - { id: %s, name: %s }\n", formatAttrID(attr.ID), attr.Name)
			}
		}
	}

	// Partition commands into mandatory and optional
	var mandatoryCmds, optionalCmds []specparse.RawCommandDef
	for _, cmd := range def.Commands {
		if cmd.Mandatory {
			mandatoryCmds = append(mandatoryCmds, cmd)
		} else {
			optionalCmds = append(optionalCmds, cmd)
		}
	}

	if len(mandatoryCmds) > 0 || len(optionalCmds) > 0 {
		b.WriteString("    commands:\n")
		if len(mandatoryCmds) > 0 {
			b.WriteString("      mandatory:\n")
			for _, cmd := range mandatoryCmds {
				fmt.Fprintf(b, "        - { id: %s, name: %s }\n", formatCmdID(cmd.ID), cmd.Name)
			}
		}
		if len(optionalCmds) > 0 {
			b.WriteString("      optional:\n")
			for _, cmd := range optionalCmds {
				fmt.Fprintf(b, "        - { id: %s, name: %s }\n", formatCmdID(cmd.ID), cmd.Name)
			}
		}
	}

	b.WriteString("\n")
}

// formatAttrID formats an attribute ID, using plain decimal for small IDs
// but keeping the same style as the existing spec manifest.
func formatAttrID(id uint16) string {
	return fmt.Sprintf("%d", id)
}

// formatCmdID formats a command ID using hex if >= 16, decimal otherwise.
func formatCmdID(id uint8) string {
	if id >= 16 {
		return fmt.Sprintf("0x%02X", id)
	}
	return fmt.Sprintf("%d", id)
}
