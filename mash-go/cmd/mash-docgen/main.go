package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	docsDir := flag.String("docs", "", "Root docs directory")
	outputDir := flag.String("output", "", "Output directory for generated Markdown")
	version := flag.String("version", "1.0", "Protocol version to generate")
	flag.Parse()

	if *docsDir == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "Usage: mash-docgen -docs <dir> -output <dir> [-version <ver>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := run(*docsDir, *outputDir, *version); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(docsDir, outputDir, version string) error {
	model, err := BuildDocModel(docsDir, version)
	if err != nil {
		return fmt.Errorf("building doc model: %w", err)
	}

	if err := generateAll(model, outputDir); err != nil {
		return err
	}

	// mkdocs.yml goes at the project root (parent of docs dir)
	projectRoot := filepath.Dir(docsDir)
	return writeMkDocsConfig(model, projectRoot, outputDir)
}
