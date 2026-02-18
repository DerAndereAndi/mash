# Stabilize base-protocol Test Suite: Group-by-Group Strategy

## Context

The base-protocol test suite has ~252 tests. Goal: **every base-protocol test passes 5 consecutive times in both sequence and shuffle mode**.

Three root causes dominate failures:
1. **Environmental** -- tests requiring multiple devices or no-device advertising (~11 tests, unsolvable with single device)
2. **State leakage** -- incomplete cleanup between tests causes cascading failures
3. **Device/harness bugs** -- real issues in test logic or device behavior

## Approach: Infrastructure-First + Group-by-Group

Fix cross-cutting cleanup gaps first (benefits all groups), then work group-by-group from most-stable to least-stable.

## Runner Refactor Track (TDD)

A concrete, PR-sliced TDD refactor plan for lifecycle determinism is documented in:

- `docs/design/runner-lifecycle-tdd-refactor-plan.md`

Use that plan in parallel with stabilization runs. Keep behavior changes behind a strict-mode flag until Phase 1 is stable.

## Phase 0: Infrastructure Hardening

Fix cleanup gaps that affect all groups. TDD for each fix.

### 0.1 Tag environmental tests
- Add `env:multi-device` tag to tests requiring 2+ devices or no-device conditions
- Tests: TC-BROWSE-001/002/003/006, TC-DISC-003, TC-MASHC-005, TC-ZONE-ADD-004, TC-NOTFOUND-001/003, TC-MASHD-006, TC-MASHO-004
- Run with `-exclude-tags env:multi-device` going forward

### 0.2 Clean pairing advertiser in teardown
- **File**: `runner/runner.go` (teardownTest)
- **Issue**: `pairingAdvertiser` only cleaned in `Runner.Close()`, leaks mDNS ads between tests
- **Fix**: Stop and nil `r.pairingAdvertiser` in `teardownTest()`

### 0.3 Clear suite zone pending notifications
- **File**: `runner/runner.go` or coordinator teardown
- **Issue**: Suite zone `pendingNotifications` not cleared (suite conn is outside pool)
- **Fix**: Clear `suite.Conn().pendingNotifications` in teardown

### 0.4 Verify: run full suite 3x, confirm no regressions from cleanup changes

## Test Groups

| # | Group | Filter | Count | Failing | Priority |
|---|-------|--------|-------|---------|----------|
| 1 | Protocol & Framing | `TC-PROTO*,TC-MSG*,TC-FRAME*,TC-CBOR*,TC-INT*,TC-NULL*,TC-COMPAT*` | ~25 | 0 | Verify |
| 2 | Zone Sim | `TC-ZONE-MGT*,TC-ZONE-CREATE*` | ~12 | 0 | Verify |
| 3 | Commissioning | `TC-COMM-00*,TC-PASE*` | ~8 | 0 | Verify |
| 4 | Admin | `TC-ADMIN*` | ~4 | 0 | Verify |
| 5 | QR | `TC-QR*` | ~6 | 0 | Verify |
| 6 | TLS Profile | `TC-TLS*` | ~25 | 0 | Verify |
| 7 | Connection Basic | `TC-CONN-00*,TC-CLOSE*,TC-KEEPALIVE*,TC-RECONN*` | ~17 | 0 | Verify |
| 8 | Connection Cap/Busy | `TC-CONN-CAP*,TC-CONN-BUSY*` | ~7 | 0 | Verify |
| 9 | Zone Wire | `TC-ZONE-REMOVE*,TC-ZONE-ADD*,TC-ZTYPE*,TC-ZONE-010` | ~17 | 0-1 | Low |
| 10 | Cert Validation | `TC-CERT-VAL*` | ~11 | 2 | Medium |
| 11 | Cert Renewal | `TC-CERT-RENEW*,TC-SEC-NONCE*` | ~7 | 0 | Verify |
| 12 | Discovery & mDNS | `TC-DISC*,TC-DSTATE*,TC-MASHO*,TC-MASHC*,TC-MASHD*,TC-BROWSE*,TC-NOTFOUND*,TC-MDNS-REC*,TC-MULTIIF*` | ~46 | 10-19 | **High** |
| 13 | Transitions & E2E | `TC-TRANS*,TC-E2E*,TC-IPV6*` | ~12 | 2-4 | Medium |
| 14 | Security & Backoff | `TC-SEC-*` | ~11 | 2-3 | Medium |
| 15 | Subscriptions | `TC-SUB*` | ~17 | 2 | Medium |
| 16 | Multi-Zone Conn | `TC-MULTI*` | ~4 | 1-2 | Medium |
| 17 | Conn Reaper | `TC-CONN-REAP*` | ~3 | 1-3 | Medium |
| 18 | Comm Window | `TC-COMM-WINDOW*,TC-COMM-ALPN*` | ~7 | 2 | Medium |

