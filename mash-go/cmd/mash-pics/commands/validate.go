package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/mash-protocol/mash-go/pkg/pics"
	"github.com/mash-protocol/mash-go/pkg/pics/rules"
)

const (
	exitSuccess      = 0
	exitCommandError = 1
	exitValidation   = 2
)

// ValidateOptions configures the validate command.
type ValidateOptions struct {
	Strict  bool
	JSON    bool
	Verbose bool
	Files   []string
}

// RunValidate runs the validate command.
func RunValidate(args []string, stdout, stderr io.Writer) int {
	opts, err := parseValidateArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitCommandError
	}

	if len(opts.Files) == 0 {
		fmt.Fprintln(stderr, "Error: no files specified")
		printValidateUsage(stderr)
		return exitCommandError
	}

	// Create rule registry
	registry := rules.NewDefaultRegistry()

	hasErrors := false
	results := make(map[string]*ValidationOutput)

	for _, file := range opts.Files {
		result := validateFile(file, registry, opts)
		results[file] = result

		if !result.Valid {
			hasErrors = true
		}

		if !opts.JSON {
			printValidationResult(stdout, file, result, opts.Verbose)
		}
	}

	if opts.JSON {
		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(output))
	}

	if hasErrors {
		return exitValidation
	}
	return exitSuccess
}

// ValidationOutput represents the validation result for a file.
type ValidationOutput struct {
	Valid    bool            `json:"valid"`
	Format   string          `json:"format,omitempty"`
	Errors   []IssueOutput   `json:"errors,omitempty"`
	Warnings []IssueOutput   `json:"warnings,omitempty"`
	Device   *DeviceOutput   `json:"device,omitempty"`
}

// IssueOutput represents a validation issue.
type IssueOutput struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Line       int      `json:"line,omitempty"`
	PICSCodes  []string `json:"pics_codes,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// DeviceOutput represents device metadata.
type DeviceOutput struct {
	Vendor  string `json:"vendor,omitempty"`
	Product string `json:"product,omitempty"`
	Model   string `json:"model,omitempty"`
	Version string `json:"version,omitempty"`
}

func validateFile(path string, registry *pics.RuleRegistry, opts ValidateOptions) *ValidationOutput {
	output := &ValidationOutput{Valid: true}

	// Parse the file
	p, err := pics.ParseFile(path)
	if err != nil {
		output.Valid = false
		output.Errors = append(output.Errors, IssueOutput{
			Code:    "PARSE",
			Message: err.Error(),
		})
		return output
	}

	output.Format = p.Format.String()

	// Add device metadata if present
	if p.Device != nil {
		output.Device = &DeviceOutput{
			Vendor:  p.Device.Vendor,
			Product: p.Device.Product,
			Model:   p.Device.Model,
			Version: p.Device.Version,
		}
	}

	// Run validation rules
	validateOpts := pics.ValidateOptions{
		Registry:    registry,
		MinSeverity: pics.SeverityWarning,
	}
	if opts.Strict {
		validateOpts.MinSeverity = pics.SeverityInfo
	}

	result := pics.NewValidator().ValidateWithOptions(p, validateOpts)

	output.Valid = result.Valid

	for _, e := range result.Errors {
		output.Errors = append(output.Errors, IssueOutput{
			Code:    e.Code,
			Message: e.Message,
			Line:    e.Line,
		})
	}

	for _, w := range result.Warnings {
		output.Warnings = append(output.Warnings, IssueOutput{
			Code:    w.Code,
			Message: w.Message,
			Line:    w.Line,
		})
	}

	return output
}

func printValidationResult(w io.Writer, file string, result *ValidationOutput, verbose bool) {
	if result.Valid && len(result.Errors) == 0 && len(result.Warnings) == 0 {
		fmt.Fprintf(w, "%s: OK\n", file)
		return
	}

	if result.Valid && len(result.Warnings) > 0 {
		fmt.Fprintf(w, "%s: OK (with %d warnings)\n", file, len(result.Warnings))
	} else if !result.Valid {
		fmt.Fprintf(w, "%s: FAILED (%d errors, %d warnings)\n", file, len(result.Errors), len(result.Warnings))
	}

	if verbose || !result.Valid {
		for _, e := range result.Errors {
			if e.Line > 0 {
				fmt.Fprintf(w, "  ERROR [line %d] %s: %s\n", e.Line, e.Code, e.Message)
			} else {
				fmt.Fprintf(w, "  ERROR %s: %s\n", e.Code, e.Message)
			}
		}
	}

	if verbose {
		for _, warn := range result.Warnings {
			if warn.Line > 0 {
				fmt.Fprintf(w, "  WARNING [line %d] %s: %s\n", warn.Line, warn.Code, warn.Message)
			} else {
				fmt.Fprintf(w, "  WARNING %s: %s\n", warn.Code, warn.Message)
			}
		}
	}
}

func parseValidateArgs(args []string) (ValidateOptions, error) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	opts := ValidateOptions{}

	fs.BoolVar(&opts.Strict, "strict", false, "Enable strict validation mode")
	fs.BoolVar(&opts.JSON, "json", false, "Output results as JSON")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Show all warnings")
	fs.BoolVar(&opts.Verbose, "v", false, "Show all warnings (shorthand)")

	// Handle --help
	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return opts, err
		}
		return opts, err
	}

	opts.Files = fs.Args()
	return opts, nil
}

func printValidateUsage(w io.Writer) {
	fmt.Fprintln(w, `
Usage: mash-pics validate [options] <files...>

Options:
  --strict     Enable strict validation (all rules as errors)
  --json       Output results as JSON
  -v, --verbose  Show all warnings

Examples:
  mash-pics validate device.pics
  mash-pics validate --strict --json *.yaml`)
}
