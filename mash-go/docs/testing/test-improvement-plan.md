# Test Improvement Plan: Making mash-test Work

**Current state:** 4/500 pass (0.8%)
**Target:** Fix infrastructure so tests that CAN pass DO pass against the reference device.

---

## Failure Analysis

| Category | Count | Root Cause |
|----------|-------|------------|
| A. Precondition PASE failure | ~280 | `ensureCommissioned()` PASE handshake gets EOF -- device closes connection |
| B. Handler output key mismatches | ~120 | Handlers return different keys than test YAMLs expect |
| C. TLS certificate trust | ~20 | Operational connections fail with `x509: unknown authority` |
| D. PASE failures in test steps | ~15 | Commission step (not precondition) gets EOF/broken pipe |
| E. Environment mismatches | ~10 | Test expects state the device isn't in |

All 500 tests have PICS requirements. The issue is NOT missing PICS filtering -- it is infrastructure gaps.

---

## Phase 1: Fix PASE Commissioning (unblocks ~295 tests -- Categories A + D)

### 1.1 Add `--test-mode` flag to mash-device

[ ] **Add flag to CLI** (`cmd/mash-device/main.go`)
  - Add `--test-mode` boolean flag (default false)
  - Pass it through `DeviceConfig.TestMode`

[ ] **Add TestMode to DeviceConfig** (`pkg/service/types.go`)
  - Add `TestMode bool` field to `DeviceConfig`

[ ] **Auto-reopen commissioning window after successful commission** (`pkg/service/device_service.go`)
  - In `handleCommissioningConnection()`, after `ExitCommissioningMode()` (line 588), if `TestMode` is true, schedule `EnterCommissioningMode()` after a short delay (e.g., 1 second) to allow the device to accept the next PASE attempt
  - This must happen AFTER the operational session is established so it doesn't interfere

[ ] **Disable PASE backoff in test mode** (`pkg/service/device_service.go`)
  - In `Start()`, when `TestMode` is true, skip creating `PASEAttemptTracker` (or set all tiers to 0ms)

[ ] **Extend commissioning window in test mode** (`pkg/service/device_service.go`)
  - When `TestMode` is true, set `CommissioningWindowDuration` to 24 hours (or skip the timer entirely)

[ ] **Disable connection cooldown in test mode** (`pkg/service/device_service.go`)
  - In `acceptCommissioningConnection()` (line 1982), skip the 500ms cooldown check when `TestMode` is true

**Tests:**
[ ] Unit test: `TestDeviceService_TestMode_ReopensWindow`
[ ] Unit test: `TestDeviceService_TestMode_NoBackoff`
[ ] Integration: start mash-device with `--test-mode`, commission twice in sequence

### 1.2 Fix test runner PASE lifecycle

[ ] **Return output map on commission failure** (`internal/testharness/runner/pase.go`)
  - In `handleCommission()`, instead of `return nil, err` on PASE failure, return:
    ```go
    return map[string]any{
        "session_established": false,
        "commission_success":  false,
        "error":               err.Error(),
    }, nil
    ```
  - This lets tests assert on `commission_success: false` without the step short-circuiting

[ ] **Add `commission_success` alias to success path** (`internal/testharness/runner/pase.go`)
  - On success, add `"commission_success": true` alongside `"session_established": true`

[ ] **Handle broken connection in precondition setup** (`internal/testharness/runner/preconditions.go`)
  - In `ensureCommissioned()`, if PASE fails with EOF, disconnect and retry once (the device may need a fresh TLS connection)

**Tests:**
[ ] Unit test: commission handler returns output map on failure
[ ] Unit test: commission handler returns `commission_success` on success

---

## Phase 2: Fix Handler Output Key Mismatches (unblocks ~120 tests -- Category B)

### 2.1 Dispatcher handlers: add `action_triggered`

**Problem:** `device_local_action` and `controller_action` are dispatchers. No sub-handler ever outputs `action_triggered: true`. At least 15 tests expect it.

[ ] **Add `action_triggered` to dispatcher output** (`internal/testharness/runner/device_handlers.go`)
  - In `handleDeviceLocalAction()`, after the sub-handler returns successfully, merge `"action_triggered": true` into the output map

[ ] **Same for controller_action** (`internal/testharness/runner/controller_handlers.go`)
  - In `handleControllerAction()`, after successful sub-handler dispatch, merge `"action_triggered": true`

**Tests:**
[ ] Unit test: `device_local_action` with sub_action `factory_reset` returns `action_triggered: true`
[ ] Unit test: `controller_action` with sub_action `remove_device` returns `action_triggered: true`

### 2.2 parse_qr: fix key names and param name

**Problem:** Handler reads param `payload` but tests pass `content`. Handler returns `valid` but tests expect `parse_success`.

[ ] **Accept both `payload` and `content` params** (`internal/testharness/runner/utility_handlers.go`)
  - In `handleParseQR()`, check `params["payload"]` first, fall back to `params["content"]`

