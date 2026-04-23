package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/internal/specparse"
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

// specItem is a normalized attribute-or-command row for spec-manifest writing.
type specItem struct {
	ID        string
	Name      string
	Mandatory bool
}

func writeFeatureSpec(b *strings.Builder, def *specparse.RawFeatureDef) {
	fmt.Fprintf(b, "  %s:\n", def.Name)
	fmt.Fprintf(b, "    id: 0x%02X\n", def.ID)
	fmt.Fprintf(b, "    revision: %d\n", def.Revision)
	fmt.Fprintf(b, "    mandatory: %v\n", def.Mandatory)

	attrs := make([]specItem, len(def.Attributes))
	for i, a := range def.Attributes {
		attrs[i] = specItem{ID: formatAttrID(a.ID), Name: a.Name, Mandatory: a.Mandatory}
	}
	writeSpecItems(b, "attributes", attrs)

	cmds := make([]specItem, len(def.Commands))
	for i, c := range def.Commands {
		cmds[i] = specItem{ID: formatCmdID(c.ID), Name: c.Name, Mandatory: c.Mandatory}
	}
	writeSpecItems(b, "commands", cmds)

	b.WriteString("\n")
}

// writeSpecItems emits a "label:" block partitioning items into mandatory/optional
// sub-blocks. Skips the label entirely when items is empty.
func writeSpecItems(b *strings.Builder, label string, items []specItem) {
	var mandatory, optional []specItem
	for _, it := range items {
		if it.Mandatory {
			mandatory = append(mandatory, it)
		} else {
			optional = append(optional, it)
		}
	}
	if len(mandatory) == 0 && len(optional) == 0 {
		return
	}
	fmt.Fprintf(b, "    %s:\n", label)
	writeSpecSection(b, "mandatory", mandatory)
	writeSpecSection(b, "optional", optional)
}

func writeSpecSection(b *strings.Builder, header string, items []specItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "      %s:\n", header)
	for _, it := range items {
		fmt.Fprintf(b, "        - { id: %s, name: %s }\n", it.ID, it.Name)
	}
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
