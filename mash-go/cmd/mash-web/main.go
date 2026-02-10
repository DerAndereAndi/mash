// Command mash-web provides an HTTP-based testing frontend for MASH protocol
// conformance testing.
//
// It offers:
//   - REST API for device discovery and test execution
//   - Simple web UI for managing test runs
//   - SQLite persistence for test run history
//
// Usage:
//
//	mash-web [flags]
//
// Flags:
//
//	-port int          HTTP server port (default 8080)
//	-tests string      Test cases directory (default "./testdata/cases")
//	-db string         SQLite database path (default "./mash-web.db")
//	-log-level string  Log level: debug, info, warn, error (default "info")
//
// Examples:
//
//	# Start the web server on default port
//	mash-web
//
//	# Start on a custom port with a specific test directory
//	mash-web -port 9000 -tests /path/to/tests
//
//	# Use an in-memory database (for testing)
//	mash-web -db :memory:
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

// Version information - set at build time via ldflags
var (
	Version   = "0.1.0"
	BuildDate = "dev"
	GitCommit = "unknown"
)

var (
	port        = flag.Int("port", 8080, "HTTP server port")
	testDir     = flag.String("tests", "./testdata/cases", "Test cases directory")
	dbPath      = flag.String("db", "./mash-web.db", "SQLite database path")
	logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	showVersion = flag.Bool("version", false, "Show version information")
)

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("mash-web %s (built %s, commit %s)\n", Version, BuildDate, GitCommit)
		return 0
	}

	// Validate test directory exists
	if info, err := os.Stat(*testDir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: test directory %q does not exist or is not a directory\n", *testDir)
		return 1
	}

	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime)
	if *logLevel == "debug" {
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	}

	// Create and configure server
	cfg := ServerConfig{
		Port:    *port,
		TestDir: *testDir,
		DBPath:  *dbPath,
		Version: Version,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create server: %v\n", err)
		return 1
	}
	defer srv.Close()

	// Start server
	log.Printf("Starting MASH Web on http://localhost:%d", *port)
	log.Printf("Test cases: %s", *testDir)
	log.Printf("Database: %s", *dbPath)

	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: server failed: %v\n", err)
		return 1
	}

	return 0
}
