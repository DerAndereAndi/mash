# Matter Test Infrastructure Analysis & MASH Design Lessons

**Date:** 2026-02-10
**Context:** Running MASH base-protocol tests with `-shuffle` exposed order-dependent
failures. This document analyzes Matter's test infrastructure to inform a proper
design for test isolation in MASH.

---

## 1. Problem: Why Shuffled Tests Fail

Running 248 base-protocol tests in random order (within precondition levels) exposed
5 categories of failures that all share a root cause: **the test harness has no formal
model of device state between tests**.

| # | Category | Example | What Happens |
|---|----------|---------|-------------|
| 1 | Phantom sockets | TC-PASE-002 → TC-COMM-001 | Failed PASE leaves TLS socket open (`connected=false`, socket alive). Socket holds device's commissioning lock. Next test gets "commissioning already in progress." |
| 2 | Aggressive teardown | Any L3 test → TC-CERT-VAL-CTRL-005 | Runner's `closeActiveZoneConns` sends explicit `RemoveZone` between tests. Device deletes zone → `zoneCount == 0` → `EnterCommissioningMode` regenerates self-signed cert. Next test expecting operational cert fails. Note: mere disconnection does NOT remove zones -- it's the runner's teardown that's too aggressive. |
| 3 | Stale device state | Any multi-zone test → TC-COMM-WINDOW-002 | Previous test left zones on device. Window test assumed clean state (0 zones). |
| 4 | Lock timing race | Phantom close → rapid next test | Socket closed but device needs >2ms to release commissioning lock. Tests within 5ms all fail. |
| 5 | Reconnect timing | TC-INFLIGHT-002 | Disconnect + immediate reconnect too fast for device-side session cleanup. |

Band-aid fixes (per-test `fresh_commission: true`, string-matching retries) were
attempted and reverted. They don't scale -- each new failure needs a new band-aid.

---

## 2. How Matter Solves This

### 2.1 One-Time Commissioning Per Suite

The single most impactful design decision in Matter's test infrastructure:

```
BEFORE ANY TESTS RUN:
  1. User puts DUT in commissioning mode
  2. Test harness performs PASE (SPAKE2+)
  3. Device attestation + certificate installation
  4. CASE (operational TLS) session established
  → This session persists for ALL tests in the suite

EACH TEST:
  setup_test()    → Lightweight: prime specific attributes, clear flags
  test_*()        → Runs against the persistent commissioned session
  teardown_test() → Lightweight: undo this test's mutations
```

The test harness user guide states:
> "The DUT will be commissioned, with the commissioning being kept throughout
> the execution of all its tests."

**Why this works:** Commissioning is expensive (~1.5s for PASE + cert exchange +
operational reconnect). Doing it once eliminates the entire class of commissioning
lock race conditions. Tests assume they start with a valid operational session.

**Optional fallback:** For maximum isolation, operators can enable
`factory-reset-between-tests` mode. This is slower but provides a clean slate
when debugging state contamination issues.

### 2.2 Test Independence via setup_test / teardown_test

Matter uses Google's Mobly framework which provides per-test lifecycle hooks:

```python
class TC_Example(MatterBaseTest):
    def setup_test(self):
        """Called before each test method -- ensure known state."""
        # e.g., turn light off, clear pending events

    def test_TC_EXAMPLE_1_1(self):
        """The actual test."""
        # Runs against already-commissioned device

    def teardown_test(self):
        """Called after each test method -- undo mutations."""
        # e.g., remove ACL entries added during test
```

Key principle: **each test is responsible for cleaning up after itself**.
The framework does not magically reset device state -- tests must explicitly
undo their mutations. This is pragmatic: full device reset is too slow, so
tests cooperate by being good citizens.

### 2.3 PICS-Based Filtering (Not Dependency DAGs)

Matter does NOT express test dependencies ("TC-A must run before TC-B").
Instead:

- **PICS guards** skip tests whose required capabilities aren't present
- Tests are assumed **independent** -- any test can run at any time
- Execution order is not guaranteed (Mobly discovers tests via introspection)

```python
def pics_TC_IDM_1_3(self) -> list[str]:
    return ["PICS.IDM.C.ATTRIBUTE_LIST"]  # Skip if not supported

# Step-level PICS in YAML:
steps:
  - step: 1
    command: ReadAttribute
    pics: "PICS.S.A0007"  # Skip step if attribute unsupported
```

PICS replaces ordering constraints. Instead of "run commissioning test first",
a test declares "I need a commissioned device" and the suite setup provides it.

### 2.4 Error Classification

Matter classifies errors into three buckets that determine handling:

| Bucket | Examples | Action |
|--------|----------|--------|
| **Infrastructure** | Network timeout, CASE expired, DNS-SD failure | Retry with backoff |
| **Device** | Unsupported cluster, ACL denied, validation failure | Fail (don't retry) |
| **Test** | Assertion mismatch, unexpected value | Fail (test is wrong) |

This is implemented through Python exception types:
- `ChipStackError` → Infrastructure (controller-level)
- `InteractionModelError` → Device (cluster-level)
- `AssertionError` → Test

The key insight: **retry logic only applies to infrastructure errors**. You never
retry a device rejection or a failed assertion.

### 2.5 TestEventTrigger for State Manipulation

Matter uses a dedicated `TestEventTrigger` command in the GeneralDiagnostics
cluster for test state manipulation:

- Requires an `enableKey` (security gate for test-only functionality)
- Pre-defined trigger codes per device type
- Used to simulate conditions hard to create normally (faults, timeouts, etc.)

```
Controller → TestEventTrigger(enableKey="00112233...", triggerCode=42) → Device
Device → Enters simulated fault state
Test → Reads attribute, verifies fault reported correctly
```

**MASH equivalent:** Already has `enable-key` and TestControl feature with
commands like `setCommissioningWindowDuration` and `triggerResetTestState`.
The infrastructure exists but isn't used systematically between tests.

### 2.6 Container Isolation

Matter's test harness runs each test suite in a fresh SDK container:

```
Test Harness:
├── Frontend (Web UI)
├── Backend (orchestration)
├── Database (PostgreSQL -- persists results)
├── SDK Container (created per suite, destroyed after)
│   └── Python + Matter SDK + chip-tool
└── OTBR Container (optional, Thread testing)
```

The per-suite container ensures no Python state leaks between suites.
Within a suite, tests share the container (and the commissioned session).

---

## 3. What MASH Can Learn

### 3.1 Suite-Level Commissioning

**Current MASH approach:** Commissioning happens lazily when the first level-3
test runs. Subsequent level-3 tests reuse the session if possible.

**Problem:** "If possible" is a complex heuristic (check `paseState.completed`,
`activeZoneConns`, connection liveness). It fails when tests mutate the session.

**Proposed:** Commission once at suite start, before any test runs. The runner
already does this for auto-PICS -- extend it to be the standard model:

```
Runner.Run():
  1. Load + filter test cases
  2. Commission device (suite-level setup)
  3. Auto-PICS discovery
  4. Run all tests against commissioned session
  5. Suite-level teardown
```

This eliminates phantom socket races, commissioning lock contention, and stale
session heuristics for the vast majority of tests. The few tests that need
commissioning-mode or disconnected state can explicitly disconnect, do their
thing, and reconnect.

### 3.2 Explicit Test Cleanup Contract

**Current MASH approach:** `teardownTest()` cleans runner-side state
(subscriptions, phantom sockets, security pool). Device-side state is only
cleaned if `deviceStateModified` is set (triggers `triggerResetTestState`).

**Problem:** Many tests mutate device state without setting `deviceStateModified`:
- Adding zones (zone_handlers)
- Changing commissioning window (trigger_handlers)
- Modifying control state (device_handlers)

**Proposed:** Two-part cleanup contract:

1. **Runner teardown** (automatic): Close extra connections, unsubscribe,
   clear buffers -- already done.

2. **Device state baseline check** (new): After each test, compare current
   device state against a baseline snapshot taken at suite start. If diverged,
   reset via `triggerResetTestState`. This doesn't require tests to be
   "good citizens" -- the framework enforces it.

```go
type DeviceBaseline struct {
    ZoneCount     int
    Commissioned  bool
    ControlState  string
    // ... other observable state
}

func (r *Runner) checkAndResetBaseline(ctx context.Context) error {
    current := r.probeDeviceState(ctx)
    if current != r.baseline {
        r.triggerResetTestState(ctx)
        r.baseline = r.probeDeviceState(ctx) // verify reset worked
    }
    return nil
}
```

### 3.3 Error Classification

**Current MASH approach:** `isTransientError()` matches error strings:
```go
strings.Contains(msg, "EOF") ||
strings.Contains(msg, "connection reset") ||
strings.Contains(msg, "broken pipe") || ...
```

**Problem:** Fragile, incomplete (missed "commissioning already in progress"),
and doesn't distinguish infrastructure vs device errors.

**Proposed:** Classify at the source:

```go
// Transport layer wraps errors
type TransportError struct {
    Err      error
    Category ErrorCategory  // Infrastructure, Device, Test
}

// PASE layer wraps errors
type PASEError struct {
    Err      error
    Code     uint8          // PASE error code from device
    Category ErrorCategory
}
```

Then retry logic becomes type-based:
```go
if err, ok := err.(*TransportError); ok && err.Category == ErrInfrastructure {
    // retry with backoff
}
```

### 3.4 Connection State Machine

**Current MASH approach:** Connection state is tracked implicitly via
`conn.connected`, `paseState`, `paseState.completed`, `conn.tlsConn != nil`.
Phantom sockets emerge when these flags disagree.

**Proposed:** Explicit state machine:

```
DISCONNECTED ──dial()──→ TLS_CONNECTED
                              │
                    paseHandshake()──→ PASE_ESTABLISHED
                                           │
                                  certExchange()──→ COMMISSIONED
                                                       │
                                              reconnect()──→ OPERATIONAL

Any state ──error/close()──→ DISCONNECTED
```

Each state has a single valid cleanup path. Transitions are validated
(can't go from DISCONNECTED to COMMISSIONED without intermediate steps).
The phantom socket problem vanishes because every non-DISCONNECTED state
has an explicit `close()` path.

### 3.5 Richer Precondition Declarations

**Current MASH approach:** 4 levels (none, commissioning, connected, commissioned)
plus ~30 simulation keys stored in execution state.

**Problem:** Level 3 is too coarse. "Needs commissioned device" doesn't express
"needs no extra zones" or "needs operational cert (not self-signed)".

**Proposed:** Keep the 4 levels for sorting but add **state assertions** that
the runner verifies before starting a test:

```yaml
preconditions:
  - device_commissioned: true      # Level 3 (sorting)
  - zone_count: 0                  # Verified: no leftover zones
  - cert_state: operational        # Verified: using zone-CA cert
```

If the assertion fails, the runner takes corrective action (close extra zones,
recommission, etc.) rather than running the test in the wrong state.

---

## 4. Comparison Table

| Aspect | Matter | MASH Current | MASH Proposed |
|--------|--------|-------------|---------------|
| Commissioning | Once per suite | Lazy on first L3 test | Once at suite start |
| Session persistence | All tests share session | Reused if consecutive L3 | All L3 tests share session |
| Test cleanup | setup_test/teardown_test per test | teardownTest (runner-side only) | teardownTest + baseline check |
| State model | Implicit (tests are good citizens) | No model | Explicit DeviceBaseline |
| Error classification | 3 buckets (infra/device/test) | String matching | Typed errors at source |
| Connection lifecycle | Controller API manages sessions | Manual with edge cases | State machine |
| Preconditions | PICS-based filtering | 4 levels + simulation keys | 4 levels + state assertions |
| Test ordering | No guarantees, assumed independent | Sorted by level, optional shuffle | Sorted by level, shuffle-safe |
| Factory reset mode | Optional per-suite config | Not available | Optional per-suite config |

---

## 5. Implementation Priorities

### Phase 1: Eliminate Shuffle Failures (Immediate)

These changes directly address the 5 failure categories from Section 1:

1. **Connection state machine** -- Formalize states, eliminate phantom sockets.
   Addresses categories 1 and 4.

2. **Suite-level commissioning** -- Commission once before tests, not lazily.
   Addresses category 4 (no commissioning lock contention between tests).

3. **Baseline device state check** -- After each test, verify device state
   matches baseline. Reset via TestEventTrigger if diverged.
   Addresses categories 2, 3, 5.

### Phase 2: Error Handling Quality

4. **Typed error classification** -- Replace `isTransientError` string matching
   with typed errors at transport/PASE layer.

5. **Infrastructure retry with backoff** -- Only retry infrastructure errors,
   with exponential backoff (not fixed delays).

### Phase 3: Richer Declarations

6. **State assertions in preconditions** -- Tests declare what device state
   they need, runner verifies and corrects.

7. **Optional factory-reset mode** -- For debugging, allow full device reset
   between tests (slow but maximum isolation).

---

## Sources

- [Matter Test-Harness User Manual v2.13](https://github.com/project-chip/certification-tool/blob/main/docs/Matter_TH_User_Guide/Matter_TH_User_Guide.adoc)
- [Matter Python Testing Framework](https://project-chip.github.io/connectedhomeip-doc/testing/python.html)
- [ChipDeviceCtrl API](https://project-chip.github.io/connectedhomeip-doc/testing/ChipDeviceCtrlAPI.html)
- [Matter PICS and PIXITs](https://project-chip.github.io/connectedhomeip-doc/testing/pics_and_pixit.html)
- [Matter Test Event Triggers (Nordic)](https://docs.nordicsemi.com/bundle/ncs-latest/page/nrf/protocols/matter/end_product/test_event_triggers.html)
- [YAML Test Suites README](https://github.com/project-chip/connectedhomeip/blob/master/src/app/tests/suites/README.md)
- [Google Mobly Framework](https://github.com/google/mobly)
- [Matter SDK Repository](https://github.com/project-chip/connectedhomeip)
