// Command mash-log is a tool for viewing and analyzing MASH protocol log files.
//
// Log files are created using the protocol logging infrastructure when running
// mash-device, mash-controller, or mash-test with the -protocol-log flag.
//
// Usage:
//
//	mash-log <command> [flags] <file.mlog>
//
// Commands:
//
//	view     View log file in human-readable format
//	export   Export log file to JSON or CSV format
//	filter   Filter log file and write to new file
//	stats    Show statistics about the log file
//
// Examples:
//
//	# View all events
//	mash-log view device.mlog
//
//	# View only wire-layer events
//	mash-log view --layer wire device.mlog
//
//	# View only outgoing messages
//	mash-log view --direction out device.mlog
//
//	# Export to JSONL
//	mash-log export --format jsonl device.mlog
//
//	# Filter by connection and save to new file
//	mash-log filter --conn-id abc12345 -o filtered.mlog device.mlog
//
//	# Show statistics
//	mash-log stats device.mlog
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mash-protocol/mash-go/cmd/mash-log/commands"
)

const usage = `mash-log - MASH Protocol Log Analyzer

Usage:
  mash-log <command> [flags] <file.mlog>

Commands:
  view     View log file in human-readable format
  export   Export log file to JSON or CSV format
  filter   Filter log file and write to new file
  stats    Show statistics about the log file

Use "mash-log <command> -help" for more information about a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "view":
		runView(args)
	case "export":
		runExport(args)
	case "filter":
		runFilter(args)
	case "stats":
		runStats(args)
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func runView(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `mash-log view - View log file in human-readable format

Usage:
  mash-log view [flags] <file.mlog>

Flags:
`)
		fs.PrintDefaults()
	}

	layer := fs.String("layer", "", "Filter by layer (transport, wire, service)")
	direction := fs.String("direction", "", "Filter by direction (in, out)")
	category := fs.String("category", "", "Filter by category (message, control, state, error)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: log file path required")
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Build filter
	var filter commands.ViewFilter

	if *layer != "" {
		l, err := commands.ParseLayerFlag(*layer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		filter.Layer = &l
	}

	if *direction != "" {
		d, err := commands.ParseDirectionFlag(*direction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		filter.Direction = &d
	}

	if *category != "" {
		c, err := commands.ParseCategoryFlag(*category)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		filter.Category = &c
	}

	if err := commands.RunView(path, filter, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `mash-log export - Export log file to JSON or CSV format

Usage:
  mash-log export [flags] <file.mlog>

Flags:
`)
		fs.PrintDefaults()
	}

	format := fs.String("format", "jsonl", "Output format (jsonl, csv)")
	output := fs.String("o", "", "Output file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: log file path required")
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	if err := commands.RunExport(path, *format, *output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runFilter(args []string) {
	fs := flag.NewFlagSet("filter", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `mash-log filter - Filter log file and write to new file

Usage:
  mash-log filter [flags] <file.mlog>

Flags:
`)
		fs.PrintDefaults()
	}

	output := fs.String("o", "", "Output file (required)")
	connID := fs.String("conn-id", "", "Filter by connection ID")
	deviceID := fs.String("device-id", "", "Filter by device ID")
	zoneID := fs.String("zone-id", "", "Filter by zone ID")
	timeStart := fs.String("time-start", "", "Filter by start time (RFC3339)")
	timeEnd := fs.String("time-end", "", "Filter by end time (RFC3339)")
	layer := fs.String("layer", "", "Filter by layer (transport, wire, service)")
	direction := fs.String("direction", "", "Filter by direction (in, out)")
	category := fs.String("category", "", "Filter by category (message, control, state, error)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: log file path required")
		fs.Usage()
		os.Exit(1)
	}

	if *output == "" {
		fmt.Fprintln(os.Stderr, "Error: output file (-o) required")
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	opts := commands.FilterOptions{
		Output:    *output,
		ConnID:    *connID,
		DeviceID:  *deviceID,
		ZoneID:    *zoneID,
		TimeStart: *timeStart,
		TimeEnd:   *timeEnd,
		Layer:     *layer,
		Direction: *direction,
		Category:  *category,
	}

	if err := commands.RunFilter(path, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `mash-log stats - Show statistics about the log file

Usage:
  mash-log stats <file.mlog>

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: log file path required")
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	if err := commands.RunStats(path, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