## Phase 1: Verify Stable Groups (Groups 1-8, 11)

Run each 5x sequential + 5x shuffled. Confirm 100% pass rate. No code changes expected.

If any fail, investigate before proceeding -- these are supposed to be solid.

### Phase 1 Execution Runbook (step-by-step)

Use this exact progression:
1. Verify each Phase 1 group alone
2. Verify pairwise group combinations
3. Verify incremental accumulated combinations
4. Verify full Phase 1 set

Only advance when the current step is fully green.

### Tool Calls / Commands

#### A) Start device (Terminal 1)

```bash
cd mash-go

go run ./cmd/mash-device \
  -type evse \
  -port 8443 \
  -setup-code 20220211 \
  -discriminator 1234 \
  -simulate \
  -enable-key deadbeefdeadbeefdeadbeefdeadbeef \
  -state-dir /tmp/mash-device-state \
  -reset \
  -log-level debug
```

#### B) Test command template (Terminal 2)

```bash
cd mash-go

BASE_ARGS=(
  -target localhost:8443
  -mode device
  -setup-code 20220211
  -enable-key deadbeefdeadbeefdeadbeefdeadbeef
  -tags base-protocol
  -exclude-tags env:multi-device
  -json
  -debug
)
```

#### B.1) Hard Isolation Rule (mandatory)

Every `mash-test` invocation must run against a freshly reset `mash-device`.
Do not run `go run ./cmd/mash-test ...` directly for Phase 1 matrix work.
Use:

```bash
cd mash-go
./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "<FILTER>"
```

#### C) Group filter lookup (Phase 1 only)

```bash
G1='TC-PROTO*,TC-MSG*,TC-FRAME*,TC-CBOR*,TC-INT*,TC-NULL*,TC-COMPAT*'
G2='TC-ZONE-MGT*,TC-ZONE-CREATE*'
G3='TC-COMM-00*,TC-PASE*'
G4='TC-ADMIN*'
G5='TC-QR*'
G6='TC-TLS*'
G7='TC-CONN-00*,TC-CLOSE*,TC-KEEPALIVE*,TC-RECONN*'
G8='TC-CONN-CAP*,TC-CONN-BUSY*'
G11='TC-CERT-RENEW*,TC-SEC-NONCE*'
```

### Step 1: Per-group gate (individual groups)

For each group `Gi` in `{G1,G2,G3,G4,G5,G6,G7,G8,G11}`:

1) Sequential gate: run 5 times
```bash
for i in 1 2 3 4 5; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$Gi"
done
```

2) Shuffled gate: run 5 times with fixed seeds
```bash
for seed in 101 202 303 404 505; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$Gi" -shuffle -shuffle-seed "$seed"
done
```

Acceptance: all 10 runs pass for the group.

### Step 2: Pairwise combinations

Start with high-risk pairings first, then adjacent pairings.

Suggested initial pairs:
- `G7,G8` (Connection Basic + Connection Cap/Busy)
- `G3,G6` (Commissioning + TLS)
- `G1,G7` (Protocol/Framing + Connection)

