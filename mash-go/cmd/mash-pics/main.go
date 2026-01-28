// mash-pics is a CLI tool for PICS validation, linting, and conversion.
package main

import (
	"fmt"
	"os"

	"github.com/mash-protocol/mash-go/cmd/mash-pics/commands"
)

const (
	exitSuccess       = 0
	exitCommandError  = 1
	exitValidation    = 2
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(exitCommandError)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var exitCode int
	switch cmd {
	case "validate":
		exitCode = commands.RunValidate(args, os.Stdout, os.Stderr)
	case "lint":
		exitCode = commands.RunLint(args, os.Stdout, os.Stderr)
	case "show":
		exitCode = commands.RunShow(args, os.Stdout, os.Stderr)
	case "convert":
		exitCode = commands.RunConvert(args, os.Stdout, os.Stderr)
	case "help", "-h", "--help":
		printUsage()
		exitCode = exitSuccess
	case "version", "-v", "--version":
		fmt.Println("mash-pics version 0.1.0")
		exitCode = exitSuccess
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		exitCode = exitCommandError
	}

	os.Exit(exitCode)
}

func printUsage() {
	fmt.Println(`mash-pics - PICS validation and conversion tool

Usage:
  mash-pics <command> [options] [files...]

Commands:
  validate   Validate PICS files against conformance rules
  lint       Check PICS files for style and consistency issues
  show       Display PICS file contents in various formats
  convert    Convert between PICS formats (key-value <-> YAML)

Options:
  -h, --help     Show this help message
  -v, --version  Show version information

Examples:
  mash-pics validate device.pics
  mash-pics lint --verbose *.yaml
  mash-pics show --format json device.pics
  mash-pics convert device.yaml -o device.pics

For command-specific help, run:
  mash-pics <command> --help`)
}
