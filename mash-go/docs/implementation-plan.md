# MASH Implementation Plan - Phase 2

> Completing the reference implementation with working networking and test harness

**Status:** Active
**Created:** 2026-01-26
**Previous Phase:** Layers 1-9 (structure complete, networking stubbed)

---

## Current State Assessment

### What's Implemented (Layers 1-9)

| Layer | Package(s) | Status | Test Coverage |
|-------|-----------|--------|---------------|
| 1 | `pkg/wire` | Complete | 10 tests |
| 2 | `pkg/features` | Complete | 10 tests |
| 3 | `pkg/transport`, `pkg/pase` | Complete | 45+12 tests |
| 4 | `pkg/cert`, `pkg/zone` | Complete | 12+5 tests |
| 5 | `pkg/connection`, `pkg/failsafe`, `pkg/subscription`, `pkg/commissioning` | Complete | 5+13+20+17 tests |
| 6 | `pkg/discovery` | Complete | 22 tests |
| 7 | `pkg/service` | Complete | 15 tests |
| 8 | `cmd/evse-example`, `cmd/cem-example` | Complete | N/A (examples) |
| 9 | `cmd/mash-device`, `cmd/mash-controller`, `cmd/mash-test` | Structure only | N/A (CLIs) |

**Total:** 254 test functions, ~57% test-to-code ratio

### What's Missing

1. **Actual Networking** - Services have TODO placeholders for:
   - TLS server in DeviceService
   - TLS client in ControllerService
   - Real mDNS advertising/browsing
   - PASE handshake over wire

2. **Test Harness** - Original plan's `internal/testharness/` never built:
   - Test case loader (YAML)
   - Test execution engine
   - Mock device/controller
   - Assertion framework
   - Reporter

3. **Integration Tests** - No end-to-end tests for:
   - Device-controller pairing
   - Multi-zone scenarios
   - Failsafe triggering
   - Subscription flow

---

## Phase 2 Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Phase 2 Additions                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  pkg/                                                               │
│    transport/                                                       │
│      server.go          # NEW: TLS server accepting connections     │
│      client.go          # NEW: TLS client connecting to devices     │
│                                                                     │
│    discovery/                                                       │
│      mdns_zeroconf.go   # NEW: Real mDNS using zeroconf library     │
│                                                                     │
│    service/                                                         │
│      device_service.go  # UPDATE: Wire up TLS server + mDNS         │
│      controller_service.go # UPDATE: Wire up TLS client + mDNS     │
│      protocol_handler.go # NEW: Message dispatch and handling       │
│                                                                     │
│  internal/                                                          │
│    testharness/                                                     │
│      loader/            # NEW: YAML test case loading               │
│      engine/            # NEW: Test execution orchestration         │
│      mock/              # NEW: Mock device and controller           │
│      assertions/        # NEW: Protocol assertion helpers           │
│      reporter/          # NEW: Test result reporting                │
│                                                                     │
│  testdata/                                                          │
│    cases/               # NEW: YAML test case files                 │
│    pics/                # NEW: Example PICS files                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## TDD Approach

Following the original plan's philosophy: **Define tests first, then implement.**