For each pair filter `P='(<A>),(<B>)'` (comma-separated glob union):

1) Sequential gate: 3 runs
```bash
for i in 1 2 3; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$P"
done
```

2) Shuffled gate: 3 runs
```bash
for seed in 111 222 333; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$P" -shuffle -shuffle-seed "$seed"
done
```

Acceptance: all 6 runs pass for each pair.

### Step 3: Incremental accumulated combinations

Build up progressively:
- `A1 = G1`
- `A2 = G1+G2`
- `A3 = G1+G2+G3`
- ...
- `A9 = G1+G2+G3+G4+G5+G6+G7+G8+G11`

For each accumulated filter `Ak`:

1) Sequential gate: 3 runs
```bash
for i in 1 2 3; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$Ak"
done
```

2) Shuffled gate: 3 runs
```bash
for seed in 123 234 345; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$Ak" -shuffle -shuffle-seed "$seed"
done
```

Acceptance: all 6 runs pass for each accumulation level.

### Step 4: Full Phase 1 acceptance

Run all Phase 1 groups together:

1) Sequential: 5 runs
```bash
for i in 1 2 3 4 5; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$A9"
done
```

2) Shuffled: 5 runs
```bash
for seed in 501 502 503 504 505; do
  ./stabilization/run_mash_test_fresh.sh "${BASE_ARGS[@]}" -filter "$A9" -shuffle -shuffle-seed "$seed"
done
```

Acceptance criteria for completing Phase 1:
- No failures in any run above
- No manual retries or ad-hoc restarts required
- Deterministic pass behavior in both sequential and shuffled mode

## Phase 2: Discovery & mDNS (Group 12) -- Highest Priority

After excluding environmental tests (~11), the remaining ~35 discovery tests have two failure patterns:

**Pattern A: Browse returns 0 when expecting 1+ (stale cache / timing)**
- TC-MASHO-001, TC-DSTATE-003, TC-DSTATE-005, TC-MASHO-004, TC-MASHC-003/006
- Root cause: `ClearSnapshot` wipes observer, then WaitFor times out because device re-advertises slowly

**Pattern B: State transition failures (shuffle-dependent)**
- TC-DSTATE-001, TC-MASHO-005
- Root cause: previous test left device in wrong state

**Strategy:**
1. Run each failing test 5x in isolation -- separate timing bugs from state leakage
2. For isolation failures: tune browse timeouts or ClearSnapshot behavior
3. For sequence failures: add targeted cleanup or waiting
4. Run full group 5x shuffled, confirm stable

## Phase 3: Remaining Medium-Priority Groups

Work through groups 9, 10, 13-18 one at a time:
1. Run group 5x isolated
2. Fix isolation failures (harness or device bugs)
3. Run group 5x shuffled
4. Fix sequence failures (state leakage)

**Group 9 (Zone Wire):** TC-ZTYPE-004/005 -- x509 unknown_ca in two_zones_connected (fresh commission uses stale CA pool)
**Group 10 (Cert Validation):** TC-CERT-VAL-005, TC-CERT-VAL-CTRL-006 -- clock skew not being applied
**Group 13 (Transitions):** TC-TRANS-001/003/004, TC-E2E-001/002 -- x509 unknown_ca + EOF during borrow
**Group 14 (Security):** TC-SEC-BACKOFF-003, TC-SEC-CONN-003 -- commissioning already in progress / reconnection
**Group 15 (Subscriptions):** TC-SUB-011, TC-SUB-RESTORE-002 -- notification delivery
**Group 16 (Multi-Zone):** TC-MULTI-003 -- trigger returns status 4
**Group 17 (Conn Reaper):** TC-CONN-REAP-001/002/003 -- suite zone occupies connection slot
**Group 18 (Comm Window):** TC-COMM-WINDOW-001/002 -- mDNS not reflecting window state

