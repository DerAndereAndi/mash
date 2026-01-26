# MASH Reference Implementation Plan

> Go reference implementation with integrated test harness

**Status:** Draft
**Created:** 2025-01-25

---

## Overview

This document serves as a **checklist and roadmap** for implementing the MASH protocol reference implementation in Go. The goal is to prove the specification works through a working implementation with comprehensive test coverage.

### Guiding Principles

1. **Implementation and tests together** - Each layer gets unit tests as it's built
2. **Prove the spec** - Tests validate that the specification is implementable and unambiguous
3. **Reference quality** - Code serves as the canonical example of how to implement MASH
4. **Incremental validation** - Each layer is testable before building the next

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         mash-go Repository                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Reference Implementation                  │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │  cmd/                                                        │   │
│  │    mash-device/     # Reference device implementation        │   │
│  │    mash-controller/ # Reference controller (EMS) impl        │   │
│  │                                                              │   │
│  │  pkg/                                                        │   │
│  │    wire/            # CBOR types, message serialization      │   │
│  │    transport/       # TLS, framing, keep-alive               │   │
│  │    pase/            # SPAKE2+ commissioning                  │   │
│  │    cert/            # X.509 generation, validation, storage  │   │
│  │    zone/            # Zone management, priority resolution   │   │
│  │    connection/      # Connection lifecycle, reconnection     │   │
│  │    failsafe/        # Failsafe timer management              │   │
│  │    subscription/    # Subscription management, coalescing    │   │
│  │    discovery/       # mDNS advertising and browsing          │   │
│  │    device/          # Device model (endpoints, features)     │   │
│  │    pics/            # PICS parser and validator              │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                       Test Harness                           │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │  cmd/                                                        │   │
│  │    mash-test/       # Test runner CLI                        │   │
│  │                                                              │   │
│  │  internal/testharness/                                       │   │
│  │    engine/          # Test case execution engine             │   │
│  │    loader/          # YAML test case loader                  │   │
│  │    mock/            # Mock device and mock controller        │   │
│  │    assertions/      # Protocol assertion helpers             │   │
│  │    reporter/        # Test result reporting                  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                     Test Specifications                      │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │  testdata/                                                   │   │
│  │    pics/            # Example PICS files                     │   │
│  │    cases/           # YAML test cases (from docs/testing/)   │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Test Harness Architecture

Your understanding is correct. The test harness has two main components:

### 1. Test Engine

Processes test specifications and orchestrates execution:

```
┌──────────────────────────────────────────────────────────────┐
│                        Test Engine                            │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────┐    ┌────────────┐    ┌────────────┐        │
│  │ PICS       │    │ Test Case  │    │ Behavior   │        │
│  │ Parser     │    │ Loader     │    │ Rules      │        │
│  └─────┬──────┘    └─────┬──────┘    └─────┬──────┘        │
│        │                 │                 │                │
│        └────────┬────────┴────────┬────────┘                │
│                 ▼                 ▼                          │
│        ┌───────────────┐ ┌───────────────┐                  │
│        │ Test Selector │ │ Test Executor │                  │
│        │ (PICS filter) │ │ (step runner) │                  │
│        └───────────────┘ └───────┬───────┘                  │
│                                  │                          │
│                          ┌───────▼───────┐                  │
│                          │   Assertions  │                  │
│                          │   Framework   │                  │
│                          └───────────────┘                  │
└──────────────────────────────────────────────────────────────┘
```

### 2. Protocol Driver (Mock Device/Controller)

Interacts with the Implementation Under Test (IUT):

```
┌──────────────────────────────────────────────────────────────┐
│                     Protocol Driver                           │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Mode: MOCK_DEVICE           Mode: MOCK_CONTROLLER           │
│  ┌──────────────────┐       ┌──────────────────┐            │
│  │ Acts as Device   │       │ Acts as Controller│            │
│  │ Tests Controller │       │ Tests Device      │            │
│  │ IUT              │       │ IUT               │            │
│  └──────────────────┘       └──────────────────┘            │
│                                                              │
│  Capabilities:                                               │
│  - TLS client/server                                         │
│  - CBOR encode/decode                                        │
│  - PASE initiator/responder                                  │
│  - Subscription send/receive                                 │
│  - State tracking                                            │
│  - Timing measurement                                        │
│  - Fault injection                                           │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Test Execution Flow

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ Device  │     │ Test    │     │Protocol │     │ IUT     │
│ PICS    │────►│ Engine  │────►│ Driver  │◄───►│(Device/ │
│ File    │     │         │     │(Mock)   │     │ Ctrl)   │
└─────────┘     └────┬────┘     └─────────┘     └─────────┘
                     │
                     ▼
              ┌─────────────┐
              │ Test Report │
              │ - Pass/Fail │
              │ - Coverage  │
              │ - Timing    │
              └─────────────┘
```

