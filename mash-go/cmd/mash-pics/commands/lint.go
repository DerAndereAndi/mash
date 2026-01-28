package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
	"github.com/mash-protocol/mash-go/pkg/pics/rules"
)

// LintOptions configures the lint command.
type LintOptions struct {
	JSON    bool
	Verbose bool
	Files   []string
}

// LintIssue represents a single lint issue.
type LintIssue struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Line       int    `json:"line,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// LintOutput represents the lint results for a file.
type LintOutput struct {
	File   string      `json:"file"`
	Issues []LintIssue `json:"issues"`
	Clean  bool        `json:"clean"`
}

// RunLint runs the lint command.
func RunLint(args []string, stdout, stderr io.Writer) int {
	opts, err := parseLintArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitCommandError
	}

	if len(opts.Files) == 0 {
		fmt.Fprintln(stderr, "Error: no files specified")
		printLintUsage(stderr)
		return exitCommandError
	}

	results := make([]LintOutput, 0, len(opts.Files))
	hasIssues := false

	for _, file := range opts.Files {
		output := lintFile(file, opts)
		results = append(results, output)

		if !output.Clean {
			hasIssues = true
		}

		if !opts.JSON {
			printLintResult(stdout, output, opts.Verbose)
		}
	}

	if opts.JSON {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(out))
	}

	if hasIssues {
		return exitValidation
	}
	return exitSuccess
}

func lintFile(path string, opts LintOptions) LintOutput {
	output := LintOutput{File: path, Clean: true}

	// Parse the file
	p, err := pics.ParseFile(path)
	if err != nil {
		output.Clean = false
		output.Issues = append(output.Issues, LintIssue{
			Code:     "PARSE",
			Severity: "error",
			Message:  err.Error(),
		})
		return output
	}

	// Run validation rules (errors only)
	registry := rules.NewDefaultRegistry()
	violations := registry.RunRules(p)

	for _, v := range violations {
		severity := "info"
		switch v.Severity {
		case pics.SeverityError:
			severity = "error"
			output.Clean = false
		case pics.SeverityWarning:
			severity = "warning"
		}

		line := 0
		if len(v.LineNumbers) > 0 {
			line = v.LineNumbers[0]
		}

		output.Issues = append(output.Issues, LintIssue{
			Code:       v.RuleID,
			Severity:   severity,
			Message:    v.Message,
			Line:       line,
			Suggestion: v.Suggestion,
		})
	}

	// Additional lint checks (beyond validation rules)

	// Check for missing VERSION declaration
	if !p.Has("MASH.S.VERSION") && !p.Has("MASH.C.VERSION") {
		output.Issues = append(output.Issues, LintIssue{
			Code:       "LINT-VERSION",
			Severity:   "suggestion",
			Message:    "Missing VERSION declaration",
			Suggestion: "Add MASH.S.VERSION=1 or MASH.C.VERSION=1",
		})
	}

	// Check for duplicate entries (by scanning ByCode map vs Entries count)
	codeCount := make(map[string]int)
	for _, entry := range p.Entries {
		codeCount[entry.Code.String()]++
	}
	for code, count := range codeCount {
		if count > 1 {
			output.Issues = append(output.Issues, LintIssue{
				Code:       "LINT-DUPLICATE",
				Severity:   "warning",
				Message:    fmt.Sprintf("Duplicate entry: %s (appears %d times)", code, count),
				Suggestion: "Remove duplicate declarations",
			})
			output.Clean = false
		}
	}

	// Check for empty features (declared but no attributes)
	featureAttrs := make(map[string]int)
	for _, entry := range p.Entries {
		if entry.Code.Feature != "" && entry.Code.Type != "" {
			featureAttrs[entry.Code.Feature]++
		}
	}
	for _, feature := range p.Features {
		if featureAttrs[feature] == 0 {
			output.Issues = append(output.Issues, LintIssue{
				Code:       "LINT-EMPTY",
				Severity:   "suggestion",
				Message:    fmt.Sprintf("Feature %s declared but has no attributes", feature),
				Suggestion: "Add attributes or remove the feature declaration",
			})
		}
	}

	// Sort issues by severity, then by line number
	sort.Slice(output.Issues, func(i, j int) bool {
		if output.Issues[i].Severity != output.Issues[j].Severity {
			return severityOrder(output.Issues[i].Severity) < severityOrder(output.Issues[j].Severity)
		}
		return output.Issues[i].Line < output.Issues[j].Line
	})

	return output
}

func severityOrder(s string) int {
	switch s {
	case "error":
		return 0
	case "warning":
		return 1
	case "suggestion":
		return 2
	default:
		return 3
	}
}

func printLintResult(w io.Writer, output LintOutput, verbose bool) {
	if output.Clean && len(output.Issues) == 0 {
		fmt.Fprintf(w, "%s: clean\n", output.File)
		return
	}

	// Count by severity
	errors := 0
	warnings := 0
	suggestions := 0
	for _, issue := range output.Issues {
		switch issue.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		case "suggestion":
			suggestions++
		}
	}

	var parts []string
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", errors))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warnings", warnings))
	}
	if suggestions > 0 {
		parts = append(parts, fmt.Sprintf("%d suggestions", suggestions))
	}

	fmt.Fprintf(w, "%s: %s\n", output.File, strings.Join(parts, ", "))

	for _, issue := range output.Issues {
		if !verbose && issue.Severity == "suggestion" {
			continue
		}

		prefix := strings.ToUpper(issue.Severity)
		if issue.Line > 0 {
			fmt.Fprintf(w, "  %s [line %d] %s: %s\n", prefix, issue.Line, issue.Code, issue.Message)
		} else {
			fmt.Fprintf(w, "  %s %s: %s\n", prefix, issue.Code, issue.Message)
		}
		if verbose && issue.Suggestion != "" {
			fmt.Fprintf(w, "    -> %s\n", issue.Suggestion)
		}
	}
}

func parseLintArgs(args []string) (LintOptions, error) {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	opts := LintOptions{}

	fs.BoolVar(&opts.JSON, "json", false, "Output results as JSON")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Show all issues including suggestions")
	fs.BoolVar(&opts.Verbose, "v", false, "Show all issues (shorthand)")

	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	opts.Files = fs.Args()
	return opts, nil
}

func printLintUsage(w io.Writer) {
	fmt.Fprintln(w, `
Usage: mash-pics lint [options] <files...>

Options:
  --json       Output results as JSON
  -v, --verbose  Show all issues including suggestions

Examples:
  mash-pics lint device.pics
  mash-pics lint --verbose *.yaml`)
}