## Phase 4: Full Suite Combination

1. Run all groups together 5x sequential (no shuffle)
2. Run all groups together 5x shuffled (different seeds)
3. Fix any cross-group interference
4. Repeat until 5x clean on both modes

## Verification Protocol

```bash
# Per-group verification
./stabilization/run_mash_test_fresh.sh -target localhost:8443 -setup-code 20220211 -enable-key deadbeefdeadbeefdeadbeefdeadbeef \
  -tags base-protocol -exclude-tags env:multi-device -filter "<GROUP_FILTER>" -json

# Full suite (exclude env tests)
./stabilization/run_mash_test_fresh.sh -target localhost:8443 -setup-code 20220211 -enable-key deadbeefdeadbeefdeadbeefdeadbeef \
  -tags base-protocol -exclude-tags env:multi-device -json
```

## Current Status (2026-02-15, evening)

### Phase 1 Results: 123 tests, 118 pass (95.9%), 5 fail
- x509 cascade **fully eliminated** by fixing RemoveZone routing (coordinator.go)
- SIGSEGV fixed by using `SetMain(&Connection{})` instead of `SetMain(nil)`
- TC-COMM-002 and TC-CONN-005 now PASS (were failing before)

### Phase 1 Remaining Failures (analyzed, root causes identified)
| Test | Error | Root Cause | Classification |
|------|-------|------------|----------------|
| TC-PASE-003 | state=CONNECTED, expected ADVERTISING | Device uses 85s HandshakeTimeout instead of per-phase 30s | Device bug |
| TC-CONN-BUSY-003 | key "busy_retry_after_present" not found | Device drops first PASE during 1s retry delay; second conn succeeds instead of busy | Test design |
| TC-CONN-CAP-001 | connections_opened=1, expected 3 | Suite zone occupies cap slot; handler only counts NEW connections | Test infrastructure |
| TC-CBOR-002 | response_received=false (5s timeout) | Context exhaustion: 5s budget consumed by reconnection during setup, handler runs with expired ctx | Test infrastructure |
| TC-TLS-CTRL-006 | key "tls_alert" not found (5s timeout) | Device doesn't check BasicConstraints CA flag on controller certs (Go x509.Verify skips it for clients) | Device bug |

### Fixes Applied (committed as b67db31)
1. **coordinator.go:240**: `SendRemoveZoneOnConn(suite.Conn(), suite.ZoneID())` instead of `SendRemoveZone()` -- suite conn is the live connection, pool.Main() is empty after detach
2. **device_service.go:382-415**: `buildOperationalTLSConfig` puts newest cert (`s.tlsCert`) first in TLS Certificates array
3. **device_service.go**: Added `sameTLSCert` helper to deduplicate certs
4. **commissioning.go:230, conn_mgr.go:329**: `SetMain(&Connection{})` instead of `SetMain(nil)` to prevent SIGSEGV

### Cross-cutting Issues (resolved)
- **x509 cascade**: Root cause was `sendRemoveZone()` silently failing because `pool.Main()` was empty after suite zone detach. Device accumulated stale TEST zones. Fixed by routing RemoveZone through `suite.Conn()`.
- **SIGSEGV**: Fixed by never setting pool.Main() to nil.

### Next Steps for Remaining 5 Failures
**Device bugs (fix in device code):**
- TC-PASE-003: Add per-phase 30s read deadlines in PASE handshake (types.go + commissioning flow)
- TC-TLS-CTRL-006: Add VerifyPeerCertificate callback rejecting cert.IsCA==true in operational TLS config

**Test infrastructure (fix in harness):**
- TC-CBOR-002: Increase test timeout or don't count precondition setup against test budget
- TC-CONN-CAP-001: Release suite zone before cap test or account for existing connections

**Test design (fix test logic):**
- TC-CONN-BUSY-003: Hold first PASE connection alive while step 2 runs (don't rely on 1s retry delay)