---

## Implementation Layers

### Layer 1: Foundation

**Goal:** Shared types and parsing infrastructure

#### 1.1 CBOR Wire Types

**Package:** `pkg/wire`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Message envelope (messageId, operation/status) | |
| [ ] | Request types (Read, Write, Subscribe, Invoke) | |
| [ ] | Response types with status codes | |
| [ ] | Notification types | |
| [ ] | Control messages (ping, pong, close) | |
| [ ] | Error codes enum | |
| [ ] | All feature attribute types | |
| [ ] | All command argument/response types | |
| [ ] | Unit tests for CBOR round-trip | |
| [ ] | Compatibility tests (unknown fields ignored) | |

**Key files:**
- `pkg/wire/message.go` - Message envelope
- `pkg/wire/request.go` - Request types
- `pkg/wire/response.go` - Response types
- `pkg/wire/features/*.go` - Feature-specific types
- `pkg/wire/errors.go` - Error codes

**Spec references:**
- [docs/interaction-model.md](interaction-model.md)
- [docs/testing/behavior/message-framing.md](testing/behavior/message-framing.md)

#### 1.2 PICS Parser

**Package:** `pkg/pics`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | PICS file parser (line-based format) | |
| [ ] | PICS code structure (Side.Feature.Type.ID.Qualifier) | |
| [ ] | Value types (boolean, integer, string, enum) | |
| [ ] | Validation against conformance rules | |
| [ ] | Dependency checking (e.g., V2X requires EMOB) | |
| [ ] | Unit tests with example PICS files | |

**Key files:**
- `pkg/pics/parser.go` - Parser implementation
- `pkg/pics/validator.go` - Conformance validation
- `pkg/pics/types.go` - PICS data structures

**Spec references:**
- [docs/testing/pics-format.md](testing/pics-format.md)
- [docs/testing/pics/pairing-connection-registry.md](testing/pics/pairing-connection-registry.md)
- [docs/conformance/README.md](conformance/README.md)

#### 1.3 Constants and Enums

**Package:** `pkg/wire`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | ControlStateEnum | |
| [ ] | ProcessStateEnum | |
| [ ] | OperatingStateEnum | |
| [ ] | ZoneTypeEnum with priorities | |
| [ ] | DeviceTypeEnum | |
| [ ] | EndpointTypeEnum | |
| [ ] | Feature IDs | |
| [ ] | Attribute IDs per feature | |
| [ ] | Command IDs per feature | |
| [ ] | Status codes | |

**Spec references:**
- [docs/features/energy-control.md](features/energy-control.md)
- [docs/features/status.md](features/status.md)

---

### Layer 2: Transport

**Goal:** TLS connection with framing and keep-alive

#### 2.1 TLS Configuration

**Package:** `pkg/transport`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | TLS 1.3 configuration (no fallback) | |
| [ ] | Cipher suite selection (AES-128-GCM mandatory) | |
| [ ] | Key exchange (P-256 mandatory, X25519 optional) | |
| [ ] | Certificate verification callbacks | |
| [ ] | ALPN: "mash/1" | |
| [ ] | Session resumption disabled | |
| [ ] | Unit tests for TLS handshake | |
| [ ] | Tests for cipher suite negotiation | |
| [ ] | Tests for certificate rejection | |

**Key files:**
- `pkg/transport/tls.go` - TLS configuration
- `pkg/transport/tls_test.go` - TLS tests

**Spec references:**
- [docs/transport.md](transport.md) Section 3
- [docs/testing/behavior/tls-profile.md](testing/behavior/tls-profile.md)

#### 2.2 Message Framing

**Package:** `pkg/transport`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Length-prefix framing (4-byte big-endian) | |
| [ ] | Maximum message size (64KB default) | |
| [ ] | Frame reader with size validation | |
| [ ] | Frame writer | |
| [ ] | Unit tests for framing | |
| [ ] | Tests for oversized messages | |
| [ ] | Tests for truncated frames | |