### Test Specification → Test Code → Implementation

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│  Behavior Spec   │────►│  Test Functions  │────►│  Implementation  │
│  (docs/testing/) │     │  (*_test.go)     │     │  (*.go)          │
└──────────────────┘     └──────────────────┘     └──────────────────┘
```

For each feature:
1. **Spec** - Review/create behavior specification in docs
2. **Test** - Write failing tests that encode the spec
3. **Implement** - Write minimal code to pass tests
4. **Refactor** - Clean up while keeping tests green

---

## Implementation Layers

### Layer 10: TLS Server (Device Side)

**Goal:** Device can accept TLS connections from controllers.

#### 10.1 TLS Server

**Package:** `pkg/transport`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| TLS 1.3 server with P-256 | transport.md §3 | TestServerTLS13Only |
| ALPN "mash/1" negotiation | transport.md §3.1 | TestServerALPN |
| Server certificate presentation | security.md §2 | TestServerCertificate |
| Client certificate request (optional) | security.md §2 | TestServerMutualTLS |
| Connection accept with framing | transport.md §4 | TestServerFraming |

**Test file:** `pkg/transport/server_test.go`

```go
// Tests to write FIRST (red phase):
func TestServerTLS13Only(t *testing.T)           // Reject TLS 1.2
func TestServerALPN(t *testing.T)                // Require mash/1
func TestServerCertificate(t *testing.T)         // Present valid cert
func TestServerMutualTLS(t *testing.T)           // Request client cert
func TestServerFraming(t *testing.T)             // Framed message exchange
func TestServerKeepAlive(t *testing.T)           // Ping/pong
func TestServerConcurrentConnections(t *testing.T)
```

#### 10.2 Protocol Handler

**Package:** `pkg/service`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| Message routing by operation | interaction-model.md | TestHandlerRouting |
| Read request handling | interaction-model.md §2 | TestHandlerRead |
| Write request handling | interaction-model.md §3 | TestHandlerWrite |
| Subscribe request handling | interaction-model.md §4 | TestHandlerSubscribe |
| Invoke request handling | interaction-model.md §5 | TestHandlerInvoke |
| Error response generation | interaction-model.md §6 | TestHandlerErrors |

**Test file:** `pkg/service/protocol_handler_test.go`

---

### Layer 11: TLS Client (Controller Side)

**Goal:** Controller can connect to devices and perform operations.

#### 11.1 TLS Client

**Package:** `pkg/transport`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| TLS 1.3 client with P-256 | transport.md §3 | TestClientTLS13Only |
| ALPN "mash/1" request | transport.md §3.1 | TestClientALPN |
| Server certificate validation | security.md §2 | TestClientCertValidation |
| Client certificate presentation | security.md §2 | TestClientMutualTLS |
| InsecureSkipVerify for commissioning | security.md §4 | TestClientCommissioningMode |

**Test file:** `pkg/transport/client_test.go`

```go
// Tests to write FIRST:
func TestClientTLS13Only(t *testing.T)
func TestClientALPN(t *testing.T)
func TestClientCertValidation(t *testing.T)
func TestClientMutualTLS(t *testing.T)
func TestClientCommissioningMode(t *testing.T)  // Skip verify initially
func TestClientReconnection(t *testing.T)
```

---

### Layer 12: mDNS Integration

**Goal:** Real device discovery using mDNS.

#### 12.1 mDNS Advertiser (Device)

**Package:** `pkg/discovery`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| Advertise _mashc._tcp (commissioning) | discovery.md §2 | TestAdvertiserCommissioning |
| Advertise _mash._tcp (operational) | discovery.md §2 | TestAdvertiserOperational |
| TXT record encoding | discovery.md §3 | TestAdvertiserTXTRecords |
| State transitions | discovery.md §4 | TestAdvertiserStateChange |
| Stop/restart advertising | discovery.md §4 | TestAdvertiserLifecycle |

**Test file:** `pkg/discovery/mdns_test.go`

#### 12.2 mDNS Browser (Controller)

**Package:** `pkg/discovery`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| Browse _mashc._tcp | discovery.md §5 | TestBrowserCommissioning |
| Browse _mash._tcp | discovery.md §5 | TestBrowserOperational |
| TXT record parsing | discovery.md §3 | TestBrowserTXTParsing |
| Discriminator filtering | discovery.md §5.1 | TestBrowserDiscriminator |
| Discovery timeout | discovery.md §5.2 | TestBrowserTimeout |

---

### Layer 13: PASE Over Wire

**Goal:** Complete commissioning handshake over TLS connection.

#### 13.1 PASE Protocol Messages

**Package:** `pkg/commissioning`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| PASERequest message format | security.md §4.2 | TestPASERequestEncoding |
| PASEResponse message format | security.md §4.2 | TestPASEResponseEncoding |
| PASEFinished message format | security.md §4.2 | TestPASEFinishedEncoding |
| Wrong password rejection | security.md §4.3 | TestPASEWrongPassword |
| Session key derivation | security.md §4.4 | TestPASESessionKey |

**Test file:** `pkg/commissioning/protocol_test.go`

#### 13.2 Certificate Exchange

**Package:** `pkg/commissioning`

| Item | Spec Reference | Test First |
|------|----------------|------------|
| CSR request from device | security.md §3.2 | TestCertExchangeCSR |
| Certificate signing by controller | security.md §3.3 | TestCertExchangeSigning |
| Certificate installation | security.md §3.4 | TestCertExchangeInstall |
| Mutual TLS reconnection | security.md §3.5 | TestCertExchangeReconnect |

---

### Layer 14: End-to-End Integration

**Goal:** Device and controller can complete full pairing flow.

#### 14.1 Integration Test Suite

**Package:** `integration_test` (top-level)

| Test | Description |
|------|-------------|
| TestE2E_Discovery | Controller discovers device via mDNS |
| TestE2E_Commissioning | Full PASE + certificate flow |
| TestE2E_ReadWrite | Read/write attributes after pairing |
| TestE2E_Subscribe | Subscription with notifications |
| TestE2E_Failsafe | Connection loss triggers failsafe |
| TestE2E_MultiZone | Second controller joins device |
| TestE2E_Reconnection | Automatic reconnection after disconnect |

**Test file:** `integration_test.go`

---

### Layer 15: Test Harness Infrastructure

**Goal:** Load and execute YAML test cases.

#### 15.1 Test Case Loader

**Package:** `internal/testharness/loader`

| Item | Test First |
|------|------------|
| Parse YAML test case format | TestLoaderParseBasic |
| Extract PICS requirements | TestLoaderPICSRequirements |
| Parse test steps | TestLoaderSteps |
| Parse assertions | TestLoaderAssertions |
| Handle invalid YAML | TestLoaderErrors |

#### 15.2 Test Engine

**Package:** `internal/testharness/engine`

| Item | Test First |
|------|------------|
| Filter tests by PICS | TestEngineFilter |
| Execute steps sequentially | TestEngineSteps |
| Handle timeouts | TestEngineTimeout |
| Collect results | TestEngineResults |
| Support preconditions | TestEnginePreconditions |

#### 15.3 Mock Device

**Package:** `internal/testharness/mock`

| Item | Test First |
|------|------------|
| Simulate device with PICS config | TestMockDeviceBasic |
| Handle Read requests | TestMockDeviceRead |
| Handle Write requests | TestMockDeviceWrite |
| Generate notifications | TestMockDeviceNotify |
| Inject faults | TestMockDeviceFaults |

#### 15.4 Mock Controller

**Package:** `internal/testharness/mock`

| Item | Test First |
|------|------------|
| Commission device | TestMockControllerCommission |
| Send commands | TestMockControllerCommands |
| Manage subscriptions | TestMockControllerSubscribe |
| Multi-zone simulation | TestMockControllerMultiZone |

#### 15.5 Assertions

**Package:** `internal/testharness/assertions`

| Item | Test First |
|------|------------|
| Assert attribute value | TestAssertValue |
| Assert state | TestAssertState |
| Assert timing (with tolerance) | TestAssertTiming |
| Assert notification received | TestAssertNotification |
| Assert error code | TestAssertError |

---

## Implementation Order

### Sprint 1: Networking Foundation (Tests First)

```
Day 1-2: Write failing tests
├── pkg/transport/server_test.go (7 tests)
├── pkg/transport/client_test.go (6 tests)
└── Red: All tests fail (no implementation)