[ ] **Add `parse_success` alias** (`internal/testharness/runner/utility_handlers.go`)
  - On success: add `"parse_success": true` alongside `"valid": true`
  - On failure: add `"parse_success": false` alongside `"valid": false`

**Tests:**
[ ] Unit test: parse_qr with `content` param works
[ ] Unit test: parse_qr returns `parse_success`

### 2.3 browse_mdns: add TXT field and count outputs

**Problem:** Handler returns `{device_found, service_count, services}` but tests expect `txt_field_*`, `devices_found`, `controllers_found`, `instances_for_device`, `error_code`.

[ ] **Flatten TXT records into output** (`internal/testharness/runner/discovery_handlers.go`)
  - In `handleBrowseMDNS()`, after collecting services, iterate TXT records of the first matching service and add `txt_field_<key>: <value>` to the output map
  - Add `txt_D_range` if the `D` (discriminator) TXT field is present

[ ] **Add count-style outputs** (`internal/testharness/runner/discovery_handlers.go`)
  - Add `devices_found: len(services)` (integer, not bool)
  - Add `controllers_found: <count>` when browsing commissioner services
  - Add `instances_for_device: <count>` when filtering by device ID

[ ] **Add instance name outputs** (`internal/testharness/runner/discovery_handlers.go`)
  - Add `instance_name: <first match>` when a single service is found

**Tests:**
[ ] Unit test: browse_mdns returns `txt_field_*` keys
[ ] Unit test: browse_mdns returns `devices_found` count

### 2.4 connect handler: fix value formats

**Problem:** Tests expect `"1.3"` but handler returns `"TLS 1.3"`. Tests expect `"secp256r1"` but handler returns `"P-256"`. Tests expect `"protocol_version"` but handler returns `"protocol version"`.

[ ] **Fix negotiated_version format** (`internal/testharness/runner/runner.go`)
  - Change `tlsVersionName()` to return `"1.3"`, `"1.2"`, etc. instead of `"TLS 1.3"`, `"TLS 1.2"`

[ ] **Fix curve name format** (`internal/testharness/runner/runner.go`)
  - Change `curveIDName()` to return both standard and OpenSSL names, or update tests to use `"P-256"`
  - Decision: use `"P-256"` (IANA standard) and update test YAMLs to match

[ ] **Fix TLS alert format** (`internal/testharness/runner/runner.go`)
  - Change `extractTLSAlert()` to return underscore-separated names: `"protocol_version"`, `"no_application_protocol"`, etc.

**Tests:**
[ ] Unit test: `tlsVersionName(tls.VersionTLS13)` returns `"1.3"`
[ ] Unit test: `extractTLSAlert()` returns underscore format

### 2.5 Other handler key aliases

[ ] **verify_mdns_not_advertising: add `not_advertising`** (`internal/testharness/runner/discovery_handlers.go`)
  - Add `"not_advertising": !advertising` to the output alongside `"advertising"`

[ ] **verify_mdns_not_browsing: add `not_browsing`** (`internal/testharness/runner/discovery_handlers.go`)
  - Same pattern: `"not_browsing": !browsing`

[ ] **controller_action delete_zone: add `zone_deleted` alias** (`internal/testharness/runner/zone_handlers.go`)
  - In `handleDeleteZone()`, add `"zone_deleted": true` alongside `"zone_removed": true`

[ ] **controller_action create_zone: add detail keys** (`internal/testharness/runner/zone_handlers.go`)
  - In `handleCreateZone()`, add `"zone_id_present": zoneID != ""` and `"zone_id_length": len(zoneID)`

[ ] **open_commissioning_connection: ensure rejection keys on failure** (`internal/testharness/runner/security_handlers.go`)
  - Already returns `rejection_at_tls_level` on failure -- verify this path works correctly for TC-SEC-CONN-001

[ ] **verify_controller_cert: add missing keys** (`internal/testharness/runner/controller_handlers.go`)
  - Add `"cert_present"`, `"signed_by_zone_ca"`, `"not_expired"` from certificate inspection

**Tests:**
[ ] Unit test for each alias addition

---

## Phase 3: Fix TLS Trust for Operational Connections (unblocks ~20 tests -- Category C)

**Problem:** Tests like TC-CONN-001, TC-MULTI-001/002 try operational TLS connections but fail because the runner doesn't trust the device's Zone CA certificate.

### 3.1 Store Zone CA during commissioning

[ ] **Save Zone CA cert after successful PASE** (`internal/testharness/runner/pase.go`)
  - After `handleCommission()` succeeds, store the Zone CA certificate in runner state so subsequent `connect` calls can use it for TLS verification

### 3.2 Use Zone CA for operational connections

[ ] **Configure TLS trust from stored Zone CA** (`internal/testharness/runner/runner.go`)
  - In `handleConnect()`, when not in commissioning mode and a Zone CA is available, add it to the TLS `RootCAs` pool instead of using `InsecureSkipVerify`

[ ] **Same for connect_as_zone** (`internal/testharness/runner/connection_handlers.go`)
  - Use stored Zone CA for TLS in `handleConnectAsZone()`

**Tests:**
[ ] Integration test: commission then connect operationally succeeds