**Key files:**
- `pkg/transport/framing.go` - Frame read/write
- `pkg/transport/framing_test.go` - Framing tests

**Spec references:**
- [docs/transport.md](transport.md) Section 4
- [docs/testing/behavior/message-framing.md](testing/behavior/message-framing.md)

#### 2.3 Keep-Alive

**Package:** `pkg/transport`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Ping message generation (30s interval) | |
| [ ] | Pong response handling (5s timeout) | |
| [ ] | Missed pong counting (3 max) | |
| [ ] | 95-second max detection delay validation | |
| [ ] | Unit tests for timing | |
| [ ] | Integration tests for connection loss detection | |

**Key files:**
- `pkg/transport/keepalive.go` - Keep-alive logic
- `pkg/transport/keepalive_test.go` - Keep-alive tests

**Spec references:**
- [docs/transport.md](transport.md) Section 5
- [docs/testing/behavior/failsafe-timing.md](testing/behavior/failsafe-timing.md)

#### 2.4 Connection Manager

**Package:** `pkg/transport`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Connection state machine (DISCONNECTED → CONNECTING → CONNECTED → CLOSING) | |
| [ ] | Graceful close handshake | |
| [ ] | Connection event callbacks | |
| [ ] | Message routing (request/response correlation) | |
| [ ] | Unit tests for state transitions | |

**Key files:**
- `pkg/transport/connection.go` - Connection management
- `pkg/transport/connection_test.go` - Connection tests

**Spec references:**
- [docs/testing/behavior/connection-state-machine.md](testing/behavior/connection-state-machine.md)

---

### Layer 3: Commissioning

**Goal:** SPAKE2+/PASE protocol for initial pairing

#### 3.1 SPAKE2+ Implementation

**Package:** `pkg/pase`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | SPAKE2+ with P-256 curve | |
| [ ] | Password derivation from setup code | |
| [ ] | Prover (controller) implementation | |
| [ ] | Verifier (device) implementation | |
| [ ] | Key confirmation (HMAC) | |
| [ ] | Session key derivation (HKDF-SHA256) | |
| [ ] | Unit tests for known test vectors | |
| [ ] | Tests for wrong password rejection | |
| [ ] | Tests for replay protection | |

**Key files:**
- `pkg/pase/spake2.go` - SPAKE2+ core
- `pkg/pase/prover.go` - Controller side
- `pkg/pase/verifier.go` - Device side
- `pkg/pase/spake2_test.go` - Tests with vectors

**Spec references:**
- [docs/security.md](security.md) Section 4
- [docs/testing/behavior/commissioning-pase.md](testing/behavior/commissioning-pase.md)

#### 3.2 Setup Code Handling

**Package:** `pkg/pase`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | 8-digit setup code generation | |
| [ ] | Setup code validation | |
| [ ] | QR code parsing (MASH:version:discriminator:setupcode:vendor:product) | |
| [ ] | QR code generation | |
| [ ] | Discriminator extraction (12-bit) | |
| [ ] | Unit tests for parsing | |
| [ ] | Tests for malformed QR codes | |

**Key files:**
- `pkg/pase/setupcode.go` - Setup code handling
- `pkg/pase/qrcode.go` - QR code format

**Spec references:**
- [docs/security.md](security.md) Section 4.1-4.2
- [docs/testing/behavior/discovery.md](testing/behavior/discovery.md)

#### 3.3 Commissioning Window

**Package:** `pkg/pase`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Window state machine (CLOSED → OPEN → PASE_IN_PROGRESS → CLOSED) | |
| [ ] | Window timeout (120s default) | |
| [ ] | Single concurrent PASE session | |
| [ ] | Window open trigger (button, command) | |
| [ ] | Unit tests for state machine | |
| [ ] | Tests for timeout | |
| [ ] | Tests for concurrent session rejection | |

**Key files:**
- `pkg/pase/window.go` - Commissioning window
- `pkg/pase/window_test.go` - Window tests

**Spec references:**
- [docs/testing/behavior/commissioning-pase.md](testing/behavior/commissioning-pase.md)

---

### Layer 4: Certificates and Zones

**Goal:** X.509 certificate management and multi-zone support

#### 4.1 Certificate Generation

