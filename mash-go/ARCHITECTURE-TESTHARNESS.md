# Test Harness Architecture

**Status:** Architecture Review (Updated)
**Date:** 2026-02-13

This document provides a full-depth architecture review of the MASH conformance test harness (`internal/testharness/`). It covers structural patterns, decomposition boundaries, and remaining areas for improvement.

---

## Overview

The test harness runs YAML-defined conformance test cases against real MASH device/controller instances. It exercises the full protocol stack -- TLS, PASE, certificate exchange, operational messaging, subscriptions, discovery, multi-zone management, and certificate renewal.

**Invoked via:** `mash-test` CLI (`cmd/mash-test/main.go`)

**Scale:** 65 YAML test case files, ~23,700 LOC production code, ~16,900 LOC test code across 6 packages

---

## Package Structure

```
internal/testharness/
├── engine/           # Test execution orchestration (8 files, ~2,600 LOC prod + ~1,310 LOC test)
│   ├── types.go           # TestResult, StepResult, ExpectResult, ExecutionState, EngineConfig, SuiteResult
│   ├── engine.go          # Run(), RunSuite(), executeStep(), store_result, expectation dispatch
│   ├── checkers.go        # 40+ assertion checkers (timing, certs, phases, snapshots, etc.)
│   ├── interpolation.go   # PICS and output interpolation ({{ }}, ${})
│   ├── keys.go            # Internal state key constants
│   └── *_test.go
│
├── loader/           # YAML test case parsing (6 files, ~680 LOC prod + ~1,075 LOC test)
│   ├── types.go           # TestCase, Step, Condition, PICSFile, PICSRequirementList
│   ├── loader.go          # LoadTestCases(), LoadPICSFile(), filtering, PICS checking, validation
│   └── *_test.go
│
├── runner/           # Protocol implementation + handler dispatch (61 files, ~19,000 LOC prod + ~13,300 LOC test)
│   │
│   │  # Core architecture (decomposed components)
│   ├── runner.go                # Runner struct, registerHandlers, delegating methods
│   ├── conn_pool.go             # ConnPool interface + connPoolImpl (connection + zone tracking)
│   ├── suite_session.go         # SuiteSession interface + suiteSessionImpl (suite zone lifecycle)
│   ├── coordinator.go           # Coordinator interface (precondition/teardown orchestration)
│   ├── conn_mgr.go              # ConnectionManager interface (connection lifecycle, crypto, health)
│   ├── dialer.go                # Dialer interface + tlsDialer (TLS connection establishment)
│   ├── commissioning.go         # Commissioning lifecycle (ensureCommissioned, transitionToOperational, etc.)
│   ├── preconditions.go         # Precondition level hierarchy, disconnect/reconnect, zone cleanup
│   ├── tiers.go                 # Connection tier system (infrastructure/protocol/application)
│   ├── retry.go                 # retryWithBackoff, dialWithRetry, contextSleep
│   ├── readiness.go             # Operational readiness probes
│   ├── errors.go                # ErrorCategory, ClassifiedError, classifyPASEError, isTransientError
│   ├── keys.go                  # ~1,200 lines of string constants (preconditions, state, output, param keys)
│   │
│   │  # Protocol handlers
│   ├── pase.go                  # SPAKE2+ handshake, cert exchange
│   ├── connection_handlers.go   # Keep-alive, close, queuing, monitoring
│   ├── security_handlers.go     # TLS testing, DEC-047 validation
│   ├── discovery_handlers.go    # mDNS advertising/browsing
│   ├── device_handlers.go       # Device state simulation
│   ├── controller_handlers.go   # Controller operations
│   ├── zone_handlers.go         # Zone creation, removal, management
│   ├── renewal_handlers.go      # Certificate renewal operations
│   ├── trigger_handlers.go      # TestControl feature invocations
│   ├── cert_handlers.go         # Certificate inspection/validation
│   ├── utility_handlers.go      # Wait, verify, comparisons, paramInt/paramFloat
│   ├── network_handlers.go      # Network partition, clock offset
│   │
│   │  # State & support
│   ├── state_types.go           # Typed state accessors (DiscoveryState, ZoneState, etc.)
│   ├── auto_pics.go             # Auto-PICS discovery
│   ├── resolver.go              # Feature/attribute/command name resolution
│   ├── diagnostics.go           # Device state snapshots, shuffle diagnostics
│   ├── debug.go                 # Debug logging, state snapshots
│   ├── test_certs.go            # Test certificate generation
│   │
│   │  # Generated mocks (for unit tests)
│   └── mocks/mocks_test.go     # mockery-generated mocks (~3,000 LOC)
│
├── reporter/         # Result output formatting (2 files, ~565 LOC prod + ~560 LOC test)
│   └── reporter.go       # TextReporter, JSONReporter (streaming + summary), JUnitReporter
│
├── mock/             # Stack-level test doubles (4 files, ~520 LOC prod + ~370 LOC test)
│   ├── device.go          # Mock DeviceService for integration tests
│   ├── controller.go      # Mock ControllerService for integration tests
│   └── errors.go          # Mock error types
│
└── assertions/       # Reusable test assertions (2 files, ~360 LOC prod + ~245 LOC test)
    └── assertions.go      # Structured assertion helpers for test files
```