Day 3-5: Implement to pass tests
├── pkg/transport/server.go
├── pkg/transport/client.go
└── Green: Tests pass

Day 6-7: Integration
├── pkg/service/device_service.go (wire up server)
├── pkg/service/controller_service.go (wire up client)
└── Manual test: Two processes can connect
```

### Sprint 2: mDNS Integration (Tests First)

```
Day 1-2: Write failing tests
├── pkg/discovery/mdns_test.go (10 tests)
└── Red: All tests fail

Day 3-4: Implement using zeroconf library
├── pkg/discovery/mdns_zeroconf.go
├── go get github.com/grandcat/zeroconf
└── Green: Tests pass

Day 5-6: Wire up to services
├── pkg/service/device_service.go (advertise)
├── pkg/service/controller_service.go (browse)
└── Manual test: Controller discovers device
```

### Sprint 3: PASE Protocol (Tests First)

```
Day 1-2: Write failing tests
├── pkg/commissioning/protocol_test.go (5 tests)
├── pkg/commissioning/exchange_test.go (4 tests)
└── Red: All tests fail

Day 3-5: Implement wire protocol
├── pkg/commissioning/protocol.go (messages over TLS)
├── pkg/commissioning/exchange.go (certificate flow)
└── Green: Tests pass

Day 6-7: Full commissioning flow
├── integration_test.go::TestE2E_Commissioning
└── Manual test: Controller commissions device
```

### Sprint 4: Integration Tests

```
Day 1-3: End-to-end test suite
├── TestE2E_Discovery
├── TestE2E_ReadWrite
├── TestE2E_Subscribe
├── TestE2E_Failsafe
└── TestE2E_MultiZone

Day 4-5: Fix issues discovered
└── Iterate until all E2E tests pass
```

### Sprint 5: Test Harness (Tests First)

```
Day 1-2: Loader tests
├── internal/testharness/loader/loader_test.go
└── Implement loader

Day 3-4: Engine tests
├── internal/testharness/engine/engine_test.go
└── Implement engine

Day 5-6: Mock device/controller tests
├── internal/testharness/mock/device_test.go
├── internal/testharness/mock/controller_test.go
└── Implement mocks

