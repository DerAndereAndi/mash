// Command mash-test is a test runner for MASH protocol conformance testing.
//
// This command runs protocol conformance tests against MASH devices or
// controllers, validating that they correctly implement the specification.
//
// Usage:
//
//	mash-test [flags] [test-pattern]
//
// Flags:
//
//	-target string          Target address (host:port) of device/controller under test
//	-mode string            Test mode: device, controller (default "device")
//	-pics string            Path to PICS file for the target
//	-tests string           Path to test cases directory
//	-timeout duration       Test timeout (default 30s)
//	-verbose                Enable verbose output
//	-json                   Output results as JSON
//	-junit                  Output results as JUnit XML
//	-insecure               Skip TLS certificate verification
//	-setup-code string      PASE setup code (8-digit numeric string)
//	-client-identity string Client identity for PASE (default: test-client)
//	-server-identity string Server identity for PASE (default: test-device)
//	-protocol-log string    File path for protocol event logging (CBOR format)
//
// PICS capability filtering is determined automatically:
//   - If -pics is provided, the static PICS file is used.
//   - If -setup-code is provided (and no -pics), PICS is auto-discovered from
//     the device at startup via commissioning.
//   - If neither is provided, all tests run without capability filtering.
//
// Examples:
//
//	# Test a device at localhost:8443
//	mash-test -target localhost:8443 -mode device
//
//	# Test with PASE commissioning (auto-discovers PICS)
//	mash-test -target localhost:8443 -setup-code 12345678
//
//	# Test specific patterns with static PICS file
//	mash-test -target 192.168.1.100:8443 -pics device.pics -tests ./testdata/cases
//
//	# Run specific test pattern with verbose output
//	mash-test -target localhost:8443 -verbose "EnergyControl.*"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/runner"
	mashlog "github.com/mash-protocol/mash-go/pkg/log"
)

var (
	target         = flag.String("target", "", "Target address (host:port) of device/controller under test")
	mode           = flag.String("mode", "device", "Test mode: device, controller")
	pics           = flag.String("pics", "", "Path to PICS file for the target")
	tests          = flag.String("tests", "./testdata/cases", "Path to test cases directory")
	timeout        = flag.Duration("timeout", 30*time.Second, "Test timeout")
	verbose        = flag.Bool("verbose", false, "Enable verbose output")
	jsonOut        = flag.Bool("json", false, "Output results as JSON")
	junitOut       = flag.Bool("junit", false, "Output results as JUnit XML")
	insecure       = flag.Bool("insecure", false, "Skip TLS certificate verification")
	setupCode      = flag.String("setup-code", "", "PASE setup code (8-digit numeric string)")
	clientIdentity = flag.String("client-identity", "", "Client identity for PASE (default: test-client)")
	serverIdentity = flag.String("server-identity", "", "Server identity for PASE (default: test-device)")
	protocolLog    = flag.String("protocol-log", "", "File path for protocol event logging (CBOR format)")
	enableKey      = flag.String("enable-key", "00112233445566778899aabbccddeeff", "128-bit hex key for TestControl triggers (32 hex chars)")
	debug          = flag.Bool("debug", false, "Enable debug logging (connection lifecycle, precondition transitions, state snapshots)")
	suiteTimeout   = flag.Duration("suite-timeout", 0, "Suite timeout (0 = auto-calculate from test timeouts)")
	filter         = flag.String("filter", "", "Filter test cases by ID regex (e.g., 'TC-CERT.*' or 'TC-READ-001|TC-WRITE-001')")
)

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()

	// Get optional test pattern (from -filter flag or positional argument)
	pattern := *filter
	if pattern == "" && flag.NArg() > 0 {
		pattern = flag.Arg(0)
	}

	// Validate configuration
	if *target == "" {
		fmt.Fprintln(os.Stderr, "Error: target address is required (-target)")
		flag.Usage()
		return 1
	}

	if *mode != "device" && *mode != "controller" {
		fmt.Fprintf(os.Stderr, "Error: mode must be 'device' or 'controller', got '%s'\n", *mode)
		flag.Usage()
		return 1
	}

	// Derive auto-PICS: when setup-code is available but no static PICS
	// file is given, automatically discover capabilities from the device.
	autoPICS := *setupCode != "" && *pics == ""

	if *setupCode == "" && *pics == "" {
		log.Println("Warning: no PICS file or setup code provided; all tests will run without capability filtering")
	}

	// Determine output format
	outputFormat := "text"
	if *jsonOut {
		outputFormat = "json"
	} else if *junitOut {
		outputFormat = "junit"
	}

	// Setup logging for text output
	if outputFormat == "text" {
		log.SetFlags(log.Ltime)
		if *verbose || *debug {
			log.SetFlags(log.Ltime | log.Lmicroseconds)
		}
		printBanner()
		log.Printf("Target: %s", *target)
		log.Printf("Mode: %s", *mode)
		if autoPICS {
			log.Println("PICS: auto-discovery enabled")
		} else if *pics != "" {
			log.Printf("PICS: %s", *pics)
		}
		if pattern != "" {
			log.Printf("Pattern: %s", pattern)
		}
		log.Println()
	}

	// Set up protocol logging if requested
	var protocolLogger *mashlog.FileLogger
	if *protocolLog != "" {
		var err error
		protocolLogger, err = mashlog.NewFileLogger(*protocolLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create protocol logger: %v\n", err)
			return 1
		}
		if outputFormat == "text" {
			log.Printf("Protocol logging to: %s", *protocolLog)
		}
	}

	// Create runner configuration
	config := &runner.Config{
		Target:             *target,
		Mode:               *mode,
		PICSFile:           *pics,
		TestDir:            *tests,
		Pattern:            pattern,
		Timeout:            *timeout,
		Verbose:            *verbose,
		Output:             os.Stdout,
		OutputFormat:       outputFormat,
		InsecureSkipVerify: *insecure,
		SetupCode:          *setupCode,
		ClientIdentity:     *clientIdentity,
		ServerIdentity:     *serverIdentity,
		EnableKey:          *enableKey,
		AutoPICS:           autoPICS,
		Debug:              *debug,
		SuiteTimeout:       *suiteTimeout,
	}
	// Only set logger when non-nil to avoid typed-nil interface issue.
	if protocolLogger != nil {
		config.ProtocolLogger = protocolLogger
	}

	// Create and run test runner. Using a separate run() function ensures
	// deferred cleanup (Runner.Close, RemoveZone) always executes, even
	// when tests fail. Previously os.Exit(1) in main() skipped defers,
	// leaving stale zones on the device that prevented commissioning mode
	// in the next test run.
	r := runner.New(config)
	defer func() {
		r.Close()
		if protocolLogger != nil {
			protocolLogger.Close()
		}
	}()

	// Suite timeout is managed by the engine's RunSuite (auto-calculated or explicit).
	// The top-level context does not impose a timeout to avoid premature cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := r.Run(ctx)
	if err != nil {
		if outputFormat == "text" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			// For JSON/JUnit, error is written to stderr
			log.Printf("Error: %v", err)
		}
		return 1
	}

	// Exit with appropriate code
	if result.FailCount > 0 {
		return 1
	}
	return 0
}

func printBanner() {
	fmt.Print(`
 __  __    _    ____  _   _   _____         _
|  \/  |  / \  / ___|| | | | |_   _|__  ___| |_
| |\/| | / _ \ \___ \| |_| |   | |/ _ \/ __| __|
| |  | |/ ___ \ ___) |  _  |   | |  __/\__ \ |_
|_|  |_/_/   \_\____/|_| |_|   |_|\___||___/\__|

Protocol Conformance Test Runner
`)
}