---

## Phase 4: Fix Environment/State Mismatches (unblocks ~10 tests -- Category E)

### 4.1 Fix setup code handling

[ ] **TC-SEC-BACKOFF-003: fix setup code in test YAML** (`testdata/cases/commissioning-security-backoff.yaml`)
  - The test passes an invalid setup code format. Either fix the YAML to use a valid 8-digit code, or update the test to expect the validation error

### 4.2 Fix tests that assume device is NOT advertising

[ ] **TC-NOTFOUND-001, TC-NOTFOUND-002: add precondition or skip** (`testdata/cases/discovery-behavior-tests.yaml`)
  - These tests expect `device_found: false` but the device IS advertising. They need either:
    (a) A precondition that ensures the device is NOT in commissioning mode, OR
    (b) A `device_local_action` to exit commissioning mode before running

[ ] **TC-DSTATE-002, TC-MASHC-002: timing-dependent tests** (`testdata/cases/`)
  - These expect the commissioning window to close after a timeout. In `--test-mode` the window stays open. Either skip these in test-mode or add a mechanism to temporarily close the window

---

## Phase 5: Design TestEventTrigger (future -- Matter-inspired)

This is the architectural investment for long-term test automation. Based on Matter's pattern:

### 5.1 Define TestControl feature

[ ] **Add TestControl feature YAML** (`docs/features/test-control/1.0.yaml`)
  - Feature on endpoint 0 (DEVICE_ROOT)
  - Command: `TriggerTestEvent(enableKey: bytes[16], eventTrigger: uint64)`
  - Attribute: `testEventTriggersEnabled: bool` (read-only)

[ ] **Define trigger opcodes per domain**
  - `0x0001_xxxx`: Commissioning (enter/exit commissioning mode, factory reset)
  - `0x0003_xxxx`: Electrical (simulate phase changes)
  - `0x0004_xxxx`: Measurement (simulate power values, SoC)
  - `0x0005_xxxx`: EnergyControl (simulate state transitions)
  - `0x0006_xxxx`: ChargingSession (EV plug/unplug, charge demand)
  - `0x0007_xxxx`: Tariff (simulate price signals)
  - `0x0008_xxxx`: Signals (simulate constraint signals)

### 5.2 Implement in mash-device

[ ] **Add TestEventTrigger handler to DeviceService**
  - Gate on `enableKey` match
  - Dispatch to per-feature handlers registered at startup
  - Only available when `--test-mode` or `--enable-key` is set

### 5.3 Add test harness handler

[ ] **Add `trigger_test_event` action handler** (`internal/testharness/runner/`)
  - Sends the TriggerTestEvent command via the operational connection
  - Returns `{trigger_sent: true, event_trigger: uint64}`

### 5.4 Update test YAMLs to use triggers for preconditions

[ ] Replace physical-state preconditions with trigger steps:
  ```yaml
  steps:
    - action: trigger_test_event
      params:
        event_trigger: 0x0006000000000002  # EV plugged in
    - action: read
      params:
        feature: ChargingSession
        attribute: sessionState
      expect:
        value: PLUGGED_IN_NO_DEMAND
  ```

---

## Phase 6: Auto-PICS from Device (future)

[ ] **Add `--auto-pics` flag to mash-test**
  - Connect to device, read endpoint 0 DeviceInfo
  - Enumerate endpoints, features, attributes
  - Build PICS dictionary dynamically
  - Use for test filtering without requiring a .yaml PICS file

---

## Implementation Order and Expected Impact

| Step | Description | Tests Fixed | Cumulative |
|------|-------------|-------------|------------|
| 1.1 | `--test-mode` for mash-device | ~280 (precondition PASE) | ~284 |
| 1.2 | Fix commission handler outputs | ~15 (step-level PASE) | ~299 |
| 2.1 | Add `action_triggered` to dispatchers | ~15 | ~314 |
| 2.2 | Fix parse_qr keys/params | ~6 | ~320 |
| 2.3 | Enrich browse_mdns outputs | ~30 | ~350 |
| 2.4 | Fix connect value formats | ~15 | ~365 |
| 2.5 | Other handler key aliases | ~15 | ~380 |
| 3.1-3.2 | TLS trust for operational connections | ~20 | ~400 |
| 4.1-4.2 | Fix environment mismatches | ~10 | ~410 |

**Expected result after Phases 1-4:** ~410/500 tests addressable. The remaining ~90 will need TestEventTrigger (Phase 5) or other device-side changes to simulate physical states (EV plugging, faults, network partitions, etc.).

---

## Notes

- All test YAMLs already have PICS requirements. No PICS additions needed.
- Handler changes are additive (new keys alongside existing ones) -- no existing tests break.
- The `--test-mode` flag is device-side only. The test runner doesn't need to know about it.
- Value format changes (TLS version, curve names, alert names) need corresponding test YAML updates if we choose to keep the handler format and update tests instead.
- Matter's TestEventTrigger is the gold standard for DUT state management. Phase 5 makes this a first-class protocol feature rather than a testing hack.
