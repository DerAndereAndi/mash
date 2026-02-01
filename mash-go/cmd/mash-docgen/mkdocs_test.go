package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMkDocsYAML(t *testing.T) {
	m := testModel(t)
	output := GenerateMkDocsYAML(m)

	// Basic structure
	mustContain(t, output, "site_name:")
	mustContain(t, output, "theme:")
	mustContain(t, output, "name: material")
	mustContain(t, output, "docs_dir: docs")

	// Homepage
	mustContain(t, output, "Home: index.md")

	// Navigation sections
	mustContain(t, output, "Protocol:")
	mustContain(t, output, "protocol-overview.md")
	mustContain(t, output, "Features:")
	mustContain(t, output, "generated/features/index.md")
	mustContain(t, output, "Use Cases:")
	mustContain(t, output, "generated/usecases/index.md")
	mustContain(t, output, "Endpoint Types:")
	mustContain(t, output, "generated/endpoints/index.md")

	// Feature Design section (hand-written)
	mustContain(t, output, "Feature Design:")
	mustContain(t, output, "features/energy-control.md")

	// Testing section
	mustContain(t, output, "Testing:")
	mustContain(t, output, "testing/README.md")

	// Generated features in nav
	mustContain(t, output, "generated/features/device-info.md")
	mustContain(t, output, "generated/features/energy-control.md")

	// Generated use cases in nav
	mustContain(t, output, "generated/usecases/gpl.md")

	// Generated endpoints in nav
	mustContain(t, output, "generated/endpoints/ev-charger.md")

	// Mermaid extension
	mustContain(t, output, "pymdownx.superfences")
	mustContain(t, output, "mermaid")

	// Extra CSS
	mustContain(t, output, "extra.css")
}

func TestGenerateMkDocsYAML_DarkModeToggle(t *testing.T) {
	m := testModel(t)
	output := GenerateMkDocsYAML(m)

	mustContain(t, output, "scheme: default")
	mustContain(t, output, "scheme: slate")
}

func TestGenerateExtraCSS(t *testing.T) {
	output := GenerateExtraCSS()

	mustContain(t, output, ".md-typeset")
	mustContain(t, output, "font-size")
	mustContain(t, output, ".mermaid")
	mustContain(t, output, "table")
}

func TestEndToEnd_MkDocsFiles(t *testing.T) {
	m := testModel(t)
	outputDir := t.TempDir()

	// Simulate the project root structure: outputDir is the project root
	docsGenDir := filepath.Join(outputDir, "docs", "generated")
	if err := os.MkdirAll(docsGenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeMkDocsConfig(m, outputDir, filepath.Join(outputDir, "docs", "generated")); err != nil {
		t.Fatalf("writeMkDocsConfig failed: %v", err)
	}

	// mkdocs.yml at project root
	mkdocsPath := filepath.Join(outputDir, "mkdocs.yml")
	if _, err := os.Stat(mkdocsPath); os.IsNotExist(err) {
		t.Error("mkdocs.yml not created")
	}

	// Homepage at docs/index.md
	homePath := filepath.Join(outputDir, "docs", "index.md")
	if _, err := os.Stat(homePath); os.IsNotExist(err) {
		t.Error("docs/index.md not created")
	}

	// CSS in generated dir
	cssPath := filepath.Join(docsGenDir, "stylesheets", "extra.css")
	if _, err := os.Stat(cssPath); os.IsNotExist(err) {
		t.Error("extra.css not created")
	}
}
