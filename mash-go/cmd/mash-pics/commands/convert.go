package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

// ConvertOptions configures the convert command.
type ConvertOptions struct {
	Input  string
	Output string // Empty means stdout
	Side   string // S or C, for D.* codes without side info
}

// featureGroup groups PICS entries by feature for organized output.
type featureGroup struct {
	name    string
	entries []pics.Entry
}

// RunConvert runs the convert command.
func RunConvert(args []string, stdout, stderr io.Writer) int {
	opts, err := parseConvertArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitCommandError
	}

	if opts.Input == "" {
		fmt.Fprintln(stderr, "Error: no input file specified")
		printConvertUsage(stderr)
		return exitCommandError
	}

	// Parse the input file
	p, err := pics.ParseFile(opts.Input)
	if err != nil {
		fmt.Fprintf(stderr, "Error parsing input: %v\n", err)
		return exitCommandError
	}

	// Convert to key-value format
	output := convertToKeyValue(p, opts.Side)

	// Write output
	if opts.Output == "" || opts.Output == "-" {
		fmt.Fprint(stdout, output)
	} else {
		if err := os.WriteFile(opts.Output, []byte(output), 0644); err != nil {
			fmt.Fprintf(stderr, "Error writing output: %v\n", err)
			return exitCommandError
		}
		fmt.Fprintf(stdout, "Converted %s -> %s\n", opts.Input, opts.Output)
	}

	return exitSuccess
}

func convertToKeyValue(p *pics.PICS, defaultSide string) string {
	var sb strings.Builder

	// Header comments
	sb.WriteString("# MASH PICS File\n")
	if p.Device != nil {
		if p.Device.Vendor != "" || p.Device.Product != "" {
			sb.WriteString(fmt.Sprintf("# Device: %s %s\n", p.Device.Vendor, p.Device.Product))
		}
		if p.Device.Model != "" {
			sb.WriteString(fmt.Sprintf("# Model: %s\n", p.Device.Model))
		}
		if p.Device.Version != "" {
			sb.WriteString(fmt.Sprintf("# Version: %s\n", p.Device.Version))
		}
	}
	sb.WriteString("#\n")
	sb.WriteString("# Converted from: " + p.SourceFile + "\n")
	sb.WriteString("\n")

	// Determine side
	side := string(p.Side)
	if side == "" {
		side = defaultSide
		if side == "" {
			side = "S" // Default to server/device
		}
	}

	// Group entries by feature for organized output
	groups := make(map[string]*featureGroup)
	groupOrder := []string{"", "TRANS", "COMM", "CERT", "ZONE", "CONN", "FAILSAFE", "SUB", "DURATION", "DISC",
		"CTRL", "ELEC", "MEAS", "STAT", "INFO", "CHRG", "SIG", "TAR", "PLAN"}

	for _, entry := range p.Entries {
		feature := entry.Code.Feature
		if groups[feature] == nil {
			groups[feature] = &featureGroup{name: feature}
		}
		groups[feature].entries = append(groups[feature].entries, entry)
	}

	// Sort entries within each group
	for _, g := range groups {
		sort.Slice(g.entries, func(i, j int) bool {
			return g.entries[i].Code.String() < g.entries[j].Code.String()
		})
	}

	// Write protocol section first
	sb.WriteString("# Protocol Support\n")
	if p.Has("MASH.S") || p.Side == pics.SideServer {
		sb.WriteString("MASH.S=1\n")
	}
	if p.Has("MASH.C") || p.Side == pics.SideClient {
		sb.WriteString("MASH.C=1\n")
	}
	if p.Version > 0 {
		sb.WriteString(fmt.Sprintf("MASH.%s.VERSION=%d\n", side, p.Version))
	}
	sb.WriteString("\n")

	// Write features section
	if len(p.Features) > 0 {
		sb.WriteString("# Features\n")
		for _, feature := range p.Features {
			sb.WriteString(fmt.Sprintf("MASH.%s.%s=1\n", side, feature))
		}
		sb.WriteString("\n")
	}

	// Write each feature group
	written := make(map[string]bool)
	for _, featureName := range groupOrder {
		g, ok := groups[featureName]
		if !ok || featureName == "" {
			continue
		}
		written[featureName] = true
		writeFeatureGroup(&sb, g, side)
	}

	// Write any remaining groups not in the predefined order
	var remaining []string
	for name := range groups {
		if !written[name] && name != "" {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		writeFeatureGroup(&sb, groups[name], side)
	}

	return sb.String()
}

func writeFeatureGroup(sb *strings.Builder, g *featureGroup, side string) {
	if len(g.entries) == 0 {
		return
	}

	sb.WriteString(fmt.Sprintf("# %s\n", g.name))

	// Separate by type
	var flags, attrs, commands, behaviors, others []pics.Entry
	for _, e := range g.entries {
		switch e.Code.Type {
		case pics.CodeTypeFlag:
			flags = append(flags, e)
		case pics.CodeTypeAttribute:
			attrs = append(attrs, e)
		case pics.CodeTypeCommand:
			commands = append(commands, e)
		case pics.CodeTypeBehavior:
			behaviors = append(behaviors, e)
		default:
			others = append(others, e)
		}
	}

	writeEntries := func(entries []pics.Entry, comment string) {
		if len(entries) == 0 {
			return
		}
		if comment != "" {
			sb.WriteString(fmt.Sprintf("# %s %s\n", g.name, comment))
		}
		for _, e := range entries {
			writeEntry(sb, e)
		}
	}

	writeEntries(flags, "Feature Flags")
	writeEntries(attrs, "Attributes")
	writeEntries(commands, "Commands")
	writeEntries(behaviors, "Behavior")
	writeEntries(others, "")

	sb.WriteString("\n")
}

func writeEntry(sb *strings.Builder, e pics.Entry) {
	code := e.Code.String()
	value := e.Value.Raw

	// Quote string values if needed
	if !e.Value.IsBool() && e.Value.Int == 0 && value != "0" {
		// It's a string value, check if it needs quotes
		if strings.Contains(value, " ") || strings.Contains(value, "=") {
			value = fmt.Sprintf("\"%s\"", value)
		}
	}

	sb.WriteString(fmt.Sprintf("%s=%s\n", code, value))
}

func parseConvertArgs(args []string) (ConvertOptions, error) {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	opts := ConvertOptions{}

	fs.StringVar(&opts.Output, "o", "", "Output file (default: stdout)")
	fs.StringVar(&opts.Output, "output", "", "Output file")
	fs.StringVar(&opts.Side, "side", "S", "Default side for D.* codes (S or C)")

	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	remaining := fs.Args()
	if len(remaining) > 0 {
		opts.Input = remaining[0]
	}

	return opts, nil
}

func printConvertUsage(w io.Writer) {
	fmt.Fprintln(w, `
Usage: mash-pics convert [options] <input-file>

Options:
  -o, --output   Output file (default: stdout)
  --side         Default side for D.* codes (S or C) [default: S]

Examples:
  mash-pics convert ev-charger.yaml -o ev-charger.pics
  mash-pics convert device.yaml > device.pics
  mash-pics convert --side C controller.yaml -o controller.pics`)
}