**Package:** `pkg/cert`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Zone CA generation (99-year validity) | |
| [ ] | Operational certificate generation (1-year validity) | |
| [ ] | CSR generation (device side) | |
| [ ] | CSR signing (controller side) | |
| [ ] | Certificate chain building | |
| [ ] | ECDSA P-256 key generation | |
| [ ] | Unit tests for certificate generation | |
| [ ] | Tests for validity periods | |

**Key files:**
- `pkg/cert/ca.go` - CA management
- `pkg/cert/operational.go` - Operational certs
- `pkg/cert/csr.go` - CSR handling

**Spec references:**
- [docs/security.md](security.md) Section 2-3

#### 4.2 Certificate Storage

**Package:** `pkg/cert`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Multi-zone certificate storage (up to 5 zones) | |
| [ ] | Persistent storage interface | |
| [ ] | In-memory storage (for testing) | |
| [ ] | File-based storage | |
| [ ] | Certificate lookup by zone | |
| [ ] | Unit tests for storage operations | |

**Key files:**
- `pkg/cert/store.go` - Storage interface
- `pkg/cert/store_memory.go` - In-memory implementation
- `pkg/cert/store_file.go` - File-based implementation

**Spec references:**
- [docs/testing/pics/pairing-connection-registry.md](testing/pics/pairing-connection-registry.md) CERT section

#### 4.3 Certificate Renewal

**Package:** `pkg/cert`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Expiry tracking | |
| [ ] | Renewal window (30 days before expiry) | |
| [ ] | In-session renewal flow | |
| [ ] | Grace period handling (optional, 7 days) | |
| [ ] | Unit tests for renewal | |
| [ ] | Tests for grace period | |

**Key files:**
- `pkg/cert/renewal.go` - Renewal logic

**Spec references:**
- [docs/security.md](security.md) Section 3.1
- [docs/testing/behavior/zone-lifecycle.md](testing/behavior/zone-lifecycle.md)

#### 4.4 Zone Management

**Package:** `pkg/zone`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Zone types (GRID_OPERATOR, BUILDING_MANAGER, HOME_MANAGER, USER_APP) | |
| [ ] | Priority values (1, 2, 3, 4) | |
| [ ] | Zone addition (commissioning) | |
| [ ] | Zone removal (graceful and forced) | |
| [ ] | Priority-based forced removal | |
| [ ] | Maximum zones (5) | |
| [ ] | Unit tests for zone operations | |
| [ ] | Tests for priority override | |

**Key files:**
- `pkg/zone/zone.go` - Zone data structures
- `pkg/zone/manager.go` - Zone management

**Spec references:**
- [docs/security.md](security.md) Section 1.2
- [docs/testing/behavior/zone-lifecycle.md](testing/behavior/zone-lifecycle.md)

#### 4.5 Multi-Zone Resolution

**Package:** `pkg/zone`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Limit resolution (most restrictive wins) | |
| [ ] | Setpoint resolution (highest priority wins) | |
| [ ] | Effective value calculation | |
| [ ] | Per-zone "my" value tracking | |
| [ ] | Duration handling per zone | |
| [ ] | Unit tests for resolution | |
| [ ] | Tests for complex multi-zone scenarios | |

**Key files:**
- `pkg/zone/resolution.go` - Resolution algorithms

**Spec references:**
- [docs/testing/behavior/multi-zone-resolution.md](testing/behavior/multi-zone-resolution.md)

---

### Layer 5: Connection Lifecycle

**Goal:** Reconnection, failsafe, and subscription management

#### 5.1 Reconnection

**Package:** `pkg/connection`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Exponential backoff (1s initial, 60s max) | |
| [ ] | Jitter (0.25 factor) | |
| [ ] | Connection state persistence | |
| [ ] | Automatic reconnection | |
| [ ] | TLS handshake completion = success | |
| [ ] | Unit tests for backoff calculation | |
| [ ] | Integration tests for reconnection | |

**Key files:**
- `pkg/connection/reconnect.go` - Reconnection logic
- `pkg/connection/backoff.go` - Backoff calculation

**Spec references:**
- [docs/transport.md](transport.md) Section 7
- [docs/testing/behavior/connection-state-machine.md](testing/behavior/connection-state-machine.md)

#### 5.2 Failsafe Timer