---

## Core Abstractions

### Component Architecture

The Runner is decomposed into five interface-based components. Each component owns a distinct responsibility and is independently testable via mock injection.

```
Runner (orchestrator, ~2,450 LOC)
  ├── pool        ConnPool           # Connection + zone tracking
  ├── suite       SuiteSession       # Suite zone identity + crypto
  ├── coordinator Coordinator        # Precondition/teardown decisions
  ├── connMgr     ConnectionManager  # Connection lifecycle + health
  ├── dialer      Dialer             # TLS dial abstraction
  │
  ├── engine      *engine.Engine     # Step dispatch + checkers
  ├── resolver    *Resolver          # Name -> numeric ID resolution
  │
  │  # Working crypto (current session)
  ├── zoneCA, controllerCert, zoneCAPool, issuedDeviceCert
  │
  │  # Commissioning state
  ├── paseState                      # SPAKE2+ session
  ├── commissionZoneType             # ZoneType for current commission
  ├── deviceStateModified            # Whether test modified device state
  ├── lastDeviceConnClose            # For cooldown timing
  └── discoveredDiscriminator        # From mDNS browse
```

### Connection State Machine

```go
type ConnState int

const (
    ConnDisconnected ConnState = iota  // No socket resources held
    ConnTLSConnected                   // Commissioning TLS (pre/during PASE)
    ConnOperational                    // Operational TLS (zone CA verified)
)
```

```go
type Connection struct {
    conn                 net.Conn
    tlsConn              *tls.Conn
    framer               *transport.Framer
    state                ConnState
    hadConnection        bool      // Ever connected? (not cleared on disconnect)
    pendingNotifications [][]byte  // Buffered notifications from invoke
}
```

**Key methods:**

| Method | Behavior |
|--------|----------|
| `transitionTo(ConnDisconnected)` | Closes socket but does NOT nil pointers (close-but-not-nil) |
| `transitionTo(ConnOperational)` | Sets state, marks `hadConnection = true`, keeps socket |
| `clearConnectionRefs()` | Nils tlsConn/conn/framer -- only from full-cleanup paths |
| `isConnected()` | `state != ConnDisconnected` |
| `isOperational()` | `state == ConnOperational` |

The close-but-not-nil pattern is critical: goroutines capture `framer` references before spawning. When the socket is closed, ReadFrame/WriteFrame return IO errors rather than nil-pointer panics.

### ConnPool Interface

Manages the main connection and per-test zone connections.

```go
type ConnPool interface {
    Main() *Connection
    SetMain(conn *Connection)
    NextMessageID() uint32

    SendRequest(data []byte, op string, expectedMsgID uint32) (*wire.Response, error)
    SendRequestWithDeadline(ctx context.Context, ...) (*wire.Response, error)

    // Zone tracking (per-test zones only; suite zone lives outside pool)
    Zone(key string) *Connection
    TrackZone(key string, conn *Connection, zoneID string)
    CloseZonesExcept(exceptKey string) time.Time
    CloseAllZones() time.Time
    ZoneCount() int
    ZoneKeys() []string

    // Subscription and notification management
    TrackSubscription(subID uint32)
    Subscriptions() []uint32
    UnsubscribeAll(conn *Connection)
    PendingNotifications() [][]byte
    ShiftNotification() ([]byte, bool)
    AppendNotification(data []byte)
}
```

