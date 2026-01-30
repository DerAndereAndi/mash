package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
	"gopkg.in/yaml.v3"
)

// ShowOptions configures the show command.
type ShowOptions struct {
	Format  string // text, json, yaml
	Feature string // filter by feature
	GroupBy string // feature, type, none
	File    string
}

// ShowOutput represents the PICS data for display.
type ShowOutput struct {
	File     string                 `json:"file,omitempty" yaml:"file,omitempty"`
	Format   string                 `json:"format,omitempty" yaml:"format,omitempty"`
	Side     string                 `json:"side,omitempty" yaml:"side,omitempty"`
	Version  string                 `json:"version,omitempty" yaml:"version,omitempty"`
	Device   *DeviceOutput          `json:"device,omitempty" yaml:"device,omitempty"`
	Features []string               `json:"features,omitempty" yaml:"features,omitempty"`
	Entries  []EntryOutput          `json:"entries,omitempty" yaml:"entries,omitempty"`
	Grouped  map[string][]EntryOutput `json:"grouped,omitempty" yaml:"grouped,omitempty"`
}

// EntryOutput represents a single PICS entry.
type EntryOutput struct {
	Code    string `json:"code" yaml:"code"`
	Value   string `json:"value" yaml:"value"`
	Line    int    `json:"line,omitempty" yaml:"line,omitempty"`
	Feature string `json:"feature,omitempty" yaml:"feature,omitempty"`
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
}

// RunShow runs the show command.
func RunShow(args []string, stdout, stderr io.Writer) int {
	opts, err := parseShowArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitCommandError
	}

	if opts.File == "" {
		fmt.Fprintln(stderr, "Error: no file specified")
		printShowUsage(stderr)
		return exitCommandError
	}

	// Parse the file
	p, err := pics.ParseFile(opts.File)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitCommandError
	}

	output := buildShowOutput(p, opts)

	switch opts.Format {
	case "json":
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Fprintln(stdout, string(data))
	case "yaml":
		data, _ := yaml.Marshal(output)
		fmt.Fprintln(stdout, string(data))
	default:
		printShowText(stdout, output, opts)
	}

	return exitSuccess
}

func buildShowOutput(p *pics.PICS, opts ShowOptions) ShowOutput {
	output := ShowOutput{
		File:     p.SourceFile,
		Format:   p.Format.String(),
		Side:     string(p.Side),
		Version:  p.Version,
		Features: p.Features,
	}

	if p.Device != nil {
		output.Device = &DeviceOutput{
			Vendor:  p.Device.Vendor,
			Product: p.Device.Product,
			Model:   p.Device.Model,
			Version: p.Device.Version,
		}
	}

	// Build entries
	var entries []EntryOutput
	for _, e := range p.Entries {
		// Filter by feature if specified
		if opts.Feature != "" && e.Code.Feature != opts.Feature {
			continue
		}

		entryType := ""
		switch e.Code.Type {
		case pics.CodeTypeAttribute:
			entryType = "attribute"
		case pics.CodeTypeCommand:
			entryType = "command"
		case pics.CodeTypeFlag:
			entryType = "flag"
		case pics.CodeTypeEvent:
			entryType = "event"
		case pics.CodeTypeBehavior:
			entryType = "behavior"
		}

		entries = append(entries, EntryOutput{
			Code:    e.Code.String(),
			Value:   e.Value.Raw,
			Line:    e.LineNumber,
			Feature: e.Code.Feature,
			Type:    entryType,
		})
	}

	// Sort by code
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Code < entries[j].Code
	})

	// Group if requested
	if opts.GroupBy != "" && opts.GroupBy != "none" {
		output.Grouped = make(map[string][]EntryOutput)
		for _, e := range entries {
			var key string
			switch opts.GroupBy {
			case "feature":
				key = e.Feature
				if key == "" {
					key = "(protocol)"
				}
			case "type":
				key = e.Type
				if key == "" {
					key = "(feature)"
				}
			}
			output.Grouped[key] = append(output.Grouped[key], e)
		}
	} else {
		output.Entries = entries
	}

	return output
}

func printShowText(w io.Writer, output ShowOutput, opts ShowOptions) {
	// Header
	fmt.Fprintf(w, "File: %s\n", output.File)
	fmt.Fprintf(w, "Format: %s\n", output.Format)
	fmt.Fprintf(w, "Side: %s\n", output.Side)
	if output.Version != "" {
		fmt.Fprintf(w, "Version: %s\n", output.Version)
	}

	// Device info
	if output.Device != nil {
		fmt.Fprintln(w, "\nDevice:")
		if output.Device.Vendor != "" {
			fmt.Fprintf(w, "  Vendor: %s\n", output.Device.Vendor)
		}
		if output.Device.Product != "" {
			fmt.Fprintf(w, "  Product: %s\n", output.Device.Product)
		}
		if output.Device.Model != "" {
			fmt.Fprintf(w, "  Model: %s\n", output.Device.Model)
		}
		if output.Device.Version != "" {
			fmt.Fprintf(w, "  Version: %s\n", output.Device.Version)
		}
	}

	// Features
	if len(output.Features) > 0 {
		fmt.Fprintf(w, "\nFeatures: %s\n", strings.Join(output.Features, ", "))
	}

	// Entries
	fmt.Fprintln(w, "\nEntries:")
	if output.Grouped != nil {
		// Get sorted keys
		var keys []string
		for k := range output.Grouped {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			fmt.Fprintf(w, "\n  [%s]\n", key)
			for _, e := range output.Grouped[key] {
				fmt.Fprintf(w, "    %s = %s\n", e.Code, e.Value)
			}
		}
	} else {
		for _, e := range output.Entries {
			fmt.Fprintf(w, "  %s = %s\n", e.Code, e.Value)
		}
	}

	fmt.Fprintf(w, "\nTotal: %d entries\n", countEntries(output))
}

func countEntries(output ShowOutput) int {
	if output.Grouped != nil {
		count := 0
		for _, entries := range output.Grouped {
			count += len(entries)
		}
		return count
	}
	return len(output.Entries)
}

func parseShowArgs(args []string) (ShowOptions, error) {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	opts := ShowOptions{}

	fs.StringVar(&opts.Format, "format", "text", "Output format (text, json, yaml)")
	fs.StringVar(&opts.Format, "f", "text", "Output format (shorthand)")
	fs.StringVar(&opts.Feature, "feature", "", "Filter by feature")
	fs.StringVar(&opts.GroupBy, "group-by", "", "Group entries (feature, type, none)")

	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	remaining := fs.Args()
	if len(remaining) > 0 {
		opts.File = remaining[0]
	}

	return opts, nil
}

func printShowUsage(w io.Writer) {
	fmt.Fprintln(w, `
Usage: mash-pics show [options] <file>

Options:
  -f, --format    Output format (text, json, yaml) [default: text]
  --feature       Filter by feature name
  --group-by      Group entries by (feature, type, none)

Examples:
  mash-pics show device.pics
  mash-pics show --format json device.yaml
  mash-pics show --feature CTRL device.pics
  mash-pics show --group-by feature device.pics`)
}