**Package:** `pkg/failsafe`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Failsafe duration (2h-24h, default 4h) | |
| [ ] | Timer start on connection loss | |
| [ ] | Timer accuracy (+/- 1% or 60s) | |
| [ ] | Failsafe limits (consumption and/or production) | |
| [ ] | Timer persistence (PICS option) | |
| [ ] | Grace period (PICS option, 5 min) | |
| [ ] | Reconnection cancels timer | |
| [ ] | Unit tests for timing | |
| [ ] | Tests for persistence across restart | |

**Key files:**
- `pkg/failsafe/timer.go` - Timer logic
- `pkg/failsafe/limits.go` - Limit application

**Spec references:**
- [docs/testing/behavior/failsafe-timing.md](testing/behavior/failsafe-timing.md)
- [docs/testing/behavior/state-machines.md](testing/behavior/state-machines.md)

#### 5.3 Subscription Management

**Package:** `pkg/subscription`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Subscription creation with intervals | |
| [ ] | Priming report (all current values) | |
| [ ] | Delta notifications (changed only) | |
| [ ] | Heartbeat at maxInterval | |
| [ ] | Last-value-wins coalescing | |
| [ ] | Bounce-back suppression | |
| [ ] | Subscription restoration on reconnect | |
| [ ] | Maximum subscriptions (PICS) | |
| [ ] | Unit tests for notification logic | |
| [ ] | Tests for coalescing | |
| [ ] | Tests for restoration | |

**Key files:**
- `pkg/subscription/subscription.go` - Subscription data
- `pkg/subscription/manager.go` - Subscription management
- `pkg/subscription/notifier.go` - Notification generation

**Spec references:**
- [docs/testing/behavior/subscription-semantics.md](testing/behavior/subscription-semantics.md)

#### 5.4 Duration Timers

**Package:** `pkg/duration`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Timer starts on command receipt | |
| [ ] | Auto-clear on expiry | |
| [ ] | Accuracy (+/- 1% or 60s) | |
| [ ] | No persistence across reconnection (default) | |
| [ ] | Per-zone timer tracking | |
| [ ] | Notification on expiry | |
| [ ] | Unit tests for timing | |

**Key files:**
- `pkg/duration/timer.go` - Duration timer logic

**Spec references:**
- [docs/testing/behavior/duration-semantics.md](testing/behavior/duration-semantics.md)

---

### Layer 6: Discovery

**Goal:** mDNS advertising and browsing

#### 6.1 mDNS Server (Device)

**Package:** `pkg/discovery`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Service type: `_mash._tcp` (operational) | |
| [ ] | Service type: `_mashc._udp` (commissionable) | |
| [ ] | TXT records (D, VP, CM, DT for pre-comm) | |
| [ ] | TXT records (DI, FW, EP, FM for post-comm) | |
| [ ] | Instance name format | |
| [ ] | State transitions (PRE_COMM ↔ OPERATIONAL) | |
| [ ] | Unit tests for TXT record generation | |

**Key files:**
- `pkg/discovery/advertiser.go` - mDNS advertising
- `pkg/discovery/txtrecords.go` - TXT record handling

**Spec references:**
- [docs/discovery.md](discovery.md)
- [docs/testing/behavior/discovery.md](testing/behavior/discovery.md)

#### 6.2 mDNS Browser (Controller)

**Package:** `pkg/discovery`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Browse `_mash._tcp` (operational devices) | |
| [ ] | Browse `_mashc._udp` (commissionable devices) | |
| [ ] | Discriminator filtering | |
| [ ] | Device capability parsing from TXT | |
| [ ] | Unit tests for parsing | |
| [ ] | Integration tests for discovery | |

**Key files:**
- `pkg/discovery/browser.go` - mDNS browsing
- `pkg/discovery/parser.go` - TXT record parsing

**Spec references:**
- [docs/discovery.md](discovery.md)
- [docs/testing/behavior/discovery.md](testing/behavior/discovery.md)

---

### Layer 7: Device Model

**Goal:** Device, endpoint, and feature abstractions

#### 7.1 Device Structure

**Package:** `pkg/device`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Device container | |
| [ ] | Endpoint collection (0 = DEVICE_ROOT) | |
| [ ] | Feature registration per endpoint | |
| [ ] | Attribute storage and access | |
| [ ] | Command handlers | |
| [ ] | Event generation | |
| [ ] | Unit tests for device model | |