`SendRequest` loops up to 10 times, buffering interleaved notifications (messageID=0) and discarding orphaned responses until the expected messageID arrives.

### SuiteSession Interface

The suite zone is a long-lived zone that persists across tests, providing a control channel for trigger delivery. It lives **outside** the ConnPool so that pool-level operations (close, cleanup) never touch it.

```go
type SuiteSession interface {
    ZoneID() string               // Suite zone ID, or ""
    ConnKey() string              // Pool key: "main-" + zoneID
    IsCommissioned() bool
    Crypto() CryptoState          // Saved zone crypto material
    Conn() *Connection            // Suite zone connection (outside pool)
    SetConn(conn *Connection)
    Record(zoneID string, crypto CryptoState)
    Clear()                       // Closes connection + nils state
}

type CryptoState struct {
    ZoneCA           *cert.ZoneCA
    ControllerCert   *cert.OperationalCert
    ZoneCAPool       *x509.CertPool
    IssuedDeviceCert *x509.Certificate
}
```

### Coordinator Interface

Encapsulates the precondition/teardown decision tree. Extracted from a monolithic 855-line function into an independently testable component.

```go
type Coordinator interface {
    SetupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error
    TeardownTest(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState)
    CurrentLevel() int
}
```

The Coordinator calls back into the Runner via `CommissioningOps` (a ~24-method callback interface) for connection lifecycle, wire operations, and state accessors.

### ConnectionManager Interface

Owns connection lifecycle, crypto state management, and session health probes.

```go
type ConnectionManager interface {
    // Connection lifecycle
    EnsureConnected(ctx context.Context, state *engine.ExecutionState) error
    EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error
    TransitionToOperational(state *engine.ExecutionState) error
    DisconnectConnection()
    EnsureDisconnected()
    ReconnectToZone(state *engine.ExecutionState) error

    // Health checks
    ProbeSessionHealth() error
    WaitForOperationalReady(timeout time.Duration) error
    WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error

    // Crypto state
    WorkingCrypto() CryptoState
    SetWorkingCrypto(crypto CryptoState)
    ClearWorkingCrypto()
    OperationalTLSConfig() *tls.Config

    // PASE state, timing, commissioning metadata
    PASEState() *PASEState
    SetPASEState(ps *PASEState)
    LastDeviceConnClose() time.Time
    // ...
}
```

### Dialer Interface

Abstracts TLS connection establishment for testability.

```go
type Dialer interface {
    DialCommissioning(ctx context.Context, target string) (*tls.Conn, error)
    DialOperational(ctx context.Context, target string, crypto CryptoState) (*tls.Conn, error)
}
```

Production implementation (`tlsDialer`) uses `transport.NewCommissioningTLSConfig()` and `buildOperationalTLSConfig()` with zone CA verification and mutual TLS.

---

## Loader Types

```go
TestCase {
    ID, Name, Description string
    PICSRequirements PICSRequirementList   // Mixed strings + key:value maps
    Preconditions    []Condition           // map[string]any
    Steps            []Step
    Postconditions   []Condition
    Timeout, Tags, Skip, SkipReason
    ConnectionTier   string               // "infrastructure", "protocol", or "application"
}

Step {
    Action      string                    // Handler name (e.g., "read", "connect")
    Params      map[string]any            // Handler parameters
    Expect      map[string]any            // Assertions
    Timeout     string
    Description string
    StoreResult string                    // Save handler output under this key
}
```

### store_result Feature

When `Step.StoreResult` is set, the engine stores the handler's primary output in state for `{{ name }}` interpolation in later steps:

1. Looks for keys in order: `device_id`, `zone_id`, `value`, `result`
2. Stores first match (or full output map) under the `StoreResult` key
3. Also flattens output keys: `store_result + "_" + key = value`