Day 7: Wire up mash-test CLI
└── cmd/mash-test/main.go uses internal/testharness
```

---

## Test Case Specifications

### YAML Test Case Format

```yaml
# testdata/cases/commissioning/TC-COMM-001.yaml
id: TC-COMM-001
name: Basic Commissioning Flow
description: Controller commissions device using setup code
pics_requirements:
  - D.COMM.SC  # Device supports setup code
  - C.COMM.SC  # Controller supports setup code

preconditions:
  - device_in_commissioning_mode: true
  - controller_has_zone_cert: true

steps:
  - action: controller_discover
    params:
      discriminator: 1234
    expect:
      device_found: true

  - action: controller_connect
    params:
      target: "{{ discovered_device }}"
      insecure: true
    expect:
      connection_established: true

  - action: controller_pase
    params:
      setup_code: "12345678"
    expect:
      pase_success: true
      session_key_derived: true

  - action: controller_issue_cert
    expect:
      csr_received: true
      cert_issued: true

  - action: controller_reconnect
    params:
      mutual_tls: true
    expect:
      connection_established: true
      certificate_verified: true

postconditions:
  - device_has_zone_cert: true
  - device_state: CONTROLLED
```

### Example PICS File

```
# testdata/pics/evse-minimal.pics
# Minimal EVSE PICS for testing

# Device capabilities
D.DI.TYPE.EVSE=true
D.DI.SOFT.VERSION=true
D.ELEC.PHASES=3
D.ELEC.MAX_CURRENT=32000

# Commissioning
D.COMM.SC=true
D.COMM.WINDOW_TIMEOUT=120

# Energy Control
D.EC.LIMIT.CONSUMPTION=true
D.EC.LIMIT.DURATION=true
D.EC.FAILSAFE.CONSUMPTION=true
D.EC.FAILSAFE.DURATION=14400

# Subscriptions
D.SUB.MAX=10
D.SUB.MIN_INTERVAL=1000
D.SUB.HEARTBEAT=true
```

---

## Dependencies

### New External Dependencies

| Library | Purpose | License |
|---------|---------|---------|
| `github.com/grandcat/zeroconf` | mDNS advertising and browsing | MIT |

### Existing Dependencies (No Changes)

- `github.com/fxamacker/cbor/v2` - CBOR
- `gopkg.in/yaml.v3` - YAML parsing

---

## Success Criteria

### Phase 2 Complete When:

1. **Networking Works**
   - [ ] `mash-device` and `mash-controller` can pair via mDNS + PASE
   - [ ] Read/Write/Subscribe/Invoke operations work over the connection
   - [ ] Reconnection works after disconnect
   - [ ] Failsafe triggers correctly

2. **Test Coverage**
   - [ ] All new code has tests written BEFORE implementation
   - [ ] Integration tests cover full pairing flow
   - [ ] >80% unit test coverage on new packages

3. **Test Harness**
   - [ ] YAML test cases can be loaded and executed
   - [ ] Mock device/controller work for testing third-party implementations
   - [ ] `mash-test` CLI produces meaningful reports

---

## File Checklist

### New Files to Create

```
pkg/transport/
├── server.go           # TLS server
├── server_test.go      # Tests FIRST
├── client.go           # TLS client
└── client_test.go      # Tests FIRST

pkg/discovery/
├── mdns_zeroconf.go    # Real mDNS implementation
└── mdns_test.go        # Tests FIRST (may need mocking)

pkg/service/
├── protocol_handler.go      # Message routing
└── protocol_handler_test.go # Tests FIRST

pkg/commissioning/
├── protocol.go         # PASE over wire
├── protocol_test.go    # Tests FIRST
├── exchange.go         # Certificate exchange
└── exchange_test.go    # Tests FIRST

internal/testharness/
├── loader/
│   ├── loader.go
│   ├── loader_test.go
│   └── types.go
├── engine/
│   ├── engine.go
│   ├── engine_test.go
│   └── executor.go
├── mock/
│   ├── device.go
│   ├── device_test.go
│   ├── controller.go
│   └── controller_test.go
├── assertions/
│   ├── assertions.go
│   └── assertions_test.go
└── reporter/
    ├── reporter.go
    ├── json.go
    └── html.go

testdata/
├── cases/
│   ├── commissioning/
│   ├── connection/
│   ├── subscription/
│   └── failsafe/
└── pics/
    ├── evse-minimal.pics
    ├── evse-full.pics
    └── controller-basic.pics

integration_test.go     # Top-level E2E tests
```

### Files to Update

```
pkg/service/device_service.go      # Wire up TLS server + mDNS
pkg/service/controller_service.go  # Wire up TLS client + mDNS
cmd/mash-test/main.go             # Use internal/testharness
go.mod                            # Add zeroconf dependency
```