**Key files:**
- `pkg/device/device.go` - Device container
- `pkg/device/endpoint.go` - Endpoint abstraction
- `pkg/device/feature.go` - Feature interface

**Spec references:**
- [docs/protocol-overview.md](protocol-overview.md)

#### 7.2 Feature Implementations

**Package:** `pkg/device/features`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | DeviceInfo feature | |
| [ ] | Electrical feature | |
| [ ] | Measurement feature | |
| [ ] | EnergyControl feature | |
| [ ] | Status feature | |
| [ ] | ChargingSession feature | |
| [ ] | Signals feature | |
| [ ] | Tariff feature | |
| [ ] | Plan feature | |
| [ ] | Unit tests per feature | |

**Key files:**
- `pkg/device/features/deviceinfo.go`
- `pkg/device/features/electrical.go`
- `pkg/device/features/measurement.go`
- `pkg/device/features/energycontrol.go`
- `pkg/device/features/status.go`
- `pkg/device/features/chargingsession.go`
- `pkg/device/features/signals.go`
- `pkg/device/features/tariff.go`
- `pkg/device/features/plan.go`

**Spec references:**
- [docs/features/](features/)

---

### Layer 8: Test Harness

**Goal:** Automated test execution against PICS

#### 8.1 Test Case Loader

**Package:** `internal/testharness/loader`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | YAML test case parser | |
| [ ] | Test case data structures | |
| [ ] | PICS requirement extraction | |
| [ ] | Step parsing with actions and verifications | |
| [ ] | Unit tests for parsing | |

**Key files:**
- `internal/testharness/loader/loader.go`
- `internal/testharness/loader/types.go`

**Spec references:**
- [docs/testing/README.md](testing/README.md)

#### 8.2 Test Engine

**Package:** `internal/testharness/engine`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Test selection based on PICS | |
| [ ] | Sequential step execution | |
| [ ] | Precondition setup | |
| [ ] | Postcondition validation | |
| [ ] | Timeout handling | |
| [ ] | Result collection | |
| [ ] | Unit tests for engine | |

**Key files:**
- `internal/testharness/engine/engine.go`
- `internal/testharness/engine/executor.go`

#### 8.3 Mock Device

**Package:** `internal/testharness/mock`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Configurable device simulation | |
| [ ] | PICS-based capability configuration | |
| [ ] | State tracking | |
| [ ] | Fault injection (delays, errors) | |
| [ ] | Message logging | |

**Key files:**
- `internal/testharness/mock/device.go`

#### 8.4 Mock Controller

**Package:** `internal/testharness/mock`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Configurable controller simulation | |
| [ ] | Multi-zone support | |
| [ ] | Commissioning flow | |
| [ ] | Command sending | |
| [ ] | Subscription management | |

**Key files:**
- `internal/testharness/mock/controller.go`

#### 8.5 Assertions

**Package:** `internal/testharness/assertions`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Attribute value assertions | |
| [ ] | State assertions | |
| [ ] | Timing assertions (+/- tolerance) | |
| [ ] | Notification assertions | |
| [ ] | Error code assertions | |
| [ ] | Presence/absence assertions | |

**Key files:**
- `internal/testharness/assertions/assertions.go`

#### 8.6 Reporter

**Package:** `internal/testharness/reporter`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | Console output | |
| [ ] | JSON report format | |
| [ ] | PICS coverage analysis | |
| [ ] | Timing statistics | |
| [ ] | Pass/fail summary | |

**Key files:**
- `internal/testharness/reporter/reporter.go`
- `internal/testharness/reporter/json.go`

---

### Layer 9: Reference Applications

**Goal:** Working device and controller implementations

#### 9.1 Reference Device

**Command:** `cmd/mash-device`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | CLI argument parsing | |
| [ ] | Configuration file support | |
| [ ] | Simulated device types (EVSE, heat pump, battery) | |
| [ ] | PICS file loading | |
| [ ] | Logging | |
| [ ] | Integration tests | |

#### 9.2 Reference Controller

**Command:** `cmd/mash-controller`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | CLI argument parsing | |
| [ ] | Zone configuration | |
| [ ] | Device discovery and commissioning | |
| [ ] | Interactive command interface | |
| [ ] | Logging | |
| [ ] | Integration tests | |

#### 9.3 Test Runner

**Command:** `cmd/mash-test`