```yaml
steps:
  - action: zone_commission
    store_result: zone1
  - action: read
    params:
      zone_id: "{{ zone1_zone_id }}"
```

---

## Execution Flow

```
CLI (mash-test)
  │
  ├── loader.LoadTestCases(dir)         → []*TestCase
  ├── loader.LoadPICSFile(path)         → *PICSFile
  │
  ├── runner.New(config)                → *Runner
  │     ├── engine.NewWithConfig()
  │     ├── registerHandlers()          → 50+ handlers registered
  │     ├── registerCheckers()          → 40+ custom checkers
  │     ├── NewConnPool()               → ConnPool (zones, subscriptions, notifications)
  │     ├── NewSuiteSession()           → SuiteSession (suite zone lifecycle)
  │     ├── NewCoordinator(...)         → Coordinator (precondition/teardown)
  │     ├── NewDialer(...)              → Dialer (TLS dial)
  │     └── NewConnectionManager(...)   → ConnectionManager (lifecycle, health)
  │
  └── runner.Run(ctx)
        ├── sortByPreconditionLevel()     → group by level, stable order within
        ├── ShuffleWithinLevels(seed)     → randomize within levels (if -shuffle)
        ├── runAutoPICS()                 → discover device capabilities
        ├── commissionSuiteZone()         → one-time suite commissioning (if L3 tests)
        │
        ├── engine.RunSuite(ctx, cases)
        │     └── for each TestCase:
        │           ├── Check PICS requirements
        │           ├── coordinator.SetupPreconditions(ctx, tc, state)
        │           │     ├── Determine level + connection tier
        │           │     ├── Handle backward transitions (commissioned → commissioning)
        │           │     ├── connMgr.EnsureCommissioned / EnsureConnected / etc.
        │           │     ├── Snapshot device state baseline
        │           │     └── Handle additional preconditions (zones, device config)
        │           ├── for each Step:
        │           │     ├── handler = handlers[step.Action]
        │           │     ├── outputs, err = handler(ctx, step, state)
        │           │     ├── state.Set(key, value) for each output
        │           │     ├── store_result processing (if configured)
        │           │     └── checkExpectation(key, expected, state) for each expect
        │           └── coordinator.TeardownTest(ctx, tc, state)
        │                 ├── Compare device state against baseline
        │                 ├── Re-send triggerResetTestState if diverged
        │                 └── closeActiveZoneConns(), reset per-test state
        │
        ├── removeSuiteZone()             → suite teardown after all tests
        └── reporter.Report(results)
```

### Connection Tier System

Tests declare (or have inferred) a connection tier that controls isolation aggressiveness:

| Tier | YAML Value | Behavior |
|------|-----------|----------|
| Infrastructure | `infrastructure` | Full disconnect + clear crypto before setup |
| Protocol | `protocol` | Disconnect TCP, preserve crypto, reconnect |
| Application | `application` | Reuse healthy commissioned connection |

```go
func connectionTierFor(tc *loader.TestCase) string {
    if tc.ConnectionTier != "" { return tc.ConnectionTier }
    // Infer from preconditions for backward compatibility
    if needed <= precondLevelCommissioning { return TierInfrastructure }
    if needsFreshCommission(tc.Preconditions) { return TierProtocol }
    return TierApplication
}
```

### Precondition Level Hierarchy

Tests are sorted by precondition level to minimize expensive state transitions:

| Level | Name | Requirements | Setup |
|-------|------|-------------|-------|
| 0 | None | Environment-only preconditions | Nothing |
| 1 | Commissioning mode | Device in commissioning mode | Ensure disconnected |
| 2 | Connected | TLS connection established | Connect |
| 3 | Commissioned | Session established (PASE + certs) | Connect + commission |

The sort ensures all level-0 tests run first, then level-1, etc. When shuffle mode is enabled, tests are randomized within each level (not across levels).

### Shuffle Mode

When `-shuffle` is enabled:

1. Seed is auto-generated or provided via `-shuffle-seed`
2. `ShuffleWithinLevels(cases, seed)` randomizes within each precondition level
3. Execution order and seed are recorded in `SuiteResult` for reproducibility
4. Device state baseline tracking ensures test-modified state is reset between tests