| Item | Description | Status |
|------|-------------|--------|
| [ ] | CLI argument parsing | |
| [ ] | PICS file input | |
| [ ] | Test case directory | |
| [ ] | Target device/controller address | |
| [ ] | Report generation | |
| [ ] | Verbose/quiet modes | |

---

## Implementation Order

### Phase 1: Foundation (Weeks 1-2)

```
Week 1:
├── [ ] pkg/wire - CBOR types
├── [ ] pkg/pics - PICS parser
└── [ ] Basic unit tests

Week 2:
├── [ ] pkg/transport/tls - TLS configuration
├── [ ] pkg/transport/framing - Message framing
└── [ ] Integration: Two processes can exchange CBOR messages
```

**Milestone:** Two Go processes can establish TLS connection and exchange framed CBOR messages.

### Phase 2: Transport Complete (Weeks 3-4)

```
Week 3:
├── [ ] pkg/transport/keepalive - Keep-alive
├── [ ] pkg/transport/connection - Connection manager
└── [ ] Connection state machine tests

Week 4:
├── [ ] pkg/connection/reconnect - Reconnection
├── [ ] pkg/failsafe - Failsafe timer
└── [ ] Integration: Connection loss detection and recovery
```

**Milestone:** Connection loss detected within 95 seconds, automatic reconnection with backoff.

### Phase 3: Commissioning (Weeks 5-6)

```
Week 5:
├── [ ] pkg/pase/spake2 - SPAKE2+ core
├── [ ] pkg/pase/setupcode - Setup code handling
└── [ ] Unit tests with test vectors

Week 6:
├── [ ] pkg/pase/window - Commissioning window
├── [ ] pkg/cert - Certificate generation
└── [ ] Integration: Full commissioning flow
```

**Milestone:** Controller can commission a device using setup code and issue operational certificate.

### Phase 4: Multi-Zone (Weeks 7-8)

```
Week 7:
├── [ ] pkg/zone - Zone management
├── [ ] pkg/zone/resolution - Multi-zone resolution
└── [ ] Unit tests for resolution

Week 8:
├── [ ] pkg/cert/renewal - Certificate renewal
├── [ ] pkg/subscription - Subscription management
└── [ ] Integration: Multiple zones on one device
```

**Milestone:** Device accepts multiple zones, correctly resolves limits, handles subscriptions.

### Phase 5: Discovery (Weeks 9-10)

```
Week 9:
├── [ ] pkg/discovery/advertiser - mDNS server
├── [ ] pkg/discovery/browser - mDNS client
└── [ ] TXT record handling

Week 10:
├── [ ] pkg/duration - Duration timers
├── [ ] End-to-end integration
└── [ ] Full pairing/connection flow
```

**Milestone:** Controller discovers device via mDNS, commissions, maintains connection.

### Phase 6: Test Harness (Weeks 11-12)

```
Week 11:
├── [ ] internal/testharness/loader - Test case loader
├── [ ] internal/testharness/engine - Test engine
└── [ ] Basic test execution

Week 12:
├── [ ] internal/testharness/mock - Mock device/controller
├── [ ] internal/testharness/assertions - Assertion framework
├── [ ] internal/testharness/reporter - Reporting
└── [ ] Run tests against reference implementation
```

**Milestone:** Test harness can execute test cases from YAML against reference implementation.

### Phase 7: Device Model & Features (Weeks 13-16)

```
Weeks 13-14:
├── [ ] pkg/device - Device model
├── [ ] DeviceInfo, Electrical, Measurement features
└── [ ] Basic Read/Write operations

Weeks 15-16:
├── [ ] EnergyControl feature
├── [ ] Status feature
├── [ ] State machines (ControlState, ProcessState)
└── [ ] Full integration tests
```

**Milestone:** Reference device with EnergyControl, limits, and state machines working.

### Phase 8: Polish (Weeks 17-18)

```
Week 17:
├── [ ] cmd/mash-device - Reference device CLI
├── [ ] cmd/mash-controller - Reference controller CLI
└── [ ] cmd/mash-test - Test runner CLI

Week 18:
├── [ ] Documentation
├── [ ] Example configurations
├── [ ] CI/CD setup
└── [ ] Release preparation
```

**Milestone:** Usable reference implementation with CLI tools.

---

## Test Coverage Goals

### Unit Test Coverage

| Package | Target |
|---------|--------|
| pkg/wire | 90%+ |
| pkg/pics | 90%+ |
| pkg/transport | 85%+ |
| pkg/pase | 90%+ |
| pkg/cert | 85%+ |
| pkg/zone | 90%+ |
| pkg/connection | 80%+ |
| pkg/failsafe | 90%+ |
| pkg/subscription | 85%+ |
| pkg/discovery | 80%+ |

### Protocol Test Coverage

| Test Suite | Test Cases | From Spec |
|------------|------------|-----------|
| TC-CONN | 5+ | connection-state-machine.md |
| TC-KEEPALIVE | 4+ | failsafe-timing.md |
| TC-FRAME | 6+ | message-framing.md |
| TC-PASE | 5+ | commissioning-pase.md |
| TC-ZONE | 13+ | multi-zone-resolution.md |
| TC-STATE | 15+ | state-machines.md |
| TC-SUB | 12+ | subscription-semantics.md |
| TC-DISC | 6+ | discovery.md |
| TC-TLS | 18+ | tls-profile.md |

---

## Dependencies

### Go Standard Library

- `crypto/tls` - TLS 1.3
- `crypto/x509` - Certificate handling
- `crypto/ecdsa` - Key generation
- `crypto/sha256` - Hashing
- `net` - TCP networking

### External Libraries

| Library | Purpose | License |
|---------|---------|---------|
| `github.com/fxamacker/cbor/v2` | CBOR encoding/decoding | MIT |
| `github.com/hashicorp/mdns` | mDNS (or alternative) | MIT |
| `github.com/stretchr/testify` | Test assertions | MIT |
| `gopkg.in/yaml.v3` | YAML parsing for test cases | Apache 2.0 |

### SPAKE2+ Considerations

- Go doesn't have a standard SPAKE2+ library
- Options:
  1. Use `github.com/cloudflare/circl` (has SPAKE2 support)
  2. Implement from RFC 9382
  3. Use CGo with a C library

---

## Success Criteria

### Specification Validation

- [ ] All 178+ test cases pass against reference implementation
- [ ] PICS examples (minimal, full-featured) can be validated
- [ ] No ambiguities discovered that require spec changes
- [ ] Timing requirements met (95s detection, failsafe accuracy)

### Implementation Quality

- [ ] All packages have unit tests with >80% coverage
- [ ] No race conditions (verified with `-race`)
- [ ] Memory usage acceptable on constrained devices
- [ ] Clean API suitable as reference

### Interoperability

- [ ] Reference device and controller work together
- [ ] Mock device can test third-party controllers
- [ ] Mock controller can test third-party devices

---

## Related Documents

| Document | Description |
|----------|-------------|
| [Protocol Overview](protocol-overview.md) | Architecture and device model |
| [Transport](transport.md) | TLS, framing, keep-alive |
| [Security](security.md) | Certificates, commissioning, zones |
| [Discovery](discovery.md) | mDNS, QR codes |
| [Testing README](testing/README.md) | Test specification approach |
| [PICS Format](testing/pics-format.md) | PICS code format |
| [Conformance Rules](conformance/README.md) | Attribute conformance |

### Behavior Specifications

| Spec | Description |
|------|-------------|
| [TLS Profile](testing/behavior/tls-profile.md) | TLS 1.3 requirements |
| [Connection State Machine](testing/behavior/connection-state-machine.md) | Connection lifecycle |
| [Message Framing](testing/behavior/message-framing.md) | CBOR encoding rules |
| [Commissioning PASE](testing/behavior/commissioning-pase.md) | SPAKE2+ flow |
| [Discovery](testing/behavior/discovery.md) | mDNS records |
| [Zone Lifecycle](testing/behavior/zone-lifecycle.md) | Zone management |
| [Multi-Zone Resolution](testing/behavior/multi-zone-resolution.md) | Limit/setpoint resolution |
| [State Machines](testing/behavior/state-machines.md) | ControlState/ProcessState |
| [Failsafe Timing](testing/behavior/failsafe-timing.md) | Failsafe timer behavior |
| [Subscription Semantics](testing/behavior/subscription-semantics.md) | Notification behavior |
| [Duration Semantics](testing/behavior/duration-semantics.md) | Timer behavior |
| [Feature Interactions](testing/behavior/feature-interactions.md) | Cross-feature behavior |