### State Lifecycle

```
Per-suite:    Runner + components persist across all tests
              SuiteSession holds zone crypto + connection
Per-test:     ExecutionState created fresh
              Coordinator manages precondition transitions
              Device state baseline captured + verified
Per-step:     Outputs accumulate in ExecutionState.Outputs
              store_result saves values for cross-step interpolation
```

---

## Handler Architecture

### Registration

All handlers are registered in `runner.go:registerHandlers()` via `engine.RegisterHandler(action, handler)`. There are 50+ handlers across 12+ files.

### Handler Groups

| File | Handlers | Responsibility |
|------|----------|---------------|
| `runner.go` | connect, disconnect, read, write, subscribe, invoke, unsubscribe, ping, close | Core protocol operations |
| `pase.go` | commission, pase_request, pase_response, cert_exchange | SPAKE2+ handshake |
| `connection_handlers.go` | keepalive_start, keepalive_stop, close_connection, wait_for_close, monitor_connection, send_raw_frame | Connection lifecycle |
| `security_handlers.go` | tls_connect, tls_connect_operational, verify_tls, probe_tls_rejection | TLS security testing |
| `discovery_handlers.go` | start_advertising, stop_advertising, browse_commissionable, browse_operational, browse_commissioners | mDNS |
| `device_handlers.go` | start_device, stop_device, configure_device, reset_device, get_device_state | Device management |
| `controller_handlers.go` | start_controller, stop_controller, controller_commission, controller_read, controller_subscribe | Controller operations |
| `zone_handlers.go` | create_zone, remove_zone, connect_zone, disconnect_zone, list_zones | Multi-zone management |
| `renewal_handlers.go` | send_renewal_request, verify_renewal_complete, renewal_nonce_binding | Certificate renewal |
| `trigger_handlers.go` | trigger (dispatches to TestControl commands) | Device state manipulation |
| `utility_handlers.go` | wait, verify_state, verify_timing, compare_values, store_value | Utilities |
| `network_handlers.go` | network_partition, network_restore, clock_offset | Network simulation |
| `cert_handlers.go` | inspect_cert, verify_cert_chain, verify_cert_validity, connect_with_cert | Certificate inspection |

### Parameter Handling

Handlers receive parameters in two ways:

1. **Raw YAML:** `step.Params` -- YAML v3 types (int, string, bool, map, etc.)
2. **Interpolated:** `engine.InterpolateParams(step.Params, state)` -- template references resolved

YAML v3 produces `int` for integer values, NOT `float64` (unlike JSON). Type-safe helpers exist:
- `paramInt(params, key, defaultVal) int`
- `paramFloat(params, key, defaultVal) float64`

---

## Error Classification System

### ErrorCategory

Three categories distinguish retryable from non-retryable errors:

```go
type ErrorCategory int

const (
    ErrCatInfrastructure ErrorCategory = iota  // Network/timing (retryable)
    ErrCatDevice                               // Device rejected (non-retryable)
    ErrCatProtocol                             // Protocol violation (non-retryable)
)

type ClassifiedError struct {
    Category ErrorCategory
    Err      error
}
```

Wrapper functions: `Infrastructure(err)`, `Device(err)`, `Protocol(err)`

### classifyPASEError

Classifies commissioning errors by priority:

1. IO errors (EOF, net.OpError, etc.) -> Infrastructure (retryable)
2. PASE error codes:
   - Codes 1,2,3,4,10 -> Device (non-retryable)
   - Code 5 (Busy) -> Infrastructure (retryable)
3. String patterns: "zone slots full" -> Device; "cooldown active" -> Infrastructure
4. Default -> Protocol (conservative: don't retry unknown errors)

### isTransientError

Two-tier check: classified errors check category first; unclassified errors fall back to `isIOError()` for backward compatibility.

### Retry Infrastructure

```go
type RetryConfig struct {
    MaxAttempts int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
}

func retryWithBackoff(ctx, cfg, fn) error          // Exponential backoff, respects ClassifiedError
func dialWithRetry(ctx, maxAttempts, dialFn) (*tls.Conn, error)  // 3 attempts, 50-200ms
func contextSleep(ctx, d) error                     // Context-aware sleep
```

`retryWithBackoff` stops early on non-Infrastructure classified errors (Device/Protocol are never worth retrying). `contextSleep` replaces bare `time.Sleep` calls throughout the codebase.

---

## Suite Zone Lifecycle

### Creation

```
Run()
  ├── needsSuiteCommissioning(cases)?  → any L3 test + enable-key provided
  ├── commissionSuiteZone(ctx)
  │     ├── Set commissionZoneType = ZoneTypeTest
  │     ├── ensureCommissioned(ctx, state)   → PASE + certs + operational TLS
  │     └── recordSuiteZone()
  │           ├── suite.Record(zoneID, crypto)
  │           ├── Move connection from ConnPool to suite.SetConn()
  │           └── pool.UntrackZone(connKey)  → removes from pool
  │
  └── [OR] auto-PICS established suite zone → preserved for tests
```

### Cross-Test Persistence

The suite zone connection lives outside the ConnPool. This means:
- `closeActiveZoneConns()` never touches it (pool-only cleanup)
- `CloseAllZones()` on the pool is safe (suite zone not tracked there)
- Backward transitions (L3 -> L1 -> L3) detach main conn but keep suite alive
- `reconnectToZone()` re-establishes operational TLS using stored `suite.Crypto()`

### Teardown

```
removeSuiteZone()
  ├── sendRemoveZone()        → on suite zone connection
  ├── closeAllZoneConns()     → pool zones
  └── ensureDisconnected()    → closes suite conn, clears all crypto
```

### disconnectConnection vs ensureDisconnected

| Method | TCP | Working Crypto | Suite Zone | PASE |
|--------|-----|---------------|------------|------|
| `disconnectConnection()` | Closed | Preserved | Preserved | Cleared |
| `ensureDisconnected()` | Closed | Cleared | Closed + cleared | Cleared |

`disconnectConnection()` is for L3->L1 transitions where reconnection is expected.
`ensureDisconnected()` is for full teardown (suite end, fresh_commission).

---

## Assertion & Checker System

### Default Checker

Performs equality comparison. For the `"present"` keyword, checks key exists with non-nil value. For lists-of-maps, uses subset matching. Falls back to `fmt.Sprintf("%v")` string comparison.

### Custom Checkers (~40+)

Registered in `checkers.go` via `RegisterEnhancedCheckers()`:

| Category | Checkers |
|----------|----------|
| **Value comparisons** | value_gt, value_gte, value_lt, value_lte, value_max, value_in, value_not, value_non_negative |
| **Collections** | contains, contains_only, map_size_equals, array_not_empty |
| **Type checks** | value_is_map, value_is_array, value_is_null, value_is_not_null, value_type |
| **Snapshots** | save_as, value_equals, issuer_fingerprint_equals |
| **Reference** | value_gte_saved, value_max_ref |
| **Domain** | keys_are_phases, values_valid_grid_phases |
| **Priming** | save_priming_value, priming_value_different_from |
| **Timing** | latency_under, average_latency_under |
| **Errors** | error_message_contains, no_error |
| **Special** | value_is_recent, value_treated_as_unknown, response_contains |

### PICS Interpolation

Expectation values can reference PICS values and state:

```yaml
expect:
  zone_count: "${MASH.S.ZONE.MAX + 1}"     # PICS arithmetic
  value: "{{ previous_reading }}"            # State reference
```

`InterpolateParamsWithPICS()` resolves both `${}` (PICS) and `{{ }}` (state) references before checking. Supports basic arithmetic (`+`, `-`, `*`, `/`).

---

## Name Resolution (resolver.go)

Maps human-readable names to numeric IDs:

```go
func (r *Resolver) ResolveFeature(nameOrID string) (uint8, error)
func (r *Resolver) ResolveAttribute(featureID uint8, nameOrID string) (uint16, error)
func (r *Resolver) ResolveEndpoint(nameOrID string) (uint8, error)
func (r *Resolver) ResolveCommand(featureID uint8, nameOrID string) (uint8, error)
```

Backed by `pkg/inspect/names.go` lookup tables. All names lowercased before lookup. Allows YAML tests to use human-readable names:

```yaml
- action: read
  params:
    endpoint: 1
    feature: Measurement
    attribute: acActivePower
```

---

## Diagnostics Pipeline

### Device State Snapshots (diagnostics.go)

The diagnostic system captures device state before and after each test:

1. **Pre-test baseline:** `RequestDeviceState()` via TestControl snapshot command
2. **Post-test snapshot:** Same command after test steps complete
3. **Diff computation:** Compare baseline vs. post-test to detect leaked state
4. **Auto-reset:** If state diverged, teardown re-sends `triggerResetTestState`

These snapshots are stored on `TestResult.DeviceStateBefore`, `DeviceStateAfter`, and `DeviceStateDiffs` for diagnostic output.

### Shuffle Seed Tracking

When shuffle mode is active, `SuiteResult.ShuffleSeed` and `SuiteResult.ExecutionOrder` are recorded for reproducibility. A failing shuffle seed can be replayed with `-shuffle-seed`.

---

## Result Reporting (reporter/)

### Reporter Interface

```go
type Reporter interface {
    ReportSuite(result *engine.SuiteResult)    // Full suite with all test details
    ReportTest(result *engine.TestResult)       // Single test (streamed, real-time)
    ReportSummary(result *engine.SuiteResult)   // Lightweight summary only
}
```

### Reporter Implementations

| Reporter | Format | Use Case |
|----------|--------|----------|
| `TextReporter` | Human-readable terminal output with slowest-tests section | Interactive use |
| `JSONReporter` | Streaming per-test JSON + lightweight summary | CI/CD integration |
| `JUnitReporter` | Jenkins-compatible XML | CI/CD integration |

### JSON Reporter

Emits two output types:

1. **Per-test streaming:** Individual `JSONTestResult` objects emitted via `ReportTest()` as tests complete
2. **Summary:** Lightweight `JSONSummaryResult` (no embedded tests array):

```go
type JSONSummaryResult struct {
    SuiteName   string  `json:"suite_name"`
    Duration    string  `json:"duration"`
    Total       int     `json:"total"`
    Passed      int     `json:"passed"`
    Failed      int     `json:"failed"`
    Skipped     int     `json:"skipped"`
    PassRate    float64 `json:"pass_rate"`
    ShuffleSeed int64   `json:"shuffle_seed,omitempty"`
}
```

The JSON reporter also includes `normalizeForJSON()` which recursively converts CBOR `map[interface{}]interface{}` to JSON-safe `map[string]interface{}` types.

---

## Integration with MASH Stack

### How the Harness Uses the Stack

The test harness creates **real** `DeviceService` and `ControllerService` instances via `device_handlers.go` and `controller_handlers.go`. It does not mock the stack.

```
Runner
  ├── Creates DeviceService (pkg/service) with TestMode=true
  │     └── Listens on localhost, uses MemoryStore for certs
  ├── Creates ControllerService (pkg/service) for controller tests
  ├── Connects directly via Dialer for protocol tests
  └── Uses commissioning.PASEClientSession for PASE tests
```

### Connection Lifecycle in Harness

```
1. start_device → DeviceService.Start() → TLS listener + mDNS
2. dialer.DialCommissioning() → TLS handshake
3. commission → PASEClientSession.Handshake() → shared secret
4. cert_exchange → CSR → sign → install → commissioning complete
5. connMgr.TransitionToOperational() → close commissioning conn → reconnect with cert
6. [operational messaging: read, write, subscribe, invoke]
7. coordinator.TeardownTest() → closeActiveZoneConns(), reset per-test state
```

### Trigger Delivery

Trigger delivery uses a cascading lookup:
1. **Main connection** (if operational)
2. **Suite zone connection** (`suite.Conn()`) as fallback

This simplified cascade replaced an earlier, more complex approach involving temporary zone creation.

---

## Test Coverage

### Unit Tests (via mock injection)

The interface-based decomposition enables focused unit testing:

| Component | Test File | Tests | Approach |
|-----------|----------|-------|----------|
| Coordinator | coordinator_test.go | 42+ | MockCommissioningOps |
| ConnPool | conn_pool_test.go | 20+ | Direct struct tests |
| ConnectionManager | conn_mgr_test.go | 15+ | Mock dependencies |
| Retry helpers | retry_test.go | 20+ | Direct function tests |
| Dialer | dialer_test.go | 10+ | Config verification |
| SuiteSession | suite_session_test.go | 10+ | Direct struct tests |
| Tiers | tiers_test.go | 8+ | Direct function tests |
| Commissioning | commissioning_test.go | 15+ | Integration-style |

Mocks are generated by mockery v3 and live in `runner/mocks/mocks_test.go` (~3,000 LOC).

### Integration Tests

The existing ~418 end-to-end integration tests (across `*_test.go` files in each handler) exercise the full harness against real stack instances. These continue to pass alongside the new unit tests.

---

## YAML Test Case Format

### Structure

```yaml
id: TC-COMM-001
name: Clean Graceful Close Sequence
description: |
  Verifies that sending a close message results in a close_ack.

pics_requirements:
  - MASH.S.TRANS.CLOSE_ACK_TIMEOUT=5
  - MASH.S.COMM

preconditions:
  - session_established: true
  - device_has_grid_zone: true

connection_tier: application          # Optional: infrastructure / protocol / application

steps:
  - action: read
    params:
      endpoint: 1
      feature: Measurement
      attribute: acActivePower
    expect:
      read_success: true
      value: "present"
    store_result: reading1            # Optional: save output for later steps

  - action: invoke
    params:
      endpoint: 0
      feature: TestControl
      command: SetFailsafeLimit
      args: {limitWatts: 5000}
    expect:
      invoke_success: true

postconditions:
  - clean_close: true

timeout: "10s"
tags: [close, graceful]
```

### Test Case Inventory

65 YAML test case files across categories:
- Commissioning (PASE, cert exchange, security, backoff)
- Certificate renewal (normal, expired, grace, nonce binding)
- Connection management (close, keepalive, reconnection)
- Protocol operations (read, write, subscribe, invoke)
- Discovery (advertising, browsing, operational)
- Multi-zone (zone creation, limits, priority)
- Security (TLS validation, certificate verification)
- Feature-specific (EnergyControl, ChargingSession, Measurement)
- Bidirectional communication

---

## Architectural Concerns

### Resolved

The following concerns from the original architecture review have been addressed:

1. **Runner God Object** -- Decomposed into five interface-based components (ConnPool, SuiteSession, Coordinator, ConnectionManager, Dialer). Runner is now an orchestrator that delegates to components. However, runner.go still sits at ~2,450 LOC because it hosts handler implementations for core protocol operations.

2. **Sleep-Based Synchronization** -- Replaced 8+ bare `time.Sleep()` calls with context-aware `retryWithBackoff()`, `dialWithRetry()`, and `contextSleep()`. Retry infrastructure respects error classification to avoid retrying non-retryable errors.

3. **Stale Session Detection** -- Replaced indirect `len(activeZoneConns) == 0` heuristic with explicit suite zone tracking via `SuiteSession` interface and device state baseline comparison.

4. **Notification Buffer Limit** -- Increased from 5 to 10 in the `SendRequest` loop within `ConnPool`.

### Remaining

5. **String-Typed Everything** -- ~1,200 lines of string constants in keys.go. No compile-time safety between handler outputs and checker inputs. Typos cause silent failures.

6. **Inconsistent Parameter Handling** -- No convention for when to use raw `step.Params` vs `InterpolateParams()`. Template references work unpredictably across handlers.

7. **Prose Assertion Auto-Pass** -- Multi-word expressions in `verify_state` always pass. Silent false-positive risk.

8. **High Stack Coupling** -- The harness uses real stack instances, inheriting all timing dependencies. Stack bugs manifest as test failures with no clear separation between "test is wrong" and "stack is wrong".

9. **Default Checker Fragility** -- Equality via `fmt.Sprintf("%v")` string comparison is fragile for complex values (maps, nested structures) where key ordering or type differences cause false failures.
